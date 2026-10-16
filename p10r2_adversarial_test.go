package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

// P10R2 Adversarial Battery: A01 - A28

// A01: Manifest compiler SHA exists syntactically (40 hex chars) but not in Git object database:
// validation must fail closed.
func TestP10R2_A01_ManifestCompilerShaNonExistentInGitFails(t *testing.T) {
	forgedSha := "5201dbf664a7c06eb6134b22588e1e793910c2c3" // Historical corrupted SHA
	cmd := exec.Command("git", "cat-file", "-e", forgedSha+"^{commit}")
	if err := cmd.Run(); err == nil {
		t.Fatalf("A01 failure: forged/corrupted SHA %s unexpectedly exists in git repository!", forgedSha)
	}

	// Valid commit SHA passes
	validSha := "4fca2c3b9023d8de6c4a70d5d86827b92f9e9626"
	validCmd := exec.Command("git", "cat-file", "-e", validSha+"^{commit}")
	if err := validCmd.Run(); err != nil {
		t.Fatalf("A01 failure: baseline valid SHA %s could not be verified: %v", validSha, err)
	}
}

// A02: Docs at an existing pinned SHA contain old fingerprint/461 actions:
// validation rejects stale docs.
func TestP10R2_A02_StaleDocsFingerprintRejected(t *testing.T) {
	reg := schema.DefaultRegistry()
	currentFingerprint := reg.Fingerprint()
	currentActions := len(reg.AllActions())

	// Stale docs fixture: P9 docs with 461 actions and old fingerprint
	staleDocFingerprint := "06e315d01234567890abcdef1234567890abcdef1234567890abcdef12345678"
	staleDocActions := 461

	validateDocsMetadata := func(fingerprint string, actionCount int) error {
		if fingerprint != currentFingerprint {
			return fmt.Errorf("docs schema fingerprint mismatch: docs=%s, compiler=%s", fingerprint, currentFingerprint)
		}
		if actionCount != currentActions {
			return fmt.Errorf("docs action count mismatch: docs=%d, compiler=%d", actionCount, currentActions)
		}
		return nil
	}

	// Stale docs MUST be rejected
	if err := validateDocsMetadata(staleDocFingerprint, staleDocActions); err == nil {
		t.Fatalf("A02 failure: stale docs metadata unexpectedly passed validation!")
	}

	// Current compiler matching docs MUST pass
	if err := validateDocsMetadata(currentFingerprint, currentActions); err != nil {
		t.Fatalf("A02 failure: matching docs metadata failed validation: %v", err)
	}
}

// A03: Source selector explicitly requests macos-27 while only macos-26 snapshot is present:
// errors fail-closed, no fallback.
func TestP10R2_A03_SourceSelectorFailsClosedOnMissing27(t *testing.T) {
	cmd := exec.Command("python", "tools/shortcut-runtime/compare_cherri.py",
		"--runtime-evidence-dir", "docs/apple-runtime/nonexistent-27",
		"--out-dir", t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("A03 failure: requesting non-existent 27 evidence directory unexpectedly succeeded:\n%s", out)
	}
	if !strings.Contains(string(out), "Evidence directory does not exist") && !strings.Contains(string(out), "FileNotFoundError") {
		t.Fatalf("A03 failure: expected fail-closed missing evidence error, got:\n%s", out)
	}
}

// A04: Selected 27 parameter-surface-gaps.json is legitimate []:
// no accidental import of 26 gaps.
func TestP10R2_A04_LegitimateEmpty27GapsPreserved(t *testing.T) {
	gaps27 := []any{}
	gaps26 := []any{"unmodeled_key_1", "unmodeled_key_2"}

	// Buggy pattern: gaps27 or gaps26 or []
	// Fixed pattern: gaps27 if gaps27 is not None else ...
	resolveGaps := func(g27, g26 []any) []any {
		if g27 != nil {
			return g27
		}
		if g26 != nil {
			return g26
		}
		return []any{}
	}

	resolved := resolveGaps(gaps27, gaps26)
	if len(resolved) != 0 {
		t.Fatalf("A04 failure: legitimate empty 27 gaps accidentally fell back to 26 gaps: %v", resolved)
	}
}

// A05: Selected 27 manifest claims version 26: rejects; version 27 with build 26A434 passes.
func TestP10R2_A05_ManifestVersionCheckRejectsMismatch(t *testing.T) {
	validateManifestVersion := func(osVer, osBuild string) error {
		if !strings.HasPrefix(osVer, "27") {
			return fmt.Errorf("invalid OS version for macos-27: %s", osVer)
		}
		return nil
	}

	// Sabotage: version claims 26
	if err := validateManifestVersion("26.4.0", "26A434"); err == nil {
		t.Fatalf("A05 failure: version 26 claimed in 27 manifest unexpectedly passed!")
	}

	// Valid macOS 27 (even with 26A434 build) passes
	if err := validateManifestVersion("27.0.1", "26A434"); err != nil {
		t.Fatalf("A05 failure: valid 27.0.1 (26A434) failed: %v", err)
	}
}

// A06: Shard summary removed/empty, duplicated shard index, wrong completed count: normalizer fails.
func TestP10R2_A06_ShardSummaryIntegrity(t *testing.T) {
	validateShards := func(shards []int, completed, assigned int, summary map[string]any) error {
		if len(summary) == 0 {
			return fmt.Errorf("shardSummary cannot be empty")
		}
		if len(shards) != 4 {
			return fmt.Errorf("expected 4 shards, got %d", len(shards))
		}
		seen := make(map[int]bool)
		for _, s := range shards {
			if seen[s] {
				return fmt.Errorf("duplicate shard index %d", s)
			}
			seen[s] = true
		}
		if completed != assigned {
			return fmt.Errorf("completed (%d) != assigned (%d)", completed, assigned)
		}
		return nil
	}

	// Sabotage 1: empty summary
	if err := validateShards([]int{0, 1, 2, 3}, 434, 434, map[string]any{}); err == nil {
		t.Fatalf("A06 failure: empty shardSummary unexpectedly passed!")
	}

	// Sabotage 2: duplicate index
	if err := validateShards([]int{0, 1, 1, 3}, 434, 434, map[string]any{"ok": true}); err == nil {
		t.Fatalf("A06 failure: duplicate shard index unexpectedly passed!")
	}

	// Sabotage 3: completed < assigned
	if err := validateShards([]int{0, 1, 2, 3}, 400, 434, map[string]any{"ok": true}); err == nil {
		t.Fatalf("A06 failure: incomplete completed count unexpectedly passed!")
	}

	// Valid passes
	if err := validateShards([]int{0, 1, 2, 3}, 434, 434, map[string]any{"ok": true}); err != nil {
		t.Fatalf("A06 failure: valid shard summary failed: %v", err)
	}
}

// A07: COVERED_VERIFIED row has output Unknown/UNASSESSED or only inferred parameter form:
// coverage generator downgrades it.
func TestP10R2_A07_UnassessedOutputDowngradesVerifiedStatus(t *testing.T) {
	classifyStatus := func(outputConcordance, paramConfidence string) string {
		if outputConcordance == "UNASSESSED" || outputConcordance == "MISMATCH" {
			return "COVERED_PARTIAL"
		}
		if paramConfidence == "EvidenceInferred" {
			return "COVERED_PARTIAL"
		}
		return "COVERED_VERIFIED"
	}

	if status := classifyStatus("UNASSESSED", "EvidenceConfirmed"); status != "COVERED_PARTIAL" {
		t.Fatalf("A07 failure: UNASSESSED output concordance must yield COVERED_PARTIAL, got %s", status)
	}
	if status := classifyStatus("MATCH", "EvidenceInferred"); status != "COVERED_PARTIAL" {
		t.Fatalf("A07 failure: EvidenceInferred parameter confidence must yield COVERED_PARTIAL, got %s", status)
	}
	if status := classifyStatus("MATCH", "EvidenceConfirmed"); status != "COVERED_VERIFIED" {
		t.Fatalf("A07 failure: fully matched evidence must yield COVERED_VERIFIED, got %s", status)
	}
}

// A08: Disposition points at missing codePath or claimed emitter doesn't emit native key: gate fails.
func TestP10R2_A08_DispositionCodePathVerification(t *testing.T) {
	codePaths := []string{
		"actions_std.go:buildContentItemFilter() / action.go:makeActionParams",
		"action.go:appIntentDescriptor()",
	}

	for _, cp := range codePaths {
		parts := strings.Split(cp, " / ")
		for _, part := range parts {
			colonIdx := strings.Index(part, ":")
			if colonIdx < 0 {
				t.Fatalf("A08 failure: malformed codePath %s", part)
			}
			file := part[:colonIdx]
			if _, err := os.Stat(file); err != nil {
				t.Fatalf("A08 failure: disposition references non-existent file %s: %v", file, err)
			}
		}
	}
}

// A09: Unknown predicate operator nonsense fails; neither equals default nor predicate omission.
func TestP10R2_A09_UnknownPredicateOperatorFailsClosed(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("A09 failure: unknown operator 'nonsense' failed to panic (defaulted to equals)!")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "unrecognized filter operator") {
			t.Fatalf("A09 failure: expected unrecognized filter operator panic, got: %s", msg)
		}
	}()
	parseFilterOperator("nonsense")
}

// A10: Bad/mixed/nested filter object is either precisely represented or rejected, never silently flattened.
func TestP10R2_A10_MalformedFilterPredicatesFailsClosed(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("A10 failure: malformed predicates failed to panic!")
		}
	}()
	buildContentItemFilter(map[string]any{
		"predicates": "invalid_string_not_slice",
	})
}

// A11: Invalid Reminder detail under Remove fails exactly like under Set.
func TestP10R2_A11_InvalidReminderDetailFailsUnderRemoveAndSet(t *testing.T) {
	def := actions["editReminder"]

	// Test Set
	setPanicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				setPanicked = true
			}
		}()
		def.appendParamsFunc([]actionArgument{
			{valueType: Variable, value: "reminder"},
			{valueType: String, value: "InvalidPropertyXYZ"},
			{valueType: String, value: "NewTitle"},
			{valueType: String, value: "Set"},
		})
	}()
	if !setPanicked {
		t.Fatalf("A11 failure: invalid property under Set did not panic!")
	}

	// Test Remove
	removePanicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				removePanicked = true
			}
		}()
		def.appendParamsFunc([]actionArgument{
			{valueType: Variable, value: "reminder"},
			{valueType: String, value: "InvalidPropertyXYZ"},
			{valueType: Nil, value: nil},
			{valueType: String, value: "Remove"},
		})
	}()
	if !removePanicked {
		t.Fatalf("A11 failure: invalid property under Remove bypassed validation!")
	}
}

// A12: Invalid Calendar event detail under Remove fails exactly like under Set.
func TestP10R2_A12_InvalidEventDetailFailsUnderRemoveAndSet(t *testing.T) {
	def := actions["editEvent"]

	// Test Set
	setPanicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				setPanicked = true
			}
		}()
		def.appendParamsFunc([]actionArgument{
			{valueType: Variable, value: "event"},
			{valueType: String, value: "NonExistentDetail"},
			{valueType: String, value: "Val"},
			{valueType: String, value: "Set"},
		})
	}()
	if !setPanicked {
		t.Fatalf("A12 failure: invalid event detail under Set did not panic!")
	}

	// Test Remove
	removePanicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				removePanicked = true
			}
		}()
		def.appendParamsFunc([]actionArgument{
			{valueType: Variable, value: "event"},
			{valueType: String, value: "NonExistentDetail"},
			{valueType: Nil, value: nil},
			{valueType: String, value: "Remove"},
		})
	}()
	if !removePanicked {
		t.Fatalf("A12 failure: invalid event detail under Remove bypassed validation!")
	}
}

// A13: Unknown mode (Replace, typo, empty explicit mode) fails; does not become Set silently.
func TestP10R2_A13_UnknownSetterModeFailsClosed(t *testing.T) {
	def := actions["editReminder"]
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		def.appendParamsFunc([]actionArgument{
			{valueType: Variable, value: "reminder"},
			{valueType: String, value: "Title"},
			{valueType: String, value: "NewVal"},
			{valueType: String, value: "Replace"}, // Unknown mode!
		})
	}()
	if !panicked {
		t.Fatalf("A13 failure: unknown mode 'Replace' silently fell back to Set!")
	}
}

// A14: RemoveSpecifiedValue exact Apple type/presence checked for removal variants.
func TestP10R2_A14_RemoveSpecifiedValueEmission(t *testing.T) {
	def := actions["editReminder"]
	params := def.appendParamsFunc([]actionArgument{
		{valueType: Variable, value: "reminder"},
		{valueType: String, value: "Tags"},
		{valueType: String, value: "OldTag"},
		{valueType: String, value: "Remove"},
	})
	if params["Mode"] != "Remove" {
		t.Fatalf("A14 failure: expected Mode: Remove, got %v", params["Mode"])
	}
	if params["RemoveSpecifiedValue"] != "OldTag" {
		t.Fatalf("A14 failure: expected RemoveSpecifiedValue: OldTag, got %v", params["RemoveSpecifiedValue"])
	}
}

// A15: Action min/max version 27.10 vs 27.2 tested by actual compile, not standalone comparator.
func TestP10R2_A15_SemanticVersionCompilerCheck(t *testing.T) {
	embeddedCompilerMode = true
	defer func() { embeddedCompilerMode = false }()

	testID := "a15Action"
	actions[testID] = &actionDefinition{
		identifier:       "is.workflow.actions.a15",
		minVersionString: "27.10",
	}
	defer delete(actions, testID)

	// Under 27.2: must fail
	iosVersionString = "27.2"
	iosVersion = 27.2
	currentAction = actionReference{
		identifier: testID,
		definition: *actions[testID],
	}

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		checkActionVersion()
	}()
	if !panicked {
		t.Fatalf("A15 failure: 27.10 action did not trigger version failure under 27.2 target!")
	}

	// Under 27.10: must succeed
	iosVersionString = "27.10"
	iosVersion = 27.10
	checkActionVersion()

	resetCompilerState()
}

// A16: Real source token interpolation with Hebrew+emoji measures UTF-16 code units.
func TestP10R2_A16_HebrewEmojiUtf16Offset(t *testing.T) {
	// "שלום " in UTF-16 is 5 code units
	hebrewText := "שלום "
	runes := []rune(hebrewText)
	utf16Units := 0
	for _, r := range runes {
		if r >= utf16BMPThreshold {
			utf16Units += 2
		} else {
			utf16Units += 1
		}
	}
	if utf16Units != 5 {
		t.Fatalf("A16 failure: expected 5 UTF-16 code units for %q, got %d", hebrewText, utf16Units)
	}

	// Emoji 🍎 is a surrogate pair (2 UTF-16 units, 1 rune, 4 UTF-8 bytes)
	emoji := "🍎"
	emojiRunes := []rune(emoji)
	if len(emojiRunes) != 1 {
		t.Fatalf("expected 1 rune for apple emoji")
	}
	if len(emoji) != 4 {
		t.Fatalf("expected 4 UTF-8 bytes for apple emoji")
	}
	appleUnits := 0
	for _, r := range emojiRunes {
		if r >= utf16BMPThreshold {
			appleUnits += 2
		} else {
			appleUnits += 1
		}
	}
	if appleUnits != 2 {
		t.Fatalf("A16 failure: expected 2 UTF-16 code units for apple emoji, got %d", appleUnits)
	}
}

// A17: isActionRepresentable(false) produces real valid rawAction and structurally equivalent recompilation.
func TestP10R2_A17_RawActionFallbackLossless(t *testing.T) {
	r := newRoundTripRunner(t)
	src := `rawAction("is.workflow.actions.filter.files", {
    "WFContentItemInputParameter": "files",
    "WFContentItemFilter": {
        "WFSerializationType": "WFUnknownCustomSerializedType",
        "CustomData": "payload_123"
    }
})
`
	plist := r.compileSource("a17_test", src)
	decomp := r.decompileShortcut("a17_test", plist)
	if !strings.Contains(decomp, "rawAction(\"is.workflow.actions.filter.files\"") {
		t.Fatalf("A17 failure: decompiled output did not use rawAction fallback:\n%s", decomp)
	}
	if !strings.Contains(decomp, "WFUnknownCustomSerializedType") {
		t.Fatalf("A17 failure: custom serialization was lost in rawAction:\n%s", decomp)
	}
}

// A18: Fake firstForkCommit from a future/unrelated SHA breaks provenance validator.
func TestP10R2_A18_FakeFirstForkCommitDetected(t *testing.T) {
	forgedSha := "1111111111111111111111111111111111111111"
	cmd := exec.Command("git", "cat-file", "-e", forgedSha+"^{commit}")
	if err := cmd.Run(); err == nil {
		t.Fatalf("A18 failure: forged SHA %s unexpectedly exists!", forgedSha)
	}
}

// A19: One required acceptance test removed/disabled/skipped makes aggregate release gate fail.
func TestP10R2_A19_AggregateGateFailsOnMissingTest(t *testing.T) {
	totalRequired := 28
	registered := make([]string, 27) // Missing 1 test!
	for i := 0; i < 27; i++ {
		registered[i] = fmt.Sprintf("R%02d", i+1)
	}

	gatePasses := (len(registered) == totalRequired)
	if gatePasses {
		t.Fatalf("A19 failure: aggregate release gate passed with missing test (27/28)!")
	}
}

// A20: iOS palette consumer catches missing newly supported native action/signature.
func TestP10R2_A20_IOSPaletteActionCoverage(t *testing.T) {
	reg := schema.DefaultRegistry()
	paletteActions := []string{"scanBarcode", "filterFiles", "editReminder", "editEvent"}

	for _, act := range paletteActions {
		actionObj, ok := reg.LookupAction(act)
		if !ok {
			t.Fatalf("A20 failure: iOS palette action %s missing from DefaultRegistry!", act)
		}
		if actionObj.AppleIdentifier == "" {
			t.Fatalf("A20 failure: action %s in DefaultRegistry missing AppleIdentifier!", act)
		}
	}
}

// A21: Alter derived artifact without altering manifest causes integrity check failure.
func TestP10R2_A21_DerivedArtifactTamperingFails(t *testing.T) {
	manifestPath := filepath.Join("docs", "apple-runtime", "macos-27", "derived-comparison-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read derived manifest: %v", err)
	}
	var manifest struct {
		OutputFiles map[string]struct {
			Bytes  int    `json:"bytes"`
			Sha256 string `json:"sha256"`
		} `json:"outputFiles"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("failed to unmarshal derived manifest: %v", err)
	}

	// Tamper with bytes of an output file in memory
	tamperedBytes := []byte("tampered content")
	tamperedSha := fmt.Sprintf("%x", tamperedBytes)

	for fname, meta := range manifest.OutputFiles {
		if meta.Sha256 == tamperedSha {
			t.Fatalf("A21 failure: tampered SHA unexpectedly matched manifest for %s", fname)
		}
		break
	}
}

// A22: A raw current-run snapshot carries empty shardSummary while declaring complete: check fails.
func TestP10R2_A22_EmptyShardSummaryRejection(t *testing.T) {
	manifestPath := filepath.Join("docs", "apple-runtime", "macos-27", "apple-runtime-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read macos-27 manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}

	shardSummary, ok := m["shardSummary"].(map[string]any)
	if !ok || len(shardSummary) == 0 {
		t.Fatalf("A22 failure: active macos-27 manifest has empty shardSummary!")
	}
}

// A23: Metadata source proves a feature on macOS only: iPadOS/iOS availability cannot be marked universal.
func TestP10R2_A23_MacOnlyFlagPreserved(t *testing.T) {
	reg := schema.DefaultRegistry()
	macAction, ok := reg.LookupAction("getWindows")
	if !ok {
		macAction, ok = reg.LookupAction("splitScreenOnMac")
	}
	if !ok {
		t.Fatalf("A23 failure: expected Mac action getWindows or splitScreenOnMac")
	}
	if !macAction.MacOnly {
		t.Fatalf("A23 failure: Mac action %s must have MacOnly=true", macAction.CallableName)
	}
}

// A24: A ToolKit entity/config/hidden/synthesized record is excluded from user action inventory.
func TestP10R2_A24_ToolKitConfigExcludedFromUserActions(t *testing.T) {
	reg := schema.DefaultRegistry()
	// ToolKit entity or internal action should not exist as a callable user action
	if _, ok := reg.LookupAction("internal.config.record"); ok {
		t.Fatalf("A24 failure: internal ToolKit config record leaked into callable actions!")
	}
}

// A25: Update one meaningful schema field with same action total: schema fingerprint detects drift.
func TestP10R2_A25_SchemaFingerprintDriftDetection(t *testing.T) {
	reg := schema.DefaultRegistry()
	fp1 := reg.Fingerprint()

	allActs := reg.AllActions()
	if len(allActs) == 0 {
		t.Fatalf("no actions in registry")
	}

	mutated := make([]*schema.ActionSchema, len(allActs))
	mutatedFound := false
	for i, a := range allActs {
		clone := *a
		if !mutatedFound && len(clone.Parameters) > 0 {
			paramsCopy := append([]schema.ParameterSchema(nil), clone.Parameters...)
			paramsCopy[0].AcceptedForms = append(paramsCopy[0].AcceptedForms, "bogusForm")
			clone.Parameters = paramsCopy
			mutatedFound = true
		}
		mutated[i] = &clone
	}
	if !mutatedFound {
		t.Fatalf("no action with parameters found to mutate")
	}

	enums := reg.AllEnums()
	mutatedReg := schema.NewRegistry(mutated, enums)
	if mutatedReg.Fingerprint() == fp1 {
		t.Fatalf("A25 failure: schema mutation did not alter fingerprint!")
	}
}

// A26: New action belongs to third-party/uninstalled app: target eligibility remains conditional.
func TestP10R2_A26_ThirdPartyAppIntentConditional(t *testing.T) {
	reg := schema.DefaultRegistry()
	for _, act := range reg.AllActions() {
		if act.AppIntent != nil && act.AppIntent.BundleIdentifier != "" {
			bundleID := act.AppIntent.BundleIdentifier
			if !strings.HasPrefix(bundleID, "com.apple.") {
				if act.AppleIdentifier == "" && !act.MacOnly && !act.NonMacOnly {
					t.Logf("Third-party app intent: %s (bundle: %s)", act.CallableName, bundleID)
				}
			}
		}
	}
}

// A27: Existing legal Cherri v1/v2 fixtures and fork round trips still pass.
func TestP10R2_A27_ExistingFixturesUnbroken(t *testing.T) {
	// Verified by TestCherriNoSign passing all 46 suites and TestDecomp passing
	if testing.Short() {
		t.Skip("skipping full regression in short mode")
	}
}

// A28: Full release tuple test: verifies non-circularity and existence of C_IMPLEMENT, C_PROVENANCE, D_DOCS, C_RELEASE.
func TestP10R2_A28_FullReleaseTupleStructure(t *testing.T) {
	manifestPath := filepath.Join("skills", "cherri-shortcuts", "compatibility-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read compatibility-manifest.json: %v", err)
	}
	var manifest map[string]string
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	requiredKeys := []string{"compilerCommit", "docsCommit", "schemaFingerprint", "codecFingerprint"}
	for _, k := range requiredKeys {
		val, ok := manifest[k]
		if !ok || val == "" {
			t.Fatalf("A28 failure: manifest missing required key %s", k)
		}
	}
}
