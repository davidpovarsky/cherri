package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

func p15PageParameterGraphError(actions []ShortcutAction, tc p15PageCallbackCase, legacy bool) error {
	if err := p15PageCallbackGraphError(actions, tc, legacy, BuildProfileDebug); err != nil {
		return err
	}
	g, err := p15PageGraphFor(actions)
	if err != nil {
		return err
	}
	entry := -1
	for i, a := range actions {
		if n, _ := a.WFWorkflowActionParameters["CustomOutputName"].(string); strings.HasSuffix(n, "_callback_input") {
			entry = i
			break
		}
	}
	if entry < 0 || entry+1 >= len(actions) || actions[entry+1].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" ||
		!g.owns(entry+1, "WFVariable", entry) {
		return fmt.Errorf("authored page read lost its actual immutable entry identity")
	}
	return nil
}

func TestP15PaginatePageCallbackNativeGraphFaults(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			tc := p15PageCallbackCases(legacy)[0]
			original, err := p15PageCallbackCompile(t, p15PageCallbackSource(tc, legacy, 3, 6), legacy, BuildProfileDebug)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15PageParameterGraphError(original, tc, legacy); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"remove_entry", "entry_source", "entry_response", "entry_name", "entry_duplicate_uuid", "entry_before_validation", "missing_void", "wrong_wait", "missing_effect", "duplicate_effect", "page_source", "body_source", "body_coercion", "body_variable"} {
				t.Run(fault, func(t *testing.T) {
					var actions []ShortcutAction
					if err := json.Unmarshal(raw, &actions); err != nil {
						t.Fatal(err)
					}
					if err := p15PageParameterGraphError(actions, tc, legacy); err != nil {
						t.Fatal("native plist-shape clone changed positive", err)
					}
					entry, page, response, end := -1, -1, -1, -1
					for i, a := range actions {
						p := a.WFWorkflowActionParameters
						if n, _ := p["CustomOutputName"].(string); strings.HasSuffix(n, "_callback_input") {
							entry = i
						}
						if a.WFWorkflowActionIdentifier == "is.workflow.actions.detect.dictionary" {
							page = i
						}
						if a.WFWorkflowActionIdentifier == "is.workflow.actions.downloadurl" {
							response = i
						}
						if a.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && p["WFTextActionText"] == "P15_CALLBACK_DONE_OUTER" {
							end = i
						}
					}
					if entry < 0 || page < 0 || response < 0 || end < 0 {
						t.Fatal("missing original graph witnesses")
					}
					ref := func(owner int) any {
						p := actions[owner].WFWorkflowActionParameters
						return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": p["UUID"], "OutputName": p["CustomOutputName"]}}
					}
					switch fault {
					case "remove_entry":
						actions = append(actions[:entry], actions[entry+1:]...)
					case "entry_source":
						actions[entry].WFWorkflowActionParameters["WFVariable"] = map[string]any{"Type": "ActionOutput", "OutputUUID": "99000000-1111-4222-8333-000000000001"}
					case "entry_response":
						actions[entry].WFWorkflowActionParameters["WFVariable"] = ref(response)
					case "entry_name":
						actions[entry].WFWorkflowActionParameters["CustomOutputName"] = "foreign"
					case "entry_duplicate_uuid":
						actions[entry].WFWorkflowActionParameters["UUID"] = actions[page].WFWorkflowActionParameters["UUID"]
					case "entry_before_validation":
						action := actions[entry]
						actions = append(actions[:entry], actions[entry+1:]...)
						at := page + 1
						actions = append(actions[:at], append([]ShortcutAction{action}, actions[at:]...)...)
					case "missing_void":
						actions = append(actions[:end+2], actions[end+3:]...)
					case "wrong_wait":
						actions[end+1].WFWorkflowActionParameters["WFDelayTime"] = 1
					case "missing_effect":
						actions = append(actions[:end], actions[end+1:]...)
					case "duplicate_effect":
						action := ShortcutAction{WFWorkflowActionIdentifier: "is.workflow.actions.gettext", WFWorkflowActionParameters: map[string]any{"WFTextActionText": "P15_CALLBACK_DONE_OUTER"}}
						actions = append(actions[:end], append([]ShortcutAction{action}, actions[end:]...)...)
					case "page_source":
						actions[page].WFWorkflowActionParameters["WFInput"] = ref(page)
					case "body_source":
						actions[entry+1].WFWorkflowActionParameters["WFVariable"] = ref(response)
					case "body_coercion":
						value := ref(entry).(map[string]any)
						value["Value"].(map[string]any)["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
						actions[entry+1].WFWorkflowActionParameters["WFVariable"] = value
					case "body_variable":
						actions[entry+1].WFWorkflowActionParameters["WFVariable"] = map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": "page"}}
					}
					if err := p15PageParameterGraphError(actions, tc, legacy); err == nil {
						t.Fatal("causal page/value/order corruption was accepted")
					}
				})
			}
		})
	}
}

func TestP15PaginatePageCallbackEmissionBoundaries(t *testing.T) {
	for _, mode := range []string{"capture_error", "body_error", "incomplete", "success", "helper_error", "zero_budget"} {
		t.Run(mode, func(t *testing.T) {
			sentinel := errors.New("original callback emission failure")
			var emitted []ShortcutAction
			bodies := 0
			callback := backend.ScopedPageCallback{
				InputName: "__test_callback_input",
				EmitAction: func(id string, p map[string]any) error {
					emitted = append(emitted, ShortcutAction{WFWorkflowActionIdentifier: id, WFWorkflowActionParameters: p})
					if mode == "capture_error" || mode == "helper_error" {
						return sentinel
					}
					return nil
				},
				Body: func(page backend.Reference) error {
					bodies++
					if page.Kind != backend.RefActionResult || page.ProducerID == "" || page.ProducerName != "__test_callback_input" {
						t.Fatal("page identity lost")
					}
					if mode == "body_error" {
						return sentinel
					}
					return nil
				},
			}
			if mode == "incomplete" {
				callback.Body = nil
			}
			if mode == "helper_error" || mode == "zero_budget" {
				params := map[string]any{"WFPaginateStartURL": "https://example.invalid/one"}
				if mode == "zero_budget" {
					params["WFPaginateMaxPages"] = 0
				}
				_, err := buildScopedPaginateMacroActions(params, "", "", &callback)
				if mode == "helper_error" {
					if !errors.Is(err, sentinel) || bodies != 0 || len(emitted) != 1 {
						t.Fatalf("helper failure lost/continued: %v/%d/%d", err, bodies, len(emitted))
					}
				} else {
					if err != nil || bodies != 0 || len(emitted) != 3 {
						t.Fatalf("zero cap executed callback: %v/%d/%d", err, bodies, len(emitted))
					}
				}
				return
			}
			b := paginateExpansion{callback: &callback}
			b.emitPageCallback(debugOutputRefNamed("88000000-1111-4222-8333-000000000001", "validated page"))
			switch mode {
			case "capture_error":
				if !errors.Is(b.err, sentinel) || bodies != 0 || len(emitted) != 1 {
					t.Fatal("capture failure emitted body or lost error")
				}
			case "body_error":
				if !errors.Is(b.err, sentinel) || bodies != 1 || len(emitted) != 1 {
					t.Fatal("body failure emitted later helper or lost error")
				}
			case "incomplete":
				if b.err == nil || bodies != 0 || len(emitted) != 0 {
					t.Fatal("incomplete contract emitted")
				}
			case "success":
				if b.err != nil || bodies != 1 || len(emitted) != 2 || emitted[1].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" || len(emitted[1].WFWorkflowActionParameters) != 0 {
					t.Fatal("normal Void boundary changed")
				}
			}
		})
	}
}
