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
		{"statement_loop", "for item in [1,2] { list([\"one\",\"two\"]) }"},
		{"if_list_outside_loop", "let values=if true { yield [\"one\"] } else { yield [\"two\"] }"},
		{"overwritten_list", "let values=for item in [1,2] { yield [\"discarded\"]; yield item }"},
		{"unknown_explicit_boundary", "let values=for item in [1,2] { yield ShortcutInput }"},
		{"untyped_native_last_action", "let values=repeat 2 { list([\"one\",\"two\"]) }"},
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
