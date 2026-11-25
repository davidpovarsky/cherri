package schema

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// The expected producer set is authored independently of the planner. These
// cases need writes to survive lexical exits and repeated loop backedges.
func TestLoopResultMutableControlFlowOrigins(t *testing.T) {
	var third = "let third=repeat 3 {yield [5,6]}\n"
	var cases = []struct {
		name, body string
		extra      string
		producers  int
	}{
		{"if_then_join", "if true {current=next}\nreturn count(current)", "", 2},
		{"if_else_join", "if false {let scalar=0} else {current=next}\nreturn count(current)", "", 2},
		{"if_both_join", "count(current)\nif true {current=next} else {current=third}\nreturn count(current)", third, 3},
		{"else_if_join", "if true {current=next} else if false {current=third}\nreturn count(current)", third, 3},
		{"menu_join", "menu(\"Pick\") {case \"A\" {current=next} case \"B\" {let scalar=0}}\nreturn count(current)", "", 2},
		{"zero_repeat_keeps_entry", "repeat 0 {current=next}\nreturn count(current)", "", 2},
		{"repeat_backedge", "repeat 3 {let previous=current\ncurrent=next\ncount(previous)}\nreturn count(current)", "", 2},
		{"for_zero_keeps_entry", "for item in [] {current=next}\nreturn count(current)", "", 2},
		{"for_nonempty_join", "for item in [1,2] {current=next}\nreturn count(current)", "", 2},
		{"block_propagates_write", "count(current)\n{current=next}\nreturn count(current)", "", 2},
		{"write_before_scalar_shadow", "count(current)\n{current=next\nlet current=9}\nreturn count(current)", "", 2},
		{"write_before_mutable_shadow", "count(current)\n{current=next\nvar current:List<List<Number>>=first\ncount(current)}\nreturn count(current)", "", 2},
		{"nested_if_in_loop", "repeat 3 {if true {current=next}}\nreturn count(current)", "", 2},
		{"loop_inside_if", "if true {repeat 3 {current=next}}\nreturn count(current)", "", 2},
		{"two_backedge_dependencies", "var other:List<List<Number>>=next\nrepeat 3 {let snapshot=other\ncurrent=snapshot\nother=third}\nreturn count(current)", third, 3},
		{"nested_local_mutable", "if true {var local:List<List<Number>>=first\ncount(local)\nlocal=next\ncount(local)}\nreturn count(current)", "", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var program = mutableLoopPlanProgram(t, mutableLoopPlanProducers+tc.extra+mutableLoopPlanBinding+tc.body)
			var before = syntax.Format(program)
			var plan = PlanLoopResultLocals(program, DefaultRegistry())
			if plan == nil {
				t.Fatal("structured reaching origins were not enrolled")
			}
			var producers int
			for _, statement := range program.Statements {
				if binding, okay := statement.(*syntax.BindingStmt); okay && LoopResultLocalBody(binding.Value) != nil {
					producers++
					if !plan.BindingValues[binding.Value] || !plan.Bodies[LoopResultLocalBody(binding.Value)] {
						t.Fatalf("consumer demand did not reach actual producer %q", binding.Name)
					}
				}
			}
			if producers != tc.producers || len(plan.Assignments) == 0 || syntax.Format(program) != before {
				t.Fatal("planning changed syntax or lost actual source identities")
			}
			for assignment, binding := range plan.Assignments {
				var cloned = *assignment
				if binding == nil || !plan.MutableBindings[binding] || !plan.AllowedValues[assignment.Value] || plan.Assignments[&cloned] != nil {
					t.Fatal("assignment admission must retain actual lexical binding and AST identities")
				}
			}
		})
	}
}
