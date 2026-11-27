package shortcutcompare

import (
	"reflect"
	"testing"
)

// The wrapper selects the same V2 function-envelope parser as a compiled
// function. The new normalization is confined to the body before its matching
// dispatch end; the main branch is independently observable.
func noContentComparatorEnvelope(body, main []any) map[string]any {
	action := func(identifier string, parameters map[string]any) map[string]any {
		return map[string]any{
			"WFWorkflowActionIdentifier": identifier,
			"WFWorkflowActionParameters": parameters,
		}
	}
	control := func(group string, mode int) map[string]any {
		return action("is.workflow.actions.conditional", map[string]any{
			"GroupingIdentifier": group, "WFControlFlowMode": mode,
		})
	}
	outer := control("outer-dispatch", 0)
	outer["WFWorkflowActionParameters"].(map[string]any)["WFInput"] = map[string]any{
		"WFSerializationType": "WFTextTokenAttachment",
		"Value":               map[string]any{"Type": "Variable", "VariableName": "_cherri_is_fn"},
	}
	fn := control("selected-function", 0)
	fn["WFWorkflowActionParameters"].(map[string]any)["WFCondition"] = 4
	fn["WFWorkflowActionParameters"].(map[string]any)["WFConditionalActionString"] = "sample"
	all := []any{
		action("is.workflow.actions.setvariable", map[string]any{"WFVariableName": "_cherri_is_fn"}),
		outer, fn,
	}
	all = append(all, body...)
	all = append(all, control("selected-function", 2), control("outer-dispatch", 1))
	all = append(all, main...)
	all = append(all, control("outer-dispatch", 2))
	return map[string]any{"WFWorkflowActions": all}
}

func noContentComparatorAction(identifier string) map[string]any {
	return map[string]any{"WFWorkflowActionIdentifier": identifier}
}

func noContentComparatorText(value string) map[string]any {
	return map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
		"WFWorkflowActionParameters": map[string]any{"WFTextActionText": value},
	}
}

func TestFunctionNoContentEpilogueNormalization(t *testing.T) {
	baseBody := func() []any { return []any{noContentComparatorText("original body")} }
	pair := func() []any {
		return []any{
			noContentComparatorAction("is.workflow.actions.nothing"),
			noContentComparatorAction("is.workflow.actions.exit"),
		}
	}
	assertComparison := func(t *testing.T, a, b map[string]any, wantEqual bool) {
		t.Helper()
		originalA, originalB := deepCloneMap(a), deepCloneMap(b)
		for direction, documents := range [][2]map[string]any{{a, b}, {b, a}} {
			got := CompareDocumentsNormalized(documents[0], documents[1])
			if got.Equal != wantEqual || (!wantEqual && (len(got.Differences) == 0 || got.Truncated)) {
				t.Fatalf("direction %d: normalized equality=%t, want %t; differences=%+v", direction, got.Equal, wantEqual, got.Differences)
			}
		}
		if !reflect.DeepEqual(a, originalA) || !reflect.DeepEqual(b, originalB) {
			t.Fatal("normalization mutated an original input document")
		}
	}
	for _, explicitEmpty := range []bool{false, true} {
		name := "omitted_parameters"
		if explicitEmpty {
			name = "empty_parameter_maps"
		}
		t.Run(name, func(t *testing.T) {
			tail := pair()
			if explicitEmpty {
				for _, raw := range tail {
					raw.(map[string]any)["WFWorkflowActionParameters"] = map[string]any{}
				}
			}
			assertComparison(t, noContentComparatorEnvelope(baseBody(), nil),
				noContentComparatorEnvelope(append(baseBody(), tail...), nil), true)
		})
	}
	t.Run("unpaired_exit_is_observable", func(t *testing.T) {
		assertComparison(t, noContentComparatorEnvelope(baseBody(), nil),
			noContentComparatorEnvelope(append(baseBody(), noContentComparatorAction("is.workflow.actions.exit")), nil), false)
	})
	t.Run("nonterminal_exit_is_observable", func(t *testing.T) {
		without := append(baseBody(), noContentComparatorText("after exit"))
		with := append(baseBody(), pair()...)
		with = append(with, noContentComparatorText("after exit"))
		assertComparison(t, noContentComparatorEnvelope(without, nil), noContentComparatorEnvelope(with, nil), false)
	})
	t.Run("branch_local_exit_is_observable", func(t *testing.T) {
		branch := func(includeExit bool) []any {
			result := []any{map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
				"WFWorkflowActionParameters": map[string]any{
					"GroupingIdentifier": "body-branch", "WFControlFlowMode": 0,
					"WFCondition": 4, "WFConditionalActionString": "1",
				},
			}}
			if includeExit {
				result = append(result, pair()...)
			}
			return append(result, map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
				"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "body-branch", "WFControlFlowMode": 2},
			})
		}
		assertComparison(t, noContentComparatorEnvelope(branch(false), nil), noContentComparatorEnvelope(branch(true), nil), false)
	})
	t.Run("main_frame_exit_is_observable", func(t *testing.T) {
		assertComparison(t, noContentComparatorEnvelope(baseBody(), nil), noContentComparatorEnvelope(baseBody(), pair()), false)
	})
	t.Run("no_function_envelope_exit_is_observable", func(t *testing.T) {
		assertComparison(t, map[string]any{"WFWorkflowActions": baseBody()},
			map[string]any{"WFWorkflowActions": append(baseBody(), pair()...)}, false)
	})
	t.Run("only_one_terminal_pair_is_removed", func(t *testing.T) {
		two := append(baseBody(), pair()...)
		two = append(two, pair()...)
		assertComparison(t, noContentComparatorEnvelope(baseBody(), nil), noContentComparatorEnvelope(two, nil), false)
	})
	for _, scenario := range []struct {
		name  string
		index int
		value any
	}{
		{"exit_null_parameters", 1, nil},
		{"exit_scalar_parameters", 1, ""},
		{"exit_list_parameters", 1, []any{}},
		{"exit_explicit_input", 1, map[string]any{"WFInput": "observable"}},
		{"exit_explicit_empty_output", 1, map[string]any{"WFOutput": ""}},
		{"nothing_null_parameters", 0, nil},
		{"nothing_scalar_parameters", 0, ""},
		{"nothing_explicit_input", 0, map[string]any{"WFInput": "observable"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			tail := pair()
			tail[scenario.index].(map[string]any)["WFWorkflowActionParameters"] = scenario.value
			assertComparison(t, noContentComparatorEnvelope(baseBody(), nil),
				noContentComparatorEnvelope(append(baseBody(), tail...), nil), false)
		})
	}
	t.Run("extra_action_metadata_is_observable", func(t *testing.T) {
		tail := pair()
		tail[1].(map[string]any)["Enabled"] = false
		assertComparison(t, noContentComparatorEnvelope(baseBody(), nil),
			noContentComparatorEnvelope(append(baseBody(), tail...), nil), false)
	})
	t.Run("value_return_is_preserved", func(t *testing.T) {
		returning := func(value string, epilogue bool) []any {
			result := []any{map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.output",
				"WFWorkflowActionParameters": map[string]any{"WFOutput": value},
			}}
			if epilogue {
				result = append(result, pair()...)
			}
			return result
		}
		assertComparison(t, noContentComparatorEnvelope(returning("value", false), nil),
			noContentComparatorEnvelope(returning("value", true), nil), true)
		assertComparison(t, noContentComparatorEnvelope(returning("value", false), nil),
			noContentComparatorEnvelope(returning("changed", true), nil), false)
	})
	t.Run("body_mutation_is_preserved", func(t *testing.T) {
		changed := append([]any{noContentComparatorText("changed body")}, pair()...)
		assertComparison(t, noContentComparatorEnvelope(baseBody(), nil), noContentComparatorEnvelope(changed, nil), false)
	})
}
