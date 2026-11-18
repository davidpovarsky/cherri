package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

func p15DateQuantityOracle(actions []ShortcutAction, unit, magnitude, condition string) error {
	starts, ends := 0, 0
	group := ""
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" {
			continue
		}
		p := action.WFWorkflowActionParameters
		switch fmt.Sprint(p["WFControlFlowMode"]) {
		case "0":
			starts++
			group, _ = p["GroupingIdentifier"].(string)
			if group == "" || fmt.Sprint(p["WFCondition"]) != condition {
				return fmt.Errorf("conditional identity/code changed: %#v", p)
			}
			quantity := debugParamMap(p["WFDuration"])
			value := debugParamMap(quantity["Value"])
			if quantity["WFSerializationType"] != "WFQuantityFieldValue" ||
				value["Magnitude"] != magnitude || value["Unit"] != unit ||
				p["WFNumberValue"] != nil || p["WFDate"] != nil {
				return fmt.Errorf("duration field/type/value changed: %#v", p)
			}
			input, err := json.Marshal(p["WFInput"])
			if err != nil || !bytes.Contains(input, []byte("\"Type\":\"CurrentDate\"")) {
				return fmt.Errorf("actual CurrentDate input lost: %s", input)
			}
		case "2":
			ends++
			if group == "" || p["GroupingIdentifier"] != group {
				return fmt.Errorf("unbalanced conditional")
			}
		}
	}
	if starts != 1 || ends != 1 {
		return fmt.Errorf("conditional cardinality %d/%d", starts, ends)
	}
	return nil
}

func p15DateQuantityNegatives() []struct{ source, span string } {
	return []struct{ source, span string }{
		{"let duration=qty(3, \"hr\")\nif CurrentDate > duration {}", "duration"},
		{"function qty(value:Number, unit:Text)->AnyContent{return Clipboard}\nif CurrentDate > qty(3, \"hr\") {}", "qty(3, \"hr\")"},
		{"if CurrentDate > Clipboard {}", "Clipboard"},
		{"if CurrentDate > chooseFromList(Clipboard) {}", "chooseFromList(Clipboard)"},
		{"if CurrentDate > qty(3, \"GB\") {}", "qty(3, \"GB\")"},
		{"if CurrentDate > qty(Clipboard, \"hr\") {}", "qty(Clipboard, \"hr\")"},
		{"let unit=text(\"hr\")\nif CurrentDate > qty(3, unit) {}", "qty(3, unit)"},
		{"if 3 > qty(3, \"hr\") {}", "qty(3, \"hr\")"},
		{"if qty(3, \"hr\") < CurrentDate {}", "qty(3, \"hr\")"},
	}
}

func p15T04CheckDateQuantities(t *testing.T) {
	t.Helper()
	metadataSource := "if CurrentDate > qty(3, \"hr\") {}"
	metadataProgram, parseErrors := p15ParseProgram(metadataSource)
	if len(parseErrors) != 0 {
		t.Fatal(parseErrors)
	}
	withoutHour := analysis.NewAnalyzer(schema.NewRegistry(nil, map[string][]string{"dateUnit": {"min"}}))
	withoutHour.Analyze(metadataProgram)
	if err := p15ExactLiteralDiagnostic(withoutHour.Diagnostics(), metadataSource, "qty(3, \"hr\")", analysis.CodeArgumentType); err != nil {
		t.Fatalf("removed canonical unit still minted duration evidence: %v", err)
	}
	program, analyzer := p15T08Analyzer(t, "let duration=qty(3, \"hr\")")
	actual := analyzer.ExprTypes()[p15BindingValue(t, program, "duration")]
	if actual == nil || !actual.Equals(types.AnyContent) {
		t.Fatalf("parameter-only constructor acquired a Content Graph guarantee: %v", actual)
	}
	for _, tc := range []struct{ op, unit, code string }{{">", "hr", "1000"}, {"<", "min", "1001"}} {
		source := "if CurrentDate " + tc.op + " qty(3, \"" + tc.unit + "\") {}"
		for _, legacy := range []bool{true, false} {
			actions := compileDebugFixture(t, source, legacy)
			if err := p15DateQuantityOracle(actions, tc.unit, "3", tc.code); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"magnitude_type", "wrong_unit", "wrong_route", "wrong_condition", "wrong_input"} {
				raw, err := json.Marshal(actions)
				if err != nil {
					t.Fatal(err)
				}
				var broken []ShortcutAction
				if err := json.Unmarshal(raw, &broken); err != nil {
					t.Fatal(err)
				}
				for i := range broken {
					p := broken[i].WFWorkflowActionParameters
					if broken[i].WFWorkflowActionIdentifier != "is.workflow.actions.conditional" || fmt.Sprint(p["WFControlFlowMode"]) != "0" {
						continue
					}
					value := debugParamMap(debugParamMap(p["WFDuration"])["Value"])
					switch mutation {
					case "magnitude_type":
						value["Magnitude"] = float64(3)
					case "wrong_unit":
						value["Unit"] = "GB"
					case "wrong_route":
						p["WFNumberValue"] = p["WFDuration"]
						delete(p, "WFDuration")
					case "wrong_condition":
						p["WFCondition"] = float64(2)
					case "wrong_input":
						p["WFInput"] = map[string]any{"Type": "Clipboard"}
					}
				}
				if p15DateQuantityOracle(broken, tc.unit, "3", tc.code) == nil {
					t.Fatalf("accepted duration graph mutation %s", mutation)
				}
			}
		}
	}
	for _, tc := range p15DateQuantityNegatives() {
		if err := p15ExactLiteralDiagnostic(p15TypeDiagnostics(t, tc.source), tc.source, tc.span, analysis.CodeArgumentType); err != nil {
			t.Fatal(err)
		}
		raw, err := CompileSourceToPlist("p15-date-quantity-negative.cherri", tc.source)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
			t.Fatalf("unproved duration emitted bytes=%d err=%v", len(raw), err)
		}
	}
}

func p15T04CheckDateQuantityService(t *testing.T) {
	t.Helper()
	svc := service.NewService(nil)
	uri := "file:///p15-date-quantity.cherri"
	svc.OpenDocument(uri, 1, "if CurrentDate > qty(3, \"hr\") {}")
	diagnostics, _ := svc.Analyze(uri)
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	for i, tc := range p15DateQuantityNegatives() {
		svc.OpenDocument(uri, i+2, tc.source)
		diagnostics, _ = svc.Analyze(uri)
		if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.span, analysis.CodeArgumentType); err != nil {
			t.Fatal(err)
		}
	}
}

func TestP15DateComparisonMigrationCLIParity(t *testing.T) {
	fixture := filepath.Join("tests", "date-comparisons.cherri")
	original, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		after, err := os.ReadFile(fixture)
		if err != nil || !bytes.Equal(original, after) {
			t.Errorf("canonical Date fixture changed: %v", err)
		}
	}()
	migrated, diagnostics, err := migrate.MigrateSourceWithDiagnostics(string(original))
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("actual migration failed: %v %v", err, diagnostics)
	}
	if !strings.Contains(migrated, "qty(3, \"hr\")") {
		t.Fatal("date duration constructor disappeared")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "date-comparisons-v2.cherri")
	if err := os.WriteFile(source, []byte(migrated), 0600); err != nil {
		t.Fatal(err)
	}
	legacyOut, v2Out := filepath.Join(dir, "legacy.shortcut"), filepath.Join(dir, "v2.shortcut")
	bin := roundTripBinary(t)
	for _, args := range [][]string{{fixture, "--legacy", "--skip-sign", "-o", legacyOut, "--load-all-actions"}, {"compile", source, "--skip-sign", "-o", v2Out}} {
		stdout, stderr, err := p15RunCLI(t, bin, "", nil, args...)
		if err != nil {
			t.Fatalf("actual CLI failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
		}
	}
	result, err := shortcutcompare.CompareFiles(legacyOut, v2Out)
	mode := "canonical"
	if err == nil && !result.Equal {
		result, err = shortcutcompare.CompareFilesNormalized(legacyOut, v2Out)
		mode = "existing_normalized"
	}
	if err != nil || !result.Equal {
		t.Fatalf("unchanged Date fixture parity: %v %+v", err, result)
	}
	t.Logf("Unchanged complete date-comparisons fixture migrated and compiled through actual CLI; parity=%s", mode)
}
