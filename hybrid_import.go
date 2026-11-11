/*
 * Copyright (c) Cherri
 * Shared Hybrid Import Planner & Renderer (P10R9)
 */

package main

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/electrikmilk/args-parser"
)

// HybridActionDecision records the per-action classification, reason code, and rendered
// Legacy/V2 statements produced by PlanHybridImport.
type HybridActionDecision struct {
	ActionIndex        int    `json:"actionIndex"`
	AppleIdentifier    string `json:"appleIdentifier"`
	Classification     string `json:"classification"` // "NATURAL_VERIFIED", "EXACT_NATIVE_FALLBACK", or "SOURCE_INVALID"
	ReasonCode         string `json:"reasonCode"`     // "KNOWN_DISAMBIGUATED", "AMBIGUOUS_VARIANT", "UNREPRESENTABLE_PARAM", "UNKNOWN_NATIVE_ID", "THIRD_PARTY_NO_CANONICAL_DEF", "CONTROL_REGION", "SOURCE_INVALID"
	ChosenDefinitionID string `json:"chosenDefinitionId,omitempty"`
	IncludeCategory    string `json:"includeCategory,omitempty"`
	LegacyCall         string `json:"legacyCall"`
	V2Call             string `json:"v2Call"`
	DiagnosticComment  string `json:"diagnosticComment,omitempty"`
}

// HybridImportPlan is the shared mode-independent plan for --fidelity=hybrid.
type HybridImportPlan struct {
	SourceHash           string                    `json:"sourceHash"`
	SourceFormat         string                    `json:"sourceFormat"`
	TotalActions         int                       `json:"totalActions"`
	NaturalVerifiedCount int                       `json:"naturalVerifiedCount"`
	ExactNativeCount     int                       `json:"exactNativeCount"`
	SourceInvalidCount   int                       `json:"sourceInvalidCount"`
	ReasonCounts         map[string]int            `json:"reasonCounts"`
	RequiredIncludes     []string                  `json:"requiredIncludes,omitempty"`
	Decisions            []HybridActionDecision    `json:"decisions"`
	ControlFlowValid     bool                      `json:"controlFlowValid"`
	ControlFlowViolation *ControlFlowViolationInfo `json:"controlFlowViolation,omitempty"`
}

const emptyXMLPlistRaw = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict></dict></plist>"

type hybridCandidateCacheEntry struct {
	verified   bool
	legacyCall string
	v2Call     string
	chosenID   string
	includeCat string
	reasonCode string
}

var (
	hybridCandidateCacheMu sync.Mutex
	hybridCandidateCache   = make(map[string]hybridCandidateCacheEntry)
)

// ClearHybridCandidateCache clears the memoized candidate verification cache (used by SSoT mutation tests).
func ClearHybridCandidateCache() {
	hybridCandidateCacheMu.Lock()
	defer hybridCandidateCacheMu.Unlock()
	hybridCandidateCache = make(map[string]hybridCandidateCacheEntry)
}

// PlanHybridImport builds a deterministic HybridImportPlan for a decoded NativeShortcutDocument
// using the canonical reverse action registry and strict candidate verification.
func PlanHybridImport(doc *NativeShortcutDocument) (*HybridImportPlan, error) {
	return PlanHybridImportWithRegistry(doc, DefaultReverseActionRegistry(), true)
}

// PlanHybridImportWithRegistry builds a HybridImportPlan using an explicit ReverseActionRegistry.
func PlanHybridImportWithRegistry(doc *NativeShortcutDocument, reg *ReverseActionRegistry, useCache bool) (*HybridImportPlan, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil NativeShortcutDocument")
	}
	if reg == nil {
		reg = DefaultReverseActionRegistry()
	}

	plan := &HybridImportPlan{
		SourceHash:           doc.SourceHash,
		SourceFormat:         doc.SourceFormat,
		TotalActions:         len(doc.Actions),
		ReasonCounts:         make(map[string]int),
		Decisions:            make([]HybridActionDecision, 0, len(doc.Actions)),
		ControlFlowValid:     doc.ControlFlowValid,
		ControlFlowViolation: doc.ControlFlowViolation,
	}

	includeSet := make(map[string]struct{})

	for _, rec := range doc.Actions {
		rawLegacy, err := formatRawLegacyActionCall(rec)
		if err != nil {
			return nil, err
		}
		rawV2, err := formatRawV2ActionCall(rec)
		if err != nil {
			return nil, err
		}

		outcome := reg.ResolveReverseActionCandidate(rec)
		if outcome.ReasonCode == "SOURCE_INVALID" {
			diag := fmt.Sprintf("// [HYBRID_DIAGNOSTIC] action %d (%s): %s", rec.Index, rec.Identifier, rec.SourceValidity)
			plan.SourceInvalidCount++
			plan.ReasonCounts["SOURCE_INVALID"]++
			plan.Decisions = append(plan.Decisions, HybridActionDecision{
				ActionIndex:       rec.Index,
				AppleIdentifier:   rec.Identifier,
				Classification:    "SOURCE_INVALID",
				ReasonCode:        "SOURCE_INVALID",
				LegacyCall:        rawLegacy,
				V2Call:            rawV2,
				DiagnosticComment: diag,
			})
			continue
		}

		if outcome.ReasonCode != "KNOWN_DISAMBIGUATED" || outcome.ChosenCandidate == nil {
			plan.ExactNativeCount++
			plan.ReasonCounts[outcome.ReasonCode]++
			plan.Decisions = append(plan.Decisions, HybridActionDecision{
				ActionIndex:     rec.Index,
				AppleIdentifier: rec.Identifier,
				Classification:  "EXACT_NATIVE_FALLBACK",
				ReasonCode:      outcome.ReasonCode,
				LegacyCall:      rawLegacy,
				V2Call:          rawV2,
			})
			continue
		}

		// Candidate is KNOWN_DISAMBIGUATED; check memoization cache or run strict candidate verification.
		cacheKey := buildHybridCandidateCacheKey(rec, outcome.ChosenCandidate.CallableName)
		var entry hybridCandidateCacheEntry
		var cached bool
		if useCache && cacheKey != "" {
			hybridCandidateCacheMu.Lock()
			entry, cached = hybridCandidateCache[cacheKey]
			hybridCandidateCacheMu.Unlock()
		}

		if !cached {
			entry = synthesizeAndVerifyCandidate(rec, *outcome.ChosenCandidate)
			if useCache && cacheKey != "" {
				hybridCandidateCacheMu.Lock()
				hybridCandidateCache[cacheKey] = entry
				hybridCandidateCacheMu.Unlock()
			}
		}

		if entry.verified {
			plan.NaturalVerifiedCount++
			plan.ReasonCounts["KNOWN_DISAMBIGUATED"]++
			if entry.includeCat != "" {
				includeSet[entry.includeCat] = struct{}{}
			}
			plan.Decisions = append(plan.Decisions, HybridActionDecision{
				ActionIndex:        rec.Index,
				AppleIdentifier:    rec.Identifier,
				Classification:     "NATURAL_VERIFIED",
				ReasonCode:         "KNOWN_DISAMBIGUATED",
				ChosenDefinitionID: entry.chosenID,
				IncludeCategory:    entry.includeCat,
				LegacyCall:         entry.legacyCall,
				V2Call:             entry.v2Call,
			})
		} else {
			fallbackReason := entry.reasonCode
			if fallbackReason == "" {
				fallbackReason = "UNREPRESENTABLE_PARAM"
			}
			plan.ExactNativeCount++
			plan.ReasonCounts[fallbackReason]++
			plan.Decisions = append(plan.Decisions, HybridActionDecision{
				ActionIndex:     rec.Index,
				AppleIdentifier: rec.Identifier,
				Classification:  "EXACT_NATIVE_FALLBACK",
				ReasonCode:      fallbackReason,
				LegacyCall:      rawLegacy,
				V2Call:          rawV2,
			})
		}
	}

	if len(includeSet) > 0 {
		plan.RequiredIncludes = make([]string, 0, len(includeSet))
		for inc := range includeSet {
			plan.RequiredIncludes = append(plan.RequiredIncludes, inc)
		}
		sort.Strings(plan.RequiredIncludes)
	}

	return plan, nil
}

func buildHybridCandidateCacheKey(rec NativeActionRecord, callableName string) string {
	pLit, err := encodePlistValueToLiteral(rec.Parameters)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%t|%s", rec.Identifier, callableName, rec.HadParametersKey, pLit)
}

func synthesizeAndVerifyCandidate(rec NativeActionRecord, cand ReverseActionCandidate) hybridCandidateCacheEntry {
	type callPair struct {
		legacyCall string
		v2Call     string
	}
	var pairs []callPair

	if leg, v2, ok := synthesizeDeclarativeHybridCalls(rec, cand, false); ok {
		pairs = append(pairs, callPair{legacyCall: leg, v2Call: v2})
	}
	if legAlt, v2Alt, ok := synthesizeDeclarativeHybridCalls(rec, cand, true); ok {
		if len(pairs) == 0 || pairs[0].legacyCall != legAlt || pairs[0].v2Call != v2Alt {
			pairs = append(pairs, callPair{legacyCall: legAlt, v2Call: v2Alt})
		}
	}

	for _, pair := range pairs {
		if verifyHybridCandidatePair(rec, cand, pair.legacyCall, pair.v2Call) {
			return hybridCandidateCacheEntry{
				verified:   true,
				legacyCall: pair.legacyCall,
				v2Call:     pair.v2Call,
				chosenID:   cand.CallableName,
				includeCat: cand.IncludeCategory,
				reasonCode: "KNOWN_DISAMBIGUATED",
			}
		}
	}

	return hybridCandidateCacheEntry{
		verified:   false,
		reasonCode: "UNREPRESENTABLE_PARAM",
	}
}

func synthesizeDeclarativeHybridCalls(rec NativeActionRecord, cand ReverseActionCandidate, preferSingleQuoteInLegacy bool) (string, string, bool) {
	if cand.HasCustomMakeParams {
		return "", "", false
	}

	params := cand.Definition.parameters
	if len(params) == 0 {
		// Verify no non-static parameters exist on rec
		for k := range rec.Parameters {
			if k == "AppIntentDescriptor" {
				continue
			}
			if _, isStatic := cand.StaticParameters[k]; !isStatic {
				return "", "", false
			}
		}
		call := cand.CallableName + "()"
		return call, call, true
	}

	lastProvided := -1
	for i, p := range params {
		if p.key == "" {
			return "", "", false
		}
		if _, exists := rec.Parameters[p.key]; exists {
			lastProvided = i
		} else if !p.optional && p.defaultValue == nil {
			// A required parameter is missing from rec.Parameters
			return "", "", false
		}
	}

	if lastProvided == -1 {
		call := cand.CallableName + "()"
		return call, call, true
	}

	// Check for duplicate parameter names in V2
	labelCounts := make(map[string]int, len(params))
	for _, p := range params {
		labelCounts[p.name]++
	}

	legacyArgs := make([]string, 0, lastProvided+1)
	v2Args := make([]string, 0, lastProvided+1)

	for i := 0; i <= lastProvided; i++ {
		p := params[i]
		rawVal, exists := rec.Parameters[p.key]
		if !exists {
			if !p.optional && p.defaultValue == nil {
				return "", "", false
			}
			legacyArgs = append(legacyArgs, "nil")
			continue
		}

		legLit, okLeg := formatHybridScalarOrArrayLiteral(rawVal, p, true, preferSingleQuoteInLegacy)
		v2Lit, okV2 := formatHybridScalarOrArrayLiteral(rawVal, p, false, false)
		if !okLeg || !okV2 {
			return "", "", false
		}
		legacyArgs = append(legacyArgs, legLit)

		if i == 0 {
			v2Args = append(v2Args, v2Lit)
		} else if p.name != "" && labelCounts[p.name] == 1 {
			v2Args = append(v2Args, fmt.Sprintf("%s: %s", p.name, v2Lit))
		} else {
			// Positional fallback if label is empty or duplicated
			if len(v2Args) != i {
				return "", "", false
			}
			v2Args = append(v2Args, v2Lit)
		}
	}

	legCall := fmt.Sprintf("%s(%s)", cand.CallableName, strings.Join(legacyArgs, ", "))
	v2Call := fmt.Sprintf("%s(%s)", cand.CallableName, strings.Join(v2Args, ", "))
	return legCall, v2Call, true
}

func formatHybridScalarOrArrayLiteral(rawVal any, param parameterDefinition, isLegacy bool, preferSingleQuoteInLegacy bool) (string, bool) {
	if rawVal == nil {
		return "", false
	}

	switch x := rawVal.(type) {
	case string:
		for _, r := range x {
			if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
				return "", false
			}
		}
		if !isLegacy && strings.Contains(x, "${") {
			return "", false
		}
		if isLegacy && preferSingleQuoteInLegacy && !strings.ContainsAny(x, "'\n\r\\") {
			return "'" + x + "'", true
		}
		if isLegacy && strings.ContainsAny(x, "{}") && param.validType != RawString {
			if !strings.ContainsAny(x, "'\n\r\\") {
				return "'" + x + "'", true
			}
			return "", false
		}
		return formatCherriDoubleQuotedString(x), true

	case bool:
		if param.validType != Bool && param.validType != Variable {
			return "", false
		}
		if x {
			return "true", true
		}
		return "false", true

	case float32:
		f := float64(x)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return "", false
		}
		return strconv.FormatFloat(f, 'f', -1, 32), true

	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "", false
		}
		s := strconv.FormatFloat(x, 'f', -1, 64)
		if param.validType == Float && !strings.Contains(s, ".") {
			s += ".0"
		}
		return s, true

	case []any:
		if param.validType != Arr {
			return "", false
		}
		elems := make([]string, 0, len(x))
		elemParam := parameterDefinition{validType: String}
		for _, item := range x {
			lit, ok := formatHybridScalarOrArrayLiteral(item, elemParam, isLegacy, preferSingleQuoteInLegacy)
			if !ok {
				return "", false
			}
			elems = append(elems, lit)
		}
		return "[" + strings.Join(elems, ", ") + "]", true

	default:
		if iv, ok := asSignedOrUnsignedInt(rawVal); ok {
			if iv.signed {
				return strconv.FormatInt(iv.sVal, 10), true
			}
			return strconv.FormatUint(iv.uVal, 10), true
		}
		return "", false
	}
}

func verifyHybridCandidatePair(rec NativeActionRecord, cand ReverseActionCandidate, legacyCall string, v2Call string) bool {
	if !verifyHybridLegacyCandidate(rec, cand, legacyCall) {
		return false
	}
	if !verifyHybridV2Candidate(rec, v2Call) {
		return false
	}
	return true
}

func verifyHybridLegacyCandidate(rec NativeActionRecord, cand ReverseActionCandidate, legacyCall string) bool {
	var sb strings.Builder
	sb.WriteString("#workflowheader ")
	sb.WriteString(formatCherriDoubleQuotedString(emptyXMLPlistRaw))
	sb.WriteByte('\n')
	if cand.IncludeCategory != "" {
		sb.WriteString(fmt.Sprintf("#include 'actions/%s'\n", cand.IncludeCategory))
	}
	sb.WriteString(legacyCall)
	sb.WriteByte('\n')

	compiledMap, err := compileLegacySourceToRawMapInMemory(sb.String())
	if err != nil {
		return false
	}
	return singleCompiledActionMatchesRecord(compiledMap, rec)
}

func verifyHybridV2Candidate(rec NativeActionRecord, v2Call string) bool {
	var sb strings.Builder
	sb.WriteString("native.workflow(native.plist(")
	sb.WriteString(formatCherriDoubleQuotedString(emptyXMLPlistRaw))
	sb.WriteString("), exact: true)\n")
	sb.WriteString(v2Call)
	sb.WriteByte('\n')

	plistBytes, err := CompileSourceToPlist("hybrid_verify.cherri", sb.String())
	if err != nil {
		return false
	}
	compiledMap, err := decodeShortcutRawMap(plistBytes)
	if err != nil {
		return false
	}
	return singleCompiledActionMatchesRecord(compiledMap, rec)
}

func singleCompiledActionMatchesRecord(compiledMap map[string]any, rec NativeActionRecord) bool {
	rawActs, ok := compiledMap["WFWorkflowActions"].([]any)
	if !ok || len(rawActs) != 1 {
		return false
	}
	act0, ok := rawActs[0].(map[string]any)
	if !ok {
		return false
	}
	ident, _ := act0["WFWorkflowActionIdentifier"].(string)
	if ident != rec.Identifier {
		return false
	}
	rawParams, hasParams := act0["WFWorkflowActionParameters"]
	if hasParams != rec.HadParametersKey {
		return false
	}
	if !hasParams {
		return true
	}
	compiledParams, _ := rawParams.(map[string]any)
	if compiledParams == nil {
		compiledParams = map[string]any{}
	}
	expectedParams := rec.Parameters
	if expectedParams == nil {
		expectedParams = map[string]any{}
	}
	return plistValuesStrictlyEqual(compiledParams, expectedParams)
}

func compileLegacySourceToRawMapInMemory(src string) (result map[string]any, err error) {
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()

	prevEmbedded := embeddedCompilerMode
	embeddedCompilerMode = true
	previousArgs := args.Args
	args.Args = map[string]string{"no-ansi": ""}

	defer func() {
		embeddedCompilerMode = prevEmbedded
		args.Args = previousArgs
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("legacy compile failed: %s", embeddedErrorMessage(recovered))
			result = nil
			resetEmbeddedFailureState()
		}
		resetMobileLanguageState()
		resetMobileDecompileState()
	}()

	resetMobileLanguageState()
	resetMobileDecompileState()
	filePath = ""
	relativePath = ""
	inputPath = ""
	outputPath = ""
	workflowName = "HybridVerify"
	contents = src

	initParse()
	generateShortcut()

	merged := mergeWorkflowHeaderMap(
		importedWorkflowHeader,
		shortcut.WFWorkflowActions,
		exactActionParamsPresence,
		shortcut,
		false,
	)
	return merged, nil
}

// RenderHybridLegacySource renders a HybridImportPlan as Legacy Cherri source and runs
// a full-workflow verification gate, demoting any action that would alter full-workflow wire parity.
func RenderHybridLegacySource(doc *NativeShortcutDocument, plan *HybridImportPlan) (string, error) {
	if doc == nil || plan == nil {
		return "", fmt.Errorf("nil document or plan")
	}

	rendered, err := formatHybridLegacySource(doc, plan)
	if err != nil {
		return "", err
	}

	// Full-workflow verification gate
	compiledMap, compErr := compileLegacySourceToRawMapInMemory(rendered)
	if compErr != nil || !plistValuesStrictlyEqual(compiledMap, doc.RawRoot) {
		if demoteMismatchedHybridActions(doc, plan, compiledMap) {
			rendered, err = formatHybridLegacySource(doc, plan)
			if err != nil {
				return "", err
			}
		}
	}
	return rendered, nil
}

// RenderHybridV2Source renders a HybridImportPlan as Cherri v2 source and runs
// a full-workflow verification gate, demoting any action that would alter full-workflow wire parity.
func RenderHybridV2Source(doc *NativeShortcutDocument, plan *HybridImportPlan) (string, error) {
	if doc == nil || plan == nil {
		return "", fmt.Errorf("nil document or plan")
	}

	rendered, err := formatHybridV2Source(doc, plan)
	if err != nil {
		return "", err
	}

	// Full-workflow verification gate
	plistBytes, compErr := CompileSourceToPlist("hybrid_workflow_verify.cherri", rendered)
	var compiledMap map[string]any
	if compErr == nil {
		compiledMap, compErr = decodeShortcutRawMap(plistBytes)
	}
	if compErr != nil || !plistValuesStrictlyEqual(compiledMap, doc.RawRoot) {
		if demoteMismatchedHybridActions(doc, plan, compiledMap) {
			rendered, err = formatHybridV2Source(doc, plan)
			if err != nil {
				return "", err
			}
		}
	}
	return rendered, nil
}

func formatHybridLegacySource(doc *NativeShortcutDocument, plan *HybridImportPlan) (string, error) {
	headerLit, err := encodePlistValueToLiteral(doc.Header)
	if err != nil {
		return "", fmt.Errorf("encode workflow header: %w", err)
	}

	var out strings.Builder
	out.WriteString("#workflowheader ")
	out.WriteString(formatCherriDoubleQuotedString(headerLit))
	out.WriteByte('\n')

	incSet := make(map[string]struct{})
	for _, dec := range plan.Decisions {
		if dec.Classification == "NATURAL_VERIFIED" && dec.IncludeCategory != "" {
			incSet[dec.IncludeCategory] = struct{}{}
		}
	}
	if len(incSet) > 0 {
		incs := make([]string, 0, len(incSet))
		for inc := range incSet {
			incs = append(incs, inc)
		}
		sort.Strings(incs)
		for _, inc := range incs {
			out.WriteString(fmt.Sprintf("#include 'actions/%s'\n", inc))
		}
	}

	for _, dec := range plan.Decisions {
		if dec.DiagnosticComment != "" {
			out.WriteString(dec.DiagnosticComment)
			out.WriteByte('\n')
		}
		out.WriteString(dec.LegacyCall)
		out.WriteByte('\n')
	}

	return out.String(), nil
}

func formatHybridV2Source(doc *NativeShortcutDocument, plan *HybridImportPlan) (string, error) {
	headerLit, err := encodePlistValueToLiteral(doc.Header)
	if err != nil {
		return "", fmt.Errorf("encode workflow header: %w", err)
	}

	var out strings.Builder
	out.WriteString("native.workflow(native.plist(")
	out.WriteString(formatCherriDoubleQuotedString(headerLit))
	out.WriteString("), exact: true)\n")

	for _, dec := range plan.Decisions {
		if dec.DiagnosticComment != "" {
			out.WriteString(dec.DiagnosticComment)
			out.WriteByte('\n')
		}
		out.WriteString(dec.V2Call)
		out.WriteByte('\n')
	}

	return out.String(), nil
}

func demoteMismatchedHybridActions(doc *NativeShortcutDocument, plan *HybridImportPlan, compiledMap map[string]any) bool {
	var compiledActs []any
	if compiledMap != nil {
		compiledActs, _ = compiledMap["WFWorkflowActions"].([]any)
	}
	changed := false
	for i := range plan.Decisions {
		dec := &plan.Decisions[i]
		if dec.Classification != "NATURAL_VERIFIED" {
			continue
		}
		rec := doc.Actions[i]
		mismatch := false
		if len(compiledActs) != len(doc.Actions) {
			mismatch = true
		} else {
			actMap, ok := compiledActs[i].(map[string]any)
			if !ok {
				mismatch = true
			} else {
				ident, _ := actMap["WFWorkflowActionIdentifier"].(string)
				rawParams, hasParams := actMap["WFWorkflowActionParameters"]
				if ident != rec.Identifier || hasParams != rec.HadParametersKey {
					mismatch = true
				} else if hasParams {
					cp, _ := rawParams.(map[string]any)
					if cp == nil {
						cp = map[string]any{}
					}
					ep := rec.Parameters
					if ep == nil {
						ep = map[string]any{}
					}
					if !plistValuesStrictlyEqual(cp, ep) {
						mismatch = true
					}
				}
			}
		}
		if mismatch {
			rawLeg, err1 := formatRawLegacyActionCall(rec)
			rawV2, err2 := formatRawV2ActionCall(rec)
			if err1 == nil && err2 == nil {
				dec.Classification = "EXACT_NATIVE_FALLBACK"
				dec.ReasonCode = "UNREPRESENTABLE_PARAM"
				dec.ChosenDefinitionID = ""
				dec.IncludeCategory = ""
				dec.LegacyCall = rawLeg
				dec.V2Call = rawV2
				plan.NaturalVerifiedCount--
				plan.ExactNativeCount++
				plan.ReasonCounts["KNOWN_DISAMBIGUATED"]--
				plan.ReasonCounts["UNREPRESENTABLE_PARAM"]++
				changed = true
			}
		}
	}

	if changed {
		incSet := make(map[string]struct{})
		for _, dec := range plan.Decisions {
			if dec.Classification == "NATURAL_VERIFIED" && dec.IncludeCategory != "" {
				incSet[dec.IncludeCategory] = struct{}{}
			}
		}
		plan.RequiredIncludes = nil
		for inc := range incSet {
			plan.RequiredIncludes = append(plan.RequiredIncludes, inc)
		}
		sort.Strings(plan.RequiredIncludes)
		slices.Sort(plan.RequiredIncludes)
	}
	return changed
}
