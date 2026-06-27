package main

import (
	"context"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"greentoken/collector/aggregator"
	"greentoken/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestGrpcHandlerStreamEnergy(t *testing.T) {
	// 1. Inicializa agregador e servidor gRPC local em uma porta randômica
	agg := aggregator.NewAggregator()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Erro ao abrir listener: %v", err)
	}
	defer lis.Close()

	grpcSrv := grpc.NewServer()
	srv := newGrpcServer(agg)
	pb.RegisterGreenTokenCollectorServer(grpcSrv, srv)

	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			// Pode falhar quando lis.Close() for chamado
		}
	}()
	defer grpcSrv.Stop()

	// 2. Conecta o cliente gRPC ao servidor local
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Erro ao conectar no servidor gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewGreenTokenCollectorClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.StreamEnergy(ctx)
	if err != nil {
		t.Fatalf("Erro ao abrir stream gRPC: %v", err)
	}

	// 3. Envia alguns eventos do cliente para o servidor
	events := []*pb.EnergyEvent{
		{
			AgentId:       "test-agent",
			Hostname:      "test-host",
			Pid:           1010,
			Workload:      "vllm",
			Model:         "llama3",
			WattsCpu:      45.0,
			WattsDram:     8.0,
			WattsGpu:      150.0,
			GpuIndex:      0,
			TokensInWindow: 200,
			WindowSeconds:  1.0,
		},
		{
			AgentId:       "test-agent",
			Hostname:      "test-host",
			Pid:           1010,
			Workload:      "vllm",
			Model:         "llama3",
			WattsCpu:      55.0,
			WattsDram:     10.0,
			WattsGpu:      160.0,
			GpuIndex:      0,
			TokensInWindow: 300,
			WindowSeconds:  1.0,
		},
	}

	for _, ev := range events {
		if err := stream.Send(ev); err != nil {
			t.Fatalf("Erro ao enviar evento: %v", err)
		}

		// Recebe Ack de volta do servidor
		ack, err := stream.Recv()
		if err != nil {
			t.Fatalf("Erro ao receber Ack: %v", err)
		}
		if !ack.Ok {
			t.Errorf("Ack retornado com falha: %s", ack.Message)
		}
	}

	// Encerra a transmissão do cliente
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("Erro ao fechar stream de envio: %v", err)
	}

	// Recebe o EOF que indica término da resposta do servidor
	_, err = stream.Recv()
	if err != io.EOF {
		t.Errorf("Esperava EOF ao finalizar stream, obteve %v", err)
	}

	// 4. Verifica se os estados foram salvos e agregados corretamente
	states := agg.GetStates()
	if len(states) != 1 {
		t.Fatalf("Esperava 1 workload no agregador, obteve %d", len(states))
	}

	state := states[0]
	if state.Pid != 1010 || state.Workload != "vllm" {
		t.Errorf("Workload agregado incorreto: %+v", state)
	}

	if state.TokensTotal != 500 {
		t.Errorf("Esperava 500 tokens totais, obteve %d", state.TokensTotal)
	}
}

func TestGrpcHandlerRateLimiter(t *testing.T) {
	// Testa o comportamento de estouro de rate limit
	agg := aggregator.NewAggregator()
	srv := newGrpcServer(agg)

	// Habilita rate limit explicitamente
	origEnv := os.Getenv("DISABLE_RATE_LIMIT")
	os.Unsetenv("DISABLE_RATE_LIMIT")
	defer os.Setenv("DISABLE_RATE_LIMIT", origEnv)

	// Simula 500 requisições
	for i := 0; i < 500; i++ {
		err := srv.checkRateLimit("agent-limit")
		if err != nil {
			t.Fatalf("Não deveria estourar o rate limit na requisição %d: %v", i+1, err)
		}
	}

	// A 501-ésima deve retornar erro de ResourceExhausted
	err := srv.checkRateLimit("agent-limit")
	if err == nil {
		t.Fatal("Esperava erro de rate limit na requisição 501, mas obteve nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Erro não é do tipo status.Status: %v", err)
	}
	if st.Code() != codes.ResourceExhausted {
		t.Errorf("Código de status incorreto. Esperava ResourceExhausted, obteve %v", st.Code())
	}
}
