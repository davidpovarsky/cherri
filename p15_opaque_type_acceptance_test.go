package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func p15T04NegativeCases() []struct{ name, source, span, code string } {
	return []struct{ name, source, span, code string }{
		{"any_annotation", "let claimed: Number = Clipboard", "Clipboard", analysis.CodeArgumentType},
		{"unvalidated_json_annotation", "let payload: JsonValue = ShortcutInput", "ShortcutInput", analysis.CodeArgumentType},
		{"unknown_annotation", "let claimed: Text = chooseFromList(Clipboard)", "chooseFromList(Clipboard)", analysis.CodeArgumentType},
		{"opaque_void_annotation", "let claimed: Void = Clipboard", "Clipboard", analysis.CodeArgumentType},
		{"function_argument", "function accept(value: Number) -> Number { return value }\naccept(Clipboard)", "Clipboard", analysis.CodeArgumentType},
		{"function_default", "function accept(value: Number = Clipboard) -> Number { return value }", "Clipboard", analysis.CodeArgumentType},
		{"function_return", "function make() -> Number { return Clipboard }", "return Clipboard", analysis.CodeReturnType},
		{"mutable_assignment", "var value: Number = 1\nvalue = Clipboard", "Clipboard", analysis.CodeArgumentType},
		{"list_annotation", "let values: List<Number> = [Clipboard]", "[Clipboard]", analysis.CodeArgumentType},
		{"map_annotation", "let values: Map<Text, Number> = {\"value\": Clipboard}", "{\"value\": Clipboard}", analysis.CodeArgumentType},
		{"unknown_arithmetic", "let opaque=chooseFromList(Clipboard)\nlet claimed=opaque * 2", "opaque", analysis.CodeArgumentType},
		{"any_arithmetic", "let claimed=Clipboard + 2", "Clipboard", analysis.CodeArgumentType},
		{"void_binding", "let claimed=debugShowLogs()", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_content_view", "let claimed=debugShowLogs().number", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_arithmetic", "let claimed=debugShowLogs() * 2", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_list", "let claimed=[debugShowLogs()]", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_map", "let claimed={\"value\":debugShowLogs()}", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_interpolation", "let claimed=f\"{debugShowLogs()}\"", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_index", "let claimed=debugShowLogs()[0]", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_argument", "text(debugShowLogs())", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_equality", "let claimed=debugShowLogs()==0", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_negation", "let claimed=!debugShowLogs()", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_index_operand", "let claimed=Clipboard[debugShowLogs()]", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_contains", "let claimed=debugShowLogs() contains 1", "debugShowLogs()", analysis.CodeArgumentType},
		{"void_logical", "let claimed=true && debugShowLogs()", "debugShowLogs()", analysis.CodeArgumentType},
		{"opaque_list_is_not_empty_proof", "let opaque=[]\nlet claimed: List<Number> = opaque", "opaque", analysis.CodeArgumentType},
		{"opaque_order", "let claimed=Clipboard<2", "Clipboard", analysis.CodeArgumentType},
	}
}

func TestP15T04OpaqueTypeProduction(t *testing.T) {
	if types.Unknown.Equals(types.AnyContent) || types.Unknown.Equals(types.Void) || types.AnyContent.Equals(types.Void) {
		t.Fatal("semantic categories collapsed")
	}
	source := "let opaque=chooseFromList(Clipboard)\nlet content=Clipboard\nlet concrete=number(Clipboard)\ndebugShowLogs()"
	program, analyzer := p15T08Analyzer(t, source)
	for _, tc := range []struct {
		name string
		want types.Type
	}{{"opaque", types.Unknown}, {"content", types.AnyContent}, {"concrete", types.Number}} {
		actual := analyzer.ExprTypes()[p15BindingValue(t, program, tc.name)]
		if actual == nil || !actual.Equals(tc.want) {
			t.Fatalf("%s: want %s got %v", tc.name, tc.want, actual)
		}
	}
	compileDebugFixture(t, source, false)
	for _, tc := range p15T04NegativeCases() {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.span, tc.code); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-t04-negative.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("unverified source claim emitted bytes=%d err=%v", len(raw), err)
			}
			broken := append([]analysis.Diagnostic(nil), diagnostics...)
			for i := range broken {
				if broken[i].Code == tc.code {
					broken[i].Span.Start.ByteOffset++
				}
			}
			if p15ExactLiteralDiagnostic(broken, tc.source, tc.span, tc.code) == nil {
				t.Fatal("wrong-span mutation accepted")
			}
		})
	}
	for _, source := range []string{
		"let content:AnyContent=Clipboard\nshow(content)",
		"let values: List<Number> = []\nshow(values)",
		"let values: Map<Text, Number> = {}\nshow(values)",
		"let value: AnyContent? = Clipboard\nshow(value)",
		"let value: Text? = none\nshow(value)",
		"let opaque:Unknown=chooseFromList(Clipboard)\nshow(opaque)",
		"let concrete:Number=number(Clipboard)\nwait(concrete)",
		"function content(value:AnyContent)->AnyContent {return value}\nshow(content(Clipboard))",
		"function done()->Void {debugShowLogs()}\ndone()",
	} {
		p15T08Analyzer(t, source)
		compileDebugFixture(t, source, false)
	}
}

func TestP15T04OpaqueTypeService(t *testing.T) {
	svc := service.NewService(nil)
	uri := "file:///p15-t04-service.cherri"
	source := "chooseFromList(Clipboard)\ndebugShowLogs()"
	svc.OpenDocument(uri, 1, source)
	diagnostics, _ := svc.Analyze(uri)
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	for i, want := range []string{"Unknown", "Void"} {
		line := strings.Split(source, "\n")[i]
		hover := svc.Hover(uri, i+1, 2)
		if hover == nil || !strings.Contains(hover.Contents, "**Returns:** "+string(rune(96))+want+string(rune(96))) {
			t.Fatalf("line%d result knowledge: %+v", i+1, hover)
		}
		signature := svc.SignatureHelp(uri, i+1, len([]rune(line)))
		if signature == nil || !strings.HasSuffix(signature.Label, " -> "+want) {
			t.Fatalf("line%d signature: %+v", i+1, signature)
		}
	}
	for i, tc := range p15T04NegativeCases() {
		svc.OpenDocument(uri, 2+i, tc.source)
		diagnostics, _ = svc.Analyze(uri)
		if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.span, tc.code); err != nil {
			t.Errorf("%s service: %v", tc.name, err)
		}
		category := map[string]string{"any_annotation": "AnyContent", "unknown_annotation": "Unknown", "void_binding": "Void"}[tc.name]
		if category != "" {
			found := false
			for _, d := range diagnostics {
				if d.Code == tc.code && strings.Contains(d.Message, category) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s service lost precise opaque category %s", tc.name, category)
			}
		}
	}
	corrected := "let concrete:Number=number(Clipboard)\nwait(concrete)"
	svc.OpenDocument(uri, 100, corrected)
	diagnostics, _ = svc.Analyze(uri)
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			t.Fatal("stale opaque diagnostic after explicit conversion", d)
		}
	}
	hover := svc.Hover(uri, 1, strings.Index(corrected, "number(")+1)
	if hover == nil || !strings.Contains(hover.Contents, fmt.Sprintf("**Returns:** %cNumber%c", 96, 96)) {
		t.Fatal("explicit producer lost concrete type")
	}
}
