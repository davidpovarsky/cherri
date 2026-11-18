package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/enums"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// EnumValues records proven literal values without rewriting the source AST.
// Map membership, not a nonempty string, distinguishes a resolved empty value.
func (a *Analyzer) EnumValues() map[syntax.Expression]string { return a.enumValues }

// EnumContexts exposes the expected domain of an expression to the shared service.
func (a *Analyzer) EnumContexts() map[syntax.Expression]string { return a.enumContexts }

func (a *Analyzer) prepareEnums(program *syntax.Program) {
	a.enumCatalog = enums.New(a.registry, program)
	a.enumValues = make(map[syntax.Expression]string)
	a.enumContexts = make(map[syntax.Expression]string)
	a.enumErrors = make(map[syntax.Expression]bool)
}

func (a *Analyzer) markEnumContext(expression syntax.Expression, name string) {
	if expression == nil || name == "" {
		return
	}
	a.enumContexts[expression] = name
	switch expression := expression.(type) {
	case *syntax.IfStmt:
		a.markEnumContext(syntax.BlockResultExpression(expression.ThenBlock), name)
		switch other := expression.ElseBlock.(type) {
		case *syntax.BlockStmt:
			a.markEnumContext(syntax.BlockResultExpression(other), name)
		case *syntax.IfStmt:
			a.markEnumContext(other, name)
		}
	case *syntax.MenuStmt:
		for _, item := range expression.Cases {
			a.markEnumContext(syntax.BlockResultExpression(item.BodyBlock), name)
		}
	case *syntax.CallExpr:
		if expression.Block != nil {
			a.markEnumContext(syntax.BlockResultExpression(expression.Block.Body), name)
		}
	}
}

func (a *Analyzer) analyzeEnumContext(expression syntax.Expression, scope *Scope, name string) types.Type {
	a.markEnumContext(expression, name)
	result := a.analyzeExpression(expression, scope)
	if name != "" {
		a.validateEnumValue(expression, name, a.enumCatalog.Values(name))
	}
	return result
}

func (a *Analyzer) enumDiagnostic(expression syntax.Expression, message string) {
	if !a.enumErrors[expression] {
		a.error(CodeEnumMember, expression.NodeSpan(), message)
		a.enumErrors[expression] = true
	}
}

// resolveEnumExpression also recognizes lowercase-qualified native/source enums.
// A lexical value binding takes precedence over a same-named enum qualifier.
func (a *Analyzer) resolveEnumExpression(expression syntax.Expression, scope *Scope) bool {
	var name, member string
	switch expression := expression.(type) {
	case *syntax.EnumMemberExpr:
		name, member = expression.TypeName, expression.Member
	case *syntax.MemberExpr:
		target, ok := expression.Target.(*syntax.IdentExpr)
		if !ok || !a.enumCatalog.Has(target.Name) {
			return false
		}
		name, member = target.Name, expression.Property
	default:
		return false
	}
	if name != "" {
		if symbol, ok := scope.Lookup(name); ok && !symbol.EnumDeclaration {
			return false
		}
	}
	expected := a.enumContexts[expression]
	if name == "" {
		name = expected
	}
	if name == "" {
		a.enumDiagnostic(expression, fmt.Sprintf("enum member .%s requires an enum parameter or type annotation", member))
		return true
	}
	if !a.enumCatalog.Has(name) {
		a.enumDiagnostic(expression, fmt.Sprintf("unknown enum %q", name))
		return true
	}
	if expected != "" && name != expected {
		a.enumDiagnostic(expression, fmt.Sprintf("enum %q is not the expected enum %q", name, expected))
		return true
	}
	value, ok := a.enumCatalog.Resolve(name, member)
	if !ok {
		a.enumDiagnostic(expression, fmt.Sprintf("enum %q has no member %q", name, member))
		return true
	}
	a.enumValues[expression] = value
	return true
}

func (a *Analyzer) enumLiteralValue(expression syntax.Expression) (string, bool) {
	if value, ok := a.enumValues[expression]; ok {
		return value, true
	}
	return literalTextValue(expression)
}

func (a *Analyzer) validateEnumValue(expression syntax.Expression, name string, values []string) {
	if expression == nil || a.enumErrors[expression] {
		return
	}
	value, literal := a.enumLiteralValue(expression)
	if !literal {
		return
	}
	for _, candidate := range values {
		if value == candidate {
			return
		}
	}
	a.enumDiagnostic(expression, fmt.Sprintf("invalid enum value %q for %q", value, name))
}

func (a *Analyzer) analyzeEnumArgument(param *schema.ParameterSchema, expression syntax.Expression, scope *Scope, callName string) types.Type {
	a.markEnumContext(expression, param.EnumName)
	result := a.analyzeExpression(expression, scope)
	a.checkAcceptedForms(param, expression, callName)
	if param.EnumName != "" || len(param.EnumValues) > 0 {
		a.validateEnumValue(expression, param.EnumName, param.EnumValues)
	}
	return result
}
