package gpu

// MapPIDsToGPUs mapeia PIDs para o índice da GPU onde estão rodando.
// Retorna um mapa onde a chave é o PID e o valor é o índice da GPU.
func MapPIDsToGPUs() (map[uint32]int, error) {
	count, err := GetDeviceCount()
	if err != nil {
		return nil, err
	}

	pidMap := make(map[uint32]int)
	for i := 0; i < count; i++ {
		pids, err := GetRunningProcesses(i)
		if err != nil {
			continue
		}
		for _, pid := range pids {
			pidMap[pid] = i
		}
	}
	return pidMap, nil
}

// milliwattsToWatts converte miliwatts (mW) para Watts (W)
func milliwattsToWatts(mw uint32) float64 {
	return float64(mw) / 1000.0
}
