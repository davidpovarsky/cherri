package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// Synthetic control data stays in t.TempDir. It is never enrolled as a native
// run. The original Python producer and independent Go intake both read it.
type previewNativeControlModel struct {
	directory         string
	raw, contract     []byte
	proof, diagnostic map[string]any
	record            EvidenceRecord
	manifest          *EvidenceManifest
}

func previewNativeJSONLine(t *testing.T, prefix string, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return prefix + string(raw) + "\n"
}
func previewNativeControlBenchmark(t *testing.T, index int) map[string]any {
	t.Helper()
	fixture, err := previewNativeFixtureAt(index)
	if err != nil {
		t.Fatal(err)
	}
	hashes := []string{}
	timings := []float64{}
	for i := 0; i < 20; i++ {
		hashes = append(hashes, nativeEncodingDigest([]byte(fixture.Source+fmt.Sprintf("// measured unrelated revision %d\n", i))))
		timings = append(timings, float64(20-i)+0.25)
	}
	reads, encodes, hits := 0, 0, 0
	if index == 2 {
		reads, encodes, hits = 1, 1, 19
	}
	return map[string]any{"schema": "cherri-native-preview-benchmark/1", "fixture": fixture.Name,
		"sourceSha256": nativeEncodingDigest([]byte(fixture.Source)), "assetSha256": fixture.AssetSHA,
		"sampleSourceSha256": hashes, "sampleCount": 20, "totalCompileMilliseconds": timings,
		"p50Milliseconds": 10.25, "p95Milliseconds": 19.25, "osVersion": "Version 27.0 (Build 24A434)",
		"simulatorModel": "iPhone17,3", "mode": "v2", "buildProfile": "debug",
		"cacheReadsDelta": reads, "cacheEncodesDelta": encodes, "cacheHitsDelta": hits, "scope": previewNativeBenchmarkScope}
}
func previewNativeControlLog(t *testing.T, methods []string) []byte {
	t.Helper()
	var text strings.Builder
	for _, method := range methods {
		parts := strings.SplitN(method, "/", 2)
		target := "CherriTests"
		if method == previewNativeUI {
			target = "CherriUITests"
		}
		boundary := "Test Case '-[" + target + "." + parts[0] + " " + parts[1] + "]'"
		text.WriteString(boundary + " started.\n")
		if method == previewNativeBenchmark {
			for i := 0; i < 3; i++ {
				text.WriteString(previewNativeJSONLine(t, previewNativeBenchmarkPrefix, previewNativeControlBenchmark(t, i)))
			}
		}
		if method == previewNativeUI {
			for i, stage := range previewNativeStages {
				source := previewNativeUISource
				if i > 0 {
					source = strings.Replace(source, "BRG28_EDIT", "BRG28_CHANGED", 1)
				}
				event := map[string]any{"schema": "cherri-native-preview-ui/1", "testId": previewNativeUI,
					"stage": stage, "sourceSha256": nativeEncodingDigest([]byte(source))}
				if i == 1 {
					event["elapsedSeconds"] = 1.25
				}
				text.WriteString(previewNativeJSONLine(t, previewNativeUIPrefix, event))
			}
			text.WriteString(previewNativeCheckpoint + "\n")
		}
		text.WriteString(boundary + " passed (0.123 seconds).\n")
	}
	return []byte(text.String())
}
func newPreviewNativeControlModel(t *testing.T, test string) *previewNativeControlModel {
	t.Helper()
	profile, ok := previewNativeProfileFor(test)
	if !ok {
		t.Fatal(test)
	}
	contract, err := os.ReadFile(resolveRepoPath(previewNativeContractPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePreviewNativeContract(contract); err != nil {
		t.Fatal(err)
	}
	raw := previewNativeControlLog(t, previewNativeMethods)
	observations, err := parsePreviewNativeLog(raw)
	if err != nil {
		t.Fatal(err)
	}
	pins, err := previewNativeCurrentPins()
	if err != nil {
		t.Fatal(err)
	}
	source, run, attempt := strings.Repeat("a", 40), "1234567", 7
	proof := map[string]any{"schema": "cherri-native-preview-proof/1", "implementationSha": source,
		"runId": run, "runAttempt": attempt, "xcodebuildExitCode": 0, "logSha256": nativeEncodingDigest(raw),
		"contractSha256": previewNativeContractSHA, "originalGateSha256": previewNativeGateSHA,
		"sourcePins": pins, "observations": observations, "noCrossTierInheritance": true}
	results := []any{}
	for _, name := range previewNativeMethods {
		results = append(results, map[string]any{"testId": name, "observedStatus": "passed"})
	}
	ids := previewNativeTestIDs("BRG28")
	sort.Strings(ids)
	diagnostic := map[string]any{"schemaVersion": "1.0", "implementationSha": source, "runId": run,
		"xcodebuildExitCode": 0, "runStatus": "PASSED", "rawLogPresent": true, "diagnosticOnly": false,
		"testResults": results, "previewNativeEvidence": map[string]any{"status": "VERIFIED_NATIVE_PREVIEW",
			"errorCodes": []any{}, "recordTestIds": ids, "verifiedTests": previewNativeMethods,
			"noCrossTierInheritance": true, "runAttempt": attempt, "contractSha256": previewNativeContractSHA, "originalGateSha256": previewNativeGateSHA}}
	fixture, err := os.ReadFile(resolveRepoPath(profile.Fixture))
	if err != nil {
		t.Fatal(err)
	}
	m := &previewNativeControlModel{directory: t.TempDir(), raw: raw, contract: contract, proof: proof, diagnostic: diagnostic,
		record: EvidenceRecord{EvidenceID: "synthetic-native-preview-control", Requirements: []string{"BRG28"},
			Tier: profile.Tier, ImplementationSHA: source, FixtureSHA256: nativeEncodingDigest(fixture), TestID: test,
			Command: []string{"xcodebuild", "test", "-scheme", "Cherri"}, Result: "passed", Assertions: profile.Assertions, Detail: profile.Scope},
		manifest: &EvidenceManifest{SchemaVersion: "1.0", ImplementationSHA: source, Runs: []CIRunRecord{{
			RunID: run, Repository: "davidpovarsky/cherri", WorkflowName: "iOS Build", Event: "workflow_dispatch",
			Attempt: attempt, HeadSHA: source, CheckoutSHA: source, Conclusion: "success"}}}}
	m.seal(t)
	return m
}
func (m *previewNativeControlModel) changeLog(raw string) {
	m.raw = []byte(raw)
	m.proof["logSha256"] = nativeEncodingDigest(m.raw)
}
func (m *previewNativeControlModel) seal(t *testing.T) {
	t.Helper()
	proof, err := json.MarshalIndent(m.proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	m.diagnostic["sanitizedLogSha256"] = nativeEncodingDigest(m.raw)
	m.diagnostic["previewNativeEvidence"].(map[string]any)["proofSha256"] = nativeEncodingDigest(proof)
	diagnostic, err := json.MarshalIndent(m.diagnostic, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"xcodetest.log": m.raw, "preview-document-contract.json": m.contract,
		"preview-native-proof.json": proof, "ios-test-diagnostics.json": diagnostic}
	m.record.Artifacts = nil
	for _, name := range []string{"xcodetest.log", "preview-document-contract.json", "preview-native-proof.json", "ios-test-diagnostics.json"} {
		raw := files[name]
		path := filepath.Join(m.directory, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		m.record.Artifacts = append(m.record.Artifacts, EvidenceArtifact{Path: path, SHA256: nativeEncodingDigest(raw), RunID: "1234567"})
	}
}
func previewNativeRequireRejected(t *testing.T, m *previewNativeControlModel, want string) {
	t.Helper()
	err := validatePreviewNativeRecord("BRG28", m.record, m.manifest)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("causal %s rejection missing: %v", want, err)
	}
}
func TestPreviewNativeThreeDedicatedRecords(t *testing.T) {
	want := []string{previewNativeIntegration, previewNativeUI, previewNativeBenchmark}
	if !reflect.DeepEqual(previewNativeTestIDs("BRG28"), want) || previewNativeTestIDs("E07") != nil || previewNativeUIAssertions("E07") != nil {
		t.Fatal("dedicated BRG28 inventory changed")
	}
	for _, test := range want {
		t.Run(test, func(t *testing.T) {
			m := newPreviewNativeControlModel(t, test)
			if err := validatePreviewNativeRecord("BRG28", m.record, m.manifest); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, method := range append(append([]string{}, previewNativeMethods...), "CherriUITests/testVisualPreviewEditing") {
		if _, known := previewNativeProfileFor(method); known {
			continue
		}
		t.Run("not_enrolled/"+method, func(t *testing.T) {
			m := newPreviewNativeControlModel(t, previewNativeUI)
			m.record.TestID = method
			previewNativeRequireRejected(t, m, "PREVIEW_RECORD")
		})
	}
}
func TestPreviewNativeOriginalIntakeCorruptions(t *testing.T) {
	cases := []struct{ name, want string }{
		{"generic_e07", "PREVIEW_RECORD"}, {"tier", "PREVIEW_RECORD"}, {"requirement", "PREVIEW_RECORD"}, {"record_source", "PREVIEW_RECORD"},
		{"command", "PREVIEW_RECORD"}, {"scope", "PREVIEW_RECORD"}, {"assertion", "PREVIEW_RECORD"}, {"fixture", "PREVIEW_FIXTURE"},
		{"missing_artifact", "PREVIEW_ARTIFACTS"}, {"duplicate_artifact", "PREVIEW_ARTIFACTS"}, {"mixed_run", "PREVIEW_ARTIFACTS"},
		{"digest", "PREVIEW_ARTIFACT_DIGEST"}, {"symlink_original", "PREVIEW_ARTIFACTS"}, {"non_numeric_run", "PREVIEW_ARTIFACTS"},
		{"missing_run", "PREVIEW_RUN"}, {"duplicate_run", "PREVIEW_RUN"}, {"failed_run", "PREVIEW_RUN"}, {"checkout", "PREVIEW_RUN"},
		{"workflow", "PREVIEW_RUN"}, {"event", "PREVIEW_RUN"}, {"attempt_zero", "PREVIEW_RUN"},
		{"attempt_binding", "PREVIEW_PROOF"}, {"proof_source", "PREVIEW_PROOF"}, {"proof_log", "PREVIEW_PROOF"},
		{"proof_gate", "PREVIEW_PROOF"}, {"proof_observation", "PREVIEW_PROOF"}, {"proof_pin", "PREVIEW_PROOF"},
		{"missing_pin", "PREVIEW_PROOF"}, {"extra_proof", "PREVIEW_PROOF"}, {"cross_tier", "PREVIEW_PROOF"}, {"contract", "PREVIEW_CONTRACT"},
		{"process_exit", "PREVIEW_DIAGNOSTIC"}, {"diagnostic_only", "PREVIEW_DIAGNOSTIC"}, {"summary", "PREVIEW_DIAGNOSTIC"},
		{"missing_outcome", "PREVIEW_DIAGNOSTIC"}, {"failed_outcome", "PREVIEW_DIAGNOSTIC"}, {"duplicate_outcome", "PREVIEW_DIAGNOSTIC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newPreviewNativeControlModel(t, previewNativeUI)
			if err := validatePreviewNativeRecord("BRG28", m.record, m.manifest); err != nil {
				t.Fatal("causal baseline:", err)
			}
			reseal := false
			switch tc.name {
			case "generic_e07":
				m.record.TestID = "CherriEditorUITests/testE07VisualEditChangesOnlySelectedNode"
			case "tier":
				m.record.Tier = "unit"
			case "requirement":
				m.record.Requirements = []string{"E07"}
			case "record_source":
				m.record.ImplementationSHA = strings.Repeat("b", 40)
			case "command":
				m.record.Command = []string{"xcodebuild", "build"}
			case "scope":
				m.record.Detail = "Physical iPad runtime benchmark"
			case "assertion":
				delete(m.record.Assertions, "actual_webkit_contenteditable_keyboard")
			case "fixture":
				m.record.FixtureSHA256 = strings.Repeat("0", 64)
			case "missing_artifact":
				m.record.Artifacts = m.record.Artifacts[:3]
			case "duplicate_artifact":
				m.record.Artifacts[3] = m.record.Artifacts[2]
			case "mixed_run":
				m.record.Artifacts[0].RunID = "7654321"
			case "digest":
				m.record.Artifacts[0].SHA256 = strings.Repeat("0", 64)
			case "symlink_original":
				path := m.record.Artifacts[0].Path
				renamed := path + ".original"
				if err := os.Rename(path, renamed); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(renamed, path); err != nil {
					t.Fatal(err)
				}
			case "non_numeric_run":
				for i := range m.record.Artifacts {
					m.record.Artifacts[i].RunID = "local-mac"
				}
			case "missing_run":
				m.manifest.Runs = nil
			case "duplicate_run":
				m.manifest.Runs = append(m.manifest.Runs, m.manifest.Runs[0])
			case "failed_run":
				m.manifest.Runs[0].Conclusion = "failure"
			case "checkout":
				m.manifest.Runs[0].CheckoutSHA = strings.Repeat("b", 40)
			case "workflow":
				m.manifest.Runs[0].WorkflowName = "Build & Test"
			case "event":
				m.manifest.Runs[0].Event = "schedule"
			case "attempt_zero":
				m.manifest.Runs[0].Attempt = 0
			case "attempt_binding":
				m.manifest.Runs[0].Attempt++
			case "proof_source":
				m.proof["implementationSha"] = strings.Repeat("b", 40)
				reseal = true
			case "proof_log":
				m.proof["logSha256"] = strings.Repeat("0", 64)
				reseal = true
			case "proof_gate":
				m.proof["originalGateSha256"] = strings.Repeat("0", 64)
				reseal = true
			case "proof_observation":
				m.proof["observations"].(map[string]any)["uiEvents"] = []any{}
				reseal = true
			case "proof_pin":
				m.proof["sourcePins"].([]any)[0].(map[string]any)["gitBlob"] = strings.Repeat("0", 40)
				reseal = true
			case "missing_pin":
				m.proof["sourcePins"] = m.proof["sourcePins"].([]any)[1:]
				reseal = true
			case "extra_proof":
				m.proof["unbound"] = true
				reseal = true
			case "cross_tier":
				m.proof["noCrossTierInheritance"] = false
				reseal = true
			case "contract":
				m.contract = append(m.contract, ' ')
				reseal = true
			case "process_exit":
				m.diagnostic["xcodebuildExitCode"] = 65
				reseal = true
			case "diagnostic_only":
				m.diagnostic["diagnosticOnly"] = true
				reseal = true
			case "summary":
				m.diagnostic["previewNativeEvidence"].(map[string]any)["recordTestIds"] = []string{previewNativeUI}
				reseal = true
			case "missing_outcome":
				m.diagnostic["testResults"] = m.diagnostic["testResults"].([]any)[1:]
				reseal = true
			case "failed_outcome":
				m.diagnostic["testResults"].([]any)[0].(map[string]any)["observedStatus"] = "failed"
				reseal = true
			case "duplicate_outcome":
				m.diagnostic["testResults"] = append(m.diagnostic["testResults"].([]any), m.diagnostic["testResults"].([]any)[0])
				reseal = true
			default:
				t.Fatal(tc.name)
			}
			if reseal {
				m.seal(t)
			}
			previewNativeRequireRejected(t, m, tc.want)
		})
	}
}
func TestPreviewNativeCompleteCohortAndUIOriginals(t *testing.T) {
	for _, missing := range previewNativeMethods {
		t.Run("missing/"+missing, func(t *testing.T) {
			m := newPreviewNativeControlModel(t, previewNativeUI)
			methods := []string{}
			for _, method := range previewNativeMethods {
				if method != missing {
					methods = append(methods, method)
				}
			}
			m.changeLog(string(previewNativeControlLog(t, methods)))
			m.seal(t)
			previewNativeRequireRejected(t, m, "PREVIEW_COHORT")
		})
	}
	cases := []struct{ name, want string }{
		{"failed_case", "PREVIEW_CASE"}, {"duplicate_case", "PREVIEW_CASE"}, {"wrong_target", "PREVIEW_CASE"},
		{"no_terminal", "PREVIEW_COHORT"}, {"interleaved_case", "PREVIEW_CASE"}, {"rejected", "PREVIEW_LOG"},
		{"missing_ui", "PREVIEW_UI"}, {"duplicate_ui", "PREVIEW_UI"}, {"ui_source", "PREVIEW_UI"}, {"ui_duration", "PREVIEW_UI"},
		{"ui_extra", "PREVIEW_UI"}, {"ui_duplicate_key", "PREVIEW_UI"}, {"ui_foreign_owner", "PREVIEW_UI"},
		{"checkpoint_suffix", "PREVIEW_CHECKPOINT"}, {"checkpoint_duplicate", "PREVIEW_CHECKPOINT"},
		{"checkpoint_missing", "PREVIEW_COHORT"}, {"checkpoint_outside", "PREVIEW_CHECKPOINT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newPreviewNativeControlModel(t, previewNativeUI)
			text := string(m.raw)
			lines := strings.Split(text, "\n")
			firstUI := ""
			for _, line := range lines {
				if strings.HasPrefix(line, previewNativeUIPrefix) {
					firstUI = line
					break
				}
			}
			switch tc.name {
			case "failed_case":
				text = strings.Replace(text, "passed (", "failed (", 1)
			case "duplicate_case":
				text += text
			case "wrong_target":
				text = strings.Replace(text, "CherriTests.CherriPreviewDocumentContextTests", "CherriUITests.CherriPreviewDocumentContextTests", 1)
			case "no_terminal":
				last := "Test Case '-[CherriUITests.CherriPreviewDocumentUITests testBRG28ActualWebKitFieldEditAndNumericRejectionUI]' passed (0.123 seconds).\n"
				if !strings.Contains(text, last) {
					t.Fatal("no terminal fixture")
				}
				text = strings.Replace(text, last, "", 1)
			case "interleaved_case":
				text = strings.Replace(text, " started.\n", " started.\nTest Case '-[OtherTests.ForeignTests testOther]' started.\n", 1)
			case "rejected":
				text += previewNativeRejected + "INVALID_UI_EVENT\n"
			case "missing_ui":
				text = strings.Replace(text, firstUI+"\n", "", 1)
			case "duplicate_ui":
				text = strings.Replace(text, firstUI, firstUI+"\n"+firstUI, 1)
			case "ui_source":
				text = strings.Replace(text, firstUI, strings.Replace(firstUI, "sourceSha256\":\"", "sourceSha256\":\"0", 1), 1)
			case "ui_duration":
				text = strings.Replace(text, "\"elapsedSeconds\":1.25", "\"elapsedSeconds\":0", 1)
			case "ui_extra":
				text = strings.Replace(text, firstUI, strings.TrimSuffix(firstUI, "}")+",\"private\":\"not allowed\"}", 1)
			case "ui_duplicate_key":
				text = strings.Replace(text, firstUI, strings.TrimSuffix(firstUI, "}")+",\"stage\":\"actual-source-built\"}", 1)
			case "ui_foreign_owner":
				text = firstUI + "\n" + text
			case "checkpoint_suffix":
				text = strings.Replace(text, previewNativeCheckpoint, previewNativeCheckpoint+" EXTRA", 1)
			case "checkpoint_duplicate":
				text = strings.Replace(text, previewNativeCheckpoint, previewNativeCheckpoint+"\n"+previewNativeCheckpoint, 1)
			case "checkpoint_missing":
				text = strings.Replace(text, previewNativeCheckpoint+"\n", "", 1)
			case "checkpoint_outside":
				text = previewNativeCheckpoint + "\n" + text
			default:
				t.Fatal(tc.name)
			}
			if text == string(m.raw) {
				t.Fatal("negative did not change original")
			}
			m.changeLog(text)
			m.seal(t)
			previewNativeRequireRejected(t, m, tc.want)
		})
	}
}
func TestPreviewNativeSixtyOriginalTimingAndCacheGuards(t *testing.T) {
	cases := []string{"fixture", "source", "sample_source", "duplicate_revision", "missing_revision", "asset",
		"missing_sample", "zero_sample", "negative_sample", "string_sample", "sample_count", "p50", "p95", "mode", "profile", "scope",
		"os_missing", "os_claimed", "model", "cache_fraction", "cache_reads", "cache_encodes", "cache_hits", "extra_key",
		"duplicate_key", "duplicate_observation", "missing_observation", "foreign_owner", "wrong_order"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			m := newPreviewNativeControlModel(t, previewNativeBenchmark)
			lines := strings.Split(string(m.raw), "\n")
			indices := []int{}
			for i, line := range lines {
				if strings.HasPrefix(line, previewNativeBenchmarkPrefix) {
					indices = append(indices, i)
				}
			}
			if len(indices) != 3 {
				t.Fatal("benchmark fixture")
			}
			index := indices[2]
			value, err := decodeNativeEncodingObject([]byte(strings.TrimPrefix(lines[index], previewNativeBenchmarkPrefix)))
			if err != nil {
				t.Fatal(err)
			}
			rawReplacement := ""
			direct := false
			want := "PREVIEW_BENCHMARK"
			switch name {
			case "fixture":
				value["fixture"] = "small"
			case "source":
				value["sourceSha256"] = strings.Repeat("0", 64)
			case "sample_source":
				value["sampleSourceSha256"].([]any)[4] = strings.Repeat("0", 64)
			case "duplicate_revision":
				value["sampleSourceSha256"].([]any)[4] = value["sampleSourceSha256"].([]any)[3]
			case "missing_revision":
				value["sampleSourceSha256"] = value["sampleSourceSha256"].([]any)[:19]
			case "asset":
				value["assetSha256"] = strings.Repeat("0", 64)
			case "missing_sample":
				value["totalCompileMilliseconds"] = value["totalCompileMilliseconds"].([]any)[:19]
			case "zero_sample":
				value["totalCompileMilliseconds"].([]any)[4] = 0
			case "negative_sample":
				value["totalCompileMilliseconds"].([]any)[4] = -1
			case "string_sample":
				value["totalCompileMilliseconds"].([]any)[4] = "16.25"
			case "sample_count":
				value["sampleCount"] = 19
			case "p50":
				value["p50Milliseconds"] = 10.5
			case "p95":
				value["p95Milliseconds"] = 19.5
			case "mode":
				value["mode"] = "legacy"
			case "profile":
				value["buildProfile"] = "release"
			case "scope":
				value["scope"] = "physical-ipad-shortcut-runtime"
			case "os_missing":
				value["osVersion"] = ""
			case "os_claimed":
				value["osVersion"] = "latest"
			case "model":
				value["simulatorModel"] = "Mac14,2"
			case "cache_fraction":
				value["cacheReadsDelta"] = 1.5
			case "cache_reads":
				value["cacheReadsDelta"] = 2
			case "cache_encodes":
				value["cacheEncodesDelta"] = 2
			case "cache_hits":
				value["cacheHitsDelta"] = 18
			case "extra_key":
				value["userPath"] = "/private/not permitted"
			case "duplicate_key":
				rawReplacement = strings.TrimSuffix(lines[index], "}") + ",\"sampleCount\":20}"
				direct = true
			case "duplicate_observation":
				rawReplacement = lines[index] + "\n" + lines[index]
				direct = true
			case "missing_observation":
				rawReplacement = ""
				direct = true
				want = "PREVIEW_COHORT"
			case "foreign_owner":
				rawReplacement = lines[index] + "\n" + strings.Join(lines, "\n")
				direct = true
			case "wrong_order":
				lines[indices[0]], lines[indices[2]] = lines[indices[2]], lines[indices[0]]
				rawReplacement = lines[index]
				direct = true
			default:
				t.Fatal(name)
			}
			if name == "foreign_owner" {
				m.changeLog(rawReplacement)
			} else {
				if !direct {
					rawReplacement = strings.TrimSuffix(previewNativeJSONLine(t, previewNativeBenchmarkPrefix, value), "\n")
				}
				lines[index] = rawReplacement
				m.changeLog(strings.Join(lines, "\n"))
			}
			m.seal(t)
			previewNativeRequireRejected(t, m, want)
		})
	}
}
func TestPreviewNativeOriginalCollector(t *testing.T) {
	m := newPreviewNativeControlModel(t, previewNativeUI)
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	headRaw, err := exec.Command("git", "--no-replace-objects", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(headRaw))
	rawPath, exitPath, output := filepath.Join(m.directory, "raw.log"), filepath.Join(m.directory, "xcode.exit"), filepath.Join(m.directory, "collector")
	if err := os.WriteFile(rawPath, m.raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exitPath, []byte("0"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-B", filepath.Join(root, "ios/scripts/collect_ios_test_evidence.py"), rawPath, output, head, "1234567", exitPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GITHUB_RUN_ATTEMPT=7")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("original collector: %v %s", err, out)
	}
	evidence, err := os.ReadFile(filepath.Join(output, "ios-test-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ Records []EvidenceRecord }
	if err := json.Unmarshal(evidence, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Records) != 3 {
		t.Fatalf("only three dedicated records required; got%d: %s", len(result.Records), evidence)
	}
	m.manifest.ImplementationSHA = head
	m.manifest.Runs[0].HeadSHA = head
	m.manifest.Runs[0].CheckoutSHA = head
	tiers := map[string]bool{}
	for _, record := range result.Records {
		for i := range record.Artifacts {
			record.Artifacts[i].Path = filepath.Join(output, filepath.Base(record.Artifacts[i].Path))
		}
		if err := validatePreviewNativeRecord("BRG28", record, m.manifest); err != nil {
			t.Fatal(err)
		}
		tiers[record.Tier] = true
	}
	if !reflect.DeepEqual(tiers, map[string]bool{"integration": true, "ios-ui": true, "benchmark": true}) {
		t.Fatal(tiers)
	}
}

func TestPreviewNativeEquivalentFiniteJSONMeasurements(t *testing.T) {
	m := newPreviewNativeControlModel(t, previewNativeBenchmark)
	text := strings.Replace(string(m.raw), `"elapsedSeconds":1.25`, `"elapsedSeconds":1.25e0`, 1)
	text = strings.ReplaceAll(text, "20.25", "20.25e0")
	if text == string(m.raw) {
		t.Fatal("numeric spelling control did not change original")
	}
	m.changeLog(text)
	m.seal(t)
	if err := validatePreviewNativeRecord("BRG28", m.record, m.manifest); err != nil {
		t.Fatal("same measured Double rejected:", err)
	}
	m.changeLog(strings.Replace(text, "1.25e0", "1.250001e0", 1))
	m.seal(t)
	previewNativeRequireRejected(t, m, "PREVIEW_PROOF")
}
