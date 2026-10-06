package main

import (
	"bytes"
	"testing"

	"howett.net/plist"
)

func TestCompileSourceToPlist(t *testing.T) {
	code := `let name = "Alice"
show(f"Hello {name}")
`
	plistBytes, err := CompileSourceToPlist("sample.cherri", code)
	if err != nil {
		t.Fatalf("unexpected compilation error: %v", err)
	}

	if len(plistBytes) == 0 {
		t.Fatalf("expected non-empty plist bytes")
	}

	// Verify unmarshaling into Shortcut struct
	var sc Shortcut
	dec := plist.NewDecoder(bytes.NewReader(plistBytes))
	if err := dec.Decode(&sc); err != nil {
		t.Fatalf("failed to decode generated plist: %v", err)
	}

	if len(sc.WFWorkflowActions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(sc.WFWorkflowActions))
	}
	if sc.WFWorkflowActions[0].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
		t.Errorf("action 0 should be gettext, got %s", sc.WFWorkflowActions[0].WFWorkflowActionIdentifier)
	}
	if sc.WFWorkflowActions[1].WFWorkflowActionIdentifier != "is.workflow.actions.showresult" {
		t.Errorf("action 1 should be showresult, got %s", sc.WFWorkflowActions[1].WFWorkflowActionIdentifier)
	}
}
