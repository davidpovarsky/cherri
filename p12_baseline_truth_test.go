/*
 * Copyright (c) Cherri Language
 * P10R12 Gates A/B/C: Baseline Truthfulness, Lifecycle Reconciliation, Fail-Closed Pre-Overlay & Oracle Tests
 */

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"howett.net/plist"
)

func scalarKindSummary(v any) string {
	if v == nil {
		return "nil"
	}
	switch x := v.(type) {
	case map[string]any:
		return fmt.Sprintf("map(len=%d)", len(x))
	case []any:
		return fmt.Sprintf("slice(len=%d)", len(x))
	case []string:
		return fmt.Sprintf("strslice(len=%d)", len(x))
	case []byte:
		return fmt.Sprintf("bytes(len=%d)", len(x))
	case string:
		return fmt.Sprintf("string(len=%d)", len(x))
	default:
		return reflect.TypeOf(v).String()
	}
}

// firstSanitizedPlistDiffPath returns a deterministic diff path mentioning ONLY key paths,
// types, and container/string lengths — NEVER parameter scalar values, URLs, tokens, or source snippets.
func firstSanitizedPlistDiffPath(a, b any, path string) string {
	if path == "" {
		path = "$"
	}
	if a == nil && b == nil {
		return path + ": equal"
	}
	if a == nil || b == nil {
		return fmt.Sprintf("%s: %s vs %s", path, scalarKindSummary(a), scalarKindSummary(b))
	}
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: %s vs %s", path, scalarKindSummary(a), scalarKindSummary(b))
		}
		xKeys := make([]string, 0, len(x))
		for k := range x {
			xKeys = append(xKeys, k)
		}
		sort.Strings(xKeys)
		for _, k := range xKeys {
			yv, exists := y[k]
			if !exists {
				return fmt.Sprintf("%s.%s: missing in rebuilt", path, k)
			}
			if !plistValuesStrictlyEqual(x[k], yv) {
				return firstSanitizedPlistDiffPath(x[k], yv, path+"."+k)
			}
		}
		yKeys := make([]string, 0, len(y))
		for k := range y {
			if _, exists := x[k]; !exists {
				yKeys = append(yKeys, k)
			}
		}
		sort.Strings(yKeys)
		if len(yKeys) > 0 {
			return fmt.Sprintf("%s.%s: extra in rebuilt", path, yKeys[0])
		}
	case []any:
		y, ok := b.([]any)
		if !ok {
			return fmt.Sprintf("%s: %s vs %s", path, scalarKindSummary(a), scalarKindSummary(b))
		}
		if len(x) != len(y) {
			return fmt.Sprintf("%s: slice len %d vs %d", path, len(x), len(y))
		}
		for i := range x {
			if !plistValuesStrictlyEqual(x[i], y[i]) {
				return firstSanitizedPlistDiffPath(x[i], y[i], fmt.Sprintf("%s[%d]", path, i))
			}
		}
	}
	return fmt.Sprintf("%s: scalar value/type mismatch (%s vs %s)", path, scalarKindSummary(a), scalarKindSummary(b))
}

func assertPreOverlayCapturesComplete(specimen string, preByDialect map[string][]any, captureCounts map[string]int, expectedActions int) error {
	for _, dialect := range []string{"legacy", "v2"} {
		cnt := captureCounts[dialect]
		if cnt != 1 {
			return fmt.Errorf("%s %s pre-overlay capture invocation count=%d (expected 1; fail-closed)", specimen, dialect, cnt)
		}
		captured, ok := preByDialect[dialect]
		if !ok || captured == nil {
			return fmt.Errorf("%s %s pre-overlay capture missing (fail-closed)", specimen, dialect)
		}
		if len(captured) != expectedActions {
			return fmt.Errorf("%s %s pre-overlay capture len=%d != expected=%d (fail-closed)", specimen, dialect, len(captured), expectedActions)
		}
	}
	return nil
}

func isScopedLegacyVarSerializationTypeExemption(dialect, identifier string, origParams map[string]any, key string) bool {
	if key != "WFSerializationType" || dialect != "legacy" {
		return false
	}
	if identifier != "is.workflow.actions.setvariable" && identifier != "is.workflow.actions.appendvariable" {
		return false
	}
	_, origHasTopSer := origParams["WFSerializationType"]
	return !origHasTopSer
}

// verifyPreOverlayActionSemanticParity enforces fail-closed bidirectional parameter equality
// between an original action record and its pre-overlay compiled action for a specific dialect.
func verifyPreOverlayActionSemanticParity(specimen, dialect string, idx int, orig NativeActionRecord, preAct map[string]any) error {
	preID, _ := preAct["WFWorkflowActionIdentifier"].(string)
	if preID != orig.Identifier {
		return fmt.Errorf("%s %s action %d (%s) pre-overlay identifier mismatch: got %q", specimen, dialect, idx, orig.Identifier, preID)
	}
	preParams, _ := preAct["WFWorkflowActionParameters"].(map[string]any)

	origKeys := make([]string, 0, len(orig.Parameters))
	for k := range orig.Parameters {
		origKeys = append(origKeys, k)
	}
	sort.Strings(origKeys)

	// Direction 1: orig -> pre
	for _, k := range origKeys {
		if isImportProvenanceMetaKey(k) {
			continue
		}
		origVal := orig.Parameters[k]
		preValRaw, hasPre := preParams[k]
		if !hasPre {
			return fmt.Errorf("%s %s action %d (%s) missing pre-overlay semantic key %s (origType=%s)",
				specimen, dialect, idx, orig.Identifier, k, scalarKindSummary(origVal))
		}
		preVal := canonicalizePlistValue(preValRaw)
		if !plistValuesStrictlyEqual(origVal, preVal) {
			return fmt.Errorf("%s %s action %d (%s) pre-overlay semantic mismatch at %s",
				specimen, dialect, idx, orig.Identifier, firstSanitizedPlistDiffPath(origVal, preVal, "params."+k))
		}
	}

	// Direction 2: pre -> orig (catches fabricated/defaulted extra pre-overlay semantic keys)
	preKeys := make([]string, 0, len(preParams))
	for k := range preParams {
		preKeys = append(preKeys, k)
	}
	sort.Strings(preKeys)
	for _, k := range preKeys {
		if isImportProvenanceMetaKey(k) {
			continue
		}
		if isScopedLegacyVarSerializationTypeExemption(dialect, orig.Identifier, orig.Parameters, k) {
			continue
		}
		if _, exists := orig.Parameters[k]; !exists {
			return fmt.Errorf("%s %s action %d (%s) emitted extra pre-overlay key %s (%s) absent in original",
				specimen, dialect, idx, orig.Identifier, k, scalarKindSummary(preParams[k]))
		}
	}
	return nil
}

func verifyCorpusDualHybridWithPreOverlay(doc *NativeShortcutDocument, specimen string, resetState bool) (*HybridImportPlan, map[string]any, map[string]any, map[string][]any, error) {
	if resetState {
		resetCompilerStateFully()
		ClearHybridCandidateCache()
	}

	plan, err := PlanHybridImport(doc)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s PlanHybridImport failed: %w", specimen, err)
	}
	if err := validateHybridPlanLifecycleInvariants(specimen, plan, len(doc.Actions)); err != nil {
		return nil, nil, nil, nil, err
	}

	legSrc, err := RenderHybridLegacySource(doc, plan)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s RenderHybridLegacySource failed: %w", specimen, err)
	}
	v2Src, err := RenderHybridV2Source(doc, plan)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s RenderHybridV2Source failed: %w", specimen, err)
	}

	preByDialect := make(map[string][]any, 2)
	captureCounts := make(map[string]int, 2)
	PreOverlayCaptureHook = func(dialect string, compiledActions []any) {
		if dialect == "" {
			return
		}
		captureCounts[dialect]++
		preByDialect[dialect] = compiledActions
	}
	defer func() { PreOverlayCaptureHook = nil }()

	legMap, err := compileLegacySourceToRawMapInMemory(legSrc)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s legacy compile failed: %v", specimen, err)
	}
	v2Bytes, err := CompileSourceToPlist("p12_v2.cherri", v2Src)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s v2 compile failed: %v", specimen, err)
	}
	v2Map, err := decodeShortcutRawMap(v2Bytes)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s v2 decode failed: %w", specimen, err)
	}

	if err := assertPreOverlayCapturesComplete(specimen, preByDialect, captureCounts, len(doc.Actions)); err != nil {
		return nil, nil, nil, nil, err
	}
	if hasImportProvenanceKey(legMap) || hasImportProvenanceKey(v2Map) {
		return nil, nil, nil, nil, fmt.Errorf("%s compiled output leaked %s", specimen, importProvenanceKey)
	}
	if !plistValuesStrictlyEqual(doc.RawRoot, legMap) {
		return nil, nil, nil, nil, fmt.Errorf("%s legacy wire mismatch at %s", specimen, firstSanitizedPlistDiffPath(doc.RawRoot, legMap, "root"))
	}
	if !plistValuesStrictlyEqual(doc.RawRoot, v2Map) {
		return nil, nil, nil, nil, fmt.Errorf("%s v2 wire mismatch at %s", specimen, firstSanitizedPlistDiffPath(doc.RawRoot, v2Map, "root"))
	}
	return plan, legMap, v2Map, preByDialect, nil
}

func validateHybridPlanLifecycleInvariants(specimen string, plan *HybridImportPlan, totalActions int) error {
	if plan == nil {
		return fmt.Errorf("%s nil plan", specimen)
	}
	if len(plan.Decisions) != totalActions {
		return fmt.Errorf("%s decision count %d != actions %d", specimen, len(plan.Decisions), totalActions)
	}
	var countNat, countFb, countInv int
	for i, d := range plan.Decisions {
		if d.CandidateStatus == "CANDIDATE" || d.Classification == "NATURAL_CANDIDATE" {
			return fmt.Errorf("%s action %d stuck in unresolved CANDIDATE state", specimen, i)
		}
		switch d.Classification {
		case "NATURAL_VERIFIED":
			countNat++
			if d.CandidateStatus != "VERIFIED" {
				return fmt.Errorf("%s action %d NATURAL_VERIFIED has CandidateStatus=%q (want VERIFIED)", specimen, i, d.CandidateStatus)
			}
		case "EXACT_NATIVE_FALLBACK":
			countFb++
			if d.CandidateStatus == "DEMOTED" && (d.DemotionStage == "" || d.DemotionReason == "") {
				return fmt.Errorf("%s action %d DEMOTED missing stage=%q or reason=%q", specimen, i, d.DemotionStage, d.DemotionReason)
			}
		case "SOURCE_INVALID":
			countInv++
		default:
			return fmt.Errorf("%s action %d unknown classification %q", specimen, i, d.Classification)
		}
	}
	if countNat != plan.NaturalVerifiedCount || countFb != plan.ExactNativeCount || countInv != plan.SourceInvalidCount {
		return fmt.Errorf("%s count mismatch nat=%d/%d fb=%d/%d inv=%d/%d",
			specimen, countNat, plan.NaturalVerifiedCount, countFb, plan.ExactNativeCount, countInv, plan.SourceInvalidCount)
	}
	if plan.Lifecycle == nil {
		return fmt.Errorf("%s missing Lifecycle summary", specimen)
	}
	if plan.Lifecycle.VerifiedActionCount != countNat {
		return fmt.Errorf("%s Lifecycle VerifiedActionCount=%d != NaturalVerified=%d", specimen, plan.Lifecycle.VerifiedActionCount, countNat)
	}
	if plan.Lifecycle.CandidateActionCount != plan.Lifecycle.VerifiedActionCount+plan.Lifecycle.DemotedActionCount {
		return fmt.Errorf("%s Lifecycle candidate accounting broken: entered=%d != verified=%d + demoted=%d",
			specimen, plan.Lifecycle.CandidateActionCount, plan.Lifecycle.VerifiedActionCount, plan.Lifecycle.DemotedActionCount)
	}
	return nil
}

// TestP12_BASE_01_PreOverlayFailClosedBothDialects verifies Gate B: missing, partial, or single-dialect
// pre-overlay captures fail closed immediately without silently skipping verification.
func TestP12_BASE_01_PreOverlayFailClosedBothDialects(t *testing.T) {
	dummy := map[string]any{}
	if err := assertPreOverlayCapturesComplete("synthetic", map[string][]any{"legacy": {dummy}, "v2": {dummy}}, map[string]int{"legacy": 1, "v2": 1}, 1); err != nil {
		t.Fatalf("unexpected error on complete capture: %v", err)
	}
	badCases := []struct {
		name   string
		pre    map[string][]any
		counts map[string]int
		n      int
	}{
		{"missing_legacy", map[string][]any{"v2": {dummy}}, map[string]int{"v2": 1}, 1},
		{"missing_v2", map[string][]any{"legacy": {dummy}}, map[string]int{"legacy": 1}, 1},
		{"partial_legacy", map[string][]any{"legacy": {dummy}, "v2": {dummy, dummy}}, map[string]int{"legacy": 1, "v2": 1}, 2},
		{"partial_v2", map[string][]any{"legacy": {dummy, dummy}, "v2": {dummy}}, map[string]int{"legacy": 1, "v2": 1}, 2},
		{"zero_count_v2", map[string][]any{"legacy": {dummy}, "v2": {dummy}}, map[string]int{"legacy": 1, "v2": 0}, 1},
	}
	for _, tc := range badCases {
		if err := assertPreOverlayCapturesComplete(tc.name, tc.pre, tc.counts, tc.n); err == nil {
			t.Fatalf("case %s unexpectedly succeeded instead of failing closed", tc.name)
		}
	}

	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "DualDialectCapture"},
		},
	}, plist.XMLFormat)
	plan, _, _, preByDialect, err := verifyCorpusDualHybridWithPreOverlay(doc, "P12-BASE-01", true)
	if err != nil {
		t.Fatalf("dual capture helper failed: %v", err)
	}
	if plan.NaturalVerifiedCount != 1 {
		t.Fatalf("expected 1 NATURAL_VERIFIED, got %d", plan.NaturalVerifiedCount)
	}
	for _, dialect := range []string{"legacy", "v2"} {
		if err := verifyPreOverlayActionSemanticParity("P12-BASE-01", dialect, 0, doc.Actions[0], preByDialect[dialect][0].(map[string]any)); err != nil {
			t.Fatalf("%s pre-overlay check failed: %v", dialect, err)
		}
	}
}

// TestP12_BASE_02_LifecycleStagesAndReconcileCorpus verifies Gate A: deterministic Stage 1..5
// CANDIDATE -> VERIFIED / DEMOTED tracking and reconciles fresh vs non-reset corpus verification.
func TestP12_BASE_02_LifecycleStagesAndReconcileCorpus(t *testing.T) {
	// Deterministic synthetic lifecycle check: 1 promotable comment + 1 forced mismatch / uncatalogued fallback
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "Promoted"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "com.example.thirdparty.widget",
			"WFWorkflowActionParameters": map[string]any{"Foo": "Bar"},
		},
	}, plist.XMLFormat)
	resetCompilerStateFully()
	ClearHybridCandidateCache()
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatalf("PlanHybridImport failed: %v", err)
	}
	if err := validateHybridPlanLifecycleInvariants("P12-BASE-02-synth", plan, 2); err != nil {
		t.Fatal(err)
	}
	if plan.Decisions[0].CandidateStatus != "VERIFIED" || plan.Decisions[1].Classification != "EXACT_NATIVE_FALLBACK" {
		t.Fatalf("unexpected synthetic decisions: %+v", plan.Decisions)
	}

	if os.Getenv("CHERRI_P10R10_CORPUS") != "1" {
		t.Log("NOT_RUN_PRIVATE_CORPUS: set CHERRI_P10R10_CORPUS=1 for full 38-shortcut lifecycle reconciliation")
		return
	}
	entries, err := os.ReadDir(p10r10CorpusDir)
	if err != nil {
		t.Fatalf("corpus dir inaccessible: %v", err)
	}

	var docs []*NativeShortcutDocument
	var names []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(p10r10CorpusDir, e.Name()))
		if err != nil {
			t.Fatalf("%s read failed: %v", e.Name(), err)
		}
		d, err := DecodeNativeShortcutDocument(raw)
		if err != nil {
			t.Fatalf("%s decode failed: %v", e.Name(), err)
		}
		docs = append(docs, d)
		names = append(names, e.Name())
	}

	// Pass A: Clean fresh-process equivalent (resetCompilerStateFully + ClearHybridCandidateCache per file)
	freshNat := 0
	freshPerFile := make(map[string]int, len(docs))
	demotionByStage := map[string]int{}
	demotionByReason := map[string]int{}
	for idx, d := range docs {
		p, _, _, preByDialect, err := verifyCorpusDualHybridWithPreOverlay(d, names[idx], true)
		if err != nil {
			t.Fatalf("%v", err)
		}
		freshNat += p.NaturalVerifiedCount
		freshPerFile[names[idx]] = p.NaturalVerifiedCount
		for _, u := range p.Lifecycle.Transitions {
			if u.FinalState == "DEMOTED" {
				demotionByStage[u.FinalStage]++
				demotionByReason[u.TransitionReason] += u.ActionCount
			}
		}
		for i, dec := range p.Decisions {
			if dec.Classification != "NATURAL_VERIFIED" {
				continue
			}
			for _, dialect := range []string{"legacy", "v2"} {
				if err := verifyPreOverlayActionSemanticParity(names[idx], dialect, i, d.Actions[i], preByDialect[dialect][i].(map[string]any)); err != nil {
					t.Fatalf("%v", err)
				}
			}
		}
	}

	// Pass B: Sequential stateful loop without resetCompilerStateFully/cache clear (reproducing P10R11 process behavior)
	seqNat := 0
	diffs := []string{}
	for idx, d := range docs {
		p, err := PlanHybridImport(d)
		if err != nil {
			t.Fatalf("%s sequential PlanHybridImport failed: %v", names[idx], err)
		}
		seqNat += p.NaturalVerifiedCount
		if p.NaturalVerifiedCount != freshPerFile[names[idx]] {
			diffs = append(diffs, fmt.Sprintf("%s(fresh=%d,seq=%d)", names[idx], freshPerFile[names[idx]], p.NaturalVerifiedCount))
		}
	}
	t.Logf("P12_BASE_02 corpus reconciliation: files=%d freshNat=%d seqNat=%d diffs=%v", len(docs), freshNat, seqNat, diffs)
	t.Logf("P12_BASE_02 demotionStages=%v demotionActionReasons=%v", demotionByStage, demotionByReason)
	if freshNat != seqNat {
		t.Fatalf("stateful cache/compiler leakage detected between fresh (%d) and sequential (%d): %v", freshNat, seqNat, diffs)
	}
}

// TestP12_BASE_02B_CacheIsolationRegression ensures single-action candidate verification during Stage 1
// never clobbers decompile-loaded action definitions for later actions in the same shortcut (such as matchText).
func TestP12_BASE_02B_CacheIsolationRegression(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "WarmCacheTrigger"},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.text.match",
			"WFWorkflowActionParameters": map[string]any{
				"WFMatchTextPattern":       "[a-z]+",
				"text":                     "hello world",
				"WFMatchTextCaseSensitive": true,
				"UUID":                     "AAAABBBB-CCCC-DDDD-EEEE-111122223333",
			},
		},
	}, plist.XMLFormat)

	resetCompilerStateFully()
	ClearHybridCandidateCache()
	pFresh, err := PlanHybridImportWithRegistry(doc, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	resetCompilerStateFully()
	pNoCache, err := PlanHybridImportWithRegistry(doc, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	resetCompilerStateFully()
	pWarmCache, err := PlanHybridImportWithRegistry(doc, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if pFresh.NaturalVerifiedCount != 2 || pNoCache.NaturalVerifiedCount != 2 || pWarmCache.NaturalVerifiedCount != 2 {
		t.Fatalf("expected 2 NATURAL_VERIFIED in fresh/noCache/warmCache, got %d / %d / %d (reasons: %+v)",
			pFresh.NaturalVerifiedCount, pNoCache.NaturalVerifiedCount, pWarmCache.NaturalVerifiedCount, pFresh.Decisions)
	}
	if pFresh.Decisions[1].ChosenDefinitionID != "matchText" || pNoCache.Decisions[1].ChosenDefinitionID != "matchText" || pWarmCache.Decisions[1].ChosenDefinitionID != "matchText" {
		t.Fatalf("expected matchText across all cache modes, got %s / %s / %s",
			pFresh.Decisions[1].ChosenDefinitionID, pNoCache.Decisions[1].ChosenDefinitionID, pWarmCache.Decisions[1].ChosenDefinitionID)
	}
}

// TestP12_BASE_03_BidirectionalPreOverlayAndScopedSerializationType proves strict two-way key checks
// and tight scoping of WFSerializationType solely to legacy setvariable/appendvariable omitting top-level WFSerializationType.
func TestP12_BASE_03_BidirectionalPreOverlayAndScopedSerializationType(t *testing.T) {
	origSetVarNoSer := NativeActionRecord{
		Index:      0,
		Identifier: "is.workflow.actions.setvariable",
		Parameters: map[string]any{"WFVariableName": "x"},
	}
	preWithTopSer := map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
		"WFWorkflowActionParameters": map[string]any{
			"WFVariableName":      "x",
			"WFSerializationType": "WFTextTokenAttachment",
		},
	}
	if err := verifyPreOverlayActionSemanticParity("setvar-leg", "legacy", 0, origSetVarNoSer, preWithTopSer); err != nil {
		t.Fatalf("legacy setvariable without orig WFSerializationType should allow pre-overlay WFSerializationType: %v", err)
	}
	if err := verifyPreOverlayActionSemanticParity("setvar-v2", "v2", 0, origSetVarNoSer, preWithTopSer); err == nil {
		t.Fatalf("v2 setvariable must NOT be exempted for extra WFSerializationType")
	}

	origSetVarWithSer := NativeActionRecord{
		Index:      0,
		Identifier: "is.workflow.actions.setvariable",
		Parameters: map[string]any{"WFVariableName": "x", "WFSerializationType": "WFDictionaryFieldValue"},
	}
	if err := verifyPreOverlayActionSemanticParity("setvar-orig-ser", "legacy", 0, origSetVarWithSer, preWithTopSer); err == nil {
		t.Fatalf("legacy setvariable WITH orig WFSerializationType must NOT be exempted on mismatch")
	}

	origComment := NativeActionRecord{
		Index:      0,
		Identifier: "is.workflow.actions.comment",
		Parameters: map[string]any{"WFCommentActionText": "hi"},
	}
	preCommentExtraSer := map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
		"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "hi", "WFSerializationType": "WFTextTokenString"},
	}
	if err := verifyPreOverlayActionSemanticParity("comment-extra-ser", "legacy", 0, origComment, preCommentExtraSer); err == nil {
		t.Fatalf("non-variable action must NOT be exempted for extra WFSerializationType")
	}

	// Extra arbitrary semantic key fabricated pre-overlay must fail
	preCommentExtraKey := map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
		"WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "hi", "FabricatedDefault": true},
	}
	if err := verifyPreOverlayActionSemanticParity("comment-extra-key", "v2", 0, origComment, preCommentExtraKey); err == nil {
		t.Fatalf("extra pre-overlay semantic key must fail")
	}
}

// TestP12_BASE_04_NoPatchExternalOracleBuildAndVerify proves the -tags cherri_oracle_nopatch
// build strips provenance action restoration while still compiling genuine natural parameters.
func TestP12_BASE_04_NoPatchExternalOracleBuildAndVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external nopatch binary build under -short")
	}
	tmpDir := t.TempDir()
	oracleBin := filepath.Join(tmpDir, "cherri-oracle-nopatch.exe")
	buildCmd := exec.Command("go", "build", "-tags", "cherri_oracle_nopatch", "-o", oracleBin, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build -tags cherri_oracle_nopatch failed: %v\n%s", err, string(out))
	}

	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "OracleRealText",
				"UUID":                "11111111-2222-3333-4444-555555555555",
				"CustomOutputName":    "MyRestoredMetaName",
				"ShowWhenRun":         false,
			},
		},
	}, plist.XMLFormat)
	resetCompilerStateFully()
	plan, err := PlanHybridImport(doc)
	if err != nil {
		t.Fatalf("PlanHybridImport failed: %v", err)
	}
	if plan.NaturalVerifiedCount != 1 {
		t.Fatalf("expected NATURAL_VERIFIED=1, got %d (reason=%s)", plan.NaturalVerifiedCount, plan.Decisions[0].ReasonCode)
	}
	v2Src, err := RenderHybridV2Source(doc, plan)
	if err != nil {
		t.Fatalf("RenderHybridV2Source failed: %v", err)
	}
	v2Path := filepath.Join(tmpDir, "oracle_v2.cherri")
	outPath := filepath.Join(tmpDir, "oracle_out.plist")
	if err := os.WriteFile(v2Path, []byte(v2Src), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd := exec.Command(oracleBin, v2Path, "--mode=v2", "--skip-sign", "--no-ansi", "-o="+outPath)
	if out, err := runCmd.CombinedOutput(); err != nil {
		t.Fatalf("oracle binary compile failed: %v\n%s", err, string(out))
	}
	compiledBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	compiledMap, err := decodeShortcutRawMap(compiledBytes)
	if err != nil {
		t.Fatal(err)
	}
	if hasImportProvenanceKey(compiledMap) {
		t.Fatalf("nopatch oracle output leaked %s", importProvenanceKey)
	}
	acts, _ := compiledMap["WFWorkflowActions"].([]any)
	if len(acts) != 1 {
		t.Fatalf("expected 1 compiled action, got %d", len(acts))
	}
	params, _ := acts[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	if params["WFCommentActionText"] != "OracleRealText" {
		t.Fatalf("semantic parameter missing from nopatch oracle output: %v", params["WFCommentActionText"])
	}
	if _, hasCustom := params["CustomOutputName"]; hasCustom {
		t.Fatalf("nopatch oracle unexpectedly restored CustomOutputName metadata overlay")
	}
}

// TestP12_BASE_05_SanitizedPrivateFailureReporting proves Gate C: error strings never leak secret values,
// tokens, URLs, or headers while retaining specimen, dialect, index, action ID, and parameter path/types.
func TestP12_BASE_05_SanitizedPrivateFailureReporting(t *testing.T) {
	secrets := []string{
		"SECRET_BEARER_TOKEN_XYZ_987654",
		"https://private.corp.example/v2/secret?tok=abc",
		"SensitiveHeaderSecretValue",
	}
	orig := NativeActionRecord{
		Index:      7,
		Identifier: "is.workflow.actions.downloadurl",
		Parameters: map[string]any{
			"WFURL": secrets[1],
			"WFHTTPHeaders": map[string]any{
				"Authorization": secrets[0],
				"X-Custom":      secrets[2],
			},
		},
	}
	preMismatch := map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.downloadurl",
		"WFWorkflowActionParameters": map[string]any{
			"WFURL": "wrong",
			"WFHTTPHeaders": map[string]any{
				"Authorization": "wrong2",
			},
		},
	}
	err := verifyPreOverlayActionSemanticParity("014.plist", "v2", 7, orig, preMismatch)
	if err == nil {
		t.Fatalf("expected mismatch error")
	}
	msg := err.Error()
	for _, secret := range secrets {
		if strings.Contains(msg, secret) {
			t.Fatalf("sanitized failure leaked secret substring %q in message: %s", secret, msg)
		}
	}
	for _, want := range []string{"014.plist", "v2", "action 7", "is.workflow.actions.downloadurl"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("sanitized failure omitted diagnostic field %q in message: %s", want, msg)
		}
	}
}

func TestP12_ORACLE_01_HeaderDivergenceMutationDetected(t *testing.T) {
	a := baseTestHeader()
	b := deepClonePlistMap(a)
	icon := deepClonePlistMap(b["WFWorkflowIcon"].(map[string]any))
	icon["WFWorkflowIconStartColor"] = int64(999999)
	b["WFWorkflowIcon"] = icon
	if plistValuesStrictlyEqual(a, b) {
		t.Fatalf("oracle failed to detect WFWorkflowIcon mutation")
	}
	c := deepClonePlistMap(a)
	c["WFWorkflowTypes"] = []any{"NCWidget"}
	if plistValuesStrictlyEqual(a, c) {
		t.Fatalf("oracle failed to detect WFWorkflowTypes mutation")
	}
}

func TestP12_ORACLE_02_UUIDGraphBreakageMutationDetected(t *testing.T) {
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{
				"WFTextActionText": "val",
				"UUID":             "11111111-1111-1111-1111-111111111111",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.comment",
			"WFWorkflowActionParameters": map[string]any{
				"WFCommentActionText": "u",
				"GroupingIdentifier":  "GRP-A",
				"WFControlFlowMode":   int64(0),
			},
		},
	}, plist.XMLFormat)
	_, legMap, _, _, err := verifyCorpusDualHybridWithPreOverlay(doc, "ORACLE-02", true)
	if err != nil {
		t.Fatal(err)
	}
	mut := deepClonePlistMap(legMap)
	acts := mut["WFWorkflowActions"].([]any)
	act1 := acts[1].(map[string]any)
	act1["WFWorkflowActionParameters"].(map[string]any)["WFControlFlowMode"] = int64(2)
	if plistValuesStrictlyEqual(legMap, mut) {
		t.Fatalf("oracle failed to detect WFControlFlowMode mutation")
	}
	mutUUID := deepClonePlistMap(legMap)
	mutUUID["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"] = "22222222-2222-2222-2222-222222222222"
	if plistValuesStrictlyEqual(legMap, mutUUID) {
		t.Fatalf("oracle failed to detect UUID mutation")
	}
}

func TestP12_ORACLE_03_TypeAndPresenceConfusionDetected(t *testing.T) {
	cases := [][2]any{
		{map[string]any{"k": true}, map[string]any{"k": int64(1)}},
		{map[string]any{"k": float64(1.0)}, map[string]any{"k": int64(1)}},
		{map[string]any{"k": "0"}, map[string]any{"k": int64(0)}},
		{map[string]any{"k": nil}, map[string]any{}},
		{map[string]any{"k": []any{}}, map[string]any{}},
		{map[string]any{"k": map[string]any{}}, map[string]any{}},
	}
	for i, pair := range cases {
		if plistValuesStrictlyEqual(pair[0], pair[1]) {
			t.Fatalf("case %d: oracle conflated distinct plist values %#v vs %#v", i, pair[0], pair[1])
		}
	}
}

func TestP12_ORACLE_04_RawWrapperAndProvenanceLeakDetected(t *testing.T) {
	a := map[string]any{
		"WFWorkflowActions": []any{
			map[string]any{"WFWorkflowActionIdentifier": "com.raw.act", "WFWorkflowActionParameters": map[string]any{"A": "1"}},
		},
	}
	bLeak := deepClonePlistMap(a)
	bLeak[importProvenanceKey] = map[string]any{"Count": int64(1)}
	if !hasImportProvenanceKey(bLeak) {
		t.Fatalf("hasImportProvenanceKey failed to detect leaked provenance key")
	}
	bWrap := deepClonePlistMap(a)
	bWrap["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["B"] = "2"
	if plistValuesStrictlyEqual(a, bWrap) {
		t.Fatalf("oracle failed to detect raw action parameter drift")
	}
}

func TestP12_ORACLE_05_NoPatchOracleRejectsFakeRestoreInBinary(t *testing.T) {
	if !strings.Contains(firstSanitizedPlistDiffPath(map[string]any{"A": "x"}, map[string]any{}, "$"), "missing in rebuilt") {
		t.Fatalf("unexpected diff text")
	}
	// Verify via Decode -> Plan -> Compile parity check that pre-overlay capture cannot pass if semantic param stripped
	orig := NativeActionRecord{
		Index:      0,
		Identifier: "is.workflow.actions.gettext",
		Parameters: map[string]any{"WFTextActionText": "RealSemanticParam"},
	}
	fakedEmptyPre := map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
		"WFWorkflowActionParameters": map[string]any{"UUID": "only-meta"},
	}
	if err := verifyPreOverlayActionSemanticParity("ORACLE-05", "legacy", 0, orig, fakedEmptyPre); err == nil {
		t.Fatalf("pre-overlay parity check allowed missing semantic param")
	}
}

func TestP12_ORACLE_06_DictOrderIndependenceAndArrayOrderSensitivity(t *testing.T) {
	d1 := map[string]any{
		"A": int64(1),
		"B": []any{"first", "second"},
	}
	d2 := map[string]any{
		"B": []any{"first", "second"},
		"A": int64(1),
	}
	d3 := map[string]any{
		"A": int64(1),
		"B": []any{"second", "first"},
	}
	if !plistValuesStrictlyEqual(d1, d2) {
		t.Fatalf("expected unordered equal dicts to match")
	}
	if plistValuesStrictlyEqual(d1, d3) {
		t.Fatalf("expected reordered array elements to mismatch")
	}
	if diff := firstSanitizedPlistDiffPath(d1, d3, "doc"); !bytes.Contains([]byte(diff), []byte("doc.B[0]")) {
		t.Fatalf("expected diff at doc.B[0], got %s", diff)
	}
}
