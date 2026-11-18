package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func p15FileNameActions(actions p15DiagnosticGraph, identifier string) []map[string]any {
	var found []map[string]any
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions."+identifier {
			p, _ := action["WFWorkflowActionParameters"].(map[string]any)
			found = append(found, p)
		}
	}
	return found
}

func p15FileNameAttachment(params map[string]any) (map[string]any, error) {
	input, ok := params["WFInput"].(map[string]any)
	if !ok || input["WFSerializationType"] != "WFTextTokenAttachment" {
		return nil, fmt.Errorf("File readback requires a native attachment")
	}
	value, ok := input["Value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("File attachment has no value")
	}
	return value, nil
}

func p15ValidateFileNameBindingGraph(actions p15DiagnosticGraph) error {
	if err := p15DiagnosticReferencesValid(actions); err != nil {
		return err
	}
	decodes := p15FileNameActions(actions, "base64encode")
	names := p15FileNameActions(actions, "setitemname")
	readers := p15FileNameActions(actions, "getitemname")
	texts := p15FileNameActions(actions, "detect.text")
	if len(decodes) != 4 || len(names) != 4 || len(readers) != 3 || len(texts) != 3 {
		return fmt.Errorf("binding probe lost an independent producer or readback")
	}
	for i, named := range names {
		if decodes[i]["WFEncodeMode"] != "Decode" || named["WFName"] != "cherri-export-control.json" {
			return fmt.Errorf("literal bytes or exact requested file name changed")
		}
		input, err := p15FileNameAttachment(named)
		if err != nil || input["Type"] != "ActionOutput" || input["OutputUUID"] != decodes[i]["UUID"] {
			return fmt.Errorf("Set Name %d does not use its independent decoded source", i)
		}
		if i == 0 && named["CustomOutputName"] != "letNamed" {
			return fmt.Errorf("let-bound control lost its named result")
		}
		if i != 0 {
			if _, present := named["CustomOutputName"]; present {
				return fmt.Errorf("var/inline control %d gained named result %v", i, named["CustomOutputName"])
			}
		}
	}
	var variableStores []map[string]any
	for _, p := range p15FileNameActions(actions, "setvariable") {
		if p["WFVariableName"] == "varNamed" {
			variableStores = append(variableStores, p)
		}
	}
	if len(variableStores) != 1 {
		return fmt.Errorf("var control lost its single store")
	}
	stored, err := p15FileNameAttachment(variableStores[0])
	if err != nil || stored["Type"] != "ActionOutput" || stored["OutputUUID"] != names[1]["UUID"] {
		return fmt.Errorf("var store does not preserve the second Set Name result")
	}
	for _, check := range []struct {
		params   map[string]any
		source   int
		variable bool
	}{
		{readers[0], 0, false}, {texts[0], 0, false},
		{readers[1], 1, true}, {texts[1], 1, true},
		{readers[2], 2, false}, {texts[2], 3, false},
	} {
		input, err := p15FileNameAttachment(check.params)
		if err != nil {
			return err
		}
		if check.variable {
			if input["Type"] != "Variable" || input["VariableName"] != "varNamed" {
				return fmt.Errorf("var readback bypasses the original variable")
			}
		} else if input["Type"] != "ActionOutput" || input["OutputUUID"] != names[check.source]["UUID"] {
			return fmt.Errorf("File readback consumes a different named result")
		}
	}
	return nil
}

func TestP15RuntimeDiagnosticFileNameBindingGraph(t *testing.T) {
	actions, source := p15CompileDiagnosticGraph(t, "qa-file-name-binding-v2")
	if err := p15ValidateFileNameBindingGraph(actions); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("docs/apple-runtime/p10r15/file-name-binding-probe.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SourceSHA256 string `json:"sourceSha256"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(source))
	if hex.EncodeToString(digest[:]) != manifest.SourceSHA256 {
		t.Fatal("reviewed source hash changed")
	}
	original, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/qa-export-file-bytes-v2.cherri")
	if err != nil {
		t.Fatal(err)
	}
	digest = sha256.Sum256(original)
	if hex.EncodeToString(digest[:]) != "4f095910b9af28d6016cd23ba06ad845f884e17941303edde4d1497bc5a144b2" {
		t.Fatal("original failing File fixture was changed")
	}
	faults := []struct {
		name   string
		change func(p15DiagnosticGraph)
	}{
		{"wrong-file-name", func(a p15DiagnosticGraph) { p15FileNameActions(a, "setitemname")[0]["WFName"] = "different.json" }},
		{"let-loses-result-name", func(a p15DiagnosticGraph) { delete(p15FileNameActions(a, "setitemname")[0], "CustomOutputName") }},
		{"var-gains-result-name", func(a p15DiagnosticGraph) { p15FileNameActions(a, "setitemname")[1]["CustomOutputName"] = "varNamed" }},
		{"inline-gains-result-name", func(a p15DiagnosticGraph) {
			p15FileNameActions(a, "setitemname")[2]["CustomOutputName"] = "inlineNamed"
		}},
		{"decoder-changed", func(a p15DiagnosticGraph) { p15FileNameActions(a, "base64encode")[0]["WFEncodeMode"] = "Encode" }},
		{"name-readback-redirected", func(a p15DiagnosticGraph) {
			p15FileNameActions(a, "getitemname")[0]["WFInput"] = p15FileNameActions(a, "getitemname")[2]["WFInput"]
		}},
		{"text-readback-redirected", func(a p15DiagnosticGraph) {
			p15FileNameActions(a, "detect.text")[0]["WFInput"] = p15FileNameActions(a, "detect.text")[2]["WFInput"]
		}},
		{"var-readback-bypassed", func(a p15DiagnosticGraph) {
			p15FileNameActions(a, "getitemname")[1]["WFInput"] = p15FileNameActions(a, "getitemname")[0]["WFInput"]
		}},
		{"var-store-input-removed", func(a p15DiagnosticGraph) {
			for _, p := range p15FileNameActions(a, "setvariable") {
				if p["WFVariableName"] == "varNamed" {
					delete(p, "WFInput")
				}
			}
		}},
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			raw, _ := json.Marshal(actions)
			var mutated p15DiagnosticGraph
			if err := json.Unmarshal(raw, &mutated); err != nil {
				t.Fatal(err)
			}
			fault.change(mutated)
			if err := p15ValidateFileNameBindingGraph(mutated); err == nil {
				t.Fatal("causal File graph corruption accepted")
			}
		})
	}
}
