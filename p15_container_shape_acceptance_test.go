package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Expected semantic kinds are authored independently from the compiler. These
// helpers describe the documented native Dictionary/array field contract; they
// do not call production codecs or run a replacement Apple interpreter.
type p15ContainerNumber string
type p15ContainerReference map[string]any

func p15ContainerText(value string) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": value}}
}

func p15ContainerFieldValue(value any) (uint64, any) {
	switch value := value.(type) {
	case string:
		return 0, p15ContainerText(value)
	case p15ContainerNumber:
		return 3, p15ContainerText(string(value))
	case bool:
		return 4, map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": value}
	case p15ContainerReference:
		return 0, map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any(value)}
	case []any:
		rows := []any{}
		for _, item := range value {
			kind, wire := p15ContainerFieldValue(item)
			rows = append(rows, map[string]any{"WFItemType": kind, "WFValue": wire})
		}
		return 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": rows}
	case map[string]any:
		keys := []string{}
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		rows := []any{}
		for _, key := range keys {
			kind, wire := p15ContainerFieldValue(value[key])
			rows = append(rows, map[string]any{"WFKey": p15ContainerText(key), "WFItemType": kind, "WFValue": wire})
		}
		return 1, map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": rows}}
	default:
		panic(fmt.Sprintf("unsupported independent container expectation %T", value))
	}
}

func p15ContainerFields(value any) (map[string]map[string]any, error) {
	outer, ok := value.(map[string]any)
	if !ok || len(outer) != 2 || outer["WFSerializationType"] != "WFDictionaryFieldValue" {
		return nil, fmt.Errorf("missing exact typed Dictionary wrapper")
	}
	inner, ok := outer["Value"].(map[string]any)
	if !ok || len(inner) != 1 {
		return nil, fmt.Errorf("Dictionary wrapper depth changed")
	}
	rows, ok := inner["WFDictionaryFieldValueItems"].([]any)
	if !ok {
		return nil, fmt.Errorf("Dictionary explicit item list missing")
	}
	fields := map[string]map[string]any{}
	previous := ""
	for index, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok || len(row) != 3 {
			return nil, fmt.Errorf("Dictionary field shape changed")
		}
		keyWrapper, ok := row["WFKey"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Dictionary key was not literal Text")
		}
		keyValue, ok := keyWrapper["Value"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Dictionary key wrapper missing")
		}
		key, ok := keyValue["string"].(string)
		if !ok || !reflect.DeepEqual(keyWrapper, p15ContainerText(key)) {
			return nil, fmt.Errorf("Dictionary key became interpolation or a reference")
		}
		if _, duplicate := fields[key]; duplicate || index > 0 && key <= previous {
			return nil, fmt.Errorf("Dictionary keys duplicated or canonical order changed")
		}
		fields[key] = row
		previous = key
	}
	return fields, nil
}

func p15FunctionContainerPayload(actions []map[string]any) (map[string]map[string]any, error) {
	var payload map[string]map[string]any
	count := 0
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			continue
		}
		parameters := p15Param(action)
		fields, err := p15ContainerFields(parameters["WFItems"])
		if err != nil {
			return nil, err
		}
		if _, found := fields["has_arg_0"]; found {
			payload = fields
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("expected exactly one actual function call envelope, got %d", count)
	}
	return payload, nil
}

func p15FunctionContainerArgumentError(actions []map[string]any, want map[string]any) error {
	fields, err := p15FunctionContainerPayload(actions)
	if err != nil {
		return err
	}
	if len(fields) != 7 {
		return fmt.Errorf("function envelope fields changed")
	}
	kind, wire := p15ContainerFieldValue(want)
	argument, found := fields["arg_0"]
	if !found || argument["WFItemType"] != kind || !reflect.DeepEqual(argument["WFValue"], wire) {
		return fmt.Errorf("authored function argument lost semantic Dictionary identity or values")
	}
	if !reflect.DeepEqual(fields["has_arg_0"], map[string]any{"WFKey": p15ContainerText("has_arg_0"), "WFItemType": uint64(0), "WFValue": p15ContainerText("1")}) {
		return fmt.Errorf("explicit structured argument lost presence")
	}
	wantArray := map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{map[string]any{"WFItemType": kind, "WFValue": wire}}}
	if fields["arguments"]["WFItemType"] != uint64(2) || !reflect.DeepEqual(fields["arguments"]["WFValue"], wantArray) {
		return fmt.Errorf("compatibility argument list lost shape/order/cardinality")
	}
	return nil
}

func TestP15FunctionArgumentContainersPreserveSemanticIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, literal string
		want          map[string]any
	}{
		{"empty", `{}`, map[string]any{}},
		{"reserved_attachment", `{"WFSerializationType":"WFTextTokenAttachment","Value":false,"WFItemType":4,"WFValue":["a","b"]}`, map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": false, "WFItemType": p15ContainerNumber("4"), "WFValue": []any{"a", "b"}}},
		{"reserved_color", `{"WFColorRepresentationType":"ordinary","Value":0}`, map[string]any{"WFColorRepresentationType": "ordinary", "Value": p15ContainerNumber("0")}},
		{"literal_braces", `{"{label}":"{label}","text":"literal{8}"}`, map[string]any{"{label}": "{label}", "text": "literal{8}"}},
		{"nested", `{"rows":[0,false,"",[],{},[1,2],{"WFSerializationType":"WFTextTokenString","Value":"ordinary"}]}`, map[string]any{"rows": []any{p15ContainerNumber("0"), false, "", []any{}, map[string]any{}, []any{p15ContainerNumber("1"), p15ContainerNumber("2")}, map[string]any{"WFSerializationType": "WFTextTokenString", "Value": "ordinary"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "function echo(value:Map<Text,AnyContent>)->Map<Text,AnyContent>{return value}\nlet result=echo(" + tc.literal + ")\ngetVariable(result)"
			if err := p15FunctionContainerArgumentError(p15CompiledActions(t, source), tc.want); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15FunctionContainerArgumentsCaptureBeforeLaterWrite(t *testing.T) {
	source := `function echo(value:Map<Text,AnyContent>,later:Text)->Map<Text,AnyContent>{return value}
var state="before"
let result=echo({"WFSerializationType":"WFTextTokenString","Value":"ordinary","observed":state},later:if true{state="after";yield "done"}else{yield "unused"})
getVariable(result)`
	actions := p15CompiledActions(t, source)
	fields, err := p15FunctionContainerPayload(actions)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := fields["arg_0"]["WFValue"].(map[string]any)
	if !ok {
		t.Fatal("snapshot argument is absent")
	}
	token, ok := value["Value"].(map[string]any)
	if !ok || value["WFSerializationType"] != "WFTextTokenAttachment" || token["Type"] != "ActionOutput" {
		t.Fatal("map was not captured before subsequent mutation")
	}
	want := map[string]any{"WFSerializationType": "WFTextTokenString", "Value": "ordinary", "observed": p15ContainerReference{"Type": "Variable", "VariableName": "state"}}
	_, expected := p15ContainerFieldValue(want)
	snapshotIndex, writeIndex := -1, -1
	for index, action := range actions {
		parameters := p15Param(action)
		if parameters["UUID"] == token["OutputUUID"] {
			if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" || !reflect.DeepEqual(parameters["WFItems"], expected) {
				t.Fatal("captured map was reinterpreted as native Text or changed values")
			}
			snapshotIndex = index
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && parameters["WFVariableName"] == "state" {
			writeIndex = index
		}
	}
	if snapshotIndex < 0 || writeIndex <= snapshotIndex {
		t.Fatal("structured argument was observed after the later assignment")
	}
}

const p15OpaqueContainerTemplate = `{"WFSerializationType":"WFDictionaryFieldValue","Value":false,"WFValue":[0,false,"",[],{},[1,2]],"WFDictionaryFieldValueItems":[{"WFKey":"opaque","WFValue":{"Value":[]}}]}`

func TestP15ExplicitNativeContainerTemplateRemainsOpaque(t *testing.T) {
	// Exact native templates explicitly opt into native.plist. A plain parameter
	// map is the existing semantic convenience route and has different behavior.
	source := `native.action("com.example.p15.container-template",parameters:native.plist(` + p15OpaqueContainerTemplate + `))`
	actions := p15CompiledActions(t, source)
	want := map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": false, "WFValue": []any{uint64(0), false, "", []any{}, map[string]any{}, []any{uint64(1), uint64(2)}}, "WFDictionaryFieldValueItems": []any{map[string]any{"WFKey": "opaque", "WFValue": map[string]any{"Value": []any{}}}}}
	if len(actions) != 1 || actions[0]["WFWorkflowActionIdentifier"] != "com.example.p15.container-template" || !reflect.DeepEqual(p15Param(actions[0]), want) {
		t.Fatalf("exact native template was encoded as semantic containers: %#v", actions)
	}
}

func TestP15FunctionContainerOracleRejectsNativeConfusion(t *testing.T) {
	want := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": false, "items": []any{p15ContainerNumber("0"), "", []any{}, map[string]any{}}}
	source := `function echo(value:Map<Text,AnyContent>)->Map<Text,AnyContent>{return value}
let result=echo({"WFSerializationType":"WFTextTokenAttachment","Value":false,"items":[0,"",[],{}]})
getVariable(result)`
	original := p15CompiledActions(t, source)
	if err := p15FunctionContainerArgumentError(original, want); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"native_token", "missing_field", "missing_empty_array", "missing_empty_dictionary", "reordered_items", "false_to_zero", "dropped_presence", "extra_wrapper", "compatibility_flatten"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15ContainerCloneNative(t, original)
			fields, err := p15FunctionContainerPayload(changed)
			if err != nil {
				t.Fatal(err)
			}
			argument := fields["arg_0"]
			body := argument["WFValue"].(map[string]any)
			inner := body["Value"].(map[string]any)
			rows := inner["WFDictionaryFieldValueItems"].([]any)
			nested, err := p15ContainerFields(body)
			if err != nil {
				t.Fatal(err)
			}
			items := nested["items"]["WFValue"].(map[string]any)["Value"].([]any)
			switch fault {
			case "native_token":
				argument["WFItemType"] = uint64(0)
				argument["WFValue"] = map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": false}
			case "missing_field":
				inner["WFDictionaryFieldValueItems"] = rows[:len(rows)-1]
			case "missing_empty_array":
				delete(items[2].(map[string]any)["WFValue"].(map[string]any), "Value")
			case "missing_empty_dictionary":
				delete(items[3].(map[string]any)["WFValue"].(map[string]any)["Value"].(map[string]any), "WFDictionaryFieldValueItems")
			case "reordered_items":
				items[0], items[1] = items[1], items[0]
			case "false_to_zero":
				nested["Value"]["WFItemType"] = uint64(3)
				nested["Value"]["WFValue"] = p15ContainerText("0")
			case "dropped_presence":
				fields["has_arg_0"]["WFValue"] = p15ContainerText("0")
			case "extra_wrapper":
				argument["WFValue"] = map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": body}
			case "compatibility_flatten":
				fields["arguments"]["WFValue"].(map[string]any)["Value"] = items
			}
			if err := p15FunctionContainerArgumentError(changed, want); err == nil {
				t.Fatal("container oracle accepted " + fault)
			}
		})
	}
}

func p15ContainerCloneNative(t *testing.T, original []map[string]any) []map[string]any {
	t.Helper()
	// Reuse the repository's strictly typed native clone, never normalize bools,
	// integers, empty collections or wrapper depth to make a comparison pass.
	var clone []map[string]any
	for _, action := range original {
		clone = append(clone, deepClonePlistMap(action))
	}
	return clone
}

func TestP15FunctionContainerUnsupportedNullStillFailsPrecisely(t *testing.T) {
	source := `function echo(value:Map<Text,AnyContent>)->Map<Text,AnyContent>{return value};echo({"value":null})`
	raw, err := CompileSourceToPlist("p15-container-null.cherri", source)
	if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), p15T05NullCode) {
		t.Fatalf("unsupported JSON null was silently fabricated or dropped: %v", err)
	}
}

func TestP15FunctionMapArgumentProducerOrderAndMultiplicity(t *testing.T) {
	source := `function echo(value:Map<Text,AnyContent>,later:Text)->Map<Text,AnyContent>{return value}
let result=echo({"producer":text("P15_MAP_FIRST")},later:text("P15_MAP_SECOND"))
getVariable(result)`
	actions := p15CompiledActions(t, source)
	positions := map[string]int{}
	identities := map[string]p15ContainerReference{}
	for index, action := range actions {
		p := p15Param(action)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
			continue
		}
		value, _ := p["WFTextActionText"].(string)
		if value != "P15_MAP_FIRST" && value != "P15_MAP_SECOND" {
			continue
		}
		if _, duplicate := positions[value]; duplicate {
			t.Fatal("function Map argument expression emitted twice", value)
		}
		uuid, _ := p["UUID"].(string)
		name, _ := p["CustomOutputName"].(string)
		if uuid == "" {
			t.Fatal("argument producer lacks identity")
		}
		positions[value] = index
		identities[value] = p15ContainerReference{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": name}
	}
	if len(positions) != 2 || positions["P15_MAP_FIRST"] >= positions["P15_MAP_SECOND"] {
		t.Fatal("function aggregate arguments evaluated out of source order")
	}
	fields, err := p15FunctionContainerPayload(actions)
	if err != nil {
		t.Fatal(err)
	}
	_, wantMap := p15ContainerFieldValue(map[string]any{"producer": identities["P15_MAP_FIRST"]})
	_, wantLater := p15ContainerFieldValue(identities["P15_MAP_SECOND"])
	if fields["arg_0"]["WFItemType"] != uint64(1) || !reflect.DeepEqual(fields["arg_0"]["WFValue"], wantMap) || !reflect.DeepEqual(fields["arg_1"]["WFValue"], wantLater) {
		t.Fatal("function argument envelope detached from once-only source-order producers")
	}
}
