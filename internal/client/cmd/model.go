package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/kennethnrk/edgernetes-ai/internal/client"
	modelpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/model"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage models",
}

// --- register ---

var modelRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new model",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		version, _ := cmd.Flags().GetString("version")
		filePath, _ := cmd.Flags().GetString("file-path")
		modelType, _ := cmd.Flags().GetString("model-type")
		modelSize, _ := cmd.Flags().GetInt64("model-size")
		replicas, _ := cmd.Flags().GetInt32("replicas")
		inputFormat, _ := cmd.Flags().GetString("input-format")

		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Models.RegisterModel(ctx, &modelpb.ModelInfo{
			Name:        name,
			Namespace:   resolveNS(),
			Version:     version,
			FilePath:    filePath,
			ModelType:   modelType,
			ModelSize:   modelSize,
			Replicas:    replicas,
			InputFormat: inputFormat,
		})
		if err != nil {
			exitOnErr(err)
		}

		fmt.Printf("Model registered: success=%v\n", resp.Success)
		return nil
	},
}

// --- deregister ---

var modelDeregisterCmd = &cobra.Command{
	Use:   "deregister [model-id]",
	Short: "Deregister a model by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Models.DeRegisterModel(ctx, &modelpb.ModelID{Id: args[0]})
		if err != nil {
			exitOnErr(err)
		}

		fmt.Printf("Model deregistered: success=%v\n", resp.Success)
		return nil
	},
}

// --- update ---

var modelUpdateCmd = &cobra.Command{
	Use:   "update [model-id]",
	Short: "Update a model's fields",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		version, _ := cmd.Flags().GetString("version")
		filePath, _ := cmd.Flags().GetString("file-path")
		modelType, _ := cmd.Flags().GetString("model-type")
		modelSize, _ := cmd.Flags().GetInt64("model-size")
		replicas, _ := cmd.Flags().GetInt32("replicas")
		inputFormat, _ := cmd.Flags().GetString("input-format")

		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Models.UpdateModel(ctx, &modelpb.UpdateModelRequest{
			Id:          args[0],
			Name:        name,
			Namespace:   resolveNS(),
			Version:     version,
			FilePath:    filePath,
			ModelType:   modelType,
			ModelSize:   modelSize,
			Replicas:    replicas,
			InputFormat: inputFormat,
		})
		if err != nil {
			exitOnErr(err)
		}

		fmt.Printf("Model updated: success=%v\n", resp.Success)
		return nil
	},
}

// --- get ---

var modelGetCmd = &cobra.Command{
	Use:   "get [model-id]",
	Short: "Get a model by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		model, err := c.Models.GetModel(ctx, &modelpb.ModelID{Id: args[0]})
		if err != nil {
			exitOnErr(err)
		}

		f := client.NewFormatter(resolveFormat())
		return f.Print(model, func() {
			f.PrintTable(
				[]string{"ID", "NAME", "NAMESPACE", "VERSION", "TYPE", "SIZE", "REPLICAS"},
				[][]string{{
					model.Id, model.Name, model.Namespace, model.Version,
					model.ModelType, strconv.FormatInt(model.ModelSize, 10),
					strconv.FormatInt(int64(model.Replicas), 10),
				}},
			)
		})
	},
}

// --- list ---

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all models",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Models.ListModels(ctx, &modelpb.None{})
		if err != nil {
			exitOnErr(err)
		}

		ns := resolveNS()
		filtered := resp.Models
		if flagNamespace != "" {
			filtered = nil
			for _, m := range resp.Models {
				if m.Namespace == ns {
					filtered = append(filtered, m)
				}
			}
		}

		f := client.NewFormatter(resolveFormat())
		return f.Print(filtered, func() {
			rows := make([][]string, 0, len(filtered))
			for _, m := range filtered {
				rows = append(rows, []string{
					m.Id, m.Name, m.Namespace, m.Version, m.ModelType,
					strconv.FormatInt(int64(m.Replicas), 10),
				})
			}
			f.PrintTable([]string{"ID", "NAME", "NAMESPACE", "VERSION", "TYPE", "REPLICAS"}, rows)
		})
	},
}

// --- status ---

var modelStatusCmd = &cobra.Command{
	Use:   "status [model-name]",
	Short: "Get deployment status of a model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Models.GetModelStatus(ctx, &modelpb.ModelName{
			Name:      args[0],
			Namespace: resolveNS(),
		})
		if err != nil {
			exitOnErr(err)
		}

		f := client.NewFormatter(resolveFormat())
		return f.Print(resp, func() {
			f.PrintTable(
				[]string{"MODEL", "ID", "STATUS", "TOTAL", "RUNNING", "PENDING", "FAILED", "UNKNOWN"},
				[][]string{{
					resp.ModelName, resp.ModelId, resp.Status,
					strconv.FormatInt(int64(resp.TotalReplicas), 10),
					strconv.FormatInt(int64(resp.Breakdown.Running), 10),
					strconv.FormatInt(int64(resp.Breakdown.Pending), 10),
					strconv.FormatInt(int64(resp.Breakdown.Failed), 10),
					strconv.FormatInt(int64(resp.Breakdown.Unknown), 10),
				}},
			)
		})
	},
}

// --- nodes ---

var modelNodesCmd = &cobra.Command{
	Use:   "nodes [model-name]",
	Short: "List nodes serving a model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Models.GetNodesByModelName(ctx, &modelpb.ModelName{
			Name:      args[0],
			Namespace: resolveNS(),
		})
		if err != nil {
			exitOnErr(err)
		}

		f := client.NewFormatter(resolveFormat())
		return f.Print(resp, func() {
			rows := make([][]string, 0, len(resp.Nodes))
			for _, n := range resp.Nodes {
				rows = append(rows, []string{
					n.NodeId, n.Ip, strconv.FormatInt(int64(n.Port), 10),
				})
			}
			f.PrintTable([]string{"NODE ID", "IP", "PORT"}, rows)
		})
	},
}

// --- upload ---

var modelUploadCmd = &cobra.Command{
	Use:   "upload [file-path]",
	Short: "Upload a model file to the control plane",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename, _ := cmd.Flags().GetString("filename")
		if filename == "" {
			filename = args[0]
		}

		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		resp, err := uploadModel(c, args[0], filename)
		if err != nil {
			exitOnErr(err)
		}

		fmt.Printf("Upload complete: success=%v path=%s message=%s\n",
			resp.Success, resp.FilePath, resp.Message)
		return nil
	},
}

func init() {
	// register flags
	modelRegisterCmd.Flags().String("name", "", "Model name (required)")
	modelRegisterCmd.Flags().String("version", "", "Model version")
	modelRegisterCmd.Flags().String("file-path", "", "Model file path or URL")
	modelRegisterCmd.Flags().String("model-type", "", "Model type (cnn|linear|decision_tree|llm)")
	modelRegisterCmd.Flags().Int64("model-size", 0, "Model size in bytes")
	modelRegisterCmd.Flags().Int32("replicas", 1, "Number of replicas")
	modelRegisterCmd.Flags().String("input-format", "", "Input format JSON schema")
	_ = modelRegisterCmd.MarkFlagRequired("name")

	// update flags
	modelUpdateCmd.Flags().String("name", "", "New model name")
	modelUpdateCmd.Flags().String("version", "", "New version")
	modelUpdateCmd.Flags().String("file-path", "", "New file path")
	modelUpdateCmd.Flags().String("model-type", "", "New model type")
	modelUpdateCmd.Flags().Int64("model-size", 0, "New model size")
	modelUpdateCmd.Flags().Int32("replicas", 0, "New replica count")
	modelUpdateCmd.Flags().String("input-format", "", "New input format")

	// upload flags
	modelUploadCmd.Flags().String("filename", "", "Override uploaded filename")

	modelCmd.AddCommand(modelRegisterCmd)
	modelCmd.AddCommand(modelDeregisterCmd)
	modelCmd.AddCommand(modelUpdateCmd)
	modelCmd.AddCommand(modelGetCmd)
	modelCmd.AddCommand(modelListCmd)
	modelCmd.AddCommand(modelStatusCmd)
	modelCmd.AddCommand(modelNodesCmd)
	modelCmd.AddCommand(modelUploadCmd)
}
