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

// The selected value is queried only after an exact key-enumeration guard.
// No All Keys operation is ever applied to the possibly missing lookup result.
func p15TaggedMapMissingGuard(actions []map[string]any) error {
	allKeys, lookup := -1, -1
	var mapID string
	for i, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvalueforkey" {
			continue
		}
		parameters := p15Param(action)
		mode, _ := p15LiteralText(parameters["WFGetDictionaryValueType"])
		if mode == "All Keys" {
			if allKeys >= 0 {
				return fmt.Errorf("duplicate key enumeration")
			}
			allKeys = i
			outputs, variables := p15V02NativeReferences(parameters["WFInput"])
			if len(outputs) != 1 || len(variables) != 0 {
				return fmt.Errorf("key enumeration lost source Dictionary")
			}
			mapID = outputs[0]
		}
		key, _ := p15LiteralText(parameters["WFDictionaryKey"])
		if key == "missing" {
			lookup = i
		}
	}
	if allKeys < 0 || lookup <= allKeys || mapID == "" {
		return fmt.Errorf("lookup precedes source-key enumeration")
	}
	foundSource := false
	for i := 0; i < allKeys; i++ {
		if actions[i]["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" && p15Param(actions[i])["UUID"] == mapID {
			foundSource = true
		}
	}
	if !foundSource {
		return fmt.Errorf("key enumeration input is not the original Dictionary")
	}
	outputs, variables := p15V02NativeReferences(p15Param(actions[lookup])["WFInput"])
	if !reflect.DeepEqual(outputs, []string{mapID}) || len(variables) != 0 {
		return fmt.Errorf("lookup reads a different Dictionary")
	}
	for start := lookup - 1; start > allKeys; start-- {
		parameters := p15Param(actions[start])
		if actions[start]["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" ||
			p15Int(parameters["WFControlFlowMode"]) != 0 || p15Int(parameters["WFCondition"]) != 4 {
			continue
		}
		outputs, variables = p15V02NativeReferences(parameters["WFInput"])
		if len(outputs) != 0 || len(variables) != 1 || !strings.Contains(variables[0], "dictionaryKeyPresent_") {
			continue
		}
		value, _ := p15LiteralText(parameters["WFConditionalActionString"])
		if value != "1" {
			return fmt.Errorf("key-presence polarity changed")
		}
		group := parameters["GroupingIdentifier"]
		for end := start + 1; end < len(actions); end++ {
			next := p15Param(actions[end])
			if next["GroupingIdentifier"] == group && p15Int(next["WFControlFlowMode"]) != 0 {
				if end <= lookup {
					return fmt.Errorf("lookup escaped key-presence branch")
				}
				return nil
			}
		}
	}
	return fmt.Errorf("missing exact key-presence guard")
}

func TestP15TaggedMissingMapKeyGuard(t *testing.T) {
	const source = "let values:Map<Text,Optional<Bool>>={\"present\":none}\nlet absent=values[\"missing\"]\nreturn absent==none"
	actions := p15CompiledActions(t, source)
	if err := p15TaggedMapMissingGuard(actions); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"lookup_before_keys", "wrong_enumeration_source", "erase_guard", "opposite_guard"} {
		t.Run(fault, func(t *testing.T) {
			broken := p15TaggedClone(t, actions)
			allKeys, lookup := -1, -1
			for i, action := range broken {
				if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvalueforkey" {
					continue
				}
				mode, _ := p15LiteralText(p15Param(action)["WFGetDictionaryValueType"])
				if mode == "All Keys" {
					allKeys = i
				}
				key, _ := p15LiteralText(p15Param(action)["WFDictionaryKey"])
				if key == "missing" {
					lookup = i
				}
			}
			switch fault {
			case "lookup_before_keys":
				broken[lookup], broken[allKeys] = broken[allKeys], broken[lookup]
			case "wrong_enumeration_source":
				p15Param(broken[allKeys])["WFInput"] = map[string]any{"Type": "ActionOutput", "OutputUUID": "WRONG_OR_MISSING_CELL"}
			case "erase_guard", "opposite_guard":
				for i := allKeys + 1; i < lookup; i++ {
					parameters := p15Param(broken[i])
					_, variables := p15V02NativeReferences(parameters["WFInput"])
					if len(variables) == 1 && strings.Contains(variables[0], "dictionaryKeyPresent_") &&
						broken[i]["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(parameters["WFControlFlowMode"]) == 0 {
						if fault == "erase_guard" {
							parameters["WFControlFlowMode"] = uint64(1)
						} else {
							parameters["WFCondition"] = uint64(5)
						}
					}
				}
			}
			if p15TaggedMapMissingGuard(broken) == nil {
				t.Fatal("map oracle accepted " + fault)
			}
		})
	}
}
