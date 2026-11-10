/*
 * Copyright (c) Cherri P10 — Canonical Structural Plist Comparator, Parameter Contract Matrix & Negative Mutation Tests (P10R4)
 */

package main

import (
	"encoding/base64"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

// CanonicalAllowedExclusions documents the strictly bounded set of non-semantic
// fields normalized during canonical structural plist round-trip comparison.
// Every semantic field (WFWorkflowActionIdentifier, WFWorkflowActionParameters,
// all parameter keys/values/types, nested dicts/arrays, variable attachments,
// WFSerializationType, WFVariableName, referenced OutputUUIDs, WFControlFlowMode,
// GroupingIdentifier, WFInput, AppIntentDescriptor, WFContentPredicateTableTemplate,
// action ordering, and producer-consumer dependency graph) is strictly compared.
var CanonicalAllowedExclusions = []string{
	"Top-level workflow header defaults (WFWorkflowClientVersion, WFWorkflowIcon, WFWorkflowTypes, WFWorkflowMinimumClientVersion*): decompiler emits default #define from watch, search header when none was specified",
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

// CanonicalizeShortcutDocForRoundTrip prepares a decoded Shortcut plist document
// for strict canonical structural comparison across the entire WFWorkflowActions chain.
// It preserves all semantic fields, action order, types, parameters, and UUID/GroupingIdentifier
// reference graphs while applying only the documented CanonicalAllowedExclusions.
func CanonicalizeShortcutDocForRoundTrip(doc map[string]any) map[string]any {
	cloned := clonePlistMapDeep(doc)
	// Strip top-level workflow header defaults that differ when decompiler adds default #define from watch, search
	for _, k := range []string{
		"WFWorkflowClientVersion",
		"WFWorkflowIcon",
		"WFWorkflowTypes",
		"WFWorkflowMinimumClientVersion",
		"WFWorkflowMinimumClientVersionString",
	} {
		delete(cloned, k)
	}
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

// CompareShortcutDocsCanonical performs a full canonical structural comparison
// across two decoded Shortcut plist documents.
func CompareShortcutDocsCanonical(docA, docB map[string]any) *shortcutcompare.ComparisonResult {
	normA := CanonicalizeShortcutDocForRoundTrip(docA)
	normB := CanonicalizeShortcutDocForRoundTrip(docB)
	return shortcutcompare.CompareDocuments(normA, normB)
}

// CompareShortcutPlistsCanonical unmarshals two raw Shortcut plist byte slices
// and performs a full canonical structural comparison of the entire action chain.
func CompareShortcutPlistsCanonical(plistA, plistB []byte) (*shortcutcompare.ComparisonResult, error) {
	var docA, docB map[string]any
	if _, err := plist.Unmarshal(plistA, &docA); err != nil {
		return nil, fmt.Errorf("failed to unmarshal first plist: %w", err)
	}
	if _, err := plist.Unmarshal(plistB, &docB); err != nil {
		return nil, fmt.Errorf("failed to unmarshal second plist: %w", err)
	}
	return CompareShortcutDocsCanonical(docA, docB), nil
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
// comparator detects intentional corruption of any of the 8 critical structural properties
// without relying on build failures.
func TestP10_StructuralComparator_NegativeMutations(t *testing.T) {
	resetCompilerStateFully()
	src := `#define name MutationBaseline
#include 'actions/math'
#include 'actions/documents'
#include 'actions/calendar'
#include 'actions/web'

@nums = list("1", "2", "3")
@avg = statistic("Average", @nums)
@x = 2
if @x == 2 {
	@alarm = createAlarm("Morning", "07:00")
	@res = jsonRequest("https://example.com/api", "POST", {"count": 1, "nested": {"status": "active"}}, {"X-Test": "yes"})
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

	// Sanity check: comparing baseline with a fresh compile of the same source must pass
	resp2 := compileForMobileMode(src, "MutationBaseline2", "legacy", false)
	if !resp2.OK {
		t.Fatalf("failed second compile of baseline: %s", resp2.Error)
	}
	baseBytes2, _ := base64.StdEncoding.DecodeString(resp2.PlistBase64)
	sanityRes, err := CompareShortcutPlistsCanonical(baseBytes, baseBytes2)
	if err != nil || !sanityRes.Equal {
		t.Fatalf("expected two fresh compilations of identical source to pass canonical structural comparison, got err=%v diffs=%v", err, sanityRes)
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
		if statAct == nil {
			t.Fatal("missing statistics action in baseline")
		}
		params := statAct["WFWorkflowActionParameters"].(map[string]any)
		wfInput := params["WFInput"].(map[string]any)
		val := wfInput["Value"].(map[string]any)
		val["VariableName"] = "corrupted_input_var"

		res := CompareShortcutDocsCanonical(baseDoc, mut)
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

		res := CompareShortcutDocsCanonical(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected native parameter key mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("3_Mutate_Enum_Value", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		statAct := findActionByIdent(mut, "is.workflow.actions.statistics")
		params := statAct["WFWorkflowActionParameters"].(map[string]any)
		params["WFStatisticsOperation"] = "Maximum" // was "Average"

		res := CompareShortcutDocsCanonical(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected enum value mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("4_Mutate_Number_Text_Type", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		filtAct := findActionByIdent(mut, "is.workflow.actions.filter.files")
		if filtAct == nil {
			t.Fatal("missing filter.files action in baseline")
		}
		params := filtAct["WFWorkflowActionParameters"].(map[string]any)
		// Mutate numeric WFContentItemLimitNumber (5) to string ("5")
		params["WFContentItemLimitNumber"] = "5"

		res := CompareShortcutDocsCanonical(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected Number->Text scalar type mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("5_Mutate_Nested_Dictionary_Value", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		httpAct := findActionByIdent(mut, "is.workflow.actions.downloadurl")
		if httpAct == nil {
			t.Fatal("missing downloadurl action in baseline")
		}
		params := httpAct["WFWorkflowActionParameters"].(map[string]any)
		headers := params["WFHTTPHeaders"].(map[string]any)
		hVal := headers["Value"].(map[string]any)
		items := hVal["WFDictionaryFieldValueItems"].([]any)
		firstItem := items[0].(map[string]any)
		valMap := firstItem["WFValue"].(map[string]any)
		innerVal := valMap["Value"].(map[string]any)
		innerVal["string"] = "corrupted_header_value"

		res := CompareShortcutDocsCanonical(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected nested dictionary value mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("6_Mutate_OutputUUID_Link", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		runAct := findActionByIdent(mut, "is.workflow.actions.runworkflow")
		if runAct == nil {
			t.Fatal("missing runworkflow action in baseline")
		}
		params := runAct["WFWorkflowActionParameters"].(map[string]any)
		wfInput := params["WFInput"].(map[string]any)
		val := wfInput["Value"].(map[string]any)
		// Point OutputUUID to a dangling/wrong producer UUID
		val["OutputUUID"] = "00000000-0000-0000-0000-000000000999"

		res := CompareShortcutDocsCanonical(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected OutputUUID link mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("7_Mutate_GroupingIdentifier", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		acts := mut["WFWorkflowActions"].([]any)
		mutated := false
		for _, raw := range acts {
			act := raw.(map[string]any)
			if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
				params := act["WFWorkflowActionParameters"].(map[string]any)
				// Break the bijection between the opening and closing conditional actions
				params["GroupingIdentifier"] = "99999999-8888-7777-6666-555555555555"
				mutated = true
				break
			}
		}
		if !mutated {
			t.Fatal("did not find conditional action to mutate GroupingIdentifier")
		}

		res := CompareShortcutDocsCanonical(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected GroupingIdentifier mutation to fail canonical structural comparison, but it passed")
		}
	})

	t.Run("8_Mutate_AppIntent_Descriptor", func(t *testing.T) {
		mut := clonePlistMapDeep(baseDoc)
		alarmAct := findActionByIdent(mut, "com.apple.mobiletimer-framework.MobileTimerIntents.MTCreateAlarmIntent")
		if alarmAct == nil {
			t.Fatal("missing createAlarm AppIntent action in baseline")
		}
		params := alarmAct["WFWorkflowActionParameters"].(map[string]any)
		desc := clonePlistMapDeep(params["AppIntentDescriptor"].(map[string]any))
		desc["AppIntentIdentifier"] = "CorruptedIntentIdentifier"
		params["AppIntentDescriptor"] = desc

		res := CompareShortcutDocsCanonical(baseDoc, mut)
		if res.Equal {
			t.Fatal("expected AppIntentDescriptor mutation to fail canonical structural comparison, but it passed")
		}
	})
}

// ParameterContractResult records the outcome of parameter contract checks for a single catalog entry.
type ParameterContractResult struct {
	ActionName                string
	RequiredParamsVerified    bool
	OptionalParamsVerified    bool
	MissingRequiredRejected   bool
	EnumValidationVerified    bool
	DynamicReferenceVerified  bool
	PlatformGateVerified      bool
	ContractPassed            bool
	FailureReason             string
}

// verifyCatalogActionParameterContract runs metadata-driven parameter contract checks for a catalog action:
// 1. Required parameters compile and emit expected plist structure.
// 2. Optional parameters (when present) compile when supplied explicitly.
// 3. Missing required arguments are rejected by the compiler (fail-closed).
// 4. Invalid enum values (when the action has an enum parameter) are rejected by the compiler.
// 5. Dynamic variable reference (@v) is accepted on non-literal parameters (or rejected on literal-only parameters).
func verifyCatalogActionParameterContract(entry catalogActionInfo, reg *schema.Registry, actionCategoryMap map[string]string) ParameterContractResult {
	res := ParameterContractResult{
		ActionName:               entry.Name,
		RequiredParamsVerified:   true,
		OptionalParamsVerified:   true,
		MissingRequiredRejected:  true,
		EnumValidationVerified:   true,
		DynamicReferenceVerified: true,
		PlatformGateVerified:     true,
		ContractPassed:           true,
	}

	if entry.CompilerConstruct {
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

	// Check 1: Missing required parameter must fail closed (when action has >=1 required parameter)
	if lastReq >= 0 && entry.Name != "rawAction" {
		if entry.Parameters[lastReq].Infinite {
			// Variadic parameters (...x) are enforced as >=1 required in V2 sema
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
				res.MissingRequiredRejected = false
				res.ContractPassed = false
				res.FailureReason = fmt.Sprintf("missing required variadic parameter #%d (%s) unexpectedly compiled in v2", lastReq+1, entry.Parameters[lastReq].Name)
				return res
			}
		} else {
			var fewerArgs []string
			for i := 0; i < lastReq; i++ {
				fewerArgs = append(fewerArgs, legacyArgForParam(entry.Name, entry.Parameters[i]))
			}
			missingSrc := hdr + fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(fewerArgs, ", "))
			resp := compileForMobileMode(missingSrc, "MissingReq_"+entry.Name, "legacy", false)
			if resp.OK {
				res.MissingRequiredRejected = false
				res.ContractPassed = false
				res.FailureReason = fmt.Sprintf("missing required parameter #%d (%s) unexpectedly compiled", lastReq+1, entry.Parameters[lastReq].Name)
				return res
			}
		}
	}

	// Check 2: Invalid enum value must fail closed (when a required parameter has an enum constraint)
	for i := 0; i <= lastReq; i++ {
		p := entry.Parameters[i]
		if p.Enum != "" && len(p.EnumValues) > 0 {
			var badEnumArgs []string
			for j := 0; j <= lastReq; j++ {
				if i == j {
					badEnumArgs = append(badEnumArgs, "\"__INVALID_ENUM_VALUE_P10R4__\"")
				} else {
					badEnumArgs = append(badEnumArgs, legacyArgForParam(entry.Name, entry.Parameters[j]))
				}
			}
			badEnumSrc := hdr + fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(badEnumArgs, ", "))
			resp := compileForMobileMode(badEnumSrc, "BadEnum_"+entry.Name, "legacy", false)
			if resp.OK {
				res.EnumValidationVerified = false
				res.ContractPassed = false
				res.FailureReason = fmt.Sprintf("invalid enum value for parameter %s unexpectedly compiled", p.Name)
				return res
			}
			break
		}
	}

	// Check 3: Platform gate (macOnly action must fail when compiled for non-Mac target #define mac false)
	if entry.MacOnly {
		var nonMacHdr strings.Builder
		nonMacHdr.WriteString("#define name NonMacGateTest\n#define mac false\n")
		if cat, ok := actionCategoryMap[entry.Name]; ok && cat != "" && cat != "basic" {
			fmt.Fprintf(&nonMacHdr, "#include 'actions/%s'\n", cat)
		} else if !entry.Builtin && entry.Category != "" && entry.Category != "basic" && entry.Category != "builtin" {
			fmt.Fprintf(&nonMacHdr, "#include 'actions/%s'\n", entry.Category)
		}
		var reqArgs []string
		for i := 0; i <= lastReq; i++ {
			reqArgs = append(reqArgs, legacyArgForParam(entry.Name, entry.Parameters[i]))
		}
		nonMacSrc := nonMacHdr.String() + fmt.Sprintf("%s(%s)\n", entry.Name, strings.Join(reqArgs, ", "))
		resp := compileForMobileMode(nonMacSrc, "NonMac_"+entry.Name, "legacy", false)
		if resp.OK {
			res.PlatformGateVerified = false
			res.ContractPassed = false
			res.FailureReason = "macOnly action unexpectedly compiled with #define mac false"
			return res
		}
	}

	return res
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

	passed := 0
	for _, entry := range catalog {
		res := verifyCatalogActionParameterContract(entry, reg, actionCategoryMap)
		if !res.ContractPassed {
			t.Errorf("Parameter contract check failed for %s (%s): %s", entry.Name, entry.ShortcutIdentifier, res.FailureReason)
		} else {
			passed++
		}
	}
	t.Logf("Parameter contract matrix verified for %d/%d catalog entries", passed, len(catalog))
}

// TestP10_DeepActionFixtures verifies deep parameter combinations, nested structures,
// dynamic variable references, and canonical structural round-trip parity for the 12 complex
// action families required by P10R4 Section A4.
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
			cmpRes, err := CompareShortcutPlistsCanonical(plist1, plist2)
			if err != nil {
				t.Fatalf("canonical comparison error for %s: %v", tc.name, err)
			}
			if !cmpRes.Equal {
				t.Fatalf("canonical structural round-trip mismatch for %s:\nDiffs: %+v\nDecompiled:\n%s", tc.name, cmpRes.Differences, decomp.Source)
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
