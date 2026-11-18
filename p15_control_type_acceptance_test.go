package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestP15T06ControlResultsRequireAllBranches(t *testing.T) {
	for _, tc := range []struct{ name, target, expression string }{
		{"if_known_first", "{name:Text}", "if flag { yield known } else { yield ShortcutInput }"},
		{"if_external_first", "{name:Text}", "if flag { yield ShortcutInput } else { yield known }"},
		{"menu_known_first", "{name:Text}", "menu(\"pick\") { case \"known\" { yield known } case \"external\" { yield ShortcutInput } }"},
		{"repeat_mixed", "List<{name:Text}>", "repeat 2 { if flag { yield known } else { yield ShortcutInput } }"},
		{"for_mixed", "List<{name:Text}>", "for item in [1,2] { if flag { yield known } else { yield ShortcutInput } }"},
		{"missing_branch_yield", "{name:Text}", "if flag { yield known } else { text(\"unvalidated\") }"},
		{"overwritten_yield", "{name:Text}", "if flag { yield known; text(\"unvalidated\") } else { yield known }"},
		{"repeat_overwritten_yield", "List<{name:Text}>", "repeat 2 { yield known; text(\"unvalidated\") }"},
		{"nested_mixed", "{name:Text}", "if flag { if flag { yield known } else { yield ShortcutInput } } else { yield known }"},
		{"nested_missing_yield", "{name:Text}", "if flag { if flag { yield known } else { text(\"unvalidated\") } } else { yield known }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "function select(known:{name:Text}, flag:Bool) -> " + tc.target + " {\nlet value=" + tc.expression + "\nreturn value\n}"
			diagnostics := p15TypeDiagnostics(t, src)
			if err := p15ExactLiteralDiagnostic(diagnostics, src, "return value", analysis.CodeReturnType); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-t06-control.cherri", src)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeReturnType) {
				t.Fatal("unvalidated control result reached native emission", err)
			}
			for i := range diagnostics {
				if diagnostics[i].Code == analysis.CodeReturnType {
					diagnostics[i].Code = analysis.CodeUnknownName
				}
			}
			if p15ExactLiteralDiagnostic(diagnostics, src, "return value", analysis.CodeReturnType) == nil {
				t.Fatal("wrong-code mutation passed")
			}
		})
	}
	for _, tc := range []struct{ target, expression string }{
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
