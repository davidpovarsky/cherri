package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

type p15F04MutableFlowCase struct {
	name, control string
	value         p15F04MutableAliasCase
	iterations    int
}

func p15F04MutableFlowCases() []p15F04MutableFlowCase {
	var cases []p15F04MutableFlowCase
	for _, control := range []string{"block", "block_shadow", "if_then", "if_else", "if_both", "menu", "repeat", "repeat_snapshot", "backedge", "shadow_mutable", "block_mutable_shadow", "for_cells", "nested_producer"} {
		cases = append(cases, p15F04MutableFlowCase{
			name: control, control: control, iterations: 3,
			value: p15F04MutableAliasCase{value: "list", first: 2, second: 1},
		})
	}
	cases = append(cases, p15F04MutableFlowCase{name: "zero_iterations", control: "repeat", iterations: 0,
		value: p15F04MutableAliasCase{value: "list", first: 0, second: 3}})
	cases = append(cases, p15F04MutableFlowCase{name: "optional_false", control: "if_then", iterations: 1,
		value: p15F04MutableAliasCase{value: "false", first: 2, second: 1, optional: true}})
	cases = append(cases, p15F04MutableFlowCase{name: "optional_absence", control: "repeat", iterations: 2,
		value: p15F04MutableAliasCase{value: "empty_text", first: 1, second: 1, optional: true, secondAbsent: true}})
	return cases
}

func (tc p15F04MutableFlowCase) source() string {
	var source = "let first=" + tc.value.loop(0, tc.value.first, false) + "\n" +
		"let second=" + tc.value.loop(1, tc.value.second, tc.value.secondAbsent) + "\n"
	if tc.control == "if_both" || tc.control == "backedge" {
		source += "let third=" + tc.value.loop(2, 3, false) + "\n"
	}
	source += "var current:List<" + tc.value.elementType() + ">=first\nlet old=current\n"
	switch tc.control {
	case "block":
		source += "{current=second}\n"
	case "block_shadow":
		source += "{current=second\nlet current=9}\n"
	case "if_then":
		source += "if 1==1 {current=second}\n"
	case "if_else":
		source += "if 1==2 {} else {current=second}\n"
	case "if_both":
		source += "if 1==1 {current=second} else {current=third}\n"
	case "menu":
		source += "menu(\"Pick\") {case \"A\" {current=second} case \"B\" {current=first}}\n"
	case "repeat":
		source += fmt.Sprintf("repeat %d {current=second}\n", tc.iterations)
	case "repeat_snapshot":
		source += fmt.Sprintf("repeat %d {let previous=current\ncurrent=second\ncount(previous)}\n", tc.iterations)
	case "backedge":
		source += fmt.Sprintf("var other:List<%s>=second\nrepeat %d {let next=other\ncurrent=next\nother=third}\n", tc.value.elementType(), tc.iterations)
	case "nested_producer":
		source += fmt.Sprintf("count(second)\nrepeat %d {let nested=%s\ncurrent=nested}\n", tc.iterations, tc.value.loop(2, 0, false))
	case "for_cells":
		source += "for item in current {current=second}\n"
	case "block_mutable_shadow":
		source += "{current=second\nvar current:List<" + tc.value.elementType() + ">=first\ncount(current)}\n"
	case "shadow_mutable":
		source += "count(second)\n{var current:List<" + tc.value.elementType() + ">=second\ncount(current)\ncurrent=first\ncount(current)}\n"
	}
	return source + "return count(old)*10+count(current)"
}

// This oracle compares every actual serialized native operation and edge;
// identities are learned only at the required producer positions. It does
// not simulate Shortcuts or claim execution on an Apple device.
func p15F04MutableFlowGraphError(actual []map[string]any, tc p15F04MutableFlowCase) (p15F04LocalTrace, error) {
	var trace = p15F04LocalTrace{positions: map[string]int{}}
	var g = &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	var first = p15F04MutableProducerGraph(g, tc.value, 0, tc.value.first, false, "first", &trace)
	var second = p15F04MutableProducerGraph(g, tc.value, 1, tc.value.second, tc.value.secondAbsent, "second", &trace)
	if first.accumulator == second.accumulator {
		g.err = fmt.Errorf("distinct authored producers reused a private accumulator")
	}
	var third p15F04MutableOrigin
	if tc.control == "if_both" || tc.control == "backedge" {
		third = p15F04MutableProducerGraph(g, tc.value, 2, 3, false, "third", &trace)
		if third.accumulator == first.accumulator || third.accumulator == second.accumulator {
			g.err = fmt.Errorf("third authored producer reused a private accumulator")
		}
	}
	trace.mark(g, "initialize")
	g.emit("setvariable", map[string]any{"WFVariableName": "current", "WFInput": p15FunctionEdge(first.collection)})
	trace.mark(g, "old_snapshot")
	var old = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("current")})
	var assign = func(name, source, role string) {
		trace.mark(g, role)
		g.emit("setvariable", map[string]any{"WFVariableName": name, "WFInput": p15FunctionEdge(source)})
	}
	switch tc.control {
	case "block", "block_shadow":
		assign("current", second.collection, "write")
		if tc.control == "block_shadow" {
			g.value("number", map[string]any{"WFNumberActionNumber": 9})
		}
	case "if_then", "if_else", "if_both":
		var one = g.value("number", map[string]any{"WFNumberActionNumber": 1})
		var compared = 1
		if tc.control == "if_else" {
			compared = 2
		}
		var branch = g.conditional(4, p15FunctionEdge(one), "WFNumberValue", compared)
		if tc.control != "if_else" {
			assign("current", second.collection, "write")
		}
		if tc.control != "if_then" {
			g.boundary(branch, 1, false)
			var source = second.collection
			if tc.control == "if_both" {
				source = third.collection
			}
			assign("current", source, "else_write")
		}
		g.boundary(branch, 2, false)
	case "menu":
		var menu = p15OptionalJoinMenuStart(g)
		p15OptionalJoinMenuCase(g, menu, "A")
		assign("current", second.collection, "write")
		p15OptionalJoinMenuCase(g, menu, "B")
		assign("current", first.collection, "else_write")
		p15OptionalJoinMenuEnd(g, menu, false)
	case "repeat", "repeat_snapshot", "backedge":
		if tc.control == "backedge" {
			assign("other", second.collection, "initialize_other")
		}
		trace.mark(g, "control_start")
		var loop = p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", tc.iterations)
		var next = second.collection
		var previous string
		if tc.control == "repeat_snapshot" {
			trace.mark(g, "iteration_snapshot")
			previous = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("current")})
		}
		if tc.control == "backedge" {
			trace.mark(g, "iteration_snapshot")
			next = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("other")})
		}
		assign("current", next, "write")
		if tc.control == "repeat_snapshot" {
			trace.mark(g, "iteration_snapshot_count")
			g.emit("count", map[string]any{"Input": p15FunctionEdge(previous)})
		}
		if tc.control == "backedge" {
			assign("other", third.collection, "backedge_write")
		}
		trace.mark(g, "control_end")
		p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	case "nested_producer":
		g.emit("count", map[string]any{"Input": p15FunctionEdge(second.collection)})
		trace.mark(g, "control_start")
		var loop = p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", tc.iterations)
		var nested = p15F04MutableProducerGraph(g, tc.value, 2, 0, false, "nested", &trace)
		if nested.accumulator == first.accumulator || nested.accumulator == second.accumulator {
			g.err = fmt.Errorf("nested authored producer reused an outer accumulator")
		}
		assign("current", nested.collection, "write")
		trace.mark(g, "control_end")
		p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	case "for_cells":
		trace.mark(g, "control_start")
		var loop = p15DiscoveryLoop(g, "repeat.each", "WFInput", p15FunctionVariable("current"))
		var cell = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("Repeat Item")})
		var value, _ = p15F04LocalDecode(g, cell, &trace)
		g.emit("setvariable", map[string]any{"WFVariableName": "item", "WFInput": p15FunctionEdge(value)})
		assign("current", second.collection, "write")
		trace.mark(g, "control_end")
		p15DiscoveryLoopEnd(g, "repeat.each", loop, false)
	case "block_mutable_shadow":
		assign("current", second.collection, "write")
		var local = p15F04PrivateName(g, "__cherri_scope_")
		if local == "current" || !strings.HasSuffix(local, "_current") {
			g.err = fmt.Errorf("write before a mutable shadow lost lexical identity")
		}
		assign(local, first.collection, "shadow_initialize")
		g.emit("count", map[string]any{"Input": p15FunctionVariable(local)})
	case "shadow_mutable":
		g.emit("count", map[string]any{"Input": p15FunctionEdge(second.collection)})
		var local = p15F04PrivateName(g, "__cherri_scope_")
		if local == "current" || !strings.HasSuffix(local, "_current") {
			g.err = fmt.Errorf("shadowed mutable declaration lost lexical identity")
		}
		assign(local, second.collection, "shadow_initialize")
		g.emit("count", map[string]any{"Input": p15FunctionVariable(local)})
		assign(local, first.collection, "shadow_write")
		g.emit("count", map[string]any{"Input": p15FunctionVariable(local)})
	}
	trace.mark(g, "old_count")
	var oldCount = g.value("count", map[string]any{"Input": p15FunctionEdge(old)})
	var scaled = g.math("×", oldCount, 10)
	trace.mark(g, "current_count")
	var currentCount = g.value("count", map[string]any{"Input": p15FunctionVariable("current")})
	var result = g.math("+", scaled, p15FunctionEdge(currentCount))
	g.output(result)
	return trace, g.compare()
}

func TestP15F04MutableControlFlowNativeGraphs(t *testing.T) {
	for _, tc := range p15F04MutableFlowCases() {
		t.Run(tc.name, func(t *testing.T) {
			var actual = p15CompiledActions(t, tc.source())
			if _, err := p15F04MutableFlowGraphError(actual, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F04MutableControlFlowGraphRejectsCorruption(t *testing.T) {
	var tc = p15F04MutableFlowCase{name: "backedge", control: "backedge", iterations: 3,
		value: p15F04MutableAliasCase{value: "list", first: 2, second: 1}}
	var healthy = p15CompiledActions(t, tc.source())
	var trace, err = p15F04MutableFlowGraphError(healthy, tc)
	if err != nil {
		t.Fatal(err)
	}
	var cases = []struct {
		name   string
		mutate func([]map[string]any)
	}{
		{"stale_rhs", func(a []map[string]any) {
			p15C01Params(a[trace.positions["write"]])["WFInput"] = p15C01Params(a[trace.positions["initialize"]])["WFInput"]
		}},
		{"snapshot_after_assignment", func(a []map[string]any) {
			var x, y = trace.positions["iteration_snapshot"], trace.positions["write"]
			a[x], a[y] = a[y], a[x]
		}},
		{"lost_loop_bound", func(a []map[string]any) {
			p15C01Params(a[trace.positions["control_start"]])["WFRepeatCount"] = 0
		}},
		{"alias_reads_wrong_variable", func(a []map[string]any) {
			p15C01Params(a[trace.positions["iteration_snapshot"]])["WFVariable"] = p15FunctionVariable("current")
		}},
		{"backedge_writes_wrong_binding", func(a []map[string]any) {
			p15C01Params(a[trace.positions["backedge_write"]])["WFVariableName"] = "current"
		}},
		{"post_loop_reads_snapshot", func(a []map[string]any) {
			p15C01Params(a[trace.positions["current_count"]])["Input"] = p15C01Params(a[trace.positions["old_count"]])["Input"]
		}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			var bad = p15C01Clone(t, healthy)
			item.mutate(bad)
			if reflect.DeepEqual(bad, healthy) {
				t.Fatal("mutation did not change actual native graph")
			}
			if _, err := p15F04MutableFlowGraphError(bad, tc); err == nil {
				t.Fatal("corrupted control-flow graph was accepted")
			}
		})
	}
}

func TestP15F04MutableControlFlowSemanticRejections(t *testing.T) {
	for _, control := range []string{"if_then", "repeat", "for_cells", "block_mutable_shadow"} {
		var tc = p15F04MutableFlowCase{control: control, iterations: 2,
			value: p15F04MutableAliasCase{value: "list", first: 2, second: 1}}
		var healthy = tc.source()
		var changes = []struct{ name, from, to string }{
			{"widened_target", "var current:List<List<Text>>=first\nlet old=current", "var current:List<List<AnyContent>>=first\nlet old=current"},
			{"different_source_element", "[\"F04_MUTABLE_NEW_ONE\"]", "[7]"},
		}
		for _, change := range changes {
			t.Run(control+"/"+change.name, func(t *testing.T) {
				if raw, err := CompileSourceToPlist("f04-structured-control.cherri", healthy); err != nil || len(raw) == 0 {
					t.Fatal("exact-typed structured control did not compile", err)
				}
				if strings.Count(healthy, change.from) != 1 {
					t.Fatal("causal type edit is not unique")
				}
				var bad = strings.Replace(healthy, change.from, change.to, 1)
				var diagnostics = p15TypeDiagnostics(t, bad)
				var rejected bool
				for _, diagnostic := range diagnostics {
					if diagnostic.Severity != analysis.SeverityError {
						continue
					}
					if diagnostic.Code == analysis.CodeTargetValueShape {
						rejected = true
					} else if diagnostic.Code != analysis.CodeArgumentType {
						t.Fatalf("unrelated diagnostic masked representation rejection: %+v", diagnostic)
					}
				}
				if !rejected {
					t.Fatalf("structured flow widened its exact cell representation: %+v", diagnostics)
				}
				if raw, err := CompileSourceToPlist("f04-structured-rejection.cherri", bad); err == nil || len(raw) != 0 {
					t.Fatal("invalid structured representation emitted a native plist", err)
				}
			})
		}
	}
}
