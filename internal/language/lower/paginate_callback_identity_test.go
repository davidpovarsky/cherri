package lower

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

type pageIdentitySession struct{ *testMockSession }

func (s *pageIdentitySession) EmitScopedPaginateCall(call backend.ResolvedCall, callback backend.ScopedPageCallback) (string, error) {
	return "", fmt.Errorf("unexpected backend page emission before identity rejection")
}
func TestPaginatePageLowerRejectsChangedAuthorityBeforeEffects(t *testing.T) {
	for _, fault := range []string{"call", "block", "body", "parameter", "name", "body_enrollment", "parameter_enrollment"} {
		t.Run(fault, func(t *testing.T) {
			text := "var values=repeat 0 {yield []}\nlet rows=paginate(\"https://example.invalid/one\") { page in\nvalues=repeat 2 {yield [\"x\"]}\nwait(0)\n}\nreturn count(values)"
			p := syntax.NewParser(source.NewFile("page-identity.cherri", "page-identity.cherri", 1, text))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := analysis.NewAnalyzer(schema.DefaultRegistry())
			a.Analyze(program)
			for _, d := range a.Diagnostics() {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			l := NewLowererWithSession(a.Registry(), &pageIdentitySession{&testMockSession{meta: map[string]any{}}})
			l.SetExprTypes(a.ExprTypes())
			l.loopResultLocalPlan = schema.PlanLoopResultLocals(program, a.Registry())
			call := program.Statements[1].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
			if l.loopResultLocalPlan == nil || !l.loopResultLocalPlan.PageCallbackSourceMatches(call) {
				t.Fatal("fixture not enrolled")
			}
			switch fault {
			case "call":
				v := *call
				call = &v
			case "block":
				v := *call.Block
				call.Block = &v
			case "body":
				v := *call.Block.Body
				call.Block.Body = &v
			case "parameter":
				v := *call.Block.Parameters[0]
				call.Block.Parameters[0] = &v
			case "name":
				call.Block.Parameters[0].Name = "other"
			case "body_enrollment":
				delete(l.loopResultLocalPlan.ScopedBodies, call)
			case "parameter_enrollment":
				delete(l.loopResultLocalPlan.PageCallbacks, call)
			}
			_, err := l.lowerScopedBlockCall(call)
			if err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") || len(l.workflow.Actions) != 0 {
				t.Fatalf("changed page authority emitted effects or escaped exact source guard: %v, actions=%d", err, len(l.workflow.Actions))
			}
		})
	}
}
