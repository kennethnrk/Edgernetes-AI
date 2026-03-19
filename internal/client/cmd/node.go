package cmd

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/kennethnrk/edgernetes-ai/internal/client"
	discoverypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/discovery"
	nodepb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/node"
)

var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Inspect nodes",
}

// --- get ---

var nodeGetCmd = &cobra.Command{
	Use:   "get [node-id]",
	Short: "Get node details by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		node, err := c.Nodes.GetNode(ctx, &nodepb.NodeID{NodeId: args[0]})
		if err != nil {
			exitOnErr(err)
		}

		f := client.NewFormatter(resolveFormat())
		return f.Print(node, func() {
			f.PrintTable(
				[]string{"NODE ID", "NAME", "IP", "PORT", "OS", "HOSTNAME"},
				[][]string{{
					node.NodeId, node.Name, node.Ip,
					strconv.FormatInt(int64(node.Port), 10),
					metaField(node, "os_type"),
					metaField(node, "hostname"),
				}},
			)
		})
	},
}

// --- list ---

var nodeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Nodes.ListNodes(ctx, &nodepb.None{})
		if err != nil {
			exitOnErr(err)
		}

		f := client.NewFormatter(resolveFormat())
		return f.Print(resp.Nodes, func() {
			rows := make([][]string, 0, len(resp.Nodes))
			for _, n := range resp.Nodes {
				rows = append(rows, []string{
					n.NodeId, n.Name, n.Ip,
					strconv.FormatInt(int64(n.Port), 10),
					metaField(n, "os_type"),
					metaField(n, "hostname"),
				})
			}
			f.PrintTable([]string{"NODE ID", "NAME", "IP", "PORT", "OS", "HOSTNAME"}, rows)
		})
	},
}

// --- endpoints ---

var nodeEndpointsCmd = &cobra.Command{
	Use:   "endpoints",
	Short: "List all node endpoints (discovery)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		defer c.Close()

		ctx, cancel := c.Context()
		defer cancel()

		resp, err := c.Discovery.GetNodes(ctx, &discoverypb.GetNodesRequest{})
		if err != nil {
			exitOnErr(err)
		}

		f := client.NewFormatter(resolveFormat())
		return f.Print(resp.Nodes, func() {
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

func init() {
	nodeCmd.AddCommand(nodeGetCmd)
	nodeCmd.AddCommand(nodeListCmd)
	nodeCmd.AddCommand(nodeEndpointsCmd)
}

// metaField safely extracts a metadata field from a NodeInfo.
func metaField(n *nodepb.NodeInfo, field string) string {
	if n.Metadata == nil {
		return ""
	}
	switch field {
	case "os_type":
		return n.Metadata.OsType
	case "hostname":
		return n.Metadata.Hostname
	case "agent_version":
		return n.Metadata.AgentVersion
	default:
		return ""
	}
}
