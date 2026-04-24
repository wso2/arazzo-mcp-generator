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

package inspect

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wso2/arazzo-mcp-generator/internal/generator"
	"github.com/wso2/arazzo-mcp-generator/internal/inspector"
)

const InspectCmdExample = `# Inspect an Arazzo spec folder (auto-detects the Arazzo file)
arazzo-mcp-gen inspect -d ./my-arazzo-folder

# Inspect a single Arazzo file directly
arazzo-mcp-gen inspect -f ./workflow.yaml`

var (
	inspectFolder string
	inspectFile   string
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Display a human-readable summary of an Arazzo specification",
	Long: `Parse an Arazzo specification and print a detailed, color-coded overview of
its structure including all workflows, steps, routing logic, and I/O bindings.

Useful for understanding a spec at a glance, reviewing an AI-generated Arazzo
file, or debugging step-flow routing before generating an MCP server.

Output includes:
  - Spec metadata (title, version, Arazzo version)
  - All source descriptions with their types and URLs
  - For each workflow:
      • Inputs with types and descriptions
      • Step-by-step flow with operation targets and parameters
      • Success criteria per step
      • onSuccess / onFailure action routing (GOTO, END, RETRY) with conditions
      • Step outputs
      • Workflow-level outputs and their expressions`,
	Example: InspectCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInspectCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	inspectCmd.Flags().StringVarP(&inspectFolder, "folder", "d", "",
		"Path to folder containing Arazzo and OpenAPI spec files")
	inspectCmd.Flags().StringVarP(&inspectFile, "file", "f", "",
		"Path to a single Arazzo specification file")
}

// Register adds the inspect command to the given parent command.
func Register(root *cobra.Command) {
	root.AddCommand(inspectCmd)
}

func runInspectCommand() error {
	if inspectFolder == "" && inspectFile == "" {
		return fmt.Errorf("either --folder (-d) or --file (-f) must be specified\n\n" +
			"Examples:\n" +
			"  arazzo-mcp-gen inspect -d ./my-arazzo-folder\n" +
			"  arazzo-mcp-gen inspect -f ./workflow.yaml")
	}
	if inspectFolder != "" && inspectFile != "" {
		return fmt.Errorf("cannot use both --folder (-d) and --file (-f) at the same time")
	}

	var filePath string

	if inspectFile != "" {
		abs, err := filepath.Abs(inspectFile)
		if err != nil {
			return fmt.Errorf("failed to resolve file path: %w", err)
		}
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("file does not exist: %s", abs)
			}
			return fmt.Errorf("failed to access file: %w", err)
		}
		filePath = abs
	} else {
		abs, err := filepath.Abs(inspectFolder)
		if err != nil {
			return fmt.Errorf("failed to resolve folder path: %w", err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("folder does not exist: %s", abs)
			}
			return fmt.Errorf("failed to access folder: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", abs)
		}
		found, err := generator.FindArazzoFile(abs)
		if err != nil {
			return err
		}
		filePath = found
	}

	return inspector.Inspect(filePath)
}
