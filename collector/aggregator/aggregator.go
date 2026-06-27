package aggregator

import (
	"fmt"
	"sync"
	"time"

	"greentoken/pb"
)

// Alpha é o fator de suavização para a média móvel exponencial (EMA).
// Um valor de 0.2 significa que o novo evento contribui com 20% e o histórico com 80%.
const Alpha = 0.2

// WorkloadState armazena o estado das métricas agregadas de um workload específico.
type WorkloadState struct {
	Pid              int32
	Workload         string
	Model            string
	AgentID          string
	Hostname         string
	GpuIndex         int32
	WattsCpu         float64
	WattsDram        float64
	WattsGpu         float64
	CostPerToken     float64
	JoulesPerRequest float64
	TokensTotal      int64
	LastUpdated      time.Time
	initialized      bool
}

// Aggregator gerencia a agregação de métricas de múltiplos agentes e workloads de forma thread-safe.
type Aggregator struct {
	mu     sync.RWMutex
	states map[string]*WorkloadState
}

// NewAggregator inicializa um novo Aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{
		states: make(map[string]*WorkloadState),
	}
}

// AddEvent processa um novo EnergyEvent e atualiza a média móvel correspondente.
func (a *Aggregator) AddEvent(event *pb.EnergyEvent) {
	if event == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Chave única baseada em PID, Workload e Model para rastreamento preciso
	key := fmt.Sprintf("%d-%s-%s", event.Pid, event.Workload, event.Model)

	state, exists := a.states[key]
	if !exists {
		state = &WorkloadState{
			Pid:      event.Pid,
			Workload: event.Workload,
			Model:    event.Model,
			AgentID:  event.AgentId,
			Hostname: event.Hostname,
			GpuIndex: event.GpuIndex,
		}
		a.states[key] = state
	}

	wattsTotal := event.WattsCpu + event.WattsDram + event.WattsGpu
	currentJoules := CalculateJoules(wattsTotal, event.WindowSeconds)
	pricePerKWh := GetKWhPrice()
	currentCost := CalculateCost(currentJoules, pricePerKWh)

	if !state.initialized {
		state.WattsCpu = event.WattsCpu
		state.WattsDram = event.WattsDram
		state.WattsGpu = event.WattsGpu
		state.JoulesPerRequest = currentJoules

		if event.TokensInWindow > 0 {
			state.CostPerToken = CalculateCostPerToken(currentCost, event.TokensInWindow)
		} else {
			state.CostPerToken = 0
		}

		state.initialized = true
	} else {
		// Atualiza médias móveis usando EMA (Exponential Moving Average)
		state.WattsCpu = (event.WattsCpu * Alpha) + (state.WattsCpu * (1 - Alpha))
		state.WattsDram = (event.WattsDram * Alpha) + (state.WattsDram * (1 - Alpha))
		state.WattsGpu = (event.WattsGpu * Alpha) + (state.WattsGpu * (1 - Alpha))
		state.JoulesPerRequest = (currentJoules * Alpha) + (state.JoulesPerRequest * (1 - Alpha))

		// Apenas atualiza a média de custo por token se houver tokens gerados nesta janela
		if event.TokensInWindow > 0 {
			currentCostPerToken := CalculateCostPerToken(currentCost, event.TokensInWindow)
			if state.CostPerToken == 0 {
				state.CostPerToken = currentCostPerToken
			} else {
				state.CostPerToken = (currentCostPerToken * Alpha) + (state.CostPerToken * (1 - Alpha))
			}
		}
	}

	state.TokensTotal += event.TokensInWindow
	state.LastUpdated = time.Now()
	// Atualiza metadados dinâmicos
	state.AgentID = event.AgentId
	state.Hostname = event.Hostname
	state.GpuIndex = event.GpuIndex
}

// GetStates retorna uma cópia de todos os estados atuais dos workloads.
func (a *Aggregator) GetStates() []WorkloadState {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]WorkloadState, 0, len(a.states))
	for _, state := range a.states {
		result = append(result, *state)
	}
	return result
}
