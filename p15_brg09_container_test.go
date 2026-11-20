package main

import (
	"fmt"
	"reflect"
	"testing"

	"howett.net/plist"
)

const p15BRG09Literal = `{"WFSerializationType":"WFTextTokenAttachment","Value":false,"WFColorRepresentationType":"ordinary","{origin}":"{origin}","ordered":[0,-2.5,9007199254740993,false,true,"",[],{},[1,2],{"flag":false,"value":origin}],"reference":origin}`

func p15BRG09SemanticSource(route string) string {
	value := p15BRG09Literal
	if route == "list" {
		value = "[" + value + ",{}]"
	}
	return "let origin=text(\"P15_BRG09_REFERENCE\")\nlet data=" + value + "\ngetVariable(data)"
}

func p15BRG09ExpectedDictionary(reference p15ContainerReference) map[string]any {
	return map[string]any{
		"WFSerializationType":       "WFTextTokenAttachment",
		"Value":                     false,
		"WFColorRepresentationType": "ordinary",
		"{origin}":                  "{origin}",
		"ordered": []any{
			p15ContainerNumber("0"), p15ContainerNumber("-2.5"), p15ContainerNumber("9007199254740993"),
			false, true, "", []any{}, map[string]any{},
			[]any{p15ContainerNumber("1"), p15ContainerNumber("2")},
			map[string]any{"flag": false, "value": reference},
		},
		"reference": reference,
	}
}

func p15BRG09Actions(root map[string]any, count int) ([]map[string]any, error) {
	raw, ok := root["WFWorkflowActions"].([]any)
	if !ok || len(raw) != count {
		return nil, fmt.Errorf("container producer/consumer action count changed: %T %d", root["WFWorkflowActions"], len(raw))
	}
	actions := make([]map[string]any, count)
	for index, item := range raw {
		action, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("action %d is not a native dictionary", index)
		}
		actions[index] = action
	}
	return actions, nil
}

func p15BRG09Producer(action map[string]any, kind, field string, value any) (p15ContainerReference, error) {
	p := p15Param(action)
	uuid, _ := p["UUID"].(string)
	name, _ := p["CustomOutputName"].(string)
	if action["WFWorkflowActionIdentifier"] != "is.workflow.actions."+kind || uuid == "" || name == "" {
		return nil, fmt.Errorf("missing typed %s producer with published identity", kind)
	}
	if !reflect.DeepEqual(p, map[string]any{"UUID": uuid, "CustomOutputName": name, field: value}) {
		return nil, fmt.Errorf("%s producer semantic parameters/wrapper depth changed", kind)
	}
	return p15ContainerReference{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": name}, nil
}

func p15BRG09SemanticError(root map[string]any, route string) error {
	count := 3
	if route == "list" {
		count = 5
	}
	actions, err := p15BRG09Actions(root, count)
	if err != nil {
		return err
	}
	origin, err := p15BRG09Producer(actions[0], "gettext", "WFTextActionText", "P15_BRG09_REFERENCE")
	if err != nil {
		return err
	}
	_, dictionary := p15ContainerFieldValue(p15BRG09ExpectedDictionary(origin))
	result, err := p15BRG09Producer(actions[1], "dictionary", "WFItems", dictionary)
	if err != nil {
		return err
	}
	if route == "list" {
		_, empty := p15ContainerFieldValue(map[string]any{})
		emptyReference, err := p15BRG09Producer(actions[2], "dictionary", "WFItems", empty)
		if err != nil {
			return err
		}
		_, first := p15ContainerFieldValue(result)
		_, second := p15ContainerFieldValue(emptyReference)
		rows := []any{
			map[string]any{"WFItemType": uint64(0), "WFValue": first},
			map[string]any{"WFItemType": uint64(0), "WFValue": second},
		}
		result, err = p15BRG09Producer(actions[3], "list", "WFItems", rows)
		if err != nil {
			return err
		}
	}
	_, expected := p15ContainerFieldValue(result)
	consumer := actions[len(actions)-1]
	if consumer["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" || !reflect.DeepEqual(p15Param(consumer)["WFVariable"], expected) {
		return fmt.Errorf("final consumer is detached from the actual typed container")
	}
	return nil
}

const p15BRG09NativeUUID = "6e7b2da1-ec9f-40ac-b592-865613708294"

const p15BRG09NativeTemplateSource = `native.action("is.workflow.actions.gettext",parameters:native.plist({"WFTextActionText":"P15_BRG09_NATIVE_REFERENCE","UUID":"6e7b2da1-ec9f-40ac-b592-865613708294","CustomOutputName":"NativeRef"}))
native.action("com.example.p15.brg09.template",parameters:native.plist({"WFSerializationType":"WFTextTokenAttachment","Value":false,"WFColorRepresentationType":"ordinary","{origin}":"{origin}","WFValue":[0,-2.5,9007199254740993,false,true,"",[],{},[1,2]],"Reference":{"WFSerializationType":"WFTextTokenAttachment","Value":{"Type":"ActionOutput","OutputUUID":"6e7b2da1-ec9f-40ac-b592-865613708294","OutputName":"NativeRef"}},"EmptyDictionaryState":{"WFSerializationType":"WFDictionaryFieldValue","Value":{"WFDictionaryFieldValueItems":[]}},"EmptyArrayState":{"WFSerializationType":"WFArrayParameterState","Value":[]}}))`

func p15BRG09ExpectedNativeTemplate() map[string]any {
	return map[string]any{
		"WFSerializationType":       "WFTextTokenAttachment",
		"Value":                     false,
		"WFColorRepresentationType": "ordinary",
		"{origin}":                  "{origin}",
		"WFValue":                   []any{uint64(0), float64(-2.5), uint64(9007199254740993), false, true, "", []any{}, map[string]any{}, []any{uint64(1), uint64(2)}},
		"Reference": map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{
			"Type": "ActionOutput", "OutputUUID": p15BRG09NativeUUID, "OutputName": "NativeRef",
		}},
		"EmptyDictionaryState": map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{}}},
		"EmptyArrayState":      map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}},
	}
}

func p15BRG09NativeTemplateError(root map[string]any) error {
	actions, err := p15BRG09Actions(root, 2)
	if err != nil {
		return err
	}
	if actions[0]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || !reflect.DeepEqual(p15Param(actions[0]), map[string]any{
		"WFTextActionText": "P15_BRG09_NATIVE_REFERENCE", "UUID": p15BRG09NativeUUID, "CustomOutputName": "NativeRef",
	}) {
		return fmt.Errorf("exact native reference producer changed")
	}
	if actions[1]["WFWorkflowActionIdentifier"] != "com.example.p15.brg09.template" || !reflect.DeepEqual(p15Param(actions[1]), p15BRG09ExpectedNativeTemplate()) {
		return fmt.Errorf("opaque native template was reinterpreted or lost exact typed data")
	}
	return nil
}

func TestBackendRecovery09SemanticContainersAndNativeTemplates(t *testing.T) {
	for _, route := range []string{"dictionary", "list"} {
		t.Run(route, func(t *testing.T) {
			root := p15MetadataNative(t, "brg09-"+route, p15BRG09SemanticSource(route))
			if err := p15BRG09SemanticError(root, route); err != nil {
				t.Fatalf("%v\n%#v", err, root["WFWorkflowActions"])
			}
		})
	}
	t.Run("native_template", func(t *testing.T) {
		root := p15MetadataNative(t, "brg09-template", p15BRG09NativeTemplateSource)
		if err := p15BRG09NativeTemplateError(root); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBackendRecovery09NativeRoundTrip(t *testing.T) {
	for _, route := range []string{"dictionary", "list", "native_template"} {
		source := p15BRG09SemanticSource(route)
		check := func(root map[string]any) error { return p15BRG09SemanticError(root, route) }
		if route == "native_template" {
			source, check = p15BRG09NativeTemplateSource, p15BRG09NativeTemplateError
		}
		root := p15MetadataNative(t, "brg09-roundtrip-"+route, source)
		if err := check(root); err != nil {
			t.Fatal(err)
		}
		observations := p15NativeImportRoundTrip(t, "brg09-"+route, root, plist.XMLFormat, plist.BinaryFormat)
		if len(observations) != 8 {
			t.Fatalf("missing actual XML/binary hybrid/lossless Legacy/V2 observations: %d", len(observations))
		}
		for _, observation := range observations {
			if err := check(observation.compiled); err != nil {
				t.Fatalf("%s %s/%s lost authored container or exact template: %v", route, observation.mode, observation.fidelity, err)
			}
		}
	}
}

func TestBackendRecovery09OracleRejectsCorruption(t *testing.T) {
	original := p15MetadataNative(t, "brg09-control", p15BRG09SemanticSource("dictionary"))
	if err := p15BRG09SemanticError(original, "dictionary"); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"omitted_rows", "extra_wrapper", "flattened_array", "missing_empty_list", "missing_empty_map", "false_to_zero", "zero_to_text", "large_number_precision", "array_order", "array_cardinality", "reference_uuid", "reference_coercion", "producer_type"} {
		t.Run("semantic/"+fault, func(t *testing.T) {
			root := deepClonePlistMap(original)
			actions, _ := p15BRG09Actions(root, 3)
			params := p15Param(actions[1])
			body := params["WFItems"].(map[string]any)
			fields, err := p15ContainerFields(body)
			if err != nil {
				t.Fatal(err)
			}
			array := fields["ordered"]["WFValue"].(map[string]any)
			items := array["Value"].([]any)
			switch fault {
			case "omitted_rows":
				delete(body["Value"].(map[string]any), "WFDictionaryFieldValueItems")
			case "extra_wrapper":
				params["WFItems"] = map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": body}
			case "flattened_array":
				array["Value"] = append(items[:8], items[8].(map[string]any)["WFValue"].(map[string]any)["Value"].([]any)...)
			case "missing_empty_list":
				delete(items[6].(map[string]any)["WFValue"].(map[string]any), "Value")
			case "missing_empty_map":
				delete(items[7].(map[string]any)["WFValue"].(map[string]any)["Value"].(map[string]any), "WFDictionaryFieldValueItems")
			case "false_to_zero":
				items[3] = map[string]any{"WFItemType": uint64(3), "WFValue": p15ContainerText("0")}
			case "zero_to_text":
				items[0].(map[string]any)["WFItemType"] = uint64(0)
			case "large_number_precision":
				items[2].(map[string]any)["WFValue"] = p15ContainerText("9007199254740992")
			case "array_order":
				items[0], items[3] = items[3], items[0]
			case "array_cardinality":
				array["Value"] = append(items, items[0])
			case "reference_uuid":
				fields["reference"]["WFValue"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "MISSING"
			case "reference_coercion":
				fields["reference"]["WFValue"].(map[string]any)["Value"].(map[string]any)["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
			case "producer_type":
				actions[1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
			}
			if err := p15BRG09SemanticError(root, "dictionary"); err == nil {
				t.Fatal("accepted semantic container corruption " + fault)
			}
		})
	}
	classic := p15MetadataNative(t, "brg09-opaque-control", p15BRG09NativeTemplateSource)
	if err := p15BRG09NativeTemplateError(classic); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"semantic_encoding", "missing_false", "missing_empty", "array_order", "large_number_precision", "reference_name"} {
		t.Run("native/"+fault, func(t *testing.T) {
			root := deepClonePlistMap(classic)
			actions, _ := p15BRG09Actions(root, 2)
			p := p15Param(actions[1])
			switch fault {
			case "semantic_encoding":
				actions[1]["WFWorkflowActionParameters"] = map[string]any{"WFItems": p}
			case "missing_false":
				delete(p, "Value")
			case "missing_empty":
				delete(p["EmptyArrayState"].(map[string]any), "Value")
			case "array_order":
				items := p["WFValue"].([]any)
				items[0], items[3] = items[3], items[0]
			case "large_number_precision":
				p["WFValue"].([]any)[2] = uint64(9007199254740992)
			case "reference_name":
				p["Reference"].(map[string]any)["Value"].(map[string]any)["OutputName"] = "WRONG"
			}
			if err := p15BRG09NativeTemplateError(root); err == nil {
				t.Fatal("accepted opaque native template corruption " + fault)
			}
		})
	}
}
