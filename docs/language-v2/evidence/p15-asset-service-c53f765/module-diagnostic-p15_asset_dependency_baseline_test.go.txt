package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

func p15AssetCompile(path, source string) (out []byte, err error) {
	var previous = embeddedCompilerMode
	embeddedCompilerMode = true
	defer func() {
		embeddedCompilerMode = previous
		if failure := recover(); failure != nil {
			out = nil
			err = fmt.Errorf("%v", failure)
		}
	}()
	return CompileSourceToPlist(path, source)
}

func p15AssetText(t *testing.T, output []byte) string {
	t.Helper()
	var workflow Shortcut
	if _, err := plist.Unmarshal(output, &workflow); err != nil {
		t.Fatal(err)
	}
	if len(workflow.WFWorkflowActions) != 1 {
		t.Fatalf("expected one canonical text producer, got %d", len(workflow.WFWorkflowActions))
	}
	action := workflow.WFWorkflowActions[0]
	if action.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
		t.Fatalf("wrong asset producer: %q", action.WFWorkflowActionIdentifier)
	}
	value, ok := action.WFWorkflowActionParameters["WFTextActionText"].(string)
	if !ok {
		t.Fatal("asset must remain canonical base64 text")
	}
	return value
}

func TestP15S03EmbeddedAssetSourceRelative(t *testing.T) {
	dir := t.TempDir()
	payload := []byte{0, 1, 2, 255, 128, 'A', '\n'}
	if err := os.WriteFile(filepath.Join(dir, "payload.bin"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := p15AssetCompile(filepath.Join(dir, "entry.cherri"), "embedFile(\"payload.bin\")\n")
	if err != nil {
		t.Fatalf("entry-relative embedded asset was not compiled: %v", err)
	}
	if got := p15AssetText(t, out); got != base64.StdEncoding.EncodeToString(payload) {
		t.Fatal("canonical embedded bytes changed")
	}
}

func TestP15S03EditorAssetDependencyDiagnostics(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "present.txt"), []byte("PUBLIC_ASSET"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, source string
		wantError    bool
	}{
		{"healthy_local_asset", "embedFile(\"present.txt\")\n", false},
		{"missing_local_asset", "embedFile(\"missing.txt\")\n", true},
		{"escaped_local_asset", "embedFile(\"../outside.txt\")\n", true},
		{"unrelated_text", "let title = \"unrelated\"\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := service.NewService(nil)
			uri := filepath.Join(dir, "entry.cherri")
			svc.OpenDocument(uri, 1, tc.source)
			diags, _ := svc.Analyze(uri)
			var errors []analysis.Diagnostic
			for _, diag := range diags {
				if diag.Severity == analysis.SeverityError {
					errors = append(errors, diag)
				}
			}
			if tc.wantError {
				if len(errors) != 1 || errors[0].Code != analysis.CodeWorkspaceAccess || errors[0].Span.Start.Line != 1 {
					t.Fatalf("expected exact asset dependency diagnostic at source: %+v", errors)
				}
				if strings.Contains(errors[0].Message, dir) {
					t.Fatal("resolved host path leaked in asset diagnostic")
				}
			} else if len(errors) != 0 {
				t.Fatalf("healthy source rejected: %+v", errors)
			}
		})
	}
}
