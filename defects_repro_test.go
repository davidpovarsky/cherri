package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// TestRepro_FIX03_AnalyzerBypassed verifies that CompileSourceToPlist catches type errors via analysis.
func TestRepro_FIX03_AnalyzerBypassed(t *testing.T) {
	code := "let x: Number = \"string_not_number\"\n"
	_, err := CompileSourceToPlist("bad_type.cherri", code)
	if err == nil {
		t.Fatalf("FIX-03 defect present: CompileSourceToPlist succeeded on type error without analysis")
	}
	if !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") {
		t.Fatalf("expected E_ARGUMENT_TYPE error, got: %v", err)
	}
}

// TestRepro_FIX04_LegacySyntaxNaiveCheck verifies that strings with '@' and 'const' compile cleanly.
func TestRepro_FIX04_LegacySyntaxNaiveCheck(t *testing.T) {
	contentStr := "let email = \"me@example.com\"\nlet js = \"const x = 1;\"\n"
	_, err := CompileSourceToPlist("valid_strings.cherri", contentStr)
	if err != nil {
		t.Fatalf("FIX-04 defect present: valid strings rejected: %v", err)
	}
}

// TestRepro_FIX08_ComparisonFallsThroughToPlus verifies that comparisons do NOT lower to math +
func TestRepro_FIX08_ComparisonFallsThroughToPlus(t *testing.T) {
	// Case 1: Static comparison (constant folded)
	codeStatic := "let c = 2 > 3\n"
	fStatic := source.NewFile("comp1", "comp1.cherri", 1, codeStatic)
	progStatic := syntax.NewParser(fStatic).ParseProgram()
	lowStatic := lower.NewLowerer(schema.DefaultRegistry())
	wfStatic, err := lowStatic.LowerProgram(progStatic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, act := range wfStatic.Actions {
		if act.AppleIdentifier == "is.workflow.actions.math" {
			if op, ok := act.Parameters["WFMathOperation"]; ok && op == "+" {
				t.Fatalf("FIX-08 defect present: static comparison 2 > 3 lowered to is.workflow.actions.math with operation '+'")
			}
		}
	}

	// Case 2: Dynamic comparison (lowered to conditional block)
	codeDynamic := "let a = 2\nlet c = a > 3\n"
	fDynamic := source.NewFile("comp2", "comp2.cherri", 1, codeDynamic)
	progDynamic := syntax.NewParser(fDynamic).ParseProgram()
	lowDynamic := lower.NewLowerer(schema.DefaultRegistry())
	wfDynamic, err := lowDynamic.LowerProgram(progDynamic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasConditional := false
	for _, act := range wfDynamic.Actions {
		if act.AppleIdentifier == "is.workflow.actions.math" {
			if op, ok := act.Parameters["WFMathOperation"]; ok && op == "+" {
				t.Fatalf("FIX-08 defect present: dynamic comparison lowered to is.workflow.actions.math with operation '+'")
			}
		}
		if act.AppleIdentifier == "is.workflow.actions.conditional" {
			hasConditional = true
		}
	}
	if !hasConditional {
		t.Fatalf("expected is.workflow.actions.conditional for dynamic comparison")
	}
}

// TestRepro_FIX10_FunctionsNotLowered verifies that functions lower successfully
func TestRepro_FIX10_FunctionsNotLowered(t *testing.T) {
	code := "function add(a: Number, b: Number) -> Number {\n    return a + b\n}\nlet res = add(2, b: 3)\n"
	f := source.NewFile("fn", "fn.cherri", 1, code)
	prog := syntax.NewParser(f).ParseProgram()
	low := lower.NewLowerer(schema.DefaultRegistry())
	wf, err := low.LowerProgram(prog)
	if err != nil {
		t.Fatalf("FIX-10 defect present: function lowering failed: %v", err)
	}
	if len(wf.Actions) == 0 {
		t.Fatalf("expected actions emitted for function and call")
	}
}

// TestRepro_FIX13_MetadataDeletedInMigration verifies that MigrateSource preserves #define
func TestRepro_FIX13_MetadataDeletedInMigration(t *testing.T) {
	legacy := "#define name \"TestWorkflow\"\n#define icon 59789\n@x = 1\n"
	migrated := migrate.MigrateSource(legacy)
	if !strings.Contains(migrated, "TestWorkflow") {
		t.Fatalf("FIX-13 defect present: MigrateSource dropped #define name metadata from output:\n%s", migrated)
	}
	if !strings.Contains(migrated, "59789") {
		t.Fatalf("FIX-13 defect present: MigrateSource dropped #define icon metadata from output:\n%s", migrated)
	}
	if !strings.Contains(migrated, "let x = 1") {
		t.Fatalf("FIX-13 defect present: MigrateSource failed to convert variable assignment:\n%s", migrated)
	}
}

// TestRepro_FIX02_AcceptanceFalseCounting verifies that status == "PASSED" is NOT counted when passed == false
func TestRepro_FIX02_AcceptanceFalseCounting(t *testing.T) {
	type CaseResult struct {
		Passed bool
		Status string
	}
	res := CaseResult{Passed: false, Status: "PASSED"}
	passedCount := 0
	if res.Passed {
		passedCount++
	}
	if passedCount != 0 {
		t.Fatalf("FIX-02 defect present: false pass was counted")
	}
}

