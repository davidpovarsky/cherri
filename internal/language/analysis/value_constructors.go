package analysis

import (
	"fmt"
	"strconv"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// ValueConstructorArgument retains source order independently of parameter order,
// so named quantities keep the existing capture-before-later-write semantics.
type ValueConstructorArgument struct {
	Parameter   int
	SourceIndex int
	Expression  syntax.Expression
}

// BindValueConstructorArguments applies the original intrinsic parameter names
// and requiredness. It is shared by analysis and lowering, including lower-only callers.
func BindValueConstructorArguments(call *syntax.CallExpr, name string) ([]ValueConstructorArgument, []Diagnostic) {
	var labels = []string{"value", "unit"}
	if name == "color" {
		labels = []string{"red", "green", "blue", "alpha"}
	}
	var bound = make([]bool, len(labels))
	var arguments []ValueConstructorArgument
	var diagnostics []Diagnostic
	var addError = func(code string, span source.Span, message string) {
		diagnostics = append(diagnostics, Diagnostic{Code: code, Severity: SeverityError, Span: span, Message: message})
	}
	var bind = func(label string, expression syntax.Expression, sourceIndex int, span source.Span) {
		var parameter = -1
		for index, candidate := range labels {
			if (label != "" && label == candidate) || (label == "" && !bound[index]) {
				parameter = index
				break
			}
		}
		if parameter < 0 {
			var code = CodeUnknownArgument
			var message = fmt.Sprintf("unknown argument label %q in %s()", label, name)
			if label == "" {
				code = CodePositionalArg
				message = fmt.Sprintf("too many positional arguments in %s()", name)
			}
			addError(code, span, message)
			return
		}
		if bound[parameter] {
			addError(CodeDuplicateArgument, span, fmt.Sprintf("duplicate argument %q in %s()", labels[parameter], name))
			return
		}
		bound[parameter] = true
		arguments = append(arguments, ValueConstructorArgument{Parameter: parameter, SourceIndex: sourceIndex, Expression: expression})
	}
	if call.PrimaryArg != nil {
		bind("", call.PrimaryArg, -1, call.PrimaryArg.NodeSpan())
	}
	for index, argument := range call.NamedArgs {
		bind(argument.Label, argument.Value, index, argument.Span)
	}
	if name == "qty" {
		for index, present := range bound {
			if !present {
				addError(CodeMissingArgument, call.Span, fmt.Sprintf("missing required argument %q in qty()", labels[index]))
			}
		}
	}
	return arguments, diagnostics
}

// QuantityUnitLiteral follows the canonical text-literal contract, which also
// permits authored text interpolation but not a bare runtime unit reference.
func QuantityUnitLiteral(expression syntax.Expression) bool {
	if _, template := expression.(*syntax.FStringExpr); template {
		return true
	}
	_, literal := literalTextValue(expression)
	return literal
}

// ColorLiteralComponents preserves the established numeric native color shape.
// Dynamic components have no supported encoder and must never become zero.
func ColorLiteralComponents(call *syntax.CallExpr) ([4]float64, []Diagnostic) {
	var components = [4]float64{0, 0, 0, 1}
	var arguments, diagnostics = BindValueConstructorArguments(call, "color")
	for _, argument := range arguments {
		if !quantityScalarLiteral(argument.Expression) {
			diagnostics = append(diagnostics, Diagnostic{Code: CodeLiteralRequired, Severity: SeverityError, Span: argument.Expression.NodeSpan(), Message: "color() requires finite numeric literal components; runtime values are unsupported"})
			continue
		}
		var expression = argument.Expression
		var sign = 1.0
		for {
			var unary, ok = expression.(*syntax.UnaryExpr)
			if !ok {
				break
			}
			if unary.Op == syntax.TokenMinus {
				sign = -sign
			}
			expression = unary.Operand
		}
		var literal = expression.(*syntax.LiteralExpr)
		var value, _ = strconv.ParseFloat(literal.Value, 64)
		components[argument.Parameter] = sign * value
	}
	return components, diagnostics
}

func (a *Analyzer) analyzeValueConstructor(call *syntax.CallExpr, name string, scope *Scope) types.Type {
	var argumentTypes = make(map[syntax.Expression]types.Type)
	if call.PrimaryArg != nil {
		argumentTypes[call.PrimaryArg] = a.analyzeExpression(call.PrimaryArg, scope)
	}
	for _, argument := range call.NamedArgs {
		argumentTypes[argument.Value] = a.analyzeExpression(argument.Value, scope)
	}
	if name == "color" {
		var _, diagnostics = ColorLiteralComponents(call)
		a.diagnostics = append(a.diagnostics, diagnostics...)
		return types.AnyContent
	}
	var arguments, diagnostics = BindValueConstructorArguments(call, name)
	a.diagnostics = append(a.diagnostics, diagnostics...)
	for _, argument := range arguments {
		var valueType = unwrapDynamicOptional(argumentTypes[argument.Expression])
		if argument.Parameter == 0 {
			if valueType != nil && !valueType.AssignableTo(types.Number) {
				a.error(CodeArgumentType, argument.Expression.NodeSpan(), "qty() magnitude expects Number, got "+valueType.String())
			}
		} else if valueType != nil && !valueType.Equals(types.Text) {
			a.error(CodeArgumentType, argument.Expression.NodeSpan(), "qty() unit expects a text literal")
		} else if !QuantityUnitLiteral(argument.Expression) {
			a.error(CodeLiteralRequired, argument.Expression.NodeSpan(), "qty() unit requires a text literal or an authored text template")
		}
	}
	return types.AnyContent
}
