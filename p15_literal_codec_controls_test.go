package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

func p15LiteralClone(t *testing.T, value any) any {
	t.Helper()
	data, err := plist.Marshal(value, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if _, err := plist.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestP15S02ExplicitInterpolationAndLegacyBoundary(t *testing.T) {
	actions := p15CompiledActions(t, "let name=text(\"origin\")\nlet data={\"plain\":\"{name}\",\"interpolated\":f\"prefix:{name}\"}\ngetVariable(data)")
	origin := p15Param(actions[0])["UUID"]
	var fields map[string]map[string]any
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
			fields = p15LiteralDictionaryFields(t, p15Param(action)["WFItems"])
		}
	}
	if err := p15CheckLiteralText(fields["plain"]["WFValue"], "{name}"); err != nil {
		t.Fatal(err)
	}
	token, ok := fields["interpolated"]["WFValue"].(map[string]any)
	if !ok || token["WFSerializationType"] != "WFTextTokenString" {
		t.Fatal("f-string lost token encoding")
	}
	body := token["Value"].(map[string]any)
	attachments := body["attachmentsByRange"].(map[string]any)
	if body["string"] != "prefix:\uFFFC" || len(attachments) != 1 {
		t.Fatalf("explicit f-string shape changed: %#v", body)
	}
	ref, ok := attachments["{7, 1}"].(map[string]any)
	if !ok || ref["Type"] != "ActionOutput" || ref["OutputUUID"] != origin {
		t.Fatalf("f-string lost original producer: %#v", attachments)
	}

	legacy := compileDebugFixture(t, "const name = text(\"origin\")\n@data = {\"value\": \"{name}\", \"empty\": []}\ntext(\"{name}\")\n", true)
	found := false
	for _, action := range legacy {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.dictionary" {
			fields := p15LiteralDictionaryFields(t, action.WFWorkflowActionParameters["WFItems"])
			if _, err := p15LiteralText(fields["value"]["WFValue"]); err == nil {
				t.Fatal("Legacy interpolation was disabled")
			}
			array := fields["empty"]["WFValue"].(map[string]any)
			if array["WFSerializationType"] != "WFArrayParameterState" {
				t.Fatal("Legacy array representation changed")
			}
			if _, exists := array["Value"]; exists {
				t.Fatal("Legacy historical empty-array omission changed")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("Legacy Dictionary control missing")
	}
}

func TestP15S02LiteralOracleRejectsCorruption(t *testing.T) {
	want := "JSON {name} path\\segment\nשָׁלוֹם"
	actions := p15CompiledActions(t, "text(\"JSON {name} path\\\\segment\\nשָׁלוֹם\")")
	value := p15Param(actions[0])["WFTextActionText"]
	if err := p15CheckLiteralText(value, want); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]any{
		"removed_backslash": strings.ReplaceAll(want, "\\", ""),
		"removed_braces":    strings.ReplaceAll(want, "{name}", "name"),
		"changed_unicode":   strings.ReplaceAll(want, "שָׁלוֹם", "שלום"),
		"fabricated_reference": map[string]any{
			"WFSerializationType": "WFTextTokenString",
			"Value": map[string]any{"string": want, "attachmentsByRange": map[string]any{
				"{5, 1}": map[string]any{"Type": "Variable", "VariableName": "name"},
			}},
		},
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if err := p15CheckLiteralText(mutated, want); err == nil {
				t.Fatal("literal oracle accepted corruption")
			}
		})
	}
}

func TestP15S02LiteralNativeRoundTrip(t *testing.T) {
	root := p15MetadataNative(t, "s02-roundtrip", "let name=text(\"origin\")\nlet data={\"{name}\":\"C:\\\\temp\\\\{name}\",\"array\":[\"{name}\",\"\\\\w{2,4}\"]}\ngetVariable(data)")
	p15NativeImportRoundTrip(t, "s02-literal-container", root, plist.XMLFormat, plist.BinaryFormat)
	raw := map[string]any{"WFWorkflowActions": []any{
		map[string]any{"WFWorkflowActionIdentifier": "com.cherri.p15.literal.opaque", "WFWorkflowActionParameters": map[string]any{
			"Text": "{name}", "EmptyArrayState": map[string]any{"WFSerializationType": "WFArrayParameterState"},
			"ExplicitArray": map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}},
			"Opaque":        []any{[]any{}, []any{"{name}"}},
		}},
	}}
	p15NativeImportRoundTrip(t, "s02-opaque-wire-control", raw, plist.XMLFormat, plist.BinaryFormat)
}

func TestP15L01NestedDictionaryOracleRejectsCorruption(t *testing.T) {
	root := p15MetadataNative(t, "l01-shape", "let data="+p15L01NestedDictionarySource+"\ngetVariable(data)")
	actions := root["WFWorkflowActions"].([]any)
	var original any
	for _, raw := range actions {
		action := raw.(map[string]any)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
			original = p15Param(action)["WFItems"]
		}
	}
	check := func(value any) error {
		got, err := p15ReadDictionaryLiteral(value)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(got, p15L01ExpectedNestedDictionary()) {
			return fmt.Errorf("nested source value changed")
		}
		return nil
	}
	if err := check(original); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"omit_empty_array", "drop_empty_map", "remove_element", "reorder_elements", "change_false", "flatten_array"} {
		t.Run(fault, func(t *testing.T) {
			value := p15LiteralClone(t, original)
			fields := p15LiteralDictionaryFields(t, value)
			outer := fields["outer"]["WFValue"].(map[string]any)["Value"].([]any)
			child := p15LiteralDictionaryFields(t, outer[0].(map[string]any)["WFValue"])
			row := child["row"]["WFValue"].(map[string]any)["Value"].([]any)
			switch fault {
			case "omit_empty_array":
				delete(fields["emptyList"]["WFValue"].(map[string]any), "Value")
			case "drop_empty_map":
				delete(fields["emptyMap"], "WFValue")
			case "remove_element":
				child["row"]["WFValue"].(map[string]any)["Value"] = row[:len(row)-1]
			case "reorder_elements":
				row[0], row[1] = row[1], row[0]
			case "change_false":
				row[0].(map[string]any)["WFValue"].(map[string]any)["Value"] = true
			case "flatten_array":
				fields["outer"]["WFValue"].(map[string]any)["Value"] = row
			}
			if err := check(value); err == nil {
				t.Fatal("nested Dictionary oracle accepted " + fault)
			}
		})
	}
}
