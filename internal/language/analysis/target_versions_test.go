package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"testing"
)

func TestTargetVersionSettersRetainCompatibility(t *testing.T) {
	for _, tc := range []struct {
		minimum, maximum float64
		call             string
		rejected         bool
	}{
		{16.1, 0, "isCharging()", true}, {16.2, 0, "isCharging()", false},
		{17, 0, "isCharging()", false}, {0, 26, "askLLM(\"probe\")", false},
		{0, 26.1, "askLLM(\"probe\")", true}, {0, 0, "isCharging()", false},
	} {
		analyzer := NewAnalyzer(schema.DefaultRegistry())
		analyzer.SetMinTargetVersion(1)
		analyzer.SetMaxTargetVersion(1)
		analyzer.SetMinTargetVersion(tc.minimum)
		analyzer.SetMaxTargetVersion(tc.maximum)
		parser := syntax.NewParser(source.NewFile("setter.cherri", "file:///setter.cherri", 1, tc.call))
		program := parser.ParseProgram()
		if len(parser.Errors()) != 0 {
			t.Fatal(parser.Errors())
		}
		analyzer.Analyze(program)
		rejected := false
		for _, diagnostic := range analyzer.Diagnostics() {
			if diagnostic.Severity == SeverityError {
				if diagnostic.Code != CodeUnsupportedTarget {
					t.Fatal(diagnostic)
				}
				rejected = true
			}
		}
		if rejected != tc.rejected || analyzer.MinTargetVersion() != tc.minimum || analyzer.MaxTargetVersion() != tc.maximum {
			t.Fatalf("legacy setter contract: %+v diagnostics=%v", tc, analyzer.Diagnostics())
		}
		if tc.minimum == 0 && analyzer.MinTargetVersionString() != "" {
			t.Fatal("cleared minimum retained old source bound")
		}
		if tc.maximum == 0 && analyzer.MaxTargetVersionString() != "" {
			t.Fatal("cleared maximum retained old source bound")
		}
	}
}

func TestTargetVersionSourceAccessorsKeepComponents(t *testing.T) {
	analyzer := runAnalyzer(t, "shortcut \"parts\" { minVersion: 16.10\nmaxVersion: \"27.2.1\" }\nisCharging()")
	if analyzer.MinTargetVersionString() != "16.10" || analyzer.MaxTargetVersionString() != "27.2.1" {
		t.Fatal("source components were lost")
	}
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity == SeverityError {
			t.Fatal(diagnostic)
		}
	}
}
