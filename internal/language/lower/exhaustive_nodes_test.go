package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Embedding the sealed syntax interface supplies its marker method while the
// concrete type remains deliberately absent from every production AST switch.
type rp16UnknownExpression struct{ syntax.Expression }

func (*rp16UnknownExpression) NodeSpan() source.Span { return source.Span{} }

type rp16UnknownStatement struct{ syntax.Statement }

func (*rp16UnknownStatement) NodeSpan() source.Span { return source.Span{} }

type rp16UnknownDeclaration struct{ syntax.Declaration }

func (*rp16UnknownDeclaration) NodeSpan() source.Span { return source.Span{} }

func TestRP16UnknownNodesFailCheckedLowering(t *testing.T) {
	cases := []struct {
		name      string
		program   *syntax.Program
		errorPart string
	}{
		{"declaration", &syntax.Program{Declarations: []syntax.Declaration{&rp16UnknownDeclaration{}}}, "unhandled declaration"},
		{"statement", &syntax.Program{Statements: []syntax.Statement{&rp16UnknownStatement{}}}, "unhandled statement"},
		{"expression", &syntax.Program{Statements: []syntax.Statement{&syntax.ExprStmt{Expr: &rp16UnknownExpression{}}}}, "unhandled expression"},
		{"nested_map_value", &syntax.Program{Statements: []syntax.Statement{&syntax.ExprStmt{Expr: &syntax.MapExpr{Entries: []syntax.MapEntry{{Key: &syntax.LiteralExpr{Type: syntax.TokenString, Value: "x"}, Value: &rp16UnknownExpression{}}}}}}}, "unhandled expression"},
		{"nested_list_item", &syntax.Program{Statements: []syntax.Statement{&syntax.ExprStmt{Expr: &syntax.ListExpr{Elements: []syntax.Expression{&rp16UnknownExpression{}}}}}}, "unhandled expression"},
		{"nested_text_reference", &syntax.Program{Statements: []syntax.Statement{&syntax.ExprStmt{Expr: &syntax.FStringExpr{Parts: []syntax.FStringPart{{IsExpr: true, Expr: &rp16UnknownExpression{}}}}}}}, "unhandled expression"},
		{"function_body", &syntax.Program{Declarations: []syntax.Declaration{&syntax.FunctionDecl{Name: "unknownBody", Body: &syntax.BlockStmt{Statements: []syntax.Statement{&syntax.ExprStmt{Expr: &rp16UnknownExpression{}}}}}}, Statements: []syntax.Statement{&syntax.ExprStmt{Expr: &syntax.CallExpr{Callee: &syntax.IdentExpr{Name: "unknownBody"}}}}}, "unhandled expression"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workflow, err := NewLowerer(nil).LowerProgram(tc.program)
			if err == nil || !strings.Contains(err.Error(), tc.errorPart) || workflow != nil {
				t.Fatalf("unknown node did not fail checked lowering explicitly: workflow=%+v err=%v; want %q and nil workflow", workflow, err, tc.errorPart)
			}
		})
	}
}
