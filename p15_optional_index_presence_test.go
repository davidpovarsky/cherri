package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15OptionalIndexCase struct{ name, typ, list, index string }

func p15OptionalIndexCases() []p15OptionalIndexCase {
	return []p15OptionalIndexCase{
		{"text", "Text", `["ready"]`, "0"}, {"empty_text", "Text", `[""]`, "0"},
		{"false", "Bool", `[false]`, "0"}, {"zero", "Number", `[0]`, "0"},
		{"empty_map", "Map<Text,AnyContent>", `[{}]`, "0"},
		{"map", "Map<Text,Text>", `[{"key":"VALUE"}]`, "0"},
		{"second_slot", "Text", `["first","second"]`, "1"},
		{"integral_decimal", "Text", `["ready"]`, "0.0"},
		{"integral_exponent", "Text", `["ready"]`, "0e0"},
	}
}
func p15OptionalIndexSource(tc p15OptionalIndexCase, route string) string {
	selected := tc.list + "[" + tc.index + "]"
	switch route {
	case "binding":
		return "var candidate:" + tc.typ + "?=" + selected + "\ngetVariable(candidate)"
	case "argument":
		return "function echo(value:" + tc.typ + "?)->" + tc.typ + "?{return value}\ngetVariable(echo(" + selected + "))"
	case "return":
		return "function pick()->" + tc.typ + "?{return " + selected + "}\ngetVariable(pick())"
	default:
		panic("unknown source route")
	}
}
func TestP15OptionalStaticIndexPublicRoutes(t *testing.T) {
	for _, tc := range p15OptionalIndexCases() {
		for _, route := range []string{"binding", "argument", "return"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				source := p15OptionalIndexSource(tc, route)
				for _, d := range p15TypeDiagnostics(t, source) {
					if d.Severity == analysis.SeverityError {
						t.Fatal(d)
					}
				}
				if raw, err := CompileSourceToPlist("p15-static-optional-index.cherri", source); err != nil || len(raw) == 0 {
					t.Fatalf("source-owned in-bounds Optional index failed: bytes=%d err=%v", len(raw), err)
				}
				svc := service.NewService(nil)
				uri := "file:///p15-static-optional-index.cherri"
				svc.OpenDocument(uri, 1, source)
				diagnostics, _ := svc.Analyze(uri)
				for _, d := range diagnostics {
					if d.Severity == analysis.SeverityError {
						t.Fatal("editor disagrees", d)
					}
				}
			})
		}
	}
}

func p15OptionalIndexPredicateError(actions []map[string]any) error {
	last := -1
	producers := map[string]map[string]any{}
	for i, a := range actions {
		p := p15Param(a)
		if id, ok := p["UUID"].(string); ok {
			producers[id] = a
		}
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0 {
			last = i
		}
	}
	if last < 0 {
		return fmt.Errorf("missing final presence predicate")
	}
	p := p15Param(actions[last])
	if p15Int(p["WFCondition"]) != 5 || p["WFConditionalActionString"] != "0" {
		return fmt.Errorf("index predicate did not compare owned absence flag: %#v", p)
	}
	wrapped, ok := p["WFInput"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing native predicate input")
	}
	wire, ok := wrapped["Variable"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing presence attachment")
	}
	ref, ok := wire["Value"].(map[string]any)
	if !ok || ref["Type"] != "ActionOutput" {
		return fmt.Errorf("presence is not a produced flag")
	}
	flag := producers[fmt.Sprint(ref["OutputUUID"])]
	if flag == nil || flag["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || p15Param(flag)["WFTextActionText"] != "1" {
		return fmt.Errorf("static indexed slot lacks exact source-owned present flag")
	}
	return nil
}
func TestP15OptionalStaticIndexPredicatesUsePresence(t *testing.T) {
	for _, tc := range p15OptionalIndexCases() {
		t.Run(tc.name, func(t *testing.T) {
			source := "let result=" + tc.list + "[" + tc.index + "]!=none\ngetVariable(result)"
			actions := p15CompiledActions(t, source)
			if err := p15OptionalIndexPredicateError(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type p15OptionalIndexNegative struct {
	name, source, target string
	extra                []string
}

func p15OptionalIndexNegatives() []p15OptionalIndexNegative {
	return []p15OptionalIndexNegative{
		{"negative", `let selected:Text?=["ready"][-1]`, `["ready"][-1]`, nil},
		{"out_of_range", `let selected:Text?=["ready"][1]`, `["ready"][1]`, nil},
		{"fraction", `let selected:Text?=["ready"][0.5]`, `["ready"][0.5]`, nil},
		{"dynamic_call", `let selected:Text?=["ready"][number(0)]`, `["ready"][number(0)]`, nil},
		{"dynamic_binding", `let index=0;let selected:Text?=["ready"][index]`, `["ready"][index]`, nil},
		{"target_alias", `let values=["ready"];let selected:Text?=values[0]`, `values[0]`, nil},
		{"map_lookup", `var values={"key":"ready"};let selected:Text?=values["key"]`, `values["key"]`, nil},
		{"optional_element", `let maybe:Text?=none;let selected:Text?=[maybe][0]`, `[maybe][0]`, []string{"maybe"}},
		{"optional_result_element", `function maybe()->Text?{return none};let selected:Text?=[maybe()][0]`, `[maybe()][0]`, []string{"maybe()"}},
		{"opaque_element", `let selected:AnyContent?=[native.action("is.workflow.actions.nothing",{})][0]`, `[native.action("is.workflow.actions.nothing",{})][0]`, nil},
	}
}
func p15OptionalIndexBoundaryError(diags []analysis.Diagnostic, tc p15OptionalIndexNegative) error {
	targets := append([]string{tc.target}, tc.extra...)
	errors := []analysis.Diagnostic{}
	for _, d := range diags {
		if d.Severity == analysis.SeverityError {
			errors = append(errors, d)
		}
	}
	if len(errors) != len(targets) {
		return fmt.Errorf("want %d independent transport diagnostics, got %+v", len(targets), errors)
	}
	for _, target := range targets {
		start := strings.LastIndex(tc.source, target)
		matches := 0
		for _, d := range errors {
			if d.Code == analysis.CodeTargetValueShape && d.Span.Start.ByteOffset == start && d.Span.End.ByteOffset == start+len(target) {
				matches++
			}
		}
		if start < 0 || matches != 1 {
			return fmt.Errorf("missing exact indexed-value boundary at %q: %+v", target, errors)
		}
	}
	return nil
}
func TestP15OptionalStaticIndexRejectsUnprovenPresence(t *testing.T) {
	for _, tc := range p15OptionalIndexNegatives() {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15OptionalIndexBoundaryError(diagnostics, tc); err != nil {
				t.Fatal(err)
			}
			if raw, err := CompileSourceToPlist("p15-unproved-index.cherri", tc.source); err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatalf("unproved indexed value emitted bytes=%d error=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-unproved-index.cherri"
			svc.OpenDocument(uri, 1, tc.source)
			got, _ := svc.Analyze(uri)
			if err := p15OptionalIndexBoundaryError(got, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func p15OptionalIndexCaptureError(actions []map[string]any) error {
	stateWrites := []int{}
	capture, list := -1, -1
	captureID := ""
	for i, a := range actions {
		p := p15Param(a)
		id, _ := a["WFWorkflowActionIdentifier"].(string)
		if id == "is.workflow.actions.setvariable" && p["WFVariableName"] == "state" {
			stateWrites = append(stateWrites, i)
		}
		if id == "is.workflow.actions.getvariable" && reflect.DeepEqual(p15FunctionComparable(p["WFVariable"], false), p15FunctionVariable("state")) {
			if capture >= 0 {
				return fmt.Errorf("duplicate authored early read")
			}
			capture = i
			captureID, _ = p["UUID"].(string)
		}
		if id == "is.workflow.actions.list" {
			if list >= 0 {
				return fmt.Errorf("duplicate native List")
			}
			list = i
		}
	}
	if len(stateWrites) != 2 || captureID == "" || list < 0 || !(stateWrites[0] < capture && capture < stateWrites[1] && stateWrites[1] < list) {
		return fmt.Errorf("earlier List slot was not captured before sibling write: writes=%v capture=%d list=%d", stateWrites, capture, list)
	}
	rows, ok := p15Param(actions[list])["WFItems"].([]any)
	if !ok || len(rows) != 2 {
		return fmt.Errorf("List cardinality changed")
	}
	row, ok := rows[0].(map[string]any)
	if !ok || p15Int(row["WFItemType"]) != 0 || !reflect.DeepEqual(p15FunctionComparable(row["WFValue"], false), p15FunctionEdge(captureID)) {
		return fmt.Errorf("selected List row no longer binds exact early snapshot")
	}
	return nil
}
func TestP15OptionalIndexKeepsLiteralEvaluationOrder(t *testing.T) {
	source := `var state="BEFORE"
getVariable([state,if true{state="AFTER";yield "SECOND"}else{yield "UNUSED"}][0])`
	actions := p15CompiledActions(t, source)
	if err := p15OptionalIndexCaptureError(actions); err != nil {
		if os.MkdirAll("dist/p15-optional-index", 0700) == nil {
			if raw, e := json.MarshalIndent(actions, "", "  "); e == nil {
				_ = os.WriteFile("dist/p15-optional-index/capture-baseline.json", raw, 0600)
			}
		}
		t.Fatal(err)
	}
}

func TestP15OptionalStaticIndexKnownPresentElement(t *testing.T) {
	source := `let maybe:Text?="";let selected:Text?=[maybe][0];getVariable(selected)`
	for _, d := range p15TypeDiagnostics(t, source) {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	if raw, err := CompileSourceToPlist("p15-known-present-index.cherri", source); err != nil || len(raw) == 0 {
		t.Fatalf("known present slot failed: bytes=%d error=%v", len(raw), err)
	}
}
