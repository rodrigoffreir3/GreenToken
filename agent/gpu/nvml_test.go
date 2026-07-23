// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gpu

import (
	"testing"
)

func TestMilliwattsToWatts(t *testing.T) {
	tests := []struct {
		mw       uint32
		expected float64
	}{
		{0, 0.0},
		{1000, 1.0},
		{2500, 2.5},
		{500, 0.5},
		{123456, 123.456},
	}

	for _, tt := range tests {
		result := milliwattsToWatts(tt.mw)
		if result != tt.expected {
			t.Errorf("milliwattsToWatts(%d) = %f; esperado %f", tt.mw, result, tt.expected)
		}
	}
}

func TestGPUStubs(t *testing.T) {
	// Inicialização da biblioteca NVML.
	err := Init()
	if err != nil {
		// Se a biblioteca/driver real não estiver disponível (ex: ERROR_LIBRARY_NOT_FOUND),
		// pulamos graciosamente os testes reais de GPU.
		t.Skipf("NVML Init falhou (esperado em ambientes sem driver/biblioteca NVIDIA): %v", err)
		return
	}
	defer func() {
		_ = Shutdown()
	}()

	count, err := GetDeviceCount()
	if err != nil {
		t.Fatalf("GetDeviceCount() falhou: %v", err)
	}

	// Se houver dispositivos ou se estiver rodando o stub, executamos validações básicas
	if count == 0 {
		power, err := GetDevicePowerUsage(0)
		if err != nil {
			t.Errorf("GetDevicePowerUsage(0) falhou: %v", err)
		}
		if power != 0.0 {
			t.Errorf("Esperado power 0.0 no stub/sem devices, obtido: %f", power)
		}

		used, total, err := GetDeviceMemoryUsage(0)
		if err != nil {
			t.Errorf("GetDeviceMemoryUsage(0) falhou: %v", err)
		}
		if used != 0 || total != 0 {
			t.Errorf("Esperado memory (0, 0) no stub/sem devices, obtido: (%d, %d)", used, total)
		}

		procs, err := GetRunningProcesses(0)
		if err != nil {
			t.Errorf("GetRunningProcesses(0) falhou: %v", err)
		}
		if procs != nil {
			t.Errorf("Esperado lista de processos nula no stub/sem devices, obtido: %v", procs)
		}

		pidMap, err := MapPIDsToGPUs()
		if err != nil {
			t.Errorf("MapPIDsToGPUs() falhou: %v", err)
		}
		if len(pidMap) != 0 {
			t.Errorf("Esperado pidMap vazio no stub/sem devices, obtido: %v", pidMap)
		}
	} else {
		// Se houver pelo menos uma GPU real detectada no ambiente
		t.Logf("NVML inicializado com sucesso. %d GPU(s) detectada(s).", count)
		for i := 0; i < count; i++ {
			power, err := GetDevicePowerUsage(i)
			if err != nil {
				t.Errorf("Falha ao ler consumo da GPU %d: %v", i, err)
			}
			t.Logf("GPU %d - Consumo: %f W", i, power)

			used, total, err := GetDeviceMemoryUsage(i)
			if err != nil {
				t.Errorf("Falha ao ler memoria da GPU %d: %v", i, err)
			}
			t.Logf("GPU %d - Memória: %d / %d bytes", i, used, total)

			_, err = GetRunningProcesses(i)
			if err != nil {
				t.Errorf("Falha ao ler processos da GPU %d: %v", i, err)
			}
		}

		pidMap, err := MapPIDsToGPUs()
		if err != nil {
			t.Fatalf("MapPIDsToGPUs() falhou: %v", err)
		}
		t.Logf("Mapeamento PID -> GPU: %v", pidMap)
	}
}
