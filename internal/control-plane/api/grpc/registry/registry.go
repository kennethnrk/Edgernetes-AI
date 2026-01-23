package grpcregistry

import (
	modelpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/model"
	nodepb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/node"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
	"google.golang.org/grpc"
)

// RegisterServices registers all gRPC services with the given gRPC server.
func RegisterServices(s *grpc.Server, store *store.Store) {
	// Register Model Registry API
	modelSrv := NewModelRegistryServer(store)
	modelpb.RegisterModelRegistryAPIServer(s, modelSrv)

	// Register Node Registry API
	nodeSrv := NewNodeRegistryServer(store)
	nodepb.RegisterNodeRegistryAPIServer(s, nodeSrv)
}
