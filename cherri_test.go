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
	"time"

	"github.com/electrikmilk/args-parser"
)

var currentTest string

func TestCherri(_ *testing.T) {
	args.Args["no-ansi"] = ""
	args.Args["comments"] = ""
	var files, err = os.ReadDir("tests")
	if err != nil {
		fmt.Println(ansi("FAILED: unable to read tests directory", red))
		panic(err)
	}
	// Fresh-process baseline per invocation (repeated runs via -count>N must
	// not inherit enumerations/definitions from a previous iteration);
	// fixtures then accumulate definitions among themselves as before.
	resetParser()
	loadStandardActions()
	for _, file := range files {
		if !strings.Contains(file.Name(), ".cherri") || file.Name() == "decomp-expected.cherri" || file.Name() == "decomp-me.cherri" {
			continue
		}
		currentTest = fmt.Sprintf("tests/%s", file.Name())
		os.Args[1] = currentTest
		fmt.Println(ansi(currentTest, underline, bold))

		compile()

		fmt.Println(ansi("✅  PASSED", green, bold))
		fmt.Print("\n")

		// Scratch-state-only reset: fixtures may rely on standard-action
		// definitions accumulated by earlier fixtures, matching the
		// documented order-dependence caveat; isolation-sensitive suites
		// use resetParser() (full baseline) instead.
		resetCompilerState()
		loadStandardActions()

		if signFailed {
			fmt.Println(ansi("Using remote service HubSign", cyan, bold))
			for i := 5; i > 0; i-- {
				fmt.Print(ansi(fmt.Sprintf("Respectfully waiting %d second(s) between tests...\r", i), cyan))
				time.Sleep(1 * time.Second)
			}
			fmt.Print("\n")
		}
	}
}

func TestCherriNoSign(t *testing.T) {
	args.Args["skip-sign"] = ""
	TestCherri(t)
}

func TestPackages(t *testing.T) {
	args.Args["no-ansi"] = ""

	// Snapshot pre-existing package artifacts so this test can never leave a
	// root manifest/packages directory behind: a stray manifest makes every
	// later compilation auto-install dependencies relative to its source dir.
	var previousInfoPlistExisted = false
	if _, statErr := os.Stat("info.plist"); statErr == nil {
		previousInfoPlistExisted = true
	}
	t.Cleanup(func() {
		if !previousInfoPlistExisted {
			if _, statErr := os.Stat("info.plist"); statErr == nil {
				if removeErr := os.Remove("info.plist"); removeErr != nil {
					t.Logf("cleanup info.plist: %v", removeErr)
				}
			}
		}
	})

	if _, statErr := os.Stat("info.plist"); !os.IsNotExist(statErr) {
		var removeErr = os.Remove("info.plist")
		handle(removeErr)
	}

	if _, statErr := os.Stat("./packages"); !os.IsNotExist(statErr) {
		var removeDirErr = os.RemoveAll("./packages")
		handle(removeDirErr)
	}

	args.Args["init"] = "@electrikmilk/package-test"
	initPackage()
	delete(args.Args, "init")

	args.Args["install"] = "https://github.com/electrikmilk/package-example"
	input := []byte("y")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.Write(input)
	if err != nil {
		t.Error(err)
	}
	err = w.Close()
	if err != nil {
		panic(err)
	}

	// Restore stdin right after the test.
	defer func(v *os.File) { os.Stdin = v }(os.Stdin)
	os.Stdin = r

	addPackage()
	fmt.Println("You entered:", string(input))
	delete(args.Args, "install")

	listPackage()

	listPackages()

	args.Args["remove"] = "@electrikmilk/package-example"
	removePackage()
	delete(args.Args, "remove")
}

func TestDecomp(t *testing.T) {
	resetParser()
	defer resetParser()

	fmt.Println("Decompiling...")
	args.Args["import"] = "tests/decomp-me.plist"
	decompile(importShortcut(args.Value("import")))

	fmt.Println("Comparing to expected...")
	var bytes, readErr = os.ReadFile("tests/decomp-expected.cherri")
	handle(readErr)

	if code.String() != string(bytes) {
		fmt.Println(ansi("Does not match expected!", red, bold))
		t.Fail()
		return
	}
	fmt.Print(ansi("✅  PASSED", green, bold) + "\n\n")
}

// TestRoundTrip decompiles compiler-generated plists to verify the full compile→decompile
// pipeline. The _unsigned.shortcut files are produced by TestCherriNoSign; individual
// sub-tests skip gracefully when the file is absent rather than failing.
//
// Run independently:
//
//	go test -run TestCherriNoSign && go test -run TestRoundTrip
func TestRoundTrip(t *testing.T) {
	args.Args["no-ansi"] = ""

	// Chosen because their plist structures are within the decompiler's action handlers.
	var candidates = []string{
		"tests/math_unsigned.shortcut",
		"tests/numbers_unsigned.shortcut",
		"tests/repeats_unsigned.shortcut",
		"tests/conditionals_unsigned.shortcut",
		"tests/dictionary_unsigned.shortcut",
		"tests/variables_unsigned.shortcut",
	}

	for _, plistPath := range candidates {
		t.Run(plistPath, func(t *testing.T) {
			defer resetParser()

			if _, statErr := os.Stat(plistPath); os.IsNotExist(statErr) {
				t.Skipf("compile output absent — run TestCherriNoSign first: %s", plistPath)
			}

			// Direct decompiler output to a temporary directory so no .cherri
			// files land in tests/, which would be picked up and compiled by
			// TestCherriNoSign. os.DevNull is not a writable file path on Windows.
			var decompOutput = filepath.Join(t.TempDir(), "decompiled.cherri")
			args.Args["output"] = decompOutput
			args.Args["import"] = plistPath
			decompile(importShortcut(args.Value("import")))
			delete(args.Args, "output")

			if code.Len() == 0 {
				t.Errorf("decompile of %s produced empty output", plistPath)
			}
		})
	}
}

func TestActionIdentifiers(t *testing.T) {
	args.Args["no-ansi"] = ""
	args.Args["skip-sign"] = ""
	delete(args.Args, "comments")
	resetParser()
	loadStandardActions()

	currentTest = "tests/zz-action-identifiers.cherri"
	os.Args[1] = currentTest

	compile()

	var expected = []string{
		"is.workflow.actions.shortid",
		"is.workflow.actions.two.parts",
		"is.workflow.actions.text.match.getgroup",
		"notion.id.CreatePageIntent",
		"com.apple.facetime.facetime",
	}

	var actual []string
	for _, a := range shortcut.WFWorkflowActions {
		actual = append(actual, a.WFWorkflowActionIdentifier)
	}

	if len(actual) != len(expected) {
		t.Fatalf("Expected %d actions, got %d: %v", len(expected), len(actual), actual)
	}

	for i, ident := range expected {
		if actual[i] != ident {
			t.Errorf("Action %d: got %q, want %q", i, actual[i], ident)
		}
	}

	resetParser()
}

func TestCapitalizeEmptyString(t *testing.T) {
	if got := capitalize(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestSanitizeIdentifierWhitespaceOnly(t *testing.T) {
	identifier := " "
	sanitizeIdentifier(&identifier)

	if identifier != "" {
		t.Fatalf("expected sanitized identifier to be empty, got %q", identifier)
	}
}

func compile() {
	defer func() {
		if recover() != nil {
			panicDebug(nil)
		}
	}()

	main()
}

func resetParser() {
	resetCompilerStateFully()
}
