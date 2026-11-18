package main

import (
	"os"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// This is a production comparison regression, not proof of per-platform version metadata.
func TestP15TargetVersionComponentsProduction(t *testing.T) {
	catalog, err := os.ReadFile("actions/device.cherri")
	if err != nil {
		t.Fatal(err)
	}
	intelligence, err := os.ReadFile("actions/intelligence.cherri")
	if err != nil {
		t.Fatal(err)
	}
	charging, ok := schema.DefaultRegistry().LookupAction("isCharging")
	llm, llmOK := schema.DefaultRegistry().LookupAction("askLLM")
	if !ok || !llmOK || charging.MinVersion != 16.2 || llm.MinVersion != 26 || llm.MaxVersion != 26 ||
		!strings.Contains(string(catalog), "action v16.2 'getbatterylevel' isCharging()") ||
		!strings.Contains(string(intelligence), "action v26 v26< askLLM(") {
		t.Fatal("production version fixtures must match the canonical action declarations")
	}
	type versionCase struct {
		field, value, call, identifier string
		allowed                        bool
	}
	var cases []versionCase
	for _, field := range []string{"minVersion", "iosVersion", "clientVersion", "version"} {
		for _, tc := range []struct {
			value   string
			allowed bool
		}{
			{"16.10", true}, {"\"16.10\"", true}, {"\"16.2.0\"", true},
			{"\"16.02\"", true}, {"\"16.1.9\"", false}, {"16.1", false},
			{"\"0\"", false},
		} {
			cases = append(cases, versionCase{field, tc.value, "isCharging()", "is.workflow.actions.getbatterylevel", tc.allowed})
		}
	}
	cases = append(cases,
		versionCase{"maxVersion", "\"26.0.1\"", "askLLM(\"probe\")", "is.workflow.actions.askllm", false},
		versionCase{"maxVersion", "\"26.00.0\"", "askLLM(\"probe\")", "is.workflow.actions.askllm", true},
		versionCase{"maxVersion", "26", "askLLM(\"probe\")", "is.workflow.actions.askllm", true},
	)
	for _, tc := range cases {
		t.Run(tc.field+"/"+tc.value, func(t *testing.T) {
			src := "shortcut \"version-probe\" { " + tc.field + ": " + tc.value + " }\n" + tc.call
			diagnostics := p15TypeDiagnostics(t, src)
			if !tc.allowed {
				if err := p15ExactLiteralDiagnostic(diagnostics, src, tc.call, analysis.CodeUnsupportedTarget); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(diagnostics[0].Message, strings.Trim(tc.value, "\"")) {
					t.Fatalf("diagnostic lost the authored components: %v", diagnostics)
				}
				raw, err := CompileSourceToPlist("p15-version.cherri", src)
				if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeUnsupportedTarget) {
					t.Fatalf("unsupported version passed native boundary: bytes=%d err=%v", len(raw), err)
				}
				mutated := append([]analysis.Diagnostic(nil), diagnostics...)
				for i := range mutated {
					if mutated[i].Code == analysis.CodeUnsupportedTarget {
						mutated[i].Code = analysis.CodeUnknownName
					}
				}
				if p15ExactLiteralDiagnostic(mutated, src, tc.call, analysis.CodeUnsupportedTarget) == nil {
					t.Fatal("unrelated diagnostic satisfied version contract")
				}
				return
			}
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			actions := compileDebugFixture(t, src, false)
			found := 0
			for _, action := range actions {
				if action.WFWorkflowActionIdentifier == tc.identifier {
					found++
					if tc.identifier == charging.AppleIdentifier && action.WFWorkflowActionParameters["Subject"] != "Is Charging" {
						t.Fatal("wrong production action variant")
					}
				}
			}
			if found != 1 {
				t.Fatalf("version accepted without expected native action: %d", found)
			}
		})
	}
}

func TestP15TargetVersionMalformedBoundsFailClosed(t *testing.T) {
	for _, field := range []string{"minVersion", "maxVersion"} {
		for _, value := range []string{
			"\"\"", "\"16..2\"", "\"16.2junk\"", "\"16.\"", "\" 16.2\"",
			"\"16.2 \"", "\"16.2.3e4\"", "\"999999999999999999999999999999999999\"",
			"true", "none", "[\"16.2\"]", "text(\"16.2\")", "-1",
		} {
			t.Run(field+"/"+value, func(t *testing.T) {
				src := "shortcut \"invalid-version\" { " + field + ": " + value + " }\ncomment(\"probe\")"
				diagnostics := p15TypeDiagnostics(t, src)
				if err := p15ExactLiteralDiagnostic(diagnostics, src, value, analysis.CodeTargetValueShape); err != nil {
					t.Fatal(err)
				}
				raw, err := CompileSourceToPlist("p15-version-invalid.cherri", src)
				if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
					t.Fatalf("invalid version was silently ignored: bytes=%d err=%v", len(raw), err)
				}
			})
		}
	}
	// No explicit bound continues to use the existing compiler default.
	compileDebugFixture(t, "isCharging()", false)
}

func TestP15TargetVersionServiceEdits(t *testing.T) {
	svc := service.NewService(nil)
	uri := "file:///p15-version-service.cherri"
	for i, tc := range []struct{ value, code, span string }{
		{"16.10", "", ""},
		{"\"16.1.9\"", analysis.CodeUnsupportedTarget, "isCharging()"},
		{"\"16.2.0\"", "", ""},
		{"\"16..2\"", analysis.CodeTargetValueShape, "\"16..2\""},
		{"27", "", ""},
	} {
		src := "shortcut \"version-editor\" { minVersion: " + tc.value + " }\nisCharging()"
		if i == 0 {
			svc.OpenDocument(uri, 1, src)
		} else {
			svc.ChangeDocument(uri, i+1, src)
		}
		diagnostics, _ := svc.Analyze(uri)
		if tc.code != "" {
			if err := p15ExactLiteralDiagnostic(diagnostics, src, tc.span, tc.code); err != nil {
				t.Fatal(err)
			}
		} else {
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					t.Fatalf("stale/wrong error after edit %d: %v", i, d)
				}
			}
		}
	}
}

func TestP15TargetVersionIncompleteMetadataRecoversInService(t *testing.T) {
	svc := service.NewService(nil)
	uri := "file:///p15-version-incomplete.cherri"
	svc.OpenDocument(uri, 1, "shortcut \"typing\" { minVersion: }")
	diagnostics, _ := svc.Analyze(uri)
	if len(diagnostics) == 0 {
		t.Fatal("incomplete value lost parser error")
	}
	svc.ChangeDocument(uri, 2, "shortcut \"typing\" { minVersion: 16.10 }\nisCharging()")
	diagnostics, _ = svc.Analyze(uri)
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
}
