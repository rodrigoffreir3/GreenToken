// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package energy

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PowercapPath permite mockar a raiz do powercap para testes
var PowercapPath = "/sys/class/powercap"

// RAPLDomain representa um domínio de telemetria RAPL mapeado no sysfs
type RAPLDomain struct {
	Name             string
	Path             string
	MaxEnergyRangeUJ int64
}

// ReadEnergyUJ lê o contador atual de energia em microjoules (uJ)
func (r *RAPLDomain) ReadEnergyUJ() (int64, error) {
	data, err := os.ReadFile(filepath.Join(r.Path, "energy_uj"))
	if err != nil {
		return 0, err
	}
	valStr := strings.TrimSpace(string(data))
	return strconv.ParseInt(valStr, 10, 64)
}

// ReadWatts calcula a média de consumo em Watts no período fornecido.
// Trata casos de wraparound usando MaxEnergyRangeUJ do domínio.
func (r *RAPLDomain) ReadWatts(d time.Duration) (float64, error) {
	startUJ, err := r.ReadEnergyUJ()
	if err != nil {
		return 0, err
	}
	time.Sleep(d)
	endUJ, err := r.ReadEnergyUJ()
	if err != nil {
		return 0, err
	}

	var delta int64
	if endUJ >= startUJ {
		delta = endUJ - startUJ
	} else {
		// Ocorreu wraparound (estouro do contador do hardware)
		if r.MaxEnergyRangeUJ > 0 {
			delta = (r.MaxEnergyRangeUJ - startUJ) + endUJ
		} else {
			delta = 0
		}
	}

	if delta < 0 {
		delta = 0
	}

	return float64(delta) / 1_000_000.0 / d.Seconds(), nil
}

// ScanDomains escaneia caminhos correspondentes a /sys/class/powercap/intel-rapl:*
// para identificar domínios RAPL expostos pelo kernel.
func ScanDomains() ([]RAPLDomain, error) {
	pattern := filepath.Join(PowercapPath, "intel-rapl:*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var domains []RAPLDomain
	for _, match := range matches {
		nameBytes, err := os.ReadFile(filepath.Join(match, "name"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameBytes))

		var maxUJ int64
		maxBytes, err := os.ReadFile(filepath.Join(match, "max_energy_range_uj"))
		if err == nil {
			if parsed, pErr := strconv.ParseInt(strings.TrimSpace(string(maxBytes)), 10, 64); pErr == nil {
				maxUJ = parsed
			}
		}

		domains = append(domains, RAPLDomain{
			Name:             name,
			Path:             match,
			MaxEnergyRangeUJ: maxUJ,
		})
	}

	return domains, nil
}
