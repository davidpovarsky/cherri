package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

func TestP15TaggedCanonicalPreviousObserver(t *testing.T) {
	cases := []struct {
		name                   string
		wantPrivate, wantError bool
	}{
		{"enrolled_output", true, false},
		{"unowned_marker", false, false},
		{"ordinary_alias", true, false},
		{"typed_alias", true, false},
		{"typed_variable", true, false},
		{"typed_dictionary_field", true, false},
		{"typed_alias_literal_carrier", true, false},
		{"backend_loop_result", true, false},
		{"private_handle_without_wire_uuid", true, false},
		{"literal_carrier_cursor", true, false},
		{"ordinary_after_private", false, false},
		{"nothing_after_private", false, false},
		{"mutable_slot", true, false},
		{"closed_private_control", true, false},
		{"unrelated_closed_control", false, false},
		{"duplicate_uuid", false, true},
		{"cyclic_alias", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &CanonicalBackendSession{}
			outputs := map[string]bool{"cell": true}
			variables := map[string]bool{}
			emit := func(id string, p map[string]any, uuid string) {
				t.Helper()
				if err := s.EmitRawAction("is.workflow.actions."+id, p, uuid, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			marker := map[string]any{"WFItems": map[string]any{"_cherri_value_cell_v1": "1", "kind": "null", "has_value": "1"}}
			switch tc.name {
			case "unowned_marker":
				emit("dictionary", marker, "authored")
			case "private_handle_without_wire_uuid":
				emit("dictionary", marker, "")
				if err := s.BindNativeActionResult("owned-handle", "unused-default", false, false, ""); err != nil {
					t.Fatal(err)
				}
				outputs = map[string]bool{"owned-handle": true}
			case "closed_private_control":
				emit("conditional", map[string]any{"GroupingIdentifier": "branch", "WFControlFlowMode": 0}, "")
				emit("dictionary", marker, "cell")
				emit("conditional", map[string]any{"GroupingIdentifier": "branch", "WFControlFlowMode": 1}, "")
				emit("gettext", map[string]any{"WFTextActionText": "ordinary"}, "ordinary")
				emit("conditional", map[string]any{"GroupingIdentifier": "branch", "WFControlFlowMode": 2}, "")
			case "unrelated_closed_control":
				emit("dictionary", marker, "cell")
				emit("conditional", map[string]any{"GroupingIdentifier": "branch", "WFControlFlowMode": 0}, "")
				emit("gettext", map[string]any{"WFTextActionText": "ordinary"}, "ordinary")
				emit("conditional", map[string]any{"GroupingIdentifier": "branch", "WFControlFlowMode": 2}, "")
			default:
				emit("dictionary", marker, "cell")
				switch tc.name {
				case "ordinary_alias":
					emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge("cell")}, "alias")
				case "typed_alias":
					emit("getvariable", map[string]any{"WFVariable": debugOutputRef("cell")}, "alias")
				case "typed_variable":
					variables["owned"] = true
					emit("getvariable", map[string]any{"WFVariable": debugLogValueRef("owned")}, "alias")
				case "typed_dictionary_field":
					emit("dictionary", map[string]any{"WFItems": []WFDictionaryFieldValueItem{{WFKey: "data", WFItemType: 0, WFValue: debugOutputRef("cell")}}}, "container")
				case "backend_loop_result":
					emit("getvariable", map[string]any{"WFVariable": backend.Reference{Kind: backend.RefLoopResult, ProducerID: "cell"}}, "alias")
				case "literal_carrier_cursor", "typed_alias_literal_carrier":
					if tc.name == "typed_alias_literal_carrier" {
						emit("getvariable", map[string]any{"WFVariable": debugOutputRef("cell")}, "alias")
					}
					emit("gettext", map[string]any{"WFTextActionText": "~LEFT"}, "snapshot")
					if err := s.BindExactTextSnapshotInput("snapshot", "~LEFT", "preserved-handle", "unused", func(id string) bool {
						return id == "is.workflow.actions.dictionary" || id == "is.workflow.actions.getvariable"
					}); err != nil {
						t.Fatal(err)
					}
				case "ordinary_after_private":
					emit("gettext", map[string]any{"WFTextActionText": "ordinary"}, "ordinary")
				case "nothing_after_private":
					emit("nothing", map[string]any{}, "")
				case "mutable_slot":
					variables["owned"] = true
					emit("setvariable", map[string]any{"WFVariableName": "owned", "WFInput": p15FunctionEdge("cell")}, "")
				case "duplicate_uuid":
					emit("gettext", map[string]any{"WFTextActionText": "collision"}, "cell")
				case "cyclic_alias":
					emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge("cycle")}, "cycle")
				}
			}
			before, err := json.Marshal(s.shortcut)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.CurrentNativeInputUsesPrivateValue(outputs, variables)
			after, marshalErr := json.Marshal(s.shortcut)
			if marshalErr != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("observer mutated canonical native bytes", marshalErr)
			}
			if (err != nil) != tc.wantError || got != tc.wantPrivate {
				t.Fatalf("private=%t error=%v; want private=%t error=%t", got, err, tc.wantPrivate, tc.wantError)
			}
			if tc.name == "private_handle_without_wire_uuid" {
				if _, added := s.shortcut.WFWorkflowActions[0].WFWorkflowActionParameters["UUID"]; added {
					t.Fatal("observer consumed a lazy result handle")
				}
			}
		})
	}
}

func TestP15TaggedPreviousObservesResolvedOutput(t *testing.T) {
	for name, source := range map[string]string{
		"resolved_text":    "let private=null\ntext(\"safe\")\nnative.action(\"is.workflow.actions.detect.text\",{})\nreturn 1",
		"resolved_count":   "let private=[none,false]\ncount(private)\nnative.action(\"is.workflow.actions.detect.text\",{})\nreturn 1",
		"resolved_nothing": "let private=null\nnothing()\nnative.action(\"is.workflow.actions.detect.text\",{})\nreturn 1",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("p15-tagged-resolved-"+name+".cherri", source)
			if err != nil || len(raw) == 0 {
				t.Fatalf("actual later canonical output was confused with stale IR: %v", err)
			}
		})
	}
	t.Run("actual_resolved_macro", func(t *testing.T) {
		session := NewCanonicalBackendSession()
		defer session.Close()
		if err := session.EmitRawAction("is.workflow.actions.dictionary", map[string]any{"WFItems": map[string]any{"k": "v"}}, "private-root", "", ""); err != nil {
			t.Fatal(err)
		}
		_, err := session.EmitResolvedCall(backend.ResolvedCall{
			DefinitionID: "debugStopTimer", AppleIdentifier: "is.workflow.actions.gettimebetweendates",
			Arguments: []backend.CallArgument{{ParameterID: "WFDebugTimerLabel", Position: 0, Value: backend.SemanticValue{Kind: backend.ValString, StrVal: "ordinary"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		got, err := session.CurrentNativeInputUsesPrivateValue(map[string]bool{"private-root": true}, nil)
		if err != nil || got {
			t.Fatalf("canonical macro output remained private: %t %v", got, err)
		}
	})
}

func TestP15TaggedClosedNativeBoundaryPaths(t *testing.T) {
	declaration := "action detect()->Text {identifier:\"is.workflow.actions.detect.text\"}\n"
	for name, source := range map[string]string{
		"private_alias":               "let value=null\nlet alias=value\nnative.action(\"is.workflow.actions.detect.text\",{})\nreturn 1",
		"declared_implicit":           declaration + "let value=null\ndetect()\nreturn 1",
		"dynamic_private_in_map":      "let nested=[[1]]\nlet mapped={\"slot\":nested}\nreturn 1",
		"implicit_debug_log":          "let value=null\ndebugLog()\nreturn 1",
		"implicit_debug_log_previous": "let value=null\ndebugLogPrevious()\nreturn 1",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("p15-tagged-closed-"+name+".cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(fmt.Sprint(err), "E_TARGET_VALUE_SHAPE") {
				t.Fatalf("private data escaped through %s: bytes=%d error=%v", name, len(raw), err)
			}
		})
	}
}
