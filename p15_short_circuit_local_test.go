package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type p15LocalFrame struct {
	group  string
	branch int
}
type p15LocalGroup struct {
	start, alternate, end int
}

// This is a structural oracle over the emitted native action graph, not an Apple
// runtime emulator. It proves guarding, identity, update count and observability;
// signed execution must separately prove the native platform's behavior.
func p15LocalCounterGraph(actions []map[string]any) error {
	groups := map[string]*p15LocalGroup{}
	named := map[string]string{}
	paths := make([][]p15LocalFrame, len(actions))
	var stack []p15LocalFrame
	var mathCount, counterSets, verdictSets int
	allowed := map[any]bool{
		"is.workflow.actions.number": true, "is.workflow.actions.gettext": true,
		"is.workflow.actions.conditional": true, "is.workflow.actions.math": true,
		"is.workflow.actions.setvariable": true, "is.workflow.actions.output": true,
	}
	for i, action := range actions {
		if !allowed[action["WFWorkflowActionIdentifier"]] {
			return fmt.Errorf("unexpected external/action dependency at %d: %v", i, action["WFWorkflowActionIdentifier"])
		}
		p := p15Param(action)
		paths[i] = append([]p15LocalFrame(nil), stack...)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.math" {
			mathCount++
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "calls" {
			counterSets++
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "verdict" {
			verdictSets++
		}
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		id, _ := p["GroupingIdentifier"].(string)
		switch p15Int(p["WFControlFlowMode"]) {
		case 0:
			if id == "" || groups[id] != nil {
				return fmt.Errorf("invalid/reused group at %d", i)
			}
			if p["WFConditionalLegacyComparisonBehavior"] != true || p["WFConditions"] != nil {
				return fmt.Errorf("unexpected guard representation at %d", i)
			}
			groups[id] = &p15LocalGroup{start: i, alternate: -1, end: -1}
			stack = append(stack, p15LocalFrame{group: id})
		case 1:
			if len(stack) == 0 || stack[len(stack)-1].group != id || groups[id].alternate != -1 {
				return fmt.Errorf("invalid else at %d", i)
			}
			groups[id].alternate = i
			stack[len(stack)-1].branch = 1
		case 2:
			if len(stack) == 0 || stack[len(stack)-1].group != id || groups[id].end != -1 {
				return fmt.Errorf("invalid end at %d", i)
			}
			groups[id].end = i
			stack = stack[:len(stack)-1]
			if name, ok := p["CustomOutputName"].(string); ok {
				if named[name] != "" {
					return fmt.Errorf("duplicate named result %s", name)
				}
				named[name] = id
			}
		default:
			return fmt.Errorf("invalid control mode at %d", i)
		}
	}
	if len(stack) != 0 || mathCount != 6 || counterSets != 7 || verdictSets != 7 {
		return fmt.Errorf("unbalanced/excess/missing counter operations: groups=%d math=%d stores=%d", len(stack), mathCount, counterSets)
	}
	predicate := func(group string, variable string, code, number int) bool {
		g := groups[group]
		if g == nil || g.end < 0 {
			return false
		}
		p := p15Param(actions[g.start])
		a := p15LocalAttachment(p["WFInput"])
		return a["Type"] == "Variable" && a["VariableName"] == variable && p15Int(p["WFCondition"]) == code && p15Int(p["WFNumberValue"]) == number
	}
	numberAt := func(i, value int) bool {
		return i >= 0 && i < len(actions) && actions[i]["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" && p15Int(p15Param(actions[i])["WFNumberActionNumber"]) == value
	}
	for _, seed := range []struct {
		name, variable string
		value          int
	}{{"zero", "flag", 0}, {"one", "truth", 1}} {
		var producer string
		for i, a := range actions {
			if p15Param(a)["CustomOutputName"] == seed.name && numberAt(i, seed.value) {
				producer, _ = p15Param(a)["UUID"].(string)
			}
		}
		if producer == "" {
			return fmt.Errorf("missing action-derived %s", seed.name)
		}
		var matches int
		for i, a := range actions {
			p := p15Param(a)
			if a["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" || p["WFVariableName"] != seed.variable {
				continue
			}
			matches++
			out := p15LocalAttachment(p["WFInput"])["OutputUUID"]
			var matched bool
			for _, g := range groups {
				if p15Param(actions[g.end])["UUID"] != out {
					continue
				}
				s := p15Param(actions[g.start])
				matched = g.end < i && g.end == g.start+4 && g.alternate == g.start+2 && p15LocalAttachment(s["WFInput"])["OutputUUID"] == producer &&
					p15Int(s["WFCondition"]) == 4 && p15Int(s["WFNumberValue"]) == 1 && numberAt(g.start+1, 1) && numberAt(g.start+3, 0)
			}
			if !matched {
				return fmt.Errorf("%s is not the computed comparison of %s", seed.variable, seed.name)
			}
		}
		if matches != 1 {
			return fmt.Errorf("missing/duplicate seed binding %s", seed.variable)
		}
	}
	andID, orID, controlID := named["andValue"], named["orValue"], named["controlValue"]
	if !predicate(andID, "flag", 4, 1) || !predicate(orID, "truth", 4, 1) || !predicate(controlID, "truth", 4, 1) {
		return fmt.Errorf("missing exact AND/OR/control predicate")
	}
	andGroup, orGroup := groups[andID], groups[orID]
	if len(paths[andGroup.start]) != 0 || len(paths[orGroup.start]) != 0 || len(paths[groups[controlID].start]) != 0 {
		return fmt.Errorf("top-level experiment is unexpectedly guarded")
	}
	if !numberAt(andGroup.alternate+1, 0) || andGroup.end != andGroup.alternate+2 || !numberAt(orGroup.start+1, 1) || orGroup.alternate != orGroup.start+2 {
		return fmt.Errorf("wrong skipped-RHS Boolean result")
	}
	rhsGroup := func(outer string, branch int) (string, error) {
		var found string
		for id, g := range groups {
			if reflect.DeepEqual(paths[g.start], []p15LocalFrame{{group: outer, branch: branch}}) {
				if found != "" {
					return "", fmt.Errorf("multiple RHS groups")
				}
				found = id
			}
		}
		if found == "" || !predicate(found, "truth", 4, 1) {
			return "", fmt.Errorf("RHS outside expected branch %d", branch)
		}
		return found, nil
	}
	andRHS, err := rhsGroup(andID, 0)
	if err != nil {
		return err
	}
	orRHS, err := rhsGroup(orID, 1)
	if err != nil {
		return err
	}
	for _, id := range []string{andRHS, orRHS, controlID} {
		g := groups[id]
		var mathBranches = map[int]bool{}
		for i := g.start + 1; i < g.end; i++ {
			a := actions[i]
			p := p15Param(a)
			if a["WFWorkflowActionIdentifier"] != "is.workflow.actions.math" {
				continue
			}
			path := paths[i]
			if len(path) != len(paths[g.start])+1 || path[len(path)-1].group != id {
				return fmt.Errorf("counter update is not directly inside RHS")
			}
			branch := path[len(path)-1].branch
			if mathBranches[branch] || p["WFMathOperation"] != "+" || p15Int(p["WFMathOperand"]) != 1 || p15LocalAttachment(p["WFInput"])["VariableName"] != "calls" {
				return fmt.Errorf("wrong/duplicate counter update")
			}
			mathBranches[branch] = true
			if i+2 >= g.end || actions[i+1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" || p15Param(actions[i+1])["WFVariableName"] != "calls" ||
				p15LocalAttachment(p15Param(actions[i+1])["WFInput"])["OutputUUID"] != p["UUID"] || !reflect.DeepEqual(paths[i], paths[i+1]) || !numberAt(i+2, 1-branch) {
				return fmt.Errorf("counter store/yield identity mismatch")
			}
		}
		if len(mathBranches) != 2 {
			return fmt.Errorf("RHS must update once in each branch")
		}
	}
	// Initial counter producer must be zero, and its store must precede all RHS.
	initial := 0
	for i, a := range actions {
		p := p15Param(a)
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "calls" && len(paths[i]) == 0 {
			initial++
			if i == 0 || i >= andGroup.start || !numberAt(i-1, 0) || p15LocalAttachment(p["WFInput"])["OutputUUID"] != p15Param(actions[i-1])["UUID"] {
				return fmt.Errorf("wrong counter initialization")
			}
		}
	}
	if initial != 1 {
		return fmt.Errorf("counter initialization count %d", initial)
	}
	successInitial := 0
	for i, a := range actions {
		p := p15Param(a)
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "verdict" && len(paths[i]) == 0 {
			successInitial++
			if i == 0 || i >= andGroup.start || actions[i-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || p15Param(actions[i-1])["WFTextActionText"] != "P15_F01_LOCAL_OK" || p15LocalAttachment(p["WFInput"])["OutputUUID"] != p15Param(actions[i-1])["UUID"] {
				return fmt.Errorf("wrong success verdict initialization")
			}
		}
	}
	if successInitial != 1 {
		return fmt.Errorf("success initialization count %d", successInitial)
	}
	// Each observable bad result has its own exact native predicate. The control
	// count check occurs after the control body, so removing the positive control
	// or moving it into a skipped branch cannot silently pass.
	checks := map[string]struct {
		name         string
		variable     bool
		code, number int
	}{
		"P15_F01_LOCAL_AND_VALUE":     {"andValue", false, 4, 1},
		"P15_F01_LOCAL_AND_EAGER":     {"calls", true, 5, 0},
		"P15_F01_LOCAL_OR_VALUE":      {"orValue", false, 5, 1},
		"P15_F01_LOCAL_OR_EAGER":      {"calls", true, 5, 0},
		"P15_F01_LOCAL_CONTROL_VALUE": {"controlValue", false, 5, 1},
		"P15_F01_LOCAL_CONTROL_COUNT": {"calls", true, 5, 1},
	}
	seen := map[string]bool{}
	for i, a := range actions {
		if a["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
			continue
		}
		marker, _ := p15Param(a)["WFTextActionText"].(string)
		check, ok := checks[marker]
		if !ok {
			continue
		}
		if seen[marker] || len(paths[i]) != 1 || paths[i][0].branch != 0 {
			return fmt.Errorf("bad/duplicate observable check %s", marker)
		}
		seen[marker] = true
		g := groups[paths[i][0].group]
		p := p15Param(actions[g.start])
		attachment := p15LocalAttachment(p["WFInput"])
		validInput := attachment["Type"] == "Variable" && attachment["VariableName"] == check.name
		if !check.variable {
			validInput = attachment["Type"] == "ActionOutput" && attachment["OutputUUID"] == p15Param(actions[groups[named[check.name]].end])["UUID"]
		}
		if !validInput || p15Int(p["WFCondition"]) != check.code || p15Int(p["WFNumberValue"]) != check.number || i+1 >= len(actions) ||
			actions[i+1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" || p15Param(actions[i+1])["WFVariableName"] != "verdict" ||
			p15LocalAttachment(p15Param(actions[i+1])["WFInput"])["OutputUUID"] != p15Param(a)["UUID"] {
			return fmt.Errorf("check %s cannot observe its failure", marker)
		}
		if marker == "P15_F01_LOCAL_CONTROL_COUNT" && g.start < groups[controlID].end {
			return fmt.Errorf("positive control count checked too early")
		}
	}
	if len(seen) != len(checks) {
		return fmt.Errorf("missing failure observations")
	}
	if len(actions) < 2 {
		return fmt.Errorf("missing output")
	}
	result := p15Param(actions[len(actions)-2])
	token, _ := result["WFTextActionText"].(map[string]any)
	body, _ := token["Value"].(map[string]any)
	attachments, _ := body["attachmentsByRange"].(map[string]any)
	if token["WFSerializationType"] != "WFTextTokenString" || body["string"] != "\uFFFC:calls=\uFFFC" ||
		p15LocalAttachment(attachments["{0, 1}"])["VariableName"] != "verdict" || p15LocalAttachment(attachments["{8, 1}"])["VariableName"] != "calls" {
		return fmt.Errorf("output hides verdict/counter")
	}
	last := actions[len(actions)-1]
	output, _ := p15Param(last)["WFOutput"].(map[string]any)
	value, _ := output["Value"].(map[string]any)
	refs, _ := value["attachmentsByRange"].(map[string]any)
	if last["WFWorkflowActionIdentifier"] != "is.workflow.actions.output" || value["string"] != "\uFFFC" || p15LocalAttachment(refs["{0, 1}"])["OutputUUID"] != result["UUID"] {
		return fmt.Errorf("output does not return the observed result")
	}
	return nil
}

func p15LocalAttachment(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m["WFSerializationType"] == "WFTextTokenAttachment" {
		return p15LocalAttachment(m["Value"])
	}
	if child, ok := m["Variable"]; ok {
		return p15LocalAttachment(child)
	}
	return m
}

func TestP15F01LocalCounterFixtureNativeGraph(t *testing.T) {
	source, err := os.ReadFile("tests/language-v2/v2-short-circuit-local-runtime.cherri")
	if err != nil {
		t.Fatal(err)
	}
	actions := p15CompiledActions(t, string(source))
	if err := p15LocalCounterGraph(actions); err != nil {
		t.Fatal(err)
	}
	t.Logf("production native graph: %d actions; expected signed result P15_F01_LOCAL_OK:calls=1; runtime not asserted", len(actions))
}

func TestP15F01LocalCounterOracleRejectsMutations(t *testing.T) {
	source, err := os.ReadFile("tests/language-v2/v2-short-circuit-local-runtime.cherri")
	if err != nil {
		t.Fatal(err)
	}
	actions := p15CompiledActions(t, string(source))
	for _, mutation := range []string{"eager_rhs", "wrong_increment", "crosswired_store", "missing_control", "external_target", "hidden_counter", "wrong_success_marker"} {
		t.Run(mutation, func(t *testing.T) {
			raw, err := json.Marshal(actions)
			if err != nil {
				t.Fatal(err)
			}
			var broken []map[string]any
			if err = json.Unmarshal(raw, &broken); err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "eager_rhs":
				for i, a := range broken {
					if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.math" {
						broken = append([]map[string]any{a}, append(broken[:i], broken[i+1:]...)...)
						break
					}
				}
			case "wrong_increment":
				for _, a := range broken {
					if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.math" {
						p15Param(a)["WFMathOperand"] = float64(2)
						break
					}
				}
			case "crosswired_store":
				for i, a := range broken {
					if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.math" {
						p15LocalAttachment(p15Param(broken[i+1])["WFInput"])["OutputUUID"] = "OTHER-PRODUCER"
						break
					}
				}
			case "missing_control":
				for _, a := range broken {
					if p15Param(a)["CustomOutputName"] == "controlValue" {
						p15Param(a)["CustomOutputName"] = "removed"
						break
					}
				}
			case "external_target":
				broken = append([]map[string]any{{"WFWorkflowActionIdentifier": "is.workflow.actions.runworkflow", "WFWorkflowActionParameters": map[string]any{"WFWorkflowName": "MISSING"}}}, broken...)
			case "wrong_success_marker":
				for _, a := range broken {
					if p15Param(a)["WFTextActionText"] == "P15_F01_LOCAL_OK" {
						p15Param(a)["WFTextActionText"] = "WRONG"
						break
					}
				}
			case "hidden_counter":
				p := p15Param(broken[len(broken)-2])
				p["WFTextActionText"] = "P15_F01_LOCAL_OK:calls=1"
			}
			if err := p15LocalCounterGraph(broken); err == nil {
				t.Fatal("causal mutation falsely passed")
			}
		})
	}
}
