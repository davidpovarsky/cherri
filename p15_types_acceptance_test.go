package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

func p15TypeDiagnostics(t *testing.T, src string) []analysis.Diagnostic {
	t.Helper()
	program, errors := p15ParseProgram(src)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	return analyzer.Diagnostics()
}

func p15ExactLiteralDiagnostic(diagnostics []analysis.Diagnostic, src, argument, code string) error {
	start := strings.LastIndex(src, argument)
	if start < 0 {
		return fmt.Errorf("test argument absent")
	}
	var errors []analysis.Diagnostic
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError && d.Code == code {
			errors = append(errors, d)
		}
	}
	if len(errors) != 1 || errors[0].Code != code || errors[0].Span.Start.ByteOffset != start || errors[0].Span.End.ByteOffset != start+len(argument) {
		return fmt.Errorf("want one %s at [%d,%d), got %#v", code, start, start+len(argument), errors)
	}
	return nil
}

func TestP15T02LiteralRequiredProduction(t *testing.T) {
	reg := schema.DefaultRegistry()
	action, ok := reg.LookupAction("setBackgroundSound")
	if !ok {
		t.Fatal("missing production action")
	}
	param, ok := action.PrimaryParameter()
	if !ok || param.EvidenceStatus != schema.EvidenceConfirmed || len(param.AcceptedForms) != 1 || param.AcceptedForms[0] != "literal" {
		t.Fatal("fixture must use confirmed literal-only native parameter")
	}
	for _, tc := range []struct{ name, source, argument string }{
		{"runtime_let", "let mode = text(\"Ocean\")\nsetBackgroundSound(mode)", "mode"},
		{"immutable_literal_binding", "let mode = \"Ocean\"\nsetBackgroundSound(mode)", "mode"},
		{"immutable_folded_binding", "let mode = \"Oc\" + \"ean\"\nsetBackgroundSound(mode)", "mode"},
		{"mutable_binding", "var mode = \"Ocean\"\nsetBackgroundSound(mode)", "mode"},
		{"direct_action", "setBackgroundSound(text(\"Ocean\"))", "text(\"Ocean\")"},
		{"named_action", "setBackgroundSound(sound: text(\"Ocean\"))", "text(\"Ocean\")"},
		{"dynamic_concat", "setBackgroundSound(\"Oc\" + text(\"ean\"))", "\"Oc\" + text(\"ean\")"},
		{"dynamic_interpolation", "let mode = text(\"Ocean\")\nsetBackgroundSound(f\"{mode}\")", "f\"{mode}\""},
		{"runtime_index", "setBackgroundSound([\"Ocean\"][0])", "[\"Ocean\"][0]"},
		{"runtime_if", "setBackgroundSound(if true { yield \"Ocean\" } else { yield \"Rain\" })", "if true { yield \"Ocean\" } else { yield \"Rain\" }"},
		{"declared_literal_title", "makeVCard(title: text(\"A\"), subtitle: \"B\")", "text(\"A\")"},
		{"macro_type", "require(text(\"ok\"), type: text(\"text\"))", "text(\"text\")"},
		{"macro_nonempty", "var flag = true\nrequire(text(\"ok\"), nonEmpty: flag)", "flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.argument, analysis.CodeLiteralRequired); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-t02.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeLiteralRequired) {
				t.Fatalf("production compile did not fail before emission: bytes=%d err=%v", len(raw), err)
			}
			// A generic error or a diagnostic attached to the wrong argument is
			// insufficient to satisfy the stable literal-required contract.
			var broken = append([]analysis.Diagnostic(nil), diagnostics...)
			for i := range broken {
				if broken[i].Code == analysis.CodeLiteralRequired {
					broken[i].Code = analysis.CodeArgumentType
				}
			}
			if p15ExactLiteralDiagnostic(broken, tc.source, tc.argument, analysis.CodeLiteralRequired) == nil {
				t.Fatal("wrong-code mutation passed")
			}
			broken = append([]analysis.Diagnostic(nil), diagnostics...)
			for i := range broken {
				if broken[i].Code == analysis.CodeLiteralRequired {
					broken[i].Span.Start.ByteOffset++
				}
			}
			if p15ExactLiteralDiagnostic(broken, tc.source, tc.argument, analysis.CodeLiteralRequired) == nil {
				t.Fatal("wrong-span mutation passed")
			}
		})
	}
	for _, expression := range []string{"\"Ocean\"", "r\"Ocean\"", "(\"Ocean\")", "\"Oc\" + \"ean\"", "(\"O\" + \"c\") + \"ean\"", "f\"Oc{\"ean\"}\""} {
		for _, named := range []bool{false, true} {
			prefix := ""
			if named {
				prefix = "sound: "
			}
			src := "setBackgroundSound(" + prefix + expression + ")"
			t.Run(fmt.Sprintf("literal/%t/%s", named, expression), func(t *testing.T) {
				for _, d := range p15TypeDiagnostics(t, src) {
					if d.Severity == analysis.SeverityError {
						t.Fatal(d)
					}
				}
				actions := compileDebugFixture(t, src, false)
				if err := p15M02MainVoidDebugError(actions); err != nil {
					t.Fatal(err)
				}
				if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != "com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent" ||
					actions[0].WFWorkflowActionParameters["backgroundSound"] != "Ocean" {
					t.Fatalf("eligible literal was evaluated dynamically or encoded incorrectly: %#v", actions)
				}
			})
		}
	}
	for _, src := range []string{
		"makeVCard(title: \"A\" + \"B\", subtitle: \"C\")",
		"require(text(\"ok\"), type: \"te\" + \"xt\", nonEmpty: !false)",
	} {
		for _, d := range p15TypeDiagnostics(t, src) {
			if d.Severity == analysis.SeverityError {
				t.Fatal(d)
			}
		}
		compileDebugFixture(t, src, false)
	}
	// Pure constants still pass through enum validation; purity is not a
	// permission to bypass the native parameter's allowed values.
	invalid := "\"Not\" + \"ASound\""
	src := "setBackgroundSound(" + invalid + ")"
	if err := p15ExactLiteralDiagnostic(p15TypeDiagnostics(t, src), src, invalid, analysis.CodeEnumMember); err != nil {
		t.Fatal(err)
	}
	// Dictionary literals explicitly support dynamic leaf values in the HTTP
	// native codec. Their outer shape is a separate established restriction.
	src = "let value=text(\"dynamic\")\njsonRequest(\"https://example.invalid\", body: {\"value\": value})"
	for _, d := range p15TypeDiagnostics(t, src) {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	compileDebugFixture(t, src, false)
}
