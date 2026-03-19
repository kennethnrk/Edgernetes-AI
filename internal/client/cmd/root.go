package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kennethnrk/edgernetes-ai/internal/client"
)

var (
	// Global flag values
	flagEndpoint  string
	flagNamespace string
	flagOutput    string
	flagTimeout   string
	flagVerbose   bool

	// Loaded once in PersistentPreRun
	cfg *client.Config
)

var rootCmd = &cobra.Command{
	Use:   "edgectl",
	Short: "Edgernetes-AI command-line client",
	Long:  "edgectl manages models, nodes, and deployments on the Edgernetes-AI control plane.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = client.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagEndpoint, "endpoint", "e", "", "Control plane address (host:port)")
	rootCmd.PersistentFlags().StringVarP(&flagNamespace, "namespace", "n", "", "Override namespace")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "", "Output format (table|json|yaml)")
	rootCmd.PersistentFlags().StringVarP(&flagTimeout, "timeout", "t", "", "Request timeout (e.g. 10s)")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable debug logging")

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(inferCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(versionCmd)
}

// --- helpers used by subcommands ---

// resolveEndpoint returns the effective endpoint.
func resolveEndpoint() string {
	if flagEndpoint != "" {
		return flagEndpoint
	}
	return cfg.Endpoint
}

// resolveNS returns the effective namespace.
func resolveNS() string {
	return client.ResolveNamespace(flagNamespace, "", "", cfg.DefaultNamespace)
}

// resolveFormat returns the effective output format.
func resolveFormat() client.Format {
	if flagOutput != "" {
		return client.ParseFormat(flagOutput)
	}
	return client.ParseFormat(cfg.OutputFormat)
}

// resolveTimeout returns the effective timeout duration.
func resolveTimeout() time.Duration {
	if flagTimeout != "" {
		d, err := time.ParseDuration(flagTimeout)
		if err == nil {
			return d
		}
	}
	return time.Duration(cfg.TimeoutSeconds) * time.Second
}

// newClient creates a connected Client using resolved config.
func newClient() (*client.Client, error) {
	return client.New(resolveEndpoint(), resolveTimeout())
}

// exitOnErr prints the error and exits.
func exitOnErr(err error) {
	fmt.Fprintln(os.Stderr, client.FormatError(err))
	os.Exit(1)
}
