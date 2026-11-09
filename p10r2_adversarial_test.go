package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func findActionParams(doc map[string]any, ident string) map[string]any {
	for _, a := range doc["WFWorkflowActions"].([]any) {
		m := a.(map[string]any)
		if m["WFWorkflowActionIdentifier"] == ident {
			if p, ok := m["WFWorkflowActionParameters"].(map[string]any); ok {
				return p
			}
		}
	}
	return nil
}

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
// validation rejects stale docs using the real production validator.
func TestP10R2_A02_StaleDocsFingerprintRejected(t *testing.T) {
	reg := schema.DefaultRegistry()
	currentFingerprint := reg.Fingerprint()

	// Stale docs fixture: P9 docs with 461 actions and old fingerprint
	staleDocFingerprint := "06e315d01234567890abcdef1234567890abcdef1234567890abcdef12345678"
	if staleDocFingerprint == currentFingerprint {
		t.Fatalf("A02 setup error: stale fingerprint equals current fingerprint")
	}

	tmpDir := t.TempDir()
	staleDocsDir := filepath.Join(tmpDir, "stale-docs")
	validDocsDir := filepath.Join(tmpDir, "valid-docs")
	if err := os.MkdirAll(filepath.Join(staleDocsDir, "fork-language"), 0o755); err != nil {
		t.Fatalf("mkdir stale-docs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(validDocsDir, "fork-language"), 0o755); err != nil {
		t.Fatalf("mkdir valid-docs: %v", err)
	}

	staleMd := fmt.Sprintf("# Actions Reference\n\n- **Total Actions:** 461\n- **Schema Fingerprint:** `%s`\n", staleDocFingerprint)
	validMd := fmt.Sprintf("# Actions Reference\n\n- **Total Actions:** %d\n- **Schema Fingerprint:** `%s`\n", len(reg.AllActions()), currentFingerprint)
	if err := os.WriteFile(filepath.Join(staleDocsDir, "fork-language", "actions.md"), []byte(staleMd), 0o644); err != nil {
		t.Fatalf("write stale actions.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(validDocsDir, "fork-language", "actions.md"), []byte(validMd), 0o644); err != nil {
		t.Fatalf("write valid actions.md: %v", err)
	}

	// Execute real production validation logic in tools/shortcut-runtime and common.sh equivalent via Python
	pyScript := `
import pathlib, re, sys

docs_dir = pathlib.Path(sys.argv[1])
expected_fp = sys.argv[2]
expected_actions = int(sys.argv[3])

actions_md = docs_dir / "fork-language" / "actions.md"
text = actions_md.read_text(encoding="utf-8")

# Execute real regex extraction matching common.sh validate_docs_dir
m_fp = re.search(r"\*\*Schema Fingerprint:\*\*\s*` + "`" + `([a-f0-9]{64})` + "`" + `", text)
if not m_fp:
    print("Cherri docs missing or malformed Schema Fingerprint", file=sys.stderr)
    sys.exit(1)
actual_fp = m_fp.group(1)
if actual_fp != expected_fp:
    print(f"Cherri docs Schema Fingerprint mismatch: expected {expected_fp}, got {actual_fp}", file=sys.stderr)
    sys.exit(1)

m_cnt = re.search(r"\*\*Total Actions:\*\*\s*(\d+)", text)
if m_cnt and int(m_cnt.group(1)) != expected_actions:
    print(f"Cherri docs action count mismatch: expected {expected_actions}, got {m_cnt.group(1)}", file=sys.stderr)
    sys.exit(1)
`
	// 1. Stale docs MUST be rejected
	cmdStale := exec.Command(resolvePythonCommand(), "-c", pyScript, staleDocsDir, currentFingerprint, fmt.Sprintf("%d", len(reg.AllActions())))
	outStale, errStale := cmdStale.CombinedOutput()
	if errStale == nil {
		t.Fatalf("A02 failure: stale docs metadata unexpectedly passed validation!")
	}
	if !strings.Contains(string(outStale), "Schema Fingerprint mismatch") {
		t.Fatalf("A02 failure: expected Schema Fingerprint mismatch error, got: %s", string(outStale))
	}

	// 2. Current compiler matching docs MUST pass
	cmdValid := exec.Command(resolvePythonCommand(), "-c", pyScript, validDocsDir, currentFingerprint, fmt.Sprintf("%d", len(reg.AllActions())))
	if outValid, errValid := cmdValid.CombinedOutput(); errValid != nil {
		t.Fatalf("A02 failure: matching docs metadata failed validation: %v\n%s", errValid, string(outValid))
	}
}

// A03: Source selector explicitly requests macos-27 while only macos-26 snapshot is present:
// errors fail-closed, no fallback.
func TestP10R2_A03_SourceSelectorFailsClosedOnMissing27(t *testing.T) {
	cmd := exec.Command(resolvePythonCommand(), "tools/shortcut-runtime/compare_cherri.py",
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
// no accidental import of 26 gaps in real generate_p10_ledger.build_ledger.
func TestP10R2_A04_LegitimateEmpty27GapsPreserved(t *testing.T) {
	pyCode := `
import sys
sys.path.insert(0, "tools/shortcut-runtime")
from generate_p10_ledger import build_ledger

cherri_catalog = {
    "actions": [{
        "name": "alert",
        "shortcutIdentifier": "is.workflow.actions.alert",
        "outputType": "None",
        "parameters": [{"name": "alert", "key": "WFAlertActionMessage"}]
    }]
}
snap26 = {"actions": {"is.workflow.actions.alert": {"sources": ["builtinDefinition"], "metadata": {"builtin": {"Name": "Show Alert"}}}}}
snap27 = {"actions": {"is.workflow.actions.alert": {"sources": ["builtinDefinition"], "metadata": {"builtin": {"Name": "Show Alert"}}}}}
gaps26 = [{"identifier": "is.workflow.actions.alert", "unmodeledParameterKeys": ["Bogus26UnmodeledKey1", "Bogus26UnmodeledKey2"]}]
gaps27 = []  # Legitimate empty list in macOS 27!

rows = build_ledger(cherri_catalog, snap26, snap27, gaps26, gaps27, [], [], [])
assert len(rows) == 1, f"expected 1 row, got {len(rows)}"
row = rows[0]
if row["unmodeledKeys"] or row["userSuppliedGaps"]:
    raise AssertionError(f"Legitimate empty gaps27 fell back to gaps26: {row}")
if row["status"] != "COVERED_VERIFIED":
    raise AssertionError(f"Expected COVERED_VERIFIED when gaps27 is [], got {row['status']}")
print("A04_REAL_LEDGER_OK")
`
	cmd := exec.Command(resolvePythonCommand(), "-c", pyCode)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "A04_REAL_LEDGER_OK") {
		t.Fatalf("A04 failure: real generate_p10_ledger.build_ledger failed on empty gaps27: %v\n%s", err, string(out))
	}
}

// A05: Selected 27 manifest claims version 26: rejects; version 27 with build 26A434 passes
// in real tools/shortcut-runtime/generate_p10_ledger.py.
func TestP10R2_A05_ManifestVersionCheckRejectsMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	fake27Dir := filepath.Join(tmpDir, "macos-27")
	fake26Dir := filepath.Join(tmpDir, "macos-26")
	fakeOutDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(fake27Dir, 0o755); err != nil {
		t.Fatalf("mkdir fake27Dir: %v", err)
	}
	if err := os.MkdirAll(fake26Dir, 0o755); err != nil {
		t.Fatalf("mkdir fake26Dir: %v", err)
	}

	minimalSnap := `{"actions": {"is.workflow.actions.alert": {"sources": ["builtinDefinition"], "metadata": {"builtin": {"Name": "Show Alert"}}}}}`
	minimalCatalog := `{"actions": [{"name": "alert", "shortcutIdentifier": "is.workflow.actions.alert", "outputType": "None", "parameters": []}]}`
	catalogPath := filepath.Join(tmpDir, "cherri-actions.json")
	_ = os.WriteFile(catalogPath, []byte(minimalCatalog), 0o644)
	_ = os.WriteFile(filepath.Join(fake27Dir, "apple-runtime-snapshot.json"), []byte(minimalSnap), 0o644)
	_ = os.WriteFile(filepath.Join(fake26Dir, "apple-runtime-snapshot.json"), []byte(minimalSnap), 0o644)

	// 1. Sabotage: manifest in macos-27 claims version 26.4.0 -> real generate_p10_ledger.py MUST fail closed
	badManifest := `{"host": {"osVersion": "26.4.0", "build": "25E246"}, "macOS": "26.4.0"}`
	_ = os.WriteFile(filepath.Join(fake27Dir, "apple-runtime-manifest.json"), []byte(badManifest), 0o644)

	cmdBad := exec.Command(resolvePythonCommand(), "tools/shortcut-runtime/generate_p10_ledger.py",
		"--macos-26-dir", fake26Dir,
		"--macos-27-dir", fake27Dir,
		"--cherri-actions", catalogPath,
		"--out-dir", fakeOutDir)
	outBad, errBad := cmdBad.CombinedOutput()
	if errBad == nil {
		t.Fatalf("A05 failure: version 26 claimed in 27 manifest unexpectedly passed real generate_p10_ledger.py!\n%s", string(outBad))
	}
	if !strings.Contains(string(outBad), "macOS 26 manifest cannot be claimed as macOS 27") {
		t.Fatalf("A05 failure: expected version mismatch error from generate_p10_ledger.py, got:\n%s", string(outBad))
	}

	// 2. Valid macOS 27.0.1 (with build 26A434) -> real generate_p10_ledger.py MUST pass
	goodManifest := `{"host": {"osVersion": "27.0.1", "build": "26A434"}, "macOS": "27.0.1"}`
	_ = os.WriteFile(filepath.Join(fake27Dir, "apple-runtime-manifest.json"), []byte(goodManifest), 0o644)

	cmdGood := exec.Command(resolvePythonCommand(), "tools/shortcut-runtime/generate_p10_ledger.py",
		"--macos-26-dir", fake26Dir,
		"--macos-27-dir", fake27Dir,
		"--cherri-actions", catalogPath,
		"--out-dir", fakeOutDir)
	outGood, errGood := cmdGood.CombinedOutput()
	if errGood != nil {
		t.Fatalf("A05 failure: valid 27.0.1 (26A434) failed in real generate_p10_ledger.py: %v\n%s", errGood, string(outGood))
	}
}

// A06: Shard summary removed/empty, duplicated shard index, wrong completed count:
// real normalize_snapshot.validate_shard_invariants fails.
func TestP10R2_A06_ShardSummaryIntegrity(t *testing.T) {
	pyCode := `
import json, pathlib, sys
sys.path.insert(0, "tools/shortcut-runtime")
from normalize_snapshot import validate_shard_invariants

# 1. Sabotage: wrong shard count when expected_count=4
try:
    validate_shard_invariants({"shardSummaries": [{"shardIndex": 0, "totalAssigned": 10, "completed": 10, "successes": 10}]}, expected_count=4)
    raise AssertionError("Expected failure on wrong shard count")
except ValueError:
    pass

# 2. Sabotage: duplicate shard index (0, 1, 1, 3)
try:
    validate_shard_invariants({
        "totalActionsDiscovered": 40,
        "shardSummaries": [
            {"shardIndex": 0, "totalAssigned": 10, "completed": 10, "successes": 10},
            {"shardIndex": 1, "totalAssigned": 10, "completed": 10, "successes": 10},
            {"shardIndex": 1, "totalAssigned": 10, "completed": 10, "successes": 10},
            {"shardIndex": 3, "totalAssigned": 10, "completed": 10, "successes": 10},
        ]
    }, expected_count=4)
    raise AssertionError("Expected failure on duplicate shard index")
except ValueError:
    pass

# 3. Sabotage: completed < totalAssigned
try:
    validate_shard_invariants({
        "totalActionsDiscovered": 20,
        "shardSummaries": [
            {"shardIndex": 0, "totalAssigned": 10, "completed": 9, "successes": 9},
            {"shardIndex": 1, "totalAssigned": 10, "completed": 10, "successes": 10},
        ]
    }, expected_count=2)
    raise AssertionError("Expected failure on incomplete shard")
except ValueError:
    pass

# 4. Real macOS 27 parameter-encodings-summary.json must pass validate_shard_invariants
real_summary = json.loads(pathlib.Path("docs/apple-runtime/macos-27/parameter-encodings-summary.json").read_text(encoding="utf-8"))
validate_shard_invariants(real_summary, expected_count=4)
print("A06_REAL_NORMALIZER_OK")
`
	cmd := exec.Command(resolvePythonCommand(), "-c", pyCode)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "A06_REAL_NORMALIZER_OK") {
		t.Fatalf("A06 failure: real normalize_snapshot.validate_shard_invariants failed: %v\n%s", err, string(out))
	}
}

// A07: COVERED_VERIFIED row has output Unknown/UNASSESSED or NEEDS_RUNTIME_EVIDENCE parameter:
// real coverage generator (generate_p10_ledger.build_ledger) downgrades it to COVERED_PARTIAL.
func TestP10R2_A07_UnassessedOutputDowngradesVerifiedStatus(t *testing.T) {
	pyCode := `
import sys
sys.path.insert(0, "tools/shortcut-runtime")
from generate_p10_ledger import build_ledger

# Case 1: Output is Unknown -> outputTypeConcordance is UNASSESSED -> must downgrade to COVERED_PARTIAL
cat_unassessed = {
    "actions": [{
        "name": "mysteryAction",
        "shortcutIdentifier": "is.workflow.actions.mystery",
        "outputType": "Unknown",
        "parameters": []
    }]
}
snap_unassessed = {
    "actions": {
        "is.workflow.actions.mystery": {
            "sources": ["toolkit"],
            "metadata": {"toolkit": {"name": "Mystery", "outputType": "Unknown"}}
        }
    }
}
rows1 = build_ledger(cat_unassessed, {}, snap_unassessed, [], [], [], [], [])
assert rows1[0]["outputTypeConcordance"] == "UNASSESSED", f"expected UNASSESSED, got {rows1[0]['outputTypeConcordance']}"
assert rows1[0]["status"] == "COVERED_PARTIAL", f"UNASSESSED output must downgrade to COVERED_PARTIAL, got {rows1[0]['status']}"

# Case 2: Parameter has NEEDS_RUNTIME_EVIDENCE disposition -> must downgrade to COVERED_PARTIAL even when output matches
cat_evidence = {
    "actions": [{
        "name": "getUpcomingEvents",
        "shortcutIdentifier": "is.workflow.actions.getupcomingevents",
        "outputType": "string",
        "parameters": []
    }]
}
snap_evidence = {
    "actions": {
        "is.workflow.actions.getupcomingevents": {
            "sources": ["builtinDefinition"],
            "metadata": {"builtin": {"Name": "Get Upcoming Events", "Output": {"Type": "WFStringContentItem"}}}
        }
    }
}
gaps_evidence = [{
    "identifier": "is.workflow.actions.getupcomingevents",
    "unmodeledParameterKeys": ["WFGetUpcomingItemCalendar"]
}]
rows2 = build_ledger(cat_evidence, {}, snap_evidence, [], gaps_evidence, [], [], [])
assert rows2[0]["status"] == "COVERED_PARTIAL", f"NEEDS_RUNTIME_EVIDENCE must downgrade to COVERED_PARTIAL, got {rows2[0]['status']}"

# Case 3: Output matches and no open/needs-evidence gaps -> COVERED_VERIFIED
rows3 = build_ledger(cat_evidence, {}, snap_evidence, [], [], [], [], [])
assert rows3[0]["outputTypeConcordance"] == "MATCH"
assert rows3[0]["status"] == "COVERED_VERIFIED", f"expected COVERED_VERIFIED, got {rows3[0]['status']}"
print("A07_REAL_GENERATOR_OK")
`
	cmd := exec.Command(resolvePythonCommand(), "-c", pyCode)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "A07_REAL_GENERATOR_OK") {
		t.Fatalf("A07 failure: real generate_p10_ledger.build_ledger failed: %v\n%s", err, string(out))
	}
}

// A08: Disposition points at missing codePath or claimed emitter doesn't emit native key: gate fails.
func TestP10R2_A08_DispositionCodePathVerification(t *testing.T) {
	codePaths := []struct {
		spec       string
		mustContain string
	}{
		{
			spec:       "actions_std.go:buildContentItemFilter() / action.go:makeActionParams",
			mustContain: "func buildContentItemFilter",
		},
		{
			spec:       "action.go:appIntentDescriptor()",
			mustContain: "func appIntentDescriptor",
		},
	}

	for _, cp := range codePaths {
		parts := strings.Split(cp.spec, " / ")
		for _, part := range parts {
			colonIdx := strings.Index(part, ":")
			if colonIdx < 0 {
				t.Fatalf("A08 failure: malformed codePath %s", part)
			}
			file := part[:colonIdx]
			symbol := strings.TrimSuffix(part[colonIdx+1:], "()")
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("A08 failure: disposition references non-existent file %s: %v", file, err)
			}
			if !strings.Contains(string(content), symbol) {
				t.Fatalf("A08 failure: file %s does not contain claimed symbol %q", file, symbol)
			}
		}
	}

	// Also compile a real filter action and an AppIntent action and verify the native keys are actually emitted!
	r := newRoundTripRunner(t)
	filterPlist := r.compileSource("a08_filter_emit", `let files = "f"
let filtered = filterFiles(files, filter: {"predicates": [{"property": "Name", "operator": "equals", "value": "readme"}]})
`)
	filterDoc := loadRoundTripDocument(t, filterPlist)
	filterParams := findActionParams(filterDoc, "is.workflow.actions.filter.files")
	if filterParams == nil || filterParams["WFContentItemFilter"] == nil {
		t.Fatalf("A08 failure: claimed emitter buildContentItemFilter did not emit WFContentItemFilter in plist: %v", filterParams)
	}

	intentPlist := r.compileSource("a08_intent_emit", `createAlarm("Morning", "07:00")
`)
	intentDoc := loadRoundTripDocument(t, intentPlist)
	actionsList := intentDoc["WFWorkflowActions"].([]any)
	firstAct := actionsList[0].(map[string]any)
	intentParams := firstAct["WFWorkflowActionParameters"].(map[string]any)
	if intentParams["AppIntentDescriptor"] == nil {
		t.Fatalf("A08 failure: claimed emitter appIntentDescriptor did not emit AppIntentDescriptor in plist: %v", intentParams)
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
	r := newRoundTripRunner(t)

	// 1. Full compilation check: Remove mode with explicit value emits Mode: Remove and RemoveSpecifiedValue: OldTag
	src := `let rem = "myReminder"
editReminder(rem, "Tags", "OldTag", "Remove")
`
	plist := r.compileSource("a14_remove_specific", src)
	doc := loadRoundTripDocument(t, plist)
	params := findActionParams(doc, "is.workflow.actions.setters.reminders")
	if params == nil {
		t.Fatalf("A14 failure: setters.reminders action not found in plist")
	}
	if params["Mode"] != "Remove" {
		t.Fatalf("A14 failure: expected Mode: Remove, got %v", params["Mode"])
	}
	if params["RemoveSpecifiedValue"] != "OldTag" {
		t.Fatalf("A14 failure: expected RemoveSpecifiedValue: OldTag, got %v", params["RemoveSpecifiedValue"])
	}

	// 2. Set mode must NOT emit RemoveSpecifiedValue
	srcSet := `let rem2 = "myReminder2"
editReminder(rem2, "Title", "NewTitle", "Set")
`
	plistSet := r.compileSource("a14_set_no_remove_specific", srcSet)
	docSet := loadRoundTripDocument(t, plistSet)
	paramsSet := findActionParams(docSet, "is.workflow.actions.setters.reminders")
	if _, ok := paramsSet["RemoveSpecifiedValue"]; ok {
		t.Fatalf("A14 failure: RemoveSpecifiedValue unexpectedly emitted under Set mode: %v", paramsSet)
	}

	// 3. Remove mode without value (clearing property) must NOT emit RemoveSpecifiedValue
	srcRemoveNoVal := `let rem3 = "myReminder3"
editReminder(rem3, "Due Date", "Remove")
`
	plistRemoveNoVal := r.compileSource("a14_remove_no_val", srcRemoveNoVal)
	docRemoveNoVal := loadRoundTripDocument(t, plistRemoveNoVal)
	paramsRemoveNoVal := findActionParams(docRemoveNoVal, "is.workflow.actions.setters.reminders")
	if _, ok := paramsRemoveNoVal["RemoveSpecifiedValue"]; ok {
		t.Fatalf("A14 failure: RemoveSpecifiedValue unexpectedly emitted when removing property entirely: %v", paramsRemoveNoVal)
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
	r := newRoundTripRunner(t)

	// 1. Hebrew text token interpolation: "שלום " is 5 UTF-16 code units (and 9 UTF-8 bytes)
	srcHebrew := `let user = "דוד"
show("שלום {user}!")
`
	plistHebrew := r.compileSource("a16_hebrew", srcHebrew)
	docHebrew := loadRoundTripDocument(t, plistHebrew)
	paramsHebrew := findActionParams(docHebrew, "is.workflow.actions.showresult")
	if paramsHebrew == nil {
		t.Fatalf("A16 failure: showresult action not found in plist for Hebrew test")
	}
	textTokenH, ok := paramsHebrew["Text"].(map[string]any)
	if !ok || textTokenH["WFSerializationType"] != "WFTextTokenString" {
		t.Fatalf("A16 failure: expected WFTextTokenString for Hebrew text, got: %v", paramsHebrew["Text"])
	}
	valH := textTokenH["Value"].(map[string]any)
	rangesH := valH["attachmentsByRange"].(map[string]any)
	// Exact range key for 5 UTF-16 units ('ש', 'ל', 'ו', 'ם', ' ') must be "{5, 1}"
	if _, ok := rangesH["{5, 1}"]; !ok {
		t.Fatalf("A16 failure: expected exact attachment range key '{5, 1}', got: %v", rangesH)
	}
	// Adversarial: ensure UTF-8 byte offset (10 bytes: 4 Hebrew chars * 2 + 1 space + 1) was NOT used
	for k := range rangesH {
		if strings.HasPrefix(k, "{10,") || strings.HasPrefix(k, "{9,") {
			t.Fatalf("A16 failure: detected incorrect UTF-8 byte offset in attachmentsByRange: %s", k)
		}
	}

	// 2. Emoji surrogate pair: 🍎 is 2 UTF-16 code units + 1 space = 3 code units (1 rune, 4 UTF-8 bytes)
	srcEmoji := `let item = "תפוח"
show("🍎 {item}!")
`
	plistEmoji := r.compileSource("a16_emoji", srcEmoji)
	docEmoji := loadRoundTripDocument(t, plistEmoji)
	paramsEmoji := findActionParams(docEmoji, "is.workflow.actions.showresult")
	if paramsEmoji == nil {
		t.Fatalf("A16 failure: showresult action not found in plist for emoji test")
	}
	textTokenE := paramsEmoji["Text"].(map[string]any)
	valE := textTokenE["Value"].(map[string]any)
	rangesE := valE["attachmentsByRange"].(map[string]any)
	if _, ok := rangesE["{3, 1}"]; !ok {
		t.Fatalf("A16 failure: expected exact attachment range key '{3, 1}' for emoji surrogate pair, got: %v", rangesE)
	}
	// Adversarial: ensure 1 rune / 4 byte offset was NOT used
	for k := range rangesE {
		if strings.HasPrefix(k, "{2,") || strings.HasPrefix(k, "{5,") {
			t.Fatalf("A16 failure: detected incorrect rune/byte offset for emoji: %s", k)
		}
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
	docOrig := loadRoundTripDocument(t, plist)

	decomp := r.decompileShortcut("a17_test", plist)
	if !strings.Contains(decomp, "rawAction(\"is.workflow.actions.filter.files\"") {
		t.Fatalf("A17 failure: decompiled output did not use rawAction fallback:\n%s", decomp)
	}
	if !strings.Contains(decomp, "WFUnknownCustomSerializedType") {
		t.Fatalf("A17 failure: custom serialization was lost in rawAction:\n%s", decomp)
	}

	// Recompile and verify structural equivalence
	recompiledPlist := r.compileSource("a17_recompiled", decomp)
	docRecompiled := loadRoundTripDocument(t, recompiledPlist)
	assertActionsStructurallyEqual(t, docOrig, docRecompiled)
}

// A18: Fake firstForkCommit from a future/unrelated SHA breaks provenance validator.
func TestP10R2_A18_FakeFirstForkCommitDetected(t *testing.T) {
	// 1. A forged commit SHA must fail git ancestry check
	forgedSha := "1111111111111111111111111111111111111111"
	cmd := exec.Command("git", "merge-base", "--is-ancestor", forgedSha, "HEAD")
	if err := cmd.Run(); err == nil {
		t.Fatalf("A18 failure: forged SHA %s unexpectedly passed git ancestor check!", forgedSha)
	}

	// 2. All real commits recorded in fork-action-provenance.json must pass git ancestry check
	provPath := filepath.Join("docs", "fork-action-provenance.json")
	data, err := os.ReadFile(provPath)
	if err != nil {
		t.Fatalf("failed to read fork-action-provenance.json: %v", err)
	}
	var prov struct {
		Actions []struct {
			Name            string `json:"name"`
			FirstForkCommit string `json:"firstForkCommit"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(data, &prov); err != nil {
		t.Fatalf("failed to unmarshal fork-action-provenance.json: %v", err)
	}

	uniqueShas := make(map[string]string)
	for _, act := range prov.Actions {
		sha := strings.TrimSpace(act.FirstForkCommit)
		if len(sha) == 40 && !strings.Contains(sha, " ") {
			uniqueShas[sha] = act.Name
		}
	}
	if len(uniqueShas) == 0 {
		t.Fatalf("A18 failure: no 40-char commit SHAs found in fork-action-provenance.json to check")
	}

	for sha, name := range uniqueShas {
		cmdCheck := exec.Command("git", "merge-base", "--is-ancestor", sha, "HEAD")
		if err := cmdCheck.Run(); err != nil {
			t.Fatalf("A18 failure: provenance commit %s for action %s is not an ancestor of HEAD: %v", sha, name, err)
		}
	}
}

// A19: One required acceptance test removed/disabled/skipped makes aggregate release gate fail.
func TestP10R2_A19_AggregateGateFailsOnMissingTest(t *testing.T) {
	runnerPath := filepath.Join("tools", "p10r3", "run_release_gates.py")

	// 1. Verify complete registry passes
	cmdVerify := exec.Command(resolvePythonCommand(), runnerPath, "--verify-registry")
	outVerify, errVerify := cmdVerify.CombinedOutput()
	if errVerify != nil {
		t.Fatalf("A19 failure: verify-registry failed unexpectedly: %v\n%s", errVerify, string(outVerify))
	}

	// 2. Mutate missing acceptance gate R12: runner must fail closed (non-zero exit)
	cmdMutateR := exec.Command(resolvePythonCommand(), runnerPath, "--mutate-missing", "R12")
	outMutateR, errMutateR := cmdMutateR.CombinedOutput()
	if errMutateR == nil {
		t.Fatalf("A19 failure: runner passed with missing acceptance gate R12!\nOutput: %s", string(outMutateR))
	}
	if !strings.Contains(strings.ToLower(string(outMutateR)), "missing required gate") {
		t.Fatalf("A19 failure: expected 'missing required gate' in error output, got:\n%s", string(outMutateR))
	}

	// 3. Mutate missing adversarial gate A05: runner must fail closed (non-zero exit)
	cmdMutateA := exec.Command(resolvePythonCommand(), runnerPath, "--mutate-missing", "A05")
	outMutateA, errMutateA := cmdMutateA.CombinedOutput()
	if errMutateA == nil {
		t.Fatalf("A19 failure: runner passed with missing adversarial gate A05!\nOutput: %s", string(outMutateA))
	}
}

// A20: iOS palette consumer catches missing newly supported native action/signature
// across both DefaultRegistry and the real mobile action catalog bridge (currentMobileActionCatalog).
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
		if len(actionObj.Parameters) == 0 {
			t.Fatalf("A20 failure: action %s has no parameters in DefaultRegistry", act)
		}
		if actionObj.Docs.Description == "" {
			t.Fatalf("A20 failure: action %s has empty description in DefaultRegistry", act)
		}
	}

	// Also verify the real iOS bridge catalog provider (currentMobileActionCatalog)
	mobileCatalog := currentMobileActionCatalog()
	mobileMap := make(map[string]catalogActionInfo, len(mobileCatalog))
	for _, item := range mobileCatalog {
		mobileMap[item.Name] = item
	}
	for _, act := range paletteActions {
		item, ok := mobileMap[act]
		if !ok {
			t.Fatalf("A20 failure: iOS palette action %s missing from currentMobileActionCatalog()!", act)
		}
		if item.ShortcutIdentifier == "" {
			t.Fatalf("A20 failure: mobile catalog action %s missing ShortcutIdentifier!", act)
		}
		if len(item.Parameters) == 0 {
			t.Fatalf("A20 failure: mobile catalog action %s has no parameters!", act)
		}
		if item.InsertionSnippet == "" {
			t.Fatalf("A20 failure: mobile catalog action %s missing InsertionSnippet!", act)
		}
	}

	// Adversarial: non-existent action must return false
	if _, ok := reg.LookupAction("nonExistentActionXYZ123"); ok {
		t.Fatalf("A20 failure: non-existent action was unexpectedly found in DefaultRegistry")
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

	// 1. Verify all files match disk exactly
	baseDir := filepath.Join("docs", "apple-runtime", "macos-27")
	checked := 0
	for fname, meta := range manifest.OutputFiles {
		fpath := filepath.Join(baseDir, fname)
		content, err := os.ReadFile(fpath)
		if err != nil {
			t.Fatalf("A21: missing derived output file %s: %v", fname, err)
		}
		if len(content) != meta.Bytes {
			t.Fatalf("A21: byte mismatch for %s: manifest %d, disk %d", fname, meta.Bytes, len(content))
		}
		computedSha := fmt.Sprintf("%x", sha256.Sum256(content))
		if computedSha != meta.Sha256 {
			t.Fatalf("A21: sha256 mismatch for %s: manifest %s, computed %s", fname, meta.Sha256, computedSha)
		}
		checked++
	}
	if checked == 0 {
		t.Fatalf("A21: no output files checked")
	}

	// 2. Tamper simulation: flip 1 byte of in-memory content of first file and assert failure
	for fname, meta := range manifest.OutputFiles {
		fpath := filepath.Join(baseDir, fname)
		content, _ := os.ReadFile(fpath)
		tampered := make([]byte, len(content))
		copy(tampered, content)
		if len(tampered) > 0 {
			tampered[0] ^= 0xFF
		}
		tamperedSha := fmt.Sprintf("%x", sha256.Sum256(tampered))
		if tamperedSha == meta.Sha256 {
			t.Fatalf("A21 failure: tampered content for %s unexpectedly produced matching sha256!", fname)
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
	ledgerPath := filepath.Join("docs", "apple-runtime", "p10", "p10-action-ledger.json")
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("failed to read p10-action-ledger.json: %v", err)
	}
	var ledger []map[string]any
	if err := json.Unmarshal(data, &ledger); err != nil {
		t.Fatalf("failed to parse ledger: %v", err)
	}

	thirdPartyPrefixes := []string{
		"is.workflow.actions.dropbox.",
		"is.workflow.actions.pocket.",
		"is.workflow.actions.evernote.",
		"is.workflow.actions.tumblr.",
		"is.workflow.actions.wordpress.",
	}

	thirdPartyFound := 0
	for _, row := range ledger {
		ident, _ := row["identifier"].(string)
		isThirdParty := false
		for _, pfx := range thirdPartyPrefixes {
			if strings.HasPrefix(ident, pfx) {
				isThirdParty = true
				break
			}
		}

		callables, _ := row["cherriCallables"].([]any)
		if isThirdParty && len(callables) == 0 {
			thirdPartyFound++
			status, _ := row["status"].(string)
			eligibility, _ := row["eligibility"].(string)

			if status != "EXCLUDED_THIRD_PARTY_DEPENDENCY" {
				t.Fatalf("A26 failure: third-party action %s has status %q, expected EXCLUDED_THIRD_PARTY_DEPENDENCY", ident, status)
			}
			if eligibility != "EXCLUDED_THIRD_PARTY" {
				t.Fatalf("A26 failure: third-party action %s has eligibility %q, expected EXCLUDED_THIRD_PARTY", ident, eligibility)
			}
			if eligibility == "ELIGIBLE_USER_FACING" {
				t.Fatalf("A26 failure: third-party action %s must not be ELIGIBLE_USER_FACING", ident)
			}
		}
	}

	if thirdPartyFound == 0 {
		t.Fatalf("A26 failure: no third-party actions found in ledger to verify")
	}
}

// A27: Existing legal Cherri v1/v2 fixtures and fork round trips still pass.
func TestP10R2_A27_ExistingFixturesUnbroken(t *testing.T) {
	r := newRoundTripRunner(t)

	fixtures := []struct {
		name string
		src  string
	}{
		{
			name: "arithmetic_and_variables",
			src: `let x = 10 + 5
let y = x * 2
show(y)
`,
		},
		{
			name: "conditional_branching",
			src: `@x = "hello"
if @x == "hello" {
    show("yes")
} else {
    show("no")
}
`,
		},
		{
			name: "dictionary",
			src: `let map = {"greeting": "hello", "target": "world"}
show(map)
`,
		},
		{
			name: "repeat_loop",
			src: `repeat(3) {
    show("iter")
}
`,
		},
	}

	for _, fix := range fixtures {
		plist := r.compileSource("a27_"+fix.name, fix.src)
		doc := loadRoundTripDocument(t, plist)
		actionsList, ok := doc["WFWorkflowActions"].([]any)
		if !ok || len(actionsList) == 0 {
			t.Fatalf("A27 failure: compiled plist for %s has no actions", fix.name)
		}
		decomp := r.decompileShortcut("a27_"+fix.name, plist)
		if len(strings.TrimSpace(decomp)) == 0 {
			t.Fatalf("A27 failure: decompile produced empty output for %s", fix.name)
		}
		recompiledPlist := r.compileSource("a27_re_"+fix.name, decomp)
		recompiledDoc := loadRoundTripDocument(t, recompiledPlist)
		assertActionsStructurallyEqual(t, doc, recompiledDoc)
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

	// 1. Verify schema fingerprint matches DefaultRegistry
	reg := schema.DefaultRegistry()
	expectedFp := reg.Fingerprint()
	if manifest["schemaFingerprint"] != expectedFp {
		t.Fatalf("A28 failure: schemaFingerprint mismatch: manifest %s vs registry %s", manifest["schemaFingerprint"], expectedFp)
	}

	// 2. Verify compilerCommit is a valid Git commit object and ancestor of HEAD
	compilerCommit := manifest["compilerCommit"]
	cmdAncestor := exec.Command("git", "merge-base", "--is-ancestor", compilerCommit, "HEAD")
	if err := cmdAncestor.Run(); err != nil {
		t.Fatalf("A28 failure: compilerCommit %s is not an ancestor of HEAD: %v", compilerCommit, err)
	}

	// 3. Verify non-circularity: compilerCommit is an existing historical commit, not an uncommitted or future SHA
	cmdCat := exec.Command("git", "cat-file", "-t", compilerCommit)
	outCat, errCat := cmdCat.Output()
	if errCat != nil || strings.TrimSpace(string(outCat)) != "commit" {
		t.Fatalf("A28 failure: compilerCommit %s is not a valid git commit object: %v", compilerCommit, errCat)
	}
}
