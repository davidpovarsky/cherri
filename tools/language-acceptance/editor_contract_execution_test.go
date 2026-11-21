package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestEditorContractInventoryMatchesImmutableRequirements(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if editorContracts.Schema != 1 || !reflect.DeepEqual(editorContractSortedIDs(), []string{"E01", "E02", "E03", "E05", "E06"}) {
		t.Fatal("changed editor requirement set")
	}
	counts := map[string][]int{"E01": {73}, "E02": {24}, "E03": {48}, "E05": {20}, "E06": {33, 35}}
	for id, spec := range editorContracts.Scenarios {
		found := false
		for _, contract := range all.Acceptance.Cases {
			if contract.ID == id {
				found = true
				if spec.Title != contract.Requirement || spec.Level != contract.MinimumTestLevel || contract.Group != "editor" {
					t.Fatalf("detached immutable %s contract: %+v", id, contract)
				}
			}
		}
		if !found || len(spec.Steps) != len(counts[id]) {
			t.Fatalf("missing %s inventory", id)
		}
		for i, step := range spec.Steps {
			if len(step.Events) != counts[id][i] || !sort.StringsAreSorted(step.Events) {
				t.Fatalf("changed %s:%s child inventory", id, step.Name)
			}
			var parents []string
			seen := map[string]bool{}
			for _, event := range step.Events {
				if seen[event] || !strings.HasPrefix(event, "TestP15"+id) {
					t.Fatal("duplicate or foreign test")
				}
				seen[event] = true
				if !strings.Contains(event, "/") {
					parents = append(parents, event)
				}
			}
			if !reflect.DeepEqual(parents, step.Tests) {
				t.Fatal("parent selection differs from fixed complete events")
			}
		}
	}
	seen := map[string]bool{}
	for _, path := range editorContracts.SourcePaths {
		if seen[path] {
			t.Fatal("duplicate source")
		}
		seen[path] = true
		if _, err := os.Stat(resolveRepoPath(path)); err != nil {
			t.Fatal(err)
		}
	}
}

// A deliberately synthetic event stream for parser fault controls only.
// It cannot create any evidence record or acceptance result.
func editorContractModelEvents(t *testing.T, step editorContractStep) []map[string]any {
	t.Helper()
	var events []map[string]any
	emit := func(action, name string) {
		row := map[string]any{"Time": "2026-11-21T01:00:00Z", "Action": action, "Package": step.PackageID}
		if name != "" {
			row["Test"] = name
		}
		if action == "pass" {
			row["Elapsed"] = 0.0
		}
		events = append(events, row)
	}
	emit("start", "")
	var walk func(string)
	walk = func(name string) {
		emit("run", name)
		for _, child := range step.Events {
			if strings.HasPrefix(child, name+"/") && !strings.Contains(strings.TrimPrefix(child, name+"/"), "/") {
				walk(child)
			}
		}
		emit("pass", name)
	}
	for _, parent := range step.Tests {
		walk(parent)
	}
	emit("pass", "")
	return events
}
func editorContractEncodeEvents(t *testing.T, events []map[string]any) []byte {
	t.Helper()
	var raw bytes.Buffer
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		raw.Write(data)
		raw.WriteByte('\n')
	}
	return raw.Bytes()
}
func TestEditorContractStrictEventIntake(t *testing.T) {
	for id, spec := range editorContracts.Scenarios {
		for _, step := range spec.Steps {
			t.Run(id+"/"+step.Name, func(t *testing.T) {
				good := editorContractModelEvents(t, step)
				if err := parseStrictNamedProductionOutputInPackage(step.PackageID, step.Events, 4*time.Minute, editorContractEncodeEvents(t, good)); err != nil {
					t.Fatal(err)
				}
				for _, fault := range []string{"missing_child", "skip", "fail", "foreign_package", "unknown_child", "duplicate_event", "missing_time", "duplicate_json_key", "invalid_utf8", "package_before_children"} {
					t.Run(fault, func(t *testing.T) {
						rows := editorContractModelEvents(t, step)
						target := 0
						for i, row := range rows {
							if row["Action"] == "run" && strings.Contains(row["Test"].(string), "/") {
								target = i
								break
							}
						}
						switch fault {
						case "missing_child":
							name := rows[target]["Test"]
							var kept []map[string]any
							for _, row := range rows {
								if row["Test"] != name {
									kept = append(kept, row)
								}
							}
							rows = kept
						case "skip", "fail":
							rows[target]["Action"] = fault
						case "foreign_package":
							rows[target]["Package"] = "foreign/package"
						case "unknown_child":
							rows[target]["Test"] = "TestForeign/child"
						case "duplicate_event":
							rows = append(rows[:target+1], append([]map[string]any{rows[target]}, rows[target+1:]...)...)
						case "missing_time":
							delete(rows[target], "Time")
						case "package_before_children":
							rows[1] = map[string]any{"Time": "2026-11-21T01:00:00Z", "Action": "pass", "Package": step.PackageID, "Elapsed": 0.0}
						}
						raw := editorContractEncodeEvents(t, rows)
						if fault == "duplicate_json_key" {
							raw = bytes.Replace(raw, []byte(`"Action":"run"`), []byte(`"Action":"run","Action":"pass"`), 1)
						}
						if fault == "invalid_utf8" {
							raw = append(raw, 0xff)
						}
						if err := parseStrictNamedProductionOutputInPackage(step.PackageID, step.Events, 4*time.Minute, raw); err == nil {
							t.Fatalf("accepted %s", fault)
						}
					})
				}
			})
		}
	}
}

func editorContractCopyRecord(t *testing.T, source EvidenceRecord) EvidenceRecord {
	t.Helper()
	raw, _ := json.Marshal(source)
	var copy EvidenceRecord
	if err := json.Unmarshal(raw, &copy); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	for i := range copy.Artifacts {
		item := &copy.Artifacts[i]
		raw, err := os.ReadFile(item.Path)
		if err != nil {
			t.Fatal(err)
		}
		parent := directory
		if filepath.Base(filepath.Dir(item.Path)) == "swift" {
			parent = filepath.Join(parent, "swift")
		}
		if err = os.MkdirAll(parent, 0700); err != nil {
			t.Fatal(err)
		}
		item.Path = filepath.Join(parent, filepath.Base(item.Path))
		if err = os.WriteFile(item.Path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return copy
}
func editorContractMutateArtifact(t *testing.T, record *EvidenceRecord, name string, mutate func([]byte) []byte) {
	t.Helper()
	for i := range record.Artifacts {
		artifact := &record.Artifacts[i]
		if filepath.Base(artifact.Path) != name {
			continue
		}
		raw, err := os.ReadFile(artifact.Path)
		if err != nil {
			t.Fatal(err)
		}
		raw = mutate(raw)
		if err = os.WriteFile(artifact.Path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		artifact.SHA256 = editorSHA(raw)
		return
	}
	t.Fatalf("missing mutation target %s", name)
}

func TestEditorContractActualProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, contract := range all.Acceptance.Cases {
		if _, ok := editorContracts.Scenarios[contract.ID]; ok {
			selected.Cases = append(selected.Cases, contract)
		}
	}
	selected.CaseCount = len(selected.Cases)
	runner := NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_EDITOR_CONTRACT_EVIDENCE_OUTPUT")
	for _, id := range editorContractSortedIDs() {
		t.Run(id, func(t *testing.T) {
			before := len(runner.records)
			runner.runEditorContract(id)
			result := runner.results[id]
			if editorContracts.Scenarios[id].SwiftProbe != "" {
				if _, missing := exec.LookPath("swiftc"); missing != nil && os.Getenv("CHERRI_EDITOR_REQUIRE_SWIFT") != "1" {
					if result.Status != "NOT_RUN" || len(runner.records) != before {
						t.Fatal("unavailable Swift produced a local pass")
					}
					t.Log(result.Detail)
					return
				}
			}
			if result.Status != "PENDING_EXTERNAL" || result.Passed || len(runner.records) != before+1 {
				t.Fatalf("actual editor scenario incomplete: %+v", result)
			}
			record := runner.records[len(runner.records)-1]
			if err := validateEditorContractRecord(id, record); err != nil {
				t.Fatal(err)
			}
			relocated := editorContractCopyRecord(t, record)
			if err := validateEditorContractRecord(id, relocated); err != nil {
				t.Fatalf("unchanged relocated evidence rejected: %v", err)
			}
			for _, fault := range []string{"wrong_source", "wrong_tier", "wrong_command", "missing_artifact", "dirty_source", "timed_out", "forged_source_digest", "missing_child"} {
				t.Run(fault, func(t *testing.T) {
					broken := editorContractCopyRecord(t, record)
					spec := editorContracts.Scenarios[id]
					prefix := editorContractPrefix(id, spec.Steps[0].Name)
					switch fault {
					case "wrong_source":
						broken.ImplementationSHA = strings.Repeat("a", 40)
					case "wrong_tier":
						broken.Tier = "ios"
					case "wrong_command":
						broken.Command = broken.Command[:len(broken.Command)-1]
					case "missing_artifact":
						broken.Artifacts = broken.Artifacts[1:]
					case "missing_child":
						editorContractMutateArtifact(t, &broken, prefix+"-go.jsonl", func(raw []byte) []byte {
							var kept bytes.Buffer
							for _, line := range bytes.Split(raw, []byte{'\n'}) {
								var event struct{ Test string }
								if json.Unmarshal(line, &event) != nil {
									continue
								}
								if event.Test != spec.Steps[0].Events[1] {
									kept.Write(line)
									kept.WriteByte('\n')
								}
							}
							return kept.Bytes()
						})
						var changed []byte
						for _, a := range broken.Artifacts {
							if filepath.Base(a.Path) == prefix+"-go.jsonl" {
								changed, _ = os.ReadFile(a.Path)
							}
						}
						editorContractMutateArtifact(t, &broken, prefix+"-process.json", func(raw []byte) []byte {
							var receipt editorProcessReceipt
							json.Unmarshal(raw, &receipt)
							receipt.RawSHA256 = editorSHA(changed)
							receipt.RawBytes = len(changed)
							out, _ := json.Marshal(receipt)
							return out
						})
					default:
						editorContractMutateArtifact(t, &broken, prefix+"-process.json", func(raw []byte) []byte {
							var receipt editorProcessReceipt
							json.Unmarshal(raw, &receipt)
							switch fault {
							case "dirty_source":
								receipt.SourceCleanBefore = false
							case "timed_out":
								value := true
								receipt.TimedOut = &value
							case "forged_source_digest":
								receipt.SourceFiles[0].SHA256 = strings.Repeat("b", 64)
							}
							out, _ := json.Marshal(receipt)
							return out
						})
					}
					if err := validateEditorContractRecord(id, broken); err == nil {
						t.Fatalf("accepted %s", fault)
					}
				})
			}
			if editorContracts.Scenarios[id].SwiftProbe != "" {
				for _, fault := range []string{"compiled_body_changed", "missing_control", "swift_timeout", "native_scope_claim"} {
					t.Run(fault, func(t *testing.T) {
						broken := editorContractCopyRecord(t, record)
						kind := editorContracts.Scenarios[id].SwiftProbe
						switch fault {
						case "compiled_body_changed":
							name := "position-generated.swift"
							if kind == "publication" {
								name = "editor-publication-generated.swift"
							}
							editorContractMutateArtifact(t, &broken, name, func(raw []byte) []byte { return append(raw, []byte("\n// changed")...) })
						default:
							editorContractMutateArtifact(t, &broken, "receipt.json", func(raw []byte) []byte {
								var receipt map[string]any
								json.Unmarshal(raw, &receipt)
								switch fault {
								case "native_scope_claim":
									receipt["uiExecuted"] = true
								case "swift_timeout":
									receipt["compile"].(map[string]any)["timeout_seconds"] = 900.0
								case "missing_control":
									key := "rows"
									if kind == "publication" {
										key = "controls"
									}
									receipt[key] = receipt[key].([]any)[1:]
								}
								out, _ := json.Marshal(receipt)
								return out
							})
						}
						if err := validateEditorContractRecord(id, broken); err == nil {
							t.Fatalf("accepted Swift %s", fault)
						}
					})
				}
			}
		})
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 5 || report.ClosureComplete {
		t.Fatalf("editor evidence escaped local scope: %+v %v", report, err)
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err = writeExecutionAudit(runner, all, auditPath); err != nil {
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
	if err = json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("changed immutable 179 accounting")
	}
	for _, row := range audit.Rows {
		if row.Status == "PASS" {
			t.Fatalf("local editor evidence granted full acceptance: %s", row.ID)
		}
	}
	proof, _ := json.MarshalIndent(map[string]any{"source": runner.implSHA, "report": report, "records": runner.records, "results": runner.results}, "", "  ")
	if runner.artifactDir != "" {
		if err = os.WriteFile(filepath.Join(runner.artifactDir, "editor-contract-evidence.json"), append(proof, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Actual source %s: %d editor local records; full requirements remain pending their required external UI evidence", runner.implSHA, len(runner.records))
}
