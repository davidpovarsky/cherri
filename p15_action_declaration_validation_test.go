package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

const p15DeclaredActionControl = "action p15Declaration(value: Text) -> Text {\nidentifier: \"com.example.p15.declaration\"\nprimary: \"value\"\nbindings: {value: {key: \"Value\", codec: \"text\"}}\n}\np15Declaration(\"CONTROL\")\n"

func p15InvalidActionDeclarations() []struct{ name, source, code, detail string } {
	var control = p15DeclaredActionControl
	return []struct{ name, source, code, detail string }{
		{"unknown_codec", strings.Replace(control, "codec: \"text\"", "codec: \"P15_MISSING_CODEC\"", 1), analysis.CodeSchemaMismatch, "P15_MISSING_CODEC"},
		{"unknown_binding", strings.Replace(control, "bindings: {value:", "bindings: {missing:", 1), analysis.CodeSchemaMismatch, "missing"},
		{"unknown_primary", strings.Replace(control, "primary: \"value\"", "primary: \"missing\"", 1), analysis.CodeSchemaMismatch, "missing"},
		{"empty_wire", strings.Replace(control, "key: \"Value\"", "key: \"\"", 1), analysis.CodeSchemaMismatch, "wire key"},
		{"duplicate_parameter", strings.Replace(control, "value: Text)", "value: Text, value: Text)", 1), analysis.CodeDuplicateName, "value"},
		{"duplicate_wire", "action p15Declaration(a: Text, b: Text) { identifier: \"com.example.p15.declaration\"\n bindings: { a: {key: \"Value\", codec: \"text\"}, b: {key: \"Value\", codec: \"text\"} } }\np15Declaration(\"A\", b: \"B\")", analysis.CodeSchemaMismatch, "Value"},
		{"unused_invalid_declaration", strings.Replace(strings.TrimSuffix(control, "p15Declaration(\"CONTROL\")\n"), "codec: \"text\"", "codec: \"P15_MISSING_CODEC\"", 1) + "text(\"unrelated\")", analysis.CodeSchemaMismatch, "P15_MISSING_CODEC"},
		{"duplicate_binding", strings.Replace(control, "bindings: {value: {key: \"Value\", codec: \"text\"}}", "bindings: {value: {key: \"Value\", codec: \"text\"}, value: {key: \"Other\", codec: \"text\"}}", 1), analysis.CodeSyntax, "duplicate action binding"},
		{"duplicate_binding_property", strings.Replace(control, "key: \"Value\", codec:", "key: \"Value\", key: \"Other\", codec:", 1), analysis.CodeSyntax, "duplicate action binding field"},
	}
}

func p15DeclarationDiagnosticError(diagnostics []analysis.Diagnostic, code, detail string) error {
	for _, d := range diagnostics {
		if d.Code == code && d.Severity == analysis.SeverityError && strings.Contains(d.Message, detail) && d.Span.End.ByteOffset > d.Span.Start.ByteOffset {
			return nil
		}
	}
	return fmt.Errorf("missing exact %s diagnostic containing %q with authored span: %+v", code, detail, diagnostics)
}

func TestP15ActionDeclarationPublicValidation(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	var signingRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signingRequests.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("P15_DECLARATION_SIGNING_MUST_NOT_RUN"))
	}))
	defer server.Close()

	for _, tc := range p15InvalidActionDeclarations() {
		t.Run(tc.name, func(t *testing.T) {
			compiledCode := tc.code
			if tc.code == analysis.CodeSyntax {
				compiledCode = "syntax error"
			}
			if data, err := CompileSourceToPlist("invalid-action.cherri", tc.source); err == nil || len(data) != 0 || !strings.Contains(err.Error(), compiledCode) || !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("embedded compiler accepted declaration or lost precise error: bytes=%d error=%v", len(data), err)
			}
			mobile := compileForMobileMode(tc.source, "P15 invalid declaration", "v2", false)
			if mobile.OK || mobile.PlistBase64 != "" || !strings.Contains(mobile.Error, compiledCode) || !strings.Contains(mobile.Error, tc.detail) {
				t.Errorf("mobile compiler accepted declaration or lost precise error: %+v", mobile)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-invalid-declaration.cherri"
			svc.OpenDocument(uri, 1, tc.source)
			diagnostics, _ := svc.Analyze(uri)
			if err := p15DeclarationDiagnosticError(diagnostics, tc.code, tc.detail); err != nil {
				t.Error(err)
			}
			dir := t.TempDir()
			input, target := filepath.Join(dir, "source.cherri"), filepath.Join(dir, "previous.shortcut")
			original := []byte("P15_PREEXISTING_OUTPUT")
			if err := os.WriteFile(input, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, original, 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := p15RunCLI(t, binary, dir, nil, "check", input, "--json")
			if err == nil || !bytes.Contains(out, []byte(tc.code)) || !bytes.Contains(out, []byte(tc.detail)) {
				t.Errorf("check accepted invalid declaration: %v %s %s", err, out, stderr)
			}
			out, stderr, err = p15RunCLI(t, binary, dir, nil, "compile", input, "--hubsign", "--signing-server="+server.URL, "--output="+target)
			if err == nil || !strings.Contains(string(out)+string(stderr), compiledCode) || !strings.Contains(string(out)+string(stderr), tc.detail) {
				t.Errorf("CLI compile missed semantic rejection: %v %s %s", err, out, stderr)
			}
			after, readErr := os.ReadFile(target)
			if readErr != nil || !bytes.Equal(original, after) {
				t.Errorf("invalid declaration changed prior destination: %v %q", readErr, after)
			}
		})
	}
	if signingRequests.Load() != 0 {
		t.Errorf("invalid declarations attempted signing %d times", signingRequests.Load())
	}
}

func TestP15ActionDeclarationHealthyControl(t *testing.T) {
	for _, source := range []string{
		p15DeclaredActionControl,
		strings.Replace(p15DeclaredActionControl, ", codec: \"text\"", "", 1),
		"action p15Declaration(value: Text) { identifier: \"com.example.p15.declaration\" }\np15Declaration(\"CONTROL\")",
	} {
		raw, err := CompileSourceToPlist("healthy-action.cherri", source)
		if err != nil {
			t.Fatal(err)
		}
		var root map[string]any
		if _, err := plist.Unmarshal(raw, &root); err != nil {
			t.Fatal(err)
		}
		actions, ok := root["WFWorkflowActions"].([]any)
		if !ok || len(actions) != 1 {
			t.Fatalf("wrong healthy action graph: %#v", root)
		}
		action := actions[0].(map[string]any)
		if action["WFWorkflowActionIdentifier"] != "com.example.p15.declaration" {
			t.Fatal("wrong custom identity")
		}
		params := action["WFWorkflowActionParameters"].(map[string]any)
		key := "Value"
		if !strings.Contains(source, "bindings:") {
			key = "value"
		}
		if len(params) != 1 || params[key] != "CONTROL" {
			t.Fatalf("healthy custom binding changed: %#v", params)
		}
		svc := service.NewService(nil)
		svc.OpenDocument("file:///healthy.cherri", 1, source)
		diagnostics, _ := svc.Analyze("file:///healthy.cherri")
		for _, d := range diagnostics {
			if d.Severity == analysis.SeverityError {
				t.Fatal(d)
			}
		}
		mobile := compileForMobileMode(source, "Healthy custom", "v2", false)
		if !mobile.OK || mobile.PlistBase64 == "" {
			t.Fatal(mobile.Error)
		}
	}
}
