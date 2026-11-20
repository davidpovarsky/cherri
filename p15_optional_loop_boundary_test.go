package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15OptionalLoopCase struct {
	name, source, result string
}

// These are current-transport safety boundaries, not a proof that Apple could
// never represent an Optional list element. Each source has an explicit main
// return; the main fallthrough policy does not determine its requested value.
func p15OptionalLoopCases() []p15OptionalLoopCase {
	return []p15OptionalLoopCase{
		{"for_literal_none", "let values=for item in [1,2] {yield none}\nreturn values", "none"},
		{"repeat_literal_none", "let values=repeat 2 {yield none}\nreturn values", "none"},
		{"immutable_none_alias", "let candidate:Text?=none\nlet values=repeat 2 {yield candidate}\nreturn values", "candidate"},
		{"inferred_none_alias", "let candidate=none\nlet values=repeat 2 {yield candidate}\nreturn values", "candidate"},
		{"mutable_optional", "var candidate:Text?=none\nlet values=repeat 2 {yield candidate}\nreturn values", "candidate"},
		// Preserve the exact previously accepted discovery source as a required
		// rejection when the collected element's presence cannot be retained.
		{"original_discovery_optional", "function target()->Text?{return none}\nlet value=repeat 2 {yield target()}\nreturn value", "target()"},
		{"function_optional_bool", "function target()->Bool?{return false}\nlet values=repeat 2 {yield target()}\nreturn values", "target()"},
		{"function_optional_map", "function target()->Map<Text,Number>?{return none}\nlet values=repeat 2 {yield target()}\nreturn values", "target()"},
		{"function_parameter", "function collect(candidate:Text?)->AnyContent {let values=repeat 2 {yield candidate}\nreturn values}\nreturn collect(none)", "candidate"},
		{"function_default", "function collect(candidate:Text?=none)->AnyContent {let values=repeat 2 {yield candidate}\nreturn values}\nreturn collect()", "candidate"},
		{"if_join_alias", "let candidate=if true {yield none} else {yield \"row\"}\nlet values=repeat 2 {yield candidate}\nreturn values", "candidate"},
		{"final_if_join", "let values=repeat 2 {if true {yield none} else {yield \"row\"}}\nreturn values", "if true {yield none} else {yield \"row\"}"},
		{"final_menu_join", "let values=repeat 2 {menu(\"Pick\") {case \"A\" {yield none} case \"B\" {yield \"row\"}}}\nreturn values", "menu(\"Pick\") {case \"A\" {yield none} case \"B\" {yield \"row\"}}"},
		{"optional_type_alias", "type MaybeText=Text?\nfunction target()->MaybeText{return none}\nlet values=repeat 2 {yield target()}\nreturn values", "target()"},
		{"optional_number_join", "let candidate=if true {yield 0} else {yield none}\nlet values=for item in [1,2] {yield candidate}\nreturn values", "candidate"},
	}
}

func p15OptionalLoopDiagnosticError(diagnostics []analysis.Diagnostic, tc p15OptionalLoopCase) error {
	if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.result, analysis.CodeTargetValueShape); err != nil {
		return err
	}
	for _, d := range diagnostics {
		if d.Severity != analysis.SeverityError {
			continue
		}
		if d.Code != analysis.CodeTargetValueShape || !strings.Contains(d.Message, "optional absence") || !strings.Contains(d.Message, "iteration") {
			return fmt.Errorf("wrong current value-loop transport boundary: %+v", d)
		}
	}
	return nil
}

func TestP15F04OptionalLoopResultsFailClosed(t *testing.T) {
	for _, tc := range p15OptionalLoopCases() {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15OptionalLoopDiagnosticError(diagnostics, tc); err != nil {
				t.Error("analysis:", err)
			} else {
				for _, fault := range []string{"removed", "shifted_span", "wrong_code"} {
					bad := append([]analysis.Diagnostic(nil), diagnostics...)
					switch fault {
					case "removed":
						bad = nil
					case "shifted_span":
						for i := range bad {
							if bad[i].Code == analysis.CodeTargetValueShape {
								bad[i].Span.Start.ByteOffset++
							}
						}
					case "wrong_code":
						for i := range bad {
							if bad[i].Code == analysis.CodeTargetValueShape {
								bad[i].Code = analysis.CodeArgumentType
							}
						}
					}
					if p15OptionalLoopDiagnosticError(bad, tc) == nil {
						t.Error("diagnostic oracle accepted causal corruption", fault)
					}
				}
			}
			svc := service.NewService(nil)
			uri := "file:///p15-f04-optional-loop-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			observed, _ := svc.Analyze(uri)
			if err := p15OptionalLoopDiagnosticError(observed, tc); err != nil {
				t.Error("service:", err)
			}
			raw, err := CompileSourceToPlist("p15-f04-optional-loop.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				if len(raw) != 0 {
					t.Logf("UNEXPECTED_NATIVE_ACCEPTANCE source_sha256=%x native_sha256=%x plist_base64=%s", sha256.Sum256([]byte(tc.source)), sha256.Sum256(raw), base64.StdEncoding.EncodeToString(raw))
				}
				t.Errorf("unrepresented Optional element reached native emission: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}

func TestP15F04OptionalLoopBoundaryPreservesPresentResults(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"map", "let values=for item in [\"row\"] {yield {\"value\":item}}\nreturn values"},
		{"map_with_list", "let values=repeat 2 {yield {\"nested\":[1,2]}}\nreturn values"},
		{"narrowed_parameter", "function collect(candidate:Text?)->AnyContent {let values=repeat 2 {if candidate!=none {yield candidate} else {yield \"fallback\"}}\nreturn values}\nreturn collect(none)"},
		{"definite_assignment", "var candidate:Text?=none\ncandidate=\"row\"\nlet values=repeat 2 {yield candidate}\nreturn values"},
		{"explicit_present_conversion", "function target()->Text?{return none}\nlet values=repeat 2 {yield f\"{target()}\"}\nreturn values"},
		{"optional_join_outside_loop", "let candidate=if true {yield none} else {yield \"row\"}\nreturn candidate"},
		{"guaranteed_return", "function finished()->Number {let values=repeat 2 {return 7; yield none}\nreturn 0}\nreturn finished()"},
		{"overwritten_none", "let values=repeat 2 {yield none; yield \"row\"}\nreturn values"},
		{"statement_for", "for item in [1,2] {let candidate:Text?=none\nshow(candidate)}\nreturn 1"},
		{"statement_repeat", "repeat 2 {let candidate:Text?=none\nshow(candidate)}\nreturn 1"},
		{"opaque_native_last_action", "let values=repeat 2 {native.action(\"is.workflow.actions.nothing\",{})}\nreturn values"},
		{"explicit_unverified_native_reference", "let values=repeat 2 {yield native.ref(\"ShortcutInput\")}\nreturn values"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			svc := service.NewService(nil)
			uri := "file:///p15-f04-present-loop-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			observed, _ := svc.Analyze(uri)
			for _, d := range observed {
				if d.Severity == analysis.SeverityError {
					t.Fatal("service:", d)
				}
			}
			raw, err := CompileSourceToPlist("p15-f04-present-loop.cherri", tc.source)
			if err != nil || len(raw) == 0 {
				t.Fatalf("present or explicitly unverified boundary rejected: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}

type p15PresentLoopGraphCase struct {
	name, kind, literal string
	count               int
}

func p15PresentLoopGraphCases() []p15PresentLoopGraphCase {
	return []p15PresentLoopGraphCase{
		{"repeat_false", "repeat", "false", 2},
		{"repeat_zero", "repeat", "0", 2},
		{"repeat_empty_text", "repeat", "\"\"", 2},
		{"zero_iterations", "repeat", "\"row\"", 0},
		{"for_false", "for", "false", 1},
		{"for_zero", "for", "0", 1},
		{"for_empty_text", "for", "\"\"", 1},
	}
}

func (tc p15PresentLoopGraphCase) source() string {
	if tc.kind == "for" {
		return "let values=for item in [\"row\"] {yield " + tc.literal + "}\nreturn values"
	}
	return fmt.Sprintf("let values=repeat %d {yield %s}\nreturn values", tc.count, tc.literal)
}

// This is a complete native-structure oracle only. It checks the authored
// count/input/body/output edges; it does not claim observed Repeat cardinality.
func p15PresentLoopGraphError(actual []map[string]any, tc p15PresentLoopGraphCase) error {
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	kind := "repeat.count"
	var loop string
	if tc.kind == "for" {
		kind = "repeat.each"
		items := g.value("list", map[string]any{"WFItems": []any{map[string]any{"WFItemType": 0, "WFValue": "row"}}})
		loop = p15DiscoveryLoop(g, kind, "WFInput", p15FunctionEdge(items))
		g.emit("setvariable", map[string]any{"WFVariableName": "item", "WFInput": p15FunctionVariable("Repeat Item")})
	} else {
		loop = p15DiscoveryLoop(g, kind, "WFRepeatCount", tc.count)
	}
	if tc.literal == "false" || tc.literal == "0" {
		g.value("number", map[string]any{"WFNumberActionNumber": 0})
	} else {
		value := ""
		if tc.literal == "\"row\"" {
			value = "row"
		}
		g.value("gettext", map[string]any{"WFTextActionText": value})
	}
	end := p15DiscoveryLoopEnd(g, kind, loop, true)
	g.output(end)
	return g.compare()
}

func TestP15F04OptionalLoopPresentScalarGraphs(t *testing.T) {
	for _, tc := range p15PresentLoopGraphCases() {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15PresentLoopGraphError(p15CompiledActions(t, tc.source()), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F04OptionalLoopPresentGraphCorruptions(t *testing.T) {
	for _, tc := range p15PresentLoopGraphCases() {
		t.Run(tc.name, func(t *testing.T) {
			original := p15CompiledActions(t, tc.source())
			if err := p15PresentLoopGraphError(original, tc); err != nil {
				t.Fatal("original complete graph rejected:", err)
			}
			for _, fault := range []string{"drop_end", "wrong_iteration_input", "erase_body", "wrong_output", "extra_iteration_action", "literal_changed"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, original)
					start, end, output := -1, -1, -1
					for i, action := range bad {
						id, _ := action["WFWorkflowActionIdentifier"].(string)
						p := p15C01Params(action)
						if strings.HasPrefix(id, "is.workflow.actions.repeat.") {
							if fmt.Sprint(p["WFControlFlowMode"]) == "0" {
								start = i
							} else if fmt.Sprint(p["WFControlFlowMode"]) == "2" {
								end = i
							}
						}
						if id == "is.workflow.actions.output" {
							output = i
						}
					}
					if start < 0 || end <= start || output <= end {
						t.Fatal("actual mutation target absent")
					}
					switch fault {
					case "drop_end":
						bad = append(bad[:end], bad[end+1:]...)
					case "wrong_iteration_input":
						if tc.kind == "for" {
							p15C01Params(bad[start])["WFInput"] = p15FunctionVariable("foreign")
						} else {
							p15C01Params(bad[start])["WFRepeatCount"] = tc.count + 1
						}
					case "erase_body":
						bad = append(bad[:end-1], bad[end:]...)
					case "wrong_output":
						p15C01Params(bad[output])["WFOutput"] = "foreign"
					case "extra_iteration_action":
						extra := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing", "WFWorkflowActionParameters": map[string]any{}}
						bad = append(bad[:end], append([]map[string]any{extra}, bad[end:]...)...)
					case "literal_changed":
						if tc.literal == "false" || tc.literal == "0" {
							p15C01Params(bad[end-1])["WFNumberActionNumber"] = 9
						} else {
							p15C01Params(bad[end-1])["WFTextActionText"] = "foreign"
						}
					}
					if p15PresentLoopGraphError(bad, tc) == nil {
						t.Fatal("complete graph accepted causal corruption", fault)
					}
				})
			}
		})
	}
}
