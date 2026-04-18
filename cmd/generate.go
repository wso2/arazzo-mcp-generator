/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wso2/arazzo-mcp-generator/internal/generator"
)

const GenerateCmdExample = `# Generate an MCP server Docker image from an Arazzo spec folder
arazzo-mcp-gen mcp-server generate -d ./my-arazzo-folder

# Generate from a single Arazzo file directly
arazzo-mcp-gen mcp-server generate -f ./my-arazzo-folder/workflow.arazzo.yaml

# Generate with a custom port
arazzo-mcp-gen mcp-server generate -d ./my-arazzo-folder -p 8080

# Generate and save build artifacts to a directory for inspection or manual editing
arazzo-mcp-gen mcp-server generate -d ./my-arazzo-folder -o ./my-output`

var (
	generateFolder string
	generateFile   string
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

You can provide input in two ways:
  -d, --folder    Path to a folder containing the Arazzo and OpenAPI spec files.
                  The folder must contain exactly one Arazzo file.
  -f, --file      Path to a single Arazzo specification file. The parent directory
                  is used to locate referenced OpenAPI spec files. Useful when a
                  folder contains multiple Arazzo files and you want to convert
                  only one.

One of --folder (-d) or --file (-f) is required (but not both).

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
	generateCmd.Flags().StringVarP(&generateFolder, "folder", "d", "", "Path to folder containing Arazzo and OpenAPI spec files")
	generateCmd.Flags().StringVarP(&generateFile, "file", "f", "", "Path to a single Arazzo specification file")
	generateCmd.Flags().IntVarP(&generatePort, "port", "p", 5000, "Port the MCP server will listen on")
	generateCmd.Flags().StringVarP(&generateOutput, "output", "o", "", "Output directory to save generated files (Dockerfile, server code, specs)")

	mcpServerCmd.AddCommand(generateCmd)
}

func runGenerateCommand() error {
	// Validate port range
	if generatePort < 1 || generatePort > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", generatePort)
	}

	// Validate flag usage: exactly one of --folder or --file must be provided
	if generateFolder == "" && generateFile == "" {
		return fmt.Errorf("either --folder (-d) or --file (-f) must be specified\n\n" +
			"Examples:\n" +
			"  arazzo-mcp-gen mcp-server generate -d ./my-arazzo-folder\n" +
			"  arazzo-mcp-gen mcp-server generate -f ./workflow.arazzo.yaml")
	}
	if generateFolder != "" && generateFile != "" {
		return fmt.Errorf("cannot use both --folder (-d) and --file (-f) at the same time")
	}

	var absFolder string
	var arazzoFilePath string

	if generateFile != "" {
		// --file mode: use the given file directly, derive folder from its parent
		absFile, err := filepath.Abs(generateFile)
		if err != nil {
			return fmt.Errorf("failed to resolve file path: %w", err)
		}
		if _, err := os.Stat(absFile); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", absFile)
		}
		arazzoFilePath = absFile
		absFolder = filepath.Dir(absFile)
	} else {
		// --folder mode: auto-detect the Arazzo file in the folder
		var err error
		absFolder, err = filepath.Abs(generateFolder)
		if err != nil {
			return fmt.Errorf("failed to resolve folder path: %w", err)
		}

		info, err := os.Stat(absFolder)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("folder does not exist: %s", absFolder)
			}
			return fmt.Errorf("failed to access folder: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", absFolder)
		}

		fmt.Println("Validating input folder...")
		arazzoFilePath, err = generator.FindArazzoFile(absFolder)
		if err != nil {
			return err
		}
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
