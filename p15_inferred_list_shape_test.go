package main

import (
	"strings"
	"testing"
)

func TestP15L01InferredNestedNativeListFailsClosed(t *testing.T) {
	var cases = []struct{ name, source string }{
		{"immutable_nonempty", "let inner=[1,2]\nlet outer=[inner]\ngetVariable(outer)"},
		{"immutable_empty", "let inner=[]\nlet outer=[inner]\ngetVariable(outer)"},
		{"mutable", "var inner=[1,2]\nlet outer=[inner]\ngetVariable(outer)"},
		{"alias", "let inner=[1,2]\nlet alias=inner\nlet outer=[alias]\ngetVariable(outer)"},
		{"typed_binding", "let inner: List<Number> = [1,2]\nlet outer=[inner]\ngetVariable(outer)"},
		{"optional_present", "let inner: List<Number>? = [1,2]\nlet outer=[inner]\ngetVariable(outer)"},
		{"optional_absent", "let inner: List<Number>? = none\nlet outer=[inner]\ngetVariable(outer)"},
		{"function_result", "function rows()->List<Number>{return [1,2]}\nlet outer=[rows()]\ngetVariable(outer)"},
		{"function_parameter", "function wrap(items: List<Number>)->List<List<Number>>{return [items]}\ngetVariable(wrap([1,2]))"},
		{"conditional_result", "let inner=if true {yield [1,2]} else {yield []}\nlet outer=[inner]\ngetVariable(outer)"},
		{"loop_result", "let inner=for item in [1,2] {yield item}\nlet outer=[inner]\ngetVariable(outer)"},
		{"widened_known_list", "let inner: AnyContent = [1,2]\nlet outer=[inner]\ngetVariable(outer)"},
		{"union_list", "let inner: List<Number> | Text = [1,2]\nlet outer=[inner]\ngetVariable(outer)"},
		{"type_alias", "type Row = List<Number>\nlet inner: Row=[1,2]\nlet outer=[inner]\ngetVariable(outer)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, errors := p15ParseProgram(tc.source); len(errors) != 0 {
				t.Fatalf("shape regression must be parser-valid: %v", errors)
			}
			expected := p15TaggedBoundaryCase{tc.name, tc.source, "outer",
				"E_TARGET_VALUE_SHAPE", "getVariable requires an explicit native/JSON conversion for structured values"}
			if tc.name == "function_parameter" {
				expected.target = "wrap([1,2])"
			}
			if tc.name == "widened_known_list" {
				expected.target, expected.code = "inner", "E_NESTED_NATIVE_LIST"
				expected.message = "nested List-valued element has no verified native content-row encoding"
			}
			p15CheckTaggedBoundary(t, expected)
			expectedCode, expectedDetail := expected.code, expected.message
			var data, err = CompileSourceToPlist("p15-inferred-nested-list.cherri", tc.source)
			if err == nil || len(data) != 0 || !strings.Contains(err.Error(), expectedCode) || !strings.Contains(err.Error(), expectedDetail) {
				t.Fatalf("inferred nested shape bypassed native List boundary: bytes=%d error=%v", len(data), err)
			}
			var mobile = compileForMobileMode(tc.source, "Inferred nested List", "v2", false)
			if mobile.OK || mobile.PlistBase64 != "" || mobile.SignedBase64 != "" || !strings.Contains(mobile.Error, expectedCode) || !strings.Contains(mobile.Error, expectedDetail) {
				t.Fatalf("mobile route bypassed List shape boundary: %+v", mobile)
			}
		})
	}
	t.Run("scalar_and_dictionary_reference_controls", func(t *testing.T) {
		var actions = p15CompiledActions(t, "let textValue=text(\"kept\")\nlet numberValue=number(0)\nlet dictionaryValue={\"rows\":[[],[1,2]]}\nlet outer=[textValue,numberValue,dictionaryValue]\ngetVariable(outer)")
		var lists int
		for _, action := range actions {
			if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" {
				lists++
				var rows, ok = p15Param(action)["WFItems"].([]any)
				if !ok || len(rows) != 3 {
					t.Fatalf("supported native List reference cardinality changed: %#v", p15Param(action))
				}
			}
		}
		if lists != 1 {
			t.Fatalf("expected one authored outer List and inline Dictionary arrays, got %d", lists)
		}
	})
}
