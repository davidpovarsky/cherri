package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func structuredValueSelectedContracts(t *testing.T) (*ContractsSet, *ContractsSet) {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, c := range all.Acceptance.Cases {
		if s, ok := structuredValueScenarios[c.ID]; ok {
			if s.Group != c.Group || s.Title != c.Requirement || !reflect.DeepEqual(parseRequiredTiers(c.MinimumTestLevel), structuredValueTiers) {
				t.Fatal("original acceptance clause changed", c.ID)
			}
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, c)
		}
	}
	for _, c := range all.Repair.Cases {
		if s, ok := structuredValueScenarios[c.ID]; ok {
			if s.Group != "repair" || s.Title != c.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(c.RequiredTestLevels, "+")), structuredValueTiers) {
				t.Fatal("original repair clause changed")
			}
			selected.Repair.Cases = append(selected.Repair.Cases, c)
		}
	}
	selected.Acceptance.CaseCount = len(selected.Acceptance.Cases)
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	if selected.Acceptance.CaseCount != 3 || selected.Repair.CaseCount != 1 {
		t.Fatal("missing independent original four clauses")
	}
	return all, selected
}
func TestStructuredValueCohortInventory(t *testing.T) {
	all, _ := structuredValueSelectedContracts(t)
	if err := validateStructuredValueInventory(); err != nil {
		t.Fatal(err)
	}
	mapping, err := BuildRequirementsEvidenceMap(all)
	if err != nil {
		t.Fatal(err)
	}
	for id, s := range structuredValueScenarios {
		row := mapping.Mappings[id]
		if !reflect.DeepEqual(row.TestIDs, structuredValueTestIDs(id)) || !reflect.DeepEqual(row.RequiredTiers, structuredValueTiers) ||
			len(row.RequiredAssertions) != 0 || len(row.RequiredUIAssertions) != 0 {
			t.Fatal("related/historical producer acquired complete clause", id)
		}
		for _, tier := range []string{"unit", "native-structure", "ios-runtime", "ios-ui", "ios-build"} {
			t.Run(id+"/partial_"+tier, func(t *testing.T) {
				if validateStructuredValueRecord(id, EvidenceRecord{Requirements: []string{id}, Tier: tier, TestID: s.producer(tier), Result: "passed"}) == nil {
					t.Fatal("labels without original cohort artifacts accepted")
				}
			})
		}
		r := NewRunner()
		if !r.runStructuredValueRequirement(id, s.Group, s.Title, []string{"unit"}) || r.results[id].Status != "FAILED" || r.structuredValues != nil {
			t.Fatal("weaker tiers launched or accepted")
		}
	}
	r := NewRunner()
	if r.runStructuredValueRequirement("RP23", "repair", "related clause", []string{"unit", "native"}) || r.structuredValues != nil {
		t.Fatal("unrelated clause inherited cohort")
	}
	if err := validateCombinedValueBlockRecord("F04", EvidenceRecord{Requirements: []string{"F04"}, Tier: "unit", TestID: combinedValueBlockScenarios["F04"].producer("unit"), Result: "passed"}); err == nil {
		t.Fatal("historical 226-event subset granted complete F04")
	}
	for _, old := range typeProductionTests["T05"] {
		if !partialTypeProducer("T05", "github.com/electrikmilk/cherri:"+old) {
			t.Fatal("old partial T05 protection removed")
		}
	}
}
func TestStructuredValueSourceBudgetPreservesDefault(t *testing.T) {
	line := func(size int64) string { return strings.Repeat("a", 40) + " blob " + fmt.Sprint(size) + "\n" }
	valid := []byte(strings.Repeat(line(10), 256))
	if err := localGateSourceBudgetBounded(valid, 256, 256); err != nil {
		t.Fatal("explicit full cohort bound rejected", err)
	}
	if err := localGateSourceBudget(valid, 256); err == nil {
		t.Fatal("historical default128 expanded")
	}
	for _, tc := range []struct {
		name         string
		raw          []byte
		count, limit int
	}{
		{"default129", []byte(strings.Repeat(line(10), 129)), 129, 128},
		{"cohort257", []byte(strings.Repeat(line(10), 257)), 257, 256},
		{"unknown_limit", valid, 256, 512},
		{"missing_object", valid[:len(valid)-len(line(10))], 256, 256},
		{"oversized_object", []byte(line((4 << 20) + 1)), 1, 256},
		{"aggregate_limit", []byte(strings.Repeat(line(4<<20), 4) + strings.Repeat(line(0), 252)), 256, 256},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := localGateSourceBudgetBounded(tc.raw, tc.count, tc.limit); err == nil {
				t.Fatal("source bound weakened")
			}
		})
	}
}
func structuredValueMutateEvents(t *testing.T, record *EvidenceRecord, fault, target string) {
	t.Helper()
	prefix := structuredValueCohortPrefix
	var changed []byte
	editorContractMutateArtifact(t, record, prefix+"-go.jsonl", func(raw []byte) []byte {
		switch fault {
		case "interior_blank_line":
			at := bytes.IndexByte(raw, '\n')
			if at < 0 {
				t.Fatal("actual Go event stream has no newline")
			}
			changed = append(append(append([]byte(nil), raw[:at+1]...), '\n'), raw[at+1:]...)
			return changed
		case "unterminated_stream":
			if len(raw) == 0 || raw[len(raw)-1] != '\n' {
				t.Fatal("actual Go event stream is already unterminated")
			}
			changed = append([]byte(nil), raw[:len(raw)-1]...)
			return changed
		case "second_trailing_newline":
			changed = append(append([]byte(nil), raw...), '\n')
			return changed
		}
		var b bytes.Buffer
		mutated := false
		for _, line := range bytes.Split(raw, []byte{'\n'}) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var event map[string]any
			if err := json.Unmarshal(line, &event); err != nil {
				t.Fatal(err)
			}
			match := event["Test"] == target
			if fault == "missing_internal_package" {
				match = event["Package"] == structuredValuePackageIDs[1]
			}
			if match {
				switch fault {
				case "missing_parent", "missing_child", "missing_internal_package":
					mutated = true
					continue
				case "skipped_child":
					if event["Action"] == "pass" {
						event["Action"] = "skip"
						mutated = true
					}
				case "wrong_package":
					event["Package"] = "foreign/package"
					mutated = true
				case "duplicate_event":
					if event["Action"] == "pass" {
						b.Write(line)
						b.WriteByte('\n')
						mutated = true
					}
				}
			}
			encoded, _ := json.Marshal(event)
			b.Write(encoded)
			b.WriteByte('\n')
		}
		if !mutated {
			t.Fatal("causal event control did not reach its target", fault, target)
		}
		changed = b.Bytes()
		return changed
	})
	editorContractMutateArtifact(t, record, prefix+"-process.json", func(raw []byte) []byte {
		var receipt editorProcessReceipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		receipt.RawSHA256 = editorSHA(changed)
		receipt.RawBytes = len(changed)
		encoded, _ := json.Marshal(receipt)
		return encoded
	})
}
func TestStructuredValueActualProductionEvidence(t *testing.T) {
	all, selected := structuredValueSelectedContracts(t)
	r := NewRunner()
	r.artifactDir = os.Getenv("CHERRI_STRUCTURED_ACCEPTANCE_ARTIFACTS")
	for _, id := range []string{"T05", "L01", "F04", "RP20"} {
		s := structuredValueScenarios[id]
		if !r.runStructuredValueRequirement(id, s.Group, s.Title, []string{"unit", "native"}) {
			t.Fatal("missing producer", id)
		}
	}
	if len(r.records) != 8 || len(r.results) != 4 || len(r.observations) != 0 || r.structuredValues == nil || r.structuredValues.Err != nil {
		t.Fatalf("incomplete actual cohort: results=%+v records=%d observation=%+v", r.results, len(r.records), r.structuredValues)
	}
	first := r.structuredValues
	for id, s := range structuredValueScenarios {
		result := r.results[id]
		if !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, structuredValueTiers) {
			t.Fatal("actual distinct clause missing", id, result)
		}
		r.runStructuredValueRequirement(id, s.Group, s.Title, []string{"unit", "native"})
	}
	if r.structuredValues != first || len(r.records) != 8 {
		t.Fatal("one explicit cohort repeated or minted duplicate records")
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: r.implSHA, Records: r.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report == nil || report.PassedCases != 4 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("cohort evidence invalid/expanded: %+v %v", report, err)
	}
	for _, record := range r.records {
		id := record.Requirements[0]
		if err := validateStructuredValueRecord(id, record); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(record.Artifacts, first.Artifacts) {
			t.Fatal("detached per-requirement original process")
		}
		if record.Tier != "unit" {
			continue
		}
		relocated := editorContractCopyRecord(t, record)
		if err := validateStructuredValueRecord(id, relocated); err != nil {
			t.Fatal("byte-identical relocation rejected", err)
		}
		for _, fault := range []string{"wrong_tier", "wrong_requirement", "cross_tier_producer", "partial_command", "missing_artifact", "wrong_scope", "wrong_cohort", "dirty_source", "wrong_source_digest", "missing_source_file", "timed_out", "nonzero_exit", "missing_parent", "missing_child", "skipped_child", "wrong_package", "duplicate_event", "missing_internal_package", "interior_blank_line", "unterminated_stream", "second_trailing_newline"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				broken := editorContractCopyRecord(t, record)
				switch fault {
				case "wrong_tier":
					broken.Tier = "ios-runtime"
				case "wrong_requirement":
					broken.Requirements = []string{"RP23"}
				case "cross_tier_producer":
					broken.TestID = structuredValueScenarios[id].producer("native-structure")
				case "partial_command":
					broken.Command = productionTestCommand([]string{structuredValueScenarios[id].Unit})
				case "missing_artifact":
					broken.Artifacts = broken.Artifacts[:2]
				case "wrong_scope":
					broken.Assertions["scope"] = "related partial checks"
				case "wrong_cohort":
					broken.Assertions["cohort"] = "F04-only"
				case "dirty_source", "wrong_source_digest", "missing_source_file", "timed_out", "nonzero_exit":
					editorContractMutateArtifact(t, &broken, structuredValueCohortPrefix+"-process.json", func(raw []byte) []byte {
						var receipt editorProcessReceipt
						if err := json.Unmarshal(raw, &receipt); err != nil {
							t.Fatal(err)
						}
						switch fault {
						case "dirty_source":
							receipt.SourceCleanAfter = false
						case "wrong_source_digest":
							receipt.SourceFiles[0].SHA256 = strings.Repeat("f", 64)
						case "missing_source_file":
							receipt.SourceFiles = receipt.SourceFiles[:len(receipt.SourceFiles)-1]
						case "timed_out":
							value := true
							receipt.TimedOut = &value
						case "nonzero_exit":
							value := 1
							receipt.ExitCode = &value
						}
						encoded, _ := json.Marshal(receipt)
						return encoded
					})
				default:
					target := structuredValueScenarios[id].Unit
					if fault != "missing_parent" {
						for _, name := range structuredValueInventory.Packages[structuredValuePackageIDs[0]] {
							if strings.HasPrefix(name, structuredValueScenarios[id].Native+"/") {
								target = name
								break
							}
						}
						if target == structuredValueScenarios[id].Unit {
							t.Fatal("dedicated child not inventoried")
						}
					}
					structuredValueMutateEvents(t, &broken, fault, target)
				}
				if err := validateStructuredValueRecord(id, broken); err == nil {
					t.Fatal("accepted forged/partial cohort", fault)
				}
			})
		}
	}
	for _, id := range []string{"T05", "L01", "F04", "RP20"} {
		t.Run(id+"/missing_native_record", func(t *testing.T) {
			broken := *manifest
			broken.Records = nil
			for _, record := range manifest.Records {
				if record.Requirements[0] != id || record.Tier != "native-structure" {
					broken.Records = append(broken.Records, record)
				}
			}
			invalid, e := ValidateEvidenceManifest(&broken, selected, "local")
			if e == nil && invalid != nil && invalid.PassedCases == 4 {
				t.Fatal("missing distinct tier granted full cohort")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(r, all, path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
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
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("cohort altered original global accounting")
	}
	for _, row := range audit.Rows {
		_, enrolled := structuredValueScenarios[row.ID]
		if enrolled {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, structuredValueInventory.Tests) {
				t.Fatal("actual cohort lost audit scope", row.ID)
			}
		} else if row.Status == "PASS" {
			t.Fatal("unexecuted requirement inherited success", row.ID)
		}
	}
	if r.artifactDir != "" {
		retained, err := stateEvidenceDirectory(r, "structured-values-intake-")
		if err != nil {
			t.Fatal(err)
		}
		for _, original := range []struct {
			name     string
			value    any
			original []byte
		}{
			{"evidence-manifest.original.json", manifest, nil},
			{"intake-report.original.json", report, nil},
			{"execution-audit.original.json", nil, raw},
		} {
			encoded := append([]byte(nil), original.original...)
			if original.original == nil {
				var err error
				encoded, err = json.MarshalIndent(original.value, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				encoded = append(encoded, '\n')
			}
			f, err := os.OpenFile(filepath.Join(retained, original.name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := f.Write(encoded)
			closeErr := f.Close()
			if writeErr != nil || closeErr != nil {
				t.Fatalf("retain original intake: write=%v close=%v", writeErr, closeErr)
			}
		}
		t.Logf("STRUCTURED_VALUE_COHORT_ORIGINAL_INTAKE path=%s", retained)
	}
	t.Logf("STRUCTURED_VALUE_COHORT_ACTUAL source=%s requirements=4 records=8 root_original=260 internal_original=70 named=%d closure_complete=false", r.implSHA, len(structuredValueInventory.Events))
}
