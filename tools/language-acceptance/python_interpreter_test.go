package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPython3InterpreterSelectionExecutesRealProbe(t *testing.T) {
	var actual, actualErr = selectPython3Interpreter("", []string{"python3", "python"})
	if actualErr != nil {
		t.Fatal("the actual canonical producer controls require an installed Python 3", actualErr)
	}
	var nonPython, executableErr = os.Executable()
	if executableErr != nil {
		t.Fatal(executableErr)
	}
	// The real Go test executable is discoverable but rejects Python flags.
	// This exercises the same discovery/execution distinction as the Store alias.
	if err := probePython3Interpreter(nonPython); err == nil {
		t.Fatal("a discoverable non-Python executable passed the interpreter probe")
	}
	var fallback, fallbackErr = selectPython3Interpreter("", []string{nonPython, actual})
	if fallbackErr != nil || fallback != actual {
		t.Fatal("an unusable discovered candidate prevented the working fallback", fallback, fallbackErr)
	}
	for _, configured := range []string{nonPython, filepath.Join(t.TempDir(), "missing-python.exe")} {
		if path, err := selectPython3Interpreter(configured, []string{actual}); err == nil || path != "" {
			t.Fatal("invalid explicit configuration silently selected a different interpreter", configured, path)
		}
	}
	t.Setenv("CHERRI_D03_PYTHON", actual)
	if path, err := documentationPython(); err != nil || path != actual {
		t.Fatal("canonical documentation did not use the verified configured runtime", path, err)
	}
	t.Setenv("CHERRI_D03_PYTHON", nonPython)
	if path, err := documentationPython(); err == nil || path != "" {
		t.Fatal("canonical documentation accepted an unusable configured interpreter", path)
	}
}

func TestPython3InterpreterRejectsVersionAndProtocolMismatch(t *testing.T) {
	var valid = `{"protocol":"cherri-python3-isolated-v1","major":3,"isolated":true}`
	if err := validatePython3Probe([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		"Python", "", valid + "\nextra-output", strings.Repeat("x", 4097),
		strings.Replace(valid, `"major":3`, `"major":2`, 1),
		strings.Replace(valid, `"major":3`, `"major":"3"`, 1),
		strings.Replace(valid, `"isolated":true`, `"isolated":false`, 1),
		strings.Replace(valid, "cherri-python3-isolated-v1", "unrelated-program", 1),
	} {
		if err := validatePython3Probe([]byte(invalid)); err == nil {
			t.Fatal("mismatched or non-isolated interpreter receipt was accepted", invalid)
		}
	}
}
