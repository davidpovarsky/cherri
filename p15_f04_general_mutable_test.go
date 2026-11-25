package main

import (
	"fmt"
	"reflect"
	"testing"
)

type p15F04GeneralMutableCase struct {
	kind     string
	inferred bool
	value    p15F04MutableAliasCase
}

func p15F04GeneralMutableCases() []p15F04GeneralMutableCase {
	var cases []p15F04GeneralMutableCase
	for _, kind := range []string{"alias", "rhs_init", "rhs_assign", "self", "producer", "assignment", "branch_assignment", "loop_assignment", "swap", "for_initial", "for_assignment"} {
		for _, inferred := range []bool{false, true} {
			cases = append(cases, p15F04GeneralMutableCase{kind: kind, inferred: inferred,
				value: p15F04MutableAliasCase{value: "list", first: 2, second: 1}})
		}
	}
	for _, value := range []string{"false", "zero", "empty_text"} {
		cases = append(cases, p15F04GeneralMutableCase{kind: "rhs_assign", inferred: true,
			value: p15F04MutableAliasCase{value: value, first: 2, second: 1, optional: true, secondAbsent: true}})
		cases = append(cases, p15F04GeneralMutableCase{kind: "assignment", inferred: true,
			value: p15F04MutableAliasCase{value: value, first: 2, second: 1, optional: true, secondAbsent: true}})
	}
	return cases
}

func (tc p15F04GeneralMutableCase) name() string {
	return fmt.Sprintf("%s/inferred_%t/%s", tc.kind, tc.inferred, tc.value.value)
}

func (tc p15F04GeneralMutableCase) bind(name, value string) string {
	var annotation string
	if !tc.inferred {
		annotation = ":List<" + tc.value.elementType() + ">"
	}
	return "var " + name + annotation + "=" + value + "\n"
}

func (tc p15F04GeneralMutableCase) directFor() string {
	var literal, _ = tc.value.payload(2)
	return "for item in first {yield " + literal + "}"
}

func (tc p15F04GeneralMutableCase) hasOther() bool {
	return tc.kind == "rhs_init" || tc.kind == "rhs_assign" || tc.kind == "swap"
}

func (tc p15F04GeneralMutableCase) source() string {
	// Independent consumers keep the pre-existing prefix valid. These cases
	// test the new storage interface; separate flow tests prove origin demand.
	var source = "let first=" + tc.value.loop(0, 2, false) + "\n" +
		"let second=" + tc.value.loop(1, 1, tc.value.secondAbsent) + "\ncount(first)\ncount(second)\n"
	var initial = "first"
	if tc.kind == "producer" {
		initial = tc.value.loop(2, 1, tc.value.secondAbsent)
	} else if tc.kind == "for_initial" {
		initial = tc.directFor()
	}
	source += tc.bind("current", initial) + "let old=current\n"
	switch tc.kind {
	case "alias", "producer", "for_initial":
		source += "current=second\n"
	case "rhs_init":
		source += tc.bind("other", "current") + "current=second\n"
	case "rhs_assign":
		source += tc.bind("other", "second") + "current=other\nother=first\n"
	case "self":
		source += "current=current\n"
	case "assignment":
		source += "current=" + tc.value.loop(2, 3, tc.value.secondAbsent) + "\n"
	case "branch_assignment":
		source += "if 1==1 {current=" + tc.value.loop(2, 3, false) + "}\n"
	case "loop_assignment":
		source += "repeat 2 {current=" + tc.value.loop(2, 0, false) + "}\n"
	case "swap":
		source += tc.bind("other", "second") + "repeat 3 {let previous=current\ncurrent=other\nother=previous}\n"
	case "for_assignment":
		source += "current=" + tc.directFor() + "\n"
	}
	if tc.hasOther() {
		return source + "return count(old)*100+count(current)*10+count(other)"
	}
	return source + "return count(old)*10+count(current)"
}

func p15F04GeneralForProducer(g *p15FunctionGraph, tc p15F04GeneralMutableCase, first string, trace *p15F04LocalTrace) p15F04MutableOrigin {
	trace.mark(g, "direct_zero_clear")
	var accumulator = p15F04ClearedCollection(g, "loopResults")
	var loop = p15DiscoveryLoop(g, "repeat.each", "WFInput", p15FunctionEdge(first))
	var cell = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("Repeat Item")})
	var item, _ = p15F04LocalDecode(g, cell, trace)
	g.emit("setvariable", map[string]any{"WFVariableName": "item", "WFInput": p15FunctionEdge(item)})
	var _, payload = tc.value.payload(2)
	var result = p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", true, payload)
	trace.mark(g, "direct_append")
	g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(result)})
	p15DiscoveryLoopEnd(g, "repeat.each", loop, false)
	var collection = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	return p15F04MutableOrigin{accumulator: accumulator, collection: collection}
}

// Expected operations, native parameters, source edges and control ordering
// are authored here. Only identities are read from required producer positions.
// This is actual compiler serialization, not an Apple-runtime substitute.
func p15F04GeneralMutableGraphError(actual []map[string]any, tc p15F04GeneralMutableCase) (p15F04LocalTrace, error) {
	var trace = p15F04LocalTrace{positions: map[string]int{}}
	var g = &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	var first = p15F04MutableProducerGraph(g, tc.value, 0, 2, false, "first", &trace)
	var second = p15F04MutableProducerGraph(g, tc.value, 1, 1, tc.value.secondAbsent, "second", &trace)
	if first.accumulator == second.accumulator {
		g.err = fmt.Errorf("independent source producers reused an accumulator")
	}
	g.emit("count", map[string]any{"Input": p15FunctionEdge(first.collection)})
	g.emit("count", map[string]any{"Input": p15FunctionEdge(second.collection)})
	var direct = func(count int, absent bool) string {
		var result = p15F04MutableProducerGraph(g, tc.value, 2, count, absent, "direct", &trace)
		if result.accumulator == first.accumulator || result.accumulator == second.accumulator {
			g.err = fmt.Errorf("direct producer reused an earlier accumulator")
		}
		return result.collection
	}
	var directFor = func() string {
		var result = p15F04GeneralForProducer(g, tc, first.collection, &trace)
		if result.accumulator == first.accumulator || result.accumulator == second.accumulator {
			g.err = fmt.Errorf("direct for producer reused an earlier accumulator")
		}
		return result.collection
	}
	var initial = first.collection
	if tc.kind == "producer" {
		initial = direct(1, tc.value.secondAbsent)
	} else if tc.kind == "for_initial" {
		initial = directFor()
	}
	var assign = func(name string, input any, role string) {
		trace.mark(g, role)
		g.emit("setvariable", map[string]any{"WFVariableName": name, "WFInput": input})
	}
	assign("current", p15FunctionEdge(initial), "initialize")
	trace.mark(g, "old_snapshot")
	var old = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("current")})
	switch tc.kind {
	case "alias", "producer", "for_initial":
		assign("current", p15FunctionEdge(second.collection), "write")
	case "rhs_init":
		assign("other", p15FunctionVariable("current"), "initialize_other")
		assign("current", p15FunctionEdge(second.collection), "write")
	case "rhs_assign":
		assign("other", p15FunctionEdge(second.collection), "initialize_other")
		assign("current", p15FunctionVariable("other"), "write")
		assign("other", p15FunctionEdge(first.collection), "later_source_write")
	case "self":
		assign("current", p15FunctionVariable("current"), "write")
	case "assignment":
		var result = direct(3, tc.value.secondAbsent)
		assign("current", p15FunctionEdge(result), "write")
	case "branch_assignment":
		var one = g.value("number", map[string]any{"WFNumberActionNumber": 1})
		var branch = g.conditional(4, p15FunctionEdge(one), "WFNumberValue", 1)
		var result = direct(3, false)
		assign("current", p15FunctionEdge(result), "write")
		g.boundary(branch, 2, false)
	case "loop_assignment":
		trace.mark(g, "outer_loop")
		var loop = p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", 2)
		var result = direct(0, false)
		assign("current", p15FunctionEdge(result), "write")
		p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	case "swap":
		assign("other", p15FunctionEdge(second.collection), "initialize_other")
		var loop = p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", 3)
		trace.mark(g, "iteration_snapshot")
		var previous = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("current")})
		assign("current", p15FunctionVariable("other"), "write")
		assign("other", p15FunctionEdge(previous), "later_source_write")
		p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	case "for_assignment":
		var result = directFor()
		assign("current", p15FunctionEdge(result), "write")
	}
	trace.mark(g, "old_count")
	var oldCount = g.value("count", map[string]any{"Input": p15FunctionEdge(old)})
	var scale = 10
	if tc.hasOther() {
		scale = 100
	}
	var scaled = g.math("×", oldCount, scale)
	trace.mark(g, "current_count")
	var current = g.value("count", map[string]any{"Input": p15FunctionVariable("current")})
	if tc.hasOther() {
		current = g.math("×", current, 10)
	}
	var result = g.math("+", scaled, p15FunctionEdge(current))
	if tc.hasOther() {
		var other = g.value("count", map[string]any{"Input": p15FunctionVariable("other")})
		result = g.math("+", result, p15FunctionEdge(other))
	}
	g.output(result)
	return trace, g.compare()
}

func TestP15F04GeneralMutableNativeGraphs(t *testing.T) {
	for _, tc := range p15F04GeneralMutableCases() {
		t.Run(tc.name(), func(t *testing.T) {
			var actual = p15CompiledActions(t, tc.source())
			if _, err := p15F04GeneralMutableGraphError(actual, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F04GeneralMutableGraphRejectsCorruption(t *testing.T) {
	var tc = p15F04GeneralMutableCase{kind: "rhs_assign", inferred: true, value: p15F04MutableAliasCase{value: "list", first: 2, second: 1}}
	var healthy = p15CompiledActions(t, tc.source())
	var trace, err = p15F04GeneralMutableGraphError(healthy, tc)
	if err != nil {
		t.Fatal(err)
	}
	var cases = []struct {
		name   string
		mutate func([]map[string]any)
	}{
		{"read_target_instead_of_source", func(a []map[string]any) {
			p15C01Params(a[trace.positions["write"]])["WFInput"] = p15FunctionVariable("current")
		}},
		{"write_source_instead_of_target", func(a []map[string]any) { p15C01Params(a[trace.positions["write"]])["WFVariableName"] = "other" }},
		{"read_after_later_source_write", func(a []map[string]any) {
			var x, y = trace.positions["write"], trace.positions["later_source_write"]
			a[x], a[y] = a[y], a[x]
		}},
		{"lose_immutable_snapshot", func(a []map[string]any) {
			p15C01Params(a[trace.positions["old_count"]])["Input"] = p15FunctionVariable("current")
		}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			var bad = p15C01Clone(t, healthy)
			item.mutate(bad)
			if reflect.DeepEqual(bad, healthy) {
				t.Fatal("mutation did not change native graph")
			}
			if _, err := p15F04GeneralMutableGraphError(bad, tc); err == nil {
				t.Fatal("corrupted mutable value transfer passed")
			}
		})
	}
}
