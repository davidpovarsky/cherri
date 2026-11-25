package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

type p15TaggedCountContextCase struct {
	name, source string
}

// These are the exact original four internal Count specimens. A private value
// may be constructed here without granting a native export of that value.
func p15TaggedCountContextCases() []p15TaggedCountContextCase {
	return []p15TaggedCountContextCase{
		{"overwritten_nested_list", "return count(repeat 2 {yield [[\"discarded\"]]\nyield []})"},
		{"ordinary_binding_in_body", "return count(repeat 2 {let payload=[[1]]\nyield payload})"},
		{"nonfinal_branch", "return count(repeat 2 {if 1==1 {yield [[1]]} else {yield []}\nyield []})"},
		{"absence_inside_array", "return count(repeat 2 {yield [[none]]})"},
	}
}

func p15TaggedAbsentCellGraph(g *p15FunctionGraph) string {
	rows := []any{
		map[string]any{"WFKey": p15FunctionText("_cherri_value_cell_v1"), "WFItemType": 0, "WFValue": p15FunctionText("1")},
		map[string]any{"WFKey": p15FunctionText("has_value"), "WFItemType": 0, "WFValue": p15FunctionText("0")},
		map[string]any{"WFKey": p15FunctionText("kind"), "WFItemType": 0, "WFValue": p15FunctionText("none")},
	}
	return g.value("dictionary", map[string]any{"WFItems": map[string]any{
		"WFSerializationType": "WFDictionaryFieldValue",
		"Value":               map[string]any{"WFDictionaryFieldValueItems": rows},
	}})
}

func p15TaggedSingleCellList(g *p15FunctionGraph, cell string) string {
	return g.value("list", map[string]any{"WFItems": []any{
		map[string]any{"WFItemType": 0, "WFValue": p15FunctionEdge(cell)},
	}})
}

// The entire expected action graph is independently composed from the source:
// preserve overwritten effects, append only one normal final result, retain the
// nested none cell, and count the explicitly cleared collection. Only required
// UUID/group/private variable alpha names are taken from their exact positions.
func p15TaggedCountContextGraph(actual []map[string]any, name string) (map[string]int, error) {
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	positions := map[string]int{"clear": len(g.expected)}
	accumulator := p15F04ClearedCollection(g, "loopResults")
	positions["loop"] = len(g.expected)
	loop := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", 2)
	payload := p15F04ArrayValue()
	switch name {
	case "overwritten_nested_list":
		positions["nested_cell"] = len(g.expected)
		cell := p15TaggedCellGraph(g, "list", p15F04ArrayValue(p15FunctionString("discarded")))
		p15TaggedSingleCellList(g, cell)
	case "ordinary_binding_in_body":
		positions["nested_cell"] = len(g.expected)
		cell := p15TaggedCellGraph(g, "list", p15F04ArrayValue(p15FunctionNumber("1")))
		list := p15TaggedSingleCellList(g, cell)
		payload = p15FunctionArgument{true, 0, p15FunctionEdge(list)}
	case "nonfinal_branch":
		one := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		branch := g.conditional(4, p15FunctionEdge(one), "WFNumberValue", 1)
		positions["nested_cell"] = len(g.expected)
		cell := p15TaggedCellGraph(g, "list", p15F04ArrayValue(p15FunctionNumber("1")))
		p15TaggedSingleCellList(g, cell)
		g.boundary(branch, 1, false)
		empty := p15F04ClearedCollection(g, "emptyList")
		g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(empty)})
		g.boundary(branch, 2, false)
	case "absence_inside_array":
		positions["nested_cell"] = len(g.expected)
		absent := p15TaggedAbsentCellGraph(g)
		outer := p15TaggedCellGraph(g, "list", p15F04ArrayValue(
			p15FunctionArgument{true, 0, p15FunctionEdge(absent)}))
		payload = p15F04ArrayValue(p15FunctionArgument{true, 0, p15FunctionEdge(outer)})
	default:
		return nil, fmt.Errorf("unregistered exact Count specimen %q", name)
	}
	positions["carrier"] = len(g.expected)
	carrier := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", true, payload)
	positions["append"] = len(g.expected)
	g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(carrier)})
	positions["end"] = len(g.expected)
	p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	collection := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	positions["count"] = len(g.expected)
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(collection)})
	positions["output"] = len(g.expected)
	g.output(count)
	return positions, g.compare()
}

func p15CheckTaggedCountContext(t *testing.T, tc p15TaggedCountContextCase) []map[string]any {
	t.Helper()
	for _, diagnostic := range p15TypeDiagnostics(t, tc.source) {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
	actual := p15CompiledActions(t, tc.source)
	if _, err := p15TaggedCountContextGraph(actual, tc.name); err != nil {
		t.Fatal(err)
	}
	return actual
}

func TestP15TaggedCountInternalContextGraphs(t *testing.T) {
	for _, tc := range p15TaggedCountContextCases() {
		t.Run(tc.name, func(t *testing.T) { p15CheckTaggedCountContext(t, tc) })
	}
}

func TestP15TaggedCountInternalContextGraphFaults(t *testing.T) {
	for _, tc := range p15TaggedCountContextCases() {
		t.Run(tc.name, func(t *testing.T) {
			actual := p15CheckTaggedCountContext(t, tc)
			pos, err := p15TaggedCountContextGraph(actual, tc.name)
			if err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{
				"uncleared_previous", "wrong_iterations", "nested_presence", "nested_kind",
				"drop_nested_cell", "wrong_collected_payload", "append_payload", "duplicate_append",
				"wrong_loop_end", "wrong_count_mode", "count_nested_payload", "return_private_payload",
				"extra_action",
			} {
				t.Run(fault, func(t *testing.T) {
					bad := p15TaggedClone(t, actual)
					nestedID, _ := p15Param(bad[pos["nested_cell"]])["UUID"].(string)
					switch fault {
					case "uncleared_previous":
						p15Param(bad[pos["clear"]+1])["WFInput"] = p15FunctionEdge("UNRELATED_PREVIOUS")
					case "wrong_iterations":
						p15Param(bad[pos["loop"]])["WFRepeatCount"] = 3
					case "nested_presence", "nested_kind":
						fields, err := p15ContainerFields(p15Param(bad[pos["nested_cell"]])["WFItems"])
						if err != nil {
							t.Fatal(err)
						}
						key, value := "kind", "unrelated"
						if fault == "nested_presence" {
							key, value = "has_value", "0"
							if tc.name == "absence_inside_array" {
								value = "1"
							}
						}
						fields[key]["WFValue"] = p15FunctionText(value)
					case "drop_nested_cell":
						at := pos["nested_cell"]
						bad = append(bad[:at], bad[at+1:]...)
					case "wrong_collected_payload":
						fields, err := p15ContainerFields(p15Param(bad[pos["carrier"]])["WFItems"])
						if err != nil {
							t.Fatal(err)
						}
						fields["value"]["WFValue"] = p15FunctionEdge("UNRELATED_PAYLOAD")
					case "append_payload":
						p15Param(bad[pos["append"]])["WFInput"] = p15FunctionEdge(nestedID)
					case "duplicate_append":
						at := pos["append"]
						bad = append(bad[:at], append([]map[string]any{bad[at]}, bad[at:]...)...)
					case "wrong_loop_end":
						p15Param(bad[pos["end"]])["GroupingIdentifier"] = "UNRELATED_LOOP"
					case "wrong_count_mode":
						p15Param(bad[pos["count"]])["WFCountType"] = "Characters"
					case "count_nested_payload":
						p15Param(bad[pos["count"]])["Input"] = p15FunctionEdge(nestedID)
					case "return_private_payload":
						p15Param(bad[pos["output"]])["WFOutput"] = p15FunctionEdge(nestedID)
					case "extra_action":
						bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					}
					if reflect.DeepEqual(actual, bad) {
						t.Fatal("mutation did not alter graph")
					}
					if _, err := p15TaggedCountContextGraph(bad, tc.name); err == nil {
						t.Fatal("whole graph accepted " + fault)
					}
				})
			}
		})
	}
}

func TestP15TaggedCountExternalContextBoundaries(t *testing.T) {
	for _, tc := range []p15TaggedBoundaryCase{
		{"ordinary_native_list", "return [[1],[]]", "[[1],[]]", analysis.CodeTargetValueShape, "structured return requires an explicit external value codec"},
		{"scope_restored_after_count", "count(repeat 2 {yield [[1]]})\nreturn [[2]]", "[[2]]", analysis.CodeTargetValueShape, "structured return requires an explicit external value codec"},
		{"standalone_discarded", "return [[\"discarded\"]]", "[[\"discarded\"]]", analysis.CodeTargetValueShape, "structured return requires an explicit external value codec"},
		{"standalone_bound_shape", "let payload=[[1]]\nreturn payload", "payload", analysis.CodeTargetValueShape, "structured return requires an explicit external value codec"},
		{"standalone_absence", "return [[none]]", "[[none]]", analysis.CodeTargetValueShape, "structured return requires an explicit external value codec"},
	} {
		t.Run(tc.name, func(t *testing.T) { p15CheckTaggedBoundary(t, tc) })
	}
}
