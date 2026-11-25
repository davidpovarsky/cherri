package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestScopedLoopCellsRejectCallAndBodyReplacementBeforeEffects(t *testing.T) {
	for _, fault := range []string{"cloned_call", "cloned_body", "other_enrolled_body", "removed_enrollment"} {
		t.Run(fault, func(t *testing.T) {
			text := "let first=measure(\"first\"){let values=repeat 1 {yield []}\ncount(values)}\nreturn measure(\"second\"){let values=repeat 2 {yield none}\ncount(values)}"
			p := syntax.NewParser(source.NewFile("scoped-identity.cherri", "scoped-identity.cherri", 1, text))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := analysis.NewAnalyzer(schema.DefaultRegistry())
			a.Analyze(program)
			if len(a.Diagnostics()) != 0 {
				t.Fatal(a.Diagnostics())
			}
			l := NewLowererWithSession(a.Registry(), &testMockSession{meta: map[string]any{}})
			l.SetExprTypes(a.ExprTypes())
			l.loopResultLocalPlan = schema.PlanLoopResultLocals(program, a.Registry())
			if l.loopResultLocalPlan == nil {
				t.Fatal("original source was not enrolled")
			}
			call := program.Statements[0].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
			other := program.Statements[1].(*syntax.ReturnStmt).Value.(*syntax.CallExpr)
			switch fault {
			case "cloned_call":
				c := *call
				call = &c
			case "cloned_body":
				b := *call.Block.Body
				s := *call.Block
				s.Body = &b
				call.Block = &s
			case "other_enrolled_body":
				call.Block.Body = other.Block.Body
			case "removed_enrollment":
				delete(l.loopResultLocalPlan.ScopedBodies, call)
			}
			_, err := l.lowerScopedBlockCall(call)
			if err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") || len(l.workflow.Actions) != 0 {
				t.Fatalf("changed source identity emitted effects or escaped exact guard: %v, actions=%d", err, len(l.workflow.Actions))
			}
		})
	}
}

func TestScopedSyntaxRejectsAbsoluteNativeNamesBeforeScope(t *testing.T) {
	for _, text := range []string{
		"return measure(\"scope\"){let safe=1\nrawAction(\"example.raw\",{})\n1}",
		"return measure(\"scope\"){let safe=1\nnative.action(\"example.raw\",{})\n1}",
	} {
		p := syntax.NewParser(source.NewFile("scoped-raw.cherri", "scoped-raw.cherri", 1, text))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		call := program.Statements[0].(*syntax.ReturnStmt).Value.(*syntax.CallExpr)
		l := NewLowerer(nil)
		restore, err := l.beginScopedBlockScope(call.Block.Body, "test")
		if err == nil || restore != nil || !strings.Contains(err.Error(), "E_BLOCK_RAW_NATIVE") ||
			len(l.lexical.scopes) != 0 || l.lexical.privatePrefix != "" || len(l.workflow.Actions) != 0 {
			t.Fatalf("absolute native name entered lexical scope: %v", err)
		}
	}
}
