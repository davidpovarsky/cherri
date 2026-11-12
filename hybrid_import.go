/*
 * Copyright (c) Cherri
 * Shared Hybrid Import Planner & Renderer (P10R9)
 */

package main

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/electrikmilk/args-parser"
	"howett.net/plist"
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
	RegionGroupID      string `json:"regionGroupId,omitempty"`
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
	Symbols              map[string]map[string]any `json:"-"`
	SymbolProducers      map[string]int            `json:"-"`
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

// PlanHybridImportWithRegistry builds a HybridImportPlan using Cherri's unified semantic
// importer (decompile.go passes + ReverseActionRegistry + fail-closed dual verification).
func PlanHybridImportWithRegistry(doc *NativeShortcutDocument, reg *ReverseActionRegistry, useCache bool) (*HybridImportPlan, error) {
	return planUnifiedSemanticHybridImport(doc, reg, useCache)
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
	sb.WriteString(formatCherriDoubleQuotedString(singleActionProvenanceHeaderLiteral(rec)))
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
	sb.WriteString(formatCherriDoubleQuotedString(singleActionProvenanceHeaderLiteral(rec)))
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

type legacyCompileLineError struct {
	line int
	msg  string
}

func (e legacyCompileLineError) Error() string {
	return fmt.Sprintf("legacy compile failed: %s", e.msg)
}

func resolveLegacyErrorSourceLine(src string) int {
	if len(lines) == 0 || lineIdx < 0 || lineIdx >= len(lines) {
		return 0
	}
	cur := lines[lineIdx]
	origLine := lineIdx + 1
	for _, inc := range includes {
		if lineIdx+1 > inc.end {
			if inc.length > 0 {
				origLine -= inc.length - 1
			}
		} else if lineIdx+1 >= inc.start && lineIdx+1 <= inc.end {
			return 0
		}
	}
	srcLines := strings.Split(src, "\n")
	if origLine >= 1 && origLine <= len(srcLines) && strings.TrimSpace(srcLines[origLine-1]) == strings.TrimSpace(cur) {
		return origLine
	}
	trimmed := strings.TrimSpace(cur)
	if trimmed != "" {
		for delta := 0; delta <= 16; delta++ {
			for _, sign := range []int{-1, 1} {
				cand := origLine + sign*delta
				if cand >= 1 && cand <= len(srcLines) && strings.TrimSpace(srcLines[cand-1]) == trimmed {
					return cand
				}
			}
		}
	}
	if origLine >= 1 && origLine <= len(srcLines) {
		return origLine
	}
	return 0
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
			msg := embeddedErrorMessage(recovered)
			if errLine := resolveLegacyErrorSourceLine(src); errLine > 0 {
				err = legacyCompileLineError{line: errLine, msg: msg}
			} else {
				err = fmt.Errorf("legacy compile failed: %s", msg)
			}
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
	format := plist.XMLFormat
	if plistValueHasXMLUnsafeStringChar(merged) {
		format = plist.BinaryFormat
	}
	plistBytes, mErr := plist.Marshal(merged, format)
	if format == plist.XMLFormat && (mErr != nil || hasUnsafePlistStringBytes(plistBytes) || bytes.Contains(plistBytes, []byte("\r"))) {
		plistBytes, mErr = plist.Marshal(merged, plist.BinaryFormat)
	}
	if mErr != nil {
		return nil, fmt.Errorf("marshal legacy compiled plist: %w", mErr)
	}
	decoded, dErr := decodeShortcutRawMap(plistBytes)
	if dErr != nil {
		if binBytes, bErr := plist.Marshal(merged, plist.BinaryFormat); bErr == nil {
			if binDecoded, binDecErr := decodeShortcutRawMap(binBytes); binDecErr == nil {
				return binDecoded, nil
			}
		}
		return nil, fmt.Errorf("decode legacy compiled plist: %w", dErr)
	}
	return decoded, nil
}

// WholeWorkflowVerificationResult reports the outcome of whole-workflow verification.
type WholeWorkflowVerificationResult struct {
	Equal       bool
	CompileErr  error
	Compiled    map[string]any
	HeaderEqual bool
}

func cloneHybridPlan(plan *HybridImportPlan) *HybridImportPlan {
	c := *plan
	c.ReasonCounts = make(map[string]int, len(plan.ReasonCounts))
	for k, v := range plan.ReasonCounts {
		c.ReasonCounts[k] = v
	}
	c.Decisions = append([]HybridActionDecision(nil), plan.Decisions...)
	c.RequiredIncludes = append([]string(nil), plan.RequiredIncludes...)
	if len(plan.Symbols) > 0 {
		c.Symbols = make(map[string]map[string]any, len(plan.Symbols))
		for k, v := range plan.Symbols {
			c.Symbols[k] = deepClonePlistMap(v)
		}
	}
	if len(plan.SymbolProducers) > 0 {
		c.SymbolProducers = make(map[string]int, len(plan.SymbolProducers))
		for k, v := range plan.SymbolProducers {
			c.SymbolProducers[k] = v
		}
	}
	return &c
}

func compileHybridSource(rendered string, legacy bool) (map[string]any, error) {
	if legacy {
		return compileLegacySourceToRawMapInMemory(rendered)
	}
	plistBytes, err := CompileSourceToPlist("hybrid_workflow_verify.cherri", rendered)
	if err != nil {
		return nil, err
	}
	return decodeShortcutRawMap(plistBytes)
}

// VerifyWholeWorkflowAgainstOriginal compiles rendered source and compares the full decoded plist tree
// (actions and header) with the original document.
func VerifyWholeWorkflowAgainstOriginal(doc *NativeShortcutDocument, rendered string, legacy bool) WholeWorkflowVerificationResult {
	if doc == nil {
		return WholeWorkflowVerificationResult{CompileErr: fmt.Errorf("nil NativeShortcutDocument")}
	}
	compiled, err := compileHybridSource(rendered, legacy)
	if err != nil {
		return WholeWorkflowVerificationResult{CompileErr: err}
	}
	if compiled == nil {
		return WholeWorkflowVerificationResult{CompileErr: fmt.Errorf("compiled plist decoded to nil")}
	}
	if hasImportProvenanceKey(compiled) {
		return WholeWorkflowVerificationResult{
			CompileErr: fmt.Errorf("compiled workflow leaked %s", importProvenanceKey),
			Compiled:   compiled,
		}
	}
	res := WholeWorkflowVerificationResult{Compiled: compiled}
	res.Equal = plistValuesStrictlyEqual(compiled, doc.RawRoot)
	if !res.Equal {
		ch := map[string]any{}
		oh := map[string]any{}
		for k, v := range compiled {
			if k != "WFWorkflowActions" {
				ch[k] = v
			}
		}
		for k, v := range doc.RawRoot {
			if k != "WFWorkflowActions" {
				oh[k] = v
			}
		}
		res.HeaderEqual = plistValuesStrictlyEqual(ch, oh)
	} else {
		res.HeaderEqual = true
	}
	return res
}

// RenderAndVerifyHybrid renders the plan and verifies the full workflow, iteratively demoting
// mismatching natural actions to exact native fallback. It fails closed: any unresolved
// divergence returns an error and leaves the caller's plan untouched.
func RenderAndVerifyHybrid(doc *NativeShortcutDocument, plan *HybridImportPlan, legacy bool) (string, error) {
	if doc == nil || plan == nil {
		return "", fmt.Errorf("nil document or plan")
	}
	if len(plan.Decisions) != len(doc.Actions) {
		return "", fmt.Errorf("hybrid verification failed: plan has %d decisions for %d actions", len(plan.Decisions), len(doc.Actions))
	}
	work := cloneHybridPlan(plan)
	format := formatHybridV2Source
	if legacy {
		format = formatHybridLegacySource
	}
	for iter := 0; iter <= len(work.Decisions)+1; iter++ {
		rendered, err := format(doc, work)
		if err != nil {
			return "", err
		}
		res := VerifyWholeWorkflowAgainstOriginal(doc, rendered, legacy)
		if res.CompileErr == nil && res.Equal {
			*plan = *work
			return rendered, nil
		}
		if res.CompileErr != nil {
			return "", fmt.Errorf("hybrid verification failed: rendered source does not compile: %w", res.CompileErr)
		}
		if !res.HeaderEqual {
			return "", fmt.Errorf("hybrid verification failed: workflow header diverges from original")
		}
		if !demoteMismatchedHybridActions(doc, work, res.Compiled) {
			return "", fmt.Errorf("hybrid verification failed: compiled workflow differs from original and no natural action can be demoted")
		}
	}
	return "", fmt.Errorf("hybrid verification failed: demotion did not converge")
}

// RenderHybridLegacySource renders a HybridImportPlan as Legacy Cherri source with fail-closed whole-workflow verification.
func RenderHybridLegacySource(doc *NativeShortcutDocument, plan *HybridImportPlan) (string, error) {
	return RenderAndVerifyHybrid(doc, plan, true)
}

// RenderHybridV2Source renders a HybridImportPlan as Cherri v2 source with fail-closed whole-workflow verification.
func RenderHybridV2Source(doc *NativeShortcutDocument, plan *HybridImportPlan) (string, error) {
	return RenderAndVerifyHybrid(doc, plan, false)
}

func effectiveSymbolsForPlan(plan *HybridImportPlan) map[string]map[string]any {
	if plan == nil || len(plan.Symbols) == 0 {
		return nil
	}
	out := make(map[string]map[string]any, len(plan.Symbols))
	for symName, meta := range plan.Symbols {
		cloned := deepClonePlistMap(meta)
		if prodIdx, ok := plan.SymbolProducers[symName]; ok && prodIdx >= 0 && prodIdx < len(plan.Decisions) {
			cloned["ProducerRaw"] = plan.Decisions[prodIdx].Classification != "NATURAL_VERIFIED"
		}
		out[symName] = cloned
	}
	return out
}

func formatHybridLegacySource(doc *NativeShortcutDocument, plan *HybridImportPlan) (string, error) {
	headerLit, err := encodePlistValueToLiteral(headerWithImportProvenanceWithSymbols(doc.Header, doc.Actions, effectiveSymbolsForPlan(plan)))
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

	writeIndentedHybridDecisions(&out, plan.Decisions, true)

	return out.String(), nil
}

func formatHybridV2Source(doc *NativeShortcutDocument, plan *HybridImportPlan) (string, error) {
	headerLit, err := encodePlistValueToLiteral(headerWithImportProvenanceWithSymbols(doc.Header, doc.Actions, effectiveSymbolsForPlan(plan)))
	if err != nil {
		return "", fmt.Errorf("encode workflow header: %w", err)
	}

	var out strings.Builder
	out.WriteString("native.workflow(native.plist(")
	out.WriteString(formatCherriDoubleQuotedString(headerLit))
	out.WriteString("), exact: true)\n")

	writeIndentedHybridDecisions(&out, plan.Decisions, false)

	return out.String(), nil
}

func writeIndentedHybridDecisions(out *strings.Builder, decisions []HybridActionDecision, legacy bool) {
	depth := 0
	for _, dec := range decisions {
		call := dec.V2Call
		if legacy {
			call = dec.LegacyCall
		}
		trimmed := strings.TrimSpace(call)
		isControl := dec.Classification == "NATURAL_VERIFIED" && dec.RegionGroupID != ""
		if isControl && strings.HasPrefix(trimmed, "}") && depth > 0 {
			depth--
		}
		indent := ""
		if depth > 0 {
			indent = strings.Repeat("\t", depth)
		}
		if dec.DiagnosticComment != "" {
			out.WriteString(indent)
			out.WriteString(dec.DiagnosticComment)
			out.WriteByte('\n')
		}
		out.WriteString(indent)
		out.WriteString(call)
		out.WriteByte('\n')
		if isControl && strings.HasSuffix(trimmed, "{") {
			depth++
		}
	}
}

func compiledActionMatchesRecord(rawCompiled any, rec NativeActionRecord) bool {
	actMap, ok := rawCompiled.(map[string]any)
	if !ok {
		return false
	}
	ident, _ := actMap["WFWorkflowActionIdentifier"].(string)
	rawParams, hasParams := actMap["WFWorkflowActionParameters"]
	if ident != rec.Identifier || hasParams != rec.HadParametersKey {
		return false
	}
	if !hasParams {
		return true
	}
	cp, _ := rawParams.(map[string]any)
	if cp == nil {
		cp = map[string]any{}
	}
	ep := rec.Parameters
	if ep == nil {
		ep = map[string]any{}
	}
	return plistValuesStrictlyEqual(cp, ep)
}

func recountHybridPlanStats(plan *HybridImportPlan) {
	plan.NaturalVerifiedCount = 0
	plan.ExactNativeCount = 0
	plan.SourceInvalidCount = 0
	plan.ReasonCounts = make(map[string]int)
	incSet := make(map[string]struct{})
	for _, dec := range plan.Decisions {
		switch dec.Classification {
		case "NATURAL_VERIFIED":
			plan.NaturalVerifiedCount++
			if dec.IncludeCategory != "" {
				incSet[dec.IncludeCategory] = struct{}{}
			}
		case "SOURCE_INVALID":
			plan.SourceInvalidCount++
		default:
			plan.ExactNativeCount++
		}
		if dec.ReasonCode != "" {
			plan.ReasonCounts[dec.ReasonCode]++
		}
	}
	plan.RequiredIncludes = nil
	for inc := range incSet {
		plan.RequiredIncludes = append(plan.RequiredIncludes, inc)
	}
	sort.Strings(plan.RequiredIncludes)
}

func demoteMismatchedHybridActions(doc *NativeShortcutDocument, plan *HybridImportPlan, compiledMap map[string]any) bool {
	var compiledActs []any
	if compiledMap != nil {
		compiledActs, _ = compiledMap["WFWorkflowActions"].([]any)
	}
	demoteIdx := make(map[int]bool)
	demoteGroups := make(map[string]bool)

	if len(compiledActs) != len(doc.Actions) {
		limit := len(compiledActs)
		if len(doc.Actions) < limit {
			limit = len(doc.Actions)
		}
		firstDiverge := limit
		for i := 0; i < limit; i++ {
			if !compiledActionMatchesRecord(compiledActs[i], doc.Actions[i]) {
				firstDiverge = i
				break
			}
		}
		target := -1
		for i := firstDiverge; i >= 0; i-- {
			if i < len(plan.Decisions) && plan.Decisions[i].Classification == "NATURAL_VERIFIED" {
				target = i
				break
			}
		}
		if target < 0 {
			for i := range plan.Decisions {
				if plan.Decisions[i].Classification == "NATURAL_VERIFIED" {
					target = i
					break
				}
			}
		}
		if target >= 0 {
			demoteIdx[target] = true
			if rg := plan.Decisions[target].RegionGroupID; rg != "" {
				demoteGroups[rg] = true
			}
		}
	} else {
		for i := range plan.Decisions {
			dec := &plan.Decisions[i]
			if dec.Classification != "NATURAL_VERIFIED" {
				continue
			}
			if !compiledActionMatchesRecord(compiledActs[i], doc.Actions[i]) {
				demoteIdx[i] = true
				if dec.RegionGroupID != "" {
					demoteGroups[dec.RegionGroupID] = true
				}
			}
		}
		// If a native fallback action mismatched because a natural producer/binding shifted it,
		// demote the nearest preceding natural action or group.
		if len(demoteIdx) == 0 {
			for i := range doc.Actions {
				if !compiledActionMatchesRecord(compiledActs[i], doc.Actions[i]) {
					for j := i - 1; j >= 0; j-- {
						if plan.Decisions[j].Classification == "NATURAL_VERIFIED" {
							demoteIdx[j] = true
							if plan.Decisions[j].RegionGroupID != "" {
								demoteGroups[plan.Decisions[j].RegionGroupID] = true
							}
							break
						}
					}
					if len(demoteIdx) > 0 {
						break
					}
				}
			}
		}
	}

	if len(demoteIdx) == 0 && len(demoteGroups) == 0 {
		return false
	}

	changed := false
	for i := range plan.Decisions {
		dec := &plan.Decisions[i]
		if dec.Classification != "NATURAL_VERIFIED" {
			continue
		}
		if !demoteIdx[i] && (dec.RegionGroupID == "" || !demoteGroups[dec.RegionGroupID]) {
			continue
		}
		rec := doc.Actions[i]
		rawLeg, err1 := formatRawLegacyActionCall(rec)
		rawV2, err2 := formatRawV2ActionCall(rec)
		if err1 != nil || err2 != nil {
			continue
		}
		dec.Classification = "EXACT_NATIVE_FALLBACK"
		if dec.RegionGroupID != "" {
			dec.ReasonCode = "CONTROL_REGION"
			dec.RegionGroupID = ""
		} else {
			dec.ReasonCode = "UNREPRESENTABLE_PARAM"
		}
		dec.ChosenDefinitionID = ""
		dec.IncludeCategory = ""
		dec.LegacyCall = rawLeg
		dec.V2Call = rawV2
		changed = true
	}

	if changed {
		recountHybridPlanStats(plan)
	}
	return changed
}

// singleActionProvenanceHeaderLiteral returns an XML plist header literal carrying provenance for one action.
func singleActionProvenanceHeaderLiteral(rec NativeActionRecord) string {
	lit, err := encodePlistValueToLiteral(headerWithImportProvenance(map[string]any{}, []NativeActionRecord{rec}))
	if err != nil {
		return emptyXMLPlistRaw
	}
	return lit
}
