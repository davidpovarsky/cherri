package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func p15MeasurePreviousCases() []p15CachePreviousCase {
	cases := append([]p15CachePreviousCase(nil), p15CachePreviousCases()...)
	return append(cases,
		p15CachePreviousCase{name: "nested_log", prefix: "text(\"P15_MEASURE_ORIGINAL\")\n", consumer: "log", nested: true},
		p15CachePreviousCase{name: "empty_text", prefix: "text(\"\")\n", consumer: "log"},
		p15CachePreviousCase{name: "zero_number", prefix: "number(0)\n", consumer: "log"},
		p15CachePreviousCase{name: "no_content_date", prefix: "nothing()\n", consumer: "date"},
	)
}

func p15MeasurePreviousSource(tc p15CachePreviousCase, legacy, measured, void bool) string {
	declaration, header := "let", ""
	if legacy {
		declaration = "const"
		if tc.consumer == "date" {
			header = "#include 'actions/calendar'\n"
		}
	}
	effects, value := "", "getTimeBetweenDates()"
	if tc.consumer == "log" {
		if tc.local {
			effects = "text(\"P15_MEASURE_BODY_INPUT\")\n"
		}
		effects += "debugLog()\n"
		value = "7"
	}
	if !measured {
		if void {
			return header + tc.prefix + effects + "wait(0)\n"
		}
		return header + tc.prefix + effects + declaration + " result = " + value + "\ngetVariable(result)\n"
	}
	body := effects + "yield " + value + "\n"
	if void {
		body = effects + "wait(0)\n"
	}
	if tc.nested {
		if void {
			body = "measure(\"same label\") {\n" + body + "}\n"
		} else {
			body = declaration + " inner = measure(\"same label\") {\n" + body + "}\nyield inner\n"
		}
	}
	call := "measure(\"same label\") {\n" + body + "}\n"
	if void {
		return header + tc.prefix + call
	}
	return header + tc.prefix + declaration + " result = " + call + "getVariable(result)\n"
}

// The ordinary consumer is distinguished from the timer's own elapsed-time
// log by its actual producer and date fields, never by its observed value.
func p15MeasurePreviousTargets(g *p15PreviousGraph, consumer string) ([]int, int, error) {
	var authored []int
	timers := 0
	for _, index := range p15PreviousTargets(g.actions, consumer) {
		if consumer == "log" {
			ref, err := p15PreviousObject(g.actions[index].WFWorkflowActionParameters["WFInput"])
			if err != nil {
				return nil, 0, err
			}
			if ref["Type"] == "ActionOutput" {
				producer, exists := g.ids[fmt.Sprint(ref["OutputUUID"])]
				if !exists || producer >= index {
					return nil, 0, fmt.Errorf("missing/forward logging producer")
				}
				action := g.actions[producer]
				p := action.WFWorkflowActionParameters
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettimebetweendates" &&
					p["WFTimeUntilFromDate"] != nil && p["WFInput"] != nil {
					timers++
					continue
				}
			}
		}
		authored = append(authored, index)
	}
	return authored, timers, nil
}

func p15MeasurePreviousEquivalence(baseline, actual []ShortcutAction, tc p15CachePreviousCase, profile BuildProfile) error {
	bg, err := p15PreviousGraphFor(baseline)
	if err != nil {
		return err
	}
	ag, err := p15PreviousGraphFor(actual)
	if err != nil {
		return err
	}
	b, _, err := p15MeasurePreviousTargets(bg, tc.consumer)
	if err != nil {
		return err
	}
	a, timers, err := p15MeasurePreviousTargets(ag, tc.consumer)
	if err != nil {
		return err
	}
	want, timerCount := 1, 0
	if tc.consumer == "log" {
		if profile == BuildProfileRelease {
			want = 0
		} else {
			timerCount = 1
			if tc.nested {
				timerCount = 2
			}
		}
	}
	if len(b) != want || len(a) != want || timers != timerCount {
		return fmt.Errorf("authored consumer or timer inventory: %d/%d/%d, want %d/%d", len(b), len(a), timers, want, timerCount)
	}
	if want == 0 {
		return nil
	}
	input := func(g *p15PreviousGraph, i int) (string, error) {
		if tc.consumer == "log" {
			return g.reference(g.actions[i].WFWorkflowActionParameters["WFInput"], i, map[int]bool{})
		}
		return g.origin(i-1, map[int]bool{})
	}
	expected, err := input(bg, b[0])
	if err != nil {
		return fmt.Errorf("ordinary input: %w", err)
	}
	observed, err := input(ag, a[0])
	if err != nil {
		return fmt.Errorf("measured input: %w", err)
	}
	if expected != observed {
		return fmt.Errorf("measurement changed authored input: %s -> %s", expected, observed)
	}
	clean := func(p map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range p {
			if k != "UUID" && k != "CustomOutputName" && !(tc.consumer == "log" && k == "WFInput") {
				out[k] = v
			}
		}
		return out
	}
	if !reflect.DeepEqual(clean(baseline[b[0]].WFWorkflowActionParameters), clean(actual[a[0]].WFWorkflowActionParameters)) {
		return fmt.Errorf("measurement changed authored consumer parameters")
	}
	return nil
}

func p15MeasurePreviousBoundaries(actions []ShortcutAction, tc p15CachePreviousCase, profile BuildProfile) error {
	captures, names := 0, map[string]bool{}
	for i, action := range actions {
		p := action.WFWorkflowActionParameters
		name, _ := p["WFVariableName"].(string)
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" ||
			!strings.HasPrefix(name, "__cherri_block_") || !strings.HasSuffix(name, "_input") {
			continue
		}
		captures++
		if names[name] {
			return fmt.Errorf("nested input slot collided")
		}
		names[name] = true
		if _, explicit := p["WFInput"]; explicit || i+2 >= len(actions) {
			return fmt.Errorf("whole-native input capture changed")
		}
		timer, restore := actions[i+1], actions[i+2]
		timerName, _ := timer.WFWorkflowActionParameters["WFVariableName"].(string)
		if timer.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" ||
			!strings.HasPrefix(timerName, debugTimerVariablePrefix) || timer.WFWorkflowActionParameters["WFInput"] == nil {
			return fmt.Errorf("input not captured before timer")
		}
		if restore.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" ||
			restore.WFWorkflowActionParameters["UUID"] == nil {
			return fmt.Errorf("input not restored immediately after timer")
		}
		ref, err := p15PreviousObject(restore.WFWorkflowActionParameters["WFVariable"])
		if err != nil || len(ref) != 2 || ref["Type"] != "Variable" || ref["VariableName"] != name {
			return fmt.Errorf("input restoration changed its typed native reference")
		}
	}
	want := 0
	if profile == BuildProfileDebug {
		want = 1
		if tc.nested {
			want = 2
		}
	}
	if captures != want {
		return fmt.Errorf("input boundary count %d, want %d", captures, want)
	}
	return nil
}

func TestP15MeasurePreviousMatchesAuthoredBaseline(t *testing.T) {
	for _, tc := range p15MeasurePreviousCases() {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", tc.name, legacy, profile), func(t *testing.T) {
					baseline, err := compileBuildProfileFixture(t, p15MeasurePreviousSource(tc, legacy, false, false), legacy, profile)
					if err != nil {
						t.Fatalf("ordinary authored program: %v", err)
					}
					actual, err := compileBuildProfileFixture(t, p15MeasurePreviousSource(tc, legacy, true, false), legacy, profile)
					if err != nil {
						t.Fatalf("measured authored program: %v", err)
					}
					if err := p15MeasurePreviousEquivalence(baseline, actual, tc, profile); err != nil {
						t.Fatal(err)
					}
					if err := p15MeasurePreviousBoundaries(actual, tc, profile); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestP15MeasureVoidPreservesPrevious(t *testing.T) {
	for _, nested := range []bool{false, true} {
		tc := p15CachePreviousCase{name: "void", prefix: "text(\"VOID_BODY_INPUT\")\n", consumer: "log", nested: nested}
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("nested=%t/legacy=%t/%s", nested, legacy, profile), func(t *testing.T) {
					baseline, err := compileBuildProfileFixture(t, p15MeasurePreviousSource(tc, legacy, false, true), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					actual, err := compileBuildProfileFixture(t, p15MeasurePreviousSource(tc, legacy, true, true), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err := p15MeasurePreviousEquivalence(baseline, actual, tc, profile); err != nil {
						t.Fatal(err)
					}
					if err := p15MeasurePreviousBoundaries(actual, tc, profile); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestP15MeasurePreviousOracleRejectsCausalFaults(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		tc := p15CachePreviousCases()[0]
		baseline, err := compileBuildProfileFixture(t, p15MeasurePreviousSource(tc, legacy, false, false), legacy, BuildProfileDebug)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := compileBuildProfileFixture(t, p15MeasurePreviousSource(tc, legacy, true, false), legacy, BuildProfileDebug)
		if err != nil {
			t.Fatal(err)
		}
		for _, fault := range []string{"explicit_capture", "capture_after_timer", "wrong_restore_slot", "coerced_restore", "removed_consumer"} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, fault), func(t *testing.T) {
				raw, err := json.Marshal(actual)
				if err != nil {
					t.Fatal(err)
				}
				var altered []ShortcutAction
				if err := json.Unmarshal(raw, &altered); err != nil {
					t.Fatal(err)
				}
				capture := -1
				for i, a := range altered {
					name, _ := a.WFWorkflowActionParameters["WFVariableName"].(string)
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" &&
						strings.HasPrefix(name, "__cherri_block_") && strings.HasSuffix(name, "_input") {
						capture = i
						break
					}
				}
				if capture < 0 || capture+2 >= len(altered) {
					t.Fatal("original graph has no measured input boundary")
				}
				switch fault {
				case "explicit_capture":
					altered[capture].WFWorkflowActionParameters["WFInput"] = "invented content"
				case "capture_after_timer":
					altered[capture], altered[capture+1] = altered[capture+1], altered[capture]
				case "wrong_restore_slot", "coerced_restore":
					ref, err := p15PreviousObject(altered[capture+2].WFWorkflowActionParameters["WFVariable"])
					if err != nil {
						t.Fatal(err)
					}
					if fault == "wrong_restore_slot" {
						ref["VariableName"] = "NOT_THE_CAPTURED_INPUT"
					} else {
						ref["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
					}
				case "removed_consumer":
					targets := p15PreviousTargets(altered, "date")
					if len(targets) != 1 {
						t.Fatal("original authored date consumer missing")
					}
					i := targets[0]
					altered = append(altered[:i], altered[i+1:]...)
				}
				if p15MeasurePreviousEquivalence(baseline, altered, tc, BuildProfileDebug) == nil &&
					p15MeasurePreviousBoundaries(altered, tc, BuildProfileDebug) == nil {
					t.Fatal("accepted causal input corruption: " + fault)
				}
			})
		}
	}
}
