package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// The two original interface fixtures produce two present empty Lists. Their
// authored text remains in the original table; this oracle specifies every
// native action, cell payload, mutable store and final Count for those texts.
func p15F04PromotedMutableInterfaceGraph(actual []map[string]any, name string) (p15F04LocalTrace, error) {
	var trace = p15F04LocalTrace{positions: map[string]int{}}
	var variable = "values"
	if name == "mutable_alias" {
		variable = "alias"
	} else if name != "mutable_sequence" {
		return trace, fmt.Errorf("unknown original mutable interface %q", name)
	}
	var g = &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	trace.mark(g, "zero_clear")
	var accumulator = p15F04ClearedCollection(g, "loopResults")
	trace.positions["zero_store"] = trace.positions["zero_clear"] + 1
	trace.mark(g, "loop")
	var loop = p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", 2)
	var cell = p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", true, p15F04ArrayValue())
	trace.mark(g, "append")
	g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
	p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	var collection = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	trace.mark(g, "store")
	g.emit("setvariable", map[string]any{"WFVariableName": variable, "WFInput": p15FunctionEdge(collection)})
	trace.mark(g, "count")
	var count = g.value("count", map[string]any{"Input": p15FunctionVariable(variable)})
	g.output(count)
	return trace, g.compare()
}

func p15F04CheckPromotedMutableInterface(t *testing.T, source, name string) {
	t.Helper()
	for _, diagnostic := range p15TypeDiagnostics(t, source) {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatalf("implemented mutable interface still rejected: %+v", diagnostic)
		}
	}
	var actual = p15CompiledActions(t, source)
	var trace, err = p15F04PromotedMutableInterfaceGraph(actual, name)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the original test IDs: each original leaf executes all five causal
	// mutations without creating replacement or extra qualification identities.
	for _, fault := range []string{"lost_iteration", "lost_cell_append", "self_read_initializer", "constant_count_input", "count_before_store"} {
		var bad = p15C01Clone(t, actual)
		switch fault {
		case "lost_iteration":
			p15C01Params(bad[trace.positions["loop"]])["WFRepeatCount"] = 1
		case "lost_cell_append":
			bad[trace.positions["append"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.nothing"
		case "self_read_initializer":
			var variable = p15C01Params(bad[trace.positions["store"]])["WFVariableName"].(string)
			p15C01Params(bad[trace.positions["store"]])["WFInput"] = p15FunctionVariable(variable)
		case "constant_count_input":
			p15C01Params(bad[trace.positions["count"]])["Input"] = 2
		case "count_before_store":
			var x, y = trace.positions["store"], trace.positions["count"]
			bad[x], bad[y] = bad[y], bad[x]
		}
		if reflect.DeepEqual(bad, actual) {
			t.Fatalf("%s did not alter the native graph", fault)
		}
		if _, err := p15F04PromotedMutableInterfaceGraph(bad, name); err == nil {
			t.Fatalf("%s passed the independent native graph oracle", fault)
		}
	}
}
