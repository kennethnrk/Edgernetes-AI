package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
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
}
