package grpcagent

import (
	"context"

	"github.com/google/uuid"
	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	deploypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// deployServer implements the DeployAPIServer interface.
type deployServer struct {
	deploypb.UnimplementedDeployAPIServer
	agent *agent.Agent
}

// NewDeployServer creates a new deploy server.
func NewDeployServer(a *agent.Agent) deploypb.DeployAPIServer {
	return &deployServer{
		agent: a,
	}
}

// DeployModel handles model deployment requests from the control-plane.
func (s *deployServer) DeployModel(ctx context.Context, req *deploypb.DeployModelRequest) (*deploypb.DeployModelResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Generate a unique replica ID
	replicaID := uuid.New().String()

	// Construct ModelReplicaDetails from request
	replicaDetails := agent.ModelReplicaDetails{
		ID:           replicaID,
		ModelID:      req.ModelId,
		Name:         req.Name,
		Version:      req.Version,
		FilePath:     req.FilePath,
		ModelType:    constants.ModelType(req.ModelType),
		ModelSize:    req.ModelSize,
		Status:       constants.ModelReplicaStatusPending,
		ErrorCode:    0,
		ErrorMessage: "",
		LogFile:      "",
	}

	// Assign model to agent
	err := s.agent.AssignModel(replicaDetails)
	if err != nil {
		return &deploypb.DeployModelResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &deploypb.DeployModelResponse{
		Success: true,
		Message: "Model deployed successfully",
	}, nil
}
