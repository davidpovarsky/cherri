package main

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestP15IM03PreservedStandardFunctionSource(t *testing.T) {
	raw, err := stdFuncs.ReadFile("stdfunc.cherri")
	if err != nil {
		t.Fatal(err)
	}
	if modules.StandardLibrarySource() != string(raw) {
		t.Fatal("module standard library is not the existing embedded upstream source")
	}
	path, text := p15ModuleProject(t, map[string]string{
		"main.cherri": "import stdfunc as std\nlet js=std.runJS(\"document.title\")\nlet selected=std.chooseFromVCard([\"One\",\"Two\"], prompt:\"Choose\")\n",
	})
	parser := syntax.NewParser(source.NewFile(source.DocumentID(path), path, 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	graph, issue := modules.Resolve(program, path, nil, schema.DefaultRegistry(), migrate.MigrateSource)
	if issue != nil {
		t.Fatal(issue)
	}
	var preserved *modules.Module
	for _, module := range graph.Modules {
		if module.Key == "builtin:stdfunc" {
			preserved = module
		}
	}
	if preserved == nil || preserved.SourceHash != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("actual imported module is not source-hash bound to preserved upstream bytes")
	}
	parameters := map[string][]string{}
	for _, declaration := range preserved.Program.Declarations {
		if fn, ok := declaration.(*syntax.FunctionDecl); ok {
			for _, parameter := range fn.Parameters {
				parameters[fn.Name] = append(parameters[fn.Name], parameter.Name)
			}
		}
	}
	if !reflect.DeepEqual(parameters, map[string][]string{"runJS": {"_script"}, "chooseFromVCard": {"_items", "_prompt"}}) {
		t.Fatalf("upstream declarations were replaced by a handwritten second library: %v", parameters)
	}
	native, err := CompileSourceToPlist(path, text)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"is.workflow.actions.text.replace", "is.workflow.actions.setitemname", "is.workflow.actions.choosefromlist", "cherri_chooseFromVCard.vcf"} {
		if !p15ModuleContains(t, native, marker) {
			t.Fatalf("preserved migration lost native structure %q", marker)
		}
	}
	for _, diagnostic := range p15CheckModuleService(t, path, text) {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
}
