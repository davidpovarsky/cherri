package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"howett.net/plist"
)

// These expectations describe Dictionary row serialization, independently of
// the production encoder. The macOS 26.7.2 observation at 6f8ec881 demonstrates
// that an omitted Value raises a nil-insertion exception, whereas an explicit
// [] survives. Direct WFArrayParameterState construction is a different input
// boundary: these tests retain the full Dictionary row wrapper. They grant no
// iOS 27 execution or function runtime acceptance.
func p15FAWire(items []any) map[string]any {
	if items == nil {
		items = []any{}
	}
	return map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": items}
}

func p15FARow(kind uint64, value any) map[string]any {
	return map[string]any{"WFItemType": kind, "WFValue": value}
}

func p15FAArrayError(value any, expected []any) error {
	wire, ok := value.(map[string]any)
	if !ok || len(wire) != 2 || wire["WFSerializationType"] != "WFArrayParameterState" {
		return fmt.Errorf("Dictionary array requires the exact wrapper and explicit Value: %#v", value)
	}
	rows, ok := wire["Value"].([]any)
	if !ok {
		return fmt.Errorf("Dictionary array Value must be an explicit array, including when empty: %#v", wire)
	}
	if expected == nil {
		expected = []any{}
	}
	if !reflect.DeepEqual(p15FunctionComparable(rows, false), p15FunctionComparable(expected, false)) {
		return fmt.Errorf("Dictionary array rows changed: got %#v, want %#v", rows, expected)
	}
	return nil
}

func p15FAEveryArray(value any) (int, error) {
	count := 0
	switch value := value.(type) {
	case map[string]any:
		if value["WFSerializationType"] == "WFArrayParameterState" {
			rows, okay := value["Value"].([]any)
			if !okay {
				return 0, fmt.Errorf("generated Dictionary array omitted explicit Value: %#v", value)
			}
			if err := p15FAArrayError(value, rows); err != nil {
				return 0, err
			}
			count++
		}
		for _, item := range value {
			found, err := p15FAEveryArray(item)
			if err != nil {
				return 0, err
			}
			count += found
		}
	case []any:
		for _, item := range value {
			found, err := p15FAEveryArray(item)
			if err != nil {
				return 0, err
			}
			count += found
		}
	}
	return count, nil
}

func p15FACompile(t *testing.T, source string, legacy bool, profile BuildProfile) map[string]any {
	t.Helper()
	if legacy {
		root, err := compileLegacySourceToRawMapInMemoryWithProfile(source, profile)
		if err != nil {
			t.Fatalf("Legacy source failed before native array observation: %v", err)
		}
		return root
	}
	raw, err := CompileSourceToPlistWithOptions("function-array-native.cherri", source, CompileOptions{BuildProfile: profile})
	if err != nil {
		t.Fatalf("V2 source failed before native array observation: %v", err)
	}
	root, err := decodeShortcutRawMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func p15FAPayload(t *testing.T, root map[string]any) (map[string]map[string]any, int, []any) {
	t.Helper()
	actions, okay := root["WFWorkflowActions"].([]any)
	if !okay {
		t.Fatal("actual compiler did not emit an action list")
	}
	var fields map[string]map[string]any
	index := -1
	for i, raw := range actions {
		action := raw.(map[string]any)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			continue
		}
		candidate, err := p15ContainerFields(p15SCParams(action)["WFItems"])
		if err != nil || candidate["function"] == nil || candidate["arguments"] == nil || candidate["cherri_functions"] == nil {
			continue
		}
		if err := p15CheckLiteralText(candidate["function"]["WFValue"], "arrayProbe"); err != nil {
			continue
		}
		if index >= 0 {
			t.Fatal("duplicated actual function sender")
		}
		fields, index = candidate, i
	}
	if index < 0 {
		t.Fatal("actual function sender missing")
	}
	payload := p15SCParams(actions[index].(map[string]any))["UUID"]
	uses := 0
	for _, raw := range actions[index+1:] {
		action := raw.(map[string]any)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
			ref, valid := p15FERef(p15SCParams(action)["WFInput"])
			if valid && ref["Type"] == "ActionOutput" && ref["OutputUUID"] == payload {
				uses++
			}
		}
	}
	if payload == "" || payload == nil || uses != 1 {
		t.Fatalf("sender does not feed exactly one actual run-workflow consumer: %v/%d", payload, uses)
	}
	return fields, index, actions
}

func TestP15FunctionArrayNativeSender(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			for _, name := range []string{"zero", "text", "empty_list", "text_list"} {
				t.Run(fmt.Sprintf("%t_%s_%s", legacy, profile, name), func(t *testing.T) {
					parameter, argument, body := "", "", `return "OK"`
					var expected []any
					if legacy {
						body = `output("OK")`
					}
					if name == "text" {
						parameter, argument = "value:Text", `"TEXT"`
						if legacy {
							parameter = "text value"
						}
						expected = []any{p15FARow(0, p15ContainerText("TEXT"))}
					}
					if name == "empty_list" || name == "text_list" {
						parameter, argument = "value:List<Text>", `[]`
						items := []any{}
						if name == "text_list" {
							argument = `["","TEXT"]`
							items = []any{p15FARow(0, p15ContainerText("")), p15FARow(0, p15ContainerText("TEXT"))}
						}
						if legacy {
							parameter = "array value"
							field := p15FARow(2, p15FAWire(items))
							field["WFKey"] = p15ContainerText("array")
							dictionary := map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{field}}}
							expected = []any{p15FARow(1, map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": dictionary})}
						} else {
							expected = []any{p15FARow(2, p15FAWire(items))}
						}
					}
					resultType := "->Text"
					prefix := ""
					if legacy {
						resultType, prefix = "", "#include 'actions/text'\n"
					}
					source := prefix + "function arrayProbe(" + parameter + ")" + resultType + " {\n" + body + "\n}\narrayProbe(" + argument + ")\n"
					root := p15FACompile(t, source, legacy, profile)
					fields, _, _ := p15FAPayload(t, root)
					if p15Int(fields["arguments"]["WFItemType"]) != 2 {
						t.Fatal("positional mirror lost its array type")
					}
					if err := p15FAArrayError(fields["arguments"]["WFValue"], expected); err != nil {
						t.Fatal(err)
					}
					if count, err := p15FAEveryArray(root); err != nil || count == 0 {
						t.Fatalf("generated native arrays: count=%d error=%v", count, err)
					}
				})
			}
		}
	}
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		for _, present := range []bool{false, true} {
			t.Run(fmt.Sprintf("optional_%s_%t", profile, present), func(t *testing.T) {
				argument := "none"
				if present {
					argument = "[]"
				}
				source := "function arrayProbe(value:List<AnyContent>?)->Text{return \"OK\"}\narrayProbe(" + argument + ")\n"
				root := p15FACompile(t, source, false, profile)
				fields, payloadIndex, actions := p15FAPayload(t, root)
				ref, okay := p15FERef(fields["arg_0"]["WFValue"])
				if !okay || ref["Type"] != "ActionOutput" {
					t.Fatal("Optional argument must keep its actual owned carrier edge")
				}
				want := map[string]any{"_cherri_optional_argument_v1": "1", "has_value": "0"}
				if present {
					want["has_value"], want["value"] = "1", []any{}
				}
				_, wantedCarrier := p15ContainerFieldValue(want)
				found := 0
				for _, raw := range actions[:payloadIndex] {
					action := raw.(map[string]any)
					p := p15SCParams(action)
					if p["UUID"] == ref["OutputUUID"] {
						if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" || !reflect.DeepEqual(p15FunctionComparable(p["WFItems"], false), p15FunctionComparable(wantedCarrier, false)) {
							t.Fatal("Optional carrier presence/typed empty array changed")
						}
						found++
					}
				}
				if found != 1 {
					t.Fatal("Optional reference does not select exactly one prior carrier")
				}
				if err := p15CheckLiteralText(fields["has_arg_0"]["WFValue"], "1"); err != nil {
					t.Fatal(err)
				}
				if err := p15FAArrayError(fields["arguments"]["WFValue"], []any{p15FARow(0, fields["arg_0"]["WFValue"])}); err != nil {
					t.Fatal(err)
				}
				if _, err := p15FAEveryArray(root); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15GeneratedArrayValueSerialization(t *testing.T) {
	ref := map[string]any{"Type": "ActionOutput", "OutputUUID": "11111111-2222-4333-8444-555555555555"}
	for _, format := range []int{plist.XMLFormat, plist.BinaryFormat} {
		for _, name := range []string{"nil", "empty", "nested_empty", "empty_text", "ordered_scalars", "reference", "typed_empty"} {
			t.Run(fmt.Sprintf("%d_%s", format, name), func(t *testing.T) {
				var rows []WFDictionaryFieldValueItem
				var expected []any
				if name == "empty" {
					rows = []WFDictionaryFieldValueItem{}
				}
				if name == "nested_empty" {
					rows = []WFDictionaryFieldValueItem{{WFItemType: 2, WFValue: makeArrayValue(nil)}}
					expected = []any{p15FARow(2, p15FAWire(nil))}
				}
				if name == "empty_text" || name == "ordered_scalars" {
					rows = []WFDictionaryFieldValueItem{{WFItemType: 0, WFValue: p15ContainerText("")}}
					expected = []any{p15FARow(0, p15ContainerText(""))}
				}
				if name == "ordered_scalars" {
					rows = append(rows, WFDictionaryFieldValueItem{WFItemType: 4, WFValue: WFBoolValue{WFSerializationType: "WFNumberSubstitutableState", Value: false}}, WFDictionaryFieldValueItem{WFItemType: 3, WFValue: p15ContainerText("0")})
					expected = append(expected, p15FARow(4, map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": false}), p15FARow(3, p15ContainerText("0")))
				}
				if name == "reference" {
					wire := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": ref}
					rows = []WFDictionaryFieldValueItem{{WFItemType: 0, WFValue: wire}}
					expected = []any{p15FARow(0, wire)}
				}
				var input any = makeArrayValue(rows)
				if name == "typed_empty" {
					input = encodeTypedLiteralParam(backend.SemanticValue{Kind: backend.ValList})
				}
				raw, err := plist.Marshal(map[string]any{"array": input}, format)
				if err != nil {
					t.Fatal(err)
				}
				var decoded map[string]any
				if _, err := plist.Unmarshal(raw, &decoded); err != nil {
					t.Fatal(err)
				}
				if err := p15FAArrayError(decoded["array"], expected); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15FunctionArrayNativeImportFidelity(t *testing.T) {
	// These opaque source bytes intentionally include the malformed historical
	// shape. Import fidelity preserves it; the authored encoder must not repair
	// arbitrary raw payloads or relabel the original b82 artifact.
	root := map[string]any{"WFWorkflowActions": []any{p15AcceptanceAction("com.cherri.public.array-fidelity", map[string]any{
		"Missing": map[string]any{"WFSerializationType": "WFArrayParameterState"},
		"Empty":   p15FAWire(nil), "Text": p15FAWire([]any{p15FARow(0, p15ContainerText(""))}),
		"Plain": []any{},
	})}}
	p15NativeImportRoundTrip(t, "array-empty-shape", root, plist.XMLFormat, plist.BinaryFormat)
}

func TestP15FunctionArrayNativeOracleFaults(t *testing.T) {
	for _, fault := range []string{"missing_value", "nil_value", "dictionary_value", "string_value", "wrong_wrapper", "extra_key", "unwrapped", "drop_item", "change_type", "reorder", "false_to_true", "change_reference"} {
		t.Run(fault, func(t *testing.T) {
			expected := []any{p15FARow(4, map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": false}), p15FARow(0, map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": "11111111-2222-4333-8444-555555555555"}})}
			original := p15FAWire(expected)
			if err := p15FAArrayError(original, expected); err != nil {
				t.Fatal(err)
			}
			bad := p15LiteralClone(t, original).(map[string]any)
			var value any = bad
			switch fault {
			case "missing_value":
				delete(bad, "Value")
			case "nil_value":
				bad["Value"] = nil
			case "dictionary_value":
				bad["Value"] = map[string]any{}
			case "string_value":
				bad["Value"] = ""
			case "wrong_wrapper":
				bad["WFSerializationType"] = "WFDictionaryFieldValue"
			case "extra_key":
				bad["unmodeled"] = true
			case "unwrapped":
				value = bad["Value"]
			case "drop_item":
				bad["Value"] = bad["Value"].([]any)[:1]
			case "change_type":
				bad["Value"].([]any)[0].(map[string]any)["WFItemType"] = 0
			case "reorder":
				rows := bad["Value"].([]any)
				rows[0], rows[1] = rows[1], rows[0]
			case "false_to_true":
				bad["Value"].([]any)[0].(map[string]any)["WFValue"].(map[string]any)["Value"] = true
			case "change_reference":
				bad["Value"].([]any)[1].(map[string]any)["WFValue"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE"
			}
			if err := p15FAArrayError(value, expected); err == nil {
				t.Fatal("array oracle accepted a causal corruption")
			}
		})
	}
}
