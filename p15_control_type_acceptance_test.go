package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func p15T06DiagnosticSet(src string, diagnostics []analysis.Diagnostic, yieldBranch string) error {
	var errors []analysis.Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			errors = append(errors, diagnostic)
		}
	}
	expectedCount, firstCode := 1, analysis.CodeReturnType
	if yieldBranch != "" {
		expectedCount, firstCode = 2, analysis.CodeYieldContext
	}
	if len(errors) != expectedCount || errors[0].Code != firstCode {
		return fmt.Errorf("want %d errors beginning with %s, got %+v", expectedCount, firstCode, errors)
	}
	if err := p15ExactLiteralDiagnostic(diagnostics, src, "return value", analysis.CodeReturnType); err != nil {
		return err
	}
	if yieldBranch != "" {
		if err := p15ExactLiteralDiagnostic(diagnostics, src, yieldBranch, analysis.CodeYieldContext); err != nil {
			return err
		}
	}
	return nil
}

func p15T06RejectDiagnosticCorruption(t *testing.T, src string, diagnostics []analysis.Diagnostic, yieldBranch string) {
	t.Helper()
	for index, diagnostic := range diagnostics {
		if diagnostic.Severity != analysis.SeverityError {
			continue
		}
		for _, fault := range []string{"missing", "code", "span", "severity"} {
			changed := append([]analysis.Diagnostic(nil), diagnostics...)
			switch fault {
			case "missing":
				changed = append(changed[:index], changed[index+1:]...)
			case "code":
				changed[index].Code = analysis.CodeUnknownName
			case "span":
				changed[index].Span.Start.ByteOffset++
			case "severity":
				changed[index].Severity = analysis.SeverityWarning
			}
			if p15T06DiagnosticSet(src, changed, yieldBranch) == nil {
				t.Fatalf("%s mutation of diagnostic %d passed", fault, index)
			}
		}
		changed := append(append([]analysis.Diagnostic(nil), diagnostics...), diagnostic)
		if p15T06DiagnosticSet(src, changed, yieldBranch) == nil {
			t.Fatal("duplicate diagnostic passed")
		}
	}
	if yieldBranch != "" {
		changed := append([]analysis.Diagnostic(nil), diagnostics...)
		var first, second = -1, -1
		for i, d := range changed {
			if d.Severity == analysis.SeverityError {
				if first < 0 {
					first = i
				} else {
					second = i
					break
				}
			}
		}
		changed[first], changed[second] = changed[second], changed[first]
		if p15T06DiagnosticSet(src, changed, yieldBranch) == nil {
			t.Fatal("wrong primary diagnostic order passed")
		}
	}
}

func TestP15T06ControlResultsRequireAllBranches(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	for _, tc := range []struct{ name, target, expression, yieldBranch string }{
		{"if_known_first", "{name:Text}", "if flag { yield known } else { yield ShortcutInput }", ""},
		{"if_external_first", "{name:Text}", "if flag { yield ShortcutInput } else { yield known }", ""},
		{"menu_known_first", "{name:Text}", "menu(\"pick\") { case \"known\" { yield known } case \"external\" { yield ShortcutInput } }", ""},
		{"repeat_mixed", "List<{name:Text}>", "repeat 2 { if flag { yield known } else { yield ShortcutInput } }", ""},
		{"for_mixed", "List<{name:Text}>", "for item in [1,2] { if flag { yield known } else { yield ShortcutInput } }", ""},
		{"missing_branch_yield", "{name:Text}", "if flag { yield known } else { text(\"unvalidated\") }", "{ text(\"unvalidated\") }"},
		{"overwritten_yield", "{name:Text}", "if flag { yield known; text(\"unvalidated\") } else { yield known }", "{ yield known; text(\"unvalidated\") }"},
		{"repeat_overwritten_yield", "List<{name:Text}>", "repeat 2 { yield known; text(\"unvalidated\") }", ""},
		{"nested_mixed", "{name:Text}", "if flag { if flag { yield known } else { yield ShortcutInput } } else { yield known }", ""},
		{"nested_missing_yield", "{name:Text}", "if flag { if flag { yield known } else { text(\"unvalidated\") } } else { yield known }", "{ text(\"unvalidated\") }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "function select(known:{name:Text}, flag:Bool) -> " + tc.target + " {\nlet value=" + tc.expression + "\nreturn value\n}"
			diagnostics := p15TypeDiagnostics(t, src)
			if err := p15T06DiagnosticSet(src, diagnostics, tc.yieldBranch); err != nil {
				t.Fatal(err)
			}
			shared := service.NewService(nil)
			uri := "file:///p15-t06-control.cherri"
			shared.OpenDocument(uri, 1, src)
			serviceDiagnostics, _ := shared.Analyze(uri)
			if err := p15T06DiagnosticSet(src, serviceDiagnostics, tc.yieldBranch); err != nil {
				t.Fatal("shared service", err)
			}
			firstCode := analysis.CodeReturnType
			if tc.yieldBranch != "" {
				firstCode = analysis.CodeYieldContext
			}
			raw, err := CompileSourceToPlist("p15-t06-control.cherri", src)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), firstCode) {
				t.Fatalf("want primary %s and no native bytes: bytes=%d err=%v", firstCode, len(raw), err)
			}
			p15T06RejectDiagnosticCorruption(t, src, diagnostics, tc.yieldBranch)
			if tc.yieldBranch != "" {
				dir := t.TempDir()
				file, target := filepath.Join(dir, "control.cherri"), filepath.Join(dir, "control.shortcut")
				if err := os.WriteFile(file, []byte(src), 0600); err != nil {
					t.Fatal(err)
				}
				stdout, stderr, compileErr := p15RunCLI(t, binary, dir, nil, "compile", file, "--skip-sign", "--output="+target)
				if compileErr == nil || !strings.Contains(string(stdout)+string(stderr), analysis.CodeYieldContext) {
					t.Fatalf("actual CLI missed primary yield rejection: %v %s %s", compileErr, stdout, stderr)
				}
				outputs, globErr := filepath.Glob(filepath.Join(dir, "*.shortcut"))
				if globErr != nil || len(outputs) != 0 {
					t.Fatalf("invalid yield emitted a native artifact: %v %v", outputs, globErr)
				}
			}
		})
	}
	for index, tc := range []struct{ target, expression string }{
		{"{name:Text}", "if flag { yield known } else { yield known }"},
		{"{name:Text}", "if flag { if flag { yield known } else { yield known } } else { yield known }"},
		{"{name:Text}", "menu(\"pick\") { case \"a\" { yield known } case \"b\" { yield known } }"},
		{"List<{name:Text}>", "repeat 2 { yield known }"},
		{"List<{name:Text}>", "for item in [1,2] { yield known }"},
	} {
		src := "function select(known:{name:Text}, flag:Bool) -> " + tc.target + " {\nlet value=" + tc.expression + "\nreturn value\n}"
		for _, diagnostic := range p15TypeDiagnostics(t, src) {
			if diagnostic.Severity == analysis.SeverityError {
				t.Fatal(diagnostic)
			}
		}
		compileDebugFixture(t, src, false)
		if index == 0 {
			dir := t.TempDir()
			file, target := filepath.Join(dir, "healthy.cherri"), filepath.Join(dir, "healthy.shortcut")
			if err := os.WriteFile(file, []byte(src), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err := p15RunCLI(t, binary, dir, nil, "compile", file, "--skip-sign", "--output="+target)
			if err != nil {
				t.Fatalf("healthy CLI control rejected: %v %s %s", err, stdout, stderr)
			}
			if raw, err := os.ReadFile(target); err != nil || len(raw) == 0 {
				t.Fatalf("healthy CLI control omitted artifact: %v", err)
			}
		}
	}
	joined := types.NewUnion(types.Text, types.Number)
	for _, tc := range []struct {
		expression string
		want       types.Type
	}{
		{"if true { yield \"one\" } else { yield 2 }", joined},
		{"if true { yield 2 } else { yield \"one\" }", joined},
		{"menu(\"pick\") { case \"a\" { yield \"one\" } case \"b\" { yield 2 } }", joined},
		{"repeat 2 { if true { yield \"one\" } else { yield 2 } }", types.NewList(joined)},
		{"for item in [1,2] { if true { yield \"one\" } else { yield 2 } }", types.NewList(joined)},
	} {
		src := "let value=" + tc.expression
		program, errors := p15ParseProgram(src)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
		analyzer.Analyze(program)
		for _, diagnostic := range analyzer.Diagnostics() {
			if diagnostic.Severity == analysis.SeverityError {
				t.Fatal(diagnostic)
			}
		}
		actual := analyzer.ExprTypes()[p15BindingValue(t, program, "value")]
		if actual == nil || !actual.Equals(tc.want) {
			t.Errorf("control join %s: got%v want%v", tc.expression, actual, tc.want)
		}
		compileDebugFixture(t, src, false)
	}
}
