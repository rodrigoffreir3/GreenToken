package main

import (
	"bytes"
	"encoding/binary"
	"sync"

	"greentoken/pb"
)

// EnergyWindow representa a janela temporal de uso de CPU de uma thread/processo vinda do eBPF.
type EnergyWindow struct {
	Pid     uint32
	Tgid    uint32
	OnCpuNs uint64
	Comm    string
}

// DecodeBPFEvent decodifica o evento binário vindo do ringbuffer eBPF.
// struct energy_window_t {
//     __u32 pid;          // 0:4
//     __u32 tgid;         // 4:8
//     __u64 on_cpu_ns;    // 8:16
//     char  comm[16];     // 16:32
// } __attribute__((packed)); => 32 bytes
func DecodeBPFEvent(rawSample []byte) (EnergyWindow, bool) {
	if len(rawSample) < 32 {
		return EnergyWindow{}, false
	}

	pid := binary.LittleEndian.Uint32(rawSample[0:4])
	tgid := binary.LittleEndian.Uint32(rawSample[4:8])
	onCpuNs := binary.LittleEndian.Uint64(rawSample[8:16])

	commBytes := rawSample[16:32]
	if idx := bytes.IndexByte(commBytes, 0); idx >= 0 {
		commBytes = commBytes[:idx]
	}
	comm := string(commBytes)

	return EnergyWindow{
		Pid:     pid,
		Tgid:    tgid,
		OnCpuNs: onCpuNs,
		Comm:    comm,
	}, true
}

// RingBuffer: buffer circular em memória para armazenar EnergyEvent do gRPC.
type RingBuffer struct {
	events []*pb.EnergyEvent
	head   int
	tail   int
	count  int
	size   int
	mu     sync.Mutex
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		events: make([]*pb.EnergyEvent, size),
		size:   size,
	}
}

// Enqueue empurra o evento e implementa a lógica FIFO (sobrescrevendo se lotar)
func (b *RingBuffer) Enqueue(e *pb.EnergyEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events[b.tail] = e
	b.tail = (b.tail + 1) % b.size

	if b.count < b.size {
		b.count++
	} else {
		b.head = (b.head + 1) % b.size
	}
}

// DequeueAll puxa todos em lote
func (b *RingBuffer) DequeueAll() []*pb.EnergyEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == 0 {
		return nil
	}

	batch := make([]*pb.EnergyEvent, 0, b.count)
	for i := 0; i < b.count; i++ {
		idx := (b.head + i) % b.size
		batch = append(batch, b.events[idx])
		b.events[idx] = nil
	}

	b.head = 0
	b.tail = 0
	b.count = 0

	return batch
}
