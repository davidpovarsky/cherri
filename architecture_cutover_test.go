/*
 * Copyright (c) Cherri Language v2.0
 * Architecture & Cutover Tests (Sections 18 & 19)
 */

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"howett.net/plist"
)

// TestArchitecture_A_ProductionCompilerInvokesCanonicalBackend verifies that
// CompileSourceToPlist invokes CanonicalBackendSession.EmitResolvedCall for normal typed actions.
func TestArchitecture_A_ProductionCompilerInvokesCanonicalBackend(t *testing.T) {
	calledAction := ""
	oldHook := backendEmitHook
	backendEmitHook = func(call backend.ResolvedCall) error {
		if call.DefinitionID != "" {
			calledAction = call.DefinitionID
		} else {
			calledAction = call.AppleIdentifier
		}
		return nil
	}
	defer func() { backendEmitHook = oldHook }()

	v2Source := `let message = "Hello from canonical backend"
show(message)
`
	out, err := CompileSourceToPlist("test.cherri", v2Source)
	if err != nil {
		t.Fatalf("unexpected compilation error: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected non-empty plist output")
	}

	if calledAction != "show" && calledAction != "is.workflow.actions.showresult" {
		t.Fatalf("expected CanonicalBackendSession.EmitResolvedCall to be invoked for 'show', got: %q", calledAction)
	}
}

// TestArchitecture_B_DisablingCanonicalBackendBreaksCompilation proves that
// the canonical backend is the real production path: if the canonical backend
// returns a sentinel error, compilation fails with that exact error.
func TestArchitecture_B_DisablingCanonicalBackendBreaksCompilation(t *testing.T) {
	sentinel := errors.New("canonical backend sentinel error: backend disabled for test")
	oldHook := backendEmitHook
	backendEmitHook = func(call backend.ResolvedCall) error {
		return sentinel
	}
	defer func() { backendEmitHook = oldHook }()

	v2Source := `let message = "This must fail"
show(message)
`
	_, err := CompileSourceToPlist("test.cherri", v2Source)
	if err == nil {
		t.Fatalf("expected compilation to fail when canonical backend fails, but got nil error")
	}

	if !strings.Contains(err.Error(), sentinel.Error()) {
		t.Fatalf("expected error to contain sentinel message %q, got: %v", sentinel.Error(), err)
	}
}

// TestArchitecture_C_NativeActionBypassesTypedAction verifies that explicit
// native.action escape bypasses typed action resolution and uses EmitRawAction.
func TestArchitecture_C_NativeActionBypassesTypedAction(t *testing.T) {
	hookCalled := false
	oldHook := backendEmitHook
	backendEmitHook = func(call backend.ResolvedCall) error {
		hookCalled = true
		return nil
	}
	defer func() { backendEmitHook = oldHook }()

	v2Source := `native.action("is.workflow.actions.comment", {"WFCommentActionText": "Native comment escape"})
`
	out, err := CompileSourceToPlist("test.cherri", v2Source)
	if err != nil {
		t.Fatalf("unexpected compilation error for native.action: %v", err)
	}

	if hookCalled {
		t.Fatalf("expected native.action to bypass typed EmitResolvedCall, but backendEmitHook was called")
	}

	var doc map[string]any
	if _, err := plist.Unmarshal(out, &doc); err != nil {
		t.Fatalf("failed to unmarshal emitted plist: %v", err)
	}

	actions, ok := doc["WFWorkflowActions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("expected at least one action emitted")
	}

	firstAction, _ := actions[0].(map[string]any)
	if firstAction["WFWorkflowActionIdentifier"] != "is.workflow.actions.comment" {
		t.Fatalf("expected action identifier is.workflow.actions.comment, got: %v", firstAction["WFWorkflowActionIdentifier"])
	}
}

// TestArchitecture_D_SharedCodecMaintenance proves that both legacy and v2
// compilers share the pure reference codec: modifying the shared codec hook
// alters the emitted plist for BOTH legacy and v2 in the exact same way.
func TestArchitecture_D_SharedCodecMaintenance(t *testing.T) {
	const alteredSentinel = "ALTERED_SHARED_CODEC_TEST_VARIABLE"

	// 1. Compile baseline (without hook)
	v2Src := `var testVar = "value"
show(testVar)
`
	v2Baseline, err := CompileSourceToPlist("test.cherri", v2Src)
	if err != nil {
		t.Fatalf("v2 baseline compile failed: %v", err)
	}
	if strings.Contains(string(v2Baseline), alteredSentinel) {
		t.Fatalf("baseline unexpectedly contains sentinel")
	}

	// 2. Install shared codec hook
	oldCodecHook := SharedCodecHook
	defer func() { SharedCodecHook = oldCodecHook }()

	SharedCodecHook = func(desc *ReferenceDescriptor, val *Value) {
		if val.VariableName == "testVar" {
			val.VariableName = alteredSentinel
		}
	}

	// 3. Compile v2 with hook
	v2Hooked, err := CompileSourceToPlist("test.cherri", v2Src)
	if err != nil {
		t.Fatalf("v2 compile with hook failed: %v", err)
	}
	if !strings.Contains(string(v2Hooked), alteredSentinel) {
		t.Fatalf("v2 output did not observe altered shared codec; expected %q in plist", alteredSentinel)
	}

	// 4. Compile legacy with hook
	legacySrc := `@testVar = "value"
show(@testVar)
`
	legacyHooked := compileLegacyInProcessHelper(t, legacySrc)
	if !strings.Contains(string(legacyHooked), alteredSentinel) {
		t.Fatalf("legacy output did not observe altered shared codec; expected %q in plist", alteredSentinel)
	}
}

func compileLegacyInProcessHelper(t *testing.T, source string) []byte {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "legacy_test.cherri")
	if err := os.WriteFile(f, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write legacy source: %v", err)
	}

	oldArgs := os.Args
	oldArgsMap := make(map[string]string)
	for k, v := range args.Args {
		oldArgsMap[k] = v
	}

	defer func() {
		os.Args = oldArgs
		args.Args = oldArgsMap
	}()

	os.Args = []string{"cherri", f}
	args.Args = map[string]string{
		"skip-sign": "",
		"no-ansi":   "",
	}

	resetParser()
	loadStandardActions()
	compile()

	outPath := filepath.Join(dir, "legacy_test_unsigned.shortcut")
	bytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read compiled legacy shortcut at %s: %v", outPath, err)
	}
	return bytes
}
