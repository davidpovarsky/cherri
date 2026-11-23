package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

// rp41MobilePreviewActions uses the same unsigned V2 producer as CherriCompileMode.
// It retains every emitted native action; no AST index or synthetic graph is supplied.
func rp41MobilePreviewActions(t *testing.T, src string, retained ...*mobileCompileResponse) []ShortcutAction {
	t.Helper()
	response := compileForMobileMode(src, "RP41PreviewSourceEdit", "v2", false)
	if !response.OK || response.Mode != "v2" || response.PlistBase64 == "" || response.SignedBase64 != "" {
		t.Fatalf("actual unsigned mobile compile failed: %+v", response)
	}
	for _, target := range retained {
		*target = response
	}
	raw, err := base64.StdEncoding.DecodeString(response.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	root, err := decodeShortcutRawMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	wire, ok := root["WFWorkflowActions"].([]any)
	if !ok || len(wire) == 0 {
		t.Fatalf("actual mobile result has no native action array: %T", root["WFWorkflowActions"])
	}
	actions := make([]ShortcutAction, 0, len(wire))
	for i, entry := range wire {
		action, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("native action %d is not a dictionary: %T", i, entry)
		}
		identifier, ok := action["WFWorkflowActionIdentifier"].(string)
		if !ok || identifier == "" {
			t.Fatalf("native action %d has no identifier", i)
		}
		parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
		actions = append(actions, ShortcutAction{WFWorkflowActionIdentifier: identifier, WFWorkflowActionParameters: parameters})
	}
	t.Logf("actual mobile source_sha256=%x unsigned_plist_sha256=%x native_actions=%d", sha256.Sum256([]byte(src)), sha256.Sum256(raw), len(actions))
	return actions
}

// The selected literal is taken from an independently compiled one-call source.
// Matching its exact native parameter value does not invent a source/native map.
func rp41NativeLiteralIndex(t *testing.T, actions []ShortcutAction, identifier, wireKey string, literal any) int {
	t.Helper()
	indices := []int{}
	for i, action := range actions {
		value, exists := action.WFWorkflowActionParameters[wireKey]
		if action.WFWorkflowActionIdentifier == identifier && exists && reflect.DeepEqual(value, literal) {
			indices = append(indices, i)
		}
	}
	if len(indices) != 1 {
		t.Fatalf("expected one actual native target for %s/%s, got indices %v", identifier, wireKey, indices)
	}
	return indices[0]
}

// RP41 component regression only: actual native preview indices must not be
// mistaken for top-level AST statement indices. UI publication and the separate
// BRG28 benchmark remain outside this Go test's evidence scope.
func TestP15RP41NativePreviewIndexSourceEdit(t *testing.T) {
	const simple = "// Preserve שלום 🍒 and spacing\nwait(2) // editable call\n// Keep the footer\n"
	const expanded = `// Preserve the original comment: שלום 🍒
if (true) {
	wait(1)
}

// Only the next authored Wait parameter may change.
wait(2) // editable call
wait(3)
wait(4)
wait(5)
wait(6)
wait(7)
wait(8)
// Preserve the final comment and all unrelated calls.
`
	// Wait is only compiled, never executed. Its primitive numeric parameter is
	// eligible for the real WebPreview editor; an integer-to-integer edit also
	// preserves that editor's original Number.isInteger guard.
	const replacement = "9"
	waitAction, ok := schema.DefaultRegistry().LookupAction("wait")
	if !ok {
		t.Fatal("canonical Wait action is unavailable")
	}
	parameter, ok := waitAction.PrimaryParameter()
	if !ok || parameter.WireKey == "" {
		t.Fatal("canonical Wait primary wire key is unavailable")
	}
	var baselinePreview mobileCompileResponse
	baseline := rp41MobilePreviewActions(t, simple, &baselinePreview)
	var literal any
	waitCount := 0
	for _, action := range baseline {
		if action.WFWorkflowActionIdentifier == waitAction.AppleIdentifier {
			var exists bool
			literal, exists = action.WFWorkflowActionParameters[parameter.WireKey]
			if !exists {
				t.Fatal("actual baseline Wait has no primary wire parameter")
			}
			waitCount++
		}
	}
	if waitCount != 1 {
		t.Fatalf("one-call baseline emitted %d native Wait actions", waitCount)
	}
	encodedLiteral, err := json.Marshal(literal)
	if err != nil || string(encodedLiteral) != "2" {
		t.Fatalf("actual Wait parameter is not the primitive number editable by WebPreview: %s (%v)", encodedLiteral, err)
	}
	var expandedPreview mobileCompileResponse
	expandedActions := rp41MobilePreviewActions(t, expanded, &expandedPreview)
	checkMapped := func(t *testing.T, src string, actions []ShortcutAction, preview mobileCompileResponse, requireExpansion bool) {
		t.Helper()
		nativeIndex := rp41NativeLiteralIndex(t, actions, waitAction.AppleIdentifier, parameter.WireKey, literal)
		if requireExpansion && nativeIndex == 1 {
			t.Fatal("fixture did not exercise native-index/top-level-statement divergence")
		}
		t.Logf("actual preview selection native_index=%d identifier=%s wire_key=%s", nativeIndex, waitAction.AppleIdentifier, parameter.WireKey)
		updated, err := rp41ApplyPreviewEdit(t, src, preview, nativeIndex, parameter.WireKey, replacement)
		if err != nil {
			t.Fatalf("mapped actual native action %d was rejected: %v", nativeIndex, err)
		}
		want := strings.Replace(src, "wait(2)", "wait("+replacement+")", 1)
		if updated != want {
			t.Fatalf("actual preview index %d edited a different source region\nwant exact source:\n%s\ngot:\n%s", nativeIndex, want, updated)
		}
		rebuilt := rp41MobilePreviewActions(t, updated)
		if len(rebuilt) != len(actions) {
			t.Fatalf("single Wait edit changed native action count: %d -> %d", len(actions), len(rebuilt))
		}
		for i := range actions {
			if rebuilt[i].WFWorkflowActionIdentifier != actions[i].WFWorkflowActionIdentifier {
				t.Fatalf("single Wait edit changed native action identity at %d", i)
			}
		}
	}
	t.Run("mapped_without_expansion", func(t *testing.T) {
		checkMapped(t, simple, baseline, baselinePreview, false)
	})
	t.Run("mapped_after_native_expansion", func(t *testing.T) {
		checkMapped(t, expanded, expandedActions, expandedPreview, true)
	})
	t.Run("generated_condition_value_read_only", func(t *testing.T) {
		// The Bool condition's Number producer is generated from a literal, not
		// a direct source action call. Its native numeric/text field is not one
		// of WebPreview's ignored structural keys such as WFControlFlowMode.
		const key = "WFNumberActionNumber"
		indices := []int{}
		for i, action := range expandedActions {
			_, exists := action.WFWorkflowActionParameters[key]
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.number" && exists {
				indices = append(indices, i)
			}
		}
		if len(indices) != 1 {
			t.Fatalf("actual Bool condition must have one generated Number action, got %v", indices)
		}
		nativeIndex := indices[0]
		encoded, err := json.Marshal(expandedActions[nativeIndex].WFWorkflowActionParameters[key])
		if err != nil {
			t.Fatal(err)
		}
		var primitive any
		if err := json.Unmarshal(encoded, &primitive); err != nil {
			t.Fatal(err)
		}
		var newExpr string
		switch primitive.(type) {
		case string:
			newExpr = "\"0\""
		case float64:
			newExpr = "0"
		default:
			t.Fatalf("generated field is not a WebPreview-editable primitive: %s", encoded)
		}
		t.Logf("actual generated selection native_index=%d identifier=is.workflow.actions.number wire_key=%s", nativeIndex, key)
		updated, err := rp41ApplyPreviewEdit(t, expanded, expandedPreview, nativeIndex, key, newExpr)
		if err == nil || strings.TrimSpace(err.Error()) == "" {
			t.Fatal("generated native value edit needs an explanatory rejection")
		}
		if updated != expanded {
			t.Fatalf("generated native value edit modified source despite rejection:\n%s", updated)
		}
	})
}
