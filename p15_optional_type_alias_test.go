package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"howett.net/plist"
)

// An alias changes the source spelling, not any native value/presence edge.
// The oracle still requires the complete independently authored function graph.
func TestP15OptionalTypeAliasNativePresence(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases() {
		t.Run(tc.name, func(t *testing.T) {
			source := "type Maybe = " + tc.typ + "?\n" + strings.ReplaceAll(p15FunctionOptionalSource(tc), tc.typ+"?", "Maybe")
			if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			if err := p15FunctionOptionalError(p15CompiledActions(t, source), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15OptionalTypeAliasChainsAndOrder(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases()[:2] {
		for _, form := range []struct {
			name, declarations, annotation string
			after                          bool
		}{
			{"chain_forward", "type Maybe = Later\ntype Later = Text?\n", "Maybe", false},
			{"late_declaration", "type Maybe = Text?\n", "Maybe", true},
			{"optional_alias_base", "type Base = Text\n", "Base?", false},
			{"optional_union_alias", "type Maybe = Text? | Number\n", "Maybe", false},
		} {
			t.Run(tc.name+"/"+form.name, func(t *testing.T) {
				source := strings.ReplaceAll(p15FunctionOptionalSource(tc), "Text?", form.annotation)
				if form.after {
					source += "\n" + form.declarations
				} else {
					source = form.declarations + source
				}
				if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
					t.Fatal(diagnostics)
				}
				if err := p15FunctionOptionalError(p15CompiledActions(t, source), tc); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func p15OptionalAliasModuleActions(t *testing.T, path, source, exactDispatch string) []map[string]any {
	t.Helper()
	path = filepath.Join(filepath.Dir(path), "p15-short-circuit.cherri")
	p15ModuleWrite(t, path, source)
	if diagnostics := p15CheckModuleService(t, path, source); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	raw, err := CompileSourceToPlist(path, source)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if _, err := plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	actions := []map[string]any{}
	for _, action := range document["WFWorkflowActions"].([]any) {
		actions = append(actions, action.(map[string]any))
	}
	if exactDispatch == "" {
		return actions
	}
	// Only the two exact source-authorized dispatch spellings are alpha names.
	// A wrong module, missing dispatch or extra call cannot be normalized away.
	changed := 0
	var rename func(any) any
	rename = func(value any) any {
		switch value := value.(type) {
		case string:
			if value == exactDispatch {
				changed++
				return "echo"
			}
		case map[string]any:
			for key, child := range value {
				value[key] = rename(child)
			}
		case []any:
			for i, child := range value {
				value[i] = rename(child)
			}
		}
		return value
	}
	for _, action := range actions {
		rename(action)
	}
	if changed != 2 {
		t.Fatalf("exact module dispatch must occur in definition and call: %d", changed)
	}
	return actions
}

func TestP15OptionalTypeAliasUsesDefiningModule(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases()[:2] {
		for _, importedFunction := range []bool{false, true} {
			name := "qualified_annotation"
			if importedFunction {
				name = "imported_function"
			}
			t.Run(tc.name+"/"+name, func(t *testing.T) {
				dependency := "type Maybe=Text?\n"
				entry := "import \"./dependency.cherri\" as dep\ntype Maybe=Number\n"
				dispatch := ""
				if importedFunction {
					dependency += "function echo(value:Maybe)->Maybe{return value}\n"
					entry += "getVariable(dep.echo(" + tc.argument + "))"
					dispatch = "module:dependency.cherri::echo"
				} else {
					entry += strings.ReplaceAll(p15FunctionOptionalSource(tc), "Text?", "dep.Maybe")
				}
				path, source := p15ModuleProject(t, map[string]string{"main.cherri": entry, "dependency.cherri": dependency})
				if err := p15FunctionOptionalError(p15OptionalAliasModuleActions(t, path, source, dispatch), tc); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15OptionalTypeAliasRejectsInvalidDeclarationsAndValues(t *testing.T) {
	for _, tc := range []struct{ name, source, code, detail string }{
		{"self_cycle", "type Maybe=Maybe?\ngetVariable(\"kept\")", "E_TYPE_ALIAS", "cyclic type alias"},
		{"mutual_cycle", "type Maybe=Later?\ntype Later=Maybe\ngetVariable(\"kept\")", "E_TYPE_ALIAS", "cyclic type alias"},
		{"duplicate", "type Maybe=Text?\ntype Maybe=Number?\ngetVariable(\"kept\")", "E_TYPE_ALIAS", "duplicate type alias"},
		{"wrong_value", "type Maybe=Text?\nfunction echo(value:Maybe)->Maybe{return value}\ngetVariable(echo(5))", analysis.CodeArgumentType, "expects Text?, got Number"},
		{"none_for_present_alias", "type Value=Text\nfunction echo(value:Value)->Value{return value}\ngetVariable(echo(none))", analysis.CodeArgumentType, "expects Text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			found := false
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError && diagnostic.Code == tc.code && strings.Contains(diagnostic.Message, tc.detail) {
					found = true
				}
			}
			if !found {
				t.Errorf("missing exact alias diagnostic %s/%s: %#v", tc.code, tc.detail, diagnostics)
			}
			raw, err := CompileSourceToPlist("p15-optional-type-alias.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) || !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("invalid alias emitted native bytes or wrong diagnostic: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}
