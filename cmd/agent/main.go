package main

import (
	"flag"
	"log"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	grpcagent "github.com/kennethnrk/edgernetes-ai/internal/agent/api/grpc"
)

func main() {

	controlPlaneAddress := flag.String("addr", "localhost:50051", "The address of the control plane")
	nodeName := flag.String("n", "", "The name of the node (defaults to hostname-random)")
	flag.Parse()

	log.Println("Agent started")

	//get my IP address
	agentInfo := agent.GetAgentInfo(nodeName)

	// Register with control-plane
	if err := grpcagent.RegisterWithControlPlane(*controlPlaneAddress, agentInfo); err != nil {
		log.Fatalf("Failed to register with control-plane: %v", err)
	}

	log.Printf("Agent registered successfully with node ID: %s", agentInfo.ID)
}
