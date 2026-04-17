package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wso2/arazzo-mcp-generator/internal/metadata"
)

var rootCmd = &cobra.Command{
	Use:     "arazzo-mcp-gen",
	Version: metadata.Version,
	Short:   "Generate MCP servers from Arazzo specifications",
	Long:    `arazzo-mcp-gen is a standalone CLI tool for generating Dockerized Python MCP servers directly from an Arazzo specification and its referenced OpenAPI spec files.`,
}

func init() {
	// Disable Cobra's default `completion` subcommand so it doesn't appear in help output.
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
