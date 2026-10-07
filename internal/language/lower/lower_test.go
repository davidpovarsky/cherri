package lower

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestLowerBasicWorkflow(t *testing.T) {
	code := `let name = "World"
show(f"Hello {name}")
`
	file := source.NewFile("test.cherri", "file:///test.cherri", 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", parser.Errors())
	}

	lowerer := NewLowerer(schema.DefaultRegistry())
	wf, err := lowerer.LowerProgram(prog)
	if err != nil {
		t.Fatalf("unexpected lowering error: %v", err)
	}

	if len(wf.Actions) != 2 {
		t.Fatalf("expected 2 actions (gettext, show), got %d", len(wf.Actions))
	}

	// Action 0: gettext for "World"
	if wf.Actions[0].AppleIdentifier != "is.workflow.actions.gettext" {
		t.Errorf("action 0 should be gettext, got %s", wf.Actions[0].AppleIdentifier)
	}

	// Action 1: show
	if wf.Actions[1].AppleIdentifier != "is.workflow.actions.showresult" {
		t.Errorf("action 1 should be showresult, got %s", wf.Actions[1].AppleIdentifier)
	}

	// Verify f-string serialization structure
	paramVal := wf.Actions[1].Parameters["Text"]
	m, ok := paramVal.(map[string]interface{})
	if !ok {
		t.Fatalf("expected WFTextTokenString dictionary in Text param, got %T: %v", paramVal, paramVal)
	}
	if m["WFSerializationType"] != "WFTextTokenString" {
		t.Errorf("expected WFSerializationType WFTextTokenString, got %v", m["WFSerializationType"])
	}
}

func TestLowerConditionalWorkflow(t *testing.T) {
	code := `if true {
    show("Yes")
} else {
    show("No")
}
`
	file := source.NewFile("cond.cherri", "file:///cond.cherri", 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()

	lowerer := NewLowerer(schema.DefaultRegistry())
	wf, err := lowerer.LowerProgram(prog)
	if err != nil {
		t.Fatalf("unexpected lowering error: %v", err)
	}

	// 6 actions: materialized condition number, conditional begin, show, conditional else, show, conditional end
	if len(wf.Actions) != 6 {
		t.Fatalf("expected 6 actions, got %d", len(wf.Actions))
	}
	if wf.Actions[1].ControlFlowMode != 0 {
		t.Errorf("expected mode 0 on first conditional, got %d", wf.Actions[1].ControlFlowMode)
	}
	if wf.Actions[3].ControlFlowMode != 1 {
		t.Errorf("expected mode 1 on else conditional, got %d", wf.Actions[3].ControlFlowMode)
	}
	if wf.Actions[5].ControlFlowMode != 2 {
		t.Errorf("expected mode 2 on end conditional, got %d", wf.Actions[5].ControlFlowMode)
	}
}
