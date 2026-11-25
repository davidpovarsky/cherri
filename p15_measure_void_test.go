package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type p15MeasureVoidCase struct {
	name, v2, legacy, flatV2, flatLegacy string
	legacyHeader                         string
	timers                               int
}

func p15MeasureVoidCases() []p15MeasureVoidCase {
	shared := func(name, body string) p15MeasureVoidCase {
		return p15MeasureVoidCase{name: name, v2: body, legacy: body, flatV2: body, flatLegacy: body, timers: 1}
	}
	cases := []p15MeasureVoidCase{
		shared("empty", ""),
		shared("wait", "wait(1)\n"),
		shared("two_effects", "wait(1)\nwait(2)\n"),
		shared("body_nothing", "nothing()\nwait(1)\nnothing()\nwait(2)\n"),
		{name: "argument_effect", v2: "setClipboard(text(\"CHERRI_VOID_ONCE\"))\nwait(1)\n", legacy: "setClipboard(text(\"CHERRI_VOID_ONCE\"))\nwait(1)\n", flatV2: "setClipboard(text(\"CHERRI_VOID_ONCE\"))\nwait(1)\n", flatLegacy: "setClipboard(text(\"CHERRI_VOID_ONCE\"))\nwait(1)\n", legacyHeader: "#include 'actions/sharing'\n", timers: 1},
		shared("value_then_void", "text(\"CHERRI_BODY_PRODUCER\")\nwait(3)\n"),
		shared("stop_before_void_tail", "stop()\nwait(1)\n"),
		{name: "binding_tail", v2: "let local = text(\"CHERRI_LOCAL\")\n", legacy: "const local = text(\"CHERRI_LOCAL\")\n", flatV2: "let local = text(\"CHERRI_LOCAL\")\n", flatLegacy: "const local = text(\"CHERRI_LOCAL\")\n", timers: 1},
		{name: "assignment_tail", v2: "var local = 1\nlocal = 2\n", legacy: "@local = 1\n@local = 2\n", flatV2: "var local = 1\nlocal = 2\n", flatLegacy: "@local = 1\n@local = 2\n", timers: 1},
		{name: "if_tail", v2: "if true { wait(1) } else { wait(2) }\n", legacy: "const condition = 1\nif condition == 1 {\nwait(1)\n} else {\nwait(2)\n}\n", flatV2: "if true { wait(1) } else { wait(2) }\n", flatLegacy: "const condition = 1\nif condition == 1 {\nwait(1)\n} else {\nwait(2)\n}\n", timers: 1},
		{name: "repeat_tail", v2: "repeat 2 { wait(1) }\n", legacy: "repeat index for 2 {\nwait(1)\n}\n", flatV2: "repeat 2 { wait(1) }\n", flatLegacy: "repeat index for 2 {\nwait(1)\n}\n", timers: 1},
		{name: "nested_empty", v2: "measure(\"same\") {\n}\n", legacy: "measure(\"same\") {\n}\n", flatV2: "nothing()\n", flatLegacy: "nothing()\n", timers: 2},
		{name: "nested_wait", v2: "measure(\"same\") {\nwait(4)\n}\n", legacy: "measure(\"same\") {\nwait(4)\n}\n", flatV2: "wait(4)\nnothing()\n", flatLegacy: "wait(4)\nnothing()\n", timers: 2},
	}
	return cases
}

func p15MeasureVoidWrapped(body string) string {
	return "text(\"CHERRI_BEFORE\")\nmeasure(\"same\") {\n" + body + "}\ntext(\"CHERRI_AFTER\")\n"
}

func p15MeasureVoidFlat(body string) string {
	return "text(\"CHERRI_BEFORE\")\n" + body + "nothing()\ntext(\"CHERRI_AFTER\")\n"
}

func p15MeasureVoidReference(value any) map[string]any {
	object, _ := value.(map[string]any)
	if nested, ok := object["Value"].(map[string]any); ok {
		return nested
	}
	return object
}

func p15MeasureVoidDateString(value any) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != 2 || object["WFSerializationType"] != "WFTextTokenString" {
		return nil, fmt.Errorf("date is not a native text token")
	}
	payload, _ := object["Value"].(map[string]any)
	if len(payload) != 2 || payload["string"] != "\uFFFC" {
		return nil, fmt.Errorf("date token is not one exact placeholder")
	}
	attachments, _ := payload["attachmentsByRange"].(map[string]any)
	ref, ok := attachments["{0, 1}"].(map[string]any)
	if !ok || len(attachments) != 1 {
		return nil, fmt.Errorf("date token attachment range changed")
	}
	return ref, nil
}

// This oracle removes only complete, causally linked timer instrumentation.
// It then compares every remaining native action and parameter with the real
// ordinary source body. UUID/name normalization preserves references and data;
// no runtime result is simulated or inferred from an exit code.
func p15MeasureVoidGraphError(actual, ordinary []ShortcutAction, timers int, profile BuildProfile) error {
	var body []ShortcutAction
	var stack []string
	var startOffsets []int
	var intervals [][2]int
	starts, stops := 0, 0
	for i := 0; i < len(actual); i++ {
		action := actual[i]
		params := action.WFWorkflowActionParameters
		name, _ := params["WFVariableName"].(string)
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && strings.HasPrefix(name, "__cherri_debug_timer_") {
			if profile != BuildProfileDebug {
				return fmt.Errorf("Release retained a timer")
			}
			if err := p15MeasureVoidAllowedKeys(params, "WFVariableName", "WFInput", "UUID", "CustomOutputName"); err != nil {
				return err
			}
			ref, err := p15MeasureVoidAttachment(params["WFInput"])
			if err != nil {
				return err
			}
			if len(ref) != 1 || ref["Type"] != "CurrentDate" {
				return fmt.Errorf("timer start lost native CurrentDate")
			}
			for _, active := range stack {
				if active == name {
					return fmt.Errorf("nested same-label timer storage collided")
				}
			}
			stack = append(stack, name)
			startOffsets = append(startOffsets, len(body))
			starts++
			continue
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettimebetweendates" {
			if profile != BuildProfileDebug || len(stack) == 0 || i+2 >= len(actual) {
				return fmt.Errorf("unpaired timer stop")
			}
			if err := p15MeasureVoidAllowedKeys(params, "WFTimeUntilFromDate", "WFInput", "WFTimeUntilUnit", "UUID", "CustomOutputName"); err != nil {
				return err
			}
			start, err := p15MeasureVoidDateString(params["WFTimeUntilFromDate"])
			if err != nil {
				return err
			}
			end, err := p15MeasureVoidDateString(params["WFInput"])
			if err != nil {
				return err
			}
			key := stack[len(stack)-1]
			if len(start) != 2 || start["Type"] != "Variable" || start["VariableName"] != key || len(end) != 1 || end["Type"] != "CurrentDate" || params["WFTimeUntilUnit"] != "Seconds" {
				return fmt.Errorf("timer order, origin, unit, or endpoint changed")
			}
			id, _ := params["UUID"].(string)
			logged, forwarded := actual[i+1], actual[i+2]
			if err := p15MeasureVoidAllowedKeys(logged.WFWorkflowActionParameters, "WFVariableName", "WFInput", "UUID", "CustomOutputName"); err != nil {
				return err
			}
			if err := p15MeasureVoidAllowedKeys(forwarded.WFWorkflowActionParameters, "WFVariable", "UUID", "CustomOutputName"); err != nil {
				return err
			}
			logref, err := p15MeasureVoidAttachment(logged.WFWorkflowActionParameters["WFInput"])
			if err != nil {
				return err
			}
			forwardref, err := p15MeasureVoidAttachment(forwarded.WFWorkflowActionParameters["WFVariable"])
			if err != nil {
				return err
			}
			if id == "" || logged.WFWorkflowActionIdentifier != "is.workflow.actions.appendvariable" || logged.WFWorkflowActionParameters["WFVariableName"] != "__cherri_debug_log" ||
				len(logref) != 3 || logref["Type"] != "ActionOutput" || logref["OutputUUID"] != id || logref["OutputName"] != "Time Between Dates" ||
				forwarded.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || len(forwardref) != 3 || forwardref["Type"] != "ActionOutput" || forwardref["OutputUUID"] != id || forwardref["OutputName"] != "Time Between Dates" {
				return fmt.Errorf("timer diagnostic does not consume its exact elapsed producer")
			}
			if i+3 >= len(actual) || actual[i+3].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" || len(actual[i+3].WFWorkflowActionParameters) != 0 {
				return fmt.Errorf("Void timer stop must end at its exact parameter-free Nothing boundary")
			}
			intervals = append(intervals, [2]int{startOffsets[len(startOffsets)-1], len(body)})
			stack = stack[:len(stack)-1]
			startOffsets = startOffsets[:len(startOffsets)-1]
			stops++
			i += 2
			continue
		}
		body = append(body, action)
	}
	expected := timers
	if profile == BuildProfileRelease {
		expected = 0
	}
	if starts != expected || stops != expected || len(stack) != 0 {
		return fmt.Errorf("timer lifecycle mismatch: starts=%d stops=%d expected=%d open=%v", starts, stops, expected, stack)
	}
	if profile == BuildProfileDebug {
		expectedIntervals, err := p15MeasureVoidExpectedIntervals(ordinary, timers)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(intervals, expectedIntervals) {
			return fmt.Errorf("timers no longer bracket their complete authored body: got %v want %v", intervals, expectedIntervals)
		}
	}
	actualGraph, err := p15MeasureVoidCanonicalGraph(body)
	if err != nil {
		return err
	}
	ordinaryGraph, err := p15MeasureVoidCanonicalGraph(ordinary)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actualGraph, ordinaryGraph) {
		got, _ := json.Marshal(actualGraph)
		want, _ := json.Marshal(ordinaryGraph)
		return fmt.Errorf("void body changed, repeated, gained a result, or lost an effect:\ngot %s\nwant %s", got, want)
	}
	return nil
}

func p15MeasureVoidCanonicalGraph(actions []ShortcutAction) (any, error) {
	ids, groups, names := map[string]string{}, map[string]string{}, map[string]string{}
	producerAt := map[string]int{}
	for i, action := range actions {
		p := action.WFWorkflowActionParameters
		if id, ok := p["UUID"].(string); ok && id != "" {
			if _, exists := ids[id]; exists {
				return nil, fmt.Errorf("duplicate producer UUID")
			}
			ids[id], producerAt[id] = fmt.Sprintf("producer_%d", i), i
		}
		if group, ok := p["GroupingIdentifier"].(string); ok && group != "" {
			if _, exists := groups[group]; !exists {
				groups[group] = fmt.Sprintf("group_%d", len(groups))
			}
		}
		for _, key := range []string{"WFVariableName", "CustomOutputName"} {
			if name, ok := p[key].(string); ok && name != "" {
				if _, exists := names[name]; !exists {
					names[name] = fmt.Sprintf("binding_%d", len(names))
				}
			}
		}
	}
	var walk func(any, string, int) (any, error)
	walk = func(value any, key string, at int) (any, error) {
		switch v := value.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, child := range v {
				item, err := walk(child, k, at)
				if err != nil {
					return nil, err
				}
				out[k] = item
			}
			return out, nil
		case []any:
			out := make([]any, len(v))
			for i, child := range v {
				item, err := walk(child, "", at)
				if err != nil {
					return nil, err
				}
				out[i] = item
			}
			return out, nil
		case string:
			switch key {
			case "UUID":
				return ids[v], nil
			case "OutputUUID":
				target, found := ids[v]
				if !found || producerAt[v] >= at {
					return nil, fmt.Errorf("body consumes missing or nonpreceding producer %q", v)
				}
				return target, nil
			case "GroupingIdentifier":
				return groups[v], nil
			case "WFVariableName", "VariableName", "CustomOutputName", "OutputName":
				if n, exists := names[v]; exists {
					return n, nil
				}
			}
			return v, nil
		default:
			return v, nil
		}
	}
	out := make([]any, len(actions))
	for i, action := range actions {
		params, err := walk(action.WFWorkflowActionParameters, "", i)
		if err != nil {
			return nil, err
		}
		out[i] = map[string]any{"id": action.WFWorkflowActionIdentifier, "parameters": params}
	}
	return out, nil
}

func TestP15MeasureVoidNativeGraphs(t *testing.T) {
	for _, test := range p15MeasureVoidCases() {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy_%t/%s", test.name, legacy, profile), func(t *testing.T) {
					body, flat, header := test.v2, test.flatV2, ""
					if legacy {
						body, flat, header = test.legacy, test.flatLegacy, test.legacyHeader
					}
					ordinary, err := compileBuildProfileFixture(t, header+p15MeasureVoidFlat(flat), legacy, profile)
					if err != nil {
						t.Fatalf("ordinary body control: %v", err)
					}
					actual, err := compileBuildProfileFixture(t, header+p15MeasureVoidWrapped(body), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err = p15MeasureVoidGraphError(actual, ordinary, test.timers, profile); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestP15MeasureVoidRetainsExistingValueControl(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy_%t/%s", legacy, profile), func(t *testing.T) {
				source := "measure(\"value control\") {\ntext(\"CHERRI_VALUE_CONTROL\")\n}\n"
				actual, err := compileBuildProfileFixture(t, source, legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				producers := p15ActionsNamed(actual, "gettext")
				if len(producers) != 1 || producers[0].WFWorkflowActionParameters["WFTextActionText"] != "CHERRI_VALUE_CONTROL" {
					t.Fatalf("existing value body lost or repeated: %v", actual)
				}
			})
		}
	}
}

func TestP15MeasureVoidLoopBoundaryGraphs(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		for _, following := range []bool{false, true} {
			for _, legacy := range []bool{false, true} {
				for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
					t.Run(fmt.Sprintf("n%d/following_%t/legacy_%t/%s", count, following, legacy, profile), func(t *testing.T) {
						begin := fmt.Sprintf("repeat %d {\n", count)
						if legacy {
							begin = fmt.Sprintf("repeat index for %d {\n", count)
						}
						follow := ""
						if following {
							follow = "text(\"CHERRI_AFTER_VOID_IN_LOOP\")\n"
						}
						prefix := "text(\"CHERRI_LOOP_BEFORE\")\n" + begin
						tail := follow + "}\ntext(\"CHERRI_LOOP_AFTER\")\n"
						actualSource := prefix + "measure(\"loop\") {\nwait(1)\n}\n" + tail
						ordinarySource := prefix + "wait(1)\nnothing()\n" + tail
						ordinary, err := compileBuildProfileFixture(t, ordinarySource, legacy, profile)
						if err != nil {
							t.Fatalf("ordinary loop control: %v", err)
						}
						actual, err := compileBuildProfileFixture(t, actualSource, legacy, profile)
						if err != nil {
							t.Fatal(err)
						}
						if err = p15MeasureVoidGraphError(actual, ordinary, 1, profile); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

func p15MeasureVoidAllowedKeys(params map[string]any, allowed ...string) error {
	for key := range params {
		found := false
		for _, want := range allowed {
			if key == want {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unreviewed timer parameter %q", key)
		}
	}
	return nil
}

func p15MeasureVoidAttachment(value any) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != 2 || object["WFSerializationType"] != "WFTextTokenAttachment" {
		return nil, fmt.Errorf("timer attachment wire type changed")
	}
	ref, ok := object["Value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("timer attachment has no structured reference")
	}
	return ref, nil
}

func p15MeasureVoidExpectedIntervals(ordinary []ShortcutAction, timers int) ([][2]int, error) {
	before, after, loop := -1, -1, false
	for i, action := range ordinary {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
			continue
		}
		switch action.WFWorkflowActionParameters["WFTextActionText"] {
		case "CHERRI_BEFORE":
			before = i
		case "CHERRI_AFTER":
			after = i
		case "CHERRI_LOOP_BEFORE":
			before, loop = i, true
		case "CHERRI_LOOP_AFTER":
			after = i
		}
	}
	// Two preserved historical Legacy fixtures are bare empty/wait bodies.
	// Their whole ordinary graph is exactly one optional delay plus Nothing.
	if before < 0 && after < 0 && timers == 1 && (len(ordinary) == 1 || len(ordinary) == 2) {
		last := len(ordinary) - 1
		if ordinary[last].WFWorkflowActionIdentifier == "is.workflow.actions.nothing" && len(ordinary[last].WFWorkflowActionParameters) == 0 &&
			(last == 0 || ordinary[0].WFWorkflowActionIdentifier == "is.workflow.actions.delay") {
			return [][2]int{{0, last}}, nil
		}
	}
	if before < 0 || after <= before {
		return nil, fmt.Errorf("source fixture boundaries missing")
	}
	start, boundary := before+1, after-1
	if loop {
		start = -1
		for i := before + 1; i < after; i++ {
			if ordinary[i].WFWorkflowActionIdentifier == "is.workflow.actions.delay" {
				start = i
				break
			}
		}
		boundary = start + 1
	}
	if start < 0 || boundary <= start-1 || boundary >= len(ordinary) ||
		ordinary[boundary].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" || len(ordinary[boundary].WFWorkflowActionParameters) != 0 {
		return nil, fmt.Errorf("source fixture exact Void boundary missing")
	}
	if timers == 1 {
		return [][2]int{{start, boundary}}, nil
	}
	if timers == 2 && !loop && boundary > 0 && ordinary[boundary-1].WFWorkflowActionIdentifier == "is.workflow.actions.nothing" && len(ordinary[boundary-1].WFWorkflowActionParameters) == 0 {
		return [][2]int{{start, boundary - 1}, {start, boundary}}, nil
	}
	return nil, fmt.Errorf("fixture timer nesting has no exact source contract")
}
