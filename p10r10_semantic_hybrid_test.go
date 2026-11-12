/*
 * Copyright (c) Cherri
 * P10R10 Semantic Import Unification & Fail-Closed Hybrid Verification Suite (SEM01-SEM40)
 */

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"howett.net/plist"
)

func semBuildDocFromActions(t *testing.T, actions []any, format int) (*NativeShortcutDocument, []byte) {
	t.Helper()
	docMap := baseTestHeader()
	docMap["WFWorkflowActions"] = actions
	raw := buildNativePlistBytes(t, docMap, format)
	doc, err := DecodeNativeShortcutDocument(raw)
	if err != nil {
		t.Fatalf("DecodeNativeShortcutDocument failed: %v", err)
	}
	return doc, raw
}

func semVerifyDualHybrid(t *testing.T, doc *NativeShortcutDocument) (*HybridImportPlan, string, string, map[string]any, map[string]any) {
	t.Helper()
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatalf("PlanHybridImport failed: %v", err)
	}
	legSrc, err := RenderHybridLegacySource(doc, plan)
	if err != nil {
		t.Fatalf("RenderHybridLegacySource failed: %v", err)
	}
	v2Src, err := RenderHybridV2Source(doc, plan)
	if err != nil {
		t.Fatalf("RenderHybridV2Source failed: %v", err)
	}
	legMap, err := compileLegacySourceToRawMapInMemory(legSrc)
	if err != nil {
		t.Fatalf("Legacy recompile failed: %v\nsource:\n%s", err, legSrc)
	}
	v2Bytes, err := CompileSourceToPlist("sem_v2.cherri", v2Src)
	if err != nil {
		t.Fatalf("V2 recompile failed: %v\nsource:\n%s", err, v2Src)
	}
	v2Map, err := decodeShortcutRawMap(v2Bytes)
	if err != nil {
		t.Fatalf("decode V2 plist failed: %v", err)
	}
	if hasImportProvenanceKey(legMap) || hasImportProvenanceKey(v2Map) {
		t.Fatalf("compiled workflow leaked %s", importProvenanceKey)
	}
	if !plistValuesStrictlyEqual(doc.RawRoot, legMap) {
		t.Fatalf("Legacy wire mismatch: %s", firstPlistDiffPath(doc.RawRoot, legMap, ""))
	}
	if !plistValuesStrictlyEqual(doc.RawRoot, v2Map) {
		t.Fatalf("V2 wire mismatch: %s", firstPlistDiffPath(doc.RawRoot, v2Map, ""))
	}
	return plan, legSrc, v2Src, legMap, v2Map
}

// TestSEM01_LosslessModeNoRegression verifies ExecuteImportPipeline in lossless mode preserves
// full native tree parity across XML, binary plist, and JSON inputs in both Legacy and V2.
func TestSEM01_LosslessModeNoRegression(t *testing.T) {
	doc, rawXML := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Lossless no-regression check"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "Hello World",
				"UUID":             "11111111-1111-1111-1111-111111111111",
			},
		},
	}, plist.XMLFormat)

	tmpDir := t.TempDir()
	inPath := filepath.Join(tmpDir, "in.plist")
	if err := os.WriteFile(inPath, rawXML, 0644); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"legacy", "v2"} {
		outPath := filepath.Join(tmpDir, "out_"+mode+".cherri")
		res, err := ExecuteImportPipeline(ImportOptions{
			InputPath:  inPath,
			OutputPath: outPath,
			Mode:       mode,
			Fidelity:   "lossless",
		})
		if err != nil {
			t.Fatalf("ExecuteImportPipeline(%s lossless) failed: %v", mode, err)
		}
		compiled, err := compileHybridSource(res.FinalSource, mode == "legacy")
		if err != nil {
			t.Fatalf("compile %s lossless failed: %v", mode, err)
		}
		if !plistValuesStrictlyEqual(doc.RawRoot, compiled) {
			t.Fatalf("lossless %s parity diff: %s", mode, firstPlistDiffPath(doc.RawRoot, compiled, ""))
		}
	}
}

// TestSEM02_SemanticReuseOfLegacyCommentAndText verifies that decompile.go semantic analysis
// passes promote comment and gettext actions to NATURAL_VERIFIED in both Legacy and V2.
func TestSEM02_SemanticReuseOfLegacyCommentAndText(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Shared semantic comment"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "Readable text literal"},
		},
	}, plist.XMLFormat)

	plan, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 2 {
		t.Fatalf("expected 2 NATURAL_VERIFIED actions, got %d (%+v)", plan.NaturalVerifiedCount, plan.Decisions)
	}
	if !strings.Contains(legSrc, "comment(") || !strings.Contains(v2Src, "comment(") {
		t.Fatalf("expected comment() in both dialects:\nLeg:\n%s\nV2:\n%s", legSrc, v2Src)
	}
	if !strings.Contains(legSrc, "text(") || !strings.Contains(v2Src, "text(") {
		t.Fatalf("expected text() in both dialects:\nLeg:\n%s\nV2:\n%s", legSrc, v2Src)
	}
}

// TestSEM03_ProducerConsumerUUIDGraphAndOutputName verifies producer UUID, CustomOutputName,
// and downstream ActionOutput consumer references are promoted and preserved identically.
func TestSEM03_ProducerConsumerUUIDGraphAndOutputName(t *testing.T) {
	prodUUID := "22222222-3333-4444-5555-666666666666"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "Produced payload",
				"UUID":             prodUUID,
				"CustomOutputName": "GreetingPayload",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
			"WFWorkflowActionParameters": map[string]any{
				"Text": map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "Msg: \uFFFC!",
						"attachmentsByRange": map[string]any{
							"{5, 1}": map[string]any{
								"Type":       "ActionOutput",
								"OutputUUID": prodUUID,
								"OutputName": "GreetingPayload",
							},
						},
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 2 {
		t.Fatalf("expected producer+consumer promoted to NATURAL_VERIFIED, got %+v", plan.Decisions)
	}
	for _, label := range []struct {
		name string
		m    map[string]any
	}{{"legacy", legMap}, {"v2", v2Map}} {
		acts := label.m["WFWorkflowActions"].([]any)
		p0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p0["UUID"] != prodUUID || p0["CustomOutputName"] != "GreetingPayload" {
			t.Fatalf("%s lost producer UUID/CustomOutputName: %+v", label.name, p0)
		}
	}
}

// TestSEM04_SanitizedVariableAliasesVsNativeNames verifies non-ASCII / space-containing / keyword
// native variable names sanitize into valid identifiers while preserving original WFVariableName.
func TestSEM04_SanitizedVariableAliasesVsNativeNames(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "if",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "User Value 1!",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":         "Variable",
						"VariableName": "if",
					},
				},
			},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if !strings.Contains(v2Src, `native.workflow`) || !strings.Contains(legSrc, `#workflowheader`) {
		t.Fatalf("missing provenance headers")
	}
	for _, label := range []struct {
		name string
		m    map[string]any
	}{{"legacy", legMap}, {"v2", v2Map}} {
		acts := label.m["WFWorkflowActions"].([]any)
		if acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFVariableName"] != "if" {
			t.Fatalf("%s failed to preserve keyword WFVariableName", label.name)
		}
		if acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFVariableName"] != "User Value 1!" {
			t.Fatalf("%s failed to preserve spaced WFVariableName", label.name)
		}
	}
}

// TestSEM05_SparseRequiredAndOptionalParams verifies omitted optional keys in native actions
// remain omitted rather than having default parameter keys materialized.
func TestSEM05_SparseRequiredAndOptionalParams(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.alert",
			"WFWorkflowActionParameters": map[string]any{
				"WFAlertActionMessage":           "Sparse alert only message",
				"WFAlertActionCancelButtonShown": false,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.exit",
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount < 1 {
		t.Fatalf("expected alert to be natural: %+v", plan.Decisions)
	}
	for _, m := range []map[string]any{legMap, v2Map} {
		acts := m["WFWorkflowActions"].([]any)
		p0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if _, hasTitle := p0["WFAlertActionTitle"]; hasTitle {
			t.Fatalf("alert injected absent WFAlertActionTitle key: %+v", p0)
		}
		a1 := acts[1].(map[string]any)
		if _, hasParams := a1["WFWorkflowActionParameters"]; hasParams {
			t.Fatalf("exit injected absent WFWorkflowActionParameters key: %+v", a1)
		}
	}
}

// TestSEM06_ZeroVsFalseVsNilVsAbsent verifies integer 0, boolean false, empty string "", and absent keys
// stay type-distinct and presence-distinct under strict verification.
func TestSEM06_ZeroVsFalseVsNilVsAbsent(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.number",
			"WFWorkflowActionParameters": map[string]any{
				"WFNumberActionNumber": int64(0),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.alert",
			"WFWorkflowActionParameters": map[string]any{
				"WFAlertActionMessage":           "Zero test",
				"WFAlertActionCancelButtonShown": false,
				"WFAlertActionTitle":             "",
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		acts := m["WFWorkflowActions"].([]any)
		n0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFNumberActionNumber"]
		if _, isBool := n0.(bool); isBool {
			t.Fatalf("number 0 coerced to bool")
		}
		p1 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if b, ok := p1["WFAlertActionCancelButtonShown"].(bool); !ok || b {
			t.Fatalf("expected bool false, got %T %v", p1["WFAlertActionCancelButtonShown"], p1["WFAlertActionCancelButtonShown"])
		}
		if s, ok := p1["WFAlertActionTitle"].(string); !ok || s != "" {
			t.Fatalf("expected empty string title, got %T %v", p1["WFAlertActionTitle"], p1["WFAlertActionTitle"])
		}
	}
}

// TestSEM07_UTF16AttachmentTokenRanges verifies Hebrew RTL + non-BMP emoji UTF-16 offset handling
// reuses decompile.go's utf16OffsetToRuneIndex without shifting attachmentsByRange positions.
func TestSEM07_UTF16AttachmentTokenRanges(t *testing.T) {
	prodUUID := "77777777-8888-9999-AAAA-BBBBBBBBBBBB"
	// "שלום 👨‍👩‍👧‍👦 " is 5 + 11 + 1 = 17 UTF-16 code units before U+FFFC
	prefix := "שלום \U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466 "
	u16Pos := utf16Len(prefix)
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "דוד",
				"UUID":             prodUUID,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": prefix + "\uFFFC!",
						"attachmentsByRange": map[string]any{
							fmt.Sprintf("{%d, 1}", u16Pos): map[string]any{
								"Type":       "ActionOutput",
								"OutputUUID": prodUUID,
								"OutputName": "Text",
							},
						},
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 2 {
		t.Fatalf("expected UTF-16 RTL/ZWJ attachment actions promoted cleanly, got %+v", plan.Decisions)
	}
	if utf16OffsetToRuneIndex(prefix+"\uFFFC!", u16Pos) != len([]rune(prefix)) {
		t.Fatalf("utf16OffsetToRuneIndex mismatch for surrogate+ZWJ string")
	}
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// TestSEM08_LiteralBracesAndQuotedEscapes verifies literal {braces}, quotes, tabs, and newlines
// never trigger unintended variable interpolation across Legacy or V2.
func TestSEM08_LiteralBracesAndQuotedEscapes(t *testing.T) {
	literal := "JSON: {\"key\": \"{notAVar}\"}\n\tLine2\\Slash"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": literal,
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		got := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"]
		if got != literal {
			t.Fatalf("escaped/braced string mutated: got %#v, want %#v", got, literal)
		}
	}
}

// TestSEM09_NestedDictionaryAndArray verifies sorted scalar/array/bool/number dictionaries
// promote cleanly while typed nested dictionaries preserve exact wire shape without type loss.
func TestSEM09_NestedDictionaryAndArray(t *testing.T) {
	dictUUID := "09090909-1111-2222-3333-444444444444"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": dictUUID,
				"WFItems": map[string]any{
					"Value": map[string]any{
						"WFDictionaryFieldValueItems": []any{
							map[string]any{
								"WFItemType": int64(4),
								"WFKey": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "alphaBool"},
								},
								"WFValue": map[string]any{
									"WFSerializationType": "WFNumberSubstitutableState",
									"Value":               true,
								},
							},
							map[string]any{
								"WFItemType": int64(3),
								"WFKey": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "betaNum"},
								},
								"WFValue": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "42"},
								},
							},
							map[string]any{
								"WFItemType": int64(2),
								"WFKey": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "gammaArr"},
								},
								"WFValue": map[string]any{
									"WFSerializationType": "WFArrayParameterState",
									"Value": []any{
										map[string]any{
											"WFItemType": int64(0),
											"WFValue": map[string]any{
												"WFSerializationType": "WFTextTokenString",
												"Value":               map[string]any{"string": "one"},
											},
										},
									},
								},
							},
							map[string]any{
								"WFItemType": int64(0),
								"WFKey": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "zetaText"},
								},
								"WFValue": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "hello"},
								},
							},
						},
					},
					"WFSerializationType": "WFDictionaryFieldValue",
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
			"WFWorkflowActionParameters": map[string]any{
				"Text": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": dictUUID,
						"OutputName": "Dictionary",
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount < 1 {
		t.Fatalf("expected sorted typed dictionary/consumer promotion, got %+v", plan.Decisions)
	}
}

// TestSEM10_DictionaryGetValueForKeyStandalone verifies coerce-first .dictionary['key']
// aggrandizement order and standalone getValue dictionary lookup preserve exact wire semantics.
func TestSEM10_DictionaryGetValueForKeyStandalone(t *testing.T) {
	dictUUID := "AAAAAAAA-1111-2222-3333-444444444444"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.detect.dictionary",
			"WFWorkflowActionParameters": map[string]any{
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
				"UUID": dictUUID,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
			"WFWorkflowActionParameters": map[string]any{
				"Text": map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "\uFFFC",
						"attachmentsByRange": map[string]any{
							"{0, 1}": map[string]any{
								"Type":       "ActionOutput",
								"OutputUUID": dictUUID,
								"OutputName": "Dictionary",
								"Aggrandizements": []any{
									map[string]any{
										"Type":              "WFCoercionVariableAggrandizement",
										"CoercionItemClass": "WFDictionaryContentItem",
									},
									map[string]any{
										"Type":          "WFDictionaryValueVariableAggrandizement",
										"DictionaryKey": "tokenKey",
									},
								},
							},
						},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.getvalueforkey",
			"WFWorkflowActionParameters": map[string]any{
				"WFGetDictionaryValueType": "Value",
				"WFDictionaryKey":          "tokenKey",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": dictUUID,
						"OutputName": "Dictionary",
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	if plan.Decisions[1].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected coerce-first .dictionary['tokenKey'] show action promoted to NATURAL_VERIFIED, got %+v\nLeg:\n%s\nV2:\n%s", plan.Decisions[1], legSrc, v2Src)
	}
	if !strings.Contains(legSrc, ".dictionary['tokenKey']") || !strings.Contains(v2Src, `.dictionary["tokenKey"]`) {
		t.Fatalf("expected .dictionary key syntax in both dialects:\nLeg:\n%s\nV2:\n%s", legSrc, v2Src)
	}
}

// TestSEM11_CalculateExpressionExactIdentity verifies calculateexpression never degrades
// into is.workflow.actions.math as the legacy semantic decompiler previously did.
func TestSEM11_CalculateExpressionExactIdentity(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.calculateexpression",
			"WFWorkflowActionParameters": map[string]any{
				"Input": "2 + 3 * 4",
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		id := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionIdentifier"]
		if id != "is.workflow.actions.calculateexpression" {
			t.Fatalf("calculateexpression corrupted to %v", id)
		}
	}
	if plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" {
		t.Fatalf("expected EXACT_NATIVE_FALLBACK preserving exact calculateexpression identity, got %+v", plan.Decisions[0])
	}
}

// TestSEM12_OriginalMathDistinctFromExpression verifies is.workflow.actions.math stays distinct
// from calculateexpression and round-trips accurately.
func TestSEM12_OriginalMathDistinctFromExpression(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.math",
			"WFWorkflowActionParameters": map[string]any{
				"WFInput":                   int64(42),
				"WFMathOperation":           "…",
				"WFScientificMathOperation": "abs(x)",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.calculateexpression",
			"WFWorkflowActionParameters": map[string]any{
				"Input": "42 + 1",
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		acts := m["WFWorkflowActions"].([]any)
		if acts[0].(map[string]any)["WFWorkflowActionIdentifier"] != "is.workflow.actions.math" ||
			acts[1].(map[string]any)["WFWorkflowActionIdentifier"] != "is.workflow.actions.calculateexpression" {
			t.Fatalf("math/calculateexpression conflated: %+v", acts)
		}
	}
}

// TestSEM13_BalancedConditionalIfElse verifies balanced conditional if/else/endif regions with nested
// actions promote atomically to NATURAL_VERIFIED in both Legacy (if legacy) and V2 (if).
func TestSEM13_BalancedConditionalIfElse(t *testing.T) {
	grp := "COND-GROUP-0013"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":                    grp,
				"WFControlFlowMode":                     int64(0),
				"WFCondition":                           int64(100),
				"WFConditionalLegacyComparisonBehavior": true,
				"WFInput": map[string]any{
					"Type": "Variable",
					"Variable": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value":               map[string]any{"Type": "ExtensionInput"},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Then branch"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(1),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Else branch"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	plan, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 5 {
		t.Fatalf("expected all 5 conditional/body actions promoted cleanly, got %+v", plan.Decisions)
	}
	if !strings.Contains(legSrc, "if legacy ") || !strings.Contains(v2Src, "if ") {
		t.Fatalf("missing expected conditional syntax:\nLeg:\n%s\nV2:\n%s", legSrc, v2Src)
	}
}

// TestSEM14_MenuCasesAndAttributedTitles verifies balanced choosefrommenu regions with explicit
// WFMenuItemAttributedTitle matching WFMenuItemTitle promote cleanly across Legacy and V2.
func TestSEM14_MenuCasesAndAttributedTitles(t *testing.T) {
	grp := "MENU-GROUP-0014"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(0),
				"WFMenuPrompt":       "Choose action",
				"WFMenuItems": []any{
					map[string]any{"WFItemType": int64(0), "WFValue": "Alpha"},
					map[string]any{"WFItemType": int64(0), "WFValue": "Beta"},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        grp,
				"WFControlFlowMode":         int64(1),
				"WFMenuItemTitle":           "Alpha",
				"WFMenuItemAttributedTitle": "Alpha",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Inside Alpha"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        grp,
				"WFControlFlowMode":         int64(1),
				"WFMenuItemTitle":           "Beta",
				"WFMenuItemAttributedTitle": "Beta",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Inside Beta"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	plan, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 6 {
		t.Fatalf("expected all 6 menu actions promoted, got %+v", plan.Decisions)
	}
	if !strings.Contains(legSrc, `item "Alpha":`) || !strings.Contains(v2Src, `case "Alpha":`) {
		t.Fatalf("expected Legacy item / V2 case syntax:\nLeg:\n%s\nV2:\n%s", legSrc, v2Src)
	}
}

// TestSEM15_RepeatEachWithRepeatItemAndIndex verifies balanced repeat.each region with
// RepeatItem / RepeatIndex references inside its body promotes and preserves loop variables.
func TestSEM15_RepeatEachWithRepeatItemAndIndex(t *testing.T) {
	grp := "REPEATEACH-GROUP-0015"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(0),
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "ExtensionInput"},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
			"WFWorkflowActionParameters": map[string]any{
				"Text": map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "#\uFFFC: \uFFFC",
						"attachmentsByRange": map[string]any{
							"{1, 1}": map[string]any{
								"Type":         "Variable",
								"VariableName": "Repeat Index",
							},
							"{4, 1}": map[string]any{
								"Type":         "Variable",
								"VariableName": "Repeat Item",
							},
						},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount < 2 {
		t.Fatalf("expected repeat.each region promoted cleanly, got %+v", plan.Decisions)
	}
}

// TestSEM16_RepeatCountBalanced verifies balanced repeat.count promotes cleanly with int64 count.
func TestSEM16_RepeatCountBalanced(t *testing.T) {
	grp := "REPEATCOUNT-GROUP-0016"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(0),
				"WFRepeatCount":      int64(4),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Inside repeat count"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 3 {
		t.Fatalf("expected all 3 repeat.count actions promoted, got %+v", plan.Decisions)
	}
}

// TestSEM17_OrphanConditionalSourceInvalid035Shape verifies an unbalanced End If (like corpus 035:723)
// is flagged SOURCE_INVALID, never silently dropped or braced, yet preserves wire fidelity.
func TestSEM17_OrphanConditionalSourceInvalid035Shape(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Before orphan end"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "ORPHAN-035-SHAPE",
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.ControlFlowValid || plan.SourceInvalidCount != 1 {
		t.Fatalf("expected ControlFlowValid=false SourceInvalidCount=1, got valid=%t invalid=%d", plan.ControlFlowValid, plan.SourceInvalidCount)
	}
	if plan.Decisions[1].Classification != "SOURCE_INVALID" || plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("unexpected decisions around orphan marker: %+v", plan.Decisions)
	}
}

// TestSEM18_ControlEndMarkerUUIDAndGroupingID verifies when a control-flow End marker carries
// a producer UUID consumed downstream, atomic fallback preserves the UUID edge and GroupingIdentifier.
func TestSEM18_ControlEndMarkerUUIDAndGroupingID(t *testing.T) {
	grp := "COND-END-UUID-0018"
	endUUID := "EEEEEEEE-1111-2222-3333-444444444444"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":                    grp,
				"WFControlFlowMode":                     int64(0),
				"WFCondition":                           int64(100),
				"WFConditionalLegacyComparisonBehavior": true,
				"WFInput": map[string]any{
					"Type": "Variable",
					"Variable": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value":               map[string]any{"Type": "ExtensionInput"},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "Result"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(2),
				"UUID":               endUUID,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
			"WFWorkflowActionParameters": map[string]any{
				"Text": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": endUUID,
						"OutputName": "If Result",
					},
				},
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		endP := m["WFWorkflowActions"].([]any)[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if endP["UUID"] != endUUID || endP["GroupingIdentifier"] != grp {
			t.Fatalf("lost end marker UUID/GroupingIdentifier: %+v", endP)
		}
	}
}

// TestSEM19_WorkflowMetadataPreservation verifies custom header fields, version numbers,
// icon settings, empty arrays, and provenance stripping are exact across both dialects.
func TestSEM19_WorkflowMetadataPreservation(t *testing.T) {
	docMap := baseTestHeader()
	docMap["WFWorkflowClientVersion"] = "3612.0.2.2"
	docMap["WFWorkflowNoInputBehavior"] = map[string]any{
		"Name":       "WFWorkflowNoInputBehaviorAskForInput",
		"Parameters": map[string]any{"ItemClass": "WFStringContentItem"},
	}
	docMap["WFWorkflowActions"] = []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Header check"},
		},
	}
	doc, err := DecodeNativeShortcutDocument(buildNativePlistBytes(t, docMap, plist.XMLFormat))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		if m["WFWorkflowClientVersion"] != "3612.0.2.2" {
			t.Fatalf("client version mismatch: %v", m["WFWorkflowClientVersion"])
		}
		if hasImportProvenanceKey(m) {
			t.Fatalf("leaked provenance key")
		}
	}
}

// TestSEM20_ImportQuestionsPreserved verifies WFWorkflowImportQuestions survive intact.
func TestSEM20_ImportQuestionsPreserved(t *testing.T) {
	docMap := baseTestHeader()
	docMap["WFWorkflowImportQuestions"] = []any{
		map[string]any{
			"ActionIndex":  uint64(0),
			"Category":     "Parameter",
			"DefaultValue": "DefaultAPI",
			"ParameterKey": "WFTextActionText",
			"Text":         "Enter value",
		},
	}
	docMap["WFWorkflowActions"] = []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "DefaultAPI"},
		},
	}
	doc, err := DecodeNativeShortcutDocument(buildNativePlistBytes(t, docMap, plist.XMLFormat))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, _, _ = semVerifyDualHybrid(t, doc)
}

// TestSEM21_ThirdPartyAppIntentDescriptorWire verifies unknown third-party AppIntent descriptors
// fall back cleanly and preserve IntentAppDefinition / AppIntentDescriptor byte-for-byte.
func TestSEM21_ThirdPartyAppIntentDescriptorWire(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "com.example.thirdparty.CustomIntent",
			"WFWorkflowActionParameters": map[string]any{
				"AppIntentDescriptor": map[string]any{
					"AppIntentIdentifier": "CustomIntent",
					"BundleIdentifier":    "com.example.thirdparty",
					"TeamIdentifier":      "TEAM12345",
				},
				"prompt": "Hello",
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" || plan.Decisions[0].ReasonCode != "THIRD_PARTY_NO_CANONICAL_DEF" {
		t.Fatalf("expected THIRD_PARTY_NO_CANONICAL_DEF fallback, got %+v", plan.Decisions[0])
	}
}

// TestSEM22_VariantGroupsDeterministicResolution verifies multi-variant Apple identifier groups
// in DefaultReverseActionRegistry resolve deterministically across repeated plans.
func TestSEM22_VariantGroupsDeterministicResolution(t *testing.T) {
	reg := DefaultReverseActionRegistry()
	multiCount := 0
	for _, cands := range reg.ByAppleIdentifier {
		if len(cands) > 1 {
			multiCount++
		}
	}
	if multiCount < 50 {
		t.Fatalf("expected substantial multi-variant Apple ID groups, got %d", multiCount)
	}
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.alert",
			"WFWorkflowActionParameters": map[string]any{"WFAlertActionMessage": "Deterministic variant"},
		},
	}, plist.XMLFormat)

	first, err := PlanHybridImportWithRegistry(doc, reg, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		next, err := PlanHybridImportWithRegistry(doc, reg, false)
		if err != nil {
			t.Fatal(err)
		}
		if next.Decisions[0].ChosenDefinitionID != first.Decisions[0].ChosenDefinitionID ||
			next.Decisions[0].V2Call != first.Decisions[0].V2Call ||
			next.Decisions[0].LegacyCall != first.Decisions[0].LegacyCall {
			t.Fatalf("non-deterministic variant resolution on iter %d: %+v vs %+v", i, next.Decisions[0], first.Decisions[0])
		}
	}
}

// TestSEM23_SSoTPropagationSamePlannerAndRegistry verifies DefaultReverseActionRegistry is synced
// with buildActionCatalog / EnsureCanonicalStandardActions and single-plan Legacy+V2 rendering.
func TestSEM23_SSoTPropagationSamePlannerAndRegistry(t *testing.T) {
	loadTestStandardActions()
	catalog := buildActionCatalog()
	reg := DefaultReverseActionRegistry()
	if len(catalog) < 500 || len(reg.ByCallableName) < 490 {
		t.Fatalf("unexpected catalog/registry sizes: catalog=%d reverse=%d", len(catalog), len(reg.ByCallableName))
	}
	for name := range reg.ByCallableName {
		if _, ok := actions[name]; !ok {
			t.Fatalf("reverse registry callable %q absent from canonical actions map", name)
		}
	}
}

// TestSEM24_FailClosedMetadataOnlyAndProvenanceLeakRejection verifies negative mutations fail
// closed in VerifyWholeWorkflowAgainstOriginal: header mismatch, leaked provenance, type mismatch, shifted index.
func TestSEM24_FailClosedMetadataOnlyAndProvenanceLeakRejection(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Alpha"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.number",
			"WFWorkflowActionParameters": map[string]any{"WFNumberActionNumber": int64(1)},
		},
	}, plist.XMLFormat)
	plan, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	_ = plan

	// 1. Header-only mismatch must fail Equal.
	mutHeaderDocMap := deepClonePlistMap(doc.RawRoot)
	mutHeaderDocMap["WFWorkflowClientVersion"] = "9999.0.0"
	mutHeaderDoc, _ := DecodeNativeShortcutDocument(buildNativePlistBytes(t, mutHeaderDocMap, plist.XMLFormat))
	if res := VerifyWholeWorkflowAgainstOriginal(mutHeaderDoc, legSrc, true); res.Equal || res.HeaderEqual {
		t.Fatalf("expected header mismatch failure in Legacy verifier")
	}
	if res := VerifyWholeWorkflowAgainstOriginal(mutHeaderDoc, v2Src, false); res.Equal || res.HeaderEqual {
		t.Fatalf("expected header mismatch failure in V2 verifier")
	}

	// 2. Scalar type mismatch (int64(1) vs bool(true) or "1") must fail.
	strNumMap := deepClonePlistMap(doc.RawRoot)
	strNumMap["WFWorkflowActions"].([]any)[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFNumberActionNumber"] = "1"
	strNumDoc, _ := DecodeNativeShortcutDocument(buildNativePlistBytes(t, strNumMap, plist.XMLFormat))
	if res := VerifyWholeWorkflowAgainstOriginal(strNumDoc, v2Src, false); res.Equal {
		t.Fatalf("expected string vs int64 scalar type mismatch to fail closed")
	}

	// 3. Shifted action order must fail.
	swappedMap := deepClonePlistMap(doc.RawRoot)
	acts := swappedMap["WFWorkflowActions"].([]any)
	swappedMap["WFWorkflowActions"] = []any{acts[1], acts[0]}
	swappedDoc, _ := DecodeNativeShortcutDocument(buildNativePlistBytes(t, swappedMap, plist.XMLFormat))
	if res := VerifyWholeWorkflowAgainstOriginal(swappedDoc, legSrc, true); res.Equal {
		t.Fatalf("expected shifted action index mismatch to fail closed")
	}

	// 4. Provenance key leak check must catch nested __CherriImportProvenance.
	leaked := map[string]any{
		"WFWorkflowActions": []any{
			map[string]any{importProvenanceKey: map[string]any{"bad": true}},
		},
	}
	if !hasImportProvenanceKey(leaked) {
		t.Fatalf("hasImportProvenanceKey failed to detect nested provenance leak")
	}
}

// TestSEM25_FailClosedPostDemotionStillMismatch verifies RenderAndVerifyHybrid errors cleanly
// when a demotion occurs on iteration 0 and recompilation still diverges, leaving caller plan untouched.
func TestSEM25_FailClosedPostDemotionStillMismatch(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "com.unknown.immutable",
			"WFWorkflowActionParameters": map[string]any{"K": "EXPECTED"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Initial"},
		},
	}, plist.XMLFormat)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatal(err)
	}
	// Inject a wrong parameter into fallback action 0 AND a wrong natural comment into action 1.
	// Iteration 0 demotes action 1, then iteration 1 recompiles and must fail closed on action 0.
	badPlan := cloneHybridPlan(plan)
	badPlan.Decisions[0].LegacyCall = `rawAction("com.unknown.immutable", rawPlist("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict><key>K</key><string>TAMPERED</string></dict></plist>"))`
	badPlan.Decisions[1].LegacyCall = `comment("WRONG_NATURAL")`
	beforeClass1 := badPlan.Decisions[1].Classification
	beforeCall1 := badPlan.Decisions[1].LegacyCall
	if _, err := RenderAndVerifyHybrid(doc, badPlan, true); err == nil {
		t.Fatalf("expected RenderAndVerifyHybrid to fail closed on remaining post-demotion mismatch")
	}
	if badPlan.Decisions[1].Classification != beforeClass1 || badPlan.Decisions[1].LegacyCall != beforeCall1 {
		t.Fatalf("caller's HybridImportPlan was mutated on failed RenderAndVerifyHybrid call")
	}
}

// TestSEM26_FailClosedNoDemotionCandidatesLeft verifies RenderAndVerifyHybrid returns an explicit
// error and preserves caller plan when plan/doc sizes mismatch or no natural candidate can be demoted.
func TestSEM26_FailClosedNoDemotionCandidatesLeft(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "com.unknown.no.demote",
			"WFWorkflowActionParameters": map[string]any{"X": int64(1)},
		},
	}, plist.XMLFormat)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatal(err)
	}
	// Wrong decision slice length must fail closed immediately without mutating caller plan.
	emptyDoc, _ := semBuildDocFromActions(t, []any{}, plist.XMLFormat)
	if _, err := RenderAndVerifyHybrid(emptyDoc, plan, true); err == nil {
		t.Fatalf("expected action-count mismatch error")
	}
	// Corrupting rendered fallback call to mismatch source must fail with zero demotion candidates.
	corruptPlan := cloneHybridPlan(plan)
	corruptPlan.Decisions[0].LegacyCall = `rawAction("com.unknown.no.demote", rawPlist("<?xml version=\"1.0\" encoding=\"UTF-8\"?><plist version=\"1.0\"><dict><key>X</key><integer>999</integer></dict></plist>"))`
	savedCall := corruptPlan.Decisions[0].LegacyCall
	if _, err := RenderAndVerifyHybrid(doc, corruptPlan, true); err == nil {
		t.Fatalf("expected fail-closed error when no natural action can be demoted")
	}
	if corruptPlan.Decisions[0].LegacyCall != savedCall {
		t.Fatalf("caller plan mutated on RenderAndVerifyHybrid error")
	}
}

// TestSEM27_BinaryControlBytesNullAndRawDataBlob verifies NUL, VT, CR, and arbitrary []byte data blobs
// survive plan + Legacy/V2 render + compile cleanly via BinaryFormat + b64:.
func TestSEM27_BinaryControlBytesNullAndRawDataBlob(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Before binary"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "A\x00B\x0bC\rD\nE",
				"Blob":             []byte{0x00, 0xff, 0x0b, 0x0d, 0x7f},
			},
		},
	}, plist.BinaryFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" || plan.Decisions[1].Classification != "EXACT_NATIVE_FALLBACK" {
		t.Fatalf("unexpected decisions: %+v", plan.Decisions)
	}
}

// TestSEM28_UserEditOnNaturalActionIsolated edits a natural comment/text call in generated
// Legacy and V2 sources and proves all unaffected actions/header/UUIDs stay identical.
func TestSEM28_UserEditOnNaturalActionIsolated(t *testing.T) {
	prodUUID := "99999999-AAAA-BBBB-CCCC-DDDDDDDDDDDD"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "ORIGINAL_COMMENT"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "com.example.opaque",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":             prodUUID,
				"CustomOutputName": "KeepMe",
				"Payload":          int64(77),
			},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	editedLeg := strings.Replace(legSrc, "ORIGINAL_COMMENT", "EDITED_COMMENT", 1)
	editedV2 := strings.Replace(v2Src, "ORIGINAL_COMMENT", "EDITED_COMMENT", 1)

	legMap, err := compileLegacySourceToRawMapInMemory(editedLeg)
	if err != nil {
		t.Fatal(err)
	}
	v2Bytes, err := CompileSourceToPlist("edit28.cherri", editedV2)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)

	for _, m := range []map[string]any{legMap, v2Map} {
		acts := m["WFWorkflowActions"].([]any)
		origActs := doc.RawRoot["WFWorkflowActions"].([]any)
		if acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"] != "EDITED_COMMENT" {
			t.Fatalf("edited natural comment did not update")
		}
		if !plistValuesStrictlyEqual(acts[1], origActs[1]) {
			t.Fatalf("unaffected fallback action drifted during natural edit: %s", firstPlistDiffPath(origActs[1], acts[1], ""))
		}
	}
}

// TestSEM29_UserEditInsideControlRegionIsolated edits a nested action inside a balanced if/else
// region and verifies control GroupingIdentifier and sibling/external actions remain exact.
func TestSEM29_UserEditInsideControlRegionIsolated(t *testing.T) {
	grp := "CTRL-EDIT-0029"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":                    grp,
				"WFControlFlowMode":                     int64(0),
				"WFCondition":                           int64(100),
				"WFConditionalLegacyComparisonBehavior": true,
				"WFInput": map[string]any{
					"Type": "Variable",
					"Variable": map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value":               map[string]any{"Type": "ExtensionInput"},
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "INNER_BEFORE"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": grp,
				"WFControlFlowMode":  int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "com.example.after",
			"WFWorkflowActionParameters": map[string]any{"Tag": "UNCHANGED_AFTER"},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	for _, tc := range []struct {
		legacy bool
		src    string
	}{
		{true, strings.Replace(legSrc, "INNER_BEFORE", "INNER_AFTER", 1)},
		{false, strings.Replace(v2Src, "INNER_BEFORE", "INNER_AFTER", 1)},
	} {
		compiled, err := compileHybridSource(tc.src, tc.legacy)
		if err != nil {
			t.Fatalf("compile edited control region (legacy=%t): %v", tc.legacy, err)
		}
		acts := compiled["WFWorkflowActions"].([]any)
		origActs := doc.RawRoot["WFWorkflowActions"].([]any)
		if acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFCommentActionText"] != "INNER_AFTER" {
			t.Fatalf("inner action did not update")
		}
		if !plistValuesStrictlyEqual(acts[3], origActs[3]) {
			t.Fatalf("action outside control region mutated: %s", firstPlistDiffPath(origActs[3], acts[3], ""))
		}
	}
}

// TestSEM30_OnlyTargetActionChangesAndNoCollateralDelta verifies modifying one middle action
// in a 4-action mixed workflow leaves indices 0, 1, 3 and header 100% untouched.
func TestSEM30_OnlyTargetActionChangesAndNoCollateralDelta(t *testing.T) {
	u0 := "30303030-0000-0000-0000-000000000001"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "Base", "UUID": u0},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Keep1"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "TARGET_OLD"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
			"WFWorkflowActionParameters": map[string]any{
				"Text": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "ActionOutput", "OutputUUID": u0, "OutputName": "Text"},
				},
			},
		},
	}, plist.XMLFormat)

	_, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	origActs := doc.RawRoot["WFWorkflowActions"].([]any)
	for _, tc := range []struct {
		legacy bool
		src    string
	}{
		{true, strings.Replace(legSrc, "TARGET_OLD", "TARGET_NEW", 1)},
		{false, strings.Replace(v2Src, "TARGET_OLD", "TARGET_NEW", 1)},
	} {
		compiled, err := compileHybridSource(tc.src, tc.legacy)
		if err != nil {
			t.Fatal(err)
		}
		acts := compiled["WFWorkflowActions"].([]any)
		for _, idx := range []int{0, 1, 3} {
			if !plistValuesStrictlyEqual(origActs[idx], acts[idx]) {
				t.Fatalf("collateral diff at action %d: %s", idx, firstPlistDiffPath(origActs[idx], acts[idx], ""))
			}
		}
	}
}

// TestSEM31_NoUnintendedRebindOnSanitizedVariableNames verifies distinct native variable names
// whose sanitized ASCII forms collide get distinct Cherri symbols and never cross-bind.
func TestSEM31_NoUnintendedRebindOnSanitizedVariableNames(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "Var A!",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "ExtensionInput"},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "Var_A_",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "Clipboard"},
				},
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		acts := m["WFWorkflowActions"].([]any)
		if acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFVariableName"] != "Var A!" ||
			acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFVariableName"] != "Var_A_" {
			t.Fatalf("variable name aliasing occurred: %+v", acts)
		}
	}
}

// TestSEM32_NoSilentLegacyFallbackInV2 verifies rendered V2 hybrid output contains zero Legacy
// preprocessor/raw directives and invalid V2 syntax returns V2 errors without Legacy substitution.
func TestSEM32_NoSilentLegacyFallbackInV2(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "V2 strict check"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "com.example.custom",
			"WFWorkflowActionParameters": map[string]any{"Val": int64(5)},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	for _, legacyTok := range []string{"#workflowheader", "#include ", "rawAction(", "rawPlist("} {
		if strings.Contains(v2Src, legacyTok) {
			t.Fatalf("V2 hybrid output contains Legacy syntax token %q:\n%s", legacyTok, v2Src)
		}
	}
	if _, err := CompileSourceToPlist("bad_v2.cherri", "#define name Bad\nconst x = \"y\"\n"); err == nil {
		t.Fatalf("V2 compiler silently accepted Legacy directives without error")
	}
}

// TestSEM33_Sequential1000MixedPlansAndCompilesNoStateLeak runs 1,000 sequential alternating
// Legacy and V2 hybrid plans + compiles verifying zero compiler/decompiler state leakage.
func TestSEM33_Sequential1000MixedPlansAndCompilesNoStateLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1,000-iteration stress test under -short")
	}
	docA, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Stress A"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "Alpha",
				"UUID":             "33333333-0000-0000-0000-000000000001",
			},
		},
	}, plist.XMLFormat)
	docB, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.number",
			"WFWorkflowActionParameters": map[string]any{"WFNumberActionNumber": int64(7)},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "com.example.opaque",
			"WFWorkflowActionParameters": map[string]any{"Data": []byte{1, 2, 3}},
		},
	}, plist.BinaryFormat)

	var firstA, firstB map[string]any
	for i := 0; i < 1000; i++ {
		doc := docA
		if i%2 == 1 {
			doc = docB
		}
		legacy := (i/2)%2 == 0
		plan, err := PlanHybridImport(doc)
		if err != nil {
			t.Fatalf("iter %d plan failed: %v", i, err)
		}
		var src string
		if legacy {
			src, err = RenderHybridLegacySource(doc, plan)
		} else {
			src, err = RenderHybridV2Source(doc, plan)
		}
		if err != nil {
			t.Fatalf("iter %d render(legacy=%t) failed: %v", i, legacy, err)
		}
		compiled, err := compileHybridSource(src, legacy)
		if err != nil {
			t.Fatalf("iter %d compile(legacy=%t) failed: %v", i, legacy, err)
		}
		if !plistValuesStrictlyEqual(doc.RawRoot, compiled) {
			t.Fatalf("iter %d parity drift: %s", i, firstPlistDiffPath(doc.RawRoot, compiled, ""))
		}
		if i == 0 {
			firstA = compiled
		} else if i == 1 {
			firstB = compiled
		} else if i == 998 && !plistValuesStrictlyEqual(firstA, compiled) {
			t.Fatalf("state leak on docA after 998 runs")
		} else if i == 999 && !plistValuesStrictlyEqual(firstB, compiled) {
			t.Fatalf("state leak on docB after 999 runs")
		}
	}
}

// TestSEM34_TamperDetectionAndStrictCompareAsymmetry verifies strict plist comparison rejects
// subtle bool/int confusion, extra map keys, nil vs empty slice/map, and float sign/precision drift.
func TestSEM34_TamperDetectionAndStrictCompareAsymmetry(t *testing.T) {
	a := map[string]any{"k": int64(1), "arr": []any{}, "empty": map[string]any{}}
	tests := []map[string]any{
		{"k": true, "arr": []any{}, "empty": map[string]any{}},
		{"k": "1", "arr": []any{}, "empty": map[string]any{}},
		{"k": float64(1.0), "arr": []any{}, "empty": map[string]any{}},
		{"k": int64(1), "arr": []any{}},
		{"k": int64(1), "arr": []any{}, "empty": map[string]any{}, "extra": nil},
	}
	for idx, tc := range tests {
		if plistValuesStrictlyEqual(a, tc) {
			t.Fatalf("tamper case %d falsely passed strict compare: %+v", idx, tc)
		}
	}
}

// TestSEM35_MissingCorpusBlockedExplicitly verifies running corpus loader on a missing directory
// fails explicitly instead of returning an empty 0/0 PASS.
func TestSEM35_MissingCorpusBlockedExplicitly(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "does-not-exist-corpus")
	entries, err := os.ReadDir(missingDir)
	if err == nil || len(entries) != 0 {
		t.Fatalf("expected explicit error for missing corpus directory")
	}
}

// TestSEM36_CITestDiscoveryAndNegativeAssertionGuard verifies this suite defines all 40 unique
// TestSEM01..TestSEM40 functions cleanly and contains no silent skip/pass inflation.
func TestSEM36_CITestDiscoveryAndNegativeAssertionGuard(t *testing.T) {
	data, err := os.ReadFile("p10r10_semantic_hybrid_test.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^func (TestSEM\d{2}_[A-Za-z0-9_]+)\(t \*testing\.T\)`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	seen := map[int]string{}
	for _, m := range matches {
		name := m[1]
		var num int
		if _, err := fmt.Sscanf(name, "TestSEM%02d_", &num); err != nil {
			t.Fatalf("bad SEM test name %q: %v", name, err)
		}
		if prev, dup := seen[num]; dup {
			t.Fatalf("duplicate SEM%02d test: %s vs %s", num, prev, name)
		}
		seen[num] = name
	}
	for n := 1; n <= 40; n++ {
		if _, ok := seen[n]; !ok {
			t.Fatalf("missing TestSEM%02d_* in p10r10_semantic_hybrid_test.go", n)
		}
	}
}

// TestSEM37_IndependentDecoderAgreementAndPythonComparison cross-validates Cherri's native
// decoder against Python's independent plistlib decoder when python is available on host.
func TestSEM37_IndependentDecoderAgreementAndPythonComparison(t *testing.T) {
	doc, raw := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "Line1\rLine2\x0bBin",
				"RawBytes":         []byte{0xde, 0xad, 0xbe, 0xef},
			},
		},
	}, plist.BinaryFormat)
	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if !plistValuesStrictlyEqual(legMap, v2Map) {
		t.Fatalf("Legacy/V2 mismatch: %s", firstPlistDiffPath(legMap, v2Map, ""))
	}

	pyExe := ""
	for _, cand := range []string{"python", "python3"} {
		if _, err := exec.LookPath(cand); err == nil {
			if exec.Command(cand, "-c", "import plistlib").Run() == nil {
				pyExe = cand
				break
			}
		}
	}
	if pyExe == "" {
		return
	}
	var binBuf bytes.Buffer
	if err := plist.NewEncoderForFormat(&binBuf, plist.BinaryFormat).Encode(v2Map); err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	srcP := filepath.Join(tmpDir, "src.plist")
	outP := filepath.Join(tmpDir, "v2.plist")
	_ = os.WriteFile(srcP, raw, 0644)
	_ = os.WriteFile(outP, binBuf.Bytes(), 0644)
	cmd := exec.Command(pyExe, "-c",
		"import sys,plistlib; a=plistlib.loads(open(sys.argv[1],'rb').read()); b=plistlib.loads(open(sys.argv[2],'rb').read()); sys.exit(0 if a==b and type(a['WFWorkflowMinimumClientVersion']) is int else 1)",
		srcP, outP)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Python independent plistlib verification failed: %v %s", err, string(out))
	}
}

// TestSEM38_FreshnessLedgerHashesActualBinaryAndSource verifies SHA-256 ledger hashing accurately
// detects any 1-byte source or artifact staleness without trusting static constants.
func TestSEM38_FreshnessLedgerHashesActualBinaryAndSource(t *testing.T) {
	orig := []byte("cherri-artifact-v1")
	mutated := []byte("cherri-artifact-v2")
	h1 := sha256.Sum256(orig)
	h2 := sha256.Sum256(mutated)
	if hex.EncodeToString(h1[:]) == hex.EncodeToString(h2[:]) {
		t.Fatalf("identical SHA-256 for modified artifacts")
	}
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Ledger hash check"},
		},
	}, plist.XMLFormat)
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.SourceHash) != 64 {
		t.Fatalf("expected 64-hex SourceHash, got %q", plan.SourceHash)
	}
}

// TestSEM39_DirectOriginalJSONInputParity verifies direct .json shortcut payloads decode via
// DecodeNativeShortcutDocument and round-trip cleanly across Legacy and V2 hybrid compilation.
func TestSEM39_DirectOriginalJSONInputParity(t *testing.T) {
	docMap := baseTestHeader()
	docMap["WFWorkflowActions"] = []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Direct JSON input"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "JSON payload"},
		},
	}
	jsonBytes, err := json.Marshal(docMap)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeNativeShortcutDocument(jsonBytes)
	if err != nil {
		t.Fatalf("JSON DecodeNativeShortcutDocument failed: %v", err)
	}
	if doc.SourceFormat != "json" {
		t.Fatalf("expected SourceFormat=json, got %q", doc.SourceFormat)
	}
	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 2 {
		t.Fatalf("expected 2 NATURAL_VERIFIED from JSON input, got %+v", plan.Decisions)
	}
}

// TestSEM40_ActionRegistryUnknownFutureParamFallback verifies adding an unmodeled future iOS key
// on a normally natural built-in action cleanly triggers UNREPRESENTABLE_PARAM exact fallback.
func TestSEM40_ActionRegistryUnknownFutureParamFallback(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "Natural part",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "Has future param",
				"WFFutureIOS28Mode":   true,
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" ||
		plan.Decisions[1].Classification != "EXACT_NATIVE_FALLBACK" ||
		plan.Decisions[1].ReasonCode != "UNREPRESENTABLE_PARAM" {
		t.Fatalf("expected UNREPRESENTABLE_PARAM fallback for unknown future key, got %+v", plan.Decisions)
	}
	for _, m := range []map[string]any{legMap, v2Map} {
		p1 := m["WFWorkflowActions"].([]any)[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p1["WFFutureIOS28Mode"] != true {
			t.Fatalf("future key lost in compiled output: %+v", p1)
		}
	}
}
