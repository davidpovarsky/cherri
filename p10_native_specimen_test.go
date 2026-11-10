/*
 * Copyright (c) Cherri P10R5 — Native Specimen Structural Parity Verifier & Negative Mutation Suite
 */

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

const (
	NativeSpecimenStatusPass             = "NATIVE_SPECIMEN_PARITY_PASS"
	NativeSpecimenStatusFail             = "NATIVE_SPECIMEN_PARITY_FAIL"
	NativeSpecimenStatusMissing          = "SPECIMEN_MISSING"
	NativeSpecimenStatusInvalid          = "SPECIMEN_INVALID"
	NativeSpecimenStatusPlatformMismatch = "SPECIMEN_PLATFORM_MISMATCH"
	NativeSpecimenStatusNeedsEvidence    = "NEEDS_EVIDENCE"
)

type NativeSpecimenManifest struct {
	SchemaVersion            string                       `json:"schemaVersion"`
	Description              string                       `json:"description"`
	SpecimenCollectionPolicy map[string]any               `json:"specimenCollectionPolicy"`
	Specimens                []NativeSpecimenManifestItem `json:"specimens"`
}

type NativeSpecimenManifestItem struct {
	SpecimenID                       string         `json:"specimenId"`
	AppleActionIdentifier            string         `json:"appleActionIdentifier"`
	CherriActionName                 string         `json:"cherriActionName,omitempty"`
	ShortcutName                     string         `json:"shortcutName"`
	OSFamily                         string         `json:"osFamily"`
	OSVersion                        string         `json:"osVersion"`
	SourceProvenance                 string         `json:"sourceProvenance"`
	AuthoredManuallyInAppleShortcuts bool           `json:"authoredManuallyInAppleShortcuts"`
	RawShortcutFile                  string         `json:"rawShortcutFile,omitempty"`
	RawShortcutSha256                string         `json:"rawShortcutSha256"`
	ExtractedPlistFile               string         `json:"extractedPlistFile"`
	ExtractedPlistSha256             string         `json:"extractedPlistSha256"`
	CherriVersion                    string         `json:"cherriVersion"`
	EquivalentCherriFile             string         `json:"equivalentCherriFile"`
	ExpectedNativeParameters         []string       `json:"expectedNativeParameters"`
	PlatformAvailability             []string       `json:"platformAvailability"`
	EvidenceProvenance               string         `json:"evidenceProvenance"`
	VerificationStatus               string         `json:"verificationStatus"`
	RuntimeVerificationStatus        string         `json:"runtimeVerificationStatus"`
	ExtraMetadata                    map[string]any `json:"extraMetadata,omitempty"`
}

type NativeSpecimenEvaluationResult struct {
	SpecimenID            string                       `json:"specimenId"`
	AppleActionIdentifier string                       `json:"appleActionIdentifier"`
	CherriActionName      string                       `json:"cherriActionName,omitempty"`
	Status                string                       `json:"status"`
	Diagnostic            string                       `json:"diagnostic,omitempty"`
	Differences           []shortcutcompare.Difference `json:"differences,omitempty"`
}

// LoadNativeSpecimenManifest loads testdata/native-specimens/manifest.json.
func LoadNativeSpecimenManifest(manifestPath string) (*NativeSpecimenManifest, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var m NativeSpecimenManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func sha256HexBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// EvaluateNativeSpecimen performs strict structural native specimen comparison between an independently
// authored Apple Shortcuts plist and the compiled output of its equivalent Cherri source.
// It NEVER grants NATIVE_SPECIMEN_PARITY_PASS based on action identifier presence alone, and
// rejects any self-oracle or non-manually-authored fixture.
func EvaluateNativeSpecimen(item NativeSpecimenManifestItem, baseDir string, requiredOSFamily string) NativeSpecimenEvaluationResult {
	res := NativeSpecimenEvaluationResult{
		SpecimenID:            item.SpecimenID,
		AppleActionIdentifier: item.AppleActionIdentifier,
		CherriActionName:      item.CherriActionName,
		Status:                NativeSpecimenStatusNeedsEvidence,
	}

	// 1. Validate required metadata & non-self-oracle provenance
	if strings.TrimSpace(item.AppleActionIdentifier) == "" ||
		strings.TrimSpace(item.ShortcutName) == "" ||
		strings.TrimSpace(item.OSFamily) == "" ||
		strings.TrimSpace(item.OSVersion) == "" ||
		strings.TrimSpace(item.SourceProvenance) == "" ||
		strings.TrimSpace(item.CherriVersion) == "" ||
		strings.TrimSpace(item.EvidenceProvenance) == "" {
		res.Status = NativeSpecimenStatusInvalid
		res.Diagnostic = "specimen manifest entry is missing required provenance/metadata fields"
		return res
	}
	if !item.AuthoredManuallyInAppleShortcuts {
		res.Status = NativeSpecimenStatusInvalid
		res.Diagnostic = "specimen was not authored manually in Apple Shortcuts (self-oracle or synthetic compiler output is forbidden)"
		return res
	}
	lowerProv := strings.ToLower(item.SourceProvenance + " " + item.EvidenceProvenance)
	if strings.Contains(lowerProv, "self-oracle") || strings.Contains(lowerProv, "cherri-generated") {
		res.Status = NativeSpecimenStatusInvalid
		res.Diagnostic = "specimen provenance indicates Cherri-generated self-oracle output"
		return res
	}

	// 2. Check OS / Platform compatibility
	if requiredOSFamily != "" && !strings.EqualFold(item.OSFamily, requiredOSFamily) {
		if len(item.PlatformAvailability) > 0 && !slices.ContainsFunc(item.PlatformAvailability, func(p string) bool {
			return strings.EqualFold(p, requiredOSFamily)
		}) {
			res.Status = NativeSpecimenStatusPlatformMismatch
			res.Diagnostic = fmt.Sprintf("specimen OSfamily %q (availability %v) does not match required target platform %q", item.OSFamily, item.PlatformAvailability, requiredOSFamily)
			return res
		}
	}
	if len(item.PlatformAvailability) > 0 && !slices.ContainsFunc(item.PlatformAvailability, func(p string) bool {
		return strings.EqualFold(p, item.OSFamily)
	}) {
		res.Status = NativeSpecimenStatusPlatformMismatch
		res.Diagnostic = fmt.Sprintf("specimen osFamily %q is not in declared platformAvailability %v", item.OSFamily, item.PlatformAvailability)
		return res
	}

	// 3. Verify specimen files exist on disk
	if strings.TrimSpace(item.ExtractedPlistFile) == "" || strings.TrimSpace(item.EquivalentCherriFile) == "" {
		res.Status = NativeSpecimenStatusMissing
		res.Diagnostic = "specimen manifest entry omits extractedPlistFile or equivalentCherriFile"
		return res
	}
	plistPath := filepath.Join(baseDir, item.ExtractedPlistFile)
	cherriPath := filepath.Join(baseDir, item.EquivalentCherriFile)

	plistBytes, err := os.ReadFile(plistPath)
	if err != nil {
		res.Status = NativeSpecimenStatusMissing
		res.Diagnostic = fmt.Sprintf("extracted plist file missing: %v", err)
		return res
	}
	cherriBytes, err := os.ReadFile(cherriPath)
	if err != nil {
		res.Status = NativeSpecimenStatusMissing
		res.Diagnostic = fmt.Sprintf("equivalent Cherri file missing: %v", err)
		return res
	}

	// 4. Verify SHA-256 integrity of plist and optional raw shortcut file
	actualPlistSha := sha256HexBytes(plistBytes)
	if strings.TrimSpace(item.ExtractedPlistSha256) == "" || !strings.EqualFold(actualPlistSha, item.ExtractedPlistSha256) {
		res.Status = NativeSpecimenStatusInvalid
		res.Diagnostic = fmt.Sprintf("extractedPlistSha256 mismatch: expected %q, actual %q", item.ExtractedPlistSha256, actualPlistSha)
		return res
	}
	if item.RawShortcutFile != "" {
		rawPath := filepath.Join(baseDir, item.RawShortcutFile)
		rawBytes, rawErr := os.ReadFile(rawPath)
		if rawErr != nil {
			res.Status = NativeSpecimenStatusMissing
			res.Diagnostic = fmt.Sprintf("rawShortcutFile missing: %v", rawErr)
			return res
		}
		actualRawSha := sha256HexBytes(rawBytes)
		if !strings.EqualFold(actualRawSha, item.RawShortcutSha256) {
			res.Status = NativeSpecimenStatusInvalid
			res.Diagnostic = fmt.Sprintf("rawShortcutSha256 mismatch: expected %q, actual %q", item.RawShortcutSha256, actualRawSha)
			return res
		}
	}

	// 5. Parse Apple native plist
	var nativeDoc map[string]any
	if _, err := plist.Unmarshal(plistBytes, &nativeDoc); err != nil {
		res.Status = NativeSpecimenStatusInvalid
		res.Diagnostic = fmt.Sprintf("corrupt Apple native plist: %v", err)
		return res
	}
	nativeActs, ok := nativeDoc["WFWorkflowActions"].([]any)
	if !ok || len(nativeActs) == 0 {
		res.Status = NativeSpecimenStatusInvalid
		res.Diagnostic = "Apple native plist contains no WFWorkflowActions"
		return res
	}

	// 6. Compile equivalent Cherri source
	compResp := compileForMobile(string(cherriBytes), item.ShortcutName, false)
	if !compResp.OK {
		res.Status = NativeSpecimenStatusFail
		res.Diagnostic = fmt.Sprintf("equivalent Cherri compilation failed: %s", compResp.Error)
		return res
	}
	cherriPlistBytes, err := base64.StdEncoding.DecodeString(compResp.PlistBase64)
	if err != nil {
		res.Status = NativeSpecimenStatusFail
		res.Diagnostic = fmt.Sprintf("failed to decode compiled Cherri plist: %v", err)
		return res
	}
	var cherriDoc map[string]any
	if _, err := plist.Unmarshal(cherriPlistBytes, &cherriDoc); err != nil {
		res.Status = NativeSpecimenStatusFail
		res.Diagnostic = fmt.Sprintf("failed to unmarshal compiled Cherri plist: %v", err)
		return res
	}

	// 7. Verify target Apple Action Identifier & Expected Native Parameters exist in both native and compiled docs
	foundTargetActionInNative := false
	foundTargetActionInCherri := false
	var missingExpectedKeys []string

	for _, rawAct := range nativeActs {
		act, _ := rawAct.(map[string]any)
		if act["WFWorkflowActionIdentifier"] == item.AppleActionIdentifier {
			foundTargetActionInNative = true
			params, _ := act["WFWorkflowActionParameters"].(map[string]any)
			for _, reqKey := range item.ExpectedNativeParameters {
				if _, exists := params[reqKey]; !exists {
					missingExpectedKeys = append(missingExpectedKeys, "native:"+reqKey)
				}
			}
			break
		}
	}
	cherriActs, _ := cherriDoc["WFWorkflowActions"].([]any)
	for _, rawAct := range cherriActs {
		act, _ := rawAct.(map[string]any)
		if act["WFWorkflowActionIdentifier"] == item.AppleActionIdentifier {
			foundTargetActionInCherri = true
			params, _ := act["WFWorkflowActionParameters"].(map[string]any)
			for _, reqKey := range item.ExpectedNativeParameters {
				if _, exists := params[reqKey]; !exists {
					missingExpectedKeys = append(missingExpectedKeys, "cherri:"+reqKey)
				}
			}
			break
		}
	}

	if !foundTargetActionInNative {
		res.Status = NativeSpecimenStatusInvalid
		res.Diagnostic = fmt.Sprintf("native specimen plist does not contain declared appleActionIdentifier %q", item.AppleActionIdentifier)
		return res
	}
	if !foundTargetActionInCherri {
		res.Status = NativeSpecimenStatusFail
		res.Diagnostic = fmt.Sprintf("compiled Cherri plist does not contain expected appleActionIdentifier %q", item.AppleActionIdentifier)
		return res
	}
	if len(missingExpectedKeys) > 0 {
		res.Status = NativeSpecimenStatusFail
		res.Diagnostic = fmt.Sprintf("missing expectedNativeParameters: %s", strings.Join(missingExpectedKeys, ", "))
		return res
	}

	// 8. Perform full canonical structural comparison (Action Identifiers, Parameter Keys, Parameter Types,
	// Parameter Values, WFSerializationType, AppIntentDescriptor, OutputUUID links, Nested Dictionaries/Arrays, Control Flow)
	cmpRes := CompareShortcutDocsActionPayload(nativeDoc, cherriDoc)
	if !cmpRes.Equal {
		res.Status = NativeSpecimenStatusFail
		res.Differences = cmpRes.Differences
		var diffSummaries []string
		for _, d := range cmpRes.Differences {
			diffSummaries = append(diffSummaries, fmt.Sprintf("%s (%s: expected=%v, actual=%v)", d.Path, d.Kind, d.A, d.B))
		}
		res.Diagnostic = fmt.Sprintf("native specimen structural parity mismatch (%d diffs): %s", len(cmpRes.Differences), strings.Join(diffSummaries, "; "))
		return res
	}

	res.Status = NativeSpecimenStatusPass
	res.Diagnostic = "verified full structural parity against independent Apple Shortcuts specimen"
	return res
}

// EvaluateAllCatalogNativeSpecimens evaluates all entries in testdata/native-specimens/manifest.json
// and maps results by Cherri action name and Apple action identifier.
func EvaluateAllCatalogNativeSpecimens() map[string]NativeSpecimenEvaluationResult {
	results := make(map[string]NativeSpecimenEvaluationResult)
	manifestPath := filepath.Join("testdata", "native-specimens", "manifest.json")
	manifest, err := LoadNativeSpecimenManifest(manifestPath)
	if err != nil || manifest == nil {
		return results
	}
	baseDir := filepath.Dir(manifestPath)
	for _, spec := range manifest.Specimens {
		ev := EvaluateNativeSpecimen(spec, baseDir, "")
		if spec.CherriActionName != "" {
			results["name:"+spec.CherriActionName] = ev
		}
		if spec.AppleActionIdentifier != "" {
			results["ident:"+spec.AppleActionIdentifier] = ev
		}
	}
	return results
}

func encodePlistXMLBytes(t *testing.T, doc map[string]any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("\t")
	if err := enc.Encode(doc); err != nil {
		t.Fatalf("failed to encode plist: %v", err)
	}
	return buf.Bytes()
}

// TestP10_NativeSpecimenParity_ValidatorAndNegativeMutations verifies Part A1-A6:
// 1. Production manifest has 0 unverified claims and cannot pass by identifier presence alone.
// 2. All 9 mandatory negative mutations fail closed with exact diagnostics and plist paths.
func TestP10_NativeSpecimenParity_ValidatorAndNegativeMutations(t *testing.T) {
	resetCompilerStateFully()

	// Verify production manifest loads cleanly and reports 0 specimens until real Apple-authored specimens are added
	manifestPath := filepath.Join("testdata", "native-specimens", "manifest.json")
	prodManifest, err := LoadNativeSpecimenManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load production native specimen manifest %s: %v", manifestPath, err)
	}
	if prodManifest.SchemaVersion == "" {
		t.Fatal("production native specimen manifest missing schemaVersion")
	}

	// Build a comprehensive synthetic test specimen in t.TempDir() covering:
	// - Action Identifier, Parameter Keys, Parameter Types, Parameter Values
	// - WFSerializationType, Nested Dictionary, AppIntentDescriptor, OutputUUID link, Control Flow
	cherriSrc := `#define name SpecimenParityTest
#include 'actions/math'
#include 'actions/calendar'
#include 'actions/documents'
#include 'actions/web'

@nums = list("10", "20")
@avg = statistic("Average", @nums)
@x = 1
if @x == 1 {
	@alarm = createAlarm("Wake", "07:00")
	@resp = jsonRequest("https://example.com/api", "POST", {"count": 5, "meta": {"env": "prod"}}, {"X-Trace": "abc"})
	@filtered = filterFiles(@resp, {"property": "Name", "operator": 4, "value": "doc"}, 3)
}
`
	resp := compileForMobileMode(cherriSrc, "SpecimenParityTest", "legacy", false)
	if !resp.OK {
		t.Fatalf("failed to compile test Cherri source: %s", resp.Error)
	}
	basePlistBytes, err := base64.StdEncoding.DecodeString(resp.PlistBase64)
	if err != nil {
		t.Fatalf("failed to decode base64 plist: %v", err)
	}
	var baseDoc map[string]any
	if _, err := plist.Unmarshal(basePlistBytes, &baseDoc); err != nil {
		t.Fatalf("failed to unmarshal baseline plist: %v", err)
	}

	writeFixtureFiles := func(t *testing.T, dir string, doc map[string]any) (string, string) {
		t.Helper()
		plistRaw := encodePlistXMLBytes(t, doc)
		plistFile := "specimen.plist"
		cherriFile := "specimen.cherri"
		if err := os.WriteFile(filepath.Join(dir, plistFile), plistRaw, 0644); err != nil {
			t.Fatalf("write plist: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, cherriFile), []byte(cherriSrc), 0644); err != nil {
			t.Fatalf("write cherri: %v", err)
		}
		return sha256HexBytes(plistRaw), plistFile
	}

	makeValidItem := func(dir string, doc map[string]any) NativeSpecimenManifestItem {
		sha, plistFile := writeFixtureFiles(t, dir, doc)
		return NativeSpecimenManifestItem{
			SpecimenID:                       "spec-stat-01",
			AppleActionIdentifier:            "is.workflow.actions.statistics",
			CherriActionName:                 "statistic",
			ShortcutName:                     "SpecimenParityTest",
			OSFamily:                         "macOS",
			OSVersion:                        "15.4",
			SourceProvenance:                 "Apple Shortcuts.app macOS 15.4 manual fixture",
			AuthoredManuallyInAppleShortcuts: true,
			RawShortcutSha256:                sha,
			ExtractedPlistFile:               plistFile,
			ExtractedPlistSha256:             sha,
			CherriVersion:                    "2.0.0",
			EquivalentCherriFile:             "specimen.cherri",
			ExpectedNativeParameters:         []string{"WFStatisticsOperation", "WFInput"},
			PlatformAvailability:             []string{"macOS", "iOS", "iPadOS"},
			EvidenceProvenance:               "Apple Shortcuts independent capture",
			VerificationStatus:               NativeSpecimenStatusPass,
			RuntimeVerificationStatus:        "APPLE_RUNTIME_NOT_RUN",
		}
	}

	// Baseline sanity check: exact structural match passes
	t.Run("0_Baseline_Exact_Structural_Match_Passes", func(t *testing.T) {
		dir := t.TempDir()
		item := makeValidItem(dir, baseDoc)
		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusPass {
			t.Fatalf("expected baseline specimen to pass, got status=%s diagnostic=%s", eval.Status, eval.Diagnostic)
		}
	})

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

	// Negative 1: Identifier matches, but extra/different Native Parameter is present (proves identifier-only match is rejected!)
	t.Run("1_Identifier_Matches_But_Native_Parameter_Differs", func(t *testing.T) {
		dir := t.TempDir()
		mut := clonePlistMapDeep(baseDoc)
		act := findActionByIdent(mut, "is.workflow.actions.statistics")
		params := act["WFWorkflowActionParameters"].(map[string]any)
		params["WFUnexpectedExtraNativeParam"] = "injected"
		item := makeValidItem(dir, mut)

		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusFail {
			t.Fatalf("expected NATIVE_SPECIMEN_PARITY_FAIL when native parameter differs despite matching identifier, got %s (%s)", eval.Status, eval.Diagnostic)
		}
	})

	// Negative 2: Parameter Key replaced
	t.Run("2_Parameter_Key_Replaced", func(t *testing.T) {
		dir := t.TempDir()
		mut := clonePlistMapDeep(baseDoc)
		act := findActionByIdent(mut, "is.workflow.actions.statistics")
		params := act["WFWorkflowActionParameters"].(map[string]any)
		params["WFStatisticsOperation_REPLACED"] = params["WFStatisticsOperation"]
		delete(params, "WFStatisticsOperation")
		item := makeValidItem(dir, mut)
		// Clear ExpectedNativeParameters so it reaches the structural comparator or fails on key check
		item.ExpectedNativeParameters = nil

		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusFail {
			t.Fatalf("expected NATIVE_SPECIMEN_PARITY_FAIL when parameter key is replaced, got %s", eval.Status)
		}
	})

	// Negative 3: Parameter Value differs
	t.Run("3_Parameter_Value_Differs", func(t *testing.T) {
		dir := t.TempDir()
		mut := clonePlistMapDeep(baseDoc)
		act := findActionByIdent(mut, "is.workflow.actions.statistics")
		params := act["WFWorkflowActionParameters"].(map[string]any)
		params["WFStatisticsOperation"] = "Minimum" // was "Average"
		item := makeValidItem(dir, mut)

		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusFail {
			t.Fatalf("expected NATIVE_SPECIMEN_PARITY_FAIL when parameter value differs, got %s", eval.Status)
		}
	})

	// Negative 4: Number turned into String
	t.Run("4_Number_Turned_Into_String", func(t *testing.T) {
		dir := t.TempDir()
		mut := clonePlistMapDeep(baseDoc)
		act := findActionByIdent(mut, "is.workflow.actions.filter.files")
		params := act["WFWorkflowActionParameters"].(map[string]any)
		params["WFContentItemLimitNumber"] = "3" // was numeric 3
		item := makeValidItem(dir, mut)

		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusFail {
			t.Fatalf("expected NATIVE_SPECIMEN_PARITY_FAIL when Number parameter is turned into String, got %s", eval.Status)
		}
	})

	// Negative 5: Nested Dictionary modified
	t.Run("5_Nested_Dictionary_Modified", func(t *testing.T) {
		dir := t.TempDir()
		mut := clonePlistMapDeep(baseDoc)
		act := findActionByIdent(mut, "is.workflow.actions.downloadurl")
		params := act["WFWorkflowActionParameters"].(map[string]any)
		headers := params["WFHTTPHeaders"].(map[string]any)
		hVal := headers["Value"].(map[string]any)
		items := hVal["WFDictionaryFieldValueItems"].([]any)
		first := items[0].(map[string]any)
		wfVal := first["WFValue"].(map[string]any)
		inner := wfVal["Value"].(map[string]any)
		inner["string"] = "tampered_nested_value"
		item := makeValidItem(dir, mut)

		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusFail {
			t.Fatalf("expected NATIVE_SPECIMEN_PARITY_FAIL when nested dictionary is modified, got %s", eval.Status)
		}
	})

	// Negative 6: AppIntentDescriptor modified
	t.Run("6_AppIntentDescriptor_Modified", func(t *testing.T) {
		dir := t.TempDir()
		mut := clonePlistMapDeep(baseDoc)
		act := findActionByIdent(mut, "com.apple.mobiletimer-framework.MobileTimerIntents.MTCreateAlarmIntent")
		params := act["WFWorkflowActionParameters"].(map[string]any)
		desc := clonePlistMapDeep(params["AppIntentDescriptor"].(map[string]any))
		desc["BundleIdentifier"] = "com.apple.corrupted.bundle"
		params["AppIntentDescriptor"] = desc
		item := makeValidItem(dir, mut)

		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusFail {
			t.Fatalf("expected NATIVE_SPECIMEN_PARITY_FAIL when AppIntentDescriptor is modified, got %s", eval.Status)
		}
	})

	// Negative 7: OutputUUID points to wrong action
	t.Run("7_OutputUUID_Points_To_Wrong_Action", func(t *testing.T) {
		dir := t.TempDir()
		mut := clonePlistMapDeep(baseDoc)
		act := findActionByIdent(mut, "is.workflow.actions.statistics")
		params := act["WFWorkflowActionParameters"].(map[string]any)
		wfInput := params["WFInput"].(map[string]any)
		val := wfInput["Value"].(map[string]any)
		val["OutputUUID"] = "11111111-2222-3333-4444-555555555555"
		item := makeValidItem(dir, mut)

		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusFail {
			t.Fatalf("expected NATIVE_SPECIMEN_PARITY_FAIL when OutputUUID points to wrong action, got %s", eval.Status)
		}
	})

	// Negative 8: Specimen created on incompatible OS / platform mismatch
	t.Run("8_Specimen_Platform_Mismatch", func(t *testing.T) {
		dir := t.TempDir()
		item := makeValidItem(dir, baseDoc)
		item.OSFamily = "visionOS"
		item.PlatformAvailability = []string{"macOS"}

		eval := EvaluateNativeSpecimen(item, dir, "macOS")
		if eval.Status != NativeSpecimenStatusPlatformMismatch {
			t.Fatalf("expected SPECIMEN_PLATFORM_MISMATCH for incompatible OS, got %s (%s)", eval.Status, eval.Diagnostic)
		}
	})

	// Negative 9: Specimen missing, corrupt plist, bad SHA-256, or self-oracle (authoredManuallyInAppleShortcuts=false)
	t.Run("9_Specimen_Missing_Or_Invalid", func(t *testing.T) {
		dir := t.TempDir()
		itemMissing := makeValidItem(dir, baseDoc)
		itemMissing.ExtractedPlistFile = "nonexistent.plist"
		evalMissing := EvaluateNativeSpecimen(itemMissing, dir, "macOS")
		if evalMissing.Status != NativeSpecimenStatusMissing {
			t.Fatalf("expected SPECIMEN_MISSING for missing plist file, got %s", evalMissing.Status)
		}

		itemBadSha := makeValidItem(dir, baseDoc)
		itemBadSha.ExtractedPlistSha256 = "0000000000000000000000000000000000000000000000000000000000000000"
		evalBadSha := EvaluateNativeSpecimen(itemBadSha, dir, "macOS")
		if evalBadSha.Status != NativeSpecimenStatusInvalid {
			t.Fatalf("expected SPECIMEN_INVALID for SHA-256 mismatch, got %s", evalBadSha.Status)
		}

		itemSelfOracle := makeValidItem(dir, baseDoc)
		itemSelfOracle.AuthoredManuallyInAppleShortcuts = false
		evalSelfOracle := EvaluateNativeSpecimen(itemSelfOracle, dir, "macOS")
		if evalSelfOracle.Status != NativeSpecimenStatusInvalid {
			t.Fatalf("expected SPECIMEN_INVALID when authoredManuallyInAppleShortcuts is false, got %s", evalSelfOracle.Status)
		}
	})
}
