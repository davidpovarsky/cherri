package shortcutcompare

import (
	"fmt"
	"reflect"
	"testing"
)

func parityClipAction(identifier string, parameters map[string]any) any {
	return map[string]any{"WFWorkflowActionIdentifier": identifier, "WFWorkflowActionParameters": parameters}
}

func parityClipAttachment(kind, name string) map[string]any {
	var value = map[string]any{"Type": kind}
	if kind == "Variable" {
		value["VariableName"] = name
	}
	if kind == "ActionOutput" {
		value["OutputUUID"] = name
		value["OutputName"] = "captured"
	}
	return map[string]any{"Value": value, "WFSerializationType": "WFTextTokenAttachment"}
}

func parityClipDocument(captured bool) map[string]any {
	var capture any
	var use any
	if captured {
		capture = parityClipAction("is.workflow.actions.getclipboard", map[string]any{"UUID": "snapshot"})
		use = parityClipAttachment("ActionOutput", "snapshot")
	} else {
		capture = parityClipAction("is.workflow.actions.setvariable", map[string]any{"WFVariableName": "captured", "WFInput": parityClipAttachment("Clipboard", "")})
		use = parityClipAttachment("Variable", "captured")
	}
	return map[string]any{"WFWorkflowActions": []any{
		capture,
		parityClipAction("is.workflow.actions.gettext", map[string]any{"UUID": "replacement", "WFTextActionText": "new clipboard"}),
		parityClipAction("is.workflow.actions.setclipboard", map[string]any{"WFInput": parityClipAttachment("ActionOutput", "replacement")}),
		parityClipAction("is.workflow.actions.showresult", map[string]any{"Text": use}),
	}}
}

func TestClipboardSnapshotNormalizationPreservesEvaluation(t *testing.T) {
	var legacy = parityClipDocument(false)
	var captured = parityClipDocument(true)
	var before = deepCloneMap(legacy)
	if result := CompareDocumentsNormalized(legacy, captured); !result.Equal {
		t.Fatalf("equivalent declaration-time Clipboard captures differ: %+v", result.Differences)
	}
	if !reflect.DeepEqual(legacy, before) {
		t.Fatal("normalization mutated its input")
	}
	for _, mutation := range []string{"late_read", "deferred_read", "wrong_reference", "getvariable_system_token", "missing_read", "transformed_capture", "unknown_capture_parameter", "multiple_writes"} {
		t.Run(mutation, func(t *testing.T) {
			var candidate = deepCloneMap(captured)
			var actions = candidate["WFWorkflowActions"].([]any)
			var first = actions[0].(map[string]any)
			var firstParams = first["WFWorkflowActionParameters"].(map[string]any)
			var consumer = actions[3].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
			switch mutation {
			case "late_read":
				actions[0], actions[1], actions[2] = actions[1], actions[2], actions[0]
			case "deferred_read":
				consumer["Text"] = parityClipAttachment("Clipboard", "")
			case "wrong_reference":
				consumer["Text"] = parityClipAttachment("ActionOutput", "replacement")
			case "getvariable_system_token":
				first["WFWorkflowActionIdentifier"] = "is.workflow.actions.getvariable"
				firstParams["WFVariable"] = parityClipAttachment("Clipboard", "")
			case "missing_read":
				candidate["WFWorkflowActions"] = actions[1:]
				consumer["Text"] = parityClipAttachment("Clipboard", "")
			case "transformed_capture", "unknown_capture_parameter", "multiple_writes":
				candidate = deepCloneMap(legacy)
				actions = candidate["WFWorkflowActions"].([]any)
				firstParams = actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
				if mutation == "transformed_capture" {
					firstParams["WFInput"].(map[string]any)["Value"].(map[string]any)["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
				} else if mutation == "unknown_capture_parameter" {
					firstParams["Unmodeled"] = true
				} else {
					candidate["WFWorkflowActions"] = append(actions[:3], append([]any{parityClipAction("is.workflow.actions.setvariable", map[string]any{"WFVariableName": "captured", "WFInput": parityClipAttachment("Clipboard", "")})}, actions[3:]...)...)
				}
			}
			if result := CompareDocumentsNormalized(legacy, candidate); result.Equal {
				t.Fatalf("%s was incorrectly accepted", mutation)
			}
		})
	}
}

func TestBuiltinFunctionSymbolsRemainScoped(t *testing.T) {
	const qualified = "module:builtin:stdfunc::runJS"
	const param = "PARAM-" + qualified + "-script"
	var fn = &CanonicalFunction{Name: qualified, Parameters: []CanonicalParam{{Name: "script", Index: 0}}, UUIDMap: map[string]string{"raw": param},
		Body: []any{parityClipAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": qualified})}}
	var functions = map[string]*CanonicalFunction{qualified: fn}
	normalizeBuiltinFunctionSymbols(functions, nil)
	if functions["runJS"] != fn || fn.UUIDMap["raw"] != "PARAM-runJS-script" {
		t.Fatalf("builtin identity did not rename coherently: %+v", functions)
	}
	if fn.Body[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"] != qualified {
		t.Fatal("ordinary payload text was rewritten")
	}
	for _, foreign := range []string{"module:file:custom::runJS", "module:builtin:other::runJS", "module:builtin:stdfunc::runJS::nested"} {
		var original = &CanonicalFunction{Name: foreign}
		var scoped = map[string]*CanonicalFunction{foreign: original}
		normalizeBuiltinFunctionSymbols(scoped, nil)
		if len(scoped) != 1 || scoped[foreign] != original {
			t.Fatalf("unrelated namespace %s collapsed", foreign)
		}
	}
	var collision = map[string]*CanonicalFunction{qualified: {Name: qualified}, "runJS": {Name: "runJS"}}
	normalizeBuiltinFunctionSymbols(collision, nil)
	if len(collision) != 2 || collision[qualified] == nil || collision["runJS"] == nil {
		t.Fatal("colliding declarations merged")
	}
	var calls = []any{parityClipAction("is.workflow.actions.dictionary", map[string]any{"WFItems": map[string]any{"Value": map[string]any{"WFDictionaryFieldValueItems": []any{
		paritySymbolField("_cherri_magic", "__cherri_fn_envelope_v2__"), paritySymbolField("function", qualified), paritySymbolField("arg_0", qualified),
	}}}})}
	var withCall = map[string]*CanonicalFunction{qualified: {Name: qualified}}
	normalizeBuiltinFunctionSymbols(withCall, calls)
	var items = calls[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFItems"].(map[string]any)["Value"].(map[string]any)["WFDictionaryFieldValueItems"].([]any)
	for i, want := range []string{"__cherri_fn_envelope_v2__", "runJS", qualified} {
		var got = items[i].(map[string]any)["WFValue"].(map[string]any)["Value"].(map[string]any)["string"]
		if got != want {
			t.Fatal(fmt.Sprintf("field %d changed scope or payload: %v", i, got))
		}
	}
}

func paritySymbolField(key, value string) any {
	return map[string]any{"WFKey": map[string]any{"Value": map[string]any{"string": key}},
		"WFValue": map[string]any{"Value": map[string]any{"string": value}}}
}
