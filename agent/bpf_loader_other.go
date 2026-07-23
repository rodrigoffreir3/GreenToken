//go:build !linux

package main

import (
	"log"
)

// ListenProcEvents fallback em sistemas não-Linux (ex: Windows ou macOS para desenvolvimento/testes locais do agent).
func ListenProcEvents(windowChan chan<- EnergyWindow) {
	log.Printf("[AGENT] Sistema não-Linux detectado. eBPF/proc desativado (modo observação local).")
	select {}
}
