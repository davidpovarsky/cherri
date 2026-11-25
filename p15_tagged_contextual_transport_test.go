package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The expected child layout comes from the authored destination, independently
// of the inferred Bool-only literal. Every producer and edge is checked.
func p15TaggedContextualGraph(actions []map[string]any, depth int, dictionary bool) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	cell := p15TaggedCellGraph(g, "bool", p15OptionalJoinValue("false"))
	if dictionary {
		array := p15F04ArrayValue(p15FunctionArgument{true, 0, p15FunctionEdge(cell)})
		p15S01Dictionary(g, p15S01ContainerRow(p15FunctionText("slot"), array.kind, array.value))
		one := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		g.output(one)
		return g.compare()
	}
	for i := 1; i < depth; i++ {
		cell = p15TaggedCellGraph(g, "list", p15F04ArrayValue(p15FunctionArgument{true, 0, p15FunctionEdge(cell)}))
	}
	list := g.value("list", map[string]any{"WFItems": []any{
		map[string]any{"WFItemType": 0, "WFValue": p15FunctionEdge(cell)},
	}})
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(list)})
	g.output(count)
	return g.compare()
}

func TestP15TaggedContextualLiteralGraphs(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		depth        int
		dictionary   bool
	}{
		{"nested_list", "let values:List<List<Optional<Bool>>>=[[false]]\nreturn count(values)", 2, false},
		{"deep_list", "let values:List<List<List<Optional<Bool>>>>=[[[false]]]\nreturn count(values)", 3, false},
		{"dictionary_array", "let values:Map<Text,List<Optional<Bool>>>={\"slot\":[false]}\nreturn 1", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15TaggedContextualGraph(actions, tc.depth, tc.dictionary); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Locate the original omitted-argument branch through its actual has_arg_0
// producer and exact equality condition. The whole fallback region is checked.
func p15TaggedDefaultRegion(actions []map[string]any) ([]map[string]any, error) {
	slot, start, alternate, end := -1, -1, -1, -1
	for i, action := range actions {
		p := p15Param(action)
		key, _ := p15LiteralText(p["WFDictionaryKey"])
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && key == "has_arg_0" {
			if slot >= 0 {
				return nil, fmt.Errorf("duplicate default-presence producer")
			}
			slot = i
		}
	}
	if slot < 0 {
		return nil, fmt.Errorf("missing supplied/default discriminator")
	}
	id, _ := p15Param(actions[slot])["UUID"].(string)
	group := ""
	for i := slot + 1; i < len(actions); i++ {
		p := p15Param(actions[i])
		if actions[i]["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		outs, vars := p15V02NativeReferences(p["WFInput"])
		operand, _ := p15LiteralText(p["WFConditionalActionString"])
		if p15Int(p["WFControlFlowMode"]) == 0 && reflect.DeepEqual(outs, []string{id}) && len(vars) == 0 {
			if p15Int(p["WFCondition"]) != 4 || operand != "1" {
				return nil, fmt.Errorf("default selection polarity changed")
			}
			start = i
			group, _ = p["GroupingIdentifier"].(string)
			break
		}
	}
	if start < 0 || group == "" {
		return nil, fmt.Errorf("missing exact argument-presence branch")
	}
	for i := start + 1; i < len(actions); i++ {
		p := p15Param(actions[i])
		if p["GroupingIdentifier"] != group {
			continue
		}
		switch p15Int(p["WFControlFlowMode"]) {
		case 1:
			if alternate >= 0 {
				return nil, fmt.Errorf("duplicate fallback branch")
			}
			alternate = i
		case 2:
			end = i
		}
		if end >= 0 {
			break
		}
	}
	if alternate <= start || end <= alternate {
		return nil, fmt.Errorf("incomplete default branch")
	}
	return actions[alternate+1 : end], nil
}

func p15TaggedDefaultGraph(actions []map[string]any, dictionary bool) error {
	region, err := p15TaggedDefaultRegion(actions)
	if err != nil {
		return err
	}
	g := &p15FunctionGraph{actual: region, groups: map[string]bool{}}
	cell := p15TaggedCellGraph(g, "bool", p15OptionalJoinValue("false"))
	if dictionary {
		p15S01Dictionary(g, p15S01ContainerRow(p15FunctionText("flag"), 0, p15FunctionEdge(cell)))
	} else {
		g.value("list", map[string]any{"WFItems": []any{map[string]any{"WFItemType": 0, "WFValue": p15FunctionEdge(cell)}}})
	}
	return g.compare()
}

func TestP15TaggedFunctionDefaultGraphs(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		dictionary   bool
	}{
		{"list", "function first(values:List<Optional<Bool>>=[false])->Bool {let item=values[0]\nreturn item==none}\nreturn first()", false},
		{"map", "function first(values:Map<Text,Optional<Bool>>={\"flag\":false})->Bool {let item=values[\"flag\"]\nreturn item==none}\nreturn first()", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15TaggedDefaultGraph(actions, tc.dictionary); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15TaggedBoundaryCoercionsAndLayouts(t *testing.T) {
	for name, source := range map[string]string{
		"null_text":           "let value=null\nreturn value.text",
		"list_text":           "let values=[none,false]\nreturn values.text",
		"map_text":            "let values:Map<Text,Optional<Bool>>={\"flag\":none}\nreturn values.text",
		"implicit_previous":   "let value=null\nnative.action(\"is.workflow.actions.detect.text\",{})\nreturn 1",
		"deep_dynamic_layout": "function consume(values:List<List<Optional<Bool>>>)->Number{return count(values)}\nlet input:List<List<Bool>>=[[false]]\nreturn consume(input)",
		"mixed_json_join":     "let value=if true {yield false}else{yield null}\nreturn value==null",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("p15-tagged-boundary-"+name+".cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
				t.Fatalf("unconverted storage crossed a semantic boundary: bytes=%d err=%v", len(raw), err)
			}
		})
	}
}

func TestP15TaggedMapMemberPresence(t *testing.T) {
	const source = "let values:Map<Text,Optional<Bool>>={\"present\":none}\nlet absent=values.missing\nreturn absent==none"
	actions := p15CompiledActions(t, source)
	if err := p15TaggedMapMissingGuard(actions); err != nil {
		t.Fatal(err)
	}
}

func TestP15TaggedExplicitNativeInputControls(t *testing.T) {
	for name, source := range map[string]string{
		"explicit_input":      "let value=null\nnative.action(\"is.workflow.actions.detect.text\",{\"WFInput\":\"safe\"})\nreturn 1",
		"self_contained_text": "let value=null\nnative.action(\"is.workflow.actions.gettext\",{\"WFTextActionText\":\"safe\"})\nreturn 1",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("p15-tagged-explicit-"+name+".cherri", source)
			if err != nil || len(raw) == 0 {
				t.Fatalf("explicit ordinary input was rejected: %v", err)
			}
		})
	}
}
