package grpcinfer

import (
	"context"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	inferpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/infer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// inferServer implements the InferAPIServer interface.
type inferServer struct {
	inferpb.UnimplementedInferAPIServer
	agent *agent.Agent
}

// NewInferServer creates a new inference server handler.
func NewInferServer(a *agent.Agent) inferpb.InferAPIServer {
	return &inferServer{
		agent: a,
	}
}

// Infer handles the incoming inference gRPC request.
func (s *inferServer) Infer(ctx context.Context, req *inferpb.InferRequest) (*inferpb.InferResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	if req.ModelId == "" {
		return nil, status.Error(codes.InvalidArgument, "model_id cannot be empty")
	}

	prediction, err := s.agent.HandleInfer(req.ModelId, req.InputData, req.IsForwarded, req.ScalingEnabled)

	if err != nil {
		return &inferpb.InferResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil // Return normal RPC response with success=false, so client knows it failed logically rather than transport failure
	}

	return &inferpb.InferResponse{
		Success:    true,
		Prediction: prediction,
	}, nil
}
