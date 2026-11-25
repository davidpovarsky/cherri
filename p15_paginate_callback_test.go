package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

type p15PageCallbackCase struct {
	name                                                              string
	prefix, body, after                                               string
	mode                                                              string
	empty, previous, localPrevious, nested, captured, shadow, headers bool
}

func p15PageCallbackCases(legacy bool) []p15PageCallbackCase {
	decl, mutable, assign := "let", "var total = 0\n", "total += 1\n"
	if legacy {
		decl, mutable, assign = "const", "@total = 0\n", "@total += 1\n"
	}
	total := "total"
	if legacy {
		total = "@total"
	}
	inner := decl + " nested = paginate(\"https://example.invalid/nested/one.json\", \"items\", \"next\", 2, 4) { page in\ngetVariable(page)\ntext(\"P15_CALLBACK_DONE_INNER\")\nwait(0)\n}\n"
	return []p15PageCallbackCase{
		{name: "reference", body: "getVariable(page)\n"},
		{name: "empty", empty: true},
		{name: "entry_previous", body: "debugLog()\n", previous: true},
		{name: "body_previous", body: "text(\"P15_CALLBACK_LOCAL_INPUT\")\ndebugLog()\n", previous: true, localPrevious: true},
		{name: "captured_write", prefix: mutable, body: assign + "getVariable(" + total + ")\n", after: "getVariable(" + total + ")\n", captured: true},
		{name: "outer_shadow", prefix: decl + " page = text(\"P15_CALLBACK_OUTER_PAGE\")\n", body: "getVariable(page)\n", after: "getVariable(page)\n", shadow: true},
		{name: "nested_same_name", body: inner + "getVariable(page)\n", nested: true},
		{name: "measure", body: "measure(\"page callback\") {\ngetVariable(page)\nwait(0)\n}\n"},
		{name: "cursor", mode: "cursor", body: "getVariable(page)\n"},
		{name: "runtime_headers", prefix: decl + " authorization = text(\"P15_CALLBACK_SYNTHETIC_AUTH\")\n", body: "getVariable(page)\n", headers: true},
	}
}
func p15PageCallbackSource(tc p15PageCallbackCase, legacy bool, pages, items int) string {
	decl, headers := "let", "{}"
	if legacy {
		decl = "const"
	}
	if tc.headers {
		leaf := "authorization"
		if legacy {
			leaf = "\"{authorization}\""
		}
		headers = "{\"Authorization\":" + leaf + "}"
	}
	mode := tc.mode
	if mode == "" {
		mode = "url"
	}
	call := fmt.Sprintf("paginate(\"https://example.invalid/pages/one.json\", \"items\", \"next\", %d, %d, %s, %q)", pages, items, headers, mode)
	body := tc.body
	if !tc.empty {
		body += "text(\"P15_CALLBACK_DONE_OUTER\")\nwait(0)\n"
	}
	return tc.prefix + decl + " rows = " + call + " { page in\n" + body + "}\n" + tc.after + "getVariable(rows)\n"
}
func p15PageCallbackCompile(t *testing.T, text string, legacy bool, profile BuildProfile) ([]ShortcutAction, error) {
	t.Helper()
	actions, err := compileBuildProfileFixture(t, text, legacy, profile)
	if dir := os.Getenv("P15_PAGINATE_CALLBACK_OUT"); dir != "" {
		record := map[string]any{"test": t.Name(), "source": text, "legacy": legacy, "profile": profile, "actions": actions}
		if err != nil {
			record["error"] = err.Error()
		}
		b, e := json.MarshalIndent(record, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		name := strings.NewReplacer("/", "__", "=", "-").Replace(t.Name()) + ".json"
		f, e := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			t.Fatal(e)
		}
		_, e = f.Write(append(b, '\n'))
		closeErr := f.Close()
		if e != nil {
			t.Fatal(e)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	return actions, err
}

type p15PageRegion struct{ id, kind string }
type p15PageGraph struct {
	actions []ShortcutAction
	owners  map[string]int
	regions [][]p15PageRegion
}

func p15PageGraphFor(actions []ShortcutAction) (*p15PageGraph, error) {
	if err := p15LegacyInlineReferenceOracle(actions); err != nil {
		return nil, err
	}
	g := &p15PageGraph{actions: actions, owners: map[string]int{}, regions: make([][]p15PageRegion, len(actions))}
	var stack []p15PageRegion
	for i, a := range actions {
		p := a.WFWorkflowActionParameters
		if id, ok := p["UUID"].(string); ok && id != "" {
			if _, dup := g.owners[id]; dup {
				return nil, fmt.Errorf("duplicate producer")
			}
			g.owners[id] = i
		}
		g.regions[i] = append([]p15PageRegion(nil), stack...)
		kind := a.WFWorkflowActionIdentifier
		if kind != "is.workflow.actions.repeat.count" && kind != "is.workflow.actions.repeat.each" && kind != "is.workflow.actions.conditional" {
			continue
		}
		id, _ := p["GroupingIdentifier"].(string)
		if id == "" {
			return nil, fmt.Errorf("control has no group")
		}
		switch fmt.Sprint(p["WFControlFlowMode"]) {
		case "0":
			stack = append(stack, p15PageRegion{id, kind})
		case "1":
			if len(stack) == 0 || stack[len(stack)-1].id != id || kind != "is.workflow.actions.conditional" {
				return nil, fmt.Errorf("foreign else")
			}
		case "2":
			if len(stack) == 0 || stack[len(stack)-1].id != id || stack[len(stack)-1].kind != kind {
				return nil, fmt.Errorf("crossed control region")
			}
			stack = stack[:len(stack)-1]
		default:
			return nil, fmt.Errorf("unknown control mode")
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("unclosed control")
	}
	return g, nil
}
func p15PageOutput(raw any) (string, error) {
	p, e := p15PreviousObject(raw)
	if e != nil {
		return "", e
	}
	id, ok := p["OutputUUID"].(string)
	if !ok || id == "" || p["Type"] != "ActionOutput" {
		return "", fmt.Errorf("not an original action output")
	}
	for k := range p {
		if k != "Type" && k != "OutputUUID" && k != "OutputName" {
			return "", fmt.Errorf("page reference was transformed")
		}
	}
	return id, nil
}
func (g *p15PageGraph) owns(i int, key string, owner int) bool {
	if i < 0 || i >= len(g.actions) || owner < 0 || owner >= i {
		return false
	}
	id, e := p15PageOutput(g.actions[i].WFWorkflowActionParameters[key])
	return e == nil && g.owners[id] == owner
}
func (g *p15PageGraph) textIndices(value string) []int {
	var out []int
	for i, a := range g.actions {
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && a.WFWorkflowActionParameters["WFTextActionText"] == value {
			out = append(out, i)
		}
	}
	return out
}
func p15PageCallbackGraphError(actions []ShortcutAction, tc p15PageCallbackCase, legacy bool, profile BuildProfile) error {
	g, err := p15PageGraphFor(actions)
	if err != nil {
		return err
	}
	var pages, entries []int
	for i, a := range actions {
		p := a.WFWorkflowActionParameters
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.detect.dictionary" {
			id, e := p15PageOutput(p["WFInput"])
			if e != nil {
				return e
			}
			owner, ok := g.owners[id]
			if !ok || owner >= i || actions[owner].WFWorkflowActionIdentifier != "is.workflow.actions.downloadurl" {
				return fmt.Errorf("page is not decoded from its actual GET")
			}
			pages = append(pages, i)
		}
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" {
			if name, ok := p["CustomOutputName"].(string); ok && strings.HasSuffix(name, "_callback_input") {
				entries = append(entries, i)
			}
		}
	}
	want := 1
	if tc.nested {
		want = 2
	}
	if len(pages) != want || len(entries) != want {
		return fmt.Errorf("page callback cardinality %d pages/%d entries, want %d", len(pages), len(entries), want)
	}
	for n, page := range pages {
		entry := entries[n]
		if !g.owns(entry, "WFVariable", page) || entry <= page || !reflect.DeepEqual(g.regions[entry], g.regions[page]) {
			return fmt.Errorf("callback did not enter from exact validated page in its original guarded iteration")
		}
		repeatCount := 0
		for _, r := range g.regions[entry] {
			if r.kind == "is.workflow.actions.repeat.count" {
				repeatCount++
			}
		}
		if repeatCount != n+1 && tc.nested {
			return fmt.Errorf("nested callback escaped its own page loop")
		}
		if !tc.nested && repeatCount != 1 {
			return fmt.Errorf("callback is not in one bounded page loop")
		}
		validatedItem, closedItems, hasItemsGuard := false, false, false
		for i := page + 1; i < entry; i++ {
			a := actions[i]
			p := a.WFWorkflowActionParameters
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.alert" && p["WFAlertActionMessage"] == "PAGINATE_ITEMS_KEY_REQUIRED" {
				hasItemsGuard = true
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.alert" && p["WFAlertActionMessage"] == "PAGINATE_ITEM_DICTIONARY_REQUIRED" {
				validatedItem = true
			}
			if validatedItem && a.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.each" && fmt.Sprint(p["WFControlFlowMode"]) == "2" {
				closedItems = true
			}
		}
		if !hasItemsGuard || !validatedItem || !closedItems {
			return fmt.Errorf("callback precedes required item validation or runs per item")
		}
		if tc.empty {
			if entry+1 >= len(actions) || actions[entry+1].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" || len(actions[entry+1].WFWorkflowActionParameters) != 0 {
				return fmt.Errorf("empty callback lacks exact Void boundary")
			}
			continue
		}
		label := "P15_CALLBACK_DONE_OUTER"
		if n == 1 {
			label = "P15_CALLBACK_DONE_INNER"
		}
		ends := g.textIndices(label)
		if len(ends) != 1 {
			return fmt.Errorf("body effect %s repeated/lost", label)
		}
		end := ends[0]
		if end <= entry || end+2 >= len(actions) || !reflect.DeepEqual(g.regions[end], g.regions[entry]) {
			return fmt.Errorf("body result escaped its lexical page region")
		}
		if actions[end+1].WFWorkflowActionIdentifier != "is.workflow.actions.wait" || fmt.Sprint(actions[end+1].WFWorkflowActionParameters["WFDelayTime"]) != "0" {
			return fmt.Errorf("body Void effect changed")
		}
		if actions[end+2].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" || len(actions[end+2].WFWorkflowActionParameters) != 0 {
			return fmt.Errorf("body value leaked past exact Void boundary")
		}
		if tc.previous && n == 0 && profile == BuildProfileDebug {
			first := entry + 1
			if tc.localPrevious {
				indices := g.textIndices("P15_CALLBACK_LOCAL_INPUT")
				if len(indices) != 1 || indices[0] != first {
					return fmt.Errorf("authored local Previous producer lost/reordered")
				}
				first++
			}
			if actions[first].WFWorkflowActionIdentifier != "is.workflow.actions.appendvariable" || actions[first].WFWorkflowActionParameters["WFVariableName"] != "__cherri_debug_log" {
				return fmt.Errorf("first consumer is not the original debug append")
			}
			if _, explicit := actions[first].WFWorkflowActionParameters["WFInput"]; explicit {
				owner := entry
				if tc.localPrevious {
					owner = entry + 1
				}
				if !g.owns(first, "WFInput", owner) {
					return fmt.Errorf("Previous consumer redirected to helper/outer input")
				}
			}
		}
	}
	if tc.shadow {
		outer := g.textIndices("P15_CALLBACK_OUTER_PAGE")
		if len(outer) != 1 || outer[0] >= pages[0] {
			return fmt.Errorf("outer page declaration lost")
		}
		found := false
		for i := entries[0] + 1; i < len(actions); i++ {
			if actions[i].WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" && g.owns(i, "WFVariable", outer[0]) {
				if len(g.regions[i]) != 0 {
					return fmt.Errorf("page parameter resolved to outer value inside callback")
				}
				found = true
			}
		}
		if !found {
			return fmt.Errorf("outer page binding did not recover after callback")
		}
	}
	if tc.captured {
		writes := 0
		for i, a := range actions {
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && a.WFWorkflowActionParameters["WFVariableName"] == "total" {
				writes++
				if writes == 1 && len(g.regions[i]) != 0 {
					return fmt.Errorf("captured initialization moved into callback")
				}
				if writes == 2 && !reflect.DeepEqual(g.regions[i], g.regions[entries[0]]) {
					return fmt.Errorf("captured write escaped callback")
				}
			}
		}
		if writes != 2 {
			return fmt.Errorf("captured slot has %d stores, want declaration+once-per-page assignment", writes)
		}
	}
	if !legacy {
		if err := p15M02MainVoidDebugError(actions); err != nil {
			return err
		}
	}
	return nil
}
func TestP15PaginatePageCallbackGraphs(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, tc := range p15PageCallbackCases(legacy) {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", tc.name, legacy, profile), func(t *testing.T) {
					actions, err := p15PageCallbackCompile(t, p15PageCallbackSource(tc, legacy, 3, 6), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err = p15PageCallbackGraphError(actions, tc, legacy, profile); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
func TestP15PaginatePageCallbackZeroBudgets(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			for _, caps := range [][2]int{{0, 6}, {3, 0}} {
				t.Run(fmt.Sprintf("%d-%d/legacy=%t/%s", caps[0], caps[1], legacy, profile), func(t *testing.T) {
					tc := p15PageCallbackCases(legacy)[2]
					actions, err := p15PageCallbackCompile(t, p15PageCallbackSource(tc, legacy, caps[0], caps[1]), legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					want := []string{"list", "setvariable", "getvariable", "getvariable"}
					if !legacy {
						want = append(want, "nothing")
					}
					if !reflect.DeepEqual(actionSuffixes(actions), want) {
						t.Fatalf("zero caps emitted body/request/helper work: %v", actionSuffixes(actions))
					}
					rows := reflect.ValueOf(actions[0].WFWorkflowActionParameters["WFItems"])
					if !rows.IsValid() || rows.Kind() != reflect.Slice || rows.IsNil() || rows.Len() != 0 {
						t.Fatal("zero cap fabricated a result")
					}
				})
			}
		}
	}
}
func TestP15PaginatePageCallbackOrdinaryControl(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, profile), func(t *testing.T) {
				tc := p15PaginateTraversalCase{mode: "url", start: "https://example.invalid/pages/one.json"}
				actions, err := p15PageCallbackCompile(t, p15PaginateTraversalSource(tc, legacy, false, false, 3, 6, true), legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				if err = p15PaginateTraversalGraphError(actions, tc, legacy); err != nil {
					t.Fatal(err)
				}
				for _, a := range actions {
					if name, _ := a.WFWorkflowActionParameters["CustomOutputName"].(string); strings.HasSuffix(name, "_callback_input") {
						t.Fatal("ordinary call gained a callback")
					}
				}
			})
		}
	}
}
func TestP15PaginatePageCallbackTypedLocalFlow(t *testing.T) {
	cases := []struct {
		name, prefix, body, tail string
		pages                    int
	}{
		{"mutable_origins", "var values=repeat 0 {yield []}\n", "values=repeat 2 {yield [\"a\",\"b\"]}\nlet n=count(values)\n", "return count(values)\n", 3},
		{"two_backedges", "var first=repeat 0 {yield []}\nvar second=repeat 1 {yield [\"seed\"]}\n", "first=second\nsecond=repeat 2 {yield [\"new\"]}\n", "return count(first)\n", 3},
		{"parameter_shadow", "let page=\"outside\"\n", "let inside:Map<Text,AnyContent>=page\nif true {\nlet page=\"inner\"\ngetVariable(page)\nwait(0)\n}\ngetVariable(page)\n", "getVariable(page)\n", 3},
		{"nested_container", "var values:List<List<Optional<Bool>>>=[[false]]\n", "values=[[none],[false]]\nlet n=count(values)\n", "return count(values)\n", 3},
		{"zero_captured", "var values=repeat 1 {yield [\"entry\"]}\n", "values=repeat 2 {yield [\"never\"]}\n", "return count(values)\n", 0},
	}
	for _, tc := range cases {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(tc.name+"/"+string(profile), func(t *testing.T) {
				src := tc.prefix + fmt.Sprintf("let rows=paginate(\"https://example.invalid/pages/one.json\", \"items\", \"next\", %d, 6) { page in\n", tc.pages) + tc.body + "wait(0)\n}\n" + tc.tail
				actions, err := p15PageCallbackCompile(t, src, false, profile)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = p15PageGraphFor(actions); err != nil {
					t.Fatal(err)
				}
				requests := 0
				for _, a := range actions {
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.downloadurl" {
						requests++
					}
				}
				want := 1
				if tc.pages == 0 {
					want = 0
				}
				if requests != want {
					t.Fatal("captured source changed request cap")
				}
			})
		}
	}
}
func TestP15PaginatePageCallbackTypedParameter(t *testing.T) {
	text := "let rows=paginate(\"https://example.invalid/pages/one.json\") { page in\ngetVariable(page)\nwait(0)\n}\ngetVariable(rows)\n"
	p := syntax.NewParser(source.NewFile("page-type.cherri", "page-type.cherri", 1, text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := analysis.NewAnalyzer(schema.DefaultRegistry())
	a.Analyze(program)
	for _, d := range a.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	call := program.Statements[0].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
	use := call.Block.Body.Statements[0].(*syntax.ExprStmt).Expr.(*syntax.CallExpr).PrimaryArg
	pageType := types.NewMap(types.Text, types.AnyContent)
	if got := a.ExprTypes()[use]; got == nil || !pageType.Equals(got) {
		t.Fatalf("page is not the original typed Dictionary: %v", got)
	}
	if got := a.ExprTypes()[call]; got == nil || !types.NewList(pageType).Equals(got) {
		t.Fatalf("callback changed typed aggregate result: %v", got)
	}
}
func TestP15PaginatePageCallbackRejectsUnconsumedValues(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		decl := "let"
		assign := "page = {}"
		immutable := "E_ASSIGN_IMMUTABLE"
		if legacy {
			decl = "const"
			assign = "@page = {}"
			immutable = "E_BLOCK_CONSTANT_ASSIGNMENT"
		}
		cases := []struct{ name, header, body, tail, diagnostic string }{
			{"value_tail", "page in", "text(\"unconsumed\")", "", "E_PAGINATE_BODY_VOID"},
			{"yield", "page in", "yield page", "", "E_PAGINATE_BODY_VOID"},
			{"return", "page in", "return page", "", "E_PAGINATE_BODY_VOID"},
			{"parameter_write", "page in", assign + "\nwait(0)", "", immutable},
			{"missing_parameter", "", "wait(0)", "", "E_PAGINATE_PARAMETER"},
			{"parameter_escape", "page in", "wait(0)", "getVariable(page)", "undefined"},
			{"typed_control_tail", "page in", "if 1 == 1 {\ntext(\"unconsumed\")\n} else {\ntext(\"other\")\n}", "", "E_PAGINATE_BODY_VOID"},
			{"zero_still_checks", "page in", "yield page", "", "E_PAGINATE_BODY_VOID"},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/legacy=%t", tc.name, legacy), func(t *testing.T) {
				cap := 3
				if tc.name == "zero_still_checks" {
					cap = 0
				}
				src := decl + " rows = " + fmt.Sprintf("paginate(\"https://example.invalid/pages/one.json\", \"items\", \"next\", %d, 6)", cap) + " { " + tc.header + "\n" + tc.body + "\n}\n" + tc.tail + "\ngetVariable(rows)\n"
				_, err := p15PageCallbackCompile(t, src, legacy, BuildProfileDebug)
				if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.diagnostic)) {
					t.Fatalf("missing precise %s rejection: %v", tc.diagnostic, err)
				}
			})
		}
	}
}
