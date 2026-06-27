package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"greentoken/collector/aggregator"
	"greentoken/collector/exporter"
	"greentoken/pb"

	"google.golang.org/grpc"
)

func main() {
	log.Println("[GreenToken Collector] Inicializando serviço...")

	// 1. Inicializa o Agregador de Métricas
	agg := aggregator.NewAggregator()

	// 2. Configura e inicia o servidor HTTP para expor métricas do Prometheus
	metricsPort := os.Getenv("GT_METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "2112"
	}
	metricsAddr := net.JoinHostPort("0.0.0.0", metricsPort)

	mux := http.NewServeMux()
	mux.Handle("/metrics", exporter.GetMetricsHandler(agg))
	
	httpServer := &http.Server{
		Addr:    metricsAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("[Prometheus Exporter] Iniciando servidor HTTP em %s/metrics\n", metricsAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[Prometheus Exporter] Servidor HTTP parou inesperadamente: %v\n", err)
		}
	}()

	// 3. Configura e inicia o servidor gRPC
	grpcPort := os.Getenv("GT_GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}
	grpcAddr := net.JoinHostPort("0.0.0.0", grpcPort)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Printf("[gRPC Server] Falha ao escutar na porta %s: %v\n", grpcAddr, err)
		// Shutdown do HTTP caso gRPC falhe no binding inicial
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
		os.Exit(1)
	}

	grpcSrv := grpc.NewServer()
	pb.RegisterGreenTokenCollectorServer(grpcSrv, newGrpcServer(agg))

	go func() {
		log.Printf("[gRPC Server] Iniciando servidor gRPC em %s\n", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Printf("[gRPC Server] Servidor gRPC parou inesperadamente: %v\n", err)
		}
	}()

	// 4. Captura sinais do sistema para shutdown gracioso (Zero Trust e Green Observa)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("[GreenToken Collector] Sinal de encerramento recebido (%s). Iniciando graceful shutdown...\n", sig)

	// Interrompe o gRPC de forma graciosa
	grpcGracefulChan := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(grpcGracefulChan)
	}()

	// Interrompe o HTTP de forma graciosa com timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	httpErr := httpServer.Shutdown(ctx)
	if httpErr != nil {
		log.Printf("[Prometheus Exporter] Erro ao encerrar HTTP de forma graciosa: %v\n", httpErr)
	} else {
		log.Println("[Prometheus Exporter] Servidor HTTP encerrado com sucesso.")
	}

	// Aguarda o término do gRPC ou timeout
	select {
	case <-grpcGracefulChan:
		log.Println("[gRPC Server] Servidor gRPC encerrado com sucesso.")
	case <-time.After(5 * time.Second):
		log.Println("[gRPC Server] Timeout ao aguardar graceful stop. Forçando encerramento...")
		grpcSrv.Stop()
	}

	log.Println("[GreenToken Collector] Serviço finalizado.")
}
