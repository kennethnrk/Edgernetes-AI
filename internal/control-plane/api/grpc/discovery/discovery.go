package grpcdiscovery

import (
	"context"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	discoverypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/discovery"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

type discoveryServer struct {
	discoverypb.UnimplementedDiscoveryAPIServer
	store *store.Store
}

// NewDiscoveryServer returns a new Discovery API server instance.
func NewDiscoveryServer(store *store.Store) discoverypb.DiscoveryAPIServer {
	return &discoveryServer{store: store}
}

// GetNodes returns all online nodes from the registry.
func (s *discoveryServer) GetNodes(ctx context.Context, req *discoverypb.GetNodesRequest) (*discoverypb.GetNodesResponse, error) {
	nodes, err := registrycontroller.ListNodesByStatuses(s.store, []constants.Status{constants.StatusOnline})
	if err != nil {
		return nil, err
	}

	endpoints := make([]*discoverypb.NodeEndpoint, 0, len(nodes))
	for _, n := range nodes {
		endpoints = append(endpoints, &discoverypb.NodeEndpoint{
			NodeId: n.ID,
			Ip:     n.IP,
			Port:   int32(n.Port),
		})
	}

	return &discoverypb.GetNodesResponse{Nodes: endpoints}, nil
}
