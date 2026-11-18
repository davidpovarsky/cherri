package main

import (
	"howett.net/plist"
	"testing"
)

// Start from a genuine imported workflow so compiler-wide provenance is active,
// then add an authored condition through the editable source surface.
func TestP15F01HybridEditedPredicateGuardsRHS(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.comment", "WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "imported anchor"}},
	}, plist.XMLFormat)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := RenderHybridV2Source(doc, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, expr := range []string{"flag", "!flag", "flag && other", "flag || other"} {
		acts := p15CompiledActions(t, prefix+"\nvar flag = false\nvar other = true\nif "+expr+" { comment(\"THEN\") } else { comment(\"ELSE\") }\n")
		p15AssertBooleanConditions(t, acts, expr)
	}
	for _, tc := range []struct {
		name, condition string
		branch          int
	}{
		{"and", "flag == true && run(\"P15_RHS\", input: true) == true", 0},
		{"or", "flag == true || run(\"P15_RHS\", input: true) == true", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acts := p15CompiledActions(t, prefix+"\nvar flag = false\nif "+tc.condition+" { comment(\"THEN\") }\n")
			if err := p15GuardedRun(acts, "P15_RHS", tc.branch); err != nil {
				t.Fatal(err)
			}
		})
	}
}
