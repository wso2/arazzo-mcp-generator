/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package generate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wso2/arazzo-mcp-generator/internal/generator"
)

const GenerateCmdExample = `# Generate an MCP server Docker image from a folder
arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder

# Generate from a single Arazzo file directly
arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder/workflow.arazzo.yaml

# Generate with a custom port
arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder -p 8080

# Generate and save build artifacts to a directory for inspection or manual editing
arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder -o ./my-output`

var (
	generatePath   string
	generatePort   int
	generateOutput string
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate an MCP server Docker image from an Arazzo specification",
	Long: `Generate a Docker image containing a Python MCP server from an Arazzo specification.

The command reads an Arazzo file and its referenced OpenAPI spec files, generates
a Python MCP server that exposes each workflow as an MCP tool, and builds a Docker
image ready to run.

Use -f to provide either an Arazzo file or a folder containing Arazzo and OpenAPI spec files.
  - Folder: the Arazzo file is auto-detected inside it.
  - File: its parent directory is used to locate referenced OpenAPI spec files.

Flags:
  -p, --port int            Port the MCP server will listen on inside the
                             container and mapped to localhost (default: 5000)
  -o, --output string       Directory to save generated build artifacts
                             (Dockerfile, mcp_server.py, arazzo specs). Files
                             persist after the build for inspection or manual
                             editing. If not set, a temporary directory is used
                             and cleaned up automatically.`,
	Example: GenerateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGenerateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	generateCmd.Flags().StringVarP(&generatePath, "file", "f", "", "Path to an Arazzo file or folder containing Arazzo and OpenAPI spec files")
	generateCmd.Flags().IntVarP(&generatePort, "port", "p", 5000, "Port the MCP server will listen on")
	generateCmd.Flags().StringVarP(&generateOutput, "output", "o", "", "Output directory to save generated files (Dockerfile, server code, specs)")
}

// Register adds the generate command to the given parent command.
func Register(parent *cobra.Command) {
	parent.AddCommand(generateCmd)
}

func runGenerateCommand() error {
	// Validate port range
	if generatePort < 1 || generatePort > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", generatePort)
	}

	if generatePath == "" {
		return fmt.Errorf("-f flag is required\n\n" +
			"Examples:\n" +
			"  arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder\n" +
			"  arazzo-mcp-gen mcp-server generate -f ./workflow.arazzo.yaml")
	}

	abs, err := filepath.Abs(generatePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", abs)
		}
		return fmt.Errorf("failed to access path: %w", err)
	}

	var absFolder string
	var arazzoFilePath string

	if info.IsDir() {
		absFolder = abs
		fmt.Println("Validating input folder...")
		var findErr error
		arazzoFilePath, findErr = generator.FindArazzoFile(absFolder)
		if findErr != nil {
			return findErr
		}
	} else {
		arazzoFilePath = abs
		absFolder = filepath.Dir(abs)
	}

	arazzoFileName := filepath.Base(arazzoFilePath)

	spec, err := generator.ParseArazzoFile(arazzoFilePath)
	if err != nil {
		return err
	}

	if err := generator.ValidateSourceDescriptions(spec, absFolder); err != nil {
		return err
	}
	fmt.Printf("Found Arazzo spec: %s with %d workflow(s)\n", spec.Info.Title, len(spec.Workflows))

	fmt.Println("Generating MCP server code...")
	serverCode, err := generator.GenerateServerCode(spec, arazzoFileName, generatePort)
	if err != nil {
		return fmt.Errorf("failed to generate server code: %w", err)
	}

	dockerfileCode := generator.GenerateDockerfile(generatePort)

	fmt.Println("Building Docker image...")
	config := generator.MCPServerBuildConfig{
		FolderPath:     absFolder,
		Port:           generatePort,
		ArazzoSpec:     spec,
		ArazzoFileName: arazzoFileName,
		ServerCode:     serverCode,
		DockerfileCode: dockerfileCode,
		OutputDir:      generateOutput,
	}

	if err := generator.BuildMCPServerImage(config); err != nil {
		return err
	}

	return nil
}
