package main

import (
	"fmt"
	"reflect"
	"testing"
)

// These expectations use the existing native text-token contract directly.
// They never stringify compiler values or call the production key encoder.
func p15ComputedKeyText(ref map[string]any, prefix, suffix string, utf16Offset int) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{
		"string":             prefix + "\uFFFC" + suffix,
		"attachmentsByRange": map[string]any{fmt.Sprintf("{%d, 1}", utf16Offset): ref},
	}}
}

func p15ComputedKeyRows(actions []map[string]any) ([]map[string]any, error) {
	payload, err := p15ContainerOrderedPayload(actions, false)
	if err != nil {
		return nil, err
	}
	wrapper, ok := payload.(map[string]any)
	if !ok || len(wrapper) != 2 || wrapper["WFSerializationType"] != "WFDictionaryFieldValue" {
		return nil, fmt.Errorf("computed-key Dictionary wrapper changed")
	}
	inner, ok := wrapper["Value"].(map[string]any)
	if !ok || len(inner) != 1 {
		return nil, fmt.Errorf("computed-key Dictionary wrapper depth changed")
	}
	items, ok := inner["WFDictionaryFieldValueItems"].([]any)
	if !ok {
		return nil, fmt.Errorf("computed-key Dictionary rows missing")
	}
	rows := make([]map[string]any, len(items))
	for index, raw := range items {
		row, ok := raw.(map[string]any)
		if !ok || len(row) != 3 {
			return nil, fmt.Errorf("computed-key row %d changed", index)
		}
		rows[index] = row
	}
	return rows, nil
}

func TestP15ComputedMapKeysPreserveTextIdentity(t *testing.T) {
	for _, name := range []string{"mutable", "action", "unicode_interpolation", "literal_braces", "numeric_literal"} {
		t.Run(name, func(t *testing.T) {
			prefix, key := "var key=\"P15_KEY_ORIGINAL\"\n", "key"
			want := p15ComputedKeyText(map[string]any{"Type": "Variable", "VariableName": "key"}, "", "", 0)
			switch name {
			case "action":
				prefix, key = "", "text(\"P15_KEY_ACTION\")"
			case "unicode_interpolation":
				key = "f\"🙂:{key}:tail\""
				want = p15ComputedKeyText(map[string]any{"Type": "Variable", "VariableName": "key"}, "🙂:", ":tail", 3)
			case "literal_braces":
				key = "\"{key}\""
				want = p15ContainerText("{key}")
			case "numeric_literal":
				prefix, key = "", "12"
				want = p15ContainerText("12")
			}
			actions := p15CompiledActions(t, prefix+"let data={"+key+":\"P15_KEY_VALUE\"}\ngetVariable(data)")
			if name == "action" {
				count := 0
				for _, action := range actions {
					p := p15Param(action)
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && p["WFTextActionText"] == "P15_KEY_ACTION" {
						uuid, ok := p["UUID"].(string)
						if !ok || uuid == "" {
							t.Fatal("computed-key action has no output identity")
						}
						ref := map[string]any{"Type": "ActionOutput", "OutputUUID": uuid}
						if outputName, ok := p["CustomOutputName"].(string); ok && outputName != "" {
							ref["OutputName"] = outputName
						}
						want = p15ComputedKeyText(ref, "", "", 0)
						count++
					}
				}
				if count != 1 {
					t.Fatalf("key action executed %d times in native graph", count)
				}
			}
			rows, err := p15ComputedKeyRows(actions)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0]["WFItemType"] != uint64(0) || !reflect.DeepEqual(rows[0]["WFValue"], p15ContainerText("P15_KEY_VALUE")) {
				t.Fatalf("computed-key field value/cardinality changed: %#v", rows)
			}
			if !reflect.DeepEqual(rows[0]["WFKey"], want) {
				t.Fatalf("computed Map key lost native text/reference identity: got %#v want %#v", rows[0]["WFKey"], want)
			}
		})
	}
}

func TestP15ComputedMapKeyWritesCaptureEarlierValue(t *testing.T) {
	source := "var state=\"P15_CONTAINER_BEFORE\"\nlet data={\"before\":state," + p15ContainerWriteExpression + ":\"key-result\"}\ngetVariable(data)"
	actions := p15CompiledActions(t, source)
	rows, err := p15ComputedKeyRows(actions)
	if err != nil || len(rows) != 2 || !reflect.DeepEqual(rows[0]["WFKey"], p15ContainerText("before")) {
		t.Fatalf("authored literal/dynamic row order changed: %#v %v", rows, err)
	}
	value, ok := rows[0]["WFValue"].(map[string]any)
	if !ok || len(value) != 2 || value["WFSerializationType"] != "WFTextTokenAttachment" {
		t.Fatal("early field lost its native reference wrapper")
	}
	if err := p15ComputedSnapshotError(actions, value["Value"]); err != nil {
		t.Fatal(err)
	}
}

func TestP15ComputedMapKeyCapturesBeforeOwnValueWrite(t *testing.T) {
	source := "var state=\"P15_CONTAINER_BEFORE\"\nlet data={state:" + p15ContainerWriteExpression + "}\ngetVariable(data)"
	actions := p15CompiledActions(t, source)
	rows, err := p15ComputedKeyRows(actions)
	if err != nil || len(rows) != 1 {
		t.Fatalf("key/value source row missing: %#v %v", rows, err)
	}
	key, ok := rows[0]["WFKey"].(map[string]any)
	if !ok {
		t.Fatal("computed key lost its text-token wrapper")
	}
	value, ok := key["Value"].(map[string]any)
	if !ok {
		t.Fatal("computed key lost its text-token value")
	}
	attachments, ok := value["attachmentsByRange"].(map[string]any)
	if !ok {
		t.Fatal("computed key lost its captured reference")
	}
	ref, ok := attachments["{0, 1}"].(map[string]any)
	if !ok || !reflect.DeepEqual(key, p15ComputedKeyText(ref, "", "", 0)) {
		t.Fatal("computed key is not exactly one reference text token")
	}
	if err := p15ComputedSnapshotError(actions, ref); err != nil {
		t.Fatal(err)
	}
}

func p15ComputedSnapshotError(actions []map[string]any, rawReference any) error {
	ref, ok := rawReference.(map[string]any)
	if !ok || len(ref) != 2 || ref["Type"] != "ActionOutput" {
		return fmt.Errorf("earlier computed-key/value reference was not snapshotted")
	}
	uuid, ok := ref["OutputUUID"].(string)
	if !ok || uuid == "" {
		return fmt.Errorf("computed-key/value snapshot has no identity")
	}
	init, capture, branch, write, stores, effects := -1, -1, -1, -1, 0, 0
	for index, action := range actions {
		p := p15Param(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "state" {
			stores++
			if init < 0 {
				init = index
			} else {
				write = index
			}
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && p["WFTextActionText"] == "P15_CONTAINER_AFTER" {
			effects++
		}
		if init >= 0 && branch < 0 && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
			branch = index
		}
		if p["UUID"] == uuid {
			if capture >= 0 || action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" || !reflect.DeepEqual(p["WFVariable"], map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": "state"}}) {
				return fmt.Errorf("computed-key snapshot is not one direct read of the original binding")
			}
			capture = index
		}
	}
	if stores != 2 || effects != 1 || !(init < capture && capture < branch && branch < write) {
		return fmt.Errorf("computed-key/value reference was not consumed once before its later write: init=%d capture=%d branch=%d write=%d stores=%d effects=%d", init, capture, branch, write, stores, effects)
	}
	return nil
}
