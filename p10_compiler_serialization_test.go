/*
 * Copyright (c) Cherri
 */
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// compileSourceWithMode writes Cherri source without auto-migrating if legacyMode is true,
// and compiles it with or without --legacy.
func (r *roundTripRunner) compileSourceWithMode(label string, source string, legacyMode bool) string {
	r.t.Helper()
	if !legacyMode {
		return r.compileSource(label, source)
	}
	srcPath := filepath.Join(r.dir, label+".cherri")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		r.t.Fatalf("%s: write legacy source: %v", label, err)
	}
	r.runCherri(label, srcPath, "--legacy", "--skip-sign", "--no-ansi")
	return filepath.Join(r.dir, label+"_unsigned.shortcut")
}

// extractDictItems parses WFDictionaryFieldValueItems into a map[key] -> (itemType, stringValue)
func extractDictItems(t *testing.T, dictAct map[string]any) map[string]struct {
	itemType uint64
	val      string
} {
	t.Helper()
	params, ok := dictAct["WFWorkflowActionParameters"].(map[string]any)
	if !ok {
		t.Fatalf("dictionary action missing WFWorkflowActionParameters")
	}
	wfItems, ok := params["WFItems"].(map[string]any)
	if !ok {
		t.Fatalf("dictionary action missing WFItems map: %T", params["WFItems"])
	}
	if wfItems["WFSerializationType"] != "WFDictionaryFieldValue" {
		t.Fatalf("expected WFItems.WFSerializationType == WFDictionaryFieldValue, got %v", wfItems["WFSerializationType"])
	}
	valMap, ok := wfItems["Value"].(map[string]any)
	if !ok {
		t.Fatalf("WFItems.Value is not a map: %T", wfItems["Value"])
	}
	itemsSlice, ok := valMap["WFDictionaryFieldValueItems"].([]any)
	if !ok {
		t.Fatalf("WFDictionaryFieldValueItems is not a slice: %T", valMap["WFDictionaryFieldValueItems"])
	}

	result := make(map[string]struct {
		itemType uint64
		val      string
	})
	for _, rawItem := range itemsSlice {
		item := rawItem.(map[string]any)
		keyMap := item["WFKey"].(map[string]any)["Value"].(map[string]any)
		keyStr := keyMap["string"].(string)

		var itemType uint64
		switch v := item["WFItemType"].(type) {
		case uint64:
			itemType = v
		case int64:
			itemType = uint64(v)
		case int:
			itemType = uint64(v)
		}

		valToken := item["WFValue"].(map[string]any)["Value"].(map[string]any)
		valStr := fmt.Sprint(valToken["string"])
		result[keyStr] = struct {
			itemType uint64
			val      string
		}{itemType: itemType, val: valStr}
	}
	return result
}

// assertDynamicWorkflowNameToken verifies that a dynamic workflow name parameter is serialized
// as a WFTextTokenString with "\uFFFC" and an attachment at "{0, 1}" referencing the target variable/action.
func assertDynamicWorkflowNameToken(t *testing.T, ctx string, rawVal any, expectedVarName string, expectedTargetUUID string) {
	t.Helper()
	tokenMap, ok := rawVal.(map[string]any)
	if !ok {
		t.Fatalf("%s: expected dynamic workflow name to be a WFTextTokenString map, got %T (%v)", ctx, rawVal, rawVal)
	}
	if tokenMap["WFSerializationType"] != "WFTextTokenString" {
		t.Fatalf("%s: expected WFSerializationType == 'WFTextTokenString', got %v", ctx, tokenMap["WFSerializationType"])
	}
	valMap, ok := tokenMap["Value"].(map[string]any)
	if !ok {
		t.Fatalf("%s: WFTextTokenString missing Value map: %v", ctx, tokenMap)
	}
	strVal, _ := valMap["string"].(string)
	if strVal != "\uFFFC" {
		t.Errorf("%s: expected dynamic token string '\\uFFFC', got %q", ctx, strVal)
	}
	atts, ok := valMap["attachmentsByRange"].(map[string]any)
	if !ok || len(atts) != 1 {
		t.Fatalf("%s: expected exactly 1 attachment in attachmentsByRange, got %v", ctx, valMap["attachmentsByRange"])
	}
	att0, ok := atts["{0, 1}"].(map[string]any)
	if !ok {
		t.Fatalf("%s: missing attachment at range '{0, 1}', got %v", ctx, atts)
	}
	switch att0["Type"] {
	case "ActionOutput":
		if att0["OutputName"] != expectedVarName {
			t.Errorf("%s: expected attachment OutputName %q, got %v", ctx, expectedVarName, att0["OutputName"])
		}
		if expectedTargetUUID != "" && att0["OutputUUID"] != expectedTargetUUID {
			t.Errorf("%s: expected attachment OutputUUID %q, got %v", ctx, expectedTargetUUID, att0["OutputUUID"])
		}
	case "Variable":
		if att0["VariableName"] != expectedVarName {
			t.Errorf("%s: expected attachment VariableName %q, got %v", ctx, expectedVarName, att0["VariableName"])
		}
	default:
		t.Fatalf("%s: unexpected attachment Type %v in %v", ctx, att0["Type"], att0)
	}
}

// Comprehensive compiler, serialization, decompile, and round-trip verification
// for Cherri P10 covering both Legacy and V2 syntax.
func TestP10_Serialization_RunAction_FixedAndDynamicName_DictPayload_LegacyAndV2(t *testing.T) {
	r := newRoundTripRunner(t)

	tests := []struct {
		name         string
		source       string
		legacyMode   bool
		isDynamic    bool
		isInlineDict bool
		targetName   string
		expectedDict map[string]struct {
			itemType uint64
			val      string
		}
	}{
		{
			name: "v2-fixed-dict-payload",
			source: `let payload = {"model": "gpt-4", "count": 2}
run("FixedServer", input: payload)
`,
			legacyMode: false,
			isDynamic:  false,
			targetName: "FixedServer",
			expectedDict: map[string]struct {
				itemType uint64
				val      string
			}{
				"model": {itemType: 0, val: "gpt-4"},
				"count": {itemType: 3, val: "2"},
			},
		},
		{
			name: "v2-inline-dict-payload",
			source: `run("InlineServer", input: {"env": "staging", "retries": 5})
`,
			legacyMode:   false,
			isDynamic:    false,
			isInlineDict: true,
			targetName:   "InlineServer",
			expectedDict: map[string]struct {
				itemType uint64
				val      string
			}{
				"env":     {itemType: 0, val: "staging"},
				"retries": {itemType: 3, val: "5"},
			},
		},
		{
			name: "v2-dynamic-dict-payload",
			source: `let target = "DynamicTarget"
let payload = {"key": "value"}
run(target, input: payload)
`,
			legacyMode: false,
			isDynamic:  true,
			targetName: "DynamicTarget",
			expectedDict: map[string]struct {
				itemType uint64
				val      string
			}{
				"key": {itemType: 0, val: "value"},
			},
		},
		{
			name: "legacy-fixed-dict-payload",
			source: `@payload = {"model": "gpt-4", "count": 2}
run("FixedServer", @payload)
`,
			legacyMode: true,
			isDynamic:  false,
			targetName: "FixedServer",
			expectedDict: map[string]struct {
				itemType uint64
				val      string
			}{
				"model": {itemType: 0, val: "gpt-4"},
				"count": {itemType: 3, val: "2"},
			},
		},
		{
			name: "legacy-dynamic-dict-payload",
			source: `@target = "DynamicTarget"
@payload = {"key": "value"}
run(@target, @payload)
`,
			legacyMode: true,
			isDynamic:  true,
			targetName: "DynamicTarget",
			expectedDict: map[string]struct {
				itemType uint64
				val      string
			}{
				"key": {itemType: 0, val: "value"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plistPath := r.compileSourceWithMode("ser_run_"+tc.name, tc.source, tc.legacyMode)
			doc := loadRoundTripDocument(t, plistPath)

			actionsList, ok := doc["WFWorkflowActions"].([]any)
			if !ok || len(actionsList) == 0 {
				t.Fatalf("%s: expected actions in compiled plist", tc.name)
			}

			var dictAct map[string]any
			var targetTextAct map[string]any
			var runAct map[string]any
			for _, a := range actionsList {
				m := a.(map[string]any)
				switch m["WFWorkflowActionIdentifier"] {
				case "is.workflow.actions.dictionary":
					dictAct = m
				case "is.workflow.actions.gettext":
					targetTextAct = m
				case "is.workflow.actions.runworkflow":
					runAct = m
				}
			}
			if runAct == nil {
				t.Fatalf("%s: expected is.workflow.actions.runworkflow action in plist", tc.name)
			}

			params := runAct["WFWorkflowActionParameters"].(map[string]any)

			// 1. Verify dictionary payload contents (keys, types, and values) in the plist
			var gotDict map[string]struct {
				itemType uint64
				val      string
			}
			var dictUUID string
			if tc.isInlineDict {
				// Inline dictionary literal is serialized directly as WFDictionaryFieldValue inside WFInput
				syntheticAct := map[string]any{
					"WFWorkflowActionParameters": map[string]any{
						"WFItems": params["WFInput"],
					},
				}
				gotDict = extractDictItems(t, syntheticAct)
			} else {
				if dictAct == nil {
					t.Fatalf("%s: expected is.workflow.actions.dictionary action in plist", tc.name)
				}
				gotDict = extractDictItems(t, dictAct)
				dictParams := dictAct["WFWorkflowActionParameters"].(map[string]any)
				dictUUID, _ = dictParams["UUID"].(string)
			}

			if len(gotDict) != len(tc.expectedDict) {
				t.Fatalf("%s: dictionary item count mismatch: got %d (%v), want %d", tc.name, len(gotDict), gotDict, len(tc.expectedDict))
			}
			for k, want := range tc.expectedDict {
				got, exists := gotDict[k]
				if !exists {
					t.Errorf("%s: missing dictionary key %q in serialized payload %v", tc.name, k, gotDict)
					continue
				}
				if got.itemType != want.itemType || got.val != want.val {
					t.Errorf("%s: dictionary key %q mismatch: got (type=%d, val=%q), want (type=%d, val=%q)",
						tc.name, k, got.itemType, got.val, want.itemType, want.val)
				}
			}

			var targetUUID string
			if targetTextAct != nil {
				targetParams := targetTextAct["WFWorkflowActionParameters"].(map[string]any)
				targetUUID, _ = targetParams["UUID"].(string)
			}

			// 2. Verify WFWorkflowName and WFWorkflow.workflowName (both static and dynamic representations)
			wfNameRaw := params["WFWorkflowName"]
			if wfNameRaw == nil {
				t.Fatalf("%s: WFWorkflowName is missing", tc.name)
			}
			wfDesc, ok := params["WFWorkflow"].(map[string]any)
			if !ok {
				t.Fatalf("%s: WFWorkflow dictionary is missing or invalid: %T", tc.name, params["WFWorkflow"])
			}
			if wfDesc["isSelf"] != false {
				t.Errorf("%s: expected WFWorkflow.isSelf == false, got %v", tc.name, wfDesc["isSelf"])
			}

			if !tc.isDynamic {
				if wfNameStr, isStr := wfNameRaw.(string); !isStr || wfNameStr != tc.targetName {
					t.Errorf("%s: expected WFWorkflowName %q, got %v", tc.name, tc.targetName, wfNameRaw)
				}
				if wfDesc["workflowName"] != tc.targetName {
					t.Errorf("%s: expected WFWorkflow.workflowName %q, got %v", tc.name, tc.targetName, wfDesc["workflowName"])
				}
			} else {
				assertDynamicWorkflowNameToken(t, tc.name+"/WFWorkflowName", wfNameRaw, "target", targetUUID)
				assertDynamicWorkflowNameToken(t, tc.name+"/WFWorkflow.workflowName", wfDesc["workflowName"], "target", targetUUID)
			}

			// 3. Verify WFInput structure and linkage to the dictionary payload
			wfInputRaw := params["WFInput"]
			if wfInputRaw == nil {
				t.Fatalf("%s: CRITICAL BUG REGRESSION: WFInput is nil/missing in run action parameters", tc.name)
			}
			wfInputMap, ok := wfInputRaw.(map[string]any)
			if !ok {
				t.Fatalf("%s: expected WFInput to be a serialized map, got %T (%v)", tc.name, wfInputRaw, wfInputRaw)
			}
			if tc.isInlineDict {
				if wfInputMap["WFSerializationType"] != "WFDictionaryFieldValue" {
					t.Errorf("%s: expected inline WFInput.WFSerializationType == 'WFDictionaryFieldValue', got %v", tc.name, wfInputMap["WFSerializationType"])
				}
			} else {
				if wfInputMap["WFSerializationType"] != "WFTextTokenAttachment" {
					t.Errorf("%s: expected WFInput.WFSerializationType == 'WFTextTokenAttachment', got %v", tc.name, wfInputMap["WFSerializationType"])
				}
				wfInputVal, ok := wfInputMap["Value"].(map[string]any)
				if !ok {
					t.Fatalf("%s: WFInput missing Value map: %v", tc.name, wfInputMap)
				}
				switch wfInputVal["Type"] {
				case "ActionOutput":
					if dictUUID != "" && wfInputVal["OutputUUID"] != dictUUID {
						t.Errorf("%s: WFInput OutputUUID %v does not match dictionary action UUID %q", tc.name, wfInputVal["OutputUUID"], dictUUID)
					}
				case "Variable":
					if wfInputVal["VariableName"] != "payload" {
						t.Errorf("%s: expected WFInput VariableName 'payload', got %v", tc.name, wfInputVal["VariableName"])
					}
				default:
					t.Fatalf("%s: unexpected WFInput.Value.Type %v in %v", tc.name, wfInputVal["Type"], wfInputVal)
				}

				// 4. Verify round-trip and decompile for variable-backed dictionary payloads
				decomp := r.decompileShortcut("ser_run_decomp_"+tc.name, plistPath)
				if !strings.Contains(decomp, "run(") {
					t.Errorf("%s: decompiler lost run action call:\n%s", tc.name, decomp)
				}
				if !strings.Contains(decomp, tc.targetName) && !strings.Contains(decomp, "target") {
					t.Errorf("%s: decompiled source missing target reference:\n%s", tc.name, decomp)
				}

				recompiledPlist := r.compileSource("ser_run_recomp_"+tc.name, decomp)
				recompiledDoc := loadRoundTripDocument(t, recompiledPlist)
				if !tc.legacyMode {
					assertActionsStructurallyEqual(t, doc, recompiledDoc, "WFWorkflow")
				} else {
					recompActions, _ := recompiledDoc["WFWorkflowActions"].([]any)
					var recompRun map[string]any
					for _, a := range recompActions {
						m := a.(map[string]any)
						if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
							recompRun = m
						}
					}
					if recompRun == nil {
						t.Fatalf("%s: recompiled legacy shortcut missing is.workflow.actions.runworkflow", tc.name)
					}
					recompParams, _ := recompRun["WFWorkflowActionParameters"].(map[string]any)
					if recompParams["WFInput"] == nil {
						t.Fatalf("%s: recompiled legacy shortcut lost WFInput parameter", tc.name)
					}
				}
			}
		})
	}
}

func TestP10_Serialization_StatisticAction_AllOperations_LegacyAndV2(t *testing.T) {
	r := newRoundTripRunner(t)

	operations := []string{
		"Average",
		"Minimum",
		"Maximum",
		"Sum",
		"Median",
		"Mode",
		"Range",
		"Standard Deviation",
	}

	for _, op := range operations {
		opClean := strings.ReplaceAll(op, " ", "_")

		t.Run("v2-"+opClean, func(t *testing.T) {
			src := `let nums = [10, 20, 30]
statistic("` + op + `", input: nums)
`
			plistPath := r.compileSource("ser_stat_v2_"+opClean, src)
			doc := loadRoundTripDocument(t, plistPath)

			var statAct map[string]any
			for _, a := range doc["WFWorkflowActions"].([]any) {
				m := a.(map[string]any)
				if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.statistics" {
					statAct = m
					break
				}
			}
			if statAct == nil {
				t.Fatalf("statistic action missing in plist for %s", op)
			}

			params := statAct["WFWorkflowActionParameters"].(map[string]any)

			// Must emit WFInput, NEVER Input
			if params["WFInput"] == nil {
				t.Fatalf("CRITICAL: statistic missing WFInput wire key for operation %s", op)
			}
			if params["Input"] != nil {
				t.Fatalf("CRITICAL: statistic emitted deprecated Input key instead of WFInput for %s", op)
			}
			if params["WFStatisticsOperation"] != op {
				t.Errorf("expected WFStatisticsOperation %q, got %v", op, params["WFStatisticsOperation"])
			}

			// Decompile and round-trip
			decomp := r.decompileShortcut("ser_stat_v2_decomp_"+opClean, plistPath)
			if !strings.Contains(decomp, "statistic(") {
				t.Errorf("lost statistic call in decompile:\n%s", decomp)
			}

			recompiledPlist := r.compileSource("ser_stat_v2_recomp_"+opClean, decomp)
			recompiledDoc := loadRoundTripDocument(t, recompiledPlist)
			assertActionsStructurallyEqual(t, doc, recompiledDoc)
		})

		t.Run("legacy-"+opClean, func(t *testing.T) {
			src := `#include 'actions/math'
@nums = [10, 20, 30]
statistic('` + op + `', @nums)
`
			plistPath := r.compileSourceWithMode("ser_stat_leg_"+opClean, src, true)
			doc := loadRoundTripDocument(t, plistPath)

			var statAct map[string]any
			for _, a := range doc["WFWorkflowActions"].([]any) {
				m := a.(map[string]any)
				if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.statistics" {
					statAct = m
					break
				}
			}
			if statAct == nil {
				t.Fatalf("legacy statistic action missing in plist for %s", op)
			}

			params := statAct["WFWorkflowActionParameters"].(map[string]any)
			if params["WFInput"] == nil {
				t.Fatalf("legacy: statistic missing WFInput wire key for operation %s", op)
			}
			if params["Input"] != nil {
				t.Fatalf("legacy: statistic emitted Input key instead of WFInput for %s", op)
			}
		})
	}
}

func TestP10_Serialization_RegexQuantifiers_And_JSON_Strings(t *testing.T) {
	r := newRoundTripRunner(t)

	t.Run("regex-quantifiers-literal-no-variable-attachment", func(t *testing.T) {
		// Literal pattern with quantifiers {8}, {4}, {12} passed directly
		src := `matchText("[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}", text: "550e8400-e29b-41d4-a716-446655440000")
`
		plistPath := r.compileSource("ser_regex_quantifiers_lit", src)
		doc := loadRoundTripDocument(t, plistPath)

		var matchAct map[string]any
		for _, a := range doc["WFWorkflowActions"].([]any) {
			m := a.(map[string]any)
			if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.text.match" {
				matchAct = m
				break
			}
		}
		if matchAct == nil {
			t.Fatalf("text.match action missing in plist")
		}

		params := matchAct["WFWorkflowActionParameters"].(map[string]any)
		rawPat := params["WFMatchTextPattern"]

		// Must NOT have variable attachments for {8}, {4}, {12}
		if tokenMap, isMap := rawPat.(map[string]any); isMap {
			if ser, hasSer := tokenMap["WFSerializationType"]; hasSer && ser == "WFTextTokenString" {
				val := tokenMap["Value"].(map[string]any)
				if atts, hasAtts := val["attachmentsByRange"].(map[string]any); hasAtts && len(atts) > 0 {
					t.Fatalf("CRITICAL: regex quantifiers converted into attachments: %v", atts)
				}
				strVal := val["string"].(string)
				if !strings.Contains(strVal, "{8}") || !strings.Contains(strVal, "{12}") {
					t.Errorf("regex pattern was corrupted: %q", strVal)
				}
			}
		} else if strVal, isStr := rawPat.(string); isStr {
			if !strings.Contains(strVal, "{8}") || !strings.Contains(strVal, "{12}") {
				t.Errorf("regex pattern was corrupted: %q", strVal)
			}
		}
	})

	t.Run("regex-quantifiers-in-text-action", func(t *testing.T) {
		src := `let pattern = "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
`
		plistPath := r.compileSource("ser_regex_text_action", src)
		doc := loadRoundTripDocument(t, plistPath)

		var textAct map[string]any
		for _, a := range doc["WFWorkflowActions"].([]any) {
			m := a.(map[string]any)
			if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
				textAct = m
				break
			}
		}
		if textAct == nil {
			t.Fatalf("gettext action missing in plist")
		}

		params := textAct["WFWorkflowActionParameters"].(map[string]any)
		rawText := params["WFTextActionText"]
		if tokenMap, isMap := rawText.(map[string]any); isMap {
			val := tokenMap["Value"].(map[string]any)
			if atts, hasAtts := val["attachmentsByRange"].(map[string]any); hasAtts && len(atts) > 0 {
				t.Fatalf("CRITICAL: regex quantifiers in text action converted to attachments: %v", atts)
			}
		}
	})

	t.Run("json-string-with-braces-not-tokenized", func(t *testing.T) {
		src := `let jsonBody = "{\"model\":\"gpt-4\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"
`
		plistPath := r.compileSource("ser_json_string", src)
		doc := loadRoundTripDocument(t, plistPath)

		var textAct map[string]any
		for _, a := range doc["WFWorkflowActions"].([]any) {
			m := a.(map[string]any)
			if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
				textAct = m
				break
			}
		}
		if textAct == nil {
			t.Fatalf("gettext action missing in plist")
		}

		params := textAct["WFWorkflowActionParameters"].(map[string]any)
		rawText := params["WFTextActionText"]
		if tokenMap, isMap := rawText.(map[string]any); isMap {
			val := tokenMap["Value"].(map[string]any)
			if atts, hasAtts := val["attachmentsByRange"].(map[string]any); hasAtts && len(atts) > 0 {
				t.Fatalf("CRITICAL: JSON string braces converted into variable attachments: %v", atts)
			}
			strVal := val["string"].(string)
			if !strings.Contains(strVal, "gpt-4") || !strings.Contains(strVal, "messages") {
				t.Errorf("JSON string content was corrupted: %q", strVal)
			}
		}
	})
}

func TestP10_Serialization_Conditionals_FailClosed(t *testing.T) {
	// 1. V2 array equality == []
	v2CodeEq := `let matches = ["apple", "banana"]
if matches == [] {
    alert(alert: "Empty")
}
`
	_, errEq := compileV2Helper(t, "cond_eq", v2CodeEq)
	if errEq == nil {
		t.Fatalf("expected compile failure for 'matches == []', got success")
	}
	if !strings.Contains(errEq.Error(), "Invalid type 'array' for conditional '=='") {
		t.Errorf("unexpected error message for == []: %v", errEq)
	}

	// 2. V2 array inequality != []
	v2CodeNeq := `let matches = ["apple", "banana"]
if matches != [] {
    alert(alert: "Not Empty")
}
`
	_, errNeq := compileV2Helper(t, "cond_neq", v2CodeNeq)
	if errNeq == nil {
		t.Fatalf("expected compile failure for 'matches != []', got success")
	}
	if !strings.Contains(errNeq.Error(), "Invalid type 'array' for conditional '!='") {
		t.Errorf("unexpected error message for != []: %v", errNeq)
	}
}
