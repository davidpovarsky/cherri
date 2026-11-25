package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

func TestValueConstructorsRejectUnsupportedInputs(t *testing.T) {
	var cases = []struct{ name, setup, expression, code string }{
		{"color_binding", "let red = 0.25\n", "color(red, 0.5, 0.75, 1)", analysis.CodeLiteralRequired},
		{"color_mutable", "var red = 0.25\n", "color(red, 0.5, 0.75, 1)", analysis.CodeLiteralRequired},
		{"color_action", "", "color(number(0.25), 0.5, 0.75, 1)", analysis.CodeLiteralRequired},
		{"color_arithmetic", "", "color(0.1 + 0.2, 0.5, 0.75, 1)", analysis.CodeLiteralRequired},
		{"color_text", "", "color(\"0.25\", 0.5, 0.75, 1)", analysis.CodeLiteralRequired},
		{"color_bool", "", "color(true, 0.5, 0.75, 1)", analysis.CodeLiteralRequired},
		{"color_extra", "", "color(0, 0, 0, 1, 0.5)", analysis.CodePositionalArg},
		{"color_duplicate", "", "color(0, red: 1)", analysis.CodeDuplicateArgument},
		{"color_unknown", "", "color(purple: 1)", analysis.CodeUnknownArgument},
		{"qty_empty", "", "qty()", analysis.CodeMissingArgument},
		{"qty_one", "", "qty(5)", analysis.CodeMissingArgument},
		{"qty_extra", "", "qty(1, \"sec\", 9)", analysis.CodePositionalArg},
		{"qty_text", "", "qty(\"oops\", \"sec\")", analysis.CodeArgumentType},
		{"qty_numeric_text", "", "qty(\"5\", \"sec\")", analysis.CodeArgumentType},
		{"qty_bool", "", "qty(true, \"sec\")", analysis.CodeArgumentType},
		{"qty_text_binding", "let amount = \"oops\"\n", "qty(amount, \"sec\")", analysis.CodeArgumentType},
		{"qty_numeric_unit", "", "qty(1, 5)", analysis.CodeArgumentType},
		{"qty_unit_binding", "let unit = \"sec\"\n", "qty(1, unit)", analysis.CodeLiteralRequired},
		{"qty_duplicate", "", "qty(1, value: 2, unit: \"sec\")", analysis.CodeDuplicateArgument},
		{"qty_unknown", "", "qty(1, units: \"sec\")", analysis.CodeUnknownArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var source = tc.setup + "let value = " + tc.expression + "\n"
			var diagnostics = p15TypeDiagnostics(t, source)
			var found = false
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError && diagnostic.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Errorf("analysis accepted invalid constructor; wanted %s, got %v", tc.code, diagnostics)
			}
			var raw, err = CompileSourceToPlist("constructor-negative.cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Errorf("compile must reject with no plist and %s; error=%v bytes=%d", tc.code, err, len(raw))
			}
		})
	}
	for index := 0; index < 4; index++ {
		var components = []string{"0.25", "0.5", "0.75", "1.0"}
		components[index] = "component"
		var source = "let component = number(0.25)\nmakeQRCode(\"probe\", foregroundColor: color(" + strings.Join(components, ", ") + "))"
		var raw, err = CompileSourceToPlist("constructor-slot.cherri", source)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeLiteralRequired) {
			t.Errorf("color slot %d silently accepted runtime component: %v", index, err)
		}
	}
}

func TestValueConstructorsColorLiteralWire(t *testing.T) {
	var cases = []struct {
		expression string
		want       [4]float64
		legacy     bool
	}{
		{"color(0.25, 0.5, 0.75, 1.0)", [4]float64{0.25, 0.5, 0.75, 1}, true},
		{"color(0.25, 0.5, 0.75, 1.0)", [4]float64{0.25, 0.5, 0.75, 1}, false},
		{"color()", [4]float64{0, 0, 0, 1}, true},
		{"color(0.25)", [4]float64{0.25, 0, 0, 1}, true},
		{"color()", [4]float64{0, 0, 0, 1}, false},
		{"color(0.25)", [4]float64{0.25, 0, 0, 1}, false},
		{"color(alpha: 0.5, blue: 0.75, red: 0.25, green: 0.5)", [4]float64{0.25, 0.5, 0.75, 0.5}, false},
		{"color(-0.25, +0.5, 2, 1)", [4]float64{-0.25, 0.5, 2, 1}, false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/legacy=%t", tc.expression, tc.legacy), func(t *testing.T) {
			var source = "makeQRCode(\"probe\", \"High\", " + tc.expression + ")"
			if tc.legacy {
				source = "#include 'actions/documents'\n" + source
			}
			var actions = compileDebugFixture(t, source, tc.legacy)
			var found = false
			for _, action := range actions {
				if action.WFWorkflowActionIdentifier != "is.workflow.actions.generatebarcode" {
					continue
				}
				found = true
				var actual = debugParamMap(action.WFWorkflowActionParameters["WFQRForegroundColor"])
				var want = map[string]any{"WFColorRepresentationType": "WFColorRepresentationTypeCGColor", "redComponent": tc.want[0], "greenComponent": tc.want[1], "blueComponent": tc.want[2], "alphaComponent": tc.want[3]}
				// The existing Legacy WFColorValue serializer omits zero RGB fields.
				if tc.legacy {
					for _, key := range []string{"redComponent", "greenComponent", "blueComponent"} {
						if want[key] == float64(0) {
							delete(want, key)
						}
					}
				}
				if !reflect.DeepEqual(actual, want) {
					t.Fatalf("color mismatch: got %#v want %#v", actual, want)
				}
			}
			if !found {
				t.Fatal("missing native color consumer")
			}
		})
	}
}

func TestValueConstructorsQuantityBindingAndUnits(t *testing.T) {
	for _, unit := range []string{"", "sec", "private-unit"} {
		for _, source := range []string{fmt.Sprintf("startTimer(qty(5, %q))", unit), fmt.Sprintf("startTimer(qty(unit: %q, value: 5))", unit)} {
			var actions = compileDebugFixture(t, source, false)
			var magnitude, _ = quantityMagnitude(t, actions, "is.workflow.actions.timer.start", "WFDuration", unit)
			if magnitude != "5" {
				t.Fatalf("wrong magnitude for named constructor: %#v", magnitude)
			}
		}
	}
	for _, source := range []string{"startTimer(qty())", "startTimer(qty(5))"} {
		var raw, err = compileLegacySourceToRawMapInMemory("#include 'actions/calendar'\n" + source)
		if err == nil || raw != nil || !strings.Contains(err.Error(), "Missing required") || strings.Contains(err.Error(), "index out of range") {
			t.Fatalf("legacy constructor should diagnose before indexing: %v", err)
		}
	}
}

func TestValueConstructorsNamedQuantityAndUnitTemplate(t *testing.T) {
	var source = "let amount = 1 + 1\nstartTimer(qty(unit: \"sec\", value: amount))"
	var actions = compileDebugFixture(t, source, false)
	quantityReference(t, actions, "is.workflow.actions.timer.start", "WFDuration", "sec", "ActionOutput", "is.workflow.actions.math")
	source = "let unit = \"sec\"\nstartTimer(qty(2, f\"{unit}\"))"
	actions = compileDebugFixture(t, source, false)
	var found = false
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.timer.start" {
			continue
		}
		var quantity = debugParamMap(action.WFWorkflowActionParameters["WFDuration"])
		var unit = debugParamMap(debugParamMap(quantity["Value"])["Unit"])
		if unit["WFSerializationType"] != "WFTextTokenString" {
			t.Fatalf("unit template lost canonical token-string form: %#v", unit)
		}
		found = true
	}
	if !found {
		t.Fatal("missing quantity template consumer")
	}
	if _, err := CompileSourceToPlist("named-date-quantity.cherri", "if CurrentDate > qty(unit: \"hr\", value: 3) { show(\"later\") }"); err != nil {
		t.Fatal(err)
	}
}
