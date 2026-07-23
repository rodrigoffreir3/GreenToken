// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
