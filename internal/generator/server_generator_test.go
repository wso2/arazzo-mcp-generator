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
