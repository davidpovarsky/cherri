package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// These are actual CanonicalBackendSession action sequences. Enrollment is
// supplied only for the source-owned private producer; backend-created slots
// are deliberately absent from the lowerer's source-variable enrollment map.
func p15TaggedSnapshotSession(t *testing.T, name string) (*CanonicalBackendSession, map[string]bool) {
	t.Helper()
	s := &CanonicalBackendSession{}
	emit := func(id string, parameters map[string]any, uuid string) {
		t.Helper()
		if err := s.EmitRawAction("is.workflow.actions."+id, parameters, uuid, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	private := func(kind string) {
		parameter := "WFItems"
		payload := any([]any{})
		if kind == "dictionary" {
			payload = map[string]any{"private": "source-owned"}
		}
		emit(kind, map[string]any{parameter: payload}, "private")
	}
	text := func(id string) { emit("gettext", map[string]any{"WFTextActionText": "ordinary"}, id) }
	set := func(slot string, input any, explicit bool) {
		parameters := map[string]any{"WFVariableName": slot}
		if explicit {
			parameters["WFInput"] = input
		}
		emit("setvariable", parameters, "")
	}
	get := func(slot, id string) {
		emit("getvariable", map[string]any{"WFVariable": p15FunctionVariable(slot)}, id)
	}
	boundary := func(mode int) {
		emit("conditional", map[string]any{"GroupingIdentifier": "branch", "WFControlFlowMode": mode, "WFInput": p15FunctionEdge("gate")}, "")
	}
	outputs := map[string]bool{"private": true}
	switch name {
	case "private_list", "private_map", "private_explicit_input", "typed_variable_read", "backend_variable_read",
		"nested_snapshots", "capture_before_helpers", "ordinary_text", "ordinary_date", "nothing",
		"ordinary_explicit_input", "unconditional_clear", "append_private", "append_to_private":
		kind := "list"
		if name == "private_map" {
			kind = "dictionary"
		}
		private(kind)
		if name == "ordinary_text" || name == "ordinary_explicit_input" || name == "append_private" {
			text("ordinary")
		}
		if name == "ordinary_date" {
			emit("date", map[string]any{"WFDateActionMode": "Current Date"}, "ordinary")
		}
		if name == "nothing" {
			emit("nothing", map[string]any{}, "")
		}
		if name == "ordinary_explicit_input" {
			set("snapshot", p15FunctionEdge("ordinary"), true)
		} else if name == "private_explicit_input" {
			set("snapshot", p15FunctionEdge("private"), true)
		} else {
			set("snapshot", nil, false)
		}
		if name == "unconditional_clear" {
			emit("nothing", map[string]any{}, "")
			set("snapshot", nil, false)
		}
		if name == "append_private" {
			emit("appendvariable", map[string]any{"WFVariableName": "snapshot", "WFInput": p15FunctionEdge("private")}, "")
		}
		if name == "append_to_private" {
			text("append-ordinary")
			emit("appendvariable", map[string]any{"WFVariableName": "snapshot", "WFInput": p15FunctionEdge("append-ordinary")}, "")
		}
		text("cache-helper")
		if name == "typed_variable_read" {
			emit("getvariable", map[string]any{"WFVariable": debugLogValueRef("snapshot")}, "restored")
		} else if name == "backend_variable_read" {
			emit("getvariable", map[string]any{"WFVariable": backend.Reference{Kind: backend.RefMutableBinding, ProducerName: "snapshot"}}, "restored")
		} else {
			get("snapshot", "restored")
		}
		if name == "nested_snapshots" {
			set("inner-snapshot", nil, false)
			text("inner-helper")
			get("inner-snapshot", "inner-restored")
		}
	case "snapshot_before_later_source_write":
		private("list")
		set("mutable-source", p15FunctionEdge("private"), true)
		get("mutable-source", "captured-source")
		set("snapshot", nil, false)
		text("later-ordinary")
		set("mutable-source", p15FunctionEdge("later-ordinary"), true)
		text("cache-helper")
		get("snapshot", "restored")
	case "private_inside_open_branch", "ordinary_inside_open_branch":
		text("gate")
		boundary(0)
		private("dictionary")
		if name == "ordinary_inside_open_branch" {
			text("ordinary")
		}
		set("snapshot", nil, false)
		text("cache-helper")
		get("snapshot", "restored")
	case "unowned_marker":
		emit("dictionary", map[string]any{"WFItems": map[string]any{"_cherri_value_cell_v1": "1", "has_value": "1", "kind": "list"}}, "authored")
		set("snapshot", nil, false)
		text("cache-helper")
		get("snapshot", "restored")
	case "future_assignment":
		private("list")
		get("snapshot", "read-before")
		text("later-ordinary")
		set("snapshot", p15FunctionEdge("later-ordinary"), true)
		emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge("read-before")}, "read-result")
	case "conditional_assignment", "conditional_clear":
		private("list")
		if name == "conditional_clear" {
			set("snapshot", p15FunctionEdge("private"), true)
		}
		text("gate")
		boundary(0)
		if name == "conditional_assignment" {
			set("snapshot", p15FunctionEdge("private"), true)
		} else {
			emit("nothing", map[string]any{}, "")
			set("snapshot", nil, false)
		}
		boundary(1)
		text("other")
		boundary(2)
		text("cache-helper")
		get("snapshot", "restored")
	case "cyclic_future_alias":
		private("list")
		emit("setvariable", map[string]any{"WFVariableName": "first", "WFInput": p15FunctionVariable("second")}, "")
		emit("setvariable", map[string]any{"WFVariableName": "second", "WFInput": p15FunctionVariable("first")}, "")
		get("first", "restored")
	default:
		t.Fatal("unregistered backend snapshot case: " + name)
	}
	return s, outputs
}

func p15TaggedSnapshotObservation(t *testing.T, name string, wantPrivate, wantError bool) {
	t.Helper()
	s, outputs := p15TaggedSnapshotSession(t, name)
	before, err := json.Marshal(s.shortcut)
	if err != nil {
		t.Fatal(err)
	}
	actual, failure := s.CurrentNativeInputUsesPrivateValue(outputs, nil, func(id string) bool {
		return id == "is.workflow.actions.date" || p15TaggedFixtureValueOutput(id)
	})
	after, err := json.Marshal(s.shortcut)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("snapshot observer changed original native actions", err)
	}
	if (failure != nil) != wantError || actual != wantPrivate {
		t.Fatalf("private=%t error=%v, want private=%t error=%t", actual, failure, wantPrivate, wantError)
	}
	if failure != nil && !strings.Contains(failure.Error(), "E_TARGET_VALUE_SHAPE") {
		t.Fatal("failure lost precise boundary code", failure)
	}
}

func TestP15TaggedBackendInputSnapshots(t *testing.T) {
	for _, tc := range []struct {
		name    string
		private bool
	}{
		{"private_list", true}, {"private_map", true}, {"private_explicit_input", true},
		{"typed_variable_read", true}, {"backend_variable_read", true}, {"nested_snapshots", true},
		{"capture_before_helpers", true}, {"snapshot_before_later_source_write", true},
		{"private_inside_open_branch", true}, {"append_private", true}, {"append_to_private", true},
		{"ordinary_text", false}, {"ordinary_date", false}, {"nothing", false},
		{"ordinary_explicit_input", false}, {"unconditional_clear", false},
		{"ordinary_inside_open_branch", false}, {"unowned_marker", false},
	} {
		t.Run(tc.name, func(t *testing.T) { p15TaggedSnapshotObservation(t, tc.name, tc.private, false) })
	}
}

func TestP15TaggedBackendAmbiguousSnapshots(t *testing.T) {
	for _, name := range []string{"future_assignment", "conditional_assignment", "conditional_clear", "cyclic_future_alias"} {
		t.Run(name, func(t *testing.T) { p15TaggedSnapshotObservation(t, name, false, true) })
	}
}
