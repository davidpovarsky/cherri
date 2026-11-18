package analysis

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Bind by parameter identity first, then analyze each supplied expression once
// in its source order. Parameter order controls the envelope slots, not effects.
func (a *Analyzer) checkUserFunctionArguments(fn *syntax.FunctionDecl, call *syntax.CallExpr, scope *Scope) {
	usedNamed := make([]bool, len(call.NamedArgs))
	namedParameters := make([]int, len(call.NamedArgs))
	for i := range namedParameters {
		namedParameters[i] = -1
	}
	unnamedArgIndex := 0
	if call.PrimaryArg != nil && len(fn.Parameters) == 0 {
		a.error(CodePositionalArg, call.PrimaryArg.NodeSpan(), fmt.Sprintf("function %q does not accept positional arguments", fn.Name))
	}
	for i, param := range fn.Parameters {
		supplied := i == 0 && call.PrimaryArg != nil
		if !supplied {
			for index, named := range call.NamedArgs {
				if named.Label != "" && (named.Label == param.Name || strings.TrimPrefix(param.Name, "_") == named.Label) {
					if usedNamed[index] {
						a.error(CodeDuplicateArgument, named.Span, fmt.Sprintf("argument label %q is ambiguous across parameters in call to %q", named.Label, fn.Name))
					} else {
						namedParameters[index] = i
					}
					usedNamed[index] = true
					supplied = true
					break
				}
			}
		}
		if !supplied {
			for unnamedArgIndex < len(call.NamedArgs) {
				index := unnamedArgIndex
				named := call.NamedArgs[index]
				unnamedArgIndex++
				if named.Label == "" {
					usedNamed[index] = true
					namedParameters[index] = i
					supplied = true
					break
				}
			}
		}
		if !supplied && !param.Optional && param.DefaultExpr == nil {
			a.error(CodeMissingArgument, call.Span, fmt.Sprintf("missing required argument %q in call to %q", param.Name, fn.Name))
		}
	}

	analyzeArgument := func(argument syntax.Expression, parameterIndex int) {
		if parameterIndex < 0 || parameterIndex >= len(fn.Parameters) {
			a.analyzeExpression(argument, scope)
			return
		}
		param := fn.Parameters[parameterIndex]
		actual := a.analyzeEnumContext(argument, scope, a.enumCatalog.TypeName(param.TypeExpr))
		if param.TypeExpr == nil {
			return
		}
		expected := a.resolveType(param.TypeExpr)
		if actual != nil && !actual.AssignableTo(expected) {
			a.error(CodeArgumentType, argument.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", param.Name, expected, actual))
		}
	}
	if call.PrimaryArg != nil {
		analyzeArgument(call.PrimaryArg, 0)
	}
	for index, named := range call.NamedArgs {
		analyzeArgument(named.Value, namedParameters[index])
		if usedNamed[index] {
			continue
		}
		if named.Label == "" {
			a.error(CodePositionalArg, named.Span, fmt.Sprintf("too many positional arguments in call to %q", fn.Name))
			continue
		}
		known := false
		for _, param := range fn.Parameters {
			if named.Label == param.Name || named.Label == strings.TrimPrefix(param.Name, "_") {
				known = true
				break
			}
		}
		if known {
			a.error(CodeDuplicateArgument, named.Span, fmt.Sprintf("duplicate argument for parameter %q in call to %q", named.Label, fn.Name))
		} else {
			a.error(CodeUnknownArgument, named.Span, fmt.Sprintf("unknown argument label %q in call to %q", named.Label, fn.Name))
		}
	}
}
