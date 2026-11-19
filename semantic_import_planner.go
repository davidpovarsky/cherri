/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

// semanticControlRegion tracks a paired control-flow construct across both Legacy and V2.
type semanticControlRegion struct {
	GroupID                string
	Kind                   string // "conditional", "choosefrommenu", "repeat.count", "repeat.each"
	Start                  int
	Mids                   []int
	End                    int
	Valid                  bool
	Parent                 *semanticControlRegion
	Children               []*semanticControlRegion
	ContainsInvalidControl bool
}

// semanticImportContext holds the single shared semantic interpretation derived from
// decompile.go's analysis passes, NativeShortcutDocument, and DefaultReverseActionRegistry.
type semanticImportContext struct {
	Doc                     *NativeShortcutDocument
	Registry                *ReverseActionRegistry
	UseCache                bool
	Actions                 []ShortcutAction
	UsedProducerUUIDs       map[string]bool
	UUIDToSymbol            map[string]string
	SymbolToUUID            map[string]string
	VarToSymbol             map[string]string
	SymbolToVar             map[string]string
	Symbols                 map[string]map[string]any
	SymbolProducers         map[string]int
	RegionsByGroup          map[string]*semanticControlRegion
	RegionsByIndex          map[int]*semanticControlRegion
	DecompiledUUIDHints     map[string]string
	MatchedActionNames      []string
	fallbackLegacyLines     []string
	fallbackV2Lines         []string
	fallbackReasons         []string
	candidateActionMarks    map[int]string
	candidateRegionOrder    []string
	postAtomicActionCnt     int
	rawRefCounter           int
	pendingLegacyDecls      []string
	insideJSONLiteral       int
	currentSynthActionIdent string
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
			CandidateStatus: "NOT_CANDIDATE",
			LegacyCall:      rawLeg,
			V2Call:          rawV2,
		}
		if rec.SourceValidity != "VALID" {
			plan.Decisions[i].Classification = "SOURCE_INVALID"
			plan.Decisions[i].ReasonCode = "SOURCE_INVALID"
			plan.Decisions[i].CandidateStatus = "SOURCE_INVALID"
			plan.Decisions[i].DiagnosticComment = fmt.Sprintf("// [HYBRID_DIAGNOSTIC] action %d (%s): %s", rec.Index, rec.Identifier, rec.SourceValidity)
		}
	}

	// Stage 1: Synthesize paired Legacy+V2 semantic candidates for control regions and actions.
	if err := ctx.synthesizeAllCandidates(plan); err != nil {
		return nil, err
	}
	ctx.rejectUnseededForwardActionOutputCandidates(plan)
	ctx.captureCandidateSnapshot(plan)

	// Stage 2: Reconcile control-flow region pairing (never leave partial natural region).
	ctx.enforceAtomicRegionsWithStage(plan, "ATOMIC_REGION", "ATOMIC_REGION_PARTIAL")
	ctx.postAtomicActionCnt = countActiveNaturalCandidates(plan)
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
	for i, rec := range doc.Actions {
		if rec.Identifier == "is.workflow.actions.dictionary" && rec.UUID != "" && doc.ProducerUUIDIndex[rec.UUID] == i {
			ctx.UsedProducerUUIDs[rec.UUID] = true
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
	origArgs := args.Args
	embeddedCompilerMode = true
	args.Args = map[string]string{"import": "", "no-ansi": ""}
	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(embeddedCompilerPanic); ok {
				err = fmt.Errorf("%s", ep.message)
			} else {
				err = fmt.Errorf("decompile semantic pass panic: %v", r)
			}
		}
		embeddedCompilerMode = origEmbedded
		resetCompilerStateFully()
		args.Args = origArgs
	}()

	resetCompilerStateFully()
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

	ctx.DecompiledUUIDHints = make(map[string]string, len(uuids))
	for k, v := range uuids {
		ctx.DecompiledUUIDHints[k] = v
	}
	ctx.Actions = make([]ShortcutAction, len(shortcut.WFWorkflowActions))
	copy(ctx.Actions, shortcut.WFWorkflowActions)
	ctx.MatchedActionNames = make([]string, len(ctx.Actions))
	for i := range ctx.Actions {
		actCopy := ctx.Actions[i]
		matchedName, _ := matchAction(&actCopy)
		ctx.MatchedActionNames[i] = matchedName
	}
	return nil
}

func markRegionAndAncestorsInvalid(reg *semanticControlRegion) {
	for cur := reg; cur != nil; cur = cur.Parent {
		cur.Valid = false
		cur.ContainsInvalidControl = true
	}
}

func (ctx *semanticImportContext) buildControlRegions() {
	stack := make([]*semanticControlRegion, 0)
	dupGroups := make(map[string]bool)
	markOpenStackInvalid := func() {
		for _, openReg := range stack {
			markRegionAndAncestorsInvalid(openReg)
		}
	}

	for i, rec := range ctx.Doc.Actions {
		kind := controlKindFromIdentifier(rec.Identifier)
		if rec.ControlFlowMode == nil {
			if rec.SourceValidity != "VALID" && (kind != "" || strings.HasPrefix(rec.SourceValidity, "SOURCE_INVALID:CONTROL_FLOW")) {
				markOpenStackInvalid()
			}
			continue
		}

		gid := rec.GroupingIdentifier
		mode := *rec.ControlFlowMode
		switch mode {
		case 0:
			var parent *semanticControlRegion
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			reg := &semanticControlRegion{
				GroupID: gid,
				Kind:    kind,
				Start:   i,
				End:     -1,
				Parent:  parent,
			}
			if parent != nil {
				parent.Children = append(parent.Children, reg)
			}
			hasErr := rec.SourceValidity != "VALID" || gid == "" || kind == ""
			if gid != "" {
				if prev, exists := ctx.RegionsByGroup[gid]; exists {
					dupGroups[gid] = true
					markRegionAndAncestorsInvalid(prev)
					hasErr = true
				} else if !hasErr {
					ctx.RegionsByGroup[gid] = reg
				}
			}
			ctx.RegionsByIndex[i] = reg
			stack = append(stack, reg)
			if hasErr {
				markRegionAndAncestorsInvalid(reg)
			}
		case 1:
			if len(stack) == 0 || stack[len(stack)-1].GroupID != gid || gid == "" ||
				stack[len(stack)-1].Kind != kind || rec.SourceValidity != "VALID" ||
				(kind != "conditional" && kind != "choosefrommenu") ||
				(kind == "conditional" && len(stack[len(stack)-1].Mids) > 0) {
				markOpenStackInvalid()
				if gid != "" {
					if reg, ok := ctx.RegionsByGroup[gid]; ok {
						markRegionAndAncestorsInvalid(reg)
					}
				}
				continue
			}
			top := stack[len(stack)-1]
			top.Mids = append(top.Mids, i)
			ctx.RegionsByIndex[i] = top
		case 2:
			if len(stack) > 0 && gid != "" && stack[len(stack)-1].GroupID == gid {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				top.End = i
				ctx.RegionsByIndex[i] = top
				if rec.SourceValidity != "VALID" || top.Kind != kind || top.Kind == "" || top.ContainsInvalidControl || dupGroups[gid] || top.Start < 0 || top.End <= top.Start {
					markRegionAndAncestorsInvalid(top)
				} else {
					top.Valid = true
				}
				continue
			}
			markOpenStackInvalid()
			if gid != "" {
				if reg, ok := ctx.RegionsByGroup[gid]; ok {
					markRegionAndAncestorsInvalid(reg)
				}
			}
		default:
			markOpenStackInvalid()
		}
	}
	// Mark any region still open on stack (and its ancestors) invalid.
	for _, r := range stack {
		markRegionAndAncestorsInvalid(r)
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

		// Reuse precomputed decompile.go identifier hint when present.
		hint := ctx.DecompiledUUIDHints[u]
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
		if rec.CustomOutputName != "" {
			hasOutName = true
			outName = rec.CustomOutputName
		} else if info != nil && info.sawName {
			hasOutName = true
			outName = info.name
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
	dictVarFromSet := make(map[string]bool)
	nonDictVarWrite := make(map[string]bool)
	allVarSet := make(map[string]bool)
	for i, rec := range ctx.Doc.Actions {
		if rec.Identifier == "is.workflow.actions.setvariable" || rec.Identifier == "is.workflow.actions.appendvariable" {
			if vn, _ := rec.Parameters["WFVariableName"].(string); vn != "" {
				allVarSet[vn] = true
				if rec.Identifier == "is.workflow.actions.appendvariable" {
					appendVarSet[vn] = true
					nonDictVarWrite[vn] = true
				} else if wfIn, hasIn := rec.Parameters["WFInput"]; hasIn {
					setVarBeforeAppend[vn] = true
					isDictAssign := false
					if inMap, okIn := wfIn.(map[string]any); okIn {
						if valMap, okVal := inMap["Value"].(map[string]any); okVal && strVal(valMap["Type"]) == "ActionOutput" {
							u := strVal(valMap["OutputUUID"])
							ags, _ := valMap["Aggrandizements"].([]any)
							if prodIdx, okP := ctx.Doc.ProducerUUIDIndex[u]; okP && prodIdx < len(ctx.Doc.Actions) && len(ags) == 0 {
								if inferValueTypeForAction(ctx.Doc.Actions[prodIdx].Identifier, canonicalDefs) == "Dictionary" {
									isDictAssign = true
								}
							}
						}
					}
					if isDictAssign {
						dictVarFromSet[vn] = true
					} else {
						nonDictVarWrite[vn] = true
					}
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
		if appendVarSet[origVar] {
			entry["AppendVariable"] = true
		}
		if dictHintVar[origVar] || (dictVarFromSet[origVar] && !nonDictVarWrite[origVar]) {
			entry["DictAggrandizement"] = true
		}
		if !dictHintVar[origVar] && appendVarSet[origVar] && !setVarBeforeAppend[origVar] {
			entry["ValueType"] = "List"
		} else if (dictHintVar[origVar] || dictVarFromSet[origVar]) && !nonDictVarWrite[origVar] {
			entry["ValueType"] = "Dictionary"
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
	case "is.workflow.actions.date", "is.workflow.actions.adjustdate", "is.workflow.actions.detect.date":
		return "Date"
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
		if reg == nil || !reg.Valid || reg.ContainsInvalidControl {
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
	ctx.pendingLegacyDecls = nil
	defer func() {
		ctx.pendingLegacyDecls = nil
	}()
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

	hasPendingDecls := len(ctx.pendingLegacyDecls) > 0
	legCall = ctx.flushPendingLegacyDecls(legCall)

	// When safe/context-free and not already verified in workflow, use cached single-action verifier!
	if !hasPendingDecls && ctx.isContextFreeRecord(rec) && rec.Identifier != "is.workflow.actions.setvariable" && rec.Identifier != "is.workflow.actions.appendvariable" {
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
	dec.Classification = "NATURAL_CANDIDATE"
	dec.ReasonCode = "KNOWN_DISAMBIGUATED"
	dec.CandidateStatus = "CANDIDATE"
	dec.DemotionStage = ""
	dec.DemotionReason = ""
	dec.ChosenDefinitionID = chosenID
	dec.IncludeCategory = includeCat
	dec.LegacyCall = legCall
	dec.V2Call = v2Call
}

func (ctx *semanticImportContext) isContextFreeRecord(rec NativeActionRecord) bool {
	if rec.UUID != "" || rec.CustomOutputName != "" || len(rec.ReferencedUUIDs) > 0 || len(rec.ReferencedVars) > 0 {
		return false
	}
	if defShowKey := actionDefaultShowMetaKey(rec.Identifier); defShowKey != "" {
		if _, hasShow := rec.Parameters[defShowKey]; !hasShow {
			return false
		}
	}
	for k := range rec.Parameters {
		if isImportProvenanceMetaKey(k) {
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
		return "", "", false
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
		lit, ok = ctx.renderAnyTextPair(rawVal, true)
	}
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
	valExpr, ok := ctx.renderVarAssignInputPair(rawIn)
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
	valExpr, ok := ctx.renderVarAssignInputPair(rawIn)
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf("@%s += %s", sym, valExpr.Legacy), fmt.Sprintf("%s += %s", sym, valExpr.V2), true
}

// renderVarAssignInputPair restricts direct variable assignment/append inputs to single-action
// attachment references; inline literals compile to two actions in Legacy (@v = literal).
func (ctx *semanticImportContext) renderVarAssignInputPair(rawIn any) (semanticExprPair, bool) {
	m, ok := rawIn.(map[string]any)
	if !ok {
		return semanticExprPair{}, false
	}
	ser, _ := m["WFSerializationType"].(string)
	if ser == "WFTextTokenAttachment" {
		return ctx.renderSerializedAttachmentPair(m, true)
	}
	if typ, _ := m["Type"].(string); typ != "" {
		return ctx.renderAttachmentBodyPair(m, true)
	}
	return semanticExprPair{}, false
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
			return ctx.renderStrictSortedDictionaryPair(v)
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

	// Re-use precomputed decompile.go matchAction heuristic captured before any verification compiles.
	if idx >= 0 && idx < len(ctx.MatchedActionNames) {
		matchedName := ctx.MatchedActionNames[idx]
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
		if isImportProvenanceMetaKey(k) {
			continue
		}
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
	switch rec.Identifier {
	case "is.workflow.actions.gettextfrompdf":
		if _, hasType := rec.Parameters["WFGetTextFromPDFTextType"]; !hasType {
			return "", "", false
		}
	case "is.workflow.actions.filter.files":
		limEn, hasLimEn := rec.Parameters["WFContentItemLimitEnabled"].(bool)
		_, hasLimNum := rec.Parameters["WFContentItemLimitNumber"]
		if (hasLimEn && (!limEn || !hasLimNum)) || (hasLimNum && !hasLimEn) {
			return "", "", false
		}
	case "is.workflow.actions.runworkflow":
		if cand.CallableName == "runSelf" {
			_, hasWF := rec.Parameters["WFWorkflow"]
			_, hasName := rec.Parameters["WFWorkflowName"]
			_, hasIn := rec.Parameters["WFInput"]
			isSelf, _ := rec.Parameters["isSelf"].(bool)
			if !isSelf || hasWF || hasName || hasIn {
				return "", "", false
			}
		} else {
			// `run` generates a fresh random WFWorkflow.workflowIdentifier UUID on every compile,
			// which cannot preserve existing workflow descriptors under strict raw-wire equality.
			return "", "", false
		}
	case "is.workflow.actions.openapp":
		if rec.Parameters["WFAppName"] != nil || rec.Parameters["WFWindowingFormat"] != nil {
			return "", "", false
		}
		selApp, hasSel := rec.Parameters["WFSelectedApp"]
		if !hasSel {
			return "", "", false
		}
		if selMap, isMap := selApp.(map[string]any); isMap {
			if len(selMap) != 1 {
				return "", "", false
			}
			bundleVal, hasBundle := selMap["BundleIdentifier"]
			if !hasBundle || !plistValuesStrictlyEqual(bundleVal, rec.Parameters["WFAppIdentifier"]) {
				return "", "", false
			}
		} else if _, isAtt := rec.Parameters["WFAppIdentifier"].(map[string]any); !isAtt {
			return "", "", false
		}
	}
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
	prevSynthIdent := ctx.currentSynthActionIdent
	ctx.currentSynthActionIdent = cand.AppleIdentifier
	defer func() {
		ctx.currentSynthActionIdent = prevSynthIdent
	}()
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
		paramKey := p.key
		if paramKey == "" {
			if cand.CallableName == "list" && p.infinite {
				paramKey = "WFItems"
			} else {
				if ep, handled, okTP := ctx.reverseTextPartsParam(rec, cand.CallableName, p); handled {
					if !okTP {
						return nil, nil, false
					}
					if ep.Legacy != "" || ep.V2 != "" {
						slots[i] = renderedSlot{present: true, legList: []string{ep.Legacy}, v2List: []string{ep.V2}, label: p.name}
					}
					continue
				}
				if ep, handled, okAsk := ctx.reverseAskDefaultParam(rec, &def, p); handled {
					if !okAsk {
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
		}
		rawVal, exists := rec.Parameters[paramKey]
		if !exists {
			if !p.optional {
				return nil, nil, false
			}
			continue
		}

		// Handle infinite/variadic parameter (e.g. list(...), ask, etc.)
		if p.infinite {
			if items, ok := rawVal.([]any); ok {
				if len(items) == 0 {
					if p.optional {
						continue
					}
					return nil, nil, false
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
			// String/RawString parameters in Cherri serialize references as WFTextTokenString unless attachment-capable.
			if (p.validType == String || p.validType == RawString) && !isAttachmentCapableTextParam(ctx.currentSynthActionIdent, p.key) {
				return semanticExprPair{}, false
			}
			return ctx.renderSerializedAttachmentPair(m, false)
		case "WFTextTokenString":
			return ctx.renderWFTextTokenStringPair(m)
		case "WFDictionaryFieldValue":
			return ctx.renderStrictSortedDictionaryPair(m)
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
		return semanticExprPair{}, false
	case Arr:
		if items, ok := rawVal.([]any); ok {
			return ctx.renderArrayLiteralPair(items)
		}
	case Dict:
		if m, ok := rawVal.(map[string]any); ok {
			if ep, ok := ctx.renderStrictSortedDictionaryPair(m); ok {
				return ep, true
			}
			return semanticExprPair{}, false
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
			return semanticExprPair{}, false
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
	// If string contains braces in Legacy, Cherri parses {x} inside "..." as interpolation unless single-quoted.
	// Inside JSON/dictionary literals, Legacy json.Unmarshal forbids single quotes and makeDictionaryItem
	// alters WFTextTokenString when '{' or '}' appears, so fail closed when insideJSONLiteral > 0.
	v2Esc := formatCherriDoubleQuotedString(s)
	legEsc := v2Esc
	if ctx.insideJSONLiteral > 0 {
		if strings.ContainsAny(s, "{}") {
			return semanticExprPair{}, false
		}
	} else if strings.Contains(s, "{") {
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

func (ctx *semanticImportContext) nextRawRefName() string {
	for {
		ctx.rawRefCounter++
		cand := fmt.Sprintf("_rawRef_%d", ctx.rawRefCounter)
		if _, exists := ctx.Symbols[cand]; !exists {
			return cand
		}
	}
}

func (ctx *semanticImportContext) flushPendingLegacyDecls(legExpr string) string {
	if len(ctx.pendingLegacyDecls) == 0 {
		return legExpr
	}
	joined := strings.Join(ctx.pendingLegacyDecls, "\n") + "\n" + legExpr
	ctx.pendingLegacyDecls = nil
	return joined
}

func (ctx *semanticImportContext) renderAttachmentBodyPair(att map[string]any, insideAssign bool) (semanticExprPair, bool) {
	return ctx.renderAttachmentBodyPairEx(att, insideAssign, false)
}

func (ctx *semanticImportContext) renderAttachmentBodyPairEx(att map[string]any, insideAssign, insideString bool) (semanticExprPair, bool) {
	typ, _ := att["Type"].(string)
	if typ == "" {
		return semanticExprPair{}, false
	}

	var legBase, v2Base string
	var needSymbolicFallback bool
	switch typ {
	case "ActionOutput":
		u, _ := att["OutputUUID"].(string)
		sym, ok := ctx.UUIDToSymbol[u]
		if !ok || sym == "" {
			return semanticExprPair{}, false
		}
		legBase = sym
		v2Base = sym
		on, hasOn := att["OutputName"].(string)
		symRec := ctx.Symbols[sym]
		symHasOutName, _ := symRec["HasOutputName"].(bool)
		symOutName, _ := symRec["OutputName"].(string)
		if hasOn != symHasOutName || (hasOn && on != symOutName) {
			needSymbolicFallback = true
		}
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
			needSymbolicFallback = true
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
		if !ok {
			return semanticExprPair{}, false
		}
		if len(agList) > 0 {
			if !needSymbolicFallback {
				legRef, v2Ref, needsLegacyFallback, okA := ctx.applyAggrandizementSuffixes(legBase, v2Base, agList, typ == "DeviceDetails", ctx.attachmentBaseValueType(att))
				if okA {
					_ = insideAssign
					if needsLegacyFallback {
						rawPair, okRaw := ctx.renderSymbolicRawRefPair(att, insideString)
						if !okRaw {
							return semanticExprPair{}, false
						}
						legRef = rawPair.Legacy
					}
					return semanticExprPair{Legacy: legRef, V2: v2Ref}, true
				}
			}
			return ctx.renderSymbolicRawRefPair(att, insideString)
		}
	}

	if needSymbolicFallback {
		return ctx.renderSymbolicRawRefPair(att, insideString)
	}
	return semanticExprPair{Legacy: legBase, V2: v2Base}, true
}

func (ctx *semanticImportContext) renderSymbolicRawRefPair(att map[string]any, insideString bool) (semanticExprPair, bool) {
	if !insideString && ctx.insideJSONLiteral > 0 {
		return semanticExprPair{}, false
	}
	for k := range att {
		switch k {
		case "Type", "OutputUUID", "OutputName", "VariableName", "Prompt", "Aggrandizements":
		default:
			return semanticExprPair{}, false
		}
	}

	typ, _ := att["Type"].(string)
	legMap := make(map[string]any)
	var legBaseSym string
	var v2Primary string
	var v2Named []string

	switch typ {
	case "ActionOutput":
		u, _ := att["OutputUUID"].(string)
		sym, ok := ctx.UUIDToSymbol[u]
		if !ok || sym == "" {
			return semanticExprPair{}, false
		}
		legBaseSym = sym
		v2Primary = sym
		symRec := ctx.Symbols[sym]
		symHasOutName, _ := symRec["HasOutputName"].(bool)
		if on, hasOn := att["OutputName"].(string); hasOn && on != "" {
			legMap["OutputName"] = on
			v2Named = append(v2Named, "outputName: "+formatCherriDoubleQuotedString(on))
		} else if symHasOutName {
			legMap["OutputName"] = ""
			v2Named = append(v2Named, `outputName: ""`)
		}
	case "Variable":
		vn, _ := att["VariableName"].(string)
		if vn == "" {
			vn, _ = att["OutputName"].(string)
		}
		if vn == "" {
			return semanticExprPair{}, false
		}
		if isBuiltinLoopVarName(vn) {
			legMap["Type"] = "Variable"
			legMap["VariableName"] = vn
			v2Named = append(v2Named, `kind: "Variable"`, "name: "+formatCherriDoubleQuotedString(vn))
		} else {
			sym, ok := ctx.VarToSymbol[vn]
			if !ok || sym == "" {
				return semanticExprPair{}, false
			}
			legBaseSym = sym
			v2Primary = sym
			if on, hasOn := att["OutputName"].(string); hasOn && on != "" {
				legMap["OutputName"] = on
				v2Named = append(v2Named, "outputName: "+formatCherriDoubleQuotedString(on))
			}
		}
	case "ExtensionInput":
		legMap["Type"] = "ExtensionInput"
		v2Primary = "ShortcutInput"
	case "CurrentDate":
		legMap["Type"] = "CurrentDate"
		v2Primary = "CurrentDate"
	case "Clipboard":
		legMap["Type"] = "Clipboard"
		v2Primary = "Clipboard"
	case "DeviceDetails":
		legMap["Type"] = "DeviceDetails"
		v2Primary = "Device"
	case "Ask":
		legMap["Type"] = "Ask"
		v2Primary = "Ask"
		if p, hasP := att["Prompt"].(string); hasP && p != "" {
			legMap["Prompt"] = p
			v2Named = append(v2Named, "prompt: "+formatCherriDoubleQuotedString(p))
		}
	default:
		return semanticExprPair{}, false
	}

	if rawAggrs, hasAggrs := att["Aggrandizements"]; hasAggrs {
		agList, ok := rawAggrs.([]any)
		if !ok {
			return semanticExprPair{}, false
		}
		if len(agList) > 0 {
			legMap["Aggrandizements"] = agList
			agPlistLit, err := encodePlistValueToLiteral(map[string]any{"Aggrandizements": agList})
			if err != nil {
				return semanticExprPair{}, false
			}
			v2Named = append(v2Named, "aggrandizements: native.plist("+formatCherriDoubleQuotedString(agPlistLit)+")")
		}
	}

	legPlistLit, err := encodePlistValueToLiteral(legMap)
	if err != nil {
		return semanticExprPair{}, false
	}
	var legCall string
	if legBaseSym != "" {
		legCall = fmt.Sprintf("rawRef(%s, rawPlist(%s))", legBaseSym, formatCherriDoubleQuotedString(legPlistLit))
	} else {
		legCall = fmt.Sprintf("rawRef(rawPlist(%s))", formatCherriDoubleQuotedString(legPlistLit))
	}
	legOut := legCall
	if insideString {
		tmp := ctx.nextRawRefName()
		ctx.pendingLegacyDecls = append(ctx.pendingLegacyDecls, fmt.Sprintf("const %s = %s", tmp, legCall))
		legOut = tmp
	}

	v2Args := make([]string, 0, 1+len(v2Named))
	if v2Primary != "" {
		v2Args = append(v2Args, v2Primary)
	}
	v2Args = append(v2Args, v2Named...)
	v2Out := fmt.Sprintf("native.ref(%s)", strings.Join(v2Args, ", "))
	return semanticExprPair{Legacy: legOut, V2: v2Out}, true
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

func (ctx *semanticImportContext) attachmentBaseValueType(att map[string]any) string {
	switch strVal(att["Type"]) {
	case "ActionOutput":
		uuid := strVal(att["OutputUUID"])
		if sym, ok := ctx.UUIDToSymbol[uuid]; ok && sym != "" {
			if valueType := strVal(ctx.Symbols[sym]["ValueType"]); valueType != "" {
				return valueType
			}
		}
		// Older import hints compare relative declaration identifiers with full
		// native IDs. Recover collection cardinality only from the existing,
		// unambiguous canonical candidate; never guess across variants.
		if ctx.Doc != nil && ctx.Registry != nil {
			if index, found := ctx.Doc.ProducerUUIDIndex[uuid]; found && index >= 0 && index < len(ctx.Doc.Actions) {
				producer := ctx.Doc.Actions[index]
				resolved := ctx.Registry.ResolveReverseActionCandidate(producer)
				candidate := resolved.ChosenCandidate
				// The scalar-only reverse fast path can reject an attachment
				// even when the native ID has exactly one declared output type.
				// Semantic import verifies the complete attachment separately.
				if candidates := ctx.Registry.ByAppleIdentifier[producer.Identifier]; candidate == nil && len(candidates) == 1 {
					candidate = &candidates[0]
				}
				if candidate != nil && candidate.Definition.outputType == Arr {
					return "List"
				}
			}
		}
	case "Variable":
		vn := strVal(att["VariableName"])
		if vn == "" {
			vn = strVal(att["OutputName"])
		}
		if vn == "" || isBuiltinLoopVarName(vn) {
			return ""
		}
		if sym, ok := ctx.VarToSymbol[vn]; ok && sym != "" {
			if vt := strVal(ctx.Symbols[sym]["ValueType"]); vt != "" {
				return vt
			}
			if da, _ := ctx.Symbols[sym]["DictAggrandizement"].(bool); da {
				return "Dictionary"
			}
			return ""
		}
	case "CurrentDate":
		return "Date"
	case "DeviceDetails":
		return "DeviceDetails"
	}
	return ""
}

func (ctx *semanticImportContext) applyAggrandizementSuffixes(legBase, v2Base string, agList []any, isDevice bool, baseVT string) (string, string, bool, bool) {
	// Convert to []Aggrandizement to reuse decompile.go's decompAggrandizements for Legacy,
	// and pair with V2's MemberExpr/IndexExpr syntax!
	// Legacy decompAggrandizements/makeAttachmentValues only emit at most one getAs/Prop followed
	// by at most one Coercion (plus single leading Coercion -> ['Key']/['Prop'] with coerceFirst).
	if len(agList) > 2 {
		return "", "", false, false
	}
	sawIndexOrProp := false
	sawCoerce := false
	needsLegacyFallback := false
	firstCoerceAlias := ""
	typed := make([]Aggrandizement, 0, len(agList))
	v2Expr := v2Base
	for _, rawA := range agList {
		m, ok := rawA.(map[string]any)
		if !ok {
			return "", "", false, false
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
			return "", "", false, false
		}
		typed = append(typed, ag)

		switch typ {
		case "WFDictionaryValueVariableAggrandizement":
			allowCoerceFirstDict := sawCoerce && len(typed) == 2 &&
				typed[0].Type == "WFCoercionVariableAggrandizement" &&
				typed[0].CoercionItemClass == "WFDictionaryContentItem"
			if ag.DictionaryKey == "" || isDevice || sawIndexOrProp || (sawCoerce && !allowCoerceFirstDict) ||
				strings.ContainsAny(ag.DictionaryKey, "'\"}\r\n") {
				return "", "", false, false
			}
			if !allowCoerceFirstDict && baseVT != "Dictionary" {
				if baseVT != "" && baseVT != "List" {
					return "", "", false, false
				}
				needsLegacyFallback = true
			}
			sawIndexOrProp = true
			v2Expr = fmt.Sprintf("%s[%s]", v2Expr, formatCherriDoubleQuotedString(ag.DictionaryKey))
		case "WFCoercionVariableAggrandizement":
			if sawCoerce {
				return "", "", false, false
			}
			sawCoerce = true
			alias, found := coercionAliasForClass(ag.CoercionItemClass)
			if !found {
				return "", "", false, false
			}
			if len(typed) == 1 {
				firstCoerceAlias = alias
			}
			v2Expr = fmt.Sprintf("%s.%s", v2Expr, alias)
		case "WFPropertyVariableAggrandizement":
			allowCoerceFirstNonDict := sawCoerce && len(typed) == 2 &&
				typed[0].Type == "WFCoercionVariableAggrandizement" &&
				typed[0].CoercionItemClass != "" &&
				typed[0].CoercionItemClass != "WFDictionaryContentItem"
			// A native property aggrandizement on a collection is not the
			// scalar field access expressed by ordinary V2 dot syntax. Keep
			// its exact attachment through the existing symbolic native.ref
			// path rather than weakening authored List cardinality checks.
			if baseVT == "List" && !allowCoerceFirstNonDict {
				return "", "", false, false
			}
			if ag.PropertyName == "" || sawIndexOrProp || (sawCoerce && !allowCoerceFirstNonDict) ||
				strings.ContainsAny(ag.PropertyName, "'\"}\r\n") {
				return "", "", false, false
			}
			if !allowCoerceFirstNonDict && baseVT == "Dictionary" {
				return "", "", false, false
			}
			if isDevice && sawCoerce {
				return "", "", false, false
			}
			sawIndexOrProp = true
			if isDevice || allowCoerceFirstNonDict {
				v2Expr = fmt.Sprintf("%s[%s]", v2Expr, formatCherriDoubleQuotedString(ag.PropertyName))
			} else if isSimpleIdent(ag.PropertyName) && !contentItemClassesLower()[strings.ToLower(ag.PropertyName)] {
				v2Expr = fmt.Sprintf("%s.%s", v2Expr, ag.PropertyName)
			} else {
				return "", "", false, false
			}
		default:
			return "", "", false, false
		}
	}

	var legRef string
	if len(typed) == 2 && !isDevice &&
		typed[0].Type == "WFCoercionVariableAggrandizement" && typed[0].CoercionItemClass == "WFDictionaryContentItem" &&
		typed[1].Type == "WFDictionaryValueVariableAggrandizement" && typed[1].DictionaryKey != "" &&
		!strings.ContainsAny(typed[1].DictionaryKey, "'\"}\r\n") {
		legRef = fmt.Sprintf("%s.dictionary['%s']", legBase, typed[1].DictionaryKey)
	} else if len(typed) == 2 && !isDevice &&
		typed[0].Type == "WFCoercionVariableAggrandizement" && typed[0].CoercionItemClass != "" &&
		typed[0].CoercionItemClass != "WFDictionaryContentItem" &&
		typed[1].Type == "WFPropertyVariableAggrandizement" && typed[1].PropertyName != "" &&
		firstCoerceAlias != "" && !strings.ContainsAny(typed[1].PropertyName, "'\"}\r\n") {
		legRef = fmt.Sprintf("%s.%s['%s']", legBase, firstCoerceAlias, typed[1].PropertyName)
	} else {
		legRef = legBase
		decompAggrandizements(&legRef, typed)
	}
	if legRef == "" || v2Expr == "" {
		return "", "", false, false
	}
	return legRef, v2Expr, needsLegacyFallback, true
}

func unwrapWFListItem(it any) (any, bool) {
	m, ok := it.(map[string]any)
	if !ok || len(m) != 2 {
		return nil, false
	}
	itType, okT := provInt(m["WFItemType"])
	val, hasVal := m["WFValue"]
	if !okT || itType != 0 || !hasVal {
		return nil, false
	}
	switch v := val.(type) {
	case string:
		return v, true
	case map[string]any:
		if ser, _ := v["WFSerializationType"].(string); ser == "WFTextTokenString" {
			return v, true
		}
	}
	return nil, false
}

func (ctx *semanticImportContext) reverseTextPartsParam(rec NativeActionRecord, callableName string, p parameterDefinition) (semanticExprPair, bool, bool) {
	if (callableName != "splitText" && callableName != "joinText") || (p.name != "separator" && p.name != "glue") {
		return semanticExprPair{}, false, false
	}
	sepMode, _ := rec.Parameters["WFTextSeparator"].(string)
	switch sepMode {
	case "New Lines":
		if _, hasCustom := rec.Parameters["WFTextCustomSeparator"]; hasCustom {
			return semanticExprPair{}, true, false
		}
		return semanticExprPair{}, true, true
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
		if s, isStr := rawCustom.(string); isStr && (s == " " || s == "\n" || (s == "" && callableName == "splitText")) {
			return semanticExprPair{}, true, false
		}
		ep, ok := ctx.renderAnyTextPair(rawCustom, true)
		return ep, true, ok
	default:
		return semanticExprPair{}, true, false
	}
}

func (ctx *semanticImportContext) reverseAskDefaultParam(rec NativeActionRecord, def *actionDefinition, p parameterDefinition) (semanticExprPair, bool, bool) {
	if def == nil || def.identifier != "ask" || p.name != "defaultValue" {
		return semanticExprPair{}, false, false
	}
	allDefaultKeys := []string{
		"WFAskActionDefaultAnswer",
		"WFAskActionDefaultAnswerNumber",
		"WFAskActionDefaultAnswerURL",
		"WFAskActionDefaultAnswerDate",
		"WFAskActionDefaultAnswerTime",
		"WFAskActionDefaultAnswerDateAndTime",
	}
	var presentKeys []string
	for _, k := range allDefaultKeys {
		if _, ok := rec.Parameters[k]; ok {
			presentKeys = append(presentKeys, k)
		}
	}
	if len(presentKeys) == 0 {
		return semanticExprPair{}, true, true
	}
	if len(presentKeys) > 1 {
		return semanticExprPair{}, true, false
	}
	actualKey := presentKeys[0]
	inputType, _ := rec.Parameters["WFInputType"].(string)
	expectedKey := "WFAskActionDefaultAnswer"
	switch inputType {
	case "Number":
		expectedKey = "WFAskActionDefaultAnswerNumber"
	case "URL":
		expectedKey = "WFAskActionDefaultAnswerURL"
	case "Date":
		expectedKey = "WFAskActionDefaultAnswerDate"
	case "Time":
		expectedKey = "WFAskActionDefaultAnswerTime"
	case "Date and Time":
		expectedKey = "WFAskActionDefaultAnswerDateAndTime"
	}
	if actualKey != expectedKey {
		return semanticExprPair{}, true, false
	}
	rawVal := rec.Parameters[actualKey]
	if actualKey == "WFAskActionDefaultAnswerNumber" {
		switch v := rawVal.(type) {
		case string:
			ep, ok := ctx.renderStringPair(v)
			return ep, true, ok
		case map[string]any:
			if ser, _ := v["WFSerializationType"].(string); ser == "WFTextTokenAttachment" {
				ep, ok := ctx.renderSerializedAttachmentPair(v, false)
				if ok {
					ep = ctx.preserveNativeOperandReference(v, ep)
				}
				return ep, true, ok
			}
		}
		return semanticExprPair{}, true, false
	}
	ep, ok := ctx.renderAnyTextPair(rawVal, true)
	if ok {
		ep = ctx.preserveNativeOperandReference(rawVal, ep)
	}
	return ep, true, ok
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
		if hasAtts {
			return semanticExprPair{}, false
		}
		// Uninterpolated WFTextTokenString: check if natural string produces WFTextTokenString cleanly when checked.
		return ctx.renderStringPair(str)
	}

	type rangeSpan struct {
		startRune int
		endRune   int
		att       semanticExprPair
	}
	prevDeclsLen := len(ctx.pendingLegacyDecls)
	spans := make([]rangeSpan, 0, len(rawAtts))
	for rngStr, rawA := range rawAtts {
		var u16Start, u16Len int
		if _, err := fmt.Sscanf(rngStr, "{%d, %d}", &u16Start, &u16Len); err != nil || u16Len <= 0 || u16Start < 0 {
			ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
			return semanticExprPair{}, false
		}
		attMap, ok := rawA.(map[string]any)
		if !ok {
			ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
			return semanticExprPair{}, false
		}
		// Reuse decompile.go's utf16OffsetToRuneIndex for exact surrogate/emoji alignment!
		sr := utf16OffsetToRuneIndex(str, u16Start)
		er := utf16OffsetToRuneIndex(str, u16Start+u16Len)
		if sr < 0 || er < sr {
			ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
			return semanticExprPair{}, false
		}
		ep, ok := ctx.renderAttachmentBodyPairEx(attMap, false, true)
		if !ok {
			ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
			return semanticExprPair{}, false
		}
		spans = append(spans, rangeSpan{startRune: sr, endRune: er, att: ep})
	}
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].startRune < spans[j].startRune
	})

	runes := []rune(str)
	var legSB, v2SB, legRawSB strings.Builder
	var expectedSpans [][2]int
	legSB.WriteByte('"')
	v2SB.WriteString(`f"`)

	cursor := 0
	for _, sp := range spans {
		if sp.startRune < cursor || sp.endRune > len(runes) {
			ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
			return semanticExprPair{}, false
		}
		litChunk := string(runes[cursor:sp.startRune])
		if checkInlineVarRegex.MatchString(litChunk) {
			ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
			return semanticExprPair{}, false
		}
		writeEscapedDualLiteralChunk(&legSB, litChunk, false)
		writeEscapedDualLiteralChunk(&v2SB, litChunk, true)
		legRawSB.WriteString(litChunk)

		spanStart := legRawSB.Len()
		legSB.WriteByte('{')
		legSB.WriteString(sp.att.Legacy)
		legSB.WriteByte('}')
		legRawSB.WriteByte('{')
		legRawSB.WriteString(sp.att.Legacy)
		legRawSB.WriteByte('}')
		expectedSpans = append(expectedSpans, [2]int{spanStart, legRawSB.Len()})

		v2SB.WriteByte('{')
		v2SB.WriteString(sp.att.V2)
		v2SB.WriteByte('}')

		cursor = sp.endRune
	}
	if cursor < len(runes) {
		tail := string(runes[cursor:])
		if checkInlineVarRegex.MatchString(tail) {
			ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
			return semanticExprPair{}, false
		}
		writeEscapedDualLiteralChunk(&legSB, tail, false)
		writeEscapedDualLiteralChunk(&v2SB, tail, true)
		legRawSB.WriteString(tail)
	}
	gotSpans := checkInlineVarRegex.FindAllStringIndex(legRawSB.String(), -1)
	if len(gotSpans) != len(expectedSpans) {
		ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
		return semanticExprPair{}, false
	}
	for idx, exp := range expectedSpans {
		if gotSpans[idx][0] != exp[0] || gotSpans[idx][1] != exp[1] {
			ctx.pendingLegacyDecls = ctx.pendingLegacyDecls[:prevDeclsLen]
			return semanticExprPair{}, false
		}
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
	ctx.insideJSONLiteral++
	defer func() {
		ctx.insideJSONLiteral--
	}()
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
	ctx.insideJSONLiteral++
	defer func() {
		ctx.insideJSONLiteral--
	}()
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
	ctx.insideJSONLiteral++
	defer func() {
		ctx.insideJSONLiteral--
	}()
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
		att, hasAtt := v["attachmentsByRange"].(map[string]any)
		if hasAtt {
			if len(att) == 0 {
				return "", semanticExprPair{}, false
			}
		} else {
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

func (ctx *semanticImportContext) isControlBranchProducerSafe(rec NativeActionRecord) bool {
	if rec.UUID != "" && ctx.UsedProducerUUIDs[rec.UUID] {
		return false
	}
	if _, hasCustom := rec.Parameters["CustomOutputName"]; hasCustom && rec.UUID == "" {
		return false
	}
	return true
}

func (ctx *semanticImportContext) controlEndAssignPrefix(endRec NativeActionRecord) (legPrefix, v2Prefix string, ok bool) {
	if endRec.UUID != "" && ctx.UsedProducerUUIDs[endRec.UUID] {
		sym, hasSym := ctx.UUIDToSymbol[endRec.UUID]
		if !hasSym || sym == "" {
			return "", "", false
		}
		return fmt.Sprintf("const %s = ", sym), fmt.Sprintf("let %s = ", sym), true
	}
	if _, hasCustom := endRec.Parameters["CustomOutputName"]; hasCustom {
		return "", "", false
	}
	return "", "", true
}

func (ctx *semanticImportContext) trySynthesizeConditionalRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	ctx.pendingLegacyDecls = nil
	defer func() {
		ctx.pendingLegacyDecls = nil
	}()
	startRec := ctx.Doc.Actions[reg.Start]
	endRec := ctx.Doc.Actions[reg.End]
	if !startRec.HadParametersKey || !endRec.HadParametersKey {
		return
	}
	if len(reg.Mids) > 1 {
		return
	}
	if !ctx.isControlBranchProducerSafe(startRec) {
		return
	}
	endAssignLeg, endAssignV2, okEnd := ctx.controlEndAssignPrefix(endRec)
	if !okEnd {
		return
	}
	allowedStart := map[string]bool{
		"GroupingIdentifier": true, "WFControlFlowMode": true,
		"WFCondition": true, "WFInput": true,
		"WFConditionalActionString": true, "WFNumberValue": true, "WFAnotherNumber": true,
		"WFDate": true, "WFAnotherDate": true, "WFEnumeration": true, "WFDuration": true,
		"WFConditions": true, "WFConditionalLegacyComparisonBehavior": true,
	}
	if !ctx.hasOnlyAllowedExtraKeys(startRec.Parameters, allowedStart) {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(endRec.Parameters, map[string]bool{"GroupingIdentifier": true, "WFControlFlowMode": true}) {
		return
	}
	if len(reg.Mids) == 1 {
		midRec := ctx.Doc.Actions[reg.Mids[0]]
		if !midRec.HadParametersKey || !ctx.isControlBranchProducerSafe(midRec) ||
			!ctx.hasOnlyAllowedExtraKeys(midRec.Parameters, map[string]bool{"GroupingIdentifier": true, "WFControlFlowMode": true}) {
			return
		}
	}

	condPair, ok := ctx.renderSingleConditionPair(startRec.Parameters)
	if !ok {
		return
	}

	// Set atomic RegionGroupID decisions across start / else / end.
	plan.Decisions[reg.Start].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.Start], ctx.flushPendingLegacyDecls(endAssignLeg+condPair.Legacy), endAssignV2+condPair.V2, "conditional", "")

	if len(reg.Mids) == 1 {
		mIdx := reg.Mids[0]
		plan.Decisions[mIdx].RegionGroupID = reg.GroupID
		applyDecisionCandidate(&plan.Decisions[mIdx], "} else {", "} else {", "conditional", "")
	}

	plan.Decisions[reg.End].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.End], "}", "}", "conditional", "")
}

func hasAnyConditionRHSKey(params map[string]any) bool {
	for _, k := range []string{
		"WFConditionalActionString", "WFNumberValue", "WFAnotherNumber",
		"WFDate", "WFAnotherDate", "WFEnumeration", "WFDuration",
	} {
		if _, ok := params[k]; ok {
			return true
		}
	}
	return false
}

func conditionBinaryOperator(condCode int) (string, bool) {
	switch condCode {
	case 4:
		return "==", true
	case 5:
		return "!=", true
	case 2:
		return ">", true
	case 3:
		return ">=", true
	case 0:
		return "<", true
	case 1:
		return "<=", true
	case 99:
		return "contains", true
	case 8:
		return "beginsWith", true
	case 9:
		return "endsWith", true
	default:
		return "", false
	}
}

func (ctx *semanticImportContext) renderConditionNumberRHS(v any) (semanticExprPair, bool) {
	if s, ok := v.(string); ok {
		q := formatCherriDoubleQuotedString(s)
		arg := fmt.Sprintf("numeric(%s)", q)
		return semanticExprPair{Legacy: arg, V2: arg}, true
	}
	return ctx.renderNumberOrAttachmentPair(v)
}

func (ctx *semanticImportContext) renderConditionDateRHS(v any) (semanticExprPair, bool) {
	if s, ok := v.(string); ok {
		if s == "" {
			return semanticExprPair{}, false
		}
		return ctx.renderStringPair(s)
	}
	if m, ok := v.(map[string]any); ok {
		ser, _ := m["WFSerializationType"].(string)
		if ser == "WFTextTokenAttachment" {
			return ctx.renderSerializedAttachmentPair(m, false)
		}
		if ser == "WFTextTokenString" {
			return ctx.renderWFTextTokenStringPair(m)
		}
		if typ, _ := m["Type"].(string); typ != "" {
			return ctx.renderAttachmentBodyPair(m, false)
		}
	}
	return semanticExprPair{}, false
}

func (ctx *semanticImportContext) conditionInputIsDate(inVarRaw any) bool {
	att, _ := inVarRaw.(map[string]any)
	if ser, _ := att["WFSerializationType"].(string); ser == "WFTextTokenAttachment" {
		att, _ = att["Value"].(map[string]any)
	}
	if att == nil {
		return false
	}
	if rawAgs, ok := att["Aggrandizements"].([]any); ok && len(rawAgs) > 0 {
		for i := len(rawAgs) - 1; i >= 0; i-- {
			if am, ok := rawAgs[i].(map[string]any); ok && strVal(am["Type"]) == "WFCoercionVariableAggrandizement" {
				return strVal(am["CoercionItemClass"]) == "WFDateContentItem"
			}
		}
	}
	return ctx.attachmentBaseValueType(att) == "Date"
}

func (ctx *semanticImportContext) renderConditionClausePair(params map[string]any) (semanticExprPair, bool) {
	condCode, okC := provInt(params["WFCondition"])
	if !okC {
		return semanticExprPair{}, false
	}
	rawInput, hasIn := params["WFInput"]
	if !hasIn {
		return semanticExprPair{}, false
	}
	inMap, okIn := rawInput.(map[string]any)
	if !okIn || len(inMap) != 2 {
		return semanticExprPair{}, false
	}
	if typ, _ := inMap["Type"].(string); typ != "Variable" {
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
	if condCode >= 0 && condCode <= 3 {
		inExpr = ctx.preserveNativeOperandReference(inVarRaw, inExpr)
	}

	switch condCode {
	case 100:
		if hasAnyConditionRHSKey(params) {
			return semanticExprPair{}, false
		}
		return semanticExprPair{Legacy: inExpr.Legacy, V2: inExpr.V2}, true
	case 101:
		if hasAnyConditionRHSKey(params) {
			return semanticExprPair{}, false
		}
		return semanticExprPair{Legacy: "!" + inExpr.Legacy, V2: "!" + inExpr.V2}, true
	case 1002:
		if hasAnyConditionRHSKey(params) {
			return semanticExprPair{}, false
		}
		return semanticExprPair{
			Legacy: fmt.Sprintf("%s isToday", inExpr.Legacy),
			V2:     fmt.Sprintf("isToday(%s)", inExpr.V2),
		}, true
	case 1003:
		if _, has := params["WFConditionalActionString"]; has {
			return semanticExprPair{}, false
		}
		if _, has := params["WFEnumeration"]; has {
			return semanticExprPair{}, false
		}
		if _, has := params["WFDuration"]; has {
			return semanticExprPair{}, false
		}
		d1Raw, hasDate1 := params["WFDate"]
		d2Raw, hasDate2 := params["WFAnotherDate"]
		n1Raw, hasNum1 := params["WFNumberValue"]
		n2Raw, hasNum2 := params["WFAnotherNumber"]
		if hasDate1 && hasDate2 && !hasNum1 && !hasNum2 && ctx.conditionInputIsDate(inVarRaw) {
			r1, ok1 := ctx.renderConditionDateRHS(d1Raw)
			r2, ok2 := ctx.renderConditionDateRHS(d2Raw)
			if !ok1 || !ok2 {
				return semanticExprPair{}, false
			}
			return semanticExprPair{
				Legacy: fmt.Sprintf("%s <> %s %s", inExpr.Legacy, r1.Legacy, r2.Legacy),
				V2:     fmt.Sprintf("isBetween(%s, %s, %s)", inExpr.V2, r1.V2, r2.V2),
			}, true
		}
		if hasDate1 || hasDate2 || !hasNum1 || !hasNum2 {
			return semanticExprPair{}, false
		}
		r1, ok1 := ctx.renderConditionNumberRHS(n1Raw)
		r2, ok2 := ctx.renderConditionNumberRHS(n2Raw)
		if !ok1 || !ok2 {
			return semanticExprPair{}, false
		}
		return semanticExprPair{
			Legacy: fmt.Sprintf("%s <> %s %s", inExpr.Legacy, r1.Legacy, r2.Legacy),
			V2:     fmt.Sprintf("isBetween(%s, %s, %s)", inExpr.V2, r1.V2, r2.V2),
		}, true
	}

	if _, hasAnotherNum := params["WFAnotherNumber"]; hasAnotherNum {
		return semanticExprPair{}, false
	}
	if _, hasAnotherDate := params["WFAnotherDate"]; hasAnotherDate {
		return semanticExprPair{}, false
	}
	if _, hasDate := params["WFDate"]; hasDate {
		return semanticExprPair{}, false
	}
	if _, hasDur := params["WFDuration"]; hasDur {
		return semanticExprPair{}, false
	}

	strCmp, hasS := params["WFConditionalActionString"]
	numCmp, hasN := params["WFNumberValue"]
	enumCmp, hasEn := params["WFEnumeration"]
	rhsCount := 0
	if hasS {
		rhsCount++
	}
	if hasN {
		rhsCount++
	}
	if hasEn {
		rhsCount++
	}
	if rhsCount > 1 {
		return semanticExprPair{}, false
	}
	if rhsCount == 0 {
		switch condCode {
		case 4:
			return semanticExprPair{
				Legacy: fmt.Sprintf("isTrue(%s)", inExpr.Legacy),
				V2:     fmt.Sprintf("isTrue(%s)", inExpr.V2),
			}, true
		case 5:
			return semanticExprPair{
				Legacy: fmt.Sprintf("isFalse(%s)", inExpr.Legacy),
				V2:     fmt.Sprintf("isFalse(%s)", inExpr.V2),
			}, true
		default:
			return semanticExprPair{}, false
		}
	}

	if condCode == 999 {
		if !hasS {
			return semanticExprPair{}, false
		}
		rhs, ok := ctx.renderAnyTextPair(strCmp, true)
		if !ok {
			return semanticExprPair{}, false
		}
		return semanticExprPair{
			Legacy: fmt.Sprintf("%s !contains %s", inExpr.Legacy, rhs.Legacy),
			V2:     fmt.Sprintf("!(%s contains %s)", inExpr.V2, rhs.V2),
		}, true
	}

	op, okOp := conditionBinaryOperator(condCode)
	if !okOp {
		return semanticExprPair{}, false
	}

	var rhsPair semanticExprPair
	switch {
	case hasEn:
		ev, okEv := enumCmp.(string)
		if !okEv {
			return semanticExprPair{}, false
		}
		q := fmt.Sprintf("condEnum(%s)", formatCherriDoubleQuotedString(ev))
		rhsPair = semanticExprPair{Legacy: q, V2: q}
	case hasS:
		rp, ok := ctx.renderAnyTextPair(strCmp, true)
		if !ok {
			return semanticExprPair{}, false
		}
		rhsPair = rp
	case hasN:
		rp, ok := ctx.renderConditionNumberRHS(numCmp)
		if !ok {
			return semanticExprPair{}, false
		}
		if condCode >= 0 && condCode <= 3 {
			rp = ctx.preserveNativeOperandReference(numCmp, rp)
		}
		rhsPair = rp
	}

	return semanticExprPair{
		Legacy: fmt.Sprintf("%s %s %s", inExpr.Legacy, op, rhsPair.Legacy),
		V2:     fmt.Sprintf("%s %s %s", inExpr.V2, op, rhsPair.V2),
	}, true
}

func (ctx *semanticImportContext) renderSingleConditionPair(params map[string]any) (semanticExprPair, bool) {
	rawLeg, hasLegKey := params["WFConditionalLegacyComparisonBehavior"]
	if hasLegKey && rawLeg != true {
		return semanticExprPair{}, false
	}
	hasLegacyFlag := hasLegKey && rawLeg == true
	legPrefix := "if "
	v2Prefix := "if modern: "
	if hasLegacyFlag {
		legPrefix = "if legacy "
		v2Prefix = "if "
	}

	if rawMulti, hasMulti := params["WFConditions"]; hasMulti {
		if _, has := params["WFCondition"]; has {
			return semanticExprPair{}, false
		}
		if _, has := params["WFInput"]; has {
			return semanticExprPair{}, false
		}
		if hasAnyConditionRHSKey(params) {
			return semanticExprPair{}, false
		}
		multiMap, okM := rawMulti.(map[string]any)
		if !okM || multiMap["WFSerializationType"] != "WFContentPredicateTableTemplate" {
			return semanticExprPair{}, false
		}
		valMap, okV := multiMap["Value"].(map[string]any)
		if !okV {
			return semanticExprPair{}, false
		}
		prefixCode, okP := provInt(valMap["WFActionParameterFilterPrefix"])
		if !okP || (prefixCode != 0 && prefixCode != 1) {
			return semanticExprPair{}, false
		}
		joiner := " || "
		if prefixCode == 1 {
			joiner = " && "
		}
		rawTmpls, okT := valMap["WFActionParameterFilterTemplates"].([]any)
		if !okT || len(rawTmpls) == 0 {
			return semanticExprPair{}, false
		}
		legParts := make([]string, len(rawTmpls))
		v2Parts := make([]string, len(rawTmpls))
		allowedTmpl := map[string]bool{
			"WFCondition": true, "WFInput": true,
			"WFConditionalActionString": true, "WFNumberValue": true, "WFAnotherNumber": true,
			"WFDate": true, "WFAnotherDate": true, "WFEnumeration": true, "WFDuration": true,
		}
		for i, rawT := range rawTmpls {
			tMap, okTM := rawT.(map[string]any)
			if !okTM {
				return semanticExprPair{}, false
			}
			for k := range tMap {
				if !allowedTmpl[k] {
					return semanticExprPair{}, false
				}
			}
			cp, okCP := ctx.renderConditionClausePair(tMap)
			if !okCP {
				return semanticExprPair{}, false
			}
			legParts[i] = cp.Legacy
			v2Parts[i] = cp.V2
		}
		return semanticExprPair{
			Legacy: legPrefix + strings.Join(legParts, joiner) + " {",
			V2:     v2Prefix + strings.Join(v2Parts, joiner) + " {",
		}, true
	}

	clausePair, ok := ctx.renderConditionClausePair(params)
	if !ok {
		return semanticExprPair{}, false
	}
	return semanticExprPair{
		Legacy: legPrefix + clausePair.Legacy + " {",
		V2:     v2Prefix + clausePair.V2 + " {",
	}, true
}

func (ctx *semanticImportContext) trySynthesizeMenuRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	ctx.pendingLegacyDecls = nil
	defer func() {
		ctx.pendingLegacyDecls = nil
	}()
	startRec := ctx.Doc.Actions[reg.Start]
	endRec := ctx.Doc.Actions[reg.End]
	if !startRec.HadParametersKey || !endRec.HadParametersKey {
		return
	}
	if !ctx.isControlBranchProducerSafe(startRec) {
		return
	}
	endAssignLeg, endAssignV2, okEnd := ctx.controlEndAssignPrefix(endRec)
	if !okEnd {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(startRec.Parameters, map[string]bool{
		"GroupingIdentifier": true, "WFControlFlowMode": true,
		"WFMenuPrompt": true, "WFMenuItems": true, "WFMenuLegacyCancelBehavior": true,
	}) {
		return
	}
	if !ctx.hasOnlyAllowedExtraKeys(endRec.Parameters, map[string]bool{"GroupingIdentifier": true, "WFControlFlowMode": true}) {
		return
	}

	legacyCancel := false
	if rawLC, hasLC := startRec.Parameters["WFMenuLegacyCancelBehavior"]; hasLC {
		b, isB := rawLC.(bool)
		if !isB || !b {
			return
		}
		legacyCancel = true
	}

	rawPrompt, hasPrompt := startRec.Parameters["WFMenuPrompt"]
	var legHeader, v2Header string
	switch {
	case !hasPrompt:
		if legacyCancel {
			legHeader = "menu legacyCancel {"
			v2Header = "menu(legacyCancel: true) {"
		} else {
			legHeader = "menu {"
			v2Header = "menu {"
		}
	default:
		if ps, okStr := rawPrompt.(string); okStr && ps == "" {
			if legacyCancel {
				legHeader = `menu legacyCancel "" {`
				v2Header = `menu("", legacyCancel: true) {`
			} else {
				legHeader = `menu "" {`
				v2Header = `menu("") {`
			}
		} else {
			promptPair, okP := ctx.renderAnyTextPair(rawPrompt, false)
			if !okP {
				return
			}
			if legacyCancel {
				legHeader = fmt.Sprintf("menu legacyCancel %s {", promptPair.Legacy)
				v2Header = fmt.Sprintf("menu(%s, legacyCancel: true) {", promptPair.V2)
			} else {
				legHeader = fmt.Sprintf("menu %s {", promptPair.Legacy)
				if strings.HasPrefix(promptPair.V2, `"`) {
					v2Header = fmt.Sprintf("menu %s {", promptPair.V2)
				} else {
					v2Header = fmt.Sprintf("menu (%s) {", promptPair.V2)
				}
			}
		}
	}

	rawItems, hasItems := startRec.Parameters["WFMenuItems"].([]any)
	if !hasItems || len(rawItems) != len(reg.Mids) || len(reg.Mids) == 0 || reg.Mids[0] != reg.Start+1 {
		return
	}
	caseHeaders := make([]semanticExprPair, len(reg.Mids))
	for ci, mIdx := range reg.Mids {
		mRec := ctx.Doc.Actions[mIdx]
		if !mRec.HadParametersKey || !ctx.isControlBranchProducerSafe(mRec) || !ctx.hasOnlyAllowedExtraKeys(mRec.Parameters, map[string]bool{
			"GroupingIdentifier": true, "WFControlFlowMode": true, "WFMenuItemTitle": true, "WFMenuItemAttributedTitle": true,
		}) {
			return
		}
		title, ok := mRec.Parameters["WFMenuItemTitle"].(string)
		if !ok {
			return
		}
		attr, hasAttr := mRec.Parameters["WFMenuItemAttributedTitle"]

		if rawStr, isRawStr := rawItems[ci].(string); isRawStr {
			if hasAttr {
				return
			}
			vp, okVP := ctx.renderStringPair(rawStr)
			if !okVP {
				return
			}
			if rawStr == title {
				caseHeaders[ci] = semanticExprPair{
					Legacy: fmt.Sprintf("item %s plain:", vp.Legacy),
					V2:     fmt.Sprintf("case %s plain:", vp.V2),
				}
			} else {
				tp, okTP := ctx.renderStringPair(title)
				if !okTP {
					return
				}
				caseHeaders[ci] = semanticExprPair{
					Legacy: fmt.Sprintf("item %s title %s plain:", vp.Legacy, tp.Legacy),
					V2:     fmt.Sprintf("case %s title %s plain:", vp.V2, tp.V2),
				}
			}
			continue
		}

		itemMap, okIM := rawItems[ci].(map[string]any)
		if !okIM || len(itemMap) != 2 {
			return
		}
		itType, okT := provInt(itemMap["WFItemType"])
		if !okT || itType != 0 {
			return
		}
		rawVal, hasVal := itemMap["WFValue"]
		if !hasVal {
			return
		}

		if valStr, isValStr := rawVal.(string); isValStr {
			if title == "" || !hasAttr || attr != title || valStr != title {
				return
			}
			tp, okTP := ctx.renderStringPair(title)
			if !okTP {
				return
			}
			caseHeaders[ci] = semanticExprPair{
				Legacy: fmt.Sprintf("item %s:", tp.Legacy),
				V2:     fmt.Sprintf("case %s:", tp.V2),
			}
			continue
		}

		valMap, isValMap := rawVal.(map[string]any)
		if !isValMap || valMap["WFSerializationType"] != "WFTextTokenString" {
			return
		}
		valInner, okInner := valMap["Value"].(map[string]any)
		if !okInner {
			return
		}
		atts, _ := valInner["attachmentsByRange"].(map[string]any)
		if len(atts) == 0 {
			return
		}
		attrMap, okAttrMap := attr.(map[string]any)
		if !hasAttr || !okAttrMap || !reflect.DeepEqual(attrMap, valInner) {
			return
		}
		vp, okVP := ctx.renderAnyTextPair(valMap, false)
		if !okVP || !strings.HasPrefix(vp.Legacy, `"`) || (!strings.HasPrefix(vp.V2, `"`) && !strings.HasPrefix(vp.V2, `f"`)) {
			return
		}
		tp, okTP := ctx.renderStringPair(title)
		if !okTP {
			return
		}
		caseHeaders[ci] = semanticExprPair{
			Legacy: fmt.Sprintf("item %s title %s:", vp.Legacy, tp.Legacy),
			V2:     fmt.Sprintf("case %s title %s:", vp.V2, tp.V2),
		}
	}

	plan.Decisions[reg.Start].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.Start], ctx.flushPendingLegacyDecls(endAssignLeg+legHeader), endAssignV2+v2Header, "menu", "")

	for ci, mIdx := range reg.Mids {
		plan.Decisions[mIdx].RegionGroupID = reg.GroupID
		applyDecisionCandidate(
			&plan.Decisions[mIdx],
			caseHeaders[ci].Legacy,
			caseHeaders[ci].V2,
			"menu", "",
		)
	}

	plan.Decisions[reg.End].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.End], "}", "}", "menu", "")
}

func (ctx *semanticImportContext) trySynthesizeRepeatCountRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	ctx.pendingLegacyDecls = nil
	defer func() {
		ctx.pendingLegacyDecls = nil
	}()
	if len(reg.Mids) != 0 {
		return
	}
	startRec := ctx.Doc.Actions[reg.Start]
	endRec := ctx.Doc.Actions[reg.End]
	if !startRec.HadParametersKey || !endRec.HadParametersKey {
		return
	}
	if !ctx.isControlBranchProducerSafe(startRec) {
		return
	}
	endAssignLeg, endAssignV2, okEnd := ctx.controlEndAssignPrefix(endRec)
	if !okEnd {
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
	var cntPair semanticExprPair
	if cntRaw, hasCnt := startRec.Parameters["WFRepeatCount"]; !hasCnt {
		cntPair = semanticExprPair{Legacy: "none", V2: "none"}
	} else {
		p, ok := ctx.renderNumberOrAttachmentPair(cntRaw)
		if !ok {
			return
		}
		cntPair = p
	}

	plan.Decisions[reg.Start].RegionGroupID = reg.GroupID
	applyDecisionCandidate(
		&plan.Decisions[reg.Start],
		ctx.flushPendingLegacyDecls(fmt.Sprintf("%srepeat _ for %s {", endAssignLeg, cntPair.Legacy)),
		fmt.Sprintf("%srepeat _ for %s {", endAssignV2, cntPair.V2),
		"repeat", "",
	)
	plan.Decisions[reg.End].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.End], "}", "}", "repeat", "")
}

func (ctx *semanticImportContext) trySynthesizeRepeatEachRegion(reg *semanticControlRegion, plan *HybridImportPlan) {
	ctx.pendingLegacyDecls = nil
	defer func() {
		ctx.pendingLegacyDecls = nil
	}()
	if len(reg.Mids) != 0 {
		return
	}
	startRec := ctx.Doc.Actions[reg.Start]
	endRec := ctx.Doc.Actions[reg.End]
	if !startRec.HadParametersKey || !endRec.HadParametersKey {
		return
	}
	if !ctx.isControlBranchProducerSafe(startRec) {
		return
	}
	endAssignLeg, endAssignV2, okEnd := ctx.controlEndAssignPrefix(endRec)
	if !okEnd {
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
		ctx.flushPendingLegacyDecls(fmt.Sprintf("%sfor _ in %s {", endAssignLeg, inPair.Legacy)),
		fmt.Sprintf("%sfor _ in %s {", endAssignV2, inPair.V2),
		"repeat.each", "",
	)
	plan.Decisions[reg.End].RegionGroupID = reg.GroupID
	applyDecisionCandidate(&plan.Decisions[reg.End], "}", "}", "repeat.each", "")
}

func (ctx *semanticImportContext) captureCandidateSnapshot(plan *HybridImportPlan) {
	ctx.candidateActionMarks = make(map[int]string)
	seenRegion := make(map[string]bool)
	for i, dec := range plan.Decisions {
		if isHybridNaturalClassification(dec.Classification) {
			ctx.candidateActionMarks[i] = dec.RegionGroupID
			if dec.RegionGroupID != "" && !seenRegion[dec.RegionGroupID] {
				seenRegion[dec.RegionGroupID] = true
				ctx.candidateRegionOrder = append(ctx.candidateRegionOrder, dec.RegionGroupID)
			}
		}
	}
}

func countActiveNaturalCandidates(plan *HybridImportPlan) int {
	n := 0
	for _, dec := range plan.Decisions {
		if isHybridNaturalClassification(dec.Classification) {
			n++
		}
	}
	return n
}

func (ctx *semanticImportContext) enforceAtomicRegions(plan *HybridImportPlan) {
	ctx.enforceAtomicRegionsWithStage(plan, "ATOMIC_REGION", "ATOMIC_REGION_PARTIAL")
}

func (ctx *semanticImportContext) enforceAtomicRegionsWithStage(plan *HybridImportPlan, stage, demoteReason string) {
	byGroup := make(map[string][]int)
	for i, dec := range plan.Decisions {
		if dec.RegionGroupID != "" {
			byGroup[dec.RegionGroupID] = append(byGroup[dec.RegionGroupID], i)
		}
	}
	for _, idxs := range byGroup {
		allNatural := true
		for _, idx := range idxs {
			if !isHybridNaturalClassification(plan.Decisions[idx].Classification) {
				allNatural = false
				break
			}
		}
		if !allNatural {
			for _, idx := range idxs {
				if isHybridNaturalClassification(plan.Decisions[idx].Classification) {
					ctx.demoteSingleIndexToFallbackWithStage(plan, idx, "CONTROL_REGION", stage, demoteReason)
				} else {
					plan.Decisions[idx].RegionGroupID = ""
				}
			}
		}
	}
}

// rejectUnseededForwardActionOutputCandidates fails closed at Stage 1 for any natural
// candidate that reads an ActionOutput UUID whose producer is at a later (or equal) index
// and remains a natural candidate (so ProducerRaw is false and the legacy const binding is not yet in scope).
func (ctx *semanticImportContext) rejectUnseededForwardActionOutputCandidates(plan *HybridImportPlan) {
	if ctx == nil || ctx.Doc == nil || plan == nil || len(ctx.Doc.ConsumerUUIDIndex) == 0 {
		return
	}
	uuidsSorted := make([]string, 0, len(ctx.Doc.ConsumerUUIDIndex))
	for u := range ctx.Doc.ConsumerUUIDIndex {
		uuidsSorted = append(uuidsSorted, u)
	}
	sort.Strings(uuidsSorted)

	revertAsStage1 := func(idx int, reason string) {
		ctx.demoteSingleIndexToFallbackWithStage(plan, idx, reason, "", "")
		plan.Decisions[idx].CandidateStatus = "NOT_CANDIDATE"
		plan.Decisions[idx].DemotionStage = ""
		plan.Decisions[idx].DemotionReason = ""
	}

	changed := true
	for changed {
		changed = false
		for _, u := range uuidsSorted {
			prodIdx, hasProd := ctx.Doc.ProducerUUIDIndex[u]
			if !hasProd || prodIdx < 0 || prodIdx >= len(plan.Decisions) {
				continue
			}
			if !isHybridNaturalClassification(plan.Decisions[prodIdx].Classification) {
				continue
			}
			consumers := ctx.Doc.ConsumerUUIDIndex[u]
			for _, cIdx := range consumers {
				if cIdx < 0 || cIdx >= len(plan.Decisions) || cIdx > prodIdx {
					continue
				}
				if !isHybridNaturalClassification(plan.Decisions[cIdx].Classification) {
					continue
				}
				gid := plan.Decisions[cIdx].RegionGroupID
				if gid != "" {
					for j := range plan.Decisions {
						if plan.Decisions[j].RegionGroupID != gid {
							continue
						}
						if isHybridNaturalClassification(plan.Decisions[j].Classification) {
							revertAsStage1(j, "CONTROL_REGION")
							changed = true
						} else {
							plan.Decisions[j].RegionGroupID = ""
						}
					}
				} else {
					revertAsStage1(cIdx, "UNREPRESENTABLE_PARAM")
					changed = true
				}
			}
		}
	}
}

func (ctx *semanticImportContext) demoteSingleIndexToFallback(plan *HybridImportPlan, idx int, reason string) {
	ctx.demoteSingleIndexToFallbackWithStage(plan, idx, reason, "PLANNER_DEMOTE", "UNSPECIFIED_DEMOTE")
}

func (ctx *semanticImportContext) demoteSingleIndexToFallbackWithStage(plan *HybridImportPlan, idx int, reason, stage, demoteReason string) {
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
	wasCandidate := isHybridNaturalClassification(plan.Decisions[idx].Classification) || plan.Decisions[idx].CandidateStatus == "CANDIDATE"
	plan.Decisions[idx].Classification = "EXACT_NATIVE_FALLBACK"
	if wasCandidate {
		plan.Decisions[idx].CandidateStatus = "DEMOTED"
		plan.Decisions[idx].DemotionStage = stage
		plan.Decisions[idx].DemotionReason = demoteReason
	}
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

func (ctx *semanticImportContext) promoteVerifiedCandidatesAndBuildLifecycle(plan *HybridImportPlan) {
	demotionReasons := make(map[string]int)
	verifiedCount := 0
	demotedCount := 0
	verifiedRegions := make(map[string]bool)
	for i := range plan.Decisions {
		dec := &plan.Decisions[i]
		if isHybridNaturalClassification(dec.Classification) {
			dec.Classification = "NATURAL_VERIFIED"
			dec.CandidateStatus = "VERIFIED"
			dec.DemotionStage = ""
			dec.DemotionReason = ""
			verifiedCount++
			if dec.RegionGroupID != "" {
				verifiedRegions[dec.RegionGroupID] = true
			}
		} else if dec.CandidateStatus == "DEMOTED" {
			demotedCount++
			key := dec.DemotionStage + ":" + dec.DemotionReason
			demotionReasons[key]++
		}
	}

	transitions := make([]HybridUnitTransition, 0, len(ctx.candidateRegionOrder)+len(plan.Decisions))
	for _, gid := range ctx.candidateRegionOrder {
		reg := ctx.RegionsByGroup[gid]
		if reg == nil {
			continue
		}
		memberIdxs := append([]int{reg.Start}, reg.Mids...)
		memberIdxs = append(memberIdxs, reg.End)
		finalState := "VERIFIED"
		finalStage := "FINAL_DUAL_VERIFY"
		reason := ""
		if !verifiedRegions[gid] {
			finalState = "DEMOTED"
			for _, idx := range memberIdxs {
				if plan.Decisions[idx].DemotionStage != "" {
					finalStage = plan.Decisions[idx].DemotionStage
					reason = plan.Decisions[idx].DemotionReason
					break
				}
			}
		}
		transitions = append(transitions, HybridUnitTransition{
			UnitID:           "region:" + gid,
			Kind:             "region",
			RegionGroupID:    gid,
			StartIndex:       reg.Start,
			EndIndex:         reg.End,
			ActionCount:      len(memberIdxs),
			AppleIdentifier:  "is.workflow.actions." + reg.Kind,
			CandidateStage:   "CANDIDATE",
			FinalState:       finalState,
			FinalStage:       finalStage,
			TransitionReason: reason,
		})
	}
	for i, dec := range plan.Decisions {
		if initGID, wasCand := ctx.candidateActionMarks[i]; wasCand && initGID == "" {
			finalState := "VERIFIED"
			finalStage := "FINAL_DUAL_VERIFY"
			reason := ""
			if dec.Classification != "NATURAL_VERIFIED" {
				finalState = "DEMOTED"
				finalStage = dec.DemotionStage
				reason = dec.DemotionReason
			}
			transitions = append(transitions, HybridUnitTransition{
				UnitID:           fmt.Sprintf("action:%d", i),
				Kind:             "action",
				StartIndex:       i,
				EndIndex:         i,
				ActionCount:      1,
				AppleIdentifier:  dec.AppleIdentifier,
				CandidateStage:   "CANDIDATE",
				FinalState:       finalState,
				FinalStage:       finalStage,
				TransitionReason: reason,
			})
		}
	}

	verifiedRegionCount := len(verifiedRegions)
	demotedRegionCount := len(ctx.candidateRegionOrder) - verifiedRegionCount
	if len(demotionReasons) == 0 {
		demotionReasons = nil
	}
	plan.Lifecycle = &HybridPlanLifecycleSummary{
		CandidateActionCount: len(ctx.candidateActionMarks),
		PostAtomicCount:      ctx.postAtomicActionCnt,
		VerifiedActionCount:  verifiedCount,
		DemotedActionCount:   demotedCount,
		DemotionReasons:      demotionReasons,
		CandidateRegionCount: len(ctx.candidateRegionOrder),
		VerifiedRegionCount:  verifiedRegionCount,
		DemotedRegionCount:   demotedRegionCount,
		Transitions:          transitions,
	}
	recountHybridPlanStats(plan)
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
		if isHybridNaturalClassification(dec.Classification) && dec.RegionGroupID != "" {
			groupIndices[dec.RegionGroupID] = append(groupIndices[dec.RegionGroupID], i)
		}
	}
	var units []semanticVerifyUnit
	for i, dec := range plan.Decisions {
		if !isHybridNaturalClassification(dec.Classification) {
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
			ctx.promoteVerifiedCandidatesAndBuildLifecycle(plan)
			return nil
		}

		changedLeg := false
		if !plistValuesStrictlyEqual(ctx.Doc.RawRoot, legMap) {
			changedLeg = demoteMismatchedHybridActionsWithStage(ctx.Doc, plan, legMap, "WIRE_VERIFY_LEGACY")
		}
		changedV2 := false
		if !changedLeg && !plistValuesStrictlyEqual(ctx.Doc.RawRoot, v2Map) {
			changedV2 = demoteMismatchedHybridActionsWithStage(ctx.Doc, plan, v2Map, "WIRE_VERIFY_V2")
		}
		if !changedLeg && !changedV2 {
			// Neither side found an obvious natural mismatch to demote; demote remaining natural units greedily.
			units := collectNaturalUnits(plan)
			if len(units) == 0 {
				return fmt.Errorf("hybrid import failed fail-closed parity check with zero natural actions remaining")
			}
			ctx.demoteUnitWithStage(plan, units[len(units)-1], "GREEDY_STABILIZE", "GREEDY_WIRE_STABILIZE")
		}
		ctx.enforceAtomicRegionsWithStage(plan, "ATOMIC_REGION", "ATOMIC_REGION_CASCADE")
		recountHybridPlanStats(plan)
	}
	return fmt.Errorf("hybrid verification did not converge within %d iterations", maxIters)
}

func (ctx *semanticImportContext) demoteUnit(plan *HybridImportPlan, u semanticVerifyUnit) {
	ctx.demoteUnitWithStage(plan, u, "PLANNER_DEMOTE", "UNIT_DEMOTE")
}

func (ctx *semanticImportContext) demoteUnitWithStage(plan *HybridImportPlan, u semanticVerifyUnit, stage, demoteReason string) {
	reason := ""
	if u.groupID != "" {
		reason = "CONTROL_REGION"
	}
	for _, idx := range u.indices {
		ctx.demoteSingleIndexToFallbackWithStage(plan, idx, reason, stage, demoteReason)
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
			if isHybridNaturalClassification(dec.Classification) && dec.IncludeCategory != "" {
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
		isControl := isHybridNaturalClassification(dec.Classification) && dec.RegionGroupID != ""
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
			if !found || aIdx < 0 || aIdx >= len(plan.Decisions) || !isHybridNaturalClassification(plan.Decisions[aIdx].Classification) {
				break
			}
			if rg := plan.Decisions[aIdx].RegionGroupID; rg != "" {
				for i := range plan.Decisions {
					if plan.Decisions[i].RegionGroupID == rg {
						ctx.demoteSingleIndexToFallbackWithStage(plan, i, "CONTROL_REGION", "LEGACY_COMPILE", "LEGACY_LINE_COMPILE_ERROR")
					}
				}
			} else {
				ctx.demoteSingleIndexToFallbackWithStage(plan, aIdx, "", "LEGACY_COMPILE", "LEGACY_LINE_COMPILE_ERROR")
			}
			ctx.enforceAtomicRegionsWithStage(plan, "ATOMIC_REGION", "ATOMIC_REGION_LEGACY_CASCADE")
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
					if isHybridNaturalClassification(plan.Decisions[aIdx].Classification) {
						if rg := plan.Decisions[aIdx].RegionGroupID; rg != "" {
							for i := range plan.Decisions {
								if plan.Decisions[i].RegionGroupID == rg {
									ctx.demoteSingleIndexToFallbackWithStage(plan, i, "CONTROL_REGION", "V2_COMPILE", "V2_LINE_COMPILE_ERROR")
								}
							}
						} else {
							ctx.demoteSingleIndexToFallbackWithStage(plan, aIdx, "", "V2_COMPILE", "V2_LINE_COMPILE_ERROR")
						}
						demotedAny = true
					}
				}
			}
			if !demotedAny {
				break
			}
			ctx.enforceAtomicRegionsWithStage(plan, "ATOMIC_REGION", "ATOMIC_REGION_V2_CASCADE")
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

	// Binary culprit isolation (QuickXplain-style): greedily keep maximal compile-clean
	// prefixes/slices in O(k log N) compiles instead of linear per-unit demotions.
	keep := make([]bool, len(units))
	var addGoodUnits func(kept []int, cands []int, skipFullCheck bool) []int
	addGoodUnits = func(kept []int, cands []int, skipFullCheck bool) []int {
		if len(cands) == 0 {
			return kept
		}
		if !skipFullCheck {
			trialIdxs := make([]int, 0, len(kept)+len(cands))
			trialIdxs = append(trialIdxs, kept...)
			trialIdxs = append(trialIdxs, cands...)
			trial := ctx.planWithSubsetUnits(plan, units, trialIdxs)
			if ctx.testPlanCompiles(trial, legacy) {
				for _, ui := range cands {
					keep[ui] = true
				}
				return trialIdxs
			}
		}
		if len(cands) == 1 {
			return kept
		}
		mid := len(cands) / 2
		kept = addGoodUnits(kept, cands[:mid], false)
		kept = addGoodUnits(kept, cands[mid:], false)
		return kept
	}

	all := make([]int, len(units))
	for i := range units {
		all[i] = i
	}
	kept := addGoodUnits(nil, all, true)

	for ui, ok := range keep {
		if !ok {
			if legacy {
				ctx.demoteUnitWithStage(plan, units[ui], "ISOLATION_BISECT", "LEGACY_BISECT_COMPILE_ERROR")
			} else {
				ctx.demoteUnitWithStage(plan, units[ui], "ISOLATION_BISECT", "V2_BISECT_COMPILE_ERROR")
			}
		}
	}
	ctx.enforceAtomicRegionsWithStage(plan, "ATOMIC_REGION", "ATOMIC_REGION_BISECT_CASCADE")
	recountHybridPlanStats(plan)

	if compiled, ok := ctx.compilePlanToRawMap(plan, legacy); ok {
		return compiled, nil
	}

	// Defensive fallback if an unexpected cross-region cascade invalidated kept units.
	_ = kept
	active := collectNaturalUnits(plan)
	if len(active) > 0 {
		// Re-run binary isolation on the post-cascade active units with full check enabled.
		keep = make([]bool, len(active))
		units = active
		activeAll := make([]int, len(active))
		for i := range active {
			activeAll[i] = i
		}
		addGoodUnits(nil, activeAll, false)
		for ui, ok := range keep {
			if !ok {
				if legacy {
					ctx.demoteUnitWithStage(plan, active[ui], "ISOLATION_BISECT", "LEGACY_INTERACTION_DEMOTE")
				} else {
					ctx.demoteUnitWithStage(plan, active[ui], "ISOLATION_BISECT", "V2_INTERACTION_DEMOTE")
				}
			}
		}
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
		if isHybridNaturalClassification(trial.Decisions[i].Classification) && !keepAction[i] {
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
