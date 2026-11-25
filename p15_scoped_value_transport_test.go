package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type p15ScopedTransportCase struct {
	name, source string
	check        func([]map[string]any) error
}

// These are the existing, independently specified body graphs. Wrapping a
// body does not authorize a different payload codec or a different consumer.
func p15ScopedTransportCases() []p15ScopedTransportCase {
	cases := []p15ScopedTransportCase{
		{"nested_list", "let values:List<List<Optional<Bool>>>=[[false]]\nreturn count(values)", func(a []map[string]any) error { return p15TaggedContextualGraph(a, 2, false) }},
		{"deep_list", "let values:List<List<List<Optional<Bool>>>>=[[[false]]]\nreturn count(values)", func(a []map[string]any) error { return p15TaggedContextualGraph(a, 3, false) }},
		{"dictionary_array", "let values:Map<Text,List<Optional<Bool>>>={\"slot\":[false]}\nreturn 1", func(a []map[string]any) error { return p15TaggedContextualGraph(a, 0, true) }},
		{"dynamic_loop_payload", "let payload=[\"a\",\"b\"]\nlet values=repeat 2 {yield payload}\nreturn count(values)", func(a []map[string]any) error { _, err := p15TaggedLoopPayloadGraph(a, false); return err }},
		{"nested_loop_payload", "let values=repeat 2 {yield [[\"inner\"],[]]}\nreturn count(values)", func(a []map[string]any) error { _, err := p15TaggedLoopPayloadGraph(a, true); return err }},
		{"distinct_logical_cells", "let values=[none,false,null,[]]\nreturn count(values)", p15TaggedFourCellGraph},
	}
	for _, tc := range p15F04LocalCellCases() {
		switch tc.name {
		case "alias_zero_none", "alias_one_empty_list", "alias_three_two_items", "alias_for_two":
			tc := tc
			cases = append(cases, p15ScopedTransportCase{tc.name, tc.source(), func(a []map[string]any) error {
				_, err := p15F04LocalCellGraphError(a, tc)
				return err
			}})
		}
	}
	return cases
}

func p15ScopedTransportSource(source string) string {
	index := strings.LastIndex(source, "\nreturn ")
	if index < 0 {
		panic("fixture must have a final explicit output")
	}
	return "return measure(\"P15_SCOPED_TRANSPORT\") {\n" + source[:index] + "\n" + source[index+len("\nreturn "):] + "\n}"
}

func p15ScopedTransportCompile(t *testing.T, source string, profile BuildProfile) []map[string]any {
	t.Helper()
	actions, err := compileBuildProfileFixture(t, source, false, profile)
	maps := make([]map[string]any, len(actions))
	for i, action := range actions {
		maps[i] = map[string]any{"WFWorkflowActionIdentifier": action.WFWorkflowActionIdentifier}
		if len(action.WFWorkflowActionParameters) != 0 {
			maps[i]["WFWorkflowActionParameters"] = action.WFWorkflowActionParameters
		}
	}
	if dir := os.Getenv("P15_SCOPED_TRANSPORT_OUT"); dir != "" {
		name := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name())
		record := map[string]any{"test": t.Name(), "profile": profile, "source": source, "actions": maps}
		if err != nil {
			record["error"] = err.Error()
		}
		raw, marshalErr := json.MarshalIndent(record, "", "  ")
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if writeErr := os.WriteFile(filepath.Join(dir, name+".json"), append(raw, '\n'), 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err != nil {
		t.Fatalf("authored composition did not compile: %v", err)
	}
	return maps
}

// Only the exact wrapper lifecycle is projected away. The actual terminal
// output remains, so the pre-existing whole-body graph checks its real edge.
// Nothing, body effects, payload fields and unexpected actions are never removed.
func p15ScopedTransportBody(actual []map[string]any, profile BuildProfile) ([]map[string]any, error) {
	n := len(actual)
	start, end := 0, n-2
	if n < 3 {
		return nil, fmt.Errorf("incomplete scoped-value graph")
	}
	if actual[n-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.output" ||
		actual[n-2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
		return nil, fmt.Errorf("missing final value restoration and output")
	}
	restore := p15Param(actual[n-2])
	if err := p15MeasureVoidAllowedKeys(restore, "WFVariable", "UUID", "CustomOutputName"); err != nil {
		return nil, err
	}
	bodyOutputs, bodyVariables := p15V02NativeReferences(restore["WFVariable"])
	finalOutputs, finalVariables := p15V02NativeReferences(p15Param(actual[n-1])["WFOutput"])
	if len(bodyOutputs) != 1 || len(bodyVariables) != 0 || !reflect.DeepEqual(bodyOutputs, finalOutputs) || len(finalVariables) != 0 {
		return nil, fmt.Errorf("restoration or output lost the exact original body producer")
	}
	if profile == BuildProfileDebug {
		if n < 7 {
			return nil, fmt.Errorf("incomplete Debug timer lifecycle")
		}
		start, end = 1, n-5
		first := p15Param(actual[0])
		key, _ := first["WFVariableName"].(string)
		if actual[0]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" || !strings.HasPrefix(key, "__cherri_debug_timer_") {
			return nil, fmt.Errorf("Debug timer does not precede the complete body")
		}
		if err := p15MeasureVoidAllowedKeys(first, "WFVariableName", "WFInput", "UUID", "CustomOutputName"); err != nil {
			return nil, err
		}
		date, err := p15MeasureVoidAttachment(first["WFInput"])
		if err != nil || len(date) != 1 || date["Type"] != "CurrentDate" {
			return nil, fmt.Errorf("timer start lost exact CurrentDate: %v", err)
		}
		stop := p15Param(actual[end])
		if actual[end]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettimebetweendates" {
			return nil, fmt.Errorf("Debug timer does not follow the complete body")
		}
		if err := p15MeasureVoidAllowedKeys(stop, "WFTimeUntilFromDate", "WFInput", "WFTimeUntilUnit", "UUID", "CustomOutputName"); err != nil {
			return nil, err
		}
		from, err := p15MeasureVoidDateString(stop["WFTimeUntilFromDate"])
		if err != nil {
			return nil, err
		}
		to, err := p15MeasureVoidDateString(stop["WFInput"])
		if err != nil {
			return nil, err
		}
		if len(from) != 2 || from["Type"] != "Variable" || from["VariableName"] != key ||
			len(to) != 1 || to["Type"] != "CurrentDate" || stop["WFTimeUntilUnit"] != "Seconds" {
			return nil, fmt.Errorf("timer changed source, endpoint, or unit")
		}
		id, _ := stop["UUID"].(string)
		if id == "" {
			return nil, fmt.Errorf("elapsed producer has no UUID")
		}
		for offset, field := range []string{"WFInput", "WFVariable"} {
			action := actual[end+1+offset]
			params := p15Param(action)
			if offset == 0 {
				if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.appendvariable" || params["WFVariableName"] != "__cherri_debug_log" {
					return nil, fmt.Errorf("elapsed log operation changed")
				}
				if err := p15MeasureVoidAllowedKeys(params, "WFInput", "WFVariableName", "UUID", "CustomOutputName"); err != nil {
					return nil, err
				}
			} else {
				if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
					return nil, fmt.Errorf("elapsed forwarding operation changed")
				}
				if err := p15MeasureVoidAllowedKeys(params, "WFVariable", "UUID", "CustomOutputName"); err != nil {
					return nil, err
				}
			}
			ref, err := p15MeasureVoidAttachment(params[field])
			if err != nil || len(ref) != 3 || ref["Type"] != "ActionOutput" || ref["OutputUUID"] != id || ref["OutputName"] != "Time Between Dates" {
				return nil, fmt.Errorf("timer diagnostic lost its exact elapsed producer: %v", err)
			}
		}
	}
	if end <= start {
		return nil, fmt.Errorf("scoped body was discarded")
	}
	// Validate all actual UUID/reference ordering, including wrapper producers.
	seen := map[string]int{}
	for i, action := range actual {
		params := p15Param(action)
		outs, _ := p15V02NativeReferences(params)
		for _, id := range outs {
			at, found := seen[id]
			if !found || at >= i {
				return nil, fmt.Errorf("missing/nonpreceding producer at action %d: %s", i, id)
			}
		}
		if id, _ := params["UUID"].(string); id != "" {
			if _, exists := seen[id]; exists {
				return nil, fmt.Errorf("duplicate actual producer %s", id)
			}
			seen[id] = i
		}
	}
	body := append([]map[string]any{}, actual[start:end]...)
	return append(body, actual[n-1]), nil
}

func TestP15ScopedValueTransportComposition(t *testing.T) {
	cases := p15ScopedTransportCases()
	if len(cases) != 10 {
		t.Fatalf("fixture inventory changed: %d", len(cases))
	}
	for _, tc := range cases {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(tc.name+"/"+string(profile)+"/ordinary", func(t *testing.T) {
				if err := tc.check(p15ScopedTransportCompile(t, tc.source, profile)); err != nil {
					t.Fatal(err)
				}
			})
			t.Run(tc.name+"/"+string(profile)+"/measure", func(t *testing.T) {
				actual := p15ScopedTransportCompile(t, p15ScopedTransportSource(tc.source), profile)
				body, err := p15ScopedTransportBody(actual, profile)
				if err != nil {
					t.Fatal(err)
				}
				if err := tc.check(body); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15ScopedValueTransportGraphFaults(t *testing.T) {
	tc := p15ScopedTransportCases()[5]
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		t.Run(string(profile), func(t *testing.T) {
			actual := p15ScopedTransportCompile(t, p15ScopedTransportSource(tc.source), profile)
			verify := func(a []map[string]any) error {
				body, err := p15ScopedTransportBody(a, profile)
				if err != nil {
					return err
				}
				return tc.check(body)
			}
			if err := verify(actual); err != nil {
				t.Fatal(err)
			}
			faults := []string{"restore_wrong_value", "output_wrong_value", "drop_body_cell", "duplicate_body_cell", "extra_nothing", "duplicate_uuid"}
			if profile == BuildProfileDebug {
				faults = append(faults, "wrong_timer_start", "wrong_elapsed_log")
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					a := p15TaggedClone(t, actual)
					start := 0
					if profile == BuildProfileDebug {
						start = 1
					}
					switch fault {
					case "restore_wrong_value":
						p15Param(a[len(a)-2])["WFVariable"] = p15FunctionEdge("UNRELATED_BODY")
					case "output_wrong_value":
						p15Param(a[len(a)-1])["WFOutput"] = p15FunctionEdge("UNRELATED_BODY")
					case "drop_body_cell":
						a = append(a[:start], a[start+1:]...)
					case "duplicate_body_cell":
						a = append(a[:start], append([]map[string]any{a[start]}, a[start:]...)...)
					case "extra_nothing":
						a = append(a, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					case "duplicate_uuid":
						p15Param(a[start+1])["UUID"] = p15Param(a[start])["UUID"]
					case "wrong_timer_start":
						p15Param(a[0])["WFInput"] = p15FunctionVariable("UNRELATED_DATE")
					case "wrong_elapsed_log":
						p15Param(a[len(a)-4])["WFInput"] = p15FunctionEdge("UNRELATED_ELAPSED")
					}
					if verify(a) == nil {
						t.Fatalf("oracle accepted causal corruption %s", fault)
					}
				})
			}
		})
	}
}
