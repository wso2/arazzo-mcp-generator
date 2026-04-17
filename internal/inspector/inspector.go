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

// Package inspector parses an Arazzo specification and renders a human-readable
// summary of its structure, workflows, steps, and step-flow routing.
package inspector

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ─── ANSI helpers ──────────────────────────────────────────────────────────────

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiCyan   = "\033[36m"
	ansiBlue   = "\033[34m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiMag    = "\033[35m"
	ansiGray   = "\033[90m"
)

func bold(s string) string   { return ansiBold + s + ansiReset }
func cyan(s string) string   { return ansiCyan + s + ansiReset }
func green(s string) string  { return ansiGreen + s + ansiReset }
func yellow(s string) string { return ansiYellow + s + ansiReset }
func dim(s string) string    { return ansiDim + s + ansiReset }
func mag(s string) string    { return ansiMag + s + ansiReset }
func blue(s string) string   { return ansiBlue + s + ansiReset }
func gray(s string) string   { return ansiGray + s + ansiReset }

// ─── YAML helpers (same lightweight approach as validator) ─────────────────────

func str(m map[string]interface{}, k string) string {
	s, _ := m[k].(string)
	return s
}

func obj(m map[string]interface{}, k string) map[string]interface{} {
	mm, _ := m[k].(map[string]interface{})
	return mm
}

func arr(m map[string]interface{}, k string) []interface{} {
	a, _ := m[k].([]interface{})
	return a
}

func maps(a []interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	for _, v := range a {
		if mm, ok := v.(map[string]interface{}); ok {
			out = append(out, mm)
		}
	}
	return out
}

// ─── Public entry point ────────────────────────────────────────────────────────

// Inspect reads, parses, and prints a comprehensive human-readable report of the
// given Arazzo specification file.
func Inspect(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	p := &printer{}
	p.printSpec(filePath, raw)
	return nil
}

// ─── Printer ───────────────────────────────────────────────────────────────────

type printer struct{}

func (p *printer) printSpec(filePath string, raw map[string]interface{}) {
	// ── Banner ──────────────────────────────────────────────────────────────
	banner := " ARAZZO SPECIFICATION INSPECTOR "
	line := strings.Repeat("═", len(banner))
	fmt.Println()
	fmt.Println(bold(cyan("╔" + line + "╗")))
	fmt.Println(bold(cyan("║")) + bold(banner) + bold(cyan("║")))
	fmt.Println(bold(cyan("╚" + line + "╝")))
	fmt.Println()

	// ── Overview ────────────────────────────────────────────────────────────
	info := obj(raw, "info")
	arazzoVer := fmt.Sprintf("%v", raw["arazzo"])

	fmt.Println(bold("📋 OVERVIEW"))
	fmt.Println(strings.Repeat("─", 52))
	if info != nil {
		printKV("  Title", str(info, "title"))
		printKV("  Version", str(info, "version"))
		printKV("  Arazzo", arazzoVer)
		if desc := str(info, "description"); desc != "" {
			wrapped := wrapText(desc, 42)
			printKV("  Description", wrapped[0])
			for _, line := range wrapped[1:] {
				fmt.Printf("  %s%s%s\n", ansiGray, line, ansiReset)
			}
		}
	}
	fmt.Println()

	// ── Source Descriptions ─────────────────────────────────────────────────
	sources := maps(arr(raw, "sourceDescriptions"))
	fmt.Printf("%s %s%d%s\n", bold("📦 SOURCE DESCRIPTIONS"), gray("("), len(sources), gray(")"))
	fmt.Println(strings.Repeat("─", 52))
	for i, sd := range sources {
		name := str(sd, "name")
		sdType := str(sd, "type")
		url := str(sd, "url")
		connector := "├─"
		if i == len(sources)-1 {
			connector = "└─"
		}
		fmt.Printf("  %s %s %s\n", gray(connector), bold(name), dim("["+sdType+"]"))
		indent := "  │  "
		if i == len(sources)-1 {
			indent = "     "
		}
		fmt.Printf("%s%s\n", indent, dim(url))
	}
	fmt.Println()

	// ── Workflows ───────────────────────────────────────────────────────────
	workflows := maps(arr(raw, "workflows"))
	fmt.Printf("%s %s%d%s\n", bold("🔄 WORKFLOWS"), gray("("), len(workflows), gray(")"))

	for wfIdx, wf := range workflows {
		p.printWorkflow(wfIdx+1, len(workflows), wf)
	}
}

func (p *printer) printWorkflow(idx, total int, wf map[string]interface{}) {
	wfID := str(wf, "workflowId")
	summary := str(wf, "summary")
	if summary == "" {
		summary = str(wf, "description")
	}

	// Workflow header box
	label := fmt.Sprintf(" Workflow %d/%d: %s ", idx, total, wfID)
	boxWidth := max(52, len(label)+2)
	fmt.Println(strings.Repeat("─", boxWidth+2))
	topPad := strings.Repeat("─", boxWidth-len(label))
	fmt.Printf("  %s%s%s%s\n",
		bold(cyan("┌")), bold(cyan(label)), bold(cyan(topPad)), bold(cyan("┐")))
	fmt.Printf("  %s%s%s\n",
		bold(cyan("│")),
		dim(padRight(" "+truncate(summary, boxWidth-1), boxWidth)),
		bold(cyan("│")))
	fmt.Printf("  %s%s%s\n",
		bold(cyan("└")), bold(cyan(strings.Repeat("─", boxWidth))), bold(cyan("┘")))
	fmt.Println()

	// Inputs
	inputs := obj(wf, "inputs")
	if inputs != nil {
		props := obj(inputs, "properties")
		if len(props) > 0 {
			fmt.Println(bold("  📌 INPUTS"))
			keys := sortedKeys(props)
			for _, name := range keys {
				propType := ""
				if propMap, ok := props[name].(map[string]interface{}); ok {
					propType = str(propMap, "type")
					if desc := str(propMap, "description"); desc != "" {
						fmt.Printf("    %s %s %s — %s\n",
							green("•"), bold(name), gray("("+propType+")"), dim(truncate(desc, 48)))
						continue
					}
				}
				fmt.Printf("    %s %s %s\n", green("•"), bold(name), gray("("+propType+")"))
			}
			fmt.Println()
		}
	}

	// Steps
	steps := maps(arr(wf, "steps"))
	if len(steps) > 0 {
		fmt.Println(bold("  📌 STEP FLOW"))
		// Build stepId → index map for flow rendering
		stepIdx := make(map[string]int)
		for i, s := range steps {
			stepIdx[str(s, "stepId")] = i + 1
		}
		for i, step := range steps {
			p.printStep(i+1, len(steps), step, stepIdx)
		}
	}

	// Outputs
	outputs := obj(wf, "outputs")
	if len(outputs) > 0 {
		fmt.Println(bold("  📌 WORKFLOW OUTPUTS"))
		keys := sortedKeys(outputs)
		for _, name := range keys {
			expr := fmt.Sprintf("%v", outputs[name])
			fmt.Printf("    %s %-20s %s %s\n",
				green("•"), bold(name), gray("←"), dim(expr))
		}
		fmt.Println()
	}
}

func (p *printer) printStep(seq, total int, step map[string]interface{}, stepIdx map[string]int) {
	stepID := str(step, "stepId")
	opID := str(step, "operationId")
	nestedWfID := str(step, "workflowId")
	opPath := fmt.Sprintf("%v", step["operationPath"])
	if opPath == "<nil>" {
		opPath = ""
	}

	// Determine the operation label
	opLabel := ""
	switch {
	case opID != "":
		opLabel = blue("operationId") + ": " + bold(opID)
	case nestedWfID != "":
		opLabel = cyan("workflowId") + ": " + bold(nestedWfID)
	case opPath != "":
		opLabel = blue("operationPath") + ": " + bold(opPath)
	default:
		opLabel = yellow("⚠ no operation defined")
	}

	// Step header
	marker := "├──"
	if seq == total {
		marker = "└──"
	}
	fmt.Printf("  %s %s[%d] %s%s  %s\n",
		gray(marker), ansiBold, seq, stepID, ansiReset, opLabel)

	indent := "  │   "
	if seq == total {
		indent = "      "
	}

	// Description
	if desc := str(step, "description"); desc != "" {
		fmt.Printf("%s%s\n", indent, dim(truncate(desc, 54)))
	}

	// Parameters
	params := maps(arr(step, "parameters"))
	if len(params) > 0 {
		var pStrs []string
		for _, p := range params {
			pName := str(p, "name")
			pIn := str(p, "in")
			pStrs = append(pStrs, pName+" "+gray("("+pIn+")"))
		}
		fmt.Printf("%s%s %s\n", indent, dim("params:"), strings.Join(pStrs, dim(", ")))
	}

	// Request body
	if rb := obj(step, "requestBody"); rb != nil {
		ct := str(rb, "contentType")
		if ct == "" {
			ct = "?"
		}
		fmt.Printf("%s%s %s\n", indent, dim("body:"), dim(ct))
	}

	// successCriteria
	criteria := maps(arr(step, "successCriteria"))
	if len(criteria) > 0 {
		var cStrs []string
		for _, c := range criteria {
			cStrs = append(cStrs, str(c, "condition"))
		}
		fmt.Printf("%s%s %s\n", indent,
			green("✓ success criteria:"), dim(strings.Join(cStrs, " AND ")))
	}

	// onSuccess
	onSuccess := maps(arr(step, "onSuccess"))
	for _, action := range onSuccess {
		p.printAction(indent, green("✓ on success"), action, stepIdx)
	}

	// onFailure
	onFailure := maps(arr(step, "onFailure"))
	for _, action := range onFailure {
		p.printAction(indent, yellow("✗ on failure"), action, stepIdx)
	}

	// Step outputs
	stepOutputs := obj(step, "outputs")
	if len(stepOutputs) > 0 {
		keys := sortedKeys(stepOutputs)
		var outStrs []string
		for _, k := range keys {
			outStrs = append(outStrs, bold(k))
		}
		fmt.Printf("%s%s %s\n", indent, dim("outputs:"), strings.Join(outStrs, dim(", ")))
	}

	fmt.Println(indent)
}

func (p *printer) printAction(indent string, label string, action map[string]interface{}, stepIdx map[string]int) {
	aType := str(action, "type")
	aName := str(action, "name")
	gotoStep := str(action, "stepId")
	gotoWf := str(action, "workflowId")

	switch aType {
	case "end":
		namePart := ""
		if aName != "" {
			namePart = " " + gray("["+aName+"]")
		}
		fmt.Printf("%s%s → %s%s\n", indent, label, bold("END"), namePart)

	case "goto":
		target := ""
		if gotoStep != "" {
			num := stepIdx[gotoStep]
			if num > 0 {
				target = fmt.Sprintf("GOTO [%d] %s", num, bold(gotoStep))
			} else {
				target = "GOTO " + bold(gotoStep)
			}
		} else if gotoWf != "" {
			target = "GOTO WORKFLOW " + bold(gotoWf)
		}
		// criteria on this action
		crit := maps(arr(action, "criteria"))
		critStr := ""
		if len(crit) > 0 {
			var cs []string
			for _, c := range crit {
				cs = append(cs, str(c, "condition"))
			}
			critStr = "  " + gray("when: "+strings.Join(cs, " AND "))
		}
		namePart := ""
		if aName != "" {
			namePart = " " + gray("["+aName+"]")
		}
		fmt.Printf("%s%s → %s%s%s\n", indent, label, target, namePart, critStr)

	case "retry":
		retryAfter := fmt.Sprintf("%v", action["retryAfter"])
		retryLimit := fmt.Sprintf("%v", action["retryLimit"])
		extra := ""
		if retryAfter != "<nil>" {
			extra += " after=" + retryAfter
		}
		if retryLimit != "<nil>" {
			extra += " limit=" + retryLimit
		}
		namePart := ""
		if aName != "" {
			namePart = " " + gray("["+aName+"]")
		}
		fmt.Printf("%s%s → %s%s%s\n", indent, label, bold("RETRY"), namePart, dim(extra))

	default:
		fmt.Printf("%s%s → %s %s\n", indent, label, bold(aType), gray("["+aName+"]"))
	}
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

func printKV(key, value string) {
	if value == "" {
		return
	}
	fmt.Printf("  %s%-14s%s %s\n", ansiBold, key+":", ansiReset, value)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

func wrapText(s string, width int) []string {
	s = strings.ReplaceAll(s, "\n", " ")
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if len(cur)+1+len(w) <= width {
			cur += " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
