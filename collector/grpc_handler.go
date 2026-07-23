// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"io"
	"log"
	"os"
	"sync"
	"time"

	"greentoken/collector/aggregator"
	"greentoken/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcServer struct {
	pb.UnimplementedGreenTokenCollectorServer
	agg        *aggregator.Aggregator
	rateLimits sync.Map // map[string]*rateLimiter
}

type rateLimiter struct {
	count int
	reset time.Time
	mu    sync.Mutex
}

func newGrpcServer(agg *aggregator.Aggregator) *grpcServer {
	return &grpcServer{
		agg: agg,
	}
}

func (s *grpcServer) checkRateLimit(agentID string) error {
	if os.Getenv("DISABLE_RATE_LIMIT") == "true" {
		return nil
	}
	if agentID == "" {
		agentID = "default-agent"
	}
	v, _ := s.rateLimits.LoadOrStore(agentID, &rateLimiter{reset: time.Now().Add(time.Minute)})
	rl := v.(*rateLimiter)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if time.Now().After(rl.reset) {
		rl.count = 0
		rl.reset = time.Now().Add(time.Minute)
	}

	rl.count++
	if rl.count > 500 {
		return status.Errorf(codes.ResourceExhausted, "rate limit exceeded for agent %s", agentID)
	}
	return nil
}

// StreamEnergy escuta os eventos de energia dos agentes em tempo real via gRPC bidirecional.
func (s *grpcServer) StreamEnergy(stream pb.GreenTokenCollector_StreamEnergyServer) error {
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			log.Printf("[gRPC Stream] Erro ao receber evento: %v", err)
			return err
		}

		// Rate Limiting
		if err := s.checkRateLimit(event.AgentId); err != nil {
			log.Printf("[gRPC Stream] Limite de requisições excedido para agent %s: %v", event.AgentId, err)
			return err
		}

		// Encaminha evento para agregação
		s.agg.AddEvent(event)

		// Responde com Ack
		ackErr := stream.Send(&pb.Ack{
			Ok:      true,
			Message: "Energy event received and aggregated successfully",
		})
		if ackErr != nil {
			log.Printf("[gRPC Stream] Erro ao enviar Ack: %v", ackErr)
			return ackErr
		}
	}
}
