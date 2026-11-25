package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestP15TaggedCanonicalMalformedControlBoundaries(t *testing.T) {
	type step struct {
		id, group string
		mode      int
	}
	for name, steps := range map[string][]step{
		"missing_start":      {{"conditional", "outer", 2}},
		"crossed_groups":     {{"conditional", "outer", 0}, {"conditional", "inner", 0}, {"conditional", "outer", 1}, {"conditional", "inner", 2}, {"conditional", "outer", 2}},
		"duplicate_start":    {{"conditional", "outer", 0}, {"conditional", "outer", 0}, {"conditional", "outer", 2}},
		"end_kind_changed":   {{"conditional", "outer", 0}, {"repeat.count", "outer", 2}},
		"repeat_alternative": {{"repeat.count", "outer", 0}, {"repeat.count", "outer", 1}, {"repeat.count", "outer", 2}},
		"unknown_mode":       {{"conditional", "outer", 0}, {"conditional", "outer", 9}, {"conditional", "outer", 2}},
		"duplicate_else":     {{"conditional", "outer", 0}, {"conditional", "outer", 1}, {"conditional", "outer", 1}, {"conditional", "outer", 2}},
		"missing_group":      {{"conditional", "", 0}, {"conditional", "", 2}},
	} {
		t.Run(name, func(t *testing.T) {
			s := &CanonicalBackendSession{}
			for _, item := range steps {
				if err := s.EmitRawAction("is.workflow.actions."+item.id, map[string]any{"GroupingIdentifier": item.group, "WFControlFlowMode": item.mode}, "", "", ""); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := json.Marshal(s.shortcut)
			_, err := s.CurrentNativeInputUsesPrivateValue(map[string]bool{"unrelated-owned-value": true}, nil, p15TaggedFixtureValueOutput)
			after, _ := json.Marshal(s.shortcut)
			if err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
				t.Fatalf("malformed native region accepted: %v", err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("observer changed malformed originals")
			}
		})
	}
}

func TestP15TaggedBranchLiteralCarrierCursor(t *testing.T) {
	for _, private := range []bool{false, true} {
		name := "ordinary"
		if private {
			name = "private"
		}
		t.Run(name, func(t *testing.T) {
			s := &CanonicalBackendSession{}
			emit := func(id string, p map[string]any, uuid string) {
				t.Helper()
				if err := s.EmitRawAction("is.workflow.actions."+id, p, uuid, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			boundary := func(mode int) {
				emit("conditional", map[string]any{"GroupingIdentifier": "branch", "WFControlFlowMode": mode}, "")
			}
			boundary(0)
			emit("dictionary", map[string]any{"WFItems": map[string]any{"kind": "null"}}, "private")
			if !private {
				emit("gettext", map[string]any{"WFTextActionText": "ordinary"}, "ordinary")
			}
			emit("gettext", map[string]any{"WFTextActionText": "~LEFT"}, "snapshot")
			if err := s.BindExactTextSnapshotInput("snapshot", "~LEFT", "preserved-source", "unused", func(id string) bool {
				return id == "is.workflow.actions.dictionary" || p15TaggedFixtureValueOutput(id)
			}); err != nil {
				t.Fatal(err)
			}
			boundary(1)
			emit("gettext", map[string]any{"WFTextActionText": "other"}, "other")
			boundary(2)
			before, _ := json.Marshal(s.shortcut)
			got, err := s.CurrentNativeInputUsesPrivateValue(map[string]bool{"private": true}, nil, p15TaggedFixtureValueOutput)
			after, _ := json.Marshal(s.shortcut)
			if got != private || err != nil {
				t.Fatalf("literal cursor private=%t err=%v; want %t", got, err, private)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("observer assigned or changed native output identity")
			}
		})
	}
}

func TestP15TaggedObserverSharedAliasGraph(t *testing.T) {
	s := &CanonicalBackendSession{}
	if err := s.EmitRawAction("is.workflow.actions.number", map[string]any{"WFNumberActionNumber": 1}, "ordinary", "", ""); err != nil {
		t.Fatal(err)
	}
	previous := "ordinary"
	for index := 0; index < 16; index++ {
		current := fmt.Sprintf("alias-%d", index)
		items := []any{p15FunctionEdge(previous), p15FunctionEdge(previous)}
		if err := s.EmitRawAction("is.workflow.actions.list", map[string]any{"WFItems": items}, current, "", ""); err != nil {
			t.Fatal(err)
		}
		previous = current
	}
	before, _ := json.Marshal(s.shortcut)
	visits := 0
	private, err := s.CurrentNativeInputUsesPrivateValue(map[string]bool{"unrelated-owned-value": true}, nil, func(id string) bool {
		visits++
		return id == "is.workflow.actions.number"
	})
	after, _ := json.Marshal(s.shortcut)
	if err != nil || private || visits != 1 {
		t.Fatalf("shared ordinary producer observed %d times; private=%t error=%v", visits, private, err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("memoized observation changed source graph")
	}
}

func TestP15TaggedObserverNativeOutputContract(t *testing.T) {
	for _, name := range []string{"nil_contract", "unknown_contract", "known_contract", "comment_after_unknown"} {
		t.Run(name, func(t *testing.T) {
			s := &CanonicalBackendSession{}
			if err := s.EmitRawAction("com.example.fixture", map[string]any{}, "opaque", "", ""); err != nil {
				t.Fatal(err)
			}
			if name == "comment_after_unknown" {
				if err := s.EmitRawAction("is.workflow.actions.comment", map[string]any{"WFCommentActionText": "no result"}, "", "", ""); err != nil {
					t.Fatal(err)
				}
			}
			var contract func(string) bool
			if name != "nil_contract" {
				contract = func(id string) bool { return name == "known_contract" && id == "com.example.fixture" }
			}
			private, err := s.CurrentNativeInputUsesPrivateValue(map[string]bool{"unrelated-owned-value": true}, nil, contract)
			if name == "known_contract" {
				if private || err != nil {
					t.Fatalf("known ordinary output rejected: private=%t error=%v", private, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
				t.Fatalf("unestablished output contract accepted: %v", err)
			}
		})
	}
}

func TestP15TaggedOpenControlInputSources(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		reject       bool
	}{
		{"if_input", "let private=null\nlet gate=true\nif gate {native.action(\"is.workflow.actions.detect.text\",{})} else {text(\"other\")}\nreturn 1", false},
		{"else_input", "let private=null\nlet gate=true\nif gate {text(\"other\")} else {native.action(\"is.workflow.actions.detect.text\",{})}\nreturn 1", false},
		{"repeat_ordinary_input", "let private=null\ntext(\"ordinary\")\nrepeat 1 {native.action(\"is.workflow.actions.detect.text\",{})}\nreturn 1", false},
		{"repeat_private_input", "let private=null\nrepeat 1 {native.action(\"is.workflow.actions.detect.text\",{})}\nreturn 1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("tagged-open-"+tc.name+".cherri", tc.source)
			if tc.reject {
				if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
					t.Fatalf("private open input escaped: bytes=%d err=%v", len(raw), err)
				}
			} else if err != nil || len(raw) == 0 {
				t.Fatalf("ordinary checked input rejected: %v", err)
			}
		})
	}
}
