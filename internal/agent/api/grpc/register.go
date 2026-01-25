package grpcagent

import (
	"context"
	"log"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	nodepb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/node"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RegisterWithControlPlane registers the agent with the control-plane and updates the agent's ID.
func RegisterWithControlPlane(controlPlaneAddr string, agentInfo *agent.Agent) error {
	// Create gRPC connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(controlPlaneAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	// Create NodeRegistryAPI client
	client := nodepb.NewNodeRegistryAPIClient(conn)

	// Convert agent.Agent to nodepb.NodeInfo
	nodeInfo := agentToNodeInfoProto(agentInfo)

	// Call RegisterNode
	resp, err := client.RegisterNode(ctx, nodeInfo)
	if err != nil {
		return err
	}

	// Update agent ID with the returned node ID
	agentInfo.ID = resp.GetNodeId()
	log.Printf("Successfully registered with control-plane. Assigned node ID: %s", agentInfo.ID)

	return nil
}

// agentToNodeInfoProto converts agent.Agent to nodepb.NodeInfo.
func agentToNodeInfoProto(a *agent.Agent) *nodepb.NodeInfo {
	pb := &nodepb.NodeInfo{
		NodeId: "", // Control-plane will generate this
		Name:   a.Name,
		Ip:     a.IP,
		Port:   int32(a.Port),
	}

	// Convert Metadata
	pb.Metadata = &nodepb.NodeMetadata{
		OsType:       a.Metadata.OSType,
		AgentVersion: a.Metadata.AgentVersion,
		Hostname:     a.Metadata.Hostname,
	}

	// Convert ResourceCapabilities
	pb.ResourceCapabilities = &nodepb.ResourceCapabilities{}

	// Convert Memory
	pb.ResourceCapabilities.Memory = &nodepb.MemoryInfo{
		Total: a.ResourceCapabilities.Memory.Total,
		Free:  a.ResourceCapabilities.Memory.Free,
		Used:  a.ResourceCapabilities.Memory.Used,
		Type:  string(a.ResourceCapabilities.Memory.Type),
	}

	// Convert Storage
	pb.ResourceCapabilities.Storage = &nodepb.StorageInfo{
		Total: a.ResourceCapabilities.Storage.Total,
		Free:  a.ResourceCapabilities.Storage.Free,
		Used:  a.ResourceCapabilities.Storage.Used,
	}

	// Convert ComputeDevices
	if len(a.ResourceCapabilities.ComputeDevices) > 0 {
		pb.ResourceCapabilities.ComputeDevices = make([]*nodepb.ComputeDevice, len(a.ResourceCapabilities.ComputeDevices))
		for i, d := range a.ResourceCapabilities.ComputeDevices {
			pb.ResourceCapabilities.ComputeDevices[i] = &nodepb.ComputeDevice{
				Type:           string(d.Type),
				Vendor:         d.Vendor,
				Model:          d.Model,
				Memory:         d.Memory,
				ComputeUnits:   int64(d.ComputeUnits),
				Tops:           float32(d.TOPS),
				PowerDrawWatts: int64(d.PowerDraw),
			}
		}
	}

	return pb
}
