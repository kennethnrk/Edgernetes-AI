package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kennethnrk/edgernetes-ai/internal/client"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage edgectl configuration",
}

var configSetEndpointCmd = &cobra.Command{
	Use:   "set-endpoint [host:port]",
	Short: "Set the control plane address",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.Endpoint = args[0]
		if err := client.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("Endpoint set to %s\n", args[0])
		return nil
	},
}

var configSetNamespaceCmd = &cobra.Command{
	Use:   "set-namespace [namespace]",
	Short: "Set the default namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.DefaultNamespace = args[0]
		if err := client.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("Default namespace set to %s\n", args[0])
		return nil
	},
}

var configSetOutputCmd = &cobra.Command{
	Use:   "set-output [table|json|yaml]",
	Short: "Set the default output format",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.OutputFormat = args[0]
		if err := client.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("Output format set to %s\n", args[0])
		return nil
	},
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Print current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := client.ConfigPath()
		fmt.Printf("Config file: %s\n", path)
		fmt.Printf("Endpoint:    %s\n", cfg.Endpoint)
		fmt.Printf("Namespace:   %s\n", cfg.DefaultNamespace)
		fmt.Printf("Output:      %s\n", cfg.OutputFormat)
		fmt.Printf("Timeout:     %ds\n", cfg.TimeoutSeconds)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configSetEndpointCmd)
	configCmd.AddCommand(configSetNamespaceCmd)
	configCmd.AddCommand(configSetOutputCmd)
	configCmd.AddCommand(configViewCmd)
}
