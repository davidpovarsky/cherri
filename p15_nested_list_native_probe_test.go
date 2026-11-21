package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

func p15NestedListPython(t *testing.T, arguments ...string) ([]byte, error) {
	t.Helper()
	python := os.Getenv("CHERRI_NESTED_LIST_PYTHON")
	if python == "" {
		python = resolvePythonCommand()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, python, append([]string{"-B"}, arguments...)...)
	command.WaitDelay = time.Second
	return command.CombinedOutput()
}

func TestP15NestedListProbeActualCompiledGraphs(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		fixture            string
		actions, mutations int
	}{
		{"qa-nested-list-control-v2", 56, 28},
		{"qa-nested-list-candidate-v2", 78, 33},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			sourcePath := filepath.Join("tools", "shortcuts-simulator-qa", "fixtures", tc.fixture+".cherri")
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(string(source), "return finalSummary\n") {
				t.Fatal("diagnostic must explicitly return its observed output")
			}
			data, err := CompileSourceToPlist(sourcePath, string(source))
			if err != nil {
				t.Fatal(err)
			}
			temporary := filepath.Join(t.TempDir(), tc.fixture+".shortcut")
			if err := os.WriteFile(temporary, data, 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{filepath.Join(root, "tools", "shortcuts-simulator-qa", "nested_list_probe.py"), "--root", root, "--fixture", tc.fixture, "--unsigned", temporary}
			out, err := p15NestedListPython(t, args...)
			if err != nil {
				t.Fatalf("actual unsigned graph rejected: %v\n%s", err, out)
			}
			marker := "CHERRI_NESTED_LIST_UNSIGNED_GRAPH_V1="
			text := strings.TrimSpace(string(out))
			if !strings.HasPrefix(text, marker) || strings.Count(text, marker) != 1 || strings.Contains(text, "\n") {
				t.Fatalf("missing unique actual-artifact graph receipt: %s", out)
			}
			var receipt struct {
				Fixture string `json:"fixtureId"`
				SHA     string `json:"unsignedPlistSha256"`
				Native  bool   `json:"nativeListTypeProved"`
				Formal  bool   `json:"formalAcceptanceGranted"`
				Graph   struct {
					Count  int    `json:"nativeActionCount"`
					Status string `json:"status"`
					Apple  string `json:"appleRuntime"`
				} `json:"graph"`
				Mutations struct {
					Names  []string `json:"rejected"`
					Status string   `json:"status"`
					Apple  string   `json:"appleRuntime"`
				} `json:"causalMutations"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(text, marker)), &receipt); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(data)
			unique := map[string]bool{}
			for _, name := range receipt.Mutations.Names {
				if name == "" || unique[name] {
					t.Fatal("mutation receipt missing or duplicated")
				}
				unique[name] = true
			}
			if receipt.Fixture != tc.fixture || receipt.SHA != hex.EncodeToString(digest[:]) ||
				receipt.Native || receipt.Formal || receipt.Graph.Count != tc.actions ||
				receipt.Graph.Status != "PASS_LOCAL_COMPLETE_GRAPH_ONLY" || receipt.Graph.Apple != "NOT_RUN" ||
				len(unique) != tc.mutations || receipt.Mutations.Status != "PASS_LOCAL_CAUSAL_CORRUPTIONS_ONLY" ||
				receipt.Mutations.Apple != "NOT_RUN" {
				t.Fatalf("invalid scoped receipt: %s", out)
			}
			t.Log(text)
			// This is the actual emitted plist with one altered output field,
			// not a fabricated successful graph or a changed source oracle.
			var workflow map[string]any
			if _, err := plist.Unmarshal(data, &workflow); err != nil {
				t.Fatal(err)
			}
			actions := workflow["WFWorkflowActions"].([]any)
			final := actions[len(actions)-1].(map[string]any)
			final["WFWorkflowActionParameters"].(map[string]any)["WFOutput"] = "FORGED_SUCCESS"
			changed, err := plist.Marshal(workflow, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(temporary, changed, 0600); err != nil {
				t.Fatal(err)
			}
			out, err = p15NestedListPython(t, args...)
			if err == nil || strings.Contains(string(out), marker) || !strings.Contains(string(out), "NESTED_LIST_ACTION_") {
				t.Fatalf("altered actual output artifact accepted or failed for unrelated reason: %v\n%s", err, out)
			}
		})
	}
}

func TestP15NestedListProbeObservationControls(t *testing.T) {
	out, err := p15NestedListPython(t, filepath.Join("tools", "shortcuts-simulator-qa", "test_nested_list_probe.py"))
	if err != nil {
		t.Fatalf("observation controls: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Ran 15 tests") || !strings.Contains(string(out), "\nOK") {
		t.Fatalf("missing complete observation controls: %s", out)
	}
	t.Log(string(out))
}
