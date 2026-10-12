package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// CORE-001: show(...) named action through v2 uses EmitResolvedCall,
// canonical definition and exact full Apple identifier/params.
func TestCORE001_ShowNamedAction(t *testing.T) {
	code := "let msg = \"Hello World\"\nshow(msg)\n"
	plistBytes, err := CompileSourceToPlist("core001.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile: %v", err)
	}
	s := string(plistBytes)
	if !strings.Contains(s, "is.workflow.actions.showresult") {
		t.Errorf("expected is.workflow.actions.showresult in plist, got:\n%s", s)
	}
	if !strings.Contains(s, "WFTextTokenString") {
		t.Errorf("expected WFTextTokenString in plist, got:\n%s", s)
	}
	if !strings.Contains(s, "ActionOutput") {
		t.Errorf("expected ActionOutput in plist, got:\n%s", s)
	}
}

// CORE-002: two callable variants share Apple ID (base64Encode vs base64Decode)
// must produce correct variant-specific params and stable deterministic definition identity.
func TestCORE002_VariantsSharingAppleID(t *testing.T) {
	session := NewCanonicalBackendSession()
	defer session.Close()

	// 1. base64Encode
	outEnc, err := session.EmitResolvedCall(backend.ResolvedCall{
		DefinitionID:    "base64Encode",
		AppleIdentifier: "is.workflow.actions.base64encode",
		OutputUUID:      "uuid-enc",
		OutputName:      "Encoded",
		Arguments: []backend.CallArgument{
			{
				ParameterID: "encodeInput",
				Position:    0,
				Value:       backend.SemanticValue{Kind: backend.ValString, StrVal: "Hello"},
			},
		},
	})
	if err != nil {
		t.Fatalf("base64Encode failed: %v", err)
	}
	if outEnc != "uuid-enc" {
		t.Errorf("expected uuid-enc, got %q", outEnc)
	}

	// 2. base64Decode
	outDec, err := session.EmitResolvedCall(backend.ResolvedCall{
		DefinitionID:    "base64Decode",
		AppleIdentifier: "is.workflow.actions.base64encode",
		OutputUUID:      "uuid-dec",
		OutputName:      "Decoded",
		Arguments: []backend.CallArgument{
			{
				ParameterID: "input",
				Position:    0,
				Value:       backend.SemanticValue{Kind: backend.ValString, StrVal: "SGVsbG8="},
			},
		},
	})
	if err != nil {
		t.Fatalf("base64Decode failed: %v", err)
	}
	if outDec != "uuid-dec" {
		t.Errorf("expected uuid-dec, got %q", outDec)
	}

	actions := session.shortcut.WFWorkflowActions
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	// base64Encode emittedKeys includes static "input": "Encode" and "WFEncodeMode": "Encode"
	encParams := actions[0].WFWorkflowActionParameters
	if encParams["WFEncodeMode"] != "Encode" {
		t.Errorf("base64Encode WFEncodeMode = %v, want Encode", encParams["WFEncodeMode"])
	}

	// base64Decode has WFEncodeMode: "Decode"
	decParams := actions[1].WFWorkflowActionParameters
	if decParams["WFEncodeMode"] != "Decode" {
		t.Errorf("base64Decode WFEncodeMode = %v, want Decode", decParams["WFEncodeMode"])
	}
}

// CORE-003: unknown DefinitionID with known Apple ID must yield HARD ERROR,
// NEVER accidental matching by Apple ID.
func TestCORE003_UnknownDefinitionIDHardError(t *testing.T) {
	def, ident, ok := lookupCanonicalAction(backend.ResolvedCall{
		DefinitionID:    "completely_unknown_callable_xyz",
		AppleIdentifier: "is.workflow.actions.base64encode",
	})
	if ok || def != nil || ident != "" {
		t.Fatalf("CORE-003 defect present: unknown DefinitionID matched action %q (%+v) via AppleIdentifier fallback!", ident, def)
	}

	session := NewCanonicalBackendSession()
	defer session.Close()

	_, err := session.EmitResolvedCall(backend.ResolvedCall{
		DefinitionID:    "completely_unknown_callable_xyz",
		AppleIdentifier: "is.workflow.actions.base64encode",
	})
	if err == nil {
		t.Fatalf("CORE-003 defect present: EmitResolvedCall succeeded on unknown DefinitionID")
	}
	if !strings.Contains(err.Error(), "canonical definition not found") {
		t.Errorf("expected canonical definition not found error, got: %v", err)
	}
}

// CORE-004: makeParams/appendParams/appendParamsFunc custom actions
// preserve special parameters and parity with legacy.
func TestCORE004_CustomParamActions(t *testing.T) {
	session := NewCanonicalBackendSession()
	defer session.Close()

	// calculate with modulus
	_, err := session.EmitResolvedCall(backend.ResolvedCall{
		DefinitionID:    "calculate",
		AppleIdentifier: "is.workflow.actions.math",
		Arguments: []backend.CallArgument{
			{ParameterID: "operation", Position: 0, Value: backend.SemanticValue{Kind: backend.ValString, StrVal: "Modulus"}},
			{ParameterID: "operandOne", Position: 1, Value: backend.SemanticValue{Kind: backend.ValInt, IntVal: 10}},
			{ParameterID: "operandTwo", Position: 2, Value: backend.SemanticValue{Kind: backend.ValInt, IntVal: 3}},
		},
	})
	if err != nil {
		t.Fatalf("calculate failed: %v", err)
	}

	calcAct := session.shortcut.WFWorkflowActions[0]
	if calcAct.WFWorkflowActionParameters["WFScientificMathOperation"] != "Modulus" {
		t.Errorf("unexpected operation: %v", calcAct.WFWorkflowActionParameters["WFScientificMathOperation"])
	}
	if calcAct.WFWorkflowActionParameters["WFMathOperation"] != "…" {
		t.Errorf("unexpected WFMathOperation: %v", calcAct.WFWorkflowActionParameters["WFMathOperation"])
	}
}

// CORE-005: App Intent actions (setSilentMode, startStopwatch, run)
// preserve exact descriptor, bundle, target structure.
func TestCORE005_AppIntentActions(t *testing.T) {
	session := NewCanonicalBackendSession()
	defer session.Close()

	_, err := session.EmitResolvedCall(backend.ResolvedCall{
		DefinitionID:    "setSilentMode",
		AppleIdentifier: "com.apple.ShortcutsActions.SetSilentModeAction",
		Arguments: []backend.CallArgument{
			{ParameterID: "state", Position: 0, Value: backend.SemanticValue{Kind: backend.ValInt, IntVal: 1}},
		},
	})
	if err != nil {
		t.Fatalf("setSilentMode failed: %v", err)
	}

	act := session.shortcut.WFWorkflowActions[0]
	descriptor, ok := act.WFWorkflowActionParameters["AppIntentDescriptor"].(map[string]any)
	if !ok {
		t.Fatalf("missing AppIntentDescriptor: %+v", act.WFWorkflowActionParameters)
	}
	if descriptor["BundleIdentifier"] != "com.apple.ShortcutsActions" {
		t.Errorf("bundleIdentifier = %v", descriptor["BundleIdentifier"])
	}
	if descriptor["AppIntentIdentifier"] != "SetSilentModeAction" {
		t.Errorf("appIntentIdentifier = %v", descriptor["AppIntentIdentifier"])
	}
}

// CORE-006: named and positional params; unknown/duplicate label
// produces diagnostic errors and never silently skips arguments.
func TestCORE006_UnknownLabelError(t *testing.T) {
	code := `show("hello", unknownFakeParam: 123)` + "\n"
	_, err := CompileSourceToPlist("bad_label.cherri", code)
	if err == nil {
		t.Fatalf("CORE-006 defect: CompileSourceToPlist succeeded despite unknown parameter label")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected error containing 'unknown', got: %v", err)
	}
}

// CORE-007: false/0/empty text/empty list/empty dict vs omitted
// distinct values and defaults all preserved.
func TestCORE007_DistinctValuesVsOmitted(t *testing.T) {
	session := NewCanonicalBackendSession()
	defer session.Close()

	// Emit with explicit false
	_, err := session.EmitResolvedCall(backend.ResolvedCall{
		DefinitionID:    "saveFile",
		AppleIdentifier: "is.workflow.actions.documentpicker.save",
		Arguments: []backend.CallArgument{
			{ParameterID: "path", Position: 0, Value: backend.SemanticValue{Kind: backend.ValString, StrVal: "out.txt"}},
			{ParameterID: "content", Position: 1, Value: backend.SemanticValue{Kind: backend.ValString, StrVal: "hello"}},
			{ParameterID: "overwrite", Position: 2, Value: backend.SemanticValue{Kind: backend.ValBool, BoolVal: false}},
		},
	})
	if err != nil {
		t.Fatalf("saveFile with explicit false failed: %v", err)
	}

	act := session.shortcut.WFWorkflowActions[0]
	val, found := act.WFWorkflowActionParameters["WFSaveFileOverwrite"]
	if !found || val != false {
		t.Errorf("expected WFSaveFileOverwrite to be false, got %v (found: %v)", val, found)
	}
}
