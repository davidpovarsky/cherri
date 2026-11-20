package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
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
		{"self_cycle", "type Maybe=Maybe?\ntext(\"kept\")", "E_TYPE_ALIAS", "cyclic type alias"},
		{"mutual_cycle", "type Maybe=Later?\ntype Later=Maybe\ntext(\"kept\")", "E_TYPE_ALIAS", "cyclic type alias"},
		{"duplicate", "type Maybe=Text?\ntype Maybe=Number?\ntext(\"kept\")", "E_TYPE_ALIAS", "duplicate type alias"},
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
			svc := service.NewService(nil)
			uri := "file:///p15-alias-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			editor, _ := svc.Analyze(uri)
			found = false
			for _, diagnostic := range editor {
				if diagnostic.Severity == analysis.SeverityError && diagnostic.Code == tc.code && strings.Contains(diagnostic.Message, tc.detail) {
					found = true
				}
			}
			if !found {
				t.Errorf("editor lost exact alias diagnostic %s/%s: %#v", tc.code, tc.detail, editor)
			}
			raw, err := CompileSourceToPlist("p15-optional-type-alias.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) || !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("invalid alias emitted native bytes or wrong diagnostic: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}

func TestP15OptionalTypeAliasAnalyzerReuse(t *testing.T) {
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	previous := analyzer.TypeAnnotations()
	for _, tc := range []struct {
		name, target, argument string
		invalid                bool
	}{
		{"first", "Text?", "none", false},
		{"changed", "Number?", "\"\"", true},
		{"restored", "Text?", "\"\"", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "type Maybe=" + tc.target + "\nfunction echo(value:Maybe)->Maybe{return value}\ngetVariable(echo(" + tc.argument + "))"
			program, errors := p15ParseProgram(source)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			analyzer.Analyze(program)
			diagnostics := analyzer.Diagnostics()
			if tc.invalid {
				if len(diagnostics) != 1 || diagnostics[0].Code != analysis.CodeArgumentType || !strings.Contains(diagnostics[0].Message, "expects Number?") {
					t.Fatal(diagnostics)
				}
			} else if len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			current := analyzer.TypeAnnotations()
			if len(current) == 0 {
				t.Fatal("missing actual annotation results")
			}
			for annotation := range previous {
				if _, retained := current[annotation]; retained {
					t.Fatal("another program's type annotation leaked into this result")
				}
			}
			previous = current
		})
	}
}

func TestP15OptionalTypeAliasModuleRejectsCallerType(t *testing.T) {
	path, source := p15ModuleProject(t, map[string]string{
		"main.cherri":       "import \"./dependency.cherri\" as dep\ntype Maybe=Number\ngetVariable(dep.echo(5))",
		"dependency.cherri": "type Maybe=Text?\nfunction echo(value:Maybe)->Maybe{return value}\n",
	})
	// This is a semantic error in the caller's argument, not an import-loader
	// path diagnostic. Require its exact caller span and defining-module type.
	const detail = "expects Text?, got Number"
	raw, err := CompileSourceToPlist(path, source)
	if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) || !strings.Contains(err.Error(), detail) {
		t.Fatalf("caller alias replaced imported function's type: bytes=%d error=%v", len(raw), err)
	}
	diagnostics := p15CheckModuleService(t, path, source)
	if err := p15ExactLiteralDiagnostic(diagnostics, source, "5", analysis.CodeArgumentType); err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, detail) {
		t.Fatal(diagnostics)
	}
}

func TestP15OptionalTypeAliasOracleRejectsCorruption(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases()[:2] {
		t.Run(tc.name, func(t *testing.T) {
			source := "type Maybe=Text?\n" + strings.ReplaceAll(p15FunctionOptionalSource(tc), "Text?", "Maybe")
			actions := p15CompiledActions(t, source)
			if err := p15FunctionOptionalError(actions, tc); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"value_presence", "argument_omission", "dispatch", "presence_decoder", "call_input", "duplicate_call"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, actions)
					if err := p15FunctionOptionalError(bad, tc); err != nil {
						t.Fatal("unchanged clone must pass", err)
					}
					fields, err := p15FunctionContainerPayload(bad)
					if err != nil {
						t.Fatal(err)
					}
					decoder, call := -1, -1
					for i, action := range bad {
						if p15Param(action)["WFDictionaryKey"] == "has_value_0" {
							decoder = i
						}
						if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
							call = i
						}
					}
					if decoder < 0 || call < 0 {
						t.Fatal("corruption target missing")
					}
					switch fault {
					case "value_presence":
						flag := "1"
						if tc.valuePresent {
							flag = "0"
						}
						fields["has_value_0"]["WFValue"] = p15FunctionText(flag)
					case "argument_omission":
						fields["has_arg_0"]["WFValue"] = p15FunctionText("0")
					case "dispatch":
						fields["function"]["WFValue"] = p15FunctionText("other.echo")
					case "presence_decoder":
						p15Param(bad[decoder])["WFDictionaryKey"] = "arg_0"
					case "call_input":
						p15Param(bad[call])["WFInput"] = p15FunctionEdge("missing-payload")
					case "duplicate_call":
						bad = append(bad[:call], append([]map[string]any{bad[call]}, bad[call:]...)...)
					}
					if p15FunctionOptionalError(bad, tc) == nil {
						t.Fatal("corrupted aliased Optional graph accepted")
					}
				})
			}
		})
	}
}

func TestP15OptionalTypeAliasPrimitiveSpellingUsesDeclarationScope(t *testing.T) {
	for _, tc := range []struct {
		name, alias string
		value       p15FunctionOptionalCase
	}{
		{"Text_is_optional_Number", "Text", p15FunctionOptionalCases()[3]},
		{"Number_is_optional_Text", "Number", p15FunctionOptionalCases()[1]},
	} {
		for _, imported := range []bool{false, true} {
			mode := "local"
			if imported {
				mode = "qualified"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				declaration := "type " + tc.alias + "=" + tc.value.typ + "?\n"
				annotation := tc.alias
				if imported {
					annotation = "dep." + annotation
				}
				body := strings.ReplaceAll(p15FunctionOptionalSource(tc.value), tc.value.typ+"?", annotation)
				var actions []map[string]any
				if imported {
					entry := "import \"./dependency.cherri\" as dep\ntype " + tc.alias + "=Bool?\n" + body
					path, source := p15ModuleProject(t, map[string]string{"main.cherri": entry, "dependency.cherri": declaration})
					actions = p15OptionalAliasModuleActions(t, path, source, "")
				} else {
					source := declaration + body
					if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
						t.Fatal(diagnostics)
					}
					actions = p15CompiledActions(t, source)
				}
				if err := p15FunctionOptionalError(actions, tc.value); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
