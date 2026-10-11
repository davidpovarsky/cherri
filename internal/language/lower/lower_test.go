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

func TestLowerFunctionEnvelopeDefaultsAndVoidReturn(t *testing.T) {
	code := `function greet(name: Text = "World") -> Text {
    return f"Hello {name}"
}
function logOnly(tag: Text) -> Void {
    if false {
        return
    }
    show(tag)
}
let g1 = greet()
let g2 = greet("Ada")
logOnly(g1)
show(ShortcutInput)
`
	file := source.NewFile("fn.cherri", "file:///fn.cherri", 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("parse errors: %v", parser.Errors())
	}

	lowerer := NewLowerer(schema.DefaultRegistry())
	wf, err := lowerer.LowerProgram(prog)
	if err != nil {
		t.Fatalf("lower error: %v", err)
	}

	hasCallerInputSave := false
	hasNothing := false
	hasRunWorkflow := false
	for _, act := range wf.Actions {
		if act.AppleIdentifier == "is.workflow.actions.setvariable" {
			if act.Parameters["WFVariableName"] == "_cherri_caller_input" {
				hasCallerInputSave = true
			}
		}
		if act.AppleIdentifier == "is.workflow.actions.nothing" {
			hasNothing = true
		}
		if act.AppleIdentifier == "is.workflow.actions.runworkflow" {
			hasRunWorkflow = true
		}
	}
	if !hasCallerInputSave {
		t.Errorf("expected _cherri_caller_input variable save in function dispatch prologue")
	}
	if !hasNothing {
		t.Errorf("expected is.workflow.actions.nothing for Void function return/fallthrough")
	}
	if !hasRunWorkflow {
		t.Errorf("expected is.workflow.actions.runworkflow for function call")
	}
}

func TestLowerShortCircuitAndExpressionsAndSafeIndexing(t *testing.T) {
	code := `let items = ["alpha", "beta"]
let safe0 = items[0]
let safeNeg = items[-1]
let safeOOB = items[99]
let scAnd = false && (items[0] == "alpha")
let scOr = true || (items[1] == "alpha")
let ifVal = if true {
    yield "T"
} else {
    yield "F"
}
let forVal = for item in items {
    yield item
}
let repVal = repeat 2 as idx {
    yield idx
}
let menuVal = menu("Pick") {
    case "A" {
        yield "Picked A"
    }
    case "B" {
        yield "Picked B"
    }
}
show(f"{safe0},{safeNeg},{safeOOB},{scAnd},{scOr},{ifVal},{forVal},{repVal},{menuVal}")
`
	file := source.NewFile("exprs.cherri", "file:///exprs.cherri", 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("parse errors: %v", parser.Errors())
	}

	lowerer := NewLowerer(schema.DefaultRegistry())
	wf, err := lowerer.LowerProgram(prog)
	if err != nil {
		t.Fatalf("lower error: %v", err)
	}

	hasCount := false
	hasGetItem := false
	hasMenu := false
	for _, act := range wf.Actions {
		switch act.AppleIdentifier {
		case "is.workflow.actions.count":
			hasCount = true
		case "is.workflow.actions.getitemfromlist":
			hasGetItem = true
		case "is.workflow.actions.choosefrommenu":
			hasMenu = true
		}
	}
	if !hasCount || !hasGetItem {
		t.Errorf("expected dynamic count bounds check and getitemfromlist for dynamic/0-based indexing (hasCount=%v, hasGetItem=%v)", hasCount, hasGetItem)
	}
	if !hasMenu {
		t.Errorf("expected choosefrommenu for value-producing menu expression")
	}
}

