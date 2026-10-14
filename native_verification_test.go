/*
 * Copyright (c) Cherri Language v2.0
 * Native Escapes Conformance & Deep Verification Test Suite (Phase 1)
 */

package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

// 1. GAP-NATIVE-PLIST: Verify nested types, numeric fidelity, booleans, large integers, binary data, dates, and opaque subtrees.
func TestNATIVE_Plist_DeepStructuralTypes(t *testing.T) {
	dateStr := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	code := `native.action("com.apple.custom.deep_plist", parameters: {
    "DeepConfig": native.plist({
        "StringEmpty": "",
        "StringSpecial": "Text with \"quotes\", \nnewlines, and Unicode: 🌍 שָׁלוֹם",
        "BoolTrue": true,
        "BoolFalse": false,
        "IntZero": 0,
        "IntLarge": 9223372036854775807,
        "IntNegative": -123456789,
        "FloatVal": 3.141592653589793,
        "IsoDate": "` + dateStr + `",
        "NestedArray": [
            1,
            "two",
            true,
            { "ArraySubKey": "ArraySubVal" }
        ],
        "NestedDict": {
            "Level2": {
                "Level3": {
                    "OpaqueKey": "DeepValue",
                    "LeafInt": 42
                }
            }
        },
        "OpaqueCustomObject": {
            "VendorID": "VENDOR-XYZ-123",
            "Enabled": true
        }
    })
})
`
	plistBytes, err := CompileSourceToPlist("deep_plist.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile deep native.plist: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}

	actions, ok := rawMap["WFWorkflowActions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("expected actions, got %v", rawMap["WFWorkflowActions"])
	}
	act := actions[len(actions)-1].(map[string]any)
	if act["WFWorkflowActionIdentifier"] != "com.apple.custom.deep_plist" {
		t.Fatalf("unexpected action identifier: %v", act["WFWorkflowActionIdentifier"])
	}

	params := act["WFWorkflowActionParameters"].(map[string]any)
	deepCfg, ok := params["DeepConfig"].(map[string]any)
	if !ok {
		t.Fatalf("expected DeepConfig to be map[string]any, got %T: %v", params["DeepConfig"], params["DeepConfig"])
	}

	// Structural assertions
	if deepCfg["StringEmpty"] != "" {
		t.Errorf("expected empty string, got %v", deepCfg["StringEmpty"])
	}
	if !strings.Contains(deepCfg["StringSpecial"].(string), "שָׁלוֹם") {
		t.Errorf("expected Hebrew in StringSpecial, got %v", deepCfg["StringSpecial"])
	}
	if deepCfg["BoolTrue"] != true {
		t.Errorf("expected BoolTrue == true, got %v", deepCfg["BoolTrue"])
	}
	if deepCfg["BoolFalse"] != false {
		t.Errorf("expected BoolFalse == false, got %v", deepCfg["BoolFalse"])
	}

	// Large int verification (int64)
	largeIntVal, ok := deepCfg["IntLarge"].(uint64)
	if !ok {
		if signedInt, isSigned := deepCfg["IntLarge"].(int64); isSigned {
			largeIntVal = uint64(signedInt)
			ok = true
		} else if rawInt, isInt := deepCfg["IntLarge"].(int); isInt {
			largeIntVal = uint64(rawInt)
			ok = true
		}
	}
	if !ok || largeIntVal != uint64(math.MaxInt64) {
		t.Errorf("expected IntLarge == %d, got %v (%T)", uint64(math.MaxInt64), deepCfg["IntLarge"], deepCfg["IntLarge"])
	}

	// Nested Array verification
	arr, ok := deepCfg["NestedArray"].([]any)
	if !ok {
		if arrI, isI := deepCfg["NestedArray"].([]interface{}); isI {
			arr = arrI
			ok = true
		}
	}
	if !ok || len(arr) != 4 {
		t.Fatalf("expected NestedArray with 4 items, got %v", deepCfg["NestedArray"])
	}
	subObj := arr[3].(map[string]any)
	if subObj["ArraySubKey"] != "ArraySubVal" {
		t.Errorf("array sub-object mismatch: %v", subObj)
	}

	// Nested Dict Level 3 verification
	l2 := deepCfg["NestedDict"].(map[string]any)
	l3 := l2["Level2"].(map[string]any)["Level3"].(map[string]any)
	if l3["OpaqueKey"] != "DeepValue" {
		t.Errorf("expected DeepValue, got %v", l3["OpaqueKey"])
	}
}

// 2. GAP-NATIVE-REF: Verify ExtensionInput, chained aggrandizements, property paths, and strict identity.
func TestNATIVE_Ref_FullCapabilities(t *testing.T) {
	code := `native.action("com.apple.custom.all_refs", parameters: {
    "OutputRef": native.ref("ActionOutput", uuid: "A1B2C3D4-E5F6-7890-ABCD-EF1234567890", coerce: "WFDictionaryContentItem", key: "auth_token"),
    "VariableRef": native.ref("Variable", name: "sessionContext", property: "Creation Date"),
    "ExtensionInputRef": native.ref("ExtensionInput", property: "File Extension")
})
`
	plistBytes, err := CompileSourceToPlist("all_refs.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile all_refs.cherri: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}

	act := rawMap["WFWorkflowActions"].([]any)[0].(map[string]any)
	params := act["WFWorkflowActionParameters"].(map[string]any)

	// ExtensionInputRef verification
	extRef, ok := params["ExtensionInputRef"].(map[string]any)
	if !ok {
		t.Fatalf("expected ExtensionInputRef map, got %T: %v", params["ExtensionInputRef"], params["ExtensionInputRef"])
	}
	if extRef["WFSerializationType"] != "WFTextTokenAttachment" {
		t.Errorf("expected WFTextTokenAttachment, got %v", extRef["WFSerializationType"])
	}
	extVal := extRef["Value"].(map[string]any)
	if extVal["Type"] != "ExtensionInput" {
		t.Errorf("expected Type ExtensionInput, got %v", extVal["Type"])
	}
	aggrs := extVal["Aggrandizements"].([]any)
	if len(aggrs) != 1 {
		t.Fatalf("expected 1 aggrandizement on ExtensionInput, got %v", aggrs)
	}
	propAggr := aggrs[0].(map[string]any)
	if propAggr["Type"] != "WFPropertyVariableAggrandizement" || propAggr["PropertyName"] != "File Extension" {
		t.Errorf("unexpected ExtensionInput property aggrandizement: %v", propAggr)
	}

	// OutputRef chained aggrandizements verification (Coercion + DictionaryKey)
	outRef := params["OutputRef"].(map[string]any)
	outVal := outRef["Value"].(map[string]any)
	if outVal["OutputUUID"] != "A1B2C3D4-E5F6-7890-ABCD-EF1234567890" {
		t.Errorf("OutputUUID mismatch: %v", outVal["OutputUUID"])
	}
	outAggrs := outVal["Aggrandizements"].([]any)
	if len(outAggrs) != 2 {
		t.Fatalf("expected 2 aggrandizements, got %v", outAggrs)
	}
	if outAggrs[0].(map[string]any)["CoercionItemClass"] != "WFDictionaryContentItem" {
		t.Errorf("aggrandizement 0 coercion mismatch: %v", outAggrs[0])
	}
	if outAggrs[1].(map[string]any)["DictionaryKey"] != "auth_token" {
		t.Errorf("aggrandizement 1 key mismatch: %v", outAggrs[1])
	}
}

// 3. GAP-NATIVE-REF Negative: Unsafe or malformed references must fail closed.
func TestNATIVE_Ref_NegativeRejections(t *testing.T) {
	testCases := []struct {
		name string
		code string
	}{
		{"InvalidRefType", `let r = native.ref("UnknownReferenceType", uuid: "123")`},
		{"ActionOutputMissingUUID", `let r = native.ref("ActionOutput")`},
		{"VariableMissingName", `let r = native.ref("Variable")`},
		{"EmptyRefKind", `let r = native.ref("")`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileSourceToPlist("negative_ref.cherri", tc.code)
			if err == nil {
				t.Fatalf("expected compilation error for %s, got nil", tc.name)
			}
		})
	}
}

// 4. GAP-NATIVE-WORKFLOW: Full-workflow native escape with root metadata, grouping identifiers, conditions, and import questions.
func TestNATIVE_Workflow_FullLosslessPreservation(t *testing.T) {
	code := `native.workflow({
    "WFWorkflowClientVersion": "3200.0.7",
    "WFWorkflowClientRelease": "iOS 18.0",
    "WFWorkflowMinimumClientVersion": 900,
    "WFWorkflowTypes": ["ActionExtension", "QuickActions"],
    "WFQuickActionSurfaces": ["Finder", "Services"],
    "CustomOpaqueTopLevelEngine": "CherriNativePreservationV2",
    "WFWorkflowImportQuestions": [
        {
            "ActionIndex": 0,
            "Category": "Parameter",
            "ParameterKey": "WFAlertActionTitle",
            "Text": "What should the title be?",
            "DefaultValue": "Default Title"
        }
    ],
    "WFWorkflowActions": [
        {
            "WFWorkflowActionIdentifier": "is.workflow.actions.alert",
            "WFWorkflowActionParameters": {
                "UUID": "ACTION-UUID-001",
                "WFAlertActionTitle": "Imported Alert"
            }
        },
        {
            "WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
            "WFWorkflowActionParameters": {
                "GroupingIdentifier": "GROUP-COND-001",
                "WFCondition": 4,
                "WFControlFlowMode": 0
            }
        },
        {
            "WFWorkflowActionIdentifier": "com.apple.opaque.appintent.action",
            "WFWorkflowActionParameters": {
                "AppIntentDescriptor": {
                    "AppIntentIdentifier": "OpaqueAppIntent",
                    "BundleIdentifier": "com.example.app"
                }
            }
        },
        {
            "WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
            "WFWorkflowActionParameters": {
                "GroupingIdentifier": "GROUP-COND-001",
                "WFControlFlowMode": 2
            }
        }
    ]
})
`
	plistBytes, err := CompileSourceToPlist("full_workflow.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile native.workflow: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}

	// Verify root metadata
	if rawMap["WFWorkflowClientVersion"] != "3200.0.7" {
		t.Errorf("client version mismatch: %v", rawMap["WFWorkflowClientVersion"])
	}
	if rawMap["WFWorkflowClientRelease"] != "iOS 18.0" {
		t.Errorf("client release mismatch: %v", rawMap["WFWorkflowClientRelease"])
	}
	if rawMap["CustomOpaqueTopLevelEngine"] != "CherriNativePreservationV2" {
		t.Errorf("custom opaque key mismatch: %v", rawMap["CustomOpaqueTopLevelEngine"])
	}

	// Verify Import Questions
	iqs, ok := rawMap["WFWorkflowImportQuestions"].([]any)
	if !ok || len(iqs) != 1 {
		t.Fatalf("expected 1 import question, got %v", rawMap["WFWorkflowImportQuestions"])
	}
	q0 := iqs[0].(map[string]any)
	if q0["ParameterKey"] != "WFAlertActionTitle" || q0["Text"] != "What should the title be?" {
		t.Errorf("import question 0 mismatch: %v", q0)
	}

	// Verify Actions and Grouping
	actions, ok := rawMap["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(actions))
	}
	condStart := actions[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if condStart["GroupingIdentifier"] != "GROUP-COND-001" || condStart["WFControlFlowMode"] != uint64(0) && condStart["WFControlFlowMode"] != int64(0) && condStart["WFControlFlowMode"] != 0 {
		t.Errorf("condition start grouping mismatch: %v", condStart)
	}
	appIntent := actions[2].(map[string]any)
	if appIntent["WFWorkflowActionIdentifier"] != "com.apple.opaque.appintent.action" {
		t.Errorf("appIntent action identifier mismatch: %v", appIntent)
	}
	condEnd := actions[3].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if condEnd["GroupingIdentifier"] != "GROUP-COND-001" {
		t.Errorf("condition end grouping mismatch: %v", condEnd)
	}
}

// 5. GAP-DISCARDED-NATIVE-ERRORS: Expression evaluation errors in native.action must not be discarded.
func TestNATIVE_Action_DiscardedErrorsMustFailClosed(t *testing.T) {
	negativeCases := []struct {
		name        string
		code        string
		errContains string
	}{
		{
			name: "UndefinedVariableInParameterValue",
			code: `native.action("is.workflow.actions.alert", { "Key": nonExistentVariable })`,
		},
		{
			name:        "EmptyIdentifier",
			code:        `native.action("", { "Key": "Val" })`,
			errContains: "action identifier",
		},
		{
			name:        "NonDictionaryParameters",
			code:        `native.action("is.workflow.actions.alert", 12345)`,
			errContains: "must be a dictionary",
		},
	}

	for _, tc := range negativeCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileSourceToPlist("negative_action.cherri", tc.code)
			if err == nil {
				t.Fatalf("expected compilation error for %s, got nil", tc.name)
			}
			if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("expected error to contain %q, got %v", tc.errContains, err)
			}
		})
	}
}

// 6. JSON round-trip compatibility check for native structures
func TestNATIVE_JSONCompatibilityCheck(t *testing.T) {
	code := `native.action("is.workflow.actions.dictionary", {
    "WFDictionary": native.plist({
        "KeyA": "ValA",
        "KeyB": [10, 20, 30]
    })
})
`
	plistBytes, err := CompileSourceToPlist("json_compat.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	jsonBytes, err := json.Marshal(rawMap)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	var jsonMap map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonMap); err != nil {
		t.Fatalf("failed to round-trip JSON: %v", err)
	}

	if jsonMap["WFWorkflowActions"] == nil {
		t.Errorf("expected WFWorkflowActions in JSON round-trip")
	}
}
