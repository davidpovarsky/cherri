package analysis

import (
	"strconv"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// The ordinary binder checks types and literal-only forms first. Resolve the
// same bounded option contract used by the canonical native expansion, so an
// analysis-only editor cannot advertise an invalid URL or unbounded limit.
func (a *Analyzer) checkPaginateArguments(call *syntax.CallExpr, arguments map[string]syntax.Expression) {
	params := make(map[string]any)
	for key, expression := range arguments {
		if key == "WFHTTPHeaders" {
			// The ordinary HTTP binder requires a literal Dictionary container
			// and supports dynamic leaves. These are not scalar traversal options.
			continue
		}
		if text, ok := literalTextValue(expression); ok {
			params[key] = text
			continue
		}
		if literal, ok := expression.(*syntax.LiteralExpr); ok && literal.Type == syntax.TokenNumber {
			number, err := strconv.ParseFloat(literal.Value, 64)
			if err == nil {
				params[key] = number
				continue
			}
		}
		// An unsupported/dynamic form must not become an omitted default.
		params[key] = nil
	}
	if _, err := schema.ResolvePaginateOptions(params); err != nil {
		a.error(CodeArgumentType, call.Span, err.Error())
	}
}
