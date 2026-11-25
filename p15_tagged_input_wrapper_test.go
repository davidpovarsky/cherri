package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// A conditional input wrapper and a named native variable descriptor share the
// "Variable" type tag but have different, exact field shapes. The observer must
// follow the wrapper's actual nested reference without inventing a slot name.
func TestP15TaggedConditionalVariableWrappers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		private bool
		reject  bool
	}{
		{"map_ordinary", false, false}, {"map_private", true, false},
		{"typed_ordinary", false, false}, {"typed_private", true, false},
		{"missing_name", false, true}, {"empty_name", false, true},
		{"wrong_name_kind", false, true}, {"extra_wrapper_field", false, true},
		{"typed_missing_reference", false, true}, {"typed_wrong_name_kind", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &CanonicalBackendSession{}
			emit := func(kind string, parameters map[string]any, id string) {
				t.Helper()
				if err := s.EmitRawAction("is.workflow.actions."+kind, parameters, id, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			emit("dictionary", map[string]any{"WFItems": map[string]any{"owned": "private"}}, "private")
			emit("gettext", map[string]any{"WFTextActionText": "ordinary"}, "ordinary")
			ref := any(p15FunctionEdge("ordinary"))
			if tc.private {
				ref = p15FunctionEdge("private")
			}
			var wrapper any = map[string]any{"Type": "Variable", "Variable": ref}
			switch tc.name {
			case "typed_ordinary", "typed_private":
				wrapper = struct {
					Type     string
					Variable any
				}{"Variable", ref}
			case "missing_name":
				wrapper = map[string]any{"Type": "Variable"}
			case "empty_name":
				wrapper = map[string]any{"Type": "Variable", "VariableName": ""}
			case "wrong_name_kind":
				wrapper = map[string]any{"Type": "Variable", "VariableName": 17}
			case "extra_wrapper_field":
				wrapper = map[string]any{"Type": "Variable", "Variable": ref, "unmodeled": "field"}
			case "typed_missing_reference":
				wrapper = struct{ Type string }{"Variable"}
			case "typed_wrong_name_kind":
				wrapper = struct {
					Type         string
					VariableName int
				}{"Variable", 17}
			}
			emit("conditional", map[string]any{
				"GroupingIdentifier": "wrapper-test", "WFControlFlowMode": 0, "WFInput": wrapper,
			}, "")
			before, err := json.Marshal(s.shortcut)
			if err != nil {
				t.Fatal(err)
			}
			private, failure := s.CurrentNativeInputUsesPrivateValue(map[string]bool{"private": true}, nil, p15TaggedFixtureValueOutput)
			after, err := json.Marshal(s.shortcut)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("wrapper observation mutated canonical actions", err)
			}
			if private != tc.private || (failure != nil) != tc.reject {
				t.Fatalf("private=%t err=%v, want private=%t reject=%t", private, failure, tc.private, tc.reject)
			}
			if failure != nil && !strings.Contains(failure.Error(), "E_TARGET_VALUE_SHAPE") {
				t.Fatal("malformed variable lost exact boundary diagnostic", failure)
			}
		})
	}
}
