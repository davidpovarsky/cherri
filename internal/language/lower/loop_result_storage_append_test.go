package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func storageAppendLowerer(t *testing.T) (*Lowerer, *syntax.AssignStmt) {
	t.Helper()
	var text = "let first=repeat 2 {yield [1,2]}\nlet next=repeat 1 {yield [3,4]}\nvar current=first\nlet old=current\ncurrent+=next\nreturn count(old)+count(current)"
	var parser = syntax.NewParser(source.NewFile("storage-append.cherri", "storage-append.cherri", 1, text))
	var program = parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	var assignment = program.Statements[4].(*syntax.AssignStmt)
	var l = NewLowerer(nil)
	l.loopResultLocalPlan = schema.PlanLoopResultLocals(program, l.registry)
	if l.loopResultLocalPlan == nil {
		t.Fatal("authored append has no source-owned plan")
	}
	var element = types.NewList(types.Number)
	var sequence = types.NewList(element)
	l.exprTypes = map[syntax.Expression]types.Type{assignment.Value: sequence}
	l.bindings["current"] = BindingRef{IsVariable: true, VariableName: "current", Type: sequence, LoopResultElement: element}
	l.bindings["next"] = BindingRef{OutputUUID: "8a1c84c9-6aab-4a57-b5d0-43358c04ea33", Type: sequence, LoopResultElement: element}
	return l, assignment
}

func TestLoopResultStorageAppendIndependentLowerBoundary(t *testing.T) {
	for _, fault := range []string{"healthy_authored_append", "changed_to_assign", "changed_to_subtract", "changed_recorded_operator", "cloned_assignment"} {
		t.Run(fault, func(t *testing.T) {
			var l, assignment = storageAppendLowerer(t)
			if err := l.validateLocalLoopResultAssignment(assignment); err != nil {
				t.Fatal("healthy exact cell append", err)
			}
			switch fault {
			case "changed_to_assign":
				assignment.Op = syntax.TokenAssign
			case "changed_to_subtract":
				assignment.Op = syntax.TokenMinusAssign
			case "changed_recorded_operator":
				var recorded = l.loopResultLocalPlan.AssignmentSources[assignment]
				recorded.Op = syntax.TokenAssign
				l.loopResultLocalPlan.AssignmentSources[assignment] = recorded
			case "cloned_assignment":
				var copy = *assignment
				assignment = &copy
			}
			var before = len(l.workflow.Actions)
			var err = l.lowerAssign(assignment)
			if fault != "healthy_authored_append" {
				if err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
					t.Fatal("changed operator/source identity reached lowering", err)
				}
				if len(l.workflow.Actions) != before {
					t.Fatal("rejected operator emitted a partial RHS or mutation")
				}
				return
			}
			if err != nil || len(l.workflow.Actions) != before+1 {
				t.Fatal("authored append did not produce exactly one native mutation", err)
			}
			var action = l.workflow.Actions[before]
			var input, okay = action.Parameters["WFInput"].(*ir.AttachmentToken)
			if action.AppleIdentifier != "is.workflow.actions.appendvariable" ||
				action.Parameters["WFVariableName"] != "current" || !okay ||
				input.Type != "ActionOutput" || input.OutputUUID != l.bindings["next"].OutputUUID {
				t.Fatal("authored append changed its action, target or evaluated source")
			}
		})
	}
}
