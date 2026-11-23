package main

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"howett.net/plist"
)

type p15N03MigrationCondition struct {
	code    int
	right   string
	depth   int
	present bool
	legacy  bool
}

// The comparison mode is a wire-preservation assertion. It does not infer
// Apple's text-comparison behavior from a successful local compilation.
func p15N03MigrationConditions(actions []map[string]any) ([]p15N03MigrationCondition, error) {
	var result []p15N03MigrationCondition
	var groups []string
	for index, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		var parameters, okay = action["WFWorkflowActionParameters"].(map[string]any)
		if !okay {
			return nil, fmt.Errorf("condition %d has no parameter dictionary", index)
		}
		var group, valid = parameters["GroupingIdentifier"].(string)
		if !valid || group == "" {
			return nil, fmt.Errorf("condition %d has no control identity", index)
		}
		switch p15Int(parameters["WFControlFlowMode"]) {
		case 0:
			var right, text = parameters["WFConditionalActionString"].(string)
			if !text {
				return nil, fmt.Errorf("condition %d has no exact Text RHS: %#v", index, parameters)
			}
			for _, unexpected := range []string{"WFConditions", "WFNumberValue", "WFAnotherNumber", "WFDate", "WFAnotherDate", "WFDuration", "WFEnumeration"} {
				if _, exists := parameters[unexpected]; exists {
					return nil, fmt.Errorf("condition %d acquired %s", index, unexpected)
				}
			}
			var input, inputOkay = parameters["WFInput"].(map[string]any)
			if !inputOkay || input["Type"] != "Variable" || input["Variable"] == nil {
				return nil, fmt.Errorf("condition %d lost its native input", index)
			}
			var raw, present = parameters["WFConditionalLegacyComparisonBehavior"]
			var legacy bool
			if present {
				var typed bool
				legacy, typed = raw.(bool)
				if !typed {
					return nil, fmt.Errorf("condition %d comparison mode is not a bool", index)
				}
			}
			result = append(result, p15N03MigrationCondition{
				code: p15Int(parameters["WFCondition"]), right: right,
				depth: len(groups), present: present, legacy: legacy,
			})
			groups = append(groups, group)
		case 1:
			if len(groups) == 0 || groups[len(groups)-1] != group {
				return nil, fmt.Errorf("condition %d has an unmatched else", index)
			}
		case 2:
			if len(groups) == 0 || groups[len(groups)-1] != group {
				return nil, fmt.Errorf("condition %d has an unmatched end", index)
			}
			groups = groups[:len(groups)-1]
		default:
			return nil, fmt.Errorf("condition %d has an invalid control mode", index)
		}
	}
	if len(groups) != 0 {
		return nil, fmt.Errorf("unclosed comparison groups")
	}
	return result, nil
}

func p15N03MigrationCompile(t *testing.T, source, mode string) []map[string]any {
	t.Helper()
	var response = compileForMobileMode(source, "P15_N03_MIGRATION", mode, false)
	if !response.OK || response.Mode != mode || response.PlistBase64 == "" || response.SignedBase64 != "" {
		t.Fatalf("actual unsigned %s route failed: mode=%s error=%s", mode, response.Mode, response.Error)
	}
	var raw, err = base64.StdEncoding.DecodeString(response.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if _, err = plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	var rows, okay = document["WFWorkflowActions"].([]any)
	if !okay {
		t.Fatal("actual production result has no actions")
	}
	var actions = make([]map[string]any, len(rows))
	for index, row := range rows {
		var action, valid = row.(map[string]any)
		if !valid {
			t.Fatal("actual production result has a malformed action", index)
		}
		actions[index] = action
	}
	return actions
}

func TestP15N03MigrationPreservesComparisonMode(t *testing.T) {
	var fixtures = []struct {
		name, body string
		want       []p15N03MigrationCondition
	}{
		{"ordinary_equal", "if subject == \"abc\" {\n text(\"EQUAL\")\n}\n",
			[]p15N03MigrationCondition{{code: 4, right: "abc"}}},
		{"ordinary_not_equal", "if subject != \"ABC\" {\n text(\"NOT_EQUAL\")\n}\n",
			[]p15N03MigrationCondition{{code: 5, right: "ABC"}}},
		{"explicit_legacy_equal", "if legacy subject == \"abc\" {\n text(\"LEGACY_EQUAL\")\n}\n",
			[]p15N03MigrationCondition{{code: 4, right: "abc", present: true, legacy: true}}},
		{"explicit_legacy_not_equal", "if legacy subject != \"ABC\" {\n text(\"LEGACY_NOT_EQUAL\")\n}\n",
			[]p15N03MigrationCondition{{code: 5, right: "ABC", present: true, legacy: true}}},
		{"nested_ordinary_outer", "if subject == \"OUTER\" {\n if legacy subject != \"INNER\" {\n  text(\"NESTED\")\n }\n}\n",
			[]p15N03MigrationCondition{
				{code: 4, right: "OUTER"},
				{code: 5, right: "INNER", depth: 1, present: true, legacy: true},
			}},
		{"nested_legacy_outer", "if legacy subject != \"OUTER\" {\n if subject == \"INNER\" {\n  text(\"NESTED\")\n }\n}\n",
			[]p15N03MigrationCondition{
				{code: 5, right: "OUTER", present: true, legacy: true},
				{code: 4, right: "INNER", depth: 1},
			}},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			var original = "const subject = text(\"AbC\")\n" + fixture.body
			var before = p15N03MigrationCompile(t, original, "legacy")
			var expected, err = p15N03MigrationConditions(before)
			if err != nil || !reflect.DeepEqual(expected, fixture.want) {
				t.Fatalf("original Legacy wire does not match authored fixture: got=%+v want=%+v error=%v", expected, fixture.want, err)
			}
			var converted, diagnostics, migrationErr = migrate.MigrateSourceWithDiagnostics(original)
			if migrationErr != nil || len(diagnostics) != 0 {
				t.Fatalf("actual migration failed: %v %v", migrationErr, diagnostics)
			}
			var after = p15N03MigrationCompile(t, converted, "v2")
			var observed, conditionErr = p15N03MigrationConditions(after)
			if conditionErr != nil || !reflect.DeepEqual(observed, expected) {
				t.Errorf("migration changed exact comparison wire: before=%+v after=%+v error=%v\nmigrated source:\n%s", expected, observed, conditionErr, converted)
			}

			// Mutating the actual Legacy graph must not satisfy the original
			// expectation: absent, true and explicitly false remain distinct.
			for index, action := range before {
				if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
					continue
				}
				var parameters = action["WFWorkflowActionParameters"].(map[string]any)
				if p15Int(parameters["WFControlFlowMode"]) != 0 {
					continue
				}
				var originalValue, present = parameters["WFConditionalLegacyComparisonBehavior"]
				if present {
					delete(parameters, "WFConditionalLegacyComparisonBehavior")
				} else {
					parameters["WFConditionalLegacyComparisonBehavior"] = true
				}
				var damaged, damageErr = p15N03MigrationConditions(before)
				if damageErr == nil && reflect.DeepEqual(damaged, expected) {
					t.Error("comparison oracle ignored changed presence", index)
				}
				parameters["WFConditionalLegacyComparisonBehavior"] = false
				damaged, damageErr = p15N03MigrationConditions(before)
				if damageErr == nil && reflect.DeepEqual(damaged, expected) {
					t.Error("comparison oracle normalized false to absent/true", index)
				}
				if present {
					parameters["WFConditionalLegacyComparisonBehavior"] = originalValue
				} else {
					delete(parameters, "WFConditionalLegacyComparisonBehavior")
				}
			}
		})
	}
}
