package cli

import (
	"doctools-cli/internal/version"
	"doctools-cli/pkg/mcp"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start stdio JSON-RPC 2.0 MCP server",
	Long:  "Start stdio JSON-RPC 2.0 MCP server for AI agents (Claude, Cursor, Antigravity, etc.)",
	RunE: func(cmd *cobra.Command, args []string) error {
		server := mcp.NewServer(mcp.WithVersion(version.Version))
		return server.Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
	},
}

func init() {
	RootCmd.AddCommand(mcpCmd)
}
