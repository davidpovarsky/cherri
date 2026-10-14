package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestMapIndexDynamicKeyLowering(t *testing.T) {
	code := `let dict = {"name": "Cherri", "version": "2.0"}
let k = "name"
let v = dict[k]
show(v)
`
	file := source.NewFile("map_idx.cherri", "file:///map_idx.cherri", 1, code)
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

	var getValueAction *struct {
		params map[string]interface{}
	}

	for _, act := range wf.Actions {
		if act.AppleIdentifier == "is.workflow.actions.getitemfromlist" {
			t.Errorf("FAIL: map index with dynamic key emitted 'is.workflow.actions.getitemfromlist'!")
		}
		if act.AppleIdentifier == "is.workflow.actions.math" {
			t.Errorf("FAIL: map index with dynamic key emitted 'is.workflow.actions.math' (+ 1.0)!")
		}
		if act.AppleIdentifier == "is.workflow.actions.getvalueforkey" {
			getValueAction = &struct {
				params map[string]interface{}
			}{params: act.Parameters}
		}
	}

	if getValueAction == nil {
		t.Fatalf("FAIL: expected 'is.workflow.actions.getvalueforkey' action, but none found!")
	}

	// Verify WFDictionaryKey has WFTextTokenString structure
	keyParam, ok := getValueAction.params["WFDictionaryKey"].(map[string]interface{})
	if !ok {
		t.Fatalf("FAIL: expected WFDictionaryKey to be map[string]interface{}, got %T: %v", getValueAction.params["WFDictionaryKey"], getValueAction.params["WFDictionaryKey"])
	}

	if keyParam["WFSerializationType"] != "WFTextTokenString" {
		t.Errorf("FAIL: expected WFSerializationType WFTextTokenString, got %v", keyParam["WFSerializationType"])
	}

	valMap, ok := keyParam["Value"].(map[string]interface{})
	if !ok {
		t.Fatalf("FAIL: expected Value map inside WFTextTokenString, got %T: %v", keyParam["Value"], keyParam["Value"])
	}

	if valMap["string"] != "\uFFFC" {
		t.Errorf("FAIL: expected string '\\uFFFC', got %q", valMap["string"])
	}

	attachments, ok := valMap["attachmentsByRange"].(map[string]interface{})
	if !ok || len(attachments) == 0 {
		t.Errorf("FAIL: expected non-empty attachmentsByRange, got %v", valMap["attachmentsByRange"])
	}
}

func TestListIndexDynamicLowering(t *testing.T) {
	code := `let items = ["apple", "banana", "cherry"]
let idx = 1
let v = items[idx]
show(v)
`
	file := source.NewFile("list_idx.cherri", "file:///list_idx.cherri", 1, code)
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

	foundGetItem := false
	foundMath := false
	for _, act := range wf.Actions {
		if act.AppleIdentifier == "is.workflow.actions.getitemfromlist" {
			foundGetItem = true
		}
		if act.AppleIdentifier == "is.workflow.actions.math" {
			foundMath = true
		}
		if act.AppleIdentifier == "is.workflow.actions.getvalueforkey" {
			t.Errorf("FAIL: list index emitted 'is.workflow.actions.getvalueforkey'!")
		}
	}

	if !foundGetItem {
		t.Errorf("expected is.workflow.actions.getitemfromlist for list indexing")
	}
	if !foundMath {
		t.Errorf("expected is.workflow.actions.math (+ 1.0) for dynamic list indexing")
	}
}

func TestMapIndexEmptyKeyLoweringError(t *testing.T) {
	code := `let dict = {"name": "Cherri"}
let v = dict[""]
`
	file := source.NewFile("empty_key.cherri", "file:///empty_key.cherri", 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", parser.Errors())
	}

	lowerer := NewLowerer(schema.DefaultRegistry())
	_, err := lowerer.LowerProgram(prog)
	if err == nil {
		t.Fatalf("FAIL: expected lowering error for empty dictionary key, got nil")
	}
	if !strings.Contains(err.Error(), "empty dictionary key is not allowed") {
		t.Errorf("FAIL: unexpected error message: %v", err)
	}
}
