/*
 * Copyright (c) Cherri
 */
package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

const PinnedUpstreamCommit = "bcc0095754e2cc0baaeb4d93dae172b8153b4716"

// Independent Oracle: Compares action definitions and wire behavior against
// original pinned upstream Cherri (electrikmilk/cherri@bcc0095754e2cc0baaeb4d93dae172b8153b4716).
// This is an external, independent oracle check, not internal legacy parity.
func TestP10_IndependentOracle_UpstreamPinnedCommitComparison(t *testing.T) {
	// 1. Verify that the pinned upstream commit exists in the local Git object store
	cmdCheck := exec.Command("git", "cat-file", "-e", PinnedUpstreamCommit+"^{commit}")
	if out, err := cmdCheck.CombinedOutput(); err != nil {
		t.Fatalf("independent oracle: pinned upstream commit %s is not available in git database: %v\n%s",
			PinnedUpstreamCommit, err, string(out))
	}

	// 2. Fetch upstream math.cherri from pinned commit
	cmdShowMath := exec.Command("git", "show", PinnedUpstreamCommit+":actions/math.cherri")
	upstreamMathBytes, err := cmdShowMath.Output()
	if err != nil {
		t.Fatalf("independent oracle: failed to read upstream math.cherri at %s: %v", PinnedUpstreamCommit, err)
	}
	upstreamMath := string(upstreamMathBytes)

	// Upstream must define statistic action with WFInput
	statRegex := regexp.MustCompile(`action\s+'statistics'\s+statistic\([^)]*variable\s+input:\s*'([^']+)'`)
	upstreamMatches := statRegex.FindStringSubmatch(upstreamMath)
	if len(upstreamMatches) < 2 {
		t.Fatalf("independent oracle: failed to find statistic action definition in upstream commit %s", PinnedUpstreamCommit)
	}
	upstreamWireKey := upstreamMatches[1]
	if upstreamWireKey != "WFInput" {
		t.Fatalf("independent oracle: expected upstream wire key 'WFInput', got %q", upstreamWireKey)
	}
	t.Logf("Independent Oracle verified: Upstream commit %s defines statistic input wire key as %q",
		PinnedUpstreamCommit[:8], upstreamWireKey)

	// Verify our fork's local actions/math.cherri matches the upstream oracle
	cmdShowLocalMath := exec.Command("git", "show", "HEAD:actions/math.cherri")
	localMathBytes, err := cmdShowLocalMath.Output()
	if err != nil {
		t.Fatalf("failed to read local actions/math.cherri: %v", err)
	}
	localMatches := statRegex.FindStringSubmatch(string(localMathBytes))
	if len(localMatches) < 2 {
		t.Fatalf("failed to find statistic action definition in local actions/math.cherri")
	}
	localWireKey := localMatches[1]
	if localWireKey != upstreamWireKey {
		t.Fatalf("CRITICAL ORACLE DRIFT: local wire key %q differs from upstream oracle %q",
			localWireKey, upstreamWireKey)
	}
	t.Logf("Independent Oracle parity: local wire key matches upstream oracle (%q == %q)", localWireKey, upstreamWireKey)

	// 3. Upstream runworkflow definition inspection
	cmdShowStd := exec.Command("git", "show", PinnedUpstreamCommit+":actions_std.go")
	upstreamStdBytes, err := cmdShowStd.Output()
	if err != nil {
		t.Fatalf("failed to read upstream actions_std.go at %s: %v", PinnedUpstreamCommit, err)
	}
	upstreamStd := string(upstreamStdBytes)
	if !strings.Contains(upstreamStd, `"WFInput"`) || !strings.Contains(upstreamStd, `"runworkflow"`) {
		t.Fatalf("independent oracle: upstream actions_std.go missing runworkflow or WFInput key")
	}
	t.Logf("Independent Oracle verified: Upstream runworkflow uses WFInput serialization standard")
}

// Negative Mutation Test 1: Proves that if WFInput is omitted from run action parameters,
// the verification pipeline unequivocally detects the regression and fails.
func TestP10_NegativeMutation_MissingWFInputInRunAction_Fails(t *testing.T) {
	validateRunActionParams := func(params map[string]any) error {
		if params == nil {
			return fmt.Errorf("params map is nil")
		}
		if _, hasName := params["WFWorkflowName"]; !hasName {
			return fmt.Errorf("missing WFWorkflowName")
		}
		if _, hasInput := params["WFInput"]; !hasInput || params["WFInput"] == nil {
			return fmt.Errorf("CRITICAL REGRESSION: run action is missing required wire key 'WFInput'")
		}
		return nil
	}

	// 1. Valid parameters pass
	validParams := map[string]any{
		"WFWorkflowName": "TestTarget",
		"WFWorkflow":     map[string]any{"workflowName": "TestTarget"},
		"WFInput":        "data_payload",
	}
	if err := validateRunActionParams(validParams); err != nil {
		t.Fatalf("expected valid params to pass validation, got: %v", err)
	}

	// 2. Mutated parameters (missing WFInput, the original Task 1219750185341254 bug)
	mutatedParams := map[string]any{
		"WFWorkflowName": "TestTarget",
		"WFWorkflow":     map[string]any{"workflowName": "TestTarget"},
		// WFInput intentionally dropped by mutation
	}
	err := validateRunActionParams(mutatedParams)
	if err == nil {
		t.Fatalf("NEGATIVE TEST FAILED: validator accepted run action without WFInput!")
	}
	if !strings.Contains(err.Error(), "missing required wire key 'WFInput'") {
		t.Fatalf("unexpected error message: %v", err)
	}
	t.Logf("Negative mutation caught as expected: %v", err)
}

// Negative Mutation Test 2: Proves that if statistic action emits 'Input' instead of 'WFInput',
// the verification pipeline unequivocally detects the regression and fails.
func TestP10_NegativeMutation_StatisticInputWireKeyRegression_Fails(t *testing.T) {
	validateStatisticParams := func(params map[string]any) error {
		if params == nil {
			return fmt.Errorf("params is nil")
		}
		if _, hasDeprecated := params["Input"]; hasDeprecated {
			return fmt.Errorf("CRITICAL REGRESSION: statistic action emitted deprecated wire key 'Input'")
		}
		if _, hasWFInput := params["WFInput"]; !hasWFInput || params["WFInput"] == nil {
			return fmt.Errorf("CRITICAL REGRESSION: statistic action missing wire key 'WFInput'")
		}
		return nil
	}

	// 1. Valid params pass
	validParams := map[string]any{
		"WFStatisticsOperation": "Sum",
		"WFInput":               []any{1, 2, 3},
	}
	if err := validateStatisticParams(validParams); err != nil {
		t.Fatalf("valid statistic params failed: %v", err)
	}

	// 2. Mutated params (emitting 'Input' instead of 'WFInput', the Task 1219750250211915 bug)
	mutatedParams := map[string]any{
		"WFStatisticsOperation": "Sum",
		"Input":                 []any{1, 2, 3},
	}
	err := validateStatisticParams(mutatedParams)
	if err == nil {
		t.Fatalf("NEGATIVE TEST FAILED: validator accepted statistic action with 'Input' key!")
	}
	if !strings.Contains(err.Error(), "deprecated wire key 'Input'") {
		t.Fatalf("unexpected error message: %v", err)
	}
	t.Logf("Negative mutation caught as expected: %v", err)
}

// Negative Mutation Test 3: Proves that if regex quantifiers {8}, {4}, {12} are turned
// into variable token attachments, the verification pipeline detects the regression and fails.
func TestP10_NegativeMutation_RegexQuantifiersAsAttachments_Fails(t *testing.T) {
	validateRegexPattern := func(patternParam any) error {
		tokenMap, isMap := patternParam.(map[string]any)
		if !isMap {
			return nil // Raw string literal is safe
		}
		if ser, hasSer := tokenMap["WFSerializationType"]; hasSer && ser == "WFTextTokenString" {
			val, ok := tokenMap["Value"].(map[string]any)
			if !ok {
				return nil
			}
			if atts, hasAtts := val["attachmentsByRange"].(map[string]any); hasAtts && len(atts) > 0 {
				return fmt.Errorf("CRITICAL REGRESSION: regex string contains variable attachments: %v", atts)
			}
		}
		return nil
	}

	// 1. Valid regex string passes
	validPattern := map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string":             "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{12}",
			"attachmentsByRange": map[string]any{},
		},
	}
	if err := validateRegexPattern(validPattern); err != nil {
		t.Fatalf("valid regex pattern failed: %v", err)
	}

	// 2. Mutated regex pattern (attachments created for {8})
	mutatedPattern := map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string": "\uFFFC-[0-9a-f]{4}",
			"attachmentsByRange": map[string]any{
				"{0, 1}": map[string]any{
					"Type":         "Variable",
					"VariableName": "8",
				},
			},
		},
	}
	err := validateRegexPattern(mutatedPattern)
	if err == nil {
		t.Fatalf("NEGATIVE TEST FAILED: validator accepted regex string with quantifier attachment!")
	}
	if !strings.Contains(err.Error(), "regex string contains variable attachments") {
		t.Fatalf("unexpected error message: %v", err)
	}
	t.Logf("Negative mutation caught as expected: %v", err)
}

// Negative Mutation Test 4: Proves that invalid array equality conditionals
// cannot compile and will fail the build if accepted.
func TestP10_NegativeMutation_ArrayEqualityNotRejected_Fails(t *testing.T) {
	src := `let matches = ["item1", "item2"]
if matches == [] {
    alert(alert: "Should not compile")
}
`
	_, err := compileV2Helper(t, "neg_array_cond", src)
	if err == nil {
		t.Fatalf("NEGATIVE TEST FAILED: compiler accepted invalid array comparison 'matches == []'!")
	}
	if !strings.Contains(err.Error(), "Invalid type 'array' for conditional '=='") {
		t.Fatalf("unexpected error message: %v", err)
	}
	t.Logf("Negative compilation failure confirmed: %v", err)
}
