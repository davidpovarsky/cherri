package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// These fixtures exercise only explicit exact-typed mutable aliases of existing
// immutable cell sequences. The oracle authors source values and every native
// operation/edge; only UUIDs, groups and private alpha names come from their
// required producer positions. This is native structure, not Apple execution.
type p15F04MutableAliasCase struct {
	name, value, consumer  string
	first, second, ordinal int
	optional, secondAbsent bool
}

func p15F04MutableAliasCases() []p15F04MutableAliasCase {
	return []p15F04MutableAliasCase{
		{name: "snapshot_two_to_zero", value: "list", consumer: "count_pair", first: 2, second: 0},
		{name: "snapshot_zero_to_one", value: "list", consumer: "count_pair", first: 0, second: 1},
		{name: "snapshot_one_to_three", value: "list", consumer: "count_pair", first: 1, second: 3},
		{name: "two_assignments_keep_three_versions", value: "list", consumer: "count_three", first: 2, second: 1},
		{name: "restore_from_immutable_snapshot", value: "list", consumer: "count_restore", first: 2, second: 0},
		{name: "index_current_present_false", value: "false", consumer: "index_current", first: 2, second: 1, optional: true},
		{name: "index_snapshot_present_zero", value: "zero", consumer: "index_snapshot", first: 1, second: 0, optional: true},
		{name: "index_current_typed_absence", value: "empty_text", consumer: "index_current", first: 1, second: 1, optional: true, secondAbsent: true},
		{name: "index_current_present_empty_text", value: "empty_text", consumer: "index_current", first: 1, second: 1, optional: true},
		{name: "index_current_after_shrink_upper_bound", value: "list", consumer: "index_current", first: 3, second: 1, ordinal: 1},
		{name: "for_current_decodes_distinct_payload", value: "list", consumer: "for_current", first: 2, second: 1},
		{name: "for_snapshot_preserves_false_presence", value: "false", consumer: "for_snapshot", first: 2, second: 0, optional: true},
		{name: "for_current_shadow_restores", value: "false", consumer: "for_shadow", first: 1, second: 2, optional: true},
	}
}

func (tc p15F04MutableAliasCase) elementType() string {
	var name string
	switch tc.value {
	case "list":
		name = "List<Text>"
	case "false":
		name = "Bool"
	case "zero":
		name = "Number"
	case "empty_text":
		name = "Text"
	default:
		panic("unmodeled mutable alias element")
	}
	if tc.optional {
		name += "?"
	}
	return name
}

func (tc p15F04MutableAliasCase) payload(origin int) (string, p15FunctionArgument) {
	if tc.value != "list" {
		return p15F04CellLiteral(tc.value), p15F04CellValue(tc.value)
	}
	// Different payloads make a stale producer observable beyond its outer
	// cardinality. Each producer still has exactly the same semantic List<Text>.
	values := [][]string{
		{"F04_MUTABLE_OLD_ONE", "F04_MUTABLE_OLD_TWO"},
		{"F04_MUTABLE_NEW_ONE"},
		{"F04_MUTABLE_LAST_ONE", "F04_MUTABLE_LAST_TWO", "F04_MUTABLE_LAST_THREE"},
	}[origin]
	literals := make([]string, len(values))
	arguments := make([]p15FunctionArgument, len(values))
	for i, value := range values {
		literals[i] = fmt.Sprintf("%q", value)
		arguments[i] = p15FunctionString(value)
	}
	return "[" + strings.Join(literals, ",") + "]", p15F04ArrayValue(arguments...)
}

func (tc p15F04MutableAliasCase) loop(origin, iterations int, absent bool) string {
	literal, _ := tc.payload(origin)
	body := "yield " + literal
	if tc.optional {
		right := 1
		if absent {
			right = 2
		}
		// Both branches establish the same T? even when the selected result is
		// None. No pure-Unknown annotation is used to manufacture provenance.
		body = fmt.Sprintf("if 1==%d {%s} else {yield none}", right, body)
	}
	return fmt.Sprintf("repeat %d {%s}", iterations, body)
}

func (tc p15F04MutableAliasCase) source() string {
	source := "let before=\"F04_MUTABLE_AUTHORED_BEFORE\"\n" +
		"let first=" + tc.loop(0, tc.first, false) + "\n" +
		"let second=" + tc.loop(1, tc.second, tc.secondAbsent) + "\n"
	if tc.consumer == "count_three" {
		source += "let third=" + tc.loop(2, 0, false) + "\n"
	}
	source += "var current:List<" + tc.elementType() + "> = first\n" +
		"let snapshot=current\ncurrent=second\n"
	switch tc.consumer {
	case "count_pair":
		return source + "return count(snapshot)*10+count(current)"
	case "count_three":
		return source + "let middle=current\ncurrent=third\nreturn count(snapshot)*100+count(middle)*10+count(current)"
	case "count_restore":
		return source + "count(current)\ncurrent=snapshot\nreturn count(snapshot)*10+count(current)"
	case "index_current":
		return source + fmt.Sprintf("count(snapshot)\nreturn current[%d] != none", tc.ordinal)
	case "index_snapshot":
		return source + fmt.Sprintf("count(current)\nreturn snapshot[%d] != none", tc.ordinal)
	case "for_current":
		return source + "count(snapshot)\nreturn for item in current {yield count(item)}"
	case "for_snapshot":
		return source + "count(current)\nreturn for item in snapshot {yield item != none}"
	case "for_shadow":
		return source + "count(snapshot)\nlet seen=for current in current {yield current != none}\nreturn count(current)"
	}
	panic("unmodeled mutable alias consumer")
}

type p15F04MutableOrigin struct {
	accumulator, collection string
}

func p15F04MutableProducerGraph(g *p15FunctionGraph, tc p15F04MutableAliasCase, origin, iterations int, absent bool, role string, trace *p15F04LocalTrace) p15F04MutableOrigin {
	trace.mark(g, role+"_zero_clear")
	accumulator := p15F04ClearedCollection(g, "loopResults")
	trace.positions[role+"_zero_store"] = trace.positions[role+"_zero_clear"] + 1
	trace.mark(g, role+"_loop")
	loop := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", iterations)
	_, payload := tc.payload(origin)
	appendCell := func(present bool) {
		if _, exists := trace.positions[role+"_cell"]; !exists {
			trace.mark(g, role+"_cell")
		}
		value := payload
		if !present {
			value = p15FunctionArgument{}
		}
		cell := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", present, value)
		if _, exists := trace.positions[role+"_append"]; !exists {
			trace.mark(g, role+"_append")
		}
		g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
	}
	if tc.optional {
		one := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		right := 1
		if absent {
			right = 2
		}
		branch := g.conditional(4, p15FunctionEdge(one), "WFNumberValue", right)
		appendCell(true)
		g.boundary(branch, 1, false)
		appendCell(false)
		g.boundary(branch, 2, false)
	} else {
		appendCell(true)
	}
	trace.mark(g, role+"_loop_end")
	p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	trace.mark(g, role+"_read")
	collection := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	return p15F04MutableOrigin{accumulator: accumulator, collection: collection}
}

func p15F04MutableAliasGraphError(actual []map[string]any, tc p15F04MutableAliasCase) (p15F04LocalTrace, error) {
	trace := p15F04LocalTrace{positions: map[string]int{}}
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	g.value("gettext", map[string]any{"WFTextActionText": "F04_MUTABLE_AUTHORED_BEFORE"})
	first := p15F04MutableProducerGraph(g, tc, 0, tc.first, false, "first", &trace)
	second := p15F04MutableProducerGraph(g, tc, 1, tc.second, tc.secondAbsent, "second", &trace)
	if first.accumulator == second.accumulator {
		g.err = fmt.Errorf("distinct authored origins reused one private accumulator")
	}
	var third p15F04MutableOrigin
	if tc.consumer == "count_three" {
		third = p15F04MutableProducerGraph(g, tc, 2, 0, false, "third", &trace)
		if third.accumulator == first.accumulator || third.accumulator == second.accumulator {
			g.err = fmt.Errorf("third authored origin reused an earlier private accumulator")
		}
	}
	trace.mark(g, "initialize_current")
	g.emit("setvariable", map[string]any{"WFVariableName": "current", "WFInput": p15FunctionEdge(first.collection)})
	trace.mark(g, "snapshot_read")
	snapshot := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("current")})
	trace.mark(g, "assign_second")
	g.emit("setvariable", map[string]any{"WFVariableName": "current", "WFInput": p15FunctionEdge(second.collection)})

	count := func(role string, input any) string {
		trace.mark(g, role)
		return g.value("count", map[string]any{"Input": input})
	}
	// Standalone Count still executes, but only consumed results require UUIDs.
	discardCount := func(role string, input any) {
		trace.mark(g, role)
		g.emit("count", map[string]any{"Input": input})
	}
	switch tc.consumer {
	case "count_pair", "count_restore":
		if tc.consumer == "count_restore" {
			discardCount("assigned_version_consumer", p15FunctionVariable("current"))
			trace.mark(g, "assign_snapshot")
			g.emit("setvariable", map[string]any{"WFVariableName": "current", "WFInput": p15FunctionEdge(snapshot)})
		}
		old := count("snapshot_count", p15FunctionEdge(snapshot))
		scaled := g.math("×", old, 10)
		current := count("current_count", p15FunctionVariable("current"))
		result := g.math("+", scaled, p15FunctionEdge(current))
		trace.mark(g, "final_output")
		g.output(result)
	case "count_three":
		trace.mark(g, "middle_snapshot_read")
		middle := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("current")})
		trace.mark(g, "assign_third")
		g.emit("setvariable", map[string]any{"WFVariableName": "current", "WFInput": p15FunctionEdge(third.collection)})
		old := count("snapshot_count", p15FunctionEdge(snapshot))
		oldScaled := g.math("×", old, 100)
		mid := count("middle_count", p15FunctionEdge(middle))
		midScaled := g.math("×", mid, 10)
		joined := g.math("+", oldScaled, p15FunctionEdge(midScaled))
		current := count("current_count", p15FunctionVariable("current"))
		result := g.math("+", joined, p15FunctionEdge(current))
		trace.mark(g, "final_output")
		g.output(result)
	case "index_current", "index_snapshot":
		var input any = p15FunctionVariable("current")
		if tc.consumer == "index_snapshot" {
			discardCount("current_count", p15FunctionVariable("current"))
			input = p15FunctionEdge(snapshot)
		} else {
			discardCount("snapshot_count", p15FunctionEdge(snapshot))
		}
		indexTrace := p15F04LocalTrace{positions: map[string]int{}}
		_, flagName := p15F04LocalIndexInputGraph(g, input, tc.ordinal, true, &indexTrace)
		result := p15F04LocalPresenceBoolean(g, p15FunctionVariable(flagName), &indexTrace)
		for role, position := range indexTrace.positions {
			trace.positions["consumer_"+role] = position
		}
		trace.positions["consumer_count"] = indexTrace.positions["one_based"] + 1
		trace.mark(g, "final_output")
		g.output(result)
	case "for_current", "for_snapshot", "for_shadow":
		var input any = p15FunctionVariable("current")
		if tc.consumer == "for_snapshot" {
			discardCount("current_count", p15FunctionVariable("current"))
			input = p15FunctionEdge(snapshot)
		} else {
			discardCount("snapshot_count", p15FunctionEdge(snapshot))
		}
		trace.mark(g, "consumer_loop")
		consumer := p15DiscoveryLoop(g, "repeat.each", "WFInput", input)
		trace.mark(g, "physical_item_snapshot")
		cell := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("Repeat Item")})
		value, presence := p15F04LocalDecode(g, cell, &trace)
		item := "item"
		if tc.consumer == "for_shadow" {
			item = p15F04PrivateName(g, "__cherri_scope_")
			if !strings.HasSuffix(item, "_current") || item == "current" || item == first.accumulator || item == second.accumulator {
				g.err = fmt.Errorf("shadowing item has no distinct lexical variable")
			}
		}
		trace.mark(g, "semantic_item_store")
		g.emit("setvariable", map[string]any{"WFVariableName": item, "WFInput": p15FunctionEdge(value)})
		if tc.consumer == "for_current" {
			payloadCount := count("decoded_payload_count", p15FunctionVariable(item))
			g.value("getvariable", map[string]any{"WFVariable": p15FunctionEdge(payloadCount)})
		} else {
			p15F04LocalPresenceBoolean(g, p15FunctionEdge(presence), &trace)
		}
		result := p15DiscoveryLoopEnd(g, "repeat.each", consumer, true)
		if tc.consumer == "for_shadow" {
			result = count("restored_current_count", p15FunctionVariable("current"))
		}
		trace.mark(g, "final_output")
		g.output(result)
	default:
		return trace, fmt.Errorf("F04_MUTABLE_ALIAS_GRAPH: unmodeled consumer")
	}
	if err := g.compare(); err != nil {
		return trace, fmt.Errorf("F04_MUTABLE_ALIAS_GRAPH: %w", err)
	}
	return trace, nil
}

func TestP15F04MutableLocalAliasNativeGraphs(t *testing.T) {
	for _, tc := range p15F04MutableAliasCases() {
		t.Run(tc.name, func(t *testing.T) {
			actual := p15CompiledActions(t, tc.source())
			if _, err := p15F04MutableAliasGraphError(actual, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F04MutableLocalAliasGraphRejectsCorruption(t *testing.T) {
	faults := []struct{ name, fixture string }{
		{"assignment_reads_prior_origin", "snapshot_two_to_zero"},
		{"assignment_writes_other_target", "snapshot_two_to_zero"},
		{"assignment_missing", "snapshot_two_to_zero"},
		{"assignment_duplicated", "snapshot_two_to_zero"},
		{"snapshot_reads_new_origin", "snapshot_two_to_zero"},
		{"snapshot_taken_after_assignment", "snapshot_two_to_zero"},
		{"old_consumer_reads_live_current", "snapshot_two_to_zero"},
		{"current_consumer_reads_stale_snapshot", "snapshot_two_to_zero"},
		{"zero_clear_inherits_old_values", "snapshot_two_to_zero"},
		{"distinct_origins_share_accumulator", "snapshot_two_to_zero"},
		{"assigned_sequence_reads_raw_repeat_output", "snapshot_two_to_zero"},
		{"middle_consumer_reads_live_current", "two_assignments_keep_three_versions"},
		{"snapshot_restore_reads_other_origin", "restore_from_immutable_snapshot"},
		{"index_count_reads_stale_snapshot", "index_current_after_shrink_upper_bound"},
		{"snapshot_index_reads_current", "index_snapshot_present_zero"},
		{"presence_decoder_reads_payload", "index_current_present_false"},
		{"for_iterates_stale_snapshot", "for_current_decodes_distinct_payload"},
		{"for_item_exposes_physical_cell", "for_current_decodes_distinct_payload"},
		{"shadow_reuses_current_variable", "for_current_shadow_restores"},
		{"after_loop_reads_inner_binding", "for_current_shadow_restores"},
	}
	cases := map[string]p15F04MutableAliasCase{}
	for _, tc := range p15F04MutableAliasCases() {
		cases[tc.name] = tc
	}
	originals := map[string][]map[string]any{}
	traces := map[string]p15F04LocalTrace{}
	for _, fault := range faults {
		if _, exists := originals[fault.fixture]; exists {
			continue
		}
		tc := cases[fault.fixture]
		original := p15CompiledActions(t, tc.source())
		trace, err := p15F04MutableAliasGraphError(original, tc)
		if err != nil {
			t.Fatal("healthy causal source", fault.fixture, err)
		}
		originals[fault.fixture], traces[fault.fixture] = original, trace
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			tc, trace := cases[fault.fixture], traces[fault.fixture]
			bad := p15C01Clone(t, originals[fault.fixture])
			if _, err := p15F04MutableAliasGraphError(bad, tc); err != nil {
				t.Fatal("unchanged causal clone", err)
			}
			before := p15C01Clone(t, bad)
			position := func(role string) int {
				at, exists := trace.positions[role]
				if !exists || at < 0 || at >= len(bad) {
					t.Fatalf("required authored role %q is absent", role)
				}
				return at
			}
			params := func(role string) map[string]any { return p15Param(bad[position(role)]) }
			edge := func(role string) map[string]any {
				id, okay := params(role)["UUID"].(string)
				if !okay || id == "" {
					t.Fatalf("required authored producer %q has no identity", role)
				}
				return p15FunctionEdge(id)
			}
			switch fault.name {
			case "assignment_reads_prior_origin":
				params("assign_second")["WFInput"] = edge("first_read")
			case "assignment_writes_other_target":
				params("assign_second")["WFVariableName"] = "OTHER_CURRENT"
			case "assignment_missing":
				at := position("assign_second")
				bad = append(bad[:at], bad[at+1:]...)
			case "assignment_duplicated":
				at := position("assign_second")
				copyOfAssignment := p15C01Clone(t, []map[string]any{bad[at]})[0]
				expanded := append([]map[string]any{}, bad[:at+1]...)
				expanded = append(expanded, copyOfAssignment)
				bad = append(expanded, bad[at+1:]...)
			case "snapshot_reads_new_origin":
				params("snapshot_read")["WFVariable"] = edge("second_read")
			case "snapshot_taken_after_assignment":
				a, b := position("snapshot_read"), position("assign_second")
				if b != a+1 {
					t.Fatal("source-point snapshot and assignment are not adjacent")
				}
				bad[a], bad[b] = bad[b], bad[a]
			case "old_consumer_reads_live_current":
				params("snapshot_count")["Input"] = p15FunctionVariable("current")
			case "current_consumer_reads_stale_snapshot":
				params("current_count")["Input"] = edge("snapshot_read")
			case "zero_clear_inherits_old_values":
				params("second_zero_store")["WFInput"] = edge("first_read")
			case "distinct_origins_share_accumulator":
				name, okay := params("first_zero_store")["WFVariableName"].(string)
				if !okay || name == "" {
					t.Fatal("first accumulator identity absent")
				}
				params("second_zero_store")["WFVariableName"] = name
				params("second_append")["WFVariableName"] = name
				params("second_read")["WFVariable"] = p15FunctionVariable(name)
			case "assigned_sequence_reads_raw_repeat_output":
				// Supply a real earlier native producer edge for this causal
				// alternative; rejection must not rely on an unresolved UUID.
				end := params("second_loop_end")
				id, _ := end["UUID"].(string)
				if id == "" {
					id = "00000000-0000-4000-8000-000000000f04"
					for _, action := range bad {
						if p15Param(action)["UUID"] == id {
							t.Fatal("causal alpha identity unexpectedly exists")
						}
					}
					end["UUID"] = id
				}
				params("second_read")["WFVariable"] = p15FunctionEdge(id)
			case "middle_consumer_reads_live_current":
				params("middle_count")["Input"] = p15FunctionVariable("current")
			case "snapshot_restore_reads_other_origin":
				params("assign_snapshot")["WFInput"] = edge("second_read")
			case "index_count_reads_stale_snapshot":
				params("consumer_count")["Input"] = edge("snapshot_read")
			case "snapshot_index_reads_current":
				params("consumer_cell_lookup")["WFInput"] = p15FunctionVariable("current")
			case "presence_decoder_reads_payload":
				params("consumer_presence_read")["WFDictionaryKey"] = "value"
			case "for_iterates_stale_snapshot":
				params("consumer_loop")["WFInput"] = edge("snapshot_read")
			case "for_item_exposes_physical_cell":
				params("semantic_item_store")["WFInput"] = p15FunctionVariable("Repeat Item")
			case "shadow_reuses_current_variable":
				params("semantic_item_store")["WFVariableName"] = "current"
			case "after_loop_reads_inner_binding":
				inner, okay := params("semantic_item_store")["WFVariableName"].(string)
				if !okay || inner == "" {
					t.Fatal("actual lexical item identity absent")
				}
				params("restored_current_count")["Input"] = p15FunctionVariable(inner)
			default:
				t.Fatal("unmodeled mutable-alias mutation")
			}
			if reflect.DeepEqual(before, bad) {
				t.Fatal("causal mutation changed no field")
			}
			if _, err := p15F04MutableAliasGraphError(bad, tc); err == nil || !strings.Contains(err.Error(), "F04_MUTABLE_ALIAS_GRAPH") {
				t.Fatal("assignment/snapshot corruption retained complete graph acceptance", fault.name, err)
			}
		})
	}
}

// Every counterexample starts from a demanded, exact-typed source that must
// compile unchanged. The single source edit then targets semantic equality or
// representation provenance; unrelated parser/name errors cannot satisfy it.
func TestP15F04MutableLocalAliasSemanticTypeBoundaries(t *testing.T) {
	cases := []struct{ name, healthy, from, to string }{
		{
			name: "explicit_anycontent_target",
			healthy: "let owned=repeat 2 {yield [\"F04_TYPED_ANY_CONTROL\"]}\n" +
				"var current:List<List<Text>> = owned\nreturn count(current)",
			from: "var current:List<List<Text>>", to: "var current:AnyContent",
		},
		{
			name: "explicit_covariant_list_target",
			healthy: "let owned=repeat 2 {yield [\"F04_TYPED_COVARIANT_CONTROL\"]}\n" +
				"var current:List<List<Text>> = owned\nreturn count(current)",
			from: "var current:List<List<Text>>", to: "var current:List<List<AnyContent>>",
		},
		{
			name: "assignment_changes_cell_element_type",
			healthy: "let original=repeat 2 {yield [\"F04_TYPED_OLD\"]}\n" +
				"let replacement=repeat 1 {yield [\"F04_TYPED_NEW\"]}\n" +
				"var current:List<List<Text>> = original\nlet snapshot=current\n" +
				"current=replacement\nreturn count(snapshot)+count(current)",
			from: "[\"F04_TYPED_NEW\"]", to: "[17]",
		},
		{
			name: "optional_cells_reassigned_from_plain_scalar_loop",
			healthy: "let original=repeat 2 {if 1==1 {yield false} else {yield none}}\n" +
				"let replacement=repeat 1 {if 1==1 {yield false} else {yield none}}\n" +
				"var current:List<Bool?> = original\nlet snapshot=current\n" +
				"current=replacement\nreturn count(snapshot)+count(current)",
			from: "let replacement=repeat 1 {if 1==1 {yield false} else {yield none}}",
			to:   "let replacement=repeat 1 {yield false}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if raw, err := CompileSourceToPlist("p15-f04-mutable-type-control.cherri", tc.healthy); err != nil || len(raw) == 0 {
				t.Fatal("unchanged demanded exact-typed control did not compile", len(raw), err)
			}
			if strings.Count(tc.healthy, tc.from) != 1 || tc.from == tc.to {
				t.Fatal("causal type/provenance edit is not unique")
			}
			bad := strings.Replace(tc.healthy, tc.from, tc.to, 1)
			diagnostics := p15TypeDiagnostics(t, bad)
			found := false
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity != analysis.SeverityError {
					continue
				}
				if diagnostic.Code == analysis.CodeTargetValueShape {
					found = true
				} else if diagnostic.Code != analysis.CodeArgumentType {
					t.Fatalf("unrelated error masked the causal type/provenance rejection: %+v; all diagnostics: %+v", diagnostic, diagnostics)
				}
			}
			if !found {
				t.Fatalf("exact semantic cell boundary was not diagnosed: %+v", diagnostics)
			}
			raw, err := CompileSourceToPlist("p15-f04-mutable-type-boundary.cherri", bad)
			if err == nil || len(raw) != 0 {
				t.Fatalf("invalid mutable cell interface emitted or missed its source diagnostic: bytes=%d err=%v diagnostics=%+v", len(raw), err, diagnostics)
			}
		})
	}
}
