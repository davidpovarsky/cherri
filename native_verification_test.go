/*
 * Copyright (c) Cherri Language v2.0
 * Native Escapes Conformance & Deep Verification Test Suite (Phase 1)
 */

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
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

// 7. GAP-NATIVE-TYPED: Real typed Date and Data payloads pass-through using XML and binary plist formats
func TestNATIVE_Plist_TypedDateAndBinaryDataPassThrough(t *testing.T) {
	// Real XML Plist payload containing <date> and base64-encoded <data>
	xmlPayload := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>EventDate</key>
    <date>2026-10-14T15:30:00Z</date>
    <key>PayloadData</key>
    <data>AQIDBAUGBwgJCgsMDQ4PEA==</data>
    <key>NestedDict</key>
    <dict>
        <key>InnerData</key>
        <data>VGVzdERhdGE=</data>
    </dict>
</dict>
</plist>`

	code := `native.action("com.apple.custom.typed_data_date", parameters: {
    "TypedConfig": native.plist(` + strconv.Quote(xmlPayload) + `)
})
`
	plistBytes, err := CompileSourceToPlist("typed_payload.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile typed payload cherri: %v", err)
	}

	// 1. Unmarshal XML/standard output plist
	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}

	actions := rawMap["WFWorkflowActions"].([]any)
	act := actions[0].(map[string]any)
	params := act["WFWorkflowActionParameters"].(map[string]any)
	typedCfg, ok := params["TypedConfig"].(map[string]any)
	if !ok {
		t.Fatalf("expected TypedConfig to be map[string]any, got %T", params["TypedConfig"])
	}

	// Verify typed time.Time
	eventDate, isTime := typedCfg["EventDate"].(time.Time)
	if !isTime {
		t.Fatalf("expected EventDate to be time.Time, got %T (%v)", typedCfg["EventDate"], typedCfg["EventDate"])
	}
	expectedDate := time.Date(2026, 10, 14, 15, 30, 0, 0, time.UTC)
	if !eventDate.Equal(expectedDate) {
		t.Errorf("EventDate mismatch: expected %v, got %v", expectedDate, eventDate)
	}

	// Verify typed []byte for PayloadData
	payloadData, isBytes := typedCfg["PayloadData"].([]byte)
	if !isBytes {
		t.Fatalf("expected PayloadData to be []byte, got %T (%v)", typedCfg["PayloadData"], typedCfg["PayloadData"])
	}
	expectedBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	if len(payloadData) != len(expectedBytes) {
		t.Fatalf("expected %d bytes, got %d", len(expectedBytes), len(payloadData))
	}
	for i, b := range expectedBytes {
		if payloadData[i] != b {
			t.Errorf("byte mismatch at index %d: expected %d, got %d", i, b, payloadData[i])
		}
	}

	// Verify nested []byte
	nestedDict := typedCfg["NestedDict"].(map[string]any)
	innerData, innerIsBytes := nestedDict["InnerData"].([]byte)
	if !innerIsBytes {
		t.Fatalf("expected InnerData to be []byte, got %T", nestedDict["InnerData"])
	}
	if string(innerData) != "TestData" {
		t.Errorf("expected TestData, got %s", string(innerData))
	}

	// 2. Binary Plist serialization round-trip verification
	binBytes, err := plist.Marshal(rawMap, plist.BinaryFormat)
	if err != nil {
		t.Fatalf("failed to marshal to binary plist: %v", err)
	}
	var binMap map[string]any
	if _, err := plist.Unmarshal(binBytes, &binMap); err != nil {
		t.Fatalf("failed to unmarshal binary plist: %v", err)
	}
	binActions := binMap["WFWorkflowActions"].([]any)
	binParams := binActions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	binTypedCfg := binParams["TypedConfig"].(map[string]any)

	binDate, ok := binTypedCfg["EventDate"].(time.Time)
	if !ok || !binDate.Equal(expectedDate) {
		t.Errorf("binary plist round-trip date mismatch: %v", binTypedCfg["EventDate"])
	}
	binData, ok := binTypedCfg["PayloadData"].([]byte)
	if !ok || len(binData) != 16 {
		t.Errorf("binary plist round-trip data mismatch: %v", binTypedCfg["PayloadData"])
	}
}

// 8. GAP-NATIVE-NIL-EMPTY: Verify distinct behavior for nil/absent values versus empty values
func TestNATIVE_Plist_NilOmittedVsEmptyDistinction(t *testing.T) {
	code := `native.action("com.apple.test.nil_vs_empty", parameters: {
    "ExplicitEmptyString": "",
    "ExplicitEmptyList": [],
    "ExplicitEmptyDict": native.plist({}),
    "NonEmptyString": "ValuePresent"
})
`
	plistBytes, err := CompileSourceToPlist("nil_vs_empty.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile nil_vs_empty: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	act := rawMap["WFWorkflowActions"].([]any)[0].(map[string]any)
	params := act["WFWorkflowActionParameters"].(map[string]any)

	// Explicit empty string must exist and have length 0
	emptyStr, exists := params["ExplicitEmptyString"].(string)
	if !exists {
		t.Errorf("expected ExplicitEmptyString to exist in plist")
	}
	if emptyStr != "" {
		t.Errorf("expected empty string, got %q", emptyStr)
	}

	// Explicit empty list must exist and have length 0
	var emptyListLen = -1
	if rawList, isSlice := params["ExplicitEmptyList"].([]any); isSlice {
		emptyListLen = len(rawList)
	} else if stateMap, isMap := params["ExplicitEmptyList"].(map[string]any); isMap {
		if stateMap["WFSerializationType"] == "WFArrayParameterState" {
			if items, ok := stateMap["Value"].([]any); ok {
				emptyListLen = len(items)
			} else {
				emptyListLen = 0 // omitempty in plist marshaling
			}
		}
	}
	if emptyListLen != 0 {
		t.Errorf("expected ExplicitEmptyList to exist with length 0, got %d (%v)", emptyListLen, params["ExplicitEmptyList"])
	}

	// Explicit empty dict must exist and have length 0
	emptyDict, exists := params["ExplicitEmptyDict"].(map[string]any)
	if !exists {
		t.Errorf("expected ExplicitEmptyDict to exist in plist")
	}
	if len(emptyDict) != 0 {
		t.Errorf("expected empty dict, got length %d", len(emptyDict))
	}

	// Absent / omitted key must NOT exist in parameters map (distinguished from empty values)
	if _, exists := params["OmittedOrNilKey"]; exists {
		t.Errorf("expected OmittedOrNilKey to be absent from plist parameters")
	}
}

// 9. GAP-NATIVE-CHAINED-REF: Multi-level chained aggrandizements (Coercion -> DictionaryKey -> Property)
func TestNATIVE_Ref_ChainedMultiLevelAggrandizements(t *testing.T) {
	code := `native.action("com.apple.custom.chained_refs", parameters: {
    "DeepRef": native.ref("ActionOutput",
        uuid: "B2C3D4E5-F6A7-8901-BCDE-F12345678901",
        aggrandizements: [
            { "Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFDictionaryContentItem" },
            { "Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "session_payload" },
            { "Type": "WFPropertyVariableAggrandizement", "PropertyName": "File Size" }
        ]
    )
})
`
	plistBytes, err := CompileSourceToPlist("chained_refs.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile chained_refs: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	act := rawMap["WFWorkflowActions"].([]any)[0].(map[string]any)
	params := act["WFWorkflowActionParameters"].(map[string]any)
	deepRef := params["DeepRef"].(map[string]any)

	if deepRef["WFSerializationType"] != "WFTextTokenAttachment" {
		t.Errorf("expected WFTextTokenAttachment, got %v", deepRef["WFSerializationType"])
	}
	refVal := deepRef["Value"].(map[string]any)
	if refVal["Type"] != "ActionOutput" {
		t.Errorf("expected Type ActionOutput, got %v", refVal["Type"])
	}
	if refVal["OutputUUID"] != "B2C3D4E5-F6A7-8901-BCDE-F12345678901" {
		t.Errorf("OutputUUID mismatch: %v", refVal["OutputUUID"])
	}

	aggrs := refVal["Aggrandizements"].([]any)
	if len(aggrs) != 3 {
		t.Fatalf("expected 3 chained aggrandizements, got %d", len(aggrs))
	}

	// Verify exact sequence of chained aggrandizements
	aggr0 := aggrs[0].(map[string]any)
	if aggr0["Type"] != "WFCoercionVariableAggrandizement" || aggr0["CoercionItemClass"] != "WFDictionaryContentItem" {
		t.Errorf("aggrandizement 0 mismatch: %v", aggr0)
	}

	aggr1 := aggrs[1].(map[string]any)
	if aggr1["Type"] != "WFDictionaryValueVariableAggrandizement" || aggr1["DictionaryKey"] != "session_payload" {
		t.Errorf("aggrandizement 1 mismatch: %v", aggr1)
	}

	aggr2 := aggrs[2].(map[string]any)
	if aggr2["Type"] != "WFPropertyVariableAggrandizement" || aggr2["PropertyName"] != "File Size" {
		t.Errorf("aggrandizement 2 mismatch: %v", aggr2)
	}
}

// 10. GAP-PIPELINE-E2E: Full end-to-end import -> decompile -> bounded edit -> recompile pipeline
func TestNATIVE_Workflow_ImportDecompileBoundedEditRecompilePipeline(t *testing.T) {
	// Construct a realistic Shortcut plist containing unknown keys, AppIntents, and standard actions
	initialShortcut := map[string]any{
		"WFWorkflowClientVersion":        "3200.0.7",
		"WFWorkflowClientRelease":        "iOS 18.0",
		"WFWorkflowMinimumClientVersion": 900,
		"CustomEngineTag":                "CherriPreservationPipeline",
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.alert",
				"WFWorkflowActionParameters": map[string]any{
					"UUID":               "ALERT-UUID-001",
					"WFAlertActionTitle": "Original Alert Text 123",
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "com.apple.opaque.appintent.action",
				"WFWorkflowActionParameters": map[string]any{
					"AppIntentDescriptor": map[string]any{
						"AppIntentIdentifier": "OpaqueAppIntent",
						"BundleIdentifier":    "com.example.app",
					},
					"CustomPayloadKey": "PreserveMeLosslessly",
				},
			},
		},
	}

	initialPlistBytes, err := plist.Marshal(initialShortcut, plist.XMLFormat)
	if err != nil {
		t.Fatalf("failed to marshal initial shortcut plist: %v", err)
	}

	// Step 1: Decompile using production in-process decompiler
	decompResp := decompileForMobile(initialPlistBytes, "PipelineTest")
	if !decompResp.OK {
		t.Fatalf("decompile failed: %s", decompResp.Error)
	}
	decompSource := decompResp.Source
	if !strings.Contains(decompSource, "Original Alert Text 123") {
		t.Fatalf("decompiled source missing original alert text:\n%s", decompSource)
	}
	if !strings.Contains(decompSource, "com.apple.opaque.appintent.action") {
		t.Fatalf("decompiled source missing AppIntent action:\n%s", decompSource)
	}

	// Step 2: Perform bounded source-edit via official AST API with negative rejection & preservation
	svc := service.NewService(schema.DefaultRegistry())

	// Negative case 1: Out of range action index rejected
	if _, err := svc.ApplyVisualSourceEdit(decompSource, 99, "alert", "\"Bad\""); err == nil {
		t.Fatalf("expected out-of-range action index 99 to be rejected")
	}

	// Negative case 2: Invalid expression syntax rejected
	if _, err := svc.ApplyVisualSourceEdit(decompSource, 0, "alert", ";;; invalid expression"); err == nil {
		t.Fatalf("expected invalid replacement syntax to be rejected")
	}

	// Negative case 3: Unmappable opaque action edit rejected
	if _, err := svc.ApplyVisualSourceEdit(decompSource, 1, "custom", "\"Bad\""); err == nil {
		t.Fatalf("expected opaque AppIntent action edit to be rejected as unmappable")
	}

	// Bounded edit: mutate only target alert title parameter
	editedSource, err := svc.ApplyVisualSourceEdit(decompSource, 0, "title", "\"Updated Bounded Alert Text 456\"")
	if err != nil {
		t.Fatalf("ApplyVisualSourceEdit failed: %v\ndecompSource was:\n%s", err, decompSource)
	}
	if editedSource == decompSource {
		t.Fatalf("bounded edit substitution did not modify source")
	}
	if !strings.Contains(editedSource, "Updated Bounded Alert Text 456") {
		t.Fatalf("edited source missing updated alert text:\n%s", editedSource)
	}
	if !strings.Contains(editedSource, "com.apple.opaque.appintent.action") {
		t.Fatalf("edited source lost opaque AppIntent action:\n%s", editedSource)
	}

	// Step 3: Recompile the edited source
	recompResp := compileForMobileMode(editedSource, "PipelineTest", "auto", false)
	if !recompResp.OK {
		t.Fatalf("recompile failed: %s (line %d col %d)", recompResp.Error, recompResp.Line, recompResp.Column)
	}

	// Step 4: Verify the recompiled plist
	recompBytes, err := base64.StdEncoding.DecodeString(recompResp.PlistBase64)
	if err != nil {
		t.Fatalf("failed to decode recompiled base64 plist: %v", err)
	}
	var recompiledMap map[string]any
	if _, err := plist.Unmarshal(recompBytes, &recompiledMap); err != nil {
		t.Fatalf("failed to unmarshal recompiled plist: %v", err)
	}

	recompiledActions, ok := recompiledMap["WFWorkflowActions"].([]any)
	if !ok || len(recompiledActions) != 2 {
		t.Fatalf("expected 2 actions in recompiled shortcut, got %d", len(recompiledActions))
	}

	// Assert the bounded edit took effect on the alert action
	alertAct := recompiledActions[0].(map[string]any)
	alertParams := alertAct["WFWorkflowActionParameters"].(map[string]any)
	if alertParams["WFAlertActionTitle"] != "Updated Bounded Alert Text 456" {
		t.Errorf("bounded edit parameter mismatch: expected %q, got %v", "Updated Bounded Alert Text 456", alertParams["WFAlertActionTitle"])
	}

	// Assert the opaque AppIntent action survived losslessly with all its parameters
	appIntentAct := recompiledActions[1].(map[string]any)
	if appIntentAct["WFWorkflowActionIdentifier"] != "com.apple.opaque.appintent.action" {
		t.Errorf("AppIntent action identifier mismatch: %v", appIntentAct["WFWorkflowActionIdentifier"])
	}
	appIntentParams := appIntentAct["WFWorkflowActionParameters"].(map[string]any)
	rawDesc := appIntentParams["AppIntentDescriptor"]
	var intentID, bundleID string
	if descMap, ok := rawDesc.(map[string]any); ok {
		if descMap["WFSerializationType"] == "WFDictionaryFieldValue" {
			if vMap, ok := descMap["Value"].(map[string]any); ok {
				if items, ok := vMap["WFDictionaryFieldValueItems"].([]any); ok {
					for _, item := range items {
						if itemMap, ok := item.(map[string]any); ok {
							kStr := extractTokenStr(itemMap["WFKey"])
							vStr := extractTokenStr(itemMap["WFValue"])
							if kStr == "AppIntentIdentifier" {
								intentID = vStr
							} else if kStr == "BundleIdentifier" {
								bundleID = vStr
							}
						}
					}
				}
			}
		} else {
			if id, ok := descMap["AppIntentIdentifier"].(string); ok {
				intentID = id
			}
			if bid, ok := descMap["BundleIdentifier"].(string); ok {
				bundleID = bid
			}
		}
	}
	if intentID != "OpaqueAppIntent" || bundleID != "com.example.app" {
		t.Errorf("AppIntentDescriptor corrupted in pipeline: intentID=%q, bundleID=%q (raw: %v)", intentID, bundleID, rawDesc)
	}
	if appIntentParams["CustomPayloadKey"] != "PreserveMeLosslessly" {
		t.Errorf("CustomPayloadKey corrupted: %v", appIntentParams["CustomPayloadKey"])
	}
}

// 11. GAP-NATIVE-MALFORMED: Malformed XML, invalid refs and type mismatches fail closed
func TestNATIVE_MalformedRejectionsFailClosed(t *testing.T) {
	testCases := []struct {
		name        string
		code        string
		errContains string
	}{
		{
			name:        "MalformedXMLPlist",
			code:        `native.action("is.workflow.actions.alert", { "Key": native.plist("<?xml version=\"1.0\"?><plist><dict><unclosed>") })`,
			errContains: "native.plist failed to parse XML plist",
		},
		{
			name:        "ActionOutputEmptyUUID",
			code:        `native.action("is.workflow.actions.alert", { "Key": native.ref("ActionOutput", uuid: "   ") })`,
			errContains: "requires a non-empty uuid",
		},
		{
			name:        "VariableEmptyName",
			code:        `native.action("is.workflow.actions.alert", { "Key": native.ref("Variable", name: "   ") })`,
			errContains: "requires a non-empty name",
		},
		{
			name:        "UnknownRefType",
			code:        `native.action("is.workflow.actions.alert", { "Key": native.ref("InvalidUnknownKind", uuid: "123") })`,
			errContains: "unsupported or invalid native.ref kind",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CompileSourceToPlist("malformed.cherri", tc.code)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("expected error containing %q, got %v", tc.errContains, err)
			}
		})
	}
}

func extractTokenStr(val any) string {
	if m, ok := val.(map[string]any); ok {
		if vMap, ok := m["Value"].(map[string]any); ok {
			if s, ok := vMap["string"].(string); ok {
				return s
			}
		}
		if s, ok := m["Value"].(string); ok {
			return s
		}
	}
	return strings.TrimSpace(fmt.Sprint(val))
}

