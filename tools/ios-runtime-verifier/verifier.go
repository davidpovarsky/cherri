package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type MenuProof struct {
	Target            string `json:"target"`
	ActualLabel       string `json:"actual_label"`
	App               string `json:"app"`
	Kind              string `json:"kind"`
	Stage             string `json:"stage"`
	Result            string `json:"result"`
	XCTestPassed      bool   `json:"xctest_passed"`
	ExactTapConfirmed bool   `json:"exact_tap_confirmed"`
	ClipboardSentinel string `json:"clipboard_sentinel,omitempty"`
	ClipboardResult   string `json:"clipboard_result"`
	RuntimePass       bool   `json:"runtime_pass"`
	HeadSHA           string `json:"head_sha,omitempty"`
	RunID             string `json:"run_id,omitempty"`
	LogFile           string `json:"log_file,omitempty"`
}

type VerificationReport struct {
	Passed      bool       `json:"passed"`
	Status      string     `json:"status"` // "PASS", "FAIL", "BLOCKED"
	AlphaProof  *MenuProof `json:"alpha_proof,omitempty"`
	BetaProof   *MenuProof `json:"beta_proof,omitempty"`
	Errors      []string   `json:"errors,omitempty"`
	Diagnostics []string   `json:"diagnostics,omitempty"`
}

var exactTapRegex = regexp.MustCompile(`CHERRI_EXACT_MENU_TAP\s+target=(\S+)\s+actual_label=(\S+)\s+app=(\S+)\s+kind=(\S+)\s+stage=(\S+)\s+result=(\S+)`)

// ParseMenuTapFromLog inspects an XCUITest helper log for exact tap proof.
func ParseMenuTapFromLog(logContent string, expectedTarget string, expectedStage string) (*MenuProof, error) {
	if strings.TrimSpace(logContent) == "" {
		return nil, fmt.Errorf("log content is empty")
	}

	// 1. Check for false-positive "Allow" tap during menu selection
	if strings.Contains(logContent, "Found and tapping 'Allow' in SpringBoard") ||
		strings.Contains(logContent, "Tap \"Allow\" Button") {
		return nil, fmt.Errorf("false positive: log shows 'Allow' button was tapped during menu selection")
	}

	// 2. Check for exact structured marker
	match := exactTapRegex.FindStringSubmatch(logContent)
	if match == nil {
		return nil, fmt.Errorf("missing CHERRI_EXACT_MENU_TAP marker for target %q", expectedTarget)
	}

	target := match[1]
	actualLabel := match[2]
	app := match[3]
	kind := match[4]
	stage := match[5]
	result := match[6]

	if target != expectedTarget {
		return nil, fmt.Errorf("target mismatch: expected %q, got %q", expectedTarget, target)
	}
	if stage != expectedStage {
		return nil, fmt.Errorf("stage mismatch: expected %q, got %q", expectedStage, stage)
	}
	if result != "tapped" {
		return nil, fmt.Errorf("unexpected tap result %q for target %q", result, target)
	}

	// 3. Verify XCTest execution succeeded (0 failures)
	xctestPassed := false
	if strings.Contains(logContent, "** TEST EXECUTE SUCCEEDED **") ||
		(strings.Contains(logContent, "Executed 1 test, with 0 failures") && strings.Contains(logContent, "passed")) {
		xctestPassed = true
	}
	if !xctestPassed {
		return nil, fmt.Errorf("XCUITest did not pass with 0 failures for target %q", target)
	}

	return &MenuProof{
		Target:            target,
		ActualLabel:       actualLabel,
		App:               app,
		Kind:              kind,
		Stage:             stage,
		Result:            result,
		XCTestPassed:      xctestPassed,
		ExactTapConfirmed: true,
	}, nil
}

// VerifyMenuExecution checks exact UI tap proof and clipboard result for a single branch.
func VerifyMenuExecution(logContent string, clipboardContent string, expectedTarget string, expectedStage string, expectedClipboard string, oppositeClipboard string) (*MenuProof, error) {
	proof, err := ParseMenuTapFromLog(logContent, expectedTarget, expectedStage)
	if err != nil {
		return nil, err
	}

	clip := strings.TrimSpace(clipboardContent)
	if clip == "" {
		return nil, fmt.Errorf("clipboard result for target %q is empty", expectedTarget)
	}
	if !strings.Contains(clip, expectedClipboard) {
		return nil, fmt.Errorf("clipboard for target %q does not contain expected %q: got %q", expectedTarget, expectedClipboard, clip)
	}
	if strings.Contains(clip, oppositeClipboard) {
		return nil, fmt.Errorf("clipboard for target %q contains opposite branch output %q", expectedTarget, oppositeClipboard)
	}

	proof.ClipboardResult = clip
	proof.RuntimePass = proof.ExactTapConfirmed && proof.XCTestPassed
	return proof, nil
}

// VerifyMenuPair validates both branches together.
func VerifyMenuPair(alphaProof *MenuProof, betaProof *MenuProof) (*VerificationReport, error) {
	report := &VerificationReport{
		Status: "FAIL",
	}

	if alphaProof == nil {
		report.Errors = append(report.Errors, "missing Alpha menu verification proof")
	} else if !alphaProof.RuntimePass || !alphaProof.ExactTapConfirmed {
		report.Errors = append(report.Errors, "Alpha menu verification did not satisfy runtime pass")
	}

	if betaProof == nil {
		report.Errors = append(report.Errors, "missing Beta menu verification proof")
	} else if !betaProof.RuntimePass || !betaProof.ExactTapConfirmed {
		report.Errors = append(report.Errors, "Beta menu verification did not satisfy runtime pass")
	}

	if len(report.Errors) > 0 {
		return report, fmt.Errorf("menu verification failed: %s", strings.Join(report.Errors, "; "))
	}

	report.Passed = true
	report.Status = "PASS"
	report.AlphaProof = alphaProof
	report.BetaProof = betaProof
	return report, nil
}

// VerifyDirectory validates evidence files in an artifacts directory.
func VerifyDirectory(dir string) (*VerificationReport, error) {
	report := &VerificationReport{
		Status: "FAIL",
	}

	// 1. Read Alpha log and clipboard
	alphaLogBytes, err := os.ReadFile(filepath.Join(dir, "ui_helper_menu_select_alpha.log"))
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("cannot read Alpha UI helper log: %v", err))
	}
	alphaClipBytes, err := os.ReadFile(filepath.Join(dir, "menu-alpha-clipboard-result.txt"))
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("cannot read Alpha clipboard result: %v", err))
	}

	// 2. Read Beta log and clipboard
	betaLogBytes, err := os.ReadFile(filepath.Join(dir, "ui_helper_menu_select_beta.log"))
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("cannot read Beta UI helper log: %v", err))
	}
	betaClipBytes, err := os.ReadFile(filepath.Join(dir, "menu-beta-clipboard-result.txt"))
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("cannot read Beta clipboard result: %v", err))
	}

	if len(report.Errors) > 0 {
		return report, fmt.Errorf("missing evidence files: %s", strings.Join(report.Errors, "; "))
	}

	alphaProof, alphaErr := VerifyMenuExecution(string(alphaLogBytes), string(alphaClipBytes), "Alpha", "menu_select_alpha", "CHERRI_MENU_ALPHA", "CHERRI_MENU_BETA")
	if alphaErr != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Alpha verification error: %v", alphaErr))
	} else {
		report.AlphaProof = alphaProof
	}

	betaProof, betaErr := VerifyMenuExecution(string(betaLogBytes), string(betaClipBytes), "Beta", "menu_select_beta", "CHERRI_MENU_BETA", "CHERRI_MENU_ALPHA")
	if betaErr != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Beta verification error: %v", betaErr))
	} else {
		report.BetaProof = betaProof
	}

	if len(report.Errors) > 0 {
		return report, fmt.Errorf("branch verification failed: %s", strings.Join(report.Errors, "; "))
	}

	// Check runtime-evidence.json if present
	evidencePath := filepath.Join(dir, "runtime-evidence.json")
	if evBytes, err := os.ReadFile(evidencePath); err == nil {
		var records []map[string]interface{}
		if err := json.Unmarshal(evBytes, &records); err == nil {
			for _, rec := range records {
				evID, _ := rec["evidence_id"].(string)
				res, _ := rec["result"].(string)
				rtPass, _ := rec["runtime_pass"].(bool)
				if evID == "ev-runtime-menu-alpha" || evID == "ev-runtime-menu-beta" {
					if res != "passed" || !rtPass {
						report.Errors = append(report.Errors, fmt.Sprintf("runtime-evidence.json record %s has result=%q, runtime_pass=%v (expected passed, true)", evID, res, rtPass))
					}
				}
			}
		}
	}

	if len(report.Errors) > 0 {
		return report, fmt.Errorf("runtime-evidence.json check failed: %s", strings.Join(report.Errors, "; "))
	}

	report.Passed = true
	report.Status = "PASS"
	return report, nil
}

func main() {
	dir := flag.String("dir", "artifacts", "Directory containing runtime evidence artifacts")
	flag.Parse()

	report, err := VerifyDirectory(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "VERIFICATION FAILED: %v\n", err)
		for _, e := range report.Errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		os.Exit(1)
	}

	fmt.Println("==================================================")
	fmt.Println("    IOS 27 MENU RUNTIME EVIDENCE VERIFIED PASS")
	fmt.Println("==================================================")
	fmt.Printf("Alpha: target=%s actual_label=%s app=%s kind=%s result=%s (clipboard=%s)\n",
		report.AlphaProof.Target, report.AlphaProof.ActualLabel, report.AlphaProof.App, report.AlphaProof.Kind, report.AlphaProof.Result, report.AlphaProof.ClipboardResult)
	fmt.Printf("Beta:  target=%s actual_label=%s app=%s kind=%s result=%s (clipboard=%s)\n",
		report.BetaProof.Target, report.BetaProof.ActualLabel, report.BetaProof.App, report.BetaProof.Kind, report.BetaProof.Result, report.BetaProof.ClipboardResult)
	fmt.Println("GATE: PASS")
	os.Exit(0)
}
