package main

import (
	"fmt"
	"strings"
	"testing"
)

func p15M02MainVoidBoundaryError(actions []map[string]any) error {
	if len(actions) == 0 {
		return fmt.Errorf("main fallthrough has no explicit empty result")
	}
	last := actions[len(actions)-1]
	if last["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" {
		return fmt.Errorf("main fallthrough does not clear unused native output: final=%#v", last)
	}
	for key := range last {
		if key != "WFWorkflowActionIdentifier" && key != "WFWorkflowActionParameters" {
			return fmt.Errorf("main empty result acquired an unexpected envelope field %s", key)
		}
	}
	if raw, present := last["WFWorkflowActionParameters"]; present {
		parameters, okay := raw.(map[string]any)
		if !okay || len(parameters) != 0 {
			return fmt.Errorf("main empty result has nonempty or malformed parameters: %#v", raw)
		}
	}
	return nil
}

func TestP15M02MainVoidFallthrough(t *testing.T) {
	cases := []struct{ name, source string }{
		{"empty", ""},
		{"unused_text", "text(\"M02_UNUSED_TEXT\")"},
		{"unused_binding", "let unused=number(37)"},
		{"void_action_after_value", "text(\"M02_UNUSED_BEFORE_VOID\")\nwait(0)"},
		{"partial_early_return", "let flag=false\nif flag { return \"M02_RETURNED\" }\ntext(\"M02_UNUSED_FALLTHROUGH\")"},
		{"function_caller_fallthrough", "function value()->Text{return \"M02_CHILD\"}\nlet unused=value()\ntext(\"M02_UNUSED_PARENT\")"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actions, _ := p15NoneActions(t, tc.source)
			if err := p15M02MainVoidBoundaryError(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15M02ExplicitMainReturnBaseline(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"text", "text(\"M02_NOISE\")\nreturn \"M02_EXPLICIT\""},
		{"empty_text", "text(\"M02_NOISE\")\nreturn \"\""},
		{"false", "text(\"M02_NOISE\")\nreturn false"},
		{"zero", "text(\"M02_NOISE\")\nreturn 0"},
		{"bare", "text(\"M02_NOISE\")\nreturn"},
		{"none", "text(\"M02_NOISE\")\nreturn none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions, _ := p15NoneActions(t, tc.source)
			if len(actions) < 2 {
				t.Fatal("missing actual main return")
			}
			last := actions[len(actions)-1]
			noContent := tc.name == "bare" || tc.name == "none"
			terminal := "is.workflow.actions.output"
			if noContent {
				terminal = "is.workflow.actions.exit"
			}
			if last["WFWorkflowActionIdentifier"] != terminal {
				t.Fatal("main return has the wrong value-presence terminal", last)
			}
			if noContent {
				if len(p15Param(last)) != 0 || actions[len(actions)-2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" {
					t.Fatal("bare/None return can leak preceding output")
				}
			} else {
				if _, ok := p15Param(last)["WFOutput"]; !ok {
					t.Fatal("present return has no explicit output")
				}
				if strings.Contains(fmt.Sprint(p15Param(last)["WFOutput"]), "M02_NOISE") {
					t.Fatal("return uses preceding incidental text")
				}
			}
		})
	}
}
