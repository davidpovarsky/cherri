package main

import "testing"

// Mutations exercise the independently authored complete contextual graph, not
// the implementation's choice of code path or only a marker/count predicate.
func TestP15TaggedContextualGraphCorruptions(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		depth        int
		dictionary   bool
	}{
		{"nested", "let values:List<List<Optional<Bool>>>=[[false]]\nreturn count(values)", 2, false},
		{"deep", "let values:List<List<List<Optional<Bool>>>>=[[[false]]]\nreturn count(values)", 3, false},
		{"dictionary_array", "let values:Map<Text,List<Optional<Bool>>>={\"slot\":[false]}\nreturn 1", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15TaggedContextualGraph(actions, tc.depth, tc.dictionary); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"kind", "presence", "false_value", "wrapper_drop", "marker", "producer_identity", "omit_inner", "source_order", "extra_action"} {
				t.Run(fault, func(t *testing.T) {
					broken := p15TaggedClone(t, actions)
					switch fault {
					case "kind", "presence", "false_value", "wrapper_drop", "marker":
						fields, err := p15ContainerFields(p15Param(broken[0])["WFItems"])
						if err != nil {
							t.Fatal(err)
						}
						key, value := "kind", any(p15FunctionText("text"))
						switch fault {
						case "presence":
							key, value = "has_value", p15FunctionText("0")
						case "false_value":
							key = "value"
							state, ok := fields[key]["WFValue"].(map[string]any)
							if !ok {
								t.Fatal("original Boolean state wrapper missing")
							}
							state["Value"] = true
							value = state
						case "wrapper_drop":
							key, value = "value", false
						case "marker":
							key, value = "_cherri_value_cell_v1", p15FunctionText("unowned")
						}
						fields[key]["WFValue"] = value
					case "producer_identity":
						p15Param(broken[0])["UUID"] = "UNRELATED_CHILD"
					case "omit_inner":
						broken = broken[1:]
					case "source_order":
						broken[0], broken[1] = broken[1], broken[0]
					case "extra_action":
						broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					}
					if p15TaggedContextualGraph(broken, tc.depth, tc.dictionary) == nil {
						t.Fatal("accepted corruption: " + fault)
					}
				})
			}
		})
	}
}

func TestP15TaggedDefaultGraphCorruptions(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		dictionary   bool
	}{
		{"list", "function first(values:List<Optional<Bool>>=[false])->Bool {let item=values[0]\nreturn item==none}\nreturn first()", false},
		{"map", "function first(values:Map<Text,Optional<Bool>>={\"flag\":false})->Bool {let item=values[\"flag\"]\nreturn item==none}\nreturn first()", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15TaggedDefaultGraph(actions, tc.dictionary); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"presence", "false_value", "wrapper_drop", "kind", "producer_identity", "omitted_discriminator", "wrong_polarity", "wrong_group", "extra_fallback"} {
				t.Run(fault, func(t *testing.T) {
					broken := p15TaggedClone(t, actions)
					region, err := p15TaggedDefaultRegion(broken)
					if err != nil || len(region) != 2 {
						t.Fatal("invalid original fallback", err)
					}
					if fault == "presence" || fault == "false_value" || fault == "wrapper_drop" || fault == "kind" {
						fields, err := p15ContainerFields(p15Param(region[0])["WFItems"])
						if err != nil {
							t.Fatal(err)
						}
						key, value := "has_value", any(p15FunctionText("0"))
						if fault == "false_value" {
							key = "value"
							state, ok := fields[key]["WFValue"].(map[string]any)
							if !ok {
								t.Fatal("original Boolean state wrapper missing")
							}
							state["Value"] = true
							value = state
						}
						if fault == "wrapper_drop" {
							key, value = "value", false
						}
						if fault == "kind" {
							key, value = "kind", p15FunctionText("number")
						}
						fields[key]["WFValue"] = value
					} else if fault == "producer_identity" {
						p15Param(region[0])["UUID"] = "UNRELATED_FALLBACK"
					} else {
						discriminator := ""
						for _, action := range broken {
							p := p15Param(action)
							key, _ := p15LiteralText(p["WFDictionaryKey"])
							if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && key == "has_arg_0" {
								discriminator, _ = p["UUID"].(string)
								if fault == "omitted_discriminator" {
									p["WFDictionaryKey"] = p15FunctionText("has_arg_1")
								}
								break
							}
						}
						if discriminator == "" {
							t.Fatal("missing original discriminator")
						}
						for _, action := range broken {
							p := p15Param(action)
							outs, vars := p15V02NativeReferences(p["WFInput"])
							if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0 && len(outs) == 1 && outs[0] == discriminator && len(vars) == 0 {
								switch fault {
								case "wrong_polarity":
									p["WFCondition"] = 5
								case "wrong_group":
									p["GroupingIdentifier"] = "UNRELATED_BRANCH"
								case "extra_fallback":
									// An extra fallback action must not be accepted even if the
									// first two expected producers are otherwise exact.
									lastID, _ := p15Param(region[1])["UUID"].(string)
									for i, candidate := range broken {
										if p15Param(candidate)["UUID"] == lastID {
											extra := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"}
											broken = append(broken[:i+1], append([]map[string]any{extra}, broken[i+1:]...)...)
											break
										}
									}
								}
								break
							}
						}
					}
					if p15TaggedDefaultGraph(broken, tc.dictionary) == nil {
						t.Fatal("accepted corruption: " + fault)
					}
				})
			}
		})
	}
}
