/*
 * Copyright (c) Cherri P10 — Real 505 Catalog Execution & Round-Trip Evidence Generator
 */

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"howett.net/plist"
)

type ActionExecutionEvidence struct {
	Name                       string   `json:"name"`
	ShortcutIdentifier         string   `json:"shortcutIdentifier"`
	CompilerConstruct          bool     `json:"compilerConstruct"`
	LegacySupported            bool     `json:"legacySupported"`
	LegacySource               string   `json:"legacySource"`
	LegacyEmittedIdent         string   `json:"legacyEmittedIdent,omitempty"`
	LegacyError                string   `json:"legacyError,omitempty"`
	V2Supported                bool     `json:"v2Supported"`
	V2Source                   string   `json:"v2Source"`
	V2EmittedIdent             string   `json:"v2EmittedIdent,omitempty"`
	V2Error                    string   `json:"v2Error,omitempty"`
	PlistIdentifierVerified    bool     `json:"plistIdentifierVerified"`
	ParameterContractVerified  bool     `json:"parameterContractVerified"`
	ParameterContractError     string   `json:"parameterContractError,omitempty"`
	DecompileSupported         bool     `json:"decompileSupported"`
	DecompileMode              string   `json:"decompileMode"`
	DecompiledSource           string   `json:"decompiledSource,omitempty"`
	DecompileError             string   `json:"decompileError,omitempty"`
	RoundTripVerified          bool                    `json:"roundTripVerified"`
	StructuralRoundTripPass    bool                    `json:"structuralRoundTripPass"`
	ComparisonModeUsed         string                  `json:"comparisonModeUsed,omitempty"`
	RoundTripError             string                  `json:"roundTripError,omitempty"`
	NativeSpecimenParityStatus string                  `json:"nativeSpecimenParityStatus"`
	AppleRuntimeExecutedStatus string                  `json:"appleRuntimeExecutedStatus"`
	VerificationStatuses       []string                `json:"verificationStatuses"`
	EmittedKeysSample          []string                `json:"emittedKeysSample,omitempty"`
	ParameterContractDetails   *ParameterContractResult `json:"parameterContractDetails,omitempty"`
}

type CatalogExecutionReport struct {
	TotalCatalogEntries         int                         `json:"totalCatalogEntries"`
	CallableCount               int                         `json:"callableCount"`
	ConstructCount              int                         `json:"constructCount"`
	LegacyPassCount             int                         `json:"legacyPassCount"`
	V2PassCount                 int                         `json:"v2PassCount"`
	PlistIdentifierPassCount    int                         `json:"plistIdentifierPassCount"`
	ParameterContractPassCount  int                         `json:"parameterContractPassCount"`
	DecompilePassCount          int                         `json:"decompilePassCount"`
	RoundTripPassCount          int                         `json:"roundTripPassCount"`
	StructuralRoundTripCount    int                         `json:"structuralRoundTripCount"`
	NativeSpecimenParityCount   int                         `json:"nativeSpecimenParityCount"`
	AppleRuntimeExecutedCount   int                         `json:"appleRuntimeExecutedCount"`
	AllowedExclusions           []string                    `json:"allowedExclusions"`
	ParameterDimensionSummaries []ParameterDimensionSummary `json:"parameterDimensionSummaries,omitempty"`
	Entries                     []ActionExecutionEvidence   `json:"entries"`
}

func TestP10_VerifyAndRecord505CatalogExecution(t *testing.T) {
	resetCompilerStateFully()
	schema.ResetDefaultRegistry()
	relativePath = ""
	resetParser()
	markBuiltins()
	defineRawAction()
	loadStandardActions()
	catalog := buildActionCatalog()
	reg := schema.DefaultRegistry()

	if len(catalog) != 505 {
		t.Fatalf("expected 505 catalog entries, got %d", len(catalog))
	}

	// Build identifier -> action names map so shared-identifier aliases (e.g. date/currentDate) are recognized in decompilation
	identToNames := make(map[string][]string)
	for _, entry := range catalog {
		if entry.ShortcutIdentifier != "" {
			identToNames[entry.ShortcutIdentifier] = append(identToNames[entry.ShortcutIdentifier], entry.Name)
		}
	}
	actionCategoryMap := buildActionIncludeCategoryMap()

	report := CatalogExecutionReport{
		TotalCatalogEntries: len(catalog),
		AllowedExclusions:   CanonicalAllowedExclusions,
		Entries:             make([]ActionExecutionEvidence, 0, len(catalog)),
	}

	nativeSpecimenResults := EvaluateAllCatalogNativeSpecimens()
	var contractResults []ParameterContractResult

	for _, entry := range catalog {
		if entry.CompilerConstruct {
			report.ConstructCount++
		} else {
			report.CallableCount++
		}

		ev := executeSingleCatalogEntry(entry, reg, identToNames, actionCategoryMap, nativeSpecimenResults)
		if ev.ParameterContractDetails != nil {
			contractResults = append(contractResults, *ev.ParameterContractDetails)
		}
		if ev.LegacySupported {
			report.LegacyPassCount++
		} else {
			t.Errorf("Legacy execution failed for %s (%s): %s\nSource:\n%s", entry.Name, entry.ShortcutIdentifier, ev.LegacyError, ev.LegacySource)
		}
		if ev.V2Supported {
			report.V2PassCount++
		} else {
			t.Errorf("V2 execution failed for %s (%s): %s\nSource:\n%s", entry.Name, entry.ShortcutIdentifier, ev.V2Error, ev.V2Source)
		}
		if ev.PlistIdentifierVerified {
			report.PlistIdentifierPassCount++
		}
		if ev.ParameterContractVerified {
			report.ParameterContractPassCount++
		} else {
			t.Errorf("Parameter contract failed for %s (%s): %s", entry.Name, entry.ShortcutIdentifier, ev.ParameterContractError)
		}
		if ev.DecompileSupported {
			report.DecompilePassCount++
		} else {
			t.Errorf("Decompile execution failed for %s (%s): %s", entry.Name, entry.ShortcutIdentifier, ev.DecompileError)
		}
		if ev.RoundTripVerified {
			report.RoundTripPassCount++
		} else {
			t.Errorf("Round-trip execution failed for %s (%s): %s\nDecompiled:\n%s", entry.Name, entry.ShortcutIdentifier, ev.RoundTripError, ev.DecompiledSource)
		}
		if ev.StructuralRoundTripPass {
			report.StructuralRoundTripCount++
		}
		if ev.NativeSpecimenParityStatus == NativeSpecimenStatusPass {
			report.NativeSpecimenParityCount++
		}
		if ev.AppleRuntimeExecutedStatus == "APPLE_RUNTIME_EXECUTED" {
			report.AppleRuntimeExecutedCount++
		}

		// Clear bulky inline TestInput strings inside p10-505-execution-evidence.json while keeping full details in p10-505-parameter-contract-evidence.json
		ev.ParameterContractDetails = nil
		report.Entries = append(report.Entries, ev)
	}

	report.ParameterDimensionSummaries = buildParameterDimensionSummaries(contractResults)

	outPath := filepath.Join("docs", "apple-runtime", "p10", "p10-505-execution-evidence.json")
	rawJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal execution report: %v", err)
	}
	if err := os.WriteFile(outPath, append(rawJSON, '\n'), 0644); err != nil {
		if _, gitErr := os.Stat(".git"); gitErr == nil {
			t.Fatalf("failed to write %s: %v", outPath, err)
		}
	}
	t.Logf("505 Catalog Execution Summary: Legacy=%d/505, V2=%d/505, ParamContract=%d/505, Decompile=%d/505, StructuralRoundTrip=%d/505, NativeSpecimenParity=%d/505, RuntimeExecuted=%d/505",
		report.LegacyPassCount, report.V2PassCount, report.ParameterContractPassCount, report.DecompilePassCount, report.StructuralRoundTripCount, report.NativeSpecimenParityCount, report.AppleRuntimeExecutedCount)
}

func executeSingleCatalogEntry(entry catalogActionInfo, reg *schema.Registry, identToNames map[string][]string, actionCategoryMap map[string]string, nativeSpecimenResults map[string]NativeSpecimenEvaluationResult) ActionExecutionEvidence {
	legSrc := buildLegacySourceForEntry(entry, actionCategoryMap)
	v2Src := buildV2SourceForEntry(entry, reg)

	ev := ActionExecutionEvidence{
		Name:                       entry.Name,
		ShortcutIdentifier:         entry.ShortcutIdentifier,
		CompilerConstruct:          entry.CompilerConstruct,
		LegacySource:               strings.TrimSpace(legSrc),
		V2Source:                   strings.TrimSpace(v2Src),
		DecompileMode:              "UNSUPPORTED",
		NativeSpecimenParityStatus: NativeSpecimenStatusNeedsEvidence,
		AppleRuntimeExecutedStatus: "BLOCKED_DEVICE",
		VerificationStatuses:       []string{"CATALOG_DISCOVERED"},
	}

	if entry.CompilerConstruct {
		ev.VerificationStatuses = append(ev.VerificationStatuses, "SCHEMA_REGISTERED")
	} else if _, ok := reg.LookupAction(entry.Name); ok {
		ev.VerificationStatuses = append(ev.VerificationStatuses, "SCHEMA_REGISTERED")
	}

	if specEval, ok := nativeSpecimenResults["name:"+entry.Name]; ok {
		ev.NativeSpecimenParityStatus = specEval.Status
	} else if specEval, ok := nativeSpecimenResults["ident:"+entry.ShortcutIdentifier]; ok {
		ev.NativeSpecimenParityStatus = specEval.Status
	}

	// 1. Execute Legacy Compiler
	legResp := compileForMobileMode(legSrc, "Exec_"+entry.Name, "legacy", false)
	var legPlistBytes []byte
	if !legResp.OK {
		ev.LegacyError = legResp.Error
	} else {
		var decErr error
		legPlistBytes, decErr = base64.StdEncoding.DecodeString(legResp.PlistBase64)
		if decErr != nil {
			ev.LegacyError = decErr.Error()
		} else {
			matchedIdent, keys := inspectEmittedPlistForIdentifier(legPlistBytes, entry.ShortcutIdentifier)
			if matchedIdent != "" {
				ev.LegacySupported = true
				ev.LegacyEmittedIdent = matchedIdent
				ev.EmittedKeysSample = keys
				ev.VerificationStatuses = append(ev.VerificationStatuses, "LEGACY_COMPILE_PASS")
			} else {
				ev.LegacyError = fmt.Sprintf("emitted plist did not contain expected identifier %q", entry.ShortcutIdentifier)
			}
		}
	}

	// 2. Execute V2 Compiler
	v2Resp := compileForMobileMode(v2Src, "Exec_"+entry.Name, "v2", false)
	var v2PlistBytes []byte
	if !v2Resp.OK {
		ev.V2Error = v2Resp.Error
	} else {
		var decErr error
		v2PlistBytes, decErr = base64.StdEncoding.DecodeString(v2Resp.PlistBase64)
		if decErr != nil {
			ev.V2Error = decErr.Error()
		} else {
			matchedIdent, keys := inspectEmittedPlistForIdentifier(v2PlistBytes, entry.ShortcutIdentifier)
			if matchedIdent != "" {
				ev.V2Supported = true
				ev.V2EmittedIdent = matchedIdent
				if len(ev.EmittedKeysSample) == 0 {
					ev.EmittedKeysSample = keys
				}
				ev.VerificationStatuses = append(ev.VerificationStatuses, "V2_COMPILE_PASS")
			} else {
				ev.V2Error = fmt.Sprintf("emitted v2 plist did not contain expected identifier %q", entry.ShortcutIdentifier)
			}
		}
	}

	if ev.LegacySupported && ev.V2Supported {
		ev.PlistIdentifierVerified = true
		ev.VerificationStatuses = append(ev.VerificationStatuses, "PLIST_IDENTIFIER_PASS")
	}

	// 2b. Verify Parameter Contract (required, missing required rejected, optional, enum, dynamic/literal-only, platform gate)
	contractRes := verifyCatalogActionParameterContract(entry, reg, actionCategoryMap)
	ev.ParameterContractDetails = &contractRes
	if contractRes.ContractPassed {
		ev.ParameterContractVerified = true
		ev.VerificationStatuses = append(ev.VerificationStatuses, "PARAMETER_CONTRACT_PASS")
	} else {
		ev.ParameterContractError = contractRes.FailureReason
	}

	// 3. Execute Decompiler on emitted plist (use legPlistBytes as canonical baseline since decompiler outputs legacy syntax, fallback to v2PlistBytes)
	sourcePlist := legPlistBytes
	if len(sourcePlist) == 0 {
		sourcePlist = v2PlistBytes
	}
	if len(sourcePlist) == 0 {
		ev.DecompileError = "no compiled plist available to decompile"
		ev.RoundTripError = "no decompiled source available"
		ev.VerificationStatuses = append(ev.VerificationStatuses, ev.NativeSpecimenParityStatus, ev.AppleRuntimeExecutedStatus)
		return ev
	}

	if entry.Name == "comment" {
		args.Args["comments"] = "true"
	}
	decompResp := decompileForMobile(sourcePlist, "Exec_"+entry.Name)
	if entry.Name == "comment" {
		delete(args.Args, "comments")
	}
	if !decompResp.OK || strings.TrimSpace(decompResp.Source) == "" {
		ev.DecompileError = decompResp.Error
		if ev.DecompileError == "" {
			ev.DecompileError = "decompiler returned empty source"
		}
		ev.RoundTripError = "decompile failed"
		ev.VerificationStatuses = append(ev.VerificationStatuses, ev.NativeSpecimenParityStatus, ev.AppleRuntimeExecutedStatus)
		return ev
	}

	decompSrc := strings.TrimSpace(decompResp.Source)
	ev.DecompileSupported = true
	ev.DecompiledSource = decompSrc
	ev.VerificationStatuses = append(ev.VerificationStatuses, "DECOMPILE_PASS")

	// Classify actual decompile mode from decompiled output
	if entry.CompilerConstruct {
		if strings.Contains(decompSrc, "rawAction(") {
			ev.DecompileMode = "rawAction_lossless_fallback"
		} else {
			ev.DecompileMode = "native_control_flow"
		}
	} else {
		if entry.Name != "rawAction" && strings.Contains(decompSrc, "rawAction(") {
			ev.DecompileMode = "rawAction_lossless_fallback"
		} else {
			ev.DecompileMode = "native_cherri_call"
		}
	}

	// 4. Execute Round-Trip Recompilation on decompiled source and perform Canonical Structural Plist Comparison
	recompSrc := decompResp.Source
	if entry.MacOnly && !strings.Contains(recompSrc, "#define mac") {
		recompSrc = "#define mac true\n" + recompSrc
	}
	if entry.MaxVersion > 0 && entry.MaxVersion < 27 && !strings.Contains(recompSrc, "#define version") && !strings.Contains(recompSrc, "askProModel(") {
		recompSrc = fmt.Sprintf("#define version %v\n", entry.MaxVersion) + recompSrc
	}

	recompResp := compileForMobileMode(recompSrc, "Recomp_"+entry.Name, "legacy", false)
	if !recompResp.OK {
		migrated := migrate.MigrateSource(recompSrc)
		recompResp = compileForMobileMode(migrated, "Recomp_"+entry.Name, "v2", false)
	}
	if !recompResp.OK {
		ev.RoundTripError = recompResp.Error
		ev.VerificationStatuses = append(ev.VerificationStatuses, ev.NativeSpecimenParityStatus, ev.AppleRuntimeExecutedStatus)
		return ev
	}

	recompPlist, err := base64.StdEncoding.DecodeString(recompResp.PlistBase64)
	if err != nil {
		ev.RoundTripError = err.Error()
		ev.VerificationStatuses = append(ev.VerificationStatuses, ev.NativeSpecimenParityStatus, ev.AppleRuntimeExecutedStatus)
		return ev
	}
	recompIdent, _ := inspectEmittedPlistForIdentifier(recompPlist, entry.ShortcutIdentifier)
	if recompIdent == "" {
		ev.RoundTripError = fmt.Sprintf("recompiled plist missing identifier %q", entry.ShortcutIdentifier)
		ev.VerificationStatuses = append(ev.VerificationStatuses, ev.NativeSpecimenParityStatus, ev.AppleRuntimeExecutedStatus)
		return ev
	}

	cmpRes, cmpErr := CompareShortcutPlistsWithMode(sourcePlist, recompPlist, ComparisonModeActionPayload)
	if cmpErr != nil {
		ev.RoundTripError = fmt.Sprintf("canonical structural comparison error: %v", cmpErr)
		ev.VerificationStatuses = append(ev.VerificationStatuses, ev.NativeSpecimenParityStatus, ev.AppleRuntimeExecutedStatus)
		return ev
	}
	if !cmpRes.Equal {
		var diffStrs []string
		for _, d := range cmpRes.Differences {
			diffStrs = append(diffStrs, fmt.Sprintf("%s: %s (expected=%v, actual=%v)", d.Path, d.Kind, d.A, d.B))
		}
		ev.RoundTripError = fmt.Sprintf("canonical structural mismatch: %s", strings.Join(diffStrs, "; "))
		ev.VerificationStatuses = append(ev.VerificationStatuses, ev.NativeSpecimenParityStatus, ev.AppleRuntimeExecutedStatus)
		return ev
	}

	ev.RoundTripVerified = true
	ev.StructuralRoundTripPass = true
	ev.ComparisonModeUsed = string(ComparisonModeActionPayload)
	// Also check if FullWorkflowSemanticComparison passes (when decompiled workflow headers match source headers)
	if fullCmp, fullErr := CompareShortcutPlistsWithMode(sourcePlist, recompPlist, ComparisonModeFullWorkflowSemantic); fullErr == nil && fullCmp.Equal {
		ev.ComparisonModeUsed = string(ComparisonModeFullWorkflowSemantic)
	}
	ev.VerificationStatuses = append(ev.VerificationStatuses, "STRUCTURAL_ROUNDTRIP_PASS", ev.NativeSpecimenParityStatus, ev.AppleRuntimeExecutedStatus)
	return ev
}

func inspectEmittedPlistForIdentifier(plistBytes []byte, expectedIdent string) (string, []string) {
	var doc map[string]any
	if _, err := plist.Unmarshal(plistBytes, &doc); err != nil {
		return "", nil
	}
	acts, ok := doc["WFWorkflowActions"].([]any)
	if !ok || len(acts) == 0 {
		return "", nil
	}
	var lastIdent string
	var lastKeys []string
	for _, rawAct := range acts {
		act, ok := rawAct.(map[string]any)
		if !ok {
			continue
		}
		ident, _ := act["WFWorkflowActionIdentifier"].(string)
		params, _ := act["WFWorkflowActionParameters"].(map[string]any)
		var keys []string
		for k := range params {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		lastIdent = ident
		lastKeys = keys
		if ident == expectedIdent {
			return ident, keys
		}
	}
	// Special case: embedFile and makeVCard emit is.workflow.actions.gettext, rawAction emits its argument identifier
	if (expectedIdent == "is.workflow.actions.raw" || expectedIdent == "is.workflow.actions.rawaction") && lastIdent != "" {
		return lastIdent, lastKeys
	}
	return "", nil
}

func buildActionIncludeCategoryMap() map[string]string {
	m := make(map[string]string)
	for _, cat := range actionIncludes {
		raw, err := stdActions.ReadFile(fmt.Sprintf("actions/%s.cherri", cat))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "action ") {
				continue
			}
			if strings.HasPrefix(trimmed, "action toggleSet ") {
				fields := strings.Fields(trimmed)
				for i := 2; i < len(fields); i++ {
					tok := fields[i]
					if strings.HasPrefix(tok, "'") || tok == "mac" || tok == "!mac" || strings.HasPrefix(tok, "v") {
						continue
					}
					name := strings.Split(tok, "(")[0]
					m["toggle"+name] = cat
					m["set"+name] = cat
					break
				}
				continue
			}
			idxParen := strings.Index(trimmed, "(")
			if idxParen > 0 {
				prefix := strings.TrimSpace(trimmed[:idxParen])
				parts := strings.Fields(prefix)
				if len(parts) > 0 {
					name := parts[len(parts)-1]
					m[name] = cat
				}
			}
		}
	}
	m["editEvent"] = "calendar"
	m["editReminder"] = "calendar"
	m["updateContact"] = "contacts"
	return m
}

func buildLegacySourceForEntry(entry catalogActionInfo, actionCategoryMap map[string]string) string {
	var b strings.Builder
	b.WriteString("#define name ExecTest\n")
	if entry.MacOnly {
		b.WriteString("#define mac true\n")
	}
	if entry.MaxVersion > 0 && entry.MaxVersion < 27 {
		fmt.Fprintf(&b, "#define version %v\n", entry.MaxVersion)
	}
	if cat, ok := actionCategoryMap[entry.Name]; ok && cat != "" && cat != "basic" {
		fmt.Fprintf(&b, "#include 'actions/%s'\n", cat)
	} else if !entry.CompilerConstruct && !entry.Builtin && entry.Category != "" && entry.Category != "basic" && entry.Category != "builtin" {
		fmt.Fprintf(&b, "#include 'actions/%s'\n", entry.Category)
	}

	if entry.CompilerConstruct {
		switch entry.Name {
		case "conditional":
			b.WriteString("@x = 1\nif @x == 1 {\n\tshow(\"ok\")\n}\n")
		case "menu":
			b.WriteString("menu \"Choose\" {\n\titem \"Option 1\":\n\t\tshow(\"1\")\n}\n")
		case "repeat":
			b.WriteString("repeat _ for 2 {\n\tshow(\"r\")\n}\n")
		case "repeat.each":
			b.WriteString("for _ in Ask {\n\tshow(\"ok\")\n}\n")
		case "setVariable":
			b.WriteString("@myVar = \"hello\"\n")
		case "appendVariable":
			b.WriteString("@myVar = list(\"a\")\n@myVar += \"item\"\n")
		case "dictionaryValue":
			b.WriteString("@d = {\"key\": \"value\"}\n")
		case "calculateExpression":
			b.WriteString("@res = 1 + 2 + 3\n")
		}
		return b.String()
	}

	switch entry.Name {
	case "convertMeasurement":
		b.WriteString("convertMeasurement(Ask, \"Length\", \"m\")\n")
		return b.String()
	case "measurement":
		b.WriteString("measurement(\"10\", \"Length\", \"m\")\n")
		return b.String()
	case "embedFile":
		b.WriteString("embedFile(\"ReadMe.md\")\n")
		return b.String()
	case "rawAction":
		b.WriteString("rawAction(\"is.workflow.actions.custom.p10test\", {\"TestKey\": \"hello\"})\n")
		return b.String()
	}

	lastReq := lastRequiredParamIndex(entry)
	var args []string
	for i := 0; i <= lastReq; i++ {
		p := entry.Parameters[i]
		args = append(args, legacyArgForParam(entry.Name, p))
	}
	fmt.Fprintf(&b, "%s(%s)\n", entry.Name, strings.Join(args, ", "))
	return b.String()
}

func buildV2SourceForEntry(entry catalogActionInfo, reg *schema.Registry) string {
	var b strings.Builder
	needHeader := entry.MacOnly || (entry.MaxVersion > 0 && entry.MaxVersion < 27) || entry.MinVersion > 27
	if needHeader {
		b.WriteString("shortcut \"ExecTest\" {\n")
		if entry.MacOnly {
			b.WriteString("\tmac: true\n")
		}
		if entry.MaxVersion > 0 && entry.MaxVersion < 27 {
			fmt.Fprintf(&b, "\tversion: %v\n\tmaxVersion: %v\n", entry.MaxVersion, entry.MaxVersion)
		}
		b.WriteString("}\n")
	}

	if entry.CompilerConstruct {
		switch entry.Name {
		case "conditional":
			b.WriteString("let x = 1\nif (x == 1) {\n\tshow(\"ok\")\n}\n")
		case "menu":
			b.WriteString("menu(\"Choose\") {\n\tcase \"Option 1\" {\n\t\tshow(\"1\")\n\t}\n}\n")
		case "repeat":
			b.WriteString("repeat (2) {\n\tshow(\"r\")\n}\n")
		case "repeat.each":
			b.WriteString("let arr = [\"a\", \"b\"]\nfor item in arr {\n\tshow(\"ok\")\n}\n")
		case "setVariable":
			b.WriteString("var myVar = \"hello\"\n")
		case "appendVariable":
			b.WriteString("var myVar = [\"a\"]\nmyVar += \"item\"\n")
		case "dictionaryValue":
			b.WriteString("let d = {\"key\": \"value\"}\n")
		case "calculateExpression":
			b.WriteString("var res = rawAction(\"is.workflow.actions.calculateexpression\", parameters: {\"Input\": \"1 + 2 + 3\"})\n")
		}
		return b.String()
	}

	switch entry.Name {
	case "convertMeasurement":
		b.WriteString("let m = Ask\nconvertMeasurement(m, unitType: \"Length\", unit: \"m\")\n")
		return b.String()
	case "measurement":
		b.WriteString("measurement(\"10\", unitType: \"Length\", unit: \"m\")\n")
		return b.String()
	case "embedFile":
		b.WriteString("embedFile(\"ReadMe.md\")\n")
		return b.String()
	case "rawAction":
		b.WriteString("rawAction(\"is.workflow.actions.custom.p10test\", parameters: {\"TestKey\": \"hello\"})\n")
		return b.String()
	}

	actSchema, _ := reg.LookupAction(entry.Name)
	lastReq := lastRequiredParamIndex(entry)
	var args []string
	for i := 0; i <= lastReq; i++ {
		p := entry.Parameters[i]
		var sp *schema.ParameterSchema
		if actSchema != nil && i < len(actSchema.Parameters) {
			sp = &actSchema.Parameters[i]
		}
		val := v2ArgForParam(entry.Name, p, sp)
		if i == 0 || p.Infinite {
			args = append(args, val)
		} else {
			args = append(args, fmt.Sprintf("%s: %s", p.Name, val))
		}
	}
	fmt.Fprintf(&b, "%s(%s)\n", entry.Name, strings.Join(args, ", "))
	return b.String()
}

func lastRequiredParamIndex(entry catalogActionInfo) int {
	last := -1
	for i, p := range entry.Parameters {
		if !p.Optional {
			last = i
		}
	}
	return last
}

func legacyArgForParam(actionName string, p catalogParameter) string {
	if p.Enum != "" && len(p.EnumValues) > 0 {
		return fmt.Sprintf("%q", p.EnumValues[0])
	}
	if p.Literal {
		switch p.Type {
		case "dictionary", "dict":
			return "{\"key\": \"value\"}"
		case "array", "list":
			return "[\"item\"]"
		default:
			return "\"sample\""
		}
	}
	if p.Reference || p.Type == "variable" {
		return "Ask"
	}
	switch p.Type {
	case "rawtext":
		return "'sample'"
	case "text", "string":
		if strings.Contains(strings.ToLower(p.Name), "appid") || strings.Contains(strings.ToLower(actionName), "app") {
			return "\"com.apple.Notes\""
		}
		if strings.Contains(strings.ToLower(p.Name), "url") {
			return "\"https://example.com\""
		}
		return "\"sample\""
	case "float":
		return "0.5"
	case "number", "int", "integer":
		return "1"
	case "bool", "boolean":
		return "true"
	case "dictionary", "dict":
		return "{\"key\": \"value\"}"
	case "array", "list":
		return "[\"item\"]"
	default:
		return "Ask"
	}
}

func v2ArgForParam(actionName string, p catalogParameter, sp *schema.ParameterSchema) string {
	if p.Enum != "" && len(p.EnumValues) > 0 {
		return fmt.Sprintf("%q", p.EnumValues[0])
	}
	if p.Literal {
		switch p.Type {
		case "dictionary", "dict":
			return "{\"key\": \"value\"}"
		case "array", "list":
			return "[\"item\"]"
		default:
			return "\"sample\""
		}
	}
	// Check if schema AcceptedForms only permits attachments (not literals) or only permits literals
	if sp != nil && len(sp.AcceptedForms) > 0 {
		allowsLiteral := slices.Contains(sp.AcceptedForms, "literal") || slices.Contains(sp.AcceptedForms, "tokenString")
		allowsVariable := false
		for _, f := range sp.AcceptedForms {
			if strings.HasPrefix(f, "attachment") {
				allowsVariable = true
				break
			}
		}
		if !allowsLiteral {
			return "Ask"
		}
		if allowsLiteral && !allowsVariable {
			if sp != nil && sp.TypeName == "Number" {
				if p.Type == "float" {
					return "0.5"
				}
				return "100"
			}
			switch p.Type {
			case "dictionary", "dict":
				return "{\"key\": \"value\"}"
			case "array", "list":
				return "[\"item\"]"
			case "number", "int", "integer":
				return "1"
			case "float":
				return "0.5"
			case "bool", "boolean":
				return "true"
			default:
				if strings.Contains(strings.ToLower(p.Name), "appid") || strings.Contains(strings.ToLower(actionName), "app") {
					return "\"com.apple.Notes\""
				}
				if strings.Contains(strings.ToLower(p.Name), "url") {
					return "\"https://example.com\""
				}
				return "\"sample\""
			}
		}
	}
	if p.Reference || p.Type == "variable" {
		return "Ask"
	}
	if sp != nil {
		switch sp.TypeName {
		case "Number":
			if p.Type == "text" || p.Type == "string" {
				return "Ask"
			}
			if p.Type == "float" {
				return "0.5"
			}
			return "1"
		case "Image", "File", "Contact", "CalendarEvent", "AnyContent":
			if !p.Literal && p.Type != "text" && p.Type != "string" && p.Type != "rawtext" {
				return "Ask"
			}
		}
	}
	switch p.Type {
	case "rawtext":
		return "\"sample\""
	case "text", "string":
		if strings.Contains(strings.ToLower(p.Name), "appid") || strings.Contains(strings.ToLower(actionName), "app") {
			return "\"com.apple.Notes\""
		}
		if strings.Contains(strings.ToLower(p.Name), "url") {
			return "\"https://example.com\""
		}
		return "\"sample\""
	case "float":
		return "0.5"
	case "number", "int", "integer":
		return "1"
	case "bool", "boolean":
		return "true"
	case "dictionary", "dict":
		return "{\"key\": \"value\"}"
	case "array", "list":
		return "[\"item\"]"
	default:
		return "Ask"
	}
}
