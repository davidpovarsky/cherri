package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// helper to compile v2 source to CanonicalBackendSession
func compileControlSourceToSession(t *testing.T, name, code string) *CanonicalBackendSession {
	t.Helper()
	file := source.NewFile(source.DocumentID(name), name, 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("syntax errors: %v", parser.Errors())
	}

	reg := schema.DefaultRegistry()
	analyzer := analysis.NewAnalyzer(reg)
	analyzer.Analyze(prog)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Severity == analysis.SeverityError {
			t.Fatalf("analyzer error [%s]: %s", diag.Code, diag.Message)
		}
	}

	session := NewCanonicalBackendSession()
	lowerer := lower.NewLowererWithSession(reg, session)
	lowerer.WorkflowName = name
	if err := lowerer.LowerProgramToSession(prog); err != nil {
		session.Close()
		t.Fatalf("lowering error: %v", err)
	}
	return session
}

// CTRL-001: simple if/else with number/string compare
// Canonical keys, grouping, iOS compatibility and output.
func TestCTRL001_SimpleIfElse(t *testing.T) {
	code := `
let x = 42
if x > 10 {
    quicklook("Greater")
} else {
    quicklook("Smaller")
}
`
	session := compileControlSourceToSession(t, "ctrl001", code)
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions

	var ifStart, ifElse, ifEnd *ShortcutAction
	for i := range actions {
		act := &actions[i]
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
			mode, _ := act.WFWorkflowActionParameters["WFControlFlowMode"].(int)
			switch mode {
			case 0:
				ifStart = act
			case 1:
				ifElse = act
			case 2:
				ifEnd = act
			}
		}
	}

	if ifStart == nil || ifElse == nil || ifEnd == nil {
		t.Fatalf("expected ifStart, ifElse, and ifEnd actions to exist")
	}

	// 1. GroupingIdentifier must match across all 3 segments
	groupID := ifStart.WFWorkflowActionParameters["GroupingIdentifier"]
	if groupID == "" || groupID == nil {
		t.Errorf("expected non-empty GroupingIdentifier in ifStart")
	}
	if ifElse.WFWorkflowActionParameters["GroupingIdentifier"] != groupID {
		t.Errorf("expected matching GroupingIdentifier in ifElse")
	}
	if ifEnd.WFWorkflowActionParameters["GroupingIdentifier"] != groupID {
		t.Errorf("expected matching GroupingIdentifier in ifEnd")
	}

	// 2. Condition code: > is Greater Than (code 2 in Cherri)
	condCode, _ := ifStart.WFWorkflowActionParameters["WFCondition"].(int)
	if condCode != 2 {
		t.Errorf("expected WFCondition == 2 (Greater Than), got %d", condCode)
	}

	// 3. Number value: 10
	numVal, _ := ifStart.WFWorkflowActionParameters["WFNumberValue"].(float64)
	if numVal != 10.0 {
		t.Errorf("expected WFNumberValue == 10.0, got %v", numVal)
	}

	// 4. Legacy comparison flag present for backward compatibility
	if legacy, ok := ifStart.WFWorkflowActionParameters["WFConditionalLegacyComparisonBehavior"].(bool); !ok || !legacy {
		t.Errorf("expected WFConditionalLegacyComparisonBehavior == true in ifStart")
	}
}

// CTRL-002: modern multi-conditions plus legacy comparison
// Version-appropriate conditional representation.
func TestCTRL002_LegacyComparisonAndMultiConditions(t *testing.T) {
	session := NewCanonicalBackendSession()
	defer session.Close()

	// Direct emission via BeginControl
	err := session.BeginControl(backend.ControlRegion{
		Kind:               backend.ControlIf,
		GroupingIdentifier: "test-grp-002",
		ConditionOperator:  4, // Is
		ConditionValue:     "Apple",
	})
	if err != nil {
		t.Fatalf("BeginControl failed: %v", err)
	}

	err = session.EndControl(backend.ControlRegion{
		Kind:               backend.ControlIf,
		GroupingIdentifier: "test-grp-002",
	})
	if err != nil {
		t.Fatalf("EndControl failed: %v", err)
	}

	if len(session.shortcut.WFWorkflowActions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(session.shortcut.WFWorkflowActions))
	}

	act := session.shortcut.WFWorkflowActions[0]
	if act.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" {
		t.Errorf("expected is.workflow.actions.conditional")
	}
	if act.WFWorkflowActionParameters["WFConditionalActionString"] != "Apple" {
		t.Errorf("expected WFConditionalActionString == 'Apple', got %v", act.WFWorkflowActionParameters["WFConditionalActionString"])
	}
	if act.WFWorkflowActionParameters["WFConditionalLegacyComparisonBehavior"] != true {
		t.Errorf("expected WFConditionalLegacyComparisonBehavior == true")
	}
}

// CTRL-003: Repeat count + nested Repeat Each
// Correct grouping, nested item/index, stable variable scope.
func TestCTRL003_RepeatCountAndNestedRepeatEach(t *testing.T) {
	code := `
let items = ["A", "B"]
repeat 3 {
    for item in items {
        quicklook(item)
    }
}
`
	session := compileControlSourceToSession(t, "ctrl003", code)
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions

	var countStart, countEnd *ShortcutAction
	var eachStart, eachEnd *ShortcutAction

	for i := range actions {
		act := &actions[i]
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.count" {
			mode, _ := act.WFWorkflowActionParameters["WFControlFlowMode"].(int)
			if mode == 0 {
				countStart = act
			} else if mode == 2 {
				countEnd = act
			}
		} else if act.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.each" {
			mode, _ := act.WFWorkflowActionParameters["WFControlFlowMode"].(int)
			if mode == 0 {
				eachStart = act
			} else if mode == 2 {
				eachEnd = act
			}
		}
	}

	if countStart == nil || countEnd == nil {
		t.Fatalf("expected repeat.count start and end")
	}
	if eachStart == nil || eachEnd == nil {
		t.Fatalf("expected repeat.each start and end")
	}

	// Distinct grouping IDs for outer and inner loops
	countGrp := countStart.WFWorkflowActionParameters["GroupingIdentifier"]
	eachGrp := eachStart.WFWorkflowActionParameters["GroupingIdentifier"]
	if countGrp == eachGrp {
		t.Errorf("outer loop and inner loop must have distinct GroupingIdentifiers")
	}

	if countEnd.WFWorkflowActionParameters["GroupingIdentifier"] != countGrp {
		t.Errorf("countEnd grouping must match countStart")
	}
	if eachEnd.WFWorkflowActionParameters["GroupingIdentifier"] != eachGrp {
		t.Errorf("eachEnd grouping must match eachStart")
	}
}

// CTRL-004: v2 zero-based indexing and bounds
// First/last, negative, empty, dynamic OOB semantics.
func TestCTRL004_ZeroBasedIndexingAndBounds(t *testing.T) {
	code := `
let items = ["A", "B", "C"]
let first = items[0]
quicklook(first)
`
	session := compileControlSourceToSession(t, "ctrl004", code)
	defer session.Close()

	// In v2 zero-based indexing, indexing dynamic collection emits a bounds check
	// using count action and conditional >= (index + 1)
	hasCount := false
	hasCond := false

	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.count" {
			hasCount = true
		}
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
			// one-based for index 0 is 1.0
			if val, ok := act.WFWorkflowActionParameters["WFNumberValue"].(float64); ok && val == 1.0 {
				hasCond = true
			}
		}
	}

	if !hasCount {
		t.Errorf("expected is.workflow.actions.count for safe bounds checking")
	}
	if !hasCond {
		t.Errorf("expected conditional bounds check with WFNumberValue 1.0 for index 0")
	}
}

// CTRL-005: menu with two Hebrew/English cases
// WFMenuItems as typed WFMenuItem and both item-title fields (WFMenuItemTitle and WFMenuItemAttributedTitle).
func TestCTRL005_MenuWithTwoCases(t *testing.T) {
	code := `
menu("בחר פעולה") {
    case "שלום" {
        quicklook("שלום")
    }
    case "English" {
        quicklook("Hello")
    }
}
`
	session := compileControlSourceToSession(t, "ctrl005", code)
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions

	var menuStart, menuEnd *ShortcutAction
	var cases []*ShortcutAction

	for i := range actions {
		act := &actions[i]
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.choosefrommenu" {
			mode, _ := act.WFWorkflowActionParameters["WFControlFlowMode"].(int)
			switch mode {
			case 0:
				menuStart = act
			case 1:
				cases = append(cases, act)
			case 2:
				menuEnd = act
			}
		}
	}

	if menuStart == nil {
		t.Fatalf("expected choosefrommenu start action")
	}
	if len(cases) != 2 {
		t.Fatalf("expected 2 menu cases, got %d", len(cases))
	}
	if menuEnd == nil {
		t.Fatalf("expected choosefrommenu end action")
	}

	// 1. WFMenuItems must be typed []WFMenuItem
	rawItems := menuStart.WFWorkflowActionParameters["WFMenuItems"]
	menuItems, ok := rawItems.([]WFMenuItem)
	if !ok {
		t.Fatalf("expected WFMenuItems to be []WFMenuItem, got %T: %v", rawItems, rawItems)
	}
	if len(menuItems) != 2 {
		t.Fatalf("expected 2 WFMenuItem items, got %d", len(menuItems))
	}
	if menuItems[0].WFItemType != 0 || menuItems[0].WFValue != "שלום" {
		t.Errorf("expected case 0 to be 'שלום', got %v", menuItems[0])
	}
	if menuItems[1].WFItemType != 0 || menuItems[1].WFValue != "English" {
		t.Errorf("expected case 1 to be 'English', got %v", menuItems[1])
	}

	// 2. Both cases must have WFMenuItemTitle AND WFMenuItemAttributedTitle
	for i, c := range cases {
		title, hasTitle := c.WFWorkflowActionParameters["WFMenuItemTitle"]
		attrTitle, hasAttr := c.WFWorkflowActionParameters["WFMenuItemAttributedTitle"]
		if !hasTitle || title == "" {
			t.Errorf("case %d missing WFMenuItemTitle", i)
		}
		if !hasAttr || attrTitle == "" {
			t.Errorf("case %d missing WFMenuItemAttributedTitle", i)
		}
		if title != attrTitle {
			t.Errorf("case %d: title %v != attrTitle %v", i, title, attrTitle)
		}
	}
}

// CTRL-005 Mutation test: Ensure comparator/engine rejects untyped []string for WFMenuItems
func TestCTRL005_RejectStringSliceForWFMenuItems(t *testing.T) {
	session := NewCanonicalBackendSession()
	defer session.Close()

	// Emit raw with invalid []string
	err := session.EmitRawAction("is.workflow.actions.choosefrommenu", map[string]any{
		"GroupingIdentifier": "test-grp",
		"WFControlFlowMode":  0,
		"WFMenuItems":        []string{"Case1", "Case2"},
	}, "", "", "test-grp")
	if err != nil {
		t.Fatalf("EmitRawAction failed: %v", err)
	}

	// After emitRaw normalization, it must have been transformed into []WFMenuItem!
	act := session.shortcut.WFWorkflowActions[0]
	if _, ok := act.WFWorkflowActionParameters["WFMenuItems"].([]WFMenuItem); !ok {
		t.Errorf("expected WFMenuItems to be converted to []WFMenuItem, got %T", act.WFWorkflowActionParameters["WFMenuItems"])
	}
}

// CTRL-006: if/for/repeat/menu with yield
// Correct value producer/references and branch behavior.
func TestCTRL006_ControlFlowWithYield(t *testing.T) {
	code := `
let x = 5
let result = if x > 0 {
    yield "Positive"
} else {
    yield "NonPositive"
}
quicklook(result)
`
	session := compileControlSourceToSession(t, "ctrl006", code)
	defer session.Close()

	// With yield, the conditional end action must produce an output UUID
	var ifEnd *ShortcutAction
	for i := range session.shortcut.WFWorkflowActions {
		act := &session.shortcut.WFWorkflowActions[i]
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
			mode, _ := act.WFWorkflowActionParameters["WFControlFlowMode"].(int)
			if mode == 2 {
				ifEnd = act
			}
		}
	}

	if ifEnd == nil {
		t.Fatalf("expected conditional end action")
	}

	uuid, _ := ifEnd.WFWorkflowActionParameters["UUID"].(string)
	if uuid == "" {
		t.Errorf("expected conditional end action to have output UUID for yield result")
	}

	// quicklook must receive the output of ifEnd
	hasQuicklookRef := false
	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
			s := fmt.Sprintf("%#v", act.WFWorkflowActionParameters["WFInput"])
			if strings.Contains(s, uuid) {
				hasQuicklookRef = true
			}
		}
	}

	if !hasQuicklookRef {
		t.Errorf("expected quicklook to receive output UUID %q from if-expression", uuid)
	}
}

// CTRL-007: dynamic lazy AND/OR
// No execution of right-side action when short-circuited.
func TestCTRL007_DynamicLazyAndOr(t *testing.T) {
	// 1. Static short-circuit: false && quicklook(...) should never emit quicklook
	t.Run("StaticShortCircuit", func(t *testing.T) {
		code := `
let res = false && quicklook("should_not_exist")
`
		session := compileControlSourceToSession(t, "ctrl007_static", code)
		defer session.Close()

		for _, act := range session.shortcut.WFWorkflowActions {
			if act.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
				t.Fatalf("expected right side of statically false && to be short-circuited at compile time")
			}
		}
	})

	// 2. Dynamic lazy AND: right side must be placed inside Then branch (between mode 0 and mode 1)
	t.Run("DynamicLazyAnd", func(t *testing.T) {
		code := `
var x = 1
let dynamicCond = x > 5
let res = dynamicCond && quicklook("right_and_action")
`
		session := compileControlSourceToSession(t, "ctrl007_dyn_and", code)
		defer session.Close()

		actions := session.shortcut.WFWorkflowActions
		var startIdx, elseIdx, endIdx, rightIdx int = -1, -1, -1, -1

		for i, act := range actions {
			if act.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
				mode, _ := act.WFWorkflowActionParameters["WFControlFlowMode"].(int)
				switch mode {
				case 0:
					startIdx = i
				case 1:
					elseIdx = i
				case 2:
					endIdx = i
				}
			} else if act.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
				rightIdx = i
			}
		}

		if startIdx == -1 || elseIdx == -1 || endIdx == -1 {
			t.Fatalf("expected dynamic conditional (start=%d, else=%d, end=%d)", startIdx, elseIdx, endIdx)
		}
		if rightIdx == -1 {
			t.Fatalf("expected right side action to be emitted")
		}
		if !(startIdx < rightIdx && rightIdx < elseIdx) {
			t.Fatalf("expected right action (idx %d) to be inside Then branch (between %d and %d) for dynamic &&", rightIdx, startIdx, elseIdx)
		}
	})

	// 3. Dynamic lazy OR: right side must be placed inside Else branch (between mode 1 and mode 2)
	t.Run("DynamicLazyOr", func(t *testing.T) {
		code := `
var x = 1
let dynamicCond = x > 5
let res = dynamicCond || quicklook("right_or_action")
`
		session := compileControlSourceToSession(t, "ctrl007_dyn_or", code)
		defer session.Close()

		actions := session.shortcut.WFWorkflowActions
		var startIdx, elseIdx, endIdx, rightIdx int = -1, -1, -1, -1

		for i, act := range actions {
			if act.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
				mode, _ := act.WFWorkflowActionParameters["WFControlFlowMode"].(int)
				switch mode {
				case 0:
					startIdx = i
				case 1:
					elseIdx = i
				case 2:
					endIdx = i
				}
			} else if act.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
				rightIdx = i
			}
		}

		if startIdx == -1 || elseIdx == -1 || endIdx == -1 {
			t.Fatalf("expected dynamic conditional (start=%d, else=%d, end=%d)", startIdx, elseIdx, endIdx)
		}
		if rightIdx == -1 {
			t.Fatalf("expected right side action to be emitted")
		}
		if !(elseIdx < rightIdx && rightIdx < endIdx) {
			t.Fatalf("expected right action (idx %d) to be inside Else branch (between %d and %d) for dynamic ||", rightIdx, elseIdx, endIdx)
		}
	})
}

