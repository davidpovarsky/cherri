package main

import (
	"fmt"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

func p15ModuleCanonicalDocument(t *testing.T, path, text string) map[string]any {
	t.Helper()
	artifact, err := CompileSourceToPlist(path, text)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if _, err := plist.Unmarshal(artifact, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestP15IM01BuiltinModuleAliasesPreserveNativeParity(t *testing.T) {
	before := schema.DefaultRegistry().Fingerprint()
	for _, tc := range []struct{ action, call string }{
		{"url", "url(\"https://example.invalid/p15/module\")"},
		{"splitText", "splitText(\"LEFT RIGHT\")"},
		{"emailAddress", "emailAddress(\"p15@example.invalid\")"},
	} {
		definition, ok := schema.DefaultRegistry().LookupAction(tc.action)
		if !ok || definition.Module == "" {
			t.Fatalf("canonical fixture action %s has no real module", tc.action)
		}
		var expected map[string]any
		if !t.Run(tc.action+"/unqualified_control", func(t *testing.T) {
			path, text := p15ModuleProject(t, map[string]string{"main.cherri": "let result = " + tc.call + "\n"})
			expected = p15ModuleCanonicalDocument(t, path, text)
		}) {
			continue
		}
		for _, alias := range []string{"library", "Library"} {
			t.Run(tc.action+"/"+alias, func(t *testing.T) {
				code := fmt.Sprintf("import %q as %s\nlet result = %s.%s\n", definition.Module, alias, alias, tc.call)
				path, text := p15ModuleProject(t, map[string]string{"main.cherri": code})
				actual := p15ModuleCanonicalDocument(t, path, text)
				if diff := shortcutcompare.CompareDocumentsStrict(expected, actual); !diff.Equal {
					t.Fatalf("module alias changed canonical native output: %+v", diff.Differences)
				}
			})
		}
	}
	if schema.DefaultRegistry().Fingerprint() != before {
		t.Fatal("module aliases mutated the shared canonical registry")
	}
}

func TestP15IM01BuiltinModuleAliasesRetainSchemaIdentity(t *testing.T) {
	text := "import web as Web\nimport text as TextActions\nimport contacts as Contacts\nimport builtin as Builtins\n"
	path, _ := p15ModuleProject(t, map[string]string{"main.cherri": text})
	parser := syntax.NewParser(source.NewFile(source.DocumentID(path), path, 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	registry := schema.DefaultRegistry()
	graph, issue := modules.Resolve(program, path, nil, registry, migrate.MigrateModuleSource)
	if issue != nil {
		t.Fatal(issue)
	}
	analyzer := analysis.NewAnalyzer(registry)
	analyzer.SetModuleGraph(graph)
	analyzer.Analyze(program)
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
	aliases := map[string]string{"web": "Web", "text": "TextActions", "contacts": "Contacts", "builtin": "Builtins"}
	checked := 0
	for _, original := range registry.AllActions() {
		alias := aliases[original.Module]
		if alias == "" {
			continue
		}
		name := alias + "." + original.CallableName
		resolved, ok := analyzer.Registry().LookupAction(name)
		if !ok {
			t.Errorf("module export missing: %s", name)
			continue
		}
		if resolved.ID != original.ID || resolved.Variant != original.Variant || resolved.AppleIdentifier != original.AppleIdentifier || resolved.CallableName != name || resolved.Builtin != original.Builtin || resolved.Custom != original.Custom {
			t.Errorf("qualified callable %s lost its canonical schema identity: ID=%q original=%q", name, resolved.ID, original.ID)
		}
		checked++
	}
	if checked < 10 {
		t.Fatalf("expected real catalog coverage, only checked %d exported definitions", checked)
	}
}
