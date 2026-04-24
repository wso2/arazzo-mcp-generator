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
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

//go:embed mcp_server.py.tmpl
var mcpServerTemplate string

// serverTemplateData holds all values the mcp_server.py.tmpl template needs.
type serverTemplateData struct {
	Title             string // Python string literal, e.g. "My API"
	ArazzoFileName    string
	Port              int
	RemoteSourcePatch string // pre-rendered block (empty or ends with "\n")
	Tools             string // pre-rendered tool functions (no trailing blank line)
}

// GenerateServerCode produces the Python MCP server script from a parsed Arazzo spec.
// The generated server uses fastmcp and arazzo_runner to expose each workflow as a tool.
// Credential inputs (detected by name/description heuristics) are exposed as regular
// MCP tool parameters so the AI agent can prompt each user for their own token.
func GenerateServerCode(spec *ArazzoSpec, arazzoFileName string, port int) (string, error) {
	if len(spec.Workflows) == 0 {
		return "", fmt.Errorf("no workflows found in Arazzo spec to generate tools from")
	}

	// ── Classify all workflow inputs ──
	workflowClassified := make(map[string]ClassifiedInputs) // key = workflowID
	for _, wf := range spec.Workflows {
		workflowClassified[wf.WorkflowID] = ClassifyInputs(wf)
	}

	// ── Pre-render: remote source URL patch block ──
	remoteSourcePatch := buildRemoteSourcePatch(spec)

	// ── Pre-render: tool functions ──
	tools := buildToolsBlock(spec, workflowClassified)

	// ── Execute template ──
	tmpl, err := template.New("mcp_server").Parse(mcpServerTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse server template: %w", err)
	}

	data := serverTemplateData{
		Title:             fmt.Sprintf("%q", spec.Info.Title),
		ArazzoFileName:    arazzoFileName,
		Port:              port,
		RemoteSourcePatch: remoteSourcePatch,
		Tools:             tools,
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("failed to render server template: %w", err)
	}
	return b.String(), nil
}

// buildRemoteSourcePatch pre-renders the optional remote source URL patch block.
// Returns "\n" when there are no remote sources (the blank line after the runner= line).
// Returns the full patch block + trailing "\n" when remote sources are present.
func buildRemoteSourcePatch(spec *ArazzoSpec) string {
	if !hasRemoteSourceDescriptions(spec) {
		return "\n"
	}
	var b strings.Builder
	b.WriteString("# Resolve relative server URLs in remote source descriptions\n")
	for _, sd := range spec.SourceDescriptions {
		if strings.HasPrefix(sd.URL, "http://") || strings.HasPrefix(sd.URL, "https://") {
			b.WriteString(fmt.Sprintf("if %q in runner.source_descriptions:\n", sd.Name))
			b.WriteString(fmt.Sprintf("    _parsed = urlparse(%q)\n", sd.URL))
			b.WriteString(fmt.Sprintf("    _base = f\"{_parsed.scheme}://{_parsed.netloc}\"\n"))
			b.WriteString(fmt.Sprintf("    for _srv in runner.source_descriptions[%q].get(\"servers\", []):\n", sd.Name))
			b.WriteString(fmt.Sprintf("        if _srv.get(\"url\", \"\") and not _srv[\"url\"].startswith(\"http\"):\n"))
			b.WriteString(fmt.Sprintf("            _srv[\"url\"] = _base + _srv[\"url\"]\n"))
		}
	}
	b.WriteString("\n")
	return b.String()
}

// buildToolsBlock pre-renders all @mcp.tool() function definitions.
func buildToolsBlock(spec *ArazzoSpec, workflowClassified map[string]ClassifiedInputs) string {
	var b strings.Builder
	for i, wf := range spec.Workflows {
		if i > 0 {
			b.WriteString("\n")
		}

		classified := workflowClassified[wf.WorkflowID]

		funcName := camelToSnake(wf.WorkflowID)
		docstring := workflowDocstringWithAuth(wf, classified)
		params := buildAllParams(classified.RegularInputs, classified.CredentialInputs)
		inputDict := buildAllInputDict(classified.RegularInputs, classified.CredentialInputs)

		b.WriteString(fmt.Sprintf("# ── Tool %d: %s workflow\n", i+1, wf.WorkflowID))
		b.WriteString("@mcp.tool()\n")
		b.WriteString(fmt.Sprintf("async def %s(%s) -> str:\n", funcName, params))
		b.WriteString(fmt.Sprintf("    \"\"\"%s\"\"\"\n", docstring))
		b.WriteString("    try:\n")
		b.WriteString(fmt.Sprintf("        result = runner.execute_workflow(%q, {%s})\n", wf.WorkflowID, inputDict))
		b.WriteString("        if result.outputs:\n")
		b.WriteString("            return f\"Workflow Success. Outputs: {result.outputs}\"\n")
		b.WriteString("        return f\"Workflow Result: {result}\"\n")
		b.WriteString("    except Exception as e:\n")
		b.WriteString("        return f\"Workflow Error: {str(e)}\"\n")
	}
	return b.String()
}

// camelToSnake converts a camelCase or PascalCase string to snake_case.
func camelToSnake(s string) string {
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
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// arazzoTypeToPython maps Arazzo/JSON Schema types to Python type hints.
func arazzoTypeToPython(t string) string {
	switch strings.ToLower(t) {
	case "integer":
		return "int"
	case "number":
		return "float"
	case "string":
		return "str"
	case "boolean":
		return "bool"
	default:
		return "str"
	}
}

// sanitizeDocstring escapes characters that would break a Python triple-quoted string.
func sanitizeDocstring(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"""`, `\"\"\"`)
	return s
}

// workflowDocstring returns the docstring for a workflow tool function.
// DEPRECATED: Use workflowDocstringWithAuth for new code.
func workflowDocstring(wf Workflow) string {
	if wf.Summary != "" {
		return sanitizeDocstring(wf.Summary)
	}
	if wf.Description != "" {
		return sanitizeDocstring(wf.Description)
	}
	return fmt.Sprintf("Execute the %s workflow", wf.WorkflowID)
}

// workflowDocstringWithAuth returns the docstring for a workflow tool function,
// appending authentication instructions when credential inputs are detected.
func workflowDocstringWithAuth(wf Workflow, classified ClassifiedInputs) string {
	base := workflowDocstring(wf)
	if len(classified.CredentialInputs) == 0 {
		return base
	}

	// Build auth note with the Python parameter names
	var credParamNames []string
	for name := range classified.CredentialInputs {
		credParamNames = append(credParamNames, "'"+toPythonParamName(name)+"'")
	}
	sort.Strings(credParamNames)

	authNote := fmt.Sprintf("\n\n    IMPORTANT: This tool requires authentication. "+
		"Please provide your credentials via the %s parameter(s).",
		strings.Join(credParamNames, " and "))

	return base + authNote
}

// hasRemoteSourceDescriptions returns true if any sourceDescription uses an HTTP(S) URL.
func hasRemoteSourceDescriptions(spec *ArazzoSpec) bool {
	for _, sd := range spec.SourceDescriptions {
		if strings.HasPrefix(sd.URL, "http://") || strings.HasPrefix(sd.URL, "https://") {
			return true
		}
	}
	return false
}

// toPythonParamName converts any input name to a valid Python parameter name using camelCase.
// Handles kebab-case (Internal-Key → internalKey) and other non-identifier characters.
func toPythonParamName(name string) string {
	// First convert hyphens and spaces to underscores temporarily to help camelizing
	result := strings.ReplaceAll(name, "-", "_")
	result = strings.ReplaceAll(result, " ", "_")

	// Convert to camelCase
	var clean strings.Builder
	capitalizeNext := false
	for i, r := range result {
		if r == '_' {
			capitalizeNext = true
			continue
		}

		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue // Skip other non-identifier chars
		}

		if i == 0 || (clean.Len() == 0) {
			clean.WriteRune(unicode.ToLower(r))
		} else if capitalizeNext {
			clean.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			clean.WriteRune(r)
		}
	}

	result = clean.String()
	if result == "" {
		result = "param"
	}
	return result
}

// buildAllParams generates the Python function parameter list including both
// regular inputs and credential inputs. Both are normalized through toPythonParamName
// to guarantee valid Python identifiers.
func buildAllParams(regular map[string]InputProperty, credentials map[string]InputProperty) string {
	var parts []string
	for name, prop := range regular {
		pyName := toPythonParamName(name)
		pyType := arazzoTypeToPython(prop.Type)
		parts = append(parts, fmt.Sprintf("%s: %s", pyName, pyType))
	}
	for name, prop := range credentials {
		pyName := toPythonParamName(name)
		pyType := arazzoTypeToPython(prop.Type)
		parts = append(parts, fmt.Sprintf("%s: %s", pyName, pyType))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// buildAllInputDict generates the Python dict literal for execute_workflow(),
// mapping each original Arazzo input name to its normalized Python variable name.
// Both regular and credential inputs are normalized via toPythonParamName.
func buildAllInputDict(regular map[string]InputProperty, credentials map[string]InputProperty) string {
	var parts []string
	for name := range regular {
		pyName := toPythonParamName(name)
		parts = append(parts, fmt.Sprintf("%q: %s", name, pyName))
	}
	for name := range credentials {
		pyName := toPythonParamName(name)
		parts = append(parts, fmt.Sprintf("%q: %s", name, pyName))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
