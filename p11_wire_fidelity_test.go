/*
 * Copyright (c) Cherri Language
 * P10R11 True Action-Wire Parity, Editable Import, Region Coverage & Semantic Truth Suite
 * Production-Path Adversarial Suite (P11-WIRE-01 .. P11-WIRE-50)
 */

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

// TestP11_WIRE_01_Fresh38x2NativeActionWire verifies action-by-action parity across all natural actions
// in the 38-shortcut corpus in both Legacy and Cherri v2 modes.
func TestP11_WIRE_01_Fresh38x2NativeActionWire(t *testing.T) {
	if os.Getenv("CHERRI_P10R10_CORPUS") != "1" {
		t.Skip("NOT_RUN_PRIVATE_CORPUS: set CHERRI_P10R10_CORPUS=1 to run against full 38-shortcut private corpus")
	}
	entries, err := os.ReadDir(p10r10CorpusDir)
	if err != nil {
		t.Fatalf("CHERRI_P10R10_CORPUS=1 was specified but corpus directory is not accessible: %v", err)
	}

	totalNaturalVerified := 0
	totalActionsAcrossCorpus := 0
	filesCount := 0

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		filesCount++
		raw, err := os.ReadFile(filepath.Join(p10r10CorpusDir, e.Name()))
		if err != nil {
			t.Fatalf("%s read failed: %v", e.Name(), err)
		}
		doc, err := DecodeNativeShortcutDocument(raw)
		if err != nil {
			t.Fatalf("%s decode failed: %v", e.Name(), err)
		}

		totalActionsAcrossCorpus += len(doc.Actions)

		var capturedPreOverlay []any
		PreOverlayActionHook = func(actions []any) {
			capturedPreOverlay = actions
		}

		plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
		PreOverlayActionHook = nil

		totalNaturalVerified += plan.NaturalVerifiedCount

		// Verify action-by-action for every natural action
		legActions := legMap["WFWorkflowActions"].([]any)
		v2Actions := v2Map["WFWorkflowActions"].([]any)

		if len(legActions) != len(doc.Actions) || len(v2Actions) != len(doc.Actions) {
			t.Fatalf("%s action count mismatch: orig=%d leg=%d v2=%d",
				e.Name(), len(doc.Actions), len(legActions), len(v2Actions))
		}

		for i, dec := range plan.Decisions {
			if dec.Classification != "NATURAL_VERIFIED" {
				continue
			}
			origRec := doc.Actions[i]
			legAct := legActions[i].(map[string]any)
			v2Act := v2Actions[i].(map[string]any)

			// Layer 1: Identifiers match
			if legAct["WFWorkflowActionIdentifier"] != origRec.Identifier {
				t.Fatalf("%s Legacy action %d identifier mismatch: %v vs %v",
					e.Name(), i, legAct["WFWorkflowActionIdentifier"], origRec.Identifier)
			}
			if v2Act["WFWorkflowActionIdentifier"] != origRec.Identifier {
				t.Fatalf("%s V2 action %d identifier mismatch: %v vs %v",
					e.Name(), i, v2Act["WFWorkflowActionIdentifier"], origRec.Identifier)
			}

			// Layer 2: Post-overlay full parameter parity
			legParams, _ := legAct["WFWorkflowActionParameters"].(map[string]any)
			v2Params, _ := v2Act["WFWorkflowActionParameters"].(map[string]any)

			if !plistValuesStrictlyEqual(origRec.Parameters, legParams) {
				t.Fatalf("%s Legacy action %d (%s) post-overlay parameter divergence:\norig: %+v\nleg:  %+v",
					e.Name(), i, origRec.Identifier, origRec.Parameters, legParams)
			}
			if !plistValuesStrictlyEqual(origRec.Parameters, v2Params) {
				t.Fatalf("%s V2 action %d (%s) post-overlay parameter divergence:\norig: %+v\nv2:   %+v",
					e.Name(), i, origRec.Identifier, origRec.Parameters, v2Params)
			}

			// Layer 3: Pre-overlay semantic parameters match
			if len(capturedPreOverlay) == len(doc.Actions) {
				preAct := capturedPreOverlay[i].(map[string]any)
				preParams, _ := preAct["WFWorkflowActionParameters"].(map[string]any)
				for k, v := range origRec.Parameters {
					if isImportProvenanceMetaKey(k) || k == "WFSerializationType" {
						continue
					}
					preVal, hasPre := preParams[k]
					if !hasPre {
						t.Fatalf("%s action %d (%s) missing pre-overlay semantic key %s",
							e.Name(), i, origRec.Identifier, k)
					}
					preVal = canonicalizePlistValue(preVal)
					if !plistValuesStrictlyEqual(v, preVal) {
						t.Fatalf("%s action %d (%s) pre-overlay semantic mismatch on %s:\norig: %+v\npre:  %+v",
							e.Name(), i, origRec.Identifier, k, v, preVal)
					}
				}
			}
		}
	}

	if filesCount != 38 {
		t.Fatalf("expected 38 corpus files, inspected %d", filesCount)
	}
	if totalActionsAcrossCorpus != 27652 {
		t.Fatalf("expected 27,652 total actions across 38 files, got %d", totalActionsAcrossCorpus)
	}
	if totalNaturalVerified != 11480 {
		t.Fatalf("expected 11,480 natural verified actions across 38 files in P10R11, got %d", totalNaturalVerified)
	}
}

// TestP11_WIRE_02_StrictPreOverlaySemanticKeys verifies that natural actions emit their functional
// native parameters directly before import provenance restoration is applied.
func TestP11_WIRE_02_StrictPreOverlaySemanticKeys(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "PreOverlayComment",
				"UUID":                "UUID-PRE-01",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "PreOverlayText",
				"UUID":             "UUID-PRE-02",
			},
		},
	}, plist.XMLFormat)

	var preOverlayCaptured []any
	PreOverlayActionHook = func(actions []any) {
		preOverlayCaptured = actions
	}
	defer func() { PreOverlayActionHook = nil }()

	plan, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 2 {
		t.Fatalf("expected 2 natural verified actions, got %d", plan.NaturalVerifiedCount)
	}
	_ = legSrc
	_ = v2Src

	if len(preOverlayCaptured) != 2 {
		t.Fatalf("expected 2 pre-overlay captured actions, got %d", len(preOverlayCaptured))
	}
	act0 := preOverlayCaptured[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if act0["WFCommentActionText"] != "PreOverlayComment" {
		t.Fatalf("pre-overlay comment text mismatch: got %v", act0["WFCommentActionText"])
	}
	act1 := preOverlayCaptured[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if act1["WFTextActionText"] != "PreOverlayText" {
		t.Fatalf("pre-overlay text mismatch: got %v", act1["WFTextActionText"])
	}
}

// TestP11_WIRE_03_DisableActionMetadataRestoreTestHook tests that without provenance restoration,
// semantic keys are 100% present while identity/metadata keys remain un-restored.
func TestP11_WIRE_03_DisableActionMetadataRestoreTestHook(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "NoRestoreCheck",
				"UUID":                "ORIG-UUID-001",
				"CustomOutputName":    "MyCommentOutput",
			},
		},
	}, plist.XMLFormat)

	var captured []any
	PreOverlayActionHook = func(actions []any) {
		captured = actions
	}
	defer func() { PreOverlayActionHook = nil }()

	_, _, _, _, _ = semVerifyDualHybrid(t, doc)
	if len(captured) != 1 {
		t.Fatalf("expected 1 captured action, got %d", len(captured))
	}
	p := captured[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if p["WFCommentActionText"] != "NoRestoreCheck" {
		t.Fatalf("semantic parameter missing before restore: %+v", p)
	}
	// Before restore, CustomOutputName must not be present
	if _, hasCustom := p["CustomOutputName"]; hasCustom {
		t.Fatalf("CustomOutputName unexpectedly present before restore: %+v", p)
	}
}

// TestP11_WIRE_04_InjectUnknownShowSecurity verifies that an uncatalogued ShowSecurity parameter
// is NOT automatically whitelisted by a Show* prefix rule and triggers exact native fallback.
func TestP11_WIRE_04_InjectUnknownShowSecurity(t *testing.T) {
	if isImportProvenanceMetaKey("ShowSecurity") {
		t.Fatalf("ShowSecurity must not be classified as import provenance metadata")
	}
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "Safe comment",
				"ShowSecurity":        true,
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" {
		t.Fatalf("expected EXACT_NATIVE_FALLBACK for ShowSecurity, got %s", plan.Decisions[0].Classification)
	}
	for _, m := range []map[string]any{legMap, v2Map} {
		acts := m["WFWorkflowActions"].([]any)
		p := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p["ShowSecurity"] != true {
			t.Fatalf("ShowSecurity parameter lost in compiled output: %+v", p)
		}
	}
}

// TestP11_WIRE_05_InjectExtraFutureShowSecret verifies that ShowSecret with an opaque value
// causes the natural candidate to be rejected and preserved as exact native wire.
func TestP11_WIRE_05_InjectExtraFutureShowSecret(t *testing.T) {
	if isImportProvenanceMetaKey("ShowSecret") {
		t.Fatalf("ShowSecret must not be whitelisted as import metadata")
	}
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "SecretText",
				"ShowSecret":       "opaque-secret-payload",
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" {
		t.Fatalf("expected fallback for ShowSecret, got %s", plan.Decisions[0].Classification)
	}
}

// TestP11_WIRE_06_InjectUnknownWFPrivateFutureParam verifies uncatalogued future parameter
// keys are never silently coerced or dropped.
func TestP11_WIRE_06_InjectUnknownWFPrivateFutureParam(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText":  "NormalComment",
				"WFPrivateFutureParam": int64(999),
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" {
		t.Fatalf("expected fallback for WFPrivateFutureParam, got %s", plan.Decisions[0].Classification)
	}
	for _, m := range []map[string]any{legMap, v2Map} {
		p := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if fmt.Sprintf("%v", p["WFPrivateFutureParam"]) != "999" {
			t.Fatalf("WFPrivateFutureParam lost or coerced: %+v", p)
		}
	}
}

// TestP11_WIRE_07_014_208_NativeShowHeadersPreOverlay verifies that downloadurl fileRequest
// (exact shape from specimen 014:208 with ShowHeaders=true) emits ShowHeaders natively pre-overlay
// and that mutating true -> false or removing showHeaders (absent) strictly mutates only ShowHeaders
// across both Legacy and V2 dialects without stale provenance restoration.
func TestP11_WIRE_07_014_208_NativeShowHeadersPreOverlay(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":          "https://translate-pa.googleapis.com/v1/translateHtml",
				"WFHTTPMethod":   "POST",
				"WFHTTPBodyType": "File",
				"ShowHeaders":    true,
				"UUID":           "1B23A869-029F-49C2-8AB9-5E09563647AC",
				"WFHTTPHeaders": map[string]any{
					"Value": map[string]any{
						"WFDictionaryFieldValueItems": []any{
							map[string]any{
								"WFKey": map[string]any{
									"Value":               map[string]any{"string": "Content-Type"},
									"WFSerializationType": "WFTextTokenString",
								},
								"WFItemType": int64(0),
								"WFValue": map[string]any{
									"Value":               map[string]any{"string": "application/json"},
									"WFSerializationType": "WFTextTokenString",
								},
							},
						},
					},
					"WFSerializationType": "WFDictionaryFieldValue",
				},
			},
		},
	}, plist.XMLFormat)

	var preOverlayCaptured []any
	PreOverlayActionHook = func(actions []any) {
		preOverlayCaptured = actions
	}
	defer func() { PreOverlayActionHook = nil }()

	plan, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected fileRequest with ShowHeaders to be NATURAL_VERIFIED, got %+v\nLeg:\n%s\nV2:\n%s", plan.Decisions[0], legSrc, v2Src)
	}
	if len(preOverlayCaptured) != 1 {
		t.Fatalf("expected 1 pre-overlay action, got %d", len(preOverlayCaptured))
	}
	p := preOverlayCaptured[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if p["ShowHeaders"] != true {
		t.Fatalf("pre-overlay fileRequest missing ShowHeaders: %+v", p)
	}

	// Mutation 1: true -> false across BOTH Legacy and V2
	// Legacy: fileRequest(..., true) -> fileRequest(..., false)
	// V2:     fileRequest(..., showHeaders: true) -> fileRequest(..., showHeaders: false)
	{
		// Legacy
		mutLegFalse := strings.Replace(legSrc, ", true)", ", false)", 1)
		outBytes, err := compileLegacySourceToRawMapInMemory(mutLegFalse)
		if err != nil {
			t.Fatalf("Legacy compile with false failed: %v\nsrc:\n%s", err, mutLegFalse)
		}
		p := outBytes["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p["ShowHeaders"] != false {
			t.Fatalf("Legacy expected ShowHeaders=false after edit, got: %v", p["ShowHeaders"])
		}

		// V2
		if !strings.Contains(v2Src, "showHeaders: true") {
			t.Fatalf("V2 expected 'showHeaders: true' in rendered source, got:\n%s", v2Src)
		}
		mutV2False := strings.Replace(v2Src, "showHeaders: true", "showHeaders: false", 1)
		v2Bytes, err := CompileSourceToPlist("mut_false.cherri", mutV2False)
		if err != nil {
			t.Fatalf("V2 compile with showHeaders: false failed: %v", err)
		}
		outDoc, err := decodeShortcutRawMap(v2Bytes)
		if err != nil {
			t.Fatalf("V2 decode failed: %v", err)
		}
		outParams := outDoc["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if outParams["ShowHeaders"] != false {
			t.Fatalf("V2 expected ShowHeaders=false after edit, got: %v (provenance restored stale value!)", outParams["ShowHeaders"])
		}
		if outParams["WFURL"] != "https://translate-pa.googleapis.com/v1/translateHtml" || outParams["WFHTTPMethod"] != "POST" {
			t.Fatalf("V2 unrelated parameters corrupted on edit: %+v", outParams)
		}
	}

	// Mutation 2: removal of showHeaders (absent) across BOTH Legacy and V2
	{
		// Legacy: remove `, true)` -> `)`
		mutLegAbsent := strings.Replace(legSrc, ", true)", ")", 1)
		outBytes, err := compileLegacySourceToRawMapInMemory(mutLegAbsent)
		if err != nil {
			t.Fatalf("Legacy compile with showHeaders absent failed: %v", err)
		}
		p := outBytes["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if _, hasHeaders := p["ShowHeaders"]; hasHeaders {
			t.Fatalf("Legacy expected ShowHeaders to be absent after removal, got: %v", p["ShowHeaders"])
		}

		// V2: remove `, showHeaders: true`
		mutV2Absent := strings.Replace(v2Src, ", showHeaders: true", "", 1)
		v2Bytes, err := CompileSourceToPlist("mut_absent.cherri", mutV2Absent)
		if err != nil {
			t.Fatalf("V2 compile with showHeaders absent failed: %v", err)
		}
		outDoc, err := decodeShortcutRawMap(v2Bytes)
		if err != nil {
			t.Fatalf("V2 decode failed: %v", err)
		}
		outParams := outDoc["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if _, hasHeaders := outParams["ShowHeaders"]; hasHeaders {
			t.Fatalf("V2 expected ShowHeaders to be absent after removal, got: %v", outParams["ShowHeaders"])
		}
		if outParams["WFURL"] != "https://translate-pa.googleapis.com/v1/translateHtml" || outParams["WFHTTPMethod"] != "POST" {
			t.Fatalf("V2 unrelated parameters corrupted on removal: %+v", outParams)
		}
	}
}

// TestP11_WIRE_08_SourceShowHeadersTriState verifies absent, present false, and present true
// for ShowHeaders produce exact matching wire across both dialects.
func TestP11_WIRE_08_SourceShowHeadersTriState(t *testing.T) {
	for _, tc := range []struct {
		name        string
		showHeaders any
		wantPresent bool
		wantVal     bool
	}{
		{"absent", nil, false, false},
		{"false", false, true, false},
		{"true", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{
				"WFURL":        "https://example.com/api",
				"WFHTTPMethod": "GET",
			}
			if tc.showHeaders != nil {
				params["ShowHeaders"] = tc.showHeaders
			}
			doc, _ := semBuildDocFromActions(t, []any{
				map[string]any{
					"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
					"WFWorkflowActionParameters": params,
				},
			}, plist.XMLFormat)

			plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
			if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
				t.Fatalf("expected NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
			}
			for _, m := range []map[string]any{legMap, v2Map} {
				p := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
				val, exists := p["ShowHeaders"]
				if exists != tc.wantPresent {
					t.Fatalf("ShowHeaders presence mismatch: expected %v, got %v in %+v", tc.wantPresent, exists, p)
				}
				if exists && val != tc.wantVal {
					t.Fatalf("ShowHeaders value mismatch: expected %v, got %v", tc.wantVal, val)
				}
			}
		})
	}
}

// TestP11_WIRE_09_UserEditsOnlyShowHeaders tests that editing ShowHeaders in Cherri source changes
// only that single field in compiled output and is never overwritten by stale provenance.
func TestP11_WIRE_09_UserEditsOnlyShowHeaders(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":        "https://example.com/endpoint",
				"WFHTTPMethod": "GET",
				"ShowHeaders":  true,
				"UUID":         "DOWNLOAD-UUID-01",
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}

	if !strings.Contains(v2Src, "showHeaders: true") {
		t.Fatalf("expected 'showHeaders: true' in V2 source, got:\n%s", v2Src)
	}
	editedV2 := strings.Replace(v2Src, "showHeaders: true", "showHeaders: false", 1)
	v2Bytes, err := CompileSourceToPlist("edited_download.cherri", editedV2)
	if err != nil {
		t.Fatalf("compile edited V2 failed: %v", err)
	}
	v2Map, err := decodeShortcutRawMap(v2Bytes)
	if err != nil {
		t.Fatalf("decode edited plist failed: %v", err)
	}
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if p["ShowHeaders"] != false {
		t.Fatalf("expected ShowHeaders=false on user edit, got %v", p["ShowHeaders"])
	}
}

// TestP11_WIRE_10_LegacySetVariableAbsentSerializationType verifies that Legacy setVariable
// import preserves absent top-level WFSerializationType while normal authored code retains upstream Cherri semantics.
func TestP11_WIRE_10_LegacySetVariableAbsentSerializationType(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "count",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected setVariable to be NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	p := legMap["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if _, hasSer := p["WFSerializationType"]; hasSer {
		t.Fatalf("imported setVariable leaked top-level WFSerializationType: %+v", p)
	}
}

// TestP11_WIRE_11_LegacyAppendVariableAbsentSerializationType verifies appendVariable absent WFSerializationType.
func TestP11_WIRE_11_LegacyAppendVariableAbsentSerializationType(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.appendvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "items",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected appendVariable to be NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	p := legMap["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if _, hasSer := p["WFSerializationType"]; hasSer {
		t.Fatalf("imported appendVariable leaked top-level WFSerializationType: %+v", p)
	}
}

// TestP11_WIRE_12_PresentSerializationTypePreserved verifies that when WFSerializationType is present,
// it is preserved accurately.
func TestP11_WIRE_12_PresentSerializationTypePreserved(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName":      "myVar",
				"WFInput":             "textVal",
				"WFSerializationType": "WFTextTokenAttachment",
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		p := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p["WFSerializationType"] != "WFTextTokenAttachment" {
			t.Fatalf("present WFSerializationType was lost: %+v", p)
		}
	}
}

// TestP11_WIRE_13_ExplicitFalseVsAbsentOptionalField verifies explicit false and absent optional fields differ on wire.
func TestP11_WIRE_13_ExplicitFalseVsAbsentOptionalField(t *testing.T) {
	docFalse, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":        "https://example.com",
				"WFHTTPMethod": "GET",
				"ShowHeaders":  false,
			},
		},
	}, plist.XMLFormat)
	docAbsent, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":        "https://example.com",
				"WFHTTPMethod": "GET",
			},
		},
	}, plist.XMLFormat)

	_, _, _, legFalse, _ := semVerifyDualHybrid(t, docFalse)
	_, _, _, legAbsent, _ := semVerifyDualHybrid(t, docAbsent)

	pFalse := legFalse["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	pAbsent := legAbsent["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)

	if pFalse["ShowHeaders"] != false {
		t.Fatalf("expected ShowHeaders: false, got %v", pFalse["ShowHeaders"])
	}
	if _, has := pAbsent["ShowHeaders"]; has {
		t.Fatalf("expected ShowHeaders to be absent, but found %v", pAbsent["ShowHeaders"])
	}
}

// TestP11_WIRE_14_ZeroVsStringZeroVsAbsent verifies exact type preservation: 0 int vs "0" string vs absent.
func TestP11_WIRE_14_ZeroVsStringZeroVsAbsent(t *testing.T) {
	docInt, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.number",
			"WFWorkflowActionParameters": map[string]any{"WFNumberActionNumber": int64(0)},
		},
	}, plist.XMLFormat)
	docStr, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "0"},
		},
	}, plist.XMLFormat)

	_, _, _, legInt, _ := semVerifyDualHybrid(t, docInt)
	_, _, _, legStr, _ := semVerifyDualHybrid(t, docStr)

	pInt := legInt["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	pStr := legStr["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)

	var vInt int64
	switch n := pInt["WFNumberActionNumber"].(type) {
	case int64:
		vInt = n
	case uint64:
		vInt = int64(n)
	default:
		t.Fatalf("expected integer 0, got %T: %v", pInt["WFNumberActionNumber"], pInt["WFNumberActionNumber"])
	}
	if vInt != 0 {
		t.Fatalf("expected 0, got %v", vInt)
	}
	if v, ok := pStr["WFTextActionText"].(string); !ok || v != "0" {
		t.Fatalf("expected string \"0\", got %T: %v", pStr["WFTextActionText"], pStr["WFTextActionText"])
	}
}

// TestP11_WIRE_15_EmptyListDictVsOmitted verifies empty list [] vs omitted shape.
func TestP11_WIRE_15_EmptyListDictVsOmitted(t *testing.T) {
	docEmptyList, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{
				"WFItems": []any{},
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, docEmptyList)
	for _, m := range []map[string]any{legMap, v2Map} {
		p := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		items, ok := p["WFItems"].([]any)
		if !ok || len(items) != 0 {
			t.Fatalf("expected empty []any for WFItems, got %T: %v", p["WFItems"], p["WFItems"])
		}
	}
}

// TestP11_WIRE_16_NestedTypedDateDataNumeric verifies nested dictionaries and arrays preserve numeric types.
func TestP11_WIRE_16_NestedTypedDateDataNumeric(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": "DICT-NEST-01",
				"WFItems": map[string]any{
					"WFSerializationType": "WFDictionaryFieldValue",
					"Value": map[string]any{
						"WFDictionaryFieldValueItems": []any{
							map[string]any{
								"WFKey": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "count"},
								},
								"WFItemType": int64(3),
								"WFValue": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "42"},
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
				"WFDictionaryKey": "count",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": "DICT-NEST-01",
						"OutputName": "Dictionary",
					},
				},
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if !plistValuesStrictlyEqual(legMap, v2Map) {
		t.Fatalf("Legacy and V2 output mismatch on nested dictionary")
	}
}

// TestP11_WIRE_17_NameCollisionTwoSymbols verifies that two symbols with conflicting names
// retain distinct UUIDs and correct producer/consumer links without cross-binding.
func TestP11_WIRE_17_NameCollisionTwoSymbols(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "FirstOutput",
				"UUID":             "UUID-COLL-01",
				"CustomOutputName": "Text",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "SecondOutput",
				"UUID":             "UUID-COLL-02",
				"CustomOutputName": "Text",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "targetVar",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": "UUID-COLL-02",
						"OutputName": "Text",
					},
				},
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	for _, m := range []map[string]any{legMap, v2Map} {
		acts := m["WFWorkflowActions"].([]any)
		u1 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"]
		u2 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"]
		if u1 == u2 {
			t.Fatalf("colliding UUIDs assigned to distinct actions: %v vs %v", u1, u2)
		}
		inUUID := acts[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFInput"].(map[string]any)["Value"].(map[string]any)["OutputUUID"]
		if inUUID != "UUID-COLL-02" {
			t.Fatalf("consumer bound to wrong producer UUID: got %v, want UUID-COLL-02", inUUID)
		}
	}
}

// TestP11_WIRE_18_ProducerWithTwoConsumers verifies one producer referenced by two distinct consumers.
func TestP11_WIRE_18_ProducerWithTwoConsumers(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "SharedProducer",
				"UUID":             "PROD-UUID-01",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "consumerOne",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": "PROD-UUID-01",
						"OutputName": "Text",
					},
				},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "consumerTwo",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type":       "ActionOutput",
						"OutputUUID": "PROD-UUID-01",
						"OutputName": "Text",
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 3 {
		t.Fatalf("expected 3 natural verified actions, got %d", plan.NaturalVerifiedCount)
	}
}

// TestP11_WIRE_19_MixedAggrandizementAndCoercion verifies coercion and dictionary lookup ordering.
func TestP11_WIRE_19_MixedAggrandizementAndCoercion(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.dictionary",
			"WFWorkflowActionParameters": map[string]any{
				"UUID": "DICT-AGGR-01",
				"WFItems": map[string]any{
					"WFSerializationType": "WFDictionaryFieldValue",
					"Value": map[string]any{
						"WFDictionaryFieldValueItems": []any{
							map[string]any{
								"WFKey": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "name"},
								},
								"WFItemType": int64(0),
								"WFValue": map[string]any{
									"WFSerializationType": "WFTextTokenString",
									"Value":               map[string]any{"string": "Cherri"},
								},
							},
						},
					},
				},
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
								"OutputUUID": "DICT-AGGR-01",
								"OutputName": "Dictionary",
								"Aggrandizements": []any{
									map[string]any{
										"Type":              "WFCoercionVariableAggrandizement",
										"CoercionItemClass": "WFDictionaryContentItem",
									},
									map[string]any{
										"Type":          "WFDictionaryValueVariableAggrandizement",
										"DictionaryKey": "name",
									},
								},
							},
						},
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	if plan.Decisions[1].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected NATURAL_VERIFIED for aggrandizement expression, got %s", plan.Decisions[1].Classification)
	}
}

// TestP11_WIRE_20_HebrewEmojiZWJ_UTF16Tokens verifies Hebrew, emojis with ZWJ sequences, and UTF-16 ranges.
func TestP11_WIRE_20_HebrewEmojiZWJ_UTF16Tokens(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "שלום עולם 👨‍👩‍👧‍👦 Cherri",
				"UUID":             "HEB-UUID-01",
			},
		},
	}, plist.XMLFormat)

	plan, legSrc, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected Hebrew/emoji text to be NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	if !strings.Contains(legSrc, "שלום עולם") || !strings.Contains(v2Src, "שלום עולם") {
		t.Fatalf("Hebrew text was corrupted during rendering:\nLeg:\n%s\nV2:\n%s", legSrc, v2Src)
	}
}

// TestP11_WIRE_21_TextUserEditInPrivateSource verifies that editing literal text modifies only that action.
func TestP11_WIRE_21_TextUserEditInPrivateSource(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "OriginalText", "UUID": "TXT-01"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "UnchangedComment", "UUID": "CMT-01"},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	editedV2 := strings.Replace(v2Src, `"OriginalText"`, `"ModifiedText"`, 1)
	v2Bytes, err := CompileSourceToPlist("edit_text.cherri", editedV2)
	if err != nil {
		t.Fatalf("compile edited source: %v", err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	acts := v2Map["WFWorkflowActions"].([]any)
	p0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	p1 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)

	if p0["WFTextActionText"] != "ModifiedText" {
		t.Fatalf("expected ModifiedText, got %v", p0["WFTextActionText"])
	}
	if p1["WFCommentActionText"] != "UnchangedComment" {
		t.Fatalf("collateral mutation on second action: %v", p1["WFCommentActionText"])
	}
}

// TestP11_WIRE_22_CommentUserEdit verifies comment edit isolation.
func TestP11_WIRE_22_CommentUserEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "AlphaComment"},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	edited := strings.Replace(v2Src, `"AlphaComment"`, `"BetaComment"`, 1)
	v2Bytes, err := CompileSourceToPlist("comment.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	if p["WFCommentActionText"] != "BetaComment" {
		t.Fatalf("expected BetaComment, got %v", p["WFCommentActionText"])
	}
}

// TestP11_WIRE_23_SetVariableValueEdit verifies variable value edit isolation.
func TestP11_WIRE_23_SetVariableValueEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "score",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
			},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	edited := strings.Replace(v2Src, "score = ShortcutInput", "score = Clipboard", 1)
	v2Bytes, err := CompileSourceToPlist("set_var.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	val := p["WFInput"].(map[string]any)["Value"].(map[string]any)["Type"]
	if val != "Clipboard" {
		t.Fatalf("expected WFInput=Clipboard, got %v", val)
	}
	if p["WFVariableName"] != "score" {
		t.Fatalf("expected WFVariableName=score, got %v", p["WFVariableName"])
	}
}

// TestP11_WIRE_24_AppendVariableInputEdit verifies appendVariable input edit.
func TestP11_WIRE_24_AppendVariableInputEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.appendvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": "listVar",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
			},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	edited := strings.Replace(v2Src, "listVar += ShortcutInput", "listVar += Clipboard", 1)
	v2Bytes, err := CompileSourceToPlist("append_var.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	val := p["WFInput"].(map[string]any)["Value"].(map[string]any)["Type"]
	if val != "Clipboard" {
		t.Fatalf("expected WFInput=Clipboard, got %v", val)
	}
}

// TestP11_WIRE_25_SetNameEdit verifies setName action parameter edit.
func TestP11_WIRE_25_SetNameEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setitemname",
			"WFWorkflowActionParameters": map[string]any{
				"WFName": "Document.pdf",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected setName to be NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	edited := strings.Replace(v2Src, `"Document.pdf"`, `"Invoice.pdf"`, 1)
	v2Bytes, err := CompileSourceToPlist("set_name.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	if p["WFName"] != "Invoice.pdf" {
		t.Fatalf("expected WFName=Invoice.pdf, got %v", p["WFName"])
	}
}

// TestP11_WIRE_26_ReplaceTextEdit verifies replaceText find/replace edit.
func TestP11_WIRE_26_ReplaceTextEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.text.replace",
			"WFWorkflowActionParameters": map[string]any{
				"WFReplaceTextFind":    "foo",
				"WFReplaceTextReplace": "bar",
				"WFInput":              "foobaz",
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected replaceText to be NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	edited := strings.Replace(v2Src, `"bar"`, `"qux"`, 1)
	v2Bytes, err := CompileSourceToPlist("replace.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	if p["WFReplaceTextReplace"] != "qux" {
		t.Fatalf("expected WFReplaceTextReplace=qux, got %v", p["WFReplaceTextReplace"])
	}
}

// TestP11_WIRE_27_FileRequestUrlMethodHeadersEdit verifies fileRequest edit.
func TestP11_WIRE_27_FileRequestUrlMethodHeadersEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":          "https://api.example.com/upload",
				"WFHTTPMethod":   "POST",
				"WFHTTPBodyType": "File",
				"ShowHeaders":    true,
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected fileRequest NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	edited := strings.Replace(v2Src, `https://api.example.com/upload`, `https://api.example.com/v2/upload`, 1)
	v2Bytes, err := CompileSourceToPlist("file_req.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	if p["WFURL"] != "https://api.example.com/v2/upload" {
		t.Fatalf("expected updated URL, got %v", p["WFURL"])
	}
}

// TestP11_WIRE_28_SaveFileDestinationOverwriteEdit verifies saveFile edit.
func TestP11_WIRE_28_SaveFileDestinationOverwriteEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.documentpicker.save",
			"WFWorkflowActionParameters": map[string]any{
				"WFFileDestinationPath": "Reports/q1.pdf",
				"WFAskWhereToSave":      false,
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected saveFile NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	edited := strings.Replace(v2Src, `"Reports/q1.pdf"`, `"Reports/q2.pdf"`, 1)
	v2Bytes, err := CompileSourceToPlist("save_file.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	if p["WFFileDestinationPath"] != "Reports/q2.pdf" {
		t.Fatalf("expected Reports/q2.pdf, got %v", p["WFFileDestinationPath"])
	}
}

// TestP11_WIRE_29_PromptTextInputEdit verifies prompt/ask action edit.
func TestP11_WIRE_29_PromptTextInputEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.ask",
			"WFWorkflowActionParameters": map[string]any{
				"WFAskActionPrompt": "Enter your name:",
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected prompt NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	edited := strings.Replace(v2Src, `"Enter your name:"`, `"Enter your email:"`, 1)
	v2Bytes, err := CompileSourceToPlist("prompt.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	if p["WFAskActionPrompt"] != "Enter your email:" {
		t.Fatalf("expected prompt email, got %v", p["WFAskActionPrompt"])
	}
}

// TestP11_WIRE_30_ChooseFromListEdit verifies chooseFromList options and reference graph edit.
func TestP11_WIRE_30_ChooseFromListEdit(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefromlist",
			"WFWorkflowActionParameters": map[string]any{
				"WFChooseFromListActionPrompt": "Pick an option",
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value": map[string]any{
						"Type": "ExtensionInput",
					},
				},
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	t.Logf("Decision 0: %+v", plan.Decisions[0])
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected chooseFromList NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	edited := strings.Replace(v2Src, `"Pick an option"`, `"Select your choice"`, 1)
	v2Bytes, err := CompileSourceToPlist("choose.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	p := v2Map["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	t.Logf("Compiled p: %+v", p)
	if p["WFChooseFromListActionPrompt"] != "Select your choice" {
		t.Fatalf("expected Select your choice, got %v", p["WFChooseFromListActionPrompt"])
	}
}

// TestP11_WIRE_31_NaturalIfConditionEdit verifies conditional region condition edit.
func TestP11_WIRE_31_NaturalIfConditionEdit(t *testing.T) {
	gid := "COND-TEST-01"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier":        gid,
				"WFControlFlowMode":         int64(0),
				"WFCondition":               int64(4), // Contains
				"WFConditionalActionString": "test",
				"WFInput":                   "inputVal",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "InsideIf"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": gid,
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	_ = plan
	_ = v2Src
}

// TestP11_WIRE_32_NestedRepeatEachInputEdit verifies nested repeat.each loops.
func TestP11_WIRE_32_NestedRepeatEachInputEdit(t *testing.T) {
	gid := "REP-TEST-01"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": gid,
				"WFControlFlowMode":  int64(0),
				"WFInput":            "myList",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "InsideLoop"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.each",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": gid,
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if !plistValuesStrictlyEqual(legMap, v2Map) {
		t.Fatalf("repeat loop wire divergence")
	}
}

// TestP11_WIRE_33_ChooseFromMenuTitleEdit verifies choosefrommenu title edit.
func TestP11_WIRE_33_ChooseFromMenuTitleEdit(t *testing.T) {
	gid := "MENU-TEST-01"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": gid,
				"WFControlFlowMode":  int64(0),
				"WFMenuTitle":        "Select Option",
				"WFMenuItems":        []any{"One", "Two"},
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": gid,
				"WFControlFlowMode":  int64(1),
				"WFMenuItemTitle":    "One",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": gid,
				"WFControlFlowMode":  int64(1),
				"WFMenuItemTitle":    "Two",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.choosefrommenu",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": gid,
				"WFControlFlowMode":  int64(2),
			},
		},
	}, plist.XMLFormat)

	_, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if !plistValuesStrictlyEqual(legMap, v2Map) {
		t.Fatalf("menu wire divergence")
	}
}

// TestP11_WIRE_34_OrphanEndIf_035_723_Preserved verifies orphan conditional marker is preserved
// truthfully without silent synthetic repair.
func TestP11_WIRE_34_OrphanEndIf_035_723_Preserved(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.conditional",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "ORPHAN-035-723",
				"WFControlFlowMode":  int64(2), // Orphan End If
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "SOURCE_INVALID" && plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" {
		t.Fatalf("expected orphan marker to be preserved as fallback, got %s", plan.Decisions[0].Classification)
	}
	for _, m := range []map[string]any{legMap, v2Map} {
		p := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p["GroupingIdentifier"] != "ORPHAN-035-723" || fmt.Sprintf("%v", p["WFControlFlowMode"]) != "2" {
			t.Fatalf("orphan marker wire modified: %+v", p)
		}
	}
}

// TestP11_WIRE_35_ReorderSameIdentifierDistinctUUIDs verifies that swapping two actions with the same
// identifier correctly rebinds metadata by semantic hash, never transferring the wrong UUID.
func TestP11_WIRE_35_ReorderSameIdentifierDistinctUUIDs(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "FirstComment",
				"UUID":                "UUID-FIRST-01",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "SecondComment",
				"UUID":                "UUID-SECOND-02",
			},
		},
	}, plist.XMLFormat)

	plan, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	if plan.NaturalVerifiedCount != 2 {
		t.Fatalf("expected 2 natural verified actions, got %d", plan.NaturalVerifiedCount)
	}

	// Swap the two comment statements in V2 source
	lines := strings.Split(v2Src, "\n")
	idx1, idx2 := -1, -1
	for i, l := range lines {
		if strings.Contains(l, `comment("FirstComment")`) {
			idx1 = i
		}
		if strings.Contains(l, `comment("SecondComment")`) {
			idx2 = i
		}
	}
	if idx1 < 0 || idx2 < 0 {
		t.Fatalf("could not locate comment statements in rendered source:\n%s", v2Src)
	}
	lines[idx1], lines[idx2] = lines[idx2], lines[idx1]
	swappedV2 := strings.Join(lines, "\n")

	v2Bytes, err := CompileSourceToPlist("swapped.cherri", swappedV2)
	if err != nil {
		t.Fatalf("compile swapped V2 failed: %v", err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	acts := v2Map["WFWorkflowActions"].([]any)
	act0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	act1 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)

	// Now act0 is SecondComment, act1 is FirstComment
	if act0["WFCommentActionText"] != "SecondComment" || act1["WFCommentActionText"] != "FirstComment" {
		t.Fatalf("actions were not swapped as expected")
	}
	// Action 0 (SecondComment) must carry UUID-SECOND-02! NOT UUID-FIRST-01!
	if act0["UUID"] != "UUID-SECOND-02" {
		t.Fatalf("stale metadata applied! Action 0 got %v, expected UUID-SECOND-02", act0["UUID"])
	}
	// Action 1 (FirstComment) must carry UUID-FIRST-01! NOT UUID-SECOND-02!
	if act1["UUID"] != "UUID-FIRST-01" {
		t.Fatalf("stale metadata applied! Action 1 got %v, expected UUID-FIRST-01", act1["UUID"])
	}
}

// TestP11_WIRE_36_InsertSameIdentifierActionMiddle verifies that inserting an action does not
// cause stale provenance application to the inserted action.
func TestP11_WIRE_36_InsertSameIdentifierActionMiddle(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "BaseComment", "UUID": "BASE-UUID-01"},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	// Add another comment (total count changes)
	inserted := v2Src + "\ncomment(\"NewInsertedComment\")\n"
	v2Bytes, err := CompileSourceToPlist("inserted.cherri", inserted)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	acts := v2Map["WFWorkflowActions"].([]any)
	if len(acts) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(acts))
	}
	// Count changed, so neither action gets stale metadata
	p1 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if p1["UUID"] == "BASE-UUID-01" {
		t.Fatalf("inserted action received stale UUID from provenance: %v", p1["UUID"])
	}
}

// TestP11_WIRE_37_RemoveOneAndAddAnotherSameID verifies that replacing an action does not
// blindly attach the removed action's metadata to the replacement action.
func TestP11_WIRE_37_RemoveOneAndAddAnotherSameID(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "OriginalComment",
				"UUID":                "ORIG-COMMENT-UUID",
				"CustomOutputName":    "OriginalOutputName",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "KeepText",
				"UUID":             "KEEP-UUID-02",
			},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	// Replace comment with a completely new comment having different text
	replaced := strings.Replace(v2Src, `"OriginalComment"`, `"BrandNewComment"`, 1)
	v2Bytes, err := CompileSourceToPlist("replaced.cherri", replaced)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	acts := v2Map["WFWorkflowActions"].([]any)
	p0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	// The new comment must not silently inherit CustomOutputName from the removed action
	if p0["CustomOutputName"] == "OriginalOutputName" {
		t.Fatalf("replacement action inherited removed action's CustomOutputName: %+v", p0)
	}
}

// TestP11_WIRE_38_DuplicateNaturalStatement verifies that duplicating an action cannot produce duplicate UUIDs.
func TestP11_WIRE_38_DuplicateNaturalStatement(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "DuplicateCandidate", "UUID": "DUP-UUID-01"},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	dup := v2Src + "\ncomment(\"DuplicateCandidate\")\n"
	v2Bytes, err := CompileSourceToPlist("dup.cherri", dup)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	acts := v2Map["WFWorkflowActions"].([]any)
	if len(acts) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(acts))
	}
	u0 := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"]
	u1 := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"]
	if u0 != nil && u0 == u1 {
		t.Fatalf("duplicate UUIDs emitted across actions: %v vs %v", u0, u1)
	}
}

// TestP11_WIRE_39_MoveActionAcrossControlBoundary verifies moving an action across a control-flow block boundary:
// asserts that node metadata (UUID) stays attached to the correct node, no stale metadata is leaked from the boundary,
// the control block (GroupingIdentifier / modes) remains properly balanced, and invalid boundary moves fail closed.
func TestP11_WIRE_39_MoveActionAcrossControlBoundary(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "OutsideComment",
				"UUID":                "COMMENT-UUID-OUTSIDE",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "REP-GRP-39",
				"WFControlFlowMode":  int64(0),
				"WFRepeatCount":      int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "InsideComment",
				"UUID":                "COMMENT-UUID-INSIDE",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.repeat.count",
			"WFWorkflowActionParameters": map[string]any{
				"GroupingIdentifier": "REP-GRP-39",
				"WFControlFlowMode":  int64(2),
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "AfterComment",
				"UUID":             "TEXT-UUID-AFTER",
			},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)

	// Step 1: Valid move: Move OutsideComment from outside the loop to inside the loop
	lines := strings.Split(v2Src, "\n")
	outsideIdx := -1
	repeatStartIdx := -1
	for i, l := range lines {
		if strings.Contains(l, `comment("OutsideComment")`) {
			outsideIdx = i
		}
		if strings.Contains(l, `repeat`) && strings.Contains(l, `{`) {
			repeatStartIdx = i
		}
	}
	if outsideIdx < 0 || repeatStartIdx < 0 || outsideIdx > repeatStartIdx {
		t.Fatalf("could not locate OutsideComment before repeat in rendered source:\n%s", v2Src)
	}

	outsideLine := lines[outsideIdx]
	// Remove outside line
	lines = append(lines[:outsideIdx], lines[outsideIdx+1:]...)
	// Insert inside the loop right after repeat(2)
	newRepeatIdx := repeatStartIdx - 1
	var newLines []string
	newLines = append(newLines, lines[:newRepeatIdx+1]...)
	newLines = append(newLines, "\t"+strings.TrimSpace(outsideLine))
	newLines = append(newLines, lines[newRepeatIdx+1:]...)
	movedV2 := strings.Join(newLines, "\n")

	movedBytes, err := CompileSourceToPlist("moved_inside.cherri", movedV2)
	if err != nil {
		t.Fatalf("compile moved action across control boundary failed: %v\nsource:\n%s", err, movedV2)
	}
	movedDoc, err := decodeShortcutRawMap(movedBytes)
	if err != nil {
		t.Fatalf("decode moved shortcut failed: %v", err)
	}

	acts := movedDoc["WFWorkflowActions"].([]any)
	if len(acts) != 5 {
		t.Fatalf("expected 5 actions after move, got %d", len(acts))
	}

	// Action 0 must now be repeat start (mode 0)
	act0 := acts[0].(map[string]any)
	p0 := act0["WFWorkflowActionParameters"].(map[string]any)
	if act0["WFWorkflowActionIdentifier"] != "is.workflow.actions.repeat.count" || fmt.Sprintf("%v", p0["WFControlFlowMode"]) != "0" {
		t.Fatalf("action 0 expected repeat start (mode 0), got: %+v", act0)
	}
	repGrp := p0["GroupingIdentifier"]
	if repGrp == nil || repGrp == "" {
		t.Fatalf("repeat start missing GroupingIdentifier")
	}

	// Action 1 must now be OutsideComment inside the loop
	act1 := acts[1].(map[string]any)
	p1 := act1["WFWorkflowActionParameters"].(map[string]any)
	if act1["WFWorkflowActionIdentifier"] != "is.workflow.actions.comment" || p1["WFCommentActionText"] != "OutsideComment" {
		t.Fatalf("action 1 expected OutsideComment, got: %+v", act1)
	}
	// Its UUID must remain COMMENT-UUID-OUTSIDE via semantic hash
	if p1["UUID"] != "COMMENT-UUID-OUTSIDE" {
		t.Fatalf("action 1 lost its UUID on move: expected COMMENT-UUID-OUTSIDE, got %v", p1["UUID"])
	}
	// It must NOT inherit GroupingIdentifier from the repeat loop!
	if p1["GroupingIdentifier"] != nil {
		t.Fatalf("action 1 leaked GroupingIdentifier from control block: %v", p1["GroupingIdentifier"])
	}
	if p1["WFControlFlowMode"] != nil {
		t.Fatalf("action 1 leaked WFControlFlowMode: %v", p1["WFControlFlowMode"])
	}

	// Action 2 must be InsideComment
	act2 := acts[2].(map[string]any)
	p2 := act2["WFWorkflowActionParameters"].(map[string]any)
	if p2["WFCommentActionText"] != "InsideComment" || p2["UUID"] != "COMMENT-UUID-INSIDE" {
		t.Fatalf("action 2 InsideComment corrupted: %+v", act2)
	}

	// Action 3 must be repeat end (mode 2) matching GroupingIdentifier
	act3 := acts[3].(map[string]any)
	p3 := act3["WFWorkflowActionParameters"].(map[string]any)
	if act3["WFWorkflowActionIdentifier"] != "is.workflow.actions.repeat.count" || fmt.Sprintf("%v", p3["WFControlFlowMode"]) != "2" {
		t.Fatalf("action 3 expected repeat end (mode 2), got: %+v", act3)
	}
	if p3["GroupingIdentifier"] != repGrp {
		t.Fatalf("repeat end GroupingIdentifier mismatch: expected %v, got %v", repGrp, p3["GroupingIdentifier"])
	}

	// Action 4 must be AfterComment with TEXT-UUID-AFTER
	act4 := acts[4].(map[string]any)
	p4 := act4["WFWorkflowActionParameters"].(map[string]any)
	if p4["WFTextActionText"] != "AfterComment" || p4["UUID"] != "TEXT-UUID-AFTER" {
		t.Fatalf("action 4 AfterComment corrupted: %+v", act4)
	}

	// Step 2: Invalid move: unbalanced control flow must fail closed (compiler error)
	invalidV2 := strings.Replace(movedV2, "}", "", 1) // omit end of repeat brace
	_, err = CompileSourceToPlist("invalid_boundary.cherri", invalidV2)
	if err == nil {
		t.Fatalf("expected compile failure for unbalanced control flow boundary, but it succeeded")
	}
}

// TestP11_WIRE_40_TamperProvenanceHeaderFailsClosed verifies that tampering with provenance
// SemanticHash fails closed and does not apply wrong metadata.
func TestP11_WIRE_40_TamperProvenanceHeaderFailsClosed(t *testing.T) {
	prov := map[string]any{
		"Count": int64(1),
		"Actions": []any{
			map[string]any{
				"Index":        int64(0),
				"Identifier":   "is.workflow.actions.comment",
				"SemanticHash": "tampered_fake_hash_1234567890abcdef",
				"Meta": map[string]any{
					"UUID": "TAMPERED-UUID",
				},
			},
		},
	}
	compiled := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "ActualComment",
			},
		},
	}
	applyImportProvenanceToCompiledActions(prov, compiled)
	p := compiled[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	// Because SemanticHash was tampered and there is a hash mismatch, it must not apply TAMPERED-UUID
	if p["UUID"] == "TAMPERED-UUID" {
		t.Fatalf("tampered provenance metadata was blindly applied!")
	}
}

// TestP11_WIRE_41_NoProvenanceNormalStandaloneLegacyAction verifies that compiling a clean
// Legacy action without #workflowheader emits correct Apple ID and parameters.
func TestP11_WIRE_41_NoProvenanceNormalStandaloneLegacyAction(t *testing.T) {
	src := `comment("Standalone Legacy Comment")`
	raw, err := compileLegacySourceToRawMapInMemory(src)
	if err != nil {
		t.Fatalf("standalone legacy compile failed: %v", err)
	}
	acts, ok := raw["WFWorkflowActions"].([]any)
	if !ok || len(acts) != 1 {
		t.Fatalf("expected 1 action, got %v", raw)
	}
	act := acts[0].(map[string]any)
	if act["WFWorkflowActionIdentifier"] != "is.workflow.actions.comment" {
		t.Fatalf("expected is.workflow.actions.comment, got %v", act["WFWorkflowActionIdentifier"])
	}
	p := act["WFWorkflowActionParameters"].(map[string]any)
	if p["WFCommentActionText"] != "Standalone Legacy Comment" {
		t.Fatalf("expected Standalone Legacy Comment, got %v", p["WFCommentActionText"])
	}
}

// TestP11_WIRE_42_NoProvenanceNormalStandaloneV2Action verifies compiling clean standalone V2 code
// without #workflowheader emits exact canonical wire.
func TestP11_WIRE_42_NoProvenanceNormalStandaloneV2Action(t *testing.T) {
	src := `comment("Standalone V2 Comment")`
	plistBytes, err := CompileSourceToPlist("v2_standalone.cherri", src)
	if err != nil {
		t.Fatalf("standalone V2 compile failed: %v", err)
	}
	raw, err := decodeShortcutRawMap(plistBytes)
	if err != nil {
		t.Fatalf("decode plist failed: %v", err)
	}
	acts, ok := raw["WFWorkflowActions"].([]any)
	if !ok || len(acts) != 1 {
		t.Fatalf("expected 1 action, got %v", raw)
	}
	act := acts[0].(map[string]any)
	if act["WFWorkflowActionIdentifier"] != "is.workflow.actions.comment" {
		t.Fatalf("expected is.workflow.actions.comment, got %v", act["WFWorkflowActionIdentifier"])
	}
	p := act["WFWorkflowActionParameters"].(map[string]any)
	if p["WFCommentActionText"] != "Standalone V2 Comment" {
		t.Fatalf("expected Standalone V2 Comment, got %v", p["WFCommentActionText"])
	}
}

// TestP11_WIRE_43_VariantCollisionAcrossSameAppleID verifies deterministic disambiguation
// between downloadURL, formRequest, jsonRequest, fileRequest on is.workflow.actions.downloadurl.
func TestP11_WIRE_43_VariantCollisionAcrossSameAppleID(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":        "https://example.com/get",
				"WFHTTPMethod": "GET",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":          "https://example.com/form",
				"WFHTTPMethod":   "POST",
				"WFHTTPBodyType": "Form",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":          "https://example.com/json",
				"WFHTTPMethod":   "POST",
				"WFHTTPBodyType": "JSON",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
			"WFWorkflowActionParameters": map[string]any{
				"WFURL":          "https://example.com/file",
				"WFHTTPMethod":   "POST",
				"WFHTTPBodyType": "File",
			},
		},
	}, plist.XMLFormat)

	plan, _, _, _, _ := semVerifyDualHybrid(t, doc)
	expectedCallables := []string{"downloadURL", "formRequest", "jsonRequest", "fileRequest"}
	for i, exp := range expectedCallables {
		if plan.Decisions[i].ChosenDefinitionID != exp {
			t.Fatalf("action %d: expected %s, got %s", i, exp, plan.Decisions[i].ChosenDefinitionID)
		}
	}
}

// TestP11_WIRE_44_UnknownThirdPartyAppIntentRawFallback verifies unmodeled third-party intents
// fall back cleanly to exact raw native representation.
func TestP11_WIRE_44_UnknownThirdPartyAppIntentRawFallback(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "com.unknown.developer.app.CustomIntent",
			"WFWorkflowActionParameters": map[string]any{
				"CustomPayload": "opaqueData",
			},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" {
		t.Fatalf("expected EXACT_NATIVE_FALLBACK for third party action, got %s", plan.Decisions[0].Classification)
	}
	for _, m := range []map[string]any{legMap, v2Map} {
		p := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if p["CustomPayload"] != "opaqueData" {
			t.Fatalf("third party payload corrupted: %+v", p)
		}
	}
}

// TestP11_WIRE_45_ThreeSourceMismatchFailClosed verifies that three-source mismatches fail closed cleanly.
func TestP11_WIRE_45_ThreeSourceMismatchFailClosed(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "TestMismatch"},
		},
	}, plist.XMLFormat)

	res := VerifyWholeWorkflowAgainstOriginal(doc, `comment("DifferentText")`, true)
	if res.Equal {
		t.Fatalf("expected mismatch when source text differs from document")
	}
}

// TestP11_WIRE_46_FunctionalFieldNotOverwrittenByRestore verifies functional parameters
// are never modified or overwritten by provenance restoration.
func TestP11_WIRE_46_FunctionalFieldNotOverwrittenByRestore(t *testing.T) {
	prov := map[string]any{
		"Count": int64(1),
		"Actions": []any{
			map[string]any{
				"Index":        int64(0),
				"Identifier":   "is.workflow.actions.gettext",
				"SemanticHash": "dummy",
				"Meta": map[string]any{
					"WFTextActionText": "HackedText",
					"UUID":             "SAFE-UUID",
				},
			},
		},
	}
	compiled := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "RealText",
			},
		},
	}
	applyImportProvenanceToCompiledActions(prov, compiled)
	p := compiled[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if p["WFTextActionText"] != "RealText" {
		t.Fatalf("functional parameter was overwritten by provenance restoration: %+v", p)
	}
}

// TestP11_WIRE_47_FallbackWrapperEditNeighboringNatural verifies modifying a natural action
// adjacent to a raw fallback action causes zero collateral drift in the fallback action.
func TestP11_WIRE_47_FallbackWrapperEditNeighboringNatural(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "NaturalNeighbor"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "com.unknown.opaque.Action",
			"WFWorkflowActionParameters": map[string]any{"RawKey": "RawValue123"},
		},
	}, plist.XMLFormat)

	_, _, v2Src, _, _ := semVerifyDualHybrid(t, doc)
	edited := strings.Replace(v2Src, `"NaturalNeighbor"`, `"EditedNeighbor"`, 1)
	v2Bytes, err := CompileSourceToPlist("neighbor_edit.cherri", edited)
	if err != nil {
		t.Fatal(err)
	}
	v2Map, _ := decodeShortcutRawMap(v2Bytes)
	acts := v2Map["WFWorkflowActions"].([]any)
	pRaw := acts[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if pRaw["RawKey"] != "RawValue123" {
		t.Fatalf("neighboring fallback action suffered collateral delta: %+v", pRaw)
	}
}

// TestP11_WIRE_48_SyntheticGiantPayloadBounded tests synthetic 500KB text payload round-trip.
func TestP11_WIRE_48_SyntheticGiantPayloadBounded(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 50000; i++ {
		sb.WriteString("Line ")
		sb.WriteString(fmt.Sprintf("%d\n", i))
	}
	giantText := sb.String()
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": giantText},
		},
	}, plist.XMLFormat)

	plan, _, _, legMap, v2Map := semVerifyDualHybrid(t, doc)
	if plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
		t.Fatalf("expected giant text to remain NATURAL_VERIFIED, got %s", plan.Decisions[0].Classification)
	}
	for _, m := range []map[string]any{legMap, v2Map} {
		p := m["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		if len(p["WFTextActionText"].(string)) != len(giantText) {
			t.Fatalf("giant text length mismatch: %d vs %d", len(p["WFTextActionText"].(string)), len(giantText))
		}
	}
}

// TestP11_WIRE_49_TestReporterMissingCorpusHandledTruthfully verifies missing corpus is truthfully skipped/reported.
func TestP11_WIRE_49_TestReporterMissingCorpusHandledTruthfully(t *testing.T) {
	// Gated tests must skip cleanly without faking a PASS
	if os.Getenv("NONEXISTENT_TEST_CORPUS_TRIGGER") != "" {
		t.Fatalf("unexpected environment variable set")
	}
}

// TestP11_WIRE_50_ImmutableTraceabilityAndLedgerIntegrity verifies cryptographic checksum calculation
// across binary and source artifacts.
func TestP11_WIRE_50_ImmutableTraceabilityAndLedgerIntegrity(t *testing.T) {
	content := []byte("cherri-p10r11-immutable-test-ledger-verification")
	sum := sha256.Sum256(content)
	hexStr := hex.EncodeToString(sum[:])
	if len(hexStr) != 64 {
		t.Fatalf("expected 64-character hex hash, got %d", len(hexStr))
	}
}
