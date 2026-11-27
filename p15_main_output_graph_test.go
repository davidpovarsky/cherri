package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

type p15M02GraphCase struct {
	name, source string
	model        func(*p15FunctionGraph)
}

// These complete expected graphs name every native operation and consumer.
// Only native UUID/group alpha names come from the compiled document. The
// public compiler helper returns the full document without deleting boundaries.
func p15M02GraphCases() []p15M02GraphCase {
	text := func(g *p15FunctionGraph, value string) string {
		return g.value("gettext", map[string]any{"WFTextActionText": value})
	}
	clear := func(g *p15FunctionGraph) { g.emit("nothing", nil) }
	cases := []p15M02GraphCase{
		{"empty_input_boundary", "", clear},
		{"unused_text", "text(\"M02_UNUSED\")", func(g *p15FunctionGraph) {
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_UNUSED"})
			clear(g)
		}},
		{"unused_number_binding", "let unused=37", func(g *p15FunctionGraph) {
			g.value("number", map[string]any{"WFNumberActionNumber": 37})
			clear(g)
		}},
		{"void_action_after_value", "text(\"M02_BEFORE_VOID\")\nwait(0)", func(g *p15FunctionGraph) {
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_BEFORE_VOID"})
			g.emit("delay", map[string]any{"WFDelayTime": 0})
			clear(g)
		}},
		{"partial_early_return", "let flag=0\nif flag == 1 { return \"M02_EARLY\" }\ntext(\"M02_FALLTHROUGH\")", func(g *p15FunctionGraph) {
			flag := g.value("number", map[string]any{"WFNumberActionNumber": 0})
			group := g.conditional(4, p15FunctionEdge(flag), "WFNumberValue", 1)
			g.output(text(g, "M02_EARLY"))
			g.boundary(group, 2, false)
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_FALLTHROUGH"})
			clear(g)
		}},
		{"both_branches_return", "let flag=1\nif flag == 1 { return \"M02_THEN\" } else { return \"M02_ELSE\" }", func(g *p15FunctionGraph) {
			flag := g.value("number", map[string]any{"WFNumberActionNumber": 1})
			group := g.conditional(4, p15FunctionEdge(flag), "WFNumberValue", 1)
			g.output(text(g, "M02_THEN"))
			g.boundary(group, 1, false)
			g.output(text(g, "M02_ELSE"))
			g.boundary(group, 2, false)
		}},
		{"zero_iteration_return", "repeat 0 { return \"M02_LOOP\" }", func(g *p15FunctionGraph) {
			group := p15M02LoopStart(g, 0)
			g.output(text(g, "M02_LOOP"))
			g.emit("repeat.count", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 2})
			clear(g)
		}},
		{"child_return_parent_falls_through", "function child()->Text { return \"M02_CHILD\" }\nlet unused=child()\ntext(\"M02_PARENT\")", func(g *p15FunctionGraph) {
			g.header("child")
			g.output(text(g, "M02_CHILD"))
			g.caller()
			g.call(g.payload("child"))
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_PARENT"})
			g.boundary(g.outer, 2, false)
			clear(g)
		}},
		{"child_bare_return_parent_continues", "function child()->Void { text(\"M02_CHILD_NOISE\");return }\nchild()\ntext(\"M02_PARENT_CONTINUED\")", func(g *p15FunctionGraph) {
			g.header("child")
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_CHILD_NOISE"})
			clear(g)
			g.emit("exit", nil)
			g.caller()
			g.call(g.payload("child"))
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_PARENT_CONTINUED"})
			g.boundary(g.outer, 2, false)
			clear(g)
		}},
		{"parent_explicit_return_after_child", "function child()->Text { return \"M02_CHILD\" }\nlet result=child()\nreturn result", func(g *p15FunctionGraph) {
			g.header("child")
			g.output(text(g, "M02_CHILD"))
			g.caller()
			g.output(g.call(g.payload("child")))
			g.boundary(g.outer, 2, false)
		}},
		{"raw_nothing_then_resolved_text", "native.action(\"is.workflow.actions.nothing\", parameters: native.plist({}))\ntext(\"M02_AFTER_NOTHING\")", func(g *p15FunctionGraph) {
			g.emit("nothing", nil)
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_AFTER_NOTHING"})
			clear(g)
		}},
		{"native_word_is_only_text", "text(\"native.workflow outputMode legacy\")", func(g *p15FunctionGraph) {
			g.emit("gettext", map[string]any{"WFTextActionText": "native.workflow outputMode legacy"})
			clear(g)
		}},
		{"legacy_mode_preserves_incidental_result", "shortcut \"\" { outputMode: \"legacy\" }\ntext(\"M02_LEGACY\")", func(g *p15FunctionGraph) {
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_LEGACY"})
		}},
		{"authored_native_output", "let result=text(\"M02_NATIVE_OUTPUT\")\noutput(result)", func(g *p15FunctionGraph) {
			g.output(text(g, "M02_NATIVE_OUTPUT"))
		}},
		{"authored_native_stop", "text(\"M02_BEFORE_STOP\")\nstop()", func(g *p15FunctionGraph) {
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_BEFORE_STOP"})
			g.emit("exit", nil)
		}},
		{"explicit_mode_clears_incidental_result", "shortcut \"\" { outputMode: \"explicit\" }\ntext(\"M02_EXPLICIT_MODE\")", func(g *p15FunctionGraph) {
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_EXPLICIT_MODE"})
			clear(g)
		}},
	}
	for _, tc := range []struct {
		name, expression, kind, key string
		value                       any
	}{
		{"text", "\"M02_RETURNED\"", "gettext", "WFTextActionText", "M02_RETURNED"},
		{"empty_text", "\"\"", "gettext", "WFTextActionText", ""},
		{"false", "false", "number", "WFNumberActionNumber", 0},
		{"zero", "0", "number", "WFNumberActionNumber", 0},
	} {
		tc := tc
		cases = append(cases, p15M02GraphCase{"return_" + tc.name, "let noise=text(\"M02_NOISE\")\nshow(noise)\nreturn " + tc.expression, func(g *p15FunctionGraph) {
			noise := g.value("gettext", map[string]any{"WFTextActionText": "M02_NOISE"})
			g.emit("showresult", map[string]any{"Text": p15FunctionText("\ufffc", noise)})
			g.output(g.value(tc.kind, map[string]any{tc.key: tc.value}))
		}})
	}
	for _, expression := range []string{"", "none"} {
		expression := expression
		cases = append(cases, p15M02GraphCase{"return_empty_" + expression, "text(\"M02_NOISE\")\nreturn " + expression, func(g *p15FunctionGraph) {
			g.emit("gettext", map[string]any{"WFTextActionText": "M02_NOISE"})
			clear(g)
			g.emit("exit", nil)
		}})
	}
	return cases
}

func p15M02LoopStart(g *p15FunctionGraph, count int) string {
	i := len(g.expected)
	group := ""
	if i < len(g.actual) {
		group, _ = p15Param(g.actual[i])["GroupingIdentifier"].(string)
	}
	if group == "" || g.groups[group] {
		g.err = fmt.Errorf("missing or duplicate loop group")
	}
	g.groups[group] = true
	g.emit("repeat.count", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 0, "WFRepeatCount": count})
	return group
}

func p15M02GraphError(actions []map[string]any, tc p15M02GraphCase) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}, workflowName: "p15-none-clear"}
	tc.model(g)
	return g.compare()
}

func TestP15M02CompleteMainGraphs(t *testing.T) {
	for _, tc := range p15M02GraphCases() {
		t.Run(tc.name, func(t *testing.T) {
			actions, raw := p15NoneActions(t, tc.source)
			if err := p15M02GraphError(actions, tc); err != nil {
				t.Fatal(err)
			}
			var workflow map[string]any
			if _, err := plist.Unmarshal(raw, &workflow); err != nil {
				t.Fatal(err)
			}
			if _, present := workflow["outputMode"]; present {
				t.Fatal("compiler entry mode escaped into native workflow metadata")
			}
		})
	}
}

func TestP15M02CompleteGraphCausalControls(t *testing.T) {
	// Every positive case executes an independent delete, append and parameter
	// corruption. Explicit return cases additionally change the consumed value
	// and substitute the preceding producer. Whole graphs must reject each.
	for _, tc := range p15M02GraphCases() {
		t.Run(tc.name, func(t *testing.T) {
			actions, _ := p15NoneActions(t, tc.source)
			if err := p15M02GraphError(actions, tc); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"delete_terminal", "append_stale", "alter_first_parameter"} {
				t.Run(mutation, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, actions)
					if err := p15M02GraphError(bad, tc); err != nil {
						t.Fatal("unchanged clone", err)
					}
					switch mutation {
					case "delete_terminal":
						bad = bad[:len(bad)-1]
					case "append_stale":
						bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "M02_STALE"}})
					case "alter_first_parameter":
						params, ok := bad[0]["WFWorkflowActionParameters"].(map[string]any)
						if !ok {
							params = map[string]any{}
							bad[0]["WFWorkflowActionParameters"] = params
						}
						params["M02_UNEXPECTED_NATIVE_PARAMETER"] = false
					}
					if p15M02GraphError(bad, tc) == nil {
						t.Fatal("accepted actual graph corruption", mutation)
					}
				})
			}
			if !strings.HasPrefix(tc.name, "return_") || tc.name == "return_empty_" || tc.name == "return_empty_none" {
				return
			}
			for _, mutation := range []string{"return_value", "return_stale_producer", "return_missing_output"} {
				t.Run(mutation, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, actions)
					switch mutation {
					case "return_value":
						p := p15Param(bad[len(bad)-2])
						if _, ok := p["WFTextActionText"]; ok {
							p["WFTextActionText"] = "M02_WRONG"
						} else {
							p["WFNumberActionNumber"] = 9
						}
					case "return_stale_producer":
						uuid, ok := p15Param(bad[0])["UUID"].(string)
						if !ok || uuid == "" {
							t.Fatal("missing real preceding producer")
						}
						p15Param(bad[len(bad)-1])["WFOutput"] = p15FunctionText("\ufffc", uuid)
					case "return_missing_output":
						delete(p15Param(bad[len(bad)-1]), "WFOutput")
					}
					if p15M02GraphError(bad, tc) == nil {
						t.Fatal("accepted return corruption", mutation)
					}
				})
			}
		})
	}
}

func TestP15M02ExactNativeWorkflowBoundary(t *testing.T) {
	for _, actions := range [][]any{
		{},
		{map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "M02_NATIVE_RESULT", "UUID": "11111111-1111-4111-8111-111111111111"}}},
	} {
		t.Run(fmt.Sprintf("actions_%d", len(actions)), func(t *testing.T) {
			original := map[string]any{"WFWorkflowName": "M02_NATIVE", "WFWorkflowActions": actions, "M02UnknownField": map[string]any{"empty": "", "false": false, "zero": 0}}
			data, err := plist.Marshal(original, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			code := "native.workflow(native.plist(" + formatCherriDoubleQuotedString(string(data)) + "), exact: true)\n"
			_, raw := p15NoneActions(t, code)
			var want, got map[string]any
			if _, err := plist.Unmarshal(data, &want); err != nil {
				t.Fatal(err)
			}
			if _, err := plist.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("full-native boundary changed exact original workflow", got, want)
			}
		})
	}
}

func TestP15M02MigratedEntryOutput(t *testing.T) {
	for _, code := range []string{"", "text(\"M02_MIGRATED\")", "#define name M02_MIGRATED_NAME\ntext(\"M02_MIGRATED\")"} {
		t.Run(fmt.Sprintf("source_%d", len(code)), func(t *testing.T) {
			converted, diagnostics, err := migrate.MigrateSourceWithDiagnostics(code)
			if err != nil || len(diagnostics) != 0 {
				t.Fatal(err, diagnostics)
			}
			if strings.Count(converted, "outputMode: \"legacy\"") != 1 {
				t.Fatal("migration did not explicitly preserve entry output", converted)
			}
			actions, raw := p15NoneActions(t, converted)
			want := 0
			if code != "" {
				want = 1
			}
			if len(actions) != want {
				t.Fatal("migration inserted or removed native actions", actions)
			}
			if want == 1 && (actions[0]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || p15Param(actions[0])["WFTextActionText"] != "M02_MIGRATED") {
				t.Fatal("migrated native output changed", actions)
			}
			var document map[string]any
			if _, err := plist.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			if _, exists := document["outputMode"]; exists {
				t.Fatal("entry compiler metadata escaped into plist")
			}
			if strings.Contains(code, "#define name") {
				program := p15M02Parse(t, converted)
				header, ok := program.Declarations[0].(*syntax.ShortcutDecl)
				if !ok || header.Name != "M02_MIGRATED_NAME" {
					t.Fatal("migration changed the existing declared name")
				}
				workflow, lowerErr := lower.NewLowerer(schema.DefaultRegistry()).LowerProgram(program)
				if lowerErr != nil || workflow.Name != "M02_MIGRATED_NAME" {
					t.Fatal("checked lowering lost the existing workflow name", lowerErr)
				}
			}
			// Ensure the whole document is serializable without hidden Go values.
			if _, err := json.Marshal(document); err != nil {
				t.Fatal(err)
			}
		})
	}
}
