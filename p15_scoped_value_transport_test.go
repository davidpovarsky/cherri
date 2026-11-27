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
	var err error
	actual, err = p15MeasureProjectInputRows(actual, 1, profile)
	if err != nil {
		return nil, err
	}
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
						start = 3 // capture, timer, and restore precede the original body
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
						p15Param(a[1])["WFInput"] = p15FunctionVariable("UNRELATED_DATE")
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

// Repeated invocations and captured writes use distinct native storage while
// retaining each returned mutable snapshot. This checks real serialized edges.
func p15ScopedLexicalStorageGraph(actions []map[string]any, profile BuildProfile, captured bool) error {
	timers := 2
	if captured {
		timers = 1
	}
	var err error
	actions, err = p15MeasureProjectInputRows(actions, timers, profile)
	if err != nil {
		return err
	}
	type write struct {
		name, value string
		at          int
	}
	writes := []write{}
	producers := map[string]int{}
	timerKeys := map[string]bool{}
	timerCount := 0
	for i, action := range actions {
		p := p15Param(action)
		if id, _ := p["UUID"].(string); id != "" {
			if _, exists := producers[id]; exists {
				return fmt.Errorf("duplicate producer")
			}
			producers[id] = i
		}
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" {
			continue
		}
		name, _ := p["WFVariableName"].(string)
		if strings.HasPrefix(name, "__cherri_debug_timer_") {
			timerCount++
			if timerKeys[name] {
				return fmt.Errorf("repeated measure shares timer storage")
			}
			timerKeys[name] = true
			continue
		}
		outs, vars := p15V02NativeReferences(p["WFInput"])
		if len(outs) != 1 || len(vars) != 0 {
			return fmt.Errorf("write lost explicit source value")
		}
		at, found := producers[outs[0]]
		if !found || at >= i || actions[at]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
			return fmt.Errorf("write source was not original Text")
		}
		value, readError := p15LiteralText(p15Param(actions[at])["WFTextActionText"])
		if readError != nil {
			return fmt.Errorf("write payload changed")
		}
		writes = append(writes, write{name, value, i})
	}
	wantTimers := 2
	if captured {
		wantTimers = 1
	}
	if profile == BuildProfileRelease {
		wantTimers = 0
	}
	if timerCount != wantTimers {
		return fmt.Errorf("timer/profile count changed")
	}
	expected := []string{"first", "second"}
	if captured {
		expected = []string{"outer", "written", "inner", "branch", "changed"}
	}
	if len(writes) != len(expected) {
		return fmt.Errorf("write count=%d want%d", len(writes), len(expected))
	}
	for i, w := range writes {
		if w.value != expected[i] {
			return fmt.Errorf("source write order changed at%d", i)
		}
		if captured && i < 2 {
			if w.name != "value" {
				return fmt.Errorf("captured outer write redirected")
			}
		} else if !strings.HasPrefix(w.name, "__cherri_block_") {
			return fmt.Errorf("scoped storage is not private")
		}
	}
	names := []string{writes[0].name, writes[1].name}
	if captured {
		if writes[2].name == writes[3].name || writes[3].name != writes[4].name {
			return fmt.Errorf("branch shadow storage collided or assignment detached")
		}
		names = []string{writes[2].name}
	} else if names[0] == names[1] {
		return fmt.Errorf("repeated invocation storage collided")
	}
	snapshots := []string{}
	for i, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
			continue
		}
		p := p15Param(action)
		outs, vars := p15V02NativeReferences(p["WFVariable"])
		if len(vars) == 0 {
			continue
		}
		if len(vars) != 1 || len(outs) != 0 || len(snapshots) >= len(names) || vars[0] != names[len(snapshots)] {
			return fmt.Errorf("mutable snapshot read wrong lexical slot")
		}
		id, _ := p["UUID"].(string)
		if id == "" {
			return fmt.Errorf("snapshot has no producer")
		}
		if profile == BuildProfileDebug {
			foundStop := false
			for j := i + 1; j < len(actions); j++ {
				if actions[j]["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettimebetweendates" {
					foundStop = true
					break
				}
				if actions[j]["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" {
					return fmt.Errorf("snapshot crossed its next source scope")
				}
			}
			if !foundStop {
				return fmt.Errorf("snapshot was not frozen before diagnostics")
			}
		}
		snapshots = append(snapshots, id)
	}
	if len(snapshots) != len(names) {
		return fmt.Errorf("missing immutable snapshot")
	}
	n := len(actions)
	if n < 2 || actions[n-2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.list" || actions[n-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.output" {
		return fmt.Errorf("final list/output changed")
	}
	items, okay := p15Param(actions[n-2])["WFItems"].([]any)
	if !okay || len(items) != 2 {
		return fmt.Errorf("final output cardinality changed")
	}
	for i, item := range items {
		outs, vars := p15V02NativeReferences(item)
		if captured && i == 0 {
			if len(outs) != 0 || !reflect.DeepEqual(vars, []string{"value"}) {
				return fmt.Errorf("outer value was hidden by scoped binding")
			}
		} else {
			index := i
			if captured {
				index = 0
			}
			if !reflect.DeepEqual(outs, []string{snapshots[index]}) || len(vars) != 0 {
				return fmt.Errorf("returned values lost original snapshot/order")
			}
		}
	}
	outs, vars := p15V02NativeReferences(p15Param(actions[n-1])["WFOutput"])
	id, _ := p15Param(actions[n-2])["UUID"].(string)
	if id == "" || !reflect.DeepEqual(outs, []string{id}) || len(vars) != 0 {
		return fmt.Errorf("final output lost actual List")
	}
	return nil
}

func TestP15ScopedValueTransportLexicalStorage(t *testing.T) {
	for name, source := range map[string]string{
		"repeated":                   "let first=measure(\"same\"){var value=\"first\"\nvalue}\nlet second=measure(\"same\"){var value=\"second\"\nvalue}\nreturn [first,second]",
		"captured_write_then_shadow": "var value=\"outer\"\nlet measured=measure(\"scope\"){value=\"written\"\nvar value=\"inner\"\nif true {var value=\"branch\"\nvalue=\"changed\"}\nvalue}\nreturn [value,measured]",
	} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(name+"/"+string(profile), func(t *testing.T) {
				actual := p15ScopedTransportCompile(t, source, profile)
				captured := name == "captured_write_then_shadow"
				if err := p15ScopedLexicalStorageGraph(actual, profile, captured); err != nil {
					t.Fatal(err)
				}
				timers := 2
				if captured {
					timers = 1
				}
				inputActions, err := p15MeasureInputProjection(p15MeasureInputRows(actual), timers, profile)
				if err != nil {
					t.Fatal(err)
				}
				for _, fault := range []string{"wrong_snapshot", "swap_results", "source_write_changed", "storage_collision"} {
					t.Run(fault, func(t *testing.T) {
						a := p15TaggedClone(t, actual)
						switch fault {
						case "wrong_snapshot":
							for i, action := range a {
								if inputActions[i] {
									continue
								}
								p := p15Param(action)
								_, vars := p15V02NativeReferences(p["WFVariable"])
								if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && len(vars) == 1 {
									p["WFVariable"] = p15FunctionVariable("UNRELATED_SLOT")
									break
								}
							}
						case "swap_results":
							items := p15Param(a[len(a)-2])["WFItems"].([]any)
							items[0], items[1] = items[1], items[0]
						case "source_write_changed":
							for _, action := range a {
								if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
									p15Param(action)["WFTextActionText"] = "ALTERED"
									break
								}
							}
						case "storage_collision":
							first := ""
							for i, action := range a {
								if inputActions[i] {
									continue
								}
								p := p15Param(action)
								name, _ := p["WFVariableName"].(string)
								if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && strings.HasPrefix(name, "__cherri_block_") {
									if first == "" {
										first = name
									} else {
										p["WFVariableName"] = first
										break
									}
								}
							}
						}
						if p15ScopedLexicalStorageGraph(a, profile, captured) == nil {
							t.Fatal("accepted " + fault)
						}
					})
				}
			})
		}
	}
}
