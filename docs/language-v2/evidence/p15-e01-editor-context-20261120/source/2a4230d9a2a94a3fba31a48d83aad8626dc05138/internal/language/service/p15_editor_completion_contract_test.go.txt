package service

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/analysis"
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
	{"canonical_number", p15E01Declarations + "number(|)", []string{"numberValue"}},
	{"lexical_shadow", p15E01Declarations + "if true {\n let numberValue = \"inner\"\n let innerNumber = 11\n needNumber(|)\n}\n", []string{"innerNumber"}},
	{"function_parameter", p15E01Declarations + "function wrap(amount: Number, title: Text) -> Number {\n return needNumber(|)\n}", []string{"amount", "numberValue"}},
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
	actual := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		if item.Kind != CompletionKindVariable {
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
