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

package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ─── Spectral integration ──────────────────────────────────────────────────────

// spectralRulesetContent is the Spectral ruleset that extends the built-in Arazzo rules.
const spectralRulesetContent = `extends: ["spectral:arazzo"]
`

// SpectralDiagnostic represents a single Spectral JSON output entry.
type SpectralDiagnostic struct {
	Code     string        `json:"code"`
	Path     []string      `json:"path"`
	Message  string        `json:"message"`
	Severity int           `json:"severity"` // 0=error, 1=warning, 2=info, 3=hint
	Range    SpectralRange `json:"range"`
	Source   string        `json:"source"`
}

// SpectralRange represents a range in the source file.
type SpectralRange struct {
	Start SpectralPosition `json:"start"`
	End   SpectralPosition `json:"end"`
}

// SpectralPosition represents a position in the source file.
type SpectralPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// IsSpectralAvailable checks if Spectral CLI can be invoked (via npx or global install).
func IsSpectralAvailable() bool {
	// Check for globally installed spectral
	if _, err := exec.LookPath("spectral"); err == nil {
		return true
	}
	// Check for npx availability (Node.js)
	if _, err := exec.LookPath("npx"); err == nil {
		return true
	}
	return false
}

// getSpectralCommand returns the command and args to invoke Spectral.
func getSpectralCommand() (string, []string) {
	// Prefer globally installed spectral
	if path, err := exec.LookPath("spectral"); err == nil {
		return path, nil
	}
	// Fall back to npx
	return "npx", []string{"--yes", "@stoplight/spectral-cli"}
}

// RunSpectral executes the Spectral linter against the given Arazzo file
// and returns parsed diagnostics.
func RunSpectral(filePath string) ([]SpectralDiagnostic, error) {
	cmdPath, baseArgs := getSpectralCommand()

	// Create a temporary ruleset file to avoid Windows path issues with spectral:arazzo
	tmpDir, err := os.MkdirTemp("", "arazzo-spectral-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	rulesetFile := filepath.Join(tmpDir, ".spectral.yaml")
	if err := os.WriteFile(rulesetFile, []byte(spectralRulesetContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp ruleset: %w", err)
	}

	// Build the full command args
	args := append(baseArgs,
		"lint",
		filePath,
		"--ruleset", rulesetFile,
		"--format", "json",
	)

	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = filepath.Dir(filePath) // Run from the spec's directory

	// Capture stdout separately from stderr — Spectral writes JSON to stdout
	// and informational messages to stderr
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Spectral exits with code 1 when it finds errors, so we ignore the exit code
	cmd.Run()

	// Spectral may write extra text after the JSON array (e.g. "No results with
	// a severity of 'error' found!") — extract just the JSON array portion.
	raw := strings.TrimSpace(stdout.String())
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")

	var diagnostics []SpectralDiagnostic

	if start < 0 || end < start {
		// No JSON array found in output at all
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return nil, fmt.Errorf("spectral produced no JSON output. stderr: %s", stderrStr)
		}
		if raw != "" {
			return nil, fmt.Errorf("spectral produced unexpected output (no JSON array): %s", raw)
		}
		// Completely empty output — treat as no issues
		return diagnostics, nil
	}

	jsonBytes := []byte(raw[start : end+1])
	if jsonErr := json.Unmarshal(jsonBytes, &diagnostics); jsonErr != nil {
		return nil, fmt.Errorf("failed to parse spectral JSON output: %w\nRaw stdout: %s\nStderr: %s",
			jsonErr, raw, strings.TrimSpace(stderr.String()))
	}

	return diagnostics, nil
}

// SpectralToResult converts Spectral diagnostics into our Result format.
func SpectralToResult(filePath string, diagnostics []SpectralDiagnostic) *Result {
	r := &Result{FilePath: filePath, Engine: "spectral"}

	if len(diagnostics) == 0 {
		r.pass("spectral", "", "Spectral validation: no issues found")
		return r
	}

	for _, d := range diagnostics {
		sev := mapSpectralSeverity(d.Severity)
		cat := categorizeSpectralRule(d.Code)
		path := formatSpectralPath(d.Path, d.Range)

		// Format the message with the rule code
		msg := fmt.Sprintf("[%s] %s", d.Code, d.Message)

		r.add(sev, cat, path, msg)
	}

	return r
}

// mapSpectralSeverity converts Spectral severity (0-3) to our Severity type.
func mapSpectralSeverity(spectralSev int) Severity {
	switch spectralSev {
	case 0: // Error
		return SevError
	case 1: // Warning
		return SevWarning
	case 2: // Info
		return SevInfo
	case 3: // Hint
		return SevInfo // Map hints to info
	default:
		return SevWarning
	}
}

// categorizeSpectralRule maps a Spectral rule code to our category system.
func categorizeSpectralRule(code string) string {
	switch {
	case strings.Contains(code, "document-schema"):
		return "structure"
	case strings.Contains(code, "info-"):
		return "structure"
	case strings.Contains(code, "source"):
		return "source"
	case strings.Contains(code, "workflow") && !strings.Contains(code, "step"):
		return "workflow"
	case strings.Contains(code, "step"):
		return "step"
	case strings.Contains(code, "script") || strings.Contains(code, "markdown"):
		return "structure"
	default:
		return "structure"
	}
}

// formatSpectralPath creates a human-readable path from Spectral's path array and range.
func formatSpectralPath(pathParts []string, rng SpectralRange) string {
	if len(pathParts) > 0 {
		return strings.Join(pathParts, ".")
	}
	// Fall back to line number
	if rng.Start.Line > 0 {
		return fmt.Sprintf("line %d", rng.Start.Line+1) // Spectral uses 0-based lines
	}
	return ""
}

// ValidateWithSpectral performs validation using Spectral as the primary engine
// and supplements with our custom checks.
// Returns (result, true, "") on success.
// Returns (nil, false, "not_found") when Spectral/Node.js is not installed.
// Returns (nil, false, "failed") when Spectral is installed but failed to run.
func ValidateWithSpectral(filePath string, folderPath string, checkRemote bool) (*Result, bool, string) {
	if !IsSpectralAvailable() {
		return nil, false, "not_found"
	}

	diagnostics, err := RunSpectral(filePath)
	if err != nil {
		// Spectral found but failed to lint — caller should fall back to built-in
		return nil, false, "failed"
	}

	// Convert Spectral results
	result := SpectralToResult(filePath, diagnostics)

	// Run our supplementary checks that Spectral doesn't cover:
	// - Source file accessibility (local file existence, remote URL probing)
	// - AND-ed $statusCode criteria warnings
	runSupplementaryChecks(result, filePath, folderPath, checkRemote)

	return result, true, ""
}

// runSupplementaryChecks adds our custom validation on top of Spectral results.
func runSupplementaryChecks(r *Result, filePath string, folderPath string, checkRemote bool) {
	data, err := readAndParseYAML(filePath)
	if err != nil {
		return // Spectral already handles parse errors
	}

	// Source accessibility checks
	if checkRemote {
		supplementSourceAccessibility(r, data, folderPath)
	}

	// AND-ed $statusCode criteria warnings
	supplementStatusCodeWarnings(r, data)
}

// supplementSourceAccessibility checks that source description files/URLs are accessible.
func supplementSourceAccessibility(r *Result, raw map[string]interface{}, folderPath string) {
	arr := getSlice(raw, "sourceDescriptions")
	sources := toMapSlice(arr)

	for i, sd := range sources {
		path := fmt.Sprintf("sourceDescriptions[%d]", i)
		name := getString(sd, "name")
		url := getString(sd, "url")

		if url == "" {
			continue
		}

		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			if err := probeURL(url); err != nil {
				r.errorf("source", path,
					fmt.Sprintf("[%s] Remote URL not accessible: %s — %v", name, url, err))
			} else {
				r.pass("source", path,
					fmt.Sprintf("[%s] Remote URL accessible: %s", name, url))
			}
		} else {
			// Local file
			resolvedPath := filepath.Join(folderPath, url)
			if !fileExists(resolvedPath) {
				r.errorf("source", path,
					fmt.Sprintf("[%s] Local file not found: %s", name, url))
			} else {
				r.pass("source", path,
					fmt.Sprintf("[%s] Local file exists: %s", name, url))
			}
		}
	}
}

// supplementStatusCodeWarnings checks for multiple AND-ed $statusCode criteria.
func supplementStatusCodeWarnings(r *Result, raw map[string]interface{}) {
	workflows := toMapSlice(getSlice(raw, "workflows"))

	for wi, wf := range workflows {
		wfID := getString(wf, "workflowId")
		steps := toMapSlice(getSlice(wf, "steps"))

		for si, step := range steps {
			stepID := getString(step, "stepId")
			scArr := getSlice(step, "successCriteria")
			if scArr == nil {
				continue
			}
			criteria := toMapSlice(scArr)

			var statusChecks []string
			for _, c := range criteria {
				cond := getString(c, "condition")
				if strings.Contains(cond, "$statusCode") {
					statusChecks = append(statusChecks, cond)
				}
			}

			if len(statusChecks) > 1 {
				path := fmt.Sprintf("workflows[%d].steps[%d].successCriteria", wi, si)
				_ = wfID // Used above in path context
				r.warning("step", path,
					fmt.Sprintf("Step '%s': multiple $statusCode criteria are AND-ed together "+
						"(%s) — this may never be satisfied simultaneously",
						stepID, strings.Join(statusChecks, " AND ")))
			}
		}
	}
}
