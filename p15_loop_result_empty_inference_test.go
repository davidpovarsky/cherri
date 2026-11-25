package main

import (
	"fmt"
	"reflect"
	"testing"
)

func p15EmptyLoopInferenceSource(initial int) string {
	return fmt.Sprintf("var current=repeat %d {yield []}\nlet old=current\ncurrent=repeat 2 {yield [\"CONCRETE\"]}\nreturn count(old)*10+count(current)", initial)
}
func p15EmptyLoopInferenceGraph(actual []map[string]any, initial int) (p15F04LocalTrace, error) {
	trace := p15F04LocalTrace{positions: map[string]int{}}
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	producer := func(iterations int, value p15FunctionArgument, role string) string {
		trace.mark(g, role+"_clear")
		accumulator := p15F04ClearedCollection(g, "loopResults")
		trace.mark(g, role+"_loop")
		group := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", iterations)
		trace.mark(g, role+"_cell")
		cell := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", true, value)
		trace.mark(g, role+"_append")
		g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
		p15DiscoveryLoopEnd(g, "repeat.count", group, false)
		return g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	}
	first := producer(initial, p15F04ArrayValue(), "empty")
	g.emit("setvariable", map[string]any{"WFVariableName": "current", "WFInput": p15FunctionEdge(first)})
	trace.mark(g, "snapshot")
	old := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("current")})
	next := producer(2, p15F04ArrayValue(p15FunctionString("CONCRETE")), "concrete")
	trace.mark(g, "write")
	g.emit("setvariable", map[string]any{"WFVariableName": "current", "WFInput": p15FunctionEdge(next)})
	trace.mark(g, "old_count")
	priorCount := g.value("count", map[string]any{"Input": p15FunctionEdge(old)})
	scaled := g.math("×", priorCount, 10)
	currentCount := g.value("count", map[string]any{"Input": p15FunctionVariable("current")})
	result := g.math("+", scaled, p15FunctionEdge(currentCount))
	g.output(result)
	return trace, g.compare()
}
func TestP15LoopEmptyInferenceKeepsExactNativeValues(t *testing.T) {
	for _, n := range []int{0, 1, 3} {
		t.Run(fmt.Sprintf("initial_%d", n), func(t *testing.T) {
			actual := p15CompiledActions(t, p15EmptyLoopInferenceSource(n))
			if _, err := p15EmptyLoopInferenceGraph(actual, n); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestP15LoopEmptyInferenceRejectsGraphCorruption(t *testing.T) {
	healthy := p15CompiledActions(t, p15EmptyLoopInferenceSource(1))
	trace, err := p15EmptyLoopInferenceGraph(healthy, 1)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func([]map[string]any)
	}{
		{"zero_instead_of_empty_row", func(a []map[string]any) { p15C01Params(a[trace.positions["empty_loop"]])["WFRepeatCount"] = 0 }},
		{"repeat_count_changed", func(a []map[string]any) { p15C01Params(a[trace.positions["concrete_loop"]])["WFRepeatCount"] = 3 }},
		{"lost_empty_cell", func(a []map[string]any) {
			a[trace.positions["empty_cell"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.nothing"
		}},
		{"snapshot_reads_later_value", func(a []map[string]any) {
			p15C01Params(a[trace.positions["old_count"]])["Input"] = p15FunctionVariable("current")
		}},
		{"wrong_slot_write", func(a []map[string]any) { p15C01Params(a[trace.positions["write"]])["WFVariableName"] = "old" }},
		{"coerced_empty_payload", func(a []map[string]any) {
			p15C01Params(a[trace.positions["empty_cell"]])["WFItems"] = map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{}}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broken := p15C01Clone(t, healthy)
			tc.mutate(broken)
			if reflect.DeepEqual(broken, healthy) {
				t.Fatal("fault did not change bytes")
			}
			if _, err := p15EmptyLoopInferenceGraph(broken, 1); err == nil {
				t.Fatal("source/value corruption accepted")
			}
		})
	}
}
