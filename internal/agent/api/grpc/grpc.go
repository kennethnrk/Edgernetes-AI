package grpcagent

import (
	"fmt"
	"log"
	"net"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
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

	log.Printf("Heartbeat gRPC server listening on %s", addr)

	// Start server (blocking)
	if err := s.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server stopped: %w", err)
	}

	return nil
}
