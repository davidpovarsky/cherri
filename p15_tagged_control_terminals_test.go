package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// These are native graph observations, not claims about executed Shortcuts.
func TestP15TaggedCanonicalControlTerminals(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		wantPrivate, wantError bool
	}{
		{"if_intermediate_private_then_text", false, false},
		{"if_private_terminal", true, false},
		{"if_private_else_terminal", true, false},
		{"nested_ordinary_terminals", false, false},
		{"nested_private_terminal", true, false},
		{"repeat_intermediate_private_then_text", false, false},
		{"repeat_private_terminal", true, false},
		{"menu_intermediate_private_then_text", false, false},
		{"menu_private_terminal", true, false},
		{"empty_if_branches", false, false},
		{"comment_after_private", true, false},
		{"comment_after_ordinary", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &CanonicalBackendSession{}
			outputs := map[string]bool{"private": true}
			emit := func(id string, p map[string]any, uuid string) {
				t.Helper()
				if err := s.EmitRawAction("is.workflow.actions."+id, p, uuid, "", ""); err != nil {
					t.Fatal(err)
				}
			}
			cell := func() { emit("dictionary", map[string]any{"WFItems": map[string]any{"kind": "null"}}, "private") }
			text := func(id string) { emit("gettext", map[string]any{"WFTextActionText": "ordinary"}, id) }
			boundary := func(id, group string, mode int) {
				emit(id, map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": mode}, "")
			}
			switch tc.name {
			case "comment_after_private", "comment_after_ordinary":
				cell()
				if tc.name == "comment_after_ordinary" {
					text("ordinary")
				}
				emit("comment", map[string]any{"WFCommentActionText": "not a value producer"}, "")
			case "repeat_intermediate_private_then_text", "repeat_private_terminal":
				boundary("repeat.count", "repeat", 0)
				cell()
				if tc.name == "repeat_intermediate_private_then_text" {
					text("body-value")
				}
				boundary("repeat.count", "repeat", 2)
			case "menu_intermediate_private_then_text", "menu_private_terminal":
				boundary("choosefrommenu", "menu", 0)
				boundary("choosefrommenu", "menu", 1)
				cell()
				if tc.name == "menu_intermediate_private_then_text" {
					text("case-one")
				}
				boundary("choosefrommenu", "menu", 1)
				text("case-two")
				boundary("choosefrommenu", "menu", 2)
			default:
				boundary("conditional", "outer", 0)
				switch tc.name {
				case "nested_ordinary_terminals", "nested_private_terminal":
					boundary("conditional", "inner", 0)
					cell()
					if tc.name == "nested_ordinary_terminals" {
						text("inner-then")
					}
					boundary("conditional", "inner", 1)
					text("inner-else")
					boundary("conditional", "inner", 2)
				case "if_intermediate_private_then_text":
					cell()
					text("then-value")
				case "if_private_terminal":
					cell()
				case "if_private_else_terminal":
					text("then-value")
				}
				boundary("conditional", "outer", 1)
				if tc.name == "if_private_else_terminal" {
					cell()
				} else if tc.name != "empty_if_branches" {
					text("else-value")
				}
				boundary("conditional", "outer", 2)
			}
			before, _ := json.Marshal(s.shortcut)
			actual, err := s.CurrentNativeInputUsesPrivateValue(outputs, nil, p15TaggedFixtureValueOutput)
			after, _ := json.Marshal(s.shortcut)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("observer changed native source")
			}
			if actual != tc.wantPrivate || (err != nil) != tc.wantError {
				t.Fatalf("private=%t err=%v; want %t/%t", actual, err, tc.wantPrivate, tc.wantError)
			}
		})
	}
}

func TestP15TaggedExplicitPrivateInputRemainsRejected(t *testing.T) {
	for name, source := range map[string]string{
		"null":          "let value=null\nnative.action(\"is.workflow.actions.detect.text\",{\"WFInput\":value})\nreturn 1",
		"optional_list": "let value=[none,false]\nnative.action(\"is.workflow.actions.detect.text\",{\"WFInput\":value})\nreturn 1",
		"nested_list":   "let value=[[1,2],[]]\nnative.action(\"is.workflow.actions.detect.text\",{\"WFInput\":value})\nreturn 1",
		"optional_map":  "let value:Map<Text,Optional<Bool>>={\"flag\":false}\nnative.action(\"is.workflow.actions.detect.text\",{\"WFInput\":value})\nreturn 1",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("tagged-explicit-"+name+".cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
				t.Fatalf("explicit private input escaped: bytes=%d err=%v", len(raw), err)
			}
		})
	}
}

// The paired authored sources ensure ordinary terminal values remain usable,
// while private terminal values and comments do not bypass the same boundary.
func TestP15TaggedBranchResultNativeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		rejected   bool
	}{
		{"if_private_intermediate_text", "let gate=true\nif gate {let value=null\ntext(\"ordinary\")} else {text(\"other\")}", false},
		{"if_private_intermediate_nothing", "let gate=true\nif gate {let value=null\nnothing()} else {nothing()}", false},
		{"repeat_private_intermediate_text", "repeat 1 {let value=null\ntext(\"ordinary\")}", false},
		{"comment_after_ordinary", "let value=null\ntext(\"ordinary\")\ncomment(\"unchanged cursor\")", false},
		{"if_private_then_terminal", "let gate=true\nif gate {let value=null} else {text(\"other\")}", true},
		{"if_private_else_terminal", "let gate=true\nif gate {text(\"other\")} else {let value=null}", true},
		{"repeat_private_terminal", "repeat 1 {let value=null}", true},
		{"comment_after_private", "let value=null\ncomment(\"unchanged cursor\")", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.body + "\nnative.action(\"is.workflow.actions.detect.text\",{})\nreturn 1"
			if tc.rejected {
				raw, err := CompileSourceToPlist("tagged-branch-"+tc.name+".cherri", source)
				if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
					t.Fatalf("private native boundary was not rejected: bytes=%d err=%v", len(raw), err)
				}
				return
			}
			actions := p15CompiledActions(t, source)
			count := 0
			for _, action := range actions {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.detect.text" {
					count++
					if _, has := p15Param(action)["WFInput"]; has {
						t.Fatal("ordinary implicit-input control became explicit")
					}
				}
			}
			if count != 1 {
				t.Fatalf("expected one actual native consumer, got %d", count)
			}
		})
	}
}
