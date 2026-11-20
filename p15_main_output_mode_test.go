package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15M02Parse(t *testing.T, text string) *syntax.Program {
	t.Helper()
	parser := syntax.NewParser(source.NewFile("m02-mode", "m02-mode.cherri", 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal("mode fixture did not parse", parser.Errors())
	}
	return program
}

func TestP15M02ValidatedModeRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, header string
		want         syntax.MainOutputMode
	}{
		{"default", "", syntax.MainOutputExplicit},
		{"explicit", "shortcut \"\" { outputMode: \"explicit\" }\n", syntax.MainOutputExplicit},
		{"legacy", "shortcut \"\" { outputMode: \"legacy\" }\n", syntax.MainOutputLegacy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.header + "text(\"MODE_RESULT\")"
			program := p15M02Parse(t, text)
			mode, err := syntax.ResolveMainOutputMode(program)
			if err != nil || mode != tc.want {
				t.Fatal(mode, err)
			}
			analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
			analyzer.Analyze(program)
			for _, d := range analyzer.Diagnostics() {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			svc := service.NewService(schema.DefaultRegistry())
			svc.OpenDocument("m02-mode.cherri", 1, text)
			diagnostics, _ := svc.Analyze("m02-mode.cherri")
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			workflow, failure := lower.NewLowerer(schema.DefaultRegistry()).LowerProgram(program)
			if failure != nil || workflow == nil {
				t.Fatal(failure)
			}
			wantActions := 2
			if tc.want == syntax.MainOutputLegacy {
				wantActions = 1
			}
			if len(workflow.Actions) != wantActions {
				t.Fatal("direct lowering has different main contract", len(workflow.Actions))
			}
			if tc.want == syntax.MainOutputExplicit && workflow.Actions[1].AppleIdentifier != "is.workflow.actions.nothing" {
				t.Fatal("missing direct main boundary")
			}
			compiled, compileErr := CompileSourceToPlist("m02-mode.cherri", text)
			if compileErr != nil || len(compiled) == 0 {
				t.Fatal(compileErr)
			}
		})
	}
}

func TestP15M02InvalidModeFailsBeforeEmission(t *testing.T) {
	for _, tc := range []struct{ name, header string }{
		{"unknown", "shortcut \"\" { outputMode: \"unknown\" }"},
		{"wrong_case", "shortcut \"\" { outputMode: \"Legacy\" }"},
		{"empty", "shortcut \"\" { outputMode: \"\" }"},
		{"number", "shortcut \"\" { outputMode: 1 }"},
		{"boolean", "shortcut \"\" { outputMode: true }"},
		{"none", "shortcut \"\" { outputMode: none }"},
		{"dynamic", "shortcut \"\" { outputMode: chosenMode }"},
		{"call", "shortcut \"\" { outputMode: text(\"legacy\") }"},
		{"list", "shortcut \"\" { outputMode: [\"legacy\"] }"},
		{"duplicate_field", "shortcut \"\" { outputMode: \"explicit\", outputMode: \"legacy\" }"},
		{"duplicate_header", "shortcut \"\" { outputMode: \"legacy\" }\nshortcut \"\" { outputMode: \"legacy\" }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.header + "\ntext(\"M02_MUST_NOT_EMIT\")"
			program := p15M02Parse(t, text)
			_, modeError := syntax.ResolveMainOutputMode(program)
			if modeError == nil || modeError.Span.Start.Line < 1 || modeError.Span.Start.Column < 1 {
				t.Fatal("missing precise mode diagnostic", modeError)
			}
			analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
			analyzer.Analyze(program)
			svc := service.NewService(schema.DefaultRegistry())
			svc.OpenDocument("m02-mode.cherri", 1, text)
			serviceDiagnostics, _ := svc.Analyze("m02-mode.cherri")
			for name, diagnostics := range map[string][]analysis.Diagnostic{"analysis": analyzer.Diagnostics(), "service": serviceDiagnostics} {
				matches := 0
				for _, d := range diagnostics {
					if d.Code != analysis.CodeMainOutputMode {
						continue
					}
					matches++
					if d.Severity != analysis.SeverityError || d.Message != modeError.Message || d.Span.Start.Line != modeError.Span.Start.Line || d.Span.Start.Column != modeError.Span.Start.Column {
						t.Fatal(name, "mismatched mode diagnostic", d, modeError)
					}
				}
				if matches != 1 {
					t.Fatal(name, "missing or duplicate mode diagnostic", diagnostics)
				}
			}
			workflow, err := lower.NewLowerer(schema.DefaultRegistry()).LowerProgram(program)
			if workflow != nil || err == nil || !strings.Contains(err.Error(), analysis.CodeMainOutputMode) {
				t.Fatal("checked lowering accepted malformed mode or exposed partial output", workflow, err)
			}
			compiled, err := CompileSourceToPlist("m02-mode.cherri", text)
			if len(compiled) != 0 || err == nil || !strings.Contains(err.Error(), analysis.CodeMainOutputMode) {
				t.Fatal("public compiler emitted invalid-mode workflow", len(compiled), err)
			}
		})
	}
}

func TestP15M02MigrationUsesEntryContext(t *testing.T) {
	legacy := "function value() {\n output(7)\n}\n"
	entry, diagnostics, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	module := migrate.MigrateModuleSource(legacy)
	if !strings.Contains(entry, "outputMode: \"legacy\"") || strings.Contains(module, "outputMode") || strings.Contains(module, "shortcut ") {
		t.Fatal("migration entry/module context was lost", entry, module)
	}
	for _, content := range []string{legacy, module} {
		program := p15M02Parse(t, "import \"m02-library.cherri\" as lib\nlet value=lib.value()")
		_, issue := modules.Resolve(program, "m02-main.cherri", map[string]string{"m02-library.cherri": content}, schema.DefaultRegistry(), migrate.MigrateModuleSource)
		if issue != nil {
			t.Fatal("imported declaration source gained entry workflow metadata", issue)
		}
	}
	for _, legacy := range []string{"#define outputMode legacy\ntext(\"x\")", "#define outputMode\ntext(\"x\")"} {
		converted, diagnostics, err := migrate.MigrateSourceWithDiagnostics(legacy)
		if err == nil || converted != "" || len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "reserved V2 entry metadata") {
			t.Fatal("reserved Legacy mode collision was accepted", converted, diagnostics, err)
		}
		if migrate.MigrateModuleSource(legacy) != legacy {
			t.Fatal("failed module migration changed original source")
		}
	}
	// Runtime initialization is still validated in module context; the
	// presence of statements must never cause entry metadata injection.
	initialized := migrate.MigrateModuleSource("const effect = text(\"M02_IMPORT_EFFECT\")\n")
	if strings.Contains(initialized, "outputMode") || strings.Contains(initialized, "shortcut ") {
		t.Fatal("module with runtime statements gained entry metadata", initialized)
	}
	initializationCaller := p15M02Parse(t, "import \"m02-library.cherri\" as lib\ntext(\"caller\")")
	_, initializationIssue := modules.Resolve(initializationCaller, "m02-main.cherri", map[string]string{"m02-library.cherri": initialized}, schema.DefaultRegistry(), migrate.MigrateModuleSource)
	if initializationIssue == nil || !strings.Contains(fmt.Sprint(initializationIssue), "E_MODULE_INITIALIZATION") {
		t.Fatal("module runtime initialization crossed the existing boundary", initializationIssue)
	}
	// A genuine module header remains forbidden; imported source never gains
	// permission to change the caller's main boundary via its own metadata.
	program := p15M02Parse(t, "import \"m02-library.cherri\" as lib\ntext(\"caller\")")
	_, issue := modules.Resolve(program, "m02-main.cherri", map[string]string{"m02-library.cherri": "shortcut \"\" { outputMode: \"legacy\" }"}, schema.DefaultRegistry(), migrate.MigrateModuleSource)
	if issue == nil || !strings.Contains(fmt.Sprint(issue), "E_MODULE_INITIALIZATION") {
		t.Fatal("import changed main mode", issue)
	}
}

func TestP15M02MigrationIsIdempotent(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		mode        syntax.MainOutputMode
	}{
		{"empty", "", syntax.MainOutputLegacy},
		{"legacy_body", "text(\"retained\")", syntax.MainOutputLegacy},
		{"legacy_header", "#define name \"Retained\"\ntext(\"retained\")", syntax.MainOutputLegacy},
		{"existing_v2_header", "shortcut \"Retained\" {}\ntext(\"retained\")", syntax.MainOutputLegacy},
		{"existing_legacy_mode", "shortcut \"Retained\" { outputMode: \"legacy\" }\ntext(\"retained\")", syntax.MainOutputLegacy},
		{"existing_explicit_mode", "shortcut \"Retained\" { outputMode: \"explicit\" }\ntext(\"retained\")", syntax.MainOutputExplicit},
		{"decoy_comment", "// shortcut \"\" { outputMode: \"explicit\" }\ntext(\"retained\")", syntax.MainOutputLegacy},
		{"decoy_literal", "text(\"outputMode: explicit\")", syntax.MainOutputLegacy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, diagnostics, err := migrate.MigrateSourceWithDiagnostics(tc.input)
			if err != nil || len(diagnostics) != 0 {
				t.Fatal("first migration failed", err, diagnostics)
			}
			second, diagnostics, err := migrate.MigrateSourceWithDiagnostics(first)
			if err != nil || len(diagnostics) != 0 || second != first {
				t.Fatal("repeated migration changed canonical source", err, diagnostics, first, second)
			}
			program := p15M02Parse(t, second)
			mode, modeError := syntax.ResolveMainOutputMode(program)
			if modeError != nil || mode != tc.mode {
				t.Fatal("migration changed existing output intent", mode, modeError)
			}
			fields := 0
			for _, declaration := range program.Declarations {
				if header, ok := declaration.(*syntax.ShortcutDecl); ok && header.Metadata != nil {
					for _, field := range header.Metadata.Fields {
						if field.Name == "outputMode" {
							fields++
						}
					}
				}
			}
			if fields != 1 {
				t.Fatal("entry mode was missing or duplicated", fields)
			}
			if _, err := CompileSourceToPlist("m02-migrated.cherri", second); err != nil {
				t.Fatal("idempotent migration did not compile", err)
			}
		})
	}
	for _, input := range []string{"function value() {\n output(7)\n}\n", "fn value() -> Number { return 7 }"} {
		first := migrate.MigrateModuleSource(input)
		second := migrate.MigrateModuleSource(first)
		if second != first || strings.Contains(second, "outputMode") || strings.Contains(second, "shortcut ") {
			t.Fatal("repeated module conversion changed context", first, second)
		}
	}
}

func TestP15M02MigrationRejectsInvalidExistingMode(t *testing.T) {
	for _, input := range []string{
		"shortcut \"\" { outputMode: \"invalid\" }",
		"shortcut \"\" { outputMode: chosenMode }",
		"shortcut \"\" { outputMode: \"explicit\", outputMode: \"legacy\" }",
		"shortcut \"\" { outputMode: \"legacy\" }\nshortcut \"\" { outputMode: \"legacy\" }",
		"#define name \"Mixed\"\nshortcut \"\" { outputMode: chosenMode }\ntext(\"untouched\")",
	} {
		converted, diagnostics, err := migrate.MigrateSourceWithDiagnostics(input)
		if err == nil || converted != "" || len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "outputMode") {
			t.Fatal("migration accepted malformed existing mode", input, converted, diagnostics, err)
		}
		if migrate.MigrateSource(input) != input || migrate.MigrateModuleSource(input) != input {
			t.Fatal("failed conversion modified original input")
		}
	}
}
