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

package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wso2/arazzo-mcp-generator/internal/utils"
)

// MCPServerBuildConfig holds all parameters needed to build the MCP server Docker image.
type MCPServerBuildConfig struct {
	FolderPath     string
	Port           int
	ArazzoSpec     *ArazzoSpec
	ArazzoFileName string
	ServerCode     string
	DockerfileCode string
	OutputDir      string // If set, save build artifacts here and keep them after build
}

// GenerateDockerfile produces the Dockerfile content for the MCP server image.
func GenerateDockerfile(port int) string {
	var b strings.Builder
	b.WriteString("FROM python:3.11-slim\n")
	b.WriteString("\n")
	b.WriteString("WORKDIR /app\n")
	b.WriteString("\n")
	b.WriteString("# Install Python dependencies\n")
	b.WriteString("RUN pip install --no-cache-dir fastmcp arazzo-runner==0.9.6\n")
	b.WriteString("\n")
	b.WriteString("# Copy the Arazzo spec files and OpenAPI spec files\n")
	b.WriteString("COPY arazzo/ ./arazzo/\n")
	b.WriteString("\n")
	b.WriteString("# Copy the generated MCP server\n")
	b.WriteString("COPY mcp_server.py .\n")
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("EXPOSE %d\n", port))
	b.WriteString("\n")
	b.WriteString("CMD [\"python\", \"mcp_server.py\"]\n")
	return b.String()
}

// BuildMCPServerImage orchestrates the full Docker image build for the MCP server.
// It creates a temporary build context, copies spec files and generated code into it,
// runs docker build, and prints the result summary.
func BuildMCPServerImage(config MCPServerBuildConfig) error {
	// Step 1: Check Docker availability
	if err := utils.IsDockerAvailable(); err != nil {
		return fmt.Errorf("%w\n\nPlease install and start Docker before running this command", err)
	}

	// Step 2: Determine build directory
	var buildDir string
	if config.OutputDir != "" {
		// Use user-specified output directory — files persist after build
		absOutputDir, err := filepath.Abs(config.OutputDir)
		if err != nil {
			return fmt.Errorf("failed to resolve output directory path: %w", err)
		}
		if err := utils.EnsureDir(absOutputDir); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		buildDir = absOutputDir
	} else {
		// Use temporary directory under ~/.wso2ap/.tmp — cleaned up after build
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		baseDir := filepath.Join(homeDir, ".wso2ap", ".tmp")
		if err := utils.EnsureDir(baseDir); err != nil {
			return fmt.Errorf("failed to create temp base directory: %w", err)
		}
		tempDir, err := os.MkdirTemp(baseDir, "mcp-server-build-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary build directory: %w", err)
		}
		defer os.RemoveAll(tempDir)
		buildDir = tempDir
	}

	// Step 3: Create arazzo/ subdirectory and copy spec files into it.
	// Exclude the build directory itself to prevent infinite recursion when
	// the output directory is inside the source folder, and exclude .git/
	// and .github/ so version-control internals are not baked into the image.
	arazzoDir := filepath.Join(buildDir, "arazzo")
	absFolderPath, err := filepath.Abs(config.FolderPath)
	if err != nil {
		return fmt.Errorf("failed to resolve source folder path: %w", err)
	}
	excludeDirs := []string{
		filepath.Clean(buildDir),
		filepath.Join(absFolderPath, ".git"),
		filepath.Join(absFolderPath, ".github"),
	}
	if err := utils.CopyDir(absFolderPath, arazzoDir, excludeDirs...); err != nil {
		return fmt.Errorf("failed to copy spec files to build context: %w", err)
	}

	// Step 4: Write the generated mcp_server.py
	serverFilePath := filepath.Join(buildDir, "mcp_server.py")
	if err := os.WriteFile(serverFilePath, []byte(config.ServerCode), 0644); err != nil {
		return fmt.Errorf("failed to write generated server code: %w", err)
	}

	// Step 5: Write the generated Dockerfile
	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(config.DockerfileCode), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Step 6: Build the Docker image
	imageName := sanitizeImageName(config.ArazzoSpec.Info.Title)
	args := []string{"build", "-t", imageName, "."}

	cmd := exec.Command("docker", args...)
	cmd.Dir = buildDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker build failed: %w", err)
	}

	// Step 7: Print success summary
	fmt.Println()
	runCmd := fmt.Sprintf("docker run -p %d:%d %s", config.Port, config.Port, imageName)
	serverURL := fmt.Sprintf("http://localhost:%d", config.Port)
	summaryLines := []string{
		"✅ MCP Server image built successfully!",
		"",
		fmt.Sprintf("Image:  %s", imageName),
		fmt.Sprintf("Run:    %s", runCmd),
		fmt.Sprintf("URL:    %s", serverURL),
		"",
		"If TLS verification must be disabled for self-signed HTTPS endpoints,",
		"run the image with: -e ARAZZO_DISABLE_TLS_VERIFY=1",
	}
	if config.OutputDir != "" {
		summaryLines = append(summaryLines, "", fmt.Sprintf("Build artifacts saved to: %s", buildDir))
	}
	utils.PrintBoxedMessage(summaryLines)

	return nil
}

// sanitizeImageName converts an Arazzo title into a valid Docker image name.
// It lowercases the string, replaces non-alphanumeric characters with hyphens,
// collapses multiple hyphens, trims leading/trailing hyphens, and appends "-mcp-server".
func sanitizeImageName(title string) string {
	name := strings.ToLower(title)
	// Replace any non-alphanumeric character with a hyphen
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	name = reg.ReplaceAllString(name, "-")
	// Trim leading and trailing hyphens
	name = strings.Trim(name, "-")
	if name == "" {
		name = "mcp-server"
	} else {
		name = name + "-mcp-server"
	}
	return name
}
