package grpcregistry

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	modelpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/model"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	statuscontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/status"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// modelRegistryServer implements the ModelRegistryAPIServer interface.
type modelRegistryServer struct {
	modelpb.UnimplementedModelRegistryAPIServer
	store *store.Store
}

// NewModelRegistryServer creates a new model registry server.
func NewModelRegistryServer(s *store.Store) modelpb.ModelRegistryAPIServer {
	return &modelRegistryServer{
		store: s,
	}
}

// RegisterModel registers a new model.
// Returns codes.InvalidArgument if the request is nil or the model name is empty.
// Returns codes.AlreadyExists if a model with the same name is already registered.
func (s *modelRegistryServer) RegisterModel(ctx context.Context, req *modelpb.ModelInfo) (*modelpb.BoolResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "model name cannot be empty")
	}

	// Generate a new UUID for the model, ignoring any ID in the request
	modelID := uuid.New().String()

	modelInfo := protoToStoreModelInfo(req)
	// Override the ID with the generated one
	modelInfo.ID = modelID

	if err := registrycontroller.RegisterModel(s.store, modelID, modelInfo); err != nil {
		if strings.Contains(err.Error(), "already registered") {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &modelpb.BoolResponse{Success: true}, nil
}

// DeRegisterModel removes a model from the registry.
func (s *modelRegistryServer) DeRegisterModel(ctx context.Context, req *modelpb.ModelID) (*modelpb.BoolResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "model ID cannot be empty")
	}

	if err := registrycontroller.DeRegisterModel(s.store, req.Id); err != nil {
		return &modelpb.BoolResponse{Success: false}, status.Error(codes.Internal, err.Error())
	}

	return &modelpb.BoolResponse{Success: true}, nil
}

// UpdateModel updates an existing model.
func (s *modelRegistryServer) UpdateModel(ctx context.Context, req *modelpb.UpdateModelRequest) (*modelpb.BoolResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "model ID cannot be empty")
	}

	modelInfo := updateRequestToStoreModelInfo(req)
	if err := registrycontroller.UpdateModelInfo(s.store, req.Id, modelInfo); err != nil {
		return &modelpb.BoolResponse{Success: false}, status.Error(codes.Internal, err.Error())
	}

	return &modelpb.BoolResponse{Success: true}, nil
}

// GetModel retrieves a model by ID.
func (s *modelRegistryServer) GetModel(ctx context.Context, req *modelpb.ModelID) (*modelpb.ModelInfo, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "model ID cannot be empty")
	}

	modelInfo, found, err := registrycontroller.GetModelByID(s.store, req.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return nil, status.Error(codes.NotFound, "model not found")
	}

	return storeModelInfoToProto(&modelInfo), nil
}

// ListModels returns all registered models.
func (s *modelRegistryServer) ListModels(ctx context.Context, req *modelpb.None) (*modelpb.ListModelsResponse, error) {
	models, err := registrycontroller.ListModels(s.store)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoModels := make([]*modelpb.ModelInfo, len(models))
	for i := range models {
		protoModels[i] = storeModelInfoToProto(&models[i])
	}

	return &modelpb.ListModelsResponse{Models: protoModels}, nil
}

// GetModelStatus retrieves the status and replica breakdown for a model.
func (s *modelRegistryServer) GetModelStatus(ctx context.Context, req *modelpb.ModelName) (*modelpb.ModelStatusResponse, error) {
	if req == nil || req.GetName() == "" || req.GetNamespace() == "" {
		return nil, status.Error(codes.InvalidArgument, "model name and namespace cannot be empty")
	}

	result, err := statuscontroller.GetModelStatus(s.store, req.GetNamespace(), req.GetName())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &modelpb.ModelStatusResponse{
		ModelName:     result.ModelName,
		ModelId:       result.ModelID,
		Status:        string(result.Status),
		TotalReplicas: result.TotalReplicas,
		Breakdown: &modelpb.ReplicaStatusBreakdown{
			Running: result.Breakdown.Running,
			Pending: result.Breakdown.Pending,
			Failed:  result.Breakdown.Failed,
			Unknown: result.Breakdown.Unknown,
		},
	}, nil
}

// GetNodesByModelName retrieves all node IPs hosting a specific model.
func (s *modelRegistryServer) GetNodesByModelName(ctx context.Context, req *modelpb.ModelName) (*modelpb.ModelNodesResponse, error) {
	if req == nil || req.GetName() == "" || req.GetNamespace() == "" {
		return nil, status.Error(codes.InvalidArgument, "model name and namespace cannot be empty")
	}

	modelID, result, err := registrycontroller.GetNodesByModelName(s.store, req.GetNamespace(), req.GetName())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoNodes := make([]*modelpb.NodeAddress, len(result))
	for i, node := range result {
		protoNodes[i] = &modelpb.NodeAddress{
			NodeId: node.NodeID,
			Ip:     node.IP,
			Port:   node.Port,
		}
	}

	return &modelpb.ModelNodesResponse{
		ModelName: req.GetName(),
		ModelId:   modelID,
		Nodes:     protoNodes,
	}, nil
}

// protoToStoreModelInfo converts a proto ModelInfo to a store ModelInfo.
func protoToStoreModelInfo(pb *modelpb.ModelInfo) store.ModelInfo {
	info := store.ModelInfo{
		ID:        pb.GetId(),
		Name:      pb.GetName(),
		Namespace: pb.GetNamespace(),
		Version:   pb.GetVersion(),
		FilePath:  pb.GetFilePath(),
		ModelType: constants.ModelType(pb.GetModelType()),
		ModelSize: pb.GetModelSize(),
		Replicas:  int(pb.GetReplicas()),
	}

	// Convert input_format string to json.RawMessage
	if inputFormat := pb.GetInputFormat(); inputFormat != "" {
		info.InputFormat = json.RawMessage(inputFormat)
	}

	return info
}

// updateRequestToStoreModelInfo converts an UpdateModelRequest to a store ModelInfo.
func updateRequestToStoreModelInfo(req *modelpb.UpdateModelRequest) store.ModelInfo {
	info := store.ModelInfo{
		ID:        req.GetId(),
		Name:      req.GetName(),
		Namespace: req.GetNamespace(),
		Version:   req.GetVersion(),
		FilePath:  req.GetFilePath(),
		ModelType: constants.ModelType(req.GetModelType()),
		ModelSize: req.GetModelSize(),
		Replicas:  int(req.GetReplicas()),
	}

	// Convert input_format string to json.RawMessage
	if inputFormat := req.GetInputFormat(); inputFormat != "" {
		info.InputFormat = json.RawMessage(inputFormat)
	}

	return info
}

// storeModelInfoToProto converts a store ModelInfo to a proto ModelInfo.
func storeModelInfoToProto(info *store.ModelInfo) *modelpb.ModelInfo {
	pb := &modelpb.ModelInfo{
		Id:          info.ID,
		Name:        info.Name,
		Namespace:   info.Namespace,
		Version:     info.Version,
		FilePath:    info.FilePath,
		ModelType:   string(info.ModelType),
		ModelSize:   info.ModelSize,
		Replicas:    int32(info.Replicas),
		InputFormat: string(info.InputFormat),
	}

	return pb
}
