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
		literal bool
	}{
		{"map_ordinary", false, false, false}, {"map_private", true, false, false},
		{"typed_ordinary", false, false, false}, {"typed_private", true, false, false},
		{"missing_name", false, true, false}, {"empty_name", false, true, false},
		{"wrong_name_kind", false, true, false}, {"extra_wrapper_field", false, true, false},
		{"typed_missing_reference", false, true, false}, {"typed_wrong_name_kind", false, true, false},
		{"dictionary_missing_name", false, false, true}, {"dictionary_empty_name", false, false, true},
		{"dictionary_wrong_name_kind", false, false, true}, {"dictionary_extra_wrapper_field", false, false, true},
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
			switch strings.TrimPrefix(tc.name, "dictionary_") {
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
			parameters := map[string]any{
				"GroupingIdentifier": "wrapper-test", "WFControlFlowMode": 0, "WFInput": wrapper,
			}
			if tc.literal {
				// Authored lookalike maps are literal Dictionaries, not native
				// descriptors. Preserve this observed behavior as a positive control.
				emit("conditional", parameters, "")
				input := s.shortcut.WFWorkflowActions[len(s.shortcut.WFWorkflowActions)-1].WFWorkflowActionParameters["WFInput"]
				if _, okay := input.(WFDictionaryFieldValue); !okay {
					t.Fatalf("lookalike did not retain its native Dictionary literal: %T", input)
				}
			} else {
				// Exact imported/native parameters must reach the observer unchanged.
				// EmitRawAction would encode malformed maps as Dictionary literals.
				if err := s.EmitExactRawAction("is.workflow.actions.conditional", parameters, true); err != nil {
					t.Fatal(err)
				}
				input := s.shortcut.WFWorkflowActions[len(s.shortcut.WFWorkflowActions)-1].WFWorkflowActionParameters["WFInput"]
				if !reflect.DeepEqual(input, wrapper) {
					t.Fatal("exact native wrapper changed before observation")
				}
			}
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
