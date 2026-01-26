package grpcagent

import (
	"context"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	agentmonitor "github.com/kennethnrk/edgernetes-ai/internal/agent/monitor"
	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// heartbeatServer implements the HeartbeatAPIServer interface.
type heartbeatServer struct {
	heartbeatpb.UnimplementedHeartbeatAPIServer
	agent *agent.Agent
}

// NewHeartbeatServer creates a new heartbeat server.
func NewHeartbeatServer(a *agent.Agent) heartbeatpb.HeartbeatAPIServer {
	return &heartbeatServer{
		agent: a,
	}
}

// RequestHeartbeat handles heartbeat requests from the control-plane.
func (s *heartbeatServer) RequestHeartbeat(ctx context.Context, req *heartbeatpb.RequestHeartbeatRequest) (*heartbeatpb.RequestHeartbeatResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Call checkHealth to get model replicas and health status
	modelReplicas, success, err := agentmonitor.CheckHealth(s.agent)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert model replicas to protobuf format
	pbModelReplicas := make([]*heartbeatpb.ModelReplicaDetails, len(modelReplicas))
	for i := range modelReplicas {
		pbModelReplicas[i] = modelReplicaToProto(&modelReplicas[i])
	}

	return &heartbeatpb.RequestHeartbeatResponse{
		NodeID:        s.agent.ID,
		ModelReplicas: pbModelReplicas,
		Success:       success,
	}, nil
}

// modelReplicaToProto converts agent.ModelReplicaDetails to heartbeatpb.ModelReplicaDetails.
func modelReplicaToProto(m *agent.ModelReplicaDetails) *heartbeatpb.ModelReplicaDetails {
	return &heartbeatpb.ModelReplicaDetails{
		ReplicaId:    m.ID,
		ModelId:      m.ModelID,
		Name:         m.Name,
		Version:      m.Version,
		FilePath:     m.FilePath,
		ModelType:    string(m.ModelType),
		ModelSize:    m.ModelSize,
		Status:       string(m.Status),
		ErrorCode:    int32(m.ErrorCode),
		ErrorMessage: m.ErrorMessage,
	}
}
