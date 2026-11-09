package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func compileV2Helper(t *testing.T, name, code string) (*CanonicalBackendSession, error) {
	t.Helper()
	file := source.NewFile(source.DocumentID(name), name, 1, code)
	p := syntax.NewParser(file)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("%v", p.Errors()[0])
	}

	reg := schema.DefaultRegistry()
	analyzer := analysis.NewAnalyzer(reg)
	analyzer.Analyze(prog)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Severity == analysis.SeverityError {
			return nil, fmt.Errorf("%s", diag.Message)
		}
	}

	session := NewCanonicalBackendSession()
	lowerer := lower.NewLowererWithSession(reg, session)
	lowerer.WorkflowName = name
	if err := lowerer.LowerProgramToSession(prog); err != nil {
		session.Close()
		return nil, err
	}
	return session, nil
}

// 1. Task 1219750185341254: run(server, payload) must retain WFInput
func TestP10_RunActionPreservesWFInput(t *testing.T) {
	v2Code := `let payload = "data"
run("General Context Server", input: payload)
`
	session, err := compileV2Helper(t, "test_run_v2", v2Code)
	if err != nil {
		t.Fatalf("v2 compile failed: %v", err)
	}
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions
	var runAct *ShortcutAction
	for _, act := range actions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.runworkflow" {
			runAct = &act
			break
		}
	}
	if runAct == nil {
		t.Fatalf("expected runworkflow action not found")
	}

	params := runAct.WFWorkflowActionParameters
	if params["WFWorkflowName"] != "General Context Server" {
		t.Errorf("expected WFWorkflowName == 'General Context Server', got %v", params["WFWorkflowName"])
	}

	if params["WFInput"] == nil {
		t.Errorf("CRITICAL BUG REGRESSION: WFInput parameter is nil/missing in run action")
	}

	wfMap, ok := params["WFWorkflow"].(map[string]any)
	if !ok {
		t.Fatalf("expected WFWorkflow dictionary in run action parameters, got %T", params["WFWorkflow"])
	}
	if wfMap["workflowName"] != "General Context Server" {
		t.Errorf("expected WFWorkflow.workflowName == 'General Context Server', got %v", wfMap["workflowName"])
	}
}

// 2. Task 1219750250211915: statistic action must emit WFInput, not Input
func TestP10_StatisticActionEmitsWFInputNotInput(t *testing.T) {
	v2Code := `let nums = [1, 2, 3]
statistic("Sum", input: nums)
`
	session, err := compileV2Helper(t, "test_statistic_v2", v2Code)
	if err != nil {
		t.Fatalf("v2 compile failed: %v", err)
	}
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions
	var statAct *ShortcutAction
	for _, act := range actions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.statistics" {
			statAct = &act
			break
		}
	}
	if statAct == nil {
		t.Fatalf("expected is.workflow.actions.statistics action not found")
	}

	params := statAct.WFWorkflowActionParameters
	if params["WFInput"] == nil {
		t.Errorf("CRITICAL: statistic action missing WFInput wire key; params: %v", params)
	}
	if params["Input"] != nil {
		t.Errorf("CRITICAL: statistic action incorrectly emitted 'Input' wire key instead of 'WFInput'")
	}
	if params["WFStatisticsOperation"] != "Sum" {
		t.Errorf("expected WFStatisticsOperation == 'Sum', got %v", params["WFStatisticsOperation"])
	}
}

// 3. Task 1219750434313969 (B): if matches == [] array comparison must fail closed
func TestP10_ArrayEqualityComparison_FailsClosed(t *testing.T) {
	v2Code := `let matches = ["apple", "banana"]
if matches == [] {
    alert(alert: "Empty")
}
`
	_, err := compileV2Helper(t, "test_array_cond", v2Code)
	if err == nil {
		t.Fatalf("expected compilation to fail for array == [] comparison, but succeeded")
	}
	if !strings.Contains(err.Error(), "Invalid type 'array' for conditional '=='") {
		t.Fatalf("expected diagnostic 'Invalid type 'array' for conditional '=='' but got: %v", err)
	}
}

// 4. Task 1219750434313969 (C): regex quantifiers {8}, {4}, {12} must not turn into variable tokens
func TestP10_RegexQuantifiersPreserved(t *testing.T) {
	v2Code := `let pattern = "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}"
matchText("[0-9a-fA-F]{8}-[0-9a-fA-F]{4}", text: pattern)
`
	session, err := compileV2Helper(t, "test_regex", v2Code)
	if err != nil {
		t.Fatalf("v2 compile failed: %v", err)
	}
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions
	var textAct *ShortcutAction
	var matchAct *ShortcutAction
	for i := range actions {
		act := &actions[i]
		switch act.WFWorkflowActionIdentifier {
		case "is.workflow.actions.gettext":
			textAct = act
		case "is.workflow.actions.text.match":
			matchAct = act
		}
	}
	if textAct == nil {
		t.Fatalf("expected gettext action not found")
	}
	if matchAct == nil {
		t.Fatalf("expected text.match action not found")
	}

	assertUnadulteratedRegex := func(label string, rawVal any) {
		switch v := rawVal.(type) {
		case string:
			if v != "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}" {
				t.Fatalf("CRITICAL BUG (%s): regex pattern string corrupted: got %q", label, v)
			}
		case map[string]any:
			if ser, hasSer := v["WFSerializationType"]; hasSer && ser == "WFTextTokenString" {
				val := v["Value"].(map[string]any)
				if s, ok := val["string"].(string); ok && s != "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}" {
					t.Fatalf("CRITICAL BUG (%s): regex WFTextTokenString corrupted: got %q", label, s)
				}
				if atts, hasAtts := val["attachmentsByRange"].(map[string]any); hasAtts && len(atts) > 0 {
					t.Fatalf("CRITICAL BUG (%s): regex quantifier turned into variable attachments: %v", label, atts)
				}
			}
		case WFTextTokenString:
			if v.Value.String != "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}" {
				t.Fatalf("CRITICAL BUG (%s): regex WFTextTokenString struct corrupted: got %q", label, v.Value.String)
			}
			if len(v.Value.AttachmentsByRange) > 0 {
				t.Fatalf("CRITICAL BUG (%s): regex quantifier turned into variable attachments: %v", label, v.Value.AttachmentsByRange)
			}
		default:
			t.Fatalf("unexpected type for %s: %T (%v)", label, rawVal, rawVal)
		}
	}

	assertUnadulteratedRegex("gettext.WFTextActionText", textAct.WFWorkflowActionParameters["WFTextActionText"])
	assertUnadulteratedRegex("matchText.WFMatchTextPattern", matchAct.WFWorkflowActionParameters["WFMatchTextPattern"])
}

// 5. Task 1219800392994574: JSON braces in string must not be parsed as inline variables
func TestP10_JSONBracesInStringNotParsedAsInlineVars(t *testing.T) {
	resetParser()
	loadStandardActions()

	// Double quotes with JSON braces
	strWithJSON := `{"model":"test"}`
	checkInlineVars(&strWithJSON) // Must NOT throw parserError

	emptyJSON := `{}`
	checkInlineVars(&emptyJSON) // Must NOT throw parserError

	spacedJSON := `{ }`
	checkInlineVars(&spacedJSON) // Must NOT throw parserError
}
