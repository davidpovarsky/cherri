/*
 * Copyright (c) Cherri Language
 * P10R12 P0 Control Regions & Control Edit Isolation Permanent Regression Suite (P12-CF-01..22, P12-EDIT-01..04)
 */

package main

import (
	"strings"
	"testing"

	"howett.net/plist"
)

func assertP12CFDualVerified(
	t *testing.T,
	specimen string,
	doc *NativeShortcutDocument,
	wantClassificationByIdx map[int]string,
	wantLegSubstrs []string,
	wantV2Substrs []string,
	forbidBothSubstrs []string,
) (*HybridImportPlan, string, string) {
	t.Helper()
	plan, _, _, preByDialect, err := verifyCorpusDualHybridWithPreOverlay(doc, specimen, true)
	if err != nil {
		t.Fatalf("%s dual hybrid parity failed: %v", specimen, err)
	}
	for idx, wantCls := range wantClassificationByIdx {
		if idx < 0 || idx >= len(plan.Decisions) {
			t.Fatalf("%s idx %d out of range (%d decisions)", specimen, idx, len(plan.Decisions))
		}
		got := plan.Decisions[idx].Classification
		if got != wantCls {
			v2Dbg, _ := RenderHybridV2Source(doc, plan)
			t.Logf("%s failing decision[%d] V2=%q Leg=%q\n--- V2 ---\n%s", specimen, idx, plan.Decisions[idx].V2Call, plan.Decisions[idx].LegacyCall, v2Dbg)
			t.Fatalf("%s action %d (%s): got classification=%s (reason=%s stage=%s demote=%s), want %s",
				specimen, idx, doc.Actions[idx].Identifier, got,
				plan.Decisions[idx].ReasonCode, plan.Decisions[idx].DemotionStage, plan.Decisions[idx].DemotionReason, wantCls)
		}
	}
	for _, dialect := range []string{"legacy", "v2"} {
		preActs := preByDialect[dialect]
		for i, dec := range plan.Decisions {
			if dec.Classification != "NATURAL_VERIFIED" {
				continue
			}
			preAct, _ := preActs[i].(map[string]any)
			if err := verifyPreOverlayActionSemanticParity(specimen, dialect, i, doc.Actions[i], preAct); err != nil {
				t.Fatalf("%s pre-overlay semantic mismatch (%s idx=%d): %v", specimen, dialect, i, err)
			}
		}
	}

	legSrc, err := RenderHybridLegacySource(doc, plan)
	if err != nil {
		t.Fatalf("%s RenderHybridLegacySource: %v", specimen, err)
	}
	v2Src, err := RenderHybridV2Source(doc, plan)
	if err != nil {
		t.Fatalf("%s RenderHybridV2Source: %v", specimen, err)
	}
	for _, sub := range wantLegSubstrs {
		if !strings.Contains(legSrc, sub) {
			t.Fatalf("%s legacy source missing %q\n--- LEGACY ---\n%s", specimen, sub, legSrc)
		}
	}
	for _, sub := range wantV2Substrs {
		if !strings.Contains(v2Src, sub) {
			t.Fatalf("%s v2 source missing %q\n--- V2 ---\n%s", specimen, sub, v2Src)
		}
	}
	for _, bad := range forbidBothSubstrs {
		if strings.Contains(legSrc, bad) {
			t.Fatalf("%s legacy source unexpectedly contains %q\n--- LEGACY ---\n%s", specimen, bad, legSrc)
		}
		if strings.Contains(v2Src, bad) {
			t.Fatalf("%s v2 source unexpectedly contains %q\n--- V2 ---\n%s", specimen, bad, v2Src)
		}
	}
	return plan, legSrc, v2Src
}

func compileHybridDialectStrict(t *testing.T, label, src string, legacy bool) map[string]any {
	t.Helper()
	resetCompilerStateFully()
	ClearHybridCandidateCache()
	if legacy {
		m, err := compileLegacySourceToRawMapInMemory(src)
		if err != nil {
			t.Fatalf("%s legacy compile failed: %v\n--- SOURCE ---\n%s", label, err, src)
		}
		if hasImportProvenanceKey(m) {
			t.Fatalf("%s legacy output leaked provenance key", label)
		}
		return m
	}
	b, err := CompileSourceToPlist("p12_cf_edit_v2.cherri", src)
	if err != nil {
		t.Fatalf("%s v2 compile failed: %v\n--- SOURCE ---\n%s", label, err, src)
	}
	m, err := decodeShortcutRawMap(b)
	if err != nil {
		t.Fatalf("%s v2 decode failed: %v", label, err)
	}
	if hasImportProvenanceKey(m) {
		t.Fatalf("%s v2 output leaked provenance key", label)
	}
	return m
}

func p12VarAttachment(varName string) map[string]any {
	return map[string]any{
		"Type": "Variable",
		"Variable": map[string]any{
			"WFSerializationType": "WFTextTokenAttachment",
			"Value": map[string]any{
				"Type":         "Variable",
				"VariableName": varName,
			},
		},
	}
}

func p12ActionOutAttachment(uid, outputName string) map[string]any {
	return map[string]any{
		"Type": "Variable",
		"Variable": map[string]any{
			"WFSerializationType": "WFTextTokenAttachment",
			"Value": map[string]any{
				"Type":       "ActionOutput",
				"OutputUUID": uid,
				"OutputName": outputName,
			},
		},
	}
}

func p12MenuItemToken(text string) map[string]any {
	return map[string]any{
		"WFItemType": int64(0),
		"WFValue":    text,
	}
}

func TestP12_CF_01_BalancedSimpleIf(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "Status",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "CurrentDate",
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-01",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12VarAttachment("Status"),
				"WFConditionalActionString": "Ready",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "inside branch",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-01",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-01", doc,
		map[int]string{
			1: "NATURAL_VERIFIED",
			2: "NATURAL_VERIFIED",
			3: "NATURAL_VERIFIED",
		},
		[]string{`if @Status == "Ready" {`, `}`},
		[]string{`if modern: Status == "Ready" {`, `}`},
		[]string{`"is.workflow.actions.conditional"`},
	)
}

func TestP12_CF_02_LegacyComparisonFlagTriState(t *testing.T) {
	makeDoc := func(legFlag *bool) *NativeShortcutDocument {
		startParams := map[string]any{
			"GroupingIdentifier":        "GRP-CF-02",
			"WFControlFlowMode":         int64(0),
			"WFCondition":               int64(4),
			"WFInput":                   p12VarAttachment("ModeFlag"),
			"WFConditionalActionString": "on",
		}
		if legFlag != nil {
			startParams["WFConditionalLegacyComparisonBehavior"] = *legFlag
		}
		doc, _ := semBuildDocFromActions(t, []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
				"WFWorkflowActionParameters": map[string]any{
					"WFTextActionText": "on",
					"UUID":             "11111111-1111-1111-1111-111111111102",
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
				"WFWorkflowActionParameters": map[string]any{
					"WFVariableName": "ModeFlag",
					"WFInput": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value": map[string]any{
							"Type":       "ActionOutput",
							"OutputUUID": "11111111-1111-1111-1111-111111111102",
							"OutputName": "Text",
						},
					},
				},
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
				"WFWorkflowActionParameters": startParams,
			},
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
				"WFWorkflowActionParameters": map[string]any{
					"GroupingIdentifier": "GRP-CF-02",
					"WFControlFlowMode":  int64(2),
				},
			},
		}, plist.XMLFormat)
		return doc
	}

	tr := true
	fl := false

	// WFConditionalLegacyComparisonBehavior: true -> `if legacy` in Legacy, `if` in V2
	assertP12CFDualVerified(t, "P12-CF-02-true", makeDoc(&tr),
		map[int]string{2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED"},
		[]string{`if legacy @ModeFlag == "on" {`},
		[]string{`if ModeFlag == "on" {`},
		[]string{`"is.workflow.actions.conditional"`},
	)

	// WFConditionalLegacyComparisonBehavior: absent -> `if` in Legacy, `if modern:` in V2
	assertP12CFDualVerified(t, "P12-CF-02-absent", makeDoc(nil),
		map[int]string{2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED"},
		[]string{`if @ModeFlag == "on" {`},
		[]string{`if modern: ModeFlag == "on" {`},
		[]string{`"is.workflow.actions.conditional"`},
	)

	// WFConditionalLegacyComparisonBehavior: false -> explicit native fallback
	assertP12CFDualVerified(t, "P12-CF-02-false", makeDoc(&fl),
		map[int]string{2: "EXACT_NATIVE_FALLBACK", 3: "EXACT_NATIVE_FALLBACK"},
		[]string{`<key>WFConditionalLegacyComparisonBehavior</key><false/>`},
		[]string{`<key>WFConditionalLegacyComparisonBehavior</key><false/>`},
		nil,
	)
}

func TestP12_CF_03_UnaryAndBinaryComparators(t *testing.T) {
	uText1 := "22222222-2222-2222-2222-222222220001"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "hello world",
				"UUID":             uText1,
			},
		},
		// Unary has value (100)
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-03-A",
				"WFControlFlowMode":  int64(0),
				"WFCondition":        int64(100),
				"WFInput":            p12ActionOutAttachment(uText1, "Text"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-03-A",
				"WFControlFlowMode":  int64(2),
			},
		},
		// Unary does not have value (101)
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-03-B",
				"WFControlFlowMode":  int64(0),
				"WFCondition":        int64(101),
				"WFInput":            p12ActionOutAttachment(uText1, "Text"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-03-B",
				"WFControlFlowMode":  int64(2),
			},
		},
		// Does not contain (999) + beginsWith (8) + endsWith (9) + numeric Between (1003)
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-03-C",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(999),
				"WFInput":                   p12ActionOutAttachment(uText1, "Text"),
				"WFConditionalActionString": "xyz",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-03-C",
				"WFControlFlowMode":  int64(2),
			},
		},
		// Stale unary 100 with leftover WFConditionalActionString MUST stay EXACT_NATIVE_FALLBACK
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-03-STALE",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(100),
				"WFInput":                   p12ActionOutAttachment(uText1, "Text"),
				"WFConditionalActionString": "stale-rhs",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-03-STALE",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-03", doc,
		map[int]string{
			1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED",
			3: "NATURAL_VERIFIED", 4: "NATURAL_VERIFIED",
			5: "NATURAL_VERIFIED", 6: "NATURAL_VERIFIED",
			7: "EXACT_NATIVE_FALLBACK", 8: "EXACT_NATIVE_FALLBACK",
		},
		[]string{`if gettext {`, `if !gettext {`, `if gettext !contains "xyz" {`, `stale-rhs`},
		[]string{`if modern: gettext {`, `if modern: !gettext {`, `!(gettext contains "xyz")`, `stale-rhs`},
		nil,
	)
}

func TestP12_CF_04_IfElseBranchAndProducerSafety(t *testing.T) {
	uText := "22222222-2222-2222-2222-222222220004"
	uIfOut := "22222222-2222-2222-2222-22222222004F"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "check",
				"UUID":             uText,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-04",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(8),
				"WFInput":                   p12ActionOutAttachment(uText, "Text"),
				"WFConditionalActionString": "ch",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "then"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-04",
				"WFControlFlowMode":  int64(1),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "otherwise"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-04",
				"WFControlFlowMode":  int64(2),
				"UUID":               uIfOut,
				"CustomOutputName":   "IfOutcome",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "Captured",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": uIfOut,
						"OutputName": "IfOutcome",
					},
				},
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-04", doc,
		map[int]string{1: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED", 5: "NATURAL_VERIFIED"},
		[]string{`const IfOutcome = if gettext beginsWith "ch" {`, `} else {`},
		[]string{`let IfOutcome = if modern: gettext beginsWith "ch" {`, `} else {`},
		[]string{`"is.workflow.actions.conditional"`},
	)
}

func TestP12_CF_05_NumericPropertyAndDistanceOrder(t *testing.T) {
	uImg := "22222222-2222-2222-2222-222222220501"
	uRes1 := "22222222-2222-2222-2222-222222220502"
	uRes2 := "22222222-2222-2222-2222-222222220503"
	uDist := "22222222-2222-2222-2222-222222220504"
	doc, _ := semBuildDocFromActions(t, []any{
		// Dictionary HOME["maxDist"] + image + two resizes + getdistance
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary",
			"WFWorkflowActionParameters": map[string]any{
				"WFItems": map[string]any{
					"WFSerializationType": "WFDictionaryFieldValue",
					"Value": map[string]any{
						"WFDictionaryFieldValueItems": []any{
							map[string]any{
								"WFItemType": int64(3),
								"WFKey": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "maxDist"},
								},
								"WFValue": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "10"},
								},
							},
						},
					},
				},
				"UUID":             "22222222-2222-2222-2222-222222220500",
				"CustomOutputName": "HOME",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.detect.images",
			"WFWorkflowActionParameters": map[string]any{
				"WFInput": p12ActionOutAttachment("22222222-2222-2222-2222-222222220500", "HOME"),
				"UUID":    uImg,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.image.resize",
			"WFWorkflowActionParameters": map[string]any{
				"WFImage":            p12ActionOutAttachment(uImg, "Images"),
				"WFImageResizeWidth": "100",
				"UUID":               uRes1,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.image.resize",
			"WFWorkflowActionParameters": map[string]any{
				"WFImage":            p12ActionOutAttachment(uImg, "Images"),
				"WFImageResizeWidth": "200",
				"UUID":               uRes2,
			},
		},
		// Numeric property comparison on .Height (verifies EnumMemberExpr + existing PropertyVariableAggrandizement)
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-05-A",
				"WFControlFlowMode":  int64(0),
				"WFCondition":        int64(2),
				"WFInput": map[string]any{
					"Type": "Variable",
					"Variable": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value": map[string]any{
							"Type":       "ActionOutput",
							"OutputUUID": uRes1,
							"OutputName": "Resized Image",
							"Aggrandizements": []any{
								map[string]any{
									"Type":             "WFPropertyVariableAggrandizement",
									"PropertyName":     "Height",
									"PropertyUserInfo": "WFImageHeight",
								},
							},
						},
					},
				},
				"WFNumberValue": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": uRes2,
						"OutputName": "Resized Image",
						"Aggrandizements": []any{
							map[string]any{
								"Type":             "WFPropertyVariableAggrandizement",
								"PropertyName":     "Height",
								"PropertyUserInfo": "WFImageHeight",
							},
						},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-05-A",
				"WFControlFlowMode":  int64(2),
			},
		},
		// getdistance (empty declared return type) > HOME['maxDist'] (verifies 031:319 fix in Legacy & V2)
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.getdistance",
			"WFWorkflowActionParameters": map[string]any{
				"WFAvoidHighways": false,
				"WFAvoidTolls":    false,
				"UUID":            uDist,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-05-B",
				"WFControlFlowMode":  int64(0),
				"WFCondition":        int64(2),
				"WFInput":            p12ActionOutAttachment(uDist, "Distance"),
				"WFNumberValue": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": "22222222-2222-2222-2222-222222220500",
						"OutputName": "HOME",
						"Aggrandizements": []any{
							map[string]any{
								"Type":          "WFDictionaryValueVariableAggrandizement",
								"DictionaryKey": "maxDist",
							},
						},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-05-B",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-05", doc,
		map[int]string{
			4: "NATURAL_VERIFIED", 5: "NATURAL_VERIFIED",
			6: "NATURAL_VERIFIED", 7: "NATURAL_VERIFIED", 8: "NATURAL_VERIFIED",
		},
		nil,
		nil,
		[]string{`"is.workflow.actions.conditional"`},
	)
}

func TestP12_CF_06_DateAndCompoundWFConditions(t *testing.T) {
	uText := "22222222-2222-2222-2222-222222220601"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "sample",
				"UUID":             uText,
			},
		},
		// Compound AND WFConditions with boundedDate=false + Date isToday (1002)
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-06-CMP",
				"WFControlFlowMode":  int64(0),
				"WFConditions": map[string]any{
					"WFSerializationType": "WFContentPredicateTableTemplate",
					"Value": map[string]any{
						"WFActionParameterFilterPrefix": int64(1),
						"WFContentPredicateBoundedDate": false,
						"WFActionParameterFilterTemplates": []any{
							map[string]any{
								"WFCondition":               int64(4),
								"WFInput":                   p12ActionOutAttachment(uText, "Text"),
								"WFConditionalActionString": "sample",
							},
							map[string]any{
								"WFCondition": int64(1002),
								"WFInput": map[string]any{
									"Type": "Variable",
									"Variable": map[string]any{
										"WFSerializationType": "WFTextTokenAttachment",
										"Value": map[string]any{
											"Type": "CurrentDate",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-06-CMP",
				"WFControlFlowMode":  int64(2),
			},
		},
		// Stale multi-RHS (both WFNumberValue and WFDate) must stay EXACT_NATIVE_FALLBACK
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-06-STALE",
				"WFControlFlowMode":  int64(0),
				"WFConditions": map[string]any{
					"WFSerializationType": "WFContentPredicateTableTemplate",
					"Value": map[string]any{
						"WFActionParameterFilterPrefix": int64(1),
						"WFContentPredicateBoundedDate": false,
						"WFActionParameterFilterTemplates": []any{
							map[string]any{
								"WFCondition":   int64(3),
								"WFInput":       p12ActionOutAttachment(uText, "Text"),
								"WFNumberValue": "1",
								"WFDate": map[string]any{
									"WFSerializationType": "WFTextTokenAttachment",
									"Value":               map[string]any{"Type": "CurrentDate"},
								},
							},
						},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-06-STALE",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-06", doc,
		map[int]string{
			1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED",
			3: "EXACT_NATIVE_FALLBACK", 4: "EXACT_NATIVE_FALLBACK",
		},
		[]string{`&&`, `isToday`},
		[]string{`&&`, `isToday(`},
		nil,
	)
}

func TestP12_CF_07_NestedConditionalsAtomic(t *testing.T) {
	uA := "22222222-2222-2222-2222-222222220701"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "alpha", "UUID": uA},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-07-OUT",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12ActionOutAttachment(uA, "Text"),
				"WFConditionalActionString": "alpha",
			},
		},
		// Nested promotable if inside outer
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-07-IN1",
				"WFControlFlowMode":  int64(0),
				"WFCondition":        int64(100),
				"WFInput":            p12ActionOutAttachment(uA, "Text"),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "nested ok"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-07-IN1",
				"WFControlFlowMode":  int64(2),
			},
		},
		// Nested unrepresentable if (stale dual RHS) inside outer natural if: inner stays native while outer stays atomic!
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-07-IN2",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12ActionOutAttachment(uA, "Text"),
				"WFConditionalActionString": "s",
				"WFNumberValue":             "9",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "nested raw body"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-07-IN2",
				"WFControlFlowMode":  int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-07-OUT",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-07", doc,
		map[int]string{
			1: "NATURAL_VERIFIED", 8: "NATURAL_VERIFIED",
			2: "NATURAL_VERIFIED", 4: "NATURAL_VERIFIED",
			5: "EXACT_NATIVE_FALLBACK", 7: "EXACT_NATIVE_FALLBACK",
		},
		nil, nil, nil,
	)
}

func TestP12_CF_08_InvalidOrphanAndCrossedControlFlow(t *testing.T) {
	// Like 035:723: valid balanced region followed by orphan End If
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-08-OK",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12VarAttachment("Flag"),
				"WFConditionalActionString": "1",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-08-OK",
				"WFControlFlowMode":  int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "ORPHAN-08",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-08", doc,
		map[int]string{
			0: "NATURAL_VERIFIED",
			1: "NATURAL_VERIFIED",
			2: "SOURCE_INVALID",
		},
		[]string{`ORPHAN-08`},
		[]string{`ORPHAN-08`},
		nil,
	)
}

func TestP12_CF_09_MenuCaseTitlesAndOrder(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-09",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "Choose action",
				"WFMenuItems":        []any{p12MenuItemToken("First"), p12MenuItemToken("Second"), p12MenuItemToken("Third")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-09", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "First", "WFMenuItemAttributedTitle": "First"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "branch 1"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-09", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Second", "WFMenuItemAttributedTitle": "Second"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-09", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Third", "WFMenuItemAttributedTitle": "Third"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-09", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-09", doc,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED", 4: "NATURAL_VERIFIED", 5: "NATURAL_VERIFIED"},
		[]string{`menu "Choose action" {`, `item "First":`, `item "Second":`, `item "Third":`},
		[]string{`menu "Choose action" {`, `case "First":`, `case "Second":`, `case "Third":`},
		[]string{`"is.workflow.actions.choosefrommenu"`},
	)
}

func TestP12_CF_10_MenuAttributedTitleVariantsAndDualCopyRejection(t *testing.T) {
	// Menu A: dict WFValue with WFTextTokenString + absent WFMenuItemAttributedTitle -> promotes (attributed items use plain title match)
	// Menu B (like 024/030): WFMenuItems has WFMenuItemAttributedTitle token AND mode 1 carries WFMenuItemAttributedTitle while mismatching/dual -> rejected safely
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-10-STR",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "String items",
				"WFMenuItems":        []any{"Alpha", "Beta"},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-10-STR", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Alpha"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-10-STR", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Beta"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-10-STR", "WFControlFlowMode": int64(2)},
		},
		// Dual-copy attributed title dict in WFMenuItems (dict in mode 1 WFMenuItemAttributedTitle differs from WFMenuItems string/token)
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-10-DUAL",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "Dual copy",
				"WFMenuItems":        []any{p12MenuItemToken("Option A")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-10-DUAL",
				"WFControlFlowMode":  int64(1),
				"WFMenuItemTitle":    "Option A",
				"WFMenuItemAttributedTitle": map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value":               map[string]any{"string": "Styled Copy Diff"},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-10-DUAL", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-10", doc,
		map[int]string{
			0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED",
			4: "EXACT_NATIVE_FALLBACK", 5: "EXACT_NATIVE_FALLBACK", 6: "EXACT_NATIVE_FALLBACK",
		},
		[]string{`item "Alpha" plain:`, `Styled Copy Diff`},
		[]string{`case "Alpha" plain:`, `Styled Copy Diff`},
		nil,
	)
}

func TestP12_CF_11_MenuPromptAndUnicodeRTLCases(t *testing.T) {
	uName := "22222222-2222-2222-2222-222222221101"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "דוד", "UUID": uName},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-11",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt": map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "בחר ￼ 🚀",
						"attachmentsByRange": map[string]any{
							"{4, 1}": map[string]any{
								"Type":       "ActionOutput",
								"OutputUUID": uName,
								"OutputName": "Text",
							},
						},
					},
				},
				"WFMenuItems": []any{p12MenuItemToken("אפשרות א׳ ✅"), p12MenuItemToken("אפשרות ב׳ 📄")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-11", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "אפשרות א׳ ✅", "WFMenuItemAttributedTitle": "אפשרות א׳ ✅"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-11", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "אפשרות ב׳ 📄", "WFMenuItemAttributedTitle": "אפשרות ב׳ 📄"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-11", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-11", doc,
		map[int]string{1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED", 4: "NATURAL_VERIFIED"},
		[]string{`item "אפשרות א׳ ✅":`, `item "אפשרות ב׳ 📄":`},
		[]string{`case "אפשרות א׳ ✅":`, `case "אפשרות ב׳ 📄":`},
		[]string{`"is.workflow.actions.choosefrommenu"`},
	)
}

func TestP12_CF_12_MenuDuplicateTitlesAndEmptyList(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-12-DUP",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "Pick one",
				"WFMenuItems":        []any{p12MenuItemToken("Same"), p12MenuItemToken("Same")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-12-DUP", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Same", "WFMenuItemAttributedTitle": "Same"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "first same"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-12-DUP", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Same", "WFMenuItemAttributedTitle": "Same"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "second same"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-12-DUP", "WFControlFlowMode": int64(2)},
		},
		// Empty WFMenuItems -> stays fallback without error
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-12-EMPTY",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "Empty",
				"WFMenuItems":        []any{},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-12-EMPTY", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-12", doc,
		map[int]string{
			0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED", 5: "NATURAL_VERIFIED",
			6: "EXACT_NATIVE_FALLBACK", 7: "EXACT_NATIVE_FALLBACK",
		},
		nil, nil, nil,
	)
}

func TestP12_CF_13_NestedMenus(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-13-OUT",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "Outer",
				"WFMenuItems":        []any{p12MenuItemToken("Submenu"), p12MenuItemToken("Direct")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-13-OUT", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Submenu", "WFMenuItemAttributedTitle": "Submenu"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-13-IN",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "Inner",
				"WFMenuItems":        []any{p12MenuItemToken("Leaf A"), p12MenuItemToken("Leaf B")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-13-IN", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Leaf A", "WFMenuItemAttributedTitle": "Leaf A"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-13-IN", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Leaf B", "WFMenuItemAttributedTitle": "Leaf B"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-13-IN", "WFControlFlowMode": int64(2)},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-13-OUT", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Direct", "WFMenuItemAttributedTitle": "Direct"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-13-OUT", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-13", doc,
		map[int]string{
			0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED",
			3: "NATURAL_VERIFIED", 4: "NATURAL_VERIFIED", 5: "NATURAL_VERIFIED",
			6: "NATURAL_VERIFIED", 7: "NATURAL_VERIFIED",
		},
		[]string{`menu "Outer"`, `menu "Inner"`, `item "Leaf A":`},
		[]string{`menu "Outer"`, `menu "Inner"`, `case "Leaf A":`},
		[]string{`"is.workflow.actions.choosefrommenu"`},
	)
}

func TestP12_CF_14_MenuOutputConsumerRefs(t *testing.T) {
	uMenuOut := "22222222-2222-2222-2222-222222221400"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-14",
				"WFControlFlowMode":  int64(0),
				"WFMenuItems":        []any{p12MenuItemToken("Yes"), p12MenuItemToken("No")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-14", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Yes", "WFMenuItemAttributedTitle": "Yes"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "picked yes"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-14", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "No", "WFMenuItemAttributedTitle": "No"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-14",
				"WFControlFlowMode":  int64(2),
				"UUID":               uMenuOut,
				"CustomOutputName":   "MenuPick",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "Chosen",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": uMenuOut,
						"OutputName": "MenuPick",
					},
				},
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-14", doc,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED", 4: "NATURAL_VERIFIED"},
		[]string{`const MenuPick = menu {`},
		[]string{`let MenuPick = menu {`},
		[]string{`"is.workflow.actions.choosefrommenu"`},
	)
}

func TestP12_CF_15_MenuLegacyCancelBehaviorAndPreCaseComment(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		// Menu with legacyCancel on Start only -> promotes
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":         "GRP-CF-15-OK",
				"WFControlFlowMode":          int64(0),
				"WFMenuPrompt":               "Legacy Cancel",
				"WFMenuLegacyCancelBehavior": true,
				"WFMenuItems":                []any{p12MenuItemToken("Run")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-15-OK", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Run", "WFMenuItemAttributedTitle": "Run"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-15-OK", "WFControlFlowMode": int64(2)},
		},
		// Menu with WFMenuLegacyCancelBehavior on Mode 1 (like 5 corpus menus) -> must stay fallback
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":         "GRP-CF-15-MID",
				"WFControlFlowMode":          int64(0),
				"WFMenuPrompt":               "Mode1 Cancel",
				"WFMenuLegacyCancelBehavior": true,
				"WFMenuItems":                []any{p12MenuItemToken("Go")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":         "GRP-CF-15-MID",
				"WFControlFlowMode":          int64(1),
				"WFMenuItemTitle":            "Go",
				"WFMenuLegacyCancelBehavior": true,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-15-MID", "WFControlFlowMode": int64(2)},
		},
		// Pre-case comment menu (like 003:80, reg.Mids[0] != reg.Start+1) -> must stay fallback
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-15-PRE",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "PreCase",
				"WFMenuItems":        []any{p12MenuItemToken("Do")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "before first case"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-15-PRE", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "Do"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-15-PRE", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-15", doc,
		map[int]string{
			0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED",
			3: "EXACT_NATIVE_FALLBACK", 4: "EXACT_NATIVE_FALLBACK", 5: "EXACT_NATIVE_FALLBACK",
			6: "EXACT_NATIVE_FALLBACK", 8: "EXACT_NATIVE_FALLBACK", 9: "EXACT_NATIVE_FALLBACK",
		},
		[]string{`menu legacyCancel "Legacy Cancel" {`},
		[]string{`menu("Legacy Cancel", legacyCancel: true) {`},
		nil,
	)
}

func TestP12_CF_16_RepeatCountLiteral(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-16",
				"WFControlFlowMode":  int64(0),
				"WFRepeatCount":      int64(5),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "loop body"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-16",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-16", doc,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED"},
		[]string{`repeat _ for 5 {`},
		[]string{`repeat _ for 5 {`},
		[]string{`"is.workflow.actions.repeat.count"`},
	)
}

func TestP12_CF_17_RepeatCountTypedVariableRef(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary",
			"WFWorkflowActionParameters": map[string]any{
				"WFItems": map[string]any{
					"WFSerializationType": "WFDictionaryFieldValue",
					"Value": map[string]any{
						"WFDictionaryFieldValueItems": []any{
							map[string]any{
								"WFItemType": int64(3),
								"WFKey": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "n"},
								},
								"WFValue": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "3"},
								},
							},
						},
					},
				},
				"UUID":             "22222222-2222-2222-2222-222222221700",
				"CustomOutputName": "Cfg",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-17",
				"WFControlFlowMode":  int64(0),
				"WFRepeatCount": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": "22222222-2222-2222-2222-222222221700",
						"OutputName": "Cfg",
						"Aggrandizements": []any{
							map[string]any{
								"Type":          "WFDictionaryValueVariableAggrandizement",
								"DictionaryKey": "n",
							},
						},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-17",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-17", doc,
		map[int]string{1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED"},
		nil, nil, []string{`"is.workflow.actions.repeat.count"`},
	)
}

func TestP12_CF_18_RepeatEachNativeInputAndOmittedInput(t *testing.T) {
	uList := "22222222-2222-2222-2222-222222221801"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{
				"WFItems": []any{"one", "two"},
				"UUID":    uList,
			},
		},
		// Explicit WFInput -> promotes
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-18-EXP",
				"WFControlFlowMode":  int64(0),
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": uList,
						"OutputName": "List",
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-18-EXP",
				"WFControlFlowMode":  int64(2),
			},
		},
		// Omitted WFInput -> must NOT fabricate a connection, stays exact fallback
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-18-OMIT",
				"WFControlFlowMode":  int64(0),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-18-OMIT",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-18", doc,
		map[int]string{
			1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED",
			3: "EXACT_NATIVE_FALLBACK", 4: "EXACT_NATIVE_FALLBACK",
		},
		[]string{`for `, ` in `, `GRP-CF-18-OMIT`},
		[]string{`for `, ` in `, `GRP-CF-18-OMIT`},
		nil,
	)
}

func TestP12_CF_19_RepeatIndexItemAndResultOutputs(t *testing.T) {
	uList := "22222222-2222-2222-2222-222222221901"
	uLoopOut := "22222222-2222-2222-2222-222222221999"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{"WFItems": []any{"a", "b"}, "UUID": uList},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-19",
				"WFControlFlowMode":  int64(0),
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": uList,
						"OutputName": "List",
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "￼:￼",
						"attachmentsByRange": map[string]any{
							"{0, 1}": map[string]any{"Type": "Variable", "VariableName": "Repeat Index"},
							"{2, 1}": map[string]any{"Type": "Variable", "VariableName": "Repeat Item"},
						},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-19",
				"WFControlFlowMode":  int64(2),
				"UUID":               uLoopOut,
				"CustomOutputName":   "LoopOut",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "Collected",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": uLoopOut,
						"OutputName": "LoopOut",
					},
				},
			},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-19", doc,
		map[int]string{1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED"},
		[]string{`const LoopOut = for `},
		[]string{`let LoopOut = for `},
		[]string{`"is.workflow.actions.repeat.each"`},
	)
}

func TestP12_CF_20_NestedRepeatAndConditional(t *testing.T) {
	uList := "22222222-2222-2222-2222-222222222001"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{"WFItems": []any{"x", "y"}, "UUID": uList},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-20-EACH",
				"WFControlFlowMode":  int64(0),
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": uList,
						"OutputName": "List",
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-20-IF",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFConditionalActionString": "x",
				"WFInput": map[string]any{
					"Type": "Variable",
					"Variable": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value":               map[string]any{"Type": "Variable", "VariableName": "Repeat Item"},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-20-CNT",
				"WFControlFlowMode":  int64(0),
				"WFRepeatCount":      int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "deep body"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-20-CNT", "WFControlFlowMode": int64(2)},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-20-IF", "WFControlFlowMode": int64(2)},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-CF-20-EACH", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	assertP12CFDualVerified(t, "P12-CF-20", doc,
		map[int]string{
			1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED",
			4: "NATURAL_VERIFIED", 5: "NATURAL_VERIFIED", 6: "NATURAL_VERIFIED", 7: "NATURAL_VERIFIED",
		},
		nil, nil,
		[]string{`"is.workflow.actions.repeat.each"`, `"is.workflow.actions.repeat.count"`, `"is.workflow.actions.conditional"`},
	)
}

func TestP12_CF_21_IncorrectRepeatGroupFailClosed(t *testing.T) {
	// Crossed repeat/conditional grouping -> planner preserves original markers fail-closed
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CROSS-1",
				"WFControlFlowMode":  int64(0),
				"WFRepeatCount":      int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CROSS-2",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12VarAttachment("A"),
				"WFConditionalActionString": "1",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CROSS-1",
				"WFControlFlowMode":  int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CROSS-2",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	if doc.ControlFlowValid {
		t.Fatalf("expected crossed control flow document to be marked invalid")
	}
	plan, _, _, _, err := verifyCorpusDualHybridWithPreOverlay(doc, "P12-CF-21", true)
	if err != nil {
		t.Fatalf("fail-closed preservation failed: %v", err)
	}
	for i, d := range plan.Decisions {
		if d.Classification == "NATURAL_VERIFIED" {
			t.Fatalf("crossed invalid marker %d promoted to NATURAL_VERIFIED unexpectedly", i)
		}
	}
}

func TestP12_CF_22_ControlRegionEditAfterBoundaryMove(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "MOVE_ME"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-CF-22",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12VarAttachment("Gate"),
				"WFConditionalActionString": "open",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "INSIDE"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-CF-22",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src := assertP12CFDualVerified(t, "P12-CF-22", doc,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED"},
		nil, nil, nil,
	)

	moveCommentInside := func(src, markerLine string) string {
		lines := strings.Split(src, "\n")
		var out []string
		var moved string
		for _, ln := range lines {
			if moved == "" && strings.Contains(ln, markerLine) {
				moved = ln
				continue
			}
			out = append(out, ln)
			if moved != "" && strings.Contains(ln, `== "open" {`) {
				out = append(out, moved)
			}
		}
		return strings.Join(out, "\n")
	}

	editedLeg := moveCommentInside(legSrc, `comment("MOVE_ME")`)
	editedV2 := moveCommentInside(v2Src, `comment("MOVE_ME")`)
	if editedLeg == legSrc || editedV2 == v2Src {
		t.Fatalf("failed to relocate MOVE_ME comment across region boundary")
	}

	legMap := compileHybridDialectStrict(t, "CF-22-leg-moved", editedLeg, true)
	v2Map := compileHybridDialectStrict(t, "CF-22-v2-moved", editedV2, false)
	for name, m := range map[string]map[string]any{"legacy": legMap, "v2": v2Map} {
		acts := m["WFWorkflowActions"].([]any)
		if len(acts) != 4 {
			t.Fatalf("%s expected 4 actions after move, got %d", name, len(acts))
		}
		id0 := acts[0].(map[string]any)["WFWorkflowActionIdentifier"]
		id1 := acts[1].(map[string]any)["WFWorkflowActionIdentifier"]
		if id0 != "is.workflow.actions.conditional" || id1 != "is.workflow.actions.comment" {
			t.Fatalf("%s expected conditional start then moved comment, got %v -> %v", name, id0, id1)
		}
		c1 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"]
		if c1 != "MOVE_ME" {
			t.Fatalf("%s expected MOVE_ME at index 1, got %v", name, c1)
		}
		g0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["GroupingIdentifier"]
		g3 := acts[3].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["GroupingIdentifier"]
		if g0 != g3 {
			t.Fatalf("%s broken GroupingIdentifier %v != %v", name, g0, g3)
		}
	}
}

func TestP12_EDIT_01_ReplaceSingleParameterIsolated(t *testing.T) {
	uText := "33333333-3333-3333-3333-333333330101"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "untouched-text", "UUID": uText},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "OLD_COMMENT"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-EDIT-01",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12ActionOutAttachment(uText, "Text"),
				"WFConditionalActionString": "keep",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-EDIT-01", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src := assertP12CFDualVerified(t, "P12-EDIT-01", doc,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED"},
		nil, nil, nil,
	)

	origActs := doc.RawRoot["WFWorkflowActions"].([]any)
	for _, tc := range []struct {
		name   string
		src    string
		legacy bool
	}{
		{"legacy", strings.Replace(legSrc, `"OLD_COMMENT"`, `"NEW_COMMENT"`, 1), true},
		{"v2", strings.Replace(v2Src, `"OLD_COMMENT"`, `"NEW_COMMENT"`, 1), false},
	} {
		m := compileHybridDialectStrict(t, "EDIT-01-"+tc.name, tc.src, tc.legacy)
		acts := m["WFWorkflowActions"].([]any)
		for _, unchangedIdx := range []int{0, 2, 3} {
			if !plistValuesStrictlyEqual(origActs[unchangedIdx], acts[unchangedIdx]) {
				t.Fatalf("%s edit spilled into action %d: %s", tc.name, unchangedIdx,
					firstSanitizedPlistDiffPath(origActs[unchangedIdx], acts[unchangedIdx], "act"))
			}
		}
		got := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"]
		if got != "NEW_COMMENT" {
			t.Fatalf("%s target comment not updated: %v", tc.name, got)
		}
	}
}

func TestP12_EDIT_02_EditConditionOperatorAndValueIsolated(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-EDIT-02-TARGET",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12VarAttachment("Mode"),
				"WFConditionalActionString": "alpha",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-EDIT-02-TARGET", "WFControlFlowMode": int64(2)},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        "GRP-EDIT-02-SIBLING",
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4),
				"WFInput":                   p12VarAttachment("Sibling"),
				"WFConditionalActionString": "stay",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-EDIT-02-SIBLING", "WFControlFlowMode": int64(2)},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src := assertP12CFDualVerified(t, "P12-EDIT-02", doc,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED"},
		nil, nil, nil,
	)

	origActs := doc.RawRoot["WFWorkflowActions"].([]any)
	for _, tc := range []struct {
		name   string
		src    string
		legacy bool
	}{
		{"legacy", strings.Replace(legSrc, `@Mode == "alpha"`, `@Mode != "omega"`, 1), true},
		{"v2", strings.Replace(v2Src, `Mode == "alpha"`, `Mode != "omega"`, 1), false},
	} {
		m := compileHybridDialectStrict(t, "EDIT-02-"+tc.name, tc.src, tc.legacy)
		acts := m["WFWorkflowActions"].([]any)
		for _, siblingIdx := range []int{2, 3} {
			if !plistValuesStrictlyEqual(origActs[siblingIdx], acts[siblingIdx]) {
				t.Fatalf("%s edit disturbed sibling action %d: %s", tc.name, siblingIdx,
					firstSanitizedPlistDiffPath(origActs[siblingIdx], acts[siblingIdx], "act"))
			}
		}
		p0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		p1 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if condInt, okC := provInt(p0["WFCondition"]); !okC || condInt != 5 || p0["WFConditionalActionString"] != "omega" {
			t.Fatalf("%s expected edited WFCondition=5/omega, got cond=%v str=%v", tc.name, p0["WFCondition"], p0["WFConditionalActionString"])
		}
		if p0["GroupingIdentifier"] == nil || p0["GroupingIdentifier"] != p1["GroupingIdentifier"] {
			t.Fatalf("%s target GroupingIdentifier broken: %v != %v", tc.name, p0["GroupingIdentifier"], p1["GroupingIdentifier"])
		}
	}
}

func TestP12_EDIT_03_EditMenuPromptAndCaseTitleIsolated(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-EDIT-03-MENU",
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "Old Prompt",
				"WFMenuItems":        []any{p12MenuItemToken("OldCase"), p12MenuItemToken("KeepCase")},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-EDIT-03-MENU", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "OldCase", "WFMenuItemAttributedTitle": "OldCase"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-EDIT-03-MENU", "WFControlFlowMode": int64(1), "WFMenuItemTitle": "KeepCase", "WFMenuItemAttributedTitle": "KeepCase"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{"GroupingIdentifier": "GRP-EDIT-03-MENU", "WFControlFlowMode": int64(2)},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "after menu untouched"},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src := assertP12CFDualVerified(t, "P12-EDIT-03", doc,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED", 4: "NATURAL_VERIFIED"},
		nil, nil, nil,
	)

	origActs := doc.RawRoot["WFWorkflowActions"].([]any)
	editMenu := func(s string) string {
		s = strings.ReplaceAll(s, `"Old Prompt"`, `"New Prompt"`)
		s = strings.ReplaceAll(s, `"OldCase"`, `"RenamedCase"`)
		return s
	}
	for _, tc := range []struct {
		name   string
		src    string
		legacy bool
	}{
		{"legacy", editMenu(legSrc), true},
		{"v2", editMenu(v2Src), false},
	} {
		m := compileHybridDialectStrict(t, "EDIT-03-"+tc.name, tc.src, tc.legacy)
		acts := m["WFWorkflowActions"].([]any)
		if !plistValuesStrictlyEqual(origActs[4], acts[4]) {
			t.Fatalf("%s menu edit altered trailing action: %s", tc.name, firstSanitizedPlistDiffPath(origActs[4], acts[4], "act4"))
		}
		p0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		p1 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		p2 := acts[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p0["WFMenuPrompt"] != "New Prompt" {
			t.Fatalf("%s expected prompt New Prompt, got %v", tc.name, p0["WFMenuPrompt"])
		}
		if p1["WFMenuItemTitle"] != "RenamedCase" || p2["WFMenuItemTitle"] != "KeepCase" {
			t.Fatalf("%s unexpected menu case titles: %v, %v", tc.name, p1["WFMenuItemTitle"], p2["WFMenuItemTitle"])
		}
	}
}

func TestP12_EDIT_04_EditRepeatCountIsolated(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-EDIT-04-LOOP",
				"WFControlFlowMode":  int64(0),
				"WFRepeatCount":      int64(3),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "body untouched"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "GRP-EDIT-04-LOOP",
				"WFControlFlowMode":  int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "outside untouched"},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src := assertP12CFDualVerified(t, "P12-EDIT-04", doc,
		map[int]string{0: "NATURAL_VERIFIED", 1: "NATURAL_VERIFIED", 2: "NATURAL_VERIFIED", 3: "NATURAL_VERIFIED"},
		nil, nil, nil,
	)

	origActs := doc.RawRoot["WFWorkflowActions"].([]any)
	for _, tc := range []struct {
		name   string
		src    string
		legacy bool
	}{
		{"legacy", strings.Replace(legSrc, "for 3 {", "for 9 {", 1), true},
		{"v2", strings.Replace(v2Src, "for 3 {", "for 9 {", 1), false},
	} {
		m := compileHybridDialectStrict(t, "EDIT-04-"+tc.name, tc.src, tc.legacy)
		acts := m["WFWorkflowActions"].([]any)
		if !plistValuesStrictlyEqual(origActs[1], acts[1]) || !plistValuesStrictlyEqual(origActs[3], acts[3]) {
			t.Fatalf("%s repeat edit altered body/outside comment", tc.name)
		}
		p0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if repInt, okR := provInt(p0["WFRepeatCount"]); !okR || repInt != 9 {
			t.Fatalf("%s expected WFRepeatCount=9, got %#v", tc.name, p0["WFRepeatCount"])
		}
	}
}
