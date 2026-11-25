package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func mutableLoopAssignmentLowerer(t *testing.T) (*Lowerer, *syntax.AssignStmt) {
	t.Helper()
	text := "let first=repeat 2 {yield [1,2]}\nlet next=repeat 0 {yield [3,4]}\nvar current:List<List<Number>>=first\nlet old=current\ncurrent=next\nreturn count(old)+count(current)"
	parser := syntax.NewParser(source.NewFile("mutable-loop-lower.cherri", "mutable-loop-lower.cherri", 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	assignment, okay := program.Statements[4].(*syntax.AssignStmt)
	if !okay {
		t.Fatal("fixture lost its actual assignment")
	}
	l := NewLowerer(nil)
	l.loopResultLocalPlan = schema.PlanLoopResultLocals(program, l.registry)
	if l.loopResultLocalPlan == nil {
		t.Fatal("healthy fixture did not receive a source-owned plan")
	}
	element := types.NewList(types.Number)
	sequence := types.NewList(element)
	l.exprTypes = map[syntax.Expression]types.Type{assignment.Value: sequence}
	l.bindings["current"] = BindingRef{IsVariable: true, VariableName: "current", Type: sequence, LoopResultElement: element}
	l.bindings["next"] = BindingRef{OutputUUID: "7cc0a35d-82c2-4b5e-bf6b-92819f6c245a", Type: sequence, LoopResultElement: element}
	return l, assignment
}

func TestLoopResultMutableAssignmentIndependentLowerBoundary(t *testing.T) {
	for _, fault := range []string{
		"absent_plan", "cloned_assignment", "unenrolled_binding", "no_explicit_type", "changed_binding_mutability", "unselected_rhs",
		"wrong_target_binding", "immutable_target", "ordinary_same_type_target",
		"ordinary_same_type_rhs", "mutable_rhs", "changed_element", "widened_target",
		"widened_rhs", "widened_rhs_binding", "compound_append",
	} {
		t.Run(fault, func(t *testing.T) {
			l, assignment := mutableLoopAssignmentLowerer(t)
			if err := l.validateLocalLoopResultAssignment(assignment); err != nil {
				t.Fatal("unchanged exact typed/source-owned assignment", err)
			}
			target, rhs := l.bindings["current"], l.bindings["next"]
			switch fault {
			case "absent_plan":
				l.loopResultLocalPlan = nil
			case "cloned_assignment":
				copy := *assignment
				assignment = &copy
			case "unenrolled_binding":
				delete(l.loopResultLocalPlan.MutableBindings, l.loopResultLocalPlan.Assignments[assignment])
			case "no_explicit_type":
				l.loopResultLocalPlan.Assignments[assignment].TypeExpr = nil
			case "changed_binding_mutability":
				l.loopResultLocalPlan.Assignments[assignment].Mutable = false
			case "unselected_rhs":
				delete(l.loopResultLocalPlan.AllowedValues, assignment.Value)
			case "wrong_target_binding":
				l.bindings["other"] = target
				assignment.Name = "other"
			case "immutable_target":
				target.IsVariable = false
				l.bindings["current"] = target
			case "ordinary_same_type_target":
				target.LoopResultElement = nil
				l.bindings["current"] = target
			case "ordinary_same_type_rhs":
				rhs.LoopResultElement = nil
				l.bindings["next"] = rhs
			case "mutable_rhs":
				rhs.IsVariable = true
				l.bindings["next"] = rhs
			case "changed_element":
				rhs.LoopResultElement = types.Text
				l.bindings["next"] = rhs
			case "widened_target":
				target.Type = types.AnyContent
				l.bindings["current"] = target
			case "widened_rhs":
				l.exprTypes[assignment.Value] = types.AnyContent
			case "widened_rhs_binding":
				rhs.Type = types.AnyContent
				l.bindings["next"] = rhs
			case "compound_append":
				assignment.Op = syntax.TokenPlusAssign
			}
			before := len(l.workflow.Actions)
			if err := l.lowerAssign(assignment); err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
				t.Fatal("changed provenance reached ordinary assignment", err)
			}
			if len(l.workflow.Actions) != before {
				t.Fatal("rejected cell assignment emitted a partial RHS or store")
			}
		})
	}
}

func TestLoopResultMutableBoundaryPreservesOrdinaryAssignment(t *testing.T) {
	parser := syntax.NewParser(source.NewFile("ordinary-assignment.cherri", "ordinary-assignment.cherri", 1, "var scalar=1\nscalar=2\nreturn scalar"))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	assignment := program.Statements[1].(*syntax.AssignStmt)
	l := NewLowerer(nil)
	l.bindings["scalar"] = BindingRef{IsVariable: true, VariableName: "scalar", Type: types.Number}
	l.exprTypes = map[syntax.Expression]types.Type{assignment.Value: types.Number}
	if err := l.lowerAssign(assignment); err != nil {
		t.Fatal("ordinary scalar assignment changed", err)
	}
	if len(l.workflow.Actions) == 0 || l.bindings["scalar"].LoopResultElement != nil {
		t.Fatal("ordinary assignment vanished or acquired private cell metadata")
	}
}
