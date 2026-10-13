package main

import (
	"strings"
	"testing"
)

func TestVerifyEvents_Success(t *testing.T) {
	// Simulate JSON stream where all 43 tests pass
	var b strings.Builder
	for _, name := range requiredTestNames {
		b.WriteString(`{"Action":"run","Test":"` + name + `"}` + "\n")
		b.WriteString(`{"Action":"pass","Test":"` + name + `"}` + "\n")
	}

	res, err := VerifyEvents(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.TotalPassed != len(requiredTestNames) {
		t.Errorf("expected %d passed, got %d", len(requiredTestNames), res.TotalPassed)
	}
	if len(res.Missing) != 0 {
		t.Errorf("expected 0 missing, got %v", res.Missing)
	}
}

func TestVerifyEvents_NegativeCorruptedRegexOmittingCodecAndCore(t *testing.T) {
	// Simulate the exact Round 2 bug where "TestCore|TestCodec" was used instead of uppercase,
	// so NO TestCORE* or TestCODEC* tests are emitted.
	var b strings.Builder
	for _, name := range requiredTestNames {
		if strings.HasPrefix(name, "TestCORE") || strings.HasPrefix(name, "TestCODEC") {
			continue // skipped by bad regex!
		}
		b.WriteString(`{"Action":"run","Test":"` + name + `"}` + "\n")
		b.WriteString(`{"Action":"pass","Test":"` + name + `"}` + "\n")
	}

	res, err := VerifyEvents(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must fail: 17 tests missing!
	if len(res.Missing) != 17 {
		t.Errorf("expected exactly 17 missing tests (7 CORE + 10 CODEC), got %d (%v)", len(res.Missing), res.Missing)
	}
	if res.FamilyCounts["CORE"] != 0 {
		t.Errorf("expected 0 CORE tests, got %d", res.FamilyCounts["CORE"])
	}
	if res.FamilyCounts["CODEC"] != 0 {
		t.Errorf("expected 0 CODEC tests, got %d", res.FamilyCounts["CODEC"])
	}
}

func TestVerifyEvents_NegativeSingleTestMissing(t *testing.T) {
	var b strings.Builder
	for _, name := range requiredTestNames {
		if name == "TestCTRL008_ASTControlFlowSourceOwnership" {
			continue
		}
		b.WriteString(`{"Action":"run","Test":"` + name + `"}` + "\n")
		b.WriteString(`{"Action":"pass","Test":"` + name + `"}` + "\n")
	}

	res, err := VerifyEvents(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Missing) != 1 || res.Missing[0] != "TestCTRL008_ASTControlFlowSourceOwnership" {
		t.Errorf("expected TestCTRL008_ASTControlFlowSourceOwnership to be missing, got %v", res.Missing)
	}
}
