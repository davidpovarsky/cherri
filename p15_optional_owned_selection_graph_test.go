package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// These are the unchanged original rejection specimens. Source-owned Optional
// cells now support them. Keep their historical test identities, and require the
// complete native graph instead of allowing an obsolete construction error.
func p15OptionalOwnedSelectionSource(name string) string {
	switch name {
	case "optional_element":
		return `let maybe:Text?=none;let selected:Text?=[maybe][0]`
	case "optional_result_element":
		return `function maybe()->Text?{return none};let selected:Text?=[maybe()][0]`
	case "optional_entry":
		return `let maybe:Text?=none;let selected:Text?={"key":maybe}["key"]`
	default:
		panic("unknown original Optional selection specimen")
	}
}

func p15OptionalSelectionFlag(g *p15FunctionGraph, purpose string) string {
	zero := g.value("gettext", map[string]any{"WFTextActionText": "0"})
	name := p15F04PrivateName(g, "__cherri_scope_")
	if !strings.Contains(name, "_"+purpose+"_") {
		g.err = fmt.Errorf("Optional selection flag has the wrong source role")
	}
	g.emit("setvariable", map[string]any{"WFVariableName": name, "WFInput": p15FunctionEdge(zero)})
	return name
}

// Independently require literal, case-sensitive, full Text equality for the
// actual enumerated key and the once-captured authored key. All regex escaping,
// anchors, match counting and Boolean polarity are part of the expected graph.
func p15OptionalSelectionExactKey(g *p15FunctionGraph, key string) string {
	ref := func(id string) map[string]any { return map[string]any{"Type": "ActionOutput", "OutputUUID": id} }
	left := g.value("gettext", map[string]any{"WFTextActionText": p15N03AffixedTokenString("~", map[string]any{"Type": "Variable", "VariableName": "Repeat Item"}, "")})
	right := g.value("gettext", map[string]any{"WFTextActionText": p15N03AffixedTokenString("~", ref(key), "")})
	quoted := g.value("text.replace", map[string]any{
		"WFReplaceTextFind": `\E`, "WFReplaceTextReplace": `\E\\E\Q`,
		"WFInput":                    p15N03AffixedTokenString("", ref(right), ""),
		"WFReplaceTextCaseSensitive": true, "WFReplaceTextRegularExpression": false,
	})
	matches := g.value("text.match", map[string]any{
		"WFMatchTextPattern": p15N03AffixedTokenString(`\A\Q`, ref(quoted), `\E\z`),
		"text":               p15N03AffixedTokenString("", ref(left), ""), "WFMatchTextCaseSensitive": true,
	})
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(matches)})
	group := g.conditional(4, p15FunctionEdge(count), "WFNumberValue", 1)
	g.emit("number", map[string]any{"WFNumberActionNumber": 1})
	g.boundary(group, 1, false)
	g.emit("number", map[string]any{"WFNumberActionNumber": 0})
	return g.boundary(group, 2, true)
}

// UUIDs, group labels and private-variable alpha names are taken only from their
// expected positions. No action identifier, field, type, payload, ordering or
// data/control edge is copied from the compiler into the expected graph.
func p15OptionalOwnedSelectionGraph(actual []map[string]any, name string) (map[string]int, error) {
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	pos := map[string]int{}
	cell := ""
	if name == "optional_result_element" {
		g.header("maybe")
		pos["function_result"] = len(g.expected)
		result := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
		g.output(result)
		g.caller()
		pos["function_call"] = len(g.expected) + 1
		result = g.call(g.payload("maybe"))
		value, presence := p15DecodeOptionalGraph(g, result, "has_value", "value")
		group := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
		pos["present_cell"] = len(g.expected)
		p15TaggedCellGraph(g, "text", p15FunctionArgument{true, 0, p15FunctionEdge(value)})
		g.boundary(group, 1, false)
		pos["absent_cell"] = len(g.expected)
		p15TaggedAbsentCellGraph(g)
		cell = g.boundary(group, 2, true)
	} else {
		g.emit("nothing", nil)
		g.emit("setvariable", map[string]any{"WFVariableName": "maybe"})
		pos["absent_cell"] = len(g.expected)
		cell = p15TaggedAbsentCellGraph(g)
	}
	pos["container"] = len(g.expected)
	if name == "optional_entry" {
		dictionary := g.value("dictionary", map[string]any{"WFItems": map[string]any{
			"WFSerializationType": "WFDictionaryFieldValue",
			"Value": map[string]any{"WFDictionaryFieldValueItems": []any{
				map[string]any{"WFKey": p15FunctionText("key"), "WFItemType": 0, "WFValue": p15FunctionEdge(cell)},
			}},
		}})
		key := g.value("gettext", map[string]any{"WFTextActionText": "key"})
		key = g.value("getvariable", map[string]any{"WFVariable": p15FunctionEdge(key)})
		keys := g.value("getvalueforkey", map[string]any{"WFInput": p15FunctionEdge(dictionary), "WFGetDictionaryValueType": "All Keys"})
		keyPresence := p15OptionalSelectionFlag(g, "dictionaryKeyPresent")
		loop := p15DiscoveryLoop(g, "repeat.each", "WFInput", p15FunctionEdge(keys))
		match := p15OptionalSelectionExactKey(g, key)
		pos["key_match"] = len(g.expected)
		matched := g.conditional(4, p15FunctionEdge(match), "WFNumberValue", 1)
		one := g.value("gettext", map[string]any{"WFTextActionText": "1"})
		g.emit("setvariable", map[string]any{"WFVariableName": keyPresence, "WFInput": p15FunctionEdge(one)})
		g.boundary(matched, 2, false)
		p15DiscoveryLoopEnd(g, "repeat.each", loop, false)
		cellPresence := p15OptionalSelectionFlag(g, "cellPresence")
		pos["key_guard"] = len(g.expected)
		exists := g.conditional(4, p15FunctionVariable(keyPresence), "WFConditionalActionString", "1")
		pos["selection"] = len(g.expected)
		selected := g.get("key", dictionary)
		pos["decode"] = len(g.expected)
		value, presence := p15DecodeOptionalGraph(g, selected, "has_value", "value")
		g.emit("setvariable", map[string]any{"WFVariableName": cellPresence, "WFInput": p15FunctionEdge(presence)})
		g.value("getvariable", map[string]any{"WFVariable": p15FunctionEdge(value)})
		g.boundary(exists, 1, false)
		g.emit("nothing", nil)
		g.boundary(exists, 2, true)
	} else {
		list := p15TaggedSingleCellList(g, cell)
		pos["clear"] = len(g.expected)
		valueName := p15F04ClearedCollection(g, "taggedIndexValue")
		presenceName := p15OptionalSelectionFlag(g, "taggedIndexPresence")
		oneBased := g.value("math", map[string]any{"WFMathOperation": "+", "WFInput": 0, "WFMathOperand": 1})
		count := g.value("count", map[string]any{"Input": p15FunctionEdge(list), "WFCountType": "Items"})
		difference := g.math("-", count, p15FunctionEdge(oneBased))
		lower := g.conditional(3, p15FunctionEdge(oneBased), "WFNumberValue", 1)
		upper := g.conditional(3, p15FunctionEdge(difference), "WFNumberValue", 0)
		pos["selection"] = len(g.expected)
		selected := g.value("getitemfromlist", map[string]any{"WFInput": p15FunctionEdge(list), "WFItemSpecifier": "Item At Index", "WFItemIndex": p15FunctionEdge(oneBased)})
		pos["decode"] = len(g.expected)
		value, presence := p15DecodeOptionalGraph(g, selected, "has_value", "value")
		g.emit("setvariable", map[string]any{"WFVariableName": presenceName, "WFInput": p15FunctionEdge(presence)})
		g.emit("setvariable", map[string]any{"WFVariableName": valueName, "WFInput": p15FunctionEdge(value)})
		g.boundary(upper, 1, false)
		g.emit("nothing", nil)
		g.boundary(upper, 2, true)
		g.boundary(lower, 1, false)
		g.emit("nothing", nil)
		g.boundary(lower, 2, true)
		g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(valueName)})
	}
	if name == "optional_result_element" {
		return pos, g.finish()
	}
	g.emit("nothing", nil)
	return pos, g.compare()
}

func p15CheckOriginalOwnedOptionalSelection(t *testing.T, tc p15OptionalIndexNegative) {
	t.Helper()
	if tc.source != p15OptionalOwnedSelectionSource(tc.name) {
		t.Fatal("the original Optional specimen was replaced")
	}
	checkDiagnostics := func(diagnostics []analysis.Diagnostic) {
		t.Helper()
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == analysis.SeverityError {
				t.Fatal("source-owned Optional transport rejected", diagnostic)
			}
		}
	}
	checkDiagnostics(p15TypeDiagnostics(t, tc.source))
	svc := service.NewService(nil)
	uri := "file:///p15-owned-optional-selection.cherri"
	svc.OpenDocument(uri, 1, tc.source)
	diagnostics, _ := svc.Analyze(uri)
	checkDiagnostics(diagnostics)
	actual := p15CompiledActions(t, tc.source)
	pos, err := p15OptionalOwnedSelectionGraph(actual, tc.name)
	if err != nil {
		t.Fatal(err)
	}
	// Keep historical named-event inventories stable. Every fault must be
	// rejected inside the original specimen, with its causal name in failures.
	faults := []string{"wrong_presence", "wrong_kind", "unexpected_payload", "foreign_container_cell",
		"wrong_selection", "wrong_decoder_presence", "truthiness_decode", "unguarded_payload", "extra_action"}
	if tc.name == "optional_entry" {
		faults = append(faults, "key_presence_is_payload_presence", "case_insensitive_key")
	}
	if tc.name == "optional_result_element" {
		faults = append(faults, "function_absence_relabel", "function_payload_relabel")
	}
	for _, fault := range faults {
		bad := p15TaggedClone(t, actual)
		if _, err := p15OptionalOwnedSelectionGraph(bad, tc.name); err != nil {
			t.Fatal("unchanged graph clone differs", err)
		}
		switch fault {
		case "wrong_presence", "wrong_kind", "unexpected_payload":
			fields, err := p15ContainerFields(p15Param(bad[pos["absent_cell"]])["WFItems"])
			if err != nil {
				t.Fatal(err)
			}
			if fault == "unexpected_payload" {
				items := p15Param(bad[pos["absent_cell"]])["WFItems"].(map[string]any)["Value"].(map[string]any)
				items["WFDictionaryFieldValueItems"] = append(items["WFDictionaryFieldValueItems"].([]any),
					map[string]any{"WFKey": p15FunctionText("value"), "WFItemType": 0, "WFValue": p15FunctionText("")})
			} else {
				key, value := "has_value", "1"
				if fault == "wrong_kind" {
					key, value = "kind", "text"
				}
				fields[key]["WFValue"] = p15FunctionText(value)
			}
		case "foreign_container_cell":
			if tc.name == "optional_entry" {
				fields, err := p15ContainerFields(p15Param(bad[pos["container"]])["WFItems"])
				if err != nil {
					t.Fatal(err)
				}
				fields["key"]["WFValue"] = p15FunctionEdge("UNRELATED_CELL")
			} else {
				p15Param(bad[pos["container"]])["WFItems"].([]any)[0].(map[string]any)["WFValue"] = p15FunctionEdge("UNRELATED_CELL")
			}
		case "wrong_selection":
			if tc.name == "optional_entry" {
				p15Param(bad[pos["selection"]])["WFDictionaryKey"] = "other"
			} else {
				p15Param(bad[pos["selection"]])["WFItemIndex"] = 2
			}
		case "wrong_decoder_presence":
			p15Param(bad[pos["decode"]])["WFDictionaryKey"] = "value"
		case "truthiness_decode":
			p15Param(bad[pos["decode"]+1])["WFCondition"] = 100
		case "unguarded_payload":
			a, b := pos["decode"]+1, pos["decode"]+2
			bad[a], bad[b] = bad[b], bad[a]
		case "extra_action":
			bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
		case "key_presence_is_payload_presence":
			p15Param(bad[pos["key_guard"]])["WFConditionalActionString"] = "0"
		case "case_insensitive_key":
			found := false
			for _, action := range bad {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.text.match" {
					p15Param(action)["WFMatchTextCaseSensitive"] = false
					found = true
				}
			}
			if !found {
				t.Fatal("missing owned key equality mutation target")
			}
		case "function_absence_relabel":
			fields, err := p15ContainerFields(p15Param(bad[pos["function_result"]])["WFItems"])
			if err != nil {
				t.Fatal(err)
			}
			fields["has_value"]["WFValue"] = p15FunctionText("1")
		case "function_payload_relabel":
			fields, err := p15ContainerFields(p15Param(bad[pos["present_cell"]])["WFItems"])
			if err != nil {
				t.Fatal(err)
			}
			fields["value"]["WFValue"] = p15FunctionText("FORGED_PRESENT_TEXT")
		}
		if _, err := p15OptionalOwnedSelectionGraph(bad, tc.name); err == nil {
			t.Fatalf("owned Optional graph accepted causal fault %s", fault)
		}
	}
}
