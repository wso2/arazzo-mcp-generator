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

package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wso2/arazzo-mcp-generator/internal/generator"
	"github.com/wso2/arazzo-mcp-generator/internal/validator"
)

const ValidateCmdExample = `# Validate an Arazzo spec folder (auto-detects the Arazzo file)
arazzo-mcp-gen validate -d ./my-arazzo-folder

# Validate a single Arazzo file
arazzo-mcp-gen validate -f ./workflow.yaml

# Validate and also check that remote source URLs are accessible
arazzo-mcp-gen validate -d ./my-arazzo-folder --check-remote

# Treat warnings as errors (useful for CI pipelines)
arazzo-mcp-gen validate -d ./my-arazzo-folder --strict`

var (
	validateFolder      string
	validateFile        string
	validateCheckRemote bool
	validateStrict      bool
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate an Arazzo specification file",
	Long: `Validate an Arazzo specification file for correctness and completeness.

Uses Spectral (https://github.com/stoplightio/spectral) with the official
'spectral:arazzo' ruleset as the primary validation engine when available.
Falls back to the built-in validator when Spectral/Node.js is not installed.

Spectral performs comprehensive checks including:
  - Full JSON Schema validation against the Arazzo 1.0.x specification
  - workflowId and stepId uniqueness and naming pattern validation
  - Step validation (operationId/operationPath/workflowId correctness)
  - Parameter, requestBody, and success criteria validation
  - Success/failure action validation (unique names, mutual exclusivity)
  - Workflow and step output expression validation
  - dependsOn validation and cross-reference checks
  - XSS prevention in markdown descriptions

Additional custom checks (always applied):
  - Source file and URL accessibility verification (with --check-remote)
  - AND-ed $statusCode criteria warnings

To install Spectral Globally: npm install -g @stoplight/spectral-cli
(without Spectral, a built-in validator is used as fallback)

Use --check-remote to also verify that remote source URLs are accessible.
Use --strict to treat warnings as errors (non-zero exit code on warnings).`,
	Example: ValidateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runValidateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	validateCmd.Flags().StringVarP(&validateFolder, "folder", "d", "",
		"Path to folder containing Arazzo and OpenAPI spec files")
	validateCmd.Flags().StringVarP(&validateFile, "file", "f", "",
		"Path to a single Arazzo specification file")
	validateCmd.Flags().BoolVar(&validateCheckRemote, "check-remote", false,
		"Check that remote source description URLs are accessible")
	validateCmd.Flags().BoolVar(&validateStrict, "strict", false,
		"Treat warnings as errors (exit code 1 on warnings)")
}

// Register adds the validate command to the given parent command.
func Register(root *cobra.Command) {
	root.AddCommand(validateCmd)
}

func runValidateCommand() error {
	// Determine what to validate
	if validateFolder == "" && validateFile == "" {
		return fmt.Errorf("either --folder (-d) or --file (-f) must be specified\n\n" +
			"Examples:\n" +
			"  arazzo-mcp-gen validate -d ./my-arazzo-folder\n" +
			"  arazzo-mcp-gen validate -f ./workflow.yaml")
	}

	if validateFolder != "" && validateFile != "" {
		return fmt.Errorf("cannot use both --folder (-d) and --file (-f) at the same time")
	}

	var filePath string
	var folderPath string

	if validateFile != "" {
		// Validate a single file
		absFile, err := filepath.Abs(validateFile)
		if err != nil {
			return fmt.Errorf("failed to resolve file path: %w", err)
		}
		if _, err := os.Stat(absFile); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", absFile)
		}
		filePath = absFile
		folderPath = filepath.Dir(absFile)
	} else {
		// Find Arazzo file in folder
		absFolder, err := filepath.Abs(validateFolder)
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

		found, err := generator.FindArazzoFile(absFolder)
		if err != nil {
			return err
		}
		filePath = found
		folderPath = absFolder
	}

	// Run validation — try Spectral first, fall back to built-in
	var result *validator.Result

	if spectralResult, ok, reason := validator.ValidateWithSpectral(filePath, folderPath, validateCheckRemote); ok {
		result = spectralResult
	} else if reason == "failed" {
		// Spectral is installed but encountered an error linting this file
		fmt.Printf("\n%s⚠  Spectral encountered an error — using built-in validator.%s\n", "\033[33m", "\033[0m")
		fmt.Printf("%s   Spectral was found but failed to lint this file.%s\n", "\033[2m", "\033[0m")
		fmt.Printf("%s   You can run Spectral manually to see the full error:%s\n", "\033[2m", "\033[0m")
		fmt.Printf("%s     npx @stoplight/spectral-cli lint \"%s\" --ruleset spectral:arazzo%s\n\n", "\033[2m", filePath, "\033[0m")
		result = validator.ValidateFile(filePath, folderPath, validateCheckRemote)
	} else {
		// Spectral / Node.js not found — use the built-in Go validator.
		fmt.Printf("\n%s⚠  Spectral not found — using built-in validator.%s\n", "\033[33m", "\033[0m")
		fmt.Printf("%s   For comprehensive validation (full JSON Schema + semantic checks),%s\n", "\033[2m", "\033[0m")
		fmt.Printf("%s   install Spectral and re-run:%s\n", "\033[2m", "\033[0m")
		fmt.Printf("%s     • Global install:  npm install -g @stoplight/spectral-cli%s\n", "\033[2m", "\033[0m")
		fmt.Printf("%s     • Via npx (Node.js required, no global install):%s\n", "\033[2m", "\033[0m")
		fmt.Printf("%s         npx @stoplight/spectral-cli lint <your-arazzo.yaml> --ruleset spectral:arazzo%s\n\n", "\033[2m", "\033[0m")
		result = validator.ValidateFile(filePath, folderPath, validateCheckRemote)
	}

	result.PrintReport()

	// Determine exit code
	if result.HasErrors() {
		os.Exit(1)
	}
	if validateStrict && result.WarningCount() > 0 {
		fmt.Printf("\n%s--strict mode: %d warning(s) treated as errors%s\n",
			"\033[33m", result.WarningCount(), "\033[0m")
		os.Exit(1)
	}

	// Print helpful suggestion if validation passed
	if !result.HasErrors() {
		fmt.Println("💡 Tip: Run 'arazzo-mcp-gen mcp-server generate -d " +
			formatFolderHint(folderPath) + "' to build an MCP server from this spec.")
	}

	return nil
}

// formatFolderHint returns a relative path if short, otherwise the absolute path
func formatFolderHint(absPath string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return absPath
	}
	rel, err := filepath.Rel(cwd, absPath)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)+".."+string(filepath.Separator)+"..") {
		return absPath
	}
	return "./" + filepath.ToSlash(rel)
}
