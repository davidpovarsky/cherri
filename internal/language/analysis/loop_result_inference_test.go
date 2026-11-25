package analysis

import (
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func emptyInferenceProgram(t *testing.T, text string) *syntax.Program {
	t.Helper()
	p := syntax.NewParser(source.NewFile("empty-inference.cherri", "empty-inference.cherri", 1, text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return program
}
func TestLocalLoopEmptyInferenceUsesConcreteSourceIdentity(t *testing.T) {
	cases := []struct{ name, text string }{
		{"zero", "var values=repeat 0 {yield []}\nvalues=repeat 2 {yield [\"x\"]}\nreturn count(values)"},
		{"present_empty_rows", "var values=repeat 2 {yield []}\nvalues=repeat 1 {yield [\"x\"]}\nreturn count(values)"},
		{"alias_chain", "var values=repeat 0 {yield []}\nvar second=values\nlet old=second\nvalues=repeat 1 {yield [\"x\"]}\nsecond=values\nreturn count(old)+count(second)"},
		{"branch", "var values=repeat 0 {yield []}\nif true {values=repeat 1 {yield [\"x\"]}} else {values=repeat 2 {yield [\"y\"]}}\nreturn count(values)"},
		{"backedges", "var values=repeat 0 {yield []}\nvar second=repeat 1 {yield [\"x\"]}\nrepeat 2 {values=second\nsecond=repeat 2 {yield [\"y\"]}}\nreturn count(values)"},
		{"append", "var values=repeat 0 {yield []}\nvalues+=repeat 2 {yield [\"x\"]}\nreturn count(values)"},
		{"shadow", "var values=repeat 0 {yield []}\nif true {var values=repeat 0 {yield []}\nvalues=repeat 1 {yield [1]}\ncount(values)}\nvalues=repeat 1 {yield [\"x\"]}\nreturn count(values)"},
		{"explicit_annotation", "var values:List<List<Text>>=repeat 0 {yield []}\nvalues=repeat 1 {yield [\"x\"]}\nreturn count(values)"},
		{"page", "var values=repeat 0 {yield []}\nlet rows=paginate(\"https://example.invalid/one\") {page in\nvalues=repeat 1 {yield [\"x\"]}\nwait(0)\n}\nreturn count(values)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program := emptyInferenceProgram(t, tc.text)
			original := syntax.Format(program)
			binding := program.Statements[0].(*syntax.BindingStmt)
			loop := binding.Value.(*syntax.RepeatStmt)
			literal := loop.BodyBlock.Statements[0].(*syntax.YieldStmt).Value.(*syntax.ListExpr)
			a := NewAnalyzer(schema.DefaultRegistry())
			a.Analyze(program)
			for _, d := range a.Diagnostics() {
				if d.Severity == SeverityError {
					t.Fatal(d)
				}
			}
			expected := types.NewList(types.NewList(types.Text))
			if got := a.ExprTypes()[loop]; got == nil || !got.Equals(expected) {
				t.Fatalf("source-bound type not inferred: %v", got)
			}
			if got := a.ExprTypes()[literal]; got == nil || !got.Equals(types.NewList(types.Text)) {
				t.Fatalf("empty literal lost its context: %v", got)
			}
			if syntax.Format(program) != original || program.Statements[0] != binding || loop.BodyBlock.Statements[0].(*syntax.YieldStmt).Value != literal {
				t.Fatal("inference rewrote authored AST")
			}
			before := append([]Diagnostic(nil), a.Diagnostics()...)
			a.Analyze(program)
			if !reflect.DeepEqual(before, a.Diagnostics()) || !a.ExprTypes()[loop].Equals(expected) {
				t.Fatal("repeat analysis leaked state/duplicated diagnostics")
			}
			if tc.name == "shadow" {
				branch := program.Statements[1].(*syntax.IfStmt)
				local := branch.ThenBlock.Statements[0].(*syntax.BindingStmt)
				if !a.ExprTypes()[local.Value].Equals(types.NewList(types.NewList(types.Number))) {
					t.Fatal("same-name declaration inherited outer type")
				}
			}
		})
	}
}
func TestLocalLoopEmptyInferenceKeepsUnsupportedEvidence(t *testing.T) {
	cases := []struct{ name, text, code string }{
		{"conflicting_concrete", "var values=repeat 0 {yield []}\nvalues=repeat 1 {yield [\"x\"]}\nvalues=repeat 1 {yield [1]}\nreturn count(values)", "E_TARGET_VALUE_SHAPE"},
		{"explicit_conflict", "var values:List<List<Number>>=repeat 0 {yield []}\nvalues=repeat 1 {yield [\"x\"]}\nreturn count(values)", "E_TARGET_VALUE_SHAPE"},
		{"unknown_source", "var values=repeat 0 {yield [missing]}\nvalues=repeat 1 {yield [\"x\"]}\nreturn count(values)", "E_UNKNOWN_NAME"},
		{"other_error_survives", "var values=repeat 0 {yield []}\nvalues=repeat 1 {yield [\"x\"]}\nreturn missing", "E_UNKNOWN_NAME"},
		{"absence_is_not_empty", "var values=repeat 0 {yield none}\nvalues=repeat 1 {yield [\"x\"]}\nreturn count(values)", "E_TARGET_VALUE_SHAPE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAnalyzer(schema.DefaultRegistry())
			a.Analyze(emptyInferenceProgram(t, tc.text))
			found := false
			for _, d := range a.Diagnostics() {
				if d.Severity == SeverityError && d.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s: %v", tc.code, a.Diagnostics())
			}
		})
	}
}
func TestLocalLoopEmptyInferenceDoesNotInventOrLeakType(t *testing.T) {
	a := NewAnalyzer(schema.DefaultRegistry())
	established := emptyInferenceProgram(t, "var values=repeat 0 {yield []}\nvalues=repeat 1 {yield [\"x\"]}\nreturn count(values)")
	a.Analyze(established)
	unknown := emptyInferenceProgram(t, "var values=repeat 0 {yield []}\nreturn count(values)")
	a.Analyze(unknown)
	binding := unknown.Statements[0].(*syntax.BindingStmt)
	got := a.ExprTypes()[binding.Value]
	if got == nil || !strings.Contains(got.String(), "Unknown") {
		t.Fatalf("unconstrained empty source acquired guessed type: %v", got)
	}
	for _, d := range a.Diagnostics() {
		if d.Severity == SeverityError {
			t.Fatal(d)
		}
	}
}
