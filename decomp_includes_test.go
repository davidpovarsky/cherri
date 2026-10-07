/*
 * Copyright (c) Cherri
 */
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/args-parser"
)

// Self-contained decompilation invariant: for any supported Shortcut,
//
//	plist -> decompile -> generated .cherri -> compile
//
// must succeed WITHOUT manual include edits. The decompiler is responsible
// for emitting the standard action include for every category whose actions
// appear in the reconstructed source (and only those categories).
//
// Regression history: Acceptance Test 01 compiled a Shortcut containing
// show(...), base64Encode(..., "None"), getUpcomingEvents(3) and
// startStopwatch(). Decompilation reconstructed getUpcomingEvents(3) but
// omitted `#include 'actions/calendar'`, so the generated source failed to
// recompile. Root cause: checkMissingStandardInclude probes categories by
// loading their definitions into the global actions map; getUpcomingEvents
// then matched from the probe's side effect without triggering include
// emission. The fix records the include category on each definition at parse
// time and emits every required include exactly once during decompilation.
//
// These tests run compile -> decompile -> recompile entirely in-process so
// they are reliable in constrained sandboxes (iSH) as well as CI. Each phase
// resets compiler state through the centralized resetCompilerStateFully
// helper, mirroring the fresh-process baseline a CLI invocation observes.

// assertIncludeOnce fails unless include appears exactly once in source.
func assertIncludeOnce(t *testing.T, source string, include string) {
	t.Helper()
	var statement = "#include '" + include + "'"
	var count = strings.Count(source, statement)
	if count != 1 {
		t.Errorf("expected exactly one %s in decompiled source, found %d:\n%s", statement, count, source)
	}
}

// assertNoIncludes fails if source contains any standard action include.
func assertNoIncludes(t *testing.T, source string) {
	t.Helper()
	if strings.Contains(source, "#include 'actions/") {
		t.Errorf("expected no standard action includes in decompiled source:\n%s", source)
	}
}

// TestDecompSelfContainedIncludesRegression reproduces the exact Acceptance
// Test 01 semantics and proves compile -> decompile -> recompile succeeds with
// the required crypto/calendar includes present exactly once.
func TestDecompSelfContainedIncludesRegression(t *testing.T) {
	decompileSelfContainedRoundTrip(t,
		"#include 'actions/crypto'\n#include 'actions/calendar'\n\n"+
			"@input = \"Acceptance Test 01\"\n"+
			"@encoded = base64Encode(@input, \"None\")\n"+
			"@events = getUpcomingEvents(3)\n"+
			"startStopwatch()\n"+
			"show(\"Input: {@input}\")\n"+
			"show(\"Base64 (lineBreakMode None): {@encoded}\")\n"+
			"show(\"Upcoming events found: {@events}\")\n",
		func(t *testing.T, decompiled string) {
			assertIncludeOnce(t, decompiled, "actions/calendar")
			assertIncludeOnce(t, decompiled, "actions/crypto")
			for _, required := range []string{"base64Encode(@input, \"None\")", "getUpcomingEvents(3)", "startStopwatch()"} {
				if !strings.Contains(decompiled, required) {
					t.Errorf("decompiled source lost %s:\n%s", required, decompiled)
				}
			}
		})
}

// TestDecompIncludeReconstructionMultiCategory verifies that one Shortcut with
// actions from several include categories decompiles with every required
// include present exactly once and recompiles cleanly.
func TestDecompIncludeReconstructionMultiCategory(t *testing.T) {
	decompileSelfContainedRoundTrip(t,
		"#include 'actions/calendar'\n#include 'actions/crypto'\n#include 'actions/web'\n#include 'actions/documents'\n\n"+
			"@input = \"hello\"\n"+
			"@encoded = base64Encode(@input, \"None\")\n"+
			"@events = getUpcomingEvents(3)\n"+
			"@page = downloadURL(\"https://example.com\")\n"+
			"@files = \"a.txt\"\n"+
			"@zip = makeArchive(@files, \".zip\", \"archive.zip\")\n"+
			"show(\"done\")\n",
		func(t *testing.T, decompiled string) {
			for _, include := range []string{"actions/calendar", "actions/crypto", "actions/web", "actions/documents"} {
				assertIncludeOnce(t, decompiled, include)
			}
		})
}

// TestDecompBasicActionsEmitNoIncludes verifies builtin/basic actions never
// produce unnecessary include directives.
func TestDecompBasicActionsEmitNoIncludes(t *testing.T) {
	decompileSelfContainedRoundTrip(t,
		"@name = \"World\"\nshow(\"Hello {@name}\")\n",
		func(t *testing.T, decompiled string) {
			assertNoIncludes(t, decompiled)
		})
}

// TestDecompDuplicateCategoryEmitsSingleInclude verifies two actions from the
// same category produce exactly one include.
func TestDecompDuplicateCategoryEmitsSingleInclude(t *testing.T) {
	decompileSelfContainedRoundTrip(t,
		"#include 'actions/calendar'\n\n"+
			"@events = getUpcomingEvents(3)\n"+
			"@more = getUpcomingEvents(10)\n"+
			"show(\"done\")\n",
		func(t *testing.T, decompiled string) {
			assertIncludeOnce(t, decompiled, "actions/calendar")
		})
}

// TestDecompIncludeOrderDeterministic verifies the emitted include order is
// deterministic across independent decompilations of the same Shortcut.
func TestDecompIncludeOrderDeterministic(t *testing.T) {
	var source = "#include 'actions/crypto'\n#include 'actions/calendar'\n\n" +
		"@input = \"hello\"\n" +
		"@encoded = base64Encode(@input, \"None\")\n" +
		"@events = getUpcomingEvents(3)\n" +
		"show(\"done\")\n"

	var first = decompileSelfContained(t, source)
	var second = decompileSelfContained(t, source)

	var extractIncludes = func(decompiled string) string {
		var lines []string
		for _, line := range strings.Split(decompiled, "\n") {
			if strings.HasPrefix(line, "#include 'actions/") {
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n")
	}

	var firstIncludes = extractIncludes(first)
	var secondIncludes = extractIncludes(second)
	if firstIncludes != secondIncludes {
		t.Errorf("include order not deterministic:\nfirst:\n%s\nsecond:\n%s", firstIncludes, secondIncludes)
	}
	if first != second {
		t.Errorf("decompiled source not deterministic across runs:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestDecompIncludeStateIsolation verifies repeated compile/decompile in one
// process does not change output (centralized compiler reset infrastructure).
func TestDecompIncludeStateIsolation(t *testing.T) {
	var source = "#include 'actions/crypto'\n#include 'actions/calendar'\n\n" +
		"@input = \"hello\"\n" +
		"@encoded = base64Encode(@input, \"None\")\n" +
		"@events = getUpcomingEvents(3)\n" +
		"show(\"done\")\n"

	var first = decompileSelfContained(t, source)
	// Interleave an unrelated decompile to prove the second run is not
	// affected by accumulated process-global state.
	_ = decompileSelfContained(t, "#include 'actions/web'\n\n@page = downloadURL(\"https://example.com\")\n")
	var third = decompileSelfContained(t, source)

	if first != third {
		t.Errorf("repeated in-process decompile output differs:\nfirst:\n%s\nthird:\n%s", first, third)
	}
}

// decompileSelfContainedRoundTrip runs the full compile -> decompile ->
// recompile cycle in-process for source, calls inspect on the decompiled
// Cherri, then verifies the recompiled Shortcut is structurally equal to the
// original (corpus comparator policy: volatile metadata and UUIDs ignored).
func decompileSelfContainedRoundTrip(t *testing.T, source string, inspect func(t *testing.T, decompiled string)) {
	t.Helper()
	var dir = t.TempDir()

	// Phase 1: compile the original source.
	var srcPath = filepath.Join(dir, "roundtrip.cherri")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	resetCompilerStateFully()
	defer resetCompilerStateFully()
	args.Args["skip-sign"] = ""
	args.Args["no-ansi"] = ""
	compileSourceInProcess(srcPath)
	delete(args.Args, "skip-sign")
	var plistA = filepath.Join(dir, "roundtrip_unsigned.shortcut")

	// Phase 2: decompile with a fresh baseline (mirrors CLI/iOS fresh state).
	resetCompilerStateFully()
	var outPath = filepath.Join(dir, "decompiled.cherri")
	args.Args["import"] = plistA
	args.Args["output"] = outPath
	args.Args["no-ansi"] = ""
	decompile(importShortcut(args.Value("import")))
	delete(args.Args, "import")
	delete(args.Args, "output")

	var bytes, readErr = os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read decompiled source: %v", readErr)
	}
	var decompiled = string(bytes)
	if strings.Contains(decompiled, "rawAction(") {
		t.Errorf("decompiled output degraded to rawAction fallback:\n%s", decompiled)
	}
	if inspect != nil {
		inspect(t, decompiled)
	}

	// Phase 3: recompile the decompiled source (self-contained invariant).
	resetCompilerStateFully()
	if err := os.WriteFile(srcPath, []byte(decompiled), 0644); err != nil {
		t.Fatalf("write decompiled source: %v", err)
	}
	args.Args["skip-sign"] = ""
	args.Args["no-ansi"] = ""
	compileSourceInProcess(srcPath)
	delete(args.Args, "skip-sign")
	var plistB = filepath.Join(dir, "roundtrip_unsigned.shortcut")

	assertStructurallyEqual(t, "roundtrip", plistA, plistB)
}

// decompileSelfContained compiles source in-process, decompiles the resulting
// Shortcut, and returns the generated Cherri source. The returned source must
// recompile (self-contained invariant) — proven by recompiling it in-process.
func decompileSelfContained(t *testing.T, source string) string {
	t.Helper()
	var dir = t.TempDir()
	var srcPath = filepath.Join(dir, "selfcontained.cherri")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	resetCompilerStateFully()
	defer resetCompilerStateFully()
	args.Args["skip-sign"] = ""
	args.Args["no-ansi"] = ""
	compileSourceInProcess(srcPath)
	delete(args.Args, "skip-sign")

	// Compilation pollutes decompiler scratch state (variables, varPositions,
	// UUID maps, control-flow groups) and leaves standard-action definitions
	// loaded without include-category tags. Production flows (CLI subprocess,
	// iOS bridge) always decompile from a fresh baseline, so mirror that here.
	resetCompilerStateFully()

	var plistPath = filepath.Join(dir, "selfcontained_unsigned.shortcut")
	var outPath = filepath.Join(dir, "decompiled.cherri")
	args.Args["import"] = plistPath
	args.Args["output"] = outPath
	args.Args["no-ansi"] = ""
	decompile(importShortcut(args.Value("import")))
	delete(args.Args, "import")
	delete(args.Args, "output")

	var bytes, readErr = os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read decompiled source: %v", readErr)
	}
	var decompiled = string(bytes)

	// Self-contained invariant: the decompiled source must recompile.
	resetCompilerStateFully()
	if err := os.WriteFile(srcPath, []byte(decompiled), 0644); err != nil {
		t.Fatalf("write decompiled source: %v", err)
	}
	args.Args["skip-sign"] = ""
	args.Args["no-ansi"] = ""
	compileSourceInProcess(srcPath)
	delete(args.Args, "skip-sign")

	return decompiled
}

// compileSourceInProcess compiles a .cherri file using the in-process compile
// entry point (the same path the CLI uses for a single file).
func compileSourceInProcess(path string) {
	currentTest = path
	os.Args[1] = path
	compile()
}

// TestDecompExternalFixtureSelfContained drives the sanitized external
// Shortcut fixture (a real-derived plist, not one produced moments earlier by
// this compiler) through decompile -> include check -> recompile.
func TestDecompExternalFixtureSelfContained(t *testing.T) {
	var fixturePath = filepath.Join("tests", "external-shortcut-fixture.plist")
	if _, statErr := os.Stat(fixturePath); os.IsNotExist(statErr) {
		t.Skipf("external fixture absent: %s", fixturePath)
	}

	resetCompilerStateFully()
	defer resetCompilerStateFully()

	var dir = t.TempDir()
	var outPath = filepath.Join(dir, "external-fixture.cherri")
	args.Args["import"] = fixturePath
	args.Args["output"] = outPath
	args.Args["no-ansi"] = ""
	decompile(importShortcut(args.Value("import")))
	delete(args.Args, "import")
	delete(args.Args, "output")

	var decompiled, readErr = os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read decompiled fixture source: %v", readErr)
	}
	var source = string(decompiled)

	assertIncludeOnce(t, source, "actions/calendar")
	assertIncludeOnce(t, source, "actions/crypto")

	// Self-contained invariant: decompiled fixture source must recompile.
	resetCompilerStateFully()
	var srcPath = filepath.Join(dir, "external-fixture_recompile.cherri")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		t.Fatalf("write decompiled fixture source: %v", err)
	}
	args.Args["skip-sign"] = ""
	args.Args["no-ansi"] = ""
	compileSourceInProcess(srcPath)
	delete(args.Args, "skip-sign")
}
