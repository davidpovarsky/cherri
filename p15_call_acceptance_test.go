package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

const p15C02Function = "function c02Choose(a: Number, b: Number = 20) -> Number { return a }\n"
const p15C02Alias = "function c02Alias(_value: Number) -> Number { return _value }\n"
const p15C02Custom = "action c02Native(a: Number, b: Text) -> Text {\nidentifier: \"com.example.c02.binding\"\nprimary: \"b\"\nbindings: { a: {key: \"A\", codec: \"number\"}, b: {key: \"B\", codec: \"text\"} }\n}\n"

type p15C02Case struct{ name, prefix, call, argument, code, parameter string }

var p15C02Cases = []p15C02Case{
	{"native_unknown", "", "number(number: 1, mystery: 2)", "mystery: 2", analysis.CodeUnknownArgument, "mystery"},
	{"native_unknown_unicode", "", "text(\"safe\", mystery: \"\U0001f30d\u05e9\")", "mystery: \"\U0001f30d\u05e9\"", analysis.CodeUnknownArgument, "mystery"},
	{"native_duplicate_named", "", "resizeImage(ShortcutInput, width: 100, width: 200)", "width: 200", analysis.CodeDuplicateArgument, "width"},
	{"native_duplicate_primary", "", "number(1, number: 2)", "number: 2", analysis.CodeDuplicateArgument, "number"},
	{"native_missing_primary", "", "number()", "number()", analysis.CodeMissingArgument, "number"},
	{"native_missing_named", "", "resizeImage(ShortcutInput)", "resizeImage(ShortcutInput)", analysis.CodeMissingArgument, "width"},
	{"native_missing_reordered", "", "resizeImage(height: 600, width: 800)", "resizeImage(height: 600, width: 800)", analysis.CodeMissingArgument, "image"},
	{"native_extra_positional", "", "number(1, 2)", "2", analysis.CodePositionalArg, ""},
	{"native_extra_after_named", "", "number(number: 1, 2)", "2", analysis.CodePositionalArg, ""},
	{"native_zero_arity", "", "isCharging(1)", "1", analysis.CodePositionalArg, ""},
	{"function_missing", p15C02Function, "c02Choose()", "c02Choose()", analysis.CodeMissingArgument, "a"},
	{"function_duplicate_named", p15C02Function, "c02Choose(a: 1, a: 2)", "a: 2", analysis.CodeDuplicateArgument, "a"},
	{"function_duplicate_primary", p15C02Function, "c02Choose(1, a: 2)", "a: 2", analysis.CodeDuplicateArgument, "a"},
	{"function_unknown", p15C02Function, "c02Choose(1, mystery: 2)", "mystery: 2", analysis.CodeUnknownArgument, "mystery"},
	{"function_extra_positional", p15C02Function, "c02Choose(1, 2, 3)", "3", analysis.CodePositionalArg, ""},
	{"function_extra_with_reserved_name", p15C02Function, "c02Choose(1, 2, b: 3)", "2", analysis.CodePositionalArg, ""},
	{"function_alias_duplicate", p15C02Alias, "c02Alias(value: 1, _value: 2)", "_value: 2", analysis.CodeDuplicateArgument, "_value"},
	{"function_ambiguous_alias_reuse", "function c02Ambiguous(x: Number, _x: Number) -> Number { return x }\n", "c02Ambiguous(x: 1)", "x: 1", analysis.CodeDuplicateArgument, "x"},
	{"function_zero_arity", "function c02Empty() -> Void { comment(\"ok\") }\n", "c02Empty(1)", "1", analysis.CodePositionalArg, ""},
	{"function_zero_unknown", "function c02Empty() -> Void { comment(\"ok\") }\n", "c02Empty(mystery: 1)", "mystery: 1", analysis.CodeUnknownArgument, "mystery"},
	{"function_required_optional_value", "function c02Nullable(value: Number?) -> Void { comment(\"ok\") }\n", "c02Nullable()", "c02Nullable()", analysis.CodeMissingArgument, "value"},
	{"custom_primary_missing_other", p15C02Custom, "c02Native(\"selected\")", "c02Native(\"selected\")", analysis.CodeMissingArgument, "a"},
	{"custom_primary_duplicate", p15C02Custom, "c02Native(\"selected\", a: 11, b: \"again\")", "b: \"again\"", analysis.CodeDuplicateArgument, "b"},
	{"custom_unknown", p15C02Custom, "c02Native(\"selected\", a: 11, mystery: 2)", "mystery: 2", analysis.CodeUnknownArgument, "mystery"},
}

func p15C02Source(tc p15C02Case) string {
	// Real non-BMP/RTL codepoints are constructed from Go escapes, avoiding
	// platform shell encoding changes and exercising actual range units.
	return "// \U0001f30d \u05e9\u05dc\u05d5\u05dd\n" + tc.prefix + tc.call
}

func p15C02Diagnostic(diags []analysis.Diagnostic, src string, tc p15C02Case) error {
	var errors []analysis.Diagnostic
	for _, d := range diags {
		if d.Severity == analysis.SeverityError {
			errors = append(errors, d)
		}
	}
	if len(errors) != 1 {
		return fmt.Errorf("want exactly one error: %+v", errors)
	}
	if err := p15ExactLiteralDiagnostic(errors, src, tc.argument, tc.code); err != nil {
		return err
	}
	if tc.parameter != "" && !strings.Contains(errors[0].Message, fmt.Sprintf("%q", tc.parameter)) {
		return fmt.Errorf("diagnostic named wrong parameter: %s", errors[0].Message)
	}
	offset := strings.LastIndex(src, tc.argument)
	if errors[0].Span.Start.UTF16Offset != len(utf16.Encode([]rune(src[:offset]))) ||
		errors[0].Span.End.UTF16Offset != len(utf16.Encode([]rune(src[:offset+len(tc.argument)]))) {
		return fmt.Errorf("wrong UTF-16 range: %+v", errors[0].Span)
	}
	return nil
}

func TestP15C02ArgumentDiagnosticsProduction(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	for _, tc := range p15C02Cases {
		t.Run(tc.name, func(t *testing.T) {
			src := p15C02Source(tc)
			diagnostics := p15TypeDiagnostics(t, src)
			if err := p15C02Diagnostic(diagnostics, src, tc); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-c02.cherri", src)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("invalid argument reached native emission: bytes=%d err=%v", len(raw), err)
			}
			for _, mutation := range []string{"wrong_code", "wrong_span", "warning"} {
				broken := append([]analysis.Diagnostic(nil), diagnostics...)
				for i := range broken {
					if broken[i].Code != tc.code {
						continue
					}
					switch mutation {
					case "wrong_code":
						broken[i].Code = analysis.CodeArgumentType
					case "wrong_span":
						broken[i].Span.Start.ByteOffset++
					case "warning":
						broken[i].Severity = analysis.SeverityWarning
					}
				}
				if p15C02Diagnostic(broken, src, tc) == nil {
					t.Fatalf("%s mutation passed", mutation)
				}
			}
		})
	}
}

func p15C02CustomWire(actions []ShortcutAction) error {
	found := 0
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier != "com.example.c02.binding" {
			continue
		}
		found++
		if p15Int(action.WFWorkflowActionParameters["A"]) != 11 || action.WFWorkflowActionParameters["B"] != "selected" {
			return fmt.Errorf("explicit non-first primary bound wrong wire parameters: %+v", action.WFWorkflowActionParameters)
		}
	}
	if found != 1 {
		return fmt.Errorf("custom source did not emit exactly its declared action: %d", found)
	}
	return nil
}

func TestP15C02AcceptedBindingsProduction(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	for _, call := range []string{
		"c02Native(\"selected\", a: 11)",
		"c02Native(b: \"selected\", a: 11)",
		"c02Native(\"selected\", 11)",
	} {
		src := p15C02Custom + call
		for _, d := range p15TypeDiagnostics(t, src) {
			if d.Severity == analysis.SeverityError {
				t.Fatal(d)
			}
		}
		actions := compileDebugFixture(t, src, false)
		if err := p15C02CustomWire(actions); err != nil {
			t.Fatal(err)
		}
		for i := range actions {
			if actions[i].WFWorkflowActionIdentifier == "com.example.c02.binding" {
				actions[i].WFWorkflowActionParameters["A"] = 22
			}
		}
		if p15C02CustomWire(actions) == nil {
			t.Fatal("wrong parameter value mutation passed")
		}
	}
	for _, src := range []string{
		"number(0)", "number(number: 0)", "text(\"\")", "text(text: \"\")",
		"resizeImage(ShortcutInput, width: 800)", "resizeImage(ShortcutInput, height: 600, width: 800)",
		"resizeImage(ShortcutInput, 800, 600)",
		p15C02Function + "c02Choose(0)",
		p15C02Function + "c02Choose(b: 3, a: 1)",
		p15C02Function + "c02Choose(1, 2)",
		p15C02Alias + "c02Alias(value: 1)",
		p15C02Alias + "c02Alias(_value: 1)",
		"function c02Names(x: Number, _x: Number) -> Number { return x }\nc02Names(1, _x: 2)",
		"function c02Names(x: Number, _x: Number) -> Number { return x }\nc02Names(_x: 2, x: 1)",
		"function c02Optional(value?: Number) -> Void { comment(\"ok\") }\nc02Optional()",
		"function c02Nullable(value: Number?) -> Void { comment(\"ok\") }\nc02Nullable(none)",
	} {
		for _, d := range p15TypeDiagnostics(t, src) {
			if d.Severity == analysis.SeverityError {
				t.Fatal(d)
			}
		}
		actions := compileDebugFixture(t, src, false)
		if len(actions) == 0 {
			t.Fatal("accepted call emitted no native action")
		}
		if strings.Contains(src, "function ") {
			found := false
			for _, action := range actions {
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.runworkflow" {
					if requireOutputUUID(action.WFWorkflowActionParameters["WFInput"]) == "" {
						t.Fatal("function call lost envelope reference")
					}
					found = true
				}
			}
			if !found {
				t.Fatal("accepted user-function call was omitted")
			}
		}
	}
}

func p15C02ExpectedRange(src, argument string) protocol.Range {
	start := strings.LastIndex(src, argument)
	position := func(offset int) protocol.Position {
		prefix := src[:offset]
		line := prefix[strings.LastIndex(prefix, "\n")+1:]
		return protocol.Position{Line: strings.Count(prefix, "\n"), Character: len(utf16.Encode([]rune(line)))}
	}
	return protocol.Range{Start: position(start), End: position(start + len(argument))}
}

func TestP15C02ArgumentServiceAndCLI(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	bin := p15AcceptanceCLI(t)
	dir := t.TempDir()
	svc := service.NewService(nil)
	uri := "file:///p15-c02-editor.cherri"
	svc.OpenDocument(uri, 1, "number(0)")
	for index, tc := range p15C02Cases {
		t.Run(tc.name, func(t *testing.T) {
			src := p15C02Source(tc)
			svc.ChangeDocument(uri, index*2+2, src)
			diagnostics, _ := svc.Analyze(uri)
			if err := p15C02Diagnostic(diagnostics, src, tc); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, tc.name+".cherri")
			if err := os.WriteFile(path, []byte(src), 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := p15RunCLI(t, bin, dir, nil, "check", path, "--json")
			if err == nil {
				t.Fatalf("CLI check accepted bad arguments: %s", out)
			}
			p15DecodeOneJSON(t, out)
			var result protocol.AnalyzeResponse
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal(err)
			}
			if result.Valid || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.code ||
				result.Diagnostics[0].Severity != "ERROR" || result.Diagnostics[0].Range != p15C02ExpectedRange(src, tc.argument) {
				t.Fatalf("CLI range/code/validity mismatch: %+v stderr=%s", result, stderr)
			}
			svc.ChangeDocument(uri, index*2+3, "number(0)")
			diagnostics, _ = svc.Analyze(uri)
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					t.Fatal("corrected editor retained argument error", d)
				}
			}
		})
	}
	path := filepath.Join(dir, "corrected.cherri")
	if err := os.WriteFile(path, []byte("number(0)"), 0600); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := p15RunCLI(t, bin, dir, nil, "check", path, "--json")
	if err != nil {
		t.Fatalf("corrected CLI failed: %v %s", err, stderr)
	}
	var corrected protocol.AnalyzeResponse
	if err := json.Unmarshal(out, &corrected); err != nil || !corrected.Valid {
		t.Fatalf("corrected CLI response: %s %v", out, err)
	}
}
