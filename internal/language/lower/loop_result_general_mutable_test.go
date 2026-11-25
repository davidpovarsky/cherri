package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func generalMutableLoopAssignmentLowerer(t *testing.T, direct bool) (*Lowerer, *syntax.AssignStmt) {
	t.Helper()
	var text = "let first=repeat 2 {yield [1,2]}\nlet next=repeat 1 {yield [3,4]}\ncount(first)\ncount(next)\nvar current=first\nlet old=current\n"
	if direct {
		text += "current=repeat 3 {yield [5,6]}\nreturn count(old)+count(current)"
	} else {
		text += "var other=next\ncurrent=other\nreturn count(old)+count(current)+count(other)"
	}
	var parser = syntax.NewParser(source.NewFile("general-mutable-lower.cherri", "general-mutable-lower.cherri", 1, text))
	var program = parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	var assignment *syntax.AssignStmt
	for _, statement := range program.Statements {
		if item, okay := statement.(*syntax.AssignStmt); okay {
			assignment = item
		}
	}
	if assignment == nil {
		t.Fatal("fixture lost its authored assignment")
	}
	var l = NewLowerer(nil)
	l.loopResultLocalPlan = schema.PlanLoopResultLocals(program, l.registry)
	if l.loopResultLocalPlan == nil {
		t.Fatal("new source was not enrolled")
	}
	var element = types.NewList(types.Number)
	var sequence = types.NewList(element)
	l.exprTypes = map[syntax.Expression]types.Type{assignment.Value: sequence}
	l.bindings["current"] = BindingRef{IsVariable: true, VariableName: "current", Type: sequence, LoopResultElement: element}
	l.bindings["other"] = BindingRef{IsVariable: true, VariableName: "other", Type: sequence, LoopResultElement: element}
	if direct {
		var body = assignment.Value.(*syntax.RepeatStmt).BodyBlock
		var payload = body.Statements[0].(*syntax.YieldStmt).Value.(*syntax.ListExpr)
		l.exprTypes[payload] = element
		for _, item := range payload.Elements {
			l.exprTypes[item] = types.Number
		}
	}
	return l, assignment
}

// These controls invoke the lower boundary directly, independently of semantic
// analysis. Admission of authored inference/mutable reads must not admit
// altered source identity or transport metadata, or emit even a partial RHS.
func TestLoopResultGeneralMutableIndependentLowerBoundary(t *testing.T) {
	for _, direct := range []bool{false, true} {
		var mode = "mutable_rhs"
		if direct {
			mode = "direct_producer"
		}
		t.Run(mode, func(t *testing.T) {
			var faults = []string{"cloned_assignment", "lost_source_record", "insert_target_annotation", "replaced_rhs_ast"}
			if direct {
				faults = append(faults, "cloned_body", "different_enrolled_body", "unselected_body", "lost_expression_type")
			} else {
				faults = append(faults, "erased_runtime_mutability", "changed_source_declaration", "retargeted_identifier")
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					var l, assignment = generalMutableLoopAssignmentLowerer(t, direct)
					if err := l.validateLocalLoopResultAssignment(assignment); err != nil {
						t.Fatal("healthy authored source", err)
					}
					switch fault {
					case "cloned_assignment":
						var copy = *assignment
						assignment = &copy
					case "lost_source_record":
						delete(l.loopResultLocalPlan.AssignmentSources, assignment)
					case "insert_target_annotation":
						var parser = syntax.NewParser(source.NewFile("annotation.cherri", "annotation.cherri", 1, "var value:List<List<Number>>"))
						var program = parser.ParseProgram()
						if len(parser.Errors()) != 0 {
							t.Fatal(parser.Errors())
						}
						l.loopResultLocalPlan.Assignments[assignment].TypeExpr = program.Statements[0].(*syntax.BindingStmt).TypeExpr
					case "replaced_rhs_ast":
						if direct {
							var copy = *assignment.Value.(*syntax.RepeatStmt)
							assignment.Value = &copy
						} else {
							var copy = *assignment.Value.(*syntax.IdentExpr)
							assignment.Value = &copy
						}
					case "cloned_body":
						var loop = assignment.Value.(*syntax.RepeatStmt)
						var copy = *loop.BodyBlock
						loop.BodyBlock = &copy
					case "different_enrolled_body":
						var loop = assignment.Value.(*syntax.RepeatStmt)
						for body := range l.loopResultLocalPlan.Bodies {
							if body != loop.BodyBlock {
								loop.BodyBlock = body
								break
							}
						}
						var payload = loop.BodyBlock.Statements[0].(*syntax.YieldStmt).Value.(*syntax.ListExpr)
						l.exprTypes[payload] = types.NewList(types.Number)
						for _, item := range payload.Elements {
							l.exprTypes[item] = types.Number
						}
					case "unselected_body":
						delete(l.loopResultLocalPlan.Bodies, assignment.Value.(*syntax.RepeatStmt).BodyBlock)
					case "lost_expression_type":
						delete(l.exprTypes, assignment.Value)
					case "erased_runtime_mutability":
						var rhs = l.bindings["other"]
						rhs.IsVariable = false
						l.bindings["other"] = rhs
					case "changed_source_declaration":
						l.loopResultLocalPlan.AssignmentSources[assignment].Source.Mutable = false
					case "retargeted_identifier":
						assignment.Value.(*syntax.IdentExpr).Name = "current"
					}
					var actions, frames, yields = len(l.workflow.Actions), len(l.loopResultFrames), len(l.loopResultYields)
					if err := l.lowerAssign(assignment); err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
						t.Fatal("altered source reached a native store", err)
					}
					if len(l.workflow.Actions) != actions || len(l.loopResultFrames) != frames || len(l.loopResultYields) != yields {
						t.Fatal("rejected source changed the native graph or producer frame")
					}
				})
			}
		})
	}
}

func TestLoopResultGeneralMutableBindingKeepsExactType(t *testing.T) {
	for _, mutableSource := range []bool{false, true} {
		var mode = "immutable_source"
		if mutableSource {
			mode = "mutable_source"
		}
		t.Run(mode, func(t *testing.T) {
			for _, fault := range []string{"widened_expression", "missing_expression_type", "widened_source_binding", "changed_runtime_mutability", "inserted_annotation"} {
				t.Run(fault, func(t *testing.T) {
					var sourceBinding = "let original=repeat 2 {yield [1,2]}\n"
					if mutableSource {
						sourceBinding = "var original=repeat 2 {yield [1,2]}\n"
					}
					var parser = syntax.NewParser(source.NewFile("inferred-binding.cherri", "inferred-binding.cherri", 1,
						sourceBinding+"var current=original\nreturn count(original)+count(current)"))
					var program = parser.ParseProgram()
					if len(parser.Errors()) != 0 {
						t.Fatal(parser.Errors())
					}
					var binding = program.Statements[1].(*syntax.BindingStmt)
					var l = NewLowerer(nil)
					l.loopResultLocalPlan = schema.PlanLoopResultLocals(program, l.registry)
					if l.loopResultLocalPlan == nil {
						t.Fatal("healthy inferred binding was not enrolled")
					}
					var element = types.NewList(types.Number)
					var sequence = types.NewList(element)
					l.bindings["original"] = BindingRef{IsVariable: mutableSource, VariableName: "original", Type: sequence, LoopResultElement: element}
					l.exprTypes = map[syntax.Expression]types.Type{binding.Value: sequence}
					var got, restore, err = l.beginLocalLoopResultBinding(binding)
					if err != nil || got == nil || !got.Equals(element) {
						t.Fatal("healthy source initializer", err)
					}
					restore()
					switch fault {
					case "widened_expression":
						l.exprTypes[binding.Value] = types.AnyContent
					case "missing_expression_type":
						delete(l.exprTypes, binding.Value)
					case "widened_source_binding":
						var rhs = l.bindings["original"]
						rhs.Type = types.AnyContent
						l.bindings["original"] = rhs
					case "changed_runtime_mutability":
						var rhs = l.bindings["original"]
						rhs.IsVariable = !mutableSource
						l.bindings["original"] = rhs
					case "inserted_annotation":
						var annotation = syntax.NewParser(source.NewFile("annotation.cherri", "annotation.cherri", 1, "var value:List<List<Number>>")).ParseProgram()
						binding.TypeExpr = annotation.Statements[0].(*syntax.BindingStmt).TypeExpr
					}
					var before = len(l.workflow.Actions)
					if err := l.lowerBinding(binding); err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
						t.Fatal("altered initializer type/provenance reached a native store", err)
					}
					if len(l.workflow.Actions) != before {
						t.Fatal("rejected initializer emitted a partial native graph")
					}
				})
			}
		})
	}
}
