package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This is a deliberately synthetic intake model. It exercises receipt rejection
// and is never an authorized source for S03/E08/RP29 production evidence.
func editorIntakeModel() editorPerformanceReport {
	report := editorPerformanceReport{
		FormatVersion: 1, SamplesPerPhase: 20,
		Environment: editorPerformanceEnvironment{
			SourceCommit: strings.Repeat("a", 40), SourceTreeClean: true, GoVersion: "go1.25.6", OS: "linux", Arch: "arm64",
			CPU: "synthetic intake-control model; not a measured host", LogicalCPUs: 4, GOMAXPROCS: 1,
			MemoryBytes: 16 << 30, Clock: "Go monotonic time.Since; microseconds; nearest-rank percentiles",
			ClockFrequencyHz: 1_000_000_000,
			CacheColdScope:   "new service and Cherri asset cache; OS filesystem cache is not evicted",
		},
		NumericLatencyBudget: "not specified by the immutable S03/E08 contracts",
	}
	for _, fixture := range []struct {
		name          string
		source, asset int
		kind          string
	}{
		{"small_source", 1068, 0, ""}, {"medium_source", 19284, 0, ""}, {"large_source", 169812, 0, ""},
		{"inline_text_4mib", 4194330, 0, ""}, {"local_text_8mib", 1093, 8 << 20, "text"}, {"local_binary_8mib", 1093, 8 << 20, "binary"},
	} {
		phases := []string{"cold_service_cache", "warm_analyze", "unrelated_edit"}
		if fixture.asset != 0 {
			phases = append(phases, "asset_edit")
		}
		for _, phase := range phases {
			m := editorPerformanceMeasurement{Fixture: fixture.name, Phase: phase,
				SourceBytes: fixture.source, SourceSHA256: editorSHA([]byte(fixture.name)),
				AssetBytes: fixture.asset, AssetKind: fixture.kind}
			for index := 0; index < 20; index++ {
				n, digest := fixture.source, m.SourceSHA256
				if phase == "unrelated_edit" {
					n += 20 + index
					digest = editorSHA([]byte(fmt.Sprintf("%s edit%d", fixture.name, index)))
				}
				m.InputBytes = append(m.InputBytes, n)
				m.InputSHA256 = append(m.InputSHA256, digest)
				ticks := int64(10_000 + 1000*index)
				m.SamplesTicks = append(m.SamplesTicks, ticks)
				m.SamplesUS = append(m.SamplesUS, float64(ticks)/1000)
				if fixture.asset != 0 {
					digest = editorSHA([]byte(fixture.name + "-asset"))
					if phase == "asset_edit" {
						digest = editorSHA([]byte(fmt.Sprintf("%s asset%d", fixture.name, index)))
					}
					m.AssetSHA256 = append(m.AssetSHA256, digest)
				}
			}
			m.P50US, m.P95US = m.SamplesUS[9], m.SamplesUS[18]
			if fixture.asset != 0 {
				switch phase {
				case "cold_service_cache":
					m.Reads, m.Encodes, m.Hits = 20, 20, 20
				case "warm_analyze":
					m.Hits = 20
				case "unrelated_edit":
					m.Hits = 40
				case "asset_edit":
					m.Reads, m.Encodes = 20, 20
				}
				m.ReadBytes = m.Reads * uint64(fixture.asset)
			}
			report.Measurements = append(report.Measurements, m)
		}
	}
	return report
}

func editorModelStream(t *testing.T, id string, report editorPerformanceReport) []byte {
	t.Helper()
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var stream bytes.Buffer
	emit := func(action, test, output string) {
		event := map[string]any{"Action": action, "Package": "github.com/electrikmilk/cherri"}
		if test != "" {
			event["Test"] = test
		}
		if output != "" {
			event["Output"] = output
		}
		line, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		stream.Write(line)
		stream.WriteByte('\n')
	}
	emit("start", "", "")
	for _, test := range editorProductionTests[id] {
		emit("run", test, "")
		if test == editorBenchmarkTest {
			for _, fixture := range editorBenchmarkFixtures {
				emit("run", test+"/"+fixture, "")
				emit("pass", test+"/"+fixture, "")
			}
			emit("output", test, editorBenchmarkMarker+string(payload)+"\n")
		}
		for _, child := range assetAuthorizationRequiredSubtests(editorProductionTests[id]) {
			if strings.HasPrefix(child, test+"/") {
				emit("run", child, "")
				emit("pass", child, "")
			}
		}
		emit("pass", test, "")
	}
	emit("pass", "", "")
	return stream.Bytes()
}

func editorModelRecord(t *testing.T, id string, report editorPerformanceReport) EvidenceRecord {
	t.Helper()
	revision, _, sourceErr := editorSourceState(resolveRepoRoot())
	if sourceErr != nil {
		t.Fatal(sourceErr)
	}
	report.Environment.SourceCommit = revision
	sources, sourceErr := editorGitSourceFiles(resolveRepoRoot(), revision, editorProductionPathsForID(id))
	if sourceErr != nil {
		t.Fatal(sourceErr)
	}
	directory := t.TempDir()
	tests := editorProductionTests[id]
	raw := editorModelStream(t, id, report)
	errorOutput := []byte("synthetic dependency progress on stderr; not a measured run\n")
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, '\n')
	exit, timeout := 0, false
	receipt := editorProcessReceipt{
		FormatVersion: 1, Requirement: id, SourceCommit: report.Environment.SourceCommit,
		SourceCleanBefore: true, SourceCleanAfter: true, Command: productionTestCommand(tests),
		RequiredTests: append([]string(nil), tests...), DiscoveredTests: append([]string(nil), tests...),
		StartedUTC: "2026-11-19T01:00:00Z", FinishedUTC: "2026-11-19T01:00:01Z", ElapsedSeconds: 1,
		ExitCode: &exit, TimedOut: &timeout, RawSHA256: editorSHA(raw), RawBytes: len(raw),
		BenchmarkSHA256: editorSHA(payload), BenchmarkBytes: len(payload),
		StderrSHA256: editorSHA(errorOutput), StderrBytes: len(errorOutput),
	}
	receipt.SourceFiles = sources
	process, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	record := EvidenceRecord{
		EvidenceID: "synthetic-editor-intake-model", Requirements: []string{id}, Tier: "benchmark",
		ImplementationSHA: report.Environment.SourceCommit, TestID: "github.com/electrikmilk/cherri:" + editorBenchmarkTest,
		Command: productionTestCommand(tests), Result: "passed", ExitCode: 0,
		Assertions: map[string]string{
			"required_production_tests": strings.Join(tests, "\n"),
			"scope":                     editorProductionScope(id),
		},
	}
	if id == "RP29" {
		record.Assertions["required_security_subtests"] = strings.Join(assetAuthorizationSecuritySubtests, "\n")
	}
	for _, file := range []struct {
		suffix string
		data   []byte
	}{{"-go.jsonl", raw}, {"-benchmark.json", payload}, {"-process.json", process}, {"-stderr.log", errorOutput}} {
		path := filepath.Join(directory, editorArtifactName(id, file.suffix))
		if err := os.WriteFile(path, file.data, 0600); err != nil {
			t.Fatal(err)
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(file.data)})
	}
	return record
}

func editorMutateModelFile(t *testing.T, record *EvidenceRecord, suffix string, mutate func([]byte) []byte) {
	t.Helper()
	for index := range record.Artifacts {
		artifact := &record.Artifacts[index]
		if !strings.HasSuffix(artifact.Path, suffix) {
			continue
		}
		raw, err := os.ReadFile(artifact.Path)
		if err != nil {
			t.Fatal(err)
		}
		raw = mutate(raw)
		if err := os.WriteFile(artifact.Path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		artifact.SHA256 = editorSHA(raw)
		return
	}
	t.Fatal("mutation target missing")
}

func TestEditorBenchmarkIntakeRejectsProxyAndFabricatedMetrics(t *testing.T) {
	good := editorIntakeModel()
	if err := validateEditorBenchmark(good, good.Environment.SourceCommit); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*editorPerformanceReport){
		"missing_phase":         func(r *editorPerformanceReport) { r.Measurements = r.Measurements[:19] },
		"duplicate_phase":       func(r *editorPerformanceReport) { r.Measurements[1] = r.Measurements[0] },
		"tiny_large_proxy":      func(r *editorPerformanceReport) { r.Measurements[6].SourceBytes = 64 },
		"dirty_measured_source": func(r *editorPerformanceReport) { r.Environment.SourceTreeClean = false },
		"no_hardware":           func(r *editorPerformanceReport) { r.Environment.CPU = "" },
		"stale_source":          func(r *editorPerformanceReport) { r.Environment.SourceCommit = strings.Repeat("b", 40) },
		"coarse_windows_clock":  func(r *editorPerformanceReport) { r.Environment.OS = "windows" },
		"no_clock_frequency":    func(r *editorPerformanceReport) { r.Environment.ClockFrequencyHz = 0 },
		"wrong_tick_conversion": func(r *editorPerformanceReport) { r.Measurements[0].SamplesTicks[0]++ },
		"invented_percentile":   func(r *editorPerformanceReport) { r.Measurements[0].P95US++ },
		"missing_raw_sample":    func(r *editorPerformanceReport) { r.Measurements[0].SamplesUS = r.Measurements[0].SamplesUS[:19] },
		"repeated_tiny_edit":    func(r *editorPerformanceReport) { r.Measurements[2].InputSHA256[1] = r.Measurements[2].InputSHA256[0] },
		"unchanged_asset_edit": func(r *editorPerformanceReport) {
			r.Measurements[15].AssetSHA256[1] = r.Measurements[15].AssetSHA256[0]
		},
		"warm_heavy_read":         func(r *editorPerformanceReport) { r.Measurements[13].Reads = 1 },
		"native_latency_claim":    func(r *editorPerformanceReport) { r.NativeUIExecution = true },
		"invented_latency_budget": func(r *editorPerformanceReport) { r.NumericLatencyBudget = "under500ms" },
		"base_fixture_drift":      func(r *editorPerformanceReport) { r.Measurements[1].SourceSHA256 = editorSHA([]byte("different")) },
	}
	encoded, _ := json.Marshal(good)
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var changed editorPerformanceReport
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			mutate(&changed)
			if err := validateEditorBenchmark(changed, good.Environment.SourceCommit); err == nil {
				t.Fatal("damaged benchmark accepted")
			}
		})
	}
}

func TestEditorEvidenceIntakeRejectsSummaryOnlyAndDetachedExecution(t *testing.T) {
	for _, id := range []string{"S03", "E08", "RP29"} {
		t.Run(id, func(t *testing.T) {
			good := editorModelRecord(t, id, editorIntakeModel())
			if err := validateEditorPerformanceRecord(id, good); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{
				"missing_artifact", "duplicate_artifact", "changed_bytes", "partial_command", "wrong_tier", "other_requirement",
				"missing_exit", "timeout", "dirty_source", "wrong_source", "missing_source_file", "missing_discovery", "false_elapsed",
				"summary_only", "missing_benchmark_child", "skipped_child", "duplicate_payload", "foreign_package",
				"report_changed_with_new_hash", "duplicate_process_key", "duplicate_report_key",
			} {
				t.Run(fault, func(t *testing.T) {
					record := editorModelRecord(t, id, editorIntakeModel())
					switch fault {
					case "missing_artifact":
						record.Artifacts = record.Artifacts[:2]
					case "duplicate_artifact":
						record.Artifacts[1] = record.Artifacts[0]
					case "changed_bytes":
						if err := os.WriteFile(record.Artifacts[0].Path, []byte("changed"), 0600); err != nil {
							t.Fatal(err)
						}
					case "partial_command":
						record.Command = productionTestCommand(editorProductionTests[id][:0])
					case "wrong_tier":
						record.Tier = "ios-ui"
					case "other_requirement":
						record.Requirements = []string{"S03", "E08"}
					case "report_changed_with_new_hash":
						editorMutateModelFile(t, &record, "-benchmark.json", func(raw []byte) []byte {
							return bytes.Replace(raw, []byte("synthetic intake-control model"), []byte("different measured host"), 1)
						})
					case "duplicate_process_key":
						editorMutateModelFile(t, &record, "-process.json", func(raw []byte) []byte { return append([]byte("{\"exit_code\":7,"), raw[1:]...) })
					case "duplicate_report_key":
						editorMutateModelFile(t, &record, "-benchmark.json", func(raw []byte) []byte { return append([]byte("{\"native_ui_execution\":true,"), raw[1:]...) })
					case "missing_exit", "timeout", "dirty_source", "wrong_source", "missing_source_file", "missing_discovery", "false_elapsed":
						editorMutateModelFile(t, &record, "-process.json", func(raw []byte) []byte {
							var receipt editorProcessReceipt
							if err := json.Unmarshal(raw, &receipt); err != nil {
								t.Fatal(err)
							}
							switch fault {
							case "missing_exit":
								receipt.ExitCode = nil
							case "timeout":
								value := true
								receipt.TimedOut = &value
							case "dirty_source":
								receipt.SourceCleanAfter = false
							case "wrong_source":
								receipt.SourceCommit = strings.Repeat("b", 40)
							case "missing_source_file":
								receipt.SourceFiles = receipt.SourceFiles[1:]
							case "missing_discovery":
								receipt.DiscoveredTests = nil
							case "false_elapsed":
								receipt.ElapsedSeconds = -1
							}
							result, _ := json.Marshal(receipt)
							return result
						})
					default:
						editorMutateModelFile(t, &record, "-go.jsonl", func(raw []byte) []byte {
							var output bytes.Buffer
							for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
								var event map[string]any
								if err := json.Unmarshal(line, &event); err != nil {
									t.Fatal(err)
								}
								test, _ := event["Test"].(string)
								action, _ := event["Action"].(string)
								if fault == "summary_only" && action == "run" {
									continue
								}
								if fault == "missing_benchmark_child" && test == editorBenchmarkTest+"/large_source" {
									continue
								}
								if fault == "skipped_child" && test == editorBenchmarkTest+"/large_source" && action == "pass" {
									event["Action"] = "skip"
								}
								if fault == "foreign_package" {
									event["Package"] = "example.invalid/foreign"
								}
								encoded, _ := json.Marshal(event)
								output.Write(encoded)
								output.WriteByte('\n')
								if fault == "duplicate_payload" && action == "output" && test == editorBenchmarkTest {
									output.Write(encoded)
									output.WriteByte('\n')
								}
							}
							return output.Bytes()
						})
						// Reconcile the raw digest deliberately: rejection must come from
						// actual discovery/stream semantics, not just a stale hash.
						editorMutateModelFile(t, &record, "-process.json", func(raw []byte) []byte {
							var receipt editorProcessReceipt
							_ = json.Unmarshal(raw, &receipt)
							stream, err := os.ReadFile(record.Artifacts[0].Path)
							if err != nil {
								t.Fatal(err)
							}
							receipt.RawSHA256, receipt.RawBytes = editorSHA(stream), len(stream)
							result, _ := json.Marshal(receipt)
							return result
						})
					}
					if err := validateEditorPerformanceRecord(id, record); err == nil {
						t.Fatal("damaged evidence minted a pass")
					}
				})
			}
		})
	}
}

func TestEditorProducerProcessTreeCancellation(t *testing.T) {
	const modeKey = "CHERRI_EDITOR_OWNED_PROCESS_FIXTURE"
	mode := os.Getenv(modeKey)
	path := os.Getenv("CHERRI_EDITOR_OWNED_PROCESS_OUTPUT")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if mode == "child" {
		for {
			if err := os.WriteFile(path, []byte(strconv.FormatInt(time.Now().UnixNano(), 10)), 0600); err != nil {
				os.Exit(2)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	if mode == "parent" {
		child := exec.Command(executable, "-test.run=^TestEditorProducerProcessTreeCancellation$", "-test.timeout=10s")
		child.Env = append(os.Environ(), modeKey+"=child")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path+".pid", []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
			t.Fatal(err)
		}
		_ = child.Wait()
		return
	}
	path = filepath.Join(t.TempDir(), "owned-child-writes.txt")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestEditorProducerProcessTreeCancellation$", "-test.timeout=10s")
	command.Env = append(os.Environ(), modeKey+"=parent", "CHERRI_EDITOR_OWNED_PROCESS_OUTPUT="+path)
	configureEditorProductionProcess(command)
	started := time.Now()
	_, err = command.CombinedOutput()
	if err == nil || ctx.Err() == nil || time.Since(started) > 8*time.Second {
		t.Fatalf("owned timeout was not bounded: %v", err)
	}
	first, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal("owned child never executed", readErr)
	}
	time.Sleep(250 * time.Millisecond)
	second, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(first, second) {
		// The PID was written by our exact test child in this private tempdir.
		pidBytes, _ := os.ReadFile(path + ".pid")
		pid, _ := strconv.Atoi(string(pidBytes))
		if pid > 0 {
			if runtime.GOOS == "windows" {
				_ = exec.Command("taskkill.exe", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
			} else if process, e := os.FindProcess(pid); e == nil {
				_ = process.Kill()
			}
		}
		t.Fatal("timed-out owned descendant continued writing")
	}
}

func TestP15EditorProducerActualContracts(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, contract := range all.Acceptance.Cases {
		if contract.ID == "S03" || contract.ID == "E08" {
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, contract)
		}
	}
	selected.Acceptance.CaseCount = len(selected.Acceptance.Cases)
	if selected.Acceptance.CaseCount != 2 {
		t.Fatal("immutable editor contracts missing")
	}
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_EDITOR_EVIDENCE_DIR")
	if runner.artifactDir == "" {
		runner.artifactDir = t.TempDir()
	}
	for _, id := range []string{"S03", "E08"} {
		runner.runEditorPerformanceRequirement(id)
	}
	for _, contract := range selected.Acceptance.Cases {
		result := runner.results[contract.ID]
		if !result.Passed || result.Status != "PASSED" || result.Requirement != contract.Requirement || result.TestLevel != contract.MinimumTestLevel {
			t.Fatalf("incomplete actual %s proof: %+v", contract.ID, result)
		}
	}
	if len(runner.records) != 3 || len(runner.observations) != 0 {
		t.Fatal("wrong editor evidence scope")
	}
	for _, record := range runner.records {
		if err := validateEditorPerformanceRecord(record.Requirements[0], record); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 2 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("actual bounded editor evidence rejected: %+v %v", report, err)
	}
	encoded, _ := json.Marshal(manifest)
	for _, fault := range []string{"missing_service", "wrong_producer", "stale_source", "invented_native", "missing_raw_artifact"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "missing_service":
				changed.Records = changed.Records[1:]
			case "wrong_producer":
				changed.Records[0].TestID = "tools/language-acceptance:runText"
			case "stale_source":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "invented_native":
				changed.Records[0].Tier = "ios-runtime"
			case "missing_raw_artifact":
				changed.Records[0].Artifacts = changed.Records[0].Artifacts[1:]
			}
			rejected, err := ValidateEvidenceManifest(&changed, selected, "local")
			if p15VerificationResult(t, rejected, "S03").Passed || rejected.ClosureComplete {
				t.Fatal("damaged actual proof minted a pass")
			}
			if fault != "missing_service" && err == nil {
				t.Fatal("invalid record was not rejected")
			}
		})
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("changed formal179 accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "S03" || row.ID == "E08" {
			var tests = editorProductionTests[row.ID]
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, tests) || !reflect.DeepEqual(row.ProductionPath, editorProductionPaths) {
				t.Fatalf("lost exact editor scenario: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatal("unexecuted requirement inherited editor proof")
		}
	}
	receipt, _ := json.Marshal(map[string]any{"source_sha": runner.implSHA, "report": report, "records": runner.records, "artifact_paths": editorSortedArtifactPaths(runner.records), "scope": "Actual local service/cache and measured desktop compiler service latency only; no Apple UI latency, race tier, or release closure."})
	t.Log("P15_EDITOR_PRODUCTION_RECEIPT:" + string(receipt))
}
