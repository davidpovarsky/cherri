package service

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// ApplyVisualSourceArgument splices one existing argument identified by a
// compiler-owned preview origin. It is separate from ApplyVisualSourceEdit,
// whose public actionIndex continues to mean a top-level AST statement index.
// The native bridge verifies the preview/source/wire binding before this call.
func (s *Service) ApplyVisualSourceArgument(src string, callStart, callEnd, argumentStart, argumentEnd int, newExpr string) (string, error) {
	if callStart < 0 || callEnd > len(src) || callStart >= callEnd ||
		argumentStart < callStart || argumentEnd > callEnd || argumentStart >= argumentEnd {
		return src, fmt.Errorf("visual edit source range is invalid")
	}
	file := source.NewFile("visual-edit", "visual-edit", 1, src)
	parser := syntax.NewParser(file)
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		return src, fmt.Errorf("cannot apply visual edit to source with syntax errors")
	}
	matchingArguments := 0
	for _, statement := range program.Statements {
		var expression syntax.Expression
		switch statement := statement.(type) {
		case *syntax.ExprStmt:
			expression = statement.Expr
		case *syntax.BindingStmt:
			expression = statement.Value
		}
		call, ok := expression.(*syntax.CallExpr)
		if !ok || call.Block != nil || call.Span.Start.ByteOffset != callStart || call.Span.End.ByteOffset != callEnd {
			continue
		}
		matches := func(argument syntax.Expression) bool {
			if argument == nil {
				return false
			}
			span := argument.NodeSpan()
			return span.Document == call.Span.Document && span.Start.ByteOffset == argumentStart && span.End.ByteOffset == argumentEnd
		}
		if matches(call.PrimaryArg) {
			matchingArguments++
		}
		for _, argument := range call.NamedArgs {
			if matches(argument.Value) {
				matchingArguments++
			}
		}
	}
	if matchingArguments != 1 {
		return src, fmt.Errorf("visual edit is not one existing argument of a direct top-level call; generated or unmapped actions are read-only")
	}
	trimmed := strings.TrimSpace(newExpr)
	if trimmed == "" {
		return src, fmt.Errorf("replacement expression cannot be empty")
	}
	expressionFile := source.NewFile("edit-expr", "edit-expr", 1, trimmed)
	expressionParser := syntax.NewParser(expressionFile)
	expressionProgram := expressionParser.ParseProgram()
	if len(expressionParser.Errors()) != 0 || len(expressionProgram.Statements) != 1 {
		return src, fmt.Errorf("invalid replacement expression")
	}
	if _, ok := expressionProgram.Statements[0].(*syntax.ExprStmt); !ok {
		return src, fmt.Errorf("replacement is not a valid expression")
	}
	return src[:argumentStart] + trimmed + src[argumentEnd:], nil
}
