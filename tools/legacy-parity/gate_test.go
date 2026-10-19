/*
 * Copyright (c) Cherri Language v2.0
 * Differential Parity Gate & Adversarial Regression Tests
 */

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

func TestGate_PositiveEndToEnd(t *testing.T) {
	binPath, err := ensureCompilerBinary()
	if err != nil {
		t.Fatalf("failed to ensure compiler binary: %v", err)
	}
	repoRoot := findRepoRoot()
	tmpDir := t.TempDir()

	// Test a canonical fixture
	fixturePath := filepath.Join(repoRoot, "tests", "enum.cherri")
	res := testFixture(binPath, fixturePath, tmpDir, repoRoot)
	if res.Status != "VERIFIED_PARITY" || !res.Equal {
		t.Fatalf("expected enum.cherri to achieve VERIFIED_PARITY, got %s: %s", res.Status, res.Detail)
	}
}


func TestGate_Negative_MissingRequiredParameterFailsGate(t *testing.T) {
	binPath, err := ensureCompilerBinary()
	if err != nil {
		t.Fatalf("failed to ensure compiler binary: %v", err)
	}
	repoRoot := findRepoRoot()
	tmpDir := t.TempDir()

	// Intentionally create a fixture with an action missing a required parameter
	faultySrc := `enum callType {
    Audio,
    Video
}

action callOnFaceTime(contact: Variable, type: CallType = "Video") {
    identifier: "com.apple.facetime.facetime"
    bindings: {
        contact: {
            key: "WFFaceTimeContact"
        },
        type: {
            key: "WFFaceTimeType"
        }
    }
}

// Missing required contact parameter
callOnFaceTime()
`
	faultyFile := filepath.Join(tmpDir, "faulty.cherri")
	if err := os.WriteFile(faultyFile, []byte(faultySrc), 0644); err != nil {
		t.Fatalf("failed to write faulty fixture: %v", err)
	}

	res := testFixture(binPath, faultyFile, tmpDir, repoRoot)
	if res.Status == "VERIFIED_PARITY" || res.Equal {
		t.Fatalf("gate must fail when required parameter is missing, but got VERIFIED_PARITY")
	}
}

func TestGate_Negative_IntentionalDifferenceDetected(t *testing.T) {
	tmpDir := t.TempDir()

	p1 := map[string]any{
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.test",
				"WFWorkflowActionParameters": map[string]any{
					"TargetParam": "CorrectValue",
				},
			},
		},
	}
	p2 := map[string]any{
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.test",
				"WFWorkflowActionParameters": map[string]any{
					"TargetParam": "AlteredValue",
				},
			},
		},
	}

	bytes1, err := plist.Marshal(p1, plist.BinaryFormat)
	if err != nil {
		t.Fatalf("marshal 1: %v", err)
	}
	bytes2, err := plist.Marshal(p2, plist.BinaryFormat)
	if err != nil {
		t.Fatalf("marshal 2: %v", err)
	}

	file1 := filepath.Join(tmpDir, "p1.shortcut")
	file2 := filepath.Join(tmpDir, "p2.shortcut")
	_ = os.WriteFile(file1, bytes1, 0644)
	_ = os.WriteFile(file2, bytes2, 0644)
	cmpResult, err := shortcutcompare.CompareFiles(file1, file2)
	if err != nil {
		t.Fatalf("compare files failed: %v", err)
	}

	if cmpResult.Equal {
		t.Fatalf("gate comparator must detect intentional parameter value difference")
	}
	if len(cmpResult.Differences) == 0 {
		t.Fatalf("expected recorded differences, got none")
	}
}

func TestGate_Negative_MissingActionParameterDetected(t *testing.T) {
	tmpDir := t.TempDir()

	p1 := map[string]any{
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.test",
				"WFWorkflowActionParameters": map[string]any{
					"RequiredKey": "Presence",
					"OptionalKey": "Secondary",
				},
			},
		},
	}
	p2 := map[string]any{
		"WFWorkflowActions": []any{
			map[string]any{
				"WFWorkflowActionIdentifier": "is.workflow.actions.test",
				"WFWorkflowActionParameters": map[string]any{
					"RequiredKey": "Presence",
					// OptionalKey omitted intentionally
				},
			},
		},
	}

	bytes1, _ := plist.Marshal(p1, plist.BinaryFormat)
	bytes2, _ := plist.Marshal(p2, plist.BinaryFormat)

	file1 := filepath.Join(tmpDir, "p1.shortcut")
	file2 := filepath.Join(tmpDir, "p2.shortcut")
	_ = os.WriteFile(file1, bytes1, 0644)
	_ = os.WriteFile(file2, bytes2, 0644)

	cmpResult, err := shortcutcompare.CompareFiles(file1, file2)
	if err != nil {
		t.Fatalf("compare files failed: %v", err)
	}

	if cmpResult.Equal {
		t.Fatalf("gate comparator must detect missing parameter difference")
	}
}
