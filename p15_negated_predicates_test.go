package main

import (
	"howett.net/plist"
	"testing"
)

func p15AssertNoComputedPresence(t *testing.T, value any) {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		if code, ok := v["WFCondition"]; ok && (p15Int(code) == 100 || p15Int(code) == 101) {
			t.Errorf("computed Boolean was lowered as native presence predicate %v", code)
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
		`left == "left" && !(right beginsWith "x")`,
		`left == "left" || !(right endsWith "x")`,
		`left == "left" && !isTrue(flag)`,
		`left == "left" && !isFalse(flag)`,
		`!isToday(CurrentDate)`,
		`!isBetween(n, 1, 3)`,
	} {
		for _, mode := range []string{"", "modern: "} {
			t.Run(mode+condition, func(t *testing.T) {
				source := "let left = text(\"left\")\nlet right = text(\"right\")\nvar flag = false\nlet n = number(2)\nif " + mode + condition + " { comment(\"THEN\") }"
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
