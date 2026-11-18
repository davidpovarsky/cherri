package main

import (
	"testing"

	"howett.net/plist"
)

// Synthetic input only: every operand is a native reference or token assembly.
// These constructs do not execute an action while assembling WFConditions.
func TestP15F01ImportedCompoundOperandsPreserveNatural(t *testing.T) {
	const producer = "51515151-5151-5151-5151-515151515151"
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any, map[string]any)
	}{
		{"coercion", func(body, clause map[string]any) {
			body["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
		}},
		{"property", func(body, clause map[string]any) {
			body["Aggrandizements"] = []any{map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"}}
		}},
		{"dictionary-key", func(body, clause map[string]any) {
			body["Aggrandizements"] = []any{
				map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFDictionaryContentItem"},
				map[string]any{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "key"},
			}
		}},
		{"native-ref-output-name", func(body, clause map[string]any) {
			body["OutputName"] = "Reference-specific name"
		}},
		{"native-ref-aggrandizements", func(body, clause map[string]any) {
			body["Aggrandizements"] = []any{map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name", "PropertyUserInfo": map[string]any{"Value": int64(7)}}}
		}},
		{"token-string-rhs", func(body, clause map[string]any) {
			clause["WFConditionalActionString"] = map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": "prefix\uFFFC",
					"attachmentsByRange": map[string]any{
						"{6, 1}": map[string]any{"Type": "ActionOutput", "OutputUUID": producer, "OutputName": "Text"},
					},
				},
			}
		}},
		{"native-ref-presence", func(body, clause map[string]any) {
			body["OutputName"] = "Reference-specific name"
			clause["WFCondition"] = int64(100)
			delete(clause, "WFConditionalActionString")
		}},
		{"presence-with-boolean-coercion", func(body, clause map[string]any) {
			body["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFBooleanContentItem"}}
			clause["WFCondition"] = int64(100)
			delete(clause, "WFConditionalActionString")
		}},
		{"presence-with-coercion", func(body, clause map[string]any) {
			body["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
			clause["WFCondition"] = int64(100)
			delete(clause, "WFConditionalActionString")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := p12ActionOutAttachment(producer, "Text")
			attachment := input["Variable"].(map[string]any)
			body := attachment["Value"].(map[string]any)
			clause := map[string]any{"WFCondition": int64(4), "WFInput": input, "WFConditionalActionString": "sample"}
			tc.mutate(body, clause)
			doc, _ := semBuildDocFromActions(t, []any{
				map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"UUID": producer, "WFTextActionText": "sample"}},
				map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.conditional", "WFWorkflowActionParameters": map[string]any{
					"GroupingIdentifier": "P15-NATIVE-OPERANDS", "WFControlFlowMode": int64(0),
					"WFConditions": map[string]any{"WFSerializationType": "WFContentPredicateTableTemplate", "Value": map[string]any{
						"WFActionParameterFilterPrefix": int64(1), "WFContentPredicateBoundedDate": false,
						"WFActionParameterFilterTemplates": []any{
							clause,
							map[string]any{"WFCondition": int64(4), "WFInput": p12ActionOutAttachment(producer, "Text"), "WFConditionalActionString": "sample"},
						},
					}},
				}},
				map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.conditional", "WFWorkflowActionParameters": map[string]any{
					"GroupingIdentifier": "P15-NATIVE-OPERANDS", "WFControlFlowMode": int64(2),
				}},
			}, plist.XMLFormat)
			plan, _, _, _, err := verifyCorpusDualHybridWithPreOverlay(doc, "P15-NATIVE-OPERANDS", true)
			if err != nil {
				t.Fatal(err)
			}
			if plan.NaturalVerifiedCount != 3 {
				for i, d := range plan.Decisions {
					if d.Classification != "NATURAL_VERIFIED" {
						t.Errorf("action %d: classification=%s stage=%s reason=%s", i, d.Classification, d.DemotionStage, d.DemotionReason)
					}
				}
				t.Fatalf("native operand lost natural promotion: got %d/3", plan.NaturalVerifiedCount)
			}
		})
	}
}

func TestP15F01NestedProducerWrappersRemainLazy(t *testing.T) {
	for _, condition := range []string{
		`run("P15_RHS", input: true).text == "ready"`,
		`run("P15_RHS", input: true)["key"] == "ready"`,
		`native.ref(run("P15_RHS", input: true), outputName: "value") == "ready"`,
		`native.ref(ShortcutInput, outputName: run("P15_RHS", input: true)) == "ready"`,
		`f"{run("P15_RHS", input: true)}" == "ready"`,
	} {
		for _, logical := range []struct {
			op     string
			branch int
		}{{"&&", 0}, {"||", 1}} {
			acts := p15CompiledActions(t, "var flag = false\nif flag == true "+logical.op+" ("+condition+") { comment(\"THEN\") }\n")
			if err := p15GuardedRun(acts, "P15_RHS", logical.branch); err != nil {
				t.Errorf("%s %s: %v", logical.op, condition, err)
			}
		}
	}
}
