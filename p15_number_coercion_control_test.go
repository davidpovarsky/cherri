package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func p15CoercionProbeNamed(a p15DiagnosticGraph, name string) map[string]any {
	for i := range a {
		p := p15DiagnosticParams(a, i)
		if p["CustomOutputName"] == name {
			return p
		}
	}
	return nil
}

func p15CoercionProbeInput(p map[string]any) map[string]any {
	wrapper, _ := p["WFInput"].(map[string]any)
	if wrapper["Type"] != "Variable" {
		return nil
	}
	return p15DiagnosticValue(wrapper["Variable"])
}

func p15CoercionProbeConditions(a p15DiagnosticGraph) []int {
	var found []int
	direct := p15CoercionProbeNamed(a, "direct")
	required := p15CoercionProbeNamed(a, "required")
	one := p15CoercionProbeNamed(a, "one")
	for i, action := range a {
		p := p15DiagnosticParams(a, i)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" || p["WFControlFlowMode"] != float64(0) {
			continue
		}
		token := p15CoercionProbeInput(p)
		if token == nil {
			continue
		}
		if token["Type"] == "Variable" && token["VariableName"] == "bound" ||
			token["Type"] == "ActionOutput" && (token["OutputUUID"] == direct["UUID"] || token["OutputUUID"] == required["UUID"] || token["OutputUUID"] == one["UUID"]) {
			found = append(found, i)
		}
	}
	return found
}

func p15ValidateNumberCoercionProbe(a p15DiagnosticGraph) error {
	if err := p15DiagnosticReferencesValid(a); err != nil {
		return err
	}
	if len(a) != 89 {
		return fmt.Errorf("explicit coercion must not add native actions: got %d", len(a))
	}
	direct := p15CoercionProbeNamed(a, "direct")
	required := p15CoercionProbeNamed(a, "required")
	one := p15CoercionProbeNamed(a, "one")
	if direct == nil || required == nil || one == nil || direct["WFNumberActionNumber"] != float64(0) || one["WFNumberActionNumber"] != float64(1) {
		return fmt.Errorf("independent zero/one number producers changed")
	}
	forward := p15DiagnosticValue(required["WFVariable"])
	if forward == nil || forward["Type"] != "ActionOutput" || forward["OutputUUID"] == direct["UUID"] {
		return fmt.Errorf("require must retain its independent original number producer")
	}
	var original map[string]any
	var stores []map[string]any
	for i, action := range a {
		p := p15DiagnosticParams(a, i)
		if p["UUID"] == forward["OutputUUID"] {
			if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.number" {
				return fmt.Errorf("require forwarding target is not a number")
			}
			original = p
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "bound" {
			stores = append(stores, p)
		}
	}
	if original == nil || original["WFNumberActionNumber"] != float64(0) || len(stores) != 1 {
		return fmt.Errorf("required zero or unique bound store changed")
	}
	stored := p15DiagnosticValue(stores[0]["WFInput"])
	if stored == nil || stored["Type"] != "ActionOutput" || stored["OutputUUID"] != required["UUID"] {
		return fmt.Errorf("bound store bypasses the required result")
	}
	cases := []struct {
		source  string
		code    float64
		rhs     float64
		coerced bool
		flag    string
	}{
		{"direct", 4, 0, false, "directEq"},
		{"direct", 5, 0, false, "directNe"},
		{"required", 4, 0, true, "requiredEq"},
		{"required", 5, 0, true, "requiredNe"},
		{"bound", 4, 0, true, "boundEq"},
		{"bound", 5, 0, true, "boundNe"},
		{"one", 5, 0, false, "oneNeZero"},
		{"direct", 5, 1, false, "zeroNeOne"},
		{"one", 4, 1, false, "oneEqOne"},
	}
	indices := p15CoercionProbeConditions(a)
	if len(indices) != len(cases) {
		return fmt.Errorf("lost an independent numeric comparison: %d", len(indices))
	}
	for n, want := range cases {
		i := indices[n]
		p := p15DiagnosticParams(a, i)
		token := p15CoercionProbeInput(p)
		if p["WFCondition"] != want.code || p["WFNumberValue"] != want.rhs {
			return fmt.Errorf("comparison %d changed its operator or numeric value", n)
		}
		if want.source == "bound" {
			if token["Type"] != "Variable" || token["VariableName"] != "bound" {
				return fmt.Errorf("bound comparison bypasses the original variable")
			}
		} else if token["Type"] != "ActionOutput" || token["OutputUUID"] != p15CoercionProbeNamed(a, want.source)["UUID"] {
			return fmt.Errorf("comparison %d changed its producer", n)
		}
		ags, ok := token["Aggrandizements"].([]any)
		if _, exists := token["Aggrandizements"]; exists && !ok {
			return fmt.Errorf("malformed comparison coercion")
		}
		if want.coerced {
			if len(ags) != 1 {
				return fmt.Errorf("comparison %d needs exactly one explicit coercion", n)
			}
			ag, _ := ags[0].(map[string]any)
			if ag["Type"] != "WFCoercionVariableAggrandizement" || ag["CoercionItemClass"] != "WFNumberContentItem" {
				return fmt.Errorf("comparison %d lost its exact Number coercion", n)
			}
		} else if len(ags) != 0 {
			return fmt.Errorf("direct control gained an unreviewed coercion")
		}
		y := p15DiagnosticParams(a, i+1)
		set := p15DiagnosticParams(a, i+2)
		end := p15DiagnosticParams(a, i+3)
		if a[i+1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || y["WFTextActionText"] != "Y" ||
			a[i+2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" || set["WFVariableName"] != want.flag ||
			a[i+3]["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" || end["WFControlFlowMode"] != float64(2) || end["GroupingIdentifier"] != p["GroupingIdentifier"] {
			return fmt.Errorf("comparison %d lost its scoped observation", n)
		}
		observed := p15DiagnosticValue(set["WFInput"])
		if observed == nil || observed["OutputUUID"] != y["UUID"] {
			return fmt.Errorf("comparison %d does not store its actual branch result", n)
		}
	}
	verdict := p15CoercionProbeNamed(a, "verdict")
	text, _ := verdict["WFTextActionText"].(map[string]any)
	value, _ := text["Value"].(map[string]any)
	attachments, _ := value["attachmentsByRange"].(map[string]any)
	if text["WFSerializationType"] != "WFTextTokenString" || len(attachments) != 9 {
		return fmt.Errorf("verdict must read all nine observed flags")
	}
	flags := map[string]int{}
	for _, attachment := range attachments {
		token, _ := attachment.(map[string]any)
		if token["Type"] != "Variable" {
			return fmt.Errorf("flag readback is not the original variable")
		}
		name, _ := token["VariableName"].(string)
		flags[name]++
	}
	for _, want := range cases {
		if flags[want.flag] != 1 {
			return fmt.Errorf("verdict flag %s is missing or duplicated", want.flag)
		}
	}
	final := p15CoercionProbeNamed(a, "finalSummary")
	text, _ = final["WFTextActionText"].(map[string]any)
	value, _ = text["Value"].(map[string]any)
	attachments, _ = value["attachmentsByRange"].(map[string]any)
	if value["string"] != "CHERRI_QA_NUMBER_COERCION_RESULT:\uFFFC\n\uFFFC" || len(attachments) != 2 {
		return fmt.Errorf("final result must carry actual verdict and trace")
	}
	var verdictSeen bool
	for _, attachment := range attachments {
		token, _ := attachment.(map[string]any)
		if token["OutputUUID"] == verdict["UUID"] {
			verdictSeen = true
		}
	}
	if !verdictSeen {
		return fmt.Errorf("final output bypasses observed verdict")
	}
	return nil
}

func TestP15NumberCoercionControlGraph(t *testing.T) {
	actions, source := p15CompileDiagnosticGraph(t, "qa-number-coercion-control-v2")
	if err := p15ValidateNumberCoercionProbe(actions); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("docs/apple-runtime/p10r15/number-coercion-control.json")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err = json.Unmarshal(manifest, &data); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != data["sourceSha256"] {
		t.Fatal("reviewed diagnostic source changed")
	}
	original, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/qa-number-comparison-diag-v2.cherri")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(original)) != "b3d1bcf9782d8210b3f1756d6e0d104f02bcf2ea46db50593bc6e45b8a2cfe99" {
		t.Fatal("original failing diagnostic changed")
	}
}

func TestP15NumberCoercionControlRejectsGraphFaults(t *testing.T) {
	actions, _ := p15CompileDiagnosticGraph(t, "qa-number-coercion-control-v2")
	faults := []struct {
		name   string
		change func(p15DiagnosticGraph)
	}{
		{"missing-required-coercion", func(a p15DiagnosticGraph) {
			delete(p15CoercionProbeInput(p15DiagnosticParams(a, p15CoercionProbeConditions(a)[2])), "Aggrandizements")
		}},
		{"missing-bound-coercion", func(a p15DiagnosticGraph) {
			delete(p15CoercionProbeInput(p15DiagnosticParams(a, p15CoercionProbeConditions(a)[4])), "Aggrandizements")
		}},
		{"wrong-content-class", func(a p15DiagnosticGraph) {
			p15CoercionProbeInput(p15DiagnosticParams(a, p15CoercionProbeConditions(a)[2]))["Aggrandizements"].([]any)[0].(map[string]any)["CoercionItemClass"] = "WFStringContentItem"
		}},
		{"different-required-producer", func(a p15DiagnosticGraph) {
			p15CoercionProbeInput(p15DiagnosticParams(a, p15CoercionProbeConditions(a)[2]))["OutputUUID"] = p15CoercionProbeNamed(a, "direct")["UUID"]
		}},
		{"wrong-inequality-code", func(a p15DiagnosticGraph) {
			p15DiagnosticParams(a, p15CoercionProbeConditions(a)[3])["WFCondition"] = float64(4)
		}},
		{"wrong-zero-operand", func(a p15DiagnosticGraph) {
			p15DiagnosticParams(a, p15CoercionProbeConditions(a)[3])["WFNumberValue"] = float64(1)
		}},
		{"lost-zero-operand", func(a p15DiagnosticGraph) {
			delete(p15DiagnosticParams(a, p15CoercionProbeConditions(a)[3]), "WFNumberValue")
		}},
		{"wrong-scoped-flag", func(a p15DiagnosticGraph) {
			p15DiagnosticParams(a, p15CoercionProbeConditions(a)[3]+2)["WFVariableName"] = "requiredEq"
		}},
		{"moved-branch-end", func(a p15DiagnosticGraph) {
			p15DiagnosticParams(a, p15CoercionProbeConditions(a)[3]+3)["GroupingIdentifier"] = "wrong"
		}},
		{"constant-verdict", func(a p15DiagnosticGraph) {
			p15CoercionProbeNamed(a, "verdict")["WFTextActionText"] = "all flags correct"
		}},
		{"constant-final-output", func(a p15DiagnosticGraph) {
			p15CoercionProbeNamed(a, "finalSummary")["WFTextActionText"] = "CHERRI_QA_NUMBER_COERCION_RESULT:DEQ=Y:DNE=N:REQ=Y:RNE=N:BEQ=Y:BNE=N:ONE_NE_ZERO=Y:ZERO_NE_ONE=Y:ONE_EQ_ONE=Y"
		}},
		{"wrong-direct-zero", func(a p15DiagnosticGraph) { p15CoercionProbeNamed(a, "direct")["WFNumberActionNumber"] = float64(1) }},
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			raw, _ := json.Marshal(actions)
			var mutated p15DiagnosticGraph
			if err := json.Unmarshal(raw, &mutated); err != nil {
				t.Fatal(err)
			}
			fault.change(mutated)
			if err := p15ValidateNumberCoercionProbe(mutated); err == nil {
				t.Fatal("causal diagnostic graph fault accepted")
			}
		})
	}
}
