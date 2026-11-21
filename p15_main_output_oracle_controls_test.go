package main

import "testing"

func TestP15M02MainVoidBoundaryRejectsMalformedFinalActions(t *testing.T) {
	actual := p15CompiledActions(t, "text(\"M02_COMPLETE_GRAPH_CONTROL\")")
	if err := p15M02MainVoidBoundaryError(actual); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing", "wrong_identifier", "implicit_input", "malformed_parameters", "null_parameters", "extra_envelope"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15C01Clone(t, actual)
			last := changed[len(changed)-1]
			switch fault {
			case "missing":
				changed = changed[:len(changed)-1]
			case "wrong_identifier":
				last["WFWorkflowActionIdentifier"] = "is.workflow.actions.getvariable"
			case "implicit_input":
				last["WFWorkflowActionParameters"] = map[string]any{"WFInput": "unowned"}
			case "malformed_parameters":
				last["WFWorkflowActionParameters"] = []any{}
			case "null_parameters":
				last["WFWorkflowActionParameters"] = nil
			case "extra_envelope":
				last["P15UnexpectedResult"] = true
			}
			if p15M02MainVoidBoundaryError(changed) == nil {
				t.Fatal("malformed terminal action passed the complete-main boundary", fault)
			}
		})
	}
}
