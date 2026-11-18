package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"howett.net/plist"
)

const p15VariadicOutputUUID = "5ea7ca0f-ef26-4b89-a904-58fd23a68420"

func p15EmitCanonicalVariadic(positions []int) (root map[string]any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("canonical session panic: %v", recovered)
		}
	}()
	session := NewCanonicalBackendSession()
	defer session.Close()
	arguments := make([]backend.CallArgument, len(positions))
	for index, position := range positions {
		arguments[index] = backend.CallArgument{
			ParameterID: "url", Position: position,
			Value: backend.SemanticValue{Kind: backend.ValString, StrVal: fmt.Sprintf("https://example.test/value-%d", index)},
		}
	}
	id, err := session.EmitResolvedCall(backend.ResolvedCall{
		DefinitionID: "url", AppleIdentifier: "is.workflow.actions.url",
		OutputUUID: p15VariadicOutputUUID, OutputName: "P15 Variadic",
		Arguments: arguments,
	})
	if err != nil {
		return nil, err
	}
	if id != p15VariadicOutputUUID {
		return nil, fmt.Errorf("canonical output identity changed: %q", id)
	}
	data, err := session.Finalize()
	if err != nil {
		return nil, err
	}
	_, err = plist.Unmarshal(data, &root)
	return root, err
}

func p15VariadicWireError(root map[string]any, count int) error {
	actions, ok := root["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 1 {
		return fmt.Errorf("expected exactly one URL action")
	}
	action := actions[0].(map[string]any)
	params := p15Param(action)
	if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.url" ||
		params["UUID"] != p15VariadicOutputUUID || params["CustomOutputName"] != "P15 Variadic" ||
		params["Show-WFURLActionURL"] != true {
		return fmt.Errorf("canonical action or output identity changed")
	}
	want := make([]any, count)
	for index := range want {
		want[index] = fmt.Sprintf("https://example.test/value-%d", index)
	}
	if !reflect.DeepEqual(params["WFURLActionURL"], want) {
		return fmt.Errorf("variadic argument order/cardinality changed: %#v", params["WFURLActionURL"])
	}
	return nil
}

func TestP15CanonicalVariadicPositionBounds(t *testing.T) {
	for _, tc := range []struct {
		name      string
		positions []int
	}{
		{"canonical_parameter_positions", []int{0, 0, 0}},
		{"sequential_argument_positions", []int{0, 1, 2}},
		{"repeated_then_inside_grown_args", []int{0, 0, 1}},
		{"repeated_then_later_inside_grown_args", []int{0, 0, 0, 2}},
		{"named_negative_then_inside_grown_args", []int{-1, 0, 1}},
		{"large_position_fallback", []int{0, 0, 99}},
		{"negative_position_fallback", []int{0, 0, -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := p15EmitCanonicalVariadic(tc.positions)
			if err != nil {
				t.Fatal(err)
			}
			if err := p15VariadicWireError(root, len(tc.positions)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15CanonicalVariadicWireRejectsCorruption(t *testing.T) {
	root, err := p15EmitCanonicalVariadic([]int{0, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"overwrite", "drop", "reorder", "duplicate", "output_identity"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15LiteralClone(t, root).(map[string]any)
			params := p15Param(changed["WFWorkflowActions"].([]any)[0].(map[string]any))
			items := params["WFURLActionURL"].([]any)
			switch fault {
			case "overwrite":
				params["WFURLActionURL"] = []any{items[0], items[2]}
			case "drop":
				params["WFURLActionURL"] = items[:2]
			case "reorder":
				items[0], items[2] = items[2], items[0]
			case "duplicate":
				items[1] = items[0]
			case "output_identity":
				params["UUID"] = "wrong-output"
			}
			if p15VariadicWireError(changed, 3) == nil {
				t.Fatal("accepted " + fault)
			}
		})
	}
}

func TestP15CanonicalFixedParameterBoundsRemainClosed(t *testing.T) {
	for _, position := range []int{-1, 1, 99} {
		t.Run(fmt.Sprint(position), func(t *testing.T) {
			session := NewCanonicalBackendSession()
			defer session.Close()
			_, err := session.EmitResolvedCall(backend.ResolvedCall{
				DefinitionID: "text", AppleIdentifier: "is.workflow.actions.gettext",
				Arguments: []backend.CallArgument{{
					ParameterID: "not-a-canonical-parameter", Position: position,
					Value: backend.SemanticValue{Kind: backend.ValString, StrVal: "P15_FIXED_CONTROL"},
				}},
			})
			if err == nil || !strings.Contains(err.Error(), "unknown parameter") {
				t.Fatalf("invalid fixed parameter was accepted: %v", err)
			}
			if len(session.shortcut.WFWorkflowActions) != 0 {
				t.Fatal("rejected fixed parameter emitted an action")
			}
		})
	}
}
