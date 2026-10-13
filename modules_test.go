/*
 * Copyright (c) Cherri Language v2.0
 * Modular Imports Conformance Test Suite (Batch 3 / IM01-IM05 / RP27)
 */

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

// 1. Valid local module import via virtual module calling exported function
func TestMODULES_VirtualModuleFunctionCall(t *testing.T) {
	helperCode := `function addNums(a: Number, b: Number) -> Number {
    return a + b
}
`
	mainCode := `import "./math_helper.cherri" as mathHelper

let res = mathHelper.addNums(10, 20)
`
	virtualMods := map[string]string{
		"./math_helper.cherri": helperCode,
	}

	plistBytes, err := CompileSourceToPlistWithModules("main.cherri", mainCode, virtualMods)
	if err != nil {
		t.Fatalf("failed to compile modular import: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal output plist: %v", err)
	}

	if len(sc.WFWorkflowActions) == 0 {
		t.Fatalf("expected emitted actions for function call, got 0")
	}
}

// 2. Built-in registry module import (e.g. web)
func TestMODULES_BuiltinModuleImport(t *testing.T) {
	code := `import web as w

let url = "https://example.com"
`
	plistBytes, err := CompileSourceToPlist("builtin_mod.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile built-in module import: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}
}

// 3. Duplicate import alias fails closed (IM01)
func TestMODULES_DuplicateImportAliasFails(t *testing.T) {
	code := `import web as w
import text as w
`
	_, err := CompileSourceToPlist("dup_alias.cherri", code)
	if err == nil {
		t.Fatalf("expected error on duplicate import alias, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate import alias") {
		t.Errorf("expected duplicate import alias error, got: %v", err)
	}
}

// 4. Import cycle detection (IM01 / RP27)
func TestMODULES_ImportCycleDetected(t *testing.T) {
	modA := `import "./modB.cherri" as b`
	modB := `import "./modA.cherri" as a`

	virtualMods := map[string]string{
		"./modA.cherri": modA,
		"./modB.cherri": modB,
	}

	mainCode := `import "./modA.cherri" as a`
	_, err := CompileSourceToPlistWithModules("main.cherri", mainCode, virtualMods)
	if err == nil {
		t.Fatalf("expected error on import cycle, got nil")
	}
	if !strings.Contains(err.Error(), "import cycle detected") {
		t.Errorf("expected import cycle error, got: %v", err)
	}
}

// 5. Workspace path traversal security rejection (IM04 / RP29)
func TestMODULES_PathTraversalRejected(t *testing.T) {
	code := `import "../../../etc/passwd" as secret`
	_, err := CompileSourceToPlist("traversal.cherri", code)
	if err == nil {
		t.Fatalf("expected error on path traversal import, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal outside workspace not permitted") {
		t.Errorf("expected path traversal error, got: %v", err)
	}
}

// 6. Remote package import security rejection (IM05 / RP27)
func TestMODULES_RemoteImportRejected(t *testing.T) {
	code := `import "https://malicious.example.com/exploit.cherri" as remotePkg`
	_, err := CompileSourceToPlist("remote.cherri", code)
	if err == nil {
		t.Fatalf("expected error on remote package import, got nil")
	}
	if !strings.Contains(err.Error(), "remote package imports not permitted") {
		t.Errorf("expected remote package import error, got: %v", err)
	}
}

// 7. Module top-level runtime initialization rejection (IM02 / RP27)
func TestMODULES_ModuleTopLevelRuntimeInitRejected(t *testing.T) {
	modWithInit := `let x = 10
function compute() -> Number { return 42 }
`
	virtualMods := map[string]string{
		"./init_mod.cherri": modWithInit,
	}

	mainCode := `import "./init_mod.cherri" as initMod`
	_, err := CompileSourceToPlistWithModules("main.cherri", mainCode, virtualMods)
	if err == nil {
		t.Fatalf("expected error on module with top-level runtime initialization, got nil")
	}
	if !strings.Contains(err.Error(), "module top-level runtime initialization rejected") {
		t.Errorf("expected top-level runtime init rejection error, got: %v", err)
	}
}

// 8. Real file on disk resolution and compilation
func TestMODULES_RealFileOnDiskImport(t *testing.T) {
	tmpDir := filepath.Join(".", "_test_mod_fixture")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("failed to create fixture dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	helperPath := filepath.Join(tmpDir, "string_helper.cherri")
	helperContent := `function greet(name: Text) -> Text {
    return "Hello " + name
}
`
	if err := os.WriteFile(helperPath, []byte(helperContent), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	mainCode := `import "./_test_mod_fixture/string_helper.cherri" as strHelper

let greeting = strHelper.greet("Cherri")
`
	plistBytes, err := CompileSourceToPlist("disk_main.cherri", mainCode)
	if err != nil {
		t.Fatalf("failed to compile on-disk module import: %v", err)
	}

	var sc Shortcut
	if _, err := plist.Unmarshal(plistBytes, &sc); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}
	if len(sc.WFWorkflowActions) == 0 {
		t.Errorf("expected emitted actions for on-disk module import, got 0")
	}
}
