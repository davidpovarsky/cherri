/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

// semanticControlRegion tracks a paired control-flow construct across both Legacy and V2.
type semanticControlRegion struct {
	GroupID string
	Kind    string // "conditional", "choosefrommenu", "repeat.count", "repeat.each"
	Start   int
	Mids    []int
	End     int
	Valid   bool
}

// semanticImportContext holds the single shared semantic interpretation derived from
// decompile.go's analysis passes, NativeShortcutDocument, and DefaultReverseActionRegistry.
type semanticImportContext struct {
	Doc                 *NativeShortcutDocument
	Registry            *ReverseActionRegistry
	UseCache            bool
	Actions             []ShortcutAction
	UsedProducerUUIDs   map[string]bool
	UUIDToSymbol        map[string]string
	SymbolToUUID        map[string]string
	VarToSymbol         map[string]string
	SymbolToVar         map[string]string
	Symbols             map[string]map[string]any
	SymbolProducers     map[string]int
	RegionsByGroup      map[string]*semanticControlRegion
	RegionsByIndex      map[int]*semanticControlRegion
	fallbackLegacyLines []string
	fallbackV2Lines     []string
	fallbackReasons     []string
}

// semanticExprPair holds paired Legacy and V2 Cherri expressions rendered from one semantic AST/value.
type semanticExprPair struct {
	Legacy string
	V2     string
}

var reservedDualSyntaxIdents = func() map[string]bool {
	m := map[string]bool{
		// V2 keywords & built-in symbols
		"if": true, "else": true, "for": true, "in": true, "repeat": true, "as": true,
		"menu": true, "case": true, "item": true, "let": true, "var": true, "func": true,
		"function": true, "action": true, "type": true, "enum": true, "import": true,
		"shortcut": true, "trigger": true, "setup": true, "copy": true, "paste": true,
		"return": true, "yield": true, "const": true, "default": true, "endif": true,
		"endmenu": true, "endrepeat": true, "true": true, "false": true, "nil": true,
		"null": true, "none": true, "and": true, "or": true, "not": true,
		"contains": true, "beginsWith": true, "endsWith": true, "isToday": true,
		"isBetween": true, "between": true, "legacy": true,
		// Shared globals
		"Ask": true, "Clipboard": true, "CurrentDate": true, "Device": true,
		"ShortcutInput": true, "RepeatItem": true, "RepeatIndex": true,
		"system": true, "native": true, "builtin": true,
	}
	for i := 2; i <= 10; i++ {
		m[fmt.Sprintf("RepeatItem%d", i)] = true
		m[fmt.Sprintf("RepeatIndex%d", i)] = true
	}
	for k, v := range cherriKeywords {
		if v {
			m[k] = true
		}
	}
	return m
}()

// planUnifiedSemanticHybridImport builds a single shared semantic import plan using Cherri's
// existing decompiler passes (decompile.go), canonical action definitions, and hybrid verifier.
func planUnifiedSemanticHybridImport(doc *NativeShortcutDocument, reg *ReverseActionRegistry, useCache bool) (*HybridImportPlan, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil NativeShortcutDocument")
	}
	if reg == nil {
		reg = DefaultReverseActionRegistry()
	}

	ctx, err := buildSemanticImportContext(doc, reg, useCache)
	if err != nil {
		return nil, err
	}

	plan := &HybridImportPlan{
		SourceHash:           doc.SourceHash,
		SourceFormat:         doc.SourceFormat,
		TotalActions:         len(doc.Actions),
		ReasonCounts:         make(map[string]int),
		Decisions:            make([]HybridActionDecision, len(doc.Actions)),
		ControlFlowValid:     doc.ControlFlowValid,
		ControlFlowViolation: doc.ControlFlowViolation,
		Symbols:              ctx.Symbols,
		SymbolProducers:      ctx.SymbolProducers,
	}

	ctx.fallbackLegacyLines = make([]string, len(doc.Actions))
	ctx.fallbackV2Lines = make([]string, len(doc.Actions))
	ctx.fallbackReasons = make([]string, len(doc.Actions))

	for i, rec := range doc.Actions {
		rawLeg, errLeg := formatRawLegacyActionCall(rec)
		if errLeg != nil {
			return nil, fmt.Errorf("action %d raw legacy fallback: %w", i, errLeg)
		}
		rawV2, errV2 := formatRawV2ActionCall(rec)
		if errV2 != nil {
			return nil, fmt.Errorf("action %d raw v2 fallback: %w", i, errV2)
		}
		defReason := defaultFallbackReason(doc, reg, rec)
		ctx.fallbackLegacyLines[i] = rawLeg
		ctx.fallbackV2Lines[i] = rawV2
		ctx.fallbackReasons[i] = defReason
		plan.Decisions[i] = HybridActionDecision{
			ActionIndex:     i,
			AppleIdentifier: rec.Identifier,
			Classification:  "EXACT_NATIVE_FALLBACK",
			ReasonCode:      defReason,
			LegacyCall:      rawLeg,
			V2Call:          rawV2,
		}
		if rec.SourceValidity != "VALID" {
			plan.Decisions[i].Classification = "SOURCE_INVALID"
			plan.Decisions[i].ReasonCode = "SOURCE_INVALID"
			plan.Decisions[i].DiagnosticComment = fmt.Sprintf("// [HYBRID_DIAGNOSTIC] action %d (%s): %s", rec.Index, rec.Identifier, rec.SourceValidity)
		}
	}

	// Stage 1: Synthesize paired Legacy+V2 semantic candidates for control regions and actions.
	if err := ctx.synthesizeAllCandidates(plan); err != nil {
		return nil, err
	}

	// Stage 2: Reconcile control-flow region pairing (never leave partial natural region).
	ctx.enforceAtomicRegions(plan)
	recountHybridPlanStats(plan)

	// Stage 3: Verify and stabilize full Legacy + V2 workflows against original doc.RawRoot.
	if err := ctx.verifyAndStabilizePlan(plan); err != nil {
		return nil, err
	}

	return plan, nil
}

func defaultFallbackReason(doc *NativeShortcutDocument, reg *ReverseActionRegistry, rec NativeActionRecord) string {
	if rec.SourceValidity != "VALID" {
		return "SOURCE_INVALID"
	}
	if rec.ControlFlowMode != nil || rec.GroupingIdentifier != "" {
		return "CONTROL_REGION"
	}
	res := reg.ResolveReverseActionCandidate(rec)
	if res.ReasonCode != "" && res.ReasonCode != "KNOWN_DISAMBIGUATED" {
		return res.ReasonCode
	}
	return "UNREPRESENTABLE_PARAM"
}

func buildSemanticImportContext(doc *NativeShortcutDocument, reg *ReverseActionRegistry, useCache bool) (*semanticImportContext, error) {
	// Capture clean plist bytes from doc.RawRoot so .json, .plist, and .shortcut decode uniformly.
	// Fall back to binary plist when XML-unsafe control bytes or bare CR would invalidate or mutate XML parsing.
	format := plist.XMLFormat
	if plistValueHasXMLUnsafeStringChar(doc.RawRoot) {
		format = plist.BinaryFormat
	}
	xmlBytes, err := plist.Marshal(doc.RawRoot, format)
	if format == plist.XMLFormat && (err != nil || hasUnsafePlistStringBytes(xmlBytes) || bytes.Contains(xmlBytes, []byte("\r"))) {
		xmlBytes, err = plist.Marshal(doc.RawRoot, plist.BinaryFormat)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal normalized shortcut for decompile analysis: %w", err)
	}

	ctx := &semanticImportContext{
		Doc:               doc,
		Registry:          reg,
		UseCache:          useCache,
		UsedProducerUUIDs: make(map[string]bool),
		UUIDToSymbol:      make(map[string]string),
		SymbolToUUID:      make(map[string]string),
		VarToSymbol:       make(map[string]string),
		SymbolToVar:       make(map[string]string),
		Symbols:           make(map[string]map[string]any),
		SymbolProducers:   make(map[string]int),
		RegionsByGroup:    make(map[string]*semanticControlRegion),
		RegionsByIndex:    make(map[int]*semanticControlRegion),
	}

	for u, consumers := range doc.ConsumerUUIDIndex {
		if len(consumers) > 0 {
			if _, hasProd := doc.ProducerUUIDIndex[u]; hasProd {
				ctx.UsedProducerUUIDs[u] = true
			}
		}
	}

	// Run decompile.go semantic analysis passes under isolated embeddedCompilerMode.
	if runErr := ctx.runDecompilePasses(xmlBytes); runErr != nil {
		// Non-fatal fallback: build minimal actions slice from doc if decompile pre-pass aborted.
		ctx.Actions = make([]ShortcutAction, len(doc.Actions))
		for i, rec := range doc.Actions {
			ctx.Actions[i] = ShortcutAction{
				WFWorkflowActionIdentifier: rec.Identifier,
				WFWorkflowActionParameters: deepClonePlistMap(rec.Parameters),
			}
		}
	}

	ctx.buildControlRegions()
	ctx.buildDualSymbolTable()
	return ctx, nil
}

func (ctx *semanticImportContext) runDecompilePasses(xmlBytes []byte) (err error) {
	origEmbedded := embeddedCompilerMode
	embeddedCompilerMode = true
	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(embeddedCompilerPanic); ok {
				err = fmt.Errorf("%s", ep.message)
			} else {
				err = fmt.Errorf("decompile semantic pass panic: %v", r)
			}
		}
		embeddedCompilerMode = origEmbedded
	}()

	resetCompilerState()
	resetMobileDecompileState()
	shortcut = Shortcut{}
	code.Reset()
	decompiledIncludes = nil
	actionByUUID = nil
	actionIndexByUUID = nil
	controlFlowGroupByUUID = nil
	emittedConstants = make(map[string]bool)
	activeControlFlowGroups = nil
	pendingFoldUUID = ""
	skipNextLoopVarAction = false
	decompilingText = false
	decompilingDictionary = false
	decompilingRawTextToken = false
	macDefinition = false
	setMacDefinition = false
	lines = nil

	if _, uErr := plist.Unmarshal(xmlBytes, &shortcut); uErr != nil {
		return uErr
	}
	prepareDecompileSemanticPasses()

	ctx.Actions = make([]ShortcutAction, len(shortcut.WFWorkflowActions))
	copy(ctx.Actions, shortcut.WFWorkflowActions)
	return nil
}

func (ctx *semanticImportContext) buildControlRegions() {
	stack := make([]*semanticControlRegion, 0)
	for i, rec := range ctx.Doc.Actions {
		if rec.SourceValidity != "VALID" || rec.ControlFlowMode == nil || rec.GroupingIdentifier == "" {
			continue
		}
		gid := rec.GroupingIdentifier
		kind := controlKindFromIdentifier(rec.Identifier)
		mode := *rec.ControlFlowMode
		switch mode {
		case 0:
			reg := &semanticControlRegion{
				GroupID: gid,
				Kind:    kind,
				Start:   i,
				End:     -1,
			}
			ctx.RegionsByGroup[gid] = reg
			ctx.RegionsByIndex[i] = reg
			stack = append(stack, reg)
		case 1:
			if reg, ok := ctx.RegionsByGroup[gid]; ok && reg.Kind == kind {
				reg.Mids = append(reg.Mids, i)
				ctx.RegionsByIndex[i] = reg
			}
		case 2:
			if reg, ok := ctx.RegionsByGroup[gid]; ok && reg.Kind == kind && reg.End == -1 {
				reg.End = i
				ctx.RegionsByIndex[i] = reg
				if len(stack) > 0 && stack[len(stack)-1].GroupID == gid {
					reg.Valid = true
					stack = stack[:len(stack)-1]
				}
			}
		}
	}
	// Mark any region still open on stack invalid.
	for _, r := range stack {
		r.Valid = false
	}
}

func controlKindFromIdentifier(ident string) string {
	switch ident {
	case "is.workflow.actions.conditional":
		return "conditional"
	case "is.workflow.actions.choosefrommenu":
		return "choosefrommenu"
	case "is.workflow.actions.repeat.count":
		return "repeat.count"
	case "is.workflow.actions.repeat.each":
		return "repeat.each"
	default:
		return ident
	}
}

func sanitizeDualSyntaxIdent(raw string, used map[string]bool, canonicalActions map[string]*actionDefinition, prefix string) string {
	ident := raw
	sanitizeIdentifier(&ident)
	if ident == "" {
		ident = prefix
	}
	// Legacy & V2 identifier grammar require ASCII letters/digits/underscore and non-digit start.
	var sb strings.Builder
	for i, r := range ident {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	clean := sb.String()
	if clean == "" || (clean[0] >= '0' && clean[0] <= '9') {
		clean = prefix + "_" + clean
	}
	if clean == "_" {
		clean = prefix + "_var"
	}
	isForbidden := func(name string) bool {
		if reservedDualSyntaxIdents[name] || used[name] {
			return true
		}
		if canonicalActions != nil {
			if _, isAct := canonicalActions[name]; isAct {
				return true
			}
		}
		if _, isCoerce := contentItemClassesLower()[strings.ToLower(name)]; isCoerce {
			return true
		}
		return false
	}
	if isForbidden(clean) {
		base := clean + "Var"
		clean = base
		for n := 2; isForbidden(clean); n++ {
			clean = fmt.Sprintf("%s_%d", base, n)
		}
	}
	used[clean] = true
	return clean
}

var contentItemClassesCache map[string]bool

func contentItemClassesLower() map[string]bool {
	if contentItemClassesCache != nil {
		return contentItemClassesCache
	}
	contentItemClassesCache = map[string]bool{
		"app": true, "article": true, "boolean": true, "contact": true, "currency": true,
		"date": true, "dictionary": true, "email": true, "file": true, "folder": true,
		"image": true, "location": true, "media": true, "number": true, "pdf": true,
		"phonenumber": true, "photomedia": true, "richtext": true, "safariwebpage": true,
		"text": true, "url": true, "vcard": true,
	}
	return contentItemClassesCache
}

type uuidConsumerOutNameInfo struct {
	sawName    bool
	sawOmitted bool
	name       string
	mixed      bool
}

func (ctx *semanticImportContext) buildDualSymbolTable() {
	canonicalDefs := EnsureCanonicalStandardActions()
	usedNames := make(map[string]bool)

	// 1. Collect OutputName consistency and Dictionary/Aggrandizement type hints across consumers.
	refInfoByUUID := make(map[string]*uuidConsumerOutNameInfo)
	dictHintUUID := make(map[string]bool)
	dictHintVar := make(map[string]bool)
	hasDictAggrandizement := func(att map[string]any) bool {
		ags, _ := att["Aggrandizements"].([]any)
		for _, a := range ags {
			if am, ok := a.(map[string]any); ok {
				if am["Type"] == "WFDictionaryValueVariableAggrandizement" {
					return true
				}
			}
		}
		return false
	}
	var collectAtt func(v any)
	collectAtt = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			switch typ, _ := x["Type"].(string); typ {
			case "ActionOutput":
				if u, _ := x["OutputUUID"].(string); u != "" {
					if hasDictAggrandizement(x) {
						dictHintUUID[u] = true
					}
					info := refInfoByUUID[u]
					if info == nil {
						info = &uuidConsumerOutNameInfo{}
						refInfoByUUID[u] = info
					}
					if on, has := x["OutputName"].(string); has {
						if info.sawOmitted || (info.sawName && info.name != on) {
							info.mixed = true
						}
						info.sawName = true
						if info.name == "" {
							info.name = on
						}
					} else {
						if info.sawName {
							info.mixed = true
						}
						info.sawOmitted = true
					}
				}
			case "Variable":
				if vn, _ := x["VariableName"].(string); vn != "" && hasDictAggrandizement(x) {
					dictHintVar[vn] = true
				}
			}
			for _, mv := range x {
				collectAtt(mv)
			}
		case []any:
			for _, elem := range x {
				collectAtt(elem)
			}
		}
	}
	for _, rec := range ctx.Doc.Actions {
		collectAtt(rec.Parameters)
	}

	// 2. Deterministic sorted allocation for Output symbols (reusing uuids computed by mapIdentifiers).
	usedUUIDs := make([]string, 0, len(ctx.UsedProducerUUIDs))
	for u := range ctx.UsedProducerUUIDs {
		usedUUIDs = append(usedUUIDs, u)
	}
	sort.Slice(usedUUIDs, func(i, j int) bool {
		pi := ctx.Doc.ProducerUUIDIndex[usedUUIDs[i]]
		pj := ctx.Doc.ProducerUUIDIndex[usedUUIDs[j]]
		if pi != pj {
			return pi < pj
		}
		return usedUUIDs[i] < usedUUIDs[j]
	})

	for _, u := range usedUUIDs {
		prodIdx, ok := ctx.Doc.ProducerUUIDIndex[u]
		if !ok {
			continue
		}
		rec := ctx.Doc.Actions[prodIdx]
		info := refInfoByUUID[u]
		if info != nil && info.mixed {
			// Consumers disagree on OutputName for this UUID; avoid unsafe symbol sharing.
			continue
		}

		// Reuse decompile.go's mapped identifier when present!
		hint := uuids[u]
		if hint == "" {
			if rec.CustomOutputName != "" {
				hint = rec.CustomOutputName
			} else if info != nil && info.name != "" {
				hint = info.name
			} else {
				hint = defaultActionOutputName(rec.Identifier)
			}
		}
		sym := sanitizeDualSyntaxIdent(hint, usedNames, canonicalDefs, "out")
		ctx.UUIDToSymbol[u] = sym
		ctx.SymbolToUUID[sym] = u
		ctx.SymbolProducers[sym] = prodIdx

		hasOutName := false
		outName := ""
		if info != nil && info.sawName {
			hasOutName = true
			outName = info.name
		} else if rec.CustomOutputName != "" && (info == nil || !info.sawOmitted) {
			hasOutName = true
			outName = rec.CustomOutputName
		}

		valType := inferValueTypeForAction(rec.Identifier, canonicalDefs)
		if dictHintUUID[u] {
			valType = "Dictionary"
		}
		ctx.Symbols[sym] = map[string]any{
			"Kind":          "Output",
			"UUID":          u,
			"OutputName":    outName,
			"HasOutputName": hasOutName,
			"ValueType":     valType,
		}
	}

	// 3. Deterministic allocation for Variables (from setvariable, appendvariable, and Variable refs).
	firstVarWriter := make(map[string]int)
	firstVarReader := make(map[string]int)
	appendVarSet := make(map[string]bool)
	setVarBeforeAppend := make(map[string]bool)
	allVarSet := make(map[string]bool)
	for i, rec := range ctx.Doc.Actions {
		if rec.Identifier == "is.workflow.actions.setvariable" || rec.Identifier == "is.workflow.actions.appendvariable" {
			if vn, _ := rec.Parameters["WFVariableName"].(string); vn != "" {
				allVarSet[vn] = true
				if rec.Identifier == "is.workflow.actions.appendvariable" {
					appendVarSet[vn] = true
				} else {
					setVarBeforeAppend[vn] = true
				}
				if _, seen := firstVarWriter[vn]; !seen {
					firstVarWriter[vn] = i
				}
			}
		}
		for _, rv := range rec.ReferencedVars {
			if rv != "" {
				allVarSet[rv] = true
				if _, seen := firstVarReader[rv]; !seen {
					firstVarReader[rv] = i
				}
			}
		}
	}

	varNames := make([]string, 0, len(allVarSet))
	for vn := range allVarSet {
		varNames = append(varNames, vn)
	}
	sort.Slice(varNames, func(i, j int) bool {
		wi, hwi := firstVarWriter[varNames[i]]
		wj, hwj := firstVarWriter[varNames[j]]
		if hwi && hwj && wi != wj {
			return wi < wj
		}
		if hwi != hwj {
			return hwi
		}
		return varNames[i] < varNames[j]
	})

	for _, origVar := range varNames {
		sym := sanitizeDualSyntaxIdent(origVar, usedNames, canonicalDefs, "v")
		ctx.VarToSymbol[origVar] = sym
		ctx.SymbolToVar[sym] = origVar

		entry := map[string]any{
			"Kind":         "Variable",
			"VariableName": origVar,
		}
		if dictHintVar[origVar] {
			entry["ValueType"] = "Dictionary"
		} else if appendVarSet[origVar] && !setVarBeforeAppend[origVar] {
			entry["ValueType"] = "List"
		}
		wIdx, hasWriter := firstVarWriter[origVar]
		rIdx, hasReader := firstVarReader[origVar]
		if !hasWriter || (hasReader && rIdx <= wIdx) {
			// Referenced before first natural write or written only externally: pre-seed Legacy+V2.
			entry["ProducerRaw"] = true
		} else {
			ctx.SymbolProducers[sym] = wIdx
		}
		ctx.Symbols[sym] = entry
	}
}

func inferValueTypeForAction(appleID string, defs map[string]*actionDefinition) string {
	switch appleID {
	case "is.workflow.actions.gettext":
		return "Text"
	case "is.workflow.actions.number", "is.workflow.actions.count", "is.workflow.actions.calculateexpression",
		"is.workflow.actions.math", "is.workflow.actions.calculate", "is.workflow.actions.round",
		"is.workflow.actions.statistics", "is.workflow.actions.randomnumber":
		return "Number"
	case "is.workflow.actions.dictionary", "is.workflow.actions.detect.dictionary", "is.workflow.actions.setvalueforkey":
		return "Dictionary"
	case "is.workflow.actions.list", "is.workflow.actions.text.split":
		return "List"
	}
	for _, def := range defs {
		if def != nil && (def.identifier == appleID || def.overrideIdentifier == appleID) {
			switch def.outputType {
			case String:
				return "Text"
			case Integer, Float:
				return "Number"
			case Bool:
				return "Bool"
			case Date:
				return "Date"
			case Dict:
				return "Dictionary"
			case Arr:
				return "List"
			}
		}
	}
	return ""
}

func (ctx *semanticImportContext) synthesizeAllCandidates(plan *HybridImportPlan) error {
	// Synthesize control-flow regions first so RegionGroupID is wired consistently.
	gids := make([]string, 0, len(ctx.RegionsByGroup))
	for gid := range ctx.RegionsByGroup {
		gids = append(gids, gid)
	}
	sort.Strings(gids)
	for _, gid := range gids {
		reg := ctx.RegionsByGroup[gid]
		if reg == nil || !reg.Valid || !ctx.Doc.ControlFlowValid {
			continue
		}
		ctx.trySynthesizeRegion(reg, plan)
	}

	// Synthesize non-control actions using decompile.go recognizers + canonical reverse registry.
	for i, rec := range ctx.Doc.Actions {
		if rec.SourceValidity != "VALID" {
			continue
		}
		if _, isCtrl := ctx.RegionsByIndex[i]; isCtrl || rec.ControlFlowMode != nil || rec.GroupingIdentifier != "" {
			continue
		}
		ctx.trySynthesizeSingleAction(i, rec, plan)
	}
	return nil
}

func (ctx *semanticImportContext) trySynthesizeSingleAction(idx int, rec NativeActionRecord, plan *HybridImportPlan) {
	if metaHasUnsupportedShape(rec) {
		return
	}

	var legCall, v2Call, chosenID, includeCat string
	var ok bool

	switch rec.Identifier {
	case "is.workflow.actions.comment":
		legCall, v2Call, ok = ctx.synthesizeComment(rec)
		chosenID = "comment"
	case "is.workflow.actions.gettext":
		legCall, v2Call, ok = ctx.synthesizeTextAction(rec)
		chosenID = "text"
	case "is.workflow.actions.number":
		legCall, v2Call, ok = ctx.synthesizeNumberAction(rec)
		chosenID = "number"
	case "is.workflow.actions.dictionary":
		legCall, v2Call, ok = ctx.synthesizeDictionaryAction(rec)
		chosenID = "dictionary"
	case "is.workflow.actions.setvariable":
		legCall, v2Call, ok = ctx.synthesizeSetVariable(rec)
		chosenID = "setVariable"
	case "is.workflow.actions.appendvariable":
		legCall, v2Call, ok = ctx.synthesizeAppendVariable(rec)
		chosenID = "appendVariable"
	default:
		legCall, v2Call, chosenID, includeCat, ok = ctx.synthesizeCanonicalOrSplitAction(idx, rec)
	}

	if !ok || legCall == "" || v2Call == "" {
		return
	}

	// Bind producer output UUID to `const sym = ...` (Legacy) / `let sym = ...` (V2) when referenced.
	if rec.UUID != "" && ctx.UsedProducerUUIDs[rec.UUID] {
		sym, hasSym := ctx.UUIDToSymbol[rec.UUID]
		if !hasSym || sym == "" {
			return
		}
		if rec.Identifier == "is.workflow.actions.setvariable" || rec.Identifier == "is.workflow.actions.appendvariable" ||
			rec.Identifier == "is.workflow.actions.comment" {
			return
		}
		legCall = fmt.Sprintf("const %s = %s", sym, legCall)
		v2Call = fmt.Sprintf("let %s = %s", sym, v2Call)
	}

	// When safe/context-free and not already verified in workflow, use cached single-action verifier!
	if ctx.isContextFreeRecord(rec) && rec.Identifier != "is.workflow.actions.setvariable" && rec.Identifier != "is.workflow.actions.appendvariable" {
		if cands := ctx.Registry.ByAppleIdentifier[rec.Identifier]; len(cands) > 0 {
			for _, cand := range cands {
				if cand.CallableName == chosenID {
					key := buildHybridCandidateCacheKey(rec, cand.CallableName)
					if ctx.UseCache {
						hybridCandidateCacheMu.Lock()
						cached, hit := hybridCandidateCache[key]
						hybridCandidateCacheMu.Unlock()
						if hit {
							if !cached.verified {
								return
							}
							applyDecisionCandidate(&plan.Decisions[idx], legCall, v2Call, chosenID, includeCat)
							return
						}
					}
					verified := verifyHybridCandidatePair(rec, cand, legCall, v2Call)
					if ctx.UseCache {
						hybridCandidateCacheMu.Lock()
						hybridCandidateCache[key] = hybridCandidateCacheEntry{
							verified:   verified,
							legacyCall: legCall,
							v2Call:     v2Call,
							chosenID:   chosenID,
							includeCat: includeCat,
						}
						hybridCandidateCacheMu.Unlock()
					}
					if !verified {
						return
					}
					applyDecisionCandidate(&plan.Decisions[idx], legCall, v2Call, chosenID, includeCat)
					return
				}
			}
		}
	}

	applyDecisionCandidate(&plan.Decisions[idx], legCall, v2Call, chosenID, includeCat)
}

func applyDecisionCandidate(dec *HybridActionDecision, legCall, v2Call, chosenID, includeCat string) {
	dec.Classification = "NATURAL_VERIFIED"
	dec.ReasonCode = "KNOWN_DISAMBIGUATED"
	dec.ChosenDefinitionID = chosenID
	dec.IncludeCategory = includeCat
	dec.LegacyCall = legCall
	dec.V2Call = v2Call
}

func (ctx *semanticImportContext) isContextFreeRecord(rec NativeActionRecord) bool {
	if rec.UUID != "" || rec.CustomOutputName != "" || len(rec.ReferencedUUIDs) > 0 || len(rec.ReferencedVars) > 0 {
		return false
	}
	for k := range rec.Parameters {
		if strings.HasPrefix(k, "Show") {
			return false
		}
	}
	return true
}

func metaHasUnsupportedShape(rec NativeActionRecord) bool {
	// If CustomOutputName is present without UUID, Legacy/V2 won't preserve standalone CustomOutputName unless UUID is present.
	if _, hasCustom := rec.Parameters["CustomOutputName"]; hasCustom {
		if _, hasUUID := rec.Parameters["UUID"]; !hasUUID {
			return true
		}
	}
	return false
}

func (ctx *semanticImportContext) hasOnlyAllowedExtraKeys(params map[string]any, allowed map[string]bool) bool {
	for k := range params {
		if isImportProvenanceMetaKey(k) || allowed[k] {
			continue
		}
		return false
	}
	return true
}

func (ctx *semanticImportContext) synthesizeComment(rec NativeActionRecord) (string, string, bool) {
	if !rec.HadParametersKey {
		return "", "", false
	}
	if !ctx.hasOnlyAllowedExtraKeys(rec.Parameters, map[string]bool{"WFCommentActionText": true}) {
		return "", "", false
	}
	txtRaw, hasText := rec.Parameters["WFCommentActionText"]
	if !hasText {
		// Empty params comment(): check whether comment() vs comment("") matches rec
		return "comment(\"\")", "comment(\"\")", true
	}
	txt, ok := txtRaw.(string)
	if !ok {
		return "", "", false
	}
	expr, ok := ctx.renderStringPair(txt)
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf("comment(%s)", expr.Legacy), fmt.Sprintf("comment(%s)", expr.V2), true
}

func (ctx *semanticImportContext) synthesizeTextAction(rec NativeActionRecord) (string, string, bool) {
	if !rec.HadParametersKey {
		return "", "", false
	}
	if !ctx.hasOnlyAllowedExtraKeys(rec.Parameters, map[string]bool{"WFTextActionText": true}) {
		return "", "", false
	}
	rawVal, hasVal := rec.Parameters["WFTextActionText"]
	if !hasVal {
		return "text(\"\")", "text(\"\")", true
	}
	expr, ok := ctx.renderAnyTextPair(rawVal, true)
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf("text(%s)", expr.Legacy), fmt.Sprintf("text(%s)", expr.V2), true
}

func (ctx *semanticImportContext) synthesizeNumberAction(rec NativeActionRecord) (string, string, bool) {
	if !rec.HadParametersKey {
		return "", "", false
	}
	if !ctx.hasOnlyAllowedExtraKeys(rec.Parameters, map[string]bool{"WFNumberActionNumber": true}) {
		return "", "", false
	}
	rawVal, hasVal := rec.Parameters["WFNumberActionNumber"]
	if !hasVal {
		return "", "", false
	}
	lit, ok := ctx.renderNumberOrAttachmentPair(rawVal)
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf("number(%s)", lit.Legacy), fmt.Sprintf("number(%s)", lit.V2), true
}

func (ctx *semanticImportContext) synthesizeDictionaryAction(rec NativeActionRecord) (string, string, bool) {
	if !rec.HadParametersKey || rec.UUID == "" || !ctx.UsedProducerUUIDs[rec.UUID] {
		return "", "", false
	}
	if !ctx.hasOnlyAllowedExtraKeys(rec.Parameters, map[string]bool{"WFItems": true}) {
		return "", "", false
	}
	rawItems, hasItems := rec.Parameters["WFItems"].(map[string]any)
	if !hasItems {
		return "", "", false
	}
	ser, _ := rawItems["WFSerializationType"].(string)
	if ser != "WFDictionaryFieldValue" {
		return "", "", false
	}
	pair, ok := ctx.renderStrictSortedDictionaryPair(rawItems)
	if !ok {
		return "", "", false
	}
	return pair.Legacy, pair.V2, true
}

func (ctx *semanticImportContext) synthesizeSetVariable(rec NativeActionRecord) (string, string, bool) {
	if !rec.HadParametersKey {
		return "", "", false
	}
	if !ctx.hasOnlyAllowedExtraKeys(rec.Parameters, map[string]bool{"WFVariableName": true, "WFInput": true}) {
		return "", "", false
	}
	vn, ok := rec.Parameters["WFVariableName"].(string)
	if !ok || vn == "" {
		return "", "", false
	}
	sym, hasSym := ctx.VarToSymbol[vn]
	if !hasSym {
		return "", "", false
	}
	// If provenance SymbolProducers[sym] differs from this index AND this variable is referenced
	// before an earlier assignment, ProducerRaw=true in header already covers it.
	rawIn, hasIn := rec.Parameters["WFInput"]
	if !hasIn {
		return "", "", false
	}
	valExpr, ok := ctx.renderVariableInputPair(rawIn)
	if !ok {
		return "", "", false
	}
	// Legacy uses `@sym = ...`; V2 uses `sym = ...` (pre-defined mutable in header provenance).
	return fmt.Sprintf("@%s = %s", sym, valExpr.Legacy), fmt.Sprintf("%s = %s", sym, valExpr.V2), true
}

func (ctx *semanticImportContext) synthesizeAppendVariable(rec NativeActionRecord) (string, string, bool) {
	if !rec.HadParametersKey {
		return "", "", false
	}
	if !ctx.hasOnlyAllowedExtraKeys(rec.Parameters, map[string]bool{"WFVariableName": true, "WFInput": true}) {
		return "", "", false
	}
	vn, ok := rec.Parameters["WFVariableName"].(string)
	if !ok || vn == "" {
		return "", "", false
	}
	sym, hasSym := ctx.VarToSymbol[vn]
	if !hasSym {
		return "", "", false
	}
	rawIn, hasIn := rec.Parameters["WFInput"]
	if !hasIn {
		return "", "", false
	}
	valExpr, ok := ctx.renderVariableInputPair(rawIn)
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf("@%s += %s", sym, valExpr.Legacy), fmt.Sprintf("%s += %s", sym, valExpr.V2), true
}

func (ctx *semanticImportContext) renderVariableInputPair(rawIn any) (semanticExprPair, bool) {
	switch v := rawIn.(type) {
	case string:
		return ctx.renderStringPair(v)
	case bool:
		if v {
			return semanticExprPair{Legacy: "true", V2: "true"}, true
		}
		return semanticExprPair{Legacy: "false", V2: "false"}, true
	case int, int64, uint64, float64:
		return ctx.renderNumberLiteralPair(v)
	case map[string]any:
		ser, _ := v["WFSerializationType"].(string)
		switch ser {
		case "WFTextTokenAttachment":
			return ctx.renderSerializedAttachmentPair(v, true)
		case "WFTextTokenString":
			return ctx.renderWFTextTokenStringPair(v)
		case "WFDictionaryFieldValue":
			return ctx.renderWFDictionaryPair(v)
		default:
			if typ, _ := v["Type"].(string); typ != "" {
				return ctx.renderAttachmentBodyPair(v, true)
			}
		}
	case []any:
		return ctx.renderArrayLiteralPair(v)
	}
	return semanticExprPair{}, false
}

func (ctx *semanticImportContext) synthesizeCanonicalOrSplitAction(idx int, rec NativeActionRecord) (string, string, string, string, bool) {
	cands := ctx.Registry.ByAppleIdentifier[rec.Identifier]
	if len(cands) == 0 {
		return "", "", "", "", false
	}

	// Pick candidate via decompile.go's matchAction when split action or multiple overloads exist,
	// with deterministic fallback over sorted candidates from DefaultReverseActionRegistry.
	ordered := ctx.selectOrderedCandidates(idx, rec, cands)
	if len(ordered) == 0 {
		return "", "", "", "", false
	}

	for _, cand := range ordered {
		legCall, v2Call, ok := ctx.synthesizeCallFromCandidate(rec, cand)
		if !ok {
			continue
		}
		return legCall, v2Call, cand.CallableName, cand.IncludeCategory, true
	}
	return "", "", "", "", false
}

func (ctx *semanticImportContext) selectOrderedCandidates(idx int, rec NativeActionRecord, cands []ReverseActionCandidate) []ReverseActionCandidate {
	if !rec.HadParametersKey {
		return nil
	}
	if len(cands) == 1 {
		return cands
	}
	var out []ReverseActionCandidate
	seen := make(map[string]bool)

	// Re-use decompile.go's matchAction / scoreActionMatch heuristic first when ShortcutAction is available.
	if idx >= 0 && idx < len(ctx.Actions) {
		actCopy := ctx.Actions[idx]
		matchedName, _ := matchAction(&actCopy)
		if matchedName != "" {
			for _, c := range cands {
				if c.CallableName == matchedName && !seen[c.CallableName] {
					seen[c.CallableName] = true
					out = append(out, c)
				}
			}
		}
	}

	// Next prefer candidates whose StaticParameters match the wire parameters cleanly.
	for _, c := range cands {
		if seen[c.CallableName] {
			continue
		}
		if staticParamsCompatible(rec.Parameters, c.StaticParameters) && requiredParamsPresent(rec.Parameters, c.RequiredParameterKeys) {
			seen[c.CallableName] = true
			out = append(out, c)
		}
	}
	for _, c := range cands {
		if !seen[c.CallableName] && staticParamsCompatible(rec.Parameters, c.StaticParameters) {
			seen[c.CallableName] = true
			out = append(out, c)
		}
	}
	return out
}

func staticParamsCompatible(wire map[string]any, staticParams map[string]any) bool {
	for k, expected := range staticParams {
		actual, ok := wire[k]
		if !ok || !plistCanonicalEqual(expected, actual) {
			return false
		}
	}
	return true
}

func requiredParamsPresent(wire map[string]any, reqKeys []string) bool {
	for _, k := range reqKeys {
		if _, ok := wire[k]; !ok {
			return false
		}
	}
	return true
}

func (ctx *semanticImportContext) synthesizeCallFromCandidate(rec NativeActionRecord, cand ReverseActionCandidate) (string, string, bool) {
	if isCompilerConstructOnlyIdentifier(cand.CallableName) {
		return "", "", false
	}
	if !staticParamsCompatible(rec.Parameters, cand.StaticParameters) {
		return "", "", false
	}

	allowedKeys := make(map[string]bool, len(cand.StaticParameters)+len(cand.Definition.parameters)+len(cand.Definition.emittedKeys))
	for k := range cand.StaticParameters {
		allowedKeys[k] = true
	}
	for _, p := range cand.Definition.parameters {
		if p.key != "" {
			allowedKeys[p.key] = true
		}
	}
	for _, ek := range cand.Definition.emittedKeys {
		if ek != "" {
			allowedKeys[ek] = true
		}
	}
	if cand.AppIntent != (appIntent{}) || cand.Definition.appIntent != (appIntent{}) {
		allowedKeys["AppIntentDescriptor"] = true
	}
	if !ctx.hasOnlyAllowedExtraKeys(rec.Parameters, allowedKeys) {
		return "", "", false
	}

	def := cand.Definition
	// If AppIntent is expected, ensure wire matches AppIntentDescriptor if present.
	if len(rec.AppIntentDescriptor) > 0 && def.appIntent != (appIntent{}) {
		expectedBundle, _ := rec.AppIntentDescriptor["BundleIdentifier"].(string)
		if def.appIntent.bundleIdentifier != "" && expectedBundle != "" && def.appIntent.bundleIdentifier != expectedBundle {
			return "", "", false
		}
	}

	if len(def.parameters) == 0 {
		if len(cand.StaticParameters) == 0 && !cand.HasCustomMakeParams && len(nonMetaKeys(rec.Parameters)) > 0 {
			return "", "", false
		}
		// Simple zero-arg call; fast single-action check handles AppIntent/StaticParam exactness cleanly.
		call := cand.CallableName + "()"
		return call, call, true
	}

	legArgs, v2NamedArgs, ok := ctx.synthesizeDefinitionArgs(rec, cand)
	if !ok {
		return "", "", false
	}

	// Trim trailing empty/omitted positional args in Legacy!
	legCall := fmt.Sprintf("%s(%s)", cand.CallableName, strings.Join(legArgs, ", "))
	var v2Call string
	if len(v2NamedArgs) == 0 {
		v2Call = cand.CallableName + "()"
	} else {
		parts := make([]string, 0, len(v2NamedArgs))
		for _, na := range v2NamedArgs {
			if na.label == "" {
				parts = append(parts, na.expr)
			} else {
				parts = append(parts, fmt.Sprintf("%s: %s", na.label, na.expr))
			}
		}
		v2Call = fmt.Sprintf("%s(%s)", cand.CallableName, strings.Join(parts, ", "))
	}
	return legCall, v2Call, true
}

func nonMetaKeys(params map[string]any) []string {
	var keys []string
	for k := range params {
		if isImportProvenanceMetaKey(k) {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

type v2NamedArg struct {
	label string
	expr  string
}

func (ctx *semanticImportContext) synthesizeDefinitionArgs(rec NativeActionRecord, cand ReverseActionCandidate) ([]string, []v2NamedArg, bool) {
	def := cand.Definition
	hasDupLabel := hasDuplicateParamLabels(def.parameters)

	type renderedSlot struct {
		present bool
		isInf   bool
		legList []string
		v2List  []string
		label   string
	}
	slots := make([]renderedSlot, len(def.parameters))

	for i, p := range def.parameters {
		if p.key == "" {
			if ep, handled, okTP := ctx.reverseTextPartsParam(rec, cand.CallableName, p); handled {
				if !okTP {
					return nil, nil, false
				}
				if ep.Legacy != "" || ep.V2 != "" {
					slots[i] = renderedSlot{present: true, legList: []string{ep.Legacy}, v2List: []string{ep.V2}, label: p.name}
				}
				continue
			}
			if !p.optional {
				return nil, nil, false
			}
			continue
		}
		rawVal, exists := rec.Parameters[p.key]
		if !exists {
			if !p.optional {
				return nil, nil, false
			}
			continue
		}

		// Handle infinite/variadic parameter (e.g. list(...), ask, etc.)
		if p.infinite {
			if items, ok := rawVal.([]any); ok {
				if len(items) == 0 && p.optional {
					continue
				}
				var lItems, vItems []string
				for _, it := range items {
					effItem := it
					if cand.CallableName == "list" {
						uItem, okU := unwrapWFListItem(it)
						if !okU {
							return nil, nil, false
						}
						effItem = uItem
					}
					ep, ok := ctx.renderParameterValuePair(p, effItem)
					if !ok {
						return nil, nil, false
					}
					lItems = append(lItems, ep.Legacy)
					vItems = append(vItems, ep.V2)
				}
				slots[i] = renderedSlot{present: true, isInf: true, legList: lItems, v2List: vItems, label: p.name}
				continue
			}
		}

		ep, ok := ctx.renderParameterValuePair(p, rawVal)
		if !ok {
			return nil, nil, false
		}
		slots[i] = renderedSlot{present: true, legList: []string{ep.Legacy}, v2List: []string{ep.V2}, label: p.name}
	}

	// Build Legacy positional arg list trimming trailing absent optionals, inserting defaults/nil for earlier gaps.
	lastPresent := -1
	for i := len(slots) - 1; i >= 0; i-- {
		if slots[i].present {
			lastPresent = i
			break
		}
	}

	var legArgs []string
	var v2Args []v2NamedArg
	if lastPresent >= 0 {
		for i := 0; i <= lastPresent; i++ {
			s := slots[i]
			p := def.parameters[i]
			if !s.present {
				// Intermediate absent optional: emit `nil` in Legacy (omitted from plist by makeActionParams)
				// and omit the named argument in V2 so optional parameter absence is preserved.
				legArgs = append(legArgs, "nil")
				continue
			}
			if s.isInf {
				legArgs = append(legArgs, s.legList...)
				for idxItem, vItem := range s.v2List {
					lbl := ""
					if i > 0 && idxItem == 0 && !hasDupLabel && p.name != "" {
						lbl = p.name
					}
					v2Args = append(v2Args, v2NamedArg{label: lbl, expr: vItem})
				}
			} else {
				legArgs = append(legArgs, s.legList[0])
				lbl := ""
				if i > 0 && !hasDupLabel && p.name != "" {
					lbl = p.name
				}
				v2Args = append(v2Args, v2NamedArg{label: lbl, expr: s.v2List[0]})
			}
		}
	}

	return legArgs, v2Args, true
}

func hasDuplicateParamLabels(params []parameterDefinition) bool {
	seen := make(map[string]int, len(params))
	for _, p := range params {
		if p.name != "" {
			seen[p.name]++
			if seen[p.name] > 1 {
				return true
			}
		}
	}
	return false
}

func (ctx *semanticImportContext) renderParameterValuePair(p parameterDefinition, rawVal any) (semanticExprPair, bool) {
	// Check attachment / token structures first across all types (Variable, Text, Integer, etc.)
	if m, ok := rawVal.(map[string]any); ok {
		ser, _ := m["WFSerializationType"].(string)
		switch ser {
		case "WFTextTokenAttachment":
			// String/RawString parameters in Cherri serialize references as WFTextTokenString, never WFTextTokenAttachment.
			if p.validType == String || p.validType == RawString {
				return semanticExprPair{}, false
			}
			return ctx.renderSerializedAttachmentPair(m, false)
		case "WFTextTokenString":
			return ctx.renderWFTextTokenStringPair(m)
		case "WFDictionaryFieldValue":
			return ctx.renderWFDictionaryPair(m)
		case "WFQuantityFieldValue":
			if p.validType == Quantity || p.qty {
				return ctx.renderQuantityPair(m)
			}
		}
		if typ, _ := m["Type"].(string); typ != "" {
			return ctx.renderAttachmentBodyPair(m, false)
		}
	}

	switch p.validType {
	case String, RawString, Enumeration:
		switch v := rawVal.(type) {
		case string:
			return ctx.renderStringPair(v)
		default:
			if lit, ok := formatHybridScalarOrArrayLiteral(rawVal, p, false, false); ok {
				return semanticExprPair{Legacy: lit, V2: lit}, true
			}
		}
	case Integer, Float:
		return ctx.renderNumberLiteralPair(rawVal)
	case Bool, True, False:
		if b, ok := rawVal.(bool); ok {
			if b {
				return semanticExprPair{Legacy: "true", V2: "true"}, true
			}
			return semanticExprPair{Legacy: "false", V2: "false"}, true
		}
	case Arr:
		if items, ok := rawVal.([]any); ok {
			return ctx.renderArrayLiteralPair(items)
		}
	case Dict:
		if m, ok := rawVal.(map[string]any); ok {
			if ep, ok := ctx.renderWFDictionaryPair(m); ok {
				return ep, true
			}
			return ctx.renderPlainDictPair(m)
		}
	case Variable:
		switch v := rawVal.(type) {
		case string:
			return ctx.renderStringPair(v)
		case bool:
			if v {
				return semanticExprPair{Legacy: "true", V2: "true"}, true
			}
			return semanticExprPair{Legacy: "false", V2: "false"}, true
		case int, int64, uint64, float64:
			return ctx.renderNumberLiteralPair(v)
		case []any:
			return ctx.renderArrayLiteralPair(v)
		case map[string]any:
			return ctx.renderPlainDictPair(v)
		}
	}

	legLit, okL := formatHybridScalarOrArrayLiteral(rawVal, p, true, true)
	v2Lit, okV := formatHybridScalarOrArrayLiteral(rawVal, p, false, false)
	if okL && okV {
		return semanticExprPair{Legacy: legLit, V2: v2Lit}, true
	}
	return semanticExprPair{}, false
}

func (ctx *semanticImportContext) renderStringPair(s string) (semanticExprPair, bool) {
	if plistValueHasXMLUnsafeStringChar(s) {
		return semanticExprPair{}, false
	}
	// If string contains braces in Legacy, Cherri parses {x} inside "..." as interpolation unless single-quoted!
	v2Esc := formatCherriDoubleQuotedString(s)
	legEsc := v2Esc
	if strings.Contains(s, "{") {
		if !strings.ContainsAny(s, "'\n\r\t\\") {
			legEsc = "'" + s + "'"
		} else {
			return semanticExprPair{}, false
		}
	}
	return semanticExprPair{Legacy: legEsc, V2: v2Esc}, true
}

func (ctx *semanticImportContext) renderNumberLiteralPair(v any) (semanticExprPair, bool) {
	switch n := v.(type) {
	case int:
		s := strconv.Itoa(n)
		return semanticExprPair{Legacy: s, V2: s}, true
	case int64:
		s := strconv.FormatInt(n, 10)
		return semanticExprPair{Legacy: s, V2: s}, true
	case uint64:
		s := strconv.FormatUint(n, 10)
		return semanticExprPair{Legacy: s, V2: s}, true
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return semanticExprPair{}, false
		}
		s := strconv.FormatFloat(n, 'g', -1, 64)
		// Preserve float type distinction in parsers where decimal point forces float.
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return semanticExprPair{Legacy: s, V2: s}, true
	default:
		return semanticExprPair{}, false
	}
}

func (ctx *semanticImportContext) renderNumberOrAttachmentPair(v any) (semanticExprPair, bool) {
	if ep, ok := ctx.renderNumberLiteralPair(v); ok {
		return ep, true
	}
	if m, ok := v.(map[string]any); ok {
		ser, _ := m["WFSerializationType"].(string)
		if ser == "WFTextTokenAttachment" {
			return ctx.renderSerializedAttachmentPair(m, false)
		}
		if typ, _ := m["Type"].(string); typ != "" {
			return ctx.renderAttachmentBodyPair(m, false)
		}
	}
	return semanticExprPair{}, false
}

func (ctx *semanticImportContext) renderAnyTextPair(v any, allowEmpty bool) (semanticExprPair, bool) {
	switch x := v.(type) {
	case string:
		if !allowEmpty && x == "" {
			return semanticExprPair{}, false
		}
		return ctx.renderStringPair(x)
	case map[string]any:
		if ser, _ := x["WFSerializationType"].(string); ser == "WFTextTokenString" {
			return ctx.renderWFTextTokenStringPair(x)
		}
		if ser, _ := x["WFSerializationType"].(string); ser == "WFTextTokenAttachment" {
			return ctx.renderSerializedAttachmentPair(x, false)
		}
	}
	return semanticExprPair{}, false
}

func (ctx *semanticImportContext) renderSerializedAttachmentPair(m map[string]any, insideAssign bool) (semanticExprPair, bool) {
	valMap, ok := m["Value"].(map[string]any)
	if !ok {
		return semanticExprPair{}, false
	}
	return ctx.renderAttachmentBodyPair(valMap, insideAssign)
}

func (ctx *semanticImportContext) renderAttachmentBodyPair(att map[string]any, insideAssign bool) (semanticExprPair, bool) {
	typ, _ := att["Type"].(string)
	if typ == "" {
		return semanticExprPair{}, false
	}

	var legBase, v2Base string
	switch typ {
	case "ActionOutput":
		u, _ := att["OutputUUID"].(string)
		sym, ok := ctx.UUIDToSymbol[u]
		if !ok || sym == "" {
			return semanticExprPair{}, false
		}
		legBase = sym
		v2Base = sym
	case "Variable":
		vn, _ := att["VariableName"].(string)
		if vn == "" {
			vn, _ = att["OutputName"].(string)
		}
		// Handle repeat built-in names transparently when referenced as Variable
		if isBuiltinLoopVarName(vn) {
			b := builtinLoopIdentFromPlistName(vn)
			legBase = b
			v2Base = b
		} else {
			sym, ok := ctx.VarToSymbol[vn]
			if !ok || sym == "" {
				return semanticExprPair{}, false
			}
			legBase = "@" + sym
			v2Base = sym
		}
	case "ExtensionInput":
		legBase = "ShortcutInput"
		v2Base = "ShortcutInput"
	case "CurrentDate":
		legBase = "CurrentDate"
		v2Base = "CurrentDate"
	case "Clipboard":
		legBase = "Clipboard"
		v2Base = "Clipboard"
	case "Ask":
		if p, _ := att["Prompt"].(string); p != "" {
			return semanticExprPair{}, false
		}
		legBase = "Ask"
		v2Base = "Ask"
	case "DeviceDetails":
		legBase = "Device"
		v2Base = "Device"
	default:
		return semanticExprPair{}, false
	}

	rawAggrs, hasAggrs := att["Aggrandizements"]
	if hasAggrs {
		agList, ok := rawAggrs.([]any)
		if ok && len(agList) > 0 {
			legRef, v2Ref, ok := ctx.applyAggrandizementSuffixes(legBase, v2Base, agList, typ == "DeviceDetails")
			if !ok {
				return semanticExprPair{}, false
			}
			_ = insideAssign
			return semanticExprPair{Legacy: legRef, V2: v2Ref}, true
		}
	}

	return semanticExprPair{Legacy: legBase, V2: v2Base}, true
}

func isBuiltinLoopVarName(plistName string) bool {
	return builtinLoopIdentFromPlistName(plistName) != ""
}

func builtinLoopIdentFromPlistName(plistName string) string {
	switch plistName {
	case "Repeat Item":
		return "RepeatItem"
	case "Repeat Index":
		return "RepeatIndex"
	}
	if strings.HasPrefix(plistName, "Repeat Item ") {
		n := strings.TrimPrefix(plistName, "Repeat Item ")
		if iv, err := strconv.Atoi(n); err == nil && iv >= 2 && iv <= 10 {
			return fmt.Sprintf("RepeatItem%d", iv)
		}
	}
	if strings.HasPrefix(plistName, "Repeat Index ") {
		n := strings.TrimPrefix(plistName, "Repeat Index ")
		if iv, err := strconv.Atoi(n); err == nil && iv >= 2 && iv <= 10 {
			return fmt.Sprintf("RepeatIndex%d", iv)
		}
	}
	return ""
}

func (ctx *semanticImportContext) applyAggrandizementSuffixes(legBase, v2Base string, agList []any, isDevice bool) (string, string, bool) {
	// Convert to []Aggrandizement to reuse decompile.go's decompAggrandizements for Legacy,
	// and pair with V2's MemberExpr/IndexExpr syntax!
	// Legacy decompAggrandizements/makeAttachmentValues only emit at most one getAs/Prop followed
	// by at most one Coercion, so skip multi-element chains whose order diverges.
	if len(agList) > 2 {
		return "", "", false
	}
	sawIndexOrProp := false
	sawCoerce := false
	typed := make([]Aggrandizement, 0, len(agList))
	v2Expr := v2Base
	for _, rawA := range agList {
		m, ok := rawA.(map[string]any)
		if !ok {
			return "", "", false
		}
		typ, _ := m["Type"].(string)
		ag := Aggrandizement{
			Type:              typ,
			CoercionItemClass: strVal(m["CoercionItemClass"]),
			DictionaryKey:     strVal(m["DictionaryKey"]),
			PropertyName:      strVal(m["PropertyName"]),
			PropertyUserInfo:  m["PropertyUserInfo"],
		}
		if ag.PropertyUserInfo != nil {
			// Complex PropertyUserInfo is outside plain dot/bracket aggrandizements.
			return "", "", false
		}
		typed = append(typed, ag)

		switch typ {
		case "WFDictionaryValueVariableAggrandizement":
			allowCoerceFirstDict := sawCoerce && len(typed) == 2 &&
				typed[0].Type == "WFCoercionVariableAggrandizement" &&
				typed[0].CoercionItemClass == "WFDictionaryContentItem"
			if ag.DictionaryKey == "" || isDevice || sawIndexOrProp || (sawCoerce && !allowCoerceFirstDict) {
				return "", "", false
			}
			sawIndexOrProp = true
			v2Expr = fmt.Sprintf("%s[%s]", v2Expr, formatCherriDoubleQuotedString(ag.DictionaryKey))
		case "WFCoercionVariableAggrandizement":
			if sawCoerce {
				return "", "", false
			}
			sawCoerce = true
			alias, found := coercionAliasForClass(ag.CoercionItemClass)
			if !found {
				return "", "", false
			}
			v2Expr = fmt.Sprintf("%s.%s", v2Expr, alias)
		case "WFPropertyVariableAggrandizement":
			if ag.PropertyName == "" || sawIndexOrProp || sawCoerce {
				return "", "", false
			}
			sawIndexOrProp = true
			if isDevice {
				v2Expr = fmt.Sprintf("%s[%s]", v2Expr, formatCherriDoubleQuotedString(ag.PropertyName))
			} else if isSimpleIdent(ag.PropertyName) && !contentItemClassesLower()[strings.ToLower(ag.PropertyName)] {
				v2Expr = fmt.Sprintf("%s.%s", v2Expr, ag.PropertyName)
			} else {
				return "", "", false
			}
		default:
			return "", "", false
		}
	}

	var legRef string
	if len(typed) == 2 && !isDevice &&
		typed[0].Type == "WFCoercionVariableAggrandizement" && typed[0].CoercionItemClass == "WFDictionaryContentItem" &&
		typed[1].Type == "WFDictionaryValueVariableAggrandizement" && typed[1].DictionaryKey != "" &&
		!strings.ContainsAny(typed[1].DictionaryKey, "'\"\r\n") {
		legRef = fmt.Sprintf("%s.dictionary['%s']", legBase, typed[1].DictionaryKey)
	} else {
		legRef = legBase
		decompAggrandizements(&legRef, typed)
	}
	if legRef == "" || v2Expr == "" {
		return "", "", false
	}
	return legRef, v2Expr, true
}

func unwrapWFListItem(it any) (any, bool) {
	if s, ok := it.(string); ok {
		return s, true
	}
	m, ok := it.(map[string]any)
	if !ok || len(m) != 2 {
		return nil, false
	}
	itType, okT := provInt(m["WFItemType"])
	val, hasVal := m["WFValue"]
	if !okT || itType != 0 || !hasVal {
		return nil, false
	}
	return val, true
}

func (ctx *semanticImportContext) reverseTextPartsParam(rec NativeActionRecord, callableName string, p parameterDefinition) (semanticExprPair, bool, bool) {
	if (callableName != "splitText" && callableName != "joinText") || (p.name != "separator" && p.name != "glue") {
		return semanticExprPair{}, false, false
	}
	if show, _ := rec.Parameters["Show-text"].(bool); !show {
		return semanticExprPair{}, true, false
	}
	sepMode, _ := rec.Parameters["WFTextSeparator"].(string)
	switch sepMode {
	case "":
		if _, hasCustom := rec.Parameters["WFTextCustomSeparator"]; hasCustom {
			return semanticExprPair{}, true, false
		}
		return semanticExprPair{}, true, true
	case "New Lines":
		if _, hasCustom := rec.Parameters["WFTextCustomSeparator"]; hasCustom {
			return semanticExprPair{}, true, false
		}
		return semanticExprPair{Legacy: `"\n"`, V2: `"\n"`}, true, true
	case "Spaces":
		if _, hasCustom := rec.Parameters["WFTextCustomSeparator"]; hasCustom {
			return semanticExprPair{}, true, false
		}
		return semanticExprPair{Legacy: `" "`, V2: `" "`}, true, true
	case "Every Character":
		if callableName != "splitText" {
			return semanticExprPair{}, true, false
		}
		if _, hasCustom := rec.Parameters["WFTextCustomSeparator"]; hasCustom {
			return semanticExprPair{}, true, false
		}
		return semanticExprPair{Legacy: `""`, V2: `""`}, true, true
	case "Custom":
		rawCustom, hasCustom := rec.Parameters["WFTextCustomSeparator"]
		if !hasCustom {
			return semanticExprPair{}, true, false
		}
		ep, ok := ctx.renderAnyTextPair(rawCustom, true)
		return ep, true, ok
	default:
		return semanticExprPair{}, true, false
	}
}

func coercionAliasForClass(class string) (string, bool) {
	// Deterministic sorted scan over contentTypes/classes matching decompile.go.
	classMap := map[string]string{
		"app": "WFAppStoreAppContentItem", "article": "WFArticleContentItem",
		"bool": "WFBooleanContentItem", "boolean": "WFBooleanContentItem",
		"contact": "WFContactContentItem", "currency": "WFCurrencyAmountContentItem",
		"date": "WFDateContentItem", "dictionary": "WFDictionaryContentItem",
		"email": "WFEmailAddressContentItem", "file": "WFGenericFileContentItem",
		"folder": "WFFolderContentItem", "image": "WFImageContentItem",
		"location": "WFLocationContentItem", "media": "WFMediaItemContentItem",
		"number": "WFNumberContentItem", "pdf": "WFPDFContentItem",
		"phone": "WFPhoneNumberContentItem", "photo": "WFPhotoMediaContentItem",
		"richtext": "WFRichTextContentItem", "webpage": "WFSafariWebPageContentItem",
		"text": "WFStringContentItem", "url": "WFURLContentItem",
		"vcard": "WFVCardContentItem",
	}
	keys := make([]string, 0, len(classMap))
	for k := range classMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if classMap[k] == class {
			return k, true
		}
	}
	return "", false
}

func isSimpleIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if !(unicode.IsLetter(r) || r == '_' || (i > 0 && unicode.IsDigit(r))) {
			return false
		}
	}
	return true
}

func strVal(v any) string {
	s, _ := v.(string)
	return s
}

func (ctx *semanticImportContext) renderWFTextTokenStringPair(m map[string]any) (semanticExprPair, bool) {
	valMap, ok := m["Value"].(map[string]any)
	if !ok {
		return semanticExprPair{}, false
	}
	str, _ := valMap["string"].(string)
	if plistValueHasXMLUnsafeStringChar(str) {
		return semanticExprPair{}, false
	}
	rawAtts, hasAtts := valMap["attachmentsByRange"].(map[string]any)
	if !hasAtts || len(rawAtts) == 0 {
		// Uninterpolated WFTextTokenString: check if natural string produces WFTextTokenString cleanly when checked.
		return ctx.renderStringPair(str)
	}

	type rangeSpan struct {
		startRune int
		endRune   int
		att       semanticExprPair
	}
	spans := make([]rangeSpan, 0, len(rawAtts))
	for rngStr, rawA := range rawAtts {
		var u16Start, u16Len int
		if _, err := fmt.Sscanf(rngStr, "{%d, %d}", &u16Start, &u16Len); err != nil || u16Len <= 0 || u16Start < 0 {
			return semanticExprPair{}, false
		}
		attMap, ok := rawA.(map[string]any)
		if !ok {
			return semanticExprPair{}, false
		}
		// Reuse decompile.go's utf16OffsetToRuneIndex for exact surrogate/emoji alignment!
		sr := utf16OffsetToRuneIndex(str, u16Start)
		er := utf16OffsetToRuneIndex(str, u16Start+u16Len)
		if sr < 0 || er < sr {
			return semanticExprPair{}, false
		}
		ep, ok := ctx.renderAttachmentBodyPair(attMap, false)
		if !ok {
			return semanticExprPair{}, false
		}
		spans = append(spans, rangeSpan{startRune: sr, endRune: er, att: ep})
	}
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].startRune < spans[j].startRune
	})

	runes := []rune(str)
	var legSB, v2SB strings.Builder
	legSB.WriteByte('"')
	v2SB.WriteString(`f"`)

	cursor := 0
	for _, sp := range spans {
		if sp.startRune < cursor || sp.endRune > len(runes) {
			return semanticExprPair{}, false
		}
		litChunk := string(runes[cursor:sp.startRune])
		if strings.Contains(litChunk, "{") {
			// Legacy Cherri cannot safely distinguish literal '{' inside double-quoted interpolation string.
			return semanticExprPair{}, false
		}
		writeEscapedDualLiteralChunk(&legSB, litChunk, false)
		writeEscapedDualLiteralChunk(&v2SB, litChunk, true)

		legSB.WriteByte('{')
		legSB.WriteString(sp.att.Legacy)
		legSB.WriteByte('}')

		v2SB.WriteByte('{')
		v2SB.WriteString(sp.att.V2)
		v2SB.WriteByte('}')

		cursor = sp.endRune
	}
	if cursor < len(runes) {
		tail := string(runes[cursor:])
		if strings.Contains(tail, "{") {
			return semanticExprPair{}, false
		}
		writeEscapedDualLiteralChunk(&legSB, tail, false)
		writeEscapedDualLiteralChunk(&v2SB, tail, true)
	}
	legSB.WriteByte('"')
	v2SB.WriteByte('"')
	return semanticExprPair{Legacy: legSB.String(), V2: v2SB.String()}, true
}

func writeEscapedDualLiteralChunk(sb *strings.Builder, chunk string, isV2FString bool) {
	for _, r := range chunk {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '{':
			if isV2FString {
				sb.WriteString(`{{`)
			} else {
				sb.WriteRune(r)
			}
		case '}':
			if isV2FString {
				sb.WriteString(`}}`)
			} else {
				sb.WriteRune(r)
			}
		default:
			sb.WriteRune(r)
		}
	}
}

func (ctx *semanticImportContext) renderArrayLiteralPair(items []any) (semanticExprPair, bool) {
	var legParts, v2Parts []string
	for _, it := range items {
		ep, ok := ctx.renderVariableInputPair(it)
		if !ok {
			return semanticExprPair{}, false
		}
		legParts = append(legParts, ep.Legacy)
		v2Parts = append(v2Parts, ep.V2)
	}
	return semanticExprPair{
		Legacy: "[" + strings.Join(legParts, ", ") + "]",
		V2:     "[" + strings.Join(v2Parts, ", ") + "]",
	}, true
}

func (ctx *semanticImportContext) renderPlainDictPair(m map[string]any) (semanticExprPair, bool) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var legEntries, v2Entries []string
	for _, k := range keys {
		kExpr, ok := ctx.renderStringPair(k)
		if !ok {
			return semanticExprPair{}, false
		}
		vExpr, ok := ctx.renderVariableInputPair(m[k])
		if !ok {
			return semanticExprPair{}, false
		}
		legEntries = append(legEntries, fmt.Sprintf("%s: %s", kExpr.Legacy, vExpr.Legacy))
		v2Entries = append(v2Entries, fmt.Sprintf("%s: %s", kExpr.V2, vExpr.V2))
	}
	return semanticExprPair{
		Legacy: "{" + strings.Join(legEntries, ", ") + "}",
		V2:     "{" + strings.Join(v2Entries, ", ") + "}",
	}, true
}

func (ctx *semanticImportContext) renderWFDictionaryPair(m map[string]any) (semanticExprPair, bool) {
	return ctx.renderWFDictionaryPairMode(m, false)
}

func (ctx *semanticImportContext) renderStrictSortedDictionaryPair(m map[string]any) (semanticExprPair, bool) {
	return ctx.renderWFDictionaryPairMode(m, true)
}

func (ctx *semanticImportContext) renderWFDictionaryPairMode(m map[string]any, requireSortedUniqueKeys bool) (semanticExprPair, bool) {
	valWrap, _ := m["Value"].(map[string]any)
	if valWrap == nil {
		return semanticExprPair{}, false
	}
	rawItems, _ := valWrap["WFDictionaryFieldValueItems"].([]any)
	var legEntries, v2Entries []string
	var prevKey string
	for idxItem, rawIt := range rawItems {
		itMap, ok := rawIt.(map[string]any)
		if !ok {
			return semanticExprPair{}, false
		}
		itemType, okT := provInt(itMap["WFItemType"])
		if !okT {
			return semanticExprPair{}, false
		}
		keyStr, keyPair, okK := ctx.unwrapWFDictionaryKey(itMap["WFKey"])
		if !okK {
			return semanticExprPair{}, false
		}
		if requireSortedUniqueKeys {
			if keyStr == "" && !keyIsEmptyPlainToken(itMap["WFKey"]) {
				return semanticExprPair{}, false
			}
			if idxItem > 0 && keyStr <= prevKey {
				return semanticExprPair{}, false
			}
			prevKey = keyStr
		}
		valPair, okV := ctx.unwrapWFDictionaryTypedValue(itemType, itMap["WFValue"])
		if !okV {
			return semanticExprPair{}, false
		}
		legEntries = append(legEntries, fmt.Sprintf("%s: %s", keyPair.Legacy, valPair.Legacy))
		v2Entries = append(v2Entries, fmt.Sprintf("%s: %s", keyPair.V2, valPair.V2))
	}
	return semanticExprPair{
		Legacy: "{" + strings.Join(legEntries, ", ") + "}",
		V2:     "{" + strings.Join(v2Entries, ", ") + "}",
	}, true
}

func keyIsEmptyPlainToken(raw any) bool {
	m, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	if ser, _ := m["WFSerializationType"].(string); ser != "WFTextTokenString" {
		return false
	}
	v, _ := m["Value"].(map[string]any)
	if v == nil {
		return false
	}
	if atts, okA := v["attachmentsByRange"].(map[string]any); okA && len(atts) > 0 {
		return false
	}
	s, okS := v["string"].(string)
	return okS && s == ""
}

func (ctx *semanticImportContext) unwrapWFDictionaryKey(raw any) (string, semanticExprPair, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		ep, okE := ctx.renderAnyTextPair(raw, true)
		return "", ep, okE
	}
	ser, _ := m["WFSerializationType"].(string)
	if ser == "WFTextTokenString" {
		v, _ := m["Value"].(map[string]any)
		if v == nil {
			return "", semanticExprPair{}, false
		}
		if att, hasAtt := v["attachmentsByRange"].(map[string]any); !hasAtt || len(att) == 0 {
			if s, okS := v["string"].(string); okS {
				ep, okE := ctx.renderStringPair(s)
				return s, ep, okE
			}
		}
		ep, okE := ctx.renderWFTextTokenStringPair(m)
		return "", ep, okE
	}
	if s, okS := m["Value"].(string); okS {
		ep, okE := ctx.renderStringPair(s)
		return s, ep, okE
	}
	ep, okE := ctx.renderAnyTextPair(m, true)
	return "", ep, okE
}

func (ctx *semanticImportContext) unwrapWFDictionaryTypedValue(itemType int, raw any) (semanticExprPair, bool) {
	m, isMap := raw.(map[string]any)
	switch itemType {
	case 0: // Text
		if isMap {
			if ser, _ := m["WFSerializationType"].(string); ser == "WFTextTokenString" {
				return ctx.renderWFTextTokenStringPair(m)
			}
			if ser, _ := m["WFSerializationType"].(string); ser == "WFTextTokenAttachment" {
				if valMap, ok := m["Value"].(map[string]any); ok {
					return ctx.renderAttachmentBodyPair(valMap, false)
				}
			}
		}
		if s, ok := raw.(string); ok {
			return ctx.renderStringPair(s)
		}
		return semanticExprPair{}, false
	case 3: // Number
		if isMap {
			if ser, _ := m["WFSerializationType"].(string); ser == "WFTextTokenString" {
				valMap, _ := m["Value"].(map[string]any)
				if valMap != nil {
					if att, hasAtt := valMap["attachmentsByRange"].(map[string]any); !hasAtt || len(att) == 0 {
						if numStr, ok := valMap["string"].(string); ok {
							if iv, err := strconv.ParseInt(numStr, 10, 64); err == nil && strconv.FormatInt(iv, 10) == numStr {
								return semanticExprPair{Legacy: numStr, V2: numStr}, true
							}
							if fv, err := strconv.ParseFloat(numStr, 64); err == nil && !math.IsNaN(fv) && !math.IsInf(fv, 0) && fmt.Sprintf("%v", fv) == numStr && strings.ContainsAny(numStr, ".eE") {
								return semanticExprPair{Legacy: numStr, V2: numStr}, true
							}
						}
					}
				}
			}
			if ser, _ := m["WFSerializationType"].(string); ser == "WFTextTokenAttachment" {
				return ctx.renderSerializedAttachmentPair(m, false)
			}
		}
		return ctx.renderNumberLiteralPair(raw)
	case 4: // Bool
		if isMap {
			if ser, _ := m["WFSerializationType"].(string); ser == "WFNumberSubstitutableState" {
				if b, ok := m["Value"].(bool); ok {
					if b {
						return semanticExprPair{Legacy: "true", V2: "true"}, true
					}
					return semanticExprPair{Legacy: "false", V2: "false"}, true
				}
			}
		}
		if b, ok := raw.(bool); ok {
			if b {
				return semanticExprPair{Legacy: "true", V2: "true"}, true
			}
			return semanticExprPair{Legacy: "false", V2: "false"}, true
		}
		return semanticExprPair{}, false
	case 2: // Array
		if isMap {
			if ser, _ := m["WFSerializationType"].(string); ser == "WFArrayParameterState" {
				elems, ok := m["Value"].([]any)
				if !ok {
					return semanticExprPair{}, false
				}
				var legParts, v2Parts []string
				for _, el := range elems {
					em, okM := el.(map[string]any)
					if !okM {
						return semanticExprPair{}, false
					}
					et, okET := provInt(em["WFItemType"])
					if !okET || et == 1 {
						return semanticExprPair{}, false
					}
					ep, okEV := ctx.unwrapWFDictionaryTypedValue(et, em["WFValue"])
					if !okEV {
						return semanticExprPair{}, false
					}
					legParts = append(legParts, ep.Legacy)
					v2Parts = append(v2Parts, ep.V2)
				}
				return semanticExprPair{
					Legacy: "[" + strings.Join(legParts, ", ") + "]",
					V2:     "[" + strings.Join(v2Parts, ", ") + "]",
				}, true
			}
		}
		return semanticExprPair{}, false
	default:
		// Skip itemType 1 (nested dict) where Legacy double-wraps WFItems differently than V2.
		return semanticExprPair{}, false
	}
}

func (ctx *semanticImportContext) renderQuantityPair(m map[string]any) (semanticExprPair, bool) {
	valMap, ok := m["Value"].(map[string]any)
	if !ok {
		return semanticExprPair{}, false
	}
	unit, okU := valMap["Unit"].(string)
	mag, hasMag := valMap["Magnitude"]
	if !okU || !hasMag || unit == "" {
		return semanticExprPair{}, false
	}
	magLit, okM := ctx.renderNumberOrAttachmentPair(mag)
	if !okM {
		return semanticExprPair{}, false
	}
	// Measurement literal syntax qty(mag, "unit") or mag "unit"? Verify carefully via compiler if used.
	_ = magLit
	return semanticExprPair{}, false
}

// ---------- Control-flow region synthesis (paired Legacy + V2 from one AST construct) ----------

func (ctx *semanticImportContext) trySynthesizeRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	switch reg.Kind {
	case "conditional":
		ctx.trySynthesizeConditionalRegion(reg, plan)
	case "choosefrommenu":
		ctx.trySynthesizeMenuRegion(reg, plan)
	case "repeat.count":
		ctx.trySynthesizeRepeatCountRegion(reg, plan)
	case "repeat.each":
		ctx.trySynthesizeRepeatEachRegion(reg, plan)
	}
}

func (ctx *semanticImportContext) trySynthesizeConditionalRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	startRec := ctx.Doc.Actions[reg.Start]
	endRec := ctx.Doc.Actions[reg.End]
	if !startRec.HadParametersKey || !endRec.HadParametersKey {
		return
	}
	if len(reg.Mids) > 1 {
		return
	}
	if endRec.UUID != "" && ctx.UsedProducerUUIDs[endRec.UUID] {
		return
	}
	allowedStart := map[string]bool{
		"GroupingIdentifier": true, "WFControlFlowMode": true,
		"WFCondition": true, "WFInput": true,
		"WFConditionalActionString": true, "WFNumberValue": true, "WFAnotherNumber": true,
		"WFDate": true, "WFAnotherDate": true, "WFConditionalLegacyComparisonBehavior": true,
	}
	if !ctx.hasOnlyAllowedExtraKeys(startRec.Parameters, allowedStart) {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(endRec.Parameters, map[string]bool{"GroupingIdentifier": true, "WFControlFlowMode": true}) {
		return
	}
	if len(reg.Mids) == 1 {
		midRec := ctx.Doc.Actions[reg.Mids[0]]
		if !midRec.HadParametersKey || !ctx.hasOnlyAllowedExtraKeys(midRec.Parameters, map[string]bool{"GroupingIdentifier": true, "WFControlFlowMode": true}) {
			return
		}
	}

	condPair, ok := ctx.renderSingleConditionPair(startRec.Parameters)
	if !ok {
		return
	}

	// Set atomic RegionGroupID decisions across start / else / end.
	plan.Decisions[reg.Start].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.Start], condPair.Legacy, condPair.V2, "conditional", "")

	if len(reg.Mids) == 1 {
		mIdx := reg.Mids[0]
		plan.Decisions[mIdx].RegionGroupID = reg.GroupID
		applyDecisionCandidate(&plan.Decisions[mIdx], "} else {", "} else {", "conditional", "")
	}

	plan.Decisions[reg.End].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.End], "}", "}", "conditional", "")
}

func (ctx *semanticImportContext) renderSingleConditionPair(params map[string]any) (semanticExprPair, bool) {
	condCode, okC := provInt(params["WFCondition"])
	if !okC {
		return semanticExprPair{}, false
	}

	rawInput, hasIn := params["WFInput"]
	if !hasIn {
		return semanticExprPair{}, false
	}
	inMap, okIn := rawInput.(map[string]any)
	if !okIn {
		return semanticExprPair{}, false
	}
	inVarRaw, hasVar := inMap["Variable"]
	if !hasVar {
		return semanticExprPair{}, false
	}
	inExpr, okE := ctx.renderVariableInputPair(inVarRaw)
	if !okE {
		return semanticExprPair{}, false
	}

	// Track WFConditionalLegacyComparisonBehavior presence accurately across dialects:
	// Legacy `if legacy ...` emits WFConditionalLegacyComparisonBehavior:true, plain `if ...` omits it.
	// V2 `if ...` emits WFConditionalLegacyComparisonBehavior:true unconditionally in emitRaw.
	hasLegacyFlag := params["WFConditionalLegacyComparisonBehavior"] == true
	if !hasLegacyFlag {
		// V2 unconditionally emits WFConditionalLegacyComparisonBehavior: true for mode 0; do not misemit.
		return semanticExprPair{}, false
	}
	legPrefix := "if legacy "
	v2Prefix := "if "

	// Unary / HasValue / Empty conditions (100 = has any value, 101 = does not have any value, 1002 = isToday)
	switch condCode {
	case 100:
		return semanticExprPair{
			Legacy: fmt.Sprintf("%s%s {", legPrefix, inExpr.Legacy),
			V2:     fmt.Sprintf("%s%s {", v2Prefix, inExpr.V2),
		}, true
	case 101:
		return semanticExprPair{
			Legacy: fmt.Sprintf("%s!%s {", legPrefix, inExpr.Legacy),
			V2:     fmt.Sprintf("%s!%s {", v2Prefix, inExpr.V2),
		}, true
	case 1002:
		return semanticExprPair{
			Legacy: fmt.Sprintf("%sisToday(%s) {", legPrefix, inExpr.Legacy),
			V2:     fmt.Sprintf("%sisToday(%s) {", v2Prefix, inExpr.V2),
		}, true
	}

	// Binary comparisons reuse Cherri operator table aligned with decompile.go:matchConditionOperator
	op := ""
	switch condCode {
	case 4:
		op = "=="
	case 5:
		op = "!="
	case 2:
		op = ">"
	case 3:
		op = ">="
	case 0:
		op = "<"
	case 1:
		op = "<="
	case 99:
		op = "contains"
	case 999:
		// `!contains`: check unary bang on contains in V2 and `!contains` in Legacy
		if cmpStr, hasS := params["WFConditionalActionString"]; hasS {
			rhs, ok := ctx.renderAnyTextPair(cmpStr, true)
			if !ok {
				return semanticExprPair{}, false
			}
			return semanticExprPair{
				Legacy: fmt.Sprintf("%s%s !contains %s {", legPrefix, inExpr.Legacy, rhs.Legacy),
				V2:     fmt.Sprintf("%s!(%s contains %s) {", v2Prefix, inExpr.V2, rhs.V2),
			}, true
		}
		return semanticExprPair{}, false
	case 8:
		op = "beginsWith"
	case 9:
		op = "endsWith"
	default:
		return semanticExprPair{}, false
	}

	var rhsPair semanticExprPair
	if strCmp, hasS := params["WFConditionalActionString"]; hasS {
		rp, ok := ctx.renderAnyTextPair(strCmp, true)
		if !ok {
			return semanticExprPair{}, false
		}
		rhsPair = rp
	} else if numCmp, hasN := params["WFNumberValue"]; hasN {
		rp, ok := ctx.renderNumberOrAttachmentPair(numCmp)
		if !ok {
			return semanticExprPair{}, false
		}
		rhsPair = rp
	} else {
		return semanticExprPair{}, false
	}

	return semanticExprPair{
		Legacy: fmt.Sprintf("%s%s %s %s {", legPrefix, inExpr.Legacy, op, rhsPair.Legacy),
		V2:     fmt.Sprintf("%s%s %s %s {", v2Prefix, inExpr.V2, op, rhsPair.V2),
	}, true
}

func (ctx *semanticImportContext) trySynthesizeMenuRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	startRec := ctx.Doc.Actions[reg.Start]
	endRec := ctx.Doc.Actions[reg.End]
	if !startRec.HadParametersKey || !endRec.HadParametersKey {
		return
	}
	if endRec.UUID != "" && ctx.UsedProducerUUIDs[endRec.UUID] {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(startRec.Parameters, map[string]bool{
		"GroupingIdentifier": true, "WFControlFlowMode": true, "WFMenuPrompt": true, "WFMenuItems": true,
	}) {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(endRec.Parameters, map[string]bool{"GroupingIdentifier": true, "WFControlFlowMode": true}) {
		return
	}

	rawPrompt, hasPrompt := startRec.Parameters["WFMenuPrompt"]
	var promptPair semanticExprPair
	if !hasPrompt {
		promptPair = semanticExprPair{Legacy: `""`, V2: `""`}
	} else {
		ep, ok := ctx.renderAnyTextPair(rawPrompt, false)
		if !ok {
			return
		}
		promptPair = ep
	}

	// Verify WFMenuItems matches the Mode 1 cases 1-to-1 and no extra WFMenuItemAttributedTitle keys alter wire shape.
	rawItems, hasItems := startRec.Parameters["WFMenuItems"].([]any)
	if !hasItems || len(rawItems) != len(reg.Mids) || len(reg.Mids) == 0 {
		return
	}
	caseTitles := make([]semanticExprPair, len(reg.Mids))
	for ci, mIdx := range reg.Mids {
		mRec := ctx.Doc.Actions[mIdx]
		if !mRec.HadParametersKey || !ctx.hasOnlyAllowedExtraKeys(mRec.Parameters, map[string]bool{
			"GroupingIdentifier": true, "WFControlFlowMode": true, "WFMenuItemTitle": true, "WFMenuItemAttributedTitle": true,
		}) {
			return
		}
		title, ok := mRec.Parameters["WFMenuItemTitle"].(string)
		if !ok || title == "" {
			return
		}
		attr, hasAttr := mRec.Parameters["WFMenuItemAttributedTitle"]
		if hasAttr && attr != title {
			return
		}
		itemMap, okIM := rawItems[ci].(map[string]any)
		if !okIM {
			return
		}
		itType, okT := provInt(itemMap["WFItemType"])
		if !okT || itType != 0 {
			return
		}
		if itemMap["WFValue"] != title {
			return
		}
		tp, okTP := ctx.renderStringPair(title)
		if !okTP {
			return
		}
		caseTitles[ci] = tp
	}

	v2Header := fmt.Sprintf("menu (%s) {", promptPair.V2)
	if strings.HasPrefix(promptPair.V2, `"`) {
		v2Header = fmt.Sprintf("menu %s {", promptPair.V2)
	}
	legHeader := fmt.Sprintf("menu %s {", promptPair.Legacy)

	plan.Decisions[reg.Start].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.Start], legHeader, v2Header, "menu", "")

	for ci, mIdx := range reg.Mids {
		plan.Decisions[mIdx].RegionGroupID = reg.GroupID
		applyDecisionCandidate(
			&plan.Decisions[mIdx],
			fmt.Sprintf("item %s:", caseTitles[ci].Legacy),
			fmt.Sprintf("case %s:", caseTitles[ci].V2),
			"menu", "",
		)
	}

	plan.Decisions[reg.End].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.End], "}", "}", "menu", "")
}

func (ctx *semanticImportContext) trySynthesizeRepeatCountRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	if len(reg.Mids) != 0 {
		return
	}
	startRec := ctx.Doc.Actions[reg.Start]
	endRec := ctx.Doc.Actions[reg.End]
	if !startRec.HadParametersKey || !endRec.HadParametersKey {
		return
	}
	if endRec.UUID != "" && ctx.UsedProducerUUIDs[endRec.UUID] {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(startRec.Parameters, map[string]bool{
		"GroupingIdentifier": true, "WFControlFlowMode": true, "WFRepeatCount": true,
	}) {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(endRec.Parameters, map[string]bool{"GroupingIdentifier": true, "WFControlFlowMode": true}) {
		return
	}
	cntRaw, hasCnt := startRec.Parameters["WFRepeatCount"]
	if !hasCnt {
		return
	}
	cntPair, ok := ctx.renderNumberOrAttachmentPair(cntRaw)
	if !ok {
		return
	}

	plan.Decisions[reg.Start].RegionGroupID = reg.GroupID
	applyDecisionCandidate(
		&plan.Decisions[reg.Start],
		fmt.Sprintf("repeat _ for %s {", cntPair.Legacy),
		fmt.Sprintf("repeat _ for %s {", cntPair.V2),
		"repeat", "",
	)
	plan.Decisions[reg.End].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.End], "}", "}", "repeat", "")
}

func (ctx *semanticImportContext) trySynthesizeRepeatEachRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	if len(reg.Mids) != 0 {
		return
	}
	startRec := ctx.Doc.Actions[reg.Start]
	endRec := ctx.Doc.Actions[reg.End]
	if !startRec.HadParametersKey || !endRec.HadParametersKey {
		return
	}
	if endRec.UUID != "" && ctx.UsedProducerUUIDs[endRec.UUID] {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(startRec.Parameters, map[string]bool{
		"GroupingIdentifier": true, "WFControlFlowMode": true, "WFInput": true,
	}) {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(endRec.Parameters, map[string]bool{"GroupingIdentifier": true, "WFControlFlowMode": true}) {
		return
	}
	inRaw, hasIn := startRec.Parameters["WFInput"]
	if !hasIn {
		return
	}
	inPair, ok := ctx.renderVariableInputPair(inRaw)
	if !ok {
		return
	}

	plan.Decisions[reg.Start].RegionGroupID = reg.GroupID
	applyDecisionCandidate(
		&plan.Decisions[reg.Start],
		fmt.Sprintf("for _ in %s {", inPair.Legacy),
		fmt.Sprintf("for _ in %s {", inPair.V2),
		"repeat.each", "",
	)
	plan.Decisions[reg.End].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.End], "}", "}", "repeat.each", "")
}

func (ctx *semanticImportContext) enforceAtomicRegions(plan *HybridImportPlan) {
	byGroup := make(map[string][]int)
	for i, dec := range plan.Decisions {
		if dec.RegionGroupID != "" {
			byGroup[dec.RegionGroupID] = append(byGroup[dec.RegionGroupID], i)
		}
	}
	for _, idxs := range byGroup {
		allNatural := true
		for _, idx := range idxs {
			if plan.Decisions[idx].Classification != "NATURAL_VERIFIED" {
				allNatural = false
				break
			}
		}
		if !allNatural {
			for _, idx := range idxs {
				ctx.demoteSingleIndexToFallback(plan, idx, "CONTROL_REGION")
			}
		}
	}
}

func (ctx *semanticImportContext) demoteSingleIndexToFallback(plan *HybridImportPlan, idx int, reason string) {
	var rawLeg, rawV2, defReason string
	if idx >= 0 && idx < len(ctx.fallbackLegacyLines) && ctx.fallbackLegacyLines[idx] != "" && idx < len(ctx.fallbackV2Lines) && ctx.fallbackV2Lines[idx] != "" {
		rawLeg = ctx.fallbackLegacyLines[idx]
		rawV2 = ctx.fallbackV2Lines[idx]
		defReason = ctx.fallbackReasons[idx]
	} else {
		rec := ctx.Doc.Actions[idx]
		var err1, err2 error
		rawLeg, err1 = formatRawLegacyActionCall(rec)
		rawV2, err2 = formatRawV2ActionCall(rec)
		if err1 != nil || err2 != nil {
			return
		}
		defReason = defaultFallbackReason(ctx.Doc, ctx.Registry, rec)
		if idx >= 0 && idx < len(ctx.fallbackLegacyLines) {
			ctx.fallbackLegacyLines[idx] = rawLeg
			ctx.fallbackV2Lines[idx] = rawV2
			ctx.fallbackReasons[idx] = defReason
		}
	}
	plan.Decisions[idx].Classification = "EXACT_NATIVE_FALLBACK"
	if reason != "" {
		plan.Decisions[idx].ReasonCode = reason
	} else if plan.Decisions[idx].RegionGroupID != "" {
		plan.Decisions[idx].ReasonCode = "CONTROL_REGION"
	} else {
		plan.Decisions[idx].ReasonCode = defReason
	}
	plan.Decisions[idx].ChosenDefinitionID = ""
	plan.Decisions[idx].IncludeCategory = ""
	plan.Decisions[idx].RegionGroupID = ""
	plan.Decisions[idx].LegacyCall = rawLeg
	plan.Decisions[idx].V2Call = rawV2
}

// ---------- Whole-workflow compile isolation + strict tree equality verification ----------

type semanticVerifyUnit struct {
	indices []int
	groupID string
}

func collectNaturalUnits(plan *HybridImportPlan) []semanticVerifyUnit {
	seenGroup := make(map[string]bool)
	groupIndices := make(map[string][]int)
	for i, dec := range plan.Decisions {
		if dec.Classification == "NATURAL_VERIFIED" && dec.RegionGroupID != "" {
			groupIndices[dec.RegionGroupID] = append(groupIndices[dec.RegionGroupID], i)
		}
	}
	var units []semanticVerifyUnit
	for i, dec := range plan.Decisions {
		if dec.Classification != "NATURAL_VERIFIED" {
			continue
		}
		if dec.RegionGroupID != "" {
			if seenGroup[dec.RegionGroupID] {
				continue
			}
			seenGroup[dec.RegionGroupID] = true
			units = append(units, semanticVerifyUnit{
				indices: groupIndices[dec.RegionGroupID],
				groupID: dec.RegionGroupID,
			})
		} else {
			units = append(units, semanticVerifyUnit{indices: []int{i}})
		}
	}
	return units
}

func (ctx *semanticImportContext) verifyAndStabilizePlan(plan *HybridImportPlan) error {
	// Ensure both Legacy and V2 compile cleanly without syntax/analysis errors by isolating
	// and demoting only offending units if needed.
	cachedLegMap, err := ctx.ensureDialectCompilesCleanly(plan, true)
	if err != nil {
		return err
	}
	cachedV2Map, err := ctx.ensureDialectCompilesCleanly(plan, false)
	if err != nil {
		return err
	}

	// Iteratively compare compiled output actions across BOTH Legacy and V2 and demote mismatches.
	maxIters := len(ctx.Doc.Actions) + 16
	for iter := 0; iter < maxIters; iter++ {
		var legMap map[string]any
		var errLeg error
		if cachedLegMap != nil {
			legMap = cachedLegMap
			cachedLegMap = nil
		} else {
			legSrc, err := formatHybridLegacySource(ctx.Doc, plan)
			if err != nil {
				return err
			}
			legMap, errLeg = compileHybridSource(legSrc, true)
			if errLeg != nil {
				m, bErr := ctx.ensureDialectCompilesCleanly(plan, true)
				if bErr != nil {
					return bErr
				}
				cachedLegMap = m
				continue
			}
		}

		var v2Map map[string]any
		var errV2 error
		if cachedV2Map != nil {
			v2Map = cachedV2Map
			cachedV2Map = nil
		} else {
			v2Src, err := formatHybridV2Source(ctx.Doc, plan)
			if err != nil {
				return err
			}
			v2Map, errV2 = compileHybridSource(v2Src, false)
			if errV2 != nil {
				m, bErr := ctx.ensureDialectCompilesCleanly(plan, false)
				if bErr != nil {
					return bErr
				}
				cachedV2Map = m
				continue
			}
		}

		if plistValuesStrictlyEqual(ctx.Doc.RawRoot, legMap) && plistValuesStrictlyEqual(ctx.Doc.RawRoot, v2Map) {
			recountHybridPlanStats(plan)
			return nil
		}

		changedLeg := false
		if !plistValuesStrictlyEqual(ctx.Doc.RawRoot, legMap) {
			changedLeg = demoteMismatchedHybridActions(ctx.Doc, plan, legMap)
		}
		changedV2 := false
		if !plistValuesStrictlyEqual(ctx.Doc.RawRoot, v2Map) {
			changedV2 = demoteMismatchedHybridActions(ctx.Doc, plan, v2Map)
		}
		if !changedLeg && !changedV2 {
			// Neither side found an obvious natural mismatch to demote; demote remaining natural units greedily.
			units := collectNaturalUnits(plan)
			if len(units) == 0 {
				return fmt.Errorf("hybrid import failed fail-closed parity check with zero natural actions remaining")
			}
			ctx.demoteUnit(plan, units[len(units)-1])
		}
		ctx.enforceAtomicRegions(plan)
		recountHybridPlanStats(plan)
	}
	return fmt.Errorf("hybrid verification did not converge within %d iterations", maxIters)
}

func (ctx *semanticImportContext) demoteUnit(plan *HybridImportPlan, u semanticVerifyUnit) {
	reason := ""
	if u.groupID != "" {
		reason = "CONTROL_REGION"
	}
	for _, idx := range u.indices {
		ctx.demoteSingleIndexToFallback(plan, idx, reason)
	}
}

func (ctx *semanticImportContext) compilePlanToRawMap(plan *HybridImportPlan, legacy bool) (map[string]any, bool) {
	var src string
	var err error
	if legacy {
		src, err = formatHybridLegacySource(ctx.Doc, plan)
	} else {
		src, err = formatHybridV2Source(ctx.Doc, plan)
	}
	if err != nil {
		return nil, false
	}
	m, cErr := compileHybridSource(src, legacy)
	return m, cErr == nil
}

func (ctx *semanticImportContext) testPlanCompiles(plan *HybridImportPlan, legacy bool) bool {
	_, ok := ctx.compilePlanToRawMap(plan, legacy)
	return ok
}

func (ctx *semanticImportContext) mapSourceLinesToActions(plan *HybridImportPlan, legacy bool) (string, map[int]int, error) {
	var out strings.Builder
	if legacy {
		hdr, err := encodePlistValueToLiteral(headerWithImportProvenanceWithSymbols(ctx.Doc.Header, ctx.Doc.Actions, effectiveSymbolsForPlan(plan)))
		if err != nil {
			return "", nil, err
		}
		out.WriteString("#workflowheader ")
		out.WriteString(formatCherriDoubleQuotedString(hdr))
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
	} else {
		hdr, err := encodePlistValueToLiteral(headerWithImportProvenanceWithSymbols(ctx.Doc.Header, ctx.Doc.Actions, effectiveSymbolsForPlan(plan)))
		if err != nil {
			return "", nil, err
		}
		out.WriteString("native.workflow(native.plist(")
		out.WriteString(formatCherriDoubleQuotedString(hdr))
		out.WriteString("), exact: true)\n")
	}

	lineToIdx := make(map[int]int, len(plan.Decisions)*2)
	curLine := strings.Count(out.String(), "\n") + 1
	depth := 0
	for _, dec := range plan.Decisions {
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
			lineToIdx[curLine] = dec.ActionIndex
			out.WriteString(indent)
			out.WriteString(dec.DiagnosticComment)
			out.WriteByte('\n')
			curLine++
		}
		nLines := strings.Count(call, "\n") + 1
		for k := 0; k < nLines; k++ {
			lineToIdx[curLine+k] = dec.ActionIndex
		}
		out.WriteString(indent)
		out.WriteString(call)
		out.WriteByte('\n')
		curLine += nLines
		if isControl && strings.HasSuffix(trimmed, "{") {
			depth++
		}
	}
	return out.String(), lineToIdx, nil
}

func (ctx *semanticImportContext) ensureDialectCompilesCleanly(plan *HybridImportPlan, legacy bool) (map[string]any, error) {
	alreadyFailedLatest := false
	if legacy {
		for pass := 0; pass < 32; pass++ {
			src, lineToIdx, err := ctx.mapSourceLinesToActions(plan, true)
			if err != nil {
				return nil, err
			}
			compiled, cErr := compileHybridSource(src, true)
			if cErr == nil {
				return compiled, nil
			}
			alreadyFailedLatest = true
			lineErr, ok := cErr.(legacyCompileLineError)
			if !ok {
				break
			}
			aIdx, found := lineToIdx[lineErr.line]
			if !found || aIdx < 0 || aIdx >= len(plan.Decisions) || plan.Decisions[aIdx].Classification != "NATURAL_VERIFIED" {
				break
			}
			if rg := plan.Decisions[aIdx].RegionGroupID; rg != "" {
				for i := range plan.Decisions {
					if plan.Decisions[i].RegionGroupID == rg {
						ctx.demoteSingleIndexToFallback(plan, i, "CONTROL_REGION")
					}
				}
			} else {
				ctx.demoteSingleIndexToFallback(plan, aIdx, "")
			}
			ctx.enforceAtomicRegions(plan)
			recountHybridPlanStats(plan)
			alreadyFailedLatest = false
		}
	} else {
		// Fast path for V2: batch-demote all parser/analyzer error lines in ~1-3ms per pass
		// without expensive full plist compilation or binary bisection.
		for pass := 0; pass < 6; pass++ {
			src, lineToIdx, err := ctx.mapSourceLinesToActions(plan, false)
			if err != nil {
				return nil, err
			}
			errLines, sErr := collectV2CompileErrorLines("hybrid_workflow_verify.cherri", src)
			if sErr == nil {
				break
			}
			demotedAny := false
			for _, ln := range errLines {
				if aIdx, ok := lineToIdx[ln]; ok && aIdx >= 0 && aIdx < len(plan.Decisions) {
					if plan.Decisions[aIdx].Classification == "NATURAL_VERIFIED" {
						if rg := plan.Decisions[aIdx].RegionGroupID; rg != "" {
							for i := range plan.Decisions {
								if plan.Decisions[i].RegionGroupID == rg {
									ctx.demoteSingleIndexToFallback(plan, i, "CONTROL_REGION")
								}
							}
						} else {
							ctx.demoteSingleIndexToFallback(plan, aIdx, "")
						}
						demotedAny = true
					}
				}
			}
			if !demotedAny {
				break
			}
			ctx.enforceAtomicRegions(plan)
			recountHybridPlanStats(plan)
		}
	}

	if !alreadyFailedLatest {
		if compiled, ok := ctx.compilePlanToRawMap(plan, legacy); ok {
			return compiled, nil
		}
	}
	units := collectNaturalUnits(plan)
	if len(units) == 0 {
		return nil, fmt.Errorf("lossless fallback itself failed to compile for legacy=%v", legacy)
	}

	// Work on a plan snapshot where all natural units are tested selectively by binary division.
	keep := make([]bool, len(units))
	var isolateBad func(subset []int)
	isolateBad = func(subset []int) {
		if len(subset) == 0 {
			return
		}
		trial := ctx.planWithSubsetUnits(plan, units, subset)
		if ctx.testPlanCompiles(trial, legacy) {
			for _, ui := range subset {
				keep[ui] = true
			}
			return
		}
		if len(subset) == 1 {
			return
		}
		mid := len(subset) / 2
		isolateBad(subset[:mid])
		isolateBad(subset[mid:])
	}

	all := make([]int, len(units))
	for i := range units {
		all[i] = i
	}
	isolateBad(all)

	// Demote any unit not in keep, then verify combined plan (handling rare cross-unit interactions).
	for ui, ok := range keep {
		if !ok {
			ctx.demoteUnit(plan, units[ui])
		}
	}
	recountHybridPlanStats(plan)

	if compiled, ok := ctx.compilePlanToRawMap(plan, legacy); ok {
		return compiled, nil
	}

	// Rare cross-unit compile interaction: drop remaining active units from end until compile succeeds.
	active := collectNaturalUnits(plan)
	for i := len(active) - 1; i >= 0; i-- {
		ctx.demoteUnit(plan, active[i])
		recountHybridPlanStats(plan)
		if compiled, ok := ctx.compilePlanToRawMap(plan, legacy); ok {
			return compiled, nil
		}
	}
	return nil, fmt.Errorf("unable to isolate compile error in legacy=%v", legacy)
}

func (ctx *semanticImportContext) planWithSubsetUnits(base *HybridImportPlan, units []semanticVerifyUnit, keepIdxs []int) *HybridImportPlan {
	trial := cloneHybridPlan(base)
	keepAction := make(map[int]bool)
	for _, ui := range keepIdxs {
		for _, aIdx := range units[ui].indices {
			keepAction[aIdx] = true
		}
	}
	for i := range trial.Decisions {
		if trial.Decisions[i].Classification == "NATURAL_VERIFIED" && !keepAction[i] {
			ctx.demoteSingleIndexToFallback(trial, i, "")
		}
	}
	recountHybridPlanStats(trial)
	return trial
}

// Keep unused imports referenced safely if conditional compilation changes.
var _ = syntax.TokenEqual
var _ = source.Span{}
var _ = analysis.NewScope
