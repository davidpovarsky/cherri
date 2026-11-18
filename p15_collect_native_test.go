package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// The existing two dialects must forward the native Repeat Results producer.
// An accumulator or an unrolled body is a different graph and is rejected.
func TestP15CollectExistingLegacyAndV2Results(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		baseline := -1
		loopBaseline := -1
		for _, size := range []int{0, 1, 100} {
			items := make([]string, size)
			for i := range items {
				items[i] = fmt.Sprint(i)
			}
			source := "let items = [" + strings.Join(items, ",") + "]\nlet results = for item in items { yield item }\ngetVariable(results)"
			if legacy {
				source = "@items = [" + strings.Join(items, ",") + "]\nconst results = for item in @items { getVariable(@item) }\ngetVariable(results)"
			}
			actions := compileDebugFixture(t, source, legacy)
			if baseline < 0 {
				baseline = len(actions)
			}
			if !legacy && len(actions) != baseline {
				t.Fatalf("V2 N=%d: literal or collection unrolled", size)
			}
			loops := p15ActionsNamed(actions, "repeat.each")
			if len(loops) != 2 || loops[0].WFWorkflowActionParameters["GroupingIdentifier"] != loops[1].WFWorkflowActionParameters["GroupingIdentifier"] {
				t.Fatal("native Repeat Each region is not paired")
			}
			if p15Int(loops[0].WFWorkflowActionParameters["WFControlFlowMode"]) != 0 || p15Int(loops[1].WFWorkflowActionParameters["WFControlFlowMode"]) != 2 {
				t.Fatal("native Repeat Each boundaries changed")
			}
			start, end := -1, -1
			for i, action := range actions {
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.each" {
					if start < 0 {
						start = i
					} else {
						end = i
					}
				}
			}
			if loopBaseline < 0 {
				loopBaseline = end - start + 1
			}
			if end-start+1 != loopBaseline {
				t.Fatal("collection body unrolled")
			}
			for _, action := range actions[start : end+1] {
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
					t.Fatal("collection body gained an accumulator")
				}
			}
			last := actions[len(actions)-1]
			if last.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || p15OutputReference(last.WFWorkflowActionParameters["WFVariable"])["OutputUUID"] != loops[1].WFWorkflowActionParameters["UUID"] {
				t.Fatal("result consumer no longer references native Repeat Results")
			}
			// Legacy constructs its input array with appends before the loop.
			// Report that real cost separately; it is not a collection accumulator.
			t.Logf("legacy=%t N=%d nativeActions=%d loopActions=%d loopAccumulatorActions=0 inputAppendActions=%d", legacy, size, len(actions), loopBaseline, len(p15ActionsNamed(actions, "appendvariable")))
		}
	}
}

// The exact callback is useful only if its counts, order and type come from
// native operations. A hard-coded success string must fail this local oracle.
func p15CollectObservableResult(actions []ShortcutAction, kind string) error {
	expected := map[string]string{"emptyCount": "count", "singleCount": "count", "manyCount": "count", "singleItem": "getitemfromlist", "firstItem": "getitemfromlist", "lastItem": "getitemfromlist"}
	if kind == "dictionaries" {
		expected = map[string]string{"resultCount": "count", "firstName": "getvalueforkey", "lastName": "getvalueforkey", "dictionaryType": "getitemtype", "nestedCount": "count", "nestedFirst": "getitemfromlist", "nestedLast": "getitemfromlist"}
	}
	producer := map[string]string{}
	var final any
	for _, action := range actions {
		p := action.WFWorkflowActionParameters
		name, _ := p["CustomOutputName"].(string)
		if suffix, needed := expected[name]; needed {
			if action.WFWorkflowActionIdentifier != "is.workflow.actions."+suffix {
				return fmt.Errorf("%s is not a native %s observation", name, suffix)
			}
			producer[name], _ = p["UUID"].(string)
		}
		if name == "finalSummary" {
			final = p["WFTextActionText"]
		}
	}
	seen := map[string]bool{}
	p15F01DiagWalk(final, func(value map[string]any) {
		if id, ok := value["OutputUUID"].(string); ok {
			seen[id] = true
		}
	})
	for name := range expected {
		if producer[name] == "" || !seen[producer[name]] {
			return fmt.Errorf("native observation %s is missing from the callback", name)
		}
	}
	return nil
}

func TestP15CollectSignedFixturesObserveNativeResults(t *testing.T) {
	for _, kind := range []string{"counts", "dictionaries"} {
		for _, dialect := range []string{"v2", "legacy"} {
			t.Run(kind+"-"+dialect, func(t *testing.T) {
				name := "tools/shortcuts-simulator-qa/fixtures/qa-collect-" + kind + "-" + dialect + ".cherri"
				src, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				actions := compileDebugFixture(t, string(src), dialect == "legacy")
				if err := p15CollectObservableResult(actions, kind); err != nil {
					t.Fatal(err)
				}
				for _, action := range actions {
					if action.WFWorkflowActionParameters["CustomOutputName"] == "finalSummary" {
						action.WFWorkflowActionParameters["WFTextActionText"] = "CHERRI_QA_COLLECT_HARDCODED_SUCCESS"
					}
				}
				if p15CollectObservableResult(actions, kind) == nil {
					t.Fatal("constant success replacement was accepted")
				}
			})
		}
	}
}
