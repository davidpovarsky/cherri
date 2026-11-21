package main

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"
)

var p15MacroOutputUUID = regexp.MustCompile("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$")

// The runtime fixtures explicitly output finalSummary. Their caller selects
// that contract; ordinary measure/fallthrough tests keep the M02 Void oracle.
func p15MacroExplicitOutputActions(t *testing.T, source string, profile BuildProfile) []ShortcutAction {
	t.Helper()
	raw, err := CompileSourceToPlistWithOptions("macro-runtime-fixture.cherri", source, CompileOptions{BuildProfile: profile})
	if err != nil {
		t.Fatal(err)
	}
	document, err := decodeShortcutRawMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows, ok := document["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("missing complete native actions")
	}
	var native []map[string]any
	for _, row := range rows {
		action, ok := row.(map[string]any)
		if !ok {
			t.Fatal("non-dictionary native action")
		}
		native = append(native, action)
	}
	actions := p15MacroRetainNativeRows(t, native)
	if err := p15MacroExplicitOutputError(actions); err != nil {
		t.Fatal(err)
	}
	return actions
}

func p15MacroRetainNativeRows(t *testing.T, native []map[string]any) []ShortcutAction {
	t.Helper()
	var actions []ShortcutAction
	for _, action := range native {
		identifier, ok := action["WFWorkflowActionIdentifier"].(string)
		if !ok {
			t.Fatal("missing native action identifier")
		}
		params, _ := action["WFWorkflowActionParameters"].(map[string]any)
		actions = append(actions, ShortcutAction{rawNativeEnvelope: action, WFWorkflowActionIdentifier: identifier, WFWorkflowActionParameters: params})
	}
	return actions
}

func p15MacroExplicitOutputError(actions []ShortcutAction) error {
	if len(actions) < 2 {
		return fmt.Errorf("explicit macro output lost producer or consumer")
	}
	positions := map[string]int{}
	summary := -1
	outputs := 0
	for i, action := range actions {
		params := action.WFWorkflowActionParameters
		if uuid, exists := params["UUID"]; exists {
			value, ok := uuid.(string)
			if !ok || !p15MacroOutputUUID.MatchString(value) || value == "00000000-0000-0000-0000-000000000000" {
				return fmt.Errorf("invalid native producer identity")
			}
			if _, duplicate := positions[value]; duplicate {
				return fmt.Errorf("duplicate native producer identity")
			}
			positions[value] = i
		}
		if params["CustomOutputName"] == "finalSummary" {
			if summary >= 0 || action.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
				return fmt.Errorf("final summary is not one Text producer")
			}
			summary = i
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.output" {
			outputs++
		}
	}
	if summary < 0 || summary >= len(actions)-1 || outputs != 1 {
		return fmt.Errorf("explicit macro output has no unique earlier summary or sole output")
	}
	uuid, ok := actions[summary].WFWorkflowActionParameters["UUID"].(string)
	if !ok || uuid == "" {
		return fmt.Errorf("final summary lost producer UUID")
	}
	expected := map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.output",
		"WFWorkflowActionParameters": map[string]any{"WFOutput": map[string]any{
			"WFSerializationType": "WFTextTokenString",
			"Value": map[string]any{"string": "\ufffc", "attachmentsByRange": map[string]any{
				"{0, 1}": map[string]any{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": "finalSummary"},
			}},
		}},
	}
	if !reflect.DeepEqual(actions[len(actions)-1].rawNativeEnvelope, expected) {
		return fmt.Errorf("explicit macro output changed exact summary reference or terminal envelope")
	}
	for i, action := range actions {
		refs, _ := p15V02NativeReferences(action.WFWorkflowActionParameters)
		for _, ref := range refs {
			producer, found := positions[ref]
			if !found || producer >= i {
				return fmt.Errorf("native consumer precedes or loses producer")
			}
		}
	}
	return nil
}

func TestP15MacroExplicitOutputBoundaryControls(t *testing.T) {
	cases := []struct {
		name    string
		profile BuildProfile
	}{
		{"qa-require-v2", BuildProfileDebug},
		{"qa-measure-v2", BuildProfileDebug},
		{"qa-measure-release-v2", BuildProfileDebug},
		{"qa-measure-release-v2", BuildProfileRelease},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/"+string(tc.profile), func(t *testing.T) {
			original := p15MacroExplicitOutputActions(t, p15ReadMacroRuntimeFixture(t, tc.name), tc.profile)
			if p15M02MainVoidDebugError(original) == nil {
				t.Fatal("explicit-output fixture was accepted as implicit Void")
			}
			native := make([]map[string]any, len(original))
			for i, action := range original {
				native[i] = action.rawNativeEnvelope
			}
			for _, fault := range []string{"missing_output", "extra_nothing", "missing_payload", "literal_payload", "changed_span", "wrong_summary_name", "wrong_summary_kind", "detached_uuid", "duplicate_uuid", "duplicate_output", "extra_output_field"} {
				t.Run(fault, func(t *testing.T) {
					rows := p15ContainerCloneNative(t, native)
					summary := -1
					for i, row := range rows {
						if p15C01Params(row)["CustomOutputName"] == "finalSummary" {
							summary = i
						}
					}
					if summary < 0 {
						t.Fatal("control missed actual summary")
					}
					last := rows[len(rows)-1]
					params := p15C01Params(last)
					token := params["WFOutput"].(map[string]any)["Value"].(map[string]any)
					attachment := token["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)
					switch fault {
					case "missing_output":
						rows = rows[:len(rows)-1]
					case "extra_nothing":
						rows = append(rows, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					case "missing_payload":
						delete(params, "WFOutput")
					case "literal_payload":
						params["WFOutput"] = "not the computed summary"
					case "changed_span":
						ranges := token["attachmentsByRange"].(map[string]any)
						ranges["{1, 1}"] = ranges["{0, 1}"]
						delete(ranges, "{0, 1}")
					case "wrong_summary_name":
						p15C01Params(rows[summary])["CustomOutputName"] = "differentSummary"
					case "wrong_summary_kind":
						rows[summary]["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
					case "detached_uuid":
						attachment["OutputUUID"] = "11111111-2222-4333-8444-555555555555"
					case "duplicate_uuid":
						p15C01Params(rows[0])["UUID"] = p15C01Params(rows[summary])["UUID"]
					case "duplicate_output":
						rows = append(rows, last)
					case "extra_output_field":
						last["Unexpected"] = true
					}
					if reflect.DeepEqual(rows, native) {
						t.Fatal("control did not mutate actual native output")
					}
					if p15MacroExplicitOutputError(p15MacroRetainNativeRows(t, rows)) == nil {
						t.Fatal("accepted corrupt explicit macro output")
					}
				})
			}
		})
	}
}
