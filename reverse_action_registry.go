/*
 * Copyright (c) Cherri
 * Deterministic Reverse Action Registry (P10R9)
 */

package main

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
)

// ReverseActionCandidate represents a canonical Cherri action definition indexed for reverse lookup
// from a native Shortcut action record.
type ReverseActionCandidate struct {
	CallableName          string
	AppleIdentifier       string
	IncludeCategory       string
	Definition            actionDefinition
	StaticParameters      map[string]any
	AppIntent             appIntent
	ParameterKeys         []string
	RequiredParameterKeys []string
	HasCustomMakeParams   bool
	HasCustomDecomp       bool
}

// ReverseActionRegistry indexes canonical Cherri action definitions by full Apple identifier
// with deterministic candidate ordering.
type ReverseActionRegistry struct {
	ByAppleIdentifier      map[string][]ReverseActionCandidate
	ByCallableName         map[string]ReverseActionCandidate
	SortedAppleIdentifiers []string
}

// ReverseResolutionOutcome reports the deterministic classification and chosen candidate (if any)
// for a single NativeActionRecord.
type ReverseResolutionOutcome struct {
	ReasonCode           string
	ChosenCandidate      *ReverseActionCandidate
	CompatibleCandidates []ReverseActionCandidate
	AllCandidates        []ReverseActionCandidate
	Detail               string
}

var (
	defaultReverseRegistryOnce sync.Once
	defaultReverseRegistry     *ReverseActionRegistry
)

// DefaultReverseActionRegistry returns the process-wide deterministic reverse registry built
// from EnsureCanonicalStandardActions().
func DefaultReverseActionRegistry() *ReverseActionRegistry {
	defaultReverseRegistryOnce.Do(func() {
		defaultReverseRegistry = BuildReverseActionRegistryFromMap(EnsureCanonicalStandardActions())
	})
	return defaultReverseRegistry
}

// BuildReverseActionRegistryFromMap constructs a deterministic ReverseActionRegistry from an
// actionDefinition map without relying on Go map iteration order.
func BuildReverseActionRegistryFromMap(defs map[string]*actionDefinition) *ReverseActionRegistry {
	names := make([]string, 0, len(defs))
	for name, def := range defs {
		if def == nil || name == "rawAction" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	byAppleID := make(map[string][]ReverseActionCandidate)
	byName := make(map[string]ReverseActionCandidate, len(names))

	for _, name := range names {
		def := defs[name]
		if def == nil {
			continue
		}
		appleID := catalogShortcutIdentifier(name, def)
		incCat := def.includeCategory
		if incCat == "" && !def.builtin && def.doc.category != "" && def.doc.category != "basic" && slices.Contains(actionIncludes, def.doc.category) {
			incCat = def.doc.category
		}
		if incCat == "basic" {
			incCat = ""
		}
		if incCat == "" && (name == "editEvent" || name == "editReminder" || name == "updateContact") && def.doc.category != "" {
			incCat = def.doc.category
		}

		var staticParams map[string]any
		if len(def.appendParams) > 0 {
			staticParams = maps.Clone(def.appendParams)
		}

		paramKeys := make([]string, 0, len(def.parameters))
		reqKeys := make([]string, 0, len(def.parameters))
		for _, p := range def.parameters {
			if p.key != "" {
				paramKeys = append(paramKeys, p.key)
				if !p.optional && p.defaultValue == nil {
					reqKeys = append(reqKeys, p.key)
				}
			}
		}

		cand := ReverseActionCandidate{
			CallableName:          name,
			AppleIdentifier:       appleID,
			IncludeCategory:       incCat,
			Definition:            *def,
			StaticParameters:      staticParams,
			AppIntent:             def.appIntent,
			ParameterKeys:         paramKeys,
			RequiredParameterKeys: reqKeys,
			HasCustomMakeParams:   def.makeParams != nil || def.appendParamsFunc != nil,
			HasCustomDecomp:       def.decomp != nil,
		}
		byName[name] = cand
		byAppleID[appleID] = append(byAppleID[appleID], cand)
	}

	sortedIDs := make([]string, 0, len(byAppleID))
	for appleID, list := range byAppleID {
		sort.Slice(list, func(i, j int) bool {
			return list[i].CallableName < list[j].CallableName
		})
		byAppleID[appleID] = list
		sortedIDs = append(sortedIDs, appleID)
	}
	sort.Strings(sortedIDs)

	return &ReverseActionRegistry{
		ByAppleIdentifier:      byAppleID,
		ByCallableName:         byName,
		SortedAppleIdentifiers: sortedIDs,
	}
}

func isControlFlowNativeAction(rec NativeActionRecord) bool {
	if rec.ControlFlowMode != nil || rec.GroupingIdentifier != "" {
		return true
	}
	switch rec.Identifier {
	case "is.workflow.actions.conditional",
		"is.workflow.actions.choosefrommenu",
		"is.workflow.actions.repeat.count",
		"is.workflow.actions.repeat.each":
		return true
	default:
		return false
	}
}

func isCompilerConstructOnlyIdentifier(ident string) bool {
	switch ident {
	case "is.workflow.actions.setvariable",
		"is.workflow.actions.appendvariable",
		"is.workflow.actions.dictionary",
		"is.workflow.actions.calculateexpression":
		return true
	default:
		return false
	}
}

func hasSerializedTokenOrAttachment(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		if st, ok := x["WFSerializationType"].(string); ok {
			switch st {
			case "WFTextTokenAttachment", "WFTextTokenString", "WFDictionaryFieldValue", "WFContentPredicateTableTemplate":
				if st == "WFTextTokenString" {
					if valMap, ok := x["Value"].(map[string]any); ok {
						if att, hasAtt := valMap["attachmentsByRange"].(map[string]any); hasAtt && len(att) > 0 {
							return true
						}
						return true
					}
				}
				return true
			}
		}
		if _, hasOutUUID := x["OutputUUID"]; hasOutUUID {
			return true
		}
		if _, hasVarName := x["VariableName"]; hasVarName {
			return true
		}
		if tStr, ok := x["Type"].(string); ok && (tStr == "ActionOutput" || tStr == "Variable" || tStr == "ExtensionInput" || tStr == "CurrentDate" || tStr == "Clipboard" || tStr == "DeviceDetails" || tStr == "Ask") {
			return true
		}
		for _, val := range x {
			if hasSerializedTokenOrAttachment(val) {
				return true
			}
		}
		return false
	case []any:
		for _, item := range x {
			if hasSerializedTokenOrAttachment(item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// ResolveReverseActionCandidate deterministically resolves a NativeActionRecord against the
// ReverseActionRegistry and classifies its candidacy or fallback reason.
func (reg *ReverseActionRegistry) ResolveReverseActionCandidate(rec NativeActionRecord) ReverseResolutionOutcome {
	if rec.SourceValidity != "" && rec.SourceValidity != "VALID" {
		return ReverseResolutionOutcome{
			ReasonCode: "SOURCE_INVALID",
			Detail:     rec.SourceValidity,
		}
	}

	if isControlFlowNativeAction(rec) {
		return ReverseResolutionOutcome{
			ReasonCode: "CONTROL_REGION",
			Detail:     "control-flow marker action requires region-level or raw preservation",
		}
	}

	allCands := reg.ByAppleIdentifier[rec.Identifier]
	if len(allCands) == 0 {
		if isCompilerConstructOnlyIdentifier(rec.Identifier) {
			return ReverseResolutionOutcome{
				ReasonCode: "UNREPRESENTABLE_PARAM",
				Detail:     fmt.Sprintf("%s is a compiler construct with variable/expression dependencies", rec.Identifier),
			}
		}
		if strings.HasPrefix(rec.Identifier, "is.workflow.actions.") {
			return ReverseResolutionOutcome{
				ReasonCode: "UNKNOWN_NATIVE_ID",
				Detail:     fmt.Sprintf("uncatalogued native identifier %s", rec.Identifier),
			}
		}
		return ReverseResolutionOutcome{
			ReasonCode: "THIRD_PARTY_NO_CANONICAL_DEF",
			Detail:     fmt.Sprintf("third-party identifier %s has no canonical definition", rec.Identifier),
		}
	}

	// Any action producing a UUID or CustomOutputName, or referencing upstream UUIDs/variables
	// or serialized token attachments, cannot be emitted as a standalone statement call without
	// altering the reference graph or dropping output UUID metadata.
	if rec.UUID != "" || rec.CustomOutputName != "" || len(rec.ReferencedUUIDs) > 0 || len(rec.ReferencedVars) > 0 || hasSerializedTokenOrAttachment(rec.Parameters) {
		return ReverseResolutionOutcome{
			ReasonCode:    "UNREPRESENTABLE_PARAM",
			AllCandidates: allCands,
			Detail:        "action carries producer UUID/CustomOutputName or serialized variable/attachment references",
		}
	}

	type scoredCandidate struct {
		cand              ReverseActionCandidate
		matchedStaticKeys int
		matchedParamKeys  int
		extraKeys         int
		missingRequired   int
	}

	var compatible []scoredCandidate
	for _, cand := range allCands {
		// 1. Check AppIntentDescriptor compatibility if definition or record has an AppIntent
		if cand.AppIntent != emptyAppIntent {
			if rec.AppIntentDescriptor == nil {
				continue
			}
			bundleID, _ := rec.AppIntentDescriptor["BundleIdentifier"].(string)
			intentID, _ := rec.AppIntentDescriptor["AppIntentIdentifier"].(string)
			if bundleID != cand.AppIntent.bundleIdentifier || intentID != cand.AppIntent.appIntentIdentifier {
				continue
			}
		} else if rec.AppIntentDescriptor != nil {
			continue
		}

		// 2. Check StaticParameters (appendParams) compatibility:
		// Every static parameter key required by cand.StaticParameters MUST exist in rec.Parameters
		// with an equal value.
		staticViolated := false
		matchedStatic := 0
		for sk, expectedVal := range cand.StaticParameters {
			actualVal, exists := rec.Parameters[sk]
			if !exists || !plistValuesStrictlyEqual(actualVal, expectedVal) {
				staticViolated = true
				break
			}
			matchedStatic++
		}
		if staticViolated {
			continue
		}

		// 3. Check parameter key coverage vs extra unknown keys
		matchedParams := 0
		extraKeys := 0
		for pk := range rec.Parameters {
			if pk == "UUID" || pk == "CustomOutputName" || pk == "GroupingIdentifier" || pk == "WFControlFlowMode" || pk == "AppIntentDescriptor" {
				continue
			}
			if _, isStatic := cand.StaticParameters[pk]; isStatic {
				continue
			}
			if slices.Contains(cand.ParameterKeys, pk) {
				matchedParams++
			} else if slices.Contains(cand.Definition.emittedKeys, pk) {
				matchedParams++
			} else {
				extraKeys++
			}
		}

		if !cand.HasCustomMakeParams && extraKeys > 0 {
			continue
		}

		// Exclude one-way custom synthesis helpers (like embedFile / makeVCard on gettext) that have
		// no parameter keys, no static parameters, and no custom decomp when rec.Parameters has keys.
		if cand.HasCustomMakeParams && !cand.HasCustomDecomp && len(cand.ParameterKeys) == 0 && len(cand.StaticParameters) == 0 && extraKeys > 0 {
			continue
		}

		missingReq := 0
		for _, rk := range cand.RequiredParameterKeys {
			if _, exists := rec.Parameters[rk]; !exists {
				missingReq++
			}
		}

		compatible = append(compatible, scoredCandidate{
			cand:              cand,
			matchedStaticKeys: matchedStatic,
			matchedParamKeys:  matchedParams,
			extraKeys:         extraKeys,
			missingRequired:   missingReq,
		})
	}

	if len(compatible) == 0 {
		if len(allCands) > 1 {
			return ReverseResolutionOutcome{
				ReasonCode:    "AMBIGUOUS_VARIANT",
				AllCandidates: allCands,
				Detail:        "multiple canonical definitions share AppleIdentifier and wire parameters lack a matching discriminator",
			}
		}
		return ReverseResolutionOutcome{
			ReasonCode:    "UNREPRESENTABLE_PARAM",
			AllCandidates: allCands,
			Detail:        "wire parameters do not match canonical definition static/parameter keys",
		}
	}

	sort.SliceStable(compatible, func(i, j int) bool {
		a, b := compatible[i], compatible[j]
		if a.extraKeys != b.extraKeys {
			return a.extraKeys < b.extraKeys
		}
		if a.missingRequired != b.missingRequired {
			return a.missingRequired < b.missingRequired
		}
		if a.matchedStaticKeys != b.matchedStaticKeys {
			return a.matchedStaticKeys > b.matchedStaticKeys
		}
		if a.matchedParamKeys != b.matchedParamKeys {
			return a.matchedParamKeys > b.matchedParamKeys
		}
		if a.cand.HasCustomMakeParams != b.cand.HasCustomMakeParams {
			return !a.cand.HasCustomMakeParams
		}
		if a.cand.Definition.defaultAction != b.cand.Definition.defaultAction {
			return a.cand.Definition.defaultAction
		}
		if len(a.cand.Definition.parameters) != len(b.cand.Definition.parameters) {
			return len(a.cand.Definition.parameters) < len(b.cand.Definition.parameters)
		}
		return a.cand.CallableName < b.cand.CallableName
	})

	compatList := make([]ReverseActionCandidate, len(compatible))
	for i, sc := range compatible {
		compatList[i] = sc.cand
	}

	// Check if top two candidates are tied on all structural discriminators without defaultAction tie-breaker
	if len(compatible) > 1 {
		top := compatible[0]
		second := compatible[1]
		if top.extraKeys == second.extraKeys &&
			top.missingRequired == second.missingRequired &&
			top.matchedStaticKeys == second.matchedStaticKeys &&
			top.matchedParamKeys == second.matchedParamKeys &&
			top.cand.HasCustomMakeParams == second.cand.HasCustomMakeParams &&
			top.cand.Definition.defaultAction == second.cand.Definition.defaultAction &&
			len(top.cand.Definition.parameters) == len(second.cand.Definition.parameters) {
			return ReverseResolutionOutcome{
				ReasonCode:           "AMBIGUOUS_VARIANT",
				CompatibleCandidates: compatList,
				AllCandidates:        allCands,
				Detail:               fmt.Sprintf("ambiguous tie between %s and %s", top.cand.CallableName, second.cand.CallableName),
			}
		}
	}

	chosen := compatible[0].cand
	return ReverseResolutionOutcome{
		ReasonCode:           "KNOWN_DISAMBIGUATED",
		ChosenCandidate:      &chosen,
		CompatibleCandidates: compatList,
		AllCandidates:        allCands,
		Detail:               fmt.Sprintf("resolved to %s", chosen.CallableName),
	}
}
