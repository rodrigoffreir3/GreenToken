package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"greentoken/agent/energy"
	"greentoken/agent/gpu"
	"greentoken/agent/tokens"
	"greentoken/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	collectorURL = flag.String("collector", "localhost:50051", "Endereço do collector gRPC (host:port)")
	agentID      = flag.String("id", "agent-default", "Identificador único deste agente")
	workloadName = flag.String("workload", "vllm", "Nome do processo do workload alvo (ex: vllm, python3, ollama)")
	modelName    = flag.String("model", "llama3", "Nome do modelo de IA (ex: llama3-8b)")
	logFilePath  = flag.String("log-file", "", "Caminho do arquivo de log da engine de inferência para sniffar tokens")
	windowSecs   = flag.Int("interval", 2, "Janela de medição em segundos")
)

type PIDStats struct {
	comm    string
	cpuNs   uint64
	gpuIdx  int
	gpuMem  uint64
}

var (
	accumulatedTokens int64
	windowChan        = make(chan EnergyWindow, 10000)
	cpuTimeMap        = make(map[uint32]*PIDStats)
	cpuTimeMapMu      sync.Mutex
)

func main() {
	flag.Parse()

	// Substituir flags com variáveis de ambiente se presentes
	if env := os.Getenv("GT_COLLECTOR_URL"); env != "" {
		*collectorURL = env
	}
	if env := os.Getenv("GT_AGENT_ID"); env != "" {
		*agentID = env
	}
	if env := os.Getenv("GT_WORKLOAD_NAME"); env != "" {
		*workloadName = env
	}
	if env := os.Getenv("GT_MODEL_NAME"); env != "" {
		*modelName = env
	}
	if env := os.Getenv("GT_LOG_FILE"); env != "" {
		*logFilePath = env
	}

	log.Printf("GreenToken Agent iniciando...")
	log.Printf("Configurações: ID=%s, Collector=%s, Workload=%s, Modelo=%s, LogFile=%s, Intervalo=%ds",
		*agentID, *collectorURL, *workloadName, *modelName, *logFilePath, *windowSecs)

	// Inicializa eBPF/Proc
	go ListenProcEvents(windowChan)

	// Consome eventos de scheduling da CPU enviados pelo BPF ou /proc scanner
	go func() {
		for event := range windowChan {
			cpuTimeMapMu.Lock()
			stat, exists := cpuTimeMap[event.Pid]
			if !exists {
				stat = &PIDStats{comm: event.Comm, gpuIdx: -1}
				cpuTimeMap[event.Pid] = stat
			}
			stat.cpuNs += event.OnCpuNs
			cpuTimeMapMu.Unlock()
		}
	}()

	// Inicializa GPU (NVML)
	if err := gpu.Init(); err != nil {
		log.Printf("GPU: NVML não inicializado (%v). Rodando sem suporte a GPU.", err)
	} else {
		defer gpu.Shutdown()
		log.Println("GPU: NVML inicializado com sucesso.")
	}

	// Sniffa tokens do arquivo de log
	if *logFilePath != "" {
		go startLogSniffer(*logFilePath)
	}

	// Inicializa Buffer Circular para resiliência de rede
	rb := NewRingBuffer(5000)

	// Goroutine para conectar e enviar stream para o Collector
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go streamToCollector(ctx, *collectorURL, rb)

	// Loop de medição periódica
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(*windowSecs) * time.Second)
	defer ticker.Stop()

	hostname, _ := os.Hostname()

	log.Println("Loop de coleta de telemetria ativo.")

	for {
		select {
		case <-sigChan:
			log.Println("Encerrando agente graciosamente...")
			return
		case <-ticker.C:
			collectAndEnqueue(hostname, rb)
		}
	}
}

func startLogSniffer(path string) {
	log.Printf("Iniciando sniffer de tokens no arquivo: %s", path)
	var file *os.File
	var err error

	for {
		file, err = os.Open(path)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	defer file.Close()

	// Posiciona no final do arquivo atual
	_, _ = file.Seek(0, io.SeekEnd)
	buffer := make([]byte, 8192)
	var pending string

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			pending += string(buffer[:n])
			lines := strings.Split(pending, "\n")
			// Guarda a última linha incompleta
			pending = lines[len(lines)-1]

			for i := 0; i < len(lines)-1; i++ {
				if t, matched := tokens.CountTokens(lines[i]); matched {
					atomic.AddInt64(&accumulatedTokens, t)
				}
			}
		}
		if err == io.EOF {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if err != nil {
			log.Printf("Erro lendo log de tokens: %v", err)
			time.Sleep(2 * time.Second)
		}
	}
}

func collectAndEnqueue(hostname string, rb *RingBuffer) {
	interval := time.Duration(*windowSecs) * time.Second

	// 1. Coleta consumo total de CPU/DRAM via RAPL
	var cpuWatts, dramWatts float64
	domains, err := energy.ScanDomains()
	if err == nil {
		for i := range domains {
			// Lê watts em paralelo ou sequencial no intervalo de tempo
			watts, err := domains[i].ReadWatts(100 * time.Millisecond)
			if err == nil {
				name := strings.ToLower(domains[i].Name)
				if strings.Contains(name, "package") {
					cpuWatts += watts
				} else if strings.Contains(name, "dram") {
					dramWatts += watts
				}
			}
		}
	}

	// 2. Coleta GPU Watts
	var gpuWatts float64
	gpuDevCount, _ := gpu.GetDeviceCount()
	gpuMap, _ := gpu.MapPIDsToGPUs()

	// Se houver GPU, lê o consumo total
	for i := 0; i < gpuDevCount; i++ {
		w, err := gpu.GetDevicePowerUsage(i)
		if err == nil {
			gpuWatts += w
		}
	}

	// 3. Clona e reseta mapa de tempos de CPU para cálculo proporcional
	cpuTimeMapMu.Lock()
	snapMap := make(map[uint32]*PIDStats, len(cpuTimeMap))
	for k, v := range cpuTimeMap {
		snapMap[k] = &PIDStats{comm: v.comm, cpuNs: v.cpuNs, gpuIdx: -1}
		v.cpuNs = 0 // Reseta para próxima janela
	}
	cpuTimeMapMu.Unlock()

	// Vincula processos a GPUs e memórias
	for pid, gpuIdx := range gpuMap {
		if stat, exists := snapMap[pid]; exists {
			stat.gpuIdx = gpuIdx
			memUsed, _, _ := gpu.GetDeviceMemoryUsage(gpuIdx)
			stat.gpuMem = memUsed
		}
	}

	// Calcula tempo total de CPU medido de todos os processos na janela
	var totalCPUNs uint64
	for _, stat := range snapMap {
		totalCPUNs += stat.cpuNs
	}

	// Lê total de tokens gerados na janela
	tokensInWindow := atomic.SwapInt64(&accumulatedTokens, 0)

	// 4. Identifica processos que coincidem com o workload selecionado
	now := time.Now().UnixNano()
	targetPID, errPID := strconv.Atoi(*workloadName)
	activeTargetPIDs := make(map[uint32]bool)

	if errPID == nil {
		activeTargetPIDs[uint32(targetPID)] = true
	} else {
		targetLower := strings.ToLower(*workloadName)
		entries, err := os.ReadDir("/proc")
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				p, errP := strconv.Atoi(entry.Name())
				if errP != nil {
					continue
				}
				cmdline, errC := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
				if errC == nil {
					cmdStr := strings.ToLower(strings.ReplaceAll(string(cmdline), "\x00", " "))
					if strings.Contains(cmdStr, targetLower) {
						activeTargetPIDs[uint32(p)] = true
					}
				}
			}
		}
	}

	var matchedPIDs []uint32
	var matchedTotalCPUNs uint64
	for pid, stat := range snapMap {
		isMatch := activeTargetPIDs[pid]

		// Heurística de robustez: se há GPU e detectamos que o processo está na GPU
		// podemos assumir que ele faz parte do workload alvo, se nenhuma outra checagem for melhor.
		if !isMatch && errPID != nil && gpuDevCount > 0 && len(gpuMap) > 0 {
			if stat.gpuIdx >= 0 {
				isMatch = true
			}
		}

		if isMatch {
			matchedPIDs = append(matchedPIDs, pid)
			matchedTotalCPUNs += stat.cpuNs
		}
	}

	tokensRemaining := tokensInWindow

	// Constrói eventos de energia para os processos alvo
	for i, pid := range matchedPIDs {
		stat := snapMap[pid]
		
		// Atribuição de energia proporcional ao tempo de CPU global
		processCPUWatts := 0.0
		processDRAMWatts := 0.0
		if totalCPUNs > 0 {
			ratio := float64(stat.cpuNs) / float64(totalCPUNs)
			processCPUWatts = cpuWatts * ratio
			processDRAMWatts = dramWatts * ratio
		}

		// Atribuição de tokens proporcional ao tempo de CPU dentre os processos do workload
		var processTokens int64 = 0
		if i == len(matchedPIDs)-1 {
			// O último PID recebe o restante dos tokens para evitar perda por arredondamento
			processTokens = tokensRemaining
		} else if matchedTotalCPUNs > 0 {
			workloadRatio := float64(stat.cpuNs) / float64(matchedTotalCPUNs)
			processTokens = int64(float64(tokensInWindow) * workloadRatio)
			tokensRemaining -= processTokens
		}

		// Atribuição de watts de GPU
		processGPUWatts := 0.0
		if stat.gpuIdx >= 0 && gpuDevCount > 0 {
			// Se o processo está rodando na GPU, atribui o consumo dessa GPU
			w, err := gpu.GetDevicePowerUsage(stat.gpuIdx)
			if err == nil {
				processGPUWatts = w
			}
		}

		cpuCount := 4.0
		if envCpu := os.Getenv("GT_CPU_COUNT"); envCpu != "" {
			if parsed, err := strconv.ParseFloat(envCpu, 64); err == nil && parsed > 0 {
				cpuCount = parsed
			}
		}
		cpuUtil := (float64(stat.cpuNs) / float64(interval.Nanoseconds())) * 100.0 / cpuCount
		if cpuUtil > 100.0 {
			cpuUtil = 100.0
		}

		// Constrói payload de telemetria
		event := &pb.EnergyEvent{
			TimestampNs:      now,
			AgentId:          *agentID,
			Hostname:         hostname,
			Pid:              int32(pid),
			Workload:         *workloadName,
			Model:            *modelName,
			WattsCpu:         processCPUWatts,
			WattsDram:        processDRAMWatts,
			WattsGpu:         processGPUWatts,
			GpuIndex:         int32(stat.gpuIdx),
			TokensInWindow:   processTokens,
			WindowSeconds:    interval.Seconds(),
			CpuUtilPct:       cpuUtil,
			GpuUtilPct:       0.0, // Preenchido se disponível
			GpuMemUsed:       stat.gpuMem,
		}

		rb.Enqueue(event)
		log.Printf("[TELEMETRIA] Enfileirado PID=%d (%s): W_cpu=%.2f W_dram=%.2f W_gpu=%.2f Tokens=%d Util_cpu=%.1f%%",
			pid, stat.comm, processCPUWatts, processDRAMWatts, processGPUWatts, processTokens, cpuUtil)
	}
}

func streamToCollector(ctx context.Context, addr string, rb *RingBuffer) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Collector inacessível em %s: %v. Tentando novamente em 5s...", addr, err)
			time.Sleep(5 * time.Second)
			continue
		}

		client := pb.NewGreenTokenCollectorClient(conn)
		stream, err := client.StreamEnergy(ctx)
		if err != nil {
			log.Printf("Falha ao abrir stream gRPC: %v. Tentando novamente em 5s...", err)
			conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("Conexão gRPC estabelecida com o Collector em %s", addr)

		// Loop de envio de eventos do buffer
		for {
			events := rb.DequeueAll()
			if len(events) > 0 {
				var sendErr error
				for _, ev := range events {
					if err := stream.Send(ev); err != nil {
						sendErr = err
						break
					}
				}
				if sendErr != nil {
					log.Printf("Erro enviando telemetria: %v. Reconectando...", sendErr)
					break
				}
			}
			time.Sleep(1 * time.Second)
		}

		stream.CloseSend()
		conn.Close()
		time.Sleep(2 * time.Second)
	}
}
