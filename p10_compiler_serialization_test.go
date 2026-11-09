/*
 * Copyright (c) Cherri
 */
package main

import (
	"strings"
	"testing"
)

// Comprehensive compiler, serialization, decompile, and round-trip verification
// for Cherri P10 covering both Legacy and V2 syntax.
func TestP10_Serialization_RunAction_FixedAndDynamicName_DictPayload_LegacyAndV2(t *testing.T) {
	r := newRoundTripRunner(t)

	tests := []struct {
		name       string
		source     string
		isDynamic  bool
		targetName string
	}{
		{
			name: "v2-fixed-dict-payload",
			source: `let payload = {"model": "gpt-4", "count": 2}
run("FixedServer", input: payload)
`,
			isDynamic:  false,
			targetName: "FixedServer",
		},
		{
			name: "v2-dynamic-dict-payload",
			source: `let target = "DynamicTarget"
let payload = {"key": "value"}
run(target, input: payload)
`,
			isDynamic:  true,
			targetName: "DynamicTarget",
		},
		{
			name: "legacy-fixed-dict-payload",
			source: `#include 'stdlib'
@payload = {"model": "gpt-4", "count": 2}
run("FixedServer", @payload)
`,
			isDynamic:  false,
			targetName: "FixedServer",
		},
		{
			name: "legacy-dynamic-dict-payload",
			source: `#include 'stdlib'
@target = "DynamicTarget"
@payload = {"key": "value"}
run(@target, @payload)
`,
			isDynamic:  true,
			targetName: "DynamicTarget",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plistPath := r.compileSource("ser_run_"+tc.name, tc.source)
			doc := loadRoundTripDocument(t, plistPath)

			actionsList, ok := doc["WFWorkflowActions"].([]any)
			if !ok || len(actionsList) == 0 {
				t.Fatalf("%s: expected actions in compiled plist", tc.name)
			}

			var runAct map[string]any
			for _, a := range actionsList {
				m := a.(map[string]any)
				if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
					runAct = m
					break
				}
			}
			if runAct == nil {
				t.Fatalf("%s: expected runworkflow action in plist", tc.name)
			}

			params := runAct["WFWorkflowActionParameters"].(map[string]any)

			// 1. Verify WFWorkflowName
			wfNameRaw := params["WFWorkflowName"]
			if wfNameRaw == nil {
				t.Fatalf("%s: WFWorkflowName is missing", tc.name)
			}
			if !tc.isDynamic {
				if wfNameStr, isStr := wfNameRaw.(string); !isStr || wfNameStr != tc.targetName {
					t.Errorf("%s: expected WFWorkflowName %q, got %v", tc.name, tc.targetName, wfNameRaw)
				}
			} else {
				// Dynamic token string
				if tokenMap, isMap := wfNameRaw.(map[string]any); isMap {
					if ser, ok := tokenMap["WFSerializationType"]; !ok || ser != "WFTextTokenString" {
						t.Errorf("%s: expected WFTextTokenString for dynamic target, got %v", tc.name, ser)
					}
				}
			}

			// 2. Verify WFWorkflow descriptor dictionary
			wfDesc, ok := params["WFWorkflow"].(map[string]any)
			if !ok {
				t.Fatalf("%s: WFWorkflow dictionary is missing or invalid: %T", tc.name, params["WFWorkflow"])
			}
			if wfDesc["workflowName"] == nil {
				t.Errorf("%s: WFWorkflow.workflowName is missing", tc.name)
			}

			// 3. Verify WFInput is retained and non-nil (the primary P10 bug fix)
			wfInput := params["WFInput"]
			if wfInput == nil {
				t.Fatalf("%s: CRITICAL BUG REGRESSION: WFInput is nil/missing in run action parameters", tc.name)
			}

			// 4. Verify round-trip and decompile
			decomp := r.decompileShortcut("ser_run_decomp_"+tc.name, plistPath)
			if !strings.Contains(decomp, "run(") {
				t.Errorf("%s: decompiler lost run action call:\n%s", tc.name, decomp)
			}
			if !strings.Contains(decomp, tc.targetName) && !strings.Contains(decomp, "target") {
				t.Errorf("%s: decompiled source missing target reference:\n%s", tc.name, decomp)
			}

			recompiledPlist := r.compileSource("ser_run_recomp_"+tc.name, decomp)
			recompiledDoc := loadRoundTripDocument(t, recompiledPlist)
			assertActionsStructurallyEqual(t, doc, recompiledDoc, "WFWorkflow")
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
			plistPath := r.compileSource("ser_stat_leg_"+opClean, src)
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
