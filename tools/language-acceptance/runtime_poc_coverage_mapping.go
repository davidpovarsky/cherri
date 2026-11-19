package main

import (
	"encoding/hex"
	"fmt"
)

const runtimePOCCoverageSourceSHA256 = "4a13f90af2bdc326e24f9dbf785f76a1ae41043e176b5a3188ab794694a19aa3"
const runtimePOCCoverageExpectedContractSHA256 = "8baa3279a770ca20e3daa6036f416360a308d6f40eb1dd0874a38afeb7d0b10c"
const runtimePOCS01UnicodeOutput = "שָׁלוֹם👩‍💻é|α|אבב|éα|🌍🌍||￼🙂αבéαשָׁ🌍"
const runtimePOCF01EffectTrace = "START|OR_TRUE|OR_FALSE|NEST_OR|NEST_AND|MIX_LEFT|MIX_RIGHT"
const runtimePOCF01BooleanResults = "and_skip=F|or_skip=T|false_or_true=T|false_or_false=F|nested_or=T|nested_and=F|outer_and=F|outer_or=T|mixed_run=T"

// The Python observer replays native-byte, reset and device receipts. Intake
// requires its exact scoped identity and bindings; local graph tests cannot
// stand in for this runtime producer or borrow its result for another ID.
func validateRuntimePOCCoverageRecord(id string, record EvidenceRecord) error {
	if (id != "S01" && id != "F01") || record.Tier != "ios-runtime" {
		return nil
	}
	var expectedTest = "ios27-runtime-poc:coverage-s01"
	if id == "F01" {
		expectedTest = "ios27-runtime-poc:coverage-f01"
	}
	var invalid = func() error {
		return fmt.Errorf("coverage runtime record %s is detached from its reviewed observer, source or receipt bindings", record.EvidenceID)
	}
	if record.TestID != expectedTest || record.FixtureSHA256 != runtimePOCCoverageSourceSHA256 ||
		len(record.Requirements) != 1 || record.Requirements[0] != id ||
		record.Assertions["expected_contract_sha256"] != runtimePOCCoverageExpectedContractSHA256 ||
		record.Assertions["fresh_reset_observed"] != "true" || len(record.Artifacts) == 0 ||
		record.Assertions["run_id"] == "" || record.Assertions["run_attempt"] == "" {
		return invalid()
	}
	for _, key := range []string{"diagnostics_sha256", "source_receipt_sha256", "coverage_result_sha256", "context_sha256", "reset_proof_sha256", "expected_contract_sha256", "observed_clipboard_sha256"} {
		var value = record.Assertions[key]
		if _, err := hex.DecodeString(value); err != nil || len(value) != 64 {
			return invalid()
		}
	}
	for _, artifact := range record.Artifacts {
		if artifact.RunID != record.Assertions["run_id"] {
			return invalid()
		}
	}
	if record.Result == "passed" {
		if record.Assertions["exact_output_match"] != "true" {
			return invalid()
		}
	} else if record.Result != "failed" || record.Assertions["exact_output_match"] != "false" {
		return invalid()
	}
	return nil
}
