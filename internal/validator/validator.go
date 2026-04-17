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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ─── Severity ──────────────────────────────────────────────────────────────────

// Severity represents the importance level of a validation finding.
type Severity int

const (
	SevPass    Severity = iota // Check passed
	SevInfo                    // Informational note
	SevWarning                 // Potential problem, spec still valid
	SevError                   // Spec is invalid per Arazzo 1.0.x
)

// ─── Issue / Result ────────────────────────────────────────────────────────────

// Issue is a single validation finding.
type Issue struct {
	Severity Severity
	Category string // "structure", "source", "workflow", "step", "expression"
	Path     string // human-readable YAML path, e.g. "workflows[0].steps[1]"
	Message  string
}

// Result aggregates all findings for one file.
type Result struct {
	FilePath string
	Issues   []Issue
	// Engine identifies which validator produced this result.
	// "spectral" = Spectral CLI, "builtin" = built-in Go validator.
	Engine string
}

func (r *Result) add(sev Severity, cat, path, msg string) {
	r.Issues = append(r.Issues, Issue{sev, cat, path, msg})
}
func (r *Result) pass(cat, path, msg string)    { r.add(SevPass, cat, path, msg) }
func (r *Result) info(cat, path, msg string)    { r.add(SevInfo, cat, path, msg) }
func (r *Result) warning(cat, path, msg string) { r.add(SevWarning, cat, path, msg) }
func (r *Result) errorf(cat, path, msg string)  { r.add(SevError, cat, path, msg) }

// ErrorCount returns the number of error-level issues.
func (r *Result) ErrorCount() int {
	n := 0
	for _, i := range r.Issues {
		if i.Severity == SevError {
			n++
		}
	}
	return n
}

// WarningCount returns the number of warning-level issues.
func (r *Result) WarningCount() int {
	n := 0
	for _, i := range r.Issues {
		if i.Severity == SevWarning {
			n++
		}
	}
	return n
}

// PassCount returns the number of pass-level issues.
func (r *Result) PassCount() int {
	n := 0
	for _, i := range r.Issues {
		if i.Severity == SevPass {
			n++
		}
	}
	return n
}

// HasErrors returns true when at least one error was recorded.
func (r *Result) HasErrors() bool { return r.ErrorCount() > 0 }

// ─── ANSI helpers ──────────────────────────────────────────────────────────────

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

func sevIcon(s Severity) string {
	switch s {
	case SevPass:
		return colorGreen + "  ✓" + colorReset
	case SevInfo:
		return colorCyan + "  ℹ" + colorReset
	case SevWarning:
		return colorYellow + "  ⚠" + colorReset
	case SevError:
		return colorRed + "  ✗" + colorReset
	}
	return "  "
}

// PrintReport writes a human-readable, colour-coded report to stdout.
func (r *Result) PrintReport() {
	fmt.Printf("\n%s%sValidating: %s%s\n", colorBold, colorCyan, r.FilePath, colorReset)
	fmt.Println(strings.Repeat("─", 60))

	// Print all issues grouped by category (skip SevPass lines for Spectral —
	// they are "all clear" sentinel entries, not per-rule pass lines).
	lastCategory := ""
	for _, iss := range r.Issues {
		if r.Engine == "spectral" && iss.Severity == SevPass {
			continue // handled in the summary block below
		}
		if iss.Category != lastCategory {
			header := categoryHeader(iss.Category)
			fmt.Printf("\n%s%s%s\n", colorBold, header, colorReset)
			lastCategory = iss.Category
		}
		pathStr := ""
		if iss.Path != "" {
			pathStr = colorDim + " [" + iss.Path + "]" + colorReset
		}
		fmt.Printf("%s %s%s\n", sevIcon(iss.Severity), iss.Message, pathStr)
	}

	// ── Summary block ──
	fmt.Println()
	fmt.Println(strings.Repeat("━", 60))
	errors := r.ErrorCount()
	warnings := r.WarningCount()

	if errors == 0 {
		fmt.Printf("%s%sValidation Result: PASSED%s\n", colorBold, colorGreen, colorReset)
	} else {
		fmt.Printf("%s%sValidation Result: FAILED%s\n", colorBold, colorRed, colorReset)
	}

	if r.Engine == "spectral" {
		// Show a meaningful Spectral-specific summary line instead of a raw pass count.
		if errors == 0 && warnings == 0 {
			fmt.Printf("  %s✓ All arazzo rules passed%s\n", colorGreen, colorReset)
		} else {
			total := errors + warnings
			fmt.Printf("  %sℹ Spectral found %d issue(s) — see details above%s\n", colorCyan, total, colorReset)
		}
		fmt.Printf("  %s⚠ %d warnings%s\n", colorYellow, warnings, colorReset)
		fmt.Printf("  %s✗ %d errors%s\n", colorRed, errors, colorReset)
		fmt.Printf("  %s─ Validated using Spectral (spectral:arazzo ruleset)%s\n", colorDim, colorReset)
	} else {
		// Built-in validator: show per-rule pass count.
		passes := r.PassCount()
		fmt.Printf("  %s✓ %d checks passed%s\n", colorGreen, passes, colorReset)
		fmt.Printf("  %s⚠ %d warnings%s\n", colorYellow, warnings, colorReset)
		fmt.Printf("  %s✗ %d errors%s\n", colorRed, errors, colorReset)
		fmt.Printf("  %s─ Using built-in validator (Spectral not available)%s\n", colorDim, colorReset)
	}
	fmt.Println()
}

func categoryHeader(cat string) string {
	switch cat {
	case "structure":
		return "📋 Structure"
	case "source":
		return "📦 Source Descriptions"
	case "workflow":
		return "🔄 Workflow"
	case "step":
		return "    📌 Steps"
	case "expression":
		return "    🔗 Expressions"
	}
	return cat
}

// ─── YAML helpers ──────────────────────────────────────────────────────────────

func getString(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	mm, _ := m[key].(map[string]interface{})
	return mm
}

func getSlice(m map[string]interface{}, key string) []interface{} {
	s, _ := m[key].([]interface{})
	return s
}

func toMapSlice(arr []interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	for _, v := range arr {
		if mm, ok := v.(map[string]interface{}); ok {
			out = append(out, mm)
		}
	}
	return out
}

// ─── Public entry point ────────────────────────────────────────────────────────

// ValidateFile performs comprehensive validation of a single Arazzo spec file.
// folderPath is the directory containing the file (for resolving relative source paths).
// If checkRemote is true, HTTP(S) source URLs are probed for accessibility.
func ValidateFile(filePath string, folderPath string, checkRemote bool) *Result {
	r := &Result{FilePath: filePath, Engine: "builtin"}

	// ── 1. Read file ────────────────────────────────────────────────────────
	data, err := os.ReadFile(filePath)
	if err != nil {
		r.errorf("structure", filePath, fmt.Sprintf("Cannot read file: %v", err))
		return r
	}

	// ── 2. Parse YAML ───────────────────────────────────────────────────────
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		r.errorf("structure", "", fmt.Sprintf("Invalid YAML syntax: %v", err))
		return r
	}
	r.pass("structure", "", "Valid YAML syntax")

	// ── 3. Arazzo version key ───────────────────────────────────────────────
	arazzoRaw, hasArazzo := raw["arazzo"]
	if !hasArazzo {
		r.errorf("structure", "arazzo", "Missing top-level 'arazzo' key — not an Arazzo specification")
		return r
	}
	// arazzo version can be a float (1.0) or string ("1.0.1") depending on YAML parsing
	arazzoVersion := fmt.Sprintf("%v", arazzoRaw)
	if strings.HasPrefix(arazzoVersion, "1.0") {
		r.pass("structure", "arazzo", fmt.Sprintf("Arazzo version: %s", arazzoVersion))
	} else {
		r.warning("structure", "arazzo",
			fmt.Sprintf("Version '%s' may not be fully supported (expected 1.0.x)", arazzoVersion))
	}

	// ── 4. Info ─────────────────────────────────────────────────────────────
	validateInfo(r, raw)

	// ── 5. Source descriptions ──────────────────────────────────────────────
	validateSources(r, raw, folderPath, checkRemote)

	// ── 6. Workflows ────────────────────────────────────────────────────────
	validateWorkflows(r, raw)

	return r
}

// ─── Validators ────────────────────────────────────────────────────────────────

func validateInfo(r *Result, raw map[string]interface{}) {
	info := getMap(raw, "info")
	if info == nil {
		r.errorf("structure", "info", "Missing required 'info' object")
		return
	}
	title := getString(info, "title")
	version := getString(info, "version")

	if title == "" {
		r.errorf("structure", "info.title", "Missing required field 'info.title'")
	}
	if version == "" {
		r.errorf("structure", "info.version", "Missing required field 'info.version'")
	}
	if title != "" && version != "" {
		r.pass("structure", "info", fmt.Sprintf("Info: %s v%s", title, version))
	}

	// Optional description – just note it
	desc := getString(info, "description")
	if desc != "" {
		r.pass("structure", "info.description", "Has description")
	}
}

func validateSources(r *Result, raw map[string]interface{}, folderPath string, checkRemote bool) {
	arr := getSlice(raw, "sourceDescriptions")
	if len(arr) == 0 {
		r.errorf("source", "sourceDescriptions", "Missing or empty 'sourceDescriptions' — at least one source is required")
		return
	}

	sources := toMapSlice(arr)
	r.pass("source", "sourceDescriptions", fmt.Sprintf("Found %d source description(s)", len(sources)))

	seenNames := make(map[string]bool)
	for i, sd := range sources {
		path := fmt.Sprintf("sourceDescriptions[%d]", i)
		name := getString(sd, "name")
		url := getString(sd, "url")
		sdType := getString(sd, "type")

		// Required fields
		if name == "" {
			r.errorf("source", path+".name", "Missing required field 'name'")
		}
		if url == "" {
			r.errorf("source", path+".url", "Missing required field 'url'")
		}
		if sdType == "" {
			r.errorf("source", path+".type", "Missing required field 'type'")
		}

		// Type validation
		if sdType != "" && sdType != "openapi" && sdType != "arazzo" {
			r.errorf("source", path+".type",
				fmt.Sprintf("Invalid type '%s' — must be 'openapi' or 'arazzo'", sdType))
		}

		// Duplicate name check
		if name != "" {
			if seenNames[name] {
				r.errorf("source", path+".name",
					fmt.Sprintf("Duplicate source name '%s'", name))
			}
			seenNames[name] = true
		}

		// Accessibility check
		if url != "" {
			if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
				if checkRemote {
					if err := probeURL(url); err != nil {
						r.errorf("source", path,
							fmt.Sprintf("[%s] Remote URL not accessible: %s — %v", name, url, err))
					} else {
						r.pass("source", path,
							fmt.Sprintf("[%s] %s → %s (accessible)", name, sdType, url))
					}
				} else {
					r.pass("source", path,
						fmt.Sprintf("[%s] %s → %s (remote, skipped accessibility check)", name, sdType, url))
				}
			} else {
				// Local file
				resolvedPath := filepath.Join(folderPath, url)
				if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
					r.errorf("source", path,
						fmt.Sprintf("[%s] Local file not found: %s", name, url))
				} else {
					r.pass("source", path,
						fmt.Sprintf("[%s] %s → %s (exists)", name, sdType, url))
				}
			}
		}
	}
}

func probeURL(url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		// Retry with GET — some servers block HEAD
		resp, err = client.Get(url)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// ─── Workflow validation ───────────────────────────────────────────────────────

func validateWorkflows(r *Result, raw map[string]interface{}) {
	arr := getSlice(raw, "workflows")
	if len(arr) == 0 {
		r.errorf("workflow", "workflows", "Missing or empty 'workflows' — at least one workflow is required")
		return
	}

	workflows := toMapSlice(arr)
	r.pass("workflow", "workflows", fmt.Sprintf("Found %d workflow(s)", len(workflows)))

	seenIDs := make(map[string]bool)
	// Collect all workflow IDs first for cross-reference validation
	allWorkflowIDs := make(map[string]bool)
	for _, wf := range workflows {
		id := getString(wf, "workflowId")
		if id != "" {
			allWorkflowIDs[id] = true
		}
	}

	for i, wf := range workflows {
		path := fmt.Sprintf("workflows[%d]", i)
		wfID := getString(wf, "workflowId")

		if wfID == "" {
			r.errorf("workflow", path+".workflowId", "Missing required field 'workflowId'")
			continue
		}

		// Duplicate check
		if seenIDs[wfID] {
			r.errorf("workflow", path, fmt.Sprintf("Duplicate workflowId '%s'", wfID))
		}
		seenIDs[wfID] = true

		// Collect step IDs for this workflow
		stepsArr := getSlice(wf, "steps")
		if len(stepsArr) == 0 {
			r.errorf("workflow", path+".steps", fmt.Sprintf("Workflow '%s' has no steps", wfID))
			continue
		}
		steps := toMapSlice(stepsArr)

		stepIDs := collectStepIDs(steps)
		r.pass("workflow", path,
			fmt.Sprintf("Workflow '%s' (%d step(s))", wfID, len(steps)))

		// Validate inputs
		validateWorkflowInputs(r, wf, path, wfID)

		// Validate each step
		seenStepIDs := make(map[string]bool)
		for j, step := range steps {
			stepPath := fmt.Sprintf("%s.steps[%d]", path, j)
			validateStep(r, step, stepPath, stepIDs, allWorkflowIDs, seenStepIDs, wf)
		}

		// Validate outputs
		validateWorkflowOutputs(r, wf, path, stepIDs)
	}
}

func collectStepIDs(steps []map[string]interface{}) map[string]bool {
	ids := make(map[string]bool)
	for _, s := range steps {
		id := getString(s, "stepId")
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

func validateWorkflowInputs(r *Result, wf map[string]interface{}, path, wfID string) {
	inputs := getMap(wf, "inputs")
	if inputs == nil {
		r.info("workflow", path+".inputs", fmt.Sprintf("Workflow '%s' has no inputs defined", wfID))
		return
	}

	props := getMap(inputs, "properties")
	if props == nil || len(props) == 0 {
		r.warning("workflow", path+".inputs",
			fmt.Sprintf("Workflow '%s' has inputs but no properties defined", wfID))
		return
	}

	var inputNames []string
	for name, rawProp := range props {
		inputNames = append(inputNames, name)

		// Validate each input property
		switch prop := rawProp.(type) {
		case map[string]interface{}:
			propType := getString(prop, "type")
			if propType == "" {
				r.warning("workflow", path+".inputs.properties."+name,
					fmt.Sprintf("Input '%s' has no type specified", name))
			} else {
				validTypes := map[string]bool{
					"string": true, "integer": true, "number": true,
					"boolean": true, "object": true, "array": true,
				}
				if !validTypes[propType] {
					r.warning("workflow", path+".inputs.properties."+name,
						fmt.Sprintf("Input '%s' has unusual type '%s'", name, propType))
				}
			}
		}
	}

	r.pass("workflow", path+".inputs",
		fmt.Sprintf("Inputs: %s", strings.Join(inputNames, ", ")))
}

// ─── Step validation ───────────────────────────────────────────────────────────

func validateStep(r *Result, step map[string]interface{}, path string,
	stepIDs, workflowIDs map[string]bool, seenStepIDs map[string]bool,
	wf map[string]interface{}) {

	stepID := getString(step, "stepId")
	if stepID == "" {
		r.errorf("step", path+".stepId", "Missing required field 'stepId'")
		return
	}

	// Duplicate stepId
	if seenStepIDs[stepID] {
		r.errorf("step", path, fmt.Sprintf("Duplicate stepId '%s' within workflow", stepID))
	}
	seenStepIDs[stepID] = true

	// Must have operationId or workflowId
	opID := getString(step, "operationId")
	nestedWfID := getString(step, "workflowId")
	_, hasOperationPath := step["operationPath"]

	if opID == "" && nestedWfID == "" && !hasOperationPath {
		r.errorf("step", path,
			fmt.Sprintf("Step '%s' must have 'operationId', 'operationPath', or 'workflowId'", stepID))
	} else if opID != "" && nestedWfID != "" {
		r.warning("step", path,
			fmt.Sprintf("Step '%s' has both 'operationId' and 'workflowId' — only one should be used", stepID))
	} else {
		target := opID
		kind := "operationId"
		if nestedWfID != "" {
			target = nestedWfID
			kind = "workflowId"
			// Validate nested workflow reference
			if !workflowIDs[nestedWfID] {
				r.warning("step", path,
					fmt.Sprintf("Step '%s' references workflowId '%s' which is not defined in this spec",
						stepID, nestedWfID))
			}
		}
		if hasOperationPath {
			kind = "operationPath"
			target = fmt.Sprintf("%v", step["operationPath"])
		}
		r.pass("step", path, fmt.Sprintf("Step '%s': %s=%s", stepID, kind, target))
	}

	// Validate parameters
	validateParameters(r, step, path, stepID, wf)

	// Validate requestBody
	validateRequestBody(r, step, path, stepID)

	// Validate successCriteria
	validateSuccessCriteria(r, step, path, stepID)

	// Validate onSuccess / onFailure
	validateActions(r, step, "onSuccess", path, stepID, stepIDs, workflowIDs)
	validateActions(r, step, "onFailure", path, stepID, stepIDs, workflowIDs)

	// Validate step outputs
	validateStepOutputs(r, step, path, stepID)
}

func validateParameters(r *Result, step map[string]interface{}, path, stepID string,
	wf map[string]interface{}) {

	paramsArr := getSlice(step, "parameters")
	if paramsArr == nil {
		return // parameters are optional
	}

	params := toMapSlice(paramsArr)
	validIn := map[string]bool{
		"path": true, "query": true, "header": true,
		"cookie": true, "body": true,
	}

	var paramNames []string
	for k, p := range params {
		pPath := fmt.Sprintf("%s.parameters[%d]", path, k)
		name := getString(p, "name")
		in := getString(p, "in")

		if name == "" {
			r.errorf("step", pPath, fmt.Sprintf("Step '%s': parameter missing 'name'", stepID))
			continue
		}
		if in == "" {
			r.errorf("step", pPath,
				fmt.Sprintf("Step '%s': parameter '%s' missing 'in' field", stepID, name))
		} else if !validIn[in] {
			r.warning("step", pPath,
				fmt.Sprintf("Step '%s': parameter '%s' has unusual 'in' value '%s'", stepID, name, in))
		}

		// Check value exists
		if _, hasValue := p["value"]; !hasValue {
			r.errorf("step", pPath,
				fmt.Sprintf("Step '%s': parameter '%s' missing 'value'", stepID, name))
		} else {
			// Validate expression in value
			valStr := fmt.Sprintf("%v", p["value"])
			if strings.HasPrefix(valStr, "$") {
				validateExpression(r, valStr, pPath+".value", stepID, wf)
			}
		}

		paramNames = append(paramNames, fmt.Sprintf("%s (%s)", name, in))
	}

	if len(paramNames) > 0 {
		r.pass("step", path+".parameters",
			fmt.Sprintf("Step '%s': parameters: %s", stepID, strings.Join(paramNames, ", ")))
	}
}

func validateRequestBody(r *Result, step map[string]interface{}, path, stepID string) {
	rb := getMap(step, "requestBody")
	if rb == nil {
		return // optional
	}

	ct := getString(rb, "contentType")
	if ct == "" {
		r.warning("step", path+".requestBody",
			fmt.Sprintf("Step '%s': requestBody missing 'contentType'", stepID))
	}

	_, hasPayload := rb["payload"]
	_, hasRef := rb["$ref"]
	if !hasPayload && !hasRef {
		r.warning("step", path+".requestBody",
			fmt.Sprintf("Step '%s': requestBody has no 'payload' or '$ref'", stepID))
	} else {
		r.pass("step", path+".requestBody",
			fmt.Sprintf("Step '%s': has requestBody (contentType=%s)", stepID, ct))
	}
}

func validateSuccessCriteria(r *Result, step map[string]interface{}, path, stepID string) {
	scArr := getSlice(step, "successCriteria")
	if scArr == nil {
		r.info("step", path+".successCriteria",
			fmt.Sprintf("Step '%s': no successCriteria defined (any response is success)", stepID))
		return
	}

	criteria := toMapSlice(scArr)
	if len(criteria) == 0 {
		r.warning("step", path+".successCriteria",
			fmt.Sprintf("Step '%s': successCriteria is empty array", stepID))
		return
	}

	// Warn about multiple criteria with conflicting $statusCode checks
	statusChecks := []string{}
	for k, c := range criteria {
		cPath := fmt.Sprintf("%s.successCriteria[%d]", path, k)
		cond := getString(c, "condition")
		if cond == "" {
			r.errorf("step", cPath,
				fmt.Sprintf("Step '%s': successCriteria entry missing 'condition'", stepID))
			continue
		}

		// Basic expression syntax check
		if !isValidConditionExpression(cond) {
			r.warning("step", cPath,
				fmt.Sprintf("Step '%s': condition '%s' may have invalid syntax", stepID, cond))
		} else {
			r.pass("step", cPath,
				fmt.Sprintf("Step '%s': successCriteria: %s", stepID, cond))
		}

		// Track $statusCode checks for conflict detection
		if strings.Contains(cond, "$statusCode") {
			statusChecks = append(statusChecks, cond)
		}
	}

	// Warn about AND-ed status code checks (arazzo-runner ANDs all criteria)
	if len(statusChecks) > 1 {
		r.warning("step", path+".successCriteria",
			fmt.Sprintf("Step '%s': multiple $statusCode criteria are AND-ed together "+
				"(%s) — this may never be satisfied simultaneously. "+
				"Use separate onSuccess/onFailure handlers with 'criteria' instead",
				stepID, strings.Join(statusChecks, " AND ")))
	}
}

func validateActions(r *Result, step map[string]interface{}, actionKey string,
	path, stepID string, stepIDs, workflowIDs map[string]bool) {

	arr := getSlice(step, actionKey)
	if arr == nil {
		return // optional
	}

	actions := toMapSlice(arr)
	for k, action := range actions {
		aPath := fmt.Sprintf("%s.%s[%d]", path, actionKey, k)
		name := getString(action, "name")
		aType := getString(action, "type")

		if name == "" {
			r.errorf("step", aPath,
				fmt.Sprintf("Step '%s': %s action missing 'name'", stepID, actionKey))
		}
		if aType == "" {
			r.errorf("step", aPath,
				fmt.Sprintf("Step '%s': %s action '%s' missing 'type'", stepID, actionKey, name))
			continue
		}

		validTypes := map[string]bool{"goto": true, "end": true, "retry": true}
		if !validTypes[aType] {
			r.errorf("step", aPath,
				fmt.Sprintf("Step '%s': %s action '%s' has invalid type '%s' (must be goto, end, or retry)",
					stepID, actionKey, name, aType))
			continue
		}

		switch aType {
		case "goto":
			gotoStepID := getString(action, "stepId")
			gotoWfID := getString(action, "workflowId")

			if gotoStepID == "" && gotoWfID == "" {
				r.errorf("step", aPath,
					fmt.Sprintf("Step '%s': goto action '%s' must specify 'stepId' or 'workflowId'",
						stepID, name))
			} else if gotoStepID != "" {
				// Validate step target
				if !stepIDs[gotoStepID] {
					r.errorf("step", aPath,
						fmt.Sprintf("Step '%s': goto references unknown stepId '%s'",
							stepID, gotoStepID))
				} else if gotoStepID == stepID {
					r.warning("step", aPath,
						fmt.Sprintf("Step '%s': goto targets itself — potential infinite loop",
							stepID))
				} else {
					r.pass("step", aPath,
						fmt.Sprintf("Step '%s': %s → goto step '%s'", stepID, actionKey, gotoStepID))
				}
			} else {
				// goto workflowId
				if !workflowIDs[gotoWfID] {
					r.warning("step", aPath,
						fmt.Sprintf("Step '%s': goto references workflowId '%s' not defined in this spec",
							stepID, gotoWfID))
				} else {
					r.pass("step", aPath,
						fmt.Sprintf("Step '%s': %s → goto workflow '%s'", stepID, actionKey, gotoWfID))
				}
			}

		case "end":
			r.pass("step", aPath,
				fmt.Sprintf("Step '%s': %s → end workflow", stepID, actionKey))

		case "retry":
			r.pass("step", aPath,
				fmt.Sprintf("Step '%s': %s → retry", stepID, actionKey))

			// Check for retryAfter and retryLimit
			if _, has := action["retryAfter"]; !has {
				r.info("step", aPath,
					fmt.Sprintf("Step '%s': retry action '%s' has no 'retryAfter'",
						stepID, name))
			}
			if _, has := action["retryLimit"]; !has {
				r.info("step", aPath,
					fmt.Sprintf("Step '%s': retry action '%s' has no 'retryLimit' — may retry indefinitely",
						stepID, name))
			}
		}

		// Validate criteria on actions (if present)
		criteriaArr := getSlice(action, "criteria")
		if criteriaArr != nil {
			crit := toMapSlice(criteriaArr)
			for ci, c := range crit {
				cPath := fmt.Sprintf("%s.criteria[%d]", aPath, ci)
				cond := getString(c, "condition")
				if cond == "" {
					r.errorf("step", cPath,
						fmt.Sprintf("Step '%s': action '%s' criteria entry missing 'condition'",
							stepID, name))
				} else if !isValidConditionExpression(cond) {
					r.warning("step", cPath,
						fmt.Sprintf("Step '%s': action criteria '%s' may have invalid syntax",
							stepID, cond))
				} else {
					r.pass("step", cPath,
						fmt.Sprintf("Step '%s': action '%s' criteria: %s", stepID, name, cond))
				}
			}
		}
	}
}

func validateStepOutputs(r *Result, step map[string]interface{}, path, stepID string) {
	outputs := getMap(step, "outputs")
	if outputs == nil {
		return
	}

	var names []string
	for name, rawExpr := range outputs {
		names = append(names, name)
		exprStr := fmt.Sprintf("%v", rawExpr)
		if strings.HasPrefix(exprStr, "$") {
			if !isValidOutputExpression(exprStr) {
				r.warning("expression", path+".outputs."+name,
					fmt.Sprintf("Step '%s': output '%s' expression '%s' may be invalid",
						stepID, name, exprStr))
			}
		}
	}

	r.pass("step", path+".outputs",
		fmt.Sprintf("Step '%s': outputs: %s", stepID, strings.Join(names, ", ")))
}

// ─── Workflow outputs ──────────────────────────────────────────────────────────

func validateWorkflowOutputs(r *Result, wf map[string]interface{}, path string,
	stepIDs map[string]bool) {

	outputs := getMap(wf, "outputs")
	if outputs == nil {
		return
	}

	var names []string
	for name, rawExpr := range outputs {
		names = append(names, name)
		exprStr := fmt.Sprintf("%v", rawExpr)

		// Check $steps.xxx references
		if strings.HasPrefix(exprStr, "$steps.") {
			parts := strings.SplitN(exprStr, ".", 4)
			if len(parts) >= 2 {
				refStepID := parts[1]
				if !stepIDs[refStepID] {
					r.errorf("expression", path+".outputs."+name,
						fmt.Sprintf("Output '%s' references unknown step '%s' in expression '%s'",
							name, refStepID, exprStr))
					continue
				}
			}
		}
	}

	if len(names) > 0 {
		r.pass("workflow", path+".outputs",
			fmt.Sprintf("Workflow outputs: %s", strings.Join(names, ", ")))
	}
}

// ─── Expression validation ─────────────────────────────────────────────────────

// Known runtime expression prefixes per Arazzo 1.0.x spec
var validExprPrefixes = []string{
	"$url", "$method", "$statusCode",
	"$request.header.", "$request.query.", "$request.body",
	"$response.header.", "$response.body",
	"$inputs.", "$outputs.",
	"$steps.", "$workflows.",
	"$sourceDescriptions.", "$components.",
}

func validateExpression(r *Result, expr, path, stepID string, wf map[string]interface{}) {
	if !strings.HasPrefix(expr, "$") {
		return
	}

	// Check if expression matches any known prefix
	valid := false
	for _, pfx := range validExprPrefixes {
		if strings.HasPrefix(expr, pfx) || expr == strings.TrimSuffix(pfx, ".") {
			valid = true
			break
		}
	}

	if !valid {
		r.warning("expression", path,
			fmt.Sprintf("Step '%s': expression '%s' doesn't match known Arazzo runtime expression patterns",
				stepID, expr))
		return
	}

	// Validate $inputs.xxx — check the input exists
	if strings.HasPrefix(expr, "$inputs.") {
		inputName := strings.TrimPrefix(expr, "$inputs.")
		inputs := getMap(wf, "inputs")
		if inputs != nil {
			props := getMap(inputs, "properties")
			if props != nil {
				if _, exists := props[inputName]; !exists {
					r.warning("expression", path,
						fmt.Sprintf("Step '%s': expression '%s' references input '%s' which is not defined in workflow inputs",
							stepID, expr, inputName))
				}
			}
		}
	}
}

// isValidConditionExpression checks that a successCriteria/criteria condition
// has valid syntax: an expression, a comparison operator, and a value.
var conditionPattern = regexp.MustCompile(
	`^\$[a-zA-Z][a-zA-Z0-9_.#/]*\s*(==|!=|<|>|<=|>=)\s*.+$`)

func isValidConditionExpression(cond string) bool {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return false
	}
	return conditionPattern.MatchString(cond)
}

// isValidOutputExpression checks a step/workflow output expression
var outputExprPattern = regexp.MustCompile(`^\$[a-zA-Z][a-zA-Z0-9_.#/]*$`)

func isValidOutputExpression(expr string) bool {
	return outputExprPattern.MatchString(strings.TrimSpace(expr))
}

// ─── Shared helpers ────────────────────────────────────────────────────────────

// readAndParseYAML reads and parses a YAML file into a map.
func readAndParseYAML(filePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
