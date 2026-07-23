package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

type Status string

const (
	StatusOK    Status = "OK"
	StatusAviso Status = "AVISO"
	StatusFalha Status = "FALHA"
)

type CheckResult struct {
	Name    string
	Status  Status
	Message string
}

// Variáveis para permitir injeção de caminhos/métodos durante testes unitários
var (
	raplSysfsPath    = "/sys/class/powercap/intel-rapl:0"
	debugfsTracePath = "/sys/kernel/debug/tracing"
	procTracePath    = "/sys/kernel/tracing"
	isRootFunc       = checkIsRoot
)

func checkIsRoot() bool {
	if runtime.GOOS == "windows" {
		return true // No Windows desenvolvimento local assume privilégios do usuário
	}
	return os.Geteuid() == 0
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	collectorAddr := fs.String("collector", "", "Endereço do collector gRPC para testar conectividade (host:port)")
	metricsURL := fs.String("metrics-url", "http://localhost:8000/metrics", "URL do endpoint /metrics da engine de inferência")
	metricsName := fs.String("metrics-name", "vllm:generation_tokens_total", "Nome da métrica Prometheus de tokens a verificar")
	fs.Parse(args)

	fmt.Println("GreenToken Doctor — diagnóstico de ambiente")
	fmt.Println("─────────────────────────────────────────────")

	results := []CheckResult{
		checkRAPL(),
		checkEBPF(),
		checkGPU(),
		checkPermissions(),
	}

	if *collectorAddr != "" {
		results = append(results, checkCollector(*collectorAddr))
	}

	if *metricsURL != "" {
		results = append(results, checkTokenSource(*metricsURL, *metricsName))
	}

	okCount, avisoCount, falhaCount := 0, 0, 0
	for _, res := range results {
		fmt.Printf("[%s]\t%s\n", res.Status, res.Message)
		switch res.Status {
		case StatusOK:
			okCount++
		case StatusAviso:
			avisoCount++
		case StatusFalha:
			falhaCount++
		}
	}

	fmt.Printf("\nResumo: %d OK, %d aviso, %d falha.\n", okCount, avisoCount, falhaCount)
	if falhaCount > 0 {
		fmt.Println("Corrija as falhas indicadas antes de rodar 'serve' em produção.")
	}
}

func checkRAPL() CheckResult {
	if _, err := os.Stat(raplSysfsPath); err == nil {
		return CheckResult{
			Name:    "RAPL",
			Status:  StatusOK,
			Message: fmt.Sprintf("RAPL disponível em %s", raplSysfsPath),
		}
	}
	return CheckResult{
		Name:    "RAPL",
		Status:  StatusAviso,
		Message: "W_cpu/W_dram serão 0. Normal em VM ou CPU não-Intel.",
	}
}

func checkEBPF() CheckResult {
	_, errDebug := os.Stat(debugfsTracePath)
	_, errProc := os.Stat(procTracePath)
	if errDebug == nil || errProc == nil {
		return CheckResult{
			Name:    "eBPF",
			Status:  StatusOK,
			Message: "eBPF/tracing disponível no kernel",
		}
	}
	return CheckResult{
		Name:    "eBPF",
		Status:  StatusAviso,
		Message: "Correlação por PID via eBPF indisponível; fallback para scanner /proc.",
	}
}

func checkGPU() CheckResult {
	return CheckResult{
		Name:    "GPU",
		Status:  StatusAviso,
		Message: "Binário sem suporte a GPU (build stub) ou driver NVIDIA ausente. W_gpu será 0.",
	}
}

func checkPermissions() CheckResult {
	if isRootFunc() {
		return CheckResult{
			Name:    "Permissões",
			Status:  StatusOK,
			Message: "Permissões elevadas (root) ativas",
		}
	}
	return CheckResult{
		Name:    "Permissões",
		Status:  StatusFalha,
		Message: "Permissões insuficientes — eBPF e RAPL exigem privilégios elevados. Rode com sudo.",
	}
}

func checkCollector(addr string) CheckResult {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err == nil {
		conn.Close()
		return CheckResult{
			Name:    "Collector",
			Status:  StatusOK,
			Message: fmt.Sprintf("Collector alcançável em %s", addr),
		}
	}
	return CheckResult{
		Name:    "Collector",
		Status:  StatusAviso,
		Message: fmt.Sprintf("Collector inalcançável em %s. Agent vai enfileirar localmente até reconectar.", addr),
	}
}

func checkTokenSource(url, metricName string) CheckResult {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return CheckResult{
			Name:    "TokenSource",
			Status:  StatusAviso,
			Message: fmt.Sprintf("Endpoint %s inalcançável: %v. Verifique se a engine de inferência está ativa.", url, err),
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CheckResult{
			Name:    "TokenSource",
			Status:  StatusAviso,
			Message: fmt.Sprintf("Erro ao ler resposta de %s", url),
		}
	}

	if strings.Contains(string(body), metricName) {
		return CheckResult{
			Name:    "TokenSource",
			Status:  StatusOK,
			Message: fmt.Sprintf("Métrica '%s' encontrada em %s", metricName, url),
		}
	}

	return CheckResult{
		Name:    "TokenSource",
		Status:  StatusAviso,
		Message: fmt.Sprintf("Métrica '%s' não encontrada em %s. Verifique se o engine de inferência expõe /metrics.", metricName, url),
	}
}
