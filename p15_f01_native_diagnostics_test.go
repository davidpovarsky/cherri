package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type p15F01DiagContract struct {
	actions, math, groups, counterStores int
	observedVariable                     string
}

var p15F01DiagContracts = map[string]p15F01DiagContract{
	"counter":   {18, 1, 0, 2, "calls"},
	"predicate": {38, 0, 4, 0, "verdict"},
	"control":   {42, 2, 4, 3, "calls"},
	"logic":     {65, 0, 10, 0, "verdict"},
}

func p15F01DiagWalk(value any, visit func(map[string]any)) {
	switch value := value.(type) {
	case map[string]any:
		visit(value)
		for _, child := range value {
			p15F01DiagWalk(child, visit)
		}
	case []any:
		for _, child := range value {
			p15F01DiagWalk(child, visit)
		}
	}
}

func p15F01DiagnosticShape(name string, actions []ShortcutAction) error {
	contract, ok := p15F01DiagContracts[name]
	if !ok || len(actions) != contract.actions {
		return fmt.Errorf("wrong diagnostic shape/name %s", name)
	}
	allowed := map[string]bool{"comment": true, "gettext": true, "number": true, "setvariable": true, "appendvariable": true, "conditional": true, "math": true, "text.combine": true, "output": true}
	uuids := map[string]int{}
	for i, action := range actions {
		if id, _ := action.WFWorkflowActionParameters["UUID"].(string); id != "" {
			if _, duplicate := uuids[id]; duplicate {
				return fmt.Errorf("duplicate producer")
			}
			uuids[id] = i
		}
	}
	starts := map[string]int{}
	named := map[string]string{}
	paths := map[string][]p15LocalFrame{}
	var stack []p15LocalFrame
	var mathCount, stores int
	semanticUUID, finalUUID := "", ""
	observed := false
	for i, action := range actions {
		short := strings.TrimPrefix(action.WFWorkflowActionIdentifier, "is.workflow.actions.")
		if !allowed[short] {
			return fmt.Errorf("external/unreviewed action %s", short)
		}
		p := action.WFWorkflowActionParameters
		var referenceError error
		p15F01DiagWalk(p, func(value map[string]any) {
			if id, ok := value["OutputUUID"].(string); ok {
				producer, exists := uuids[id]
				if !exists || producer >= i {
					referenceError = fmt.Errorf("missing/forward producer at %d", i)
				}
			}
		})
		if referenceError != nil {
			return referenceError
		}
		if short == "math" {
			mathCount++
			input := p15LocalAttachment(p["WFInput"])
			if input["Type"] != "Variable" || input["VariableName"] != "calls" || p["WFMathOperation"] != "+" || p15Int(p["WFMathOperand"]) != 1 {
				return fmt.Errorf("counter is not one update of calls")
			}
			if i+1 >= len(actions) {
				return fmt.Errorf("missing counter store")
			}
			next := actions[i+1]
			if next.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" || next.WFWorkflowActionParameters["WFVariableName"] != "calls" || requireOutputUUID(next.WFWorkflowActionParameters["WFInput"]) != p["UUID"] {
				return fmt.Errorf("counter update lost producer identity")
			}
			if name == "counter" && len(stack) != 0 {
				return fmt.Errorf("standalone counter gained control dependency")
			}
			if name == "control" && len(stack) == 0 {
				return fmt.Errorf("ordinary conditional counter escaped its branch")
			}
		}
		if short == "setvariable" && p["WFVariableName"] == "calls" {
			stores++
		}
		if short == "conditional" {
			group, _ := p["GroupingIdentifier"].(string)
			switch p15Int(p["WFControlFlowMode"]) {
			case 0:
				if group == "" {
					return fmt.Errorf("missing group")
				}
				if _, duplicate := starts[group]; duplicate {
					return fmt.Errorf("reused group")
				}
				if p["WFConditionalLegacyComparisonBehavior"] != true {
					return fmt.Errorf("unexpected native comparison contract")
				}
				starts[group] = i
				paths[group] = append([]p15LocalFrame(nil), stack...)
				stack = append(stack, p15LocalFrame{group: group})
			case 1:
				if len(stack) == 0 || stack[len(stack)-1].group != group || stack[len(stack)-1].branch != 0 {
					return fmt.Errorf("invalid alternate")
				}
				stack[len(stack)-1].branch = 1
			case 2:
				if len(stack) == 0 || stack[len(stack)-1].group != group {
					return fmt.Errorf("unbalanced end")
				}
				if result, ok := p["CustomOutputName"].(string); ok {
					named[result] = group
				}
				stack = stack[:len(stack)-1]
			default:
				return fmt.Errorf("invalid mode")
			}
		}
		if short == "gettext" && p["CustomOutputName"] == "semanticResult" {
			semanticUUID, _ = p["UUID"].(string)
			p15F01DiagWalk(p["WFTextActionText"], func(token map[string]any) {
				if token["Type"] == "Variable" && token["VariableName"] == contract.observedVariable {
					observed = true
				}
			})
		}
		if short == "gettext" && p["CustomOutputName"] == "finalSummary" {
			finalUUID, _ = p["UUID"].(string)
			hasSemantic := false
			p15F01DiagWalk(p["WFTextActionText"], func(token map[string]any) {
				if token["OutputUUID"] == semanticUUID && semanticUUID != "" {
					hasSemantic = true
				}
			})
			if !hasSemantic {
				return fmt.Errorf("callback marker detached from semantic result")
			}
		}
	}
	if len(stack) != 0 || len(starts) != contract.groups || mathCount != contract.math || stores != contract.counterStores || !observed || semanticUUID == "" || finalUUID == "" {
		return fmt.Errorf("lost isolation/result: groups=%d math=%d stores=%d observed=%t", len(starts), mathCount, stores, observed)
	}
	output := actions[len(actions)-1]
	returnedReferences := 0
	returnedFinal := false
	p15F01DiagWalk(output.WFWorkflowActionParameters["WFOutput"], func(token map[string]any) {
		if id, reference := token["OutputUUID"].(string); reference {
			returnedReferences++
			returnedFinal = id == finalUUID
		}
	})
	if output.WFWorkflowActionIdentifier != "is.workflow.actions.output" || returnedReferences != 1 || !returnedFinal {
		return fmt.Errorf("final result not returned")
	}
	if name == "logic" {
		for _, check := range []struct {
			result, variable string
			branch           int
		}{{"andValue", "flag", 0}, {"orValue", "truth", 1}} {
			group := named[check.result]
			start, exists := starts[group]
			if !exists {
				return fmt.Errorf("missing logical result")
			}
			p := actions[start].WFWorkflowActionParameters
			if p15LocalAttachment(p["WFInput"])["VariableName"] != check.variable || p15Int(p["WFCondition"]) != 4 || p15Int(p["WFNumberValue"]) != 1 {
				return fmt.Errorf("wrong logical guard")
			}
			children := 0
			for child, path := range paths {
				if len(path) > 0 && path[len(path)-1].group == group {
					if path[len(path)-1].branch != check.branch || p15LocalAttachment(actions[starts[child]].WFWorkflowActionParameters["WFInput"])["VariableName"] != "truth" {
						return fmt.Errorf("RHS no longer in its required guard branch")
					}
					children++
				}
			}
			if children != 1 {
				return fmt.Errorf("missing/extra logical RHS")
			}
		}
	}
	return nil
}

func TestP15F01DiagnosticFixturesPreserveIsolation(t *testing.T) {
	original, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/qa-short-circuit-local-v2.cherri")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(original)
	if hex.EncodeToString(sum[:]) != "9e55dc6bb913b8cbb06d2ce8cdd167dcc6a0697ba7347949b260d8e2cc768a82" {
		t.Fatal("failed signed fixture was changed")
	}
	for name := range p15F01DiagContracts {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/qa-f01-diag-" + name + "-v2.cherri")
			if err != nil {
				t.Fatal(err)
			}
			if err := p15F01DiagnosticShape(name, compileDebugFixture(t, string(src), false)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F01DiagnosticOracleRejectsCausalMutations(t *testing.T) {
	for _, mutation := range []string{"wrong_increment", "wrong_counter_store", "hidden_counter", "external_action", "wrong_and_guard"} {
		name := "counter"
		if mutation == "wrong_and_guard" {
			name = "logic"
		}
		src, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/qa-f01-diag-" + name + "-v2.cherri")
		if err != nil {
			t.Fatal(err)
		}
		actions := compileDebugFixture(t, string(src), false)
		for i := range actions {
			p := actions[i].WFWorkflowActionParameters
			switch mutation {
			case "wrong_increment":
				if actions[i].WFWorkflowActionIdentifier == "is.workflow.actions.math" {
					p["WFMathOperand"] = 2
				}
			case "wrong_counter_store":
				if actions[i].WFWorkflowActionIdentifier == "is.workflow.actions.math" {
					actions[i+1].WFWorkflowActionParameters["WFInput"] = map[string]any{"Value": map[string]any{"Type": "ActionOutput", "OutputUUID": "missing"}}
				}
			case "hidden_counter":
				if p["CustomOutputName"] == "semanticResult" {
					p["WFTextActionText"] = "calls=1"
				}
			case "external_action":
				if i == 0 {
					actions[i].WFWorkflowActionIdentifier = "is.workflow.actions.runworkflow"
				}
			case "wrong_and_guard":
				if actions[i].WFWorkflowActionIdentifier == "is.workflow.actions.conditional" && p15LocalAttachment(p["WFInput"])["VariableName"] == "flag" {
					p["WFCondition"] = 5
				}
			}
		}
		if p15F01DiagnosticShape(name, actions) == nil {
			raw, _ := json.Marshal(actions)
			t.Fatalf("%s mutation passed: %s", mutation, raw)
		}
	}
}
