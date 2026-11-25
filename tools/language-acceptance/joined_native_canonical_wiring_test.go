package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These are synthetic intake controls. They never export an acceptance manifest,
// launch Xcode, or count as executed native requirement evidence.
func joinedNativeCanonicalContracts(t *testing.T, id string) *ContractsSet {
	t.Helper()
	return &ContractsSet{
		Acceptance: &ContractFile{}, Repair: &RepairContractFile{},
		Gates: &GatesContractFile{GateCount: 1, Gates: []RecoveryGate{joinedPublicContractForTest(t, id)}},
	}
}

func TestJoinedNativeCanonicalMapping(t *testing.T) {
	for _, id := range []string{"BRG25", "BRG27"} {
		t.Run(id, func(t *testing.T) {
			contracts := joinedNativeCanonicalContracts(t, id)
			mapping, err := BuildRequirementsEvidenceMap(contracts)
			if err != nil {
				t.Fatal(err)
			}
			row := mapping.Mappings[id]
			var native []string
			for _, name := range row.TestIDs {
				if strings.HasPrefix(name, "Cherri") {
					native = append(native, name)
				}
			}
			if !reflect.DeepEqual(native, joinedNativeEditorTestIDs(id)) {
				t.Fatalf("formal native inventory differs: %v", native)
			}
			if !reflect.DeepEqual(row.RequiredUIAssertions, joinedNativeEditorUIAssertions(id)) {
				t.Fatal("dedicated UI assertion inventory differs")
			}
			if !reflect.DeepEqual(row.RequiredTiers, joinedPublicScenarios[id].RequiredTiers) {
				t.Fatal("immutable requirement tiers changed")
			}
			for _, path := range joinedNativeSourcePaths {
				found := false
				for _, actual := range row.FixtureIdentities {
					found = found || actual == path
				}
				if !found {
					t.Fatal("dedicated native source absent from declared fixture inventory:", path)
				}
			}
		})
	}
}

func TestJoinedNativeCanonicalManifestBoundary(t *testing.T) {
	for _, test := range []string{joinedBRG25Bridge, joinedBRG25UI, joinedBRG27UI} {
		t.Run(test, func(t *testing.T) {
			model := newJoinedNativeIntakeModel(t, test)
			id := model.record.Requirements[0]
			if err := validateJoinedNativeEditorRecord(id, model.record, model.manifest); err != nil {
				t.Fatal("independent healthy control rejected:", err)
			}
			model.manifest.Records = []EvidenceRecord{model.record}
			report, err := ValidateEvidenceManifest(model.manifest, joinedNativeCanonicalContracts(t, id), "local")
			if report == nil || err == nil {
				t.Fatal("native-only metadata must retain missing local obligations")
			}
			want := "requirement " + id + " has unsatisfied local tier"
			if !reflect.DeepEqual(report.ValidationErrors, []string{want}) {
				t.Fatalf("canonical route did not admit exactly the independently validated native record: %v", report.ValidationErrors)
			}
			result := p15VerificationResult(t, report, id)
			if result.Passed || report.ClosureComplete || !reflect.DeepEqual(result.ExecutedLevels, []string{model.record.Tier}) {
				t.Fatalf("native tier inherited local or full acceptance: %+v", result)
			}
		})
	}
}

func TestJoinedNativeCanonicalOriginalCorruptions(t *testing.T) {
	for _, test := range []string{joinedBRG25Bridge, joinedBRG25UI, joinedBRG27UI} {
		for _, fault := range []string{"stage", "proof", "producer"} {
			t.Run(test+"/"+fault, func(t *testing.T) {
				model := newJoinedNativeIntakeModel(t, test)
				id := model.record.Requirements[0]
				if err := validateJoinedNativeEditorRecord(id, model.record, model.manifest); err != nil {
					t.Fatal("causal base rejected:", err)
				}
				want := ""
				switch fault {
				case "stage":
					model.changeLog(strings.Replace(string(model.raw), "\"sequence\":0", "\"sequence\":77", 1))
					want = "JOINED_STAGE"
				case "proof":
					model.proof["noCrossTierInheritance"] = false
					want = "JOINED_PROOF"
				case "producer":
					model.record.TestID += "/borrowed"
					want = "JOINED_RECORD"
				}
				model.seal(t)
				model.manifest.Records = []EvidenceRecord{model.record}
				report, err := ValidateEvidenceManifest(model.manifest, joinedNativeCanonicalContracts(t, id), "local")
				if err == nil || report == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("canonical intake did not replay its original validator (%s): %v", want, err)
				}
				result := p15VerificationResult(t, report, id)
				if result.Passed || len(result.ExecutedLevels) != 0 || report.ClosureComplete {
					t.Fatalf("corrupted native originals acquired acceptance credit: %+v", result)
				}
			})
		}
	}
}

func TestJoinedNativeCanonicalAuditRetainsEvidenceIdentity(t *testing.T) {
	for _, test := range []string{joinedBRG25Bridge, joinedBRG25UI, joinedBRG27UI} {
		t.Run(test, func(t *testing.T) {
			model := newJoinedNativeIntakeModel(t, test)
			id := model.record.Requirements[0]
			runner := &Runner{implSHA: model.record.ImplementationSHA,
				results: map[string]CaseResult{id: {ID: id, Status: "NOT_RUN"}},
				records: []EvidenceRecord{model.record}}
			path := filepath.Join(t.TempDir(), "synthetic-intake-audit.json")
			if err := writeExecutionAudit(runner, joinedNativeCanonicalContracts(t, id), path); err != nil {
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
			if len(audit.Rows) != 1 || audit.ClosureComplete || len(audit.Rows[0].Tier) != 0 {
				t.Fatal("audit invented execution from a linked identity")
			}
			row := audit.Rows[0]
			found := false
			for _, name := range row.TestNames {
				found = found || name == test
			}
			if !found || !strings.Contains(row.Scope, "native tiers") {
				t.Fatal("audit erased dedicated evidence identity or scope")
			}
		})
	}
}
