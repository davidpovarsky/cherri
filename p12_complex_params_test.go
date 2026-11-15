/*
 * Copyright (c) Cherri
 */

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/electrikmilk/args-parser"
	"howett.net/plist"
)

func p12BuildNativeDoc(t *testing.T, label string, actions []any) *NativeShortcutDocument {
	t.Helper()
	base := compileHybridDialectStrict(t, label+"_base_hdr", "#define version 27\n", true)
	base["WFWorkflowActions"] = actions
	raw, err := plist.Marshal(base, plist.XMLFormat)
	if err != nil {
		t.Fatalf("%s marshal synthetic plist: %v", label, err)
	}
	doc, err := DecodeNativeShortcutDocument(raw)
	if err != nil {
		t.Fatalf("%s decode synthetic doc: %v", label, err)
	}
	return doc
}

func p12UTF16Len(s string) int64 {
	return int64(len(utf16.Encode([]rune(s))))
}

func p12RawVarAttachment(varName string) map[string]any {
	return map[string]any{
		"Value": map[string]any{
			"Type":         "Variable",
			"VariableName": varName,
		},
		"WFSerializationType": "WFTextTokenAttachment",
	}
}

func p12RawActionOutAttachment(uid, outputName string) map[string]any {
	return map[string]any{
		"Value": map[string]any{
			"OutputName": outputName,
			"OutputUUID": uid,
			"Type":       "ActionOutput",
		},
		"WFSerializationType": "WFTextTokenAttachment",
	}
}

func p12TextTokenStringWithVar(prefix, varName, suffix string) map[string]any {
	attPos := p12UTF16Len(prefix)
	fullStr := prefix + "\ufffc" + suffix
	return map[string]any{
		"Value": map[string]any{
			"attachmentsByRange": map[string]any{
				fmt.Sprintf("{%d, 1}", attPos): map[string]any{
					"Type":         "Variable",
					"VariableName": varName,
				},
			},
			"string": fullStr,
		},
		"WFSerializationType": "WFTextTokenString",
	}
}

func p12TextTokenStringWithOut(prefix, uid, outputName, suffix string) map[string]any {
	attPos := p12UTF16Len(prefix)
	fullStr := prefix + "\ufffc" + suffix
	return map[string]any{
		"Value": map[string]any{
			"attachmentsByRange": map[string]any{
				fmt.Sprintf("{%d, 1}", attPos): map[string]any{
					"OutputName": outputName,
					"OutputUUID": uid,
					"Type":       "ActionOutput",
				},
			},
			"string": fullStr,
		},
		"WFSerializationType": "WFTextTokenString",
	}
}

func p12TextTokenStringLiteral(s string) map[string]any {
	return map[string]any{
		"Value": map[string]any{
			"string": s,
		},
		"WFSerializationType": "WFTextTokenString",
	}
}

func p12DictFieldItem(itemType int64, keyStr string, val any) map[string]any {
	return map[string]any{
		"WFItemType": itemType,
		"WFKey":      p12TextTokenStringLiteral(keyStr),
		"WFValue":    val,
	}
}

func p12DictWrapper(items []any) map[string]any {
	return map[string]any{
		"Value": map[string]any{
			"WFDictionaryFieldValueItems": items,
		},
		"WFSerializationType": "WFDictionaryFieldValue",
	}
}

func assertP12ParDualVerified(
	t *testing.T,
	label string,
	actions []any,
	wantCls map[int]string,
	wantReasons map[int]string,
	inspectPlan func(t *testing.T, plan *HybridImportPlan, legSrc, v2Src string),
) *HybridImportPlan {
	t.Helper()
	doc := p12BuildNativeDoc(t, label, actions)
	plan, legMap, v2Map, preByDialect, err := verifyCorpusDualHybridWithPreOverlay(doc, label, true)
	if err != nil {
		t.Fatalf("%s verifyCorpusDualHybridWithPreOverlay failed: %v", label, err)
	}
	if plan.Lifecycle != nil && plan.Lifecycle.DemotedActionCount != 0 {
		t.Fatalf("%s unexpected Stage-3 demotions=%d (%v)", label, plan.Lifecycle.DemotedActionCount, plan.Lifecycle.DemotionReasons)
	}
	for idx, want := range wantCls {
		if idx < 0 || idx >= len(plan.Decisions) {
			t.Fatalf("%s index %d out of range (%d decisions)", label, idx, len(plan.Decisions))
		}
		got := plan.Decisions[idx].Classification
		if got != want {
			t.Fatalf("%s action %d (%s): got classification=%s (reason=%s stage=%s demote=%s), want %s",
				label, idx, plan.Decisions[idx].AppleIdentifier, got,
				plan.Decisions[idx].ReasonCode, plan.Decisions[idx].DemotionStage, plan.Decisions[idx].DemotionReason, want)
		}
	}
	for idx, wantR := range wantReasons {
		gotR := plan.Decisions[idx].ReasonCode
		if gotR != wantR {
			t.Fatalf("%s action %d (%s): got reasonCode=%s, want %s",
				label, idx, plan.Decisions[idx].AppleIdentifier, gotR, wantR)
		}
	}

	// Bidirectional strict full-tree equality + per-action pre-overlay equality for every NATURAL_VERIFIED action.
	for _, pair := range []struct {
		dialect string
		out     map[string]any
	}{
		{"legacy", legMap},
		{"v2", v2Map},
	} {
		if !plistValuesStrictlyEqual(doc.RawRoot, pair.out) {
			t.Fatalf("%s %s full-tree strict parity mismatch", label, pair.dialect)
		}
		preActs := preByDialect[pair.dialect]
		for i, dec := range plan.Decisions {
			if dec.Classification != "NATURAL_VERIFIED" {
				continue
			}
			preAct, ok := preActs[i].(map[string]any)
			if !ok {
				t.Fatalf("%s %s pre-overlay action %d not a map", label, pair.dialect, i)
			}
			if err := verifyPreOverlayActionSemanticParity(label, pair.dialect, i, doc.Actions[i], preAct); err != nil {
				t.Fatalf("%s pre-overlay parity failure: %v", label, err)
			}
		}
	}

	if inspectPlan != nil {
		legSrc, err := RenderHybridLegacySource(doc, plan)
		if err != nil {
			t.Fatalf("%s RenderHybridLegacySource: %v", label, err)
		}
		v2Src, err := RenderHybridV2Source(doc, plan)
		if err != nil {
			t.Fatalf("%s RenderHybridV2Source: %v", label, err)
		}
		inspectPlan(t, plan, legSrc, v2Src)
	}
	return plan
}

func TestP12_PAR_01_WFTextTokenStringUTF16EmojiAndBraces(t *testing.T) {
	u0 := "11111111-1111-4111-8111-000000000000"
	u1 := "11111111-1111-4111-8111-000000000001"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u0,
				"WFTextActionText": "alpha",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "userTag",
				"WFInput":        p12RawActionOutAttachment(u0, "Text"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u1,
				"WFTextActionText": p12TextTokenStringWithVar("🎉 {a:1} 👋 ", "userTag", " & {hello world} 🚀"),
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_01", actions,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED"}, nil,
		func(t *testing.T, plan *HybridImportPlan, legSrc, v2Src string) {
			if !strings.Contains(v2Src, "{{a:1}}") {
				t.Fatalf("expected V2 f-string escaped literal braces in %s", v2Src)
			}
			if !strings.Contains(legSrc, "{a:1}") || !strings.Contains(legSrc, "{@userTag}") {
				t.Fatalf("expected Legacy literal brace + variable interpolation in %s", legSrc)
			}
		},
	)
}

func TestP12_PAR_02_WFTextTokenStringLiteralBraceCollisionFailsClosed(t *testing.T) {
	u1 := "22222222-2222-4222-8222-000000000001"
	u2 := "22222222-2222-4222-8222-000000000002"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u1,
				"WFTextActionText": "val",
			},
		},
		// Collision: literal "{var_1}" inside WFTextTokenString with real attachment triggers Legacy checkInlineVar.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u2,
				"WFTextActionText": p12TextTokenStringWithOut("literal {var_1} and ", u1, "Text", " end"),
			},
		},
		// Direct literal WFTextTokenString inside setvariable compiles to 2 actions in Legacy -> must fail Stage 1 (not Stage 3).
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "directLit",
				"WFInput": map[string]any{
					"Value":               map[string]any{"string": "alpha"},
					"WFSerializationType": "WFTextTokenString",
				},
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_02", actions,
		map[int]string{
			0: "NATURAL_VERIFIED",
			1: "EXACT_NATIVE_FALLBACK",
			2: "EXACT_NATIVE_FALLBACK",
		},
		map[int]string{
			1: "UNREPRESENTABLE_PARAM",
			2: "UNREPRESENTABLE_PARAM",
		},
		nil,
	)
}

func TestP12_PAR_03_RTLBidiTextAndDictKeysPreserved(t *testing.T) {
	u0 := "33333333-3333-4333-8333-000000000000"
	u1 := "33333333-3333-4333-8333-000000000001"
	u2 := "33333333-3333-4333-8333-000000000002"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u0,
				"WFTextActionText": "ירושלים",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "cityVar",
				"WFInput":        p12RawActionOutAttachment(u0, "Text"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u1,
				"WFTextActionText": p12TextTokenStringWithVar("שלום 👋 [מזרח] ", "cityVar", " — 100% ב-Shortcuts!"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u2,
				"WFItems": p12DictWrapper([]any{
					p12DictFieldItem(0, "אבן", p12TextTokenStringLiteral("значение 🎉")),
					p12DictFieldItem(0, "שלום", p12TextTokenStringLiteral("مرحبا")),
				}),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "rtlDict",
				"WFInput":        p12RawActionOutAttachment(u2, "Dictionary"),
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_03", actions,
		map[int]string{
			0: "NATURAL_VERIFIED",
			1: "NATURAL_VERIFIED",
			2: "NATURAL_VERIFIED",
			3: "NATURAL_VERIFIED",
			4: "NATURAL_VERIFIED",
		},
		nil, nil,
	)
}

func TestP12_PAR_04_PromptMultilineBoolDefaultAndInputTypeAbsence(t *testing.T) {
	u1 := "44444444-4444-4444-8444-000000000001"
	u2 := "44444444-4444-4444-8444-000000000002"
	u3 := "44444444-4444-4444-8444-000000000003"
	actions := []any{
		// Single-line prompt: explicit WFAllowsMultilineText=false (bool), WFInputType omitted.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                  u1,
				"WFAskActionPrompt":     "Enter short title",
				"WFAllowsMultilineText": false,
			},
		},
		// Prompt with default text + multiline true + WFInputType="Text".
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                     u2,
				"WFAskActionPrompt":        "Enter note",
				"WFAskActionDefaultAnswer": "default line",
				"WFInputType":              "Text",
				"WFAllowsMultilineText":    true,
			},
		},
		// Numeric prompt with WFInputType="Number" and WFAskActionDefaultAnswerNumber="42".
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                           u3,
				"WFAskActionPrompt":              "Enter count",
				"WFAskActionDefaultAnswerNumber": "42",
				"WFInputType":                    "Number",
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_04", actions,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED"},
		nil,
		func(t *testing.T, plan *HybridImportPlan, legSrc, v2Src string) {
			if !strings.Contains(v2Src, "multiline: false") || !strings.Contains(legSrc, "prompt(") {
				t.Fatalf("expected typed multiline: false prompt in rendered source:\n%s", v2Src)
			}
		},
	)
}

func TestP12_PAR_05_ListMixedTextTokenAndAttachmentRejection(t *testing.T) {
	u1 := "55555555-5555-4555-8555-000000000001"
	u2 := "55555555-5555-4555-8555-000000000002"
	u3 := "55555555-5555-4555-8555-000000000003"
	u4 := "55555555-5555-4555-8555-000000000004"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u1,
				"WFTextActionText": "itemTwo",
			},
		},
		// Valid list with WFItemType=0 plain string and interpolated WFTextTokenString.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u2,
				"WFItems": []any{
					map[string]any{"WFItemType": int64(0), "WFValue": "staticItem"},
					map[string]any{"WFItemType": int64(0), "WFValue": p12TextTokenStringWithOut("prefix:", u1, "Text", "")},
				},
			},
		},
		// Unsupported bare string item -> must fail closed to EXACT_NATIVE_FALLBACK.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":    u3,
				"WFItems": []any{"bareString"},
			},
		},
		// Unsupported WFTextTokenAttachment item inside WFItemType=0 -> must fail closed to EXACT_NATIVE_FALLBACK.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u4,
				"WFItems": []any{
					map[string]any{
						"WFItemType": int64(0),
						"WFValue":    p12RawActionOutAttachment(u1, "Text"),
					},
				},
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_05", actions,
		map[int]string{
			0: "NATURAL_VERIFIED",
			1: "NATURAL_VERIFIED",
			2: "EXACT_NATIVE_FALLBACK",
			3: "EXACT_NATIVE_FALLBACK",
		},
		map[int]string{
			2: "UNREPRESENTABLE_PARAM",
			3: "UNREPRESENTABLE_PARAM",
		},
		nil,
	)
}

func TestP12_PAR_06_TypedVarOutputAndDictAggrandizementProvenance(t *testing.T) {
	u1 := "66666666-6666-4666-8666-000000000001"
	u2 := "66666666-6666-4666-8666-000000000002"
	u3 := "66666666-6666-4666-8666-000000000003"
	u4 := "66666666-6666-4666-8666-000000000004"
	actions := []any{
		// Empty string dictionary emits empty String and empty WFDictionaryFieldValueItems (never omitted).
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":    u1,
				"WFItems": p12DictWrapper([]any{}),
			},
		},
		// Empty WFTextTokenStringValue.String inside text with single variable/literal empty token.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.appendvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "bucket",
				"WFInput":        p12RawActionOutAttachment(u1, "Dictionary"),
			},
		},
		// Variable with WFDictionaryValueKey aggrandizement.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u2,
				"WFTextActionText": map[string]any{
					"Value": map[string]any{
						"attachmentsByRange": map[string]any{
							"{0, 1}": map[string]any{
								"OutputName": "Dictionary",
								"OutputUUID": u1,
								"Type":       "ActionOutput",
								"Aggrandizements": []any{
									map[string]any{
										"CoercionItemClass": "WFDictionaryContentItem",
										"Type":              "WFCoercionVariableAggrandizement",
									},
									map[string]any{
										"DictionaryKey": "nestedKey",
										"Type":          "WFDictionaryValueVariableAggrandizement",
									},
								},
							},
						},
						"string": "\ufffc",
					},
					"WFSerializationType": "WFTextTokenString",
				},
			},
		},
		// Reference the appended variable ("Variable" type rather than "ActionOutput").
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u3,
				"WFTextActionText": p12TextTokenStringWithVar("", "bucket", " done"),
			},
		},
		// Literal empty string gettext.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u4,
				"WFTextActionText": "",
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_06", actions,
		map[int]string{
			0: "NATURAL_VERIFIED",
			1: "NATURAL_VERIFIED",
			2: "NATURAL_VERIFIED",
			3: "NATURAL_VERIFIED",
			4: "NATURAL_VERIFIED",
		},
		nil, nil,
	)
}

func TestP12_PAR_07_AbsentVsFalseAndZeroNumericTriState(t *testing.T) {
	u1 := "77777777-7777-4777-8777-000000000001"
	actions := []any{
		// Numeric zero vs float vs string-encoded number (promoted in P10R13 via number("1") preserving string wire type).
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.number",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                 u1,
				"WFNumberActionNumber": int64(0),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.number",
			"WFWorkflowActionParameters": map[string]any{
				"WFNumberActionNumber": float64(3.5),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.number",
			"WFWorkflowActionParameters": map[string]any{
				"WFNumberActionNumber": "1",
			},
		},
		// Alert with omitted cancel button vs explicit false.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.alert",
			"WFWorkflowActionParameters": map[string]any{
				"WFAlertActionMessage":           "Explicit cancel shown false",
				"WFAlertActionTitle":             "Notice",
				"WFAlertActionCancelButtonShown": false,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.alert",
			"WFWorkflowActionParameters": map[string]any{
				"WFAlertActionMessage": "Cancel button omitted",
				"WFAlertActionTitle":   "Notice",
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_07", actions,
		map[int]string{
			0: "NATURAL_VERIFIED",
			1: "NATURAL_VERIFIED",
			2: "NATURAL_VERIFIED",
			3: "NATURAL_VERIFIED",
			4: "EXACT_NATIVE_FALLBACK",
		},
		map[int]string{
			4: "AMBIGUOUS_VARIANT",
		},
		nil,
	)
}

func TestP12_PAR_08_NestedDictFailClosedAndSortedScalarsPromote(t *testing.T) {
	u1 := "88888888-8888-4888-8888-000000000001"
	u2 := "88888888-8888-4888-8888-000000000002"
	u3 := "88888888-8888-4888-8888-000000000003"
	u4 := "88888888-8888-4888-8888-000000000004"
	u5 := "88888888-8888-4888-8888-000000000005"

	// Sorted scalar+bool+number dictionary promotes cleanly to NATURAL_VERIFIED when consumed.
	sortedDict := p12DictWrapper([]any{
		p12DictFieldItem(4, "flag", map[string]any{"Value": true, "WFSerializationType": "WFNumberSubstitutableState"}),
		p12DictFieldItem(0, "greeting", p12TextTokenStringLiteral("hi there")),
		p12DictFieldItem(3, "num", p12TextTokenStringLiteral("7")),
	})

	// Uninterpolated literal brace inside JSON dict value must fail closed at Stage 1 (avoid single-quoted invalid JSON).
	braceInDict := p12DictWrapper([]any{
		p12DictFieldItem(0, "greeting", p12TextTokenStringLiteral("hi {a-b}")),
	})

	// Unsorted keys fail closed (Cherri map iteration sorts keys on compile).
	unsortedDict := p12DictWrapper([]any{
		p12DictFieldItem(0, "z_key", p12TextTokenStringLiteral("1")),
		p12DictFieldItem(0, "a_key", p12TextTokenStringLiteral("2")),
	})

	// Explicit empty attachmentsByRange map on WFKey fails closed cleanly at Stage 1 (not Stage 3).
	explicitEmptyAttDict := map[string]any{
		"Value": map[string]any{
			"WFDictionaryFieldValueItems": []any{
				map[string]any{
					"WFItemType": int64(0),
					"WFKey": map[string]any{
						"Value": map[string]any{
							"attachmentsByRange": map[string]any{},
							"string":             "k",
						},
						"WFSerializationType": "WFTextTokenString",
					},
					"WFValue": p12TextTokenStringLiteral("v"),
				},
			},
		},
		"WFSerializationType": "WFDictionaryFieldValue",
	}

	// Nested dict (WFItemType=1) fails closed cleanly at Stage 1 across Legacy & V2 wrapper shape difference.
	nestedDict := p12DictWrapper([]any{
		p12DictFieldItem(1, "inner", p12DictWrapper([]any{p12DictFieldItem(0, "sub", p12TextTokenStringLiteral("val"))})),
	})

	actions := []any{
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary", "WFWorkflowActionParameters": map[string]any{"UUID": u1, "WFItems": sortedDict}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary", "WFWorkflowActionParameters": map[string]any{"UUID": u2, "WFItems": braceInDict}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary", "WFWorkflowActionParameters": map[string]any{"UUID": u3, "WFItems": unsortedDict}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary", "WFWorkflowActionParameters": map[string]any{"UUID": u4, "WFItems": explicitEmptyAttDict}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary", "WFWorkflowActionParameters": map[string]any{"UUID": u5, "WFItems": nestedDict}},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "useSorted",
				"WFInput":        p12RawActionOutAttachment(u1, "Dictionary"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "useBraceDict",
				"WFInput":        p12RawActionOutAttachment(u2, "Dictionary"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "useUnsorted",
				"WFInput":        p12RawActionOutAttachment(u3, "Dictionary"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "useEmptyAtt",
				"WFInput":        p12RawActionOutAttachment(u4, "Dictionary"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "useNested",
				"WFInput":        p12RawActionOutAttachment(u5, "Dictionary"),
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_08", actions,
		map[int]string{
			0: "NATURAL_VERIFIED",
			1: "EXACT_NATIVE_FALLBACK",
			2: "EXACT_NATIVE_FALLBACK",
			3: "EXACT_NATIVE_FALLBACK",
			4: "EXACT_NATIVE_FALLBACK",
		},
		nil, nil,
	)
}

func TestP12_PAR_09_EnumExtensionsAndOpenAppNoStage3Demotion(t *testing.T) {
	u1 := "99999999-9999-4999-8999-000000000001"
	u2 := "99999999-9999-4999-8999-000000000002"
	u3 := "99999999-9999-4999-8999-000000000003"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.date",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u1,
				"WFDateActionDate": "2026-11-15",
				"WFDateActionMode": "Specified Date",
			},
		},
		// Uses extended enum value "Total Time" preserved outside upstream calendar.cherri.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettimebetweendates",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                u2,
				"WFInput":             p12ActionOutAttachment(u1, "Date"),
				"WFTimeUntilFromDate": "Now",
				"WFTimeUntilUnit":     "Total Time",
			},
		},
		// openApp has defaultAction=true (def.identifier==""), must NOT trigger Stage-3 demotion or wrong key handling.
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.openapp",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u3,
				"WFSelectedApp": map[string]any{
					"BundleIdentifier": "com.apple.mobilenotes",
					"Name":             "Notes",
				},
			},
		},
	}

	// Authoring smoke check: "Total Time" and extended contactDetail choices must compile cleanly in BOTH dialects.
	for _, legacy := range []bool{false, true} {
		src := "let d = date(\"2026-11-15\")\nlet _t = getTimeBetweenDates(d, \"Now\", \"Total Time\")\nlet _c = getContactDetail(Ask, \"Contact Photo\")\n"
		if legacy {
			src = "#define version 27\n#include 'actions/calendar'\n#include 'actions/contacts'\n@d = date(\"2026-11-15\")\n@t = getTimeBetweenDates(@d, \"Now\", \"Total Time\")\n@c = getContactDetail(Ask, \"Contact Photo\")\n"
		}
		_ = compileHybridDialectStrict(t, fmt.Sprintf("PAR_09_author_%v", legacy), src, legacy)
	}

	doc := p12BuildNativeDoc(t, "PAR_09_roundtrip", actions)
	plan, legMap, v2Map, _, err := verifyCorpusDualHybridWithPreOverlay(doc, "PAR_09_roundtrip", true)
	if err != nil {
		t.Fatalf("PAR_09_roundtrip failed: %v", err)
	}
	if plan.Lifecycle != nil && plan.Lifecycle.DemotedActionCount != 0 {
		t.Fatalf("PAR_09 unexpected Stage-3 demotion: %+v", plan.Lifecycle)
	}
	if !plistValuesStrictlyEqual(doc.RawRoot, legMap) || !plistValuesStrictlyEqual(doc.RawRoot, v2Map) {
		t.Fatalf("PAR_09 strict roundtrip mismatch")
	}
}

func TestP12_PAR_10_ThirdPartyUnknownActionPreservedUnchanged(t *testing.T) {
	u1 := "aaaaaaaa-aaaa-4aaa-8aaa-000000000001"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "com.thirdparty.widget.CustomIntent",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u1,
				"AppIntentDescriptor": map[string]any{
					"AppIntentIdentifier": "CustomIntent",
					"BundleIdentifier":    "com.thirdparty.widget",
					"Name":                "Widget",
					"TeamIdentifier":      "ABCDE12345",
				},
				"customPayload": map[string]any{
					"nestedFlag": true,
					"count":      int64(3),
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.dropbox.savefile",
			"WFWorkflowActionParameters": map[string]any{
				"WFAskWhereToSave": false,
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_10", actions,
		map[int]string{
			0: "EXACT_NATIVE_FALLBACK",
			1: "EXACT_NATIVE_FALLBACK",
		},
		map[int]string{
			0: "THIRD_PARTY_NO_CANONICAL_DEF",
		},
		nil,
	)
}

func TestP12_PAR_11_DualDialectStrictPreOverlayNoDemotionGate(t *testing.T) {
	u1 := "bbbbbbbb-bbbb-4bbb-8bbb-000000000001"
	u2 := "bbbbbbbb-bbbb-4bbb-8bbb-000000000002"
	u3 := "bbbbbbbb-bbbb-4bbb-8bbb-000000000003"
	u4 := "bbbbbbbb-bbbb-4bbb-8bbb-000000000004"
	gCond := "cccccccc-cccc-4ccc-8ccc-000000000001"

	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                  u1,
				"WFAskActionPrompt":     "Name?",
				"WFAllowsMultilineText": false,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u2,
				"WFItems": []any{
					map[string]any{"WFItemType": int64(0), "WFValue": "first"},
					map[string]any{"WFItemType": int64(0), "WFValue": p12TextTokenStringWithOut("🎉 ", u1, "Provided Input", " [end]")},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":                    gCond,
				"WFControlFlowMode":                     int64(0),
				"WFInput":                               p12ActionOutAttachment(u1, "Provided Input"),
				"WFCondition":                           int64(4),
				"WFConditionalActionString":             "admin",
				"WFConditionalLegacyComparisonBehavior": true,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u3,
				"WFTextActionText": "matched 🎉 {role}",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": gCond,
				"WFControlFlowMode":  int64(2),
				"UUID":               u4,
			},
		},
	}

	assertP12ParDualVerified(t, "PAR_11", actions,
		map[int]string{
			0: "NATURAL_VERIFIED",
			1: "NATURAL_VERIFIED",
			2: "NATURAL_VERIFIED",
			3: "NATURAL_VERIFIED",
			4: "NATURAL_VERIFIED",
		},
		nil, nil,
	)
}

func TestP12_PAR_12_CorpusComplexParamsAndZeroStage3Gate(t *testing.T) {
	if os.Getenv("CHERRI_P10R10_CORPUS") != "1" {
		t.Log("NOT_RUN_PRIVATE_CORPUS: set CHERRI_P10R10_CORPUS=1 for full 38-shortcut P10R12 parity and zero-demotion verification")
		return
	}
	entries, err := os.ReadDir(p10r10CorpusDir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".plist") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatalf("no corpus plists found in %s", p10r10CorpusDir)
	}

	var totalActions, naturalTotal, fallbackTotal, invalidTotal, demotedTotal, thirdPartyCount int
	var reasonTotals = map[string]int{}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(p10r10CorpusDir, name))
		if err != nil {
			t.Fatalf("%s read: %v", name, err)
		}
		doc, err := DecodeNativeShortcutDocument(raw)
		if err != nil {
			t.Fatalf("%s decode: %v", name, err)
		}
		plan, legMap, v2Map, preByDialect, err := verifyCorpusDualHybridWithPreOverlay(doc, name, true)
		if err != nil {
			t.Fatalf("%s dual hybrid pre-overlay verification failed: %v", name, err)
		}
		if !plistValuesStrictlyEqual(doc.RawRoot, legMap) {
			t.Fatalf("%s legacy full-tree strict parity failed", name)
		}
		if !plistValuesStrictlyEqual(doc.RawRoot, v2Map) {
			t.Fatalf("%s v2 full-tree strict parity failed", name)
		}
		if plan.Lifecycle != nil && plan.Lifecycle.DemotedActionCount > 0 {
			demotedTotal += plan.Lifecycle.DemotedActionCount
			t.Fatalf("%s had Stage-3 demotions=%d (%v)", name, plan.Lifecycle.DemotedActionCount, plan.Lifecycle.DemotionReasons)
		}
		for _, dialect := range []string{"legacy", "v2"} {
			preActs := preByDialect[dialect]
			for i, dec := range plan.Decisions {
				if dec.Classification != "NATURAL_VERIFIED" {
					continue
				}
				preAct, ok := preActs[i].(map[string]any)
				if !ok {
					t.Fatalf("%s %s action %d missing pre-overlay map", name, dialect, i)
				}
				if err := verifyPreOverlayActionSemanticParity(name, dialect, i, doc.Actions[i], preAct); err != nil {
					t.Fatalf("%s pre-overlay mismatch: %v", name, err)
				}
			}
		}

		totalActions += len(plan.Decisions)
		for _, d := range plan.Decisions {
			switch d.Classification {
			case "NATURAL_VERIFIED":
				naturalTotal++
			case "EXACT_NATIVE_FALLBACK":
				fallbackTotal++
			case "SOURCE_INVALID":
				invalidTotal++
			}
			if d.ReasonCode == "THIRD_PARTY_NO_CANONICAL_DEF" {
				thirdPartyCount++
			}
			if d.ReasonCode != "" {
				reasonTotals[d.ReasonCode]++
			}
		}
	}

	if demotedTotal != 0 {
		t.Fatalf("expected 0 Stage-3 demotions, got %d", demotedTotal)
	}
	if naturalTotal <= 22213 {
		t.Fatalf("expected NATURAL_VERIFIED > P10R11/P12-baseline 22213, got %d", naturalTotal)
	}
	if thirdPartyCount == 0 {
		t.Fatalf("expected non-zero THIRD_PARTY_NO_CANONICAL_DEF preserved")
	}
	t.Logf("P12_PAR_12 corpus verified: files=%d actions=%d natural=%d fallback=%d invalid=%d demotions=%d thirdParty=%d reasons=%v",
		len(names), totalActions, naturalTotal, fallbackTotal, invalidTotal, demotedTotal, thirdPartyCount, reasonTotals)
}

func TestP12_EDIT_05_EditPromptMultilineBoolAndDefaultIsolated(t *testing.T) {
	u1 := "e5000001-0000-4000-8000-000000000001"
	u2 := "e5000001-0000-4000-8000-000000000002"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                     u1,
				"WFAskActionPrompt":        "Original question?",
				"WFAskActionDefaultAnswer": "seed",
				"WFAllowsMultilineText":    false,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "untouched sibling " + u2,
			},
		},
	}
	doc := p12BuildNativeDoc(t, "EDIT_05", actions)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatalf("PlanHybridImport: %v", err)
	}
	for _, legacy := range []bool{false, true} {
		var src string
		if legacy {
			src, err = RenderHybridLegacySource(doc, plan)
		} else {
			src, err = RenderHybridV2Source(doc, plan)
		}
		if err != nil {
			t.Fatalf("render legacy=%v: %v", legacy, err)
		}
		edited := strings.Replace(src, `"Original question?"`, `"Edited question?"`, 1)
		edited = strings.Replace(edited, "false", "true", 1)
		if edited == src {
			t.Fatalf("edit replacement had no effect in legacy=%v:\n%s", legacy, src)
		}
		out := compileHybridDialectStrict(t, fmt.Sprintf("EDIT_05_%v", legacy), edited, legacy)
		outActs := out["WFWorkflowActions"].([]any)
		if len(outActs) != 2 {
			t.Fatalf("expected 2 actions after edit, got %d", len(outActs))
		}
		p0 := outActs[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p0["WFAskActionPrompt"] != "Edited question?" {
			t.Fatalf("WFAskActionPrompt=%v, want Edited question?", p0["WFAskActionPrompt"])
		}
		if ml, ok := p0["WFAllowsMultilineText"].(bool); !ok || !ml {
			t.Fatalf("WFAllowsMultilineText=%T(%v), want bool(true)", p0["WFAllowsMultilineText"], p0["WFAllowsMultilineText"])
		}
		if !plistValuesStrictlyEqual(actions[1], outActs[1]) {
			t.Fatalf("sibling comment action mutated unexpectedly: %#v", outActs[1])
		}
	}
}

func TestP12_EDIT_06_EditListItemAddReorderIsolated(t *testing.T) {
	u1 := "e6000001-0000-4000-8000-000000000001"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u1,
				"WFItems": []any{
					map[string]any{"WFItemType": int64(0), "WFValue": "alpha"},
					map[string]any{"WFItemType": int64(0), "WFValue": "beta"},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.alert",
			"WFWorkflowActionParameters": map[string]any{
				"WFAlertActionMessage":           "keep alert",
				"WFAlertActionCancelButtonShown": false,
			},
		},
	}
	doc := p12BuildNativeDoc(t, "EDIT_06", actions)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatalf("PlanHybridImport: %v", err)
	}
	for _, legacy := range []bool{false, true} {
		var src string
		if legacy {
			src, err = RenderHybridLegacySource(doc, plan)
		} else {
			src, err = RenderHybridV2Source(doc, plan)
		}
		if err != nil {
			t.Fatalf("render legacy=%v: %v", legacy, err)
		}
		edited := strings.Replace(src, `("alpha", "beta")`, `("beta", "gamma", "alpha")`, 1)
		if edited == src {
			t.Fatalf("list replacement did not match source legacy=%v:\n%s", legacy, src)
		}
		out := compileHybridDialectStrict(t, fmt.Sprintf("EDIT_06_%v", legacy), edited, legacy)
		outActs := out["WFWorkflowActions"].([]any)
		items := outActs[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFItems"].([]any)
		if len(items) != 3 {
			t.Fatalf("expected 3 list items, got %d", len(items))
		}
		for idx, want := range []string{"beta", "gamma", "alpha"} {
			m := items[idx].(map[string]any)
			if fmt.Sprint(m["WFItemType"]) != "0" || m["WFValue"] != want {
				t.Fatalf("item %d = %#v, want WFItemType=0 WFValue=%q", idx, m, want)
			}
		}
		if !plistValuesStrictlyEqual(actions[1], outActs[1]) {
			t.Fatalf("adjacent alert action mutated during list edit")
		}
	}
}

func TestP12_EDIT_07_EditInterpolatedTextWithBracesAndEmojiIsolated(t *testing.T) {
	u0 := "e7000001-0000-4000-8000-000000000000"
	u1 := "e7000001-0000-4000-8000-000000000001"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u0,
				"WFTextActionText": "VIP",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "badge",
				"WFInput":        p12RawActionOutAttachment(u0, "Text"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u1,
				"WFTextActionText": p12TextTokenStringWithVar("🎉 {old-tag} ", "badge", " end"),
			},
		},
	}
	doc := p12BuildNativeDoc(t, "EDIT_07", actions)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatalf("PlanHybridImport: %v", err)
	}
	for _, legacy := range []bool{false, true} {
		var src string
		if legacy {
			src, err = RenderHybridLegacySource(doc, plan)
		} else {
			src, err = RenderHybridV2Source(doc, plan)
		}
		if err != nil {
			t.Fatalf("render legacy=%v: %v", legacy, err)
		}
		edited := src
		if legacy {
			edited = strings.Replace(edited, "{old-tag} {@badge}", "{שלום} 🚀 {@badge}", 1)
		} else {
			edited = strings.Replace(edited, "{{old-tag}} {badge}", "{{שלום}} 🚀 {badge}", 1)
		}
		if edited == src {
			t.Fatalf("text edit replacement had no effect legacy=%v:\n%s", legacy, src)
		}
		out := compileHybridDialectStrict(t, fmt.Sprintf("EDIT_07_%v", legacy), edited, legacy)
		outActs := out["WFWorkflowActions"].([]any)
		tok := outActs[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"].(map[string]any)
		val := tok["Value"].(map[string]any)
		wantPrefix := "🎉 {שלום} 🚀 "
		wantPos := fmt.Sprintf("{%d, 1}", p12UTF16Len(wantPrefix))
		atts := val["attachmentsByRange"].(map[string]any)
		if _, ok := atts[wantPos]; !ok {
			t.Fatalf("expected attachment at UTF-16 range %s, got %#v (string=%q)", wantPos, atts, val["string"])
		}
		if !plistValuesStrictlyEqual(actions[0], outActs[0]) || !plistValuesStrictlyEqual(actions[1], outActs[1]) {
			t.Fatalf("prior actions mutated on gettext edit")
		}
	}
}

func TestP12_EDIT_08_DuplicateReorderRemoveNaturalActionPreservesAdjacentFallback(t *testing.T) {
	u1 := "e8000001-0000-4000-8000-000000000001"
	u2 := "e8000001-0000-4000-8000-000000000002"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             u1,
				"WFTextActionText": "first-step",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "com.thirdparty.demo.Action",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u2,
				"AppIntentDescriptor": map[string]any{
					"AppIntentIdentifier": "Action",
					"BundleIdentifier":    "com.thirdparty.demo",
					"Name":                "Demo",
				},
				"mode": "strict-keep",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "remove-me",
			},
		},
	}
	doc := p12BuildNativeDoc(t, "EDIT_08", actions)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatalf("PlanHybridImport: %v", err)
	}
	if plan.Decisions[1].Classification != "EXACT_NATIVE_FALLBACK" || plan.Decisions[1].ReasonCode != "THIRD_PARTY_NO_CANONICAL_DEF" {
		t.Fatalf("unexpected classification for third-party fallback: %+v", plan.Decisions[1])
	}

	for _, legacy := range []bool{false, true} {
		var src string
		if legacy {
			src, err = RenderHybridLegacySource(doc, plan)
		} else {
			src, err = RenderHybridV2Source(doc, plan)
		}
		if err != nil {
			t.Fatalf("render legacy=%v: %v", legacy, err)
		}
		lines := strings.Split(src, "\n")
		var thirdIdx, firstIdx, remIdx = -1, -1, -1
		for idx, ln := range lines {
			switch {
			case strings.Contains(ln, `"first-step"`):
				firstIdx = idx
			case strings.Contains(ln, "com.thirdparty.demo.Action") || strings.Contains(ln, "strict-keep"):
				thirdIdx = idx
			case strings.Contains(ln, `"remove-me"`):
				remIdx = idx
			}
		}
		if firstIdx < 0 || thirdIdx < 0 || remIdx < 0 {
			t.Fatalf("failed to locate expected lines in rendered source legacy=%v:\n%s", legacy, src)
		}
		// Remove "remove-me", duplicate "first-step" AFTER the third-party fallback.
		firstLine := lines[firstIdx]
		var rebuilt []string
		for idx, ln := range lines {
			if idx == remIdx {
				continue
			}
			rebuilt = append(rebuilt, ln)
			if idx == thirdIdx {
				rebuilt = append(rebuilt, firstLine)
			}
		}
		out := compileHybridDialectStrict(t, fmt.Sprintf("EDIT_08_%v", legacy), strings.Join(rebuilt, "\n"), legacy)
		outActs := out["WFWorkflowActions"].([]any)
		if len(outActs) != 3 {
			t.Fatalf("expected 3 actions (first, fallback, duplicated first), got %d", len(outActs))
		}
		if !plistValuesStrictlyEqual(actions[1], outActs[1]) {
			t.Fatalf("third-party fallback action mutated during surrounding duplicate/remove edit: %#v", outActs[1])
		}
	}
}

func TestP12_PAR_13_ForwardActionOutputAndCLIImportParity(t *testing.T) {
	u0 := "f1300001-0000-4000-8000-000000000001"
	u1 := "f1300001-0000-4000-8000-000000000002"
	actions := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": u0,
				"WFAskActionPrompt": map[string]any{
					"Value": map[string]any{
						"string": "Read forward \ufffc?",
						"attachmentsByRange": map[string]any{
							"{13, 1}": map[string]any{
								"OutputName": "Ask for Input",
								"OutputUUID": u1,
								"Type":       "ActionOutput",
							},
						},
					},
					"WFSerializationType": "WFTextTokenString",
				},
				"WFAskActionDefaultAnswer": "",
				"WFAllowsMultilineText":    true,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":                     u1,
				"WFAskActionPrompt":        "Target value?",
				"WFAskActionDefaultAnswer": "seed",
				"WFAllowsMultilineText":    false,
			},
		},
	}
	doc := p12BuildNativeDoc(t, "PAR_13_FWD", actions)

	origArgs := args.Args
	defer func() { args.Args = origArgs }()

	var renderedByVariant = make(map[string]string)
	for _, variant := range []string{"no_import_arg", "with_import_arg"} {
		resetCompilerStateFully()
		ClearHybridCandidateCache()
		if variant == "with_import_arg" {
			args.Args = map[string]string{"import": "synthetic.plist", "no-ansi": ""}
		} else {
			args.Args = map[string]string{"no-ansi": ""}
		}
		plan, err := PlanHybridImport(doc)
		if err != nil {
			t.Fatalf("%s PlanHybridImport: %v", variant, err)
		}
		if plan.Lifecycle != nil && plan.Lifecycle.DemotedActionCount != 0 {
			t.Fatalf("%s expected 0 Stage-3 demotions, got %d (%v)", variant, plan.Lifecycle.DemotedActionCount, plan.Lifecycle.DemotionReasons)
		}
		if plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" || plan.Decisions[0].ReasonCode != "UNREPRESENTABLE_PARAM" {
			t.Fatalf("%s expected forward-ref consumer at Stage-1 UNREPRESENTABLE_PARAM, got %+v", variant, plan.Decisions[0])
		}
		if plan.Decisions[1].Classification != "NATURAL_VERIFIED" {
			t.Fatalf("%s expected producer ask at index 1 to remain NATURAL_VERIFIED, got %+v", variant, plan.Decisions[1])
		}
		if _, hasAskCap := plan.Symbols["Ask1"]; hasAskCap {
			t.Fatalf("%s unexpected capitalized Ask1 identifier leak in plan.Symbols: %v", variant, plan.Symbols)
		}
		legSrc, err := RenderHybridLegacySource(doc, plan)
		if err != nil {
			t.Fatalf("%s render legacy: %v", variant, err)
		}
		v2Src, err := RenderHybridV2Source(doc, plan)
		if err != nil {
			t.Fatalf("%s render v2: %v", variant, err)
		}
		renderedByVariant[variant+":legacy"] = legSrc
		renderedByVariant[variant+":v2"] = v2Src

		for _, legacy := range []bool{true, false} {
			src := legSrc
			if !legacy {
				src = v2Src
			}
			out := compileHybridDialectStrict(t, fmt.Sprintf("%s_%v", variant, legacy), src, legacy)
			if !plistValuesStrictlyEqual(doc.RawRoot, out) {
				t.Fatalf("%s legacy=%v full-tree strict parity failed", variant, legacy)
			}
		}
	}
	if renderedByVariant["no_import_arg:legacy"] != renderedByVariant["with_import_arg:legacy"] {
		t.Fatalf("legacy rendered source differed by args.Args['import'] presence")
	}
	if renderedByVariant["no_import_arg:v2"] != renderedByVariant["with_import_arg:v2"] {
		t.Fatalf("v2 rendered source differed by args.Args['import'] presence")
	}
}
