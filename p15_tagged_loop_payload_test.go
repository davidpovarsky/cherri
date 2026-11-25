package main

import (
	"fmt"
	"sort"
	"testing"
)

// All required operations, payload fields and source edges are authored here;
// only UUID/private-variable alpha names come from their expected positions.
func p15TaggedCellGraph(g *p15FunctionGraph, kind string, payload p15FunctionArgument) string {
	fields := map[string]p15FunctionArgument{
		"_cherri_value_cell_v1": p15FunctionString("1"),
		"has_value":             p15FunctionString("1"),
		"kind":                  p15FunctionString(kind),
		"value":                 payload,
	}
	keys := []string{}
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := []any{}
	for _, key := range keys {
		field := fields[key]
		rows = append(rows, map[string]any{"WFKey": p15FunctionText(key), "WFItemType": field.kind, "WFValue": field.value})
	}
	return g.value("dictionary", map[string]any{"WFItems": map[string]any{
		"WFSerializationType": "WFDictionaryFieldValue",
		"Value":               map[string]any{"WFDictionaryFieldValueItems": rows},
	}})
}

func p15TaggedLoopPayloadGraph(actions []map[string]any, nested bool) (map[string]int, error) {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	positions := map[string]int{}
	payload := ""
	if !nested {
		positions["payload"] = len(g.expected)
		payload = g.value("list", map[string]any{"WFItems": []any{
			map[string]any{"WFItemType": 0, "WFValue": "a"},
			map[string]any{"WFItemType": 0, "WFValue": "b"},
		}})
	}
	positions["clear"] = len(g.expected)
	accumulator := p15F04ClearedCollection(g, "loopResults")
	loop := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", 2)
	value := p15FunctionArgument{true, 0, p15FunctionEdge(payload)}
	if nested {
		positions["payload"] = len(g.expected)
		first := p15TaggedCellGraph(g, "list", p15F04ArrayValue(p15FunctionString("inner")))
		second := p15TaggedCellGraph(g, "list", p15F04ArrayValue())
		value = p15F04ArrayValue(
			p15FunctionArgument{true, 0, p15FunctionEdge(first)},
			p15FunctionArgument{true, 0, p15FunctionEdge(second)},
		)
	}
	positions["carrier"] = len(g.expected)
	cell := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_loop_result_cell_v1", true, value)
	positions["append"] = len(g.expected)
	g.emit("appendvariable", map[string]any{"WFVariableName": accumulator, "WFInput": p15FunctionEdge(cell)})
	positions["end"] = len(g.expected)
	p15DiscoveryLoopEnd(g, "repeat.count", loop, false)
	collection := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	positions["count"] = len(g.expected)
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(collection)})
	g.output(count)
	return positions, g.compare()
}

func p15CheckTaggedLoopPayload(t *testing.T, source string, nested bool) {
	t.Helper()
	actions := p15CompiledActions(t, source)
	positions, err := p15TaggedLoopPayloadGraph(actions, nested)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{
		"prior_value_not_cleared", "wrong_payload", "wrong_presence",
		"wrong_marker", "append_payload", "wrong_loop_boundary", "count_payload", "extra_action",
	} {
		t.Run(fault, func(t *testing.T) {
			broken := p15TaggedClone(t, actions)
			payloadID, _ := p15Param(broken[positions["payload"]])["UUID"].(string)
			switch fault {
			case "prior_value_not_cleared":
				p15Param(broken[positions["clear"]+1])["WFInput"] = p15FunctionEdge("UNRELATED_PREVIOUS")
			case "wrong_payload", "wrong_presence", "wrong_marker":
				fields, err := p15ContainerFields(p15Param(broken[positions["carrier"]])["WFItems"])
				if err != nil {
					t.Fatal(err)
				}
				key, value := "value", any(p15FunctionEdge("UNRELATED_PAYLOAD"))
				if fault == "wrong_presence" {
					key, value = "has_value", p15FunctionText("0")
				}
				if fault == "wrong_marker" {
					key, value = "_cherri_loop_result_cell_v1", p15FunctionText("unowned")
				}
				fields[key]["WFValue"] = value
			case "append_payload":
				p15Param(broken[positions["append"]])["WFInput"] = p15FunctionEdge(payloadID)
			case "wrong_loop_boundary":
				p15Param(broken[positions["end"]])["GroupingIdentifier"] = "UNRELATED_LOOP"
			case "count_payload":
				p15Param(broken[positions["count"]])["Input"] = p15FunctionEdge(payloadID)
			case "extra_action":
				broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			}
			if _, err := p15TaggedLoopPayloadGraph(broken, nested); err == nil {
				t.Fatal("accepted " + fault)
			}
		})
	}
}

func TestP15TaggedLoopPayloadNativeGraph(t *testing.T) {
	cases := []struct {
		name, source string
		nested       bool
	}{
		{"dynamic_list_payload", "let payload=[\"a\",\"b\"]\nlet values=repeat 2 {yield payload}\nreturn count(values)", false},
		{"nested_list_payload", "let values=repeat 2 {yield [[\"inner\"],[]]}\nreturn count(values)", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { p15CheckTaggedLoopPayload(t, tc.source, tc.nested) })
	}
}

// These exercise the source-owned representation in index/for/alias consumers.
// Native execution and type/count/value observations are a separate tier.
func TestP15TaggedLoopPayloadConsumers(t *testing.T) {
	for name, source := range map[string]string{
		"dynamic_zero":             "let payload=[1,2]\nlet values=repeat 0 {yield payload}\nreturn count(values)",
		"dynamic_for":              "let payload=[1,2]\nlet values=for _ in [1,2] {yield payload}\nreturn count(values)",
		"dynamic_index":            "let payload=[1,2]\nlet values=repeat 2 {yield payload}\nlet item=values[0]\nif item!=none {return count(item)}\nreturn -1",
		"dynamic_decoded_for":      "let payload=[1,2]\nlet values=repeat 2 {yield payload}\nvar total=0\nfor item in values {total+=count(item)}\nreturn total",
		"nested_decoded_for":       "let payload:List<List<Number>>=[[1,2],[]]\nlet values=repeat 2 {yield payload}\nvar total=0\nfor item in values {total+=count(item)}\nreturn total",
		"optional_slots_payload":   "let payload:List<Optional<Bool>>=[none,false]\nlet values=repeat 2 {yield payload}\nreturn count(values)",
		"mutable_payload_snapshot": "var payload=[1]\nlet values=repeat 2 {let old=payload\npayload=[2,3]\nyield old}\nreturn count(values)",
		"map_payload":              "let payload={\"x\":1}\nlet values=repeat 2 {if 1==1 {yield payload}else{yield none}}\nreturn count(values)",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("p15-tagged-loop-"+name+".cherri", source)
			if err != nil || len(raw) == 0 {
				t.Fatal(fmt.Sprintf("checked payload failed: bytes=%d err=%v", len(raw), err))
			}
		})
	}
}
