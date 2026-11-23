package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// The declared item shadows the actual source-owned iterable. Both the input
// capture before shadowing and restoration after the body are observable.
const p15F04LocalScopeSource = `let before="F04_SCOPE_BEFORE"
let values=repeat 2 {yield none}
let seen=for values in values {yield values != none}
return count(values)`

func p15F04LocalScopeGraph(actual []map[string]any) (p15F04LocalTrace, error) {
	trace := p15F04LocalTrace{positions: map[string]int{}}
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	g.value("gettext", map[string]any{"WFTextActionText": "F04_SCOPE_BEFORE"})
	accumulator := p15F04ClearedCollection(g, "loopResults")
	producer := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", 2)
	cell := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", false, p15FunctionArgument{})
	g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
	p15DiscoveryLoopEnd(g, "repeat.count", producer, false)
	trace.mark(g, "outer_sequence")
	collection := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	trace.mark(g, "consumer_loop")
	consumer := p15DiscoveryLoop(g, "repeat.each", "WFInput", p15FunctionEdge(collection))
	trace.mark(g, "item_capture")
	selected := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("Repeat Item")})
	value, presence := p15F04LocalDecode(g, selected, &trace)
	innerName := p15F04PrivateName(g, "__cherri_scope_")
	if !strings.HasSuffix(innerName, "_values") || innerName == accumulator {
		g.err = fmt.Errorf("F04_LOCAL_SCOPE_IDENTITY: shadowing item has no distinct lexical variable")
	}
	trace.mark(g, "inner_item")
	g.emit("setvariable", map[string]any{"WFVariableName": innerName, "WFInput": p15FunctionEdge(value)})
	p15F04LocalPresenceBoolean(g, p15FunctionEdge(presence), &trace)
	p15DiscoveryLoopEnd(g, "repeat.each", consumer, true)
	trace.mark(g, "restored_outer_consumer")
	result := g.value("count", map[string]any{"Input": p15FunctionEdge(collection)})
	g.output(result)
	if err := g.compare(); err != nil {
		return trace, fmt.Errorf("F04_LOCAL_SCOPE_GRAPH: %w", err)
	}
	return trace, nil
}

func TestP15F04LocalCellScopeRestoration(t *testing.T) {
	original := p15CompiledActions(t, p15F04LocalScopeSource)
	trace, err := p15F04LocalScopeGraph(original)
	if err != nil {
		t.Fatal("healthy shadow/capture/restoration source", err)
	}
	t.Run("complete_source_capture_and_restoration", func(t *testing.T) {
		if _, err := p15F04LocalScopeGraph(p15C01Clone(t, original)); err != nil {
			t.Fatal(err)
		}
	})
	for _, mutation := range []string{"for_reads_shadow_instead_of_outer", "body_exposes_physical_cell", "after_loop_reads_inner", "shadow_reuses_collection_variable"} {
		t.Run(mutation, func(t *testing.T) {
			bad := p15C01Clone(t, original)
			if _, err := p15F04LocalScopeGraph(bad); err != nil {
				t.Fatal("unchanged causal clone", err)
			}
			before := p15C01Clone(t, bad)
			inner := p15Param(bad[trace.positions["inner_item"]])["WFVariableName"]
			outer := p15Param(bad[trace.positions["outer_sequence"]])["WFVariable"]
			switch mutation {
			case "for_reads_shadow_instead_of_outer":
				p15Param(bad[trace.positions["consumer_loop"]])["WFInput"] = p15FunctionVariable(inner.(string))
			case "body_exposes_physical_cell":
				p15Param(bad[trace.positions["inner_item"]])["WFInput"] = p15FunctionVariable("Repeat Item")
			case "after_loop_reads_inner":
				p15Param(bad[trace.positions["restored_outer_consumer"]])["Input"] = p15FunctionVariable(inner.(string))
			case "shadow_reuses_collection_variable":
				p15Param(bad[trace.positions["inner_item"]])["WFVariableName"] = outer.(map[string]any)["Value"].(map[string]any)["VariableName"]
			}
			if reflect.DeepEqual(before, bad) {
				t.Fatal("causal edit changed no field")
			}
			if _, err := p15F04LocalScopeGraph(bad); err == nil || !strings.Contains(err.Error(), "F04_LOCAL_SCOPE_GRAPH") {
				t.Fatal("scope corruption was not rejected", err)
			}
		})
	}
}

// The existing qualified closed-Count oracle already describes the complete
// dynamic Optional cell construction. Moving the exact same producer into a
// local binding must keep that independently tracked flag even when Count's
// result is discarded and there is no source None comparison.
func TestP15F04LocalCellOptionalDemand(t *testing.T) {
	source := "var candidate:Text?=none\nlet values=repeat 2 {yield candidate}\ncount(values)\nreturn 7"
	tc := p15F04CellCountCase{name: "local_binding_unused_count", route: "mutable", value: "none", count: 2}
	original := p15CompiledActions(t, source)
	if err := p15F04CellCountGraphError(original, tc); err != nil {
		t.Fatal("actual local cell Optional construction", err)
	}
	t.Run("local_constructor_retains_independent_flag", func(t *testing.T) {
		if err := p15F04CellCountGraphError(p15C01Clone(t, original), tc); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("changed_presence_branch_rejected", func(t *testing.T) {
		bad := p15C01Clone(t, original)
		if err := p15F04CellCountGraphError(bad, tc); err != nil {
			t.Fatal("unchanged causal clone", err)
		}
		found := -1
		for i, action := range bad {
			p := p15Param(action)
			if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" || p["WFConditionalActionString"] != "1" {
				continue
			}
			if found != -1 {
				t.Fatal("ambiguous authored Optional presence guard")
			}
			found = i
		}
		if found < 0 {
			t.Fatal("independent presence guard absent")
		}
		p15Param(bad[found])["WFConditionalActionString"] = "0"
		if err := p15F04CellCountGraphError(bad, tc); err == nil {
			t.Fatal("wrong presence branch retained graph acceptance")
		}
	})
}

// Both failures found during static review are kept separate from the original
// RED fixture: demand must reach the exact origin, and legacy main output is
// not permission to expose a private representation.
func TestP15F04LocalCellAdmissionBoundaries(t *testing.T) {
	cases := []struct{ name, source, result string }{
		{"unused_repeat_keeps_old_boundary", "let values=repeat 2 {yield []}", "[]"},
		{"unused_for_keeps_old_boundary", "let values=for item in [1,2] {yield []}", "[]"},
		{"consuming_other_origin_is_insufficient", "let unused=repeat 2 {yield [\"UNCONSUMED\"]}\nlet values=repeat 2 {yield []}\nreturn count(values)", "[\"UNCONSUMED\"]"},
		{"legacy_mode_with_real_consumer_is_not_selected", "shortcut \"\" {outputMode:\"legacy\"}\nlet values=repeat 2 {yield []}\nreturn count(values)", "[]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15ExactLiteralDiagnostic(p15TypeDiagnostics(t, tc.source), tc.source, tc.result, analysis.CodeTargetValueShape); err != nil {
				t.Fatal("exact source admission boundary", err)
			}
			raw, err := CompileSourceToPlist("p15-f04-local-admission.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatal("unselected source path emitted native cells", len(raw), err)
			}
		})
	}
}
