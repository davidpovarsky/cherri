package lower

import (
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// EnableVisualSourceOrigins opts this request into a bounded mobile preview map.
// Pointer identity and the original document both matter: copied/lowered calls
// and imported documents must not become editable merely by sharing offsets.
// Nested calls, control bodies, function bodies and copy bodies are not enrolled.
func (l *Lowerer) EnableVisualSourceOrigins(program *syntax.Program) {
	l.visualSourceCalls = make(map[*syntax.CallExpr]source.Span)
	if program == nil {
		return
	}
	for _, statement := range program.Statements {
		var expression syntax.Expression
		switch statement := statement.(type) {
		case *syntax.ExprStmt:
			expression = statement.Expr
		case *syntax.BindingStmt:
			expression = statement.Value
		}
		call, ok := expression.(*syntax.CallExpr)
		if !ok || call.Block != nil || call.Span.Document != program.Span.Document {
			continue
		}
		l.visualSourceCalls[call] = call.Span
	}
}

func (l *Lowerer) visualSourceOrigin(call *syntax.CallExpr) *backend.VisualSourceOrigin {
	span, ok := l.visualSourceCalls[call]
	if !ok || call.Span != span {
		return nil
	}
	return &backend.VisualSourceOrigin{CallSpan: span}
}
