package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// These synthetic records are corruption controls, never exported native runs.
type joinedNativeIntakeModel struct {
	directory         string
	raw, contract     []byte
	proof, diagnostic map[string]any
	record            EvidenceRecord
	manifest          *EvidenceManifest
}

func joinedNativeSyntheticLog(t *testing.T, tests []string) ([]byte, []byte) {
	t.Helper()
	raw, err := os.ReadFile(resolveRepoPath(joinedNativeContractPath))
	if err != nil {
		t.Fatal(err)
	}
	contract, err := loadJoinedNativeContract(raw)
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, test := range tests {
		profile, _ := joinedNativeProfileFor(test)
		parts := strings.Split(test, "/")
		target := "CherriUITests"
		if profile.Producer == "ios-bridge" {
			target = "CherriTests"
		}
		boundary := "Test Case '-[" + target + "." + parts[0] + " " + parts[1] + "]'"
		text.WriteString(boundary + " started.\n")
		for index := range contract.Methods[test].Stages {
			event := joinedNativeExpectedEvent(contract, nativeEncodingDigest(raw), test, index)
			encoded, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			text.WriteString(joinedNativePrefix + string(encoded) + "\n")
		}
		text.WriteString("[CHECKPOINT:" + profile.Checkpoint + "]\n" + boundary + " passed (0.123 seconds).\n")
	}
	return []byte(text.String()), raw
}

func newJoinedNativeIntakeModel(t *testing.T, test string) *joinedNativeIntakeModel {
	t.Helper()
	profile, ok := joinedNativeProfileFor(test)
	if !ok {
		t.Fatal(test)
	}
	raw, contractRaw := joinedNativeSyntheticLog(t, []string{joinedBRG25Bridge, joinedBRG27Bridge, joinedBRG25UI, joinedBRG27UI})
	contract, err := loadJoinedNativeContract(contractRaw)
	if err != nil {
		t.Fatal(err)
	}
	methods, err := parseJoinedNativeLog(raw, contract, nativeEncodingDigest(contractRaw))
	if err != nil {
		t.Fatal(err)
	}
	pins, err := joinedNativeCurrentPins()
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Repeat("a", 40)
	run := "1234567"
	attempt := 7
	proof := map[string]any{"schema": "cherri-joined-native-editor-proof/1", "implementationSha": source,
		"runId": run, "runAttempt": attempt, "xcodebuildExitCode": 0, "logSha256": nativeEncodingDigest(raw),
		"contractSha256": nativeEncodingDigest(contractRaw), "sourcePins": pins, "methods": methods, "noCrossTierInheritance": true}
	verified, records := joinedNativeMethodInventory(methods)
	results := []any{}
	for _, name := range verified {
		results = append(results, map[string]any{"testId": name, "observedStatus": "passed"})
	}
	diagnostic := map[string]any{"schemaVersion": "1.0", "implementationSha": source, "runId": run,
		"xcodebuildExitCode": 0, "runStatus": "PASSED", "rawLogPresent": true, "diagnosticOnly": false,
		"testResults": results, "joinedEditorEvidence": map[string]any{"status": "VERIFIED_JOINED_NATIVE",
			"errorCodes": []any{}, "noCrossTierInheritance": true, "runAttempt": attempt,
			"contractSha256": nativeEncodingDigest(contractRaw), "verifiedTests": verified, "recordTestIds": records}}
	fixture, err := os.ReadFile(resolveRepoPath(profile.Fixture))
	if err != nil {
		t.Fatal(err)
	}
	model := &joinedNativeIntakeModel{directory: t.TempDir(), raw: raw, contract: contractRaw, proof: proof, diagnostic: diagnostic,
		record: EvidenceRecord{EvidenceID: "synthetic-joined-native-control", Requirements: []string{profile.Requirement},
			Tier: profile.Tier, ImplementationSHA: source, FixtureSHA256: nativeEncodingDigest(fixture),
			TestID: test, Command: []string{"xcodebuild", "test", "-scheme", "Cherri"}, Result: "passed",
			Assertions: profile.Assertions, Detail: profile.Scope},
		manifest: &EvidenceManifest{SchemaVersion: "1.0", ImplementationSHA: source, Runs: []CIRunRecord{{
			RunID: run, Repository: "davidpovarsky/cherri", WorkflowName: "iOS Build", Event: "workflow_dispatch",
			Attempt: attempt, HeadSHA: source, CheckoutSHA: source, Conclusion: "success"}}}}
	model.seal(t)
	return model
}

func (m *joinedNativeIntakeModel) changeLog(text string) {
	m.raw = []byte(text)
	m.proof["logSha256"] = nativeEncodingDigest(m.raw)
}

func (m *joinedNativeIntakeModel) seal(t *testing.T) {
	t.Helper()
	proof, err := json.MarshalIndent(m.proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	m.diagnostic["sanitizedLogSha256"] = nativeEncodingDigest(m.raw)
	m.diagnostic["joinedEditorEvidence"].(map[string]any)["proofSha256"] = nativeEncodingDigest(proof)
	diagnostic, err := json.MarshalIndent(m.diagnostic, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"xcodetest.log": m.raw, "ios-test-diagnostics.json": diagnostic,
		"joined-editor-contract.json": m.contract, "joined-editor-proof.json": proof}
	m.record.Artifacts = nil
	for _, name := range []string{"xcodetest.log", "ios-test-diagnostics.json", "joined-editor-contract.json", "joined-editor-proof.json"} {
		path := filepath.Join(m.directory, name)
		raw := files[name]
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		m.record.Artifacts = append(m.record.Artifacts, EvidenceArtifact{Path: path, SHA256: nativeEncodingDigest(raw), RunID: "1234567"})
	}
}

func TestJoinedNativeEditorThreeDedicatedRecords(t *testing.T) {
	for _, test := range []string{joinedBRG25Bridge, joinedBRG25UI, joinedBRG27UI} {
		t.Run(test, func(t *testing.T) {
			model := newJoinedNativeIntakeModel(t, test)
			if err := validateJoinedNativeEditorRecord(model.record.Requirements[0], model.record, model.manifest); err != nil {
				t.Fatal(err)
			}
		})
	}
	if !reflect.DeepEqual(joinedNativeEditorTestIDs("BRG25"), []string{joinedBRG25Bridge, joinedBRG25UI}) ||
		!reflect.DeepEqual(joinedNativeEditorTestIDs("BRG27"), []string{joinedBRG27UI}) {
		t.Fatal("auxiliary native bridge or generic case entered formal inventory")
	}
	aux := newJoinedNativeIntakeModel(t, joinedBRG27Bridge)
	if err := validateJoinedNativeEditorRecord("BRG27", aux.record, aux.manifest); err == nil {
		t.Fatal("BRG27 C ABI case acquired a formal LSP/service/native tier")
	}
}

func TestJoinedNativeEditorOriginalIntakeCorruptions(t *testing.T) {
	cases := []struct{ name, want string }{
		{"borrowed_case", "JOINED_RECORD"}, {"wrong_tier", "JOINED_RECORD"}, {"wrong_requirement", "JOINED_RECORD"},
		{"record_source", "JOINED_RECORD"}, {"command", "JOINED_RECORD"}, {"scope", "JOINED_RECORD"}, {"assertion", "JOINED_RECORD"},
		{"fixture", "JOINED_FIXTURE"}, {"missing_artifact", "JOINED_ARTIFACTS"}, {"mixed_run", "JOINED_ARTIFACTS"},
		{"duplicate_artifact", "JOINED_ARTIFACTS"}, {"digest", "JOINED_ARTIFACT_DIGEST"},
		{"missing_run", "JOINED_RUN"}, {"duplicate_run", "JOINED_RUN"}, {"failed_run", "JOINED_RUN"},
		{"checkout", "JOINED_RUN"}, {"workflow", "JOINED_RUN"}, {"event", "JOINED_RUN"},
		{"attempt", "JOINED_PROOF"}, {"proof_source", "JOINED_PROOF"}, {"proof_log", "JOINED_PROOF"},
		{"proof_method", "JOINED_PROOF"}, {"source_pin", "JOINED_PROOF"}, {"extra_source_pin", "JOINED_PROOF"},
		{"extra_proof_key", "JOINED_PROOF"}, {"cross_tier", "JOINED_PROOF"}, {"contract", "JOINED_CONTRACT"},
		{"process_exit", "JOINED_DIAGNOSTIC"}, {"diagnostic_only", "JOINED_DIAGNOSTIC"},
		{"scope_summary", "JOINED_DIAGNOSTIC"}, {"missing_outcome", "JOINED_DIAGNOSTIC"},
		{"failed_outcome", "JOINED_DIAGNOSTIC"}, {"duplicate_outcome", "JOINED_DIAGNOSTIC"},
		{"missing_stage", "JOINED_STAGE"}, {"duplicate_stage", "JOINED_STAGE"}, {"stage_hash", "JOINED_STAGE"},
		{"stage_sequence", "JOINED_STAGE"}, {"stage_unknown_key", "JOINED_STAGE"}, {"stage_duplicate_key", "JOINED_STAGE"},
		{"checkpoint_suffix", "JOINED_CHECKPOINT"}, {"checkpoint_duplicate", "JOINED_CHECKPOINT"},
		{"checkpoint_missing", "JOINED_METHOD"}, {"checkpoint_outside", "JOINED_CHECKPOINT"},
		{"failed_case", "JOINED_CASE"}, {"duplicate_case", "JOINED_CASE"}, {"wrong_target", "JOINED_CASE"},
		{"rejected_marker", "JOINED_LOG"}, {"missing_companion", "JOINED_COMPANION"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newJoinedNativeIntakeModel(t, joinedBRG27UI)
			if err := validateJoinedNativeEditorRecord("BRG27", m.record, m.manifest); err != nil {
				t.Fatal("causal base rejected:", err)
			}
			reseal := false
			switch tc.name {
			case "borrowed_case":
				m.record.TestID = "CherriUITests/testBRG27ActualEditorWorkflowUI"
			case "wrong_tier":
				m.record.Tier = "service"
			case "wrong_requirement":
				m.record.Requirements = []string{"BRG25"}
			case "record_source":
				m.record.ImplementationSHA = strings.Repeat("b", 40)
			case "command":
				m.record.Command = []string{"xcodebuild", "build"}
			case "scope":
				m.record.Detail = "all 179 requirements passed"
			case "assertion":
				delete(m.record.Assertions, "shared_textedit_applied_ui")
			case "fixture":
				m.record.FixtureSHA256 = strings.Repeat("0", 64)
			case "missing_artifact":
				m.record.Artifacts = m.record.Artifacts[:3]
			case "mixed_run":
				m.record.Artifacts[0].RunID = "999"
			case "duplicate_artifact":
				m.record.Artifacts[3] = m.record.Artifacts[2]
			case "digest":
				m.record.Artifacts[0].SHA256 = strings.Repeat("0", 64)
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
			case "attempt":
				m.manifest.Runs[0].Attempt++
			case "proof_source":
				m.proof["implementationSha"] = strings.Repeat("b", 40)
				reseal = true
			case "proof_log":
				m.proof["logSha256"] = strings.Repeat("0", 64)
				reseal = true
			case "proof_method":
				delete(m.proof["methods"].(map[string]any), joinedBRG25UI)
				reseal = true
			case "source_pin":
				m.proof["sourcePins"].([]any)[0].(map[string]any)["sha256"] = strings.Repeat("0", 64)
				reseal = true
			case "extra_source_pin":
				m.proof["sourcePins"] = append(m.proof["sourcePins"].([]any), m.proof["sourcePins"].([]any)[0])
				reseal = true
			case "extra_proof_key":
				m.proof["extra"] = true
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
			case "scope_summary":
				m.diagnostic["joinedEditorEvidence"].(map[string]any)["recordTestIds"] = []string{joinedBRG27Bridge}
				reseal = true
			case "missing_outcome":
				m.diagnostic["testResults"] = m.diagnostic["testResults"].([]any)[:1]
				reseal = true
			case "failed_outcome":
				m.diagnostic["testResults"].([]any)[0].(map[string]any)["observedStatus"] = "failed"
				reseal = true
			case "duplicate_outcome":
				m.diagnostic["testResults"] = append(m.diagnostic["testResults"].([]any), m.diagnostic["testResults"].([]any)[0])
				reseal = true
			case "missing_stage", "duplicate_stage", "stage_hash", "stage_sequence", "stage_unknown_key", "stage_duplicate_key":
				lines := strings.Split(string(m.raw), "\n")
				index := 0
				for i, line := range lines {
					if strings.HasPrefix(line, joinedNativePrefix) {
						index = i
						break
					}
				}
				line := lines[index]
				replacement := line
				switch tc.name {
				case "missing_stage":
					replacement = ""
				case "duplicate_stage":
					replacement = line + "\n" + line
				case "stage_hash":
					replacement = strings.Replace(line, `"inputSha256":"`, `"inputSha256":"0`, 1)
				case "stage_sequence":
					replacement = strings.Replace(line, `"sequence":0`, `"sequence":0.0`, 1)
				case "stage_unknown_key":
					replacement = strings.TrimSuffix(line, "}") + `,"extra":true}`
				case "stage_duplicate_key":
					replacement = strings.TrimSuffix(line, "}") + `,"sequence":0}`
				}
				m.changeLog(strings.Replace(string(m.raw), line, replacement, 1))
				reseal = true
			case "checkpoint_suffix", "checkpoint_duplicate", "checkpoint_missing", "checkpoint_outside":
				checkpoint := "[CHECKPOINT:BRG25_NATIVE_MOBILE_JOINED_VERIFIED]"
				replacement := checkpoint
				switch tc.name {
				case "checkpoint_suffix":
					replacement += " EXTRA"
				case "checkpoint_duplicate":
					replacement += "\n" + checkpoint
				case "checkpoint_missing":
					replacement = ""
				case "checkpoint_outside":
					m.changeLog(checkpoint + "\n" + string(m.raw))
					reseal = true
				}
				if tc.name != "checkpoint_outside" {
					m.changeLog(strings.Replace(string(m.raw), checkpoint, replacement, 1))
					reseal = true
				}
			case "failed_case":
				m.changeLog(strings.Replace(string(m.raw), "passed (", "failed (", 1))
				reseal = true
			case "duplicate_case":
				m.changeLog(string(m.raw) + string(m.raw))
				reseal = true
			case "wrong_target":
				m.changeLog(strings.Replace(string(m.raw), "CherriTests.CherriJoinedBridgeTests", "CherriUITests.CherriJoinedBridgeTests", 1))
				reseal = true
			case "rejected_marker":
				m.changeLog(string(m.raw) + joinedNativeRejected + "INVALID_EVENT\n")
				reseal = true
			case "missing_companion":
				raw, _ := joinedNativeSyntheticLog(t, []string{joinedBRG25Bridge, joinedBRG25UI, joinedBRG27UI})
				m.changeLog(string(raw))
				reseal = true
			default:
				t.Fatal("unknown corruption")
			}
			if reseal {
				m.seal(t)
			}
			if err := validateJoinedNativeEditorRecord("BRG27", m.record, m.manifest); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("causal %s guard missing: %v", tc.want, err)
			}
		})
	}
}

func TestJoinedNativeEditorOriginalCollector(t *testing.T) {
	m := newJoinedNativeIntakeModel(t, joinedBRG27UI)
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	headCmd := exec.Command("git", "--no-replace-objects", "-C", root, "rev-parse", "HEAD")
	headRaw, err := headCmd.Output()
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
	cmd := exec.CommandContext(ctx, "python3", "-B", filepath.Join(root, "ios/scripts/collect_ios_test_evidence.py"),
		rawPath, output, head, "1234567", exitPath)
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
		t.Fatalf("exact three tiers required; got %d: %s", len(result.Records), evidence)
	}
	m.manifest.ImplementationSHA = head
	m.manifest.Runs[0].HeadSHA = head
	m.manifest.Runs[0].CheckoutSHA = head
	for _, record := range result.Records {
		for i := range record.Artifacts {
			record.Artifacts[i].Path = filepath.Join(output, filepath.Base(record.Artifacts[i].Path))
		}
		if err := validateJoinedNativeEditorRecord(record.Requirements[0], record, m.manifest); err != nil {
			t.Fatal(err)
		}
	}
}

func TestJoinedNativeEditorTerminalTargetOnlyRelabels(t *testing.T) {
	for _, method := range []string{joinedBRG25Bridge, joinedBRG27Bridge, joinedBRG25UI, joinedBRG27UI} {
		t.Run(method, func(t *testing.T) {
			m := newJoinedNativeIntakeModel(t, joinedBRG27UI)
			if err := validateJoinedNativeEditorRecord("BRG27", m.record, m.manifest); err != nil {
				t.Fatal("causal baseline:", err)
			}
			profile, _ := joinedNativeProfileFor(method)
			parts := strings.SplitN(method, "/", 2)
			target, other := "CherriUITests", "CherriTests"
			if profile.Producer == "ios-bridge" {
				target, other = other, target
			}
			terminal := "Test Case '-[" + target + "." + parts[0] + " " + parts[1] + "]' passed (0.123 seconds)."
			replacement := "Test Case '-[" + other + "." + parts[0] + " " + parts[1] + "]' passed (0.123 seconds)."
			text := string(m.raw)
			if strings.Count(text, terminal) != 1 {
				t.Fatal("missing unique original terminal")
			}
			text = strings.Replace(text, terminal, replacement, 1)
			if !strings.Contains(text, "Test Case '-["+target+"."+parts[0]+" "+parts[1]+"]' started.") {
				t.Fatal("start was incorrectly mutated")
			}
			m.changeLog(text)
			m.seal(t)
			if err := validateJoinedNativeEditorRecord("BRG27", m.record, m.manifest); err == nil || !strings.Contains(err.Error(), "JOINED_CASE") {
				t.Fatalf("terminal-only target relabel was not rejected: %v", err)
			}
		})
	}
}

// Two new synthetic native-intake models exercise only the new dedicated
// records. They are not exported as executions of XCTest or actual app UI.
func newRP39RP41NativeIntakeModel(t *testing.T, test string) *joinedNativeIntakeModel {
	t.Helper()
	m := newJoinedNativeIntakeModel(t, test)
	raw, contractRaw := joinedNativeSyntheticLog(t, []string{test})
	contract, err := loadJoinedNativeContract(contractRaw)
	if err != nil {
		t.Fatal(err)
	}
	methods, err := parseJoinedNativeLog(raw, contract, nativeEncodingDigest(contractRaw))
	if err != nil {
		t.Fatal(err)
	}
	verified, records := joinedNativeMethodInventory(methods)
	m.changeLog(string(raw))
	m.proof["methods"] = methods
	m.diagnostic["testResults"] = []any{map[string]any{"testId": test, "observedStatus": "passed"}}
	summary := m.diagnostic["joinedEditorEvidence"].(map[string]any)
	summary["verifiedTests"], summary["recordTestIds"] = verified, records
	m.seal(t)
	return m
}

func TestJoinedNativeEditorRP39RP41DedicatedRecords(t *testing.T) {
	for _, row := range []struct{ id, test string }{{"RP39", joinedRP39UI}, {"RP41", joinedRP41UI}} {
		t.Run(row.id, func(t *testing.T) {
			m := newRP39RP41NativeIntakeModel(t, row.test)
			validate := validateCombinedEditorRecordWithManifest
			if row.id == "RP41" {
				validate = validatePreviewEditRecordWithManifest
			}
			if err := validate(row.id, m.record, m.manifest); err != nil {
				t.Fatal("healthy synthetic native boundary:", err)
			}
			if err := validate(row.id, m.record, nil); err == nil {
				t.Fatal("a native row acquired credit without actual run authority")
			}
			if !reflect.DeepEqual(joinedNativeEditorTestIDs(row.id), []string{row.test}) {
				t.Fatal("generic or auxiliary producer entered dedicated inventory")
			}
			ids := []string{}
			assertions := map[string]string{}
			applyRuntimeAndUIMappings(row.id, []string{"ios-ui"}, &ids, map[string]string{}, assertions)
			if !reflect.DeepEqual(ids, []string{row.test}) || !reflect.DeepEqual(assertions, m.record.Assertions) {
				t.Fatal("current requirement mapping detached from dedicated UI scope")
			}
		})
	}
}

func TestJoinedNativeEditorRP39RP41OriginalBoundaryControls(t *testing.T) {
	for _, row := range []struct{ id, test, borrowed string }{
		{"RP39", joinedRP39UI, joinedBRG27UI},
		{"RP41", joinedRP41UI, "CherriEditorUITests/testE07VisualParameterEditingCanvasUI"},
	} {
		t.Run(row.id, func(t *testing.T) {
			for _, fault := range []string{"borrowed_case", "wrong_tier", "record_source", "missing_stage",
				"missing_observer_pin", "failed_run", "failed_outcome", "local_producer_relabel"} {
				t.Run(fault, func(t *testing.T) {
					m := newRP39RP41NativeIntakeModel(t, row.test)
					validate := validateCombinedEditorRecordWithManifest
					if row.id == "RP41" {
						validate = validatePreviewEditRecordWithManifest
					}
					if err := validate(row.id, m.record, m.manifest); err != nil {
						t.Fatal("causal healthy model:", err)
					}
					reseal := false
					switch fault {
					case "borrowed_case":
						m.record.TestID = row.borrowed
					case "wrong_tier":
						m.record.Tier = "integration"
					case "record_source":
						m.record.ImplementationSHA = strings.Repeat("b", 40)
					case "missing_stage":
						lines := strings.Split(string(m.raw), "\n")
						for index, line := range lines {
							if strings.HasPrefix(line, joinedNativePrefix) {
								lines = append(lines[:index], lines[index+1:]...)
								break
							}
						}
						m.changeLog(strings.Join(lines, "\n"))
						reseal = true
					case "missing_observer_pin":
						pins := m.proof["sourcePins"].([]any)
						found := false
						for index, item := range pins {
							if item.(map[string]any)["path"] == "ios/CherriApp/Diagnostics/EditorDeliveryObservation.swift" {
								m.proof["sourcePins"] = append(pins[:index], pins[index+1:]...)
								found = true
								break
							}
						}
						if !found {
							t.Fatal("new observer was absent from the healthy source authority")
						}
						reseal = true
					case "failed_run":
						m.manifest.Runs[0].Conclusion = "failure"
					case "failed_outcome":
						m.diagnostic["testResults"].([]any)[0].(map[string]any)["observedStatus"] = "failed"
						reseal = true
					case "local_producer_relabel":
						m.record.TestID = combinedEditorTestIDs()[0]
						if row.id == "RP41" {
							m.record.TestID = previewEditTestIDs()[0]
						}
					}
					if reseal {
						m.seal(t)
					}
					if err := validate(row.id, m.record, m.manifest); err == nil {
						t.Fatalf("new dedicated native boundary accepted %s", fault)
					}
				})
			}
		})
	}
}
