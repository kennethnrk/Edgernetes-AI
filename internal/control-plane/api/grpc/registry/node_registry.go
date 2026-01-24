package grpcregistry

import (
	"context"

	"github.com/google/uuid"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	nodepb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/node"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// nodeRegistryServer implements the NodeRegistryAPIServer interface.
type nodeRegistryServer struct {
	nodepb.UnimplementedNodeRegistryAPIServer
	store *store.Store
}

// NewNodeRegistryServer creates a new node registry server.
func NewNodeRegistryServer(s *store.Store) nodepb.NodeRegistryAPIServer {
	return &nodeRegistryServer{
		store: s,
	}
}

// RegisterNode registers a new node.
func (s *nodeRegistryServer) RegisterNode(ctx context.Context, req *nodepb.NodeInfo) (*nodepb.RegisterNodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Generate a new UUID for the node, ignoring any ID in the request
	nodeID := uuid.New().String()

	nodeInfo := protoToStoreNodeInfo(req)
	// Override the ID with the generated one
	nodeInfo.ID = nodeID

	if err := registrycontroller.RegisterNode(s.store, nodeID, nodeInfo); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &nodepb.RegisterNodeResponse{NodeId: nodeID}, nil
}

// DeRegisterNode removes a node from the registry.
func (s *nodeRegistryServer) DeRegisterNode(ctx context.Context, req *nodepb.NodeID) (*nodepb.BoolResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node ID cannot be empty")
	}

	if err := registrycontroller.DeRegisterNode(s.store, req.GetNodeId()); err != nil {
		return &nodepb.BoolResponse{Success: false}, status.Error(codes.Internal, err.Error())
	}

	return &nodepb.BoolResponse{Success: true}, nil
}

// UpdateNode updates an existing node.
func (s *nodeRegistryServer) UpdateNode(ctx context.Context, req *nodepb.UpdateNodeRequest) (*nodepb.BoolResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node ID cannot be empty")
	}

	// Get existing node to preserve fields not in UpdateNodeRequest
	existing, found, err := registrycontroller.GetNodeByID(s.store, req.GetNodeId())
	if err != nil {
		return &nodepb.BoolResponse{Success: false}, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &nodepb.BoolResponse{Success: false}, status.Error(codes.NotFound, "node not found")
	}

	// Update only the fields provided in the request
	nodeInfo := updateRequestToStoreNodeInfo(req, &existing)
	if err := registrycontroller.UpdateNodeInfo(s.store, req.GetNodeId(), nodeInfo); err != nil {
		return &nodepb.BoolResponse{Success: false}, status.Error(codes.Internal, err.Error())
	}

	return &nodepb.BoolResponse{Success: true}, nil
}

// GetNode retrieves a node by ID.
func (s *nodeRegistryServer) GetNode(ctx context.Context, req *nodepb.NodeID) (*nodepb.NodeInfo, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node ID cannot be empty")
	}

	nodeInfo, found, err := registrycontroller.GetNodeByID(s.store, req.GetNodeId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return nil, status.Error(codes.NotFound, "node not found")
	}

	return storeNodeInfoToProto(&nodeInfo), nil
}

// ListNodes returns all registered nodes.
func (s *nodeRegistryServer) ListNodes(ctx context.Context, req *nodepb.None) (*nodepb.ListNodesResponse, error) {
	nodes, err := registrycontroller.ListNodes(s.store)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoNodes := make([]*nodepb.NodeInfo, len(nodes))
	for i := range nodes {
		protoNodes[i] = storeNodeInfoToProto(&nodes[i])
	}

	return &nodepb.ListNodesResponse{Nodes: protoNodes}, nil
}

// protoToStoreNodeInfo converts a proto NodeInfo to a store NodeInfo.
func protoToStoreNodeInfo(pb *nodepb.NodeInfo) store.NodeInfo {
	info := store.NodeInfo{
		ID:   pb.GetNodeId(),
		Name: pb.GetName(),
		IP:   pb.GetIp(),
		Port: int(pb.GetPort()),
	}

	if pb.GetMetadata() != nil {
		info.Metadata = store.NodeMetadata{
			OSType:       pb.GetMetadata().GetOsType(),
			AgentVersion: pb.GetMetadata().GetAgentVersion(),
			Hostname:     pb.GetMetadata().GetHostname(),
		}
	}

	if pb.GetNetworkInfo() != nil {
		info.NetworkInfo = store.NetworkInfo{
			Type:      pb.GetNetworkInfo().GetType(),
			Bandwidth: pb.GetNetworkInfo().GetBandwidth(),
			IsMetered: pb.GetNetworkInfo().GetIsMetered(),
			Latency:   int(pb.GetNetworkInfo().GetLatency()),
		}
	}

	if pb.GetResourceCapabilities() != nil {
		rc := pb.GetResourceCapabilities()
		info.ResourceCapabilities = store.ResourceCapabilities{}

		if rc.GetMemory() != nil {
			info.ResourceCapabilities.Memory = store.MemoryInfo{
				Total: rc.GetMemory().GetTotal(),
				Free:  rc.GetMemory().GetFree(),
				Used:  rc.GetMemory().GetUsed(),
				Type:  constants.MemoryType(rc.GetMemory().GetType()),
			}
		}

		if rc.GetStorage() != nil {
			info.ResourceCapabilities.Storage = store.StorageInfo{
				Total:      rc.GetStorage().GetTotal(),
				Free:       rc.GetStorage().GetFree(),
				Used:       rc.GetStorage().GetUsed(),
				ReadSpeed:  rc.GetStorage().GetReadSpeed(),
				WriteSpeed: rc.GetStorage().GetWriteSpeed(),
			}
		}

		if devices := rc.GetComputeDevices(); len(devices) > 0 {
			info.ResourceCapabilities.ComputeDevices = make([]store.ComputeDevice, len(devices))
			for i, d := range devices {
				info.ResourceCapabilities.ComputeDevices[i] = store.ComputeDevice{
					Type:         constants.ComputeDeviceType(d.GetType()),
					Vendor:       d.GetVendor(),
					Model:        d.GetModel(),
					Memory:       d.GetMemory(),
					ComputeUnits: int(d.GetComputeUnits()),
					TOPS:         float64(d.GetTops()),
					PowerDraw:    int(d.GetPowerDrawWatts()),
					IsAvailable:  true, // Default to available
				}
			}
		}
	}

	return info
}

// updateRequestToStoreNodeInfo converts an UpdateNodeRequest to a store NodeInfo, preserving existing fields.
func updateRequestToStoreNodeInfo(req *nodepb.UpdateNodeRequest, existing *store.NodeInfo) store.NodeInfo {
	info := *existing // Start with existing node info

	if req.GetMetadata() != nil {
		info.Metadata = store.NodeMetadata{
			OSType:       req.GetMetadata().GetOsType(),
			AgentVersion: req.GetMetadata().GetAgentVersion(),
			Hostname:     req.GetMetadata().GetHostname(),
		}
	}

	if req.GetNetworkInfo() != nil {
		info.NetworkInfo = store.NetworkInfo{
			Type:      req.GetNetworkInfo().GetType(),
			Bandwidth: req.GetNetworkInfo().GetBandwidth(),
			IsMetered: req.GetNetworkInfo().GetIsMetered(),
			Latency:   int(req.GetNetworkInfo().GetLatency()),
		}
	}

	if req.GetResourceCapabilities() != nil {
		rc := req.GetResourceCapabilities()

		if rc.GetMemory() != nil {
			info.ResourceCapabilities.Memory = store.MemoryInfo{
				Total: rc.GetMemory().GetTotal(),
				Free:  rc.GetMemory().GetFree(),
				Used:  rc.GetMemory().GetUsed(),
				Type:  constants.MemoryType(rc.GetMemory().GetType()),
			}
		}

		if rc.GetStorage() != nil {
			info.ResourceCapabilities.Storage = store.StorageInfo{
				Total:      rc.GetStorage().GetTotal(),
				Free:       rc.GetStorage().GetFree(),
				Used:       rc.GetStorage().GetUsed(),
				ReadSpeed:  rc.GetStorage().GetReadSpeed(),
				WriteSpeed: rc.GetStorage().GetWriteSpeed(),
			}
		}

		if devices := rc.GetComputeDevices(); len(devices) > 0 {
			info.ResourceCapabilities.ComputeDevices = make([]store.ComputeDevice, len(devices))
			for i, d := range devices {
				info.ResourceCapabilities.ComputeDevices[i] = store.ComputeDevice{
					Type:         constants.ComputeDeviceType(d.GetType()),
					Vendor:       d.GetVendor(),
					Model:        d.GetModel(),
					Memory:       d.GetMemory(),
					ComputeUnits: int(d.GetComputeUnits()),
					TOPS:         float64(d.GetTops()),
					PowerDraw:    int(d.GetPowerDrawWatts()),
					IsAvailable:  true, // Default to available
				}
			}
		}
	}

	return info
}

// storeNodeInfoToProto converts a store NodeInfo to a proto NodeInfo.
func storeNodeInfoToProto(info *store.NodeInfo) *nodepb.NodeInfo {
	pb := &nodepb.NodeInfo{
		NodeId: info.ID,
		Name:   info.Name,
		Ip:     info.IP,
		Port:   int32(info.Port),
	}

	// Convert Metadata
	pb.Metadata = &nodepb.NodeMetadata{
		OsType:       info.Metadata.OSType,
		AgentVersion: info.Metadata.AgentVersion,
		Hostname:     info.Metadata.Hostname,
	}

	// Convert NetworkInfo
	pb.NetworkInfo = &nodepb.NetworkInfo{
		Type:      info.NetworkInfo.Type,
		Bandwidth: info.NetworkInfo.Bandwidth,
		IsMetered: info.NetworkInfo.IsMetered,
		Latency:   int64(info.NetworkInfo.Latency),
	}

	// Convert ResourceCapabilities
	pb.ResourceCapabilities = &nodepb.ResourceCapabilities{}

	// Convert Memory
	pb.ResourceCapabilities.Memory = &nodepb.MemoryInfo{
		Total: info.ResourceCapabilities.Memory.Total,
		Free:  info.ResourceCapabilities.Memory.Free,
		Used:  info.ResourceCapabilities.Memory.Used,
		Type:  string(info.ResourceCapabilities.Memory.Type),
	}

	// Convert Storage
	pb.ResourceCapabilities.Storage = &nodepb.StorageInfo{
		Total:      info.ResourceCapabilities.Storage.Total,
		Free:       info.ResourceCapabilities.Storage.Free,
		Used:       info.ResourceCapabilities.Storage.Used,
		ReadSpeed:  info.ResourceCapabilities.Storage.ReadSpeed,
		WriteSpeed: info.ResourceCapabilities.Storage.WriteSpeed,
	}

	// Convert ComputeDevices
	if len(info.ResourceCapabilities.ComputeDevices) > 0 {
		pb.ResourceCapabilities.ComputeDevices = make([]*nodepb.ComputeDevice, len(info.ResourceCapabilities.ComputeDevices))
		for i, d := range info.ResourceCapabilities.ComputeDevices {
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
