package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// KnownPresentDictionaryIndexes carries checked source facts for exact AST
// nodes. It does not infer key existence from native content or its truthiness.
// Imported function bodies retain their defining module's analysis.
func (a *Analyzer) KnownPresentDictionaryIndexes() map[syntax.Expression]bool {
	result := make(map[syntax.Expression]bool)
	for expression, present := range a.dictionaryIndexes {
		if present {
			result[expression] = true
		}
	}
	if a.moduleGraph != nil {
		for _, child := range a.moduleAnalyses {
			for expression, present := range child.dictionaryIndexes {
				if present {
					result[expression] = true
				}
			}
		}
	}
	return result
}

// A Dictionary origin survives immutable aliases and the checked canonical
// require passthrough. Mutable bindings and arbitrary calls cannot supply it.
// Facts are recorded after all children have been analyzed, without evaluating
// source a second time or rewriting any expression.
func (a *Analyzer) recordDictionaryProvenance(expression syntax.Expression, scope *Scope) {
	if a.dictionaryOrigins == nil {
		a.dictionaryOrigins = make(map[syntax.Expression]*syntax.MapExpr)
	}
	if a.dictionaryIndexes == nil {
		a.dictionaryIndexes = make(map[syntax.Expression]bool)
	}
	delete(a.dictionaryOrigins, expression)
	delete(a.dictionaryIndexes, expression)
	switch node := expression.(type) {
	case *syntax.MapExpr:
		a.dictionaryOrigins[expression] = node
	case *syntax.IdentExpr:
		if symbol, found := scope.Lookup(node.Name); found && symbol.Initialized && !symbol.Mutable {
			a.dictionaryOrigins[expression] = symbol.dictionaryOrigin
		}
	case *syntax.CallExpr:
		action := a.resolvedActions[node]
		if action == nil || !action.CompilerConstruct || action.CallableName != "require" {
			return
		}
		input := node.PrimaryArg
		for _, argument := range node.NamedArgs {
			if argument.Label == "value" && input == nil {
				input = argument.Value
			}
		}
		a.dictionaryOrigins[expression] = a.dictionaryOrigins[input]
	case *syntax.IndexExpr:
		element, found := syntax.StaticDictionaryElement(a.dictionaryOrigins[node.Target], node.Index)
		if found && types.DefinitelyPresent(a.exprTypes[element]) {
			a.dictionaryIndexes[expression] = true
		}
	}
}
