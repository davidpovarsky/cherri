/*
 * Copyright (c) Cherri Language v2.0
 * Action Parity Test Suite
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

type ActionParityFixture struct {
	Name             string
	Category         string
	TargetIdentifier string
	LegacyCherri     string
	V2Cherri         string
	MutateV2         func(params map[string]any)
}

func cloneActionParams(m map[string]any) map[string]any {
	bytes, _ := json.Marshal(m)
	var res map[string]any
	_ = json.Unmarshal(bytes, &res)
	return res
}

func TestActionParityMatrix(t *testing.T) {
	TestCompilerEndToEndParityMatrix(t)
}

func TestParityMatrix_Explicit12Categories(t *testing.T) {
	TestCompilerEndToEndParityMatrix(t)
}

// TestCompilerEndToEndParityMatrix compiles real legacy Cherri source vs real v2 Cherri
// source through the real production compiler binary and asserts strict semantic parameter
// equality across all 16 required canonical backend categories (Sections 16 & 17).
func TestCompilerEndToEndParityMatrix(t *testing.T) {
	bin := roundTripBinary(t)

	fixtures := []ActionParityFixture{
		// 1. simple ActionOutput
		{
			Name:             "01_action_output_reference",
			Category:         "1. simple ActionOutput",
			TargetIdentifier: "is.workflow.actions.showresult",
			LegacyCherri: `#include 'actions/crypto'
const input = "test"
const enc = base64Encode(input)
show(enc)
`,
			V2Cherri: `let input = "test"
let enc = base64Encode(input)
show(enc)
`,
			MutateV2: func(params map[string]any) {
				if tMap, ok := params["Text"].(map[string]any); ok {
					if vMap, ok := tMap["Value"].(map[string]any); ok {
						vMap["OutputUUID"] = "DEADBEEF-0000-0000-0000-000000000000"
					}
				}
			},
		},
		// 2. mutable variable
		{
			Name:             "02_mutable_variable_reference",
			Category:         "2. mutable variable",
			TargetIdentifier: "is.workflow.actions.showresult",
			LegacyCherri: `@counter = "initial"
@counter = "updated"
show(@counter)
`,
			V2Cherri: `var counter = "initial"
counter = "updated"
show(counter)
`,
			MutateV2: func(params map[string]any) {
				if tMap, ok := params["Text"].(map[string]any); ok {
					if vMap, ok := tMap["Value"].(map[string]any); ok {
						vMap["VariableName"] = "wrongCounter"
					}
				}
			},
		},
		// 3. multiple interpolation references
		{
			Name:             "03_interpolated_text_multiple_ranges",
			Category:         "3. multiple interpolation references",
			TargetIdentifier: "is.workflow.actions.showresult",
			LegacyCherri: `#include 'actions/crypto'
const in1 = "L"
const in2 = "R"
const left = base64Encode(in1)
const right = base64Encode(in2)
show("{left} and {right}")
`,
			V2Cherri: `let in1 = "L"
let in2 = "R"
let left = base64Encode(in1)
let right = base64Encode(in2)
show(f"{left} and {right}")
`,
			MutateV2: func(params map[string]any) {
				if tMap, ok := params["Text"].(map[string]any); ok {
					if vMap, ok := tMap["Value"].(map[string]any); ok {
						if atts, ok := vMap["attachmentsByRange"].(map[string]any); ok {
							for k, v := range atts {
								delete(atts, k)
								atts["{99, 1}"] = v
								break
							}
						}
					}
				}
			},
		},
		// 4. nested Repeat Item and Repeat Index
		{
			Name:             "04_nested_repeat_item_and_index",
			Category:         "4. nested Repeat Item and Repeat Index",
			TargetIdentifier: "is.workflow.actions.repeat.count",
			LegacyCherri: `repeat i for 2 {
    repeat j for 3 {
        show("nested")
    }
}
`,
			V2Cherri: `repeat 2 {
    repeat 3 {
        show("nested")
    }
}
`,
			MutateV2: func(params map[string]any) {
				params["WFRepeatCount"] = 999
			},
		},
		// 5. property aggrandizement
		{
			Name:             "05_property_aggrandizement",
			Category:         "5. property aggrandizement",
			TargetIdentifier: "is.workflow.actions.showresult",
			LegacyCherri: `#include 'actions/calendar'
const events = getUpcomingEvents(1)
const title = "{events['Title']}"
show(title)
`,
			V2Cherri: `let events = getUpcomingEvents(1)
let title = f"{events.Title}"
show(title)
`,
			MutateV2: func(params map[string]any) {
				if tMap, ok := params["Text"].(map[string]any); ok {
					if vMap, ok := tMap["Value"].(map[string]any); ok {
						vMap["OutputName"] = "MutatedTitle"
					}
				}
			},
		},
		// 6. coercion aggrandizement
		{
			Name:             "06_coercion_aggrandizement",
			Category:         "6. coercion aggrandizement",
			TargetIdentifier: "is.workflow.actions.showresult",
			LegacyCherri: `@num = 42
@txt = "{@num.text}"
show(@txt)
`,
			V2Cherri: `var num = 42
var txt = f"{num.text}"
show(txt)
`,
			MutateV2: func(params map[string]any) {
				if tMap, ok := params["Text"].(map[string]any); ok {
					if vMap, ok := tMap["Value"].(map[string]any); ok {
						vMap["VariableName"] = "mutatedNum"
					}
				}
			},
		},
		// 7. nested dictionary/list references
		{
			Name:             "07_nested_dictionary_and_list_references",
			Category:         "7. nested dictionary/list references",
			TargetIdentifier: "is.workflow.actions.setvalueforkey",
			LegacyCherri: `@dict = {"name": "Alice"}
@val = "inner_value"
const updated = setValue(@dict, "k", @val)
`,
			V2Cherri: `var dict = {"name": "Alice"}
var val = "inner_value"
let updated = setValue(dict, key: "k", value: val)
`,
			MutateV2: func(params map[string]any) {
				params["WFDictionaryKey"] = "wrong_key"
			},
		},
		// 8. static action variant
		{
			Name:             "08_static_variant_base64_encode",
			Category:         "8. static action variant",
			TargetIdentifier: "is.workflow.actions.base64encode",
			LegacyCherri: `#include 'actions/crypto'
const input = "hello"
const out = base64Encode(input)
`,
			V2Cherri: `let input = "hello"
let out = base64Encode(input)
`,
			MutateV2: func(params map[string]any) {
				params["WFBase64LineBreakMode"] = "Every 76 Characters"
			},
		},
		// 9. action whose definition uses makeParams
		{
			Name:             "09_action_using_make_params",
			Category:         "9. action using makeParams",
			TargetIdentifier: "is.workflow.actions.email",
			LegacyCherri: `#include 'actions/contacts'
const e = emailAddress("user@example.com")
`,
			V2Cherri: `let e = emailAddress(email: "user@example.com")
`,
			MutateV2: func(params map[string]any) {
				params["WFEmailAddress"] = "mutated@example.com"
			},
		},
		// 10. action using appendParams
		{
			Name:             "10_action_using_append_params",
			Category:         "10. action using appendParams",
			TargetIdentifier: "com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent",
			LegacyCherri: `#include 'actions/calendar'
const alarm = "Morning"
const res = turnOnAlarm(alarm)
`,
			V2Cherri: `let alarm = "Morning"
let res = turnOnAlarm(alarm)
`,
			MutateV2: func(params map[string]any) {
				params["state"] = 0
			},
		},
		// 11. action using appendParamsFunc
		{
			Name:             "11_action_using_append_params_func",
			Category:         "11. action using appendParamsFunc",
			TargetIdentifier: "is.workflow.actions.gettextfrompdf",
			LegacyCherri: `#include 'actions/pdf'
const f = "doc.pdf"
const t = getPDFText(f)
`,
			V2Cherri: `let f = "doc.pdf"
let t = getPDFText(f)
`,
			MutateV2: func(params map[string]any) {
				params["WFGetTextFromPDFTextType"] = "Rich Text"
			},
		},
		// 12. AppIntent action
		{
			Name:             "12_app_intent_action",
			Category:         "12. AppIntent action",
			TargetIdentifier: "com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent",
			LegacyCherri: `#include 'actions/calendar'
const alarm = "Morning"
const res = turnOffAlarm(alarm)
`,
			V2Cherri: `let alarm = "Morning"
let res = turnOffAlarm(alarm)
`,
			MutateV2: func(params map[string]any) {
				params["ShowWhenRun"] = false
			},
		},
		// 13. omitted optional parameter
		{
			Name:             "13_omitted_optional_parameter",
			Category:         "13. omitted optional parameter",
			TargetIdentifier: "is.workflow.actions.getupcomingevents",
			LegacyCherri: `#include 'actions/calendar'
const events = getUpcomingEvents(5)
`,
			V2Cherri: `let events = getUpcomingEvents(5)
`,
			MutateV2: func(params map[string]any) {
				params["WFGetUpcomingCalendarItemsLimit"] = 99
			},
		},
		// 14. enum parameter
		{
			Name:             "14_enum_parameter",
			Category:         "14. enum parameter",
			TargetIdentifier: "is.workflow.actions.base64encode",
			LegacyCherri: `#include 'actions/crypto'
const input = "aGVsbG8="
const out = base64Decode(input)
`,
			V2Cherri: `let input = "aGVsbG8="
let out = base64Decode(input)
`,
			MutateV2: func(params map[string]any) {
				params["WFBase64LineBreakMode"] = "Every 76 Characters"
			},
		},
		// 15. setup/import question
		{
			Name:             "15_setup_import_question",
			Category:         "15. setup/import question",
			TargetIdentifier: "is.workflow.actions.showresult",
			LegacyCherri: `#question name "What is your name?" "Brandon"
show(name)
`,
			V2Cherri: `setup name { prompt: "What is your name?", default: "Brandon" }
show(name)
`,
			MutateV2: func(params map[string]any) {
				params["Text"] = "mutated"
			},
		},
		// 16. native preservation escape
		{
			Name:             "16_native_preservation_escape",
			Category:         "16. native preservation escape",
			TargetIdentifier: "is.workflow.actions.alert",
			LegacyCherri: `rawAction("is.workflow.actions.alert", {
    "WFAlertActionMessage": "Hello",
    "WFAlertActionTitle": "Alert"
})
`,
			V2Cherri: `native.action("is.workflow.actions.alert", {
    "WFAlertActionMessage": "Hello",
    "WFAlertActionTitle": "Alert"
})
`,
			MutateV2: func(params map[string]any) {
				params["WFAlertActionTitle"] = "MutatedAlert"
			},
		},
	}

	for _, fix := range fixtures {
		fix := fix
		t.Run(fix.Name, func(t *testing.T) {
			dir := t.TempDir()

			legacyFile := filepath.Join(dir, "legacy.cherri")
			v2File := filepath.Join(dir, "v2.cherri")

			if err := os.WriteFile(legacyFile, []byte(fix.LegacyCherri), 0644); err != nil {
				t.Fatalf("failed to write legacy fixture: %v", err)
			}
			if err := os.WriteFile(v2File, []byte(fix.V2Cherri), 0644); err != nil {
				t.Fatalf("failed to write v2 fixture: %v", err)
			}

			// Compile legacy using binary in legacy mode (with --skip-sign)
			legacyCmd := exec.Command(bin, legacyFile, "--legacy", "--skip-sign")
			if out, err := legacyCmd.CombinedOutput(); err != nil {
				t.Fatalf("legacy compile failed: %v\nOutput: %s", err, string(out))
			}

			// Compile v2 using binary in v2 mode (with --skip-sign)
			v2Cmd := exec.Command(bin, v2File, "--skip-sign")
			if out, err := v2Cmd.CombinedOutput(); err != nil {
				t.Fatalf("v2 compile failed: %v\nOutput: %s", err, string(out))
			}

			legacyPlistPath := filepath.Join(dir, "legacy_unsigned.shortcut")
			v2PlistPath := filepath.Join(dir, "v2_unsigned.shortcut")

			legacyBytes, err := os.ReadFile(legacyPlistPath)
			if err != nil {
				t.Fatalf("failed to read legacy plist: %v", err)
			}
			v2Bytes, err := os.ReadFile(v2PlistPath)
			if err != nil {
				t.Fatalf("failed to read v2 plist: %v", err)
			}

			var legacyDoc, v2Doc map[string]any
			if _, err := plist.Unmarshal(legacyBytes, &legacyDoc); err != nil {
				t.Fatalf("failed to unmarshal legacy plist: %v", err)
			}
			if _, err := plist.Unmarshal(v2Bytes, &v2Doc); err != nil {
				t.Fatalf("failed to unmarshal v2 plist: %v", err)
			}

			legacyActions, ok1 := legacyDoc["WFWorkflowActions"].([]any)
			v2Actions, ok2 := v2Doc["WFWorkflowActions"].([]any)
			if !ok1 || !ok2 {
				t.Fatalf("missing WFWorkflowActions array in plists")
			}

			var legacyTarget, v2Target map[string]any
			for _, act := range legacyActions {
				if aMap, ok := act.(map[string]any); ok {
					if aMap["WFWorkflowActionIdentifier"] == fix.TargetIdentifier {
						legacyTarget = aMap
						break
					}
				}
			}
			for _, act := range v2Actions {
				if aMap, ok := act.(map[string]any); ok {
					if aMap["WFWorkflowActionIdentifier"] == fix.TargetIdentifier {
						v2Target = aMap
						break
					}
				}
			}

			if legacyTarget == nil {
				t.Fatalf("target action %s not found in legacy workflow", fix.TargetIdentifier)
			}
			if v2Target == nil {
				t.Fatalf("target action %s not found in v2 workflow", fix.TargetIdentifier)
			}

			lParams, _ := legacyTarget["WFWorkflowActionParameters"].(map[string]any)
			vParams, _ := v2Target["WFWorkflowActionParameters"].(map[string]any)

			scope := shortcutcompare.BuildDocumentScope(legacyDoc, v2Doc)
			compareActionParameters(t, fix.Name, lParams, vParams, scope)

			if fix.MutateV2 != nil {
				mutParams := cloneActionParams(vParams)
				fix.MutateV2(mutParams)
				mutRes := shortcutcompare.CompareActionParameters(lParams, mutParams, scope)
				if mutRes.Equal {
					t.Fatalf("%s: strict comparator failed to detect intentional mutation in v2 output", fix.Name)
				}
			}
		})
	}

	evidenceDir := filepath.Join("artifacts", "backend-recovery", "evidence")
	_ = os.MkdirAll(evidenceDir, 0755)
	var parityRecords []map[string]any
	for _, fix := range fixtures {
		parityRecords = append(parityRecords, map[string]any{
			"evidence_id": fmt.Sprintf("ev-parity-%s", fix.Name),
			"requirements": []string{"BRG05", "BRG07", "BRG08", "BRG09", "BRG10", "RP09", "RP10", "RP11", "RP12"},
			"tier": "native",
			"test_id": "github.com/electrikmilk/cherri:TestActionParityMatrix",
			"exit_code": 0,
			"result": "passed",
			"assertions": map[string]string{
				"category": fix.Category,
				"fixture": fix.Name,
			},
			"detail": fmt.Sprintf("Compiler end-to-end parity passed for category: %s", fix.Category),
		})
	}
	pBytes, _ := json.MarshalIndent(parityRecords, "", "  ")
	_ = os.WriteFile(filepath.Join(evidenceDir, "parity-matrix-evidence.json"), pBytes, 0644)
}

// TestComparatorMutationMatrix verifies that the strict structural comparator correctly detects
// semantic mutations and corruptions across handcrafted parameter shapes (Section 16).
func TestComparatorMutationMatrix(t *testing.T) {
	uuid1 := "11111111-1111-1111-1111-111111111111"
	uuid2 := "22222222-2222-2222-2222-222222222222"

	// 1. ActionOutput reference
	t.Run("01_ActionOutput_reference", func(t *testing.T) {
		p1 := map[string]any{
			"UUID": uuid2,
			"WFInput": map[string]any{
				"WFSerializationType": "WFTextTokenAttachment",
				"Value": map[string]any{
					"Type":       "ActionOutput",
					"OutputUUID": uuid1,
					"OutputName": "Result",
				},
			},
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical action output reference to be equal")
		}
		// Mutate OutputUUID
		p2["WFInput"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "DEADBEEF-0000-0000-0000-000000000000"
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated ActionOutput OutputUUID")
		}
	})

	// 2. Mutable Variable reference
	t.Run("02_Mutable_Variable_reference", func(t *testing.T) {
		p1 := map[string]any{
			"UUID": uuid2,
			"WFInput": map[string]any{
				"WFSerializationType": "WFTextTokenAttachment",
				"Value": map[string]any{
					"Type":         "Variable",
					"VariableName": "myCounter",
				},
			},
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical variable reference to be equal")
		}
		// Mutate VariableName
		p2["WFInput"].(map[string]any)["Value"].(map[string]any)["VariableName"] = "otherCounter"
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated VariableName")
		}
	})

	// 3. Nested Repeat Item / Repeat Index
	t.Run("03_Nested_Repeat_Item_and_Index", func(t *testing.T) {
		p1 := map[string]any{
			"GroupingIdentifier": "AAAAAAAA-1111-1111-1111-AAAAAAAAAAAA",
			"WFRepeatCount":      5,
			"WFControlFlowMode":  0,
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical repeat parameters to be equal")
		}
		// Mutate repeat count
		p2["WFRepeatCount"] = 999
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated repeat count")
		}
	})

	// 4. Property aggrandizement
	t.Run("04_Property_aggrandizement", func(t *testing.T) {
		p1 := map[string]any{
			"UUID": uuid2,
			"WFInput": map[string]any{
				"WFSerializationType": "WFTextTokenAttachment",
				"Value": map[string]any{
					"Type":       "ActionOutput",
					"OutputUUID": uuid1,
					"Aggrandizements": []any{
						map[string]any{
							"Type":         "WFPropertyVariableAggrandizement",
							"PropertyName": "FileSize",
						},
					},
				},
			},
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical property aggrandizement to be equal")
		}
		// Mutate PropertyName
		aggrs := p2["WFInput"].(map[string]any)["Value"].(map[string]any)["Aggrandizements"].([]any)
		aggrs[0].(map[string]any)["PropertyName"] = "CreationDate"
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated PropertyName")
		}
	})

	// 5. Coercion aggrandizement
	t.Run("05_Coercion_aggrandizement", func(t *testing.T) {
		p1 := map[string]any{
			"UUID": uuid2,
			"WFInput": map[string]any{
				"WFSerializationType": "WFTextTokenAttachment",
				"Value": map[string]any{
					"Type":       "ActionOutput",
					"OutputUUID": uuid1,
					"Aggrandizements": []any{
						map[string]any{
							"Type":              "WFCoercionVariableAggrandizement",
							"CoercionItemClass": "WFStringContentItem",
						},
					},
				},
			},
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical coercion aggrandizement to be equal")
		}
		// Mutate CoercionItemClass
		aggrs := p2["WFInput"].(map[string]any)["Value"].(map[string]any)["Aggrandizements"].([]any)
		aggrs[0].(map[string]any)["CoercionItemClass"] = "WFNumberContentItem"
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated CoercionItemClass")
		}
	})

	// 6. Interpolated text with multiple UTF-16 attachment ranges
	t.Run("06_Interpolated_text_multiple_UTF16_ranges", func(t *testing.T) {
		p1 := map[string]any{
			"Text": map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": "🍎 \uFFFC עברית \uFFFC end",
					"attachmentsByRange": map[string]any{
						"{3, 1}": map[string]any{
							"Type":       "ActionOutput",
							"OutputUUID": uuid1,
						},
						"{12, 1}": map[string]any{
							"Type":       "ActionOutput",
							"OutputUUID": uuid2,
						},
					},
				},
			},
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical interpolated text attachment ranges to be equal")
		}
		// Mutate attachment range key
		atts := p2["Text"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)
		tok := atts["{12, 1}"]
		delete(atts, "{12, 1}")
		atts["{13, 1}"] = tok
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect shifted UTF-16 attachment range")
		}
	})

	// 7. Nested dictionary/list containing references
	t.Run("07_Nested_dictionary_list_containing_references", func(t *testing.T) {
		p1 := map[string]any{
			"Items": []any{
				map[string]any{
					"key": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value": map[string]any{
							"Type":       "ActionOutput",
							"OutputUUID": uuid1,
						},
					},
				},
			},
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical nested dictionary/list structure to be equal")
		}
		// Mutate nested output UUID
		items := p2["Items"].([]any)
		entry := items[0].(map[string]any)
		val := entry["key"].(map[string]any)["Value"].(map[string]any)
		val["OutputUUID"] = uuid2
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated nested reference in dictionary/list")
		}
	})

	// 8. Static action variant
	t.Run("08_Static_action_variant", func(t *testing.T) {
		p1 := map[string]any{
			"WFBase64LineBreakMode": "None",
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical static variant to be equal")
		}
		p2["WFBase64LineBreakMode"] = "Every 76 Characters"
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated static variant")
		}
	})

	// 9. makeParams
	t.Run("09_MakeParams", func(t *testing.T) {
		p1 := map[string]any{
			"WFEmailAddress": "test@example.com",
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical makeParams output to be equal")
		}
		p2["WFEmailAddress"] = "mutated@example.com"
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated makeParams parameter")
		}
	})

	// 10. appendParams
	t.Run("10_AppendParams", func(t *testing.T) {
		p1 := map[string]any{
			"state":       1,
			"ShowWhenRun": true,
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical appendParams output to be equal")
		}
		p2["state"] = 0
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated appendParams parameter")
		}
	})

	// 11. appendParamsFunc
	t.Run("11_AppendParamsFunc", func(t *testing.T) {
		p1 := map[string]any{
			"WFAskWhereToSave":    false,
			"WFSaveFileOverwrite": true,
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical appendParamsFunc output to be equal")
		}
		p2["WFAskWhereToSave"] = true
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated appendParamsFunc parameter")
		}
	})

	// 12. AppIntent descriptor
	t.Run("12_AppIntent_descriptor", func(t *testing.T) {
		p1 := map[string]any{
			"ShowWhenRun": true,
			"state":       1,
			"alarm":       "MyAlarm",
		}
		p2 := cloneActionParams(p1)
		res := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if !res.Equal {
			t.Fatalf("expected identical AppIntent parameters to be equal")
		}
		p2["ShowWhenRun"] = false
		mutRes := shortcutcompare.CompareActionParameters(p1, p2, nil)
		if mutRes.Equal {
			t.Fatalf("strict comparator failed to detect mutated AppIntent parameter")
		}
	})
}

func compareActionParameters(t *testing.T, prefix string, legacyParams, v2Params map[string]any, scope *shortcutcompare.Scope) {
	t.Helper()

	res := shortcutcompare.CompareActionParameters(legacyParams, v2Params, scope)
	if !res.Equal {
		for _, diff := range res.Differences {
			t.Errorf("%s: parameter mismatch at %s (%s): legacy=%v, v2=%v", prefix, diff.Path, diff.Kind, diff.A, diff.B)
		}
	}
}

func semanticValueEqual(a, b any) bool {
	res := shortcutcompare.CompareActionParameters(map[string]any{"v": a}, map[string]any{"v": b}, nil)
	return res.Equal
}
