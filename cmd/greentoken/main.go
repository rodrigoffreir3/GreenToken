package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"
)

// Prometheus Query API Response Structs
type PromResponse struct {
	Status string   `json:"status"`
	Data   PromData `json:"data"`
}

type PromData struct {
	ResultType string       `json:"resultType"`
	Result     []PromResult `json:"result"`
}

type PromResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"` // [timestamp, valueString]
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "serve":
		runServe(os.Args[2:])
	case "report":
		runReport(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Printf("Comando desconhecido: %s\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("GreenToken — AI Energy FinOps Observability CLI")
	fmt.Println("\nUso:")
	fmt.Println("  greentoken <comando> [flags]")
	fmt.Println("\nComandos disponíveis:")
	fmt.Println("  serve     Inicializa o collector gRPC e o exporter Prometheus")
	fmt.Println("  report    Consulta o Prometheus e imprime um relatório de custo por token")
	fmt.Println("  doctor    Executa um diagnóstico completo do ambiente")
	fmt.Println("\nExecute 'greentoken <comando> --help' para ver as flags de cada comando.")
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	binPath := fs.String("bin", "", "Caminho alternativo para o binário greentoken-collector")
	grpcPort := fs.String("grpc-port", "50051", "Porta para o servidor gRPC")
	metricsPort := fs.String("metrics-port", "2112", "Porta para o exportador Prometheus")
	fs.Parse(args)

	// Procura o binário do collector
	collectorBin := "greentoken-collector"
	if *binPath != "" {
		collectorBin = *binPath
	} else {
		// Tenta caminhos comuns
		paths := []string{"./greentoken-collector", "./bin/greentoken-collector", "../collector/greentoken-collector", "greentoken-collector"}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				collectorBin = p
				break
			}
		}
	}

	fmt.Printf("Iniciando Collector utilizando o binário: %s...\n", collectorBin)
	cmd := exec.Command(collectorBin)
	cmd.Env = append(os.Environ(),
		"GT_GRPC_PORT="+*grpcPort,
		"GT_METRICS_PORT="+*metricsPort,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		fmt.Printf("Erro ao iniciar o Collector (%s): %v\n", collectorBin, err)
		fmt.Println("Certifique-se de que o collector está compilado. Execute 'make build-collector' primeiro.")
		os.Exit(1)
	}

	// Encaminha sinais de interrupção
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nEnviando sinal de encerramento para o Collector...")
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}()

	err = cmd.Wait()
	if err != nil {
		fmt.Printf("Collector finalizado com erro: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Collector encerrado com sucesso.")
}

func runReport(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	promURL := fs.String("prometheus", "http://localhost:9090", "URL do servidor Prometheus")
	modelName := fs.String("model", "", "Filtrar por nome do modelo de IA")
	fs.Parse(args)

	// Constrói a query do Prometheus
	query := "greentoken_cost_per_token"
	if *modelName != "" {
		query = fmt.Sprintf(`greentoken_cost_per_token{model="%s"}`, *modelName)
	}

	apiURL := fmt.Sprintf("%s/api/v1/query?query=%s", *promURL, url.QueryEscape(query))
	fmt.Printf("Consultando métricas no Prometheus em: %s...\n\n", *promURL)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		fmt.Printf("Erro ao conectar ao Prometheus: %v\n", err)
		fmt.Println("Dica: Certifique-se de que o Prometheus está rodando (via docker-compose).")
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Prometheus retornou erro HTTP %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var promResp PromResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		fmt.Printf("Erro ao decodificar resposta do Prometheus: %v\n", err)
		os.Exit(1)
	}

	if promResp.Status != "success" || len(promResp.Data.Result) == 0 {
		fmt.Println("Nenhuma métrica de custo/token encontrada.")
		fmt.Println("Dica: Verifique se o Agent e o Collector estão ativos e se houve inferências rodando.")
		return
	}

	// Desenha a tabela
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.Debug)
	fmt.Fprintln(w, "MODELO\tWORKLOAD\tPID\tGPU IDX\tCUSTO POR TOKEN")
	fmt.Fprintln(w, "------\t--------\t---\t-------\t---------------")

	for _, res := range promResp.Data.Result {
		model := res.Metric["model"]
		workload := res.Metric["workload"]
		pid := res.Metric["pid"]
		gpuIdx := res.Metric["gpu_index"]
		if gpuIdx == "-1" {
			gpuIdx = "N/A"
		}
		
		val := "0.0"
		if len(res.Value) > 1 {
			val = fmt.Sprintf("%v", res.Value[1])
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t$ %s\n", model, workload, pid, gpuIdx, val)
	}
	w.Flush()
	fmt.Println()
}
