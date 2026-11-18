package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15V02Range(tc p15V02Case) protocol.Range {
	start := p15V02TargetOffset(tc)
	position := func(offset int) protocol.Position {
		prefix := tc.source[:offset]
		tail := prefix[strings.LastIndex(prefix, "\n")+1:]
		return protocol.Position{Line: strings.Count(prefix, "\n"), Character: len(utf16.Encode([]rune(tail)))}
	}
	return protocol.Range{Start: position(start), End: position(start + len(tc.target))}
}

func TestP15V02BindingServiceAndCLI(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	binary := p15AcceptanceCLI(t)
	dir := t.TempDir()
	svc := service.NewService(nil)
	uri := "file:///p15-v02-live.cherri"
	svc.OpenDocument(uri, 1, "number(0)")
	for index, tc := range p15V02Invalid {
		t.Run(tc.name, func(t *testing.T) {
			svc.ChangeDocument(uri, 2+index*2, tc.source)
			diagnostics, _ := svc.Analyze(uri)
			if err := p15V02Diagnostic(diagnostics, tc); err != nil {
				t.Fatal("shared service", err)
			}
			path := filepath.Join(dir, fmt.Sprintf("invalid-%d.cherri", index))
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			output, stderr, err := p15RunCLI(t, binary, dir, nil, "check", path, "--json")
			if err == nil {
				t.Fatal("CLI accepted invalid binding", tc.name)
			}
			var result protocol.AnalyzeResponse
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatal(err, string(output), stderr)
			}
			errors := []protocol.DiagnosticItem{}
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Severity == "ERROR" {
					errors = append(errors, diagnostic)
				}
			}
			if result.Valid || len(errors) != 1 || errors[0].Code != tc.code || errors[0].Range != p15V02Range(tc) {
				t.Fatalf("actual CLI code/range/exit mismatch: %+v", result)
			}
			svc.ChangeDocument(uri, 3+index*2, "var value=1\nvalue+=2\nnumber(value)")
			diagnostics, _ = svc.Analyze(uri)
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal("corrected source retained stale flow state", diagnostic)
				}
			}
		})
	}
	for index, tc := range p15V02Valid {
		path := filepath.Join(dir, fmt.Sprintf("valid-%d.cherri", index))
		if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		output, stderr, err := p15RunCLI(t, binary, dir, nil, "check", path, "--json")
		if err != nil {
			t.Fatal(tc.name, err, string(output), stderr)
		}
		var result protocol.AnalyzeResponse
		if err := json.Unmarshal(output, &result); err != nil || !result.Valid {
			t.Fatal(tc.name, string(output), err)
		}
	}

	// Narrowing is recorded per original expression, not by mutating a symbol's
	// declared assignment type or rewriting the source AST.
	source := p15V02Optional + "if maybe!=none {let before=maybe; if true {let maybe=2;let local=maybe};let again=maybe;maybe=none;let after=maybe}\nlet outside=maybe"
	program, errors := p15ParseProgram(source)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	before := syntax.Format(program)
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
	if syntax.Format(program) != before {
		t.Fatal("flow analysis rewrote the source AST")
	}
	for _, tc := range []struct{ name, want string }{{"before", "Text"}, {"local", "Number"}, {"again", "Text"}, {"after", "Text?"}, {"outside", "Text?"}} {
		offset := strings.Index(source, tc.name+"=maybe") + len(tc.name) + 1
		found := false
		for expression, inferred := range analyzer.ExprTypes() {
			if name, ok := expression.(*syntax.IdentExpr); ok && name.Span.Start.ByteOffset == offset {
				if inferred == nil || inferred.String() != tc.want {
					t.Fatalf("%s inferred %v, want %s", tc.name, inferred, tc.want)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("missing original expression type", tc.name)
		}
	}
}

func TestP15V02ParameterScopes(t *testing.T) {
	svc := service.NewService(nil)
	uri := "file:///p15-v02-parameters.cherri"
	marked := "function value(first: Number=1, second: Number=fi|rst)->Number{return second}\nnumber(value())"
	cursor := strings.Index(marked, "|")
	source := strings.Replace(marked, "|", "", 1)
	svc.OpenDocument(uri, 1, source)
	line, col := p15C04CompletionPosition(source, cursor)
	got := svc.Definition(uri, line, col)
	want := strings.Index(source, "first")
	if got == nil || got.Start.ByteOffset != want || got.End.ByteOffset != want+len("first") {
		t.Errorf("earlier parameter default resolves to %+v; want exact first identifier", got)
	}
	if raw, err := CompileSourceToPlist("p15-v02-default.cherri", source); err != nil || len(raw) == 0 {
		t.Fatal("valid earlier-parameter default did not compile", err)
	}

	marked = "function invalid(value: Number=va|lue)->Number{return value}"
	cursor = strings.Index(marked, "|")
	source = strings.Replace(marked, "|", "", 1)
	svc.ChangeDocument(uri, 2, source)
	line, col = p15C04CompletionPosition(source, cursor)
	if got := svc.Definition(uri, line, col); got != nil {
		t.Errorf("parameter incorrectly visible in its own default: %+v", got)
	}
	diagnostics, _ := svc.Analyze(uri)
	found := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == analysis.CodeUnknownName {
			found = true
		}
	}
	if !found {
		t.Fatal("self default should fail before its parameter binding exists")
	}

	source = "function identity(value: Number=1)->Number{return value}\nnumber(identity())"
	svc.ChangeDocument(uri, 3, source)
	line, col = p15C04CompletionPosition(source, strings.Index(source, "value")+1)
	updated, edits, err := svc.Rename(uri, line, col, "renamed")
	expected := strings.ReplaceAll(source, "value", "renamed")
	if err != nil || updated != expected || len(edits) != 2 {
		t.Fatalf("parameter rename corrupted annotation/default or scope: %v %+v %q", err, edits, updated)
	}
	if raw, err := CompileSourceToPlist("p15-v02-renamed.cherri", updated); err != nil || len(raw) == 0 {
		t.Fatal("renamed parameter did not compile", err)
	}
}
