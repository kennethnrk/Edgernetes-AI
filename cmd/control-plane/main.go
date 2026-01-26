package main

import (
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	grpcregistry "github.com/kennethnrk/edgernetes-ai/internal/control-plane/api/grpc/registry"
	heartbeatcontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/heartbeat"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
	"google.golang.org/grpc"
)

func main() {
	dataDir := filepath.Join(".", "data", "control-plane-store")
	if env := os.Getenv("STORE_DATA_DIR"); env != "" {
		dataDir = env
	}

	log.Println("Initializing data store at", dataDir)
	store, err := store.New(dataDir)
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	addr := os.Getenv("CONTROL_PLANE_GRPC_ADDR")
	if addr == "" {
		addr = ":50051"
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}

	s := grpc.NewServer()
	grpcregistry.RegisterServices(s, store)

	intervalSeconds := 10

	if envInterval := os.Getenv("HEARTBEAT_INTERVAL_SECONDS"); envInterval != "" {
		if parsed, err := strconv.Atoi(envInterval); err == nil && parsed > 0 {
			intervalSeconds = parsed
		} else {
			log.Printf("Invalid HEARTBEAT_INTERVAL_SECONDS value '%s', using default 10 seconds", envInterval)
		}
	}
	interval := time.Duration(intervalSeconds) * time.Second

	// Start heartbeat handler in a separate goroutine
	go heartbeatcontroller.StartHeartbeatHandler(store, interval)

	log.Printf("control-plane gRPC server listening on %s", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server stopped: %v", err)
	}
}
