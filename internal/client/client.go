package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	deploypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"
	discoverypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/discovery"
	inferpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/infer"
	modelpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/model"
	nodepb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/node"
)

// Client wraps all gRPC service stubs for the Edgernetes-AI control plane.
type Client struct {
	conn    *grpc.ClientConn
	timeout time.Duration

	Models    modelpb.ModelRegistryAPIClient
	Nodes     nodepb.NodeRegistryAPIClient
	Deploy    deploypb.DeployAPIClient
	Transfer  deploypb.ModelTransferServiceClient
	Infer     inferpb.InferAPIClient
	Discovery discoverypb.DiscoveryAPIClient
}

// New creates a Client connected to the given control plane address.
func New(endpoint string, timeout time.Duration) (*Client, error) {
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", endpoint, err)
	}

	return &Client{
		conn:      conn,
		timeout:   timeout,
		Models:    modelpb.NewModelRegistryAPIClient(conn),
		Nodes:     nodepb.NewNodeRegistryAPIClient(conn),
		Deploy:    deploypb.NewDeployAPIClient(conn),
		Transfer:  deploypb.NewModelTransferServiceClient(conn),
		Infer:     inferpb.NewInferAPIClient(conn),
		Discovery: discoverypb.NewDiscoveryAPIClient(conn),
	}, nil
}

// Close tears down the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Context returns a context with the configured timeout.
func (c *Client) Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.timeout)
}
