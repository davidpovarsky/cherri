package main

import (
	"encoding/json"
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
	uuid, validUUID := p15Param(actions[seed])["UUID"].(string)
	check := func(value any) bool {
		ref := p15DiagnosticValue(value)
		return validUUID && uuid != "" && ref["Type"] == "ActionOutput" && ref["OutputUUID"] == uuid
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

func TestP15S01EnclosingNativeCarrierSequenceControls(t *testing.T) {
	prefix := "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\n"
	for _, tc := range []struct{ name, call string }{
		{"two_pure_carriers", "native.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"\uFFFC{suffix}\",\"PublicSecond\":f\"second\uFFFC{suffix}\"})\n"},
		{"later_authored_canonical_producer", "native.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"\uFFFC{suffix}\",\"PublicSecond\":text(\"S01_ENCLOSING_SEED\")})\n"},
		{"no_literal_carrier", "native.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"normal{suffix}\"})\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, prefix+tc.call)
			if err := p15S01EnclosingInputError(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct{ name, field, want string }{
		{"plain", "f\"\uFFFCname\"", "\uFFFCname"},
		{"constant_number", "f\"\uFFFC{1}\"", "\uFFFC1"},
		{"constant_bool", "f\"\uFFFC{true}\"", "\uFFFCtrue"},
	} {
		t.Run("pure_"+tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, prefix+"native.action(\"is.workflow.actions.setitemname\", {\"WFName\":"+tc.field+"})\n")
			if len(actions) != 3 || p15Param(actions[2])["WFName"] != tc.want {
				t.Fatalf("pure string emitted helpers or changed bytes: %#v", actions)
			}
			if err := p15S01EnclosingInputError(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("open_control_explicit_%t", explicit), func(t *testing.T) {
			input := ""
			if explicit {
				input = "\"WFInput\":seed,"
			}
			source := prefix + "if true {\nnative.action(\"is.workflow.actions.setitemname\", {" + input + "\"WFName\":f\"\uFFFC{suffix}\"})\n}\n"
			data, err := CompileSourceToPlist("s01-open-control.cherri", source)
			if explicit {
				if err != nil || len(data) == 0 {
					t.Fatalf("explicit input must retain support: bytes=%d error=%v", len(data), err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "S01_IMPLICIT_INPUT_UNAVAILABLE") || len(data) != 0 {
				t.Fatalf("open control input must not be guessed: bytes=%d error=%v", len(data), err)
			}
		})
	}
}

func TestP15S01EnclosingInputOracleRejectsCausalCorruption(t *testing.T) {
	original := p15CompiledActions(t, "let suffix=text(\"SUFFIX\")\nlet seed=text(\"S01_ENCLOSING_SEED\")\nnative.action(\"is.workflow.actions.setitemname\", {\"WFName\":f\"\uFFFC{suffix}\"})\n")
	if err := p15S01EnclosingInputError(original); err != nil {
		t.Fatalf("healthy production control: %v", err)
	}
	if len(original) != 5 {
		t.Fatalf("reviewed control has changed action topology: %d", len(original))
	}
	clone := func() []map[string]any {
		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var result []map[string]any
		if err = json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	mutations := []struct {
		name   string
		change func([]map[string]any) []map[string]any
	}{
		{"removed_restore", func(a []map[string]any) []map[string]any { return append(a[:3], a[4:]...) }},
		{"stale_reference", func(a []map[string]any) []map[string]any {
			p15DiagnosticValue(p15Param(a[3])["WFVariable"])["OutputUUID"] = p15Param(a[0])["UUID"]
			return a
		}},
		{"literal_replaces_reference", func(a []map[string]any) []map[string]any {
			p15Param(a[3])["WFVariable"] = "S01_ENCLOSING_SEED"
			return a
		}},
		{"wrong_reference_kind", func(a []map[string]any) []map[string]any {
			p15DiagnosticValue(p15Param(a[3])["WFVariable"])["Type"] = "Variable"
			return a
		}},
		{"missing_source_identity", func(a []map[string]any) []map[string]any { delete(p15Param(a[1]), "UUID"); return a }},
		{"changed_source_identity", func(a []map[string]any) []map[string]any {
			p15Param(a[1])["UUID"] = "different-public-source"
			return a
		}},
		{"explicit_carrier_input", func(a []map[string]any) []map[string]any {
			p15Param(a[4])["WFInput"] = map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": p15Param(a[2])["UUID"]}}
			return a
		}},
		{"cleared_input", func(a []map[string]any) []map[string]any {
			a[3] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"}
			return a
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if err := p15S01EnclosingInputError(mutation.change(clone())); err == nil {
				t.Fatal("causal input corruption accepted")
			}
		})
	}
}
