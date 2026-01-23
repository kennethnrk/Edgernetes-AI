package main

import (
	"log"
	"net"
	"os"
	"path/filepath"

	grpcregistry "github.com/kennethnrk/edgernetes-ai/internal/control-plane/api/grpc/registry"
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

	log.Printf("control-plane gRPC server listening on %s", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC server stopped: %v", err)
	}
}
