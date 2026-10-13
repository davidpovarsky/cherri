/*
 * Copyright (c) Cherri Language v2.0
 * Native Escapes Conformance Test Suite (Batches 1 & 2)
 */

package main

import (
	"strings"
	"testing"

	"howett.net/plist"
)

// 1. native.action with empty identifier fails closed
func TestNATIVE_EmptyIdentifierFails(t *testing.T) {
	code := `native.action("", {})`
	_, err := CompileSourceToPlist("empty_id.cherri", code)
	if err == nil {
		t.Fatalf("expected error for empty action identifier, got nil")
	}
	if !strings.Contains(err.Error(), "non-empty action identifier") && !strings.Contains(err.Error(), "action identifier must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 2. native.action with non-dictionary parameters fails closed
func TestNATIVE_ParametersNotDictionaryFails(t *testing.T) {
	code := `native.action("is.workflow.actions.alert", "not_a_dictionary")`
	_, err := CompileSourceToPlist("bad_params.cherri", code)
	if err == nil {
		t.Fatalf("expected error when parameters is not a dictionary, got nil")
	}
	if !strings.Contains(err.Error(), "must be a dictionary") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 3. native.action with undefined variable in parameters fails closed
func TestNATIVE_InvalidExpressionInParametersFails(t *testing.T) {
	code := `native.action("is.workflow.actions.alert", { "Key": undefinedVariable })`
	_, err := CompileSourceToPlist("bad_expr.cherri", code)
	if err == nil {
		t.Fatalf("expected semantic error for undefined variable in parameters, got nil")
	}
}

// 4. native.action valid emission with custom parameters
func TestNATIVE_ValidNativeActionEmission(t *testing.T) {
	code := `let alert = native.action("is.workflow.actions.alert", {
    "WFAlertActionMessage": "Hello from Native",
    "WFAlertActionTitle": "Custom Native Title"
})
`
	plistBytes, err := CompileSourceToPlist("valid_native.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile valid native.action: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}

	if len(sc.WFWorkflowActions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(sc.WFWorkflowActions))
	}

	act := sc.WFWorkflowActions[0]
	if act.WFWorkflowActionIdentifier != "is.workflow.actions.alert" {
		t.Errorf("expected identifier is.workflow.actions.alert, got %s", act.WFWorkflowActionIdentifier)
	}
	if act.WFWorkflowActionParameters["WFAlertActionMessage"] != "Hello from Native" {
		t.Errorf("expected WFAlertActionMessage 'Hello from Native', got %v", act.WFWorkflowActionParameters["WFAlertActionMessage"])
	}
	if act.WFWorkflowActionParameters["WFAlertActionTitle"] != "Custom Native Title" {
		t.Errorf("expected WFAlertActionTitle 'Custom Native Title', got %v", act.WFWorkflowActionParameters["WFAlertActionTitle"])
	}
	if act.WFWorkflowActionParameters["UUID"] == "" {
		t.Errorf("expected non-empty UUID for bound native.action")
	}
}

// 5. native.plist typed dictionary pass-through
func TestNATIVE_PlistPassThrough(t *testing.T) {
	code := `native.action("com.apple.custom.data", parameters: {
    "Payload": native.plist({
        "CustomBool": true,
        "CustomInt": 42,
        "CustomFloat": 3.14,
        "CustomList": [1, 2, "three"],
        "Nested": {
            "InnerKey": "InnerValue"
        }
    })
})
`
	plistBytes, err := CompileSourceToPlist("native_plist.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile native.plist: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	if len(sc.WFWorkflowActions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(sc.WFWorkflowActions))
	}

	act := sc.WFWorkflowActions[0]
	if act.WFWorkflowActionIdentifier != "com.apple.custom.data" {
		t.Errorf("expected com.apple.custom.data, got %s", act.WFWorkflowActionIdentifier)
	}

	payload, ok := act.WFWorkflowActionParameters["Payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for Payload, got %T: %v", act.WFWorkflowActionParameters["Payload"], act.WFWorkflowActionParameters["Payload"])
	}

	if payload["CustomBool"] != true {
		t.Errorf("expected CustomBool true, got %v (%T)", payload["CustomBool"], payload["CustomBool"])
	}
	if nested, ok := payload["Nested"].(map[string]any); !ok || nested["InnerKey"] != "InnerValue" {
		t.Errorf("expected nested InnerKey 'InnerValue', got %v", payload["Nested"])
	}
}

// 6. native.ref with action output, variable, and chained aggrandizements
func TestNATIVE_RefAggrandizements(t *testing.T) {
	code := `native.action("com.apple.custom.ref", parameters: {
    "InputDict": native.ref("ActionOutput", uuid: "11111111-2222-3333-4444-555555555555", coerce: "WFDictionaryContentItem", key: "username"),
    "InputProp": native.ref("Variable", name: "myVariable", property: "File Size")
})
`
	plistBytes, err := CompileSourceToPlist("native_ref.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile native.ref: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	if len(sc.WFWorkflowActions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(sc.WFWorkflowActions))
	}

	params := sc.WFWorkflowActions[0].WFWorkflowActionParameters
	inputDict, ok := params["InputDict"].(map[string]any)
	if !ok {
		if m, isM := params["InputDict"].(map[string]interface{}); isM {
			inputDict = m
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected map for InputDict, got %T: %v", params["InputDict"], params["InputDict"])
	}
	if inputDict["WFSerializationType"] != "WFTextTokenAttachment" {
		t.Errorf("expected WFSerializationType WFTextTokenAttachment, got %v", inputDict["WFSerializationType"])
	}
	valMap, ok := inputDict["Value"].(map[string]any)
	if !ok {
		if m, isM := inputDict["Value"].(map[string]interface{}); isM {
			valMap = m
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected map for Value, got %T: %v", inputDict["Value"], inputDict["Value"])
	}
	if valMap["OutputUUID"] != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("expected OutputUUID '11111111-2222-3333-4444-555555555555', got %v", valMap["OutputUUID"])
	}
	aggrs, ok := valMap["Aggrandizements"].([]any)
	if !ok {
		if s, isS := valMap["Aggrandizements"].([]interface{}); isS {
			aggrs = s
			ok = true
		}
	}
	if !ok || len(aggrs) != 2 {
		t.Fatalf("expected 2 aggrandizements, got %v", valMap["Aggrandizements"])
	}
	a0, _ := aggrs[0].(map[string]any)
	if a0 == nil {
		if m, isM := aggrs[0].(map[string]interface{}); isM {
			a0 = m
		}
	}
	if a0["Type"] != "WFCoercionVariableAggrandizement" || a0["CoercionItemClass"] != "WFDictionaryContentItem" {
		t.Errorf("aggrandizement 0 mismatch: %v", a0)
	}
	a1, _ := aggrs[1].(map[string]any)
	if a1 == nil {
		if m, isM := aggrs[1].(map[string]interface{}); isM {
			a1 = m
		}
	}
	if a1["Type"] != "WFDictionaryValueVariableAggrandizement" || a1["DictionaryKey"] != "username" {
		t.Errorf("aggrandizement 1 mismatch: %v", a1)
	}

	inputProp, ok := params["InputProp"].(map[string]any)
	if !ok {
		if m, isM := params["InputProp"].(map[string]interface{}); isM {
			inputProp = m
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected map for InputProp, got %T: %v", params["InputProp"], params["InputProp"])
	}
	if inputProp["WFSerializationType"] != "WFTextTokenAttachment" {
		t.Errorf("expected WFSerializationType WFTextTokenAttachment, got %v", inputProp["WFSerializationType"])
	}
	propVal, _ := inputProp["Value"].(map[string]any)
	if propVal == nil {
		if m, isM := inputProp["Value"].(map[string]interface{}); isM {
			propVal = m
		}
	}
	if propVal["VariableName"] != "myVariable" {
		t.Errorf("expected VariableName 'myVariable', got %v", propVal["VariableName"])
	}
	propAggrs, _ := propVal["Aggrandizements"].([]any)
	if propAggrs == nil {
		if s, isS := propVal["Aggrandizements"].([]interface{}); isS {
			propAggrs = s
		}
	}
	if len(propAggrs) != 1 {
		t.Fatalf("expected 1 aggrandizement on InputProp, got %v", propAggrs)
	}
	pa0, _ := propAggrs[0].(map[string]any)
	if pa0 == nil {
		if m, isM := propAggrs[0].(map[string]interface{}); isM {
			pa0 = m
		}
	}
	if pa0["PropertyName"] != "File Size" {
		t.Errorf("property aggrandizement mismatch: %v", pa0)
	}
}

// 7. native.ref negative tests: missing uuid on ActionOutput, missing name on Variable
func TestNATIVE_RefNegativeValidation(t *testing.T) {
	codeMissingUUID := `let r = native.ref("ActionOutput")`
	if _, err := CompileSourceToPlist("missing_uuid.cherri", codeMissingUUID); err == nil {
		t.Fatalf("expected error for ActionOutput without uuid, got nil")
	}

	codeMissingName := `let r = native.ref("Variable")`
	if _, err := CompileSourceToPlist("missing_name.cherri", codeMissingName); err == nil {
		t.Fatalf("expected error for Variable without name, got nil")
	}
}

// 8. native.workflow full workflow import and unknown metadata preservation
func TestNATIVE_WorkflowPreservation(t *testing.T) {
	code := `native.workflow({
    "WFWorkflowClientVersion": "9999",
    "WFWorkflowCustomUnknownKey": "PreserveThisOpaqueValue",
    "WFWorkflowActions": [
        {
            "WFWorkflowActionIdentifier": "com.apple.custom.native_wf_action",
            "WFWorkflowActionParameters": {
                "Key1": "Val1"
            }
        }
    ]
})
`
	plistBytes, err := CompileSourceToPlist("native_wf.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile native.workflow: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal output plist into map: %v", err)
	}

	if rawMap["WFWorkflowClientVersion"] != "9999" {
		t.Errorf("expected client version 9999, got %v", rawMap["WFWorkflowClientVersion"])
	}
	if rawMap["WFWorkflowCustomUnknownKey"] != "PreserveThisOpaqueValue" {
		t.Errorf("expected unknown key preserved, got %v", rawMap["WFWorkflowCustomUnknownKey"])
	}

	actions, ok := rawMap["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("expected 1 action in WFWorkflowActions, got %v", rawMap["WFWorkflowActions"])
	}
	actMap, ok := actions[0].(map[string]any)
	if !ok || actMap["WFWorkflowActionIdentifier"] != "com.apple.custom.native_wf_action" {
		t.Errorf("action identifier mismatch: %v", actions[0])
	}
}
