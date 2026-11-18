package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"
)

// The diagnostic must observe native counts from the actual explicit-nil
// assignments. It accepts the current omitted-input form and a later explicit
// clear form, but reports that distinction; neither is a native success claim.
func p15NilClearProbeGraph(actions p15DiagnosticGraph) (map[string]string, error) {
	if err := p15DiagnosticReferencesValid(actions); err != nil {
		return nil, err
	}
	names := map[string]int{}
	observations := map[string]bool{"bindingCount": true, "assignmentCount": true, "controlCount": true, "presentCount": true, "traceEnvelope": true, "finalSummary": true}
	for i := range actions {
		if name, _ := p15DiagnosticParams(actions, i)["CustomOutputName"].(string); observations[name] {
			if _, duplicate := names[name]; duplicate {
				return nil, fmt.Errorf("duplicate named observation %s", name)
			}
			names[name] = i
		}
	}
	parameters := func(i int) map[string]any { return p15DiagnosticParams(actions, i) }
	forms := map[string]string{}
	for output, variable := range map[string]string{
		"bindingCount": "nilBinding", "assignmentCount": "nilAssignment",
		"controlCount": "__cherri_qa_nil_clear_control", "presentCount": "presentText",
	} {
		index, found := names[output]
		if !found || actions[index]["WFWorkflowActionIdentifier"] != "is.workflow.actions.count" {
			return nil, fmt.Errorf("%s is not a native Count observation", output)
		}
		token := p15DiagnosticValue(parameters(index)["Input"])
		if token["Type"] != "Variable" || token["VariableName"] != variable || token["Aggrandizements"] != nil {
			return nil, fmt.Errorf("%s does not count its independent variable", output)
		}
		store := -1
		var stores []int
		for j := 0; j < index; j++ {
			if parameters(j)["WFVariableName"] == variable {
				if actions[j]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" {
					return nil, fmt.Errorf("%s has an unexpected variable mutation", variable)
				}
				store = j
				stores = append(stores, j)
			}
		}
		if store < 1 {
			return nil, fmt.Errorf("%s has no actual assignment", variable)
		}
		wantStores := 1
		if output == "assignmentCount" {
			wantStores = 2
		}
		if len(stores) != wantStores {
			return nil, fmt.Errorf("%s has %d stores; expected %d", variable, len(stores), wantStores)
		}
		if output == "assignmentCount" {
			initial := stores[0]
			if initial < 1 || initial >= store-1 {
				return nil, fmt.Errorf("nil reassignment lost the earlier initialized value")
			}
			initialInput := p15DiagnosticValue(parameters(initial)["WFInput"])
			initialValue := parameters(initial - 1)
			if actions[initial-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" ||
				initialValue["WFTextActionText"] != "P15_NIL_OLD_VALUE" ||
				initialInput["Type"] != "ActionOutput" || initialInput["OutputUUID"] != initialValue["UUID"] ||
				initialInput["Aggrandizements"] != nil {
				return nil, fmt.Errorf("nil reassignment did not start from the exact nonempty producer")
			}
		}
		if output == "presentCount" {
			input := p15DiagnosticValue(parameters(store)["WFInput"])
			prior := parameters(store - 1)
			if actions[store-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" ||
				prior["WFTextActionText"] != "P15_NIL_PRESENT_VALUE" ||
				input["Type"] != "ActionOutput" || input["OutputUUID"] != prior["UUID"] || input["Aggrandizements"] != nil {
				return nil, fmt.Errorf("present Text positive control changed")
			}
			continue
		}
		if _, present := parameters(store)["WFInput"]; present {
			return nil, fmt.Errorf("%s is not an absent-input Set", variable)
		}
		seed := store - 1
		forms[output] = "OMITTED_INPUT_WITHOUT_NOTHING"
		if actions[seed]["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" {
			if len(parameters(seed)) != 0 {
				return nil, fmt.Errorf("Nothing acquired parameters")
			}
			forms[output] = "EXPLICIT_NOTHING_THEN_SET"
			seed--
		}
		wantSeed := map[string]string{
			"bindingCount":    "P15_NIL_PRIOR_BINDING",
			"assignmentCount": "P15_NIL_PRIOR_ASSIGNMENT",
			"controlCount":    "P15_NIL_PRIOR_CONTROL",
		}[output]
		if seed < 0 || actions[seed]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" ||
			parameters(seed)["WFTextActionText"] != wantSeed {
			return nil, fmt.Errorf("%s lost its immediately preceding nonempty seed", variable)
		}
		if output == "controlCount" && forms[output] != "EXPLICIT_NOTHING_THEN_SET" {
			return nil, fmt.Errorf("independent Legacy clear control lost Nothing")
		}
	}
	summary, found := names["finalSummary"]
	if !found {
		return nil, fmt.Errorf("missing observable summary")
	}
	wrapper, _ := parameters(summary)["WFTextActionText"].(map[string]any)
	value, _ := wrapper["Value"].(map[string]any)
	want := "CHERRI_QA_NIL_CLEAR:binding=\uFFFC:assignment=\uFFFC:control=\uFFFC:present=\uFFFC\n\uFFFC"
	if wrapper["WFSerializationType"] != "WFTextTokenString" || value["string"] != want {
		return nil, fmt.Errorf("summary was hard-coded or its independent fields changed")
	}
	attachments, _ := value["attachmentsByRange"].(map[string]any)
	if len(attachments) != 5 {
		return nil, fmt.Errorf("summary lost a count or trace attachment")
	}
	expectedNames := []string{"bindingCount", "assignmentCount", "controlCount", "presentCount", "traceEnvelope"}
	next := 0
	for offset, unit := range utf16.Encode([]rune(want)) {
		if unit != '\uFFFC' {
			continue
		}
		token, _ := attachments[fmt.Sprintf("{%d, 1}", offset)].(map[string]any)
		name := expectedNames[next]
		producer, found := names[name]
		if !found || token["Type"] != "ActionOutput" || token["OutputUUID"] != parameters(producer)["UUID"] ||
			token["OutputName"] != name || token["Aggrandizements"] != nil {
			return nil, fmt.Errorf("summary field %s has the wrong native producer", name)
		}
		next++
	}
	return forms, nil
}

func TestP15NilClearDiagnosticNativeGraph(t *testing.T) {
	actions, source := p15CompileDiagnosticGraph(t, "qa-nil-clear-v2")
	for _, required := range []string{"var nilBinding: Text? = nil", "nilAssignment = nil", "var presentText: Text? =", `debugExportTrace(runId: "qa-nil-clear-v2")`} {
		if !strings.Contains(source, required) {
			t.Fatalf("source lost the required diagnostic %q", required)
		}
	}
	forms, err := p15NilClearProbeGraph(actions)
	if err != nil {
		t.Fatal(err)
	}
	report, _ := json.Marshal(map[string]any{"nativeActionCount": len(actions), "nilForms": forms, "runtimeObserved": false})
	t.Log("P15_NIL_CLEAR_PROBE:" + string(report))
}

func TestP15NilClearDiagnosticRejectsFalseObservations(t *testing.T) {
	good, _ := p15CompileDiagnosticGraph(t, "qa-nil-clear-v2")
	if _, err := p15NilClearProbeGraph(good); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing_seed", "wrong_count_variable", "constant_count", "no_control_nothing", "control_inherits_input", "present_lost_input", "hardcoded_summary", "swapped_summary_fields", "missing_count_uuid", "removed_initial_store", "retargeted_initial_store"} {
		t.Run(fault, func(t *testing.T) {
			raw, _ := json.Marshal(good)
			var actions p15DiagnosticGraph
			if err := json.Unmarshal(raw, &actions); err != nil {
				t.Fatal(err)
			}
			if fault == "removed_initial_store" || fault == "retargeted_initial_store" {
				for i := range actions {
					p := p15DiagnosticParams(actions, i)
					if p["WFVariableName"] == "nilAssignment" && p["WFInput"] != nil {
						if fault == "removed_initial_store" {
							actions = append(actions[:i], actions[i+1:]...)
						} else {
							p["WFVariableName"] = "__wrong_initial_variable"
						}
						break
					}
				}
			}
			for i, a := range actions {
				p := p15DiagnosticParams(actions, i)
				switch {
				case fault == "missing_seed" && p["WFTextActionText"] == "P15_NIL_PRIOR_BINDING":
					p["WFTextActionText"] = ""
				case fault == "wrong_count_variable" && p["CustomOutputName"] == "bindingCount":
					p15DiagnosticValue(p["Input"])["VariableName"] = "__cherri_qa_nil_clear_control"
				case fault == "constant_count" && p["CustomOutputName"] == "assignmentCount":
					a["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
					p["WFNumberActionNumber"] = 0
				case p["WFVariableName"] == "__cherri_qa_nil_clear_control":
					if fault == "no_control_nothing" {
						actions[i-1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
					}
					if fault == "control_inherits_input" {
						for j := range actions {
							if p15DiagnosticParams(actions, j)["CustomOutputName"] == "assignmentCount" {
								p["WFInput"] = p15AcceptanceRef(p15DiagnosticParams(actions, j)["UUID"].(string), nil)
							}
						}
					}
				case fault == "present_lost_input" && p["WFVariableName"] == "presentText":
					delete(p, "WFInput")
				case fault == "missing_count_uuid" && p["CustomOutputName"] == "controlCount":
					delete(p, "UUID")
				case p["CustomOutputName"] == "finalSummary":
					if fault == "hardcoded_summary" {
						p["WFTextActionText"] = "CHERRI_QA_NIL_CLEAR:binding=0:assignment=0:control=0:present=1"
					}
					if fault == "swapped_summary_fields" {
						fields := p["WFTextActionText"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)
						keys := map[string]string{}
						for key, value := range fields {
							name, _ := value.(map[string]any)["OutputName"].(string)
							keys[name] = key
						}
						a, b := keys["bindingCount"], keys["assignmentCount"]
						fields[a], fields[b] = fields[b], fields[a]
					}
				}
			}
			if _, err := p15NilClearProbeGraph(actions); err == nil {
				t.Fatalf("accepted false observation %s", fault)
			}
		})
	}
}
