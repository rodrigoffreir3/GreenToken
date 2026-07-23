// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !gpu

package gpu

// Init inicializa a biblioteca NVML (no stub, não faz nada).
func Init() error {
	return nil
}

// GPUSupported retorna se o binário foi compilado com suporte real a GPU.
func GPUSupported() bool {
	return false
}

// Shutdown finaliza o uso da biblioteca NVML (no stub, não faz nada).
func Shutdown() error {
	return nil
}

// GetDeviceCount retorna o número de dispositivos GPU detectados (no stub, retorna 0).
func GetDeviceCount() (int, error) {
	return 0, nil
}

// GetDevicePowerUsage retorna o consumo atual da GPU em Watts (no stub, retorna 0).
func GetDevicePowerUsage(index int) (float64, error) {
	return 0.0, nil
}

// GetDeviceMemoryUsage retorna a memória usada e total em bytes (no stub, retorna 0, 0).
func GetDeviceMemoryUsage(index int) (uint64, uint64, error) {
	return 0, 0, nil
}

// GetRunningProcesses retorna a lista de PIDs em execução no dispositivo GPU especificado (no stub, retorna nil).
func GetRunningProcesses(index int) ([]uint32, error) {
	return nil, nil
}
