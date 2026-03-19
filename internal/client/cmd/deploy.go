package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	deploypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"
)

var deployCmd = &cobra.Command{
	Use:   "deploy [model-id]",
	Short: "Deploy a model to the cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		instances, _ := cmd.Flags().GetInt32("instances")
		filePath, _ := cmd.Flags().GetString("file-path")
		name, _ := cmd.Flags().GetString("name")
		version, _ := cmd.Flags().GetString("version")
		modelType, _ := cmd.Flags().GetString("model-type")
		modelSize, _ := cmd.Flags().GetInt64("model-size")
		sha256Hash, _ := cmd.Flags().GetString("sha256")

		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Deploy.DeployModel(ctx, &deploypb.DeployModelRequest{
			ModelId:       args[0],
			Name:          name,
			Version:       version,
			FilePath:      filePath,
			ModelType:     modelType,
			ModelSize:     modelSize,
			InstanceCount: instances,
			Namespace:     resolveNS(),
			Sha256Hash:    sha256Hash,
		})
		if err != nil {
			exitOnErr(err)
		}

		fmt.Printf("Deploy: success=%v message=%s\n", resp.Success, resp.Message)
		return nil
	},
}

func init() {
	deployCmd.Flags().Int32("instances", 1, "Worker pool size per node")
	deployCmd.Flags().String("file-path", "", "Override model file path or URL")
	deployCmd.Flags().String("name", "", "Model name")
	deployCmd.Flags().String("version", "", "Model version")
	deployCmd.Flags().String("model-type", "", "Model type")
	deployCmd.Flags().Int64("model-size", 0, "Model size in bytes")
	deployCmd.Flags().String("sha256", "", "SHA256 hash of the model file")
}
