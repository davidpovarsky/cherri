package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func p15MeasureActions(t *testing.T, source string, profile BuildProfile) []ShortcutAction {
	t.Helper()
	actions, err := compileBuildProfileFixture(t, source, false, profile)
	if err != nil {
		t.Fatal(err)
	}
	return actions
}

func p15ActionsNamed(actions []ShortcutAction, suffix string) []ShortcutAction {
	var found []ShortcutAction
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions."+suffix {
			found = append(found, action)
		}
	}
	return found
}

func p15OutputReference(value any) map[string]any {
	object := debugParamMap(value)
	if nested, found := object["Value"]; found {
		object = debugParamMap(nested)
	}
	return object
}

func TestP15_MeasurePreservesOriginalDictionaryProducer(t *testing.T) {
	source := `let result = measure("dictionary") {
    let payload = {"flag": false, "nested": [1, {"name": "typed"}]}
    payload
}
getVariable(result)`
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		actions := p15MeasureActions(t, source, profile)
		t.Logf("measure dictionary profile=%s nativeActions=%d", profile, len(actions))
		dictionaries := p15ActionsNamed(actions, "dictionary")
		if len(dictionaries) != 1 {
			t.Fatalf("body dictionary rerun/lost: %v", actionSuffixes(actions))
		}
		producer := dictionaries[0].WFWorkflowActionParameters["UUID"]
		last := actions[len(actions)-1]
		if last.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
			t.Fatal("consumer changed")
		}
		if ref := p15OutputReference(last.WFWorkflowActionParameters["WFVariable"]); ref["Type"] != "ActionOutput" || ref["OutputUUID"] != producer {
			t.Fatalf("result retargeted to timing output: %v", ref)
		}
		restore := actions[len(actions)-2]
		if ref := p15OutputReference(restore.WFWorkflowActionParameters["WFVariable"]); ref["OutputUUID"] != producer {
			t.Fatalf("implicit output not restored: %v", restore)
		}
		if len(p15ActionsNamed(actions, "gettext")) != 0 || len(p15ActionsNamed(actions, "number")) != 0 {
			t.Fatal("typed dictionary coerced to text/number")
		}
		if len(p15ActionsNamed(actions, "date")) != 0 {
			t.Fatal("redundant Get Current Date action emitted")
		}
		timers := len(p15ActionsNamed(actions, "gettimebetweendates"))
		if (profile == BuildProfileDebug && timers != 1) || (profile == BuildProfileRelease && timers != 0) {
			t.Fatalf("profile timing actions=%d", timers)
		}
	}
}

func TestP15_MeasureNestedSameLabelRunsBodyOnce(t *testing.T) {
	source := `let result = measure("same") {
    let inner = measure("same") { run("CHERRI_SYNTHETIC_ONCE") }
    inner
}
getVariable(result)`
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		actions := p15MeasureActions(t, source, profile)
		calls := p15ActionsNamed(actions, "runworkflow")
		if len(calls) != 1 {
			t.Fatalf("source effect was removed or rerun: %v", actionSuffixes(actions))
		}
		producer := calls[0].WFWorkflowActionParameters["UUID"]
		last := actions[len(actions)-1]
		if p15OutputReference(last.WFWorkflowActionParameters["WFVariable"])["OutputUUID"] != producer {
			t.Fatal("nested return lost original producer")
		}
		keys := map[string]bool{}
		for _, action := range p15ActionsNamed(actions, "setvariable") {
			name, _ := action.WFWorkflowActionParameters["WFVariableName"].(string)
			if strings.HasPrefix(name, "__cherri_debug_timer_") {
				keys[name] = true
			}
		}
		if profile == BuildProfileDebug && len(keys) != 2 {
			t.Fatalf("same-label timers collide: %v", keys)
		}
		if profile == BuildProfileRelease && (len(keys) != 0 || len(p15ActionsNamed(actions, "appendvariable")) != 0) {
			t.Fatal("release retained timer diagnostics")
		}
	}
}

func TestP15_MeasureShadowedNativeStorageDoesNotOverwriteOuter(t *testing.T) {
	source := `var value = "outer"
let measured = measure("scope") {
    var value = "inner"
    if true { var value = "branch"; value = "changed" }
    value
}
getVariable(value)
getVariable(measured)`
	actions := p15MeasureActions(t, source, BuildProfileRelease)
	outerWrites := 0
	localNames := map[string]bool{}
	for _, action := range p15ActionsNamed(actions, "setvariable") {
		name, _ := action.WFWorkflowActionParameters["WFVariableName"].(string)
		if name == "value" {
			outerWrites++
		}
		if strings.HasPrefix(name, "__cherri_block_") {
			localNames[name] = true
		}
	}
	if outerWrites != 1 || len(localNames) != 2 {
		t.Fatalf("native scope leaked: outer writes=%d local names=%v", outerWrites, localNames)
	}
	outerRead := p15OutputReference(actions[len(actions)-2].WFWorkflowActionParameters["WFVariable"])
	if outerRead["Type"] != "Variable" || outerRead["VariableName"] != "value" {
		t.Fatalf("outer variable binding lost: %v", outerRead)
	}
}

func TestP15_MeasureWaitAndNativeControlResults(t *testing.T) {
	for _, source := range []string{
		`let result = measure("wait") { wait(1); text("done") }; getVariable(result)`,
		`let result = measure("if") { if true { yield "left" } else { yield "right" } }; getVariable(result)`,
		`let result = measure("loop") { for item in [1, 2] { yield item } }; getVariable(result)`,
	} {
		actions := p15MeasureActions(t, source, BuildProfileDebug)
		if len(p15ActionsNamed(actions, "gettimebetweendates")) != 1 {
			t.Fatal("missing elapsed diagnostic")
		}
		if strings.Contains(source, "wait(1)") && len(p15ActionsNamed(actions, "delay")) != 1 {
			t.Fatalf("wait body rerun/lost: %v", actionSuffixes(actions))
		}
		last := p15OutputReference(actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])
		for _, timer := range p15ActionsNamed(actions, "gettimebetweendates") {
			if last["OutputUUID"] == timer.WFWorkflowActionParameters["UUID"] {
				t.Fatal("returned elapsed number instead of body value")
			}
		}
	}
}

func TestP15_MeasureFreezesCurrentDateBeforeStoppingTimer(t *testing.T) {
	actions := p15MeasureActions(t, `let result = measure("date") { CurrentDate }; getVariable(result)`, BuildProfileDebug)
	snapshotIndex, elapsedIndex := -1, -1
	var snapshotUUID any
	for index, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettimebetweendates" {
			elapsedIndex = index
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" && p15OutputReference(action.WFWorkflowActionParameters["WFVariable"])["Type"] == "CurrentDate" {
			snapshotIndex, snapshotUUID = index, action.WFWorkflowActionParameters["UUID"]
		}
	}
	if snapshotIndex < 0 || elapsedIndex <= snapshotIndex {
		t.Fatalf("CurrentDate not frozen at body boundary: %v", actionSuffixes(actions))
	}
	last := p15OutputReference(actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])
	if last["OutputUUID"] != snapshotUUID {
		t.Fatalf("consumer rereads CurrentDate instead of its snapshot: %v", last)
	}
}

func TestP15_MeasureRejectsUnprovenForms(t *testing.T) {
	for _, test := range []struct{ source, diagnostic string }{
		{`let result = measure("empty") {}`, "E_BLOCK_VALUE_REQUIRED"},
		{`let result = measure("void") { wait(1) }`, "E_BLOCK_VALUE_REQUIRED"},
		{`let label = "dynamic"; let result = measure(label) { 1 }`, "E_LITERAL_REQUIRED"},
		{`let result = measure("missing") { none }`, "E_BLOCK_VALUE_REQUIRED"},
		{`let result = measure("raw") { native.action("is.workflow.actions.gettext", parameters: {"WFTextActionText": "raw"}) }`, "E_BLOCK_RAW_NATIVE"},
		{`let result = measure("raw") { rawAction("is.workflow.actions.gettext", {"WFTextActionText": "raw"}) }`, "E_BLOCK_RAW_NATIVE"},
	} {
		if _, err := CompileSourceToPlist("negative-measure.cherri", test.source); err == nil || !strings.Contains(err.Error(), test.diagnostic) {
			t.Fatalf("expected %s for %s, got %v", test.diagnostic, test.source, err)
		}
	}
}

func TestP15_CollectUsesExistingRepeatResultsWithoutAccumulator(t *testing.T) {
	for _, size := range []int{0, 1, 100} {
		items := make([]string, size)
		for index := range items {
			items[index] = fmt.Sprint(index)
		}
		source := "let result = for item in [" + strings.Join(items, ",") + "] { yield item }\ngetVariable(result)"
		actions := p15MeasureActions(t, source, BuildProfileDebug)
		var boundaries []int
		for index, action := range actions {
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.each" {
				boundaries = append(boundaries, index)
			}
		}
		if len(boundaries) != 2 {
			t.Fatalf("missing Repeat Each boundaries: %v", actionSuffixes(actions))
		}
		start, end := boundaries[0], boundaries[1]
		region := actions[start : end+1]
		if got := strings.Join(actionSuffixes(region), ","); got != "repeat.each,setvariable,getvariable,repeat.each" {
			t.Fatalf("existing four-action loop body changed/unrolled at N=%d: %s", size, got)
		}
		inputActions := actions[:start]
		numbers, lists := p15ActionsNamed(inputActions, "number"), p15ActionsNamed(inputActions, "list")
		if size == 0 {
			if len(numbers) != 0 || len(lists) != 0 || strings.Join(actionSuffixes(inputActions), ",") != "nothing,setvariable,getvariable" {
				t.Fatalf("empty input must use the existing cleared-variable bridge: %v", actionSuffixes(inputActions))
			}
			data, err := json.Marshal(actions)
			if err != nil {
				t.Fatal(err)
			}
			var wire []map[string]any
			if err = json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			// Follow the exact Repeat input UUID through Get Variable to a
			// prior Nothing -> Set Variable with no implicit input. This is
			// structural proof; this test does not run Apple Shortcuts.
			if _, err = p15EmptyOrigin(wire, start, p15EmptyParams(wire[start])["WFInput"]); err != nil {
				t.Fatalf("empty collect input lost its cleared producer: %v", err)
			}
			for _, fault := range []string{"prior_input", "missing_nothing", "wrong_variable", "wrong_repeat_uuid", "duplicate_uuid"} {
				if err = json.Unmarshal(data, &wire); err != nil {
					t.Fatal(err)
				}
				switch fault {
				case "prior_input":
					p15EmptyParams(wire[1])["WFInput"] = "not empty"
				case "missing_nothing":
					wire[0]["WFWorkflowActionIdentifier"] = "is.workflow.actions.comment"
				case "wrong_variable":
					p15EmptyParams(wire[1])["WFVariableName"] = "unrelated"
				case "wrong_repeat_uuid":
					debugParamMap(debugParamMap(p15EmptyParams(wire[start])["WFInput"])["Value"])["OutputUUID"] = "unrelated"
				case "duplicate_uuid":
					p15EmptyParams(wire[1])["UUID"] = p15EmptyParams(wire[2])["UUID"]
				}
				if _, err = p15EmptyOrigin(wire, start, p15EmptyParams(wire[start])["WFInput"]); err == nil {
					t.Fatalf("empty collect input oracle accepted %s mutation", fault)
				}
			}
		} else {
			if len(numbers) != size || len(lists) != 1 {
				t.Fatalf("typed input construction lost values: N=%d Number=%d List=%d", size, len(numbers), len(lists))
			}
			for index, number := range numbers {
				if fmt.Sprint(number.WFWorkflowActionParameters["WFNumberActionNumber"]) != fmt.Sprint(index) {
					t.Fatalf("numeric input value changed at N=%d item=%d: %v", size, index, number)
				}
			}
			input := p15OutputReference(actions[start].WFWorkflowActionParameters["WFInput"])
			if input["Type"] != "ActionOutput" || input["OutputUUID"] != lists[0].WFWorkflowActionParameters["UUID"] {
				t.Fatalf("loop is not bound to the typed input List: %v", input)
			}
		}
		if len(actions[end+1:]) != 1 {
			t.Fatalf("unexpected post-loop work: %v", actionSuffixes(actions[end+1:]))
		}
		t.Logf("collect N=%d total=%d inputConstruction=%d repeatRegion=%d resultConsumer=%d", size, len(actions), len(inputActions), len(region), len(actions[end+1:]))
		loops := p15ActionsNamed(actions, "repeat.each")
		if len(loops) != 2 || loops[0].WFWorkflowActionParameters["GroupingIdentifier"] != loops[1].WFWorkflowActionParameters["GroupingIdentifier"] {
			t.Fatal("repeat control region not paired")
		}
		if fmt.Sprint(loops[0].WFWorkflowActionParameters["WFControlFlowMode"]) != "0" || fmt.Sprint(loops[1].WFWorkflowActionParameters["WFControlFlowMode"]) != "2" {
			t.Fatal("repeat mode mismatch")
		}
		if len(p15ActionsNamed(actions, "appendvariable")) != 0 {
			t.Fatal("collection added an accumulator")
		}
		last := p15OutputReference(actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])
		if last["OutputUUID"] != loops[1].WFWorkflowActionParameters["UUID"] {
			t.Fatalf("consumer does not use Repeat Results: %v", last)
		}
	}
	accumulator := p15MeasureActions(t, `var result: List<Number> = []; for item in [1, 2] { result += item }; getVariable(result)`, BuildProfileDebug)
	if len(p15ActionsNamed(accumulator, "appendvariable")) == 0 {
		t.Fatal("comparison fixture no longer models an explicit accumulator")
	}
}

func TestP15_CollectTypedBodyHasSingleNativeProducer(t *testing.T) {
	for _, test := range []struct{ body, producer string }{
		{`{"name": "typed", "flag": false}`, "dictionary"},
		{`["nested", item]`, "list"},
	} {
		source := "let result = for item in [1, 2] { yield " + test.body + " }\ngetVariable(result)"
		actions := p15MeasureActions(t, source, BuildProfileDebug)
		expected := 1
		if test.producer == "list" {
			expected++
		} // one input List and one body List
		if len(p15ActionsNamed(actions, test.producer)) != expected {
			t.Fatalf("body producer duplicated/lost: %v", actionSuffixes(actions))
		}
		if len(p15ActionsNamed(actions, "appendvariable")) != 0 {
			t.Fatal("typed collection added an accumulator")
		}
	}
}

func TestP15_MeasureAndCollectReusableFixturesCompile(t *testing.T) {
	for _, name := range []string{"v2-measure-runtime.cherri", "v2-collect-existing-repeat-results.cherri"} {
		path := filepath.Join("tests", "language-v2", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := CompileSourceToPlist(path, string(content)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
