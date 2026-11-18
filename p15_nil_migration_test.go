package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

func p15NilMigration(t *testing.T, legacy string) string {
	t.Helper()
	migrated, diagnostics, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("migration failed: %v %v", err, diagnostics)
	}
	return migrated
}

func p15NilBinding(t *testing.T, source, name string) *syntax.BindingStmt {
	t.Helper()
	program, errors := p15ParseProgram(source)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, statement := range program.Statements {
		if binding, ok := statement.(*syntax.BindingStmt); ok && binding.Name == name {
			return binding
		}
	}
	t.Fatalf("missing binding %q in %s", name, source)
	return nil
}

func p15NilClearOracle(actions []ShortcutAction, name string) error {
	count := 0
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" ||
			action.WFWorkflowActionParameters["WFVariableName"] != name {
			continue
		}
		count++
		_, hasInput := action.WFWorkflowActionParameters["WFInput"]
		if count == 1 && !hasInput {
			return fmt.Errorf("initial assignment lost its producer input")
		}
		if count == 2 && hasInput {
			return fmt.Errorf("explicit clear gained an input")
		}
	}
	if count != 2 {
		return fmt.Errorf("want exactly initial assignment and clear, got %d", count)
	}
	return nil
}

func TestP15NilExplicitOptionalWireControl(t *testing.T) {
	for _, tc := range []struct {
		source string
		legacy bool
	}{
		{"#include 'actions/documents'\n@media = getFile(nil)\n@media = nil\n", true},
		{"var media: File? = getFile(nil)\nmedia = none\n", false},
	} {
		actions := compileDebugFixture(t, tc.source, tc.legacy)
		if err := p15NilClearOracle(actions, "media"); err != nil {
			t.Fatal(err)
		}
		for _, mutation := range []string{"supply_input", "remove_clear", "retarget_clear"} {
			broken := p15CloneVCardActions(t, actions)
			for i := len(broken) - 1; i >= 0; i-- {
				action := &broken[i]
				if action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" ||
					action.WFWorkflowActionParameters["WFVariableName"] != "media" {
					continue
				}
				switch mutation {
				case "supply_input":
					action.WFWorkflowActionParameters["WFInput"] = map[string]any{}
				case "remove_clear":
					broken = append(broken[:i], broken[i+1:]...)
				case "retarget_clear":
					action.WFWorkflowActionParameters["WFVariableName"] = "another"
				}
				break
			}
			if p15NilClearOracle(broken, "media") == nil {
				t.Fatalf("clear oracle accepted %q", mutation)
			}
		}
	}
}

func TestP15NilMigrationOptionalEvidence(t *testing.T) {
	for _, tc := range []struct{ name, source, binding, inner string }{
		{"file", "@media = getFile(nil)\n@media = nil\n", "media", "File"},
		{"header_offsets", "#define name NilContract\n@media = getFile(nil)\n@media = nil\n", "media", "File"},
		{"typed_legacy", "@value: number\n@value = nil\n", "value", "Number"},
		{"number", "@value = 1\n@value = nil\n", "value", "Number"},
		{"text", "@value = \"safe\"\n@value = nil\n", "value", "Text"},
		{"false_present", "@value = false\n@value = nil\n", "value", "Bool"},
		{"zero_present", "@value = 0\n@value = nil\n", "value", "Number"},
		{"conditional_clear", "@value = 1\nif true {\n@value = nil\n}\n", "value", "Number"},
		{"repeat_clear", "@value = 1\nrepeat i for 1 {\n@value = nil\n}\n", "value", "Number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			migrated := p15NilMigration(t, tc.source)
			binding := p15NilBinding(t, migrated, tc.binding)
			optional, ok := binding.TypeExpr.(*syntax.OptionalTypeAnnotation)
			if !ok || !binding.Mutable {
				t.Fatalf("Legacy clear lost explicit mutable Optional contract:\n%s", migrated)
			}
			inner, ok := optional.Inner.(*syntax.NamedTypeAnnotation)
			if !ok || inner.Name != tc.inner {
				t.Fatalf("wrong evidenced Optional element: %#v", optional.Inner)
			}
			for _, diagnostic := range p15TypeDiagnostics(t, migrated) {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			actions := compileDebugFixture(t, migrated, false)
			if err := p15NilClearOracle(actions, tc.binding); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15NilMigrationKeepsAuthoredContracts(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"authored_inferred", "var media = getFile(nil)\nmedia = none\n"},
		{"authored_explicit", "var media: File = getFile(nil)\nmedia = none\n"},
		{"unknown_to_file", "var media: File? = getFile(nil)\nmedia = native.action(\"com.example.p15.opaque\", {})\n"},
		{"anycontent_to_file", "var media: File? = getFile(nil)\nmedia = ShortcutInput\n"},
		{"legacy_unknown_reassignment", "@media = getFile(nil)\n@media = nil\n@media = native.action(\"com.example.p15.opaque\", {})\n"},
		{"legacy_anycontent_reassignment", "@media = getFile(nil)\n@media = nil\n@media = ShortcutInput\n"},
		{"authored_shadow", "@value = 1\n@value = nil\nif true {\nvar value = 2\nvalue = nil\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			migrated := p15NilMigration(t, tc.source)
			raw, err := CompileSourceToPlist("p15-nil-contract.cherri", migrated)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
				t.Fatalf("migration weakened an authored/opaque contract: bytes=%d err=%v\n%s", len(raw), err, migrated)
			}
			if tc.name == "authored_shadow" {
				if !strings.Contains(migrated, "var value: Number?") || strings.Contains(migrated, "var value: Number? = 2") {
					t.Fatalf("Optional declaration crossed lexical identity:\n%s", migrated)
				}
			}
		})
	}
	for _, source := range []string{
		"@value = 1\n// @value = nil\n",
		"@value = 1\ntext(\"@value = nil\")\n",
		"@value = 1\n/*\n@value = nil\n*/\n",
		"@value = 1\n@value = 2\n",
	} {
		migrated := p15NilMigration(t, source)
		if binding := p15NilBinding(t, migrated, "value"); binding.TypeExpr != nil {
			t.Fatalf("non-clear text widened binding:\n%s", migrated)
		}
	}
}

func TestP15NilMigrationStructuredScopeEvidence(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"function", "function clearValue() {\n@value = 1\n@value = nil\n}\nclearValue()\n"},
		{"copy", "copy clearValue {\n@value = 1\n@value = nil\n}\npaste clearValue\n"},
		{"scoped_block", "@value = 1\nmeasure(\"clear-value\") {\n@value = nil\ntext(\"done\")\n}\n"},
		{"for_body", "@value = 1\nfor item in [1] {\n@value = nil\n}\n"},
		{"menu_body", "@value = 1\nmenu \"pick\" {\n    item \"clear\":\n        @value = nil\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			migrated := p15NilMigration(t, tc.source)
			if strings.Count(migrated, "var value: Number?") != 1 {
				t.Fatalf("wrong scoped Optional target:\n%s", migrated)
			}
			if tc.name == "copy" && (!strings.Contains(migrated, "copy clearValue") || !strings.Contains(migrated, "paste clearValue")) {
				t.Fatalf("analysis mutated the source-format tree:\n%s", migrated)
			}
			compileDebugFixture(t, migrated, false)
		})
	}
	for _, tc := range []struct{ name, source, inner string }{
		{"list", "@value = [1, 2]\n@value = nil\n", "List"},
		{"map", "@value = {\"item\": 1}\n@value = nil\n", "Map"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			migrated := p15NilMigration(t, tc.source)
			binding := p15NilBinding(t, migrated, "value")
			optional, ok := binding.TypeExpr.(*syntax.OptionalTypeAnnotation)
			if !ok {
				t.Fatalf("missing collection Optional:\n%s", migrated)
			}
			generic, ok := optional.Inner.(*syntax.GenericTypeAnnotation)
			if !ok || generic.Name != tc.inner {
				t.Fatalf("lost collection shape: %#v", optional.Inner)
			}
			actions := compileDebugFixture(t, migrated, false)
			if err := p15NilClearOracle(actions, "value"); err != nil {
				t.Fatal(err)
			}
		})
	}
	migrated := p15NilMigration(t, "@value = 1\ncopy unused {\n@value = nil\n}\ntext(\"done\")\n")
	if binding := p15NilBinding(t, migrated, "value"); binding.TypeExpr != nil {
		t.Fatalf("an unused copy block widened a runtime binding:\n%s", migrated)
	}
	compileDebugFixture(t, migrated, false)
}

func TestP15NilMigrationLoopAndParameterMask(t *testing.T) {
	for _, source := range []string{
		"@value = 1\nfor value in [2] {\nvalue = none\n}\n@value = 3\n",
		"@value = 1\nfunction mask(value: Number) -> Void {\nvalue = none\n}\n@value = 3\n",
	} {
		migrated := p15NilMigration(t, source)
		if binding := p15NilBinding(t, migrated, "value"); binding.TypeExpr != nil {
			t.Fatalf("masked assignment widened outer Legacy binding:\n%s", migrated)
		}
		raw, err := CompileSourceToPlist("p15-nil-mask.cherri", migrated)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeAssignImmutable) {
			t.Fatalf("masked immutable write was accepted: bytes=%d err=%v\n%s", len(raw), err, migrated)
		}
	}
}

func TestP15NilMigrationOriginalCLIParity(t *testing.T) {
	fixture := filepath.Join("tests", "nil.cherri")
	original, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		after, err := os.ReadFile(fixture)
		if err != nil || !bytes.Equal(original, after) {
			t.Errorf("canonical nil fixture changed: %v", err)
		}
	}()
	bin := roundTripBinary(t)
	migrated, stderr, err := p15RunCLI(t, bin, "", nil, "migrate", fixture)
	if err != nil {
		t.Fatalf("actual migration CLI failed: %v %s", err, stderr)
	}
	tmp := t.TempDir()
	v2Source := filepath.Join(tmp, "nil_v2.cherri")
	if err := os.WriteFile(v2Source, migrated, 0600); err != nil {
		t.Fatal(err)
	}
	legacyOut, v2Out := filepath.Join(tmp, "nil_legacy.shortcut"), filepath.Join(tmp, "nil_v2.shortcut")
	for _, args := range [][]string{
		{fixture, "--legacy", "--skip-sign", "--load-all-actions", "-o", legacyOut},
		{"compile", v2Source, "--skip-sign", "-o", v2Out},
	} {
		stdout, stderr, err := p15RunCLI(t, bin, "", nil, args...)
		if err != nil {
			t.Fatalf("actual compiler CLI failed: %v\n%s\n%s", err, stdout, stderr)
		}
	}
	result, err := shortcutcompare.CompareFilesNormalized(legacyOut, v2Out)
	if err != nil || !result.Equal {
		t.Fatalf("unchanged nil fixture differential: err=%v result=%+v", err, result)
	}
	t.Log("actual migrate + Legacy/V2 CLI paths preserve canonical nil fixture with existing normalized parity")
}
