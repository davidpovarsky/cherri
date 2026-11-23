package main

import (
	"fmt"
	"testing"

	"howett.net/plist"
)

// These are synthetic native input records, not Apple runtime observations.
// Exact Legacy/V2 pre-overlay equality and the natural-action count both matter:
// a fallback must not conceal a changed native Boolean predicate.
func TestP15F01ImportedBooleanHelpersPreserveNativePredicates(t *testing.T) {
	const producer = "61616161-6161-4161-8161-616161616161"
	for _, operand := range []string{"symbol", "global_boolean_coercion"} {
		for _, code := range []int64{4, 5} {
			for _, compound := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/code_%d/compound_%t", operand, code, compound), func(t *testing.T) {
					var anchor = map[string]any{
						"WFWorkflowActionIdentifier": "is.workflow.actions.getbatterylevel",
						"WFWorkflowActionParameters": map[string]any{"UUID": producer, "Subject": "Is Charging"},
					}
					var body = map[string]any{"Type": "ActionOutput", "OutputUUID": producer}
					if operand == "global_boolean_coercion" {
						anchor = map[string]any{
							"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
							"WFWorkflowActionParameters": map[string]any{"UUID": producer, "WFTextActionText": "synthetic anchor"},
						}
						body = map[string]any{
							"Type": "ExtensionInput",
							"Aggrandizements": []any{map[string]any{
								"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFBooleanContentItem",
							}},
						}
					}
					var input = map[string]any{
						"Type":     "Variable",
						"Variable": map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": body},
					}
					var params = map[string]any{
						"GroupingIdentifier": "P15-NATIVE-BOOLEAN", "WFControlFlowMode": int64(0),
						"WFCondition": code, "WFInput": input,
					}
					if compound {
						delete(params, "WFCondition")
						delete(params, "WFInput")
						params["WFConditions"] = map[string]any{
							"WFSerializationType": "WFContentPredicateTableTemplate",
							"Value": map[string]any{
								"WFActionParameterFilterPrefix": int64(1), "WFContentPredicateBoundedDate": false,
								"WFActionParameterFilterTemplates": []any{
									map[string]any{"WFCondition": code, "WFInput": input},
									map[string]any{"WFCondition": int64(9) - code, "WFInput": input},
								},
							},
						}
					}
					var doc, _ = semBuildDocFromActions(t, []any{
						anchor,
						map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.conditional", "WFWorkflowActionParameters": params},
						map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.conditional", "WFWorkflowActionParameters": map[string]any{
							"GroupingIdentifier": "P15-NATIVE-BOOLEAN", "WFControlFlowMode": int64(2),
						}},
					}, plist.XMLFormat)
					var plan, _, _, _, err = verifyCorpusDualHybridWithPreOverlay(doc, "P15-NATIVE-BOOLEAN", true)
					if err != nil {
						t.Fatal(err)
					}
					if plan.NaturalVerifiedCount != 3 {
						t.Fatalf("native Boolean predicate lost natural promotion: got %d/3; decisions=%+v", plan.NaturalVerifiedCount, plan.Decisions)
					}
				})
			}
		}
	}
}

// This compares two authored language forms through the production compiler.
// Only random action/group identities are mapped; all actions, reference links,
// operand keys, values, plist number types and coercions remain in the comparison.
// It is source-level equivalence, not an observation of the battery runtime.
func TestP15F01AuthoredBooleanHelpersMatchExplicitComparisons(t *testing.T) {
	var normalize = func(t *testing.T, actions []map[string]any) []any {
		t.Helper()
		var ids = map[string]string{}
		var groups = map[string]string{}
		for i, action := range actions {
			var p = p15Param(action)
			if id, ok := p["UUID"].(string); ok {
				if id == "" || ids[id] != "" {
					t.Fatal("empty or duplicate action identity")
				}
				ids[id] = fmt.Sprintf("ACTION_%d", i)
			}
			if group, ok := p["GroupingIdentifier"].(string); ok && groups[group] == "" {
				groups[group] = fmt.Sprintf("GROUP_%d", i)
			}
		}
		var visit func(any)
		visit = func(value any) {
			switch v := value.(type) {
			case map[string]any:
				for key, child := range v {
					if key == "UUID" || key == "OutputUUID" || key == "GroupingIdentifier" {
						var original, ok = child.(string)
						var replacement = ids[original]
						if key == "GroupingIdentifier" {
							replacement = groups[original]
						}
						if !ok || original == "" || replacement == "" {
							t.Fatalf("unbound %s identity", key)
						}
						v[key] = replacement
					} else {
						visit(child)
					}
				}
			case []any:
				for _, child := range v {
					visit(child)
				}
			}
		}
		var normalized = make([]any, len(actions))
		for i, action := range actions {
			visit(action)
			normalized[i] = action
		}
		return normalized
	}
	for _, tc := range []struct {
		name, setup, operand string
	}{
		{"numeric_var", "var flag = false\n", "flag"},
		{"numeric_let", "let flag = true\n", "flag"},
		{"native_direct", "", "isCharging()"},
		{"native_let", "let flag = isCharging()\n", "flag"},
		{"native_var", "var flag = isCharging()\n", "flag"},
		{"native_snapshot", "var value = isCharging()\nlet flag = value\n", "flag"},
		{"native_second_subject", "", "connectedToCharger()"},
	} {
		for _, helper := range []string{"isTrue", "isFalse"} {
			for _, mode := range []string{"", "modern: "} {
				t.Run(tc.name+"/"+helper+"/"+mode, func(t *testing.T) {
					var op = " == true"
					if helper == "isFalse" {
						op = " != true"
					}
					var suffix = " { comment(\"THEN\") } else { comment(\"ELSE\") }"
					var actual = normalize(t, p15CompiledActions(t, tc.setup+"if "+mode+helper+"("+tc.operand+")"+suffix))
					var expected = normalize(t, p15CompiledActions(t, tc.setup+"if "+mode+tc.operand+op+suffix))
					if !plistValuesStrictlyEqual(actual, expected) {
						t.Fatalf("helper graph differs from explicit Bool comparison:\nactual=%#v\nexpected=%#v", actual, expected)
					}
				})
			}
		}
	}
	for _, mode := range []string{"", "modern: "} {
		t.Run("compound/"+mode, func(t *testing.T) {
			var prefix = "var flag = false\nif " + mode
			var suffix = " { comment(\"THEN\") }"
			var actual = normalize(t, p15CompiledActions(t, prefix+"isTrue(flag) && isFalse(flag)"+suffix))
			var expected = normalize(t, p15CompiledActions(t, prefix+"flag == true && flag != true"+suffix))
			if !plistValuesStrictlyEqual(actual, expected) {
				t.Fatal("compound helpers differ from explicit Bool comparison graph")
			}
		})
	}
}
