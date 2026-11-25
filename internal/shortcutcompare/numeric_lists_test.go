package shortcutcompare

import (
	"path/filepath"
	"reflect"
	"testing"
)

func numericListPair(t *testing.T, fixture string) (map[string]any, map[string]any) {
	t.Helper()
	load := func(lane string) map[string]any {
		d, err := LoadShortcutDocument(filepath.Join("testdata", "numeric-lists", fixture+"-"+lane+".shortcut"))
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	return load("legacy"), load("v2")
}
func numericListActions(d map[string]any) []any { return d["WFWorkflowActions"].([]any) }
func numericListAction(d map[string]any, i int) map[string]any {
	return numericListActions(d)[i].(map[string]any)
}
func numericListParams(d map[string]any, i int) map[string]any {
	a := numericListAction(d, i)
	p, ok := a["WFWorkflowActionParameters"].(map[string]any)
	if !ok {
		p = map[string]any{}
		a["WFWorkflowActionParameters"] = p
	}
	return p
}
func numericListRef(id string) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": id, "OutputName": "Number"}}
}
func numericListInput(d map[string]any, i int) map[string]any {
	return numericListParams(d, i)["WFInput"].(map[string]any)["Value"].(map[string]any)
}
func numericListRows(d map[string]any) []any { return numericListParams(d, 3)["WFItems"].([]any) }
func numericListRowRef(d map[string]any, i int) map[string]any {
	return numericListRows(d)[i].(map[string]any)["WFValue"].(map[string]any)["Value"].(map[string]any)
}
func numericListAppendAction(id string, params map[string]any) map[string]any {
	return map[string]any{"WFWorkflowActionIdentifier": id, "WFWorkflowActionParameters": params}
}

func TestNumericProceduralListParityRetainsRealNumberGraph(t *testing.T) {
	for _, fixture := range []string{"arrays", "controls"} {
		t.Run(fixture, func(t *testing.T) {
			a, b := numericListPair(t, fixture)
			beforeA, beforeB := deepCloneMap(a), deepCloneMap(b)
			if CompareDocuments(a, b).Equal {
				t.Fatal("raw compiler representations unexpectedly equal")
			}
			if result := CompareDocumentsNormalized(a, b); !result.Equal {
				t.Fatalf("typed numeric dataflow differs: %+v", result.Differences)
			}
			normalized := NormalizeDocumentForParity(a)
			var originalNumbers, normalizedNumbers []map[string]any
			for _, raw := range numericListActions(a) {
				act := raw.(map[string]any)
				if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
					originalNumbers = append(originalNumbers, act["WFWorkflowActionParameters"].(map[string]any))
				}
			}
			var rows []any
			for _, raw := range numericListActions(normalized) {
				act := raw.(map[string]any)
				p := act["WFWorkflowActionParameters"].(map[string]any)
				if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
					normalizedNumbers = append(normalizedNumbers, p)
				}
				if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" {
					rows = p["WFItems"].([]any)
				}
			}
			if len(normalizedNumbers) != len(originalNumbers) || len(rows) != len(originalNumbers) {
				t.Fatalf("Number producers/cardinality erased: %d/%d/%d", len(originalNumbers), len(normalizedNumbers), len(rows))
			}
			for i, p := range originalNumbers {
				if !reflect.DeepEqual(p["WFNumberActionNumber"], normalizedNumbers[i]["WFNumberActionNumber"]) || p["UUID"] != normalizedNumbers[i]["UUID"] {
					t.Fatalf("producer %d scalar type/value/identity changed", i)
				}
				row := rows[i].(map[string]any)
				wrapped, ok := row["WFValue"].(map[string]any)
				if !ok || wrapped["WFSerializationType"] != "WFTextTokenAttachment" {
					t.Fatalf("numeric row %d became scalar/text: %#v", i, row)
				}
				ref := wrapped["Value"].(map[string]any)
				if ref["Type"] != "ActionOutput" || ref["OutputUUID"] != p["UUID"] {
					t.Fatalf("numeric row %d edge changed: %#v", i, ref)
				}
			}
			if !reflect.DeepEqual(a, beforeA) || !reflect.DeepEqual(b, beforeB) {
				t.Fatal("normalization mutated captured source")
			}
		})
	}
}

func TestNumericProceduralListMixedLiteralTextRetainsTypes(t *testing.T) {
	a, b := numericListPair(t, "arrays")
	numericListAction(a, 4)["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
	ap := numericListParams(a, 4)
	delete(ap, "WFNumberActionNumber")
	ap["WFTextActionText"] = "7"
	// A real Text producer may fold to a text row; the other Number producers remain.
	numericListRows(b)[1].(map[string]any)["WFValue"] = "7"
	acts := numericListActions(b)
	b["WFWorkflowActions"] = append(acts[:1], acts[2:]...)
	if result := CompareDocumentsNormalized(a, b); !result.Equal {
		t.Fatalf("mixed literal types were not preserved: %+v", result.Differences)
	}
	normalized := NormalizeDocumentForParity(a)
	numbers := 0
	foundText := false
	for _, raw := range numericListActions(normalized) {
		act := raw.(map[string]any)
		p := act["WFWorkflowActionParameters"].(map[string]any)
		if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
			numbers++
		}
		if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" {
			rows := p["WFItems"].([]any)
			foundText = rows[1].(map[string]any)["WFValue"] == "7"
		}
	}
	if numbers != 2 || !foundText {
		t.Fatal("numeric-looking text collapsed into Number", numbers, foundText)
	}
}

func TestNumericProceduralListRejectsChangedNativeValuesAndEdges(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"reference_output_name_conflict": func(d map[string]any) { numericListRowRef(d, 0)["OutputName"] = "WRONG" },
		"producer_output_name_conflict":  func(d map[string]any) { numericListParams(d, 0)["CustomOutputName"] = "WRONG" },
		"number_value":                   func(d map[string]any) { numericListParams(d, 0)["WFNumberActionNumber"] = 8 },
		"number_to_text": func(d map[string]any) {
			numericListAction(d, 0)["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
			p := numericListParams(d, 0)
			delete(p, "WFNumberActionNumber")
			p["WFTextActionText"] = "7"
		},
		"row_text":                   func(d map[string]any) { numericListRows(d)[0].(map[string]any)["WFValue"] = "7" },
		"row_numeric_scalar":         func(d map[string]any) { numericListRows(d)[0].(map[string]any)["WFValue"] = 7 },
		"row_type":                   func(d map[string]any) { numericListRows(d)[0].(map[string]any)["WFItemType"] = 1 },
		"row_count":                  func(d map[string]any) { numericListParams(d, 3)["WFItems"] = numericListRows(d)[:2] },
		"row_value_order":            func(d map[string]any) { r := numericListRows(d); r[0], r[2] = r[2], r[0] },
		"equal_value_identity_order": func(d map[string]any) { r := numericListRows(d); r[0], r[1] = r[1], r[0] },
		"duplicate_reference":        func(d map[string]any) { numericListRowRef(d, 1)["OutputUUID"] = numericListParams(d, 0)["UUID"] },
		"dangling_reference":         func(d map[string]any) { numericListRowRef(d, 0)["OutputUUID"] = "missing-producer" },
		"missing_reference":          func(d map[string]any) { delete(numericListRowRef(d, 0), "OutputUUID") },
		"aggrandized_reference": func(d map[string]any) {
			numericListRowRef(d, 0)["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
		},
		"unknown_row_field": func(d map[string]any) { numericListRows(d)[0].(map[string]any)["UnknownSemanticFlag"] = true },
		"disabled_number":   func(d map[string]any) { numericListAction(d, 0)["WFActionEnabled"] = false },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			a, b := numericListPair(t, "arrays")
			mutate(b)
			if CompareDocumentsNormalized(a, b).Equal {
				t.Fatal("numeric list corruption normalized away")
			}
		})
	}
}

func TestNumericProceduralListUnsafeWitnessStaysRaw(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"reference_output_name_conflict": func(d map[string]any) { numericListInput(d, 3)["OutputName"] = "WRONG" },
		"producer_output_name_conflict":  func(d map[string]any) { numericListParams(d, 2)["CustomOutputName"] = "WRONG" },
		"missing_producer_uuid":          func(d map[string]any) { delete(numericListParams(d, 2), "UUID") },
		"empty_producer_uuid": func(d map[string]any) {
			numericListParams(d, 2)["UUID"] = ""
			numericListInput(d, 3)["OutputUUID"] = ""
		},
		"duplicate_producer_uuid": func(d map[string]any) {
			id := numericListParams(d, 2)["UUID"]
			numericListParams(d, 4)["UUID"] = id
			numericListInput(d, 5)["OutputUUID"] = id
		},
		"forward_reference":  func(d map[string]any) { numericListInput(d, 3)["OutputUUID"] = numericListParams(d, 4)["UUID"] },
		"dangling_reference": func(d map[string]any) { numericListInput(d, 3)["OutputUUID"] = "missing" },
		"reference_type":     func(d map[string]any) { numericListInput(d, 3)["Type"] = "Variable" },
		"reference_aggrandizement": func(d map[string]any) {
			numericListInput(d, 3)["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
		},
		"reference_extra_field": func(d map[string]any) { numericListInput(d, 3)["UnknownSemanticFlag"] = true },
		"reference_wrapper": func(d map[string]any) {
			numericListParams(d, 3)["WFInput"].(map[string]any)["WFSerializationType"] = "WFTextTokenString"
		},
		"unknown_number_parameter":  func(d map[string]any) { numericListParams(d, 2)["UnknownSemanticFlag"] = true },
		"unknown_append_parameter":  func(d map[string]any) { numericListParams(d, 3)["UnknownSemanticFlag"] = true },
		"unknown_set_parameter":     func(d map[string]any) { numericListParams(d, 1)["UnknownSemanticFlag"] = true },
		"unknown_number_top_level":  func(d map[string]any) { numericListAction(d, 2)["UnknownSemanticFlag"] = true },
		"authored_origin_lookalike": func(d map[string]any) { numericListAction(d, 2)["__cherri_parity_question_origin_authored"] = 2 },
		"disabled_number":           func(d map[string]any) { numericListAction(d, 2)["WFActionEnabled"] = false },
		"disabled_append":           func(d map[string]any) { numericListAction(d, 3)["WFActionEnabled"] = false },
		"disabled_clear":            func(d map[string]any) { numericListAction(d, 0)["WFActionEnabled"] = false },
		"non_numeric_number":        func(d map[string]any) { numericListParams(d, 2)["WFNumberActionNumber"] = "7" },
		"boolean_number":            func(d map[string]any) { numericListParams(d, 2)["WFNumberActionNumber"] = true },
		"external_nested_use": func(d map[string]any) {
			d["CustomPayload"] = map[string]any{"Nested": []any{numericListRef(numericListParams(d, 2)["UUID"].(string))}}
		},
		"external_action_use": func(d map[string]any) {
			d["WFWorkflowActions"] = append(numericListActions(d), numericListAppendAction("is.workflow.actions.output", map[string]any{"WFOutput": numericListRef(numericListParams(d, 2)["UUID"].(string))}))
		},
		"duplicate_external_producer": func(d map[string]any) {
			d["WFWorkflowActions"] = append(numericListActions(d), numericListAppendAction("is.workflow.actions.number", map[string]any{"UUID": numericListParams(d, 2)["UUID"], "WFNumberActionNumber": 11}))
		},
		"intervening_side_effect": func(d map[string]any) {
			a := numericListActions(d)
			n := append([]any{}, a[:4]...)
			n = append(n, numericListAppendAction("is.workflow.actions.alert", map[string]any{"WFAlertActionMessage": "boundary"}))
			d["WFWorkflowActions"] = append(n, a[4:]...)
		},
		"variable_pre_read": func(d map[string]any) {
			pre := numericListAppendAction("is.workflow.actions.output", map[string]any{"WFOutput": map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": "items"}}})
			d["WFWorkflowActions"] = append([]any{pre}, numericListActions(d)...)
		},
		"other_variable_write": func(d map[string]any) {
			d["WFWorkflowActions"] = append(numericListActions(d), numericListAppendAction("is.workflow.actions.setvariable", map[string]any{"WFVariableName": "items"}))
		},
		"wrong_append_variable":      func(d map[string]any) { numericListParams(d, 5)["WFVariableName"] = "other" },
		"malformed_clear_parameters": func(d map[string]any) { numericListAction(d, 0)["WFWorkflowActionParameters"] = "malformed" },
		"clear_is_not_nothing": func(d map[string]any) {
			numericListAction(d, 0)["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
			numericListParams(d, 0)["WFTextActionText"] = "not empty"
		},
		"numeric_append_to_existing_list": func(d map[string]any) {
			numericListAction(d, 0)["WFWorkflowActionIdentifier"] = "is.workflow.actions.list"
			p := numericListParams(d, 0)
			p["UUID"] = "prior-list"
			p["WFItems"] = []any{}
			numericListParams(d, 1)["WFInput"] = numericListRef("prior-list")
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			a, b := numericListPair(t, "arrays")
			mutate(a)
			before := deepCloneMap(a)
			if got := NormalizeDocumentForParity(a); !reflect.DeepEqual(got, before) {
				t.Fatal("unsafe numeric chain received a partial or fallback normalization")
			}
			if CompareDocumentsNormalized(a, b).Equal {
				t.Fatal("unsafe witness accepted parity")
			}
		})
	}
}

func TestNumericProceduralListImportQuestionIdentity(t *testing.T) {
	question := func(index int) []any {
		return []any{map[string]any{"ActionIndex": index, "ParameterKey": "WFNumberActionNumber", "Text": "Number?", "DefaultValue": 7}}
	}
	t.Run("retained_number", func(t *testing.T) {
		a, b := numericListPair(t, "arrays")
		a["WFWorkflowImportQuestions"] = question(2)
		b["WFWorkflowImportQuestions"] = question(0)
		// Force a collision-free origin marker without granting arbitrary authored fields a bypass.
		a["__cherri_parity_question_origin"] = "authored"
		b["__cherri_parity_question_origin"] = "authored"
		if result := CompareDocumentsNormalized(a, b); !result.Equal {
			t.Fatal("retained producer question lost identity", result.Differences)
		}
		b["WFWorkflowImportQuestions"] = question(1)
		if CompareDocumentsNormalized(a, b).Equal {
			t.Fatal("question retargeted to distinct equal-valued Number")
		}
	})
	for _, index := range []int{0, 1, 3} {
		t.Run("removed_target_"+string(rune('0'+index)), func(t *testing.T) {
			a, _ := numericListPair(t, "arrays")
			a["WFWorkflowImportQuestions"] = question(index)
			before := deepCloneMap(a)
			if got := NormalizeDocumentForParity(a); !reflect.DeepEqual(got, before) {
				t.Fatal("removed question target was rebound")
			}
		})
	}
}

func TestNumericProceduralListConsistentDisplayName(t *testing.T) {
	a, b := numericListPair(t, "arrays")
	numericListParams(b, 0)["CustomOutputName"] = "Renamed Number"
	numericListRowRef(b, 0)["OutputName"] = "Renamed Number"
	if result := CompareDocumentsNormalized(a, b); !result.Equal {
		t.Fatal("consistent display name change lost the original numeric edge", result.Differences)
	}
}

func TestNumericProceduralListDoesNotCaptureIndependentDeclarations(t *testing.T) {
	a, b := numericListPair(t, "types")
	for _, d := range []map[string]any{a, b} {
		for _, raw := range numericListActions(d) {
			if raw.(map[string]any)["WFWorkflowActionIdentifier"] == "is.workflow.actions.appendvariable" {
				t.Fatal("declaration control unexpectedly contains list construction")
			}
		}
	}
	if result := CompareDocumentsNormalized(a, b); !result.Equal {
		t.Fatal("independent Number declaration was mistaken for a procedural list", result.Differences)
	}
}
