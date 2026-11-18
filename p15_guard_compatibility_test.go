package main

import (
	"fmt"
	"reflect"
	"testing"

	"howett.net/plist"
)

func p15ConditionalStarts(actions []map[string]any) []map[string]any {
	var starts []map[string]any
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p15Param(action)["WFControlFlowMode"]) == 0 {
			starts = append(starts, p15Param(action))
		}
	}
	return starts
}

// Compare the serialized predicate with an independently authored ordinary If.
// Generated grouping/output identities are deliberately outside the predicate;
// the input is compared too whenever both sources use the same named variable.
func p15SameNativePredicate(got, want map[string]any, compareInput bool) error {
	project := func(p map[string]any) map[string]any {
		result := map[string]any{}
		for k, value := range p {
			switch k {
			case "GroupingIdentifier", "UUID", "CustomOutputName", "WFControlFlowMode":
				continue
			case "WFInput":
				if !compareInput {
					continue
				}
			}
			result[k] = value
		}
		return result
	}
	if !reflect.DeepEqual(project(got), project(want)) {
		return fmt.Errorf("native predicate differs: got %#v, authored If %#v", project(got), project(want))
	}
	if got["WFInput"] == nil {
		return fmt.Errorf("generated guard has no native input")
	}
	return nil
}

func TestP15GeneratedGuardsMatchAuthoredNativeIf(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, expression, reference string
	}{
		{"comparison", "var value = number(1)", "value == 1", "value == 1"},
		{"not", "var value: Bool = true", "!value", "value == true"},
		{"and", "var value: Bool = true", "value && true", "value == true"},
		{"or", "var value: Bool = false", "value || false", "value == true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reference := p15ConditionalStarts(p15CompiledActions(t, tc.prefix+"\nif "+tc.reference+" { comment(\"reference\") }"))
			generated := p15ConditionalStarts(p15CompiledActions(t, tc.prefix+"\nlet result = "+tc.expression+"\nquicklook(result)"))
			if len(reference) != 1 || len(generated) != 1 {
				t.Fatalf("expected one authored/generated start, got %d/%d", len(reference), len(generated))
			}
			if err := p15SameNativePredicate(generated[0], reference[0], true); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15GeneratedBoundsGuardsMatchAuthoredNativeIf(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		values           []int
	}{
		{"decimal_literal_index", "items[0.0]", []int{1}},
		{"integer_literal_index", "items[0]", []int{1, 0}},
		{"dynamic_index", "items[index]", []int{1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, "var items = [\"first\", \"second\"]\nvar index = number(0)\nlet result = "+tc.expression+"\nquicklook(result)")
			starts := p15ConditionalStarts(actions)
			if len(starts) != len(tc.values) {
				t.Fatalf("bounds starts: got %d, want %d", len(starts), len(tc.values))
			}
			for i, value := range tc.values {
				reference := p15ConditionalStarts(p15CompiledActions(t, fmt.Sprintf("var input=number(2)\nif input >= %d { comment(\"reference\") }", value)))
				if len(reference) != 1 {
					t.Fatal("missing authored bounds reference")
				}
				if err := p15SameNativePredicate(starts[i], reference[0], false); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestP15GuardPredicateOracleRejectsMutations(t *testing.T) {
	reference := p15ConditionalStarts(p15CompiledActions(t, "var value=number(1)\nif value == 1 { comment(\"reference\") }"))[0]
	for _, mutation := range []string{"missing_compatibility", "wrong_comparison", "wrong_number", "missing_input"} {
		broken := map[string]any{}
		for key, value := range reference {
			broken[key] = value
		}
		switch mutation {
		case "missing_compatibility":
			delete(broken, "WFConditionalLegacyComparisonBehavior")
		case "wrong_comparison":
			broken["WFCondition"] = uint64(5)
		case "wrong_number":
			broken["WFNumberValue"] = float64(0)
		case "missing_input":
			delete(broken, "WFInput")
		}
		if err := p15SameNativePredicate(broken, reference, true); err == nil {
			t.Errorf("accepted %s mutation", mutation)
		}
	}
}

func TestP15GeneratedGuardChangePreservesExplicitModernIf(t *testing.T) {
	for _, condition := range []string{"value == 1", "value == 1 && value == 2"} {
		starts := p15ConditionalStarts(p15CompiledActions(t, "var value=number(1)\nif modern: "+condition+" { comment(\"modern\") }"))
		if len(starts) != 1 {
			t.Fatalf("expected one modern conditional, got %d", len(starts))
		}
		if _, present := starts[0]["WFConditionalLegacyComparisonBehavior"]; present {
			t.Fatalf("explicit modern condition gained compatibility behavior: %#v", starts[0])
		}
		if condition != "value == 1" && starts[0]["WFConditions"] == nil {
			t.Fatal("explicit modern compound condition lost its native predicate table")
		}
	}
}

func TestP15GeneratedGuardChangePreservesImportedCompatibility(t *testing.T) {
	for _, variant := range []string{"absent", "false", "true"} {
		t.Run(variant, func(t *testing.T) {
			const producer = "15151515-1515-1515-1515-151515151515"
			params := map[string]any{
				"GroupingIdentifier": "P15-GUARD-PARITY",
				"WFControlFlowMode":  int64(0), "WFCondition": int64(4),
				"WFInput": p12ActionOutAttachment(producer, "Text"), "WFConditionalActionString": "ready",
			}
			if variant != "absent" {
				params["WFConditionalLegacyComparisonBehavior"] = variant == "true"
			}
			doc, _ := semBuildDocFromActions(t, []any{
				map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"UUID": producer, "WFTextActionText": "ready"}},
				map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.conditional", "WFWorkflowActionParameters": params},
				map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.conditional", "WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "P15-GUARD-PARITY", "WFControlFlowMode": int64(2)}},
			}, plist.XMLFormat)
			plan, _, _, _, err := verifyCorpusDualHybridWithPreOverlay(doc, "P15-GUARD-"+variant, true)
			if err != nil {
				t.Fatal(err)
			}
			want := "NATURAL_VERIFIED"
			if variant == "false" {
				want = "EXACT_NATIVE_FALLBACK"
			}
			if plan.Decisions[1].Classification != want || plan.Decisions[2].Classification != want {
				t.Fatalf("original imported predicate classification changed: %#v", plan.Decisions)
			}
		})
	}
}
