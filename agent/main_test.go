package main

import (
	"testing"
)

type MockTokenSource struct {
	values []int64
	index  int
}

func (m *MockTokenSource) CumulativeTokens() (int64, error) {
	if m.index < len(m.values) {
		val := m.values[m.index]
		m.index++
		return val, nil
	}
	return m.values[len(m.values)-1], nil
}

func (m *MockTokenSource) Name() string {
	return "mock"
}

func TestCollectAndEnqueue_DeltaCumulative(t *testing.T) {
	// Setup global state for matching
	*targetPID = 1234
	*workloadName = "test-workload"
	
	cpuTimeMapMu.Lock()
	cpuTimeMap[1234] = &PIDStats{comm: "test-proc", cpuNs: 1000000, gpuIdx: -1}
	cpuTimeMapMu.Unlock()

	rb := NewRingBuffer(100)
	
	// Mock Source
	mockSource := &MockTokenSource{
		values: []int64{100, 250, 30}, // 100(initial), 250(normal delta), 30(restart)
	}

	var previousCumulative int64
	var baselineEstablished bool
	initial, _ := mockSource.CumulativeTokens()
	previousCumulative = initial
	baselineEstablished = true

	if previousCumulative != 100 {
		t.Fatalf("Esperava 100, obteve %d", previousCumulative)
	}

	// Janela 1: 100 -> 250 (delta 150)
	collectAndEnqueue("localhost", rb, mockSource, &previousCumulative, &baselineEstablished)
	
	events := rb.DequeueAll()
	if len(events) != 1 {
		t.Fatalf("Esperava 1 evento, obteve %d", len(events))
	}
	
	ev := events[0]
	if ev.TokensInWindow != 150 {
		t.Errorf("Esperava 150 tokens na janela, obteve %d", ev.TokensInWindow)
	}
	if previousCumulative != 250 {
		t.Errorf("Esperava previousCumulative=250, obteve %d", previousCumulative)
	}

	// Restaurar estatísticas para que o processo apareça de novo na próxima janela
	cpuTimeMapMu.Lock()
	cpuTimeMap[1234].cpuNs = 1000000
	cpuTimeMapMu.Unlock()

	// Janela 2: Engine Restart: 250 -> 30 (delta 0, ou seja, protegido contra negativo)
	// Como o TokenSource mockado retorna 30, o delta seria 30 - 250 = -220,
	// mas main.go protege com < 0 -> 0.
	collectAndEnqueue("localhost", rb, mockSource, &previousCumulative, &baselineEstablished)
	
	events2 := rb.DequeueAll()
	if len(events2) != 1 {
		t.Fatalf("Esperava 1 evento na janela 2, obteve %d", len(events2))
	}
	
	ev2 := events2[0]
	if ev2.TokensInWindow != 0 {
		t.Errorf("Esperava 0 tokens (proteção negativo), obteve %d", ev2.TokensInWindow)
	}
	if previousCumulative != 30 {
		t.Errorf("Esperava previousCumulative=30 apos restart, obteve %d", previousCumulative)
	}
}
