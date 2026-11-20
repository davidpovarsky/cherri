package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

// Legacy preprocessing expands this apparent directive inside a multiline
// string. A syntax-aware conversion cannot silently change that observed
// meaning into literal text. This edge remains explicitly unsupported.
func TestP15RP33AmbiguousLegacyIncludeFailsClosed(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	directory := t.TempDir()
	original := []byte("text(\"start\n#include 'helper.cherri'\nend\")\n")
	input := filepath.Join(directory, "ambiguous.cherri")
	_, helper := p15RP33FullProgram()
	for path, data := range map[string][]byte{input: original, filepath.Join(directory, "helper.cherri"): []byte(helper)} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	nativePath := filepath.Join(directory, "original.shortcut")
	stdout, stderr, err := p15RunCLI(t, binary, directory, nil, input, "--legacy", "--skip-sign", "--output="+nativePath)
	if err != nil {
		t.Fatal("Legacy edge witness failed to compile", err, string(stdout), string(stderr))
	}
	native, err := os.ReadFile(nativePath)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if _, err := plist.Unmarshal(native, &document); err != nil {
		t.Fatal(err)
	}
	actions := document["WFWorkflowActions"].([]any)
	if len(actions) != 1 || actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"] != "start\n\n\n\n\nend" {
		t.Fatal("original Legacy preprocessing witness changed", actions)
	}
	for _, mode := range []string{"preview", "write"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{"migrate", input}
			if mode == "write" {
				args = append(args, "--write")
			}
			stdout, stderr, err := p15RunCLI(t, binary, directory, nil, args...)
			if err == nil || len(stdout) != 0 || !strings.Contains(string(stderr), "ambiguous Legacy include inside multiline literal") {
				t.Errorf("unsafe conversion was not rejected with its explicit diagnostic: err=%v stdout=%s stderr=%s", err, stdout, stderr)
			}
			for path, want := range map[string][]byte{input: original, nativePath: native, filepath.Join(directory, "helper.cherri"): []byte(helper)} {
				got, readErr := os.ReadFile(path)
				if readErr != nil || !bytes.Equal(got, want) {
					t.Errorf("failed conversion changed original %s: %v", filepath.Base(path), readErr)
				}
			}
		})
	}
}
