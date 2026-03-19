package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kennethnrk/edgernetes-ai/internal/client"
	inferpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/infer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var inferCmd = &cobra.Command{
	Use:   "infer",
	Short: "Run inference on a model",
	RunE: func(cmd *cobra.Command, args []string) error {
		modelID, _ := cmd.Flags().GetString("model-id")
		inputStr, _ := cmd.Flags().GetString("input")
		target, _ := cmd.Flags().GetString("target")
		scaling, _ := cmd.Flags().GetBool("scaling")

		inputData, err := parseFloatList(inputStr)
		if err != nil {
			return fmt.Errorf("invalid --input: %w", err)
		}

		req := &inferpb.InferRequest{
			ModelId:        modelID,
			InputData:      inputData,
			ScalingEnabled: scaling,
			IsForwarded:    false,
		}

		var resp *inferpb.InferResponse

		if target != "" {
			// Direct agent inference
			conn, err := grpc.NewClient(target,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				return fmt.Errorf("failed to connect to agent %s: %w", target, err)
			}
			defer conn.Close()

			agentInfer := inferpb.NewInferAPIClient(conn)
			c2, err := client.New(resolveEndpoint(), resolveTimeout())
			if err != nil {
				return err
			}
			ctx, cancel := c2.Context()
			defer cancel()
			c2.Close()

			resp, err = agentInfer.Infer(ctx, req)
			if err != nil {
				exitOnErr(err)
			}
		} else {
			// Via control plane
			c, err := newClient()
			if err != nil {
				return err
			}
			defer c.Close()

			ctx, cancel := c.Context()
			defer cancel()

			resp, err = c.Infer.Infer(ctx, req)
			if err != nil {
				exitOnErr(err)
			}
		}

		f := client.NewFormatter(resolveFormat())
		return f.Print(resp, func() {
			f.PrintTable(
				[]string{"SUCCESS", "PREDICTION", "ERROR"},
				[][]string{{
					strconv.FormatBool(resp.Success),
					strconv.FormatFloat(float64(resp.Prediction), 'f', 6, 32),
					resp.ErrorMessage,
				}},
			)
		})
	},
}

func init() {
	inferCmd.Flags().String("model-id", "", "Model ID to infer on (required)")
	inferCmd.Flags().String("input", "", "Comma-separated float input data")
	inferCmd.Flags().String("target", "", "Agent address (host:port) for direct inference")
	inferCmd.Flags().Bool("scaling", false, "Enable auto-scaling")
	_ = inferCmd.MarkFlagRequired("model-id")
	_ = inferCmd.MarkFlagRequired("input")
}

// parseFloatList parses "1.0,2.0,3.0" into []float32.
func parseFloatList(s string) ([]float32, error) {
	parts := strings.Split(s, ",")
	result := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseFloat(p, 32)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q as float: %w", p, err)
		}
		result = append(result, float32(v))
	}
	return result, nil
}
