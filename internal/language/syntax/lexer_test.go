package syntax

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
)

func TestLexerBasicTokens(t *testing.T) {
	src := `let x: Number = 42
var name = "David"
// line comment
/* nested /* block */ comment */
let isTrue = true
let raw = r"\d+\s+"
let multi = """line 1
line 2"""
`
	file := source.NewFile("test.cherri", "file:///test.cherri", 1, src)
	lexer := NewLexer(file)

	expected := []struct {
		typ TokenType
		txt string
	}{
		{TokenLet, "let"},
		{TokenIdent, "x"},
		{TokenColon, ":"},
		{TokenIdent, "Number"},
		{TokenAssign, "="},
		{TokenNumber, "42"},
		{TokenVar, "var"},
		{TokenIdent, "name"},
		{TokenAssign, "="},
		{TokenString, "David"},
		{TokenLet, "let"},
		{TokenIdent, "isTrue"},
		{TokenAssign, "="},
		{TokenTrue, "true"},
		{TokenLet, "let"},
		{TokenIdent, "raw"},
		{TokenAssign, "="},
		{TokenRawString, `\d+\s+`},
		{TokenLet, "let"},
		{TokenIdent, "multi"},
		{TokenAssign, "="},
		{TokenString, "line 1\nline 2"},
		{TokenEOF, ""},
	}

	for i, exp := range expected {
		tok := lexer.NextToken()
		if tok.Type != exp.typ {
			t.Fatalf("token %d: expected type %v, got %v (%q)", i, exp.typ, tok.Type, tok.Text)
		}
		if tok.Text != exp.txt {
			t.Fatalf("token %d: expected text %q, got %q", i, exp.txt, tok.Text)
		}
	}
}

func TestLexerFString(t *testing.T) {
	src := `f"Hello {name}, literal {{brace}}"`
	file := source.NewFile("fstring.cherri", "file:///fstring.cherri", 1, src)
	lexer := NewLexer(file)

	// Token 1: TokenFStringStart
	tok := lexer.NextToken()
	if tok.Type != TokenFStringStart {
		t.Fatalf("expected TokenFStringStart, got %v", tok)
	}

	// Chunk 1: "Hello "
	chunk1 := lexer.NextToken()
	if chunk1.Type != TokenFStringPart || chunk1.Text != "Hello " {
		t.Fatalf("expected chunk 'Hello ', got %v", chunk1)
	}

	// Brace: '{'
	brace := lexer.NextToken()
	if brace.Type != TokenLBrace {
		t.Fatalf("expected TokenLBrace, got %v", brace)
	}

	// Expression: name
	tokName := lexer.NextToken()
	if tokName.Type != TokenIdent || tokName.Text != "name" {
		t.Fatalf("expected TokenIdent 'name', got %v", tokName)
	}

	// Closing brace '}'
	rbrace := lexer.NextToken()
	if rbrace.Type != TokenRBrace {
		t.Fatalf("expected TokenRBrace, got %v", rbrace)
	}

	// Chunk 2: ", literal {brace}"
	chunk2 := lexer.NextToken()
	if chunk2.Type != TokenFStringPart || chunk2.Text != ", literal {brace}" {
		t.Fatalf("expected chunk ', literal {brace}', got %v", chunk2)
	}

	// End quote
	end := lexer.NextToken()
	if end.Type != TokenFStringEnd {
		t.Fatalf("expected TokenFStringEnd, got %v", end)
	}
}
