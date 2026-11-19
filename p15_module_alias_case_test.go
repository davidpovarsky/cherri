package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

func TestP15IM01ModuleAliasCasePreservesNamespace(t *testing.T) {
	for _, tc := range []struct{ name, first, second, nested string }{
		{"lowercase_control", "first", "second", "nested"},
		{"uppercase_modules", "First", "Second", "Nested"},
		{"mixed_modules", "First", "second", "Nested"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, text := p15ModuleProject(t, map[string]string{
				"main.cherri":     fmt.Sprintf("import \"./a.cherri\" as %s\nimport \"./b.cherri\" as %s\nlet firstValue = %s.value()\nlet secondValue = %s.value()\n", tc.first, tc.second, tc.first, tc.second),
				"a.cherri":        fmt.Sprintf("import \"./sub/leaf.cherri\" as %s\nfunction value() -> Text { return %s.leaf() }\n", tc.nested, tc.nested),
				"b.cherri":        "function value() -> Text { return \"P15_MODULE_UPPER_B\" }\n",
				"sub/leaf.cherri": "function leaf() -> Text { return \"P15_MODULE_UPPER_LEAF\" }\n",
			})
			for _, diagnostic := range p15CheckModuleService(t, path, text) {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			native, err := CompileSourceToPlist(path, text)
			if err != nil {
				t.Fatalf("valid module alias case was rejected: %v", err)
			}
			got := p15ObserveModuleGraph(t, native)
			leaf, second := got.markerDispatch["P15_MODULE_UPPER_LEAF"], got.markerDispatch["P15_MODULE_UPPER_B"]
			if leaf == "" || second == "" || leaf == second || len(got.mainCalls) != 2 {
				t.Fatalf("module dispatch identities missing or collapsed: %+v", got)
			}
			first := got.mainCalls[0]
			if first == leaf || first == second || got.mainCalls[1] != second || !reflect.DeepEqual(got.bodyCalls[first], []string{leaf}) {
				t.Fatalf("alias case changed nested function binding: %+v", got)
			}
		})
	}
}

func TestP15IM01UppercaseModuleRejectsUnexportedCallable(t *testing.T) {
	path, text := p15ModuleProject(t, map[string]string{
		"main.cherri": "import \"./dep.cherri\" as Helper\nlet result = Helper.getText(\"not exported\")\n",
		"dep.cherri":  "function value() -> Text { return \"P15_EXPORTED_ONLY\" }\n",
	})
	diagnostics := p15CheckModuleService(t, path, text)
	found := false
	for _, diagnostic := range diagnostics {
		found = found || (diagnostic.Severity == analysis.SeverityError && diagnostic.Code == analysis.CodeUnknownAction && strings.Contains(diagnostic.Message, "does not export callable"))
	}
	if !found {
		t.Fatalf("service did not reject the actual missing module export: %v", diagnostics)
	}
	artifact, err := CompileSourceToPlist(path, text)
	if err == nil || !strings.Contains(err.Error(), "does not export callable") || len(artifact) != 0 {
		t.Fatalf("missing uppercase module export reached output: artifact=%d error=%v", len(artifact), err)
	}
}
