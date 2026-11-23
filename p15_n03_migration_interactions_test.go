package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15N03CanonicalMigrationSource(t *testing.T, text string) string {
	t.Helper()
	var parser = syntax.NewParser(source.NewFile("n03-migration-expected", "n03.cherri", 1, text))
	var program = parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal("independently authored expected source is invalid", parser.Errors())
	}
	return syntax.Format(program)
}

func p15N03MigrationModes(actions []map[string]any) ([]any, error) {
	var modes []any
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		var parameters = action["WFWorkflowActionParameters"].(map[string]any)
		if p15Int(parameters["WFControlFlowMode"]) != 0 {
			continue
		}
		var mode any
		if value, present := parameters["WFConditionalLegacyComparisonBehavior"]; present {
			if _, valid := value.(bool); !valid {
				return nil, fmt.Errorf("comparison mode has wrong native type")
			}
			mode = value
		}
		modes = append(modes, []any{p15Int(parameters["WFCondition"]), mode})
	}
	return modes, nil
}

func TestP15N03MigrationComparisonRewriteInteractions(t *testing.T) {
	const legacyText = "const subject = text(\"AbC\")\n"
	const canonicalText = "let subject = text(\"AbC\")\n"
	const header = "shortcut \"\" { outputMode: \"legacy\" }\n"
	var fixtures = []struct {
		name, original, expected string
		legacyDifferential       bool
	}{
		{"not_contains", legacyText + "if subject !contains \"b\" { text(\"YES\") }\n",
			canonicalText + "if modern: !(subject contains \"b\") { text(\"YES\") }\n", true},
		{"legacy_not_contains", legacyText + "if legacy subject !contains \"b\" { text(\"YES\") }\n",
			canonicalText + "if legacy: !(subject contains \"b\") { text(\"YES\") }\n", true},
		// These two operators have existing converter rewrites, but the Legacy
		// compiler's conditions map does not accept them. They are deliberately
		// conversion/AST/V2 controls, not successful Legacy-route evidence.
		{"converter_not_begins", legacyText + "if subject !beginsWith \"A\" { text(\"YES\") }\n",
			canonicalText + "if modern: !(subject beginsWith \"A\") { text(\"YES\") }\n", false},
		{"converter_legacy_not_begins", legacyText + "if legacy subject !beginsWith \"A\" { text(\"YES\") }\n",
			canonicalText + "if legacy: !(subject beginsWith \"A\") { text(\"YES\") }\n", false},
		{"converter_not_ends", legacyText + "if subject !endsWith \"C\" { text(\"YES\") }\n",
			canonicalText + "if modern: !(subject endsWith \"C\") { text(\"YES\") }\n", false},
		{"converter_legacy_not_ends", legacyText + "if legacy subject !endsWith \"C\" { text(\"YES\") }\n",
			canonicalText + "if legacy: !(subject endsWith \"C\") { text(\"YES\") }\n", false},
		{"between", "const value = number(2)\nif value <> 1 3 { text(\"YES\") }\n",
			"let value = number(2)\nif modern: isBetween(value, 1, 3) { text(\"YES\") }\n", true},
		{"legacy_between", "const value = number(2)\nif legacy value <> 1 3 { text(\"YES\") }\n",
			"let value = number(2)\nif legacy: isBetween(value, 1, 3) { text(\"YES\") }\n", true},
		{"today", "if CurrentDate isToday { text(\"YES\") }\n",
			"if modern: isToday(CurrentDate) { text(\"YES\") }\n", true},
		{"legacy_today", "if legacy CurrentDate isToday { text(\"YES\") }\n",
			"if legacy: isToday(CurrentDate) { text(\"YES\") }\n", true},
		{"inline_nested", legacyText + "if subject == \"OUTER\" { if legacy subject != \"INNER\" { text(\"YES\") } }\n",
			canonicalText + "if modern: subject == \"OUTER\" { if legacy: subject != \"INNER\" { text(\"YES\") } }\n", true},
		{"else_if", legacyText + "if subject == \"FIRST\" { text(\"FIRST\") } else if legacy subject == \"SECOND\" { text(\"SECOND\") }\n",
			canonicalText + "if modern: subject == \"FIRST\" { text(\"FIRST\") } else if legacy: subject == \"SECOND\" { text(\"SECOND\") }\n", false},
		{"comment_and_literal_decoys", legacyText + "// if legacy subject == \"COMMENT\"\ntext('if legacy literal')\nif subject == \"ACTUAL\" { text(\"YES\") }\n",
			canonicalText + "// if legacy subject == \"COMMENT\"\ntext(\"if legacy literal\")\nif modern: subject == \"ACTUAL\" { text(\"YES\") }\n", true},
		{"canonical_unmarked", canonicalText + "if subject == \"a\" { text(\"YES\") }\n",
			canonicalText + "if subject == \"a\" { text(\"YES\") }\n", false},
		{"canonical_modern", canonicalText + "if modern: subject == \"a\" { text(\"YES\") }\n",
			canonicalText + "if modern: subject == \"a\" { text(\"YES\") }\n", false},
		{"canonical_legacy_identifier", "let legacy = text(\"AbC\")\nif legacy == \"a\" { text(\"YES\") }\n",
			"let legacy = text(\"AbC\")\nif legacy == \"a\" { text(\"YES\") }\n", false},
		{"keyword_comment", legacyText + "if /* KEEP_BEFORE */ legacy /* KEEP_AFTER */ subject == \"a\" { text(\"YES\") }\n",
			canonicalText + "if legacy: /* KEEP_BEFORE */ /* KEEP_AFTER */ subject == \"a\" { text(\"YES\") }\n", false},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			var converted, diagnostics, err = migrate.MigrateSourceWithDiagnostics(fixture.original)
			if err != nil || len(diagnostics) != 0 {
				t.Fatalf("actual migration failed: %v %v", err, diagnostics)
			}
			var expected = p15N03CanonicalMigrationSource(t, header+fixture.expected)
			if converted != expected {
				t.Fatalf("migration changed the independently authored complete AST/format:\nactual:\n%s\nexpected:\n%s", converted, expected)
			}
			var after = p15N03MigrationCompile(t, converted, "v2")
			if fixture.legacyDifferential {
				var before = p15N03MigrationCompile(t, fixture.original, "legacy")
				var oldModes, oldErr = p15N03MigrationModes(before)
				var newModes, newErr = p15N03MigrationModes(after)
				if oldErr != nil || newErr != nil || len(oldModes) == 0 || !reflect.DeepEqual(oldModes, newModes) {
					t.Fatalf("supported Legacy comparison code/mode changed: old=%v new=%v errors=%v/%v", oldModes, newModes, oldErr, newErr)
				}
			}
			var repeated, repeatedDiagnostics, repeatedErr = migrate.MigrateSourceWithDiagnostics(converted)
			if repeatedErr != nil || len(repeatedDiagnostics) != 0 || repeated != converted {
				t.Fatalf("a second migration changed canonical comparison intent: %v %v\nfirst:\n%s\nsecond:\n%s", repeatedErr, repeatedDiagnostics, converted, repeated)
			}
		})
	}
}
