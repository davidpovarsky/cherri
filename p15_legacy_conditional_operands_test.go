package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type p15ConditionOperandCase struct {
	name, left, op, right, third string
}

var p15ConditionOperandCases = []p15ConditionOperandCase{
	{"number_action", "7", "==", "number(7)", ""},
	{"number_order", "7", ">", "number(6)", ""},
	{"negative_action", "-3", "==", "number(-3)", ""},
	{"decimal_action", "1.5", "==", "number(1.5)", ""},
	{"text_action", "\"RIGHT\"", "==", "text(\"RIGHT\")", ""},
	{"raw_unicode_action", "'שלום 🍒'", "==", "text(\"שלום 🍒\")", ""},
	{"both_actions", "number(7)", "==", "number(7)", ""},
	{"nested_action", "text(\"RIGHT\")", "==", "text(text(\"RIGHT\"))", ""},
	{"expression", "3", "==", "(1 + 2)", ""},
	{"between_actions", "2", "<>", "number(1)", "number(3)"},
	{"between_upper_action", "2", "<>", "1", "number(3)"},
	{"between_expression", "2", "<>", "(0 + 1)", "(2 + 1)"},
}

func p15ConditionOperandSource(tc p15ConditionOperandCase, version string, explicit bool) string {
	source := "#define version " + version + "\ntext(\"BEFORE_OPERANDS\")\n"
	left, right, third := tc.left, tc.right, tc.third
	if explicit {
		source += "const operandLeft = " + left + "\n"
		left = "operandLeft"
		// Preserve literal comparison fields; only authored expression producers
		// become explicit references in the independent source control.
		if strings.Contains(right, "(") {
			source += "const operandRight = " + right + "\n"
			right = "operandRight"
		}
		if strings.Contains(third, "(") {
			source += "const operandThird = " + third + "\n"
			third = "operandThird"
		}
	}
	source += "if " + left + " " + tc.op + " " + right
	if third != "" {
		source += " " + third
	}
	return source + " {\nwait(1)\n} else {\nwait(2)\n}\ntext(\"AFTER_OPERANDS\")\n"
}

func TestP15LegacyConditionalWrittenOperands(t *testing.T) {
	for _, tc := range p15ConditionOperandCases {
		for _, version := range []string{"17", "27"} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(tc.name+"/v"+version+"/"+string(profile), func(t *testing.T) {
					control, err := compileBuildProfileFixture(t, p15ConditionOperandSource(tc, version, true), true, profile)
					if err != nil {
						t.Fatalf("explicit operand control: %v", err)
					}
					want, err := p15ConditionGraphIdentity(control)
					if err != nil {
						t.Fatalf("explicit operand graph: %v", err)
					}
					actual, err := compileBuildProfileFixture(t, p15ConditionOperandSource(tc, version, false), true, profile)
					if err != nil {
						t.Fatalf("written conditional operands: %v", err)
					}
					got, err := p15ConditionGraphIdentity(actual)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("written operand producer/order/typed reference differs from explicit source\ngot=%v\nwant=%v", got, want)
					}
				})
			}
		}
	}
}

func TestP15LegacyConditionalOperandSnapshots(t *testing.T) {
	for _, version := range []string{"17", "27"} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run("v"+version+"/"+string(profile), func(t *testing.T) {
				prefix := "#define version " + version + "\n@snapshot = \"BEFORE\"\n"
				later := "measure(\"condition_later\") {\n@snapshot = \"AFTER\"\ntext(\"BEFORE\")\n}"
				body := " {\nwait(1)\n} else {\nwait(2)\n}\ntext(\"AFTER_CONDITION\")\n"
				control := prefix + "const captured = getVariable(@snapshot)\nconst later = " + later + "\nif captured == later" + body
				source := prefix + "if @snapshot == " + later + body
				actions, err := compileBuildProfileFixture(t, control, true, profile)
				if err != nil {
					t.Fatalf("explicit source-order control: %v", err)
				}
				want, err := p15ConditionGraphIdentity(actions)
				if err != nil {
					t.Fatal(err)
				}
				actions, err = compileBuildProfileFixture(t, source, true, profile)
				if err != nil {
					t.Fatalf("written source-order condition: %v", err)
				}
				got, err := p15ConditionGraphIdentity(actions)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("mutable LHS was not captured before the later RHS producer\ngot=%v\nwant=%v", got, want)
				}
			})
		}
	}
}

func TestP15LegacyConditionalVoidOperandBoundary(t *testing.T) {
	for _, tc := range []struct{ name, condition string }{
		{"left", "wait(1) == 1"},
		{"right", "1 == wait(1)"},
		{"lower_bound", "2 <> wait(1) 3"},
		{"upper_bound", "2 <> 1 wait(1)"},
	} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(tc.name+"/"+string(profile), func(t *testing.T) {
				if _, err := compileBuildProfileFixture(t, "#define version 27\nif 1 == 1 {\nwait(1)\n}\n", true, profile); err != nil {
					t.Fatalf("healthy condition boundary control: %v", err)
				}
				_, err := compileBuildProfileFixture(t, "#define version 27\nif "+tc.condition+" {\nwait(1)\n}\n", true, profile)
				if err == nil || !strings.Contains(err.Error(), "E_ARGUMENT_VALUE_REQUIRED") {
					t.Fatalf("void operand must fail at the value boundary, got %v", err)
				}
			})
		}
	}
}

func TestP15LegacyConditionalPureCompoundInputs(t *testing.T) {
	for _, operator := range []string{"&&", "||"} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("%s/%s", operator, profile), func(t *testing.T) {
				body := " {\nwait(1)\n} else {\nwait(2)\n}\n"
				control := "#define version 27\nconst a = 1\nconst b = \"שלום\"\nif a == 1 " + operator + " b == \"שלום\"" + body
				source := "#define version 27\nif 1 == 1 " + operator + " \"שלום\" == \"שלום\"" + body
				actions, err := compileBuildProfileFixture(t, control, true, profile)
				if err != nil {
					t.Fatalf("pure compound control: %v", err)
				}
				want, err := p15ConditionGraphIdentity(actions)
				if err != nil {
					t.Fatal(err)
				}
				actions, err = compileBuildProfileFixture(t, source, true, profile)
				if err != nil {
					t.Fatal(err)
				}
				got, err := p15ConditionGraphIdentity(actions)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("pure modern native filter changed\ngot=%v\nwant=%v", got, want)
				}
			})
		}
	}
}

func TestP15LegacyConditionalOperandOracleControls(t *testing.T) {
	control, err := compileBuildProfileFixture(t, p15ConditionOperandSource(p15ConditionOperandCases[1], "27", true), true, BuildProfileDebug)
	if err != nil {
		t.Fatal(err)
	}
	want, err := p15ConditionGraphIdentity(control)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"drop_right", "duplicate_right", "swap_producers", "wrong_right_value", "wrong_right_type", "foreign_right_edge", "wrong_right_name"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15C03Clone(t, control)
			var numbers []int
			for i, a := range bad {
				if strings.HasSuffix(a.WFWorkflowActionIdentifier, ".number") {
					numbers = append(numbers, i)
				}
			}
			if len(numbers) != 2 {
				t.Fatalf("expected two explicit number producers, got %v", numbers)
			}
			left, right := numbers[0], numbers[1]
			switch fault {
			case "drop_right":
				bad = append(bad[:right], bad[right+1:]...)
			case "duplicate_right":
				bad = append(bad, bad[right])
			case "swap_producers":
				bad[left], bad[right] = bad[right], bad[left]
			case "wrong_right_value":
				bad[right].WFWorkflowActionParameters["WFNumberActionNumber"] = 99
			case "wrong_right_type":
				bad[right].WFWorkflowActionIdentifier = "is.workflow.actions.gettext"
			case "foreign_right_edge":
				bad[right].WFWorkflowActionParameters["UUID"] = "foreign-right-producer"
			case "wrong_right_name":
				bad[right].WFWorkflowActionParameters["CustomOutputName"] = "foreign-right-name"
			}
			got, err := p15ConditionGraphIdentity(bad)
			if err == nil && reflect.DeepEqual(got, want) {
				t.Fatal("operand graph oracle accepted", fault)
			}
		})
	}
}
