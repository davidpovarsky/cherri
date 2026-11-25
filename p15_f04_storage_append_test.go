package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type p15F04StorageAppendCase struct {
	kind       string
	iterations int
	inferred   bool
	value      p15F04MutableAliasCase
}

func p15F04StorageAppendCases() []p15F04StorageAppendCase {
	var cases []p15F04StorageAppendCase
	var specs = []struct {
		kind       string
		iterations int
	}{
		{"overwritten_initializer", 0}, {"unread_assigned_version", 0},
		{"other_origin_demand", 0}, {"all_versions_dead", 0},
		{"branch_deadwrite", 0}, {"loop_deadwrite", 0}, {"loop_deadwrite", 3},
		{"append_immutable", 0}, {"append_mutable_rhs", 0}, {"append_self", 0},
		{"append_direct", 0}, {"append_direct", 1}, {"append_direct", 3},
		{"append_repeat", 0}, {"append_repeat", 3}, {"append_for", 0},
		{"append_then_whole", 0}, {"append_shadow", 0},
		{"append_rhs_write", 0}, {"append_rhs_write", 1},
	}
	for _, spec := range specs {
		for _, inferred := range []bool{false, true} {
			cases = append(cases, p15F04StorageAppendCase{kind: spec.kind, iterations: spec.iterations, inferred: inferred,
				value: p15F04MutableAliasCase{value: "list", first: 2, second: 1}})
		}
	}
	for _, value := range []string{"false", "zero", "empty_text"} {
		for _, kind := range []string{"append_immutable", "append_direct"} {
			cases = append(cases, p15F04StorageAppendCase{kind: kind, iterations: 1, inferred: true,
				value: p15F04MutableAliasCase{value: value, first: 2, second: 1, optional: true, secondAbsent: true}})
		}
	}
	return append(cases, p15F04StorageAppendCase{kind: "existing_control", inferred: true,
		value: p15F04MutableAliasCase{value: "list", first: 2, second: 1}})
}

func (tc p15F04StorageAppendCase) name() string {
	return fmt.Sprintf("%s/n%d/inferred_%t/%s", tc.kind, tc.iterations, tc.inferred, tc.value.value)
}

func (tc p15F04StorageAppendCase) bind(name, value string) string {
	return (p15F04GeneralMutableCase{inferred: tc.inferred, value: tc.value}).bind(name, value)
}

func (tc p15F04StorageAppendCase) needsSecond() bool {
	return tc.kind != "append_self" && tc.kind != "append_direct"
}

func (tc p15F04StorageAppendCase) snapshotsOld() bool {
	return tc.kind == "unread_assigned_version" ||
		strings.HasPrefix(tc.kind, "append_") && tc.kind != "append_then_whole" && tc.kind != "append_shadow"
}

func (tc p15F04StorageAppendCase) source() string {
	var source = "let first=" + tc.value.loop(0, 2, false) + "\n"
	if tc.needsSecond() {
		source += "let second=" + tc.value.loop(1, 1, tc.value.secondAbsent) + "\n"
	}
	if tc.kind == "other_origin_demand" {
		source += "let other=" + tc.value.loop(2, 4, false) + "\n"
	}
	if tc.kind == "existing_control" {
		source += "count(second)\n"
	}
	source += tc.bind("current", "first")
	if tc.snapshotsOld() {
		source += "let old=current\n"
	}
	switch tc.kind {
	case "overwritten_initializer", "unread_assigned_version", "other_origin_demand", "all_versions_dead":
		source += "current=second\n"
	case "branch_deadwrite":
		source += "if 1==1 {current=second}\n"
	case "loop_deadwrite":
		source += fmt.Sprintf("repeat %d {current=second}\n", tc.iterations)
	case "append_immutable":
		source += "current+=second\n"
	case "append_mutable_rhs":
		source += tc.bind("other", "second") + "current+=other\nother=first\n"
	case "append_self":
		source += "current+=current\n"
	case "append_direct":
		source += "current+=" + tc.value.loop(2, tc.iterations, tc.value.secondAbsent) + "\n"
	case "append_repeat":
		source += fmt.Sprintf("repeat %d {current+=second}\n", tc.iterations)
	case "append_for":
		source += "for item in current {current+=second}\n"
	case "append_then_whole":
		source += "current+=second\ncurrent=" + tc.value.loop(2, 3, false) + "\n"
	case "append_shadow":
		source += "if 1==1 {" + tc.bind("current", "second") + "current+=first\ncount(current)}\n"
	case "append_rhs_write":
		var literal, _ = tc.value.payload(2)
		source += fmt.Sprintf("current+=repeat %d {current=second\nyield %s}\n", tc.iterations, literal)
	}
	switch tc.kind {
	case "all_versions_dead":
		return source + "return 7"
	case "unread_assigned_version":
		return source + "return count(old)"
	case "branch_deadwrite", "loop_deadwrite":
		return source + "return count(first)"
	case "other_origin_demand":
		return source + "return count(other)*10+count(current)"
	case "append_mutable_rhs":
		return source + "return count(old)*100+count(current)*10+count(other)"
	}
	if tc.snapshotsOld() {
		return source + "return count(old)*10+count(current)"
	}
	return source + "return count(current)"
}

func p15F04StorageAppendGraph(actual []map[string]any, tc p15F04StorageAppendCase) (p15F04LocalTrace, error) {
	var trace = p15F04LocalTrace{positions: map[string]int{}}
	var g = &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	var first = p15F04MutableProducerGraph(g, tc.value, 0, 2, false, "first", &trace)
	var second, other p15F04MutableOrigin
	if tc.needsSecond() {
		second = p15F04MutableProducerGraph(g, tc.value, 1, 1, tc.value.secondAbsent, "second", &trace)
		if first.accumulator == second.accumulator {
			g.err = fmt.Errorf("stored source versions reused an accumulator")
		}
	}
	if tc.kind == "other_origin_demand" {
		other = p15F04MutableProducerGraph(g, tc.value, 2, 4, false, "other", &trace)
		if other.accumulator == first.accumulator || other.accumulator == second.accumulator {
			g.err = fmt.Errorf("unrelated consumer reused mutable storage")
		}
	}
	if tc.kind == "existing_control" {
		g.emit("count", map[string]any{"Input": p15FunctionEdge(second.collection)})
	}
	var store = func(name string, input any, appendValue bool, role string) {
		trace.mark(g, role)
		var action = "setvariable"
		if appendValue {
			action = "appendvariable"
		}
		g.emit(action, map[string]any{"WFVariableName": name, "WFInput": input})
	}
	store("current", p15FunctionEdge(first.collection), false, "initialize")
	var old string
	if tc.snapshotsOld() {
		trace.mark(g, "old_snapshot")
		old = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("current")})
	}
	var direct = func(iterations int, absent bool) string {
		var result = p15F04MutableProducerGraph(g, tc.value, 2, iterations, absent, "direct", &trace)
		if result.accumulator == first.accumulator || result.accumulator == second.accumulator {
			g.err = fmt.Errorf("direct assigned producer reused prior storage")
		}
		return result.collection
	}
	var branchStart = func() string {
		var one = g.value("number", map[string]any{"WFNumberActionNumber": 1})
		return g.conditional(4, p15FunctionEdge(one), "WFNumberValue", 1)
	}
	switch tc.kind {
	case "overwritten_initializer", "unread_assigned_version", "other_origin_demand", "all_versions_dead":
		store("current", p15FunctionEdge(second.collection), false, "write")
	case "branch_deadwrite":
		var group = branchStart()
		store("current", p15FunctionEdge(second.collection), false, "write")
		g.boundary(group, 2, false)
	case "loop_deadwrite", "append_repeat":
		var loop = p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", tc.iterations)
		store("current", p15FunctionEdge(second.collection), tc.kind == "append_repeat", "write")
		p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	case "append_immutable":
		store("current", p15FunctionEdge(second.collection), true, "write")
	case "append_mutable_rhs":
		store("other", p15FunctionEdge(second.collection), false, "initialize_other")
		store("current", p15FunctionVariable("other"), true, "write")
		store("other", p15FunctionEdge(first.collection), false, "later_source_write")
	case "append_self":
		store("current", p15FunctionVariable("current"), true, "write")
	case "append_direct":
		var result = direct(tc.iterations, tc.value.secondAbsent)
		store("current", p15FunctionEdge(result), true, "write")
	case "append_for":
		var loop = p15DiscoveryLoop(g, "repeat.each", "WFInput", p15FunctionVariable("current"))
		var cell = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("Repeat Item")})
		var item, _ = p15F04LocalDecode(g, cell, &trace)
		g.emit("setvariable", map[string]any{"WFVariableName": "item", "WFInput": p15FunctionEdge(item)})
		store("current", p15FunctionEdge(second.collection), true, "write")
		p15DiscoveryLoopEnd(g, "repeat.each", loop, false)
	case "append_then_whole":
		store("current", p15FunctionEdge(second.collection), true, "append")
		var result = direct(3, false)
		store("current", p15FunctionEdge(result), false, "write")
	case "append_shadow":
		var group = branchStart()
		var inner = p15F04PrivateName(g, "__cherri_scope_")
		if !strings.HasSuffix(inner, "_current") {
			g.err = fmt.Errorf("shadow storage did not retain lexical identity")
		}
		store(inner, p15FunctionEdge(second.collection), false, "initialize_shadow")
		store(inner, p15FunctionEdge(first.collection), true, "write")
		g.emit("count", map[string]any{"Input": p15FunctionVariable(inner)})
		g.boundary(group, 2, false)
	case "append_rhs_write":
		trace.mark(g, "direct_zero_clear")
		var accumulator = p15F04ClearedCollection(g, "loopResults")
		if accumulator == first.accumulator || accumulator == second.accumulator {
			g.err = fmt.Errorf("side-effecting RHS reused prior storage")
		}
		var loop = p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", tc.iterations)
		store("current", p15FunctionEdge(second.collection), false, "rhs_target_write")
		var _, payload = tc.value.payload(2)
		var cell = p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", true, payload)
		g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
		p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
		var result = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
		store("current", p15FunctionEdge(result), true, "write")
	}
	var count = func(input any, role string) string {
		trace.mark(g, role)
		return g.value("count", map[string]any{"Input": input})
	}
	var result string
	switch tc.kind {
	case "all_versions_dead":
		result = g.value("number", map[string]any{"WFNumberActionNumber": 7})
	case "unread_assigned_version":
		result = count(p15FunctionEdge(old), "old_count")
	case "branch_deadwrite", "loop_deadwrite":
		result = count(p15FunctionEdge(first.collection), "source_count")
	case "other_origin_demand":
		result = g.math("×", count(p15FunctionEdge(other.collection), "other_count"), 10)
		result = g.math("+", result, p15FunctionEdge(count(p15FunctionVariable("current"), "current_count")))
	default:
		if tc.snapshotsOld() {
			var scale = 10
			if tc.kind == "append_mutable_rhs" {
				scale = 100
			}
			result = g.math("×", count(p15FunctionEdge(old), "old_count"), scale)
			var current = count(p15FunctionVariable("current"), "current_count")
			if tc.kind == "append_mutable_rhs" {
				current = g.math("×", current, 10)
			}
			result = g.math("+", result, p15FunctionEdge(current))
			if tc.kind == "append_mutable_rhs" {
				result = g.math("+", result, p15FunctionEdge(count(p15FunctionVariable("other"), "other_count")))
			}
		} else {
			result = count(p15FunctionVariable("current"), "current_count")
		}
	}
	g.output(result)
	return trace, g.compare()
}

func TestP15F04DeclarationStorageNativeGraphs(t *testing.T) {
	for _, tc := range p15F04StorageAppendCases() {
		t.Run(tc.name(), func(t *testing.T) {
			var actual = p15CompiledActions(t, tc.source())
			if _, err := p15F04StorageAppendGraph(actual, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F04DeclarationStorageGraphRejectsCorruption(t *testing.T) {
	var tc = p15F04StorageAppendCase{kind: "append_immutable", inferred: true,
		value: p15F04MutableAliasCase{value: "list", first: 2, second: 1}}
	var actual = p15CompiledActions(t, tc.source())
	var trace, err = p15F04StorageAppendGraph(actual, tc)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"append_became_overwrite", "wrong_append_source", "wrong_target", "late_snapshot", "stale_current_count", "lost_initializer_clear", "lost_payload"} {
		t.Run(fault, func(t *testing.T) {
			var bad = p15C01Clone(t, actual)
			switch fault {
			case "append_became_overwrite":
				bad[trace.positions["write"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.setvariable"
			case "wrong_append_source":
				p15C01Params(bad[trace.positions["write"]])["WFInput"] = p15FunctionVariable("current")
			case "wrong_target":
				p15C01Params(bad[trace.positions["write"]])["WFVariableName"] = "second"
			case "late_snapshot":
				var x, y = trace.positions["old_snapshot"], trace.positions["write"]
				bad[x], bad[y] = bad[y], bad[x]
			case "stale_current_count":
				var snapshot = p15C01Params(bad[trace.positions["old_snapshot"]])["UUID"].(string)
				p15C01Params(bad[trace.positions["current_count"]])["Input"] = p15FunctionEdge(snapshot)
			case "lost_initializer_clear":
				bad[trace.positions["first_zero_clear"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
			case "lost_payload":
				p15C01Params(bad[trace.positions["second_cell"]])["WFItems"] = map[string]any{}
			}
			if reflect.DeepEqual(bad, actual) {
				t.Fatal("fault did not change native actions")
			}
			if _, err := p15F04StorageAppendGraph(bad, tc); err == nil {
				t.Fatal("corrupted storage/append graph passed")
			}
		})
	}
}
