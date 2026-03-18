package grpcregistry

import (
	deploypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"
	discoverypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/discovery"
	modelpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/model"
	nodepb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/node"
	grpcdiscovery "github.com/kennethnrk/edgernetes-ai/internal/control-plane/api/grpc/discovery"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
	"google.golang.org/grpc"
)

// RegisterServices registers all gRPC services with the given gRPC server.
// modelDir is the directory on disk where uploaded model files will be stored.
func RegisterServices(s *grpc.Server, store *store.Store, modelDir string) {
	// Register Model Registry API
	modelSrv := NewModelRegistryServer(store)
	modelpb.RegisterModelRegistryAPIServer(s, modelSrv)

	// Register Node Registry API
	nodeSrv := NewNodeRegistryServer(store)
	nodepb.RegisterNodeRegistryAPIServer(s, nodeSrv)

	// Register Model Transfer Service (gRPC streaming fallback for model file delivery)
	transferSrv := NewModelTransferServer(store, modelDir)
	deploypb.RegisterModelTransferServiceServer(s, transferSrv)

	// Register Discovery API
	discoverySrv := grpcdiscovery.NewDiscoveryServer(store)
	discoverypb.RegisterDiscoveryAPIServer(s, discoverySrv)
}
