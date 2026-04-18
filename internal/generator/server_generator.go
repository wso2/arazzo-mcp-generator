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

package generator

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

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

	var b strings.Builder

	// ── Imports ──
	b.WriteString("import requests\n")
	b.WriteString("from urllib.parse import urlparse\n")
	b.WriteString("from fastmcp import FastMCP\n")
	b.WriteString("from arazzo_runner import ArazzoRunner\n")
	b.WriteString("\n")

	// Initialize FastMCP server
	b.WriteString(fmt.Sprintf("# Initialize FastMCP server\n"))
	b.WriteString(fmt.Sprintf("mcp = FastMCP(%q)\n", spec.Info.Title))
	b.WriteString("\n")

	// Load the Arazzo file
	b.WriteString("# Load the Arazzo file\n")
	b.WriteString("_http = requests.Session()\n")
	b.WriteString("# Set ARAZZO_MCP_INSECURE=1 to disable TLS certificate verification (e.g. for self-signed certs).\n")
	b.WriteString("import os as _os\n")
	b.WriteString("if _os.environ.get(\"ARAZZO_MCP_INSECURE\", \"\").strip() == \"1\":\n")
	b.WriteString("    _http.verify = False\n")
	b.WriteString("    import urllib3; urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)\n")
	b.WriteString(fmt.Sprintf("runner = ArazzoRunner.from_arazzo_path(\"./arazzo/%s\", http_client=_http)\n", arazzoFileName))
	b.WriteString("\n")

	// Fix relative server URLs for URL-based source descriptions
	if hasRemoteSourceDescriptions(spec) {
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
	}

	// ── Monkey-patch: fix arazzo-runner GOTO off-by-one bug ──
	// The library's execute_next_step always advances current_step_id by +1.
	// After a GOTO sets current_step_id to the target, the next call skips
	// past it.  This patch adjusts current_step_id after a GOTO so the +1
	// correctly lands on the intended target step.
	b.WriteString("# ── Fix arazzo-runner GOTO off-by-one bug ──\n")
	b.WriteString("_original_execute_next_step = ArazzoRunner.execute_next_step\n")
	b.WriteString("\n")
	b.WriteString("def _fixed_execute_next_step(self, execution_id):\n")
	b.WriteString("    result = _original_execute_next_step(self, execution_id)\n")
	b.WriteString("    status = result.get(\"status\")\n")
	b.WriteString("    if hasattr(status, \"value\"):\n")
	b.WriteString("        status = status.value\n")
	b.WriteString("    if status == \"goto_step\":\n")
	b.WriteString("        target_step_id = result.get(\"step_id\")\n")
	b.WriteString("        state = self.execution_states[execution_id]\n")
	b.WriteString("        workflow = None\n")
	b.WriteString("        for wf in (self.arazzo_doc or {}).get(\"workflows\", []):\n")
	b.WriteString("            if wf.get(\"workflowId\") == state.workflow_id:\n")
	b.WriteString("                workflow = wf\n")
	b.WriteString("                break\n")
	b.WriteString("        if workflow:\n")
	b.WriteString("            steps = workflow.get(\"steps\", [])\n")
	b.WriteString("            for idx, step in enumerate(steps):\n")
	b.WriteString("                if step.get(\"stepId\") == target_step_id:\n")
	b.WriteString("                    if idx == 0:\n")
	b.WriteString("                        state.current_step_id = None\n")
	b.WriteString("                    else:\n")
	b.WriteString("                        state.current_step_id = steps[idx - 1].get(\"stepId\")\n")
	b.WriteString("                    break\n")
	b.WriteString("    return result\n")
	b.WriteString("\n")
	b.WriteString("ArazzoRunner.execute_next_step = _fixed_execute_next_step\n")
	b.WriteString("\n")

	// ── Generate a tool for each workflow ──
	for i, wf := range spec.Workflows {
		if i > 0 {
			b.WriteString("\n")
		}

		classified := workflowClassified[wf.WorkflowID]

		funcName := camelToSnake(wf.WorkflowID)
		docstring := workflowDocstringWithAuth(wf, classified)

		// Function params = regular inputs + credential inputs (as tool params)
		params := buildAllParams(classified.RegularInputs, classified.CredentialInputs)

		// Input dict = maps all params (original Arazzo name → Python variable name)
		inputDict := buildAllInputDict(classified.RegularInputs, classified.CredentialInputs)

		b.WriteString(fmt.Sprintf("# ── Tool %d: %s workflow\n", i+1, wf.WorkflowID))
		b.WriteString("@mcp.tool()\n")
		b.WriteString(fmt.Sprintf("async def %s(%s) -> str:\n", funcName, params))
		// Write multi-line docstring
		b.WriteString(fmt.Sprintf("    \"\"\"%s\"\"\"\n", docstring))
		b.WriteString("    try:\n")
		b.WriteString(fmt.Sprintf("        result = runner.execute_workflow(%q, {%s})\n", wf.WorkflowID, inputDict))
		b.WriteString("        if result.outputs:\n")
		b.WriteString("            return f\"Workflow Success. Outputs: {result.outputs}\"\n")
		b.WriteString("        return f\"Workflow Result: {result}\"\n")
		b.WriteString("    except Exception as e:\n")
		b.WriteString("        return f\"Workflow Error: {str(e)}\"\n")
	}

	// Main entry point
	b.WriteString("\n")
	b.WriteString("\nif __name__ == \"__main__\":\n")
	b.WriteString(fmt.Sprintf("    mcp.run(transport=\"http\", host=\"0.0.0.0\", port=%d, stateless_http=True)\n", port))

	return b.String(), nil
}

// camelToSnake converts a camelCase or PascalCase string to snake_case.
func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rune(s[i-1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					result.WriteRune('_')
				} else if unicode.IsUpper(prev) && i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
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

// workflowDocstring returns the docstring for a workflow tool function.
// DEPRECATED: Use workflowDocstringWithAuth for new code.
func workflowDocstring(wf Workflow) string {
	if wf.Summary != "" {
		return wf.Summary
	}
	if wf.Description != "" {
		return wf.Description
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
		"Please provide your WSO2 access token in the %s parameter. "+
		"If the user does not have a token, ask them to generate one from the WSO2 API Manager Developer Portal (devportal).",
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

// buildParams generates the Python function parameter list from workflow inputs.
// e.g. "pet_id: int, pet_name: str"
// DEPRECATED: Use buildParamsFromMap for new code that separates credentials.
func buildParams(wf Workflow) string { //what are the inputs
	if wf.Inputs == nil || len(wf.Inputs.Properties) == 0 {
		return ""
	}

	var parts []string
	for name, prop := range wf.Inputs.Properties {
		pyType := arazzoTypeToPython(prop.Type)
		parts = append(parts, fmt.Sprintf("%s: %s", name, pyType))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// buildParamsFromMap generates Python function parameters from the regular inputs only.
// Credential inputs are excluded because they come from environment variables.
func buildParamsFromMap(inputs map[string]InputProperty) string {
	if len(inputs) == 0 {
		return ""
	}
	var parts []string
	for name, prop := range inputs {
		pyType := arazzoTypeToPython(prop.Type)
		parts = append(parts, fmt.Sprintf("%s: %s", name, pyType))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// buildInputDict generates the Python dict literal mapping original param names
// to their snake_case function argument names.
// e.g. `"petId": pet_id, "petName": pet_name`
// DEPRECATED: Use buildInputDictWithCredentials for new code.
func buildInputDict(wf Workflow) string { //actial values for the inputs
	if wf.Inputs == nil || len(wf.Inputs.Properties) == 0 {
		return ""
	}

	var parts []string
	for name := range wf.Inputs.Properties {
		parts = append(parts, fmt.Sprintf("%q: %s", name, name))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// buildInputDictWithCredentials generates the Python dict literal for execute_workflow(),
// combining regular param references (function args) with credential env var references.
// e.g. `"petId": petId, "apiKey": PETSTORE_API_API_KEY`
func buildInputDictWithCredentials(
	regular map[string]InputProperty,
	credentials map[string]InputProperty,
	specTitle string,
) string {
	var parts []string
	for name := range regular {
		parts = append(parts, fmt.Sprintf("%q: %s", name, name))
	}
	for name := range credentials {
		envVar := CredentialEnvVarName(specTitle, name)
		parts = append(parts, fmt.Sprintf("%q: %s", name, envVar))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
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
// regular inputs (names used as-is) and credential inputs (names converted to
// valid Python identifiers via toPythonParamName). All params become str/int/etc.
func buildAllParams(regular map[string]InputProperty, credentials map[string]InputProperty) string {
	var parts []string
	for name, prop := range regular {
		pyType := arazzoTypeToPython(prop.Type)
		parts = append(parts, fmt.Sprintf("%s: %s", name, pyType))
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
// mapping each original Arazzo input name to its Python variable name.
// Regular inputs use their original name as-is (e.g. "petId": petId).
// Credential inputs use the converted Python name (e.g. "internalKey": internal_key).
func buildAllInputDict(regular map[string]InputProperty, credentials map[string]InputProperty) string {
	var parts []string
	for name := range regular {
		parts = append(parts, fmt.Sprintf("%q: %s", name, name))
	}
	for name := range credentials {
		pyName := toPythonParamName(name)
		parts = append(parts, fmt.Sprintf("%q: %s", name, pyName))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// sortedKeys returns the keys of a map sorted alphabetically.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
