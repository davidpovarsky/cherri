package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"howett.net/plist"
)

func p15CompiledActions(t *testing.T, body string) []map[string]any {
	t.Helper()
	var data, err = CompileSourceToPlist("p15-short-circuit.cherri", body)
	if err != nil {
		t.Fatalf("production compile: %v", err)
	}
	var doc map[string]any
	if _, err = plist.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	var raw = doc["WFWorkflowActions"].([]any)
	var acts = make([]map[string]any, len(raw))
	for i, a := range raw {
		acts[i] = a.(map[string]any)
	}
	return acts
}

func p15Param(a map[string]any) map[string]any {
	return a["WFWorkflowActionParameters"].(map[string]any)
}

func p15Int(v any) int {
	switch n := v.(type) {
	case uint64:
		return int(n)
	case int64:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	}
	return -1
}

// This oracle reads the serialized Apple control tree. It does not call a
// lowering helper or normalize away native key, type, order, or group identity.
func p15GuardedRun(acts []map[string]any, target string, branch int) error {
	type frame struct {
		group string
		mode  int
	}
	var stack []frame
	var count int
	for i, a := range acts {
		var p = p15Param(a)
		switch a["WFWorkflowActionIdentifier"] {
		case "is.workflow.actions.conditional":
			var group, _ = p["GroupingIdentifier"].(string)
			switch p15Int(p["WFControlFlowMode"]) {
			case 0:
				if group == "" {
					return fmt.Errorf("missing group at %d", i)
				}
				stack = append(stack, frame{group: group})
			case 1:
				if len(stack) == 0 || stack[len(stack)-1].group != group {
					return fmt.Errorf("unmatched else at %d", i)
				}
				stack[len(stack)-1].mode = 1
			case 2:
				if len(stack) == 0 || stack[len(stack)-1].group != group {
					return fmt.Errorf("unmatched end at %d", i)
				}
				stack = stack[:len(stack)-1]
			default:
				return fmt.Errorf("invalid control mode at %d", i)
			}
		case "is.workflow.actions.runworkflow":
			if p["WFWorkflowName"] == target {
				count++
				if len(stack) == 0 || stack[len(stack)-1].mode != branch {
					return fmt.Errorf("RHS action at %d is outside lazy branch %d", i, branch)
				}
			}
		}
	}
	if len(stack) != 0 {
		return fmt.Errorf("unclosed groups: %d", len(stack))
	}
	if count != 1 {
		return fmt.Errorf("RHS must appear exactly once, got %d", count)
	}
	return nil
}

func TestP15F01StatementAndExpressionGuardRHS(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		branch    int
	}{
		{"and_statement", "var left = false\nif left && run(\"P15_RHS\", input: true) == true { comment(\"THEN\") }", 0},
		{"or_statement", "var left = true\nif left || run(\"P15_RHS\", input: true) == true { comment(\"THEN\") }", 1},
		{"and_comparison", "var left = false\nif left == true && run(\"P15_RHS\", input: true) == true { comment(\"THEN\") }", 0},
		{"or_comparison", "var left = true\nif left == true || run(\"P15_RHS\", input: true) == true { comment(\"THEN\") }", 1},
		{"and_expression", "var left = false\nlet value = left && (run(\"P15_RHS\", input: true) == true)", 0},
		{"or_expression", "var left = true\nlet value = left || (run(\"P15_RHS\", input: true) == true)", 1},
		{"mixed_statement", "var left = false\nvar next = true\nif left && (next || run(\"P15_RHS\", input: true) == true) { comment(\"THEN\") }", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var acts = p15CompiledActions(t, tc.src)
			if err := p15GuardedRun(acts, "P15_RHS", tc.branch); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F01StaticSkipsMissingTarget(t *testing.T) {
	for _, expr := range []string{
		"false && (run(\"P15_MISSING_ERROR\", input: true) == true)",
		"true || (run(\"P15_MISSING_ERROR\", input: true) == true)",
	} {
		for _, src := range []string{"let value = " + expr, "if " + expr + " { comment(\"THEN\") }"} {
			var acts = p15CompiledActions(t, src)
			for _, a := range acts {
				if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
					t.Fatalf("statically skipped RHS was emitted for %s", src)
				}
			}
		}
	}
}

func TestP15F01BooleanStatementUsesValueNotPresence(t *testing.T) {
	for _, expr := range []string{"flag", "!flag", "flag && other", "flag || other"} {
		var acts = p15CompiledActions(t, "var flag = false\nvar other = true\nif "+expr+" { comment(\"THEN\") } else { comment(\"ELSE\") }")
		p15AssertBooleanConditions(t, acts, expr)
	}
}

func p15AssertBooleanConditions(t *testing.T, acts []map[string]any, expr string) {
	t.Helper()
	var starts int
	for _, a := range acts {
		if a["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		var p = p15Param(a)
		if p15Int(p["WFControlFlowMode"]) != 0 {
			continue
		}
		starts++
		var code = p15Int(p["WFCondition"])
		if code != 4 && code != 5 {
			t.Fatalf("%s uses presence/compound predicate %v for a boolean, expected equality or inequality", expr, p)
		}
		if p15Int(p["WFNumberValue"]) != 1 {
			t.Fatalf("%s does not compare to native true (1): %#v", expr, p)
		}
	}
	if starts == 0 {
		t.Fatalf("missing condition for %s", expr)
	}
}

func TestP15F01GuardOracleRejectsMutations(t *testing.T) {
	var acts = p15CompiledActions(t, "var flag = false\nlet value = flag && (run(\"P15_RHS\", input: true) == true)")
	if err := p15GuardedRun(acts, "P15_RHS", 0); err != nil {
		t.Fatal(err)
	}
	var rhs int
	for i, a := range acts {
		if p15Param(a)["WFWorkflowName"] == "P15_RHS" {
			rhs = i
		}
	}
	var eager = append([]map[string]any{acts[rhs]}, acts[:rhs]...)
	eager = append(eager, acts[rhs+1:]...)
	if err := p15GuardedRun(eager, "P15_RHS", 0); err == nil {
		t.Fatal("eager RHS mutation falsely passed")
	}
	var duplicate = append([]map[string]any{}, acts...)
	duplicate = append(duplicate[:rhs], append([]map[string]any{acts[rhs]}, duplicate[rhs:]...)...)
	if err := p15GuardedRun(duplicate, "P15_RHS", 0); err == nil {
		t.Fatal("duplicate RHS mutation falsely passed")
	}
	var broken = append([]map[string]any{}, acts...)
	for i, a := range broken {
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p15Param(a)["WFControlFlowMode"]) == 2 {
			var clone = map[string]any{}
			for k, v := range a {
				clone[k] = v
			}
			var p = map[string]any{}
			for k, v := range p15Param(a) {
				p[k] = v
			}
			p["GroupingIdentifier"] = "MUTATED"
			clone["WFWorkflowActionParameters"] = p
			broken[i] = clone
			break
		}
	}
	if err := p15GuardedRun(broken, "P15_RHS", 0); err == nil || !strings.Contains(err.Error(), "unmatched") {
		t.Fatalf("broken group mutation: %v", err)
	}
}

func TestP15F01RuntimeFixtureCompiles(t *testing.T) {
	var body, err = os.ReadFile("tests/language-v2/v2-short-circuit-runtime.cherri")
	if err != nil {
		t.Fatal(err)
	}
	var acts = p15CompiledActions(t, string(body))
	if err := p15GuardedRun(acts, "CHERRI_P15_MISSING_AND", 0); err != nil {
		t.Fatal(err)
	}
	if err := p15GuardedRun(acts, "CHERRI_P15_MISSING_OR", 1); err != nil {
		t.Fatal(err)
	}
}
