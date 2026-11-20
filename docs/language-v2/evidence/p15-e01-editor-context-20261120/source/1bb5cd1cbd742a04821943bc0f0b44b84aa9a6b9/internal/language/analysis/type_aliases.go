package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

type typeAliasBinding struct {
	declaration *syntax.TypeDecl
	owner       *Analyzer
}

// TypeAnnotations returns the semantic results for these exact AST nodes.
// Imported declarations retain the result from their defining module; callers
// and lowering must not reinterpret an alias in a different module namespace.
func (a *Analyzer) TypeAnnotations() map[syntax.TypeAnnotation]types.Type {
	result := make(map[syntax.TypeAnnotation]types.Type)
	for annotation, value := range a.typeAnnotations {
		result[annotation] = value
	}
	if a.moduleGraph != nil {
		for _, child := range a.moduleAnalyses {
			for annotation, value := range child.typeAnnotations {
				result[annotation] = value
			}
		}
	}
	return result
}

func (a *Analyzer) prepareTypeAliases(program *syntax.Program) {
	a.typeAliases = make(map[string]typeAliasBinding)
	a.resolvingTypeAliases = make(map[*syntax.TypeDecl]bool)
	for _, declaration := range program.Declarations {
		if declaration, ok := declaration.(*syntax.TypeDecl); ok {
			if _, duplicate := a.typeAliases[declaration.Name]; duplicate {
				a.error(CodeTypeAlias, declaration.Span, fmt.Sprintf("duplicate type alias %q", declaration.Name))
				continue
			}
			a.typeAliases[declaration.Name] = typeAliasBinding{declaration, a}
		}
	}
	if a.moduleNode != nil {
		for _, declaration := range program.Declarations {
			imported, ok := declaration.(*syntax.ImportDecl)
			if !ok {
				continue
			}
			module := a.moduleNode.Imports[imported]
			child := a.moduleAnalyses[module]
			if child == nil {
				continue
			}
			for _, exported := range module.Program.Declarations {
				if exported, ok := exported.(*syntax.TypeDecl); ok {
					a.typeAliases[imported.Alias+"."+exported.Name] = typeAliasBinding{exported, child}
				}
			}
		}
	}
	// Validate every declaration, including unused aliases, in source order.
	// Collecting first makes a forward alias independent of declaration order.
	for _, declaration := range program.Declarations {
		if declaration, ok := declaration.(*syntax.TypeDecl); ok {
			a.resolveTypeAlias(declaration)
		}
	}
}

func (a *Analyzer) resolveTypeAlias(declaration *syntax.TypeDecl) types.Type {
	if a.resolvingTypeAliases[declaration] {
		a.error(CodeTypeAlias, declaration.Span, fmt.Sprintf("cyclic type alias %q", declaration.Name))
		return types.Unknown
	}
	a.resolvingTypeAliases[declaration] = true
	defer delete(a.resolvingTypeAliases, declaration)
	return a.resolveType(declaration.TypeExpr)
}
