package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

type p15RP16ApprovedCase struct {
	Role    string `json:"role"`
	Kind    string `json:"kind"`
	Source  string `json:"source"`
	Witness string `json:"witness"`
	Value   string `json:"value"`
}

func p15RP16Cases(t *testing.T) []p15RP16ApprovedCase {
	t.Helper()
	var raw, err = os.ReadFile("tests/language-v2/rp16-approved-nodes.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []p15RP16ApprovedCase
	var decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}
func p15RP16DeclaredKinds(t *testing.T) []string {
	t.Helper()
	var file, err = goparser.ParseFile(gotoken.NewFileSet(), "internal/language/syntax/ast.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var roles = map[string]string{"declarationNode": "declaration", "statementNode": "statement", "expressionNode": "expression", "typeAnnotationNode": "type_annotation"}
	var kinds []string
	for _, declaration := range file.Decls {
		var method, ok = declaration.(*goast.FuncDecl)
		if !ok || method.Recv == nil || len(method.Recv.List) != 1 {
			continue
		}
		var role, recognized = roles[method.Name.Name]
		if !recognized {
			continue
		}
		var pointer, pointerOK = method.Recv.List[0].Type.(*goast.StarExpr)
		if !pointerOK {
			t.Fatalf("AST marker has unexpected receiver: %s", method.Name.Name)
		}
		var receiver, named = pointer.X.(*goast.Ident)
		if !named {
			t.Fatal("AST marker is not a named node")
		}
		kinds = append(kinds, role+"/"+receiver.Name)
	}
	sort.Strings(kinds)
	return kinds
}
func p15RP16ValidateCaseInventory(cases []p15RP16ApprovedCase, declared []string) error {
	var actual = make([]string, 0, len(cases))
	for _, fixture := range cases {
		if fixture.Source == "" || fixture.Witness == "" {
			return fmt.Errorf("incomplete fixture %s/%s", fixture.Role, fixture.Kind)
		}
		actual = append(actual, fixture.Role+"/"+fixture.Kind)
	}
	sort.Strings(actual)
	if !reflect.DeepEqual(actual, declared) {
		return fmt.Errorf("approved role inventory differs: fixtures=%v declaration markers=%v", actual, declared)
	}
	return nil
}
func p15RP16ObservedKinds(program *syntax.Program) (map[string]int, error) {
	var observed = map[string]int{}
	var roleTypes = map[string]reflect.Type{
		"declaration":     reflect.TypeOf((*syntax.Declaration)(nil)).Elem(),
		"statement":       reflect.TypeOf((*syntax.Statement)(nil)).Elem(),
		"expression":      reflect.TypeOf((*syntax.Expression)(nil)).Elem(),
		"type_annotation": reflect.TypeOf((*syntax.TypeAnnotation)(nil)).Elem(),
	}
	var record = func(value reflect.Value, role string) error {
		var matches []string
		for candidate, iface := range roleTypes {
			if value.Type().Implements(iface) {
				matches = append(matches, candidate)
			}
		}
		if len(matches) == 0 {
			return nil
		}
		if role == "" {
			if len(matches) != 1 {
				return fmt.Errorf("ambiguous node role for %s", value.Type())
			}
			role = matches[0]
		}
		if !value.Type().Implements(roleTypes[role]) {
			return fmt.Errorf("node %s does not implement %s", value.Type(), role)
		}
		var typ = value.Type()
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		observed[role+"/"+typ.Name()]++
		return nil
	}
	var walk func(reflect.Value, string, bool) error
	walk = func(value reflect.Value, role string, counted bool) error {
		if !value.IsValid() {
			return nil
		}
		if value.Kind() == reflect.Interface {
			if value.IsNil() {
				return nil
			}
			for candidate, iface := range roleTypes {
				if value.Type() == iface {
					role = candidate
				}
			}
			return walk(value.Elem(), role, false)
		}
		switch value.Kind() {
		case reflect.Pointer:
			if value.IsNil() {
				return nil
			}
			if err := record(value, role); err != nil {
				return err
			}
			return walk(value.Elem(), "", true)
		case reflect.Struct:
			if !counted && value.CanAddr() {
				if err := record(value.Addr(), role); err != nil {
					return err
				}
			}
			for index := 0; index < value.NumField(); index++ {
				if err := walk(value.Field(index), "", false); err != nil {
					return err
				}
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				if err := walk(value.Index(index), role, false); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var err = walk(reflect.ValueOf(program), "", false)
	return observed, err
}

const p15RP16VirtualModule = "action marker(value: Text): Text { identifier: \"is.workflow.actions.gettext\" primary: \"value\" bindings: {value: {key: \"WFTextActionText\"}} }\n"

func p15RP16CompileApproved(t *testing.T, fixture p15RP16ApprovedCase) map[string]any {
	t.Helper()
	var filename = "rp16-approved.cherri"
	var parser = syntax.NewParser(source.NewFile("rp16-approved", filename, 1, fixture.Source))
	var program = parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatalf("approved node source parse failed: %v", parser.Errors())
	}
	var observed, err = p15RP16ObservedKinds(program)
	if err != nil {
		t.Fatal(err)
	}
	if observed[fixture.Role+"/"+fixture.Kind] == 0 {
		t.Fatalf("source does not contain actual %s/%s in its intended role: %v", fixture.Role, fixture.Kind, observed)
	}
	var virtual = map[string]string{"rp16-module.cherri": p15RP16VirtualModule}
	var registry = schema.DefaultRegistry()
	var graph, issue = modules.Resolve(program, filename, virtual, registry, migrate.MigrateSource)
	if issue != nil {
		t.Fatal(issue)
	}
	var analyzer = analysis.NewAnalyzer(registry)
	analyzer.SetModuleGraph(graph)
	analyzer.Analyze(program)
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatalf("approved node analysis failed: %+v", diagnostic)
		}
	}
	var lowerer = lower.NewLowerer(analyzer.Registry())
	lowerer.SetBaseFilePath(filename)
	lowerer.SetModuleGraph(graph)
	lowerer.SetModuleRegistries(analyzer.ModuleRegistries())
	lowerer.SetExprTypes(analyzer.ExprTypes())
	lowerer.SetEnumValues(analyzer.EnumValues())
	var workflow, lowerErr = lowerer.LowerProgram(program)
	if lowerErr != nil || workflow == nil {
		t.Fatalf("approved node checked lowering failed: %v", lowerErr)
	}
	if fixture.Witness == "metadata" && workflow.Name != fixture.Value {
		t.Fatal("Shortcut declaration name was dropped during checked lowering")
	}
	var raw, compileErr = CompileSourceToPlistWithModules(filename, fixture.Source, virtual)
	if compileErr != nil {
		t.Fatalf("approved node canonical compile failed: %v", compileErr)
	}
	var native map[string]any
	if _, err = plist.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	return native
}
func TestP15RP16ApprovedNodeInventory(t *testing.T) {
	var cases = p15RP16Cases(t)
	if err := p15RP16ValidateCaseInventory(cases, p15RP16DeclaredKinds(t)); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range cases {
		t.Run(fixture.Role+"_"+fixture.Kind, func(t *testing.T) {
			var native = p15RP16CompileApproved(t, fixture)
			if err := p15RP16NativeWitness(fixture, native); err != nil {
				t.Fatal(err)
			}
			var encoded, err = plist.Marshal(native, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			var corrupted map[string]any
			if _, err = plist.Unmarshal(encoded, &corrupted); err != nil {
				t.Fatal(err)
			}
			if !p15RP16CorruptWitness(fixture, corrupted) || reflect.DeepEqual(native, corrupted) {
				t.Fatal("native causal control did not change the observed witness")
			}
			if p15RP16NativeWitness(fixture, corrupted) == nil {
				t.Fatal("native witness accepted its causal corruption")
			}
		})
	}
}
func TestP15RP16InventoryControls(t *testing.T) {
	var cases = p15RP16Cases(t)
	var declared = p15RP16DeclaredKinds(t)
	if err := p15RP16ValidateCaseInventory(cases, declared); err != nil {
		t.Fatal(err)
	}
	for index, fixture := range cases {
		t.Run("missing_"+fixture.Role+"_"+fixture.Kind, func(t *testing.T) {
			var missing = append([]p15RP16ApprovedCase(nil), cases[:index]...)
			missing = append(missing, cases[index+1:]...)
			if p15RP16ValidateCaseInventory(missing, declared) == nil {
				t.Fatal("missing approved kind accepted")
			}
		})
	}
	for _, name := range []string{"duplicate_kind", "unknown_kind", "wrong_role"} {
		t.Run(name, func(t *testing.T) {
			var changed = append([]p15RP16ApprovedCase(nil), cases...)
			switch name {
			case "duplicate_kind":
				changed = append(changed, cases[0])
			case "unknown_kind":
				changed[0].Kind = "UnknownNode"
			case "wrong_role":
				changed[0].Role = "expression"
			}
			if p15RP16ValidateCaseInventory(changed, declared) == nil {
				t.Fatal("false approved inventory accepted")
			}
		})
	}
}
