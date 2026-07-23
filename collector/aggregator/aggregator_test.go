// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aggregator

import (
	"testing"
	"time"

	"greentoken/pb"
)

func TestAggregatorAddEvent(t *testing.T) {
	agg := NewAggregator()

	// 1. Adicionar primeiro evento
	ev1 := &pb.EnergyEvent{
		AgentId:       "agent-1",
		Hostname:      "host-1",
		Pid:           1234,
		Workload:      "vllm",
		Model:         "llama3",
		WattsCpu:      50.0,
		WattsDram:     10.0,
		WattsGpu:      100.0,
		GpuIndex:      0,
		TokensInWindow: 100,
		WindowSeconds:  2.0,
	}

	agg.AddEvent(ev1)
	states := agg.GetStates()
	if len(states) != 1 {
		t.Fatalf("Esperava 1 estado de workload, obteve %d", len(states))
	}

	state := states[0]
	if state.Pid != 1234 || state.Workload != "vllm" || state.Model != "llama3" {
		t.Errorf("Identificadores incorretos no estado: %+v", state)
	}

	if state.WattsCpu != 50.0 || state.WattsDram != 10.0 || state.WattsGpu != 100.0 {
		t.Errorf("Primeiro evento devia definir as métricas diretamente, obteve: CPU=%f, DRAM=%f, GPU=%f", state.WattsCpu, state.WattsDram, state.WattsGpu)
	}

	expectedJoules := (50.0 + 10.0 + 100.0) * 2.0 // 320 Joules
	if state.JoulesPerRequest != expectedJoules {
		t.Errorf("Esperava JoulesPerRequest = %f, obteve %f", expectedJoules, state.JoulesPerRequest)
	}

	if state.TokensTotal != 100 {
		t.Errorf("Esperava TokensTotal = 100, obteve %d", state.TokensTotal)
	}

	// 2. Adicionar segundo evento para calcular EMA
	ev2 := &pb.EnergyEvent{
		AgentId:       "agent-1",
		Hostname:      "host-1",
		Pid:           1234,
		Workload:      "vllm",
		Model:         "llama3",
		WattsCpu:      60.0,
		WattsDram:     12.0,
		WattsGpu:      120.0,
		GpuIndex:      0,
		TokensInWindow: 200,
		WindowSeconds:  2.0,
	}

	agg.AddEvent(ev2)
	states = agg.GetStates()
	state = states[0]

	// EMA formula: (novo * Alpha) + (anterior * (1 - Alpha))
	expectedCpu := (60.0 * Alpha) + (50.0 * (1 - Alpha))
	if state.WattsCpu != expectedCpu {
		t.Errorf("EMA WattsCpu incorreto. Esperava %f, obteve %f", expectedCpu, state.WattsCpu)
	}

	expectedTokens := int64(300)
	if state.TokensTotal != expectedTokens {
		t.Errorf("TokensTotal acumulados incorretos. Esperava %d, obteve %d", expectedTokens, state.TokensTotal)
	}

	// 3. Testar isolamento por PID/workload/model
	ev3 := &pb.EnergyEvent{
		AgentId:       "agent-1",
		Hostname:      "host-1",
		Pid:           5555,
		Workload:      "ollama",
		Model:         "mistral",
		WattsCpu:      20.0,
		WattsDram:     5.0,
		WattsGpu:      0.0,
		GpuIndex:      -1,
		TokensInWindow: 50,
		WindowSeconds:  1.0,
	}

	agg.AddEvent(ev3)
	states = agg.GetStates()
	if len(states) != 2 {
		t.Fatalf("Esperava 2 estados de workload separados, obteve %d", len(states))
	}

	var foundOllama, foundVllm bool
	for _, st := range states {
		if st.Workload == "ollama" {
			foundOllama = true
			if st.Pid != 5555 || st.Model != "mistral" {
				t.Errorf("Estado do ollama corrompido: %+v", st)
			}
		} else if st.Workload == "vllm" {
			foundVllm = true
		}
	}

	if !foundOllama || !foundVllm {
		t.Errorf("Faltou encontrar algum dos workloads agregados: ollama=%v, vllm=%v", foundOllama, foundVllm)
	}
}

func TestAggregatorStaleCleanup(t *testing.T) {
	// Apenas para verificar se o LastUpdated é preenchido
	agg := NewAggregator()
	ev := &pb.EnergyEvent{
		Pid:      9999,
		Workload: "vllm",
		Model:    "phi3",
	}
	agg.AddEvent(ev)
	states := agg.GetStates()
	if len(states) == 0 {
		t.Fatal("Esperava estado cadastrado")
	}
	if states[0].LastUpdated.IsZero() {
		t.Error("LastUpdated devia estar preenchido")
	}
	if time.Since(states[0].LastUpdated) > time.Second {
		t.Error("LastUpdated devia ser recente")
	}
}
