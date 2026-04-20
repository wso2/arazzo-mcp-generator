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
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// ArazzoSpec represents the top-level structure of an Arazzo specification file.
type ArazzoSpec struct {
	Arazzo             string              `yaml:"arazzo"`
	Info               ArazzoInfo          `yaml:"info"`
	SourceDescriptions []SourceDescription `yaml:"sourceDescriptions"`
	Workflows          []Workflow          `yaml:"workflows"`
}

// ArazzoInfo contains metadata about the Arazzo specification.
type ArazzoInfo struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

// SourceDescription describes an API source referenced by the Arazzo specification.
type SourceDescription struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	URL  string `yaml:"url"`
}

// Workflow represents a single workflow defined in the Arazzo specification.
type Workflow struct {
	WorkflowID  string         `yaml:"workflowId"`
	Summary     string         `yaml:"summary"`
	Description string         `yaml:"description"`
	Inputs      *WorkflowInput `yaml:"inputs"`
}

// WorkflowInput describes the input schema for a workflow.
type WorkflowInput struct {
	Type       string                   `yaml:"type"`
	Properties map[string]InputProperty `yaml:"properties"`
}

// InputProperty describes a single input parameter for a workflow.
type InputProperty struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// FindArazzoFile scans the given folder for a YAML file that contains the
// top-level "arazzo" key. Returns the path to the Arazzo file.
// Returns an error if no Arazzo file is found or if multiple are found.
func FindArazzoFile(folderPath string) (string, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return "", fmt.Errorf("failed to read folder '%s': %w", folderPath, err)
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(folderPath, entry.Name())
		if isArazzoFile(filePath) {
			matches = append(matches, filePath)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no Arazzo specification file found in folder '%s'\n\nAn Arazzo file must be a .yaml or .yml file containing the top-level 'arazzo' key", folderPath)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple Arazzo specification files found in folder '%s':\n  %s\n\nPlease ensure only one Arazzo file exists in the folder", folderPath, strings.Join(matches, "\n  "))
	}

	return matches[0], nil
}

// isArazzoFile checks if a YAML file contains the top-level "arazzo" key.
func isArazzoFile(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}

	_, hasArazzo := raw["arazzo"]
	return hasArazzo
}

// ParseArazzoFile reads and parses an Arazzo specification YAML file.
func ParseArazzoFile(filePath string) (*ArazzoSpec, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Arazzo file '%s': %w", filePath, err)
	}

	var spec ArazzoSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse Arazzo file '%s': %w", filePath, err)
	}

	if spec.Arazzo == "" {
		return nil, fmt.Errorf("invalid Arazzo file '%s': missing 'arazzo' version field", filePath)
	}
	if spec.Info.Title == "" {
		return nil, fmt.Errorf("invalid Arazzo file '%s': missing 'info.title' field", filePath)
	}
	if len(spec.Workflows) == 0 {
		return nil, fmt.Errorf("invalid Arazzo file '%s': no workflows defined", filePath)
	}

	return &spec, nil
}

// ValidateSourceDescriptions checks that all OpenAPI spec files referenced in
// the Arazzo sourceDescriptions exist in the given folder.
func ValidateSourceDescriptions(spec *ArazzoSpec, folderPath string) error {
	var missing []string
	for i, sd := range spec.SourceDescriptions {
		if sd.Type != "openapi" {
			continue
		}
		// If the URL is an HTTP/HTTPS URL, check accessibility instead of a local file
		if strings.HasPrefix(sd.URL, "http://") || strings.HasPrefix(sd.URL, "https://") {
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Head(sd.URL)
			if err != nil {
				missing = append(missing, fmt.Sprintf("  sourceDescriptions[%d]: '%s' (name: '%s') - URL not accessible", i, sd.URL, sd.Name))
			} else {
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					missing = append(missing, fmt.Sprintf("  sourceDescriptions[%d]: '%s' (name: '%s') - URL not accessible", i, sd.URL, sd.Name))
				}
				resp.Body.Close()
			}
			continue
		}

		// Resolve the URL relative to the folder
		resolvedPath := filepath.Join(folderPath, sd.URL)
		if _, err := os.Stat(resolvedPath); err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, fmt.Sprintf("  sourceDescriptions[%d]: '%s' (name: '%s')", i, sd.URL, sd.Name))
			} else {
				return fmt.Errorf("failed to check source file '%s': %w", resolvedPath, err)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing OpenAPI spec files referenced in Arazzo specification:\n%s\n\nPlease ensure all referenced files are present in the folder '%s'", strings.Join(missing, "\n"), folderPath)
	}

	return nil
}

// ─── Credential Detection ──────────────────────────────────────────────────────

// ClassifiedInputs separates a workflow's inputs into regular params and
// inputs that appear to contain credentials or other sensitive values.
type ClassifiedInputs struct {
	RegularInputs    map[string]InputProperty // Non-sensitive workflow inputs
	CredentialInputs map[string]InputProperty // Inputs identified as credential-like for downstream handling
}

// IsCredentialInput returns true if the input property name or description suggests
// it contains a credential or other sensitive value (API key, token, password, etc.).
// This function only classifies inputs; it does not determine how generated code
// ultimately supplies or exposes those inputs.
// credentialTerms is the set of lowercase word-tokens and compound-token pairs
// that indicate a credential input. Checked against whole tokens only (never as
// substrings) so names like "monkeyId" or "authorId" are not false-positives.
var credentialTerms = map[string]bool{
	"key": true, "token": true, "password": true, "secret": true,
	"auth": true, "authentication": true, "authorization": true,
	"credential": true, "credentials": true, "bearer": true,
	// compound pairs (also checked as a single token for un-split identifiers)
	"apikey": true, "clientid": true, "clientsecret": true,
}

// nameTokens splits a camelCase/snake_case/kebab-case identifier into lowercase
// word tokens by splitting on non-alphanumeric separators and camelCase boundaries.
func nameTokens(s string) []string {
	var tokens []string
	var cur strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if cur.Len() > 0 {
				tokens = append(tokens, strings.ToLower(cur.String()))
				cur.Reset()
			}
		} else if i > 0 && unicode.IsUpper(r) && unicode.IsLower(runes[i-1]) {
			tokens = append(tokens, strings.ToLower(cur.String()))
			cur.Reset()
			cur.WriteRune(r)
		} else {
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, strings.ToLower(cur.String()))
	}
	return tokens
}

func IsCredentialInput(name string, prop InputProperty) bool {
	// Tokenize the name on non-alphanumeric separators and camelCase boundaries,
	// then match whole tokens (and adjacent-token compounds) against known credential
	// terms. This avoids false positives like "key" inside "monkeyId" or "auth"
	// inside "authorId".
	tokens := nameTokens(name)
	for _, t := range tokens {
		if credentialTerms[t] {
			return true
		}
	}
	// Also check each adjacent pair as a compound (e.g. clientId → "clientid").
	for i := 0; i < len(tokens)-1; i++ {
		if credentialTerms[tokens[i]+tokens[i+1]] {
			return true
		}
	}

	// Check the description (case-insensitive)
	lowerDesc := strings.ToLower(prop.Description)
	descKeywords := []string{
		"api key", "api-key", "token", "password", "secret",
		"authentication", "authorization", "credential", "bearer",
		"oauth", "client id", "client secret", "access token",
	}
	for _, keyword := range descKeywords {
		if strings.Contains(lowerDesc, keyword) {
			return true
		}
	}

	return false
}

// ClassifyInputs splits workflow inputs into regular and credential categories.
func ClassifyInputs(wf Workflow) ClassifiedInputs {
	result := ClassifiedInputs{
		RegularInputs:    make(map[string]InputProperty),
		CredentialInputs: make(map[string]InputProperty),
	}
	if wf.Inputs == nil {
		return result
	}
	for name, prop := range wf.Inputs.Properties {
		if IsCredentialInput(name, prop) {
			result.CredentialInputs[name] = prop
		} else {
			result.RegularInputs[name] = prop
		}
	}
	return result
}

// CredentialEnvVarName generates a Docker environment variable name from the
// Arazzo spec title and the input property name.
// Example: title="Petstore API", inputName="apiKey" → "PETSTORE_API_API_KEY"
func CredentialEnvVarName(specTitle string, inputName string) string {
	// Convert title: uppercase, replace non-alnum with underscore
	title := strings.ToUpper(specTitle)
	reg := regexp.MustCompile(`[^A-Z0-9]+`)
	title = reg.ReplaceAllString(title, "_")
	title = strings.Trim(title, "_")

	// Convert input name: insert underscore before capitals, then uppercase
	inputSnake := camelToSnakeUpper(inputName)

	return title + "_" + inputSnake
}

// camelToSnakeUpper converts camelCase to UPPER_SNAKE_CASE.
func camelToSnakeUpper(s string) string {
	runes := []rune(s)
	var result strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					result.WriteRune('_')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					result.WriteRune('_')
				}
			}
			result.WriteRune(unicode.ToUpper(r))
		} else {
			result.WriteRune(unicode.ToUpper(r))
		}
	}
	return result.String()
}
