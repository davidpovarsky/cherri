package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// This bridge invokes the existing production intake. It neither rewrites a
// collector reference nor supplies the separately required app integration.
func documentationConsumerCIIntake(data []byte, implementation string, contracts *ContractsSet) (*AcceptanceReport, error) {
	var bundle struct {
		Records []EvidenceRecord `json:"records"`
	}
	if err := decodeDocumentationJSON(data, &bundle); err != nil {
		return nil, err
	}
	if len(bundle.Records) != 1 {
		return nil, fmt.Errorf("D03 CI requires exactly one original consumer record")
	}
	var record = bundle.Records[0]
	if record.EvidenceID != "ev-d03-published-installed-documentation" || record.ImplementationSHA != implementation ||
		record.Tier != "docs" || record.TestID != documentationConsumerTestID {
		return nil, fmt.Errorf("D03 CI collector identity differs")
	}
	var rawPins, err = documentationGitBlob(resolveRepoRoot(), implementation, "skills/cherri-shortcuts/compatibility-manifest.json")
	if err != nil {
		return nil, err
	}
	var pins documentationPins
	if err := decodeDocumentationJSON(rawPins, &pins); err != nil {
		return nil, err
	}
	var manifest = EvidenceManifest{SchemaVersion: "1", ImplementationSHA: implementation,
		DocsSHA: pins.DocsCommit, SchemaFingerprint: pins.SchemaFingerprint, CodecFingerprint: pins.CodecFingerprint,
		Records: bundle.Records}
	if err := validateDocumentationRecord("D03", record, &manifest); err != nil {
		return nil, err
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range contracts.Acceptance.Cases {
		if row.ID == "D03" {
			selected.Acceptance.Cases = append(selected.Acceptance.Cases, row)
		}
	}
	selected.Acceptance.CaseCount = len(selected.Acceptance.Cases)
	if selected.Acceptance.CaseCount != 1 || selected.Acceptance.Cases[0].MinimumTestLevel != "docs+integration" {
		return nil, fmt.Errorf("immutable D03 docs+integration contract missing")
	}
	var report, validationErr = ValidateEvidenceManifest(&manifest, selected, "local")
	if validationErr == nil || report == nil || report.TotalCases != 1 || report.PassedCases != 0 ||
		report.ClosureComplete || len(report.Results) != 1 || report.Results[0].ID != "D03" ||
		report.Results[0].Passed || !reflect.DeepEqual(report.Results[0].ExecutedLevels, []string{"docs"}) ||
		!reflect.DeepEqual(report.ValidationErrors, []string{"requirement D03 has unsatisfied local tier"}) {
		return nil, fmt.Errorf("D03 producer intake is invalid or accidentally closed missing app integration: %+v %v", report, validationErr)
	}
	return report, nil
}

func TestDocumentationConsumerCIActualIntake(t *testing.T) {
	var input = os.Getenv("CHERRI_D03_CONSUMER_EVIDENCE")
	if input == "" {
		if os.Getenv("CHERRI_D03_CI_REQUIRED") == "1" {
			t.Fatal("explicit D03 CI gate has no actual collector output")
		}
		t.Skip("actual installed/public D03 consumer collection is an explicit external CI job")
	}
	if runtime.GOOS == "windows" {
		t.Fatal("D03 installed consumer collection/intake requires the same POSIX workspace")
	}
	var implementation = resolveCurrentGitSHA()
	if implementation != os.Getenv("GITHUB_SHA") || !documentationHash(implementation, 40) {
		t.Fatal("D03 CI source is not the exact checked-out workflow commit")
	}
	var clean, cleanErr = exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=all").Output()
	if cleanErr != nil || len(clean) != 0 {
		t.Fatal("D03 CI source checkout is not clean", cleanErr)
	}
	var info, statErr = os.Lstat(input)
	if statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 4*1024*1024 {
		t.Fatal("D03 CI collector output is missing, linked or oversized", statErr)
	}
	var original, err = os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	var contracts, contractErr = LoadAllContracts("", "", "")
	if contractErr != nil {
		t.Fatal(contractErr)
	}
	var report, intakeErr = documentationConsumerCIIntake(original, implementation, contracts)
	if intakeErr != nil {
		t.Fatal(intakeErr)
	}
	var after, readErr = os.ReadFile(input)
	if readErr != nil || !reflect.DeepEqual(after, original) {
		t.Fatal("D03 original collector bytes changed during intake", readErr)
	}
	var output = os.Getenv("CHERRI_D03_INTAKE_RECEIPT")
	if output == "" || !filepath.IsAbs(output) {
		t.Fatal("D03 CI must retain an explicit run-owned intake receipt")
	}
	var receipt = map[string]any{"status": "PASS_D03_DOCS_PRODUCER_INTAKE_ONLY", "implementation_sha": implementation,
		"collector_evidence_sha256": nativeEncodingDigest(original), "original_references_unchanged": true,
		"documentation_record_accepted": true, "app_integration_executed": false, "d03_complete": false,
		"closure_complete": false, "report": report}
	var encoded, encodeErr = json.MarshalIndent(receipt, "", "  ")
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	var file, createErr = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if createErr != nil {
		t.Fatal(createErr)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("P15_D03_CONSUMER_CI_INTAKE:", string(encoded))
}

func TestDocumentationConsumerCIBundleControls(t *testing.T) {
	var model = newDocumentationIntakeModel(t)
	var record = documentationMaterializeRecord(t, model.consumer, model.docBytes, "")
	record.EvidenceID = "ev-d03-published-installed-documentation"
	var healthy = documentationModelJSON(t, map[string]any{"records": []EvidenceRecord{record}})
	var report, err = documentationConsumerCIIntake(healthy, model.manifest.ImplementationSHA, model.contracts)
	if err != nil || report.PassedCases != 0 || report.ClosureComplete {
		t.Fatalf("explicitly synthetic bridge control: %+v %v", report, err)
	}
	for _, name := range []string{"extra_wrapper_key", "duplicate_record", "borrowed_implementation", "app_record", "duplicate_json_key"} {
		t.Run(name, func(t *testing.T) {
			var changed = record
			var wrapper = map[string]any{"records": []EvidenceRecord{changed}}
			switch name {
			case "extra_wrapper_key":
				wrapper["assume_passed"] = true
			case "duplicate_record":
				wrapper["records"] = []EvidenceRecord{record, record}
			case "borrowed_implementation":
				changed.ImplementationSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
				wrapper["records"] = []EvidenceRecord{changed}
			case "app_record":
				wrapper["records"] = []EvidenceRecord{model.app}
			}
			var encoded = documentationModelJSON(t, wrapper)
			if name == "duplicate_json_key" {
				encoded = append([]byte(`{"records":[],`), healthy[1:]...)
			}
			if _, err := documentationConsumerCIIntake(encoded, model.manifest.ImplementationSHA, model.contracts); err == nil {
				t.Fatal("invalid transport wrapper was accepted")
			}
		})
	}
}
