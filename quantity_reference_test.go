package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
)

// Original Cherri bcc00957 and upstream 98d5191 emit bare reference dictionaries
// in Magnitude. A successful compile alone also accepts the broken Go string.
func quantityMagnitude(t *testing.T, actions []ShortcutAction, identifier, key, unit string) (any, int) {
	t.Helper()
	var found = -1
	var magnitude any
	for index, action := range actions {
		if action.WFWorkflowActionIdentifier != identifier {
			continue
		}
		if found >= 0 {
			t.Fatalf("duplicate quantity consumer %s", identifier)
		}
		found = index
		var quantity = debugParamMap(action.WFWorkflowActionParameters[key])
		var value = debugParamMap(quantity["Value"])
		if quantity["WFSerializationType"] != "WFQuantityFieldValue" || len(value) != 2 || value["Unit"] != unit {
			t.Fatalf("quantity field/type/unit changed: %#v", quantity)
		}
		magnitude = value["Magnitude"]
	}
	if found < 0 {
		t.Fatalf("missing quantity consumer %s", identifier)
	}
	return magnitude, found
}

func quantityReference(t *testing.T, actions []ShortcutAction, identifier, key, unit, kind, producer string) map[string]any {
	t.Helper()
	var magnitude, consumer = quantityMagnitude(t, actions, identifier, key, unit)
	var reference, ok = magnitude.(map[string]any)
	if !ok || reference["Type"] != kind || reference["Value"] != nil || reference["WFSerializationType"] != nil {
		t.Fatalf("magnitude must be a bare %s reference, got %T %#v", kind, magnitude, magnitude)
	}
	if kind == "ActionOutput" {
		var uuid, _ = reference["OutputUUID"].(string)
		if uuid == "" {
			t.Fatal("quantity reference has no producer UUID")
		}
		var matches = 0
		for index, action := range actions {
			if action.WFWorkflowActionParameters["UUID"] == uuid {
				if index >= consumer || action.WFWorkflowActionIdentifier != producer {
					t.Fatalf("quantity references the wrong/forward producer: %#v", action)
				}
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("quantity producer matches = %d, want 1", matches)
		}
	}
	return reference
}

func TestQuantityDynamicReferenceShapes(t *testing.T) {
	var cases = []struct {
		name, legacy, v2, kind, producer, variable string
		transformations                            []map[string]any
	}{
		{name: "arithmetic", legacy: "const seconds = 1 + 1", v2: "let seconds = 1 + 1", kind: "ActionOutput", producer: "is.workflow.actions.math"},
		{name: "action_result", legacy: "const seconds = round(2.2)", v2: "let seconds = round(2.2)", kind: "ActionOutput", producer: "is.workflow.actions.round"},
		{name: "mutable", legacy: "@seconds = 2", v2: "var seconds = 2", kind: "Variable", variable: "seconds"},
		{name: "global_input", legacy: "", v2: "", kind: "ExtensionInput", transformations: []map[string]any{{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFNumberContentItem"}}},
		{name: "global_clipboard", legacy: "", v2: "", kind: "Clipboard", transformations: []map[string]any{{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFNumberContentItem"}}},
		{name: "device_property", legacy: "", v2: "", kind: "DeviceDetails", transformations: []map[string]any{{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Screen Width"}, {"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFNumberContentItem"}}},
		{name: "dictionary_key", legacy: "const data = {\"seconds\":2}", v2: "let data = {\"seconds\":2}", kind: "ActionOutput", producer: "is.workflow.actions.dictionary", transformations: []map[string]any{{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "seconds"}, {"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFNumberContentItem"}}},
	}
	for _, tc := range cases {
		for _, legacy := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tc.name, legacy), func(t *testing.T) {
				var source = tc.v2
				var operand = "seconds"
				if legacy {
					source = "#include 'actions/calendar'\n#include 'actions/math'\n" + tc.legacy
					if tc.variable != "" {
						operand = "@seconds"
					}
				}
				switch tc.name {
				case "global_input":
					operand = "ShortcutInput.number"
				case "global_clipboard":
					operand = "Clipboard.number"
				case "device_property":
					operand = "Device[\"Screen Width\"].number"
				case "dictionary_key":
					operand = "data[\"seconds\"].number"
				}
				source += "\nstartTimer(qty(" + operand + ", \"sec\"))\n"
				var actions = compileDebugFixture(t, source, legacy)
				var reference = quantityReference(t, actions, "is.workflow.actions.timer.start", "WFDuration", "sec", tc.kind, tc.producer)
				if tc.variable != "" && reference["VariableName"] != tc.variable {
					t.Fatalf("wrong mutable binding: %#v", reference)
				}
				var transformations = reference["Aggrandizements"]
				if len(tc.transformations) == 0 {
					if transformations != nil {
						t.Fatalf("unexpected transforms: %#v", transformations)
					}
				} else {
					var actual, ok = transformations.([]any)
					if !ok || len(actual) != len(tc.transformations) {
						t.Fatalf("wrong transform cardinality: %#v", transformations)
					}
					for i, want := range tc.transformations {
						if !reflect.DeepEqual(debugParamMap(actual[i]), want) {
							t.Fatalf("transform %d got %#v want %#v", i, actual[i], want)
						}
					}
				}
				if tc.kind != "ActionOutput" && (reference["OutputUUID"] != nil || reference["OutputName"] != nil) {
					t.Fatalf("global/mutable identity changed: %#v", reference)
				}
			})
		}
	}
}

func TestQuantityStaticMagnitudeShapes(t *testing.T) {
	for _, value := range []string{"0", "2", "2.25", "-1"} {
		for _, legacy := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/legacy=%t", value, legacy), func(t *testing.T) {
				var source = "startTimer(qty(" + value + ", \"sec\"))"
				if legacy {
					source = "#include 'actions/calendar'\n" + source
				}
				var actions = compileDebugFixture(t, source, legacy)
				var magnitude, _ = quantityMagnitude(t, actions, "is.workflow.actions.timer.start", "WFDuration", "sec")
				if magnitude != value {
					t.Fatalf("literal magnitude got %T %#v, want string %q", magnitude, magnitude, value)
				}
			})
		}
	}
}

func TestQuantityCatalogDynamicParameters(t *testing.T) {
	resetCompilerStateFully()
	defer resetParser()
	relativePath = ""
	markBuiltins()
	defineRawAction()
	loadStandardActions()
	var catalog = buildActionCatalog()
	var categories = buildActionIncludeCategoryMap()
	var tested = 0
	for _, entry := range catalog {
		for parameterIndex, parameter := range entry.Parameters {
			if parameter.Type != "qty" {
				continue
			}
			tested++
			t.Run(entry.Name+"/"+parameter.Name, func(t *testing.T) {
				var limit = parameterIndex
				for i, p := range entry.Parameters {
					if !p.Optional && i > limit {
						limit = i
					}
				}
				var arguments []string
				var unit = "sec"
				if len(parameter.EnumValues) > 0 {
					unit = parameter.EnumValues[0]
				}
				for i := 0; i <= limit; i++ {
					if i == parameterIndex {
						arguments = append(arguments, fmt.Sprintf("qty(quantityMagnitude, %q)", unit))
					} else {
						arguments = append(arguments, legacyNonDefaultArgForParam(entry.Name, entry.Parameters[i]))
					}
				}
				var source = "#define mac true\n"
				if category := categories[entry.Name]; category != "" {
					source += "#include 'actions/" + category + "'\n"
				}
				source += "const quantityMagnitude = 2 + 3\n" + entry.Name + "(" + strings.Join(arguments, ", ") + ")\n"
				var migrated, diagnostics, err = migrate.MigrateSourceWithDiagnostics(source)
				if err != nil || len(diagnostics) != 0 {
					t.Fatalf("quantity fixture migration: %v %v", err, diagnostics)
				}
				for _, legacy := range []bool{true, false} {
					var fixture = migrated
					if legacy {
						fixture = source
					}
					var actions = compileDebugFixture(t, fixture, legacy)
					quantityReference(t, actions, entry.ShortcutIdentifier, parameter.Key, unit, "ActionOutput", "is.workflow.actions.math")
				}
			})
		}
	}
	if tested < 4 {
		t.Fatalf("quantity catalog coverage unexpectedly shrank: %d", tested)
	}
	t.Logf("checked all %d catalog quantity parameters in Legacy and V2", tested)
}

func TestQuantityNestedProducerAndNativeIdentity(t *testing.T) {
	for _, tc := range []struct{ name, source, producer string }{
		{"nested_action", "startTimer(qty(round(2.2), \"sec\"))", "is.workflow.actions.round"},
		{"native_result", "let seconds = native.action(\"is.workflow.actions.number\", {\"WFNumberActionNumber\":2})\nstartTimer(qty(seconds, \"sec\"))", "is.workflow.actions.number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var actions = compileDebugFixture(t, tc.source, false)
			quantityReference(t, actions, "is.workflow.actions.timer.start", "WFDuration", "sec", "ActionOutput", tc.producer)
			var producers = 0
			for _, action := range actions {
				if action.WFWorkflowActionIdentifier == tc.producer {
					producers++
				}
			}
			if producers != 1 {
				t.Fatalf("quantity operand evaluated %d times", producers)
			}
		})
	}
}

func TestQuantityArgumentValueSnapshot(t *testing.T) {
	var header, diagnostics, err = migrate.MigrateSourceWithDiagnostics("#define mac true\n")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("Mac metadata migration: %v %v", err, diagnostics)
	}
	var source = header + `
var seconds = 2
makeSizedDiskImage("Quantity Snapshot", contents:ShortcutInput, diskSize:qty(seconds,"MB"), encrypt:measure("change"){seconds=9;false})
`
	var actions = compileDebugFixture(t, source, false)
	var reference = quantityReference(t, actions, "is.workflow.actions.makediskimage", "ImageSize", "MB", "ActionOutput", "is.workflow.actions.getvariable")
	var capture, mutation = -1, -1
	for index, action := range actions {
		var parameters = action.WFWorkflowActionParameters
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" && parameters["UUID"] == reference["OutputUUID"] {
			var variable = debugParamMap(debugParamMap(parameters["WFVariable"])["Value"])
			if variable["Type"] != "Variable" || variable["VariableName"] != "seconds" {
				t.Fatalf("snapshot read wrong binding: %#v", parameters)
			}
			capture = index
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && parameters["WFVariableName"] == "seconds" {
			mutation = index
		}
	}
	if capture < 0 || mutation < 0 || capture >= mutation {
		t.Fatalf("quantity capture=%d must precede later mutation=%d", capture, mutation)
	}
}
