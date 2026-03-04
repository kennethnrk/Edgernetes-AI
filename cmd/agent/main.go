package main

import (
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	grpcagent "github.com/kennethnrk/edgernetes-ai/internal/agent/api/grpc"
)

func main() {

	controlPlaneAddress := flag.String("addr", "localhost:50051", "The address of the control plane")
	nodeName := flag.String("n", "", "The name of the node (defaults to hostname-random)")
	flag.Parse()

	log.Println("Agent started")

	// Heartbeat server address (where the agent will listen for control-plane heartbeats)
	serverAddr := os.Getenv("AGENT_GRPC_ADDR")
	if serverAddr == "" {
		serverAddr = ":50052"
	}

	// Get agent info and set Port to the heartbeat server port so the control-plane
	// can reach us at node.IP:node.Port for heartbeat calls
	agentInfo := agent.GetAgentInfo(nodeName)
	if port := portFromAddr(serverAddr); port > 0 {
		agentInfo.Port = port
	}

	// Register with control-plane (control-plane will use agentInfo.IP:agentInfo.Port for heartbeats)
	if err := grpcagent.RegisterWithControlPlane(*controlPlaneAddress, agentInfo); err != nil {
		log.Fatalf("Failed to register with control-plane: %v", err)
	}

	log.Printf("Agent registered successfully with node ID: %s (heartbeat at %s:%d)", agentInfo.ID, agentInfo.IP, agentInfo.Port)

	// Start a goroutine to monitor heartbeat staleness and re-register if needed
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if agentInfo.IsHeartbeatStale(60 * time.Second) {
				log.Printf("Heartbeat is stale (no request for >60s). Re-registering node %s...", agentInfo.ID)

				// Attempt to deregister first
				if agentInfo.ID != "" {
					if err := grpcagent.DeregisterWithControlPlane(*controlPlaneAddress, agentInfo.ID); err != nil {
						log.Printf("Failed to deregister with control-plane (ignoring): %v", err)
					}
				}

				if err := grpcagent.RegisterWithControlPlane(*controlPlaneAddress, agentInfo); err != nil {
					log.Printf("Failed to re-register with control-plane: %v", err)
				} else {
					log.Printf("Successfully re-registered agent. Node ID: %s", agentInfo.ID)
					agentInfo.UpdateLastHeartbeat() // reset heartbeat timer after successful re-registration
				}
			}
		}
	}()

	log.Printf("Starting agent gRPC server on %s", serverAddr)
	if err := grpcagent.StartGRPCServer(agentInfo, serverAddr); err != nil {
		log.Fatalf("Failed to start agent gRPC server: %v", err)
	}
}

// portFromAddr parses a host:port address and returns the port number (e.g. ":50052" -> 50052).
func portFromAddr(addr string) int {
	// Handle ":port" form
	if strings.HasPrefix(addr, ":") {
		if p, err := strconv.Atoi(addr[1:]); err == nil && p > 0 && p < 65536 {
			return p
		}
		return 0
	}
	// Handle "host:port" form
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p <= 0 || p >= 65536 {
		return 0
	}
	return p
}
