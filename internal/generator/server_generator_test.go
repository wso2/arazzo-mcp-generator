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
	"strings"
	"testing"
)

func TestToPythonFuncName(t *testing.T) {
	tests := []struct {
		name       string
		workflowID string
		want       string
	}{
		{"kebab case", "place-order", "place_order"},
		{"camel case", "placeOrder", "place_order"},
		{"pascal case", "PlaceOrder", "place_order"},
		{"already snake", "place_order", "place_order"},
		{"spaces", "place order", "place_order"},
		{"dots", "pet.upsert.flow", "pet_upsert_flow"},
		{"mixed separators", "placeOrder-v2", "place_order_v2"},
		{"repeated separators", "place--order", "place_order"},
		{"leading and trailing separators", "-place-order-", "place_order"},
		{"leading digit", "1st-workflow", "workflow_1st_workflow"},
		{"python keyword", "import", "workflow_import"},
		{"punctuation only", "---", "workflow"},
		{"empty", "", "workflow"},
		{"acronym", "getHTTPResponse", "get_http_response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toPythonFuncName(tt.workflowID); got != tt.want {
				t.Errorf("toPythonFuncName(%q) = %q, want %q", tt.workflowID, got, tt.want)
			}
		})
	}
}

// TestToPythonFuncNameProducesValidIdentifiers guards the property that actually
// matters: whatever comes out must be usable as a Python function name.
func TestToPythonFuncNameProducesValidIdentifiers(t *testing.T) {
	workflowIDs := []string{
		"place-order", "place order", "1st", "import", "", "---",
		"a.b/c:d", "UPPER-CASE", "trailing-", "-leading", "with(parens)",
	}

	for _, id := range workflowIDs {
		got := toPythonFuncName(id)
		if got == "" {
			t.Errorf("toPythonFuncName(%q) returned an empty name", id)
			continue
		}
		if pythonKeywords[got] {
			t.Errorf("toPythonFuncName(%q) = %q, which is a Python keyword", id, got)
		}
		for i, r := range got {
			isValid := r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
			if !isValid {
				t.Errorf("toPythonFuncName(%q) = %q, invalid rune %q at %d", id, got, r, i)
			}
			if i == 0 && r >= '0' && r <= '9' {
				t.Errorf("toPythonFuncName(%q) = %q, starts with a digit", id, got)
			}
		}
	}
}

// TestBuildToolsBlockDeduplicatesFuncNames covers two workflowIds that normalize
// to the same identifier — without the suffix the second def would shadow the
// first and one tool would silently disappear.
func TestBuildToolsBlockDeduplicatesFuncNames(t *testing.T) {
	spec := &ArazzoSpec{
		Workflows: []Workflow{
			{WorkflowID: "place-order"},
			{WorkflowID: "placeOrder"},
			{WorkflowID: "place_order"},
		},
	}
	classified := map[string]ClassifiedInputs{
		"place-order": {}, "placeOrder": {}, "place_order": {},
	}

	got := buildToolsBlock(spec, classified)

	for _, want := range []string{
		"async def place_order(",
		"async def place_order_2(",
		"async def place_order_3(",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated tools block missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestToPythonParamName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"kebab case", "Internal-Key", "internalKey"},
		{"snake case", "api_key", "apiKey"},
		{"spaces", "user name", "userName"},
		{"plain", "name", "name"},
		{"empty", "", "param"},
		{"punctuation only", "---", "param"},
		{"leading digit", "2fa-code", "param_2faCode"},
		{"digits only", "123", "param_123"},
		{"python keyword", "class", "param_class"},
		{"python keyword after camelizing", "Import", "param_import"},
		{"keyword prefix is fine", "classId", "classId"},
		{"digit not leading is fine", "code2fa", "code2fa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toPythonParamName(tt.input); got != tt.want {
				t.Errorf("toPythonParamName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestToPythonParamNameProducesValidIdentifiers guards the property that
// matters: every result must be usable as a Python parameter name.
func TestToPythonParamNameProducesValidIdentifiers(t *testing.T) {
	inputs := []string{
		"2fa-code", "123", "class", "def", "return", "import", "", "---",
		"Internal-Key", "user name", "a.b/c:d", "9lives", "for",
	}

	for _, in := range inputs {
		got := toPythonParamName(in)
		if got == "" {
			t.Errorf("toPythonParamName(%q) returned an empty name", in)
			continue
		}
		if pythonKeywords[got] {
			t.Errorf("toPythonParamName(%q) = %q, which is a Python keyword", in, got)
		}
		for i, r := range got {
			isValid := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
			if !isValid {
				t.Errorf("toPythonParamName(%q) = %q, invalid rune %q at %d", in, got, r, i)
			}
			if i == 0 && r >= '0' && r <= '9' {
				t.Errorf("toPythonParamName(%q) = %q, starts with a digit", in, got)
			}
		}
	}
}

// TestGenerateServerCodeLeadingDigitAndKeyword reproduces issue #9: inputs
// whose names start with a digit or are Python keywords must not be emitted
// as raw parameter names, and the original Arazzo names must still be passed
// through to execute_workflow.
func TestGenerateServerCodeLeadingDigitAndKeyword(t *testing.T) {
	spec := &ArazzoSpec{
		Arazzo: "1.0.1",
		Info:   ArazzoInfo{Title: "Test", Version: "1.0.0"},
		Workflows: []Workflow{
			{
				WorkflowID: "do-auth",
				Inputs: &WorkflowInput{
					Type: "object",
					Properties: map[string]InputProperty{
						"2fa-code": {Type: "string"},
						"class":    {Type: "string"},
						"name":     {Type: "string"},
					},
				},
			},
		},
	}

	code, err := GenerateServerCode(spec, "test.arazzo.yaml", 8080)
	if err != nil {
		t.Fatalf("GenerateServerCode returned error: %v", err)
	}

	// Parameters appear as "(x: str" or ", x: str" in the def signature.
	for _, bad := range []string{"(2faCode:", ", 2faCode:", "(class:", ", class:"} {
		if strings.Contains(code, bad) {
			t.Errorf("generated code still contains invalid parameter %q", bad)
		}
	}
	for _, want := range []string{
		"param_2faCode: str",
		"param_class: str",
		"name: str",
		`"2fa-code": param_2faCode`,
		`"class": param_class`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code missing %q", want)
		}
	}
}
