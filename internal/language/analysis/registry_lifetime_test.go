package analysis

import (
	"fmt"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func registryProgram(t *testing.T, code string) *syntax.Program {
	t.Helper()
	parser := syntax.NewParser(source.NewFile("registry.cherri", "registry.cherri", 1, code))
	program := parser.ParseProgram()
	if errs := parser.Errors(); len(errs) != 0 {
		t.Fatalf("synthetic source parse: %v", errs)
	}
	return program
}

func registryCustomSource(name string) string {
	return fmt.Sprintf("action %s(value: Text) -> Text {\nidentifier: \"com.example.analysis.local\"\nprimary: \"value\"\nbindings: {value: {key: \"Value\", codec: \"text\"}}\n}\n%s(value: \"LOCAL\")\n", name, name)
}

func TestAnalyzerRegistryInputIsolation(t *testing.T) {
	registry := schema.NewRegistry(schema.DefaultRegistry().AllActions(), schema.DefaultRegistry().AllEnums())
	before, _ := registry.LookupAction("url")
	fingerprint := registry.Fingerprint()
	analyzer := NewAnalyzer(registry)
	analyzer.Analyze(registryProgram(t, registryCustomSource("url")))
	if diags := analyzer.Diagnostics(); len(diags) != 0 {
		t.Fatalf("custom source lost valid context: %v", diags)
	}
	after, _ := registry.LookupAction("url")
	if after.AppleIdentifier != before.AppleIdentifier || registry.Fingerprint() != fingerprint || len(registry.LookupByIdentifier("com.example.analysis.local")) != 0 {
		t.Error("analysis mutated its supplied registry")
	}
	later := NewAnalyzer(registry)
	later.Analyze(registryProgram(t, "url(url: [\"https://example.invalid/base\"])"))
	if diags := later.Diagnostics(); len(diags) != 0 {
		t.Errorf("independent analyzer inherited custom declaration: %v", diags)
	}
}

func TestAnalyzerRegistryReuseIsolation(t *testing.T) {
	registry := schema.NewRegistry(schema.DefaultRegistry().AllActions(), schema.DefaultRegistry().AllEnums())
	analyzer := NewAnalyzer(registry)
	analyzer.Analyze(registryProgram(t, registryCustomSource("url")+"unknownRegistryAction()\n"))
	if len(analyzer.Diagnostics()) == 0 {
		t.Fatal("negative control did not diagnose unknown action")
	}
	oldTypes := analyzer.ExprTypes()
	if len(oldTypes) == 0 {
		t.Fatal("first analysis has no expression evidence")
	}
	analyzer.Analyze(registryProgram(t, "url(url: [\"https://example.invalid/base\"])"))
	if diags := analyzer.Diagnostics(); len(diags) != 0 {
		t.Errorf("next analysis retained diagnostics or custom schema: %v", diags)
	}
	for expression := range oldTypes {
		if _, exists := analyzer.ExprTypes()[expression]; exists {
			t.Error("expression proof survived into a different program")
			break
		}
	}
}

func TestAnalyzerRegistryReuseTargetConfiguration(t *testing.T) {
	analyzer := NewAnalyzer(schema.DefaultRegistry())
	analyzer.SetTargets([]string{"ios"})
	analyzer.SetMinTargetVersion(16.2)
	analyzer.SetMaxTargetVersion(27)
	header := "shortcut \"first\" { target: \"macos\"\nminVersion: \"26.10\"\nmaxVersion: \"27.2.1\" }\ntext(\"first\")"
	analyzer.Analyze(registryProgram(t, header))
	if analyzer.MinTargetVersionString() != "26.10" || analyzer.MaxTargetVersionString() != "27.2.1" || len(analyzer.Targets()) != 1 || analyzer.Targets()[0] != "macos" {
		t.Fatal("source header was not applied to its program")
	}
	// Changing one API setting after a program must not retain that program's
	// other source-derived settings as implicit defaults.
	analyzer.SetMinTargetVersion(17)
	analyzer.Analyze(registryProgram(t, "text(\"second\")"))
	if analyzer.MinTargetVersionString() != "17" || analyzer.MaxTargetVersionString() != "27" || len(analyzer.Targets()) != 1 || analyzer.Targets()[0] != "ios" {
		t.Fatalf("source header escaped its analysis: target=%v min=%s max=%s", analyzer.Targets(), analyzer.MinTargetVersionString(), analyzer.MaxTargetVersionString())
	}
	analyzer.SetTargets(nil)
	analyzer.SetMinTargetVersion(0)
	analyzer.SetMaxTargetVersion(0)
	analyzer.Analyze(registryProgram(t, "text(\"third\")"))
	if len(analyzer.Targets()) != 0 || analyzer.IsMacTarget() != nil || analyzer.MinTargetVersionString() != "" || analyzer.MaxTargetVersionString() != "" {
		t.Fatal("explicit API clearing did not survive the next analysis")
	}
}

func TestAnalyzerRegistryConstructorSnapshot(t *testing.T) {
	registry := schema.NewRegistry(schema.DefaultRegistry().AllActions(), schema.DefaultRegistry().AllEnums())
	analyzer := NewAnalyzer(registry)
	registry.RegisterCustomAction(&schema.ActionSchema{CallableName: "url", AppleIdentifier: "com.example.external"})
	analyzer.Analyze(registryProgram(t, "url(url: [\"https://example.invalid/snapshot\"])"))
	if diags := analyzer.Diagnostics(); len(diags) != 0 {
		t.Fatalf("later registry mutation changed a constructed analyzer: %v", diags)
	}
}
