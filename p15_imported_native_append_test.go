package main

import (
	"fmt"
	"howett.net/plist"
	"strings"
	"testing"
)

func p15ImportedAppendSource(t *testing.T, flag any, body string) string {
	t.Helper()
	entry := map[string]any{"Kind": "Variable", "VariableName": "Target"}
	if flag != nil {
		entry["AppendVariable"] = flag
	}
	header := map[string]any{"WFWorkflowName": "P15_Imported_Append", "__CherriImportProvenance": map[string]any{"Symbols": map[string]any{"target": entry}}}
	b, err := plist.Marshal(header, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("native.workflow(native.plist(%q), exact: true)\n%s", string(b), body)
}

func p15ImportedAppendBody(projection string) string {
	return fmt.Sprintf("let parsed = getDictionary(text(%q))\ntarget += parsed%s\nshow(target)\n", "{\"key\":\"VALUE\"}", projection)
}

func TestP15ImportedAppendOptionalNativeProjection(t *testing.T) {
	for name, projection := range map[string]string{"dictionary_key": "[\"key\"]", "coerced_dictionary_key": ".Dictionary[\"key\"]"} {
		t.Run(name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("imported-append.cherri", p15ImportedAppendSource(t, true, p15ImportedAppendBody(projection)))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := decodeShortcutRawMap(raw)
			if err != nil {
				t.Fatal(err)
			}
			actions, ok := doc["WFWorkflowActions"].([]any)
			if !ok {
				t.Fatalf("actions type %T", doc["WFWorkflowActions"])
			}
			expected := []string{"is.workflow.actions.gettext", "is.workflow.actions.detect.dictionary", "is.workflow.actions.appendvariable", "is.workflow.actions.showresult"}
			if len(actions) != len(expected) {
				t.Fatalf("expected4 exact native actions got%d", len(actions))
			}
			for i, raw := range actions {
				a := raw.(map[string]any)
				if a["WFWorkflowActionIdentifier"] != expected[i] {
					t.Fatalf("action%d mismatch %v", i, a)
				}
				if strings.Contains(fmt.Sprint(a), "_cherri_optional_presence_") {
					t.Fatal("native append fabricated presence")
				}
			}
		})
	}
}

func TestP15ImportedAppendPresentControl(t *testing.T) {
	source := p15ImportedAppendSource(t, true, "target += \"VALUE\"\nshow(target)\n")
	if _, err := CompileSourceToPlist("imported-append-present.cherri", source); err != nil {
		t.Fatal(err)
	}
}

func TestP15ImportedAppendOptionalBoundaries(t *testing.T) {
	for name, flag := range map[string]any{"missing_marker": nil, "false_marker": false, "nonboolean_marker": "true"} {
		t.Run(name, func(t *testing.T) {
			_, err := CompileSourceToPlist("append-marker-negative.cherri", p15ImportedAppendSource(t, flag, p15ImportedAppendBody("[\"key\"]")))
			if err == nil || !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") {
				t.Fatalf("expected present-operand guard, got %v", err)
			}
		})
	}
	for name, body := range map[string]string{
		"authored_optional_lhs":  "var count: Number? = none\ncount += 1\n",
		"authored_optional_rhs":  "var count: Number = 0\nlet parsed=getDictionary(text(\"{}\"))\ncount += parsed[\"key\"]\n",
		"authored_shadow":        "if true {\nvar target: Number? = none\ntarget += 1\n}\n",
		"minus_is_arithmetic":    strings.Replace(p15ImportedAppendBody("[\"key\"]"), "target +=", "target -=", 1),
		"multiply_is_arithmetic": strings.Replace(p15ImportedAppendBody("[\"key\"]"), "target +=", "target *=", 1),
		"divide_is_arithmetic":   strings.Replace(p15ImportedAppendBody("[\"key\"]"), "target +=", "target /=", 1),
		"none_literal":           "target += none\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := CompileSourceToPlist("append-optional-negative.cherri", p15ImportedAppendSource(t, true, body))
			if err == nil || !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") {
				t.Fatalf("expected argument type rejection, got %v", err)
			}
		})
	}
}

func TestP15ImportedAppendDoesNotInventOptionalPresence(t *testing.T) {
	body := p15ImportedAppendBody("[\"key\"]") + "function echo(value: AnyContent?) -> AnyContent? {\nreturn value\n}\nlet result=echo(target)\nreturn result\n"
	_, err := CompileSourceToPlist("append-presence-negative.cherri", p15ImportedAppendSource(t, true, body))
	if err == nil || (!strings.Contains(err.Error(), "E_ARGUMENT_TYPE") && !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE")) {
		t.Fatalf("expected independent Optional presence rejection, got %v", err)
	}
}
