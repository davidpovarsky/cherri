package main

import (
	"fmt"
	"testing"
)

type p15UnusedOptionalCase struct{ name, body string }

func p15UnusedOptionalCases() []p15UnusedOptionalCase {
	return []p15UnusedOptionalCase{
		{"none", `text("SEED");var candidate:Text?=none`},
		{"empty", `text("SEED");var candidate:Text?=""`},
		{"false", `text("SEED");var candidate:Bool?=false`},
		{"zero", `text("SEED");var candidate:Number?=0`},
		{"assignment", `var candidate:Text?="";candidate=none`},
		{"alias", `var candidate:Text?=none;let alias:Text?=candidate`},
		{"branch", `var candidate:Text?=none;if number(1)>0{candidate=""}`},
		{"loop", `var candidate:Text?=none;repeat 1{candidate=""}`},
	}
}

func p15UnusedOptionalSource(tc p15UnusedOptionalCase) string {
	return "function main()->Text{" + tc.body + `;return "DONE"}` + "\ngetVariable(main())"
}

// A source binding remains present in the emitted program; only metadata with
// no semantic consumer is omitted. This oracle specifies the entire graph.
func p15UnusedOptionalGraphError(actions []map[string]any, name string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	g.header("main")
	clear := func() { g.emit("nothing", nil); g.emit("setvariable", map[string]any{"WFVariableName": "candidate"}) }
	present := func(kind, key string, value any) {
		producer := g.value(kind, map[string]any{key: value})
		g.emit("setvariable", map[string]any{"WFVariableName": "candidate", "WFInput": p15FunctionEdge(producer)})
	}
	switch name {
	case "none", "empty", "false", "zero":
		g.emit("gettext", map[string]any{"WFTextActionText": "SEED"})
		switch name {
		case "none":
			clear()
		case "empty":
			present("gettext", "WFTextActionText", "")
		default:
			present("number", "WFNumberActionNumber", 0)
		}
	case "assignment":
		present("gettext", "WFTextActionText", "")
		clear()
	case "alias":
		clear()
		g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("candidate")})
	case "branch":
		clear()
		condition := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		group := g.conditional(2, p15FunctionEdge(condition), "WFNumberValue", 0)
		present("gettext", "WFTextActionText", "")
		g.boundary(group, 2, false)
	case "loop":
		clear()
		index := len(g.expected)
		group := ""
		if index < len(g.actual) {
			group, _ = p15Param(g.actual[index])["GroupingIdentifier"].(string)
		}
		if group == "" || g.groups[group] {
			return fmt.Errorf("missing or reused Repeat group")
		}
		g.groups[group] = true
		g.emit("repeat.count", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 0, "WFRepeatCount": 1})
		present("gettext", "WFTextActionText", "")
		g.emit("repeat.count", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 2})
	default:
		return fmt.Errorf("unknown unused Optional scenario %q", name)
	}
	g.output(g.value("gettext", map[string]any{"WFTextActionText": "DONE"}))
	g.caller()
	result := g.call(g.payload("main"))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalUnconsumedPresenceKeepsNativeSequence(t *testing.T) {
	for _, tc := range p15UnusedOptionalCases() {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15UnusedOptionalGraphError(p15CompiledActions(t, p15UnusedOptionalSource(tc)), tc.name); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15OptionalUnconsumedPresenceOracleRejectsCorruption(t *testing.T) {
	tc := p15UnusedOptionalCases()[0]
	original := p15CompiledActions(t, p15UnusedOptionalSource(tc))
	if err := p15UnusedOptionalGraphError(original, tc.name); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"extra_metadata", "missing_seed", "missing_nothing", "wrong_store", "fabricated_empty_input", "lost_output"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15ContainerCloneNative(t, original)
			if err := p15UnusedOptionalGraphError(changed, tc.name); err != nil {
				t.Fatal("unchanged clone rejected", err)
			}
			applied := false
			for i, a := range changed {
				p := p15Param(a)
				if p["WFTextActionText"] == "SEED" {
					if fault == "missing_seed" {
						p["WFTextActionText"] = ""
						applied = true
						break
					}
					if fault == "extra_metadata" {
						extra := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "0"}}
						changed = append(changed[:i+1], append([]map[string]any{extra}, changed[i+1:]...)...)
						applied = true
						break
					}
				}
				if p["WFVariableName"] == "candidate" {
					switch fault {
					case "missing_nothing":
						changed = append(changed[:i-1], changed[i:]...)
						applied = true
					case "wrong_store":
						p["WFVariableName"] = "WRONG"
						applied = true
					case "fabricated_empty_input":
						p["WFInput"] = ""
						applied = true
					}
					if applied {
						break
					}
				}
				if p["WFTextActionText"] == "DONE" && fault == "lost_output" {
					p["WFTextActionText"] = ""
					applied = true
					break
				}
			}
			if !applied {
				t.Fatal("mutation not applied")
			}
			if err := p15UnusedOptionalGraphError(changed, tc.name); err == nil {
				t.Fatal("corrupt sequence accepted", fault)
			}
		})
	}
}
