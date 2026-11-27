package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var p15MeasureInputSlotName = regexp.MustCompile("^__cherri_block_[0-9a-fA-F]{32}_input$")

type p15MeasureInputBoundary struct {
	capture, timer, restore    int
	name, captureID, restoreID string
}

// Verify the whole input capture/timer/restore boundary before either
// alpha-renaming its private binding or projecting it from an ordinary-body
// comparison. No authored action, native value, or timer operation is removed.
func p15MeasureInputBoundaries(actions []ShortcutAction, timers int, profile BuildProfile) ([]p15MeasureInputBoundary, error) {
	if _, err := p15MeasureVoidCanonicalGraph(actions); err != nil {
		return nil, fmt.Errorf("input boundary full graph: %w", err)
	}
	want := timers
	if profile == BuildProfileRelease {
		want = 0
	}
	records := []p15MeasureInputBoundary{}
	names := map[string]p15MeasureInputBoundary{}
	for i, action := range actions {
		p := action.WFWorkflowActionParameters
		timer, _ := p["WFVariableName"].(string)
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" || !strings.HasPrefix(timer, "__cherri_debug_timer_") {
			continue
		}
		if profile != BuildProfileDebug || i == 0 || i+1 >= len(actions) || !p15ConditionalTimerName.MatchString(timer) {
			return nil, fmt.Errorf("timer has no exact Debug input boundary")
		}
		if err := p15MeasureVoidAllowedKeys(p, "WFVariableName", "WFInput", "UUID", "CustomOutputName"); err != nil {
			return nil, err
		}
		clock, err := p15MeasureVoidAttachment(p["WFInput"])
		if err != nil || len(clock) != 1 || clock["Type"] != "CurrentDate" {
			return nil, fmt.Errorf("input boundary timer lost exact CurrentDate")
		}
		capture, restore := actions[i-1], actions[i+1]
		cp, rp := capture.WFWorkflowActionParameters, restore.WFWorkflowActionParameters
		name, _ := cp["WFVariableName"].(string)
		if capture.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" || !p15MeasureInputSlotName.MatchString(name) {
			return nil, fmt.Errorf("timer is not immediately preceded by its private native input capture")
		}
		if err := p15MeasureVoidAllowedKeys(cp, "WFVariableName", "UUID", "CustomOutputName"); err != nil {
			return nil, fmt.Errorf("input capture must keep WFInput absent: %w", err)
		}
		if _, duplicate := names[name]; duplicate {
			return nil, fmt.Errorf("input captures share private storage")
		}
		if restore.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
			return nil, fmt.Errorf("timer is not immediately followed by native input restoration")
		}
		if err := p15MeasureVoidAllowedKeys(rp, "WFVariable", "UUID", "CustomOutputName"); err != nil {
			return nil, err
		}
		ref, err := p15MeasureVoidAttachment(rp["WFVariable"])
		if err != nil || len(ref) != 2 || ref["Type"] != "Variable" || ref["VariableName"] != name {
			return nil, fmt.Errorf("input restoration changed its exact uncoerced native variable")
		}
		rid, _ := rp["UUID"].(string)
		if rid == "" {
			return nil, fmt.Errorf("input restoration has no actual producer UUID")
		}
		cid, _ := cp["UUID"].(string)
		record := p15MeasureInputBoundary{i - 1, i, i + 1, name, cid, rid}
		records = append(records, record)
		names[name] = record
	}
	if len(records) != want {
		return nil, fmt.Errorf("input boundary inventory: got %d want %d", len(records), want)
	}
	for i, action := range actions {
		p := action.WFWorkflowActionParameters
		name, _ := p["WFVariableName"].(string)
		if p15MeasureInputSlotName.MatchString(name) {
			record, found := names[name]
			if !found || i != record.capture {
				return nil, fmt.Errorf("orphan or repeated input slot declaration")
			}
		}
		_, variables := p15V02NativeReferences(p)
		for _, variable := range variables {
			if !p15MeasureInputSlotName.MatchString(variable) {
				continue
			}
			record, found := names[variable]
			if !found || i != record.restore {
				return nil, fmt.Errorf("private input slot read outside its exact restoration")
			}
		}
	}
	return records, nil
}

// The existing ordinary-body fixtures do not explicitly consume an input
// helper's UUID. Refuse projection if they do: erasing such a dependency would
// conceal a body change. Previous-consuming fixtures retain their full graph
// and their separate ancestry oracle.
func p15MeasureInputProjection(actions []ShortcutAction, timers int, profile BuildProfile) (map[int]bool, error) {
	records, err := p15MeasureInputBoundaries(actions, timers, profile)
	if err != nil {
		return nil, err
	}
	removed, ids := map[int]bool{}, map[string]bool{}
	for _, record := range records {
		removed[record.capture], removed[record.restore] = true, true
		ids[record.restoreID] = true
		if record.captureID != "" {
			ids[record.captureID] = true
		}
	}
	for _, action := range actions {
		outputs, _ := p15V02NativeReferences(action.WFWorkflowActionParameters)
		for _, id := range outputs {
			if ids[id] {
				return nil, fmt.Errorf("body depends on an input producer that cannot be projected away")
			}
		}
	}
	return removed, nil
}

func p15MeasureProjectInput(actions []ShortcutAction, timers int, profile BuildProfile) ([]ShortcutAction, error) {
	removed, err := p15MeasureInputProjection(actions, timers, profile)
	if err != nil {
		return nil, err
	}
	out := make([]ShortcutAction, 0, len(actions)-len(removed))
	for i, action := range actions {
		if !removed[i] {
			out = append(out, action)
		}
	}
	return out, nil
}

func p15MeasureInputRows(actions []map[string]any) []ShortcutAction {
	typed := make([]ShortcutAction, len(actions))
	for i, action := range actions {
		typed[i].WFWorkflowActionIdentifier, _ = action["WFWorkflowActionIdentifier"].(string)
		typed[i].WFWorkflowActionParameters = p15Param(action)
	}
	return typed
}

func p15MeasureProjectInputRows(actions []map[string]any, timers int, profile BuildProfile) ([]map[string]any, error) {
	removed, err := p15MeasureInputProjection(p15MeasureInputRows(actions), timers, profile)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(actions)-len(removed))
	for i, action := range actions {
		if !removed[i] {
			out = append(out, action)
		}
	}
	return out, nil
}

func TestP15MeasureInputBoundaryOracleControls(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_%t", legacy), func(t *testing.T) {
			body := "measure(\"same\") {\nwait(4)\n}\n"
			ordinary, err := compileBuildProfileFixture(t, p15MeasureVoidFlat("wait(4)\nnothing()\n"), legacy, BuildProfileDebug)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := compileBuildProfileFixture(t, p15MeasureVoidWrapped(body), legacy, BuildProfileDebug)
			if err != nil {
				t.Fatal(err)
			}
			if err := p15MeasureVoidGraphError(actual, ordinary, 2, BuildProfileDebug); err != nil {
				t.Fatalf("healthy complete graph before mutation: %v", err)
			}
			boundaries, err := p15MeasureInputBoundaries(actual, 2, BuildProfileDebug)
			if err != nil || len(boundaries) != 2 {
				t.Fatalf("healthy boundaries: %v", err)
			}
			first, second := boundaries[0], boundaries[1]
			for _, fault := range []string{
				"drop_capture", "drop_restore", "capture_explicit_input", "capture_null_input",
				"restore_foreign_slot", "restore_coercion", "restore_missing_uuid",
				"capture_after_timer", "nested_slot_collision", "extra_capture_parameter",
				"external_restore_consumer", "duplicate_helper_uuid", "orphan_capture", "release_instrumentation",
			} {
				t.Run(fault, func(t *testing.T) {
					bad := p15MeasureVoidClone(t, actual)
					profile := BuildProfileDebug
					switch fault {
					case "drop_capture":
						bad = append(bad[:first.capture], bad[first.capture+1:]...)
					case "drop_restore":
						bad = append(bad[:first.restore], bad[first.restore+1:]...)
					case "capture_explicit_input":
						bad[first.capture].WFWorkflowActionParameters["WFInput"] = p15FunctionVariable("UNRELATED")
					case "capture_null_input":
						bad[first.capture].WFWorkflowActionParameters["WFInput"] = nil
					case "restore_foreign_slot":
						bad[first.restore].WFWorkflowActionParameters["WFVariable"] = p15FunctionVariable("UNRELATED")
					case "restore_coercion":
						ref, err := p15MeasureVoidAttachment(bad[first.restore].WFWorkflowActionParameters["WFVariable"])
						if err != nil {
							t.Fatal(err)
						}
						ref["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
					case "restore_missing_uuid":
						delete(bad[first.restore].WFWorkflowActionParameters, "UUID")
					case "capture_after_timer":
						bad[first.capture], bad[first.timer] = bad[first.timer], bad[first.capture]
					case "nested_slot_collision":
						bad[second.capture].WFWorkflowActionParameters["WFVariableName"] = first.name
						bad[second.restore].WFWorkflowActionParameters["WFVariable"] = p15FunctionVariable(first.name)
					case "extra_capture_parameter":
						bad[first.capture].WFWorkflowActionParameters["WFCoercion"] = "Text"
					case "external_restore_consumer":
						// A valid preceding edge still cannot disappear from the ordinary graph.
						bad[len(bad)-1].WFWorkflowActionParameters["WFTextActionText"] = p15FunctionEdge(first.restoreID)
					case "duplicate_helper_uuid":
						bad[first.capture].WFWorkflowActionParameters["UUID"] = first.restoreID
					case "orphan_capture":
						extra := p15MeasureVoidClone(t, []ShortcutAction{bad[first.capture]})[0]
						extra.WFWorkflowActionParameters["WFVariableName"] = "__cherri_block_00000000000000000000000000000000_input"
						delete(extra.WFWorkflowActionParameters, "UUID")
						bad = append(bad, extra)
					case "release_instrumentation":
						profile = BuildProfileRelease
					}
					if err := p15MeasureVoidGraphError(bad, ordinary, 2, profile); err == nil {
						t.Fatalf("input boundary oracle accepted %s", fault)
					}
				})
			}
		})
	}
}

func TestP15ConditionalInputBindingIdentityControls(t *testing.T) {
	source := "text(\"BEFORE\")\nmeasure(\"condition\") {\nwait(1)\n}\ntext(\"AFTER\")\n"
	actual, err := compileBuildProfileFixture(t, source, true, BuildProfileDebug)
	if err != nil {
		t.Fatal(err)
	}
	boundaries, err := p15MeasureInputBoundaries(actual, 1, BuildProfileDebug)
	if err != nil || len(boundaries) != 1 {
		t.Fatalf("healthy boundary: %v", err)
	}
	boundary := boundaries[0]
	want, err := p15ConditionSnapshotGraphIdentity(actual, BuildProfileDebug)
	if err != nil {
		t.Fatal(err)
	}
	renamed := p15MeasureVoidClone(t, actual)
	name := "__cherri_block_11111111111111111111111111111111_input"
	if name == boundary.name {
		t.Fatal("fixture private slot collision")
	}
	renamed[boundary.capture].WFWorkflowActionParameters["WFVariableName"] = name
	renamed[boundary.restore].WFWorkflowActionParameters["WFVariable"] = p15FunctionVariable(name)
	t.Run("consistent_private_input_hygiene", func(t *testing.T) {
		got, err := p15ConditionSnapshotGraphIdentity(renamed, BuildProfileDebug)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("private input identity changed: %v", err)
		}
	})
	t.Run("literal_input_spelling_remains_exact", func(t *testing.T) {
		left, right := p15MeasureVoidClone(t, actual), p15MeasureVoidClone(t, renamed)
		left[len(left)-1].WFWorkflowActionParameters["WFTextActionText"] = boundary.name
		right[len(right)-1].WFWorkflowActionParameters["WFTextActionText"] = name
		a, err := p15ConditionSnapshotGraphIdentity(left, BuildProfileDebug)
		if err != nil {
			t.Fatal(err)
		}
		b, err := p15ConditionSnapshotGraphIdentity(right, BuildProfileDebug)
		if err != nil {
			t.Fatal(err)
		}
		if reflect.DeepEqual(a, b) {
			t.Fatal("input normalization rewrote a literal payload")
		}
	})
}
