/*
 * Copyright (c) Cherri Language v2.0
 * Structural Shortcut Comparator Mutation Test Suite
 */

package shortcutcompare

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Helper to deep clone a map via JSON
func cloneDoc(doc map[string]any) map[string]any {
	bytes, _ := json.Marshal(doc)
	var res map[string]any
	_ = json.Unmarshal(bytes, &res)
	return res
}

func validBaseDoc() map[string]any {
	return map[string]any{
		"WFWorkflowClientVersion": "3036.0.4.2",
		"WFWorkflowTypes":         []any{"ActionExtension"},
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
				"WFWorkflowActionParameters": map[string]any{
					"UUID": "11111111-1111-1111-1111-111111111111",
					"WFTextActionText": "Hello World",
					"CustomOutputName": "TextOutput",
					"LiteralID": "99999999-9999-9999-9999-999999999999", // ordinary UUID literal
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.base64encode",
				"WFWorkflowActionParameters": map[string]any{
					"UUID": "22222222-2222-2222-2222-222222222222",
					"WFBase64LineBreakMode": "Every 76 Characters",
					"WFInput": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value": map[string]any{
							"Type":       "ActionOutput",
							"OutputUUID": "11111111-1111-1111-1111-111111111111",
							"OutputName": "TextOutput",
							"Aggrandizements": []any{
								map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"},
								map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"},
							},
						},
					},
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
				"WFWorkflowActionParameters": map[string]any{
					"GroupingIdentifier": "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA",
					"WFControlFlowMode":  0,
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
				"WFWorkflowActionParameters": map[string]any{
					"GroupingIdentifier": "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA",
					"WFControlFlowMode":  2,
				},
			},
		},
		"WFWorkflowImportQuestions": []any{
			map[string]any{
				"ActionIndex":  0,
				"Category":     "Parameter",
				"ParameterKey": "WFTextActionText",
				"Text":         "Enter greeting",
			},
		},
	}
}

func recordCounterExample(t *testing.T, name string, res *ComparisonResult) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "cherri-compare-counterexamples")
	_ = os.MkdirAll(dir, 0755)
	data, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, name+".json"), data, 0644)
}

func TestAlphaRenamingPasses(t *testing.T) {
	docA := validBaseDoc()
	docB := validBaseDoc()

	// Consistently rename action 0 producer and action 1 reference
	actionsB := docB["WFWorkflowActions"].([]any)
	act0Params := actionsB[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act0Params["UUID"] = "BBBBBBBB-1111-1111-1111-BBBBBBBBBBBB"

	act1Params := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act1Params["UUID"] = "BBBBBBBB-2222-2222-2222-BBBBBBBBBBBB"
	inputVal := act1Params["WFInput"].(map[string]any)["Value"].(map[string]any)
	inputVal["OutputUUID"] = "BBBBBBBB-1111-1111-1111-BBBBBBBBBBBB"

	// Consistently rename group identifier
	act2Params := actionsB[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act2Params["GroupingIdentifier"] = "CCCCCCCC-AAAA-AAAA-AAAA-CCCCCCCCCCCC"
	act3Params := actionsB[3].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act3Params["GroupingIdentifier"] = "CCCCCCCC-AAAA-AAAA-AAAA-CCCCCCCCCCCC"

	res := CompareDocuments(docA, docB)
	if !res.Equal {
		t.Fatalf("valid consistent alpha-renaming must pass, got differences: %+v", res.Differences)
	}
}

// 1. Change producer UUID to another existing producer
func TestMutation01_SwapProducerUUID(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	// Change reference in action 1 to point to action 1 itself instead of action 0
	actionsB := docB["WFWorkflowActions"].([]any)
	act1Params := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	inputVal := act1Params["WFInput"].(map[string]any)["Value"].(map[string]any)
	inputVal["OutputUUID"] = "22222222-2222-2222-2222-222222222222"

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 01 (swap producer UUID) to be rejected")
	}
	recordCounterExample(t, "mutation_01", res)
}

// 2. Reference a nonexistent producer
func TestMutation02_NonexistentProducer(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	actionsB := docB["WFWorkflowActions"].([]any)
	act1Params := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	inputVal := act1Params["WFInput"].(map[string]any)["Value"].(map[string]any)
	inputVal["OutputUUID"] = "DEADBEEF-DEAD-BEEF-DEAD-BEEFDEADBEEF"

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 02 (nonexistent producer) to be rejected")
	}
	recordCounterExample(t, "mutation_02", res)
}

// 3. Change a variable binding name/scope
func TestMutation03_ChangeVariableName(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	// Change input token to Variable token with different name
	actionsA := docA["WFWorkflowActions"].([]any)
	act1ParamsA := actionsA[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act1ParamsA["WFInput"] = map[string]any{
		"WFSerializationType": "WFTextTokenAttachment",
		"Value": map[string]any{"Type": "Variable", "VariableName": "varOriginal"},
	}

	actionsB := docB["WFWorkflowActions"].([]any)
	act1ParamsB := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act1ParamsB["WFInput"] = map[string]any{
		"WFSerializationType": "WFTextTokenAttachment",
		"Value": map[string]any{"Type": "Variable", "VariableName": "varModified"},
	}

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 03 (change variable name) to be rejected")
	}
	recordCounterExample(t, "mutation_03", res)
}

// 4. Replace a token with unrelated token-string text
func TestMutation04_ReplaceTokenWithString(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	actionsB := docB["WFWorkflowActions"].([]any)
	act1ParamsB := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act1ParamsB["WFInput"] = map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string": "unrelated string text",
		},
	}

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 04 (replace token with token string) to be rejected")
	}
	recordCounterExample(t, "mutation_04", res)
}

// 5. Delete one transformation; change its property; reverse transformation order
func TestMutation05_Transformations(t *testing.T) {
	// 5a: Delete one transformation
	{
		docA := validBaseDoc()
		docB := cloneDoc(docA)
		actionsB := docB["WFWorkflowActions"].([]any)
		act1ParamsB := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		inputVal := act1ParamsB["WFInput"].(map[string]any)["Value"].(map[string]any)
		inputVal["Aggrandizements"] = []any{
			map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"},
		}
		res := CompareDocuments(docA, docB)
		if res.Equal {
			t.Fatal("expected mutation 05a (delete transform) to be rejected")
		}
	}

	// 5b: Change property of transformation
	{
		docA := validBaseDoc()
		docB := cloneDoc(docA)
		actionsB := docB["WFWorkflowActions"].([]any)
		act1ParamsB := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		inputVal := act1ParamsB["WFInput"].(map[string]any)["Value"].(map[string]any)
		inputVal["Aggrandizements"] = []any{
			map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "FileSize"},
			map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"},
		}
		res := CompareDocuments(docA, docB)
		if res.Equal {
			t.Fatal("expected mutation 05b (change property) to be rejected")
		}
	}

	// 5c: Reverse transformation order
	{
		docA := validBaseDoc()
		docB := cloneDoc(docA)
		actionsB := docB["WFWorkflowActions"].([]any)
		act1ParamsB := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		inputVal := act1ParamsB["WFInput"].(map[string]any)["Value"].(map[string]any)
		inputVal["Aggrandizements"] = []any{
			map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"},
			map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"},
		}
		res := CompareDocuments(docA, docB)
		if res.Equal {
			t.Fatal("expected mutation 05c (reverse transform order) to be rejected")
		}
	}
}

// 6. Shift an attachment range by one UTF-16 code unit, including a leading non-BMP character
func TestMutation06_ShiftAttachmentRange(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	// Non-BMP character 🍎 (U+1F34E, surrogate pair in UTF-16 = 2 code units)
	actionsA := docA["WFWorkflowActions"].([]any)
	act0ParamsA := actionsA[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act0ParamsA["WFTextActionText"] = map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string": "🍎 \uFFFC world",
			"attachmentsByRange": map[string]any{
				"{3, 1}": map[string]any{"Type": "Variable", "VariableName": "X"},
			},
		},
	}

	actionsB := docB["WFWorkflowActions"].([]any)
	act0ParamsB := actionsB[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act0ParamsB["WFTextActionText"] = map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string": "🍎 \uFFFC world",
			"attachmentsByRange": map[string]any{
				"{4, 1}": map[string]any{"Type": "Variable", "VariableName": "X"}, // Shifted by 1 code unit
			},
		},
	}

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 06 (shifted range) to be rejected")
	}
	recordCounterExample(t, "mutation_06", res)
}

// 7. Add/remove an unknown nested key; test both operand orders
func TestMutation07_NestedKeyAsymmetry(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	actionsB := docB["WFWorkflowActions"].([]any)
	act0ParamsB := actionsB[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act0ParamsB["UnexpectedExtraKey"] = "extra_value"

	// Test A vs B
	resAB := CompareDocuments(docA, docB)
	if resAB.Equal {
		t.Fatal("expected mutation 07 (A vs B) to be rejected")
	}

	// Test B vs A
	resBA := CompareDocuments(docB, docA)
	if resBA.Equal {
		t.Fatal("expected mutation 07 (B vs A) to be rejected")
	}
}

// 8. Change number to text, false to omitted, empty string to absent
func TestMutation08_ScalarTypesAndOmission(t *testing.T) {
	// 8a: Number to text
	{
		docA := validBaseDoc()
		docB := cloneDoc(docA)
		actionsA := docA["WFWorkflowActions"].([]any)
		actionsA[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["Count"] = 42
		actionsB := docB["WFWorkflowActions"].([]any)
		actionsB[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["Count"] = "42"
		res := CompareDocuments(docA, docB)
		if res.Equal {
			t.Fatal("expected mutation 08a (number vs text) to be rejected")
		}
	}

	// 8b: False to omitted
	{
		docA := validBaseDoc()
		docB := cloneDoc(docA)
		actionsA := docA["WFWorkflowActions"].([]any)
		actionsA[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["Disabled"] = false
		res := CompareDocuments(docA, docB)
		if res.Equal {
			t.Fatal("expected mutation 08b (false vs omitted) to be rejected")
		}
	}

	// 8c: Empty string to absent
	{
		docA := validBaseDoc()
		docB := cloneDoc(docA)
		actionsA := docA["WFWorkflowActions"].([]any)
		actionsA[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["Comment"] = ""
		res := CompareDocuments(docA, docB)
		if res.Equal {
			t.Fatal("expected mutation 08c (empty string vs absent) to be rejected")
		}
	}
}

// 9. Reorder actions or associate an end marker with another group
func TestMutation09_ReorderActionsAndGroupMarkers(t *testing.T) {
	// 9a: Reorder actions
	{
		docA := validBaseDoc()
		docB := cloneDoc(docA)
		actionsB := docB["WFWorkflowActions"].([]any)
		actionsB[0], actionsB[1] = actionsB[1], actionsB[0]
		res := CompareDocuments(docA, docB)
		if res.Equal {
			t.Fatal("expected mutation 09a (reorder actions) to be rejected")
		}
	}

	// 9b: Associate end marker with another group
	{
		docA := validBaseDoc()
		docB := cloneDoc(docA)
		actionsB := docB["WFWorkflowActions"].([]any)
		act3Params := actionsB[3].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		act3Params["GroupingIdentifier"] = "ZZZZZZZZ-ZZZZ-ZZZZ-ZZZZ-ZZZZZZZZZZZZ"
		res := CompareDocuments(docA, docB)
		if res.Equal {
			t.Fatal("expected mutation 09b (wrong group on end marker) to be rejected")
		}
	}
}

// 10. Rebind an inner-loop reference to the outer-loop output
func TestMutation10_RebindLoopReference(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	// Add an action 2 producer (representing outer loop output)
	actionsA := docA["WFWorkflowActions"].([]any)
	actionsA[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"] = "33333333-3333-3333-3333-333333333333"

	actionsB := docB["WFWorkflowActions"].([]any)
	actionsB[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"] = "33333333-3333-3333-3333-333333333333"

	// In docB, action 1 references action 2 (outer) instead of action 0 (inner)
	inputValB := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFInput"].(map[string]any)["Value"].(map[string]any)
	inputValB["OutputUUID"] = "33333333-3333-3333-3333-333333333333"

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 10 (rebind loop ref) to be rejected")
	}
	recordCounterExample(t, "mutation_10", res)
}

// 11. Change a static variant parameter or AppIntent descriptor field
func TestMutation11_StaticVariantOrAppIntent(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	actionsB := docB["WFWorkflowActions"].([]any)
	act1ParamsB := actionsB[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act1ParamsB["WFBase64LineBreakMode"] = "None" // changed variant parameter

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 11 (variant param changed) to be rejected")
	}
	recordCounterExample(t, "mutation_11", res)
}

// 12. Retarget an import question, including ActionIndex zero
func TestMutation12_RetargetImportQuestion(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	questionsB := docB["WFWorkflowImportQuestions"].([]any)
	questionsB[0].(map[string]any)["ActionIndex"] = 1 // Retarget from 0 to 1

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 12 (retarget question) to be rejected")
	}
	recordCounterExample(t, "mutation_12", res)
}

// 13. Change an ordinary UUID-shaped string literal
func TestMutation13_ChangeOrdinaryUUIDLiteral(t *testing.T) {
	docA := validBaseDoc()
	docB := cloneDoc(docA)

	actionsB := docB["WFWorkflowActions"].([]any)
	act0ParamsB := actionsB[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	// Mutate LiteralID (which is an ordinary string literal that looks like a UUID)
	act0ParamsB["LiteralID"] = "88888888-8888-8888-8888-888888888888"

	res := CompareDocuments(docA, docB)
	if res.Equal {
		t.Fatal("expected mutation 13 (ordinary UUID literal changed) to be rejected")
	}
	recordCounterExample(t, "mutation_13", res)
}
