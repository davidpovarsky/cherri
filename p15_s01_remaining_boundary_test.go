package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestP15S01OrdinaryLiteralContainerPlaceholderSafety(t *testing.T) {
	cases := []struct{ name, source string }{
		{"list_immutable", "let value=[\"L￼🙂éR\"]\ngetVariable(value)\n"},
		{"list_mutable", "var value=[\"L￼🙂éR\"]\ngetVariable(value)\n"},
		{"map_immutable", "let value={\"key\":\"L￼🙂éR\"}\ngetVariable(value)\n"},
		{"map_mutable", "var value={\"key\":\"L￼🙂éR\"}\ngetVariable(value)\n"},
		{"map_key", "let value={\"K￼K\":\"value\"}\ngetVariable(value)\n"},
		{"map_nested_list", "let value={\"nested\":[\"L￼🙂éR\"]}\ngetVariable(value)\n"},
		{"list_nested_map", "let value=[{\"key\":\"L￼🙂éR\"}]\ngetVariable(value)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			for index, action := range actions {
				if err := p15S01CompletePlaceholderCoverage(action); err != nil {
					t.Fatalf("ordinary literal field at action %d is unsafe: %v", index, err)
				}
			}
		})
	}
}

func p15S01EnclosingInputError(actions []map[string]any, selected ...string) error {
	identifier := "is.workflow.actions.setitemname"
	if len(selected) == 1 {
		identifier = selected[0]
	}
	seed, target := -1, -1
	for index, action := range actions {
		if p15Param(action)["WFTextActionText"] == "S01_ENCLOSING_SEED" {
			seed = index
		}
		if action["WFWorkflowActionIdentifier"] == identifier {
			target = index
		}
	}
	if seed < 0 || target <= seed {
		return fmt.Errorf("missing explicit public seed or enclosing native action")
	}
	uuid := p15Param(actions[seed])["UUID"]
	check := func(value any) bool {
		ref := p15DiagnosticValue(value)
		return ref["Type"] == "ActionOutput" && ref["OutputUUID"] == uuid
	}
	if value, present := p15Param(actions[target])["WFInput"]; present {
		if !check(value) {
			return fmt.Errorf("enclosing native input is not the original seed")
		}
		return nil
	}
	if target == seed+1 {
		return nil
	}
	previous := actions[target-1]
	if previous["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && check(p15Param(previous)["WFVariable"]) {
		return nil
	}
	return fmt.Errorf("pure literal helper replaced enclosing native implicit input: seed=%d target=%d previous=%v", seed, target, previous["WFWorkflowActionIdentifier"])
}

func TestP15S01EnclosingNativeImplicitInputIsPreserved(t *testing.T) {
	cases := []struct{ name, source string }{
		{"literal_implicit", "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"￼name\"})\n"},
		{"literal_explicit_control", "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFInput\":seed,\"WFName\":f\"￼name\"})\n"},
		{"interpolated_implicit", "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"￼{suffix}\"})\n"},
		{"interpolated_explicit_control", "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFInput\":seed,\"WFName\":f\"￼{suffix}\"})\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15S01EnclosingInputError(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15S01EnclosingNativeInputBoundaries(t *testing.T) {
	prefix := "let suffix=text(\"SUFFIX\")\n"
	target := "native.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"\uFFFC{suffix}\"})\n"
	cases := []struct {
		name, seed string
		exact      bool
	}{
		{"unbound_native_producer", "native.action(\"is.workflow.actions.gettext\", {\"WFTextActionText\":\"S01_ENCLOSING_SEED\"})\n", false},
		{"exact_raw_named_producer", "native.action(\"is.workflow.actions.gettext\", parameters:native.plist({\"UUID\":\"s01-exact-input\",\"WFTextActionText\":\"S01_ENCLOSING_SEED\"}))\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, prefix+tc.seed+target)
			if err := p15S01EnclosingInputError(actions); err != nil {
				t.Fatal(err)
			}
			if tc.exact {
				want := map[string]any{"UUID": "s01-exact-input", "WFTextActionText": "S01_ENCLOSING_SEED"}
				if !reflect.DeepEqual(p15Param(actions[1]), want) {
					t.Fatalf("exact raw producer changed: %#v", p15Param(actions[1]))
				}
			}
		})
	}
	t.Run("later_authored_parameter_call", func(t *testing.T) {
		actions := p15CompiledActions(t, prefix+"let seed=text(\"S01_ENCLOSING_SEED\")\n"+
			"native.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"\uFFFC{suffix}\",\"PublicProbe\":native.action(\"is.workflow.actions.detect.text\", {})})\n")
		if err := p15S01EnclosingInputError(actions, "is.workflow.actions.detect.text"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("initial_shortcut_input", func(t *testing.T) {
		actions := p15CompiledActions(t, "native.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"\uFFFC{ShortcutInput}\"})\n")
		last := len(actions) - 1
		if last < 1 || actions[last-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" ||
			p15DiagnosticValue(p15Param(actions[last-1])["WFVariable"])["Type"] != "ExtensionInput" {
			t.Fatal("literal carrier replaced initial ShortcutInput")
		}
	})
	t.Run("explicit_no_output", func(t *testing.T) {
		actions := p15CompiledActions(t, prefix+"native.action(\"is.workflow.actions.nothing\",{})\n"+target)
		last := len(actions) - 1
		if last < 1 || actions[last-1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" {
			t.Fatal("literal carrier replaced explicit no-output state")
		}
	})
	t.Run("exact_raw_missing_identity_fails_closed", func(t *testing.T) {
		source := prefix + "native.action(\"is.workflow.actions.gettext\", parameters:native.plist({\"WFTextActionText\":\"S01_ENCLOSING_SEED\"}))\n" + target
		data, err := CompileSourceToPlist("s01-raw-implicit.cherri", source)
		if err == nil || !strings.Contains(err.Error(), "NATIVE_OUTPUT_ID_MISSING") || len(data) != 0 {
			t.Fatalf("implicit result must not invent identity for opaque raw producer: bytes=%d error=%v", len(data), err)
		}
	})
	t.Run("explicit_input_does_not_consume_raw_previous", func(t *testing.T) {
		source := prefix + "let seed=text(\"S01_ENCLOSING_SEED\")\n" +
			"native.action(\"is.workflow.actions.gettext\", parameters:native.plist({\"WFTextActionText\":\"UNUSED_RAW\"}))\n" +
			"native.action(\"is.workflow.actions.setitemname\", {\"WFInput\":seed,\"WFName\":f\"\uFFFC{suffix}\"})\n"
		actions := p15CompiledActions(t, source)
		if err := p15S01EnclosingInputError(actions); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"WFTextActionText": "UNUSED_RAW"}
		if !reflect.DeepEqual(p15Param(actions[2]), want) {
			t.Fatalf("unused exact producer changed: %#v", p15Param(actions[2]))
		}
	})
}
