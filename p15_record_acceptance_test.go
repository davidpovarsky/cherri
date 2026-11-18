package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestP15T06RecordAnnotationsDoNotValidateExternalValues(t *testing.T) {
	for _, destination := range []string{
		"{name: Text}", "{name: Text}?", "List<{name: Text}>", "Map<Text, {name: Text}>", "Text | {name: Text}",
	} {
		for _, origin := range []struct{ name, prefix, value string }{
			{"content", "", "ShortcutInput"},
			{"unknown", "let raw=run(\"P15_EXTERNAL_RECORD\")\n", "raw"},
			{"native_dictionary", "", "ShortcutInput.dictionary"},
			{"category_guard", "", "require(ShortcutInput, type: \"dictionary\")"},
		} {
			t.Run(origin.name+"/"+destination, func(t *testing.T) {
				src := origin.prefix + "let person: " + destination + " = " + origin.value
				diagnostics := p15TypeDiagnostics(t, src)
				if err := p15ExactLiteralDiagnostic(diagnostics, src, origin.value, analysis.CodeArgumentType); err != nil {
					t.Fatal(err)
				}
				raw, err := CompileSourceToPlist("p15-t06.cherri", src)
				if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
					t.Fatalf("unvalidated record reached native emission: bytes=%d err=%v", len(raw), err)
				}
			})
		}
	}
	for _, tc := range []struct{ name, source, argument, code string }{
		{"assignment", "var person: {name: Text}\nperson = ShortcutInput", "ShortcutInput", analysis.CodeArgumentType},
		{"function_argument", "function take(person: {name: Text}) -> Void { comment(\"ok\") }\ntake(ShortcutInput)", "ShortcutInput", analysis.CodeArgumentType},
		{"function_named_argument", "function take(person: {name: Text}) -> Void { comment(\"ok\") }\ntake(person: ShortcutInput)", "ShortcutInput", analysis.CodeArgumentType},
		{"function_second_positional", "function take(prefix: Text, person: {name: Text}) -> Void { comment(prefix) }\ntake(\"ok\", ShortcutInput)", "ShortcutInput", analysis.CodeArgumentType},
		{"function_public_parameter_name", "function take(_person: {name: Text}) -> Void { comment(\"ok\") }\ntake(person: ShortcutInput)", "ShortcutInput", analysis.CodeArgumentType},
		{"function_optional_record", "function take(person: {name: Text}?) -> Void { comment(\"ok\") }\ntake(ShortcutInput)", "ShortcutInput", analysis.CodeArgumentType},
		{"function_list_record", "function take(people: List<{name: Text}>) -> Void { comment(\"ok\") }\ntake(ShortcutInput)", "ShortcutInput", analysis.CodeArgumentType},
		{"function_default", "function take(person: {name: Text} = ShortcutInput) -> Void { comment(\"ok\") }", "ShortcutInput", analysis.CodeArgumentType},
		{"external_return", "function load() -> {name: Text} { return ShortcutInput }", "return ShortcutInput", analysis.CodeReturnType},
		{"json_return", "function load(value: JsonValue) -> {name: Text} { return value }", "return value", analysis.CodeReturnType},
		{"unknown_record_field", "function narrow(value: {name: Unknown}) -> {name: Text} { return value }", "return value", analysis.CodeReturnType},
		{"content_record_field", "function narrow(value: {name: AnyContent}) -> {name: Text} { return value }", "return value", analysis.CodeReturnType},
		{"optional_field_presence", "function narrow(value: {name?: Text}) -> {name: Text} { return value }", "return value", analysis.CodeReturnType},
		{"nested_list_field", "function narrow(value: {names: List<Unknown>}) -> {names: List<Text>} { return value }", "return value", analysis.CodeReturnType},
		{"nested_map_field", "function narrow(value: {names: Map<Text, AnyContent>}) -> {names: Map<Text, Text>} { return value }", "return value", analysis.CodeReturnType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := p15TypeDiagnostics(t, tc.source)
			if err := p15ExactLiteralDiagnostic(diags, tc.source, tc.argument, tc.code); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-t06.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("record proof bypass: bytes=%d err=%v", len(raw), err)
			}
			var mutated = append([]analysis.Diagnostic(nil), diags...)
			for i := range mutated {
				if mutated[i].Code == tc.code {
					mutated[i].Code = analysis.CodeUnknownName
				}
			}
			if p15ExactLiteralDiagnostic(mutated, tc.source, tc.argument, tc.code) == nil {
				t.Fatal("unrelated diagnostic mutation passed")
			}
		})
	}
	for _, src := range []string{
		"let raw=run(\"P15_EXTERNAL_RECORD\")\nshow(raw)\nshow(ShortcutInput)",
		"function identity(person: {name: Text, age?: Number}) -> {name: Text} { return person }",
		"function opaque(person: {name: Unknown}) -> {name: AnyContent} { return person }",
		"function optional(person: {name: Text}) -> {name?: Text} { return person }",
		"function nullable(person: {name: Text}) -> {name: Text}? { return person }",
		"function either(person: {name: Text}) -> Text | {name: Text} { return person }",
		"function same(person: {names: List<Text>}) -> {names: List<Text>} { return person }",
		"function fields(person: {name: Text}) -> Text { return person.name }",
		"function take(person: {name: Text}) -> Void { comment(\"ok\") }\nfunction forward(person: {name: Text}) -> Void { take(person) }",
		"function take(prefix: Text, _person: {name: Text}) -> Void { comment(prefix) }\nfunction forward(person: {name: Text}) -> Void { take(\"ok\", person: person) }",
		"function plain(value: AnyContent) -> Void { show(value) }\nplain(ShortcutInput)",
		"let payload: JsonValue = ShortcutInput\nshow(payload)",
		"let n: Number=number(3)\nlet s: Text=text(\"ok\")\nshow(n)\nshow(s)",
	} {
		for _, d := range p15TypeDiagnostics(t, src) {
			if d.Severity == analysis.SeverityError {
				t.Fatal(d)
			}
		}
		compileDebugFixture(t, src, false)
	}
	// Removing the record annotation must preserve the actual external type,
	// not invent a record guarantee or inject runtime validation actions.
	src := "let raw=run(\"P15_EXTERNAL_RECORD\")\nlet content=ShortcutInput"
	program, errors := p15ParseProgram(src)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	for name, want := range map[string]types.Type{"raw": types.AnyContent, "content": types.AnyContent} {
		got := analyzer.ExprTypes()[p15BindingValue(t, program, name)]
		if got == nil || !(got.Kind() == types.KindUnknown || got.Kind() == types.KindAnyContent) {
			t.Fatalf("%s invented external type %v; expected opaque %v", name, got, want)
		}
	}
}

func TestP15T06RecordProofTypeRelations(t *testing.T) {
	record := types.NewRecord([]types.RecordField{{Name: "name", Type: types.Text}})
	destinations := []types.Type{record, types.NewOptional(record), types.NewList(record), types.NewMap(types.Text, record), types.NewUnion(types.Text, record)}
	for _, source := range []types.Type{types.Unknown, types.AnyContent} {
		for _, destination := range destinations {
			if source.AssignableTo(destination) {
				t.Errorf("unverified %s establishes %s", source, destination)
			}
		}
		for _, destination := range []types.Type{types.Unknown, types.AnyContent, types.Text, types.Number, types.NewList(types.AnyContent)} {
			if !source.AssignableTo(destination) {
				t.Errorf("unrelated content compatibility changed: %s -> %s", source, destination)
			}
		}
	}
	for _, source := range []types.Type{types.Unknown, types.AnyContent, types.NewList(types.Unknown), types.NewMap(types.Text, types.AnyContent)} {
		target := types.Type(types.Text)
		switch source.Kind() {
		case types.KindList:
			target = types.NewList(types.Text)
		case types.KindMap:
			target = types.NewMap(types.Text, types.Text)
		}
		from := types.NewRecord([]types.RecordField{{Name: "field", Type: source}})
		to := types.NewRecord([]types.RecordField{{Name: "field", Type: target}})
		if from.AssignableTo(to) {
			t.Error(fmt.Sprintf("unverified record field narrowed: %s -> %s", from, to))
		}
	}
	if !record.AssignableTo(record) {
		t.Fatal("verified record identity rejected")
	}
}
