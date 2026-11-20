package main

import (
	"fmt"
	"testing"
)

func p15DictionaryIndexBoundarySource(name string) string {
	body := ""
	switch name {
	case "capture":
		body = `var state="BEFORE";getVariable({"key":state,"later":if true{state="AFTER";yield "SECOND"}else{yield "UNUSED"}}["key"])`
	case "reserved_keys":
		body = `getVariable({"WFSerializationType":"WFTextTokenString","Value":{"string":"WRONG"},"key":""}["key"])`
	case "empty_list":
		body = `getVariable({"key":[]}["key"])`
	case "empty_map":
		body = `getVariable({"key":{}}["key"])`
	default:
		panic("unknown direct Dictionary boundary")
	}
	return "function probe()->Text{" + body + `;return "DONE"}` + "\ngetVariable(probe())"
}

func p15DictionaryKeyEdge(dictionary, key string) map[string]any {
	return p15FunctionAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": dictionary,
		"Aggrandizements": []any{map[string]any{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": key}}})
}

func p15DictionaryIndexBoundaryGraphError(actions []map[string]any, name string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	g.header("probe")
	values := map[string]any{}
	switch name {
	case "capture":
		before := g.value("gettext", map[string]any{"WFTextActionText": "BEFORE"})
		g.emit("setvariable", map[string]any{"WFVariableName": "state", "WFInput": p15FunctionEdge(before)})
		early := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("state")})
		condition := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		group := g.conditional(4, p15FunctionEdge(condition), "WFNumberValue", 1)
		after := g.value("gettext", map[string]any{"WFTextActionText": "AFTER"})
		g.emit("setvariable", map[string]any{"WFVariableName": "state", "WFInput": p15FunctionEdge(after)})
		g.value("gettext", map[string]any{"WFTextActionText": "SECOND"})
		g.boundary(group, 1, false)
		g.value("gettext", map[string]any{"WFTextActionText": "UNUSED"})
		later := g.boundary(group, 2, true)
		values = map[string]any{"key": p15ContainerReference{"Type": "ActionOutput", "OutputUUID": early}, "later": p15ContainerReference{"Type": "ActionOutput", "OutputUUID": later}}
	case "reserved_keys":
		values = map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "WRONG"}, "key": ""}
	case "empty_list":
		values["key"] = []any{}
	case "empty_map":
		values["key"] = map[string]any{}
	default:
		return fmt.Errorf("unmodeled direct Dictionary boundary %q", name)
	}
	_, wire := p15ContainerFieldValue(values)
	dictionary := g.value("dictionary", map[string]any{"WFItems": wire})
	g.emit("getvariable", map[string]any{"WFVariable": p15DictionaryKeyEdge(dictionary, "key")})
	g.output(g.value("gettext", map[string]any{"WFTextActionText": "DONE"}))
	g.caller()
	result := g.call(g.payload("probe"))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalDictionaryDirectIndexKeepsTypedValuesAndOrder(t *testing.T) {
	for _, name := range []string{"capture", "reserved_keys", "empty_list", "empty_map"} {
		t.Run(name, func(t *testing.T) {
			if err := p15DictionaryIndexBoundaryGraphError(p15CompiledActions(t, p15DictionaryIndexBoundarySource(name)), name); err != nil {
				t.Fatal(err)
			}
		})
	}
}
