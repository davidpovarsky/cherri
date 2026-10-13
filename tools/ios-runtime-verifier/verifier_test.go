package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Positive test template log for an authentic exact tap
func mockLog(target string, stage string, kind string, app string) string {
	return `
2026-10-13 00:38:43.025970+0000 ImportHelperUITests-Runner[58767:158493] [Default] Running tests...
Test Case '-[ImportHelperUITests.ImportHelperUITests testInteractWithShortcuts]' started.
Strict menu selection stage: '` + stage + `' with target '` + target + `'
Found exact ` + kind + ` '` + target + `' in ` + app + `, tapping...
CHERRI_EXACT_MENU_TAP target=` + target + ` actual_label=` + target + ` app=` + app + ` kind=` + kind + ` stage=` + stage + ` result=tapped
UI Test Helper finished strict menu tap. Target: ` + target + `
Test Case '-[ImportHelperUITests.ImportHelperUITests testInteractWithShortcuts]' passed (3.992 seconds).
	 Executed 1 test, with 0 failures (0 unexpected) in 3.992 (4.014) seconds
** TEST EXECUTE SUCCEEDED **
`
}

// Case 1: Alpha clipboard correct but XCTest clicked Allow or no exact marker -> FAIL
// Demonstrates that the prior Round 3 Alpha log fails verification!
func TestCase1_PriorRound3AlphaAllowTap_MustFail(t *testing.T) {
	// Authentic Round 3 helper log snippet from run 38327180296 where Allow was tapped instead of Alpha
	round3AlphaLog := `
Active UI Helper stage: 'menu_select_alpha', menuTarget: 'Alpha'
Looking for menu item: 'Alpha'
    t =     3.35s Waiting 2.0s for "Alpha" StaticText to exist
    t =     5.40s Capturing element debug description
    t =    18.24s Waiting 1.0s for "Allow" Button to exist
Found and tapping 'Allow' in SpringBoard...
    t =    19.35s Tap "Allow" Button
    t =    19.50s Synthesize event
    t =    19.84s Wait for com.apple.springboard to idle
UI Test Helper finished. Tapped button: true
Test Case '-[ImportHelperUITests.ImportHelperUITests testInteractWithShortcuts]' passed (21.441 seconds).
	 Executed 1 test, with 0 failures (0 unexpected) in 21.441 (21.461) seconds
** TEST EXECUTE SUCCEEDED **
`
	_, err := VerifyMenuExecution(round3AlphaLog, "CHERRI_MENU_ALPHA\n", "Alpha", "menu_select_alpha", "CHERRI_MENU_ALPHA", "CHERRI_MENU_BETA")
	if err == nil {
		t.Fatalf("EXPECTED VERIFIER FAILURE on Round 3 Allow-tap log, but verifier passed!")
	}
	if !strings.Contains(err.Error(), "Allow") && !strings.Contains(err.Error(), "missing CHERRI_EXACT_MENU_TAP") {
		t.Errorf("expected rejection reason mentioning Allow or missing tap marker, got: %v", err)
	}
}

// Case 2: Alpha exact tap logged but clipboard contains CHERRI_MENU_BETA -> FAIL
func TestCase2_MismatchedClipboardOutput_MustFail(t *testing.T) {
	log := mockLog("Alpha", "menu_select_alpha", "staticText", "SpringBoard")
	_, err := VerifyMenuExecution(log, "CHERRI_MENU_BETA\n", "Alpha", "menu_select_alpha", "CHERRI_MENU_ALPHA", "CHERRI_MENU_BETA")
	if err == nil {
		t.Fatalf("expected verifier failure when clipboard contains opposite branch output CHERRI_MENU_BETA, but it passed")
	}
	if !strings.Contains(err.Error(), "does not contain expected") && !strings.Contains(err.Error(), "opposite branch") {
		t.Errorf("expected mismatch error message, got: %v", err)
	}
}

// Case 3: Beta success but Alpha missing -> FAIL
func TestCase3_BetaPassedButAlphaMissing_MustFail(t *testing.T) {
	betaLog := mockLog("Beta", "menu_select_beta", "staticText", "SpringBoard")
	betaProof, err := VerifyMenuExecution(betaLog, "CHERRI_MENU_BETA\n", "Beta", "menu_select_beta", "CHERRI_MENU_BETA", "CHERRI_MENU_ALPHA")
	if err != nil {
		t.Fatalf("unexpected error on valid Beta proof: %v", err)
	}

	report, err := VerifyMenuPair(nil, betaProof)
	if err == nil || report.Passed {
		t.Fatalf("expected VerifyMenuPair to fail when Alpha proof is missing, but got pass")
	}
	if !strings.Contains(err.Error(), "missing Alpha") {
		t.Errorf("expected error about missing Alpha, got: %v", err)
	}
}

// Case 4: Missing/corrupted XCTest log, mismatched branch, or missing required evidence -> FAIL
func TestCase4_CorruptedLogOrMismatchedStage_MustFail(t *testing.T) {
	// 4a. Corrupted empty log
	_, err := VerifyMenuExecution("", "CHERRI_MENU_ALPHA", "Alpha", "menu_select_alpha", "CHERRI_MENU_ALPHA", "CHERRI_MENU_BETA")
	if err == nil {
		t.Fatalf("expected failure on empty log")
	}

	// 4b. Mismatched stage
	log := mockLog("Alpha", "menu_select_beta", "staticText", "SpringBoard")
	_, err = VerifyMenuExecution(log, "CHERRI_MENU_ALPHA", "Alpha", "menu_select_alpha", "CHERRI_MENU_ALPHA", "CHERRI_MENU_BETA")
	if err == nil {
		t.Fatalf("expected failure on mismatched stage")
	}

	// 4c. Log without XCTest success
	failingXCTestLog := `
Strict menu selection stage: 'menu_select_alpha' with target 'Alpha'
CHERRI_EXACT_MENU_TAP target=Alpha actual_label=Alpha app=SpringBoard kind=staticText stage=menu_select_alpha result=tapped
Test Case '-[ImportHelperUITests.ImportHelperUITests testInteractWithShortcuts]' failed (2.0s).
** TEST EXECUTE FAILED **
`
	_, err = VerifyMenuExecution(failingXCTestLog, "CHERRI_MENU_ALPHA", "Alpha", "menu_select_alpha", "CHERRI_MENU_ALPHA", "CHERRI_MENU_BETA")
	if err == nil {
		t.Fatalf("expected failure on failed XCTest")
	}
}

// Case 5: BLOCKED status / directory missing evidence -> FAIL (status retains BLOCKED / FAIL)
func TestCase5_BlockedStatusOrMissingDirFiles_MustFail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "verifier_blocked_test_*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// In an empty directory, VerifyDirectory must fail
	report, err := VerifyDirectory(tempDir)
	if err == nil || report.Passed {
		t.Fatalf("expected VerifyDirectory to fail on empty directory, but it passed")
	}
	if report.Status == "PASS" {
		t.Fatalf("report status must not be PASS")
	}
}

// Case 6: Both exact-target tap proofs plus correct fresh clipboard results -> PASS
func TestCase6_BothAlphaAndBetaExactTapAndClipboard_MustPass(t *testing.T) {
	alphaLog := mockLog("Alpha", "menu_select_alpha", "staticText", "SpringBoard")
	betaLog := mockLog("Beta", "menu_select_beta", "staticText", "SpringBoard")

	alphaProof, errA := VerifyMenuExecution(alphaLog, "CHERRI_MENU_ALPHA\n", "Alpha", "menu_select_alpha", "CHERRI_MENU_ALPHA", "CHERRI_MENU_BETA")
	if errA != nil {
		t.Fatalf("unexpected failure for Alpha: %v", errA)
	}

	betaProof, errB := VerifyMenuExecution(betaLog, "CHERRI_MENU_BETA\n", "Beta", "menu_select_beta", "CHERRI_MENU_BETA", "CHERRI_MENU_ALPHA")
	if errB != nil {
		t.Fatalf("unexpected failure for Beta: %v", errB)
	}

	report, err := VerifyMenuPair(alphaProof, betaProof)
	if err != nil {
		t.Fatalf("unexpected failure on VerifyMenuPair: %v", err)
	}
	if !report.Passed || report.Status != "PASS" {
		t.Fatalf("expected report.Passed = true, status = PASS, got %v / %q", report.Passed, report.Status)
	}

	// Also verify directory e2e flow with temp dir
	tempDir, err := os.MkdirTemp("", "verifier_pass_test_*")
	if err != nil {
		t.Fatalf("cannot create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.WriteFile(filepath.Join(tempDir, "ui_helper_menu_select_alpha.log"), []byte(alphaLog), 0644)
	os.WriteFile(filepath.Join(tempDir, "menu-alpha-clipboard-result.txt"), []byte("CHERRI_MENU_ALPHA\n"), 0644)
	os.WriteFile(filepath.Join(tempDir, "ui_helper_menu_select_beta.log"), []byte(betaLog), 0644)
	os.WriteFile(filepath.Join(tempDir, "menu-beta-clipboard-result.txt"), []byte("CHERRI_MENU_BETA\n"), 0644)
	os.WriteFile(filepath.Join(tempDir, "runtime-evidence.json"), []byte(`[
  {"evidence_id": "ev-runtime-menu-alpha", "result": "passed", "runtime_pass": true},
  {"evidence_id": "ev-runtime-menu-beta", "result": "passed", "runtime_pass": true}
]`), 0644)

	dirReport, err := VerifyDirectory(tempDir)
	if err != nil {
		t.Fatalf("unexpected VerifyDirectory failure: %v", err)
	}
	if !dirReport.Passed || dirReport.Status != "PASS" {
		t.Fatalf("expected dirReport to PASS, got %v / %q", dirReport.Passed, dirReport.Status)
	}
}
