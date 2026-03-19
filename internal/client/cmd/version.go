package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const clientVersion = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the edgectl version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("edgectl version %s\n", clientVersion)
	},
}
