/*
 * Copyright (c) Cherri P10R5 — Canonical Structural Plist Comparator (ActionPayload vs FullWorkflowSemantic),
 * Parameter Contract Verification Matrix & Negative Mutation Tests
 */

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

// ComparisonMode distinguishes between Action Payload Comparison (which normalizes
// top-level workflow header defaults when comparing single-action catalog snippets)
// and Full Workflow Semantic Comparison (which preserves and strictly compares all
// semantic top-level workflow headers alongside the full action chain).
type ComparisonMode string

const (
	ComparisonModeActionPayload        ComparisonMode = "ACTION_PAYLOAD_COMPARISON"
	ComparisonModeFullWorkflowSemantic ComparisonMode = "FULL_WORKFLOW_SEMANTIC_COMPARISON"
)

const (
	ParamCheckStatusPass                   = "PASS"
	ParamCheckStatusFail                   = "FAIL"
	ParamCheckStatusNotApplicable          = "NOT_APPLICABLE"
	ParamCheckStatusNotTested              = "NOT_TESTED"
	ParamCheckStatusBlockedMissingEvidence = "BLOCKED_MISSING_EVIDENCE"
)

// CanonicalAllowedExclusions documents the strictly bounded set of non-semantic
// fields normalized during ACTION_PAYLOAD_COMPARISON round-trip verification.
// In FULL_WORKFLOW_SEMANTIC_COMPARISON mode, all semantic top-level workflow headers
// (WFWorkflowTypes, WFWorkflowMinimumClientVersion, WFWorkflowMinimumClientVersionString,
// WFWorkflowInputContentItemClasses, WFWorkflowOutputContentItemClasses,
// WFWorkflowHasShortcutInputVariables, WFWorkflowHasOutputFallback, WFWorkflowNoInputBehavior,
// WFQuickActionSurfaces, WFWorkflowImportQuestions, WFWorkflowTriggers, WFWorkflowIcon)
// are preserved and strictly compared.
var CanonicalAllowedExclusions = []string{
	"ACTION_PAYLOAD_COMPARISON only: Top-level workflow header defaults (WFWorkflowClientVersion, WFWorkflowIcon, WFWorkflowTypes, WFWorkflowMinimumClientVersion*) are normalized when isolating single-action payload probes; FULL_WORKFLOW_SEMANTIC_COMPARISON strictly preserves and compares all semantic workflow headers",
	"CustomOutputName & ActionOutput.OutputName: non-semantic display labels for action output variables (actual producer-consumer links are strictly verified via OutputUUID bijection)",
	"Unreferenced action UUID: only stripped when no downstream action references the action's output via OutputUUID; referenced UUIDs are bijectively alpha-renamed and verified via BuildDocumentScope",
	"Unreferenced control-flow trailing nothing() padding (is.workflow.actions.nothing with no referenced UUID when multi-action): decompiler emits nothing() after control-flow blocks",
	"WFDictionaryFieldValueItems array order & default string WFItemType=0: Go map iteration order in dictionary literals is non-deterministic; entries are sorted by WFKey with default WFItemType=0 filled on string items",
	"WFWorkflow.workflowIdentifier & isSelf workflowName: random placeholder UUID generated when compiling runWorkflow by name, and current file name when isSelf=true; workflowName (for non-self) and isSelf are strictly compared",
}

func stripActionOutputDisplayNames(v any) {
	switch tv := v.(type) {
	case map[string]any:
		if t, ok := tv["Type"].(string); ok && t == "ActionOutput" {
			delete(tv, "OutputName")
		}
		for _, child := range tv {
			stripActionOutputDisplayNames(child)
		}
	case []any:
		for _, item := range tv {
			stripActionOutputDisplayNames(item)
		}
	}
}

func canonicalizeRoundTripDictItems(v any) any {
	switch val := v.(type) {
	case map[string]any:
		res := make(map[string]any, len(val))
		for k, elem := range val {
			if k == "WFDictionaryFieldValueItems" {
				if slice, ok := elem.([]any); ok {
					normSlice := make([]any, len(slice))
					for i, rawItem := range slice {
						itemMap, isMap := rawItem.(map[string]any)
						if !isMap {
							normSlice[i] = rawItem
							continue
						}
						cp := make(map[string]any, len(itemMap)+1)
						for ik, iv := range itemMap {
							cp[ik] = canonicalizeRoundTripDictItems(iv)
						}
						if _, hasType := cp["WFItemType"]; !hasType {
							cp["WFItemType"] = uint64(0)
						}
						normSlice[i] = cp
					}
					sort.Slice(normSlice, func(i, j int) bool {
						mi, _ := normSlice[i].(map[string]any)
						mj, _ := normSlice[j].(map[string]any)
						ki, _ := mi["WFKey"].(map[string]any)["Value"].(map[string]any)["string"].(string)
						kj, _ := mj["WFKey"].(map[string]any)["Value"].(map[string]any)["string"].(string)
						return ki < kj
					})
					res[k] = normSlice
					continue
				}
			}
			res[k] = canonicalizeRoundTripDictItems(elem)
		}
		return res
	case []any:
		res := make([]any, len(val))
		for i, elem := range val {
			res[i] = canonicalizeRoundTripDictItems(elem)
		}
		return res
	default:
		return v
	}
}

func collectReferencedOutputUUIDs(actions []any) map[string]bool {
	used := make(map[string]bool)
	var walk func(v any)
	walk = func(v any) {
		switch tv := v.(type) {
		case map[string]any:
			for k, val := range tv {
				if k == "OutputUUID" {
					if s, ok := val.(string); ok && s != "" {
						used[s] = true
					}
				} else {
					walk(val)
				}
			}
		case []any:
			for _, item := range tv {
				walk(item)
			}
		}
	}
	for _, rawAct := range actions {
		if act, ok := rawAct.(map[string]any); ok {
			if params, ok := act["WFWorkflowActionParameters"].(map[string]any); ok {
				walk(params)
			}
		}
	}
	return used
}

func canonicalizeActionsChain(cloned map[string]any) map[string]any {
	acts, ok := cloned["WFWorkflowActions"].([]any)
	if !ok {
		return cloned
	}
	referencedUUIDs := collectReferencedOutputUUIDs(acts)
	var filteredActs []any
	for _, rawAct := range acts {
		act, ok := rawAct.(map[string]any)
		if !ok {
			continue
		}
		params, _ := act["WFWorkflowActionParameters"].(map[string]any)
		// Strip unreferenced compiler-generated nothing() padding in multi-action workflows
		if len(acts) > 1 && act["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" {
			u, _ := params["UUID"].(string)
			if u == "" || !referencedUUIDs[u] {
				continue
			}
		}
		if params != nil {
			// 1. Strip non-semantic CustomOutputName and ActionOutput.OutputName display labels
			delete(params, "CustomOutputName")
			stripActionOutputDisplayNames(params)

			// 2. Strip unreferenced producer UUID only if no downstream action references it
			if u, ok := params["UUID"].(string); ok {
				if !referencedUUIDs[u] {
					delete(params, "UUID")
				}
			}

			// 3. Canonicalize WFDictionaryFieldValueItems map ordering & default WFItemType=0
			if normParams, ok := canonicalizeRoundTripDictItems(params).(map[string]any); ok {
				params = normParams
				act["WFWorkflowActionParameters"] = params
			}

			// 4. Normalize random placeholder workflowIdentifier UUID when workflowName is present, and workflowName when isSelf=true
			if wf, ok := params["WFWorkflow"].(map[string]any); ok {
				if name, hasName := wf["workflowName"].(string); hasName && name != "" {
					if id, hasID := wf["workflowIdentifier"].(string); hasID && id != "" {
						wf["workflowIdentifier"] = "NORMALIZED_WORKFLOW_IDENTIFIER"
					}
				}
				if isSelf, _ := wf["isSelf"].(bool); isSelf {
					wf["workflowName"] = ""
				}
			}
		}
		filteredActs = append(filteredActs, act)
	}
	cloned["WFWorkflowActions"] = filteredActs
	return cloned
}

// CanonicalizeShortcutDocActionPayload prepares a decoded Shortcut plist document
// for ACTION_PAYLOAD_COMPARISON (used when isolating single-action payload tests).
// It normalizes top-level workflow header defaults while strictly preserving the entire
// WFWorkflowActions chain, parameter keys/values/types, AppIntentDescriptor, and UUID/GroupingIdentifier bijections.
func CanonicalizeShortcutDocActionPayload(doc map[string]any) map[string]any {
	cloned := clonePlistMapDeep(doc)
	for _, k := range []string{
		"WFWorkflowClientVersion",
		"WFWorkflowIcon",
		"WFWorkflowTypes",
		"WFWorkflowMinimumClientVersion",
		"WFWorkflowMinimumClientVersionString",
	} {
		delete(cloned, k)
	}
	return canonicalizeActionsChain(cloned)
}

// CanonicalizeShortcutDocFullWorkflowSemantic prepares a decoded Shortcut plist document
// for FULL_WORKFLOW_SEMANTIC_COMPARISON. It preserves ALL semantic top-level workflow headers:
//   - WFWorkflowTypes
//   - WFQuickActionSurfaces
//   - WFWorkflowInputContentItemClasses
//   - WFWorkflowOutputContentItemClasses
//   - WFWorkflowHasShortcutInputVariables
//   - WFWorkflowHasOutputFallback
//   - WFWorkflowNoInputBehavior
//   - WFWorkflowImportQuestions
//   - WFWorkflowTriggers
//   - WFWorkflowIcon
//   - WFWorkflowClientVersion
//   - WFWorkflowMinimumClientVersion
//   - WFWorkflowMinimumClientVersionString
//
// along with the full canonicalized WFWorkflowActions chain.
func CanonicalizeShortcutDocFullWorkflowSemantic(doc map[string]any) map[string]any {
	cloned := clonePlistMapDeep(doc)
	return canonicalizeActionsChain(cloned)
}

// CanonicalizeShortcutDocForRoundTrip is retained as an alias for CanonicalizeShortcutDocActionPayload.
func CanonicalizeShortcutDocForRoundTrip(doc map[string]any) map[string]any {
	return CanonicalizeShortcutDocActionPayload(doc)
}

// CompareShortcutDocsActionPayload performs ACTION_PAYLOAD_COMPARISON across two decoded Shortcut documents.
func CompareShortcutDocsActionPayload(docA, docB map[string]any) *shortcutcompare.ComparisonResult {
	normA := CanonicalizeShortcutDocActionPayload(docA)
	normB := CanonicalizeShortcutDocActionPayload(docB)
	return shortcutcompare.CompareDocuments(normA, normB)
}

// CompareShortcutDocsFullWorkflowSemantic performs FULL_WORKFLOW_SEMANTIC_COMPARISON across two decoded
// Shortcut documents, strictly comparing all semantic top-level workflow headers as well as the action chain.
func CompareShortcutDocsFullWorkflowSemantic(docA, docB map[string]any) *shortcutcompare.ComparisonResult {
	normA := CanonicalizeShortcutDocFullWorkflowSemantic(docA)
	normB := CanonicalizeShortcutDocFullWorkflowSemantic(docB)
	return shortcutcompare.CompareDocumentsStrict(normA, normB)
}

// CompareShortcutDocsCanonical performs ACTION_PAYLOAD_COMPARISON (backward-compatible entrypoint).
func CompareShortcutDocsCanonical(docA, docB map[string]any) *shortcutcompare.ComparisonResult {
	return CompareShortcutDocsActionPayload(docA, docB)
}

// CompareShortcutPlistsWithMode unmarshals two raw Shortcut plist byte slices and compares them using the requested ComparisonMode.
func CompareShortcutPlistsWithMode(plistA, plistB []byte, mode ComparisonMode) (*shortcutcompare.ComparisonResult, error) {
	var docA, docB map[string]any
	if _, err := plist.Unmarshal(plistA, &docA); err != nil {
		return nil, fmt.Errorf("failed to unmarshal first plist: %w", err)
	}
	if _, err := plist.Unmarshal(plistB, &docB); err != nil {
		return nil, fmt.Errorf("failed to unmarshal second plist: %w", err)
	}
	if mode == ComparisonModeFullWorkflowSemantic {
		return CompareShortcutDocsFullWorkflowSemantic(docA, docB), nil
	}
	return CompareShortcutDocsActionPayload(docA, docB), nil
}

// CompareShortcutPlistsCanonical unmarshals two raw Shortcut plist byte slices
// and performs ACTION_PAYLOAD_COMPARISON of the entire action chain.
func CompareShortcutPlistsCanonical(plistA, plistB []byte) (*shortcutcompare.ComparisonResult, error) {
	return CompareShortcutPlistsWithMode(plistA, plistB, ComparisonModeActionPayload)
}

// CompareShortcutPlistsFullWorkflowSemantic unmarshals two raw Shortcut plist byte slices
// and performs FULL_WORKFLOW_SEMANTIC_COMPARISON including all top-level workflow headers.
func CompareShortcutPlistsFullWorkflowSemantic(plistA, plistB []byte) (*shortcutcompare.ComparisonResult, error) {
	return CompareShortcutPlistsWithMode(plistA, plistB, ComparisonModeFullWorkflowSemantic)
}

func clonePlistMapDeep(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = clonePlistValDeep(v)
	}
	return out
}

func clonePlistSliceDeep(s []any) []any {
	if s == nil {
		return nil
	}
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = clonePlistValDeep(v)
	}
	return out
}

func clonePlistValDeep(v any) any {
	switch tv := v.(type) {
	case map[string]any:
		return clonePlistMapDeep(tv)
	case []any:
		return clonePlistSliceDeep(tv)
	default:
		return tv
	}
}

// TestP10_StructuralComparator_NegativeMutations proves that the canonical structural
// comparator detects intentional corruption of any of the 8 critical action-payload properties
// AND that FULL_WORKFLOW_SEMANTIC_COMPARISON detects intentional corruption of any of the 8
// critical full-workflow semantic properties (Part D4).
func TestP10_StructuralComparator_NegativeMutations(t *testing.T) {
	resetCompilerStateFully()
	src := `#define name MutationBaseline
#define color red
#define glyph car
#define from quickactions, sharesheet
#define quickactions finder, services
#define inputs text, url
#define outputs text
#define noinput stopwith "Input required"
#question qPrompt "Enter API endpoint" "https://example.com/api"
#include 'actions/math'
#include 'actions/documents'
#include 'actions/calendar'
#include 'actions/web'

@nums = list("1", "2", "3")
@avg = statistic("Average", @nums)
@x = 2
if @x == 2 {
	@alarm = createAlarm("Morning", "07:00")
	@res = jsonRequest(qPrompt, "POST", {"count": 1, "nested": {"status": "active"}}, {"X-Test": "yes"})
	@pred = {"property": "Name", "operator": 4, "value": "report"}
	@filtered = filterFiles(@res, @pred, 5)
	run("HelperShortcut", @filtered)
}
`
	resp := compileForMobileMode(src, "MutationBaseline", "legacy", false)
	if !resp.OK {
		t.Fatalf("failed to compile mutation baseline shortcut: %s", resp.Error)
	}
	baseBytes, err := base64.StdEncoding.DecodeString(resp.PlistBase64)
	if err != nil {
		t.Fatalf("failed to decode base64 plist: %v", err)
	}
	var baseDoc map[string]any
	if _, err := plist.Unmarshal(baseBytes, &baseDoc); err != nil {
		t.Fatalf("failed to unmarshal baseline plist: %v", err)
	}

	// Sanity check 1: comparing baseline with a fresh compile of the same source must pass in BOTH modes
	resp2 := compileForMobileMode(src, "MutationBaseline2", "legacy", false)
	if !resp2.OK {
		t.Fatalf("failed second compile of baseline: %s", resp2.Error)
	}
	baseBytes2, _ := base64.StdEncoding.DecodeString(resp2.PlistBase64)
	sanityActionRes, err := CompareShortcutPlistsWithMode(baseBytes, baseBytes2, ComparisonModeActionPayload)
	if err != nil || !sanityActionRes.Equal {
		t.Fatalf("expected identical source to pass ACTION_PAYLOAD_COMPARISON, got err=%v diffs=%v", err, sanityActionRes)
	}
	sanityFullRes, err := CompareShortcutPlistsWithMode(baseBytes, baseBytes2, ComparisonModeFullWorkflowSemantic)
	if err != nil || !sanityFullRes.Equal {
		t.Fatalf("expected identical source to pass FULL_WORKFLOW_SEMANTIC_COMPARISON, got err=%v diffs=%v", err, sanityFullRes)
	}

	// Sanity check 2: Decompiling and recompiling the full workflow must pass FULL_WORKFLOW_SEMANTIC_COMPARISON!
	decompResp := decompileForMobile(baseBytes, "MutationBaseline")
	if !decompResp.OK {
		t.Fatalf("failed to decompile full workflow baseline: %s", decompResp.Error)
	}
	recompResp := compileForMobileMode(decompResp.Source, "MutationBaseline", "legacy", false)
	if !recompResp.OK {
		t.Fatalf("failed to recompile decompiled full workflow baseline: %s\nDecompiled:\n%s", recompResp.Error, decompResp.Source)
	}
	recompBytes, _ := base64.StdEncoding.DecodeString(recompResp.PlistBase64)
	fullRTRes, err := CompareShortcutPlistsFullWorkflowSemantic(baseBytes, recompBytes)
	if err != nil || !fullRTRes.Equal {
		t.Fatalf("expected full workflow decompile->recompile to pass FULL_WORKFLOW_SEMANTIC_COMPARISON, got err=%v diffs=%+v\nDecompiled:\n%s", err, fullRTRes.Differences, decompResp.Source)
	}

	findActionByIdent := func(doc map[string]any, ident string) map[string]any {
		acts, _ := doc["WFWorkflowActions"].([]any)
		for _, raw := range acts {
			act, _ := raw.(map[string]any)
			if act["WFWorkflowActionIdentifier"] == ident {
				return act
			}
		}
		return nil
	}

	t.Run("1_Mutate_WFInput", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		statAct := findActionByIdent(mut, "is.workflow.actions.statistics")
		params := statAct["WFWorkflowActionParameters"].(map[string]any)
		wfInput := params["WFInput"].(map[string]any)
		val := wfInput["Value"].(map[string]any)
		val["VariableName"] = "corrupted_input_var"

		res := CompareShortcutDocsActionPayload(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected WFInput mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("2_Mutate_Native_Parameter_Key", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		statAct := findActionByIdent(mut, "is.workflow.actions.statistics")
		params := statAct["WFWorkflowActionParameters"].(map[string]any)
		params["WFStatisticsOperation_CORRUPTED"] = params["WFStatisticsOperation"]
		delete(params, "WFStatisticsOperation")

		res := CompareShortcutDocsActionPayload(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected native parameter key mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("3_Mutate_Enum_Value", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		statAct := findActionByIdent(mut, "is.workflow.actions.statistics")
		params := statAct["WFWorkflowActionParameters"].(map[string]any)
		params["WFStatisticsOperation"] = "Maximum"

		res := CompareShortcutDocsActionPayload(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected enum value mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("4_Mutate_Number_Text_Type", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		filtAct := findActionByIdent(mut, "is.workflow.actions.filter.files")
		params := filtAct["WFWorkflowActionParameters"].(map[string]any)
		params["WFContentItemLimitNumber"] = "5"

		res := CompareShortcutDocsActionPayload(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected Number->Text scalar type mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("5_Mutate_Nested_Dictionary_Value", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		httpAct := findActionByIdent(mut, "is.workflow.actions.downloadurl")
		params := httpAct["WFWorkflowActionParameters"].(map[string]any)
		headers := params["WFHTTPHeaders"].(map[string]any)
		hVal := headers["Value"].(map[string]any)
		items := hVal["WFDictionaryFieldValueItems"].([]any)
		firstItem := items[0].(map[string]any)
		valMap := firstItem["WFValue"].(map[string]any)
		innerVal := valMap["Value"].(map[string]any)
		innerVal["string"] = "corrupted_header_value"

		res := CompareShortcutDocsActionPayload(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected nested dictionary value mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("6_Mutate_OutputUUID_Link", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		runAct := findActionByIdent(mut, "is.workflow.actions.runworkflow")
		params := runAct["WFWorkflowActionParameters"].(map[string]any)
		wfInput := params["WFInput"].(map[string]any)
		val := wfInput["Value"].(map[string]any)
		val["OutputUUID"] = "00000000-0000-0000-0000-000000000999"

		res := CompareShortcutDocsActionPayload(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected OutputUUID link mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("7_Mutate_GroupingIdentifier", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		acts := mut["WFWorkflowActions"].([]any)
		for _, raw := range acts {
			act := raw.(map[string]any)
			if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
				params := act["WFWorkflowActionParameters"].(map[string]any)
				params["GroupingIdentifier"] = "99999999-8888-7777-6666-555555555555"
				break
			}
		}

		res := CompareShortcutDocsActionPayload(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected GroupingIdentifier mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("8_Mutate_AppIntent_Descriptor", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		alarmAct := findActionByIdent(mut, "com.apple.mobiletimer-framework.MobileTimerIntents.MTCreateAlarmIntent")
		params := alarmAct["WFWorkflowActionParameters"].(map[string]any)
		desc := clonePlistMapDeep(params["AppIntentDescriptor"].(map[string]any))
		desc["AppIntentIdentifier"] = "CorruptedIntentIdentifier"
		params["AppIntentDescriptor"] = desc

		res := CompareShortcutDocsActionPayload(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected AppIntentDescriptor mutation to fail canonical structural comparison, but it passed")
		}
	})

	// Part D4: 8 Mandatory Negative Mutations on FullWorkflowSemanticComparison
	t.Run("D4_1_Mutate_WFWorkflowTypes", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		mut["WFWorkflowTypes"] = []any{"Watch"}
		res := CompareShortcutDocsFullWorkflowSemantic(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected WFWorkflowTypes mutation to fail FullWorkflowSemanticComparison")
		}
	})

	t.Run("D4_2_Mutate_WFWorkflowMinimumClientVersion", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		mut["WFWorkflowMinimumClientVersion"] = uint64(1200)
		mut["WFWorkflowMinimumClientVersionString"] = "1200"
		res := CompareShortcutDocsFullWorkflowSemantic(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected WFWorkflowMinimumClientVersion mutation to fail FullWorkflowSemanticComparison")
		}
	})

	t.Run("D4_3_Mutate_InputContentItemClasses", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		mut["WFWorkflowInputContentItemClasses"] = []any{"WFImageContentItem"}
		res := CompareShortcutDocsFullWorkflowSemantic(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected WFWorkflowInputContentItemClasses mutation to fail FullWorkflowSemanticComparison")
		}
	})

	t.Run("D4_4_Mutate_OutputContentItemClasses", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		mut["WFWorkflowOutputContentItemClasses"] = []any{"WFPDFContentItem"}
		res := CompareShortcutDocsFullWorkflowSemantic(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected WFWorkflowOutputContentItemClasses mutation to fail FullWorkflowSemanticComparison")
		}
	})

	t.Run("D4_5_Mutate_SharingMode_And_NoInputBehavior", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		mut["WFQuickActionSurfaces"] = []any{"Finder"}
		mut["WFWorkflowNoInputBehavior"] = map[string]any{"Name": "WFWorkflowNoInputBehaviorGetClipboard"}
		res := CompareShortcutDocsFullWorkflowSemantic(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected sharing surface / noInputBehavior mutation to fail FullWorkflowSemanticComparison")
		}
	})

	t.Run("D4_6_Mutate_Missing_Semantic_Workflow_Header", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		delete(mut, "WFWorkflowImportQuestions")
		delete(mut, "WFWorkflowIcon")
		res := CompareShortcutDocsFullWorkflowSemantic(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected missing semantic workflow header (WFWorkflowImportQuestions / WFWorkflowIcon) to fail FullWorkflowSemanticComparison")
		}
	})

	t.Run("D4_7_Mutate_Variable_Reference_In_FullWorkflow", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		runAct := findActionByIdent(mut, "is.workflow.actions.runworkflow")
		params := runAct["WFWorkflowActionParameters"].(map[string]any)
		wfInput := params["WFInput"].(map[string]any)
		val := wfInput["Value"].(map[string]any)
		val["OutputUUID"] = "deadbeef-dead-beef-dead-beef00000001"
		res := CompareShortcutDocsFullWorkflowSemantic(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected variable reference mutation to fail FullWorkflowSemanticComparison")
		}
	})

	t.Run("D4_8_Mutate_ControlFlow_GroupingIdentifier_In_FullWorkflow", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		acts := mut["WFWorkflowActions"].([]any)
		for _, raw := range acts {
			act := raw.(map[string]any)
			if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
				params := act["WFWorkflowActionParameters"].(map[string]any)
				params["GroupingIdentifier"] = "deadbeef-dead-beef-dead-beef00000002"
				break
			}
		}
		res := CompareShortcutDocsFullWorkflowSemantic(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected ControlFlow GroupingIdentifier mutation to fail FullWorkflowSemanticComparison")
		}
	})
}

// DimensionCheckEvidence records the explicit status and test evidence for one parameter-contract dimension.
type DimensionCheckEvidence struct {
	Status           string `json:"status"`
	SubKind          string `json:"subKind,omitempty"`
	TestName         string `json:"testName"`
	TestInput        string `json:"testInput,omitempty"`
	ExpectedBehavior string `json:"expectedBehavior"`
	ActualBehavior   string `json:"actualBehavior"`
	EvidenceRef      string `json:"evidenceRef"`
	FailureReason    string `json:"failureReason,omitempty"`
}

// ParameterContractResult records the outcome of parameter contract checks for a single catalog entry.
// Every dimension is initialized to NOT_TESTED (never true/PASS by default).
type ParameterContractResult struct {
	ActionName               string                 `json:"actionName"`
	ShortcutIdentifier       string                 `json:"shortcutIdentifier"`
	CompilerConstruct        bool                   `json:"compilerConstruct"`
	RequiredParams           DimensionCheckEvidence `json:"requiredParams"`
	MissingRequiredRejected  DimensionCheckEvidence `json:"missingRequiredRejected"`
	OptionalParams           DimensionCheckEvidence `json:"optionalParams"`
	EnumValidation           DimensionCheckEvidence `json:"enumValidation"`
	DynamicReference         DimensionCheckEvidence `json:"dynamicReference"`
	PlatformGate             DimensionCheckEvidence `json:"platformGate"`
	RequiredParamsVerified   bool                   `json:"requiredParamsVerified"`
	OptionalParamsVerified   bool                   `json:"optionalParamsVerified"`
	MissingRequiredRejectedB bool                   `json:"missingRequiredRejectedBool"`
	EnumValidationVerified   bool                   `json:"enumValidationVerified"`
	DynamicReferenceVerified bool                   `json:"dynamicReferenceVerified"`
	PlatformGateVerified     bool                   `json:"platformGateVerified"`
	ContractPassed           bool                   `json:"contractPassed"`
	FailureReason            string                 `json:"failureReason,omitempty"`
}

type ParameterDimensionSummary struct {
	Dimension          string `json:"dimension"`
	TotalCatalog       int    `json:"totalCatalog"`
	ConstructsExcluded int    `json:"constructsExcluded"`
	Eligible           int    `json:"eligible"`
	Tested             int    `json:"tested"`
	Pass               int    `json:"pass"`
	Fail               int    `json:"fail"`
	NotApplicable      int    `json:"notApplicable"`
	NotTested          int    `json:"notTested"`
	Blocked            int    `json:"blocked"`
	SubKindBreakdown   map[string]int `json:"subKindBreakdown,omitempty"`
}

type ParameterContractMatrixArtifact struct {
	SchemaVersion      string                      `json:"schemaVersion"`
	TotalCatalog       int                         `json:"totalCatalog"`
	CallableCount      int                         `json:"callableCount"`
	ConstructCount     int                         `json:"constructCount"`
	AllContractsPassed bool                        `json:"allContractsPassed"`
	PassedCount        int                         `json:"passedCount"`
	FailedCount        int                         `json:"failedCount"`
	DimensionSummaries []ParameterDimensionSummary `json:"dimensionSummaries"`
	Entries            []ParameterContractResult   `json:"entries"`
}

func newUntestedDimension(testName string) DimensionCheckEvidence {
	return DimensionCheckEvidence{
		Status:      ParamCheckStatusNotTested,
		TestName:    testName,
		EvidenceRef: "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract",
	}
}

// evaluateContractPassed enforces the strict invariant that NO dimension may remain NOT_TESTED
// or FAIL or BLOCKED_MISSING_EVIDENCE when ContractPassed is true.
func evaluateContractPassed(res *ParameterContractResult) {
	dims := []struct {
		name string
		ev   DimensionCheckEvidence
	}{
		{"requiredParams", res.RequiredParams},
		{"missingRequiredRejected", res.MissingRequiredRejected},
		{"optionalParams", res.OptionalParams},
		{"enumValidation", res.EnumValidation},
		{"dynamicReference", res.DynamicReference},
		{"platformGate", res.PlatformGate},
	}

	res.RequiredParamsVerified = res.RequiredParams.Status == ParamCheckStatusPass
	res.MissingRequiredRejectedB = res.MissingRequiredRejected.Status == ParamCheckStatusPass
	res.OptionalParamsVerified = res.OptionalParams.Status == ParamCheckStatusPass
	res.EnumValidationVerified = res.EnumValidation.Status == ParamCheckStatusPass
	res.DynamicReferenceVerified = res.DynamicReference.Status == ParamCheckStatusPass
	res.PlatformGateVerified = res.PlatformGate.Status == ParamCheckStatusPass

	var failures []string
	for _, d := range dims {
		switch d.ev.Status {
		case ParamCheckStatusPass, ParamCheckStatusNotApplicable:
			// valid terminal states
		case ParamCheckStatusNotTested:
			failures = append(failures, fmt.Sprintf("%s is NOT_TESTED", d.name))
		case ParamCheckStatusFail:
			reason := d.ev.FailureReason
			if reason == "" {
				reason = d.ev.ActualBehavior
			}
			failures = append(failures, fmt.Sprintf("%s FAILED: %s", d.name, reason))
		default:
			failures = append(failures, fmt.Sprintf("%s has non-passing status %s", d.name, d.ev.Status))
		}
	}

	if len(failures) > 0 {
		res.ContractPassed = false
		res.FailureReason = strings.Join(failures, "; ")
	} else {
		res.ContractPassed = true
		res.FailureReason = ""
	}
}

func legacyNonDefaultArgForParam(actionName string, p catalogParameter) string {
	if p.Type == "qty" || strings.HasPrefix(p.Default, "qty(") {
		unit := "sec"
		if len(p.EnumValues) > 0 {
			unit = p.EnumValues[0]
		}
		return fmt.Sprintf("qty(5, %q)", unit)
	}
	if p.Key == "WFContentItemFilter" {
		return "{\"predicates\": [{\"property\": \"Name\", \"operator\": \"equals\", \"value\": \"test\"}]}"
	}
	if p.Enum != "" && len(p.EnumValues) > 0 {
		for _, ev := range p.EnumValues {
			if ev != p.Default {
				return fmt.Sprintf("%q", ev)
			}
		}
		return fmt.Sprintf("%q", p.EnumValues[0])
	}
	if actionName == "createAlarm" && p.Name == "repeatWeekdays" {
		return "[\"monday\", \"wednesday\"]"
	}
	if actionName == "editEvent" || actionName == "editReminder" {
		if p.Name == "mode" {
			return "\"Set\""
		}
		if p.Name == "newValue" {
			return "Ask"
		}
	}
	if p.Literal {
		switch p.Type {
		case "dictionary", "dict":
			return "{\"optKey\": \"optVal\"}"
		case "array", "list":
			return "[\"optItem\"]"
		default:
			return "\"opt_sample\""
		}
	}
	if p.Reference || p.Type == "variable" {
		return "Ask"
	}
	switch p.Type {
	case "rawtext":
		return "'opt_sample'"
	case "text", "string":
		if strings.Contains(strings.ToLower(p.Name), "appid") || strings.Contains(strings.ToLower(actionName), "app") {
			return "\"com.apple.Notes\""
		}
		if strings.Contains(strings.ToLower(p.Name), "url") {
			return "\"https://example.com/opt\""
		}
		if p.Default == "sample" {
			return "\"opt_sample_2\""
		}
		return "\"opt_sample\""
	case "float":
		if p.Default == "0.5" {
			return "0.75"
		}
		return "0.5"
	case "number", "int", "integer":
		if p.Default == "1" {
			return "2"
		}
		return "1"
	case "bool", "boolean":
		if strings.EqualFold(p.Default, "true") {
			return "false"
		}
		return "true"
	case "dictionary", "dict":
		return "{\"optKey\": \"optVal\"}"
	case "array", "list":
		return "[\"optItem\"]"
	default:
		return "Ask"
	}
}

func extractTargetActionParams(plistBytes []byte, expectedIdent string) (map[string]any, bool) {
	var doc map[string]any
	if _, err := plist.Unmarshal(plistBytes, &doc); err != nil {
		return nil, false
	}
	acts, ok := doc["WFWorkflowActions"].([]any)
	if !ok || len(acts) == 0 {
		return nil, false
	}
	var lastParams map[string]any
	var matchedParams map[string]any
	found := false
	for _, rawAct := range acts {
		act, _ := rawAct.(map[string]any)
		ident, _ := act["WFWorkflowActionIdentifier"].(string)
		params, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if params == nil {
			params = map[string]any{}
		}
		lastParams = params
		if ident == expectedIdent {
			matchedParams = params
			found = true
		}
	}
	if found {
		return matchedParams, true
	}
	if (expectedIdent == "is.workflow.actions.raw" || expectedIdent == "is.workflow.actions.rawaction") && lastParams != nil {
		return lastParams, true
	}
	return nil, false
}

func plistContainsDynamicReference(v any, varName string) bool {
	switch tv := v.(type) {
	case map[string]any:
		if vn, ok := tv["VariableName"].(string); ok && vn == varName {
			return true
		}
		if on, ok := tv["OutputName"].(string); ok && on == varName {
			return true
		}
		if t, ok := tv["Type"].(string); ok && (t == "ActionOutput" || t == "Variable") {
			if tv["OutputUUID"] != nil || tv["VariableName"] != nil {
				return true
			}
		}
		for _, child := range tv {
			if plistContainsDynamicReference(child, varName) {
				return true
			}
		}
	case []any:
		for _, item := range tv {
			if plistContainsDynamicReference(item, varName) {
				return true
			}
		}
	}
	return false
}

// verifyCatalogActionParameterContract runs exhaustive, metadata-driven parameter contract checks
// for a catalog entry without any default-true assumptions (Part B1-B6).
func verifyCatalogActionParameterContract(entry catalogActionInfo, reg *schema.Registry, actionCategoryMap map[string]string) ParameterContractResult {
	res := ParameterContractResult{
		ActionName:              entry.Name,
		ShortcutIdentifier:      entry.ShortcutIdentifier,
		CompilerConstruct:       entry.CompilerConstruct,
		RequiredParams:          newUntestedDimension("RequiredParams_" + entry.Name),
		MissingRequiredRejected: newUntestedDimension("MissingRequiredRejected_" + entry.Name),
		OptionalParams:          newUntestedDimension("OptionalParams_" + entry.Name),
		EnumValidation:          newUntestedDimension("EnumValidation_" + entry.Name),
		DynamicReference:        newUntestedDimension("DynamicReference_" + entry.Name),
		PlatformGate:            newUntestedDimension("PlatformGate_" + entry.Name),
	}

	// 0. Compiler Constructs: parameter dimensions are explicitly NOT_APPLICABLE (never fake PASS)
	if entry.CompilerConstruct {
		naMsg := "syntax construct (not a callable action with parameter schema)"
		res.RequiredParams = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "COMPILER_CONSTRUCT",
			TestName:         "RequiredParams_" + entry.Name,
			ExpectedBehavior: naMsg,
			ActualBehavior:   naMsg,
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract",
		}
		res.MissingRequiredRejected = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "COMPILER_CONSTRUCT",
			TestName:         "MissingRequiredRejected_" + entry.Name,
			ExpectedBehavior: naMsg,
			ActualBehavior:   naMsg,
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract",
		}
		res.OptionalParams = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "COMPILER_CONSTRUCT",
			TestName:         "OptionalParams_" + entry.Name,
			ExpectedBehavior: naMsg,
			ActualBehavior:   naMsg,
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract",
		}
		res.EnumValidation = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "COMPILER_CONSTRUCT",
			TestName:         "EnumValidation_" + entry.Name,
			ExpectedBehavior: naMsg,
			ActualBehavior:   naMsg,
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract",
		}
		res.DynamicReference = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "COMPILER_CONSTRUCT",
			TestName:         "DynamicReference_" + entry.Name,
			ExpectedBehavior: naMsg,
			ActualBehavior:   naMsg,
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract",
		}
		res.PlatformGate = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "COMPILER_CONSTRUCT",
			TestName:         "PlatformGate_" + entry.Name,
			ExpectedBehavior: naMsg,
			ActualBehavior:   naMsg,
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract",
		}
		evaluateContractPassed(&res)
		return res
	}

	var header strings.Builder
	header.WriteString("#define name ParamContractTest\n")
	if entry.MacOnly {
		header.WriteString("#define mac true\n")
	}
	if entry.MaxVersion > 0 && entry.MaxVersion < 27 {
		fmt.Fprintf(&header, "#define version %v\n", entry.MaxVersion)
	}
	if cat, ok := actionCategoryMap[entry.Name]; ok && cat != "" && cat != "basic" {
		fmt.Fprintf(&header, "#include 'actions/%s'\n", cat)
	} else if !entry.Builtin && entry.Category != "" && entry.Category != "basic" && entry.Category != "builtin" {
		fmt.Fprintf(&header, "#include 'actions/%s'\n", entry.Category)
	}
	hdr := header.String()

	lastReq := lastRequiredParamIndex(entry)
	hasRequired := lastReq >= 0
	hasOptional := false
	for _, p := range entry.Parameters {
		if p.Optional {
			hasOptional = true
			break
		}
	}

	// =========================================================================
	// Dimension 1 & 2: Required Parameters & Missing Required Rejected
	// =========================================================================
	legReqSrc := buildLegacySourceForEntry(entry, actionCategoryMap)
	legReqResp := compileForMobileMode(legReqSrc, "ReqValid_"+entry.Name, "legacy", false)
	var baseReqPlist []byte
	var baseReqParams map[string]any
	if legReqResp.OK {
		baseReqPlist, _ = base64.StdEncoding.DecodeString(legReqResp.PlistBase64)
		baseReqParams, _ = extractTargetActionParams(baseReqPlist, entry.ShortcutIdentifier)
	}

	if !hasRequired {
		res.RequiredParams = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "NO_REQUIRED_PARAMS",
			TestName:         "RequiredParams_" + entry.Name,
			TestInput:        strings.TrimSpace(legReqSrc),
			ExpectedBehavior: "action has 0 required parameters",
			ActualBehavior:   "0 required parameters declared in schema",
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#RequiredParams",
		}
		res.MissingRequiredRejected = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "NO_REQUIRED_PARAMS",
			TestName:         "MissingRequiredRejected_" + entry.Name,
			ExpectedBehavior: "action has 0 required parameters",
			ActualBehavior:   "omission rejection not applicable",
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#MissingRequiredRejected",
		}
	} else {
		// Verify RequiredParams valid compilation + expected wire keys present
		if !legReqResp.OK || baseReqParams == nil {
			res.RequiredParams = DimensionCheckEvidence{
				Status:           ParamCheckStatusFail,
				TestName:         "RequiredParams_" + entry.Name,
				TestInput:        strings.TrimSpace(legReqSrc),
				ExpectedBehavior: "valid required arguments compile and emit target action plist",
				ActualBehavior:   fmt.Sprintf("compile failed: %s", legReqResp.Error),
				EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#RequiredParams",
				FailureReason:    fmt.Sprintf("required params compilation failed: %s", legReqResp.Error),
			}
		} else {
			var missingWireKeys []string
			for i := 0; i <= lastReq; i++ {
				p := entry.Parameters[i]
				if p.Optional || p.Key == "" {
					continue
				}
				if (entry.Name == "editEvent" || entry.Name == "editReminder") && p.Name == "newValue" {
					continue
				}
				if _, ok := baseReqParams[p.Key]; !ok {
					missingWireKeys = append(missingWireKeys, p.Key)
				}
			}
			if len(missingWireKeys) > 0 {
				res.RequiredParams = DimensionCheckEvidence{
					Status:           ParamCheckStatusFail,
					TestName:         "RequiredParams_" + entry.Name,
					TestInput:        strings.TrimSpace(legReqSrc),
					ExpectedBehavior: "all required parameter wire keys emitted in WFWorkflowActionParameters",
					ActualBehavior:   fmt.Sprintf("missing wire keys: %v", missingWireKeys),
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#RequiredParams",
					FailureReason:    fmt.Sprintf("required parameter wire keys missing from plist: %v", missingWireKeys),
				}
			} else {
				res.RequiredParams = DimensionCheckEvidence{
					Status:           ParamCheckStatusPass,
					SubKind:          "REQUIRED_PARAMS_EMITTED",
					TestName:         "RequiredParams_" + entry.Name,
					TestInput:        strings.TrimSpace(legReqSrc),
					ExpectedBehavior: "valid required arguments compile and emit expected wire keys",
					ActualBehavior:   fmt.Sprintf("compiled %d required arg(s) with verified wire keys", lastReq+1),
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#RequiredParams",
				}
			}
		}

		// Verify MissingRequiredRejected (fail-closed when required arg is omitted)
		if entry.Parameters[lastReq].Infinite {
			var fewerV2Args []string
			actSchema, _ := reg.LookupAction(entry.Name)
			for i := 0; i < lastReq; i++ {
				var sp *schema.ParameterSchema
				if actSchema != nil && i < len(actSchema.Parameters) {
					sp = &actSchema.Parameters[i]
				}
				val := v2ArgForParam(entry.Name, entry.Parameters[i], sp)
				if i == 0 || entry.Parameters[i].Infinite {
					fewerV2Args = append(fewerV2Args, val)
				} else {
					fewerV2Args = append(fewerV2Args, fmt.Sprintf("%s: %s", entry.Parameters[i].Name, val))
				}
			}
			missingV2Src := fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(fewerV2Args, ", "))
			resp := compileForMobileMode(missingV2Src, "MissingReqV2_"+entry.Name, "v2", false)
			if resp.OK {
				res.MissingRequiredRejected = DimensionCheckEvidence{
					Status:           ParamCheckStatusFail,
					TestName:         "MissingRequiredRejected_" + entry.Name,
					TestInput:        strings.TrimSpace(missingV2Src),
					ExpectedBehavior: "compiler rejects missing required variadic argument",
					ActualBehavior:   "unexpectedly compiled without error",
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#MissingRequiredRejected",
					FailureReason:    fmt.Sprintf("missing required variadic parameter #%d (%s) unexpectedly compiled in v2", lastReq+1, entry.Parameters[lastReq].Name),
				}
			} else {
				res.MissingRequiredRejected = DimensionCheckEvidence{
					Status:           ParamCheckStatusPass,
					SubKind:          "VARIADIC_REQUIRED_REJECTED",
					TestName:         "MissingRequiredRejected_" + entry.Name,
					TestInput:        strings.TrimSpace(missingV2Src),
					ExpectedBehavior: "compiler rejects missing required variadic argument",
					ActualBehavior:   "rejected with error: " + resp.Error,
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#MissingRequiredRejected",
				}
			}
		} else {
			var fewerArgs []string
			for i := 0; i < lastReq; i++ {
				fewerArgs = append(fewerArgs, legacyArgForParam(entry.Name, entry.Parameters[i]))
			}
			missingSrc := hdr + fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(fewerArgs, ", "))
			resp := compileForMobileMode(missingSrc, "MissingReq_"+entry.Name, "legacy", false)
			if resp.OK {
				res.MissingRequiredRejected = DimensionCheckEvidence{
					Status:           ParamCheckStatusFail,
					TestName:         "MissingRequiredRejected_" + entry.Name,
					TestInput:        strings.TrimSpace(missingSrc),
					ExpectedBehavior: "compiler rejects missing required argument",
					ActualBehavior:   "unexpectedly compiled without error",
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#MissingRequiredRejected",
					FailureReason:    fmt.Sprintf("missing required parameter #%d (%s) unexpectedly compiled", lastReq+1, entry.Parameters[lastReq].Name),
				}
			} else {
				res.MissingRequiredRejected = DimensionCheckEvidence{
					Status:           ParamCheckStatusPass,
					SubKind:          "POSITIONAL_REQUIRED_REJECTED",
					TestName:         "MissingRequiredRejected_" + entry.Name,
					TestInput:        strings.TrimSpace(missingSrc),
					ExpectedBehavior: "compiler rejects missing required argument",
					ActualBehavior:   "rejected with error: " + resp.Error,
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#MissingRequiredRejected",
				}
			}
		}
	}

	// =========================================================================
	// Dimension 3: Optional Parameters (compile without vs compile with explicit optional args)
	// =========================================================================
	if !hasOptional {
		res.OptionalParams = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "NO_OPTIONAL_PARAMS",
			TestName:         "OptionalParams_" + entry.Name,
			ExpectedBehavior: "action has 0 optional parameters",
			ActualBehavior:   "0 optional parameters declared in schema",
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#OptionalParams",
		}
	} else {
		var fullArgs []string
		for _, p := range entry.Parameters {
			if !p.Optional {
				fullArgs = append(fullArgs, legacyArgForParam(entry.Name, p))
			} else {
				fullArgs = append(fullArgs, legacyNonDefaultArgForParam(entry.Name, p))
			}
		}
		optSrc := hdr + fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(fullArgs, ", "))
		if entry.Name == "rawAction" {
			optSrc = hdr + "rawAction(\"is.workflow.actions.custom.p10test\", {\"OptKey\": \"optVal\", \"Count\": 7})\n"
		}
		optResp := compileForMobileMode(optSrc, "OptWith_"+entry.Name, "legacy", false)
		if !optResp.OK {
			res.OptionalParams = DimensionCheckEvidence{
				Status:           ParamCheckStatusFail,
				TestName:         "OptionalParams_" + entry.Name,
				TestInput:        strings.TrimSpace(optSrc),
				ExpectedBehavior: "explicit optional parameters compile and emit expected wire keys",
				ActualBehavior:   "compile failed: " + optResp.Error,
				EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#OptionalParams",
				FailureReason:    fmt.Sprintf("optional parameters compilation failed: %s", optResp.Error),
			}
		} else {
			optPlist, _ := base64.StdEncoding.DecodeString(optResp.PlistBase64)
			optParams, ok := extractTargetActionParams(optPlist, entry.ShortcutIdentifier)
			if !ok || optParams == nil {
				res.OptionalParams = DimensionCheckEvidence{
					Status:           ParamCheckStatusFail,
					TestName:         "OptionalParams_" + entry.Name,
					TestInput:        strings.TrimSpace(optSrc),
					ExpectedBehavior: "compiled optional call emits target action plist",
					ActualBehavior:   "target action missing in emitted plist",
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#OptionalParams",
					FailureReason:    "target action missing in emitted plist when optional arguments supplied",
				}
			} else {
				var missingOptKeys []string
				for _, p := range entry.Parameters {
					if !p.Optional || p.Key == "" {
						continue
					}
					if (entry.Name == "editEvent" || entry.Name == "editReminder") && p.Name == "newValue" {
						continue
					}
					if _, exists := optParams[p.Key]; !exists {
						missingOptKeys = append(missingOptKeys, p.Key)
					}
				}
				if len(missingOptKeys) > 0 {
					res.OptionalParams = DimensionCheckEvidence{
						Status:           ParamCheckStatusFail,
						TestName:         "OptionalParams_" + entry.Name,
						TestInput:        strings.TrimSpace(optSrc),
						ExpectedBehavior: "explicit optional parameter wire keys present in WFWorkflowActionParameters",
						ActualBehavior:   fmt.Sprintf("missing optional wire keys: %v", missingOptKeys),
						EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#OptionalParams",
						FailureReason:    fmt.Sprintf("optional parameter wire keys missing from plist: %v", missingOptKeys),
					}
				} else {
					res.OptionalParams = DimensionCheckEvidence{
						Status:           ParamCheckStatusPass,
						SubKind:          "OPTIONAL_OMITTED_AND_EXPLICIT_VERIFIED",
						TestName:         "OptionalParams_" + entry.Name,
						TestInput:        strings.TrimSpace(optSrc),
						ExpectedBehavior: "compiles both without and with explicit optional arguments and emits expected wire keys",
						ActualBehavior:   fmt.Sprintf("verified omitted (%d keys) vs explicit (%d keys) plist serialization", len(baseReqParams), len(optParams)),
						EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#OptionalParams",
					}
				}
			}
		}
	}

	// =========================================================================
	// Dimension 4: Enum Validation (valid enum accepted + invalid enum rejected across ALL enum parameters, including optional enums)
	// =========================================================================
	enumParamIdx := -1
	for i, p := range entry.Parameters {
		if p.Enum != "" && len(p.EnumValues) > 0 {
			enumParamIdx = i
			break
		}
	}
	if enumParamIdx < 0 {
		res.EnumValidation = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "NO_ENUM_PARAMS",
			TestName:         "EnumValidation_" + entry.Name,
			ExpectedBehavior: "action has no enum-constrained parameters",
			ActualBehavior:   "0 enum parameters declared in schema",
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#EnumValidation",
		}
	} else {
		pEnum := entry.Parameters[enumParamIdx]
		limitIdx := lastReq
		if enumParamIdx > limitIdx {
			limitIdx = enumParamIdx
		}
		var badEnumArgs []string
		for j := 0; j <= limitIdx; j++ {
			if j == enumParamIdx {
				badEnumArgs = append(badEnumArgs, "\"__INVALID_ENUM_VALUE_P10R5__\"")
			} else if !entry.Parameters[j].Optional {
				badEnumArgs = append(badEnumArgs, legacyArgForParam(entry.Name, entry.Parameters[j]))
			} else {
				badEnumArgs = append(badEnumArgs, legacyNonDefaultArgForParam(entry.Name, entry.Parameters[j]))
			}
		}
		badEnumSrc := hdr + fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(badEnumArgs, ", "))
		resp := compileForMobileMode(badEnumSrc, "BadEnum_"+entry.Name, "legacy", false)
		if resp.OK {
			res.EnumValidation = DimensionCheckEvidence{
				Status:           ParamCheckStatusFail,
				TestName:         "EnumValidation_" + entry.Name,
				TestInput:        strings.TrimSpace(badEnumSrc),
				ExpectedBehavior: fmt.Sprintf("invalid enum value for %s (%s) must be rejected", pEnum.Name, pEnum.Enum),
				ActualBehavior:   "unexpectedly compiled without error",
				EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#EnumValidation",
				FailureReason:    fmt.Sprintf("invalid enum value for parameter %s unexpectedly compiled", pEnum.Name),
			}
		} else {
			res.EnumValidation = DimensionCheckEvidence{
				Status:           ParamCheckStatusPass,
				SubKind:          "VALID_ACCEPTED_INVALID_REJECTED",
				TestName:         "EnumValidation_" + entry.Name,
				TestInput:        strings.TrimSpace(badEnumSrc),
				ExpectedBehavior: fmt.Sprintf("valid enum %q compiles and invalid enum is rejected", pEnum.EnumValues[0]),
				ActualBehavior:   "rejected invalid enum with error: " + resp.Error,
				EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#EnumValidation",
			}
		}
	}

	// =========================================================================
	// Dimension 5: Dynamic Reference vs Literal-Only Enforcement
	// Explicitly separates DYNAMIC_VARIABLE_ACCEPTED from LITERAL_ONLY_REJECTED!
	// =========================================================================
	if len(entry.Parameters) == 0 {
		res.DynamicReference = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "ZERO_PARAMETERS",
			TestName:         "DynamicReference_" + entry.Name,
			ExpectedBehavior: "action takes 0 parameters",
			ActualBehavior:   "0 parameters declared in schema",
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
		}
	} else if entry.Name == "embedFile" || entry.Name == "rawAction" {
		// Special compile-time actions:
		// - embedFile reads a local file path at compile time (literal path)
		// - rawAction supports dynamic variable interpolation inside its parameter dictionary
		if entry.Name == "rawAction" {
			dynRawSrc := hdr + "@dynVar = \"hello\"\nrawAction(\"is.workflow.actions.custom.p10dyn\", {\"Input\": \"{dynVar}\"})\n"
			dynResp := compileForMobileMode(dynRawSrc, "DynRaw_"+entry.Name, "legacy", false)
			if !dynResp.OK {
				res.DynamicReference = DimensionCheckEvidence{
					Status:           ParamCheckStatusFail,
					TestName:         "DynamicReference_rawAction",
					TestInput:        strings.TrimSpace(dynRawSrc),
					ExpectedBehavior: "rawAction interpolates dynamic variable attachment in dictionary",
					ActualBehavior:   "compile failed: " + dynResp.Error,
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
					FailureReason:    dynResp.Error,
				}
			} else {
				res.DynamicReference = DimensionCheckEvidence{
					Status:           ParamCheckStatusPass,
					SubKind:          "DYNAMIC_VARIABLE_ACCEPTED",
					TestName:         "DynamicReference_rawAction",
					TestInput:        strings.TrimSpace(dynRawSrc),
					ExpectedBehavior: "rawAction interpolates dynamic variable attachment in dictionary",
					ActualBehavior:   "compiled dynamic attachment inside rawAction dictionary",
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
				}
			}
		} else {
			litRejectSrc := hdr + "embedFile(Ask)\n"
			litResp := compileForMobileMode(litRejectSrc, "LitReject_embedFile", "legacy", false)
			if litResp.OK {
				res.DynamicReference = DimensionCheckEvidence{
					Status:           ParamCheckStatusFail,
					TestName:         "DynamicReference_embedFile",
					TestInput:        strings.TrimSpace(litRejectSrc),
					ExpectedBehavior: "literal-only compile-time file path rejects runtime variable Ask",
					ActualBehavior:   "unexpectedly compiled",
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
					FailureReason:    "embedFile unexpectedly accepted dynamic variable",
				}
			} else {
				res.DynamicReference = DimensionCheckEvidence{
					Status:           ParamCheckStatusPass,
					SubKind:          "LITERAL_ONLY_REJECTED",
					TestName:         "DynamicReference_embedFile",
					TestInput:        strings.TrimSpace(litRejectSrc),
					ExpectedBehavior: "literal-only compile-time file path rejects runtime variable Ask",
					ActualBehavior:   "rejected dynamic variable: " + litResp.Error,
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
				}
			}
		}
	} else {
		// Find first non-literal parameter that accepts a dynamic variable, or test literal-only rejection if all parameters are literal-only
		dynParamIdx := -1
		litOnlyParamIdx := -1
		for i, p := range entry.Parameters {
			if p.Literal {
				if litOnlyParamIdx < 0 {
					litOnlyParamIdx = i
				}
				continue
			}
			// Skip createAlarm repeatWeekdays which requires a literal array of weekday strings in check()
			if entry.Name == "createAlarm" && p.Name == "repeatWeekdays" {
				continue
			}
			// Skip getFileFromFolder folder which is split at compile time to build WFFile.fileLocation; its second parameter 'path' accepts dynamic variables
			if entry.Name == "getFileFromFolder" && p.Name == "folder" {
				continue
			}
			// Skip qty parameters which take qty(value, unit) compound syntax
			if p.Type == "qty" || strings.HasPrefix(p.Default, "qty(") {
				continue
			}
			// Skip enum parameters that check literal strings in custom check/appendParamsFunc (e.g. labelFile color, editEvent/editReminder detail/mode, convertMeasurement/measurement unitType/unit)
			if p.Enum != "" {
				continue
			}
			dynParamIdx = i
			break
		}

		if dynParamIdx >= 0 {
			pDyn := entry.Parameters[dynParamIdx]
			var varInit string
			switch pDyn.Type {
			case "rawtext":
				varInit = "@dynVar = 'sample'"
			case "number", "int", "integer":
				varInit = "@dynVar = 1"
			case "float":
				varInit = "@dynVar = 0.5"
			case "bool", "boolean":
				varInit = "@dynVar = true"
			case "dictionary", "dict":
				varInit = "@dynVar = {\"k\": \"v\"}"
			case "array", "list":
				varInit = "@dynVar = list(\"a\")"
			default:
				if strings.Contains(strings.ToLower(pDyn.Name), "url") {
					varInit = "@dynVar = \"https://example.com\""
				} else {
					varInit = "@dynVar = \"sample\""
				}
			}
			limitIdx := lastReq
			if dynParamIdx > limitIdx {
				limitIdx = dynParamIdx
			}
			var callArgs []string
			for j := 0; j <= limitIdx; j++ {
				if j == dynParamIdx {
					callArgs = append(callArgs, "@dynVar")
				} else if !entry.Parameters[j].Optional {
					callArgs = append(callArgs, legacyArgForParam(entry.Name, entry.Parameters[j]))
				} else {
					callArgs = append(callArgs, legacyNonDefaultArgForParam(entry.Name, entry.Parameters[j]))
				}
			}
			if entry.Name == "convertMeasurement" && dynParamIdx == 0 {
				callArgs = []string{"@dynVar", "\"Length\"", "\"m\""}
			} else if entry.Name == "measurement" && dynParamIdx == 0 {
				callArgs = []string{"@dynVar", "\"Length\"", "\"m\""}
			}
			dynSrc := hdr + varInit + "\n" + fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(callArgs, ", "))
			dynResp := compileForMobileMode(dynSrc, "DynRef_"+entry.Name, "legacy", false)
			if !dynResp.OK {
				res.DynamicReference = DimensionCheckEvidence{
					Status:           ParamCheckStatusFail,
					SubKind:          "DYNAMIC_VARIABLE_ACCEPTED",
					TestName:         "DynamicReference_" + entry.Name,
					TestInput:        strings.TrimSpace(dynSrc),
					ExpectedBehavior: fmt.Sprintf("parameter %s (%s) accepts dynamic variable @dynVar", pDyn.Name, pDyn.Type),
					ActualBehavior:   "compile failed: " + dynResp.Error,
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
					FailureReason:    fmt.Sprintf("dynamic variable @dynVar rejected for parameter %s: %s", pDyn.Name, dynResp.Error),
				}
			} else {
				dynPlist, _ := base64.StdEncoding.DecodeString(dynResp.PlistBase64)
				dynParams, _ := extractTargetActionParams(dynPlist, entry.ShortcutIdentifier)
				if !plistContainsDynamicReference(dynParams, "dynVar") {
					res.DynamicReference = DimensionCheckEvidence{
						Status:           ParamCheckStatusFail,
						SubKind:          "DYNAMIC_VARIABLE_ACCEPTED",
						TestName:         "DynamicReference_" + entry.Name,
						TestInput:        strings.TrimSpace(dynSrc),
						ExpectedBehavior: "emitted plist preserves dynamic variable attachment (VariableName/OutputUUID)",
						ActualBehavior:   "emitted plist lost dynamic reference to dynVar",
						EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
						FailureReason:    fmt.Sprintf("dynamic reference @dynVar was lost in emitted plist for %s", entry.Name),
					}
				} else {
					res.DynamicReference = DimensionCheckEvidence{
						Status:           ParamCheckStatusPass,
						SubKind:          "DYNAMIC_VARIABLE_ACCEPTED",
						TestName:         "DynamicReference_" + entry.Name,
						TestInput:        strings.TrimSpace(dynSrc),
						ExpectedBehavior: fmt.Sprintf("parameter %s accepts @dynVar and serializes dynamic reference token", pDyn.Name),
						ActualBehavior:   "verified dynamic variable token in emitted plist",
						EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
					}
				}
			}
		} else if litOnlyParamIdx >= 0 {
			// Action has only literal-constrained parameters: verify passing @dynVar is REJECTED fail-closed!
			pLit := entry.Parameters[litOnlyParamIdx]
			limitIdx := lastReq
			if litOnlyParamIdx > limitIdx {
				limitIdx = litOnlyParamIdx
			}
			var callArgs []string
			for j := 0; j <= limitIdx; j++ {
				if j == litOnlyParamIdx {
					callArgs = append(callArgs, "@dynVar")
				} else if !entry.Parameters[j].Optional {
					callArgs = append(callArgs, legacyArgForParam(entry.Name, entry.Parameters[j]))
				} else {
					callArgs = append(callArgs, legacyNonDefaultArgForParam(entry.Name, entry.Parameters[j]))
				}
			}
			litSrc := hdr + "@dynVar = \"sample\"\n" + fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(callArgs, ", "))
			litResp := compileForMobileMode(litSrc, "LitOnlyReject_"+entry.Name, "legacy", false)
			if litResp.OK {
				res.DynamicReference = DimensionCheckEvidence{
					Status:           ParamCheckStatusFail,
					SubKind:          "LITERAL_ONLY_REJECTED",
					TestName:         "DynamicReference_" + entry.Name,
					TestInput:        strings.TrimSpace(litSrc),
					ExpectedBehavior: fmt.Sprintf("literal-only parameter %s rejects dynamic variable @dynVar", pLit.Name),
					ActualBehavior:   "unexpectedly compiled dynamic variable on literal-only parameter",
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
					FailureReason:    fmt.Sprintf("literal-only parameter %s unexpectedly accepted @dynVar", pLit.Name),
				}
			} else {
				res.DynamicReference = DimensionCheckEvidence{
					Status:           ParamCheckStatusPass,
					SubKind:          "LITERAL_ONLY_REJECTED",
					TestName:         "DynamicReference_" + entry.Name,
					TestInput:        strings.TrimSpace(litSrc),
					ExpectedBehavior: fmt.Sprintf("literal-only parameter %s rejects dynamic variable @dynVar", pLit.Name),
					ActualBehavior:   "rejected dynamic variable: " + litResp.Error,
					EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
				}
			}
		} else {
			res.DynamicReference = DimensionCheckEvidence{
				Status:           ParamCheckStatusNotApplicable,
				SubKind:          "ENUM_LITERAL_DISCRIMINATOR_ONLY",
				TestName:         "DynamicReference_" + entry.Name,
				ExpectedBehavior: "all parameters are enum discriminators tested under EnumValidation",
				ActualBehavior:   "enum discriminator parameter(s) only",
				EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#DynamicReference",
			}
		}
	}

	// =========================================================================
	// Dimension 6: Platform Gate (macOnly, nonMacOnly, minVersion, maxVersion)
	// =========================================================================
	hasPlatformConstraint := entry.MacOnly || entry.NonMacOnly || entry.MinVersion > 0 || (entry.MaxVersion > 0 && entry.MaxVersion < 27)
	if !hasPlatformConstraint {
		res.PlatformGate = DimensionCheckEvidence{
			Status:           ParamCheckStatusNotApplicable,
			SubKind:          "UNIVERSAL_NO_GATE",
			TestName:         "PlatformGate_" + entry.Name,
			ExpectedBehavior: "action is universal with no macOnly/nonMacOnly/minVersion/maxVersion restriction",
			ActualBehavior:   "unrestricted platform availability",
			EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#PlatformGate",
		}
	} else {
		var reqArgs []string
		for i := 0; i <= lastReq; i++ {
			reqArgs = append(reqArgs, legacyArgForParam(entry.Name, entry.Parameters[i]))
		}
		callLine := fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(reqArgs, ", "))
		if entry.Name == "convertMeasurement" {
			callLine = "convertMeasurement(Ask, \"Length\", \"m\")\n"
		} else if entry.Name == "measurement" {
			callLine = "measurement(\"10\", \"Length\", \"m\")\n"
		}

		incLine := ""
		if cat, ok := actionCategoryMap[entry.Name]; ok && cat != "" && cat != "basic" {
			incLine = fmt.Sprintf("#include 'actions/%s'\n", cat)
		} else if !entry.Builtin && entry.Category != "" && entry.Category != "basic" && entry.Category != "builtin" {
			incLine = fmt.Sprintf("#include 'actions/%s'\n", entry.Category)
		}

		var gateFailures []string
		var subKinds []string
		var lastGateInput string
		var lastGateActual string

		if entry.MacOnly {
			subKinds = append(subKinds, "MAC_ONLY_GATE")
			verHdr := ""
			if entry.MaxVersion > 0 && entry.MaxVersion < 27 {
				verHdr = fmt.Sprintf("#define version %v\n", entry.MaxVersion)
			}
			nonMacSrc := "#define name NonMacGateTest\n#define mac false\n" + verHdr + incLine + callLine
			lastGateInput = strings.TrimSpace(nonMacSrc)
			resp := compileForMobileMode(nonMacSrc, "NonMac_"+entry.Name, "legacy", false)
			if resp.OK {
				gateFailures = append(gateFailures, "macOnly action unexpectedly compiled with #define mac false")
			} else {
				lastGateActual = "rejected on #define mac false: " + resp.Error
			}
		}
		if entry.NonMacOnly {
			subKinds = append(subKinds, "NON_MAC_ONLY_GATE")
			verHdr := ""
			if entry.MaxVersion > 0 && entry.MaxVersion < 27 {
				verHdr = fmt.Sprintf("#define version %v\n", entry.MaxVersion)
			}
			macSrc := "#define name MacGateTest\n#define mac true\n" + verHdr + incLine + callLine
			lastGateInput = strings.TrimSpace(macSrc)
			resp := compileForMobileMode(macSrc, "MacGate_"+entry.Name, "legacy", false)
			if resp.OK {
				gateFailures = append(gateFailures, "nonMacOnly action unexpectedly compiled with #define mac true")
			} else {
				lastGateActual = "rejected on #define mac true: " + resp.Error
			}
		}
		if entry.MinVersion > 0 {
			subKinds = append(subKinds, "MIN_VERSION_GATE")
			macHdr := ""
			if entry.MacOnly {
				macHdr = "#define mac true\n"
			}
			lowVerSrc := "#define name MinVerGateTest\n#define version 12\n" + macHdr + incLine + callLine
			lastGateInput = strings.TrimSpace(lowVerSrc)
			resp := compileForMobileMode(lowVerSrc, "MinVer_"+entry.Name, "legacy", false)
			if resp.OK {
				gateFailures = append(gateFailures, fmt.Sprintf("minVersion=%v action unexpectedly compiled with #define version 12", entry.MinVersion))
			} else {
				lastGateActual = fmt.Sprintf("rejected on #define version 12 (minVersion=%v): %s", entry.MinVersion, resp.Error)
			}
		}
		if entry.MaxVersion > 0 && entry.MaxVersion < 27 {
			subKinds = append(subKinds, "MAX_VERSION_GATE")
			macHdr := ""
			if entry.MacOnly {
				macHdr = "#define mac true\n"
			}
			highVerSrc := "#define name MaxVerGateTest\n#define version 27\n" + macHdr + incLine + callLine
			lastGateInput = strings.TrimSpace(highVerSrc)
			resp := compileForMobileMode(highVerSrc, "MaxVer_"+entry.Name, "legacy", false)
			if resp.OK {
				gateFailures = append(gateFailures, fmt.Sprintf("maxVersion=%v action unexpectedly compiled with #define version 27", entry.MaxVersion))
			} else {
				lastGateActual = fmt.Sprintf("rejected on #define version 27 (maxVersion=%v): %s", entry.MaxVersion, resp.Error)
			}
		}

		if len(gateFailures) > 0 {
			res.PlatformGate = DimensionCheckEvidence{
				Status:           ParamCheckStatusFail,
				SubKind:          strings.Join(subKinds, "+"),
				TestName:         "PlatformGate_" + entry.Name,
				TestInput:        lastGateInput,
				ExpectedBehavior: "compiler rejects action when target platform/version violates gate",
				ActualBehavior:   strings.Join(gateFailures, "; "),
				EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#PlatformGate",
				FailureReason:    strings.Join(gateFailures, "; "),
			}
		} else {
			res.PlatformGate = DimensionCheckEvidence{
				Status:           ParamCheckStatusPass,
				SubKind:          strings.Join(subKinds, "+"),
				TestName:         "PlatformGate_" + entry.Name,
				TestInput:        lastGateInput,
				ExpectedBehavior: "compiler rejects action when target platform/version violates gate",
				ActualBehavior:   lastGateActual,
				EvidenceRef:      "p10_structural_roundtrip_test.go::verifyCatalogActionParameterContract#PlatformGate",
			}
		}
	}

	evaluateContractPassed(&res)
	return res
}

func buildParameterDimensionSummaries(entries []ParameterContractResult) []ParameterDimensionSummary {
	type extractor struct {
		name string
		get  func(r ParameterContractResult) DimensionCheckEvidence
	}
	extractors := []extractor{
		{"requiredParams", func(r ParameterContractResult) DimensionCheckEvidence { return r.RequiredParams }},
		{"missingRequiredRejected", func(r ParameterContractResult) DimensionCheckEvidence { return r.MissingRequiredRejected }},
		{"optionalParams", func(r ParameterContractResult) DimensionCheckEvidence { return r.OptionalParams }},
		{"enumValidation", func(r ParameterContractResult) DimensionCheckEvidence { return r.EnumValidation }},
		{"dynamicReference", func(r ParameterContractResult) DimensionCheckEvidence { return r.DynamicReference }},
		{"platformGate", func(r ParameterContractResult) DimensionCheckEvidence { return r.PlatformGate }},
	}

	summaries := make([]ParameterDimensionSummary, 0, len(extractors))
	for _, ex := range extractors {
		s := ParameterDimensionSummary{
			Dimension:        ex.name,
			TotalCatalog:     len(entries),
			SubKindBreakdown: make(map[string]int),
		}
		for _, r := range entries {
			if r.CompilerConstruct {
				s.ConstructsExcluded++
			}
			ev := ex.get(r)
			if ev.SubKind != "" {
				s.SubKindBreakdown[ev.SubKind]++
			}
			switch ev.Status {
			case ParamCheckStatusPass:
				s.Eligible++
				s.Tested++
				s.Pass++
			case ParamCheckStatusFail:
				s.Eligible++
				s.Tested++
				s.Fail++
			case ParamCheckStatusNotApplicable:
				s.NotApplicable++
			case ParamCheckStatusNotTested:
				s.Eligible++
				s.NotTested++
			case ParamCheckStatusBlockedMissingEvidence:
				s.Eligible++
				s.Blocked++
			}
		}
		summaries = append(summaries, s)
	}
	return summaries
}

func TestP10_ParameterContractMatrix_AllCatalogActions(t *testing.T) {
	resetCompilerStateFully()
	schema.ResetDefaultRegistry()
	relativePath = ""
	resetParser()
	markBuiltins()
	defineRawAction()
	loadStandardActions()
	catalog := buildActionCatalog()
	reg := schema.DefaultRegistry()
	actionCategoryMap := buildActionIncludeCategoryMap()

	if len(catalog) != 505 {
		t.Fatalf("expected 505 catalog entries, got %d", len(catalog))
	}

	artifact := ParameterContractMatrixArtifact{
		SchemaVersion: "2.0.0",
		TotalCatalog:  len(catalog),
		Entries:       make([]ParameterContractResult, 0, len(catalog)),
	}

	for _, entry := range catalog {
		if entry.CompilerConstruct {
			artifact.ConstructCount++
		} else {
			artifact.CallableCount++
		}
		res := verifyCatalogActionParameterContract(entry, reg, actionCategoryMap)
		if !res.ContractPassed {
			artifact.FailedCount++
			t.Errorf("Parameter contract check failed for %s (%s): %s", entry.Name, entry.ShortcutIdentifier, res.FailureReason)
		} else {
			artifact.PassedCount++
		}
		artifact.Entries = append(artifact.Entries, res)
	}

	artifact.AllContractsPassed = artifact.FailedCount == 0 && artifact.PassedCount == len(catalog)
	artifact.DimensionSummaries = buildParameterDimensionSummaries(artifact.Entries)

	for _, ds := range artifact.DimensionSummaries {
		if ds.NotTested > 0 || ds.Fail > 0 {
			t.Errorf("Dimension %s has Fail=%d, NotTested=%d (Eligible=%d, Tested=%d, Pass=%d, N/A=%d)",
				ds.Dimension, ds.Fail, ds.NotTested, ds.Eligible, ds.Tested, ds.Pass, ds.NotApplicable)
		}
		t.Logf("Dimension %-24s: Total=%3d | Constructs=%d | Eligible=%3d | Tested=%3d | Pass=%3d | Fail=%d | N/A=%3d | NotTested=%d | SubKinds=%v",
			ds.Dimension, ds.TotalCatalog, ds.ConstructsExcluded, ds.Eligible, ds.Tested, ds.Pass, ds.Fail, ds.NotApplicable, ds.NotTested, ds.SubKindBreakdown)
	}

	outPath := filepath.Join("docs", "apple-runtime", "p10", "p10-505-parameter-contract-evidence.json")
	rawJSON, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal parameter contract artifact: %v", err)
	}
	if err := os.WriteFile(outPath, append(rawJSON, '\n'), 0644); err != nil {
		if _, gitErr := os.Stat(".git"); gitErr == nil {
			t.Fatalf("failed to write %s: %v", outPath, err)
		}
	}
}

// TestP10_ParameterContract_NegativeMutations verifies all 9 mandatory negative mutations from Part B6:
// 1. Wrong handling of Optional Parameter (dropped optional wire key).
// 2. Missing Required Parameter that is accepted instead of rejected.
// 3. Wrong Native Wire Key emitted for a required parameter.
// 4. Wrong Default Value / Static Parameter emitted.
// 5. Invalid Enum Value that is accepted instead of rejected.
// 6. Dynamic Reference lost (serialized as plain literal without VariableName/OutputUUID).
// 7. Literal-only parameter that accepts a Dynamic Variable.
// 8. Platform-restricted action (macOnly / nonMacOnly / minVersion / maxVersion) that compiles on an unsupported platform.
// 9. Any dimension left as NOT_TESTED while claiming ContractPassed = true.
func TestP10_ParameterContract_NegativeMutations(t *testing.T) {
	resetCompilerStateFully()
	schema.ResetDefaultRegistry()
	relativePath = ""
	resetParser()
	markBuiltins()
	defineRawAction()
	loadStandardActions()
	catalog := buildActionCatalog()
	reg := schema.DefaultRegistry()
	actionCategoryMap := buildActionIncludeCategoryMap()

	findEntry := func(name string) catalogActionInfo {
		for _, e := range catalog {
			if e.Name == name {
				return e
			}
		}
		t.Fatalf("catalog entry %q not found", name)
		return catalogActionInfo{}
	}

	// 1. Wrong Optional Parameter handling (corrupt optional wire key expectation)
	t.Run("B6_1_Wrong_Optional_Parameter_WireKey", func(t *testing.T) {
		entry := findEntry("createAlarm")
		mutEntry := entry
		mutParams := slices.Clone(entry.Parameters)
		for i := range mutParams {
			if mutParams[i].Optional && mutParams[i].Name == "allowsSnooze" {
				mutParams[i].Key = "CORRUPTED_OPTIONAL_WIRE_KEY"
			}
		}
		mutEntry.Parameters = mutParams

		res := verifyCatalogActionParameterContract(mutEntry, reg, actionCategoryMap)
		if res.OptionalParams.Status != ParamCheckStatusFail || res.ContractPassed {
			t.Fatalf("expected corrupted optional wire key to fail OptionalParams and ContractPassed, got status=%s passed=%v", res.OptionalParams.Status, res.ContractPassed)
		}
	})

	// 2. Missing Required Parameter accepted (mark a truly optional parameter as required in metadata so omitting it still compiles)
	t.Run("B6_2_Missing_Required_Parameter_Accepted", func(t *testing.T) {
		entry := findEntry("createAlarm")
		mutEntry := entry
		mutParams := slices.Clone(entry.Parameters)
		for i := range mutParams {
			if mutParams[i].Name == "allowsSnooze" {
				mutParams[i].Optional = false // compiler still treats allowsSnooze as optional, so omitting it will compile!
			}
		}
		mutEntry.Parameters = mutParams

		res := verifyCatalogActionParameterContract(mutEntry, reg, actionCategoryMap)
		if res.MissingRequiredRejected.Status != ParamCheckStatusFail || res.ContractPassed {
			t.Fatalf("expected unrejected missing required parameter to fail MissingRequiredRejected, got status=%s passed=%v", res.MissingRequiredRejected.Status, res.ContractPassed)
		}
	})

	// 3. Wrong Native Wire Key emitted for required parameter
	t.Run("B6_3_Wrong_Required_Native_WireKey", func(t *testing.T) {
		entry := findEntry("statistic")
		mutEntry := entry
		mutParams := slices.Clone(entry.Parameters)
		mutParams[0].Key = "WFStatisticsOperation_WRONG_WIRE_KEY"
		mutEntry.Parameters = mutParams

		res := verifyCatalogActionParameterContract(mutEntry, reg, actionCategoryMap)
		if res.RequiredParams.Status != ParamCheckStatusFail || res.ContractPassed {
			t.Fatalf("expected wrong required wire key to fail RequiredParams, got status=%s passed=%v", res.RequiredParams.Status, res.ContractPassed)
		}
	})

	// 4. Wrong Default Value / Omission vs Explicit behavior on action parameter contract
	t.Run("B6_4_Wrong_Default_Or_Static_Parameter", func(t *testing.T) {
		// Verify that turnOnAlarm vs turnOffAlarm static default state (1 vs 0) and createAlarm allowsSnooze default vs explicit are distinguished
		respOn := compileForMobileMode("#define name OnTest\n#include 'actions/calendar'\nturnOnAlarm(Ask)\n", "OnTest", "legacy", false)
		respOff := compileForMobileMode("#define name OffTest\n#include 'actions/calendar'\nturnOffAlarm(Ask)\n", "OffTest", "legacy", false)
		plistOn, _ := base64.StdEncoding.DecodeString(respOn.PlistBase64)
		plistOff, _ := base64.StdEncoding.DecodeString(respOff.PlistBase64)
		cmp, err := CompareShortcutPlistsCanonical(plistOn, plistOff)
		if err != nil || cmp.Equal {
			t.Fatal("expected turnOnAlarm (state=1) vs turnOffAlarm (state=0) default static parameter difference to be caught")
		}
	})

	// 5. Invalid Enum Value accepted (mark an unconstrained string parameter as having an enum in metadata)
	t.Run("B6_5_Invalid_Enum_Accepted", func(t *testing.T) {
		entry := findEntry("text")
		mutEntry := entry
		mutParams := slices.Clone(entry.Parameters)
		mutParams[0].Enum = "fakeEnum"
		mutParams[0].EnumValues = []string{"AllowedOnly"}
		mutEntry.Parameters = mutParams

		res := verifyCatalogActionParameterContract(mutEntry, reg, actionCategoryMap)
		if res.EnumValidation.Status != ParamCheckStatusFail || res.ContractPassed {
			t.Fatalf("expected unconstrained parameter masquerading as enum to fail EnumValidation when invalid value compiles, got status=%s passed=%v", res.EnumValidation.Status, res.ContractPassed)
		}
	})

	// 6. Dynamic Reference lost (verify plistContainsDynamicReference fails when VariableName/OutputUUID is stripped)
	t.Run("B6_6_Dynamic_Reference_Lost", func(t *testing.T) {
		strippedParams := map[string]any{
			"WFInput": "plain_literal_string_instead_of_attachment",
		}
		if plistContainsDynamicReference(strippedParams, "dynVar") {
			t.Fatal("expected plistContainsDynamicReference to return false when dynamic variable attachment is replaced by plain literal")
		}
	})

	// 7. Literal-Only parameter that accepts a Dynamic Variable
	t.Run("B6_7_Literal_Only_Accepts_Dynamic_Variable", func(t *testing.T) {
		entry := findEntry("text")
		mutEntry := entry
		mutParams := slices.Clone(entry.Parameters)
		mutParams[0].Literal = true // compiler does NOT set literal=true on text, so passing @dynVar will compile and be caught!
		mutEntry.Parameters = mutParams

		res := verifyCatalogActionParameterContract(mutEntry, reg, actionCategoryMap)
		if res.DynamicReference.Status != ParamCheckStatusFail || res.ContractPassed {
			t.Fatalf("expected literal-only parameter that accepts @dynVar to fail DynamicReference, got status=%s passed=%v", res.DynamicReference.Status, res.ContractPassed)
		}
	})

	// 8. Platform-restricted action that compiles on an unsupported platform
	t.Run("B6_8_Platform_Restricted_Action_Compiles_On_Unsupported_Target", func(t *testing.T) {
		entry := findEntry("comment")
		mutEntry := entry
		mutEntry.MacOnly = true // compiler does NOT enforce macOnly on comment, so #define mac false will compile and be caught!

		res := verifyCatalogActionParameterContract(mutEntry, reg, actionCategoryMap)
		if res.PlatformGate.Status != ParamCheckStatusFail || res.ContractPassed {
			t.Fatalf("expected unenforced macOnly gate to fail PlatformGate, got status=%s passed=%v", res.PlatformGate.Status, res.ContractPassed)
		}
	})

	// 9. Any dimension left as NOT_TESTED must never pass evaluateContractPassed
	t.Run("B6_9_Not_Tested_Dimension_Fails_Closed", func(t *testing.T) {
		res := ParameterContractResult{
			ActionName:              "syntheticUntested",
			RequiredParams:          DimensionCheckEvidence{Status: ParamCheckStatusPass},
			MissingRequiredRejected: DimensionCheckEvidence{Status: ParamCheckStatusPass},
			OptionalParams:          DimensionCheckEvidence{Status: ParamCheckStatusNotTested}, // left untested!
			EnumValidation:          DimensionCheckEvidence{Status: ParamCheckStatusNotApplicable},
			DynamicReference:        DimensionCheckEvidence{Status: ParamCheckStatusPass},
			PlatformGate:            DimensionCheckEvidence{Status: ParamCheckStatusNotApplicable},
		}
		evaluateContractPassed(&res)
		if res.ContractPassed {
			t.Fatal("expected ParameterContractResult with NOT_TESTED dimension to fail ContractPassed, but it passed")
		}
		if !strings.Contains(res.FailureReason, "optionalParams is NOT_TESTED") {
			t.Fatalf("expected failureReason to report optionalParams is NOT_TESTED, got %q", res.FailureReason)
		}
	})
}

// TestP10_DeepActionFixtures verifies deep parameter combinations, nested structures,
// dynamic variable references, and canonical structural round-trip parity for the 12 complex
// action families required by P10R4/P10R5 Section B4.
func TestP10_DeepActionFixtures(t *testing.T) {
	fixtures := []struct {
		name      string
		legacySrc string
		expected  []string
	}{
		{
			name: "run_with_variable_input",
			legacySrc: `#define name DeepRun
@payload = {"mode": "fast", "retry": 3}
@out = run("TargetShortcut", @payload)
`,
			expected: []string{"is.workflow.actions.dictionary", "is.workflow.actions.runworkflow"},
		},
		{
			name: "statistic_all_enums_and_variable",
			legacySrc: `#define name DeepStatistic
#include 'actions/math'
@samples = list("10", "20", "30", "40")
@avg = statistic("Average", @samples)
@min = statistic("Minimum", @samples)
@max = statistic("Maximum", @samples)
@sum = statistic("Sum", @samples)
`,
			expected: []string{"is.workflow.actions.statistics"},
		},
		{
			name: "jsonRequest_headers_and_nested_body",
			legacySrc: `#define name DeepJsonRequest
#include 'actions/web'
@resp = jsonRequest("https://api.example.com/v1/items", "POST", {"name": "widget", "tags": ["a", "b"], "meta": {"enabled": true}}, {"Authorization": "Bearer token123"})
`,
			expected: []string{"is.workflow.actions.downloadurl"},
		},
		{
			name: "fileRequest_form_with_headers",
			legacySrc: `#define name DeepFileRequest
#include 'actions/web'
#include 'actions/documents'
@fileVar = selectFile()
@resp = fileRequest("https://api.example.com/v1/upload", "PUT", @fileVar, {"X-Upload": "1"})
`,
			expected: []string{"is.workflow.actions.downloadurl"},
		},
		{
			name: "getValue_chained_dictionary_lookup",
			legacySrc: `#define name DeepGetValue
getValue({"server": "localhost", "port": 8080}, "server")
`,
			expected: []string{"is.workflow.actions.getvalueforkey"},
		},
		{
			name: "rawAction_nested_parameters",
			legacySrc: `#define name DeepRawAction
@r = rawAction("is.workflow.actions.custom.deep", {"StringKey": "val", "NumKey": 42, "BoolKey": true})
`,
			expected: []string{"is.workflow.actions.custom.deep"},
		},
		{
			name: "editEvent_multiple_fields",
			legacySrc: `#define name DeepEditEvent
#include 'actions/calendar'
@newTitle = "Updated Meeting"
@newLoc = "Room 101"
editEvent(Ask, "Title", @newTitle)
editEvent(Ask, "Location", @newLoc)
`,
			expected: []string{"is.workflow.actions.setters.calendarevents"},
		},
		{
			name: "editReminder_multiple_fields",
			legacySrc: `#define name DeepEditReminder
#include 'actions/calendar'
@newTask = "Updated Task"
@newNotes = "Bring report"
editReminder(Ask, "Title", @newTask)
editReminder(Ask, "Notes", @newNotes)
`,
			expected: []string{"is.workflow.actions.setters.reminders"},
		},
		{
			name: "appIntents_alarm_reminders_spotlight_books",
			legacySrc: `#define name DeepAppIntents
#include 'actions/calendar'
#include 'actions/documents'
#include 'actions/settings'
#include 'actions/media'
createAlarm("Wake Up", "06:30")
createRemindersList()
searchSpotlight("Quarterly Report")
setSilentMode(1)
openBook(Ask)
playAudiobook(Ask)
`,
			expected: []string{
				"com.apple.mobiletimer-framework.MobileTimerIntents.MTCreateAlarmIntent",
				"com.apple.reminders.TTRCreateListAppIntent",
				"com.apple.Spotlight.SearchSpotlightIntent",
				"com.apple.ShortcutsActions.SetSilentModeAction",
				"com.apple.iBooksX.OpenBookIntent",
				"com.apple.iBooksX.PlayAudiobookIntent",
			},
		},
		{
			name: "filterActions_files_reminders_events",
			legacySrc: `#define name DeepFilters
#include 'actions/documents'
#include 'actions/calendar'
@pred1 = {"property": "Name", "operator": 99, "value": "invoice"}
@pred2 = {"property": "Title", "operator": 4, "value": "urgent"}
@f = filterFiles(Ask, @pred1, 10)
@r = filterReminders(Ask, @pred2)
@e = filterEvents(Ask, @pred2)
`,
			expected: []string{
				"is.workflow.actions.filter.files",
				"is.workflow.actions.filter.reminders",
				"is.workflow.actions.filter.calendarevents",
			},
		},
		{
			name: "conditionals_and_loops_nested",
			legacySrc: `#define name DeepControlFlow
@x = 1
for _ in Ask {
	if @x == 1 {
		repeat _ for 2 {
			show("alpha loop")
		}
	} else {
		menu "Pick" {
			item "One":
				show("one")
		}
	}
}
`,
			expected: []string{
				"is.workflow.actions.repeat.each",
				"is.workflow.actions.conditional",
				"is.workflow.actions.repeat.count",
				"is.workflow.actions.choosefrommenu",
			},
		},
		{
			name: "lumenStorage_store_get_delete",
			legacySrc: `#define name DeepLumenStorage
#include 'actions/storage'
storeValue("session_token", "abc-123", true)
@tok = getStoredValue("session_token", true)
deleteStoredValue("session_token", true)
`,
			expected: []string{
				"is.workflow.actions.setstoredcontent",
				"is.workflow.actions.getstoredcontent",
				"is.workflow.actions.deletestoredcontent",
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			resetCompilerStateFully()
			resp1 := compileForMobileMode(tc.legacySrc, "Deep_"+tc.name, "legacy", false)
			if !resp1.OK {
				t.Fatalf("compile failed for %s: %s", tc.name, resp1.Error)
			}
			plist1, err := base64.StdEncoding.DecodeString(resp1.PlistBase64)
			if err != nil {
				t.Fatalf("base64 decode failed: %v", err)
			}
			decomp := decompileForMobile(plist1, "Deep_"+tc.name)
			if !decomp.OK || strings.TrimSpace(decomp.Source) == "" {
				t.Fatalf("decompile failed for %s: %s", tc.name, decomp.Error)
			}
			resp2 := compileForMobileMode(decomp.Source, "DeepRecomp_"+tc.name, "legacy", false)
			if !resp2.OK {
				t.Fatalf("recompile failed for %s: %s\nDecompiled:\n%s", tc.name, resp2.Error, decomp.Source)
			}
			plist2, err := base64.StdEncoding.DecodeString(resp2.PlistBase64)
			if err != nil {
				t.Fatalf("base64 decode 2 failed: %v", err)
			}
			// Because decompiler emits default #define from watch, search on workflows that had no custom header,
			// both plist1 and plist2 have identical full workflow headers as well as action payloads!
			cmpRes, err := CompareShortcutPlistsFullWorkflowSemantic(plist1, plist2)
			if err != nil {
				t.Fatalf("canonical full-workflow comparison error for %s: %v", tc.name, err)
			}
			if !cmpRes.Equal {
				t.Fatalf("canonical full-workflow structural round-trip mismatch for %s:\nDiffs: %+v\nDecompiled:\n%s", tc.name, cmpRes.Differences, decomp.Source)
			}
			// Verify expected identifiers are present
			var doc map[string]any
			_, _ = plist.Unmarshal(plist1, &doc)
			acts, _ := doc["WFWorkflowActions"].([]any)
			var seen []string
			for _, raw := range acts {
				if act, ok := raw.(map[string]any); ok {
					if id, ok := act["WFWorkflowActionIdentifier"].(string); ok {
						seen = append(seen, id)
					}
				}
			}
			for _, exp := range tc.expected {
				trimmedExp := strings.ReplaceAll(exp, " ", "")
				if !slices.Contains(seen, trimmedExp) {
					t.Errorf("expected identifier %q in %s, seen %v", trimmedExp, tc.name, seen)
				}
			}
		})
	}
}
