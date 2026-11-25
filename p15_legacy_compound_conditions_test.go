package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

type p15LegacyLogicOperand struct{ left, op, right string }
type p15LegacyLogicCase struct {
	name, operator, placement string
	operands                  []p15LegacyLogicOperand
	measure                   bool
}

func p15LegacyLogicCases() []p15LegacyLogicCase {
	p := func(left, right string) p15LegacyLogicOperand { return p15LegacyLogicOperand{left, "==", right} }
	return []p15LegacyLogicCase{
		{"and_false_first", "&&", "", []p15LegacyLogicOperand{p("0", "1"), p("1", "1")}, false},
		{"and_false_second", "&&", "", []p15LegacyLogicOperand{p("1", "1"), p("0", "1")}, false},
		{"and_true", "&&", "", []p15LegacyLogicOperand{p("1", "1"), p("1", "1")}, false},
		{"or_false", "||", "", []p15LegacyLogicOperand{p("0", "1"), p("0", "1")}, false},
		{"or_second_true", "||", "", []p15LegacyLogicOperand{p("0", "1"), p("1", "1")}, false},
		{"or_first_true", "||", "", []p15LegacyLogicOperand{p("1", "1"), p("0", "1")}, false},
		{"and_three", "&&", "", []p15LegacyLogicOperand{p("1", "1"), p("0", "1"), p("1", "1")}, false},
		{"or_three", "||", "", []p15LegacyLogicOperand{p("0", "1"), p("1", "1"), p("0", "1")}, false},
		{"and_action_skip", "&&", "", []p15LegacyLogicOperand{p("0", "1"), p("number(7)", "7")}, false},
		{"and_action_take", "&&", "", []p15LegacyLogicOperand{p("1", "1"), p("text(\"YES\")", "\"YES\"")}, false},
		{"or_action_skip", "||", "", []p15LegacyLogicOperand{p("1", "1"), p("text(text(\"YES\"))", "\"YES\"")}, false},
		{"or_action_take", "||", "", []p15LegacyLogicOperand{p("0", "1"), p("number(-3)", "-3")}, false},
		{"and_three_actions", "&&", "", []p15LegacyLogicOperand{p("1", "1"), p("number(0)", "1"), p("number(7)", "7")}, false},
		{"or_three_actions", "||", "", []p15LegacyLogicOperand{p("0", "1"), p("number(1)", "1"), p("number(7)", "7")}, false},
		{"and_effect", "&&", "", []p15LegacyLogicOperand{p("0", "1"), p("measure(\"logic_effect\") {\n@observed = \"CHANGED\"\nnumber(1)\n}", "1")}, true},
		{"or_effect", "||", "", []p15LegacyLogicOperand{p("0", "1"), p("measure(\"logic_effect\") {\n@observed = \"CHANGED\"\nnumber(1)\n}", "1")}, true},
		{"repeat_action", "&&", "repeat", []p15LegacyLogicOperand{p("1", "1"), p("number(7)", "7")}, false},
		{"nested_action", "||", "conditional", []p15LegacyLogicOperand{p("0", "1"), p("number(7)", "7")}, false},
	}
}

func p15LegacyLogicSource(tc p15LegacyLogicCase, version string, explicit bool) string {
	source := "text(\"BEFORE_LOGIC\")\n@observed = \"BEFORE\"\n"
	body := " {\nwait(1)\n} else {\nwait(2)\n}\ntext(@observed)\ntext(\"AFTER_LOGIC\")\n"
	if explicit {
		initial := "1"
		if tc.operator == "||" {
			initial = "0"
		}
		source += "@logicResult = " + initial + "\n"
		for i, operand := range tc.operands {
			source += "if @logicResult == " + initial + " {\n"
			left := operand.left
			if !strings.HasPrefix(left, "@") {
				name := fmt.Sprintf("logicInput%d", i)
				source += "const " + name + " = " + left + "\n"
				left = name
			}
			right := operand.right
			if strings.Contains(right, "(") {
				name := fmt.Sprintf("logicOperand%d", i)
				source += "const " + name + " = " + right + "\n"
				right = name
			}
			source += "if " + left + " " + operand.op + " " + right + " {\n@logicResult = 1\n} else {\n@logicResult = 0\n}\n}\n"
		}
		source += "if @logicResult == 1" + body
	} else {
		parts := make([]string, len(tc.operands))
		for i, operand := range tc.operands {
			parts[i] = operand.left + " " + operand.op + " " + operand.right
		}
		source += "if " + strings.Join(parts, " "+tc.operator+" ") + body
	}
	switch tc.placement {
	case "repeat":
		source = "repeat index for 2 {\n" + source + "}\n"
	case "conditional":
		source = "if 1 == 1 {\n" + source + "}\n"
	}
	return "#define version " + version + "\n" + source
}

var p15LegacyLogicPrivateName = regexp.MustCompile("^__cherri_condition_[0-9a-f]{32}$")

// Compare the complete serialized native graph with an independently authored
// sequence of ordinary single conditions. Only the validated private carrier's
// declaration/reference spelling and existing graph identities are alpha names.
// This proves compiler structure, never an execution of Apple's runtime.
func p15LegacyLogicIdentity(actions []ShortcutAction, tc p15LegacyLogicCase, profile BuildProfile, explicit bool) ([]map[string]any, error) {
	var rows []map[string]any
	var err error
	if tc.measure {
		rows, err = p15ConditionSnapshotGraphIdentity(actions, profile)
	} else {
		rows, err = p15ConditionGraphIdentity(actions)
	}
	if err != nil {
		return nil, err
	}
	name := "logicResult"
	if !explicit {
		names := map[string]bool{}
		for _, row := range rows {
			p, _ := row["WFWorkflowActionParameters"].(map[string]any)
			n, _ := p["WFVariableName"].(string)
			if row["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p15LegacyLogicPrivateName.MatchString(n) {
				names[n] = true
			}
		}
		if len(names) != 1 {
			return nil, fmt.Errorf("expected one hygienic compound result carrier, got %d", len(names))
		}
		for n := range names {
			name = n
		}
	}
	writes, reads := 0, 0
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == "Variable" && v["VariableName"] == name {
				reads++
				v["VariableName"] = "logical-result-carrier"
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	for _, row := range rows {
		p, _ := row["WFWorkflowActionParameters"].(map[string]any)
		if row["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == name {
			writes++
			p["WFVariableName"] = "logical-result-carrier"
		}
		walk(row)
	}
	if writes != 1+2*len(tc.operands) || reads != 1+len(tc.operands) {
		return nil, fmt.Errorf("carrier declaration/read cardinality: writes=%d reads=%d", writes, reads)
	}
	return rows, nil
}

func TestP15LegacyCompoundWrittenConditions(t *testing.T) {
	for index, tc := range p15LegacyLogicCases() {
		versions := []string{"17", "27"}
		if index < 8 {
			versions = []string{"17"}
		} // Modern pure predicates retain their existing native filter encoding.
		for _, version := range versions {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(tc.name+"/v"+version+"/"+string(profile), func(t *testing.T) {
					control, err := compileBuildProfileFixture(t, p15LegacyLogicSource(tc, version, true), true, profile)
					if err != nil {
						t.Fatalf("independent single-condition source control: %v", err)
					}
					want, err := p15LegacyLogicIdentity(control, tc, profile, true)
					if err != nil {
						t.Fatalf("independent single-condition graph control: %v", err)
					}
					actual, err := compileBuildProfileFixture(t, p15LegacyLogicSource(tc, version, false), true, profile)
					if err != nil {
						t.Fatalf("written compound: %v", err)
					}
					got, err := p15LegacyLogicIdentity(actual, tc, profile, false)
					if err != nil {
						t.Fatalf("written compound graph: %v", err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("compound lost typed comparison, lazy producer placement, order or final consumer\ngot=%v\nwant=%v", got, want)
					}
				})
			}
		}
	}
}

func TestP15LegacyCompoundIndependentControls(t *testing.T) {
	for _, index := range []int{8, 11, 14, 15} {
		tc := p15LegacyLogicCases()[index]
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(tc.name+"/"+string(profile), func(t *testing.T) {
				a, err := compileBuildProfileFixture(t, p15LegacyLogicSource(tc, "17", true), true, profile)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := p15LegacyLogicIdentity(a, tc, profile, true); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15LegacyCompoundCausalGraphControls(t *testing.T) {
	tc := p15LegacyLogicCases()[8]
	control, err := compileBuildProfileFixture(t, p15LegacyLogicSource(tc, "17", true), true, BuildProfileRelease)
	if err != nil {
		t.Fatal(err)
	}
	want, err := p15LegacyLogicIdentity(control, tc, BuildProfileRelease, true)
	if err != nil {
		t.Fatal(err)
	}
	var starts []int
	later := -1
	for i, action := range control {
		p := action.WFWorkflowActionParameters
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0 {
			starts = append(starts, i)
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.number" && fmt.Sprint(p["WFNumberActionNumber"]) == "7" {
			later = i
		}
	}
	if len(starts) != 5 || later < 0 {
		t.Fatal("independent control shape missing")
	}
	for _, fault := range []string{"wrong_guard", "drop_later", "eager_later", "duplicate_later", "wrong_comparison", "wrong_group", "wrong_carrier", "wrong_literal"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15C03Clone(t, control)
			switch fault {
			case "wrong_guard":
				bad[starts[2]].WFWorkflowActionParameters["WFNumberValue"] = 0
			case "drop_later":
				bad = append(bad[:later], bad[later+1:]...)
			case "eager_later":
				a := bad[later]
				bad = append(bad[:later], bad[later+1:]...)
				bad = append([]ShortcutAction{a}, bad...)
			case "duplicate_later":
				bad = append(bad, bad[later])
			case "wrong_comparison":
				bad[starts[3]].WFWorkflowActionParameters["WFNumberValue"] = 99
			case "wrong_group":
				bad[starts[2]].WFWorkflowActionParameters["GroupingIdentifier"] = "foreign-group"
			case "wrong_carrier":
				for i := range bad {
					if bad[i].WFWorkflowActionParameters["WFVariableName"] == "logicResult" {
						bad[i].WFWorkflowActionParameters["WFVariableName"] = "foreign-result"
						break
					}
				}
			case "wrong_literal":
				for i := range bad {
					if bad[i].WFWorkflowActionParameters["WFTextActionText"] == "BEFORE_LOGIC" {
						bad[i].WFWorkflowActionParameters["WFTextActionText"] = "WRONG_LOGIC"
						break
					}
				}
			}
			got, err := p15LegacyLogicIdentity(bad, tc, BuildProfileRelease, true)
			if err == nil && reflect.DeepEqual(got, want) {
				t.Fatal("compound oracle accepted", fault)
			}
		})
	}
}
