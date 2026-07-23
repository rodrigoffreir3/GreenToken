//go:build linux

package main

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -I/usr/include/x86_64-linux-gnu" bpf bpf/sched_monitor.c

// ListenProcEvents avalia suporte eBPF, acionando fallback silencioso se falhar.
func ListenProcEvents(windowChan chan<- EnergyWindow) {
	err := startBPFListener(windowChan)
	if err != nil {
		log.Printf("eBPF inacessível ou sem suporte (%v). Acionando graceful fallback para passivo...", err)
		startPassiveProcScanner(windowChan)
	}
}

func startBPFListener(windowChan chan<- EnergyWindow) error {
	objs := &bpfObjects{}
	if err := loadBpfObjects(objs, nil); err != nil {
		return err
	}
	defer objs.Close()

	// Anexa ao tracepoint sched/sched_switch
	tp, err := link.Tracepoint("sched", "sched_switch", objs.HandleSchedSwitch, nil)
	if err != nil {
		return err
	}
	defer tp.Close()

	// Abre o ring buffer do BPF
	rd, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		return err
	}
	defer rd.Close()

	log.Println("eBPF: Monitorando sched_switch para GreenToken...")

	for {
		record, err := rd.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return nil
			}
			log.Printf("Erro lendo do ringbuf eBPF: %v", err)
			continue
		}

		event, ok := DecodeBPFEvent(record.RawSample)
		if !ok {
			continue
		}

		select {
		case windowChan <- event:
		default:
			// Evita travar se o canal estiver temporariamente cheio
		}
	}
}

func startPassiveProcScanner(windowChan chan<- EnergyWindow) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Rastreia o tempo acumulado anterior (jiffies) por PID
	prevCPUTimes := make(map[uint32]uint64)

	// CLK_TCK padrão do Linux é 100
	const clkTck = 100
	const nsPerTick = 1000000000 / clkTck

	for range ticker.C {
		files, err := os.ReadDir("/proc")
		if err != nil {
			continue
		}

		for _, f := range files {
			if !f.IsDir() {
				continue
			}
			pidVal, err := strconv.ParseUint(f.Name(), 10, 32)
			if err != nil {
				continue // Ignora o que não for numérico (diretório de processo)
			}
			pid := uint32(pidVal)

			// Lê /proc/<pid>/stat
			statBytes, err := os.ReadFile("/proc/" + f.Name() + "/stat")
			if err != nil {
				continue
			}

			// Procura os delimitadores do comando para contornar nomes com espaços
			idxStart := bytes.IndexByte(statBytes, '(')
			idxEnd := bytes.LastIndexByte(statBytes, ')')
			if idxStart < 0 || idxEnd < 0 || idxEnd <= idxStart {
				continue
			}

			comm := string(statBytes[idxStart+1 : idxEnd])
			rest := strings.Fields(string(statBytes[idxEnd+1:]))
			if len(rest) < 13 { // utime é o 12º elemento do resto e stime é o 13º
				continue
			}

			utime, err1 := strconv.ParseUint(rest[11], 10, 64)
			stime, err2 := strconv.ParseUint(rest[12], 10, 64)
			if err1 != nil || err2 != nil {
				continue
			}

			totalJiffies := utime + stime
			prev, exists := prevCPUTimes[pid]
			prevCPUTimes[pid] = totalJiffies

			if exists && totalJiffies > prev {
				deltaJiffies := totalJiffies - prev
				deltaNs := deltaJiffies * nsPerTick

				select {
				case windowChan <- EnergyWindow{
					Pid:     pid,
					Tgid:    pid, // No fallback passivo, simplificamos tgid como o próprio pid
					OnCpuNs: deltaNs,
					Comm:    comm,
				}:
				default:
					// Evita travar se o canal de destino estiver cheio
				}
			}
		}
	}
}
