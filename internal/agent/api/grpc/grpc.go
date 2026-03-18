package grpcagent

import (
	"fmt"
	"log"
	"net"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	deploypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"
	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
	inferpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/infer"
	grpcinfer "github.com/kennethnrk/edgernetes-ai/internal/agent/api/grpc/infer"
	"google.golang.org/grpc"
)

// StartHeartbeatServer initializes and starts the heartbeat gRPC server.
// It listens on the specified address and handles heartbeat requests from the control-plane.
func StartGRPCServer(a *agent.Agent, addr string) error {
	// Create TCP listener
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Create gRPC server
	s := grpc.NewServer()

	// Create and register heartbeat server
	heartbeatSrv := NewHeartbeatServer(a)
	heartbeatpb.RegisterHeartbeatAPIServer(s, heartbeatSrv)

	// Create and register deploy server
	deploySrv := NewDeployServer(a)
	deploypb.RegisterDeployAPIServer(s, deploySrv)

	// Create and register infer server
	inferSrv := grpcinfer.NewInferServer(a)
	inferpb.RegisterInferAPIServer(s, inferSrv)

	log.Printf("Heartbeat, Deploy, & Infer gRPC servers listening on %s", addr)

	// Start server (blocking)
	if err := s.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server stopped: %w", err)
	}

	return nil
}
