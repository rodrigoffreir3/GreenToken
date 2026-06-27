package energy

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var RaplBasePath = "/sys/class/powercap/intel-rapl/intel-rapl:0"

func ReadEnergyUJ() (int64, error) {
	data, err := os.ReadFile(filepath.Join(RaplBasePath, "energy_uj"))
	if err != nil {
		return 0, err
	}
	valStr := strings.TrimSpace(string(data))
	return strconv.ParseInt(valStr, 10, 64)
}

// StartRAPLTelemetry mantém um ticker de leitura passivo c/ feedback em logs
func StartRAPLTelemetry() {
	_, err := ReadEnergyUJ()
	if err != nil {
		log.Printf("RAPL Energy Telemetry não reportada do kernel. Graceful fallback (desativado).")
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastUJ int64
	lastUJ, _ = ReadEnergyUJ()

	for range ticker.C {
		currentUJ, err := ReadEnergyUJ()
		if err == nil {
			deltaUJ := currentUJ - lastUJ
			if deltaUJ < 0 {
				lastUJ = currentUJ
				continue
			}
			watts := float64(deltaUJ) / 1_000_000.0 / 2.0 // uJ -> J / 2s -> W
			if watts > 0 {
				log.Printf("[RAPL ENERGY] Consumo de Pacote Estimado: %.2f W", watts)
			}
			lastUJ = currentUJ
		}
	}
}

// ReadWatts mede o consumo médio em Watts ao longo de uma duração fornecida
func ReadWatts(d time.Duration) float64 {
	startUJ, err := ReadEnergyUJ()
	if err != nil {
		return 0.0 // Graceful fallback
	}
	time.Sleep(d)
	endUJ, err := ReadEnergyUJ()
	if err != nil {
		return 0.0
	}
	delta := endUJ - startUJ
	if delta < 0 {
		return 0.0
	}
	return float64(delta) / 1_000_000.0 / d.Seconds()
}
