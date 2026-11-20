package main

import (
	"strings"
	"testing"
)

// Exact original source of the metadata/input-order characterization. Its
// candidate presence is unconsumed, so optimization must omit the private flag.
const p15OptionalImplicitUnconsumedSource = `function echo(seed:Text)->Text{
text("OPTIONAL_PREVIOUS_SEED")
var candidate:Text?=f"{native.action("is.workflow.actions.detect.text", {})}"
native.action("is.workflow.actions.detect.text", {})
candidate=none
native.action("is.workflow.actions.detect.text", {})
return "DONE"
}
getVariable(echo("unused"))`

func p15OptionalImplicitDemandedSource() string {
	return strings.Replace(p15OptionalImplicitUnconsumedSource, `return "DONE"`, "getVariable(candidate!=none)\n"+`return "DONE"`, 1)
}

func p15UnusedImplicitInputGraphError(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	g.get("arg_0", envelope)
	g.emit("gettext", map[string]any{"WFTextActionText": "OPTIONAL_PREVIOUS_SEED"})
	authored := g.value("detect.text", map[string]any{"UUID": "alpha"})
	value := g.value("gettext", map[string]any{"WFTextActionText": p15FunctionText("\ufffc", authored)})
	g.emit("setvariable", map[string]any{"WFVariableName": "candidate", "WFInput": p15FunctionEdge(value)})
	g.emit("detect.text", nil)
	g.emit("nothing", nil)
	g.emit("setvariable", map[string]any{"WFVariableName": "candidate"})
	g.emit("detect.text", nil)
	g.output(g.value("gettext", map[string]any{"WFTextActionText": "DONE"}))
	g.caller()
	result := g.call(g.payload("echo", p15FunctionString("unused")))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalUnconsumedImplicitInputSequence(t *testing.T) {
	removed := strings.Replace(p15OptionalImplicitDemandedSource(), "getVariable(candidate!=none)\n", "", 1)
	if removed != p15OptionalImplicitUnconsumedSource {
		t.Fatal("removing demand did not recover exact original source")
	}
	for _, tc := range []struct{ name, source string }{{"original", p15OptionalImplicitUnconsumedSource}, {"removed_consumer", removed}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15UnusedImplicitInputGraphError(p15CompiledActions(t, tc.source)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
