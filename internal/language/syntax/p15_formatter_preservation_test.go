package syntax

import (
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
)

func p15FormatParse(t *testing.T, text string) *Program {
	t.Helper()
	parser := NewParser(source.NewFile("format-test", "format-test.cherri", 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatalf("valid formatter fixture did not parse: %v\n%s", parser.Errors(), text)
	}
	return program
}

func p15FormatLiteralValues(t *testing.T, text string) []string {
	t.Helper()
	lexer := NewLexer(source.NewFile("format-token-test", "format-test.cherri", 1, text))
	var values []string
	for {
		token := lexer.NextToken()
		if token.Type == TokenEOF {
			return values
		}
		if token.Type == TokenError {
			t.Fatalf("formatted source is invalid: %s\n%s", token.Text, text)
		}
		switch token.Type {
		case TokenString, TokenRawString, TokenFStringPart:
			values = append(values, token.Text)
		}
	}
}

func TestP15E04LiteralValuesAndIdempotence(t *testing.T) {
	cases := []struct{ name, code string }{
		{"raw_backslashes", `let value = r"C:\Temp\folder\n\t\u0041"
show(value)
`},
		{"raw_triple_quotes", `let value = r"""first "quoted" line
C:\second\🙂שלום"""
show(value)
`},
		{"normal_controls", `let value = "\0\u0001\u0007\u0008\u000b\u000c\u001b\u007f"
show(value)
`},
		{"single_quoted_escape", `let value = 'quote " and \n newline and \\ slash'
show(value)
`},
		{"triple_normal", `let value = """line one
literal \n and "quote" שלום🙂"""
show(value)
`},
		{"interpolated_controls", `let first = "שלום🙂"
let value = f"prefix\0\u0007\\ {{braces}} {first} suffix"
show(value)
`},
		{"unicode_and_grouping", `let שֵׁם = "👩‍💻é"
let value = f"{שֵׁם}|{(5 - 2) * 3}"
show(value)
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := p15FormatParse(t, tc.code)
			want := p15FormatLiteralValues(t, tc.code)
			first := Format(original)
			again := p15FormatParse(t, first)
			got := p15FormatLiteralValues(t, first)
			if !reflect.DeepEqual(want, got) {
				t.Errorf("literal bytes changed: want %q; got %q\n%s", want, got, first)
			}
			second := Format(again)
			if first != second {
				t.Errorf("second format changes source:\nFIRST\n%s\nSECOND\n%s", first, second)
			}
		})
	}
}

func TestP15E04AllCommentContentsPreserved(t *testing.T) {
	text := "// header שלום🙂  \nlet value = r\"C:\\data\" // suffix  \n/* middle\n exact spacing */\nif true {\n // inner  \n show(value)\n}\n// final comment  \n"
	original := p15FormatParse(t, text)
	formatted := Format(original)
	actual := p15FormatParse(t, formatted)
	var want, got []string
	for _, comment := range original.Comments {
		want = append(want, comment.Text)
	}
	for _, comment := range actual.Comments {
		got = append(got, comment.Text)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("comment text changed: want %q; got %q\n%s", want, got, formatted)
	}
	if second := Format(actual); second != formatted {
		t.Fatal("comment-bearing source is not idempotent")
	}
}
