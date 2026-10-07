/*
 * Copyright (c) Cherri Language v2.0
 * Action Parity Test Suite
 */

package main

import (
	"encoding/json"
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
	bin := roundTripBinary(t)

	fixtures := []ActionParityFixture{
		{
			Name:             "static_variant_base64_encode",
			Category:         "static variant parameter",
			TargetIdentifier: "is.workflow.actions.base64encode",
			LegacyCherri: `#include 'actions/crypto'
const input = "hello"
const out = base64Encode(input)
`,
			V2Cherri: `let input = "hello"
let out = base64Encode(input)
`,
			MutateV2: func(params map[string]any) {
				params["WFBase64LineBreakMode"] = "None"
			},
		},
		{
			Name:             "static_variant_base64_decode",
			Category:         "static variant parameter",
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
		{
			Name:             "enum_and_omitted_calendar_events",
			Category:         "enum and optional omitted parameter",
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
		{
			Name:             "app_intent_alarm",
			Category:         "AppIntent descriptor",
			TargetIdentifier: "com.apple.mobiletimer-framework.MobileTimerIntents.MTToggleAlarmIntent",
			LegacyCherri: `#include 'actions/calendar'
const alarm = "TestAlarm"
const res = turnOnAlarm(alarm)
`,
			V2Cherri: `let alarm = "TestAlarm"
let res = turnOnAlarm(alarm)
`,
			MutateV2: func(params map[string]any) {
				params["ShowWhenRun"] = false
			},
		},
		{
			Name:             "text_token_variable_input",
			Category:         "variable/token parameter",
			TargetIdentifier: "is.workflow.actions.showresult",
			LegacyCherri: `const val = "Hello World"
show(val)
`,
			V2Cherri: `let val = "Hello World"
show(val)
`,
			MutateV2: func(params map[string]any) {
				if tMap, ok := params["Text"].(map[string]any); ok {
					if vMap, ok := tMap["Value"].(map[string]any); ok {
						vMap["VariableName"] = "mutatedVariable"
					}
				}
			},
		},
		{
			Name:             "custom_serializer_save_file",
			Category:         "custom parameter builder with static params",
			TargetIdentifier: "is.workflow.actions.documentpicker.save",
			LegacyCherri: `#include 'actions/documents'
const content = "evidence data"
const file = saveFile("output.txt", content, true)
`,
			V2Cherri: `let content = "evidence data"
let file = saveFile("output.txt", content, overwrite: true)
`,
			MutateV2: func(params map[string]any) {
				params["WFAskWhereToSave"] = true
			},
		},
		{
			Name:             "nested_javascript_execution",
			Category:         "nested token / custom action",
			TargetIdentifier: "is.workflow.actions.runjavascriptonwebpage",
			LegacyCherri: `#include 'actions/web'
const res = runJavaScriptOnWebpage("document.title;")
`,
			V2Cherri: `let res = runJavaScriptOnWebpage("document.title;")
`,
			MutateV2: func(params map[string]any) {
				params["WFJavaScript"] = "document.body;"
			},
		},
		{
			Name:             "action_output_reference",
			Category:         "ActionOutput reference",
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
		{
			Name:             "mutable_variable_reference",
			Category:         "mutable Variable reference",
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
		{
			Name:             "nested_repeat_item_and_index",
			Category:         "nested Repeat Item / Repeat Index",
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
		{
			Name:             "interpolated_text_multiple_ranges",
			Category:         "interpolated text with multiple UTF-16 attachment ranges",
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
							// Shift attachment range key to an incorrect range
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
}

func TestParityMatrix_Explicit12Categories(t *testing.T) {
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
