package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kennethnrk/edgernetes-ai/internal/client"
	modelpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/model"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply models from a YAML manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		manifest, err := client.ParseManifest(file)
		if err != nil {
			return err
		}

		if dryRun {
			fmt.Println("Dry run — the following models would be applied:")
			for _, m := range manifest.Models {
				ns := client.ResolveNamespace(flagNamespace, m.Namespace, manifest.Namespace, cfg.DefaultNamespace)
				fmt.Printf("  [%s] %s (version=%s, type=%s, replicas=%d)\n",
					ns, m.Name, m.Version, m.ModelType, m.Replicas)
			}
			return nil
		}

		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		var succeeded, failed int
		type result struct {
			ns     string
			name   string
			status string
		}
		results := make([]result, 0, len(manifest.Models))

		for _, m := range manifest.Models {
			ns := client.ResolveNamespace(flagNamespace, m.Namespace, manifest.Namespace, cfg.DefaultNamespace)

			ctx, cancel := c.Context()
			_, err := c.Models.RegisterModel(ctx, &modelpb.ModelInfo{
				Name:        m.Name,
				Namespace:   ns,
				Version:     m.Version,
				FilePath:    m.FilePath,
				ModelType:   m.ModelType,
				ModelSize:   m.ModelSize,
				Replicas:    m.Replicas,
				InputFormat: m.InputFormat,
			})
			cancel()

			if err != nil {
				failed++
				results = append(results, result{ns, m.Name, "✗ " + client.FormatError(err)})
			} else {
				succeeded++
				results = append(results, result{ns, m.Name, "✓ registered"})
			}
		}

		// Print results table
		f := client.NewFormatter(client.FormatTable)
		f.PrintTable(
			[]string{"NAMESPACE", "MODEL", "STATUS"},
			func() [][]string {
				rows := make([][]string, 0, len(results))
				for _, r := range results {
					rows = append(rows, []string{r.ns, r.name, r.status})
				}
				return rows
			}(),
		)

		fmt.Printf("\nApplied %d models: %d succeeded, %d failed\n",
			len(manifest.Models), succeeded, failed)

		return nil
	},
}

func init() {
	applyCmd.Flags().StringP("file", "f", "", "Path to YAML manifest file (required)")
	_ = applyCmd.MarkFlagRequired("file")
	applyCmd.Flags().Bool("dry-run", false, "Validate only, don't submit")
}
