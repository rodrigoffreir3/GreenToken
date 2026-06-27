//go:build gpu

package gpu

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// Init inicializa a biblioteca NVML.
func Init() error {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("falha ao inicializar NVML: %s", nvml.ErrorString(ret))
	}
	return nil
}

// Shutdown finaliza o uso da biblioteca NVML.
func Shutdown() error {
	ret := nvml.Shutdown()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("falha ao finalizar NVML: %s", nvml.ErrorString(ret))
	}
	return nil
}

// GetDeviceCount retorna o número de GPUs Nvidia no sistema.
func GetDeviceCount() (int, error) {
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return 0, fmt.Errorf("falha ao obter quantidade de dispositivos GPU: %s", nvml.ErrorString(ret))
	}
	return int(count), nil
}

// GetDevicePowerUsage retorna o consumo instantâneo da GPU em Watts (convertido de mW).
func GetDevicePowerUsage(index int) (float64, error) {
	device, ret := nvml.DeviceGetHandleByIndex(index)
	if ret != nvml.SUCCESS {
		return 0.0, fmt.Errorf("falha ao obter handle da GPU %d: %s", index, nvml.ErrorString(ret))
	}

	powerMW, ret := nvml.DeviceGetPowerUsage(device)
	if ret != nvml.SUCCESS {
		return 0.0, fmt.Errorf("falha ao obter consumo de energia da GPU %d: %s", index, nvml.ErrorString(ret))
	}

	// Converte miliwatts (mW) para Watts (W) usando a função comum
	return milliwattsToWatts(powerMW), nil
}

// GetDeviceMemoryUsage retorna a memória de GPU em bytes: (usada, total).
func GetDeviceMemoryUsage(index int) (uint64, uint64, error) {
	device, ret := nvml.DeviceGetHandleByIndex(index)
	if ret != nvml.SUCCESS {
		return 0, 0, fmt.Errorf("falha ao obter handle da GPU %d: %s", index, nvml.ErrorString(ret))
	}

	memInfo, ret := nvml.DeviceGetMemoryInfo(device)
	if ret != nvml.SUCCESS {
		return 0, 0, fmt.Errorf("falha ao obter informacoes de memoria da GPU %d: %s", index, nvml.ErrorString(ret))
	}

	return memInfo.Used, memInfo.Total, nil
}

// GetRunningProcesses retorna a lista de PIDs rodando na GPU (combina computação e gráficos).
func GetRunningProcesses(index int) ([]uint32, error) {
	device, ret := nvml.DeviceGetHandleByIndex(index)
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("falha ao obter handle da GPU %d: %s", index, nvml.ErrorString(ret))
	}

	var pids []uint32
	seen := make(map[uint32]bool)

	// Processos de Computação
	compProcs, ret := nvml.DeviceGetComputeRunningProcesses(device)
	if ret == nvml.SUCCESS {
		for _, proc := range compProcs {
			if !seen[proc.Pid] {
				seen[proc.Pid] = true
				pids = append(pids, proc.Pid)
			}
		}
	}

	// Processos Gráficos
	graphProcs, ret := nvml.DeviceGetGraphicsRunningProcesses(device)
	if ret == nvml.SUCCESS {
		for _, proc := range graphProcs {
			if !seen[proc.Pid] {
				seen[proc.Pid] = true
				pids = append(pids, proc.Pid)
			}
		}
	}

	return pids, nil
}
