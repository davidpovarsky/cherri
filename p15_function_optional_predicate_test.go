package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

type p15OptionalPredicateCase struct {
	name, expression, context string
	code                      int
	value                     p15FunctionOptionalCase
}

func p15OptionalPredicateCases() []p15OptionalPredicateCase {
	var cases []p15OptionalPredicateCase
	for _, value := range p15FunctionOptionalCases()[:6] {
		for _, predicate := range []struct {
			name, expression string
			code             int
		}{
			{"equal", "value==none", 4}, {"different", "value!=none", 5},
			{"reverse_equal", "none==value", 4}, {"reverse_different", "none!=value", 5},
			{"negated_equal", "!(value==none)", 5}, {"negated_different", "!(none!=value)", 4},
		} {
			for _, context := range []string{"value", "statement", "yield"} {
				cases = append(cases, p15OptionalPredicateCase{value.name + "/" + predicate.name + "/" + context, predicate.expression, context, predicate.code, value})
			}
		}
	}
	return cases
}

func p15OptionalPredicateSource(tc p15OptionalPredicateCase) string {
	body, result := "return "+tc.expression, "Bool"
	if tc.context == "statement" {
		body, result = "if "+tc.expression+"{return \"MATCH\"}else{return \"OTHER\"}", "Text"
	} else if tc.context == "yield" {
		body, result = "return if "+tc.expression+"{yield \"MATCH\"}else{yield \"OTHER\"}", "Text"
	}
	return "function echo(value:" + tc.value.typ + "?)->" + result + "{" + body + "}\ngetVariable(echo(" + tc.value.argument + "))"
}

// Compare the full independent wire graph. The authored None predicate must
// read the compiler's Text presence flag; native truthiness of the value never
// participates, including false, zero and explicit empty containers.
func p15OptionalPredicateError(actions []map[string]any, tc p15OptionalPredicateCase) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	_, presence := p15DecodeOptionalArgumentGraph(g, envelope, 0)
	predicate := g.conditional(tc.code, p15FunctionEdge(presence), "WFConditionalActionString", "0")
	if tc.context == "value" {
		g.emit("number", map[string]any{"WFNumberActionNumber": float64(1)})
		g.boundary(predicate, 1, false)
		g.emit("number", map[string]any{"WFNumberActionNumber": float64(0)})
		g.output(g.boundary(predicate, 2, true))
	} else {
		matched := g.value("gettext", map[string]any{"WFTextActionText": "MATCH"})
		if tc.context == "statement" {
			g.output(matched)
		}
		g.boundary(predicate, 1, false)
		other := g.value("gettext", map[string]any{"WFTextActionText": "OTHER"})
		if tc.context == "statement" {
			g.output(other)
			g.boundary(predicate, 2, false)
			// Existing non-optional function fallthrough clearing remains intact.
			g.emit("nothing", nil)
			g.emit("output", nil)
		} else {
			g.output(g.boundary(predicate, 2, true))
		}
	}
	g.caller()
	result := g.call(p15FunctionOptionalPayload(g, tc.value))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalNonePredicatesUsePresence(t *testing.T) {
	for _, tc := range p15OptionalPredicateCases() {
		t.Run(tc.name, func(t *testing.T) {
			source := p15OptionalPredicateSource(tc)
			if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			if err := p15OptionalPredicateError(p15CompiledActions(t, source), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15OptionalNonePredicateOracleRejectsCorruption(t *testing.T) {
	tc := p15OptionalPredicateCases()[0]
	actions := p15CompiledActions(t, p15OptionalPredicateSource(tc))
	if err := p15OptionalPredicateError(actions, tc); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"value_instead_of_presence", "truthiness", "reversed_polarity", "changed_absence_token", "invented_flag_edge", "dropped_boolean_branch"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15ContainerCloneNative(t, actions)
			if err := p15OptionalPredicateError(bad, tc); err != nil {
				t.Fatal("unchanged clone must pass", err)
			}
			predicate, decoder := -1, -1
			for i, action := range bad {
				p := p15Param(action)
				if p["WFDictionaryKey"] == "arg_0" {
					decoder = i
				}
				if p["WFConditionalActionString"] == "0" {
					predicate = i
				}
			}
			if predicate < 0 || decoder < 0 {
				t.Fatal("mutation target missing")
			}
			p := p15Param(bad[predicate])
			switch fault {
			case "value_instead_of_presence":
				p["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15FunctionEdge(p15Param(bad[decoder])["UUID"].(string))}
			case "truthiness":
				p["WFCondition"] = uint64(101)
			case "reversed_polarity":
				p["WFCondition"] = uint64(5)
			case "changed_absence_token":
				p["WFConditionalActionString"] = "1"
			case "invented_flag_edge":
				p["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15FunctionEdge("missing-presence")}
			case "dropped_boolean_branch":
				bad = append(bad[:predicate+1], bad[predicate+2:]...)
			}
			if p15OptionalPredicateError(bad, tc) == nil {
				t.Fatal("damaged semantic presence predicate accepted")
			}
		})
	}
}

func p15OptionalCallPredicateError(actions []map[string]any, tc p15FunctionOptionalCase, code int) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	value, presence := p15DecodeOptionalArgumentGraph(g, envelope, 0)
	resultGroup := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionArgument{true, 0, p15FunctionEdge(value)})
	g.boundary(resultGroup, 1, false)
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
	g.output(g.boundary(resultGroup, 2, true))
	g.caller()
	result := g.call(p15FunctionOptionalPayload(g, tc))
	_, resultPresence := p15DecodeOptionalGraph(g, result, "has_value", "value")
	group := g.conditional(code, p15FunctionEdge(resultPresence), "WFConditionalActionString", "0")
	g.emit("number", map[string]any{"WFNumberActionNumber": float64(1)})
	g.boundary(group, 1, false)
	g.emit("number", map[string]any{"WFNumberActionNumber": float64(0)})
	boolean := g.boundary(group, 2, true)
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(boolean)})
	return g.finish()
}

func TestP15OptionalCallResultPredicateEvaluatesOnce(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases()[:2] {
		for _, comparison := range []struct {
			label, op string
			code      int
			reverse   bool
		}{
			{"equal", "==", 4, false}, {"different", "!=", 5, false},
			{"reverse_equal", "==", 4, true}, {"reverse_different", "!=", 5, true},
		} {
			t.Run(tc.name+"/"+comparison.label, func(t *testing.T) {
				call := "echo(" + tc.argument + ")"
				expression := call + comparison.op + "none"
				if comparison.reverse {
					expression = "none" + comparison.op + call
				}
				source := "function echo(value:Text?)->Text?{return value}\ngetVariable(" + expression + ")"
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

func p15OptionalFallthroughError(actions []map[string]any, branch bool) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	seed := g.get("arg_0", envelope)
	if branch {
		// The declared Text parameter is decoded from this function's actual
		// arg_0 producer. N03 owns the explicit Text snapshots and Boolean
		// comparison, followed by the authored return branch.
		region, err := p15N03ExactNativeRegionAt(actions, len(g.expected))
		if err != nil {
			return err
		}
		left := map[string]any{"Type": "ActionOutput", "OutputUUID": seed, "OutputName": "seed"}
		if region.Operator != 4 || !reflect.DeepEqual(region.Left, left) || region.Right != "RETURN" || len(region.Result) != 2 {
			return fmt.Errorf("optional return exact Text operand/operator/result differs")
		}
		inner := p15Param(actions[region.InnerStart])["GroupingIdentifier"].(string)
		for group := range g.groups {
			if strings.EqualFold(group, inner) {
				return fmt.Errorf("optional return reuses an exact Text condition group")
			}
		}
		g.groups[inner] = true
		// Only a fully independently validated region is composed into the
		// surrounding function oracle. No unvalidated operation is adopted.
		for _, action := range actions[region.Start:region.Next] {
			g.expected = append(g.expected, p15SCCopy(action).(map[string]any))
		}
		if _, err := p15N03ExactResultGuard(actions, region, region.Next); err != nil {
			return err
		}
		group := g.conditional(4, p15FunctionEdge(region.Result["OutputUUID"].(string)), "WFNumberValue", float64(1))
		g.output(p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionString("")))
		g.boundary(group, 2, false)
	}
	g.emit("gettext", map[string]any{"WFTextActionText": "MUST_NOT_LEAK"})
	g.output(p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{}))
	g.caller()
	result := g.call(g.payload("echo", p15FunctionString("unused")))
	value, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(value)})
	return g.finish()
}

func TestP15OptionalReturnCompleteness(t *testing.T) {
	t.Run("bare_return_is_void", func(t *testing.T) {
		source := "function echo(seed:Text)->Text?{return}\ngetVariable(echo(\"unused\"))"
		diagnostics := p15TypeDiagnostics(t, source)
		if len(diagnostics) != 1 || diagnostics[0].Code != analysis.CodeReturnType || !strings.Contains(diagnostics[0].Message, "Void") {
			t.Fatal("bare return must remain a Void type error", diagnostics)
		}
		if raw, err := CompileSourceToPlist("p15-optional-bare.cherri", source); err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeReturnType) {
			t.Fatalf("bare typed return emitted bytes=%d error=%v", len(raw), err)
		}
	})
	for _, branch := range []bool{false, true} {
		t.Run(fmt.Sprintf("implicit_none/branch_%v", branch), func(t *testing.T) {
			body := "text(\"MUST_NOT_LEAK\")"
			if branch {
				body = "if seed==\"RETURN\"{return \"\"};" + body
			}
			source := "function echo(seed:Text)->Text?{" + body + "}\ngetVariable(echo(\"unused\"))"
			if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			actions := p15CompiledActions(t, source)
			if err := p15OptionalFallthroughError(actions, branch); err != nil {
				t.Fatal(err)
			}
			faults := []string{"changed_presence", "invented_empty_value", "lost_carrier", "stale_output", "missing_decoder"}
			if branch {
				faults = append(faults, "lost_text_coercion", "wrong_text_operand", "inverted_text_operator", "detached_text_guard")
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, actions)
					if err := p15OptionalFallthroughError(bad, branch); err != nil {
						t.Fatal("unchanged clone must pass", err)
					}
					carrier, output, decoder := -1, -1, -1
					var fields map[string]map[string]any
					for i, action := range bad {
						p := p15Param(action)
						if candidate, err := p15ContainerFields(p["WFItems"]); err == nil && candidate["_cherri_optional_result_v1"] != nil && reflect.DeepEqual(candidate["has_value"]["WFValue"], p15FunctionText("0")) {
							carrier = i
							fields = candidate
						}
						if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.output" {
							output = i
						}
						if p["WFDictionaryKey"] == "has_value" {
							decoder = i
						}
					}
					if fields == nil || carrier < 0 || output < 0 || decoder < 0 {
						t.Fatal("mutation target missing")
					}
					switch fault {
					case "changed_presence":
						fields["has_value"]["WFValue"] = p15FunctionText("1")
					case "invented_empty_value":
						fields["has_value"]["WFKey"] = p15FunctionText("value")
						fields["has_value"]["WFValue"] = p15FunctionText("")
					case "lost_carrier":
						bad[carrier]["WFWorkflowActionIdentifier"] = "is.workflow.actions.nothing"
					case "stale_output":
						p15Param(bad[output])["WFOutput"] = p15FunctionText("MUST_NOT_LEAK")
					case "missing_decoder":
						p15Param(bad[decoder])["WFDictionaryKey"] = "value"
					case "lost_text_coercion", "wrong_text_operand", "inverted_text_operator", "detached_text_guard":
						regions, err := p15N03ExactNativeRegions(bad, 1)
						if err != nil {
							t.Fatal("positive exact Text mutation target missing", err)
						}
						region := regions[0]
						switch fault {
						case "lost_text_coercion":
							// Preserve the historical control identity: remove the
							// explicit Text carrier now owned by the snapshot.
							p15Param(bad[region.LeftSnapshot])["WFTextActionText"] = region.Left
						case "wrong_text_operand":
							wrong := map[string]any{"Type": "ActionOutput", "OutputUUID": p15Param(bad[0])["UUID"], "OutputName": "seed"}
							p15Param(bad[region.LeftSnapshot])["WFTextActionText"] = p15N03AffixedTokenString("~", wrong, "")
						case "inverted_text_operator":
							p15Param(bad[region.InnerStart])["WFCondition"] = uint64(5)
						case "detached_text_guard":
							p15Param(bad[region.Next])["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15FunctionEdge(p15Param(bad[0])["UUID"].(string))}
						}
					}
					if p15OptionalFallthroughError(bad, branch) == nil {
						t.Fatal("damaged implicit None accepted")
					}
				})
			}
		})
	}
}
