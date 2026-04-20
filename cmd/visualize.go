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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wso2/arazzo-mcp-generator/internal/generator"
	"github.com/wso2/arazzo-mcp-generator/internal/visualizer"
)

const VisualizeCmdExample = `# Visualize an Arazzo spec folder (auto-detects the Arazzo file)
arazzo-mcp-gen visualize -d ./my-arazzo-folder

# Visualize a single Arazzo file directly
arazzo-mcp-gen visualize -f ./workflow.yaml

# Save the diagram to a Markdown file (auto-wraps in code fences)
arazzo-mcp-gen visualize -f ./workflow.yaml -o diagram.md

# Save raw Mermaid syntax to a .mmd file
arazzo-mcp-gen visualize -d ./my-arazzo-folder -o flow.mmd`

var (
	vizFolder string
	vizFile   string
	vizOutput string
)

var visualizeCmd = &cobra.Command{
	Use:     "visualize",
	Aliases: []string{"viz"},
	Short:   "Generate a Mermaid flowchart diagram from an Arazzo specification",
	Long: `Parse an Arazzo specification and generate a Mermaid flowchart diagram that
visualizes all workflows, step flows, branching logic, and routing.

The diagram shows:
  - Start and end points for each workflow
  - Steps with their operation targets and success criteria
  - onSuccess routing (goto, end, retry) with condition labels
  - onFailure routing with condition labels
  - Implicit sequential flows and implicit end points
  - Cross-workflow references
  - Dashed arrows for default/fallthrough paths when all routes are conditional

Default behaviour (no --output flag):
  Opens a rendered HTML diagram in your system browser using the Mermaid CDN.
  No extra tools needed — just a browser.

Output file formats (--output / -o):
  - .md  file: Mermaid wrapped in markdown code fences (GitHub, VS Code preview)
  - .mmd file: Raw Mermaid syntax (for mermaid-cli / mmdc tool)

Tip: paste Mermaid source into https://mermaid.live for a shareable link.`,
	Example: VisualizeCmdExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runVisualizeCommand()
	},
}

func init() {
	visualizeCmd.Flags().StringVarP(&vizFolder, "folder", "d", "",
		"Path to folder containing Arazzo and OpenAPI spec files")
	visualizeCmd.Flags().StringVarP(&vizFile, "file", "f", "",
		"Path to a single Arazzo specification file")
	visualizeCmd.Flags().StringVarP(&vizOutput, "output", "o", "",
		"Output file path (.md wraps in fences, .mmd writes raw Mermaid)")
	rootCmd.AddCommand(visualizeCmd)
}

func runVisualizeCommand() error {
	if vizFolder == "" && vizFile == "" {
		return fmt.Errorf("either --folder (-d) or --file (-f) must be specified\n\n" +
			"Examples:\n" +
			"  arazzo-mcp-gen visualize -d ./my-arazzo-folder\n" +
			"  arazzo-mcp-gen visualize -f ./workflow.yaml")
	}
	if vizFolder != "" && vizFile != "" {
		return fmt.Errorf("cannot use both --folder (-d) and --file (-f) at the same time")
	}

	var filePath string

	if vizFile != "" {
		abs, err := filepath.Abs(vizFile)
		if err != nil {
			return fmt.Errorf("failed to resolve file path: %w", err)
		}
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", abs)
		}
		filePath = abs
	} else {
		abs, err := filepath.Abs(vizFolder)
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

	return visualizer.Visualize(filePath, vizOutput)
}
