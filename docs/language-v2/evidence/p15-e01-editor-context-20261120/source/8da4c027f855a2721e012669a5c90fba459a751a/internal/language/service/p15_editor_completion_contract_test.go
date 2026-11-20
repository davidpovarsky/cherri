package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"os"
)

const p15E01Declarations = "type Amount = Number\n" +
	"function needNumber(value: Number) -> Number { return value }\n" +
	"function needText(value: Text) -> Text { return value }\n" +
	"function needBool(value: Bool) -> Bool { return value }\n" +
	"function needList(value: List<Number>) -> List<Number> { return value }\n" +
	"function needOptional(value: Number?) -> Number? { return value }\n" +
	"function needAlias(value: Amount) -> Amount { return value }\n" +
	"function named(value: Number, label: Text) -> Text { return label }\n" +
	"let numberValue = 7\nlet textValue = \"seven\"\nlet flagValue = false\n" +
	"let listValue = [1, 2]\nlet optionalValue: Number? = none\nlet opaqueValue = Clipboard\n"

type p15E01CompletionCase struct {
	name, marked string
	want         []string
}

var p15E01CompletionCases = []p15E01CompletionCase{
	{"primary_number", p15E01Declarations + "needNumber(|)\nlet laterNumber = 8\n", []string{"numberValue"}},
	{"named_text", p15E01Declarations + "named(1, label: |)", []string{"textValue"}},
	{"primary_bool", p15E01Declarations + "needBool(|)", []string{"flagValue"}},
	{"primary_list", p15E01Declarations + "needList(|)", []string{"listValue"}},
	{"optional_parameter", p15E01Declarations + "needOptional(|)", []string{"numberValue", "optionalValue"}},
	{"alias_parameter", p15E01Declarations + "needAlias(|)", []string{"numberValue"}},
	{"canonical_number", p15E01Declarations + "round(|)", []string{"numberValue"}},
	{"canonical_broad_input", p15E01Declarations + "number(|)", []string{"flagValue", "listValue", "numberValue", "opaqueValue", "optionalValue", "textValue"}},
	{"lexical_shadow", p15E01Declarations + "if true {\n let numberValue = \"inner\"\n let innerNumber = 11\n needNumber(|)\n}\n", []string{"innerNumber"}},
	{"function_parameter", p15E01Declarations + "function wrap(amount: Number, title: Text) -> Number {\n return needNumber(|)\n}", []string{"amount"}},
	{"narrowed_optional", p15E01Declarations + "var maybe: Number? = if true { yield 7 } else { yield none }\nif maybe != none { needNumber(|) }", []string{"numberValue", "maybe"}},
	{"reassigned_none", p15E01Declarations + "var maybe: Number? = 7\nif maybe != none { maybe = none; needNumber(|) }", []string{"numberValue"}},
	{"uninitialized_binding", p15E01Declarations + "var missing: Number\nneedNumber(|)", []string{"numberValue"}},
	{"loop_item_and_index", p15E01Declarations + "for (index, item) in [\"a\"] { needNumber(|) }", []string{"index", "numberValue"}},
	{"default_parameter_scope", p15E01Declarations + "function defaults(first: Number = 1, second: Number = needNumber(|)) -> Number { return second }", []string{"first"}},
	{"user_function_shadows_native", p15E01Declarations + "function round(value: Text) -> Text { return value }\nround(|)", []string{"textValue"}},
	{"named_after_write", p15E01Declarations + "function combine(note: Text, value: Number) -> Number { return value }\nvar maybe: Number? = 7\ncombine(if true { maybe = none; yield \"set\" } else { yield \"other\" }, value: |)", []string{"numberValue"}},
	{"named_before_write", p15E01Declarations + "function combine(note: Text, value: Number) -> Number { return value }\nvar maybe: Number? = 7\ncombine(value: |, note: if true { maybe = none; yield \"set\" } else { yield \"other\" })", []string{"numberValue", "maybe"}},
	{"outside_value_context", "let first = 1\nlet second = \"x\"\nshow(first)\n|", []string{"first", "second"}},
}

func p15EditorCursor(t *testing.T, marked string) (string, int, int) {
	t.Helper()
	if strings.Count(marked, "|") != 1 {
		t.Fatal("source must have exactly one independent cursor")
	}
	offset := strings.Index(marked, "|")
	prefix := marked[:offset]
	return marked[:offset] + marked[offset+1:], strings.Count(prefix, "\n") + 1,
		utf8.RuneCountInString(prefix[strings.LastIndex(prefix, "\n")+1:]) + 1
}

func p15E01VariableSetError(items []CompletionItem, want []string) error {
	return p15E01KindSetError(items, CompletionKindVariable, want)
}

func p15E01KindSetError(items []CompletionItem, kind CompletionItemKind, want []string) error {
	actual := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		if item.Kind != kind {
			continue
		}
		if seen[item.Label] {
			return fmt.Errorf("duplicate variable %q", item.Label)
		}
		seen[item.Label] = true
		actual = append(actual, item.Label)
	}
	expected := append([]string{}, want...)
	sort.Strings(actual)
	sort.Strings(expected)
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("actual contextual variables %v, want exactly %v", actual, expected)
	}
	return nil
}

func TestP15E01ExpectedTypeCompletion(t *testing.T) {
	for _, tc := range p15E01CompletionCases {
		t.Run(tc.name, func(t *testing.T) {
			source, line, col := p15EditorCursor(t, tc.marked)
			svc := NewService(nil)
			const uri = "file:///p15-e01-completion.cherri"
			svc.OpenDocument(uri, 1, source)
			if err := p15E01VariableSetError(svc.Complete(uri, line, col), tc.want); err != nil {
				t.Fatal(err)
			}
			if tc.name == "outside_value_context" {
				return
			}
			// Every offered candidate is checked through the same actual analyzer
			// in the authored call; no inferred expected type is used as an oracle.
			for _, name := range tc.want {
				svc.ChangeDocument(uri, 2, strings.Replace(tc.marked, "|", name, 1))
				diagnostics, _ := svc.Analyze(uri)
				for _, d := range diagnostics {
					if d.Severity == analysis.SeverityError {
						t.Fatalf("offered candidate %s does not satisfy the source: %v", name, d)
					}
				}
			}
		})
	}
}

func TestP15E01CompletionOracleRejectsMutations(t *testing.T) {
	const uri = "file:///p15-e01-oracle.cherri"
	svc := NewService(nil)
	source, line, col := p15EditorCursor(t, "let first = 1\nlet second = \"x\"\nshow(first)\n|")
	svc.OpenDocument(uri, 1, source)
	actual := svc.Complete(uri, line, col)
	want := []string{"first", "second"}
	if err := p15E01VariableSetError(actual, want); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "unrelated", "duplicate", "wrong_kind"} {
		t.Run(name, func(t *testing.T) {
			items := append([]CompletionItem{}, actual...)
			variable := -1
			for i, item := range items {
				if item.Kind == CompletionKindVariable {
					variable = i
					break
				}
			}
			if variable < 0 {
				t.Fatal("actual service supplied no mutation target")
			}
			switch name {
			case "missing":
				items = append(items[:variable], items[variable+1:]...)
			case "unrelated":
				items = append(items, CompletionItem{Kind: CompletionKindVariable, Label: "foreign"})
			case "duplicate":
				items = append(items, items[variable])
			case "wrong_kind":
				items[variable].Kind = CompletionKindFunction
			}
			if p15E01VariableSetError(items, want) == nil {
				t.Fatal("corrupted actual service candidates were accepted")
			}
		})
	}
}

func TestP15E01CandidateInsertionControls(t *testing.T) {
	for _, tc := range p15E01CompletionCases {
		if tc.name == "outside_value_context" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range tc.want {
				svc := NewService(nil)
				const uri = "file:///p15-e01-positive.cherri"
				svc.OpenDocument(uri, 1, strings.Replace(tc.marked, "|", name, 1))
				diagnostics, _ := svc.Analyze(uri)
				for _, d := range diagnostics {
					if d.Severity == analysis.SeverityError {
						t.Fatalf("independent source/candidate control %s: %v", name, d)
					}
				}
			}
		})
	}
}

func TestP15E01CanonicalInputContracts(t *testing.T) {
	for _, tc := range []struct {
		name, file, declaration string
		input                   types.Type
	}{
		{"number", "../../../actions/basic.cherri", "action number(variable number: 'WFNumberActionNumber'): number", types.AnyContent},
		{"round", "../../../actions/math.cherri", "action default round(number number: 'WFInput'", types.Number},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(tc.file)
			if err != nil || !strings.Contains(string(raw), tc.declaration) {
				t.Fatal("independent canonical declaration absent", err)
			}
			action, found := schema.DefaultRegistry().LookupAction(tc.name)
			if !found {
				t.Fatal("canonical action missing", tc.name)
			}
			primary, found := action.PrimaryParameter()
			if !found || primary.Type == nil || !primary.Type.Equals(tc.input) ||
				action.OutputType == nil || !action.OutputType.Equals(types.Number) {
				t.Fatalf("input/output semantic contract differs: %+v", action)
			}
		})
	}
}

func TestP15E01ContextualLabelsAndEnums(t *testing.T) {
	const declarations = "enum Color { Red = \"native-red\", Blue = \"native-blue\" }\n" +
		"enum Other { Foreign = \"foreign\" }\n" +
		"function paint(color: Color, note: Text = \"\") -> Text { return note }\n" +
		"function named(value: Number, label: Text) -> Text { return label }\n"
	for _, tc := range []struct {
		name, marked string
		kind         CompletionItemKind
		want         []string
	}{
		{"primary_already_bound", "named(1, |)", CompletionKindParameter, []string{"label"}},
		{"named_already_bound", "named(label: \"value: ,\", |)", CompletionKindParameter, []string{"value"}},
		{"multiline_named", "named(\n label: \"quoted, colon: and 🙂\",\n |)", CompletionKindParameter, []string{"value"}},
		{"canonical_primary_bound", "formatNumber(1, |)", CompletionKindParameter, []string{"decimalPlaces"}},
		{"function_enum", "paint(.|)", CompletionKindEnum, []string{"Blue", "Red"}},
		{"qualified_enum", "paint(Color.|)", CompletionKindEnum, []string{"Blue", "Red"}},
		{"named_enum", "paint(note: \"x\", color: .|)", CompletionKindEnum, []string{"Blue", "Red"}},
		{"ambiguous_dot", ".|", CompletionKindEnum, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, line, col := p15EditorCursor(t, declarations+tc.marked)
			svc := NewService(nil)
			const uri = "file:///p15-e01-labels.cherri"
			svc.OpenDocument(uri, 1, source)
			actual := svc.Complete(uri, line, col)
			if err := p15E01KindSetError(actual, tc.kind, tc.want); err != nil {
				t.Fatal(err)
			}
			corrupt := append([]CompletionItem{}, actual...)
			corrupt = append(corrupt, CompletionItem{Kind: tc.kind, Label: "foreign_domain_or_parameter"})
			if p15E01KindSetError(corrupt, tc.kind, tc.want) == nil {
				t.Fatal("unrelated contextual candidate was accepted")
			}
		})
	}
}

func TestP15E01QualifiedModuleOwnership(t *testing.T) {
	const dependency = "type Amount = Number\nfunction take(amount: Amount, label: Text = \"\") -> Number { return amount }\n"
	const declarations = "import \"./dependency.cherri\" as dep\ntype Amount = Text\nlet numberValue = 7\nlet textValue = \"seven\"\n"
	for _, tc := range []struct {
		name, marked, completed string
		kind                    CompletionItemKind
		want                    []string
	}{
		{"qualified_primary", "dep.take(|)", "dep.take(numberValue)", CompletionKindVariable, []string{"numberValue"}},
		{"qualified_named", "dep.take(label: \"x\", amount: |)", "dep.take(label: \"x\", amount: numberValue)", CompletionKindVariable, []string{"numberValue"}},
		{"qualified_labels", "dep.take(1, |)", "dep.take(1, label: \"x\")", CompletionKindParameter, []string{"label"}},
		{"qualified_partial_identifier", "dep.take(nu|)", "dep.take(numberValue)", CompletionKindVariable, []string{"numberValue"}},
		{"unknown_export_no_borrow", "dep.round(|)", "", CompletionKindParameter, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "dependency.cherri"), []byte(dependency), 0600); err != nil {
				t.Fatal(err)
			}
			entry := filepath.Join(directory, "main.cherri")
			path := filepath.ToSlash(entry)
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			svc := NewService(nil)
			text, line, col := p15EditorCursor(t, declarations+tc.marked)
			svc.OpenDocument(uri, 1, text)
			if err := p15E01KindSetError(svc.Complete(uri, line, col), tc.kind, tc.want); err != nil {
				t.Fatal(err)
			}
			if tc.completed != "" {
				svc.ChangeDocument(uri, 2, declarations+tc.completed)
				diagnostics, _ := svc.Analyze(uri)
				for _, d := range diagnostics {
					if d.Severity == analysis.SeverityError {
						t.Fatal("completed qualified source failed", d)
					}
				}
			}
		})
	}
}

func TestP15E01ContextFactIsolation(t *testing.T) {
	const uri = "file:///p15-e01-isolation.cherri"
	svc := NewService(nil)
	code, line, col := p15EditorCursor(t, p15E01Declarations+"needNumber(|)")
	svc.OpenDocument(uri, 1, code)
	doc := svc.documents[uri]
	_, context := enumCompletionTokens(doc.File, byteOffsetAtLineCol(doc.File, line, col))
	info, values, found := doc.Analyzer.EditorCallAt(context.calleeSpan.End.ByteOffset, 0)
	if !found || len(info.Parameters) != 1 || !info.Parameters[0].Type.Equals(types.Number) ||
		values["numberValue"] == nil || !values["numberValue"].Equals(types.Number) {
		t.Fatal("actual contextual semantic facts absent", info, values)
	}
	t.Run("returned_collections_are_copies", func(t *testing.T) {
		info.Parameters[0].Type = types.Text
		info.Parameters[0].Label = "forged"
		values["numberValue"] = types.Text
		values["foreign"] = types.Number
		if err := p15E01VariableSetError(svc.Complete(uri, line, col), []string{"numberValue"}); err != nil {
			t.Fatal(err)
		}
		if err := p15E01KindSetError(svc.Complete(uri, line, col), CompletionKindParameter, []string{"value"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("analysis_resets_old_exact_identity", func(t *testing.T) {
		file := source.NewFile(source.DocumentID(uri), uri, 2, "let fresh=\"value\"\n")
		parser := syntax.NewParser(file)
		doc.Analyzer.Analyze(parser.ParseProgram())
		if _, _, found := doc.Analyzer.EditorCallAt(context.calleeSpan.End.ByteOffset, 0); found {
			t.Fatal("old call facts survived fresh analysis")
		}
	})
	t.Run("fresh_document_does_not_borrow_old_context", func(t *testing.T) {
		code, line, col := p15EditorCursor(t, p15E01Declarations+"needText(|)")
		svc.ChangeDocument(uri, 3, code)
		if err := p15E01VariableSetError(svc.Complete(uri, line, col), []string{"textValue"}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestP15E01IncompleteModuleRecoveryRemainsBounded(t *testing.T) {
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer server.Close()
	for _, tc := range []struct{ name, header, dependency, want string }{
		{"remote", fmt.Sprintf("import %q as dep\n", server.URL+"/module.cherri"), "", "E_WORKSPACE_ACCESS"},
		{"outside", "import \"../outside.cherri\" as dep\n", "", "E_WORKSPACE_ACCESS"},
		{"invalid_dependency", "import \"./dependency.cherri\" as dep\n", "function broken(", "E_SYNTAX"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			container := t.TempDir()
			directory := filepath.Join(container, "workspace")
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
			const outside = "function forbidden(value:Number)->Number{return value}\n"
			if err := os.WriteFile(filepath.Join(container, "outside.cherri"), []byte(outside), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.dependency != "" {
				if err := os.WriteFile(filepath.Join(directory, "dependency.cherri"), []byte(tc.dependency), 0600); err != nil {
					t.Fatal(err)
				}
			}
			entry := filepath.Join(directory, "main.cherri")
			path := filepath.ToSlash(entry)
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			svc := NewService(nil)
			code, line, col := p15EditorCursor(t, tc.header+"dep.take(value: |)")
			svc.OpenDocument(uri, 1, code)
			if len(svc.documents[uri].ParseErrors) == 0 {
				t.Fatal("incomplete source no longer exercises parser recovery")
			}
			diagnostics, _ := svc.Analyze(uri)
			found := false
			for _, d := range diagnostics {
				found = found || d.Code == tc.want
			}
			if !found {
				t.Fatalf("required bounded module error %s absent: %+v", tc.want, diagnostics)
			}
			if err := p15E01KindSetError(svc.Complete(uri, line, col), CompletionKindParameter, []string{}); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(container, "outside.cherri"))
			if err != nil || string(raw) != outside {
				t.Fatal("outside source changed", err)
			}
			files, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if tc.dependency != "" {
				want = 1
			}
			if len(files) != want {
				t.Fatal("analysis wrote workspace artifacts", files)
			}
		})
	}
	if hits != 0 {
		t.Fatalf("remote module completion issued %d network requests", hits)
	}
}
