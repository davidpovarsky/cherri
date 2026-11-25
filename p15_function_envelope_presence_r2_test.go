package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

const p15FunctionProbeR2Path = "tools/shortcuts-simulator-qa/fixtures/qa-function-envelope-presence-v2-r2.cherri"

// This independently checks the legal sender fields needed by the public
// diagnostic. It is deliberately not a complete native workflow oracle.
func p15FunctionProbeR2Senders(actions []map[string]any) (map[string]int, error) {
	if err := p15SCReferences(actions); err != nil {
		return nil, err
	}
	positions := map[string]int{}
	for i, action := range actions {
		if id, ok := p15SCParams(action)["UUID"].(string); ok {
			positions[id] = i
		}
	}
	found := map[string]int{}
	rawReads := 0
	traceRunIDs := 0
	for i, action := range actions {
		p := p15SCParams(action)
		if token, okay := p["WFTextActionText"].(map[string]any); okay {
			if value, okay := token["Value"].(map[string]any); okay {
				if text, okay := value["string"].(string); okay && strings.Contains(text, "\"kind\":\"COLLECTOR_EVENT\"") {
					if strings.Count(text, "\"runId\":\"qa-function-envelope-presence-v2-r2\"") != 2 || strings.Count(text, "\"runId\":") != 2 {
						return nil, fmt.Errorf("R2 collector run identity changed")
					}
					traceRunIDs++
				}
			}
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.detect.dictionary" {
			ref, exact := p15FERef(p["WFInput"])
			if exact && reflect.DeepEqual(ref, map[string]any{"Type": "ExtensionInput"}) {
				rawReads++
			}
		}
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" || p["CustomOutputName"] != "FunctionPayload" {
			continue
		}
		fields, err := p15ContainerFields(p["WFItems"])
		if err != nil {
			return nil, err
		}
		name := ""
		for _, candidate := range []string{"probeZero", "probeDefault", "probeOptional"} {
			_, expected := p15ContainerFieldValue(candidate)
			if reflect.DeepEqual(fields["function"]["WFValue"], expected) {
				name = candidate
			}
		}
		if name == "" {
			return nil, fmt.Errorf("unexpected diagnostic function payload")
		}
		if _, duplicate := found[name]; duplicate {
			return nil, fmt.Errorf("duplicate diagnostic call")
		}
		found[name] = i
		expected := map[string]any{
			"_cherri_caller_input": p15ContainerReference(p15SCVariable("_cherri_caller_input")),
			"_cherri_magic":        "__cherri_fn_envelope_v2__", "cherri_functions": "1", "function": name,
			"arguments": []any{},
		}
		if name == "probeDefault" {
			expected["has_arg_0"] = "0"
			expected["arguments"] = []any{""}
		}
		if name == "probeOptional" {
			field := fields["arg_0"]
			if field == nil {
				return nil, fmt.Errorf("supplied Optional row absent")
			}
			ref, ok := p15FERef(field["WFValue"])
			id, idOkay := ref["OutputUUID"].(string)
			index, exists := positions[id]
			if !ok || !idOkay || ref["Type"] != "ActionOutput" || !exists || index >= i {
				return nil, fmt.Errorf("Optional argument lacks actual prior carrier")
			}
			carrier := actions[index]
			cp := p15SCParams(carrier)
			_, absent := p15ContainerFieldValue(map[string]any{"_cherri_optional_argument_v1": "1", "has_value": "0"})
			if carrier["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" ||
				cp["CustomOutputName"] != "OptionalValue" || !reflect.DeepEqual(cp["WFItems"], absent) {
				return nil, fmt.Errorf("supplied None lost its exact owned absent carrier")
			}
			argument := p15ContainerReference(map[string]any{"Type": "ActionOutput", "OutputUUID": id})
			expected["has_arg_0"] = "1"
			expected["arg_0"] = argument
			expected["arguments"] = []any{argument}
		}
		_, want := p15ContainerFieldValue(expected)
		if !reflect.DeepEqual(p15FunctionComparable(p["WFItems"], false), p15FunctionComparable(want, false)) {
			return nil, fmt.Errorf("diagnostic legal sender changed for %s", name)
		}
		id, okay := p["UUID"].(string)
		if !okay || len(p) != 3 || i+1 >= len(actions) {
			return nil, fmt.Errorf("diagnostic payload action shape")
		}
		call := actions[i+1]
		params := p15SCParams(call)
		expectedCall := map[string]any{
			"WFInput":        p15FunctionEdge(id),
			"WFWorkflowName": "CherriFunctionEnvelopePresenceProbeR2",
			"WFWorkflow":     map[string]any{"isSelf": true, "workflowName": "CherriFunctionEnvelopePresenceProbeR2"},
		}
		if call["WFWorkflowActionIdentifier"] != "is.workflow.actions.runworkflow" ||
			!reflect.DeepEqual(p15FunctionComparable(params, true), p15FunctionComparable(expectedCall, true)) {
			return nil, fmt.Errorf("diagnostic call target/input changed")
		}
	}
	if len(found) != 3 || rawReads != 5 || traceRunIDs != 1 {
		return nil, fmt.Errorf("diagnostic raw read/call inventory changed: %d/%d", rawReads, len(found))
	}
	return found, nil
}

// This compiles the R2 diagnostic under a distinct self-call and trace identity.
// Complete graph/trace validation and an explicit no-input Apple invocation
// are still prerequisites. It grants no RP26/BRG15 or native execution credit.
func TestP15FunctionEnvelopePresenceProbeR2Preparation(t *testing.T) {
	source, err := os.ReadFile(p15FunctionProbeR2Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(source)); got != "8c044ebef8e0ecd1481a99eb030ef843de2a9ddc4b4fb129545501d1bdb28be1" {
		t.Fatal("public diagnostic source changed", got)
	}
	raw, err := CompileSourceToPlist("CherriFunctionEnvelopePresenceProbeR2.cherri", string(source))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("FUNCTION_ENVELOPE_PRESENCE_R2_PREPARATION source_sha256=%x native_sha256=%x bytes=%d plist_base64=%s",
		sha256.Sum256(source), sha256.Sum256(raw), len(raw), base64.StdEncoding.EncodeToString(raw))
	var document map[string]any
	if _, err := plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	items, ok := document["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("native action array absent")
	}
	actions := make([]map[string]any, len(items))
	for i, item := range items {
		action, ok := item.(map[string]any)
		if !ok {
			t.Fatal("native action shape", i)
		}
		actions[i] = action
	}
	positions, err := p15FunctionProbeR2Senders(actions)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"caller_replaced", "magic_marker", "function_name", "default_flag", "optional_flag", "optional_presence", "optional_marker", "zero_mirror", "call_target", "call_edge", "raw_probe_input", "empty_value_deleted", "old_workflow_target", "trace_run_id"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15SCClone(t, actions)
			field := func(name, key string) map[string]any {
				fields, err := p15ContainerFields(p15SCParams(bad[positions[name]])["WFItems"])
				if err != nil {
					t.Fatal(err)
				}
				return fields[key]
			}
			switch fault {
			case "caller_replaced":
				field("probeZero", "_cherri_caller_input")["WFValue"] = p15ContainerText("REPLACED")
			case "magic_marker":
				field("probeZero", "_cherri_magic")["WFValue"] = p15ContainerText("WRONG")
			case "function_name":
				field("probeZero", "function")["WFValue"] = p15ContainerText("foreign")
			case "default_flag":
				field("probeDefault", "has_arg_0")["WFValue"] = p15ContainerText("1")
			case "optional_flag":
				field("probeOptional", "has_arg_0")["WFValue"] = p15ContainerText("0")
			case "optional_presence", "optional_marker":
				ref, _ := p15FERef(field("probeOptional", "arg_0")["WFValue"])
				for _, action := range bad {
					if p15SCParams(action)["UUID"] == ref["OutputUUID"] {
						fields, err := p15ContainerFields(p15SCParams(action)["WFItems"])
						if err != nil {
							t.Fatal(err)
						}
						key := "has_value"
						value := "1"
						if fault == "optional_marker" {
							key = "_cherri_optional_argument_v1"
							value = "0"
						}
						fields[key]["WFValue"] = p15ContainerText(value)
					}
				}
			case "zero_mirror":
				field("probeZero", "arguments")["WFValue"] = []any{}
			case "empty_value_deleted":
				delete(field("probeZero", "arguments")["WFValue"].(map[string]any), "Value")
			case "old_workflow_target":
				p := p15SCParams(bad[positions["probeZero"]+1])
				p["WFWorkflowName"] = "CherriFunctionEnvelopePresenceProbe"
				p["WFWorkflow"].(map[string]any)["workflowName"] = "CherriFunctionEnvelopePresenceProbe"
			case "trace_run_id":
				for _, action := range bad {
					if token, okay := p15SCParams(action)["WFTextActionText"].(map[string]any); okay {
						if value, okay := token["Value"].(map[string]any); okay {
							if text, okay := value["string"].(string); okay {
								value["string"] = strings.ReplaceAll(text, "qa-function-envelope-presence-v2-r2", "qa-function-envelope-presence-v2")
							}
						}
					}
				}
			case "call_target":
				p15SCParams(bad[positions["probeZero"]+1])["WFWorkflowName"] = "OtherWorkflow"
			case "call_edge":
				p15SCParams(bad[positions["probeZero"]+1])["WFInput"] = p15FunctionVariable("_cherri_caller_input")
			case "raw_probe_input":
				for _, action := range bad {
					p := p15SCParams(action)
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.detect.dictionary" {
						ref, exact := p15FERef(p["WFInput"])
						if exact && reflect.DeepEqual(ref, map[string]any{"Type": "ExtensionInput"}) {
							p["WFInput"] = p15FunctionVariable("_cherri_caller_input")
							break
						}
					}
				}
			}
			if _, err := p15FunctionProbeR2Senders(bad); err == nil {
				t.Fatal("sender observer accepted causal damage", fault)
			}
		})
	}
	if strings.Count(string(source), "if callerKey == 1 {") != 3 {
		t.Fatal("caller absent-key observation guard changed")
	}
	t.Log("Preparation only: authored row existence does not establish runtime key preservation or caller absence.")
}
