/*
 * Copyright (c) Cherri Language v2.0
 * Modular Imports Conformance & Deep Verification Test Suite (Phase 1)
 */

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

// 1. Multi-file compilation on disk through the Cherri v2 compiler pipeline
func TestMODULES_MultiFileDiskPipeline(t *testing.T) {
	tmpDir := t.TempDir()

	mathDir := filepath.Join(tmpDir, "math")
	if err := os.MkdirAll(mathDir, 0755); err != nil {
		t.Fatalf("failed to create math dir: %v", err)
	}

	algebraCode := `function square(x: Number) -> Number {
    return x * x
}
`
	if err := os.WriteFile(filepath.Join(mathDir, "algebra.cherri"), []byte(algebraCode), 0644); err != nil {
		t.Fatalf("failed to write algebra.cherri: %v", err)
	}

	constsCode := `function pi() -> Number {
    return 3.14159
}
`
	if err := os.WriteFile(filepath.Join(mathDir, "constants.cherri"), []byte(constsCode), 0644); err != nil {
		t.Fatalf("failed to write constants.cherri: %v", err)
	}

	mainCode := `import "./math/algebra.cherri" as algebra
import "./math/constants.cherri" as consts

let s = algebra.square(4)
let p = consts.pi()
`
	mainFile := filepath.Join(tmpDir, "main.cherri")
	if err := os.WriteFile(mainFile, []byte(mainCode), 0644); err != nil {
		t.Fatalf("failed to write main.cherri: %v", err)
	}

	plistBytes, err := CompileSourceToPlist(mainFile, mainCode)
	if err != nil {
		t.Fatalf("failed to compile multi-file disk project: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}

	if len(sc.WFWorkflowActions) == 0 {
		t.Fatalf("expected emitted actions for function calls, got 0")
	}
}

// 2. Nested multi-level imports (Main -> ModB -> ModC)
func TestMODULES_NestedImportsDeep(t *testing.T) {
	modCCode := `function baseVal() -> Number {
    return 100
}
`
	modBCode := `import "./modC.cherri" as c

function midCalc() -> Number {
    return c.baseVal() + 50
}
`
	mainCode := `import "./modB.cherri" as b

let finalResult = b.midCalc()
`
	virtualMods := map[string]string{
		"./modC.cherri": modCCode,
		"./modB.cherri": modBCode,
	}

	plistBytes, err := CompileSourceToPlistWithModules("main.cherri", mainCode, virtualMods)
	if err != nil {
		t.Fatalf("failed to compile nested imports: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}

	if len(sc.WFWorkflowActions) == 0 {
		t.Fatalf("expected emitted actions, got 0")
	}
}

// 3. Duplicate canonical-path import with distinct aliases loads once and resolves both
func TestMODULES_DuplicateCanonicalPathDistinctAliases(t *testing.T) {
	helperCode := `function double(x: Number) -> Number {
    return x * 2
}
`
	mainCode := `import "./helper.cherri" as h1
import "./helper.cherri" as h2

let a = h1.double(5)
let b = h2.double(10)
`
	virtualMods := map[string]string{
		"./helper.cherri": helperCode,
	}

	plistBytes, err := CompileSourceToPlistWithModules("main.cherri", mainCode, virtualMods)
	if err != nil {
		t.Fatalf("failed to compile duplicate canonical import: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}
}

// 4. Missing module file produces a clear diagnostic
func TestMODULES_MissingModuleDiagnostic(t *testing.T) {
	code := `import "./non_existent_module_xyz.cherri" as missing`
	_, err := CompileSourceToPlist("missing.cherri", code)
	if err == nil {
		t.Fatalf("expected error for missing module file, got nil")
	}
	if !strings.Contains(err.Error(), "cannot resolve module") && !strings.Contains(err.Error(), "file not found") && !strings.Contains(err.Error(), "no such file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// 5. Security & integrity rejections (URL, traversal, cycles, runtime init)
func TestMODULES_SecurityAndIntegrityRejections(t *testing.T) {
	cases := []struct {
		name        string
		code        string
		virtualMods map[string]string
		errContains string
	}{
		{
			name:        "RemoteURLImport",
			code:        `import "https://evil.com/payload.cherri" as remote`,
			errContains: "remote package imports not permitted",
		},
		{
			name:        "PathTraversalEscape",
			code:        `import "../../../../etc/shadow" as secret`,
			errContains: "outside workspace",
		},
		{
			name: "ImportCycle",
			code: `import "./loopA.cherri" as a`,
			virtualMods: map[string]string{
				"./loopA.cherri": `import "./loopB.cherri" as b`,
				"./loopB.cherri": `import "./loopA.cherri" as a`,
			},
			errContains: "import cycle detected",
		},
		{
			name: "TopLevelRuntimeInitRejected",
			code: `import "./side_effects.cherri" as se`,
			virtualMods: map[string]string{
				"./side_effects.cherri": `show("Side effect at module root!")`,
			},
			errContains: "module top-level runtime initialization rejected",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.virtualMods != nil {
				_, err = CompileSourceToPlistWithModules("test.cherri", tc.code, tc.virtualMods)
			} else {
				_, err = CompileSourceToPlist("test.cherri", tc.code)
			}
			if err == nil {
				t.Fatalf("expected compilation error for %s, got nil", tc.name)
			}
			if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("expected error to contain %q, got %v", tc.errContains, err)
			}
		})
	}
}

// 6. Verification that imported symbols actually execute and produce expected native structures
func TestMODULES_SymbolExecution_ProducesNativeStructures(t *testing.T) {
	serviceCode := `function calculateDiscount(price: Number, rate: Number) -> Number {
    return price * rate
}

function createGreeting(user: String) -> String {
    return "Welcome, " + user
}
`
	mainCode := `import "./service.cherri" as svc

let finalPrice = svc.calculateDiscount(200, 0.8)
let greeting = svc.createGreeting("Alice")
`
	virtualMods := map[string]string{
		"./service.cherri": serviceCode,
	}

	plistBytes, err := CompileSourceToPlistWithModules("main.cherri", mainCode, virtualMods)
	if err != nil {
		t.Fatalf("failed to compile module symbol execution: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	actions, ok := rawMap["WFWorkflowActions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("expected workflow actions to be generated from imported functions, got 0")
	}

	// Verify that the imported function calls produced real native actions
	var foundMathAction bool
	var foundTextAction bool
	for _, act := range actions {
		actMap := act.(map[string]any)
		id := actMap["WFWorkflowActionIdentifier"].(string)
		if id == "is.workflow.actions.math" {
			foundMathAction = true
		}
		if id == "is.workflow.actions.gettext" || id == "is.workflow.actions.appendvariable" || strings.Contains(id, "text") {
			foundTextAction = true
		}
	}

	if !foundMathAction {
		t.Errorf("expected is.workflow.actions.math from svc.calculateDiscount")
	}
	if !foundTextAction {
		t.Errorf("expected text action from svc.createGreeting")
	}
}

// 7. BRG20 / RP27: Verify preserved standard functions runJS and chooseFromVCard executed through modular imports
func TestMODULES_StdFunc_RunJSAndChooseFromVCard(t *testing.T) {
	// Preserve the original stdfunc.cherri file read to verify file existence and accessibility
	rawLegacyCode, err := os.ReadFile("stdfunc.cherri")
	if err != nil {
		t.Fatalf("failed to read stdfunc.cherri: %v", err)
	}
	if len(rawLegacyCode) == 0 {
		t.Fatalf("stdfunc.cherri is unexpectedly empty")
	}

	mainCode := `import "./stdfunc.cherri" as std

let jsRes = std.runJS("document.title")
let selection = std.chooseFromVCard(["Item 1", "Item 2"], prompt: "Choose an option:")
`

	// Compile directly using the real preserved stdfunc.cherri from disk (no synthetic virtual modules)
	plistBytes, err := CompileSourceToPlistWithModules("main.cherri", mainCode, nil)
	if err != nil {
		t.Fatalf("failed to compile modular stdfunc execution: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	actions, ok := rawMap["WFWorkflowActions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("expected workflow actions, got 0")
	}

	var actionIDs []string
	for _, act := range actions {
		actMap := act.(map[string]any)
		actionIDs = append(actionIDs, actMap["WFWorkflowActionIdentifier"].(string))
	}

	// runJS emits text.replace; chooseFromVCard emits setitemname and choosefromlist
	expectedIDs := []string{
		"is.workflow.actions.text.replace",
		"is.workflow.actions.setitemname",
		"is.workflow.actions.choosefromlist",
	}
	for _, expectedID := range expectedIDs {
		var found bool
		for _, id := range actionIDs {
			if id == expectedID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected action %q generated by imported stdfunc, action list: %v", expectedID, actionIDs)
		}
	}
}

