package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"howett.net/plist"
)

var codeBlockRegex = regexp.MustCompile("(?s)```cherri[ \\t]*\\r?\\n(.*?)\\r?\\n```")
var docsDiagnosticAnnotation = regexp.MustCompile("<!--\\s*cherri-test:\\s*diagnostic\\s+(E_[A-Z0-9_]+)\\s*-->\\s*$")

// This is the public CLI and canonical backend, not just parser/analysis smoke.
// Historical reference pages and generated signature notation are not runnable
// programs; active guides must classify deliberately invalid code explicitly.
func TestDocsExamplesExtractAndVerify(t *testing.T) {
	binary := roundTripBinary(t)
	docFile := filepath.Join("docs", "language-v2", "language-guide.md")
	data, err := os.ReadFile(docFile)
	if err != nil {
		t.Fatal(err)
	}
	matches := codeBlockRegex.FindAllSubmatchIndex(data, -1)
	if len(matches) < 18 {
		t.Fatalf("expected full active guide coverage, found %d", len(matches))
	}
	positive, negative := 0, 0
	for index, match := range matches {
		t.Run(fmt.Sprintf("block_%02d", index+1), func(t *testing.T) {
			code := strings.TrimSpace(string(data[match[2]:match[3]]))
			if code == "" {
				t.Fatal("empty active code fence")
			}
			expected := ""
			if annotation := docsDiagnosticAnnotation.FindSubmatch(data[:match[0]]); len(annotation) > 1 {
				expected = string(annotation[1])
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "snippet.cherri")
			if err := os.WriteFile(path, []byte(code+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, checkErr := p15RunCLI(t, binary, dir, nil, "check", path, "--json", "--no-ansi")
			var checked struct {
				Valid       bool `json:"valid"`
				Diagnostics []struct {
					Code     string `json:"code"`
					Severity string `json:"severity"`
				} `json:"diagnostics"`
			}
			if err := json.Unmarshal(stdout, &checked); err != nil {
				t.Fatalf("invalid structured check: %v\n%s\n%s", err, stdout, stderr)
			}
			var codes []string
			for _, diagnostic := range checked.Diagnostics {
				if diagnostic.Severity == "ERROR" {
					codes = append(codes, diagnostic.Code)
				}
			}
			if expected != "" {
				if checkErr == nil || checked.Valid || len(codes) != 1 || codes[0] != expected {
					t.Fatalf("expected exact %s, got %v (exit %v): %s", expected, codes, checkErr, stderr)
				}
				negative++
			} else {
				if checkErr != nil || !checked.Valid || len(codes) != 0 {
					t.Fatalf("runnable documentation check failed: %v\n%s\n%s", checkErr, stdout, stderr)
				}
				positive++
			}
			out, compileErrOutput, compileErr := p15RunCLI(t, binary, dir, nil, path, "--skip-sign", "--no-ansi")
			unsigned := filepath.Join(dir, "snippet_unsigned.shortcut")
			if expected != "" {
				if compileErr == nil {
					t.Fatal("deliberately invalid example compiled")
				}
				if _, err := os.Stat(unsigned); !os.IsNotExist(err) {
					t.Fatal("invalid example left an unsigned artifact")
				}
				return
			}
			if compileErr != nil {
				t.Fatalf("real unsigned compilation failed: %v\n%s\n%s\nCode:\n%s", compileErr, out, compileErrOutput, code)
			}
			raw, err := os.ReadFile(unsigned)
			if err != nil {
				t.Fatal(err)
			}
			var workflow map[string]any
			if _, err := plist.Unmarshal(raw, &workflow); err != nil {
				t.Fatalf("not a real native workflow: %v", err)
			}
			if _, ok := workflow["WFWorkflowActions"].([]any); !ok {
				t.Fatalf("missing native action array: %T", workflow["WFWorkflowActions"])
			}
		})
	}
	if positive == 0 || negative == 0 {
		t.Fatalf("missing real positive/negative coverage: %d/%d", positive, negative)
	}
	t.Logf("DOCS_EXECUTED runnable=%d exact_diagnostics=%d; actual public check and unsigned compiler; no Apple runtime claim", positive, negative)
}
