package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

type p15F04CellCountCase struct {
	name, route, value string
	count              int
}

func p15F04CellCountCases() []p15F04CellCountCase {
	return []p15F04CellCountCase{
		{"repeat_zero_none", "repeat", "none", 0},
		{"repeat_two_none", "repeat", "none", 2},
		{"repeat_zero_empty_list", "repeat", "empty_list", 0},
		{"repeat_three_two_items", "repeat", "two_items", 3},
		{"repeat_nested_list_payload", "repeat", "nested_list", 2},
		{"for_zero_none", "for_zero", "none", 0},
		{"for_two_empty_list", "for_two", "empty_list", 2},
		{"if_false_vs_none", "if", "false", 2},
		{"if_zero_vs_none", "if", "zero", 2},
		{"if_empty_text_vs_none", "if", "empty_text", 2},
		{"if_empty_list_vs_none", "if", "empty_list", 2},
		{"if_empty_map_vs_none", "if", "empty_map", 2},
		{"menu_list_vs_none", "menu", "empty_list", 2},
		{"overwritten_yield_not_collected", "overwritten", "empty_list", 2},
		{"return_path_not_collected", "return", "empty_list", 2},
		{"bound_list_payload", "bound", "two_items", 2},
		{"mutable_optional_unused_count", "mutable", "none", 2},
	}
}

func p15F04CellLiteral(value string) string {
	switch value {
	case "none":
		return "none"
	case "false":
		return "false"
	case "zero":
		return "0"
	case "empty_text":
		return "\"\""
	case "empty_list":
		return "[]"
	case "empty_map":
		return "{}"
	case "two_items":
		return "[\"first\",\"second\"]"
	case "nested_list":
		return "[[\"inner\"],[]]"
	default:
		panic("unmodeled authored loop payload: " + value)
	}
}

func (tc p15F04CellCountCase) source() string {
	body := "yield " + p15F04CellLiteral(tc.value)
	prefix := ""
	switch tc.route {
	case "if":
		body = "if 1 == 1 {" + body + "} else {yield none}"
	case "menu":
		body = "menu(\"Pick\") {case \"A\" {" + body + "} case \"B\" {yield none}}"
	case "overwritten":
		body = "yield none\n" + body
	case "return":
		body = "if 1 == 1 {return 7} else {" + body + "}"
	case "bound":
		prefix = "let payload=[\"first\",\"second\"]\n"
		body = "yield payload"
	case "mutable":
		prefix = "var candidate:Text?=none\n"
		body = "yield candidate"
	}
	loop := fmt.Sprintf("repeat %d {%s}", tc.count, body)
	if tc.route == "for_zero" {
		loop = "for _ in [] {" + body + "}"
	} else if tc.route == "for_two" {
		loop = "for _ in [\"a\",\"b\"] {" + body + "}"
	}
	if tc.route == "mutable" {
		// No return/None comparison consumes candidate's presence. The closed
		// Count input itself must retain it even though Count is unused.
		return prefix + "count(" + loop + ")\nreturn 7"
	}
	return prefix + "return count(" + loop + ")"
}

func p15F04ArrayValue(values ...p15FunctionArgument) p15FunctionArgument {
	rows := []any{}
	for _, value := range values {
		rows = append(rows, map[string]any{"WFItemType": value.kind, "WFValue": value.value})
	}
	return p15FunctionArgument{true, 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": rows}}
}

func p15F04CellValue(value string) p15FunctionArgument {
	switch value {
	case "none":
		return p15FunctionArgument{}
	case "two_items":
		return p15F04ArrayValue(p15FunctionString("first"), p15FunctionString("second"))
	case "nested_list":
		return p15F04ArrayValue(p15F04ArrayValue(p15FunctionString("inner")), p15F04ArrayValue())
	default:
		return p15OptionalJoinValue(value)
	}
}

func p15F04PrivateName(g *p15FunctionGraph, prefix string) string {
	if len(g.expected) >= len(g.actual) {
		g.err = fmt.Errorf("F04_CELL_PRIVATE_NAME: missing private variable")
		return ""
	}
	name, okay := p15C01Params(g.actual[len(g.expected)])["WFVariableName"].(string)
	if !okay || !strings.HasPrefix(name, prefix) {
		g.err = fmt.Errorf("F04_CELL_PRIVATE_NAME: missing compiler-owned variable")
	}
	return name
}

func p15F04ClearedCollection(g *p15FunctionGraph, purpose string) string {
	g.emit("nothing", nil)
	name := p15F04PrivateName(g, "__cherri_scope_")
	if !strings.Contains(name, "_"+purpose+"_") {
		g.err = fmt.Errorf("F04_CELL_PRIVATE_NAME: wrong collection role")
	}
	g.emit("setvariable", map[string]any{"WFVariableName": name})
	return name
}

func p15F04AppendCell(g *p15FunctionGraph, accumulator string, present bool, value p15FunctionArgument) {
	cell := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", present, value)
	g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
}

// Every action, payload, branch, scope, collection edge, and final consumer is
// independently authored. Only actual UUID/group/private variable alpha names
// are read at the exact required producer positions. No Apple runtime is run.
func p15F04CellCountGraphError(actual []map[string]any, tc p15F04CellCountCase) error {
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	payload, presenceName := "", ""
	if tc.route == "bound" {
		payload = g.value("list", map[string]any{"WFItems": []any{
			map[string]any{"WFItemType": 0, "WFValue": "first"},
			map[string]any{"WFItemType": 0, "WFValue": "second"},
		}})
	}
	if tc.route == "mutable" {
		flag := g.value("gettext", map[string]any{"WFTextActionText": "0"})
		presenceName = p15F04PrivateName(g, "_cherri_optional_presence_")
		g.emit("setvariable", map[string]any{"WFVariableName": presenceName, "WFInput": p15FunctionEdge(flag)})
		g.emit("nothing", nil)
		g.emit("setvariable", map[string]any{"WFVariableName": "candidate"})
	}
	iterable := ""
	if tc.route == "for_zero" {
		empty := p15F04ClearedCollection(g, "emptyList")
		iterable = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(empty)})
	} else if tc.route == "for_two" {
		iterable = g.value("list", map[string]any{"WFItems": []any{
			map[string]any{"WFItemType": 0, "WFValue": "a"},
			map[string]any{"WFItemType": 0, "WFValue": "b"},
		}})
	}
	accumulator := p15F04ClearedCollection(g, "loopResults")
	if accumulator == presenceName || accumulator == "candidate" || accumulator == "payload" {
		return fmt.Errorf("F04_CELL_PRIVATE_NAME: collection aliases authored data")
	}
	kind, field, input := "repeat.count", "WFRepeatCount", any(tc.count)
	if iterable != "" {
		kind, field, input = "repeat.each", "WFInput", p15FunctionEdge(iterable)
	}
	loop := p15DiscoveryLoop(g, kind, field, input)
	value := p15F04CellValue(tc.value)
	switch tc.route {
	case "if", "return":
		one := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		branch := g.conditional(4, p15FunctionEdge(one), "WFNumberValue", 1)
		if tc.route == "return" {
			seven := g.value("number", map[string]any{"WFNumberActionNumber": 7})
			g.output(seven)
		} else {
			p15F04AppendCell(g, accumulator, true, value)
		}
		g.boundary(branch, 1, false)
		if tc.route == "return" {
			p15F04AppendCell(g, accumulator, true, value)
		} else {
			p15F04AppendCell(g, accumulator, false, p15FunctionArgument{})
		}
		g.boundary(branch, 2, false)
	case "menu":
		group := p15OptionalJoinMenuStart(g)
		p15OptionalJoinMenuCase(g, group, "A")
		p15F04AppendCell(g, accumulator, true, value)
		p15OptionalJoinMenuCase(g, group, "B")
		p15F04AppendCell(g, accumulator, false, p15FunctionArgument{})
		p15OptionalJoinMenuEnd(g, group, false)
	case "mutable":
		cell := p15OptionalJoinDynamicCarrier(g, "_cherri_loop_result_cell_v1",
			p15FunctionArgument{true, 0, p15FunctionVariable("candidate")}, p15FunctionVariable(presenceName))
		g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
	case "bound":
		p15F04AppendCell(g, accumulator, true, p15FunctionArgument{true, 0, p15FunctionEdge(payload)})
	default:
		if tc.route == "overwritten" {
			g.emit("nothing", nil)
		}
		p15F04AppendCell(g, accumulator, tc.value != "none", value)
	}
	p15DiscoveryLoopEnd(g, kind, loop, false)
	collected := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	// Canonical Count omits its default Items parameter; require that exact wire.
	countParams := map[string]any{"Input": p15FunctionEdge(collected)}
	if tc.route == "mutable" {
		g.emit("count", countParams)
		seven := g.value("number", map[string]any{"WFNumberActionNumber": 7})
		g.output(seven)
	} else {
		count := g.value("count", countParams)
		g.output(count)
	}
	if err := g.compare(); err != nil {
		return fmt.Errorf("F04_CELL_GRAPH: %w", err)
	}
	return nil
}

func TestP15F04ClosedItemsCountNativeGraphs(t *testing.T) {
	for _, tc := range p15F04CellCountCases() {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15F04CellCountGraphError(p15CompiledActions(t, tc.source()), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F04ClosedItemsCountGraphCorruptions(t *testing.T) {
	cases := p15F04CellCountCases()
	selections := map[string]p15F04CellCountCase{}
	originals := map[string][]map[string]any{}
	for _, tc := range cases {
		if tc.name == "repeat_three_two_items" || tc.name == "if_false_vs_none" || tc.name == "mutable_optional_unused_count" {
			selections[tc.name] = tc
			originals[tc.name] = p15CompiledActions(t, tc.source())
			if err := p15F04CellCountGraphError(originals[tc.name], tc); err != nil {
				t.Fatal("original complete graph rejected:", err)
			}
		}
	}
	for _, fault := range []struct{ name, fixture string }{
		{"missing_zero_clear", "repeat_three_two_items"},
		{"zero_seeded_with_unrelated", "repeat_three_two_items"},
		{"payload_instead_of_cell", "repeat_three_two_items"},
		{"missing_append", "repeat_three_two_items"},
		{"duplicate_append", "repeat_three_two_items"},
		{"append_to_other_variable", "repeat_three_two_items"},
		{"wrong_iteration_count", "repeat_three_two_items"},
		{"reordered_payload", "repeat_three_two_items"},
		{"flattened_payload", "repeat_three_two_items"},
		{"wrong_count_mode", "repeat_three_two_items"},
		{"count_uses_raw_repeat", "repeat_three_two_items"},
		{"final_consumer_bypass", "repeat_three_two_items"},
		{"present_false_marked_absent", "if_false_vs_none"},
		{"none_marked_present", "if_false_vs_none"},
		{"false_payload_changed", "if_false_vs_none"},
		{"presence_uses_payload", "mutable_optional_unused_count"},
		{"unrelated_extra_action", "repeat_three_two_items"},
	} {
		t.Run(fault.name, func(t *testing.T) {
			tc := selections[fault.fixture]
			bad := p15ContainerCloneNative(t, originals[fault.fixture])
			if err := p15F04CellCountGraphError(bad, tc); err != nil {
				t.Fatal("copy rejected:", err)
			}
			appendAt, countAt, startAt, endAt, outputAt := -1, -1, -1, -1, -1
			cells := []int{}
			for i, action := range bad {
				id, _ := action["WFWorkflowActionIdentifier"].(string)
				p := p15C01Params(action)
				switch id {
				case "is.workflow.actions.appendvariable":
					if appendAt < 0 {
						appendAt = i
					}
				case "is.workflow.actions.count":
					countAt = i
				case "is.workflow.actions.output":
					outputAt = i
				case "is.workflow.actions.repeat.count":
					if p15Int(p["WFControlFlowMode"]) == 0 {
						startAt = i
					} else if p15Int(p["WFControlFlowMode"]) == 2 {
						endAt = i
					}
				case "is.workflow.actions.dictionary":
					if fields, err := p15ContainerFields(p["WFItems"]); err == nil && fields["_cherri_loop_result_cell_v1"] != nil {
						cells = append(cells, i)
					}
				}
			}
			if appendAt < 0 || countAt < 0 || startAt < 0 || endAt < 0 || outputAt < 0 || len(cells) == 0 {
				t.Fatal("missing causal mutation target")
			}
			switch fault.name {
			case "missing_zero_clear":
				bad = bad[1:]
			case "zero_seeded_with_unrelated":
				p15C01Params(bad[1])["WFInput"] = p15FunctionVariable("unrelated")
			case "payload_instead_of_cell":
				p15C01Params(bad[appendAt])["WFInput"] = p15FunctionVariable("payload")
			case "missing_append":
				bad = append(bad[:appendAt], bad[appendAt+1:]...)
			case "duplicate_append":
				copied := p15ContainerCloneNative(t, []map[string]any{bad[appendAt]})[0]
				bad = append(bad[:appendAt+1], append([]map[string]any{copied}, bad[appendAt+1:]...)...)
			case "append_to_other_variable":
				p15C01Params(bad[appendAt])["WFVariableName"] = "another_collection"
			case "wrong_iteration_count":
				p15C01Params(bad[startAt])["WFRepeatCount"] = 99
			case "reordered_payload", "flattened_payload":
				fields, err := p15ContainerFields(p15C01Params(bad[cells[0]])["WFItems"])
				if err != nil {
					t.Fatal(err)
				}
				if fault.name == "reordered_payload" {
					fields["value"]["WFValue"] = p15F04ArrayValue(p15FunctionString("second"), p15FunctionString("first")).value
				} else {
					fields["value"]["WFItemType"] = 0
					fields["value"]["WFValue"] = p15FunctionText("firstsecond")
				}
			case "wrong_count_mode":
				p15C01Params(bad[countAt])["WFCountType"] = "Characters"
			case "count_uses_raw_repeat":
				p15C01Params(bad[endAt])["UUID"] = "CAUSAL-RAW-REPEAT"
				p15C01Params(bad[countAt])["Input"] = p15FunctionEdge("CAUSAL-RAW-REPEAT")
			case "final_consumer_bypass":
				p15C01Params(bad[outputAt])["WFOutput"] = p15FunctionText("\ufffc", fmt.Sprint(p15C01Params(bad[cells[0]])["UUID"]))
			case "present_false_marked_absent", "false_payload_changed":
				fields, err := p15ContainerFields(p15C01Params(bad[cells[0]])["WFItems"])
				if err != nil {
					t.Fatal(err)
				}
				if fault.name == "present_false_marked_absent" {
					fields["has_value"]["WFValue"] = p15FunctionText("0")
				} else {
					fields["value"]["WFValue"] = map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": true}
				}
			case "none_marked_present":
				if len(cells) != 2 {
					t.Fatal("expected two original presence alternatives")
				}
				fields, err := p15ContainerFields(p15C01Params(bad[cells[1]])["WFItems"])
				if err != nil {
					t.Fatal(err)
				}
				fields["has_value"]["WFValue"] = p15FunctionText("1")
			case "presence_uses_payload":
				changed := false
				for _, action := range bad {
					p := p15C01Params(action)
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0 {
						p["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15FunctionVariable("candidate")}
						changed = true
						break
					}
				}
				if !changed {
					t.Fatal("presence predicate absent")
				}
			case "unrelated_extra_action":
				bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			default:
				t.Fatal("unmodeled fault")
			}
			if err := p15F04CellCountGraphError(bad, tc); err == nil {
				t.Fatal("causal corruption accepted")
			}
		})
	}
}

func TestP15F04ClosedItemsCountPreservesUnsupportedBoundaries(t *testing.T) {
	cases := []struct{ name, source string }{
		{"ordinary_export", "return repeat 2 {yield []}"},
		{"function_transport", "function use(values:AnyContent)->Number{return 0}\nreturn use(repeat 2 {yield none})"},
		{"character_count", "return count(repeat 2 {yield []},type:\"Characters\")"},
		{"named_items_not_enrolled", "return count(repeat 2 {yield []},type:\"Items\")"},
		{"source_function_shadow", "function count(value:AnyContent)->Number{return 0}\nreturn count(repeat 2 {yield []})"},
		{"action_in_payload", "function value()->List<Text>{return []}\nreturn count(repeat 2 {yield value()})"},
		{"earlier_action_in_body", "function value()->Number{return 1}\nreturn count(repeat 2 {value()\nyield []})"},
		{"nested_loop_result", "return count(repeat 2 {yield repeat 2 {yield []}})"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			found := false
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					if d.Code != analysis.CodeTargetValueShape {
						t.Fatalf("unrelated rejection: %+v", d)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("unsupported transport lost its target-shape rejection")
			}
			data, err := CompileSourceToPlist("p15-f04-closed-count-boundary.cherri", tc.source)
			if err == nil || len(data) != 0 {
				t.Fatal("unsupported consumer produced native output")
			}
		})
	}
}

// This is the compiler-owned Dictionary-field context, not permission to
// encode an arbitrary native List. The complete positive graphs reuse the
// existing cell oracle; precise negative diagnostics keep the scope narrow.
func TestP15F04CellPayloadContextBoundaries(t *testing.T) {
	for _, tc := range []p15F04CellCountCase{
		{"final_if_nested_list", "if", "nested_list", 2},
		{"final_menu_nested_list", "menu", "nested_list", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15F04CellCountGraphError(p15CompiledActions(t, tc.source()), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct{ name, source, code string }{
		{"overwritten_nested_list", "return count(repeat 2 {yield [[\"discarded\"]]\nyield []})", analysis.CodeNestedNativeList},
		{"ordinary_binding_in_body", "return count(repeat 2 {let payload=[[1]]\nyield payload})", analysis.CodeNestedNativeList},
		{"ordinary_native_list", "return [[1],[]]", analysis.CodeNestedNativeList},
		{"nonfinal_branch", "return count(repeat 2 {if 1==1 {yield [[1]]} else {yield []}\nyield []})", analysis.CodeNestedNativeList},
		{"scope_restored_after_count", "count(repeat 2 {yield [[1]]})\nreturn [[2]]", analysis.CodeNestedNativeList},
		{"absence_inside_array", "return count(repeat 2 {yield [[none]]})", analysis.CodeTargetValueShape},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found := false
			for _, diagnostic := range p15TypeDiagnostics(t, tc.source) {
				if diagnostic.Severity != analysis.SeverityError {
					continue
				}
				if diagnostic.Code != tc.code {
					t.Fatalf("unrelated rejection: %+v", diagnostic)
				}
				found = true
			}
			if !found {
				t.Fatalf("missing preserved boundary %s", tc.code)
			}
			data, err := CompileSourceToPlist("p15-f04-cell-context-boundary.cherri", tc.source)
			if err == nil || len(data) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("boundary produced output or another failure: bytes=%d error=%v", len(data), err)
			}
		})
	}
}
