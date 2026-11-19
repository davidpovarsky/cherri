package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

type p15ModuleGraphObservation struct {
	markerDispatch map[string]string
	mainCalls      []string
	bodyCalls      map[string][]string
}

func p15ObserveModuleGraph(t *testing.T, raw []byte) p15ModuleGraphObservation {
	t.Helper()
	var document map[string]any
	if _, err := plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	actions, ok := document["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("missing native actions")
	}
	result := p15ModuleGraphObservation{markerDispatch: map[string]string{}, bodyCalls: map[string][]string{}}
	functionGetters := map[string]bool{}
	type frame struct{ group, dispatch string }
	var stack []frame
	for _, item := range actions {
		action := item.(map[string]any)
		p := p15C01Params(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && p["WFDictionaryKey"] == "function" {
			functionGetters[fmt.Sprint(p["UUID"])] = true
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
			group, _ := p["GroupingIdentifier"].(string)
			switch p15Int(p["WFControlFlowMode"]) {
			case 0:
				dispatch := ""
				outputs, _ := p15V02NativeReferences(p["WFInput"])
				if len(outputs) == 1 && functionGetters[outputs[0]] {
					if p15Int(p["WFCondition"]) != 4 {
						t.Fatal("function dispatcher no longer compares exact identity")
					}
					dispatch, _ = p["WFConditionalActionString"].(string)
					if dispatch == "" {
						t.Fatal("empty function dispatcher identity")
					}
				}
				stack = append(stack, frame{group, dispatch})
			case 1:
				if len(stack) == 0 || stack[len(stack)-1].group != group {
					t.Fatal("unmatched native else")
				}
				stack[len(stack)-1].dispatch = ""
			case 2:
				if len(stack) == 0 || stack[len(stack)-1].group != group {
					t.Fatal("unmatched native end")
				}
				stack = stack[:len(stack)-1]
			}
		}
		active := ""
		for _, scope := range stack {
			if scope.dispatch != "" {
				active = scope.dispatch
			}
		}
		if marker, ok := p["WFTextActionText"].(string); ok && strings.HasPrefix(marker, "P15_MODULE_") {
			if active == "" || result.markerDispatch[marker] != "" {
				t.Fatalf("marker %q has missing/duplicate function body", marker)
			}
			result.markerDispatch[marker] = active
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
			fields := p15LiteralDictionaryFields(t, p["WFItems"])
			if fields["_cherri_magic"] == nil {
				continue
			}
			value, ok := fields["function"]["WFValue"].(map[string]any)
			if !ok {
				t.Fatal("function dispatch is not a typed native text value")
			}
			text, ok := value["Value"].(map[string]any)
			if !ok {
				t.Fatal("function dispatch has no native text payload")
			}
			target, _ := text["string"].(string)
			if target == "" {
				t.Fatal("function call target missing")
			}
			if active == "" {
				result.mainCalls = append(result.mainCalls, target)
			} else {
				result.bodyCalls[active] = append(result.bodyCalls[active], target)
			}
		}
	}
	if len(stack) != 0 {
		t.Fatal("unclosed native control")
	}
	return result
}

func p15ModuleProject(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		p15ModuleWrite(t, filepath.Join(root, filepath.FromSlash(name)), text)
	}
	main := filepath.Join(root, "main.cherri")
	return main, files["main.cherri"]
}

func p15CheckModuleService(t *testing.T, path, text string) []analysis.Diagnostic {
	t.Helper()
	absolute := filepath.ToSlash(path)
	if !strings.HasPrefix(absolute, "/") {
		absolute = "/" + absolute
	}
	uri := (&url.URL{Scheme: "file", Path: absolute}).String()
	svc := service.NewService(nil)
	svc.OpenDocument(uri, 1, text)
	diagnostics, _ := svc.Analyze(uri)
	return diagnostics
}

func TestP15IM01ModuleNamespaceDispatch(t *testing.T) {
	for _, sameName := range []bool{false, true} {
		nameA, nameB := "valueA", "valueB"
		if sameName {
			nameA, nameB = "value", "value"
		}
		t.Run(fmt.Sprintf("same_export_name_%t", sameName), func(t *testing.T) {
			path, text := p15ModuleProject(t, map[string]string{
				"main.cherri": fmt.Sprintf("import \"./a.cherri\" as a\nimport \"./b.cherri\" as b\nshow(a.%s())\nshow(b.%s())\n", nameA, nameB),
				"a.cherri":    fmt.Sprintf("function %s() -> Text { return \"P15_MODULE_A\" }\n", nameA),
				"b.cherri":    fmt.Sprintf("function %s() -> Text { return \"P15_MODULE_B\" }\n", nameB),
			})
			raw, err := CompileSourceToPlist(path, text)
			if err != nil {
				t.Fatal(err)
			}
			graph := p15ObserveModuleGraph(t, raw)
			a, b := graph.markerDispatch["P15_MODULE_A"], graph.markerDispatch["P15_MODULE_B"]
			if a == "" || b == "" || a == b {
				t.Fatalf("distinct module bodies lost their identities: A=%q B=%q", a, b)
			}
			if !reflect.DeepEqual(graph.mainCalls, []string{a, b}) {
				t.Fatalf("module aliases call the wrong native function bodies: %v, want %v", graph.mainCalls, []string{a, b})
			}
		})
	}
}

func TestP15IM01NestedModuleNamespaceIsolation(t *testing.T) {
	path, text := p15ModuleProject(t, map[string]string{
		"main.cherri":    "import \"./a/main.cherri\" as a\nimport \"./b/main.cherri\" as b\nshow(a.outer())\nshow(b.outer())\n",
		"a/main.cherri":  "import \"./value.cherri\" as local\nfunction outer() -> Text { text(\"P15_MODULE_OUTER_A\"); return local.value() }\n",
		"b/main.cherri":  "import \"./value.cherri\" as local\nfunction outer() -> Text { text(\"P15_MODULE_OUTER_B\"); return local.value() }\n",
		"a/value.cherri": "function value() -> Text { return \"P15_MODULE_INNER_A\" }\n",
		"b/value.cherri": "function value() -> Text { return \"P15_MODULE_INNER_B\" }\n",
	})
	raw, err := CompileSourceToPlist(path, text)
	if err != nil {
		t.Fatal(err)
	}
	graph := p15ObserveModuleGraph(t, raw)
	a, b := graph.markerDispatch["P15_MODULE_OUTER_A"], graph.markerDispatch["P15_MODULE_OUTER_B"]
	innerA, innerB := graph.markerDispatch["P15_MODULE_INNER_A"], graph.markerDispatch["P15_MODULE_INNER_B"]
	if a == "" || b == "" || innerA == "" || innerB == "" || a == b || innerA == innerB {
		t.Fatalf("module body identities missing/colliding: %+v", graph.markerDispatch)
	}
	if !reflect.DeepEqual(graph.mainCalls, []string{a, b}) ||
		!reflect.DeepEqual(graph.bodyCalls[a], []string{innerA}) ||
		!reflect.DeepEqual(graph.bodyCalls[b], []string{innerB}) {
		t.Fatalf("nested aliases escaped their defining module: %+v", graph)
	}
}

func TestP15IM01CanonicalRelativeModuleIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, nestedImport, leafPath string
	}{
		{"same_spelling_distinct_files", "./next/module.cherri", "next/next/module.cherri"},
		{"ordinary_nested_control", "./leaf.cherri", "next/leaf.cherri"},
		{"sibling_inside_entry_workspace", "../sibling.cherri", "sibling.cherri"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, text := p15ModuleProject(t, map[string]string{
				"main.cherri":        "import \"./next/module.cherri\" as chain\nshow(chain.outer())\n",
				"next/module.cherri": fmt.Sprintf("import %q as leaf\nfunction outer() -> Text { return leaf.value() }\n", tc.nestedImport),
				tc.leafPath:          "function value() -> Text { return \"P15_MODULE_LEAF\" }\n",
			})
			raw, err := CompileSourceToPlist(path, text)
			if err != nil {
				t.Fatalf("distinct authorized module files were rejected: %v", err)
			}
			if !p15ModuleContains(t, raw, "P15_MODULE_LEAF") {
				t.Fatal("resolved module did not supply its native body")
			}
			for _, diagnostic := range p15CheckModuleService(t, path, text) {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatalf("service rejected authorized relative module: %+v", diagnostic)
				}
			}
		})
	}
}

func TestP15IM01ModuleServiceResolvesActualDependencies(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		files      map[string]string
	}{
		{"missing", "cannot resolve module", map[string]string{"main.cherri": "import \"./missing.cherri\" as missing\n"}},
		{"cycle", "import cycle detected", map[string]string{"main.cherri": "import \"./a.cherri\" as a\n", "a.cherri": "import \"./b.cherri\" as b\n", "b.cherri": "import \"./a.cherri\" as a\n"}},
		{"runtime_initialization", "module top-level runtime initialization rejected", map[string]string{"main.cherri": "import \"./dep.cherri\" as dep\n", "dep.cherri": "let value = text(\"P15_MODULE_SIDE_EFFECT\")\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, text := p15ModuleProject(t, tc.files)
			raw, err := CompileSourceToPlist(path, text)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("compile did not fail closed for actual dependency: %v, bytes=%d", err, len(raw))
			}
			found := false
			for _, diagnostic := range p15CheckModuleService(t, path, text) {
				if diagnostic.Severity == analysis.SeverityError && strings.Contains(diagnostic.Message, tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("service silently accepted %s dependency", tc.name)
			}
		})
	}
}

func TestP15IM05RemoteImportsNeverFetch(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write([]byte("function marker() -> Text { return \"P15_MODULE_REMOTE\" }\n"))
	}))
	defer server.Close()
	path, text := p15ModuleProject(t, map[string]string{"main.cherri": fmt.Sprintf("import %q as remote\n", server.URL+"/module.cherri")})
	raw, err := CompileSourceToPlist(path, text)
	if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "remote package imports not permitted") {
		t.Fatalf("remote compile was not rejected: %v", err)
	}
	found := false
	for _, diagnostic := range p15CheckModuleService(t, path, text) {
		if diagnostic.Severity == analysis.SeverityError && strings.Contains(diagnostic.Message, "remote package imports not permitted") {
			found = true
		}
	}
	if !found || requests.Load() != 0 {
		t.Fatalf("remote analysis rejection=%t, actual HTTP requests=%d", found, requests.Load())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "module.cherri")); !os.IsNotExist(err) {
		t.Fatal("remote import created a local installation artifact")
	}
}

func TestP15IM01DuplicateImportAndLocalNamesStayDistinct(t *testing.T) {
	path, text := p15ModuleProject(t, map[string]string{
		"main.cherri": "import \"./dep.cherri\" as first\nimport \"./folder/../dep\" as second\nfunction value() -> Text { return \"P15_MODULE_MAIN\" }\nshow(first.value())\nshow(second.value())\nshow(value())\n",
		"dep.cherri":  "function value() -> Text { return \"P15_MODULE_DEP\" }\n",
	})
	raw, err := CompileSourceToPlist(path, text)
	if err != nil {
		t.Fatal(err)
	}
	graph := p15ObserveModuleGraph(t, raw)
	dependency, main := graph.markerDispatch["P15_MODULE_DEP"], graph.markerDispatch["P15_MODULE_MAIN"]
	if dependency == "" || main == "" || dependency == main || len(graph.markerDispatch) != 2 {
		t.Fatalf("duplicate imports or local names altered function identity: %+v", graph)
	}
	if !reflect.DeepEqual(graph.mainCalls, []string{dependency, dependency, main}) {
		t.Fatalf("canonical duplicate aliases/local name target wrong bodies: %v", graph.mainCalls)
	}
}

func TestP15IM01ModuleServiceRefreshAndExports(t *testing.T) {
	path, text := p15ModuleProject(t, map[string]string{
		"main.cherri": "import \"./dep.cherri\" as dep\nshow(dep.value())\n",
		"dep.cherri":  "function value() -> Text { return \"P15_MODULE_INITIAL\" }\n",
	})
	absolute := filepath.ToSlash(path)
	if !strings.HasPrefix(absolute, "/") {
		absolute = "/" + absolute
	}
	uri := (&url.URL{Scheme: "file", Path: absolute}).String()
	svc := service.NewService(nil)
	svc.OpenDocument(uri, 1, text)
	assertDiagnostic := func(want string) {
		t.Helper()
		diagnostics, _ := svc.Analyze(uri)
		var errors []string
		for _, d := range diagnostics {
			if d.Severity == analysis.SeverityError {
				errors = append(errors, d.Message)
			}
		}
		if want == "" && len(errors) != 0 {
			t.Fatalf("authorized dependencies failed: %v", errors)
		}
		if want != "" && !strings.Contains(strings.Join(errors, "\n"), want) {
			t.Fatalf("stale/missing dependency diagnostic %q: %v", want, errors)
		}
	}
	assertDiagnostic("")
	p15ModuleWrite(t, filepath.Join(filepath.Dir(path), "dep.cherri"), "let value=text(\"P15_MODULE_INVALID_INIT\")\n")
	assertDiagnostic("module top-level runtime initialization rejected")
	p15ModuleWrite(t, filepath.Join(filepath.Dir(path), "dep.cherri"), "function other() -> Text { return \"P15_MODULE_OTHER\" }\n")
	assertDiagnostic("does not export callable")
	p15ModuleWrite(t, filepath.Join(filepath.Dir(path), "dep.cherri"), "function value() -> Text { return \"P15_MODULE_RESTORED\" }\n")
	assertDiagnostic("")
	dependencyPath := filepath.ToSlash(filepath.Join(filepath.Dir(path), "dep.cherri"))
	if !strings.HasPrefix(dependencyPath, "/") {
		dependencyPath = "/" + dependencyPath
	}
	dependencyURI := (&url.URL{Scheme: "file", Path: dependencyPath}).String()
	svc.OpenDocument(dependencyURI, 1, "let value=text(\"P15_MODULE_OVERLAY_INIT\")\n")
	assertDiagnostic("module top-level runtime initialization rejected")
	svc.CloseDocument(dependencyURI)
	assertDiagnostic("")
	for _, call := range []string{"dep.missing()", "dep.getText(\"not an export\")"} {
		bad := "import \"./dep.cherri\" as dep\n" + call + "\n"
		raw, err := CompileSourceToPlist(path, bad)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "does not export callable") {
			t.Fatalf("unknown module export fell back to a global callable: %v", err)
		}
	}
}
