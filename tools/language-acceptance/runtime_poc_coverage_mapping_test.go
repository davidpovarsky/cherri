package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRuntimePOCCoverageMappingsMatchReviewedFixture(t *testing.T) {
	var root = resolveRepoRoot()
	var data, err = os.ReadFile(filepath.Join(root, "tests", "runtime_poc", "CherriS01F01CoveragePOC.expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var digest = sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != runtimePOCCoverageExpectedContractSHA256 {
		t.Fatal("coverage mapping lost the reviewed expected-byte contract")
	}
	var expected struct {
		SourcePath              string            `json:"source_path"`
		SourceSHA256            string            `json:"source_sha256"`
		Requirements            []string          `json:"requirements"`
		ExpectedClipboard       string            `json:"expected_clipboard"`
		ExpectedClipboardSHA256 string            `json:"expected_clipboard_sha256"`
		ExpectedFields          map[string]string `json:"expected_fields"`
		RuntimeStatus           string            `json:"runtime_status"`
	}
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	var source, sourceErr = os.ReadFile(filepath.Join(root, filepath.FromSlash(expected.SourcePath)))
	if sourceErr != nil {
		t.Fatal(sourceErr)
	}
	digest = sha256.Sum256(source)
	if expected.SourceSHA256 != runtimePOCCoverageSourceSHA256 || hex.EncodeToString(digest[:]) != runtimePOCCoverageSourceSHA256 ||
		!reflect.DeepEqual(expected.Requirements, []string{"S01", "F01"}) || expected.RuntimeStatus != "NOT_RUN" {
		t.Fatal("reviewed source identity or non-runtime oracle scope changed")
	}
	var fields = map[string]string{"S01_OUTPUT": runtimePOCS01UnicodeOutput,
		"F01_TRACE": runtimePOCF01EffectTrace, "F01_RESULTS": runtimePOCF01BooleanResults}
	var clipboard = "CHERRI_S01_F01_COVERAGE_OK\nS01_OUTPUT=" + fields["S01_OUTPUT"] +
		"\nF01_TRACE=" + fields["F01_TRACE"] + "\nF01_RESULTS=" + fields["F01_RESULTS"]
	digest = sha256.Sum256([]byte(clipboard))
	if !reflect.DeepEqual(fields, expected.ExpectedFields) || clipboard != expected.ExpectedClipboard ||
		hex.EncodeToString(digest[:]) != expected.ExpectedClipboardSHA256 {
		t.Fatal("Go intake oracle differs from exact reviewed Unicode and Boolean bytes")
	}
	if literalCollectionProductionTestForTier("S01", "ios-runtime") != "ios27-runtime-poc:coverage-s01" {
		t.Fatal("S01 tier authorization still points to the weaker fixture")
	}
	var contracts, contractErr = LoadAllContracts("", "", "")
	if contractErr != nil {
		t.Fatal(contractErr)
	}
	var mapping, mapErr = BuildRequirementsEvidenceMap(contracts)
	if mapErr != nil {
		t.Fatal(mapErr)
	}
	for _, id := range []string{"S01", "F01"} {
		var wanted = map[string]string{"s01_unicode_output": fields["S01_OUTPUT"]}
		var testID = "ios27-runtime-poc:coverage-s01"
		var oldID = "ios27-runtime-poc:unicode"
		if id == "F01" {
			wanted = map[string]string{"f01_effect_trace": fields["F01_TRACE"], "f01_boolean_results": fields["F01_RESULTS"]}
			testID, oldID = "ios27-runtime-poc:coverage-f01", "ios27-runtime-poc:conditionals"
		}
		var declaration = mapping.Mappings[id]
		if !reflect.DeepEqual(declaration.RequiredAssertions, wanted) {
			t.Fatal("wrong exact coverage assertions", id)
		}
		var found bool
		for _, producer := range declaration.TestIDs {
			if producer == oldID {
				t.Fatal("old weaker fixture still authorizes coverage", id)
			}
			if producer == testID {
				found = true
			}
		}
		if !found {
			t.Fatal("exact coverage producer absent", id)
		}
	}
	for _, id := range []string{"RP19", "RP18", "BRG08", "BRG13"} {
		for _, producer := range mapping.Mappings[id].TestIDs {
			if producer == "ios27-runtime-poc:coverage-s01" || producer == "ios27-runtime-poc:coverage-f01" {
				t.Fatal("broader obligation inherited coverage fixture", id)
			}
		}
	}
}
