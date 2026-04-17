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

// Package visualizer generates Mermaid flowchart diagrams from Arazzo specifications.
// It handles branching (onSuccess/onFailure), goto routing, end actions, retry loops,
// implicit sequential flows, workflow references, and multi-workflow specs.
package visualizer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// ─── YAML helpers ──────────────────────────────────────────────────────────────

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

// Visualize reads an Arazzo spec and generates a Mermaid flowchart.
//   - If outputFile ends in ".md"  → wrap diagram in markdown fences and save.
//   - If outputFile ends in ".mmd" → save raw Mermaid syntax.
//   - If outputFile is empty       → render an HTML page and open it in the
//     system default browser so the diagram is visible immediately.
func Visualize(filePath string, outputFile string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	info := obj(raw, "info")
	title := ""
	if info != nil {
		title = str(info, "title")
	}

	mermaid := generateMermaid(raw)

	if outputFile != "" {
		var content string
		if strings.HasSuffix(strings.ToLower(outputFile), ".md") {
			content = "```mermaid\n" + mermaid + "```\n"
		} else {
			content = mermaid
		}
		dir := filepath.Dir(outputFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Mermaid diagram written to: %s\n", outputFile)
		return nil
	}

	// No output file → render HTML and open in browser
	return openInBrowser(title, mermaid)
}

// openInBrowser writes a self-contained HTML file to a temp directory and
// opens it with the OS default browser. The HTML uses the Mermaid CDN so no
// npm / mermaid-cli install is required.
func openInBrowser(title, mermaid string) error {
	tmpDir, err := os.MkdirTemp("", "arazzo-viz-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	htmlContent := buildHTML(title, mermaid)

	htmlFile := filepath.Join(tmpDir, "diagram.html")
	if err := os.WriteFile(htmlFile, []byte(htmlContent), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	if err := openBrowser(htmlFile); err != nil {
		// Browser open failed — fall back to printing the raw Mermaid + hints
		fmt.Fprintln(os.Stderr, "Could not open browser automatically. Raw Mermaid diagram:")
		fmt.Println(mermaid)
		fmt.Printf("\nDiagram HTML saved at: %s\n", htmlFile)
		fmt.Println("Open it manually in a browser, or paste the Mermaid code at https://mermaid.live")
		return nil
	}

	fmt.Printf("Diagram opened in browser  (HTML: %s)\n", htmlFile)
	fmt.Println("Tip: paste the Mermaid source into https://mermaid.live for a shareable link.")
	return nil
}

// openBrowser opens a local file path in the OS default browser.
func openBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", "start", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default: // linux and *BSDs
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// buildHTML returns a self-contained HTML page that uses the Mermaid JS CDN
// to render the diagram. No internet needed after the browser loads the page
// (CDN fetch happens at open time).
func buildHTML(title, mermaid string) string {
	pageTitle := "Arazzo Workflow Diagram"
	if title != "" {
		pageTitle = title + " — Arazzo Workflow Diagram"
	}

	// Escape backticks/backslashes for the JS template literal
	mermaidJS := strings.ReplaceAll(mermaid, "\\", "\\\\")
	mermaidJS = strings.ReplaceAll(mermaidJS, "`", "\\`")

	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>` + pageTitle + `</title>
  <script src="https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js"></script>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
      background: #f8f9fa;
      color: #212529;
      padding: 24px;
    }
    h1 {
      font-size: 1.25rem;
      font-weight: 600;
      margin-bottom: 20px;
      color: #343a40;
    }
    #diagram-container {
      background: #fff;
      border: 1px solid #dee2e6;
      border-radius: 8px;
      padding: 32px;
      overflow: auto;
      box-shadow: 0 1px 4px rgba(0,0,0,.08);
    }
    .mermaid svg {
      max-width: 100%;
      height: auto;
    }
    footer {
      margin-top: 16px;
      font-size: 0.8rem;
      color: #6c757d;
    }
    a { color: #0d6efd; }
  </style>
</head>
<body>
  <h1>` + pageTitle + `</h1>
  <div id="diagram-container">
    <div class="mermaid">` + "\n" + mermaid + `
    </div>
  </div>
  <footer>
    Generated by <strong>arazzo-mcp-gen</strong> &mdash;
    <a href="https://mermaid.live" target="_blank">Open in Mermaid Live Editor</a>
  </footer>
  <script>
    mermaid.initialize({ startOnLoad: true, theme: 'default', securityLevel: 'loose' });
  </script>
</body>
</html>`
}

// ─── Mermaid generation ────────────────────────────────────────────────────────

func generateMermaid(raw map[string]interface{}) string {
	var b strings.Builder

	info := obj(raw, "info")
	title := ""
	if info != nil {
		title = str(info, "title")
	}

	b.WriteString("flowchart TD\n")
	if title != "" {
		b.WriteString(fmt.Sprintf("    %%%% %s\n", esc(title)))
	}

	workflows := maps(arr(raw, "workflows"))
	for wfIdx, wf := range workflows {
		b.WriteString("\n")
		generateWorkflowSubgraph(&b, wfIdx, wf)
	}

	// Styling classes
	b.WriteString("\n")
	b.WriteString("    classDef startNode fill:#d4edda,stroke:#28a745,stroke-width:2px,color:#155724\n")
	b.WriteString("    classDef stepNode fill:#ffffff,stroke:#495057,stroke-width:1px,color:#212529\n")
	b.WriteString("    classDef endNode fill:#f8d7da,stroke:#dc3545,stroke-width:2px,color:#721c24\n")

	return b.String()
}

func generateWorkflowSubgraph(b *strings.Builder, wfIdx int, wf map[string]interface{}) {
	wfID := str(wf, "workflowId")
	summary := str(wf, "summary")
	prefix := fmt.Sprintf("wf%d", wfIdx)

	// Subgraph label
	subLabel := wfID
	if summary != "" {
		subLabel = wfID + ": " + trunc(summary, 60)
	}
	b.WriteString(fmt.Sprintf("    subgraph %s_sub[\"%s\"]\n", prefix, esc(subLabel)))
	b.WriteString("        direction TB\n")

	steps := maps(arr(wf, "steps"))

	// Build stepId → node-ID map
	stepNodeID := make(map[string]string)
	for i, step := range steps {
		sid := str(step, "stepId")
		stepNodeID[sid] = fmt.Sprintf("%s_s%d", prefix, i)
	}

	// ── Start node ──────────────────────────────────────────────────────────
	startID := prefix + "_start"
	b.WriteString(fmt.Sprintf("        %s([\"▶ Start\"])\n", startID))
	if len(steps) > 0 {
		b.WriteString(fmt.Sprintf("        %s --> %s\n", startID, fmt.Sprintf("%s_s0", prefix)))
	}

	endCounter := 0

	for i, step := range steps {
		nodeID := fmt.Sprintf("%s_s%d", prefix, i)
		generateStepNode(b, prefix, nodeID, i, step, steps, stepNodeID, &endCounter)
	}

	// ── Apply style classes ─────────────────────────────────────────────────
	b.WriteString(fmt.Sprintf("        class %s startNode\n", startID))
	for i := range steps {
		b.WriteString(fmt.Sprintf("        class %s_s%d stepNode\n", prefix, i))
	}
	for i := 0; i < endCounter; i++ {
		b.WriteString(fmt.Sprintf("        class %s_end%d endNode\n", prefix, i))
	}

	b.WriteString("    end\n")
}

// generateStepNode emits the Mermaid node definition, edges for onSuccess/onFailure,
// and implicit sequential or END edges when no explicit routing is defined.
func generateStepNode(
	b *strings.Builder,
	prefix, nodeID string,
	idx int,
	step map[string]interface{},
	allSteps []map[string]interface{},
	stepNodeID map[string]string,
	endCounter *int,
) {
	stepID := str(step, "stepId")
	opID := str(step, "operationId")
	nestedWfID := str(step, "workflowId")
	opPath := fmtVal(step["operationPath"])
	seq := idx + 1

	// ── Build node label ────────────────────────────────────────────────────
	var parts []string
	parts = append(parts, fmt.Sprintf("[%d] %s", seq, stepID))

	switch {
	case opID != "":
		parts = append(parts, "op: "+opID)
	case nestedWfID != "":
		parts = append(parts, "workflow: "+nestedWfID)
	case opPath != "":
		parts = append(parts, "path: "+opPath)
	}

	criteria := maps(arr(step, "successCriteria"))
	if len(criteria) > 0 {
		var conds []string
		for _, c := range criteria {
			conds = append(conds, trunc(str(c, "condition"), 40))
		}
		parts = append(parts, "✓ "+strings.Join(conds, " AND "))
	}

	label := strings.Join(parts, "<br/>")
	b.WriteString(fmt.Sprintf("        %s[\"%s\"]\n", nodeID, esc(label)))

	// ── Edges from onSuccess / onFailure actions ────────────────────────────
	onSuccess := maps(arr(step, "onSuccess"))
	onFailure := maps(arr(step, "onFailure"))
	hasExplicit := len(onSuccess) > 0 || len(onFailure) > 0

	for _, action := range onSuccess {
		writeActionEdge(b, prefix, nodeID, action, "✓", stepNodeID, endCounter)
	}
	for _, action := range onFailure {
		writeActionEdge(b, prefix, nodeID, action, "✗", stepNodeID, endCounter)
	}

	// ── Implicit flow ───────────────────────────────────────────────────────
	if !hasExplicit {
		// No explicit routing → sequential next or implicit END
		if idx < len(allSteps)-1 {
			next := fmt.Sprintf("%s_s%d", prefix, idx+1)
			b.WriteString(fmt.Sprintf("        %s --> %s\n", nodeID, next))
		} else {
			endID := fmt.Sprintf("%s_end%d", prefix, *endCounter)
			*endCounter++
			b.WriteString(fmt.Sprintf("        %s([\"⏹ END\"])\n", endID))
			b.WriteString(fmt.Sprintf("        %s --> %s\n", nodeID, endID))
		}
	} else {
		// If ALL onSuccess actions have criteria and none already target the next
		// step, there is an implicit fallthrough to the next sequential step (Arazzo
		// spec: unmatched criteria → continue sequentially).
		if idx < len(allSteps)-1 && len(onSuccess) > 0 {
			allHaveCriteria := true
			nextStepID := str(allSteps[idx+1], "stepId")
			alreadyTargetsNext := false

			for _, action := range onSuccess {
				if len(maps(arr(action, "criteria"))) == 0 {
					allHaveCriteria = false
					break
				}
				if str(action, "type") == "goto" && str(action, "stepId") == nextStepID {
					alreadyTargetsNext = true
				}
			}

			if allHaveCriteria && !alreadyTargetsNext {
				next := fmt.Sprintf("%s_s%d", prefix, idx+1)
				b.WriteString(fmt.Sprintf("        %s -.->|\"(no match)\"| %s\n", nodeID, next))
			}
		}

		// If step is last AND has no END actions among its routes, add implicit END
		if idx == len(allSteps)-1 {
			hasEnd := false
			for _, a := range onSuccess {
				if str(a, "type") == "end" {
					hasEnd = true
				}
			}
			for _, a := range onFailure {
				if str(a, "type") == "end" {
					hasEnd = true
				}
			}
			if !hasEnd {
				endID := fmt.Sprintf("%s_end%d", prefix, *endCounter)
				*endCounter++
				b.WriteString(fmt.Sprintf("        %s([\"⏹ END\"])\n", endID))
				b.WriteString(fmt.Sprintf("        %s --> %s\n", nodeID, endID))
			}
		}
	}
}

// writeActionEdge writes a Mermaid edge for one onSuccess/onFailure action.
func writeActionEdge(
	b *strings.Builder,
	prefix, sourceID string,
	action map[string]interface{},
	symbol string, // "✓" or "✗"
	stepNodeID map[string]string,
	endCounter *int,
) {
	aType := str(action, "type")
	aName := str(action, "name")
	gotoStep := str(action, "stepId")
	gotoWf := str(action, "workflowId")

	// ── Build edge label ────────────────────────────────────────────────────
	labelHead := symbol
	if aName != "" {
		labelHead += " " + aName
	}

	actionCriteria := maps(arr(action, "criteria"))
	criteriaStr := ""
	if len(actionCriteria) > 0 {
		var cs []string
		for _, c := range actionCriteria {
			cond := str(c, "condition")
			if cond != "" {
				cs = append(cs, trunc(cond, 40))
			}
		}
		if len(cs) > 0 {
			criteriaStr = "<br/>when: " + strings.Join(cs, " AND ")
		}
	}

	edgeLabel := labelHead + criteriaStr

	switch aType {
	case "goto":
		if gotoStep != "" {
			targetID, found := stepNodeID[gotoStep]
			if found {
				b.WriteString(fmt.Sprintf("        %s -->|\"%s\"| %s\n",
					sourceID, esc(edgeLabel), targetID))
			} else {
				// Reference to a step not in this workflow (rare)
				extID := prefix + "_ext_" + sanitizeID(gotoStep)
				b.WriteString(fmt.Sprintf("        %s[\"%s\"]\n", extID, esc("→ "+gotoStep)))
				b.WriteString(fmt.Sprintf("        %s -->|\"%s\"| %s\n",
					sourceID, esc(edgeLabel), extID))
			}
		} else if gotoWf != "" {
			extID := prefix + "_wfref_" + sanitizeID(gotoWf)
			b.WriteString(fmt.Sprintf("        %s[[\"%s\"]]\n",
				extID, esc("↗ Workflow: "+gotoWf)))
			b.WriteString(fmt.Sprintf("        %s -->|\"%s\"| %s\n",
				sourceID, esc(edgeLabel), extID))
		}

	case "end":
		endID := fmt.Sprintf("%s_end%d", prefix, *endCounter)
		*endCounter++
		b.WriteString(fmt.Sprintf("        %s([\"⏹ END\"])\n", endID))
		b.WriteString(fmt.Sprintf("        %s -->|\"%s\"| %s\n",
			sourceID, esc(edgeLabel), endID))

	case "retry":
		retryAfter := fmtVal(action["retryAfter"])
		retryLimit := fmtVal(action["retryLimit"])
		extras := ""
		if retryAfter != "" {
			extras += " after=" + retryAfter
		}
		if retryLimit != "" {
			extras += " limit=" + retryLimit
		}
		retryLabel := edgeLabel
		if extras != "" {
			retryLabel += "<br/>" + strings.TrimSpace(extras)
		}
		// Retry loops back to itself
		b.WriteString(fmt.Sprintf("        %s -->|\"%s\"| %s\n",
			sourceID, esc(retryLabel), sourceID))

	default:
		// Unknown / future action types
		b.WriteString(fmt.Sprintf("        %s -->|\"%s %s\"| %s\n",
			sourceID, esc(symbol+" "+aType), esc(aName), sourceID))
	}
}

// ─── String helpers ────────────────────────────────────────────────────────────

// esc escapes characters that are problematic inside Mermaid quoted strings.
func esc(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "#", "＃") // fullwidth # avoids Mermaid comment issue
	return s
}

// sanitizeID produces a Mermaid-safe node identifier from an arbitrary string.
func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// trunc shortens s to at most n characters, replacing the tail with "…".
func trunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

// fmtVal converts an interface{} to a non-"<nil>" string, or returns "".
func fmtVal(v interface{}) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" {
		return ""
	}
	return s
}
