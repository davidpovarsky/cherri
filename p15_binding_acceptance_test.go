package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15V02Case struct {
	name, source, target, code string
}

const p15V02Flag = "let flag = number(0) == 1\n"
const p15V02Optional = "var maybe: Text? = [\"ready\"][0]\n"

var p15V02Invalid = []p15V02Case{
	{"unicode_read", "// \U0001f9ea\nvar \u05d0: Number\ntext(\"\U0001f9ea\"); number(\u05d0)", "\u05d0", analysis.CodeReadBeforeInit},
	{"guard_truthiness", p15V02Optional + "if maybe {let bad: Text = maybe}", "maybe", analysis.CodeArgumentType},
	{"guard_json_null", p15V02Optional + "if maybe != null {let bad: Text = maybe}", "maybe", analysis.CodeArgumentType},

	{"immutable", "let value = 1\nvalue = 2", "value = 2", analysis.CodeAssignImmutable},
	{"immutable_nested", "let value = 1\nif true { value += 2 }", "value += 2", analysis.CodeAssignImmutable},
	{"duplicate", "let value = 1\nlet value = 2", "let value = 2", analysis.CodeDuplicateName},
	{"unknown_scope", "if true {let local = 1}\nnumber(local)", "local", analysis.CodeUnknownName},
	{"forward_read", "let copy = later\nlet later = 1", "later", analysis.CodeUnknownName},
	{"before_init", "var value: Number\nnumber(value)", "value", analysis.CodeReadBeforeInit},
	{"self_init", "var value: Number\nvalue = value + 1", "value", analysis.CodeReadBeforeInit},
	{"compound_add", "var value: Number\nvalue += 1", "value += 1", analysis.CodeReadBeforeInit},
	{"compound_multiply", "var value: Number\nvalue *= 2", "value *= 2", analysis.CodeReadBeforeInit},
	{"if_missing_else", p15V02Flag + "var value: Number\nif flag {value = 1}\nnumber(value)", "value", analysis.CodeReadBeforeInit},
	{"if_else_reads_uninitialized", p15V02Flag + "var value: Number\nif flag {value = 1} else {number(value)}", "value", analysis.CodeReadBeforeInit},
	{"nested_missing_path", p15V02Flag + "var value: Number\nif flag {if flag {value = 1}} else {value = 2}\nnumber(value)", "value", analysis.CodeReadBeforeInit},
	{"repeat_zero", "var value: Number\nrepeat 0 {value = 1}\nnumber(value)", "value", analysis.CodeReadBeforeInit},
	{"for_empty", "var value: Number\nfor item in [] {value = 1}\nnumber(value)", "value", analysis.CodeReadBeforeInit},
	{"menu_missing_path", "var value: Number\nmenu \"Pick\" { case \"A\": {value = 1} case \"B\": {number(2)} }\nnumber(value)", "value", analysis.CodeReadBeforeInit},
	{"and_skipped_assignment", "var value: Number\nlet gate = false && (if true {value = 1; yield true} else {value = 2; yield false})\nnumber(value)", "value", analysis.CodeReadBeforeInit},
	{"or_skipped_assignment", "var value: Number\nlet gate = true || (if true {value = 1; yield true} else {value = 2; yield false})\nnumber(value)", "value", analysis.CodeReadBeforeInit},
	{"narrow_outside", p15V02Optional + "if maybe != none {let good: Text = maybe}\nlet bad: Text = maybe", "maybe", analysis.CodeArgumentType},
	{"narrow_reassigned_none", p15V02Optional + "if maybe != none {maybe = none; let bad: Text = maybe}", "maybe", analysis.CodeArgumentType},
	{"narrow_reassigned_optional", p15V02Optional + "if maybe != none {maybe = [\"other\"][0]; let bad: Text = maybe}", "maybe", analysis.CodeArgumentType},
	{"narrow_branch_write", p15V02Optional + p15V02Flag + "if maybe != none {if flag {maybe = none}; let bad: Text = maybe}", "maybe", analysis.CodeArgumentType},
	{"narrow_loop_write", p15V02Optional + "if maybe != none {repeat 2 {let bad: Text = maybe; maybe = none}}", "maybe", analysis.CodeArgumentType},
	{"narrow_rhs_write", p15V02Optional + "if maybe != none && (if true {maybe = none; yield true} else {yield true}) {let bad: Text = maybe}", "maybe", analysis.CodeArgumentType},
}

var p15V02Valid = []p15V02Case{
	{"mutable", "var value = 1\nvalue = 2\nvalue += 3\nnumber(value)", "", ""},
	{"initialized_later", "var value: Number\nvalue = 1\nnumber(value)", "", ""},
	{"if_all_paths", p15V02Flag + "var value: Number\nif flag {value = 1} else {value = 2}\nnumber(value)", "", ""},
	{"if_else_if_all_paths", p15V02Flag + "var value: Number\nif flag {value = 1} else if flag {value = 2} else {value = 3}\nnumber(value)", "", ""},
	{"loop_initialized", "var value = 0\nrepeat 0 {value = 1}\nfor item in [] {value = 2}\nnumber(value)", "", ""},
	{"menu_all_paths", "var value: Number\nmenu \"Pick\" { case \"A\": {value = 1} case \"B\": {value = 2} }\nnumber(value)", "", ""},
	{"scoped_once", "var value: Number\nlet measured = measure(\"init\") {value = 1; value}\nnumber(value)", "", ""},
	{"shadow_initializer", "let value = 10\nif true {let value = value + 1; number(value)}\nnumber(value)", "", ""},
	{"narrow_then", p15V02Optional + "if maybe != none {let good: Text = maybe; text(good)}", "", ""},
	{"narrow_else", p15V02Optional + "if maybe == none {text(\"missing\")} else {let good: Text = maybe; text(good)}", "", ""},
	{"narrow_reversed", p15V02Optional + "if none != maybe {let good: Text = maybe; text(good)}", "", ""},
	{"narrow_not", p15V02Optional + "if !(maybe == none) {let good: Text = maybe; text(good)}", "", ""},
	{"narrow_and_rhs", p15V02Optional + "if maybe != none && (measure(\"guard\") {let good: Text = maybe; true}) {text(\"ok\")}", "", ""},
	{"narrow_or_rhs", p15V02Optional + "if maybe == none || (measure(\"guard\") {let good: Text = maybe; true}) {text(\"ok\")}", "", ""},
	{"narrow_known_reassignment", p15V02Optional + "if maybe != none {maybe = \"changed\"; let good: Text = maybe; text(good)}", "", ""},
	{"narrow_unwritten_loop", p15V02Optional + "if maybe != none {repeat 2 {let good: Text = maybe; text(good)}}", "", ""},
	{"narrow_loop_shadow", p15V02Optional + "if maybe != none {repeat 2 {let good: Text = maybe; var maybe: Text? = none; maybe = none; text(good)}}", "", ""},
	{"narrow_early_return", "function guarded(maybe: Text?) -> Text {if maybe == none {return \"missing\"}; return maybe}\ntext(guarded([\"ready\"][0]))", "", ""},
}

func p15V02TargetOffset(tc p15V02Case) int {
	expected := strings.LastIndex(tc.source, tc.target)
	// These reads precede a declaration/assignment with the same identifier.
	if tc.name == "forward_read" {
		expected = strings.Index(tc.source, "later")
	}
	if tc.name == "narrow_loop_write" {
		expected = strings.Index(tc.source, "Text = maybe") + len("Text = ")
	}
	return expected
}

// JSON null remains unsupported, and it must not narrow an Optional value.
// Both independent diagnostics are required at their own exact source spans.
func p15V02ExpectedDiagnostics(tc p15V02Case) []p15V02Case {
	if tc.name == "guard_json_null" {
		return []p15V02Case{
			{name: "unsupported_json_null", source: tc.source, target: "null", code: analysis.CodeNativeJSONNull},
			tc,
		}
	}
	return []p15V02Case{tc}
}

func p15V02Diagnostic(diagnostics []analysis.Diagnostic, tc p15V02Case) error {
	var errors []analysis.Diagnostic
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			errors = append(errors, d)
		}
	}
	expected := p15V02ExpectedDiagnostics(tc)
	if len(errors) != len(expected) {
		return fmt.Errorf("%s: want exactly %d independent errors, got %+v", tc.name, len(expected), errors)
	}
	for _, want := range expected {
		start := p15V02TargetOffset(want)
		matches := 0
		for _, actual := range errors {
			if actual.Code == want.code && actual.Span.Start.ByteOffset == start && actual.Span.End.ByteOffset == start+len(want.target) {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("%s: want one %s at [%d,%d), got %+v", tc.name, want.code, start, start+len(want.target), errors)
		}
	}
	return nil
}

func TestP15V02NullGuardDiagnosticOracleRejectsCorruption(t *testing.T) {
	var tc p15V02Case
	for _, item := range p15V02Invalid {
		if item.name == "guard_json_null" {
			tc = item
			break
		}
	}
	if tc.name == "" {
		t.Fatal("missing null guard fixture")
	}
	diagnostics := p15TypeDiagnostics(t, tc.source)
	if err := p15V02Diagnostic(diagnostics, tc); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing_null", "missing_unsafe_read", "wrong_null_code", "wrong_null_span", "unexpected_error"} {
		t.Run(fault, func(t *testing.T) {
			changed := append([]analysis.Diagnostic(nil), diagnostics...)
			switch fault {
			case "missing_null":
				changed = changed[1:]
			case "missing_unsafe_read":
				changed = changed[:1]
			case "wrong_null_code":
				changed[0].Code = analysis.CodeUnknownName
			case "wrong_null_span":
				changed[0].Span.Start.ByteOffset++
			case "unexpected_error":
				changed = append(changed, changed[0])
			}
			if p15V02Diagnostic(changed, tc) == nil {
				t.Fatal("accepted " + fault)
			}
		})
	}
}

func TestP15V02BindingFlowProduction(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	for _, tc := range p15V02Invalid {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15V02Diagnostic(diagnostics, tc); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-v02-invalid.cherri", tc.source)
			// CompileSource returns the first error; analysis/service retain the
			// complete independent diagnostic set checked above.
			firstCode := p15V02ExpectedDiagnostics(tc)[0].code
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), firstCode) {
				t.Fatalf("invalid binding emitted native output: bytes=%d err=%v", len(raw), err)
			}
			changed := append([]analysis.Diagnostic(nil), diagnostics...)
			for i := range changed {
				if changed[i].Severity == analysis.SeverityError {
					changed[i].Code = "WRONG_CODE"
				}
			}
			if p15V02Diagnostic(changed, tc) == nil {
				t.Fatal("wrong diagnostic mutation passed")
			}
		})
	}
	for _, tc := range p15V02Valid {
		t.Run(tc.name, func(t *testing.T) {
			for _, d := range p15TypeDiagnostics(t, tc.source) {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			if raw, err := CompileSourceToPlist("p15-v02-valid.cherri", tc.source); err != nil || len(raw) == 0 {
				t.Fatalf("valid binding failed actual compile: bytes=%d err=%v", len(raw), err)
			}
		})
	}
}

func TestP15V02BindingScopeService(t *testing.T) {
	svc := service.NewService(nil)
	uri := "file:///p15-v02-scopes.cherri"
	for _, tc := range []struct {
		name, marked, declaration string
		last                      bool
	}{
		{"initializer_outer", "let value=10\nif true {let value=va|lue+1;number(value)}\nnumber(value)", "let value", false},
		{"inside_local", "let value=10\nif true {let value=value+1;number(va|lue)}\nnumber(value)", "let value", true},
		{"outside_outer", "let value=10\nif true {let value=value+1;number(value)}\nnumber(va|lue)", "let value", false},
		{"declaration_self", "let value=10\nif true {let va|lue=value+1;number(value)}", "let value", true},
		{"scoped_initializer", "let value=10\nlet result=measure(\"scope\") {let value=va|lue+1;value}", "let value", false},
		{"scoped_local", "let value=10\nlet result=measure(\"scope\") {let value=value+1;va|lue}", "let value", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cursor := strings.Index(tc.marked, "|")
			text := strings.Replace(tc.marked, "|", "", 1)
			svc.OpenDocument(uri, 1, text)
			line, col := p15C04CompletionPosition(text, cursor)
			got := svc.Definition(uri, line, col)
			want := strings.Index(text, tc.declaration) + len("let ")
			if tc.last {
				want = strings.LastIndex(text, tc.declaration) + len("let ")
			}
			if got == nil || got.Start.ByteOffset != want || got.End.ByteOffset != want+len("value") {
				t.Fatalf("definition=%+v want [%d,%d)", got, want, want+len("value"))
			}
		})
	}
	source := "let value=10\nif true {let value=value+1;number(value)}\nnumber(value)"
	diagnostics := p15TypeDiagnostics(t, source)
	if len(diagnostics) != 1 || diagnostics[0].Severity != analysis.SeverityWarning ||
		diagnostics[0].Code != analysis.CodeShadowing ||
		diagnostics[0].Span.Start.ByteOffset != strings.LastIndex(source, "let value") {
		t.Fatalf("shadow warning lost or misattributed: %+v", diagnostics)
	}
}
