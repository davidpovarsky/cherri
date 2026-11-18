package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

func p15T04CustomDeclaration(name, result string) string {
	arrow := ""
	if result != "" {
		arrow = " -> " + result
	}
	return fmt.Sprintf("action %s(value: Text)%s {\nidentifier: %q\nbindings: {value: {key: \"Input\", codec: \"text\"}}\n}\n", name, arrow, "com.example.p15.custom."+name)
}

func p15T04DeclaredCustomSources() []struct{ name, result, source string } {
	var cases []struct{ name, result, source string }
	for _, tc := range []struct{ suffix, result string }{
		{"Text", "Text"}, {"Number", "Number"}, {"Union", "Text | Number"},
		{"List", "List<Text>"}, {"Map", "Map<Text, Number>"},
	} {
		name := "p15T04Declared" + tc.suffix
		source := p15T04CustomDeclaration(name, tc.result) +
			"let result: " + tc.result + " = " + name + "(\"value\")\nshow(result)"
		cases = append(cases, struct{ name, result, source string }{name, tc.result, source})
	}
	return cases
}

func p15T04CheckCustomDeclaredResults(t *testing.T) {
	t.Helper()
	for _, tc := range p15T04DeclaredCustomSources() {
		t.Run(tc.name, func(t *testing.T) {
			program, analyzer := p15T08Analyzer(t, tc.source)
			actual := analyzer.ExprTypes()[p15BindingValue(t, program, "result")]
			if actual == nil || actual.Name() != tc.result {
				t.Fatalf("declared result contract lost: want %s, got %v", tc.result, actual)
			}
			// This is a declaration/serialized-reference check for a synthetic
			// custom identifier, not proof of an installed native action.
			actions := compileDebugFixture(t, tc.source, false)
			if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != "com.example.p15.custom."+tc.name {
				t.Fatalf("custom producer changed or duplicated: %#v", actions)
			}
			producer, _ := actions[0].WFWorkflowActionParameters["UUID"].(string)
			outputs, variables := p15V02NativeReferences(actions[1].WFWorkflowActionParameters)
			if producer == "" || len(outputs) != 1 || outputs[0] != producer || len(variables) != 0 {
				t.Fatalf("declared result lost actual producer identity: producer=%q outputs=%v variables=%v", producer, outputs, variables)
			}
		})
	}
	for _, tc := range p15T04UnknownCustomSources() {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15ExactLiteralDiagnostic(p15TypeDiagnostics(t, tc.source), tc.source, tc.span, analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-custom-output-negative.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
				t.Fatalf("unproved custom output emitted bytes=%d err=%v", len(raw), err)
			}
		})
	}
	compileDebugFixture(t, p15T04CustomDeclaration("p15T04DeclaredVoidStatement", "Void")+"p15T04DeclaredVoidStatement(\"value\")", false)
}

func p15T04UnknownCustomSources() []struct{ name, source, span string } {
	var cases []struct{ name, source, span string }
	for _, tc := range []struct{ suffix, result string }{
		{"Undeclared", ""}, {"Unknown", "Unknown"}, {"Any", "AnyContent"}, {"Void", "Void"},
	} {
		name := "p15T04Opaque" + tc.suffix
		call := name + "(\"value\")"
		binding := "let result: Text = "
		if tc.result == "Void" {
			binding = "let result = "
		}
		cases = append(cases, struct{ name, source, span string }{
			name, p15T04CustomDeclaration(name, tc.result) + binding + call, call,
		})
	}
	return cases
}

func p15T04CheckCustomDeclaredResultsService(t *testing.T) {
	t.Helper()
	for _, tc := range p15T04DeclaredCustomSources() {
		t.Run(tc.name, func(t *testing.T) {
			svc := service.NewService(nil)
			uri := "file:///p15-custom-output-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			diagnostics, _ := svc.Analyze(uri)
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			prefix := tc.source[:strings.LastIndex(tc.source, tc.name+"(")]
			line := strings.Count(prefix, "\n") + 1
			column := len([]rune(prefix[strings.LastIndex(prefix, "\n")+1:])) + 2
			hover := svc.Hover(uri, line, column)
			if hover == nil || !strings.Contains(hover.Contents, fmt.Sprintf("**Returns:** %c%s%c", 96, tc.result, 96)) {
				t.Fatalf("service lost declared custom result: %+v", hover)
			}
		})
	}
	for _, tc := range p15T04UnknownCustomSources() {
		svc := service.NewService(nil)
		uri := "file:///p15-custom-output-negative-" + tc.name + ".cherri"
		svc.OpenDocument(uri, 1, tc.source)
		diagnostics, _ := svc.Analyze(uri)
		if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.span, analysis.CodeArgumentType); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}
