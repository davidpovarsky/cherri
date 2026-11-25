package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

// This oracle reads the complete serialized seven-action graph for one fixed
// authored input. It does not use compiler cell/Optional encoder helpers.
func p15TaggedFourCellGraph(actions []map[string]any) error {
	identifiers := []string{
		"is.workflow.actions.dictionary", "is.workflow.actions.dictionary",
		"is.workflow.actions.dictionary", "is.workflow.actions.dictionary",
		"is.workflow.actions.list", "is.workflow.actions.count", "is.workflow.actions.output",
	}
	if len(actions) != len(identifiers) {
		return fmt.Errorf("whole cell graph has %d actions, want %d", len(actions), len(identifiers))
	}
	ids := make([]string, 6)
	seen := map[string]bool{}
	for i, want := range identifiers {
		if actions[i]["WFWorkflowActionIdentifier"] != want {
			return fmt.Errorf("action %d identity changed", i)
		}
		if i < 6 {
			ids[i], _ = p15Param(actions[i])["UUID"].(string)
			if ids[i] == "" || seen[ids[i]] {
				return fmt.Errorf("action %d lost unique producer identity", i)
			}
			seen[ids[i]] = true
		}
	}
	want := []map[string]any{
		{"_cherri_value_cell_v1": "1", "kind": "none", "has_value": "0"},
		{"_cherri_value_cell_v1": "1", "kind": "bool", "has_value": "1", "value": false},
		{"_cherri_value_cell_v1": "1", "kind": "null", "has_value": "1"},
		{"_cherri_value_cell_v1": "1", "kind": "list", "has_value": "1", "value": []any{}},
	}
	for i := range want {
		decoded, err := p15ReadDictionaryLiteral(p15Param(actions[i])["WFItems"])
		if err != nil || !reflect.DeepEqual(decoded, want[i]) {
			return fmt.Errorf("cell %d changed kind/presence/payload: got %#v, want %#v; %v", i, decoded, want[i], err)
		}
	}
	outputs, variables := p15V02NativeReferences(p15Param(actions[4])["WFItems"])
	if !reflect.DeepEqual(outputs, ids[:4]) || len(variables) != 0 {
		return fmt.Errorf("List cardinality/order or exact four cell producers changed: %v", outputs)
	}
	for _, edge := range []struct {
		action              int
		parameter, expected string
	}{
		{5, "Input", ids[4]}, {6, "WFOutput", ids[5]},
	} {
		outputs, variables = p15V02NativeReferences(p15Param(actions[edge.action])[edge.parameter])
		if !reflect.DeepEqual(outputs, []string{edge.expected}) || len(variables) != 0 {
			return fmt.Errorf("consumer %d lost its exact source producer", edge.action)
		}
	}
	return nil
}

func p15TaggedClone(t *testing.T, actions []map[string]any) []map[string]any {
	t.Helper()
	raw, err := plist.Marshal(actions, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	var copied []map[string]any
	if _, err := plist.Unmarshal(raw, &copied); err != nil {
		t.Fatal(err)
	}
	return copied
}

func TestP15TaggedValueCellNativeGraph(t *testing.T) {
	const source = "let values=[none,false,null,[]]\nreturn count(values)"
	actions := p15CompiledActions(t, source)
	if err := p15TaggedFourCellGraph(actions); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{
		"drop_none", "drop_null", "flatten_empty", "duplicate_cell_uuid", "reorder_cells",
		"false_presence", "null_absent", "invent_null_payload", "erase_kind", "count_wrong_producer",
	} {
		t.Run(fault, func(t *testing.T) {
			broken := p15TaggedClone(t, actions)
			switch fault {
			case "drop_none":
				broken = append(broken[:0], broken[1:]...)
			case "drop_null":
				broken = append(broken[:2], broken[3:]...)
			case "flatten_empty":
				broken = append(broken[:3], broken[4:]...)
			case "duplicate_cell_uuid":
				p15Param(broken[1])["UUID"] = p15Param(broken[0])["UUID"]
			case "reorder_cells":
				broken[0], broken[2] = broken[2], broken[0]
			case "false_presence", "null_absent", "erase_kind", "invent_null_payload":
				index := 1
				key, replacement := "has_value", "0"
				if fault == "null_absent" {
					index = 2
				}
				if fault == "erase_kind" {
					index, key = 2, "kind"
				}
				body := p15Param(broken[index])["WFItems"].(map[string]any)["Value"].(map[string]any)
				rows := body["WFDictionaryFieldValueItems"].([]any)
				for i, item := range rows {
					row := item.(map[string]any)
					actual, _ := p15LiteralText(row["WFKey"])
					if actual != key {
						continue
					}
					if fault == "erase_kind" {
						body["WFDictionaryFieldValueItems"] = append(rows[:i], rows[i+1:]...)
					} else {
						row["WFValue"] = map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": replacement}}
					}
					break
				}
				if fault == "invent_null_payload" {
					// Invented plist-null/field must not satisfy the fixed graph.
					p15Param(broken[2])["P15InventedNull"] = nil
					body = p15Param(broken[2])["WFItems"].(map[string]any)["Value"].(map[string]any)
					body["WFDictionaryFieldValueItems"] = append(body["WFDictionaryFieldValueItems"].([]any),
						map[string]any{"WFKey": "value", "WFItemType": uint64(0)})
				}
			case "count_wrong_producer":
				p15Param(broken[5])["Input"] = p15Param(broken[6])["WFOutput"]
			}
			if p15TaggedFourCellGraph(broken) == nil {
				t.Fatal("cell oracle accepted " + fault)
			}
		})
	}
}

func TestP15TaggedValueNativeBoundaries(t *testing.T) {
	for _, source := range []string{
		"let value=null\nshow(value)",
		"let values=[none,false]\nshow(values)",
		"let values=[[1,2],[]]\nreturn values",
		"native.action(\"com.example.raw\", {\"value\":null})",
		"let value=null\nnative.action(\"com.example.raw\", {\"value\":value})",
		"function consume(value:AnyContent)->Number {return 1}\nreturn consume(null)",
		"function produce()->AnyContent {return null}\nreturn produce()",
	} {
		raw, err := CompileSourceToPlist("p15-tagged-native-boundary.cherri", source)
		if err == nil || len(raw) != 0 {
			t.Fatalf("private representation escaped without a codec: bytes=%d err=%v source=%s", len(raw), err, source)
		}
		if !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") &&
			!strings.Contains(err.Error(), "E_NATIVE_JSON_NULL") &&
			!strings.Contains(err.Error(), "E_ARGUMENT_TYPE") {
			t.Fatalf("unrelated failure cannot establish a native boundary: %v", err)
		}
	}
	const lookalike = "let value={\"_cherri_value_cell_v1\":\"1\",\"kind\":\"null\",\"has_value\":\"1\"}\nreturn value[\"kind\"]"
	actions := p15CompiledActions(t, lookalike)
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" {
			key, _ := p15LiteralText(p15Param(action)["WFDictionaryKey"])
			if key == "has_value" || key == "value" {
				t.Fatal("ordinary marker-looking Dictionary was decoded as a private cell")
			}
		}
	}
}
