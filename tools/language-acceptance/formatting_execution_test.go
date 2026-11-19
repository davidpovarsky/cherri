package main

import "testing"

func TestFormatterProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "E04", formattingProductionTests["E04"],
		map[string]string{"integration": "TestP15E04ActualCLIAndServiceNativeIdentity"},
		(*Runner).runFormattingRequirement, "P15_FORMATTER_PRODUCTION_RECEIPT:",
		"Actual syntax, service and public CLI read/write/check operations; exact literal/comment bytes, idempotence, strict compiled native graph identity, invalid-source preservation and changed-value/comment controls. No Apple runtime or project closure claim.")
}

func TestFormatterEvidenceRejectsCrossTierAndTruncatedScenario(t *testing.T) {
	tests := formattingProductionTests["E04"]
	record := EvidenceRecord{
		EvidenceID: "formatter-input-control", Tier: "integration",
		TestID:     "github.com/electrikmilk/cherri:" + tests[1],
		Command:    productionTestCommand(tests),
		Assertions: map[string]string{"required_production_tests": tests[0] + "\n" + tests[1] + "\n" + tests[2]},
	}
	if err := validateFormattingRecord("E04", record); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"unit_producer", "wrong_tier", "truncated_command", "missing_oracle"} {
		broken := record
		broken.Assertions = map[string]string{"required_production_tests": record.Assertions["required_production_tests"]}
		switch mutation {
		case "unit_producer":
			broken.TestID = "github.com/electrikmilk/cherri:" + tests[0]
		case "wrong_tier":
			broken.Tier = "ios"
		case "truncated_command":
			broken.Command = productionTestCommand(tests[:1])
		case "missing_oracle":
			broken.Assertions["required_production_tests"] = tests[0] + "\n" + tests[1]
		}
		if err := validateFormattingRecord("E04", broken); err == nil {
			t.Errorf("accepted %s", mutation)
		}
	}
}
