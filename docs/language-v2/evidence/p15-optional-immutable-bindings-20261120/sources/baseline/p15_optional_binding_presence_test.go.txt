package main

import (
	"strings"
	"testing"
)

func TestP15OptionalImmutableAliasesRetainPresence(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases()[:6] {
		for _, alias := range []struct{ name, body string }{
			{"one", "let alias=value;return alias"},
			{"chain", "let first=value;let alias=first;return alias"},
			{"annotated", "let alias:" + tc.typ + "?=value;return alias"},
		} {
			t.Run(tc.name+"/"+alias.name, func(t *testing.T) {
				source := strings.Replace(p15FunctionOptionalSource(tc), "{return value}", "{"+alias.body+"}", 1)
				if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
					t.Fatal(diagnostics)
				}
				// Aliasing an immutable native producer requires the same complete
				// value/presence graph; display labels are alpha names only.
				if err := p15FunctionOptionalError(p15CompiledActions(t, source), tc); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15OptionalBoundCallResultKeepsPresence(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases()[:2] {
		for _, alias := range []struct{ name, body string }{
			{"one", "let alias=echo(ARG)"},
			{"chain", "let first=echo(ARG);let alias=first"},
		} {
			for _, comparison := range []struct {
				op   string
				code int
			}{{"==", 4}, {"!=", 5}} {
				t.Run(tc.name+"/"+alias.name+"/"+comparison.op, func(t *testing.T) {
					source := "function echo(value:Text?)->Text?{return value}\n" + strings.Replace(alias.body, "ARG", tc.argument, 1) + "\ngetVariable(alias" + comparison.op + "none)"
					if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
						t.Fatal(diagnostics)
					}
					if err := p15OptionalCallPredicateError(p15CompiledActions(t, source), tc, comparison.code); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestP15OptionalImmutableNoneReturn(t *testing.T) {
	source := "function echo(seed:Text)->Text?{text(\"MUST_NOT_LEAK\");let candidate:Text?=none;return candidate}\ngetVariable(echo(\"unused\"))"
	if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	actions := p15CompiledActions(t, source)
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	g.get("arg_0", envelope)
	g.emit("gettext", map[string]any{"WFTextActionText": "MUST_NOT_LEAK"})
	g.emit("nothing", nil)
	g.emit("setvariable", map[string]any{"WFVariableName": "candidate"})
	g.output(p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{}))
	g.caller()
	result := g.call(g.payload("echo", p15FunctionString("unused")))
	value, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(value)})
	if err := g.finish(); err != nil {
		t.Fatal(err)
	}
}

func TestP15OptionalImmutablePresentValuesDoNotBecomeNone(t *testing.T) {
	for _, tc := range []struct {
		name, typ, literal, action, key, flag string
		value                                 any
	}{
		{"none", "Text", "none", "", "", "0", nil},
		{"empty", "Text", "\"\"", "gettext", "WFTextActionText", "1", ""},
		{"false", "Bool", "false", "number", "WFNumberActionNumber", "1", float64(0)},
		{"zero", "Number", "0", "number", "WFNumberActionNumber", "1", uint64(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "function echo(seed:Text)->Bool{let candidate:" + tc.typ + "?=" + tc.literal + ";return candidate!=none}\ngetVariable(echo(\"unused\"))"
			if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			g := &p15FunctionGraph{actual: p15CompiledActions(t, source), groups: map[string]bool{}}
			envelope := g.header("echo")
			g.get("arg_0", envelope)
			if tc.action == "" {
				g.emit("nothing", nil)
				g.emit("setvariable", map[string]any{"WFVariableName": "candidate"})
			} else {
				g.value(tc.action, map[string]any{tc.key: tc.value})
			}
			presence := g.value("gettext", map[string]any{"WFTextActionText": tc.flag})
			group := g.conditional(5, p15FunctionEdge(presence), "WFConditionalActionString", "0")
			g.emit("number", map[string]any{"WFNumberActionNumber": float64(1)})
			g.boundary(group, 1, false)
			g.emit("number", map[string]any{"WFNumberActionNumber": float64(0)})
			g.output(g.boundary(group, 2, true))
			g.caller()
			result := g.call(g.payload("echo", p15FunctionString("unused")))
			g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
			if err := g.finish(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
