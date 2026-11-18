package main

import (
	"os"
	"strings"
	"testing"
)

func TestSkillEditProductionEvidence(t *testing.T) {
	if os.Getenv("CHERRI_K05_LIVE_SIGNING") != "1" {
		t.Skip("dedicated K05 formal evidence requires an explicit real synthetic signing run")
	}
	testNamedProductionEvidence(t, "K05", skillProductionTests["K05"], nil, (*Runner).runSkillEditRequirement,
		"P15_SKILL_EDIT_RECEIPT:",
		"Actual V2 repair/build/import/one-field edit, exact full typed-document gate, preserved originals/prior outputs, blocked unauthorized/missing-comparator cases and real configured signing-service output over the verified bytes. No Apple validity, import, installed-consumer or runtime inference.")
}

func TestSkillEditSkippedLiveTestIsNotEvidence(t *testing.T) {
	t.Setenv("CHERRI_K05_LIVE_SIGNING", "")
	passed, detail := executeNamedProductionTests([]string{"TestP15K05LiveSigning"})
	if passed || !strings.Contains(detail, "missing=[TestP15K05LiveSigning]") {
		t.Fatalf("skipped real signing test minted production evidence: passed=%v %s", passed, detail)
	}
}
