package schema

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func pagePlanFixture(t *testing.T) (*syntax.Program, *syntax.CallExpr, *LoopResultLocalPlan) {
	t.Helper()
	p := scopedPlanProgram(t, "var values=repeat 0 {yield []}\nlet rows=paginate(\"https://example.invalid/one\") { page in\nlet prior=values\nvalues=repeat 2 {yield [\"x\"]}\nlet n=count(prior)\nwait(0)\n}\nreturn count(values)")
	call := p.Statements[1].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
	plan := PlanLoopResultLocals(p, DefaultRegistry())
	if plan == nil || !plan.PageCallbackSourceMatches(call) {
		t.Fatal("original page/captured flow was not enrolled")
	}
	return p, call, plan
}

func TestPaginatePagePlannerPreservesActualLexicalFlow(t *testing.T) {
	program, call, plan := pagePlanFixture(t)
	outer := program.Statements[0].(*syntax.BindingStmt)
	assignment := call.Block.Body.Statements[1].(*syntax.AssignStmt)
	if plan.Assignments[assignment] != outer || !plan.MutableBindings[outer] || !plan.AssignmentSourceMatches(assignment) {
		t.Fatal("page backedge lost the captured declaration or authored RHS")
	}
	formatted := syntax.Format(program)
	if !strings.Contains(formatted, "{ page in\n") {
		t.Fatal("formatter dropped lexical header")
	}
	reparsed := scopedPlanProgram(t, formatted)
	if syntax.Format(reparsed) != formatted {
		t.Fatal("page header did not survive syntax roundtrip")
	}
}

func TestPaginatePagePlannerRejectsCopiedSourceAuthority(t *testing.T) {
	for _, fault := range []string{"call", "block", "body", "parameter", "parameter_name", "parameter_optional", "body_enrollment", "parameter_enrollment"} {
		t.Run(fault, func(t *testing.T) {
			_, call, plan := pagePlanFixture(t)
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
			case "parameter_name":
				call.Block.Parameters[0].Name = "other"
			case "parameter_optional":
				call.Block.Parameters[0].Optional = true
			case "body_enrollment":
				delete(plan.ScopedBodies, call)
			case "parameter_enrollment":
				delete(plan.PageCallbacks, call)
			}
			if plan.PageCallbackSourceMatches(call) {
				t.Fatal("changed source acquired page cell authority")
			}
		})
	}
}

func TestPaginatePageDelayRequiresExactExplicitNativeContract(t *testing.T) {
	p := scopedPlanProgram(t, "wait(0)\n")
	call := p.Statements[0].(*syntax.ExprStmt).Expr.(*syntax.CallExpr)
	original, ok := DefaultRegistry().LookupAction("wait")
	if !ok || !LoopResultExplicitDelay(call, original) {
		t.Fatal("canonical numeric Delay rejected")
	}
	for _, fault := range []string{"callee", "omitted", "named", "native_id", "definition", "output", "key", "codec", "optional", "reference", "accepted_forms", "custom", "variant", "static"} {
		t.Run(fault, func(t *testing.T) {
			c := *call
			a := *original
			a.Parameters = append([]ParameterSchema(nil), original.Parameters...)
			switch fault {
			case "callee":
				c.Callee = &syntax.IdentExpr{Name: "alias"}
			case "omitted":
				c.PrimaryArg = nil
			case "named":
				c.NamedArgs = []syntax.NamedArg{{Label: "seconds", Value: c.PrimaryArg}}
			case "native_id":
				a.AppleIdentifier = "example.delay"
			case "definition":
				a.ID = "foreign"
			case "output":
				a.OutputType = types.AnyContent
			case "key":
				a.Parameters[0].WireKey = "WFInput"
			case "codec":
				a.Parameters[0].Codec = "token"
			case "optional":
				a.Parameters[0].Optional = true
			case "reference":
				a.Parameters[0].Reference = true
			case "accepted_forms":
				a.Parameters[0].AcceptedForms = []string{"attachment(output)"}
			case "custom":
				a.Custom = false
			case "variant":
				a.Variant = "other"
			case "static":
				a.StaticParameters = map[string]any{"WFDelayTime": 0}
			}
			if LoopResultExplicitDelay(&c, &a) {
				t.Fatal("noncanonical/implicit action gained closed-cell admission")
			}
		})
	}
}
