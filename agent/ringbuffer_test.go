package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"greentoken/pb"
)

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(3)

	ev1 := &pb.EnergyEvent{Workload: "vllm-1"}
	ev2 := &pb.EnergyEvent{Workload: "vllm-2"}
	ev3 := &pb.EnergyEvent{Workload: "vllm-3"}
	ev4 := &pb.EnergyEvent{Workload: "vllm-4"}

	rb.Enqueue(ev1)
	rb.Enqueue(ev2)
	rb.Enqueue(ev3)

	if rb.count != 3 {
		t.Errorf("Esperava tamanho 3, obteve %d", rb.count)
	}

	// Deve sobrescrever ev1
	rb.Enqueue(ev4)

	batch := rb.DequeueAll()
	if len(batch) != 3 {
		t.Fatalf("Esperava lote de tamanho 3, obteve %d", len(batch))
	}

	if batch[0].Workload != "vllm-2" || batch[1].Workload != "vllm-3" || batch[2].Workload != "vllm-4" {
		t.Errorf("Ordem FIFO incorreta, obteve %v", batch)
	}

	if rb.count != 0 {
		t.Errorf("Esperava 0 elementos após DequeueAll, obteve %d", rb.count)
	}
}

func TestDecodeBPFEvent(t *testing.T) {
	buf := new(bytes.Buffer)

	// Escrever dados conhecidos
	var pid uint32 = 1234
	var tgid uint32 = 5678
	var onCpuNs uint64 = 987654321
	commBytes := [16]byte{'m', 'y', '-', 'l', 'l', 'm', 0}

	binary.Write(buf, binary.LittleEndian, pid)
	binary.Write(buf, binary.LittleEndian, tgid)
	binary.Write(buf, binary.LittleEndian, onCpuNs)
	buf.Write(commBytes[:])

	// Decodificar
	window, ok := DecodeBPFEvent(buf.Bytes())
	if !ok {
		t.Fatalf("Falha ao decodificar evento eBPF válido")
	}

	if window.Pid != pid {
		t.Errorf("PID incorreto: esperado %d, obtido %d", pid, window.Pid)
	}
	if window.Tgid != tgid {
		t.Errorf("TGID incorreto: esperado %d, obtido %d", tgid, window.Tgid)
	}
	if window.OnCpuNs != onCpuNs {
		t.Errorf("on_cpu_ns incorreto: esperado %d, obtido %d", onCpuNs, window.OnCpuNs)
	}
	if window.Comm != "my-llm" {
		t.Errorf("Comm incorreto: esperado 'my-llm', obtido %q", window.Comm)
	}
}
