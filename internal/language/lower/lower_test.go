package lower

import (
	"math"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
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

type testMockSession struct {
	meta map[string]any
}

func (m *testMockSession) EmitResolvedCall(call backend.ResolvedCall) (string, error) {
	return "", nil
}
func (m *testMockSession) BeginControl(region backend.ControlRegion) error { return nil }
func (m *testMockSession) EndControl(region backend.ControlRegion) error   { return nil }
func (m *testMockSession) EmitRawAction(appleIdentifier string, params map[string]any, outputUUID, outputName, groupingID string) error {
	return nil
}
func (m *testMockSession) RenameOutput(outputUUID string, newName string) {}
func (m *testMockSession) SetMetadata(name string, value any) {
	if name == "trigger" {
		if list, ok := m.meta["trigger"].([]map[string]any); ok {
			m.meta["trigger"] = append(list, value.(map[string]any))
		} else {
			m.meta["trigger"] = []map[string]any{value.(map[string]any)}
		}
	} else {
		m.meta[name] = value
	}
}
func (m *testMockSession) AddImportQuestion(question map[string]any)              {}
func (m *testMockSession) BindImportQuestion(questionName string, wireKey string) {}
func (m *testMockSession) Finalize() ([]byte, error)                              { return nil, nil }

func TestLowerTriggersWithParameters(t *testing.T) {
	code := `shortcut "Automations" {
    trigger battery(level: 50)
    trigger screenshot(locations: ["photos", "files"])
    trigger wifi(event: .joined)
    trigger bluetooth(event: .connect)
    trigger stageManager(event: .on)
    trigger display(event: .connect)
    trigger charging(event: .connect)
}
show("done")
`
	file := source.NewFile("auto.cherri", "file:///auto.cherri", 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("parse errors: %v", parser.Errors())
	}

	session := &testMockSession{meta: make(map[string]any)}
	lowerer := NewLowererWithSession(schema.DefaultRegistry(), session)
	_, err := lowerer.LowerProgram(prog)
	if err != nil {
		t.Fatalf("lower error: %v", err)
	}

	meta := session.meta
	trigs, ok := meta["trigger"].([]map[string]any)
	if !ok || len(trigs) != 7 {
		t.Fatalf("expected 7 triggers, got %v", meta["trigger"])
	}

	for _, trig := range trigs {
		id := trig["Identifier"].(string)
		params := trig["Parameters"].(map[string]any)
		switch id {
		case "WFBatteryLevelTrigger":
			lvl := params["WFBatteryLevel"].(float64)
			if math.Abs(lvl-49.99) > 0.001 {
				t.Errorf("expected battery level 49.99, got %v", lvl)
			}
		case "WFScreenshotTrigger":
			locs := params["ScreenshotLocations"].([]string)
			if len(locs) != 2 || locs[0] != "photos" || locs[1] != "files" {
				t.Errorf("expected screenshot locations [photos, files], got %v", locs)
			}
		case "WFWifiTrigger":
			if params["WFConnectionType"] != "joined" {
				t.Errorf("expected wifi connectionType joined, got %v", params["WFConnectionType"])
			}
		case "WFBluetoothTrigger":
			if params["WFBluetoothConnectionType"] != "connect" {
				t.Errorf("expected bluetooth connectionType connect, got %v", params["WFBluetoothConnectionType"])
			}
		case "WFStageManagerTrigger":
			if params["WFStageManagerType"] != "on" {
				t.Errorf("expected stageManager type on, got %v", params["WFStageManagerType"])
			}
		case "WFExternalDisplayTrigger":
			if params["WFConnectionType"] != "connect" {
				t.Errorf("expected display connectionType connect, got %v", params["WFConnectionType"])
			}
		case "WFPlugInTrigger":
			if params["WFConnectionType"] != "connect" {
				t.Errorf("expected charging connectionType connect, got %v", params["WFConnectionType"])
			}
		}
	}
}

func TestLowerUnusedFunctionDispatcherOmitted(t *testing.T) {
	code := `function unusedFunc() -> Text {
    return "never called"
}
show("direct action without dispatcher")
`
	file := source.NewFile("unused.cherri", "file:///unused.cherri", 1, code)
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

	for _, act := range wf.Actions {
		if act.AppleIdentifier == "is.workflow.actions.setvariable" {
			if act.Parameters["WFVariableName"] == "_cherri_is_fn" {
				t.Fatalf("unused function dispatcher was emitted when no functions were called")
			}
		}
	}
}

func TestLowerListAppendVariable(t *testing.T) {
	code := `var mutable = ["x", "y"]
mutable += "z"
`
	file := source.NewFile("append.cherri", "file:///append.cherri", 1, code)
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

	foundAppend := false
	for _, act := range wf.Actions {
		if act.AppleIdentifier == "is.workflow.actions.appendvariable" {
			foundAppend = true
			if act.Parameters["WFVariableName"] != "mutable" {
				t.Errorf("expected variable name 'mutable', got %v", act.Parameters["WFVariableName"])
			}
		}
	}
	if !foundAppend {
		t.Fatalf("expected is.workflow.actions.appendvariable in emitted actions")
	}
}


