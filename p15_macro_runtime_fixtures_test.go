package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func p15ReadMacroRuntimeFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("tools", "shortcuts-simulator-qa", "fixtures", name+".cherri"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestP15_MacroRuntimeFixturesUseCanonicalGuardsAndTimers(t *testing.T) {
	require := p15MacroExplicitOutputActions(t, p15ReadMacroRuntimeFixture(t, "qa-require-v2"), BuildProfileDebug)
	// The native metadata distinguishes Get Type from Get File of Type.
	// Using the latter can silently alter the value and invalidate a type guard.
	if len(p15ActionsNamed(require, "gettypeaction")) != 0 || len(p15ActionsNamed(require, "getitemtype")) == 0 {
		t.Fatal("require runtime fixture did not use canonical Get Type")
	}
	measure := p15MacroExplicitOutputActions(t, p15ReadMacroRuntimeFixture(t, "qa-measure-v2"), BuildProfileDebug)
	if len(p15ActionsNamed(measure, "gettimebetweendates")) != 3 {
		t.Fatal("Debug fixture must retain two nested measurements plus its independent outer timing assertion")
	}
	if len(p15ActionsNamed(measure, "delay")) != 1 || len(p15ActionsNamed(measure, "dictionary")) != 1 {
		t.Fatal("Debug runtime body was lost or duplicated")
	}
	t.Logf("require Debug actions=%d; measure Debug actions=%d", len(require), len(measure))
}

func TestP15_MeasureReleaseRuntimeFixtureRetainsEffectsAndValueProducers(t *testing.T) {
	source := p15ReadMacroRuntimeFixture(t, "qa-measure-release-v2")
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		actions := p15MacroExplicitOutputActions(t, source, profile)
		if len(p15ActionsNamed(actions, "delay")) != 1 || len(p15ActionsNamed(actions, "dictionary")) != 1 {
			t.Fatalf("%s lost or duplicated the single native body effect/value", profile)
		}
		for _, name := range []string{"outerRuns", "innerRuns"} {
			writes := 0
			for _, action := range p15ActionsNamed(actions, "setvariable") {
				if action.WFWorkflowActionParameters["WFVariableName"] == name {
					writes++
				}
			}
			if writes != 2 {
				t.Fatalf("%s counter %s expected one initial and one body write, got %d", profile, name, writes)
			}
		}
		if profile == BuildProfileRelease {
			encoded, err := json.Marshal(actions)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"__cherri_debug_", "__cherri_qa_trace", "is.workflow.actions.gettimebetweendates"} {
				if strings.Contains(string(encoded), forbidden) {
					t.Fatalf("Release retained diagnostic graph data: %s", forbidden)
				}
			}
		} else if len(p15ActionsNamed(actions, "gettimebetweendates")) != 2 {
			t.Fatal("Debug comparison must demonstrate that the same source ordinarily produces timer actions")
		}
		t.Logf("measure Release fixture profile=%s actions=%d", profile, len(actions))
	}
}
