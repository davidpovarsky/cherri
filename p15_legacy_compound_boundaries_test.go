package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestP15LegacyCompoundRightOperandGuards(t *testing.T) {
	cases := []p15LegacyLogicCase{
		{name: "rhs_and_skip", operator: "&&", operands: []p15LegacyLogicOperand{{"0", "==", "1"}, {"7", "==", "number(7)"}}},
		{name: "rhs_or_take", operator: "||", operands: []p15LegacyLogicOperand{{"0", "==", "1"}, {"7", "==", "number(7)"}}},
		{name: "both_sides_skip", operator: "&&", operands: []p15LegacyLogicOperand{{"0", "==", "1"}, {"number(7)", "==", "number(7)"}}},
	}
	for _, tc := range cases {
		for _, version := range []string{"17", "18", "27"} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(tc.name+"/v"+version+"/"+string(profile), func(t *testing.T) {
					control, err := compileBuildProfileFixture(t, p15LegacyLogicSource(tc, version, true), true, profile)
					if err != nil {
						t.Fatalf("independent RHS source control: %v", err)
					}
					want, err := p15LegacyLogicIdentity(control, tc, profile, true)
					if err != nil {
						t.Fatalf("independent RHS graph control: %v", err)
					}
					actual, err := compileBuildProfileFixture(t, p15LegacyLogicSource(tc, version, false), true, profile)
					if err != nil {
						t.Fatalf("written compound RHS: %v", err)
					}
					got, err := p15LegacyLogicIdentity(actual, tc, profile, false)
					if err != nil {
						t.Fatalf("written compound RHS graph: %v", err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("later RHS producer escaped its guard or changed ordering\ngot=%v\nwant=%v", got, want)
					}
				})
			}
		}
	}
}

func TestP15LegacyCompoundModernPureBoundary(t *testing.T) {
	for _, version := range []string{"18", "27"} {
		for _, operator := range []string{"&&", "||"} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("v%s/%s/%s", version, operator, profile), func(t *testing.T) {
					prefix := "#define version " + version + "\n"
					body := " {\nwait(1)\n} else {\nwait(2)\n}\n"
					control, err := compileBuildProfileFixture(t, prefix+"const a = 1\nconst b = \"שלום\"\nif a == 1 "+operator+" b == \"שלום\""+body, true, profile)
					if err != nil {
						t.Fatal(err)
					}
					want, err := p15ConditionGraphIdentity(control)
					if err != nil {
						t.Fatal(err)
					}
					actual, err := compileBuildProfileFixture(t, prefix+"if 1 == 1 "+operator+" \"שלום\" == \"שלום\""+body, true, profile)
					if err != nil {
						t.Fatal(err)
					}
					got, err := p15ConditionGraphIdentity(actual)
					if err != nil {
						t.Fatal(err)
					}
					nativeFilters := 0
					for _, action := range actual {
						if action.WFWorkflowActionParameters["WFConditions"] != nil {
							nativeFilters++
						}
					}
					if nativeFilters != 1 || !reflect.DeepEqual(got, want) {
						t.Fatalf("pure native filter boundary changed: filters=%d\ngot=%v\nwant=%v", nativeFilters, got, want)
					}
				})
			}
		}
	}
}

func TestP15LegacyCompoundMixedGrammarBoundary(t *testing.T) {
	for _, version := range []string{"17", "18", "27"} {
		for i, mixed := range []string{"1 == 1 && 0 == 1 || 1 == 1", "1 == 1 || 0 == 1 && 1 == 1"} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("v%s/mixed%d/%s", version, i, profile), func(t *testing.T) {
					source := "#define version " + version + "\nif " + mixed + " {\nwait(1)\n}\n"
					_, err := compileBuildProfileFixture(t, source, true, profile)
					if err == nil || !strings.Contains(err.Error(), "only the initially set logical operator") {
						t.Fatalf("Legacy mixed-operator boundary changed or flattened: %v", err)
					}
				})
			}
		}
	}
}
