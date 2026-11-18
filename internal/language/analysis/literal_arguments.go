package analysis

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// A compiler-declared literal requirement is authoritative even when the native
// accepted-form evidence is inferred. Otherwise enforce only confirmed slots
// whose complete accepted-form set excludes both references and token strings.
func requiresLiteralArgument(param *schema.ParameterSchema) bool {
	if param.Literal {
		return true
	}
	if param.EvidenceStatus != schema.EvidenceConfirmed {
		return false
	}
	literal := false
	for _, form := range param.AcceptedForms {
		switch form {
		case "literal":
			literal = true
		case "tokenString", "attachment(output)", "attachment(variable)":
			return false
		default:
			return false
		}
	}
	return literal
}

// Eligible expressions are deliberately limited to values that the existing
// lowerer already materializes without an action: scalar literals, pure text
// concatenation/interpolation, pure Boolean operations, enums, and recursively
// literal containers. A let binding remains a runtime snapshot; this function
// never looks through a symbol, action, index, member, or control expression.
func eligibleLiteralArgument(expr syntax.Expression) bool {
	switch e := expr.(type) {
	case *syntax.LiteralExpr, *syntax.EnumMemberExpr:
		return true
	case *syntax.ListExpr:
		for _, item := range e.Elements {
			if !eligibleLiteralArgument(item) {
				return false
			}
		}
		return true
	case *syntax.MapExpr:
		for _, entry := range e.Entries {
			if !eligibleLiteralArgument(entry.Key) || !eligibleLiteralArgument(entry.Value) {
				return false
			}
		}
		return true
	}
	if _, ok := literalTextValue(expr); ok {
		return true
	}
	_, ok := literalBoolValue(expr)
	return ok
}

// Keep enum checking on the folded value as well as direct string literals.
// This evaluator has no symbols or effects and mirrors only already-supported
// pure text folding, not the compiler's general expression execution.
func literalTextValue(expr syntax.Expression) (string, bool) {
	switch e := expr.(type) {
	case *syntax.LiteralExpr:
		if e.Type == syntax.TokenString || e.Type == syntax.TokenRawString {
			return e.Value, true
		}
	case *syntax.BinaryExpr:
		if e.Op != syntax.TokenPlus {
			return "", false
		}
		left, lok := literalTextValue(e.Left)
		right, rok := literalTextValue(e.Right)
		if lok && rok {
			return left + right, true
		}
	case *syntax.FStringExpr:
		var text strings.Builder
		for _, part := range e.Parts {
			if !part.IsExpr {
				text.WriteString(part.Text)
				continue
			}
			value, ok := literalTextValue(part.Expr)
			if !ok {
				return "", false
			}
			text.WriteString(value)
		}
		return text.String(), true
	}
	return "", false
}

func literalBoolValue(expr syntax.Expression) (bool, bool) {
	switch e := expr.(type) {
	case *syntax.LiteralExpr:
		if e.Type == syntax.TokenTrue {
			return true, true
		}
		if e.Type == syntax.TokenFalse {
			return false, true
		}
	case *syntax.UnaryExpr:
		if e.Op == syntax.TokenBang {
			value, ok := literalBoolValue(e.Operand)
			return !value, ok
		}
	case *syntax.BinaryExpr:
		left, lok := literalBoolValue(e.Left)
		right, rok := literalBoolValue(e.Right)
		if !lok || !rok {
			return false, false
		}
		switch e.Op {
		case syntax.TokenAnd:
			return left && right, true
		case syntax.TokenOr:
			return left || right, true
		case syntax.TokenEqual:
			return left == right, true
		case syntax.TokenNotEqual:
			return left != right, true
		}
	}
	return false, false
}
