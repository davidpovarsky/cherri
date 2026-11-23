package main

import (
	"os"
	"testing"

	"howett.net/plist"
)

func p15AssertNoComputedPresence(t *testing.T, value any) {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		if code, ok := v["WFCondition"]; ok {
			switch p15Int(code) {
			case 100, 101:
				t.Errorf("computed Boolean was lowered as native presence predicate %v", code)
			case 4, 5:
				_, number := v["WFNumberValue"]
				_, text := v["WFConditionalActionString"]
				_, enumeration := v["WFEnumeration"]
				if !number && !text && !enumeration {
					t.Errorf("authored comparison %v omitted its value operand", code)
				}
			}
		}
		for _, child := range v {
			p15AssertNoComputedPresence(t, child)
		}
	case []any:
		for _, child := range v {
			p15AssertNoComputedPresence(t, child)
		}
	}
}

func TestP15F01NegatedComputedPredicatesUseBooleanValues(t *testing.T) {
	for _, condition := range []string{
		`isTrue(flag)`,
		`isFalse(flag)`,
		`!isTrue(truth)`,
		`!isFalse(flag)`,
		`isTrue(flag) && isFalse(truth)`,
		`isFalse(flag) || isTrue(truth)`,
		`left == "left" && !(right beginsWith "x")`,
		`left == "left" || !(right endsWith "x")`,
		`left == "left" && !isTrue(flag)`,
		`left == "left" && !isFalse(flag)`,
		`!isToday(CurrentDate)`,
		`!isBetween(n, 1, 3)`,
	} {
		for _, mode := range []string{"", "modern: "} {
			t.Run(mode+condition, func(t *testing.T) {
				source := "let left = text(\"left\")\nlet right = text(\"right\")\nvar flag = false\nvar truth = true\nlet n = number(2)\nif " + mode + condition + " { comment(\"THEN\") }"
				acts := p15CompiledActions(t, source)
				for _, act := range acts {
					p15AssertNoComputedPresence(t, p15Param(act))
				}
			})
		}
	}
}

func TestP15F01PredicateValuesRemainTypedAndLazy(t *testing.T) {
	for _, expr := range []string{
		`isTrue(false)`, `isFalse(true)`, `isToday(CurrentDate)`, `isBetween(2, 1, 3)`, `between(2, 1, 3)`,
	} {
		acts := p15CompiledActions(t, "let result: Bool = "+expr+"\nif result { comment(\"TRUE\") }")
		for _, act := range acts {
			p15AssertNoComputedPresence(t, p15Param(act))
		}
	}
	for _, expr := range []string{
		`!isTrue(run("P15_RHS", input: true))`,
		`!isFalse(run("P15_RHS", input: true))`,
		`isBetween(run("P15_RHS", input: true), 1, 3)`,
		`!isToday(run("P15_RHS", input: true))`,
	} {
		for _, logical := range []struct {
			op     string
			branch int
		}{{"&&", 0}, {"||", 1}} {
			acts := p15CompiledActions(t, "var flag = false\nif flag == true "+logical.op+" "+expr+" { comment(\"THEN\") }")
			if err := p15GuardedRun(acts, "P15_RHS", logical.branch); err != nil {
				t.Errorf("%s %s: %v", logical.op, expr, err)
			}
		}
	}
}

func TestP15F01NegatedAuthoredPredicatesInImportedWorkflow(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
		"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "synthetic imported anchor"},
	}}, plist.XMLFormat)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := RenderHybridV2Source(doc, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, condition := range []string{
		`left == "left" && !(right beginsWith "x")`,
		`left == "left" && !isTrue(flag)`,
		`left == "left" || !isFalse(flag)`,
	} {
		acts := p15CompiledActions(t, prefix+"\nlet left=text(\"left\")\nlet right=text(\"right\")\nvar flag=false\nif "+condition+" { comment(\"THEN\") }")
		for _, act := range acts {
			p15AssertNoComputedPresence(t, p15Param(act))
		}
	}
}

// The unchanged original native fixture must retain an explicit comparison
// operand inside each computed Boolean helper, as well as its outer negation.
func TestP15F01FullRuntimeFixtureBooleanComparisons(t *testing.T) {
	body, err := os.ReadFile("tests/language-v2/v2-short-circuit-runtime.cherri")
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range p15CompiledActions(t, string(body)) {
		p15AssertNoComputedPresence(t, p15Param(action))
	}
}
