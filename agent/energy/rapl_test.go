// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package energy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadEnergyUJ(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rapl_test")
	defer os.RemoveAll(tmpDir)

	RaplBasePath = tmpDir
	os.WriteFile(filepath.Join(tmpDir, "energy_uj"), []byte("2564000\n"), 0644)

	val, err := ReadEnergyUJ()
	if err != nil {
		t.Fatalf("A leitura via kernel handler falhou simuladamente: %v", err)
	}

	if val != 2564000 {
		t.Errorf("Parse RAPL uJ incorreto: %d", val)
	}
}

func TestFallbackRAPL(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rapl_test_empty")
	defer os.RemoveAll(tmpDir)
	RaplBasePath = tmpDir

	// Iniciar a goroutine deve falhar silenciosamente (sem panic)
	StartRAPLTelemetry()
}

func TestScanDomains(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "powercap_test")
	defer os.RemoveAll(tmpDir)

	oldPowercapPath := PowercapPath
	PowercapPath = tmpDir
	defer func() { PowercapPath = oldPowercapPath }()

	// Criando dominios simulados
	d1 := filepath.Join(tmpDir, "intel-rapl:0")
	_ = os.MkdirAll(d1, 0755)
	_ = os.WriteFile(filepath.Join(d1, "name"), []byte("package-0\n"), 0644)
	_ = os.WriteFile(filepath.Join(d1, "max_energy_range_uj"), []byte("262143328850\n"), 0644)
	_ = os.WriteFile(filepath.Join(d1, "energy_uj"), []byte("5000000\n"), 0644)

	d2 := filepath.Join(tmpDir, "intel-rapl:0:1")
	_ = os.MkdirAll(d2, 0755)
	_ = os.WriteFile(filepath.Join(d2, "name"), []byte("dram\n"), 0644)
	_ = os.WriteFile(filepath.Join(d2, "max_energy_range_uj"), []byte("262143328850\n"), 0644)
	_ = os.WriteFile(filepath.Join(d2, "energy_uj"), []byte("2000000\n"), 0644)

	domains, err := ScanDomains()
	if err != nil {
		t.Fatalf("ScanDomains falhou: %v", err)
	}

	if len(domains) != 2 {
		t.Errorf("Esperava 2 dominios, obteve %d", len(domains))
	}

	foundPackage := false
	foundDRAM := false
	for _, d := range domains {
		if d.Name == "package-0" {
			foundPackage = true
			if d.MaxEnergyRangeUJ != 262143328850 {
				t.Errorf("max_energy_range_uj incorreto para package-0: %d", d.MaxEnergyRangeUJ)
			}
		}
		if d.Name == "dram" {
			foundDRAM = true
		}
	}

	if !foundPackage || !foundDRAM {
		t.Errorf("Dominios esperados nao encontrados. foundPackage: %t, foundDRAM: %t", foundPackage, foundDRAM)
	}
}

func TestRAPLDomainReadWatts(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "rapl_domain_test")
	defer os.RemoveAll(tmpDir)

	_ = os.WriteFile(filepath.Join(tmpDir, "name"), []byte("dram\n"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "max_energy_range_uj"), []byte("10000000\n"), 0644)

	domain := RAPLDomain{
		Name:             "dram",
		Path:             tmpDir,
		MaxEnergyRangeUJ: 10000000,
	}

	// 1. Sem wraparound: 5000000 uJ -> 7000000 uJ (delta = 2000000 uJ)
	_ = os.WriteFile(filepath.Join(tmpDir, "energy_uj"), []byte("5000000\n"), 0644)
	go func() {
		time.Sleep(5 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(tmpDir, "energy_uj"), []byte("7000000\n"), 0644)
	}()

	watts, err := domain.ReadWatts(10 * time.Millisecond)
	if err != nil {
		t.Fatalf("Erro ao ler watts: %v", err)
	}
	if watts <= 0 {
		t.Errorf("Consumo esperado maior que 0, obtido: %f", watts)
	}

	// 2. Com wraparound: 8000000 uJ -> 2000000 uJ
	// max_energy_range_uj = 10000000 uJ.
	// delta = (10000000 - 8000000) + 2000000 = 4000000 uJ
	_ = os.WriteFile(filepath.Join(tmpDir, "energy_uj"), []byte("8000000\n"), 0644)
	go func() {
		time.Sleep(5 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(tmpDir, "energy_uj"), []byte("2000000\n"), 0644)
	}()

	wattsWrap, err := domain.ReadWatts(10 * time.Millisecond)
	if err != nil {
		t.Fatalf("Erro ao ler watts com wraparound: %v", err)
	}
	if wattsWrap <= 0 {
		t.Errorf("Consumo esperado com wraparound maior que 0, obtido: %f", wattsWrap)
	}
}
