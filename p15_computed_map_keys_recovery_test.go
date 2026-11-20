package main

import (
	"fmt"
	"reflect"
	"testing"

	"howett.net/plist"
)

func p15ComputedMarkerRows(value any, marker string) []map[string]any {
	var found []map[string]any
	var visit func(any)
	visit = func(value any) {
		switch value := value.(type) {
		case []map[string]any:
			for _, child := range value {
				visit(child)
			}
		case []any:
			for _, child := range value {
				visit(child)
			}
		case map[string]any:
			if value["WFSerializationType"] == "WFDictionaryFieldValue" {
				if inner, ok := value["Value"].(map[string]any); ok {
					if rows, ok := inner["WFDictionaryFieldValueItems"].([]any); ok {
						for _, raw := range rows {
							if row, ok := raw.(map[string]any); ok && reflect.DeepEqual(row["WFValue"], p15ContainerText(marker)) {
								found = append(found, row)
							}
						}
					}
				}
			}
			for _, child := range value {
				visit(child)
			}
		}
	}
	visit(value)
	return found
}

func TestP15ComputedMapKeyProductionRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		copies       int
	}{
		{"let", "let data={key:\"P15_KEY_ROUTE\"}\ngetVariable(data)", 1},
		{"var", "var data={key:\"P15_KEY_ROUTE\"}\ngetVariable(data)", 1},
		{"yield", "let data=if true {yield {key:\"P15_KEY_ROUTE\"}} else {yield {\"unused\":\"x\"}}\ngetVariable(data)", 1},
		{"function_argument", "function echo(value:Map<Text,AnyContent>)->Map<Text,AnyContent>{return value}\nlet data=echo({key:\"P15_KEY_ROUTE\"})\ngetVariable(data)", 2},
		{"native_list", "let data=[{key:\"P15_KEY_ROUTE\"}]\ngetVariable(data)", 1},
		{"dictionary_array", "let data={\"rows\":[{key:\"P15_KEY_ROUTE\"}]}\ngetVariable(data)", 1},
		{"canonical_list_argument", "chooseFromList([{key:\"P15_KEY_ROUTE\"}])", 1},
		{"canonical_named_dictionary", "jsonRequest(\"https://example.invalid/compile-only\",body:{key:\"P15_KEY_ROUTE\"})", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, "var key=\"P15_ROUTE_KEY\"\n"+tc.source)
			rows := p15ComputedMarkerRows(actions, "P15_KEY_ROUTE")
			if len(rows) != tc.copies {
				t.Fatalf("typed dynamic Map route lost or duplicated rows: got %d want %d", len(rows), tc.copies)
			}
			want := p15ComputedKeyText(map[string]any{"Type": "Variable", "VariableName": "key"}, "", "", 0)
			for _, row := range rows {
				if len(row) != 3 || row["WFItemType"] != uint64(0) || !reflect.DeepEqual(row["WFKey"], want) {
					t.Fatalf("route lost explicit dynamic key/value identity: %#v", row)
				}
			}
		})
	}
}

const p15ComputedMixedKeySource = "var key=\"z\"\nlet data={\"z\":\"first\",key:\"second\",\"a\":\"third\"}\ngetVariable(data)"

func p15ComputedMixedKeyError(actions []map[string]any) error {
	rows, err := p15ComputedKeyRows(actions)
	if err != nil || len(rows) != 3 {
		return fmt.Errorf("mixed key cardinality changed: %d %v", len(rows), err)
	}
	keys := []any{p15ContainerText("z"), p15ComputedKeyText(map[string]any{"Type": "Variable", "VariableName": "key"}, "", "", 0), p15ContainerText("a")}
	values := []string{"first", "second", "third"}
	for index, row := range rows {
		want := map[string]any{"WFKey": keys[index], "WFItemType": uint64(0), "WFValue": p15ContainerText(values[index])}
		if !reflect.DeepEqual(row, want) {
			return fmt.Errorf("mixed literal/computed keys were sorted, overwritten or changed at source row %d", index)
		}
	}
	return nil
}

func TestP15ComputedMapKeysKeepAuthoredCollisionOrder(t *testing.T) {
	if err := p15ComputedMixedKeyError(p15CompiledActions(t, p15ComputedMixedKeySource)); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"sort_literals", "drop_duplicate", "change_value"} {
		t.Run(fault, func(t *testing.T) {
			actions := p15CompiledActions(t, p15ComputedMixedKeySource)
			payload, _ := p15ContainerOrderedPayload(actions, false)
			inner := payload.(map[string]any)["Value"].(map[string]any)
			rows := inner["WFDictionaryFieldValueItems"].([]any)
			switch fault {
			case "sort_literals":
				rows[0], rows[2] = rows[2], rows[0]
			case "drop_duplicate":
				inner["WFDictionaryFieldValueItems"] = rows[1:]
			case "change_value":
				rows[1].(map[string]any)["WFValue"] = p15ContainerText("overwritten")
			}
			if err := p15ComputedMixedKeyError(actions); err == nil {
				t.Fatal("mixed key oracle accepted " + fault)
			}
		})
	}
}

func p15ComputedKeyReference(row map[string]any) (map[string]any, error) {
	key, ok := row["WFKey"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("computed key has no text wrapper")
	}
	value, ok := key["Value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("computed key has no text value")
	}
	attachments, ok := value["attachmentsByRange"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("computed key has no reference")
	}
	ref, ok := attachments["{0, 1}"].(map[string]any)
	if !ok || !reflect.DeepEqual(key, p15ComputedKeyText(ref, "", "", 0)) {
		return nil, fmt.Errorf("computed key changed its one-reference shape")
	}
	return ref, nil
}

func TestP15ComputedNestedKeysCaptureBeforeLaterWrite(t *testing.T) {
	for _, route := range []string{"map", "array"} {
		t.Run(route, func(t *testing.T) {
			value := "{state:\"P15_NESTED_KEY\"}"
			if route == "array" {
				value = "[" + value + "]"
			}
			source := "var state=\"P15_CONTAINER_BEFORE\"\nlet data={\"nested\":" + value + ",\"write\":" + p15ContainerWriteExpression + "}\ngetVariable(data)"
			actions := p15CompiledActions(t, source)
			rows := p15ComputedMarkerRows(actions, "P15_NESTED_KEY")
			if len(rows) != 1 {
				t.Fatalf("nested key missing/duplicated: %d", len(rows))
			}
			ref, err := p15ComputedKeyReference(rows[0])
			if err != nil {
				t.Fatal(err)
			}
			if err := p15ComputedSnapshotError(actions, ref); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15ComputedEarlierKeyCapturesBeforeLaterKeyWrite(t *testing.T) {
	source := "var state=\"P15_CONTAINER_BEFORE\"\nlet data={state:\"P15_EARLY_KEY\"," + p15ContainerWriteExpression + ":\"later\"}\ngetVariable(data)"
	actions := p15CompiledActions(t, source)
	rows, err := p15ComputedKeyRows(actions)
	if err != nil || len(rows) != 2 || !reflect.DeepEqual(rows[0]["WFValue"], p15ContainerText("P15_EARLY_KEY")) {
		t.Fatalf("earlier key row reordered or missing: %#v %v", rows, err)
	}
	ref, err := p15ComputedKeyReference(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := p15ComputedSnapshotError(actions, ref); err != nil {
		t.Fatal(err)
	}
}

func TestP15ComputedKeysKeepUnwrittenBindingReference(t *testing.T) {
	for _, tc := range []struct{ name, later string }{
		{"pure", "text(\"safe\")"},
		{"different_binding", "if true {other=\"after\";yield \"safe\"} else {yield \"unused\"}"},
		{"shadowed_binding", "if true {var state=\"local\";state=\"after\";yield \"safe\"} else {yield \"unused\"}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "var state=\"original\"\nvar other=\"original-other\"\nlet data={state:\"first\"," + tc.later + ":\"second\"}\ngetVariable(data)"
			actions := p15CompiledActions(t, source)
			rows, err := p15ComputedKeyRows(actions)
			want := p15ComputedKeyText(map[string]any{"Type": "Variable", "VariableName": "state"}, "", "", 0)
			if err != nil || len(rows) != 2 || !reflect.DeepEqual(rows[0]["WFKey"], want) {
				t.Fatalf("unwritten outer key changed reference: %#v %v", rows, err)
			}
			for _, action := range actions {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && reflect.DeepEqual(p15Param(action)["WFVariable"], map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": "state"}}) {
					t.Fatal("unwritten binding gained an unnecessary snapshot action")
				}
			}
		})
	}
}

const p15ComputedUnicodeKeySource = "var key=\"P15_UNICODE_KEY\"\nlet data={f\"🙂:{key}:tail\":\"P15_UNICODE_VALUE\"}\ngetVariable(data)"

func p15ComputedUnicodeKeyError(actions []map[string]any) error {
	rows, err := p15ComputedKeyRows(actions)
	if err != nil || len(rows) != 1 {
		return fmt.Errorf("Unicode computed key row missing/duplicated: %d %v", len(rows), err)
	}
	want := map[string]any{
		"WFKey":      p15ComputedKeyText(map[string]any{"Type": "Variable", "VariableName": "key"}, "🙂:", ":tail", 3),
		"WFItemType": uint64(0), "WFValue": p15ContainerText("P15_UNICODE_VALUE"),
	}
	if !reflect.DeepEqual(rows[0], want) {
		return fmt.Errorf("Unicode key token, UTF-16 offset, reference or value changed")
	}
	return nil
}

func TestP15ComputedMapKeyOracleRejectsCorruption(t *testing.T) {
	original := p15CompiledActions(t, p15ComputedUnicodeKeySource)
	if err := p15ComputedUnicodeKeyError(original); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"stringify", "missing_attachment", "wrong_variable", "wrong_offset", "extra_attachment", "wrong_value", "wrong_kind", "extra_field"} {
		t.Run(fault, func(t *testing.T) {
			actions := p15ContainerCloneNative(t, original)
			rows, _ := p15ComputedKeyRows(actions)
			row := rows[0]
			value := row["WFKey"].(map[string]any)["Value"].(map[string]any)
			attachments := value["attachmentsByRange"].(map[string]any)
			switch fault {
			case "stringify":
				row["WFKey"] = p15ContainerText("map[internal compiler value]")
			case "missing_attachment":
				delete(value, "attachmentsByRange")
			case "wrong_variable":
				attachments["{3, 1}"].(map[string]any)["VariableName"] = "wrong"
			case "wrong_offset":
				attachments["{2, 1}"] = attachments["{3, 1}"]
				delete(attachments, "{3, 1}")
			case "extra_attachment":
				attachments["{4, 1}"] = attachments["{3, 1}"]
			case "wrong_value":
				row["WFValue"] = p15ContainerText("changed")
			case "wrong_kind":
				row["WFItemType"] = uint64(3)
			case "extra_field":
				row["Invented"] = true
			}
			if err := p15ComputedUnicodeKeyError(actions); err == nil {
				t.Fatal("computed key oracle accepted " + fault)
			}
		})
	}
}

func TestP15ComputedMapKeyNativeRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		check        func([]map[string]any) error
	}{
		{"unicode", p15ComputedUnicodeKeySource, p15ComputedUnicodeKeyError},
		{"mixed_collision_order", p15ComputedMixedKeySource, p15ComputedMixedKeyError},
	} {
		root := p15MetadataNative(t, "computed-key-"+tc.name, tc.source)
		check := func(root map[string]any) error {
			raw, ok := root["WFWorkflowActions"].([]any)
			if !ok {
				return fmt.Errorf("actual native actions missing")
			}
			actions, err := p15BRG09Actions(root, len(raw))
			if err != nil {
				return err
			}
			return tc.check(actions)
		}
		if err := check(root); err != nil {
			t.Fatal(err)
		}
		observations := p15NativeImportRoundTrip(t, "computed-key-"+tc.name, root, plist.XMLFormat, plist.BinaryFormat)
		if len(observations) != 8 {
			t.Fatalf("missing actual XML/binary hybrid/lossless Legacy/V2 observations: %d", len(observations))
		}
		for _, observation := range observations {
			if err := check(observation.compiled); err != nil {
				t.Fatalf("%s %s/%s lost independent computed-key expectation: %v", tc.name, observation.mode, observation.fidelity, err)
			}
		}
	}
}
