package schema

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func mutableLoopPlanProgram(t *testing.T, text string) *syntax.Program {
	t.Helper()
	parser := syntax.NewParser(source.NewFile("mutable-loop-plan.cherri", "mutable-loop-plan.cherri", 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	return program
}

const mutableLoopPlanProducers = "let first=repeat 2 {yield [1,2]}\nlet next=repeat 0 {yield [3,4]}\n"
const mutableLoopPlanBinding = "var current:List<List<Number>>=first\n"

func TestLoopResultMutableAliasPlanReachingOrigins(t *testing.T) {
	cases := []struct {
		name, text  string
		producers   []string
		assignments int
	}{
		{"old_snapshot_and_new_value", mutableLoopPlanProducers + mutableLoopPlanBinding + "let old=current\ncurrent=next\nreturn count(old)+count(current)", []string{"first", "next"}, 1},
		{"initializer_has_independent_consumer", mutableLoopPlanProducers + "count(first)\n" + mutableLoopPlanBinding + "current=next\nreturn count(current)", []string{"first", "next"}, 1},
		{"same_immutable_source", "let first=repeat 1 {yield [1,2]}\n" + mutableLoopPlanBinding + "let old=current\ncurrent=first\nreturn count(old)+count(current)", []string{"first"}, 1},
		{"two_reaching_versions", mutableLoopPlanProducers + "let final=repeat 3 {yield [5,6]}\n" + mutableLoopPlanBinding + "let old=current\ncurrent=next\nlet middle=current\ncurrent=final\nreturn count(old)+count(middle)+count(current)", []string{"first", "next", "final"}, 2},
		{"ordinary_scalar_write_preserved", mutableLoopPlanProducers + "var scalar=0\nscalar+=1\n" + mutableLoopPlanBinding + "let old=current\ncurrent=next\nreturn count(old)+count(current)+scalar", []string{"first", "next"}, 1},
		{"read_only_shadow_restores_outer", mutableLoopPlanProducers + mutableLoopPlanBinding + "let old=current\nif true {let current=9}\ncurrent=next\nreturn count(old)+count(current)", []string{"first", "next"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program := mutableLoopPlanProgram(t, tc.text)
			before := syntax.Format(program)
			plan := PlanLoopResultLocals(program, DefaultRegistry())
			if plan == nil {
				t.Fatal("source-owned reaching definitions were not enrolled")
			}
			bindings := map[string]*syntax.BindingStmt{}
			for _, statement := range program.Statements {
				if binding, okay := statement.(*syntax.BindingStmt); okay {
					bindings[binding.Name] = binding
				}
			}
			current := bindings["current"]
			if current == nil || !plan.MutableBindings[current] || len(plan.MutableBindings) != 1 || len(plan.Assignments) != tc.assignments {
				t.Fatal("mutable admission lost its exact source identities")
			}
			for _, name := range tc.producers {
				binding := bindings[name]
				if binding == nil || !plan.BindingValues[binding.Value] || !plan.Bodies[LoopResultLocalBody(binding.Value)] {
					t.Fatalf("real consumer demand did not reach producer %q", name)
				}
			}
			for assignment, target := range plan.Assignments {
				if target != current || !plan.AllowedValues[assignment.Value] {
					t.Fatal("assignment does not join its actual target and RHS")
				}
				copy := *assignment
				if plan.Assignments[&copy] != nil {
					t.Fatal("same-text cloned assignment inherited enrollment")
				}
			}
			copy := *current
			if plan.MutableBindings[&copy] || syntax.Format(program) != before {
				t.Fatal("planning used binding spelling or changed authored syntax")
			}
		})
	}
}

// Retain the original qualification IDs and all eighteen authored sources.
// Seven former scope limits now have explicit positive admission expectations;
// the eleven representation/enrollment boundaries keep their original rejection.
func TestLoopResultMutableAliasPlanKeepsClosedBoundaries(t *testing.T) {
	demanded := mutableLoopPlanProducers + "count(first)\ncount(next)\n"
	initialized := demanded + mutableLoopPlanBinding
	cases := []struct {
		name, text string
		admitted   bool
	}{
		{"inferred_mutable_alias", demanded + "var current=first\nreturn count(current)", false},
		{"direct_mutable_producer", demanded + "var current:List<List<Number>>=repeat 1 {yield [1,2]}\nreturn count(current)", false},
		{"uninitialized_alias", demanded + "var current:List<List<Number>>\ncurrent=first\nreturn count(current)", false},
		{"mutable_rhs_initializer", initialized + "var other:List<List<Number>>=current\nreturn count(other)+count(current)", false},
		{"mutable_rhs_assignment", initialized + "var other:List<List<Number>>=next\ncurrent=other\nreturn count(other)+count(current)", false},
		{"compound_assignment", initialized + "current+=next\nreturn count(current)", false},
		{"nonidentifier_rhs", initialized + "current=repeat 1 {yield [1,2]}\nreturn count(current)", false},
		{"ordinary_list_rhs", initialized + "let ordinary=[[1,2]]\ncurrent=ordinary\nreturn count(current)", false},
		{"if_write", initialized + "if true {current=next}\nreturn count(current)", true},
		{"else_write", initialized + "if true {let scalar=0} else {current=next}\nreturn count(current)", true},
		{"menu_write", initialized + "menu(\"Pick\") {case \"A\" {current=next} case \"B\" {let scalar=0}}\nreturn count(current)", true},
		{"repeat_write", initialized + "repeat 1 {current=next}\nreturn count(current)", true},
		{"for_write", initialized + "for item in [1] {current=next}\nreturn count(current)", true},
		{"block_write", initialized + "{current=next}\nreturn count(current)", true},
		{"nested_mutable_binding", demanded + "if true {var current:List<List<Number>>=first\ncount(current)}\nreturn count(next)", true},
		{"unread_overwritten_initializer", mutableLoopPlanProducers + mutableLoopPlanBinding + "current=next\nreturn count(current)", false},
		{"unread_assigned_version", mutableLoopPlanProducers + mutableLoopPlanBinding + "let old=current\ncurrent=next\nreturn count(old)", false},
		{"other_origin_demand_is_insufficient", mutableLoopPlanProducers + "let other=repeat 1 {yield [7,8]}\n" + mutableLoopPlanBinding + "current=next\nreturn count(other)+count(current)", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program := mutableLoopPlanProgram(t, tc.text)
			before := syntax.Format(program)
			var plan = PlanLoopResultLocals(program, DefaultRegistry())
			if tc.admitted {
				if plan == nil || len(plan.MutableBindings) != 1 {
					t.Fatal("exact source-owned structured mutable binding was not enrolled")
				}
				for assignment, binding := range plan.Assignments {
					var cloned = *assignment
					if !plan.MutableBindings[binding] || !plan.AllowedValues[assignment.Value] || plan.Assignments[&cloned] != nil {
						t.Fatal("structured assignment lost its original AST identity")
					}
				}
			} else if plan != nil {
				t.Fatal("unenrolled mutable/control-flow/representation path acquired a cell plan")
			}
			if syntax.Format(program) != before {
				t.Fatal("rejection changed authored syntax")
			}
		})
	}
}
