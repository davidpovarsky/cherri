package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

type p15F04LocalCellCase struct {
	name, producer, value, consumer, ordinal string
	iterations                               int
	optional, dynamic, historical            bool
}

func p15F04LocalCellCases() []p15F04LocalCellCase {
	cases := []p15F04LocalCellCase{
		{name: "alias_zero_none", producer: "repeat", value: "none", consumer: "count", iterations: 0},
		{name: "alias_one_empty_list", producer: "repeat", value: "empty_list", consumer: "count", iterations: 1},
		{name: "alias_three_two_items", producer: "repeat", value: "two_items", consumer: "count", iterations: 3},
		{name: "alias_for_zero", producer: "for_zero", value: "empty_list", consumer: "count"},
		{name: "alias_for_two", producer: "for_two", value: "two_items", consumer: "count", iterations: 2},
		{name: "index_negative", producer: "repeat", value: "two_items", consumer: "index_presence", ordinal: "-1", iterations: 2},
		{name: "index_upper_boundary", producer: "repeat", value: "two_items", consumer: "index_presence", ordinal: "2", iterations: 2},
		{name: "index_fractional", producer: "repeat", value: "two_items", consumer: "index_presence", ordinal: "0.5", iterations: 2},
		{name: "index_dynamic_once", producer: "repeat", value: "two_items", consumer: "index_presence", ordinal: "position", iterations: 2, dynamic: true},
		{name: "index_decoded_payload_count", producer: "repeat", value: "two_items", consumer: "index_count", ordinal: "0", iterations: 2},
		{name: "for_zero_optional", producer: "repeat", value: "none", consumer: "for_presence", iterations: 0},
		{name: "for_decoded_lists", producer: "repeat", value: "two_items", consumer: "for_count", iterations: 2},
		{name: "for_item_optional_alias", producer: "repeat", value: "false", consumer: "for_alias_presence", iterations: 2, optional: true},
		{name: "overwritten_yield_once", producer: "overwritten", value: "empty_list", consumer: "count", iterations: 2},
	}
	for _, value := range []string{"none", "false", "zero", "empty_text", "empty_list", "empty_map", "two_items"} {
		cases = append(cases, p15F04LocalCellCase{
			name: "index_" + value, producer: "repeat", value: value,
			consumer: "index_presence", ordinal: "0", iterations: 2, optional: value != "none",
		})
	}
	for _, value := range []string{"none", "false", "zero", "empty_text", "empty_list", "empty_map"} {
		cases = append(cases, p15F04LocalCellCase{
			name: "for_" + value, producer: "repeat", value: value,
			consumer: "for_presence", iterations: 2, optional: value != "none",
		})
	}
	return cases
}

func (tc p15F04LocalCellCase) source() string {
	body := "yield " + p15F04CellLiteral(tc.value)
	if tc.optional {
		body = "if 1==1 {" + body + "} else {yield none}"
	}
	if tc.producer == "overwritten" {
		body = "yield none\n" + body
	}
	loop := fmt.Sprintf("repeat %d {%s}", tc.iterations, body)
	switch tc.producer {
	case "for_zero":
		loop = "for _ in [] {" + body + "}"
	case "for_two":
		loop = "for _ in [\"a\",\"b\"] {" + body + "}"
	}
	prefix := "let before=\"F04_AUTHORED_BEFORE\"\n"
	if tc.historical {
		prefix = ""
	}
	if tc.dynamic {
		prefix += "let position=1\n"
	}
	prefix += "let values=" + loop + "\n"
	collection := "values"
	if !tc.historical {
		prefix += "let alias=values\n"
		collection = "alias"
	}
	switch tc.consumer {
	case "count":
		return prefix + "return count(" + collection + ")"
	case "index_presence":
		return prefix + "return " + collection + "[" + tc.ordinal + "] != none"
	case "index_value":
		return prefix + "return " + collection + "[" + tc.ordinal + "]"
	case "index_count":
		return prefix + "let selected=" + collection + "[" + tc.ordinal + "]\nif selected != none {return count(selected)}\nreturn -1"
	case "for_presence":
		return prefix + "return for item in " + collection + " {yield item != none}"
	case "for_alias_presence":
		return prefix + "return for item in " + collection + " {let aliasItem=item\nyield aliasItem != none}"
	case "for_count":
		return prefix + "return for item in " + collection + " {yield count(item)}"
	}
	panic("unmodeled local cell consumer")
}

type p15F04LocalTrace struct {
	positions map[string]int
}

func (trace *p15F04LocalTrace) mark(g *p15FunctionGraph, name string) {
	trace.positions[name] = len(g.expected)
}

func p15F04LocalDecode(g *p15FunctionGraph, cell string, trace *p15F04LocalTrace) (string, string) {
	trace.mark(g, "presence_read")
	presence := g.get("has_value", cell)
	trace.mark(g, "presence_guard")
	branch := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
	trace.mark(g, "payload_read")
	g.get("value", cell)
	g.boundary(branch, 1, false)
	g.emit("nothing", nil)
	value := g.boundary(branch, 2, true)
	return value, presence
}

func p15F04LocalPresenceBoolean(g *p15FunctionGraph, flag any, trace *p15F04LocalTrace) string {
	trace.mark(g, "semantic_presence_consumer")
	group := g.conditional(5, flag, "WFConditionalActionString", "0")
	g.emit("number", map[string]any{"WFNumberActionNumber": 1})
	g.boundary(group, 1, false)
	g.emit("number", map[string]any{"WFNumberActionNumber": 0})
	return g.boundary(group, 2, true)
}

func p15F04LocalIndexGraph(g *p15FunctionGraph, collection string, ordinal any, integral bool, trace *p15F04LocalTrace) (string, string) {
	return p15F04LocalIndexInputGraph(g, p15FunctionEdge(collection), ordinal, integral, trace)
}

// The authored input may be an immutable producer edge or a mutable variable.
// Keep the same complete bounds/presence decoder for both transports.
func p15F04LocalIndexInputGraph(g *p15FunctionGraph, collection any, ordinal any, integral bool, trace *p15F04LocalTrace) (string, string) {
	g.emit("nothing", nil)
	valueName := p15F04PrivateName(g, "__cherri_scope_")
	if !strings.Contains(valueName, "_loopIndexValue_") {
		g.err = fmt.Errorf("F04_LOCAL_PRIVATE_ROLE: index value variable")
	}
	g.emit("setvariable", map[string]any{"WFVariableName": valueName})
	zero := g.value("gettext", map[string]any{"WFTextActionText": "0"})
	presenceName := p15F04PrivateName(g, "__cherri_scope_")
	if !strings.Contains(presenceName, "_loopIndexPresence_") || presenceName == valueName {
		g.err = fmt.Errorf("F04_LOCAL_PRIVATE_ROLE: separate index presence variable")
	}
	trace.mark(g, "index_presence_initialize")
	g.emit("setvariable", map[string]any{"WFVariableName": presenceName, "WFInput": p15FunctionEdge(zero)})

	integralGroup := ""
	if !integral {
		trace.mark(g, "integral_remainder")
		remainder := g.value("math", map[string]any{
			"WFMathOperation": "…", "WFScientificMathOperation": "Modulus",
			"WFInput": ordinal, "WFScientificMathOperand": 1,
		})
		trace.mark(g, "integral_guard")
		integralGroup = g.conditional(4, p15FunctionEdge(remainder), "WFNumberValue", 0)
	}
	trace.mark(g, "one_based")
	one := g.value("math", map[string]any{"WFMathOperation": "+", "WFInput": ordinal, "WFMathOperand": 1})
	count := g.value("count", map[string]any{"Input": collection, "WFCountType": "Items"})
	difference := g.math("-", count, p15FunctionEdge(one))
	trace.mark(g, "lower_guard")
	lower := g.conditional(3, p15FunctionEdge(one), "WFNumberValue", 1)
	trace.mark(g, "upper_guard")
	upper := g.conditional(3, p15FunctionEdge(difference), "WFNumberValue", 0)
	trace.mark(g, "cell_lookup")
	cell := g.value("getitemfromlist", map[string]any{
		"WFInput": collection, "WFItemSpecifier": "Item At Index", "WFItemIndex": p15FunctionEdge(one),
	})
	value, presence := p15F04LocalDecode(g, cell, trace)
	trace.mark(g, "index_presence_store")
	g.emit("setvariable", map[string]any{"WFVariableName": presenceName, "WFInput": p15FunctionEdge(presence)})
	trace.mark(g, "index_value_store")
	g.emit("setvariable", map[string]any{"WFVariableName": valueName, "WFInput": p15FunctionEdge(value)})
	g.boundary(upper, 1, false)
	g.emit("nothing", nil)
	g.boundary(upper, 2, true)
	g.boundary(lower, 1, false)
	g.emit("nothing", nil)
	g.boundary(lower, 2, true)
	if integralGroup != "" {
		g.boundary(integralGroup, 1, false)
		g.emit("nothing", nil)
		g.boundary(integralGroup, 2, true)
	}
	trace.mark(g, "index_value_read")
	result := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(valueName)})
	return result, presenceName
}

// Complete source-to-consumer inventory. Only UUID/group/private variable
// alpha names come from the actual producers at their required positions.
func p15F04LocalCellGraphError(actual []map[string]any, tc p15F04LocalCellCase) (p15F04LocalTrace, error) {
	trace := p15F04LocalTrace{positions: map[string]int{}}
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	if !tc.historical {
		g.value("gettext", map[string]any{"WFTextActionText": "F04_AUTHORED_BEFORE"})
	}
	indexSource := ""
	if tc.dynamic {
		indexSource = g.value("number", map[string]any{"WFNumberActionNumber": 1})
	}
	iterable := ""
	if tc.producer == "for_zero" {
		empty := p15F04ClearedCollection(g, "emptyList")
		iterable = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(empty)})
	} else if tc.producer == "for_two" {
		iterable = g.value("list", map[string]any{"WFItems": []any{
			map[string]any{"WFItemType": 0, "WFValue": "a"},
			map[string]any{"WFItemType": 0, "WFValue": "b"},
		}})
	}
	trace.mark(g, "zero_clear")
	accumulator := p15F04ClearedCollection(g, "loopResults")
	trace.positions["zero_store"] = trace.positions["zero_clear"] + 1
	kind, field, input := "repeat.count", "WFRepeatCount", any(tc.iterations)
	if iterable != "" {
		kind, field, input = "repeat.each", "WFInput", p15FunctionEdge(iterable)
	}
	trace.mark(g, "producer_loop")
	loop := p15DiscoveryLoop(g, kind, field, input)
	appendCell := func(present bool) {
		value := p15F04CellValue(tc.value)
		if !present {
			value = p15FunctionArgument{}
		}
		if _, exists := trace.positions["cell_producer"]; !exists {
			trace.mark(g, "cell_producer")
		}
		cell := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", present, value)
		if _, exists := trace.positions["append"]; !exists {
			trace.mark(g, "append")
		}
		g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
	}
	if tc.optional {
		one := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		branch := g.conditional(4, p15FunctionEdge(one), "WFNumberValue", 1)
		appendCell(true)
		g.boundary(branch, 1, false)
		appendCell(false)
		g.boundary(branch, 2, false)
	} else {
		if tc.producer == "overwritten" {
			g.emit("nothing", nil)
		}
		appendCell(tc.value != "none")
	}
	p15DiscoveryLoopEnd(g, kind, loop, false)
	trace.mark(g, "collection_read")
	collection := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	switch tc.consumer {
	case "count":
		trace.mark(g, "count_consumer")
		count := g.value("count", map[string]any{"Input": p15FunctionEdge(collection)})
		trace.mark(g, "final_output")
		g.output(count)
	case "index_presence", "index_count", "index_value":
		var ordinal any
		integral := !tc.dynamic && tc.ordinal != "0.5"
		switch tc.ordinal {
		case "0":
			ordinal = 0
		case "2":
			ordinal = 2
		case "-1":
			ordinal = -1
		case "0.5":
			ordinal = 0.5
		case "position":
			ordinal = p15FunctionEdge(indexSource)
		default:
			return trace, fmt.Errorf("unmodeled index source")
		}
		value, flagName := p15F04LocalIndexGraph(g, collection, ordinal, integral, &trace)
		if tc.consumer == "index_presence" {
			result := p15F04LocalPresenceBoolean(g, p15FunctionVariable(flagName), &trace)
			trace.mark(g, "final_output")
			g.output(result)
		} else if tc.consumer == "index_value" {
			trace.mark(g, "final_output")
			g.output(value)
		} else {
			// Immutable selected binds the decoded value and snapshots its
			// independently mutable index-presence slot before the branch.
			flag := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(flagName)})
			branch := g.conditional(5, p15FunctionEdge(flag), "WFConditionalActionString", "0")
			trace.mark(g, "decoded_payload_count")
			count := g.value("count", map[string]any{"Input": p15FunctionEdge(value)})
			g.output(count)
			g.boundary(branch, 2, false)
			absent := g.value("number", map[string]any{"WFNumberActionNumber": -1})
			trace.mark(g, "final_output")
			g.output(absent)
		}
	case "for_presence", "for_alias_presence", "for_count":
		trace.mark(g, "consumer_loop")
		consumer := p15DiscoveryLoop(g, "repeat.each", "WFInput", p15FunctionEdge(collection))
		trace.mark(g, "physical_item_snapshot")
		cell := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("Repeat Item")})
		value, presence := p15F04LocalDecode(g, cell, &trace)
		trace.mark(g, "semantic_item_store")
		g.emit("setvariable", map[string]any{"WFVariableName": "item", "WFInput": p15FunctionEdge(value)})
		if tc.consumer == "for_count" {
			trace.mark(g, "decoded_payload_count")
			count := g.value("count", map[string]any{"Input": p15FunctionVariable("item")})
			// Yield materializes this action result at the loop boundary.
			// Require the exact Count producer, never the item or cell.
			trace.mark(g, "decoded_payload_yield")
			g.value("getvariable", map[string]any{"WFVariable": p15FunctionEdge(count)})
		} else {
			if tc.consumer == "for_alias_presence" {
				g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("item")})
			}
			p15F04LocalPresenceBoolean(g, p15FunctionEdge(presence), &trace)
		}
		result := p15DiscoveryLoopEnd(g, "repeat.each", consumer, true)
		trace.mark(g, "final_output")
		g.output(result)
	default:
		return trace, fmt.Errorf("unmodeled consumer")
	}
	if err := g.compare(); err != nil {
		return trace, fmt.Errorf("F04_LOCAL_GRAPH: %w", err)
	}
	return trace, nil
}

func TestP15F04LocalCellConsumerNativeGraphs(t *testing.T) {
	for _, tc := range p15F04LocalCellCases() {
		t.Run(tc.name, func(t *testing.T) {
			actual := p15CompiledActions(t, tc.source())
			if _, err := p15F04LocalCellGraphError(actual, tc); err != nil {
				t.Fatalf("%v\nnative actions: %#v", err, actual)
			}
		})
	}
}

// These two exact source fixtures previously belonged to the closed-Count
// unsupported set. Move their assertions only with the new production path;
// original RED/Count evidence remains historical and unmodified.
func TestP15F04LocalCellBoundaryPromotions(t *testing.T) {
	for _, tc := range []p15F04LocalCellCase{
		{name: "aliased_collection", producer: "repeat", value: "empty_list", consumer: "count", iterations: 2, historical: true},
		{name: "index_consumer", producer: "repeat", value: "empty_list", consumer: "index_value", ordinal: "0", iterations: 2, historical: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := p15F04LocalCellGraphError(p15CompiledActions(t, tc.source()), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F04LocalCellGraphRejectsCorruption(t *testing.T) {
	selections := map[string]p15F04LocalCellCase{}
	for _, tc := range p15F04LocalCellCases() {
		if tc.name == "index_false" || tc.name == "for_false" || tc.name == "index_dynamic_once" || tc.name == "alias_one_empty_list" || tc.name == "for_decoded_lists" {
			selections[tc.name] = tc
		}
	}
	faults := []struct{ name, fixture, role string }{
		{"zero_collection_inherits_previous", "index_false", "zero_store"},
		{"append_payload_not_cell", "index_false", "append"},
		{"append_wrong_accumulator", "index_false", "append"},
		{"duplicate_append", "index_false", "append"},
		{"drop_append", "index_false", "append"},
		{"count_export_reads_cells", "alias_one_empty_list", "final_output"},
		{"index_ordinal_shift", "index_false", "one_based"},
		{"index_lower_bound", "index_false", "lower_guard"},
		{"index_upper_bound", "index_false", "upper_guard"},
		{"index_integral_guard", "index_dynamic_once", "integral_guard"},
		{"index_foreign_collection", "index_false", "cell_lookup"},
		{"decode_presence_from_payload", "index_false", "presence_read"},
		{"decode_payload_without_present_guard", "index_false", "presence_guard"},
		{"decode_wrong_payload_key", "index_false", "payload_read"},
		{"presence_default_true", "index_false", "index_presence_initialize"},
		{"presence_store_uses_payload", "index_false", "index_presence_store"},
		{"value_store_uses_cell", "index_false", "index_value_store"},
		{"for_iterates_wrong_sequence", "for_false", "consumer_loop"},
		{"for_store_exposes_physical_cell", "for_false", "semantic_item_store"},
		{"for_presence_uses_native_truthiness", "for_false", "semantic_presence_consumer"},
		{"for_decoder_reads_other_item", "for_false", "physical_item_snapshot"},
		{"for_count_yield_reads_item_instead_of_count", "for_decoded_lists", "decoded_payload_yield"},
		{"extra_unmodeled_action", "index_false", "final_output"},
	}
	originals := map[string][]map[string]any{}
	traces := map[string]p15F04LocalTrace{}
	for name, tc := range selections {
		original := p15CompiledActions(t, tc.source())
		trace, err := p15F04LocalCellGraphError(original, tc)
		if err != nil {
			t.Fatal("healthy causal source", name, err)
		}
		originals[name], traces[name] = original, trace
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			tc, trace := selections[fault.fixture], traces[fault.fixture]
			bad := p15C01Clone(t, originals[fault.fixture])
			if _, err := p15F04LocalCellGraphError(bad, tc); err != nil {
				t.Fatal("unchanged clone rejected before mutation", err)
			}
			before := p15C01Clone(t, bad)
			at, exists := trace.positions[fault.role]
			if !exists || at >= len(bad) {
				t.Fatal("exact authored causal role absent", fault)
			}
			parameter := p15Param(bad[at])
			switch fault.name {
			case "zero_collection_inherits_previous":
				parameter["WFInput"] = "AUTHORED_PREVIOUS"
			case "append_payload_not_cell":
				parameter["WFInput"] = p15FunctionVariable("UNBOXED_PAYLOAD")
			case "append_wrong_accumulator":
				parameter["WFVariableName"] = "UNRELATED_COLLECTION"
			case "duplicate_append":
				copyOfAppend := p15C01Clone(t, []map[string]any{bad[at]})[0]
				expanded := append([]map[string]any{}, bad[:at+1]...)
				expanded = append(expanded, copyOfAppend)
				bad = append(expanded, bad[at+1:]...)
			case "drop_append":
				bad = append(bad[:at], bad[at+1:]...)
			case "count_export_reads_cells":
				id, _ := p15Param(bad[trace.positions["collection_read"]])["UUID"].(string)
				parameter["WFOutput"] = p15FunctionText("\ufffc", id)
			case "index_ordinal_shift":
				parameter["WFMathOperand"] = 2
			case "index_lower_bound", "index_upper_bound":
				parameter["WFCondition"] = 2
			case "index_integral_guard":
				parameter["WFNumberValue"] = 1
			case "index_foreign_collection":
				parameter["WFInput"] = p15FunctionVariable("OTHER_COLLECTION")
			case "decode_presence_from_payload":
				parameter["WFDictionaryKey"] = "value"
			case "decode_payload_without_present_guard":
				parameter["WFConditionalActionString"] = "0"
			case "decode_wrong_payload_key":
				parameter["WFDictionaryKey"] = "has_value"
			case "presence_default_true":
				parameter["WFInput"] = "1"
			case "presence_store_uses_payload":
				id, _ := p15Param(bad[trace.positions["payload_read"]])["UUID"].(string)
				parameter["WFInput"] = p15FunctionEdge(id)
			case "value_store_uses_cell":
				id, _ := p15Param(bad[trace.positions["cell_lookup"]])["UUID"].(string)
				parameter["WFInput"] = p15FunctionEdge(id)
			case "for_iterates_wrong_sequence":
				parameter["WFInput"] = p15FunctionVariable("OTHER_COLLECTION")
			case "for_store_exposes_physical_cell":
				parameter["WFInput"] = p15FunctionVariable("Repeat Item")
			case "for_presence_uses_native_truthiness":
				parameter["WFCondition"] = 100
				delete(parameter, "WFConditionalActionString")
			case "for_decoder_reads_other_item":
				parameter["WFVariable"] = p15FunctionVariable("Repeat Item 2")
			case "for_count_yield_reads_item_instead_of_count":
				parameter["WFVariable"] = p15FunctionVariable("item")
			case "extra_unmodeled_action":
				bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			default:
				t.Fatal("unmodeled causal mutation")
			}
			if reflect.DeepEqual(before, bad) {
				t.Fatal("causal mutation changed no field")
			}
			if _, err := p15F04LocalCellGraphError(bad, tc); err == nil || !strings.Contains(err.Error(), "F04_LOCAL_GRAPH") {
				t.Fatal("corruption did not reach the complete graph rejection", fault, err)
			}
		})
	}
}

func TestP15F04LocalCellUnsupportedInterfaces(t *testing.T) {
	cases := []struct{ name, source string }{
		{"private_sequence_export", "let values=repeat 2 {yield []}\nreturn values"},
		{"mutable_sequence", "var values=repeat 2 {yield []}\nreturn count(values)"},
		{"mutable_alias", "let values=repeat 2 {yield []}\nvar alias=values\nreturn count(alias)"},
		{"widened_alias", "let values=repeat 2 {yield []}\nlet alias:AnyContent=values\nreturn count(alias)"},
		{"function_export", "function consume(value:AnyContent)->Number{return 1}\nlet values=repeat 2 {yield []}\nreturn consume(values)"},
		{"raw_native_export", "let values=repeat 2 {yield []}\nnative.action(\"com.example.export\", {\"WFInput\":values})"},
		{"omitted_previous_action", "let values=repeat 2 {yield []}\nnative.action(\"is.workflow.actions.detect.text\", {})"},
		{"unselected_action_after_cells", "let values=repeat 2 {yield []}\nwait(0)\nreturn count(values)"},
		{"named_count_mode", "let values=repeat 2 {yield []}\nreturn count(values,type:\"Items\")"},
		{"alternate_count_mode", "let values=repeat 2 {yield []}\nreturn count(values,type:\"Characters\")"},
		{"private_sequence_in_map", "let values=repeat 2 {yield []}\nreturn {\"cells\":values}"},
		{"private_sequence_in_list", "let values=repeat 2 {yield []}\nreturn [values]"},
		{"dynamic_list_payload", "let payload=[\"a\",\"b\"]\nlet values=repeat 2 {yield payload}\nreturn count(values)"},
		{"nested_list_payload", "let values=repeat 2 {yield [[\"inner\"],[]]}\nreturn count(values)"},
		{"nested_cell_payload", "let values=repeat 2 {yield repeat 2 {yield []}}\nreturn count(values)"},
		{"raw_repeat_item_alias", "let values=repeat 2 {yield none}\nreturn for item in values {yield RepeatItem}"},
		{"source_count_shadow", "function count(value:AnyContent)->Number{return 0}\nlet values=repeat 2 {yield []}\nreturn count(values)"},
		{"local_count_shadow", "let count=0\nlet values=repeat 2 {yield []}\nreturn count(values)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found := false
			for _, diagnostic := range p15TypeDiagnostics(t, tc.source) {
				if diagnostic.Severity != analysis.SeverityError {
					continue
				}
				if diagnostic.Code != analysis.CodeTargetValueShape && diagnostic.Code != analysis.CodeNestedNativeList {
					t.Fatalf("unrelated interface rejection: %+v", diagnostic)
				}
				found = true
			}
			if !found {
				t.Fatal("unsupported cell interface was admitted")
			}
			raw, err := CompileSourceToPlist("p15-f04-local-interface.cherri", tc.source)
			if err == nil || len(raw) != 0 {
				t.Fatal("unsupported interface emitted native output")
			}
		})
	}
	t.Run("marker_shaped_data_is_ordinary", func(t *testing.T) {
		source := "let ordinary=[{\"_cherri_loop_result_cell_v1\":\"1\",\"has_value\":\"0\",\"value\":[]}]\nreturn count(ordinary)"
		actions := p15CompiledActions(t, source)
		for _, action := range actions {
			p := p15Param(action)
			if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.appendvariable" ||
				action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && (p["WFDictionaryKey"] == "has_value" || p["WFDictionaryKey"] == "value") {
				t.Fatal("ordinary marker-shaped data acquired compiler cell provenance")
			}
		}
	})
}
