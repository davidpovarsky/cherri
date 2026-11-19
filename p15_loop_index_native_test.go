package main

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
)

func p15F02NativeToken(value any, kind, identity string) bool {
	var envelope, ok = value.(map[string]any)
	if !ok || envelope["WFSerializationType"] != "WFTextTokenAttachment" {
		return false
	}
	var token, found = envelope["Value"].(map[string]any)
	if !found || token["Type"] != kind {
		return false
	}
	var key = "OutputUUID"
	if kind == "Variable" {
		key = "VariableName"
	}
	return token[key] == identity
}

func p15F02NativeBindings(actions []map[string]any, expected map[string]string, zeroBased bool) error {
	var found = map[string]bool{}
	var stack []string
	for i, action := range actions {
		var params = p15Param(action)
		var id = action["WFWorkflowActionIdentifier"]
		if id == "is.workflow.actions.repeat.count" || id == "is.workflow.actions.repeat.each" {
			var group, _ = params["GroupingIdentifier"].(string)
			if group == "" {
				return fmt.Errorf("missing native loop owner at %d", i)
			}
			switch p15Int(params["WFControlFlowMode"]) {
			case 0:
				stack = append(stack, group)
			case 2:
				if len(stack) == 0 || stack[len(stack)-1] != group {
					return fmt.Errorf("native loop owner mismatch at %d", i)
				}
				stack = stack[:len(stack)-1]
			default:
				return fmt.Errorf("unexpected loop control mode at %d", i)
			}
			continue
		}
		if id != "is.workflow.actions.setvariable" {
			continue
		}
		var name, _ = params["WFVariableName"].(string)
		var owner, wanted = expected[name]
		if !wanted {
			continue
		}
		if found[name] || len(stack) == 0 {
			return fmt.Errorf("index %q is duplicate or outside its loop", name)
		}
		found[name] = true
		var actualOwner = "Repeat Index"
		if len(stack) > 1 {
			actualOwner = fmt.Sprintf("Repeat Index %d", len(stack))
		}
		if actualOwner != owner {
			return fmt.Errorf("index %q is in %s, want %s", name, actualOwner, owner)
		}
		if !zeroBased {
			if !p15F02NativeToken(params["WFInput"], "Variable", owner) {
				return fmt.Errorf("legacy index %q no longer reads native one-based %s: %#v", name, owner, params["WFInput"])
			}
			continue
		}
		if i == 0 {
			return fmt.Errorf("index %q has no conversion producer", name)
		}
		var previous = actions[i-1]
		var math = p15Param(previous)
		var uuid, _ = math["UUID"].(string)
		if previous["WFWorkflowActionIdentifier"] != "is.workflow.actions.math" || math["WFMathOperation"] != "-" || p15Int(math["WFMathOperand"]) != 1 || uuid == "" || !p15F02NativeToken(math["WFInput"], "Variable", owner) || !p15F02NativeToken(params["WFInput"], "ActionOutput", uuid) {
			return fmt.Errorf("index %q does not capture %s minus one from its native producer: previous=%#v binding=%#v", name, owner, previous, params)
		}
	}
	if len(stack) != 0 || len(found) != len(expected) {
		return fmt.Errorf("incomplete native index graph: found=%v expected=%v unclosed=%v", found, expected, stack)
	}
	return nil
}

func TestP15F02UnchangedRuntimeFixtureNativeIndices(t *testing.T) {
	var source, err = os.ReadFile("tests/runtime_poc/CherriRuntimePOC.cherri")
	if err != nil {
		t.Fatal(err)
	}
	var actions = p15CompiledActions(t, string(source))
	if err := p15F02NativeBindings(actions, map[string]string{"outerIndex": "Repeat Index", "innerIndex": "Repeat Index 2"}, true); err != nil {
		t.Fatal(err)
	}
}

func TestP15F02MigrationPreservesLegacyOneBasedIndices(t *testing.T) {
	var source = `#define name P15_F02_LEGACY_INDEX
repeat outerIndex for 2 {
    for item in list("A", "B") {
        repeat innerIndex for 3 {
            number(@innerIndex)
        }
    }
    number(@outerIndex)
}
repeat siblingIndex for 2 {
    number(@siblingIndex)
}`
	var converted, diagnostics, err = migrate.MigrateSourceWithDiagnostics(source)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("migration: %v %v", err, diagnostics)
	}
	var actions = p15CompiledActions(t, converted)
	if err := p15F02NativeBindings(actions, map[string]string{"outerIndex": "Repeat Index", "innerIndex": "Repeat Index 3", "siblingIndex": "Repeat Index"}, false); err != nil {
		t.Fatalf("%v\nsource:\n%s", err, converted)
	}
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.math" {
			t.Fatal("legacy native indices acquired a V2 index conversion")
		}
	}
}

func TestP15F02RawNativeIndexGlobalsRemainOneBased(t *testing.T) {
	var actions = p15CompiledActions(t, `repeat 2 {
    number(RepeatIndex)
    for item in ["A", "B"] { number(RepeatIndex2); number(RepeatIndex) }
}`)
	var actual []string
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.math" {
			t.Fatal("raw native RepeatIndex globals were converted")
		}
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.number" {
			continue
		}
		var value = p15Param(action)["WFNumberActionNumber"]
		for _, name := range []string{"Repeat Index", "Repeat Index 2"} {
			if p15F02NativeToken(value, "Variable", name) {
				actual = append(actual, name)
			}
		}
	}
	var expected = []string{"Repeat Index", "Repeat Index 2", "Repeat Index"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("native index reference order: got %v, want %v", actual, expected)
	}
}
