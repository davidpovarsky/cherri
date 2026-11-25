package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A source type can be widened for language assignment without converting the
// runtime storage that its declaration already selected. This fact mirrors the
// lowerer's declaration-owned TaggedTransport; it is not a new type assertion.
func (a *Analyzer) taggedExpressionType(expr syntax.Expression, scope *Scope) types.Type {
	if name, ok := expr.(*syntax.IdentExpr); ok && scope != nil {
		if symbol, found := scope.Lookup(name.Name); found && symbol.taggedTransport != nil {
			return symbol.taggedTransport
		}
	}
	return a.exprTypes[expr]
}

func (a *Analyzer) hasErrorsSince(start int) bool {
	for _, diagnostic := range a.diagnostics[start:] {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

// These checks run only after ordinary type/argument validation has succeeded.
// Unsupported private storage is diagnosed at the consuming source expression
// before the service/CLI advertises a compilable program. Lowering retains its
// independent fail-closed checks for callers that bypass checked analysis.
func (a *Analyzer) checkTaggedExpressionBoundary(expr syntax.Expression, value types.Type, scope *Scope) {
	switch node := expr.(type) {
	case *syntax.CallExpr:
		if node.Block != nil {
			return // A compiler-owned scoped block returns its authored value.
		}
		if taggedRawCall(node) {
			if node.PrimaryArg != nil && a.checkTaggedRawReference(node.PrimaryArg, scope) {
				return
			}
			for _, argument := range node.NamedArgs {
				if a.checkTaggedRawReference(argument.Value, scope) {
					return
				}
			}
			return
		}
		action := a.resolvedActions[node]
		if action == nil {
			return // User functions have their own typed envelope boundary.
		}
		name := action.CallableName
		if identifier, ok := node.Callee.(*syntax.IdentExpr); ok {
			name = identifier.Name
		}
		if name == "count" && schema.LoopResultItemsCountConsumer(node, action) && node.PrimaryArg != nil {
			switch unwrapOptionalType(a.taggedExpressionType(node.PrimaryArg, scope)).(type) {
			case types.ListType, types.MapType:
				// Preserve the native Items Count of the existing outer
				// content object. No cell payload or Map-key conversion.
				return
			}
		}
		arguments := []syntax.Expression{node.PrimaryArg}
		for _, argument := range node.NamedArgs {
			arguments = append(arguments, argument.Value)
		}
		for _, argument := range arguments {
			if argument != nil && types.PrivateValueTransport(a.taggedExpressionType(argument, scope)) {
				a.error(CodeTargetValueShape, argument.NodeSpan(), fmt.Sprintf("%s requires an explicit native/JSON conversion for structured values", name))
				return
			}
		}
		if types.PrivateValueTransport(value) {
			a.error(CodeTargetValueShape, node.Span, fmt.Sprintf("native %s cannot manufacture a compiler-owned structured result", name))
		}
	case *syntax.FStringExpr:
		for _, part := range node.Parts {
			if part.IsExpr && types.PrivateValueTransport(a.taggedExpressionType(part.Expr, scope)) {
				a.error(CodeTargetValueShape, part.Expr.NodeSpan(), "structured interpolation requires an explicit text/JSON codec")
				return
			}
		}
	case *syntax.ListExpr:
		if a.rawNativeDataContext || a.inlineDictionaryArray || types.TaggedListElements(value) {
			return
		}
		// An AnyContent annotation can hide a known native List. It does not
		// enroll that opaque source into the new typed-cell representation.
		for _, element := range node.Elements {
			if a.isNativeListElement(element, a.exprTypes[element], scope) {
				a.error(CodeNestedNativeList, element.NodeSpan(), "nested List-valued element has no verified native content-row encoding; preserve nested arrays in Dictionary fields")
			}
		}
	case *syntax.MapExpr:
		if a.rawNativeDataContext {
			return
		}
		for _, entry := range node.Entries {
			if !types.InlineDataNeedsCells(a.taggedExpressionType(entry.Value, scope)) {
				if a.checkTaggedInlineReference(entry.Value, scope) {
					return
				}
			}
		}
	}
}

func taggedRawCall(call *syntax.CallExpr) bool {
	if name, ok := call.Callee.(*syntax.IdentExpr); ok {
		return name.Name == "rawAction" || name.Name == "action"
	}
	member, ok := call.Callee.(*syntax.MemberExpr)
	if !ok {
		return false
	}
	namespace, ok := member.Target.(*syntax.IdentExpr)
	return ok && namespace.Name == "native" &&
		(member.Property == "action" || member.Property == "plist" || member.Property == "ref" || member.Property == "workflow")
}

func (a *Analyzer) checkTaggedRawReference(expr syntax.Expression, scope *Scope) bool {
	switch value := expr.(type) {
	case *syntax.LiteralExpr:
		return false // Literal raw null already has E_NATIVE_JSON_NULL.
	case *syntax.MapExpr:
		for _, entry := range value.Entries {
			if a.checkTaggedRawReference(entry.Key, scope) || a.checkTaggedRawReference(entry.Value, scope) {
				return true
			}
		}
		return false
	case *syntax.ListExpr:
		for _, element := range value.Elements {
			if a.checkTaggedRawReference(element, scope) {
				return true
			}
		}
		return false
	}
	if types.PrivateValueTransport(a.taggedExpressionType(expr, scope)) {
		a.error(CodeTargetValueShape, expr.NodeSpan(), "raw native metadata cannot consume a private structured value")
		return true
	}
	return false
}

// Literal Dictionary arrays have an established recursive native codec.
// A runtime private List is a different representation, even at that location.
func (a *Analyzer) checkTaggedInlineReference(expr syntax.Expression, scope *Scope) bool {
	switch value := expr.(type) {
	case *syntax.ListExpr:
		for _, element := range value.Elements {
			if a.checkTaggedInlineReference(element, scope) {
				return true
			}
		}
		return false
	case *syntax.MapExpr:
		return false // Each nested Map was checked in its own analyzed context.
	}
	if types.PrivateValueTransport(a.taggedExpressionType(expr, scope)) {
		a.error(CodeTargetValueShape, expr.NodeSpan(), "dynamic native value requires an explicit Dictionary-array conversion")
		return true
	}
	return false
}

func (a *Analyzer) checkTaggedMenuValue(expr syntax.Expression, scope *Scope) {
	if expr != nil && types.PrivateValueTransport(a.taggedExpressionType(expr, scope)) {
		a.error(CodeTargetValueShape, expr.NodeSpan(), "menu label requires an explicit text/JSON codec")
	}
}
