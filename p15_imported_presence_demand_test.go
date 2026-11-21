package main

import (
	"fmt"
	"howett.net/plist"
	"strings"
	"testing"
)

func p15ImportedPresenceSource(t *testing.T, body string) string {
	t.Helper()
	header := map[string]any{
		"WFWorkflowName": "P15_Imported_Presence",
		"__CherriImportProvenance": map[string]any{"Symbols": map[string]any{
			"target": map[string]any{"Kind": "Variable", "VariableName": "Target", "ValueType": ""},
		}},
	}
	b, err := plist.Marshal(header, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("native.workflow(native.plist(%q), exact: true)\n%s", string(b), body)
}

func p15ImportedPresenceDynamicBody() string {
	return fmt.Sprintf("let parsed = getDictionary(text(%q))\ntarget = parsed[%q]\ntarget = parsed[%q]\n", "{\"key\":\"VALUE\"}", "key", "key")
}

func TestP15ImportedPresenceUnusedAssignment(t *testing.T) {
	source := p15ImportedPresenceSource(t, p15ImportedPresenceDynamicBody()+"show(target)\n")
	b, err := CompileSourceToPlist("imported-presence.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := decodeShortcutRawMap(b)
	if err != nil {
		t.Fatal(err)
	}
	actions, ok := doc["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatalf("actions type %T", doc["WFWorkflowActions"])
	}
	expected := []string{"is.workflow.actions.gettext", "is.workflow.actions.detect.dictionary", "is.workflow.actions.setvariable", "is.workflow.actions.setvariable", "is.workflow.actions.showresult"}
	if len(actions) != len(expected) {
		t.Fatalf("expected exactly %d native actions, got %d", len(expected), len(actions))
	}
	for i, raw := range actions {
		action := raw.(map[string]any)
		if action["WFWorkflowActionIdentifier"] != expected[i] {
			t.Fatalf("action %d expected %s got %v", i, expected[i], action["WFWorkflowActionIdentifier"])
		}
		params, _ := action["WFWorkflowActionParameters"].(map[string]any)
		if strings.Contains(fmt.Sprint(params), "_cherri_optional_presence_") {
			t.Fatal("unused imported variable emitted an absence flag")
		}
	}
}

func TestP15ImportedPresenceLiteralAssignmentControl(t *testing.T) {
	source := p15ImportedPresenceSource(t, "target = \"VALUE\"\nshow(target)\n")
	if _, err := CompileSourceToPlist("imported-literal-control.cherri", source); err != nil {
		t.Fatal(err)
	}
}

func TestP15ImportedPresenceRequiredRemainsFailClosed(t *testing.T) {
	for name, tail := range map[string]string{
		"none_comparison":  "if target == none {\nshow(\"ABSENT\")\n}\n",
		"alias_comparison": "let alias=target\nif alias != none {\nshow(\"PRESENT\")\n}\n",
		"user_function":    "function echo(value: AnyContent?) -> AnyContent? {\nreturn value\n}\nlet result = echo(target)\nreturn result\n",
	} {
		t.Run(name, func(t *testing.T) {
			source := p15ImportedPresenceSource(t, p15ImportedPresenceDynamicBody()+tail)
			if _, err := CompileSourceToPlist("imported-presence-required.cherri", source); err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
				t.Fatalf("expected independent absence rejection; got %v", err)
			}
		})
	}
}
