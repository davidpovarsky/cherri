package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestP15MeasureVoidRejectsConsumptionAndImplicitControlValues(t *testing.T) {
	cases := []struct{ name, v2, legacy, wantV2, wantLegacy string }{
		{"binding_empty", "let result = measure(\"empty\") {}\n", "const result = measure(\"empty\") {\n}\n", "E_BLOCK_VOID_VALUE", "E_BLOCK_VOID_VALUE"},
		{"mutable_binding", "var result = measure(\"wait\") { wait(1) }\n", "@result = measure(\"wait\") {\nwait(1)\n}\n", "E_BLOCK_VOID_VALUE", "E_BLOCK_VOID_VALUE"},
		{"argument", "getVariable(measure(\"wait\") { wait(1) })\n", "getVariable(measure(\"wait\") {\nwait(1)\n})\n", "E_ARGUMENT_TYPE", "E_BLOCK_VALUE_REQUIRED"},
		{"nested_binding", "measure(\"outer\") { let result = measure(\"inner\") { wait(1) }; text(\"value\") }\n", "measure(\"outer\") {\nconst result = measure(\"inner\") {\nwait(1)\n}\ntext(\"value\")\n}\n", "E_BLOCK_VOID_VALUE", "E_BLOCK_VOID_VALUE"},
		{"return_value", "function value() -> Text { return measure(\"empty\") {} }\nlet result = value()\n", "output(measure(\"empty\") {\n})\n", "E_RETURN_TYPE", "E_BLOCK_VALUE_REQUIRED"},
		{"explicit_void_yield", "measure(\"yield\") { yield wait(1) }\n", "measure(\"yield\") {\nyield wait(1)\n}\n", "E_BLOCK_VALUE_REQUIRED", "E_BLOCK_VALUE_REQUIRED"},
		{"if_text_value", "measure(\"if\") { if true { text(\"one\") } else { text(\"two\") } }\n", "const condition = 1\nmeasure(\"if\") {\nif condition == 1 {\ntext(\"one\")\n} else {\ntext(\"two\")\n}\n}\n", "E_BLOCK_CONTROL_RESULT", "E_BLOCK_CONTROL_RESULT"},
		{"if_unknown_value", "measure(\"if\") { if true { run(\"CHERRI_UNKNOWN\") } else { wait(1) } }\n", "const condition = 1\nmeasure(\"if\") {\nif condition == 1 {\nrun(\"CHERRI_UNKNOWN\")\n} else {\nwait(1)\n}\n}\n", "E_BLOCK_CONTROL_RESULT", "E_BLOCK_CONTROL_RESULT"},
		{"repeat_text_value", "measure(\"repeat\") { repeat 2 { text(\"value\") } }\n", "measure(\"repeat\") {\nrepeat index for 2 {\ntext(\"value\")\n}\n}\n", "E_BLOCK_CONTROL_RESULT", "E_BLOCK_CONTROL_RESULT"},
		{"nested_control_value", "measure(\"nested\") { if true { repeat 2 { text(\"value\") } } else { wait(1) } }\n", "const condition = 1\nmeasure(\"nested\") {\nif condition == 1 {\nrepeat index for 2 {\ntext(\"value\")\n}\n} else {\nwait(1)\n}\n}\n", "E_BLOCK_CONTROL_RESULT", "E_BLOCK_CONTROL_RESULT"},
	}
	for _, c := range cases {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy_%t/%s", c.name, legacy, profile), func(t *testing.T) {
					src, want := c.v2, c.wantV2
					if legacy {
						src, want = c.legacy, c.wantLegacy
					}
					actions, err := compileBuildProfileFixture(t, src, legacy, profile)
					if err == nil || !strings.Contains(err.Error(), want) || len(actions) != 0 {
						t.Fatalf("unsupported consumption/control reached emission: want=%s actions=%v err=%v", want, actions, err)
					}
					// Rejection must not poison the compiler's next real compilation.
					ordinary, err := compileBuildProfileFixture(t, "text(\"CHERRI_AFTER_REJECTION\")\n", legacy, profile)
					if err != nil || len(p15ActionsNamed(ordinary, "gettext")) != 1 {
						t.Fatalf("rejected block leaked compiler state: %v %v", ordinary, err)
					}
				})
			}
		}
	}
}

func p15MeasureVoidClone(t *testing.T, actions []ShortcutAction) []ShortcutAction {
	t.Helper()
	data, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	var result []ShortcutAction
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func p15MeasureVoidIndex(t *testing.T, actions []ShortcutAction, suffix string, last bool) int {
	t.Helper()
	result := -1
	for i, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions."+suffix {
			result = i
			if !last {
				break
			}
		}
	}
	if result < 0 {
		t.Fatalf("healthy graph lost %s", suffix)
	}
	return result
}

func TestP15MeasureVoidGraphRejectsCausalMutations(t *testing.T) {
	common := []string{"lost_effect", "duplicate_effect", "changed_delay", "reverse_effects", "lost_boundary", "boundary_input", "boundary_output", "early_boundary", "lost_authored_nothing"}
	debug := []string{"wrong_start_clock", "wrong_stop_origin", "wrong_elapsed_origin", "wrong_elapsed_name", "stop_extra_parameter", "date_extra_reference", "late_start", "early_stop"}
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy_%t/%s", legacy, profile), func(t *testing.T) {
				body := "nothing()\nwait(1)\nwait(2)\n"
				ordinary, err := compileBuildProfileFixture(t, p15MeasureVoidFlat(body), legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := compileBuildProfileFixture(t, p15MeasureVoidWrapped(body), legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				if err = p15MeasureVoidGraphError(actual, ordinary, 1, profile); err != nil {
					t.Fatalf("healthy producer must pass before any mutation: %v", err)
				}
				faults := append([]string{}, common...)
				if profile == BuildProfileDebug {
					faults = append(faults, debug...)
				}
				for _, fault := range faults {
					t.Run(fault, func(t *testing.T) {
						bad := p15MeasureVoidClone(t, actual)
						firstDelay := p15MeasureVoidIndex(t, bad, "delay", false)
						lastDelay := p15MeasureVoidIndex(t, bad, "delay", true)
						firstNothing := p15MeasureVoidIndex(t, bad, "nothing", false)
						after := -1
						for i, action := range bad {
							if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && action.WFWorkflowActionParameters["WFTextActionText"] == "CHERRI_AFTER" {
								after = i
								break
							}
						}
						if after < 1 {
							t.Fatal("healthy source boundary missing")
						}
						boundary := after - 1
						switch fault {
						case "lost_effect":
							bad = append(bad[:firstDelay], bad[firstDelay+1:]...)
						case "duplicate_effect":
							bad = append(bad[:firstDelay], append([]ShortcutAction{bad[firstDelay]}, bad[firstDelay:]...)...)
						case "changed_delay":
							bad[firstDelay].WFWorkflowActionParameters["WFDelayTime"] = float64(99)
						case "reverse_effects":
							bad[firstDelay], bad[lastDelay] = bad[lastDelay], bad[firstDelay]
						case "lost_boundary":
							bad = append(bad[:boundary], bad[boundary+1:]...)
						case "boundary_input":
							if bad[boundary].WFWorkflowActionParameters == nil {
								bad[boundary].WFWorkflowActionParameters = map[string]any{}
							}
							bad[boundary].WFWorkflowActionParameters["WFInput"] = "unexpected prior value"
						case "boundary_output":
							if bad[boundary].WFWorkflowActionParameters == nil {
								bad[boundary].WFWorkflowActionParameters = map[string]any{}
							}
							bad[boundary].WFWorkflowActionParameters["UUID"] = "invented-void-result"
						case "early_boundary":
							bad[boundary], bad[lastDelay] = bad[lastDelay], bad[boundary]
						case "lost_authored_nothing":
							bad = append(bad[:firstNothing], bad[firstNothing+1:]...)
						default:
							start := -1
							for i, action := range bad {
								if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && strings.HasPrefix(fmt.Sprint(action.WFWorkflowActionParameters["WFVariableName"]), "__cherri_debug_timer_") {
									start = i
									break
								}
							}
							stop := p15MeasureVoidIndex(t, bad, "gettimebetweendates", false)
							if start < 0 {
								t.Fatal("healthy timer start missing")
							}
							switch fault {
							case "wrong_start_clock":
								ref, err := p15MeasureVoidAttachment(bad[start].WFWorkflowActionParameters["WFInput"])
								if err != nil {
									t.Fatal(err)
								}
								ref["Type"] = "Clipboard"
							case "wrong_stop_origin":
								ref, err := p15MeasureVoidDateString(bad[stop].WFWorkflowActionParameters["WFTimeUntilFromDate"])
								if err != nil {
									t.Fatal(err)
								}
								ref["VariableName"] = "unrelated"
							case "wrong_elapsed_origin":
								ref, err := p15MeasureVoidAttachment(bad[stop+1].WFWorkflowActionParameters["WFInput"])
								if err != nil {
									t.Fatal(err)
								}
								ref["OutputUUID"] = "wrong"
							case "wrong_elapsed_name":
								ref, err := p15MeasureVoidAttachment(bad[stop+2].WFWorkflowActionParameters["WFVariable"])
								if err != nil {
									t.Fatal(err)
								}
								ref["OutputName"] = "other"
							case "stop_extra_parameter":
								bad[stop].WFWorkflowActionParameters["WFCoercion"] = "Text"
							case "date_extra_reference":
								ref, err := p15MeasureVoidDateString(bad[stop].WFWorkflowActionParameters["WFInput"])
								if err != nil {
									t.Fatal(err)
								}
								ref["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
							case "late_start":
								action := bad[start]
								bad = append(bad[:start], bad[start+1:]...)
								position := firstDelay
								bad = append(bad[:position], append([]ShortcutAction{action}, bad[position:]...)...)
							case "early_stop":
								timer := append([]ShortcutAction{}, bad[stop:stop+3]...)
								bad = append(bad[:stop], bad[stop+3:]...)
								bad = append(bad[:lastDelay], append(timer, bad[lastDelay:]...)...)
							}
						}
						if err = p15MeasureVoidGraphError(bad, ordinary, 1, profile); err == nil {
							t.Fatalf("oracle accepted %s", fault)
						}
					})
				}
			})
		}
	}
}
