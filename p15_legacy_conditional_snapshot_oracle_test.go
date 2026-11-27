package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var p15ConditionalTimerName = regexp.MustCompile("^__cherri_debug_timer_[0-9a-f]{16}$")

// A measure timer is a compiler-owned mutable binding whose hygiene salt
// depends on source position. Normalize only its declaration and verified
// native Variable references, never an equal-looking literal payload.
func p15ConditionSnapshotGraphIdentity(actions []ShortcutAction, profile BuildProfile) ([]map[string]any, error) {
	boundaries, err := p15MeasureInputBoundaries(actions, 1, profile)
	if err != nil {
		return nil, err
	}
	rows, err := p15ConditionGraphIdentity(actions)
	if err != nil {
		return nil, err
	}
	timers := map[string]int{}
	for i, row := range rows {
		params, _ := row["WFWorkflowActionParameters"].(map[string]any)
		name, _ := params["WFVariableName"].(string)
		if !p15ConditionalTimerName.MatchString(name) {
			continue
		}
		if row["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" ||
			p15OutputReference(params["WFInput"])["Type"] != "CurrentDate" {
			return nil, fmt.Errorf("timer declaration lost its exact native CurrentDate input")
		}
		if _, duplicate := timers[name]; duplicate {
			return nil, fmt.Errorf("duplicate timer declaration")
		}
		timers[name] = i
	}
	expected := 0
	if profile == BuildProfileDebug {
		expected = 1
	}
	if len(timers) != expected {
		return nil, fmt.Errorf("expected %d authored measure timers, got %d", expected, len(timers))
	}
	reads := map[string]int{}
	var walk func(any, int) error
	walk = func(value any, consumer int) error {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == "Variable" {
				name, _ := v["VariableName"].(string)
				if strings.HasPrefix(name, "__cherri_debug_timer_") {
					producer, found := timers[name]
					if !found || producer >= consumer {
						return fmt.Errorf("timer reference has no preceding declaration")
					}
					reads[name]++
					v["VariableName"] = fmt.Sprintf("private-timer-at-%d", producer)
				}
			}
			for _, child := range v {
				if err := walk(child, consumer); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := walk(child, consumer); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for i, row := range rows {
		if err := walk(row, i); err != nil {
			return nil, err
		}
	}
	for name, index := range timers {
		if reads[name] != 1 {
			return nil, fmt.Errorf("measure timer requires one exact native read, got %d", reads[name])
		}
		params := rows[index]["WFWorkflowActionParameters"].(map[string]any)
		params["WFVariableName"] = fmt.Sprintf("private-timer-at-%d", index)
	}
	for _, boundary := range boundaries {
		name := fmt.Sprintf("private-input-at-%d", boundary.capture)
		capture := rows[boundary.capture]["WFWorkflowActionParameters"].(map[string]any)
		restore := rows[boundary.restore]["WFWorkflowActionParameters"].(map[string]any)
		capture["WFVariableName"] = name
		ref := restore["WFVariable"].(map[string]any)["Value"].(map[string]any)
		ref["VariableName"] = name
	}
	return rows, nil
}

func p15ConditionalRenameTimerReferences(value any, from, to string) {
	switch v := value.(type) {
	case map[string]any:
		if v["Type"] == "Variable" && v["VariableName"] == from {
			v["VariableName"] = to
		}
		for _, child := range v {
			p15ConditionalRenameTimerReferences(child, from, to)
		}
	case []any:
		for _, child := range v {
			p15ConditionalRenameTimerReferences(child, from, to)
		}
	}
}

func TestP15LegacyConditionalSnapshotIdentityControls(t *testing.T) {
	source := "#define version 27\n@snapshot = \"BEFORE\"\nconst captured = getVariable(@snapshot)\nconst later = measure(\"condition_later\") {\n@snapshot = \"AFTER\"\ntext(\"BEFORE\")\n}\nif captured == later {\nwait(1)\n} else {\nwait(2)\n}\ntext(\"AFTER_CONDITION\")\n"
	control, err := compileBuildProfileFixture(t, source, true, BuildProfileDebug)
	if err != nil {
		t.Fatal(err)
	}
	want, err := p15ConditionSnapshotGraphIdentity(control, BuildProfileDebug)
	if err != nil {
		t.Fatal(err)
	}
	declaration, timer := -1, ""
	for i, action := range control {
		name, _ := action.WFWorkflowActionParameters["WFVariableName"].(string)
		if p15ConditionalTimerName.MatchString(name) {
			declaration, timer = i, name
		}
	}
	if declaration < 0 {
		t.Fatal("healthy timer declaration missing")
	}
	for _, fault := range []string{"drop_timer", "duplicate_timer", "wrong_clock", "foreign_timer_reference", "authored_variable_reference", "timer_after_read"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15C03Clone(t, control)
			switch fault {
			case "drop_timer":
				bad = append(bad[:declaration], bad[declaration+1:]...)
			case "duplicate_timer":
				bad = append(bad, bad[declaration])
			case "wrong_clock":
				bad[declaration].WFWorkflowActionParameters["WFInput"] = map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "Clipboard"},
				}
			case "foreign_timer_reference", "authored_variable_reference":
				name := "__cherri_debug_timer_0000000000000000"
				if fault == "authored_variable_reference" {
					name = "snapshot"
				}
				for i := range bad {
					p15ConditionalRenameTimerReferences(bad[i].WFWorkflowActionParameters, timer, name)
				}
			case "timer_after_read":
				row := bad[declaration]
				bad = append(bad[:declaration], bad[declaration+1:]...)
				bad = append(bad, row)
			}
			got, err := p15ConditionSnapshotGraphIdentity(bad, BuildProfileDebug)
			if err == nil && reflect.DeepEqual(got, want) {
				t.Fatal("timer identity oracle accepted", fault)
			}
		})
	}
	renamed := p15C03Clone(t, control)
	newName := "__cherri_debug_timer_1111111111111111"
	if timer == newName {
		t.Fatal("test timer salt unexpectedly collided")
	}
	renamed[declaration].WFWorkflowActionParameters["WFVariableName"] = newName
	for i := range renamed {
		p15ConditionalRenameTimerReferences(renamed[i].WFWorkflowActionParameters, timer, newName)
	}
	t.Run("consistent_private_hygiene", func(t *testing.T) {
		got, err := p15ConditionSnapshotGraphIdentity(renamed, BuildProfileDebug)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("consistent private binding identity was not preserved: %v", err)
		}
	})
	t.Run("literal_timer_spelling_remains_exact", func(t *testing.T) {
		a, b := p15C03Clone(t, control), p15C03Clone(t, renamed)
		a = append(a, ShortcutAction{WFWorkflowActionIdentifier: "is.workflow.actions.gettext",
			WFWorkflowActionParameters: map[string]any{"WFTextActionText": timer}})
		b = append(b, ShortcutAction{WFWorkflowActionIdentifier: "is.workflow.actions.gettext",
			WFWorkflowActionParameters: map[string]any{"WFTextActionText": newName}})
		left, err := p15ConditionSnapshotGraphIdentity(a, BuildProfileDebug)
		if err != nil {
			t.Fatal(err)
		}
		right, err := p15ConditionSnapshotGraphIdentity(b, BuildProfileDebug)
		if err != nil {
			t.Fatal(err)
		}
		if reflect.DeepEqual(left, right) {
			t.Fatal("private identity normalization rewrote a literal value")
		}
	})
}
