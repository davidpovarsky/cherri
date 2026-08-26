/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	args "github.com/electrikmilk/args-parser"
)

// Fork-specific parameter validation is exercised through the normal
// parser/checkArg machinery: these tests compile deliberately invalid sources
// and assert the standard diagnostics fire instead of a successful compile.
// No parallel validation engine is involved.

// The package initializer pre-populates some builtin definitions, so emptiness
// of the actions map cannot indicate whether DSL-defined actions were loaded.
var loadTestActionsOnce sync.Once

func loadTestStandardActions() {
	loadTestActionsOnce.Do(loadStandardActions)
}

// expectCompileError compiles source expecting Cherri's parser/type checker to
// abort with a diagnostic containing wantSubstring.
func expectCompileError(t *testing.T, label string, source string, wantSubstring string) {
	t.Helper()

	loadTestStandardActions()

	var previousNoAnsi, hadNoAnsi = args.Args["no-ansi"]
	delete(args.Args, "no-ansi")
	var previousDebug, hadDebug = args.Args["debug"]
	args.Args["debug"] = ""
	var previousSkipSign, hadSkipSign = args.Args["skip-sign"]
	args.Args["skip-sign"] = ""

	var dir = t.TempDir()
	var file = filepath.Join(dir, "negative-"+label+".cherri")
	var writeErr = os.WriteFile(file, []byte(source), 0644)
	if writeErr != nil {
		t.Fatalf("%s: write fixture: %v", label, writeErr)
	}

	var previousArg1 = os.Args[1]
	os.Args[1] = file

	var captured = captureStdout(t)
	defer func() {
		os.Args[1] = previousArg1
		restoreArg(args.Args, "no-ansi", previousNoAnsi, hadNoAnsi)
		restoreArg(args.Args, "debug", previousDebug, hadDebug)
		restoreArg(args.Args, "skip-sign", previousSkipSign, hadSkipSign)
		resetParser()
		if recovered := recover(); recovered != nil {
			t.Fatalf("%s: unexpected panic escaped harness: %v", label, recovered)
		}
	}()

	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected compile failure containing %q, but compilation succeeded", label, wantSubstring)
			}
		}()
		compile()
	}()

	var output = captured()
	if !strings.Contains(output, wantSubstring) {
		t.Errorf("%s: diagnostic does not mention %q.\nCaptured output:\n%s", label, wantSubstring, output)
	}
}

func restoreArg(target map[string]string, key string, previous string, had bool) {
	if had {
		target[key] = previous
	} else {
		delete(target, key)
	}
}

// captureStdout swaps os.Stdout for a pipe and returns a function that waits
// for all written bytes and returns them.
func captureStdout(t *testing.T) func() string {
	t.Helper()

	var original = os.Stdout
	var reader, writer, pipeErr = os.Pipe()
	if pipeErr != nil {
		t.Fatalf("pipe: %v", pipeErr)
	}
	os.Stdout = writer

	var done = make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, reader)
		close(done)
	}()

	return func() string {
		os.Stdout = original
		_ = writer.Close()
		<-done
		_ = reader.Close()
		return buf.String()
	}
}

func TestNegativeBase64LineBreakModeRejectsBool(t *testing.T) {
	expectCompileError(t, "base64-linebreak-bool",
		"@input = \"hello\"\nbase64Encode(@input, true)\n",
		"for argument 'lineBreakMode'")
}

func TestNegativeGetUpcomingEventsRejectsTextCount(t *testing.T) {
	expectCompileError(t, "upcoming-count-text",
		"const events = getUpcomingEvents(\"5\")\n",
		"for argument 'count'")
}

func TestNegativeGetUpcomingEventsRejectsNumberDateSpecifier(t *testing.T) {
	expectCompileError(t, "upcoming-datespecifier-number",
		"const events = getUpcomingEvents(5, 123)\n",
		"for argument 'dateSpecifier'")
}

func TestNegativeOpenAppSlideOverRejectsText(t *testing.T) {
	expectCompileError(t, "openapp-slideover-text",
		"openApp(\"com.example.app\", \"true\")\n",
		"for argument 'slideOver'")
}

func TestNegativeSaveFileMissingRequiredPath(t *testing.T) {
	expectCompileError(t, "savefile-missing-path",
		"@content = \"data\"\nsaveFile()\n",
		"Missing required 1st argument")
}

func TestNegativeBase64EncodeRejectsReferenceInput(t *testing.T) {
	// Seed a decoded --refs-style reference so the bare identifier resolves
	// through the normal reference machinery rather than failing as unknown.
	references["negFolderRef"] = map[string]any{"fileLocation": map[string]any{}}
	defer delete(references, "negFolderRef")
	expectCompileError(t, "base64-ref-input",
		"base64Encode(negFolderRef)\n",
		"not allowed for argument 'encodeInput'")
}

func TestPositiveForkReferenceParameters(t *testing.T) {
	references["posFolderRef"] = map[string]any{"displayName": "Documents", "fileLocation": map[string]any{}}
	defer delete(references, "posFolderRef")

	loadTestStandardActions()
	args.Args["debug"] = ""
	args.Args["skip-sign"] = ""
	delete(args.Args, "no-ansi")

	var dir = t.TempDir()
	var file = filepath.Join(dir, "positive-refs.cherri")
	var source = "@content = \"data\"\n" +
		"saveFile(\"batch.txt\", @content, false, posFolderRef)\n" +
		"runJavaScriptOnWebpage(\"document.title;\", @content)\n"
	if writeErr := os.WriteFile(file, []byte(source), 0644); writeErr != nil {
		t.Fatalf("write fixture: %v", writeErr)
	}

	var previousArg1 = os.Args[1]
	os.Args[1] = file
	defer func() {
		os.Args[1] = previousArg1
		resetParser()
		if recovered := recover(); recovered != nil {
			t.Fatalf("reference parameters rejected by fork-modified actions: %v", recovered)
		}
	}()

	compile()

	var foundSave, foundJS bool
	for _, compiledAction := range shortcut.WFWorkflowActions {
		switch compiledAction.WFWorkflowActionIdentifier {
		case "is.workflow.actions.documentpicker.save":
			foundSave = compiledAction.WFWorkflowActionParameters["WFFolder"] != nil
		case "is.workflow.actions.runjavascriptonwebpage":
			foundJS = compiledAction.WFWorkflowActionParameters["WFInput"] != nil
		}
	}
	if !foundSave {
		t.Error("saveFile did not emit WFFolder from the folder reference parameter")
	}
	if !foundJS {
		t.Error("runJavaScriptOnWebpage did not emit WFInput from the input parameter")
	}
}

func TestNegativeMalformedLegacyConditional(t *testing.T) {
	expectCompileError(t, "legacy-malformed",
		"@x = 5\nif legacy { @x = 6 }\n",
		"Expected condition after 'legacy' keyword")
}

func TestNegativeGetUpcomingEventsTooManyArguments(t *testing.T) {
	expectCompileError(t, "upcoming-extra-arg",
		"const events = getUpcomingEvents(5, \"Today\", \"May 20, 2025\", \"extra\")\n",
		"Too many arguments")
}
