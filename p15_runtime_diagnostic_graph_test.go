package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

type p15DiagnosticGraph []map[string]any

func p15CompileDiagnosticGraph(t *testing.T, name string) (p15DiagnosticGraph, string) {
	t.Helper()
	path := "tools/shortcuts-simulator-qa/fixtures/" + name + ".cherri"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := CompileSourceToPlist(path, string(source))
	if err != nil {
		t.Fatal(err)
	}
	var workflow map[string]any
	if _, err = plist.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(workflow["WFWorkflowActions"])
	if err != nil {
		t.Fatal(err)
	}
	var actions p15DiagnosticGraph
	if err = json.Unmarshal(raw, &actions); err != nil {
		t.Fatal(err)
	}
	return actions, string(source)
}

func p15DiagnosticParams(actions p15DiagnosticGraph, index int) map[string]any {
	if index < 0 || index >= len(actions) {
		return nil
	}
	params, _ := actions[index]["WFWorkflowActionParameters"].(map[string]any)
	return params
}

func p15DiagnosticValue(value any) map[string]any {
	wrapper, _ := value.(map[string]any)
	if wrapper["WFSerializationType"] != "WFTextTokenAttachment" {
		return nil
	}
	result, _ := wrapper["Value"].(map[string]any)
	return result
}

func p15DiagnosticReferencesValid(actions p15DiagnosticGraph) error {
	producers := map[string]bool{}
	for i := range actions {
		if id, _ := p15DiagnosticParams(actions, i)["UUID"].(string); id != "" {
			if producers[id] {
				return fmt.Errorf("duplicate producer UUID")
			}
			producers[id] = true
		}
	}
	var walk func(any) error
	walk = func(value any) error {
		switch v := value.(type) {
		case map[string]any:
			if id, found := v["OutputUUID"]; found {
				text, ok := id.(string)
				if !ok || !producers[text] {
					return fmt.Errorf("dangling output reference")
				}
			}
			for _, item := range v {
				if err := walk(item); err != nil {
					return err
				}
			}
		case []any:
			for _, item := range v {
				if err := walk(item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, action := range actions {
		if err := walk(action); err != nil {
			return err
		}
	}
	return nil
}

func p15ValidateCollectEmptyDiagnostic(actions p15DiagnosticGraph) error {
	if len(actions) != 29 {
		return fmt.Errorf("unexpected action count: %d", len(actions))
	}
	if err := p15DiagnosticReferencesValid(actions); err != nil {
		return err
	}
	wantKinds := map[int]string{
		5: "list", 6: "repeat.each", 7: "setvariable", 8: "getvariable",
		9: "repeat.each", 10: "count", 11: "count", 12: "nothing",
		13: "setvariable", 14: "repeat.each", 15: "setvariable",
		16: "getvariable", 17: "repeat.each", 18: "count", 19: "list", 20: "count",
	}
	for i, kind := range wantKinds {
		if actions[i]["WFWorkflowActionIdentifier"] != "is.workflow.actions."+kind {
			return fmt.Errorf("wrong native operation at %d", i)
		}
	}
	p := func(i int) map[string]any { return p15DiagnosticParams(actions, i) }
	empty, ok := p(5)["WFItems"].([]any)
	if !ok || len(empty) != 0 {
		return fmt.Errorf("direct List must contain zero rows")
	}
	if p(6)["WFControlFlowMode"] != float64(0) || p(9)["WFControlFlowMode"] != float64(2) ||
		p(6)["GroupingIdentifier"] != p(9)["GroupingIdentifier"] ||
		p15DiagnosticValue(p(6)["WFInput"])["OutputUUID"] != p(5)["UUID"] {
		return fmt.Errorf("empty List Repeat edge changed")
	}
	for consumer, producer := range map[int]int{10: 9, 11: 5, 18: 17, 20: 19} {
		value := p15DiagnosticValue(p(consumer)["Input"])
		if value["Type"] != "ActionOutput" || value["OutputUUID"] != p(producer)["UUID"] {
			return fmt.Errorf("count %d no longer consumes producer %d", consumer, producer)
		}
		if _, coercion := value["Aggrandizements"]; coercion {
			return fmt.Errorf("count input gained a coercion")
		}
	}
	if len(p(12)) != 0 || !reflect.DeepEqual(p(13), map[string]any{"WFVariableName": "__cherri_qa_cleared_empty"}) {
		return fmt.Errorf("Nothing followed by implicit Set Variable clearing changed")
	}
	if p(14)["GroupingIdentifier"] != p(17)["GroupingIdentifier"] ||
		p(14)["WFControlFlowMode"] != float64(0) || p(17)["WFControlFlowMode"] != float64(2) ||
		p15DiagnosticValue(p(14)["WFInput"])["VariableName"] != "__cherri_qa_cleared_empty" ||
		p15DiagnosticValue(p(15)["WFInput"])["VariableName"] != "Repeat Item" ||
		p(15)["WFVariableName"] != "__cherri_qa_cleared_item" ||
		p15DiagnosticValue(p(16)["WFVariable"])["VariableName"] != "__cherri_qa_cleared_item" {
		return fmt.Errorf("cleared-variable Repeat flow changed")
	}
	positive, ok := p(19)["WFItems"].([]any)
	if !ok || len(positive) != 1 {
		return fmt.Errorf("positive empty-Text control must contain one row")
	}
	row, _ := positive[0].(map[string]any)
	if row["WFItemType"] != float64(0) || row["WFValue"] != "" {
		return fmt.Errorf("positive control is not one empty Text")
	}
	token, _ := p(26)["WFTextActionText"].(map[string]any)
	value, _ := token["Value"].(map[string]any)
	if token["WFSerializationType"] != "WFTextTokenString" ||
		value["string"] != "CHERRI_QA_COLLECT_EMPTY_SPLIT:direct=\uFFFC:repeat=\uFFFC:cleared=\uFFFC:emptyText=\uFFFC\n\uFFFC" {
		return fmt.Errorf("summary shape changed")
	}
	attachments, _ := value["attachmentsByRange"].(map[string]any)
	got := map[string]any{}
	for _, item := range attachments {
		ref, _ := item.(map[string]any)
		name, _ := ref["OutputName"].(string)
		got[name] = ref["OutputUUID"]
	}
	for name, index := range map[string]int{"directCount": 11, "collectedCount": 10, "clearedCount": 18, "emptyTextCount": 20} {
		if got[name] != p(index)["UUID"] {
			return fmt.Errorf("summary no longer reports independent %s", name)
		}
	}
	return nil
}

func TestP15RuntimeDiagnosticCollectEmptyGraph(t *testing.T) {
	actions, _ := p15CompileDiagnosticGraph(t, "qa-collect-empty-split-v2")
	if err := p15ValidateCollectEmptyDiagnostic(actions); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		edit func(p15DiagnosticGraph)
	}{
		{"missing empty rows", func(a p15DiagnosticGraph) { delete(p15DiagnosticParams(a, 5), "WFItems") }},
		{"empty becomes singleton", func(a p15DiagnosticGraph) {
			p15DiagnosticParams(a, 5)["WFItems"] = []any{map[string]any{"WFItemType": float64(0), "WFValue": ""}}
		}},
		{"direct count uses Repeat result", func(a p15DiagnosticGraph) {
			p15DiagnosticValue(p15DiagnosticParams(a, 11)["Input"])["OutputUUID"] = p15DiagnosticParams(a, 9)["UUID"]
		}},
		{"collected count uses input", func(a p15DiagnosticGraph) {
			p15DiagnosticValue(p15DiagnosticParams(a, 10)["Input"])["OutputUUID"] = p15DiagnosticParams(a, 5)["UUID"]
		}},
		{"clearing Nothing replaced", func(a p15DiagnosticGraph) { a[12]["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext" }},
		{"clear gains prior input", func(a p15DiagnosticGraph) {
			p15DiagnosticParams(a, 13)["WFInput"] = p15DiagnosticParams(a, 6)["WFInput"]
		}},
		{"cleared Count redirected", func(a p15DiagnosticGraph) {
			p15DiagnosticValue(p15DiagnosticParams(a, 18)["Input"])["OutputUUID"] = p15DiagnosticParams(a, 9)["UUID"]
		}},
		{"cleared Repeat group changed", func(a p15DiagnosticGraph) { p15DiagnosticParams(a, 17)["GroupingIdentifier"] = "wrong-group" }},
		{"singleton empty Text removed", func(a p15DiagnosticGraph) { p15DiagnosticParams(a, 19)["WFItems"] = []any{} }},
		{"empty List loses producer UUID", func(a p15DiagnosticGraph) { delete(p15DiagnosticParams(a, 5), "UUID") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, _ := json.Marshal(actions)
			var changed p15DiagnosticGraph
			if err := json.Unmarshal(raw, &changed); err != nil {
				t.Fatal(err)
			}
			test.edit(changed)
			if err := p15ValidateCollectEmptyDiagnostic(changed); err == nil {
				t.Fatal("causal native graph mutation accepted")
			}
		})
	}
}

func TestP15RuntimeDiagnosticFileBytesGraph(t *testing.T) {
	actions, source := p15CompileDiagnosticGraph(t, "qa-export-file-bytes-v2")
	if err := p15DiagnosticReferencesValid(actions); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(source, "getType(") || strings.Contains(source, "debugExport(") || strings.Contains(source, "getDictionary(") {
		t.Fatal("File bytes control gained an excluded operation")
	}
	decodeUUID, namedUUID := "", ""
	readers := 0
	for i, action := range actions {
		p := p15DiagnosticParams(actions, i)
		switch action["WFWorkflowActionIdentifier"] {
		case "is.workflow.actions.base64encode":
			if p["WFEncodeMode"] != "Decode" {
				t.Fatal("File source is not literal byte decoding")
			}
			decodeUUID, _ = p["UUID"].(string)
		case "is.workflow.actions.setitemname":
			if decodeUUID == "" || p15DiagnosticValue(p["WFInput"])["OutputUUID"] != decodeUUID {
				t.Fatal("Set Name does not consume decoded File")
			}
			namedUUID, _ = p["UUID"].(string)
		case "is.workflow.actions.getitemname", "is.workflow.actions.detect.text":
			if namedUUID == "" || p15DiagnosticValue(p["WFInput"])["OutputUUID"] != namedUUID {
				t.Fatal("File name/text readback does not consume named File")
			}
			readers++
		case "is.workflow.actions.gettypeaction", "is.workflow.actions.detect.dictionary":
			t.Fatal("File bytes control has an unreviewed conversion/parser")
		}
	}
	if decodeUUID == "" || namedUUID == "" || readers != 2 {
		t.Fatal("File bytes producer/readback controls are incomplete")
	}
}

func TestP15RuntimeDiagnosticIfResultLetVarSplit(t *testing.T) {
	letActions, letSource := p15CompileDiagnosticGraph(t, "qa-f01-ifresult-let-v2")
	varActions, varSource := p15CompileDiagnosticGraph(t, "qa-f01-ifresult-var-v2")
	normalized := strings.ReplaceAll(varSource, "qa-f01-ifresult-var-v2", "qa-f01-ifresult-let-v2")
	normalized = strings.Replace(normalized, "var controlValue:", "let controlValue:", 1)
	if normalized != letSource || len(varActions) != len(letActions)+1 {
		t.Fatal("If-result split changed more than the reviewed binding and trace identity")
	}
	for index, actions := range []p15DiagnosticGraph{letActions, varActions} {
		if err := p15DiagnosticReferencesValid(actions); err != nil {
			t.Fatal(err)
		}
		namedEnds, stores := 0, 0
		for i, action := range actions {
			p := p15DiagnosticParams(actions, i)
			switch action["WFWorkflowActionIdentifier"] {
			case "is.workflow.actions.conditional":
				if p["WFControlFlowMode"] == float64(2) && p["CustomOutputName"] == "controlValue" {
					namedEnds++
				}
			case "is.workflow.actions.setvariable":
				if p["WFVariableName"] == "controlValue" {
					stores++
				}
			case "is.workflow.actions.math", "is.workflow.actions.runworkflow":
				t.Fatal("If-result split gained Math or external Run")
			}
		}
		if index == 0 && (namedEnds != 1 || stores != 0) || index == 1 && (namedEnds != 0 || stores != 1) {
			t.Fatal("let/var native result paths no longer differ as intended")
		}
	}
}
