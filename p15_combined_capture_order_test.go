package main

import (
	"fmt"
	"strings"
	"testing"
)

// These distinct repair/recovery producers execute the complete local clauses
// anew. The existing requirement records are not inputs, and no Apple effect
// result can be obtained from these serialized native graph checks.
func TestP15RP12NamedEffectsUnit(t *testing.T) {
	t.Run("written_order_flow", TestP15C01SourceOrderFlow)
	t.Run("invalid_bindings", TestP15C02ArgumentDiagnosticsProduction)
}
func TestP15RP12NamedEffectsNative(t *testing.T) {
	t.Run("written_order_and_owners", TestP15C01SourceOrderNativeStructure)
	t.Run("default_effects_once", TestP15C01DefaultEffectsOnce)
	t.Run("default_guards", TestP15C01DefaultGuardStructure)
	t.Run("argument_snapshots", TestP15C01ArgumentValueSnapshot)
	t.Run("snapshot_boundaries", TestP15C01SnapshotBoundaries)
	t.Run("nested_custom_result", TestP15C01NestedCustomResult)
	t.Run("aggregate_snapshots", TestP15C01AggregateArgumentSnapshots)
	t.Run("accepted_bindings", TestP15C02AcceptedBindingsProduction)
}
func TestBackendRecovery10CaptureOrderUnit(t *testing.T) {
	t.Run("named_effects", TestP15RP12NamedEffectsUnit)
}
func TestBackendRecovery10CaptureOrderNative(t *testing.T) {
	t.Run("joined_capture_order", p15CombinedCaptureOrderGraph)
	t.Run("variable_snapshot", TestP15V01DeclarationSnapshotProduction)
	t.Run("clipboard_snapshot", TestP15V03ClipboardCapturedOnceProduction)
	t.Run("terminal_consumers", TestP15BindingCaptureTerminalCausality)
	t.Run("named_effects", TestP15RP12NamedEffectsNative)
}

func p15CombinedCaptureOrderError(actions []map[string]any) error {
	for _, kind := range []string{"Variable", "Clipboard"} {
		if err := p15BindingSnapshotError(actions, kind); err != nil {
			return err
		}
	}
	ids, err := p15C01MarkerOrder(actions, []string{"P15_C01_B", "P15_C01_A"})
	if err != nil {
		return err
	}
	var fields map[string]any
	count := 0
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "com.example.c01.order" {
			fields = p15C01Params(action)
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("joined scenario has %d named calls, want exactly one", count)
	}
	return p15C01References(fields, map[string]string{"A": "P15_C01_A", "B": "P15_C01_B"}, ids)
}
func p15CombinedCaptureOrderGraph(t *testing.T) {
	t.Helper()
	source := p15C01Custom + strings.Replace(p15BindingFixture(t, "CherriBindingCapturePOC"),
		"let result =", "c01Native(b:text(\"P15_C01_B\"),a:text(\"P15_C01_A\"))\nlet result =", 1)
	actions := p15CompiledActions(t, source)
	if err := p15CombinedCaptureOrderError(actions); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"wrong_binding", "swapped_effects", "duplicate_effect", "late_capture", "late_consumer_read", "scope_rebinding", "missing_saved_use", "duplicate_call"} {
		t.Run(fault, func(t *testing.T) {
			broken := p15C01Clone(t, actions)
			markers := []int{}
			capture, call := -1, -1
			for i, action := range broken {
				p := p15C01Params(action)
				if text, ok := p["WFTextActionText"].(string); ok && strings.HasPrefix(text, "P15_C01_") {
					markers = append(markers, i)
				}
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getclipboard" {
					capture = i
				}
				if action["WFWorkflowActionIdentifier"] == "com.example.c01.order" {
					call = i
				}
			}
			if len(markers) != 2 || capture < 0 || call < 0 {
				t.Fatal("joined control lacks observed producers")
			}
			switch fault {
			case "wrong_binding":
				ids, err := p15C01MarkerOrder(broken, []string{"P15_C01_B", "P15_C01_A"})
				if err != nil {
					t.Fatal(err)
				}
				if p15C01ChangeUUID(p15C01Params(broken[call])["A"], ids["P15_C01_A"], ids["P15_C01_B"]) != 1 {
					t.Fatal("fault missed actual named field")
				}
			case "swapped_effects":
				broken[markers[0]], broken[markers[1]] = broken[markers[1]], broken[markers[0]]
			case "duplicate_effect":
				broken = append(broken[:len(broken)-2], append([]map[string]any{broken[markers[0]]}, broken[len(broken)-2:]...)...)
			case "late_capture":
				saved := broken[capture]
				broken = append(broken[:capture], broken[capture+1:]...)
				broken = append(broken[:len(broken)-2], append([]map[string]any{saved}, broken[len(broken)-2:]...)...)
			case "duplicate_call":
				broken = append(broken[:call], append([]map[string]any{broken[call]}, broken[call:]...)...)
			case "scope_rebinding":
				changed := false
				for _, action := range broken {
					refs := p15BindingDescriptors(p15C01Params(action)["WFVariable"], "Variable")
					for _, ref := range refs {
						if ref["VariableName"] == "numCaptureSrc" {
							ref["VariableName"] = "otherScope"
							changed = true
						}
					}
				}
				if !changed {
					t.Fatal("fault missed actual capture source")
				}
			case "late_consumer_read", "missing_saved_use":
				changed := false
				for _, action := range broken {
					text, refs, ok := p15BindingTokenText(p15C01Params(action)["WFTextActionText"])
					if !ok || !strings.HasPrefix(text, "CHERRI_BINDING_CAPTURE_OK\n") {
						continue
					}
					key := p15BindingResultRanges()[0]
					if fault == "missing_saved_use" {
						delete(refs, key)
					} else {
						refs[key] = map[string]any{"Type": "Variable", "VariableName": "numCaptureSrc"}
					}
					changed = true
				}
				if !changed {
					t.Fatal("fault missed actual saved consumer")
				}
			}
			if p15CombinedCaptureOrderError(broken) == nil {
				t.Fatalf("joined native scenario accepted %s", fault)
			}
		})
	}
}
