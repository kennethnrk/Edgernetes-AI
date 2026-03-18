package agent

import (
	"context"
	"fmt"
	"time"

	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
	inferpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/infer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// forwardInfer connects to another agent's gRPC server and delegates the inference request.
func (a *Agent) forwardInfer(target *heartbeatpb.EndpointDetail, modelID string, inputData []float32, scalingEnabled bool) (float32, error) {
	peerAddr := fmt.Sprintf("%s:%d", target.Ip, target.Port)

	conn, err := grpc.NewClient(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return 0, fmt.Errorf("failed to connect to peer %s: %v", peerAddr, err)
	}
	defer conn.Close()

	client := inferpb.NewInferAPIClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := client.Infer(ctx, &inferpb.InferRequest{
		ModelId:        modelID,
		InputData:      inputData,
		ScalingEnabled: scalingEnabled,
		IsForwarded:    true,
	})

	if err != nil {
		return 0, fmt.Errorf("forwarded inference error: %v", err)
	}

	if !resp.Success {
		return 0, fmt.Errorf("peer evaluation returned failure: %s", resp.ErrorMessage)
	}

	return resp.Prediction, nil
}
