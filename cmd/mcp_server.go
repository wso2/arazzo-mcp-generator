package cmd

import (
	"github.com/spf13/cobra"
)

var mcpServerCmd = &cobra.Command{
	Use:   "mcp-server",
	Short: "Manage MCP server generation and operations",
}

func init() {
	rootCmd.AddCommand(mcpServerCmd)
}
