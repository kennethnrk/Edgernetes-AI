package heartbeatcaller

import (
	"context"
	"fmt"

	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CallHeartbeat calls the heartbeat API to get the heartbeat response.
func CallHeartbeat(node store.NodeInfo) (*heartbeatpb.RequestHeartbeatResponse, error) {
	nodeAddr := fmt.Sprintf("%s:%d", node.IP, node.Port)

	conn, err := grpc.NewClient(nodeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := heartbeatpb.NewHeartbeatAPIClient(conn)
	resp, err := client.RequestHeartbeat(context.Background(), &heartbeatpb.RequestHeartbeatRequest{
		NodeID: node.ID,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
