package main

import (
	"fmt"
	"testing"
)

func TestP15S01OrdinaryLiteralContainerPlaceholderSafety(t *testing.T) {
	cases := []struct{ name, source string }{
		{"list_immutable", "let value=[\"L￼🙂éR\"]\ngetVariable(value)\n"},
		{"list_mutable", "var value=[\"L￼🙂éR\"]\ngetVariable(value)\n"},
		{"map_immutable", "let value={\"key\":\"L￼🙂éR\"}\ngetVariable(value)\n"},
		{"map_mutable", "var value={\"key\":\"L￼🙂éR\"}\ngetVariable(value)\n"},
		{"map_key", "let value={\"K￼K\":\"value\"}\ngetVariable(value)\n"},
		{"map_nested_list", "let value={\"nested\":[\"L￼🙂éR\"]}\ngetVariable(value)\n"},
		{"list_nested_map", "let value=[{\"key\":\"L￼🙂éR\"}]\ngetVariable(value)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			for index, action := range actions {
				if err := p15S01CompletePlaceholderCoverage(action); err != nil {
					t.Fatalf("ordinary literal field at action %d is unsafe: %v", index, err)
				}
			}
		})
	}
}

func p15S01EnclosingInputError(actions []map[string]any) error {
	seed, target := -1, -1
	for index, action := range actions {
		if p15Param(action)["WFTextActionText"] == "S01_ENCLOSING_SEED" {
			seed = index
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setitemname" {
			target = index
		}
	}
	if seed < 0 || target <= seed {
		return fmt.Errorf("missing explicit public seed or enclosing native action")
	}
	uuid := p15Param(actions[seed])["UUID"]
	check := func(value any) bool {
		ref := p15DiagnosticValue(value)
		return ref["Type"] == "ActionOutput" && ref["OutputUUID"] == uuid
	}
	if value, present := p15Param(actions[target])["WFInput"]; present {
		if !check(value) {
			return fmt.Errorf("enclosing native input is not the original seed")
		}
		return nil
	}
	if target == seed+1 {
		return nil
	}
	previous := actions[target-1]
	if previous["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && check(p15Param(previous)["WFVariable"]) {
		return nil
	}
	return fmt.Errorf("pure literal helper replaced enclosing native implicit input: seed=%d target=%d previous=%v", seed, target, previous["WFWorkflowActionIdentifier"])
}

func TestP15S01EnclosingNativeImplicitInputIsPreserved(t *testing.T) {
	cases := []struct{ name, source string }{
		{"literal_implicit", "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"￼name\"})\n"},
		{"literal_explicit_control", "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFInput\":seed,\"WFName\":f\"￼name\"})\n"},
		{"interpolated_implicit", "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"￼{suffix}\"})\n"},
		{"interpolated_explicit_control", "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFInput\":seed,\"WFName\":f\"￼{suffix}\"})\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15S01EnclosingInputError(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}
