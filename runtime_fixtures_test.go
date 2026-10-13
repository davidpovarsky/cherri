package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	args "github.com/electrikmilk/args-parser"
)

func TestRuntimeFixtures_DictionaryApproachesStructural(t *testing.T) {
	fixturePath := filepath.Join("tests", "runtime_poc", "CherriDictionaryRuntimePOC.cherri")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", fixturePath, err)
	}

	session := compileSourceToSession(t, "dict_poc", string(content))
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions

	var (
		foundDetectDict = false
		foundNativeDict = 0
		foundSetName    = false
		foundCoercionD  = false
	)

	for _, act := range actions {
		s := fmt.Sprintf("%#v", act.WFWorkflowActionParameters)
		if strings.Contains(s, "WFCoercionVariableAggrandizement") &&
			strings.Contains(s, "WFDictionaryContentItem") {
			foundCoercionD = true
		}

		switch act.WFWorkflowActionIdentifier {
		case "is.workflow.actions.detect.dictionary":
			foundDetectDict = true
			if act.WFWorkflowActionParameters["WFInput"] == nil {
				t.Errorf("Approach A detect.dictionary must specify WFInput")
			}
		case "is.workflow.actions.dictionary":
			foundNativeDict++
			items := act.WFWorkflowActionParameters["WFItems"]
			if items == nil {
				t.Errorf("Approach B native dictionary must specify WFItems")
			}
		case "is.workflow.actions.setitemname":
			if act.WFWorkflowActionParameters["WFName"] == "sample.json" {
				foundSetName = true
			}
		}
	}

	if !foundDetectDict {
		t.Errorf("Approach A: expected is.workflow.actions.detect.dictionary")
	}
	if foundNativeDict < 2 {
		t.Errorf("Approach B: expected at least 2 native dictionary actions (let and var), found %d", foundNativeDict)
	}
	if !foundSetName {
		t.Errorf("Approach C: expected is.workflow.actions.setitemname with WFName='sample.json'")
	}
	if !foundCoercionD {
		t.Errorf("Approach D: expected WFCoercionVariableAggrandizement with WFDictionaryContentItem")
	}
}

func TestRuntimeFixtures_MenuInteractiveStructural(t *testing.T) {
	fixturePath := filepath.Join("tests", "runtime_poc", "CherriMenuRuntimePOC.cherri")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", fixturePath, err)
	}

	session := compileSourceToSession(t, "menu_poc", string(content))
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions

	var (
		foundMenuStart = false
		foundCaseAlpha = false
		foundCaseBeta  = false
		foundMenuEnd   = false
		startGroupID   = ""
	)

	for _, act := range actions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.choosefrommenu" {
			params := act.WFWorkflowActionParameters
			mode, _ := params["WFControlFlowMode"].(int)
			grp, _ := params["GroupingIdentifier"].(string)

			switch mode {
			case 0:
				foundMenuStart = true
				startGroupID = grp
				prompt, _ := params["WFMenuPrompt"].(string)
				if prompt != "Choose test branch" {
					t.Errorf("expected menu prompt 'Choose test branch', got %q", prompt)
				}
				itemsStr := fmt.Sprintf("%#v", params["WFMenuItems"])
				if !strings.Contains(itemsStr, "Alpha") || !strings.Contains(itemsStr, "Beta") {
					t.Errorf("expected WFMenuItems containing Alpha and Beta, got %s", itemsStr)
				}
			case 1:
				if grp != startGroupID {
					t.Errorf("menu case grouping ID %q does not match start %q", grp, startGroupID)
				}
				title, _ := params["WFMenuItemTitle"].(string)
				if title == "Alpha" {
					foundCaseAlpha = true
				} else if title == "Beta" {
					foundCaseBeta = true
				}
			case 2:
				foundMenuEnd = true
				if grp != startGroupID {
					t.Errorf("menu end grouping ID %q does not match start %q", grp, startGroupID)
				}
			}
		}
	}

	if !foundMenuStart {
		t.Errorf("expected choosefrommenu mode 0 (start)")
	}
	if !foundCaseAlpha {
		t.Errorf("expected choosefrommenu mode 1 (case Alpha)")
	}
	if !foundCaseBeta {
		t.Errorf("expected choosefrommenu mode 1 (case Beta)")
	}
	if !foundMenuEnd {
		t.Errorf("expected choosefrommenu mode 2 (end)")
	}
}

func TestRuntimeFixtures_LegacyIfStructural(t *testing.T) {
	fixturePath := filepath.Join("tests", "runtime_poc", "CherriLegacyIfPOC.cherri")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", fixturePath, err)
	}

	// Verify legacy compilation
	defer resetParser()
	resetParser()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "CherriLegacyIfPOC.cherri")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to write tmp file: %v", err)
	}

	origArgs := args.Args
	defer func() { args.Args = origArgs }()
	args.Args = map[string]string{
		"legacy":    "",
		"skip-sign": "",
		"no-ansi":   "",
	}
	currentTest = srcFile
	os.Args = []string{"cherri", srcFile}

	compile()

	var (
		foundIfStart = false
		foundIfElse  = false
		foundIfEnd   = false
		startGroupID = ""
	)

	for _, act := range shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
			params := act.WFWorkflowActionParameters
			mode, _ := params["WFControlFlowMode"].(int)
			grp, _ := params["GroupingIdentifier"].(string)

			switch mode {
			case 0:
				foundIfStart = true
				startGroupID = grp
				cond, _ := params["WFCondition"].(int)
				if cond != 4 { // Equals
					t.Errorf("expected WFCondition 4 (Equals), got %v", cond)
				}
				numVal, _ := params["WFNumberValue"].(int)
				if numVal != 5 {
					t.Errorf("expected WFNumberValue 5, got %v", numVal)
				}
			case 1:
				foundIfElse = true
				if grp != startGroupID {
					t.Errorf("if else grouping ID %q does not match start %q", grp, startGroupID)
				}
			case 2:
				foundIfEnd = true
				if grp != startGroupID {
					t.Errorf("if end grouping ID %q does not match start %q", grp, startGroupID)
				}
			}
		}
	}

	if !foundIfStart {
		t.Errorf("expected conditional mode 0 (start)")
	}
	if !foundIfElse {
		t.Errorf("expected conditional mode 1 (else)")
	}
	if !foundIfEnd {
		t.Errorf("expected conditional mode 2 (end)")
	}
}
