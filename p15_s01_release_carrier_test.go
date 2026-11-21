package main

import (
	"fmt"
	"testing"
)

func p15S01ProfileActions(t *testing.T, source string, profile BuildProfile) []map[string]any {
	t.Helper()
	data, err := CompileSourceToPlistWithOptions("p15-short-circuit.cherri", source, CompileOptions{BuildProfile: profile})
	if err != nil {
		t.Fatal(err)
	}
	root, err := decodeShortcutRawMap(data)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := root["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("actual compiled action array absent")
	}
	actions := make([]map[string]any, len(raw))
	for index, value := range raw {
		actions[index] = value.(map[string]any)
	}
	return actions
}

// Release removes diagnostic instrumentation after evaluating authored calls.
// A literal carrier needed solely by a removed diagnostic is instrumentation,
// and must not replace the previous value of a following implicit-input call.
func TestP15S01ReleaseDoesNotRetainUnconsumedLiteralCarriers(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		effect           bool
	}{
		{"ordinary_control", `debugTrace("ordinary")`, false},
		{"literal_control", `debugTrace("L` + "\uFFFC" + `R")`, false},
		{"computed_template", `debugTrace(f"L` + "\uFFFC" + `{seed}")`, false},
		{"two_literal_chunks", `debugTrace(f"L` + "\uFFFC" + `{seed}R` + "\uFFFC" + `{seed}")`, false},
		{"authored_effect", `debugTrace(f"L` + "\uFFFC" + `{text("AUTHORED")}")`, true},
		{"literal_label_control", `debugLog(label:"L` + "\uFFFC" + `R")`, false},
		{"trace_event_control", `debugTraceEvent(stage:"before",label:"L` + "\uFFFC" + `R")`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "let seed=text(\"SEED\")\n" + tc.expression + "\nnative.action(\"is.workflow.actions.setitemname\",{\"WFName\":\"TARGET\"})\n"
			actions := p15S01ProfileActions(t, source, BuildProfileRelease)
			if err := p15S01ReleaseGraphError(actions, tc.effect); err != nil {
				t.Fatalf("%v; actual=%#v", err, actions)
			}
		})
	}
}

func TestP15S01DebugAndSemanticReleaseKeepConsumedCarriers(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		profile          BuildProfile
	}{
		{"debug_diagnostic", `let seed=text("SEED")` + "\n" + `debugTrace(f"` + p15S01ContainerLiteral + `{seed}")`, BuildProfileDebug},
		{"release_require", `require({"key":"` + p15S01ContainerLiteral + `"},nonEmpty:true)`, BuildProfileRelease},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15S01ProfileActions(t, tc.expression, tc.profile)
			if err := p15M02MainVoidBoundaryError(actions); err != nil {
				t.Fatal(err)
			}
			carrier, err := p15S01LiteralCarrierReference(actions, p15S01ContainerLiteral)
			if err != nil {
				t.Fatal(err)
			}
			if carrier["OutputUUID"] == "" {
				t.Fatal("missing actual retained carrier")
			}
			for _, action := range actions {
				if err := p15S01CompletePlaceholderCoverage(action); err != nil {
					t.Fatal(fmt.Errorf("consumed text: %w", err))
				}
			}
		})
	}
}

func p15S01ReleaseGraphError(actions []map[string]any, effect bool) error {
	if err := p15M02MainVoidBoundaryError(actions); err != nil {
		return err
	}
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	g.value("gettext", map[string]any{"WFTextActionText": "SEED"})
	if effect {
		g.value("gettext", map[string]any{"WFTextActionText": "AUTHORED"})
	}
	g.emit("setitemname", map[string]any{"WFName": "TARGET"})
	g.emit("nothing", nil)
	return p15S01CompleteGraph(g)
}

func TestP15S01ReleaseGraphRejectsCausalMutations(t *testing.T) {
	for _, effect := range []bool{false, true} {
		t.Run(fmt.Sprintf("authored_effect_%t", effect), func(t *testing.T) {
			expression := "debugTrace(f\"L\\uFFFC{seed}\")"
			if effect {
				expression = "debugTrace(f\"L\\uFFFC{text(\"AUTHORED\")}\")"
			}
			source := "let seed=text(\"SEED\")\n" + expression + "\nnative.action(\"is.workflow.actions.setitemname\",{\"WFName\":\"TARGET\"})\n"
			original := p15S01ProfileActions(t, source, BuildProfileRelease)
			if err := p15S01ReleaseGraphError(original, effect); err != nil {
				t.Fatal("healthy complete Release graph", err)
			}
			for _, fault := range []string{"missing_clear", "clear_input", "clear_identity", "changed_effect", "missing_effect", "carrier_leaked"} {
				t.Run(fault, func(t *testing.T) {
					broken := p15C01Clone(t, original)
					last := len(broken) - 1
					switch fault {
					case "missing_clear":
						broken = broken[:last]
					case "clear_input":
						broken[last]["WFWorkflowActionParameters"] = map[string]any{"WFInput": "LEAK"}
					case "clear_identity":
						broken[last]["WFWorkflowActionParameters"] = map[string]any{"UUID": "LEAK"}
					case "changed_effect", "missing_effect":
						at := 0
						if effect {
							at = 1
						}
						if fault == "changed_effect" {
							p15Param(broken[at])["WFTextActionText"] = "CHANGED"
						} else {
							broken = append(broken[:at], broken[at+1:]...)
						}
					case "carrier_leaked":
						carrier := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "L\uFFFC", "UUID": "LEAK", "CustomOutputName": "Text"}}
						broken = append(broken[:last], carrier, broken[last])
					}
					if err := p15S01ReleaseGraphError(broken, effect); err == nil {
						t.Fatal("accepted changed Release graph", fault)
					}
				})
			}
		})
	}
}
