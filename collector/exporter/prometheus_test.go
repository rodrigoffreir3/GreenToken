// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package exporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"greentoken/collector/aggregator"
	"greentoken/pb"
)

func TestPrometheusExporter(t *testing.T) {
	agg := aggregator.NewAggregator()

	// Adiciona dados mockados no agregador
	agg.AddEvent(&pb.EnergyEvent{
		AgentId:       "agent-abc",
		Hostname:      "host-xyz",
		Pid:           4321,
		Workload:      "llama.cpp",
		Model:         "phi-3",
		WattsCpu:      25.5,
		WattsDram:     5.2,
		WattsGpu:      0.0,
		GpuIndex:      -1,
		TokensInWindow: 350,
		WindowSeconds:  1.5,
	})

	handler := GetMetricsHandler(agg)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("Erro ao chamar endpoint de métricas: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("Esperava status 200 OK, obteve %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("Erro ao ler resposta: %v", err)
	}

	metricsOutput := string(body)

	// Valida se as métricas customizadas estão presentes no output do Prometheus
	expectedMetrics := []string{
		`greentoken_watts_cpu{agent_id="agent-abc",gpu_index="-1",hostname="host-xyz",model="phi-3",pid="4321",workload="llama.cpp"} 25.5`,
		`greentoken_watts_dram{agent_id="agent-abc",gpu_index="-1",hostname="host-xyz",model="phi-3",pid="4321",workload="llama.cpp"} 5.2`,
		`greentoken_watts_gpu{agent_id="agent-abc",gpu_index="-1",hostname="host-xyz",model="phi-3",pid="4321",workload="llama.cpp"} 0`,
		`greentoken_tokens_total{agent_id="agent-abc",gpu_index="-1",hostname="host-xyz",model="phi-3",pid="4321",workload="llama.cpp"} 350`,
		`greentoken_cost_per_token{agent_id="agent-abc",gpu_index="-1",hostname="host-xyz",model="phi-3",pid="4321",workload="llama.cpp"}`,
		`greentoken_joules_per_request{agent_id="agent-abc",gpu_index="-1",hostname="host-xyz",model="phi-3",pid="4321",workload="llama.cpp"} 46.05`, // (25.5+5.2+0)*1.5 = 46.05
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(metricsOutput, metric) {
			t.Errorf("Esperava encontrar a métrica '%s' no output, mas não foi encontrada.\nOutput completo:\n%s", metric, metricsOutput)
		}
	}
}
