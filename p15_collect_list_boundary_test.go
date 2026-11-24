package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

func TestP15F04KnownListLoopResultsFailClosed(t *testing.T) {
	for _, tc := range []struct{ name, source, result string }{
		{"for_literal", "let values=for item in [1,2] { yield [item, \"tail\"] }", "[item, \"tail\"]"},
		{"for_empty", "let values=for item in [1,2] { yield [] }", "[]"},
		{"repeat_literal", "let values=repeat 2 { yield [\"one\"] }", "[\"one\"]"},
		{"repeat_empty", "let values=repeat 2 { yield [] }", "[]"},
		{"bound_list", "let pair=[\"left\",\"tail\"]\nlet values=for item in [1,2] { yield pair }", "pair"},
		{"mutable_list", "var pair=[\"left\",\"tail\"]\nlet values=repeat 2 { yield pair }", "pair"},
		{"function_result", "function pair() -> List<Text> { return [\"left\",\"tail\"] }\nlet values=repeat 2 { yield pair() }", "pair()"},
		{"optional_list", "function collect(items:List<Text>?) -> AnyContent { let values=repeat 2 { yield items }\nreturn values }", "items"},
		{"if_union", "let values=for item in [1,2] { if item==1 { yield [\"one\"] } else { yield \"two\" } }", "if item==1 { yield [\"one\"] } else { yield \"two\" }"},
		{"menu_union", "let values=repeat 2 { menu(\"pick\") { case \"list\" { yield [\"one\"] } case \"scalar\" { yield \"two\" } } }", "menu(\"pick\") { case \"list\" { yield [\"one\"] } case \"scalar\" { yield \"two\" } }"},
		{"nested_loop", "let values=for item in [1,2] { for inner in [3,4] { yield inner } }", "for inner in [3,4] { yield inner }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.result, analysis.CodeTargetValueShape); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-f04-list-boundary.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatalf("unsupported nested List reached native emission: bytes=%d err=%v", len(raw), err)
			}
		})
	}
}

func TestP15F04SupportedLoopResultsRemainAvailable(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"text", "let values=for item in [\"left\",\"right\"] { yield item }"},
		{"number_zero", "let values=repeat 2 { yield 0 }"},
		{"false", "let values=repeat 2 { yield false }"},
		{"empty_text", "let values=repeat 2 { yield \"\" }"},
		{"record", "let values=for item in [1,2] { yield {\"value\":item} }"},
		{"record_containing_list", "let values=repeat 2 { yield {\"nested\":[\"one\",\"two\"]} }"},
		{"opaque_native_reference", "var items=[\"one\",\"two\"]\nlet values=repeat 2 { yield native.ref(\"Variable\", name:\"items\") }"},
		{"guaranteed_return", "function finished() -> Number { let values=repeat 2 { return 7; yield [\"unreachable\"] }\nreturn 0 }\nlet value=finished()"},
		{"statement_loop", "for item in [1,2] { list(\"one\",\"two\") }"},
		{"if_list_outside_loop", "let values=if true { yield [\"one\"] } else { yield [\"two\"] }"},
		{"overwritten_list", "let values=for item in [1,2] { yield [\"discarded\"]; yield item }"},
		{"unknown_explicit_boundary", "let values=for item in [1,2] { yield ShortcutInput }"},
		{"untyped_native_last_action", "let values=repeat 2 { list(\"one\",\"two\") }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, diagnostic := range p15TypeDiagnostics(t, tc.source) {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			raw, err := CompileSourceToPlist("p15-f04-supported-boundary.cherri", tc.source)
			if err != nil || len(raw) == 0 {
				t.Fatalf("existing supported or explicitly unverified path rejected: bytes=%d err=%v", len(raw), err)
			}
		})
	}
}

func TestP15F04ReturnOnlyLoopBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, loop, termination, payload, nativeKind string
		repeatCount                                  int
	}{
		{"repeat_zero_list", "repeat 0", "return 7", "[\"tail\"]", "repeat.count", 0},
		{"for_list", "for item in [1,2]", "return 7", "[\"tail\"]", "repeat.each", -1},
		{"for_optional", "for item in [1,2]", "return 7", "none", "repeat.each", -1},
		{"for_zero_optional", "for item in []", "return 7", "none", "repeat.each", -1},
		{"repeat_if_all_return", "repeat 2", "if flag {return 7} else {return 8}", "[\"tail\"]", "repeat.count", 2},
		{"for_menu_all_return", "for item in [1,2]", "menu(\"Pick\") {case \"A\" {return 7} case \"B\" {return 8}}", "none", "repeat.each", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "function finished(flag:Bool)->Number {let values=" + tc.loop + " {" + tc.termination + "\nyield " + tc.payload + "}\nreturn 0}\nreturn finished(true)"
			for _, diagnostic := range p15TypeDiagnostics(t, source) {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			actions := p15CompiledActions(t, source)
			starts, ends := 0, 0
			group := ""
			for _, action := range actions {
				identifier := action["WFWorkflowActionIdentifier"]
				parameters := p15Param(action)
				if identifier == "is.workflow.actions.appendvariable" {
					name, _ := parameters["WFVariableName"].(string)
					if strings.Contains(name, "loopResults_") {
						t.Fatal("return-only body acquired a result-cell append")
					}
				}
				if identifier != "is.workflow.actions."+tc.nativeKind {
					continue
				}
				switch p15Int(parameters["WFControlFlowMode"]) {
				case 0:
					starts++
					group, _ = parameters["GroupingIdentifier"].(string)
					if group == "" {
						t.Fatal("loop start lost its group identity")
					}
					if tc.repeatCount >= 0 && p15Int(parameters["WFRepeatCount"]) != tc.repeatCount {
						t.Fatal("ordinary loop count changed", parameters)
					}
				case 2:
					ends++
					if group == "" || parameters["GroupingIdentifier"] != group {
						t.Fatal("loop end lost its start ancestry")
					}
				}
			}
			if starts != 1 || ends != 1 {
				t.Fatalf("ordinary loop boundaries changed: starts=%d ends=%d", starts, ends)
			}
		})
	}
}

func TestP15F04NormalContinuationKeepsShapeBoundary(t *testing.T) {
	// Removing one guaranteed return must not admit a normally completed
	// List/Optional result. A return inside a zero-iteration inner loop is not
	// a guaranteed return from the outer body.
	for _, tc := range []struct{ name, loop, termination, payload string }{
		{"if_one_path_returns_list", "repeat 2", "if flag {return 7} else {let kept=8}", "[\"tail\"]"},
		{"if_one_path_returns_optional", "repeat 2", "if flag {return 7} else {let kept=8}", "none"},
		{"menu_one_path_returns_optional", "for item in [1,2]", "menu(\"Pick\") {case \"A\" {return 7} case \"B\" {let kept=8}}", "none"},
		{"inner_zero_repeat_return", "repeat 2", "repeat 0 {return 7}", "[\"tail\"]"},
		{"zero_repeat_normal_result", "repeat 0", "", "[\"tail\"]"},
		{"zero_for_normal_optional", "for item in []", "", "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "function finished(flag:Bool)->Number {let values=" + tc.loop + " {" + tc.termination + "\nyield " + tc.payload + "}\nreturn 0}\nreturn finished(true)"
			diagnostics := p15TypeDiagnostics(t, source)
			if err := p15ExactLiteralDiagnostic(diagnostics, source, tc.payload, analysis.CodeTargetValueShape); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-f04-normal-continuation.cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatalf("normal List/Optional result escaped the shape boundary: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}
