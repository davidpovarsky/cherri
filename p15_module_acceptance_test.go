package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15AssertModuleRejection(t *testing.T, path, text, code, message string) {
	t.Helper()
	raw, err := CompileSourceToPlist(path, text)
	if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "["+code+"]") || !strings.Contains(err.Error(), message) {
		t.Fatalf("module compile did not reject the exact cause: error=%v artifact_bytes=%d", err, len(raw))
	}
	if strings.Contains(err.Error(), filepath.Dir(path)) {
		t.Fatal("module diagnostic disclosed the absolute workspace path")
	}
	found := false
	for _, diagnostic := range p15CheckModuleService(t, path, text) {
		if diagnostic.Severity == analysis.SeverityError && string(diagnostic.Code) == code && strings.Contains(diagnostic.Message, message) {
			found = true
		}
		if strings.Contains(diagnostic.Message, filepath.Dir(path)) {
			t.Fatal("service diagnostic disclosed the absolute workspace path")
		}
	}
	if !found {
		t.Fatalf("service omitted the exact module diagnostic %s: %s", code, message)
	}
}

func TestP15IM01CyclePathAndDuplicateAliasDiagnostics(t *testing.T) {
	t.Run("cycle_reports_canonical_relative_chain", func(t *testing.T) {
		path, text := p15ModuleProject(t, map[string]string{
			"main.cherri": "import \"./a.cherri\" as a\n",
			"a.cherri":    "import \"./sub/../b.cherri\" as b\n",
			"b.cherri":    "import \"./a\" as a\n",
		})
		p15AssertModuleRejection(t, path, text, "E_IMPORT_CYCLE", "main.cherri -> a.cherri -> b.cherri -> a.cherri")
	})
	t.Run("duplicate_alias_rejected", func(t *testing.T) {
		path, text := p15ModuleProject(t, map[string]string{
			"main.cherri": "import \"./a.cherri\" as same\nimport \"./b.cherri\" as same\n",
			"a.cherri":    "function value() -> Text { return \"P15_MODULE_A\" }\n",
			"b.cherri":    "function value() -> Text { return \"P15_MODULE_B\" }\n",
		})
		p15AssertModuleRejection(t, path, text, "E_DUPLICATE_NAME", "duplicate import alias \"same\"")
	})
}

func TestP15IM02RejectsModuleRuntimeInitialization(t *testing.T) {
	for name, body := range map[string]string{
		"binding": "let value = text(\"P15_MODULE_INIT_BINDING\")\n",
		"action":  "text(\"P15_MODULE_INIT_ACTION\")\n",
		"control": "if true { text(\"P15_MODULE_INIT_CONTROL\") }\n",
	} {
		t.Run(name, func(t *testing.T) {
			path, text := p15ModuleProject(t, map[string]string{
				"main.cherri":       "import \"./dependency.cherri\" as dep\n",
				"dependency.cherri": body,
			})
			p15AssertModuleRejection(t, path, text, "E_MODULE_INITIALIZATION", "module top-level runtime initialization rejected")
		})
	}
	t.Run("function_body_is_not_module_initialization", func(t *testing.T) {
		path, text := p15ModuleProject(t, map[string]string{
			"main.cherri":       "import \"./dependency.cherri\" as dep\nlet value=dep.value()\n",
			"dependency.cherri": "function value() -> Text { return \"P15_MODULE_FUNCTION_CONTROL\" }\n",
		})
		raw, err := CompileSourceToPlist(path, text)
		if err != nil || !p15ModuleContains(t, raw, "P15_MODULE_FUNCTION_CONTROL") {
			t.Fatalf("legitimate module function body was rejected/lost: %v", err)
		}
		for _, diagnostic := range p15CheckModuleService(t, path, text) {
			if diagnostic.Severity == analysis.SeverityError {
				t.Fatal(diagnostic)
			}
		}
	})
}

func TestP15IM02LegacyMigrationDoesNotDropInitialization(t *testing.T) {
	legacyFunction := "function value(text item) {\n output(\"P15_MODULE_LEGACY_CONTROL\")\n}\n"
	for name, initializer := range map[string]string{
		"binding":               "const leaked = text(\"P15_MODULE_LEGACY_INIT\")\n",
		"action":                "text(\"P15_MODULE_LEGACY_INIT\")\n",
		"function_only_control": "",
	} {
		t.Run(name, func(t *testing.T) {
			legacy := "#include 'actions/text'\n" + legacyFunction + initializer
			before := syntax.NewParser(source.NewFile("legacy.cherri", "legacy.cherri", 1, legacy))
			before.ParseProgram()
			if len(before.Errors()) == 0 {
				t.Fatal("legacy fixture no longer exercises the migration path")
			}
			migrated := migrate.MigrateSource(legacy)
			after := syntax.NewParser(source.NewFile("legacy.cherri", "legacy.cherri", 1, migrated))
			program := after.ParseProgram()
			if len(after.Errors()) != 0 {
				t.Fatalf("legacy fixture migration did not parse: %v", after.Errors())
			}
			path, text := p15ModuleProject(t, map[string]string{
				"main.cherri":   "import \"./legacy.cherri\" as dep\nlet value=dep.value(\"unused\")\n",
				"legacy.cherri": legacy,
			})
			if initializer != "" {
				if !strings.Contains(migrated, "P15_MODULE_LEGACY_INIT") || len(program.Statements) == 0 {
					t.Fatal("migration silently dropped top-level runtime initialization")
				}
				p15AssertModuleRejection(t, path, text, "E_MODULE_INITIALIZATION", "module top-level runtime initialization rejected")
				return
			}
			if len(program.Statements) != 0 {
				t.Fatal("function-only migration invented top-level initialization")
			}
			raw, err := CompileSourceToPlist(path, text)
			if err != nil || !p15ModuleContains(t, raw, "P15_MODULE_LEGACY_CONTROL") {
				t.Fatalf("definition-only legacy module was rejected/lost: %v", err)
			}
			for _, diagnostic := range p15CheckModuleService(t, path, text) {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
		})
	}
}

func TestP15IM03DirectServiceWithoutMain(t *testing.T) {
	// A separate package invocation is essential: this test binary imports main,
	// but the actual service producer must obtain stdfunc without main's init.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	const required = "TestP15IM03DirectServiceStandardLibrary"
	command := []string{"test", "-p=1", "-count=1", "-json", "-timeout=60s", "./internal/language/service", "-run", "^" + required + "$"}
	cmd := exec.CommandContext(ctx, "go", command...)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		if len(raw) > 8192 {
			raw = raw[len(raw)-8192:]
		}
		t.Fatalf("direct service package failed: %v\n%s", err, raw)
	}
	found := false
	for _, line := range strings.Split(string(raw), "\n") {
		var event struct{ Action, Package, Test string }
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Action == "fail" || event.Action == "skip" {
			t.Fatalf("direct service producer failed/skipped: %+v", event)
		}
		if event.Package == "github.com/electrikmilk/cherri/internal/language/service" && event.Test == required && event.Action == "pass" {
			found = true
		}
	}
	if !found {
		t.Fatal("direct service test was not discovered and passed")
	}
	t.Logf("Executed separate package without main: go %s", strings.Join(command, " "))
}

func TestP15IM05UnresolvedLocalDependencyNoInstallation(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprintf("dependency_exists_%t", exists), func(t *testing.T) {
			files := map[string]string{
				"main.cherri": "import \"./dependency.cherri\" as dep\nlet value=dep.value()\n",
			}
			if exists {
				files["dependency.cherri"] = "function value() -> Text { return \"P15_MODULE_LOCAL_CONTROL\" }\n"
			}
			path, text := p15ModuleProject(t, files)
			before, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			names := func(entries []os.DirEntry) []string {
				var result []string
				for _, entry := range entries {
					result = append(result, entry.Name())
				}
				return result
			}
			if !exists {
				p15AssertModuleRejection(t, path, text, "E_MODULE_DEPENDENCY", "cannot resolve module \"./dependency.cherri\": file not found")
			} else {
				raw, err := CompileSourceToPlist(path, text)
				if err != nil || !p15ModuleContains(t, raw, "P15_MODULE_LOCAL_CONTROL") {
					t.Fatalf("local dependency control failed: %v", err)
				}
				for _, diagnostic := range p15CheckModuleService(t, path, text) {
					if diagnostic.Severity == analysis.SeverityError {
						t.Fatal(diagnostic)
					}
				}
			}
			after, err := os.ReadDir(filepath.Dir(path))
			if err != nil || !reflect.DeepEqual(names(before), names(after)) {
				t.Fatalf("module resolution created/deleted installation files: %v", err)
			}
			for name, original := range files {
				actual, err := os.ReadFile(filepath.Join(filepath.Dir(path), name))
				if err != nil || string(actual) != original {
					t.Fatalf("module resolution mutated source %q: %v", name, err)
				}
			}
		})
	}
}
