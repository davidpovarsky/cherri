/*
 * Copyright (c) Cherri
 */
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	args "github.com/electrikmilk/args-parser"
)

// Regression coverage for compiler global-state isolation:
//
//	neutral -> noisy -> neutral -> noisy -> neutral
//
// Every phase runs in-process and phases are separated ONLY by
// resetCompilerState(). If any per-compilation mutable global survived the
// reset, later neutral compilations would differ from the fresh-process
// reference. Before the centralized helper existed, sequential compiles
// emitted stale WFWorkflowClientVersion ("900") and icon colors
// (-1263359489) planted by the old partial test-only reset.

var isoNeutralSource = `// neutral probe
@t = "neutral"
if @t {
	nothing()
}
`

var isoNoisySource = `#define version 18
#define color red

function ping(): text {
	output("pong")
}

// first comment while functions are defined
const p = ping()
nothing()
`

func isoCompile(t *testing.T, dir string, label string, source string) map[string]any {
	t.Helper()
	loadTestStandardActions()

	var srcPath = filepath.Join(dir, label+".cherri")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		t.Fatalf("%s: write source: %v", label, err)
	}

	var previousArg1 = os.Args[1]
	os.Args[1] = srcPath
	defer func() { os.Args[1] = previousArg1 }()

	compile()

	var plistPath = filepath.Join(dir, label+"_unsigned.shortcut")
	if _, err := os.Stat(plistPath); err != nil {
		if entries, derr := os.ReadDir(dir); derr == nil {
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Fatalf("%s: compiled plist missing (%v); dir contains: %v", label, err, names)
		}
		t.Fatalf("%s: compiled plist missing: %v", label, err)
	}
	return loadRoundTripDocument(t, plistPath)
}

func TestCompileStateIsolation(t *testing.T) {
	var previousNoAnsi, hadNoAnsi = args.Args["no-ansi"]
	var previousSkipSign, hadSkipSign = args.Args["skip-sign"]
	args.Args["no-ansi"] = ""
	args.Args["skip-sign"] = ""
	defer func() {
		restoreArg(args.Args, "no-ansi", previousNoAnsi, hadNoAnsi)
		restoreArg(args.Args, "skip-sign", previousSkipSign, hadSkipSign)
	}()

	var dir = t.TempDir()

	var reference = canonicalizeRoundTripDocument(isoCompile(t, dir, "a1_neutral", isoNeutralSource))

	isoCompile(t, dir, "b1_noisy", isoNoisySource)
	resetParser()
	var afterNoisy = canonicalizeRoundTripDocument(isoCompile(t, dir, "a2_neutral", isoNeutralSource))

	isoCompile(t, dir, "b2_noisy", isoNoisySource)
	resetParser()
	var afterSecondCycle = canonicalizeRoundTripDocument(isoCompile(t, dir, "a3_neutral", isoNeutralSource))

	resetParser()

	if !reflect.DeepEqual(reference, afterNoisy) {
		t.Errorf("state leaked from noisy compilation into subsequent neutral compilation (fresh vs post-noisy)")
	}
	if !reflect.DeepEqual(afterNoisy, afterSecondCycle) {
		t.Errorf("repeated isolated compilations are not stable (post-noisy vs second cycle)")
	}
}

// isoDecompileOnce decompiles the shared reference-bearing fixture and
// returns the generated Cherri source.
func isoDecompileOnce(t *testing.T, plistPath string) string {
	t.Helper()
	decompile(importShortcut(plistPath))
	var output = code.String()
	code.Reset()
	return output
}

// TestDecompileStateIsolation proves repeated in-process decompilations of
// the same reference-bearing plist produce identical output. The fixture is
// full of CustomOutputName references; leaked extractedReferences/references
// previously made skipDuplicateReference drop valid references on every
// pass after the first.
func TestDecompileStateIsolation(t *testing.T) {
	var previousNoAnsi, hadNoAnsi = args.Args["no-ansi"]
	args.Args["no-ansi"] = ""
	defer func() {
		restoreArg(args.Args, "no-ansi", previousNoAnsi, hadNoAnsi)
	}()

	var expectedBytes, err = os.ReadFile(filepath.Join("tests", "decomp-expected.cherri"))
	if err != nil {
		t.Fatalf("read golden decompiled source: %v", err)
	}
	var newlineNormalize = strings.NewReplacer("\r\n", "\n")
	var expected = strings.TrimRight(newlineNormalize.Replace(string(expectedBytes)), "\n")

	var plistPath = filepath.Join("tests", "decomp-me.plist")

	var firstRun = strings.TrimRight(newlineNormalize.Replace(isoDecompileOnce(t, plistPath)), "\n")
	resetParser()
	var secondRun = strings.TrimRight(newlineNormalize.Replace(isoDecompileOnce(t, plistPath)), "\n")
	resetParser()
	var thirdRun = strings.TrimRight(newlineNormalize.Replace(isoDecompileOnce(t, plistPath)), "\n")

	resetParser()

	if secondRun != firstRun {
		t.Errorf("second decompilation differs from the first: extractedReferences/references state leaked")
	}
	if thirdRun != firstRun {
		t.Errorf("third decompilation differs from the first: state leaked across two prior passes")
	}
	if firstRun != expected {
		t.Errorf("decompilation output diverges from golden fixture even on a clean pass")
	}
}
