package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15OptionalForwardingCase struct{ name, source string }

func p15OptionalForwardingCases() []p15OptionalForwardingCase {
	const echo = "function echo(value:Text?)->Text?{return value}\n"
	cases := []p15OptionalForwardingCase{
		{"immutable_none", echo + "let candidate:Text?=none\ngetVariable(echo(candidate))"},
		{"mutable_none", echo + "var candidate:Text?=none\ngetVariable(echo(candidate))"},
		{"inferred_none", echo + "let candidate=none\ngetVariable(echo(candidate))"},
		{"immutable_alias", echo + "let initial:Text?=none\nlet candidate=initial\ngetVariable(echo(candidate))"},
		{"returned_none_default", "function echo(value:Text?=text(\"DEFAULT\"))->Text?{return value}\ngetVariable(echo(echo(none)))"},
		{"parameter_none", echo + "function relay(candidate:Text?)->Text?{return echo(candidate)}\ngetVariable(relay(none))"},
		{"parameter_empty", echo + "function relay(candidate:Text?)->Text?{return echo(candidate)}\ngetVariable(relay(\"\"))"},
		{"named_parameter", echo + "function relay(seed:Text,candidate:Text?)->Text?{return echo(candidate)}\ngetVariable(relay(candidate:none,seed:\"SEED\"))"},
		{"later_assignment", "function pick(candidate:Text?,later:Text)->Text?{return candidate}\nvar candidate:Text?=none\ngetVariable(pick(candidate,later:if true{candidate=\"\";yield \"WRITTEN\"}else{yield \"UNUSED\"}))"},
	}
	for _, tc := range p15FunctionOptionalCases()[:6] {
		cases = append(cases, p15OptionalForwardingCase{
			"returned_" + tc.name,
			"function echo(value:" + tc.typ + "?)->" + tc.typ + "?{return value}\ngetVariable(echo(echo(" + tc.argument + ")))"})
	}
	return cases
}

func TestP15OptionalForwardingRequiresOwnedPresence(t *testing.T) {
	const opaque = `native.action("is.workflow.actions.nothing",{})`
	for _, tc := range []p15OptionalForwardingCase{
		{"opaque_argument", "function echo(value:Text?)->Text?{return value}\ngetVariable(echo(" + opaque + "))"},
		{"opaque_return", "function absent(seed:Text)->Text?{return " + opaque + "}\ngetVariable(absent(\"SEED\"))"},
		{"opaque_binding", "let candidate:Text?=" + opaque + "\ngetVariable(candidate)"},
		{"opaque_assignment", "var candidate:Text?=none\ncandidate=" + opaque + "\ngetVariable(candidate)"},
		{"opaque_default", "function echo(value:Text?=" + opaque + ")->Text?{return value}\ngetVariable(echo())"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check := func(diagnostics []analysis.Diagnostic) {
				t.Helper()
				var errors []analysis.Diagnostic
				for _, d := range diagnostics {
					if d.Severity == analysis.SeverityError {
						errors = append(errors, d)
					}
				}
				start := strings.Index(tc.source, opaque)
				if len(errors) != 1 || errors[0].Code != analysis.CodeTargetValueShape ||
					errors[0].Span.Start.ByteOffset != start || errors[0].Span.End.ByteOffset != start+len(opaque) ||
					!strings.Contains(errors[0].Message, "compiler-owned") || !strings.Contains(errors[0].Message, "presence") {
					t.Errorf("opaque native output must not fabricate Optional presence: %#v", errors)
				}
			}
			check(p15TypeDiagnostics(t, tc.source))
			svc := service.NewService(nil)
			uri := "file:///p15-optional-presence-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			diagnostics, _ := svc.Analyze(uri)
			check(diagnostics)
			raw, err := CompileSourceToPlist("p15-optional-presence.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Errorf("opaque optional boundary emitted bytes=%d error=%v", len(raw), err)
			}
		})
	}
}

// These public-route controls establish the unsupported semantic boundary on
// the clean tests-only source. A successful compiler run is not Apple runtime
// evidence; the completed component also requires an independent native graph.
func TestP15OptionalForwardingPublicRoutes(t *testing.T) {
	for _, tc := range p15OptionalForwardingCases() {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if len(diagnostics) != 0 {
				t.Errorf("compiler-owned Optional value must remain transportable: %#v", diagnostics)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-optional-forwarding-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			editorDiagnostics, _ := svc.Analyze(uri)
			if len(editorDiagnostics) != 0 {
				t.Errorf("editor must accept the same Optional forwarding source: %#v", editorDiagnostics)
			}
			raw, err := CompileSourceToPlist("p15-short-circuit.cherri", tc.source)
			if err != nil || len(raw) == 0 {
				t.Fatalf("Optional forwarding failed: bytes=%d error=%v", len(raw), err)
			}
			if !strings.Contains(string(raw), "_cherri_optional_result_v1") {
				t.Fatal("Optional result lost its compiler-owned presence carrier")
			}
		})
	}
}
