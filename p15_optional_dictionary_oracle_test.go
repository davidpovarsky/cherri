package main

import (
	"fmt"
	"strings"
	"testing"
)

func p15OptionalDictionaryLiteralValue(name string) any {
	switch name {
	case "text":
		return "ready"
	case "empty_text":
		return ""
	case "false":
		return false
	case "zero":
		return p15ContainerNumber("0")
	case "empty_list":
		return []any{}
	case "list":
		return []any{"one", "two"}
	case "empty_map":
		return map[string]any{}
	case "map":
		return map[string]any{"nested": "VALUE"}
	default:
		panic("unknown Dictionary literal")
	}
}

// Require retains its actual count/type rejection path and returns the exact
// original Dictionary edge. These checks do not infer the indexed key exists;
// that separate fact originates in the authored literal, not native truthiness.
func p15OptionalDictionaryRequireGraph(g *p15FunctionGraph, dictionary string) string {
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(dictionary), "WFCountType": "Items"})
	reject := func(input string, field string, operand any) {
		group := g.conditional(5, p15FunctionEdge(input), field, operand)
		g.emit("alert", map[string]any{"WFAlertActionMessage": "Requirement failed", "WFAlertActionCancelButtonShown": false})
		g.emit("exit", nil)
		g.boundary(group, 2, false)
	}
	reject(count, "WFNumberValue", 1)
	actual := g.value("getitemtype", map[string]any{"WFInput": p15FunctionEdge(dictionary)})
	_, sample := p15ContainerFieldValue(map[string]any{"cherri_type_witness": true})
	witness := g.value("dictionary", map[string]any{"WFItems": sample})
	expected := g.value("getitemtype", map[string]any{"WFInput": p15FunctionEdge(witness)})
	reject(actual, "WFConditionalActionString", p15FunctionText("\ufffc", expected))
	return g.value("getvariable", map[string]any{"WFVariable": p15FunctionEdge(dictionary)})
}

func p15OptionalDictionaryGraphError(actions []map[string]any, tc p15OptionalDictionaryCase, route string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	value, presence := p15DecodeOptionalArgumentGraph(g, envelope, 0)
	p15ForwardResultGraph(g, value, presence)
	g.caller()
	_, wire := p15ContainerFieldValue(map[string]any{"key": p15OptionalDictionaryLiteralValue(tc.name)})
	dictionary := g.value("dictionary", map[string]any{"WFItems": wire})
	if route == "require" || route == "named_require_alias" {
		dictionary = p15OptionalDictionaryRequireGraph(g, dictionary)
	}
	output, _ := p15ForwardCallGraph(g, "echo", p15FunctionArgument{true, 0, p15DictionaryKeyEdge(dictionary, "key")}, "1", nil, nil)
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(output)})
	return g.finish()
}

func TestP15OptionalDictionaryCompleteNativeGraph(t *testing.T) {
	for _, tc := range p15OptionalDictionaryCases() {
		for _, route := range []string{"literal", "alias", "require", "named_require_alias"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				if err := p15OptionalDictionaryGraphError(p15CompiledActions(t, p15OptionalDictionarySource(tc, route, "argument")), tc, route); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15OptionalDictionaryImportedBodyAndDemand(t *testing.T) {
	// An imported function consumes a mutable caller binding. The unchanged
	// complete forwarding oracle requires the private flag assignment/edges.
	entry := "import \"./dependency.cherri\" as dep\nvar candidate:Text?=none\ngetVariable(dep.echo(candidate))"
	dependency := "function echo(value:Text?)->Text?{return value}\n"
	path, source := p15ModuleProject(t, map[string]string{"main.cherri": entry, "dependency.cherri": dependency})
	actions := p15OptionalAliasModuleActions(t, path, source, "module:dependency.cherri::echo")
	if err := p15OptionalForwardingGraphError(actions, "mutable_none"); err != nil {
		t.Fatal(err)
	}
	// The indexed proof belongs to the dependency's exact AST, not the caller.
	for _, tc := range p15OptionalDictionaryCases() {
		t.Run(tc.name, func(t *testing.T) {
			prefix, selected := p15OptionalDictionarySelection(tc, "require")
			dependency = "function echo()->" + tc.typ + "?{" + prefix + "return " + selected + "}\n"
			entry = "import \"./dependency.cherri\" as dep\ngetVariable(dep.echo())"
			path, source = p15ModuleProject(t, map[string]string{"main.cherri": entry, "dependency.cherri": dependency})
			actions = p15OptionalAliasModuleActions(t, path, source, "module:dependency.cherri::echo")
			g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
			g.header("echo")
			_, wire := p15ContainerFieldValue(map[string]any{"key": p15OptionalDictionaryLiteralValue(tc.name)})
			dictionary := g.value("dictionary", map[string]any{"WFItems": wire})
			dictionary = p15OptionalDictionaryRequireGraph(g, dictionary)
			carrier := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionArgument{true, 0, p15DictionaryKeyEdge(dictionary, "key")})
			g.output(carrier)
			g.caller()
			result := g.call(g.payload("echo"))
			value, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
			g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(value)})
			if err := g.finish(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func p15MutateDictionaryLookupGraph(actions []map[string]any, fault string) bool {
	for _, a := range actions {
		p := p15Param(a)
		kind, _ := a["WFWorkflowActionIdentifier"].(string)
		if kind == "is.workflow.actions.dictionary" {
			fields, err := p15ContainerFields(p["WFItems"])
			if err != nil {
				continue
			}
			if row := fields["key"]; row != nil {
				switch fault {
				case "wrong_key":
					row["WFKey"] = p15FunctionText("other")
					return true
				case "wrong_kind":
					row["WFItemType"] = 3
					return true
				case "omitted_empty_array":
					delete(row["WFValue"].(map[string]any), "Value")
					return true
				case "empty_as_none":
					row["WFValue"] = p15FunctionText("")
					return true
				}
			}
			if fields["cherri_type_witness"] != nil && fault == "wrong_type_witness" {
				fields["cherri_type_witness"]["WFValue"] = p15FunctionText("true")
				return true
			}
			if fields["_cherri_optional_argument_v1"] != nil {
				switch fault {
				case "wrong_presence":
					fields["has_value"]["WFValue"] = p15FunctionText("0")
					return true
				case "value_as_presence":
					fields["has_value"]["WFValue"] = fields["value"]["WFValue"]
					return true
				case "wrong_lookup":
					descriptor := fields["value"]["WFValue"].(map[string]any)["Value"].(map[string]any)
					descriptor["Aggrandizements"].([]any)[0].(map[string]any)["DictionaryKey"] = "other"
					return true
				}
			}
		}
		if kind == "is.workflow.actions.count" && fault == "wrong_count_input" {
			p["Input"] = p15FunctionVariable("unrelated")
			return true
		}
		if kind == "is.workflow.actions.conditional" && p15Int(p["WFCondition"]) == 5 && p["WFNumberValue"] != nil && fault == "wrong_count_guard" {
			p["WFNumberValue"] = 0
			return true
		}
		if kind == "is.workflow.actions.alert" && fault == "ignored_rejection" {
			p["WFAlertActionCancelButtonShown"] = true
			return true
		}
		if kind == "is.workflow.actions.getvariable" && p["UUID"] != nil && fault == "wrong_passthrough" {
			p["WFVariable"] = p15FunctionVariable("unrelated")
			return true
		}
	}
	return false
}

func TestP15OptionalDictionaryNativeOracleRejectsCorruption(t *testing.T) {
	tc := p15OptionalDictionaryCases()[4]
	original := p15CompiledActions(t, p15OptionalDictionarySource(tc, "require", "argument"))
	if err := p15OptionalDictionaryGraphError(original, tc, "require"); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"wrong_key", "wrong_kind", "omitted_empty_array", "empty_as_none", "wrong_type_witness", "wrong_presence", "value_as_presence", "wrong_lookup", "wrong_count_input", "wrong_count_guard", "ignored_rejection", "wrong_passthrough"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15ContainerCloneNative(t, original)
			if err := p15OptionalDictionaryGraphError(changed, tc, "require"); err != nil {
				t.Fatal("unchanged clone rejected", err)
			}
			if !p15MutateDictionaryLookupGraph(changed, fault) {
				t.Fatal("mutation not applied", fault)
			}
			if err := p15OptionalDictionaryGraphError(changed, tc, "require"); err == nil {
				t.Fatal("wrong Dictionary/presence edge accepted", fault)
			}
		})
	}
	for _, name := range []string{"capture", "reserved_keys", "empty_list", "empty_map"} {
		t.Run("direct/"+name, func(t *testing.T) {
			actual := p15CompiledActions(t, p15DictionaryIndexBoundarySource(name))
			if err := p15DictionaryIndexBoundaryGraphError(actual, name); err != nil {
				t.Fatal(err)
			}
			changed := p15ContainerCloneNative(t, actual)
			applied := false
			for i, a := range changed {
				p := p15Param(a)
				if name == "capture" && a["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && strings.Contains(fmt.Sprint(p["WFVariable"]), "state") {
					changed = append(changed[:i], changed[i+1:]...)
					applied = true
					break
				}
				if name != "capture" && a["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
					fields, err := p15ContainerFields(p["WFItems"])
					if err != nil || fields["key"] == nil {
						continue
					}
					fields["key"]["WFValue"] = p15FunctionText("LOST")
					applied = true
					break
				}
			}
			if !applied {
				t.Fatal("direct boundary mutation not applied")
			}
			if err := p15DictionaryIndexBoundaryGraphError(changed, name); err == nil {
				t.Fatal("direct value loss accepted")
			}
		})
	}
}
