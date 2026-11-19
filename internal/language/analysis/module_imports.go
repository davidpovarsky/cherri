package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// SetModuleGraph supplies the same authorized, parsed dependency graph used by lowering.
func (a *Analyzer) SetModuleGraph(graph *modules.Graph) {
	a.moduleGraph, a.moduleNode = nil, nil
	if graph != nil && modules.HasImports(graph.Root.Program) {
		a.moduleGraph = graph
		a.moduleNode = graph.Root
	}
}

func (a *Analyzer) ModuleRegistries() map[*modules.Module]*schema.Registry {
	result := map[*modules.Module]*schema.Registry{}
	if a.moduleGraph == nil {
		return result
	}
	result[a.moduleGraph.Root] = a.registry
	for module, analyzer := range a.moduleAnalyses {
		result[module] = analyzer.Registry()
	}
	return result
}

func (a *Analyzer) analyzeModuleDependencies() {
	if a.moduleGraph == nil || a.moduleNode != a.moduleGraph.Root {
		return
	}
	a.moduleAnalyses = map[*modules.Module]*Analyzer{}
	target := NewAnalyzer(a.baseRegistry)
	target.configuredTarget = a.configuredTarget
	target.beginProgram()
	for _, declaration := range a.moduleGraph.Root.Program.Declarations {
		if header, ok := declaration.(*syntax.ShortcutDecl); ok {
			target.analyzeShortcutDecl(header)
		}
	}
	config := analyzerTargetConfig{
		isMac: cloneTargetBool(target.isMacTarget), targets: append([]string(nil), target.targets...),
		explicit: target.hasExplicitTarget, min: target.minTargetVersion, max: target.maxTargetVersion,
		minSource: target.minTargetVersionSource, maxSource: target.maxTargetVersionSource,
	}
	for _, module := range a.moduleGraph.Modules {
		if module == a.moduleGraph.Root || module.Builtin != "" {
			continue
		}
		child := NewAnalyzer(a.baseRegistry)
		child.configuredTarget = config
		child.moduleGraph, child.moduleNode = a.moduleGraph, module
		child.moduleAnalyses = a.moduleAnalyses
		child.Analyze(module.Program)
		a.moduleAnalyses[module] = child
		a.diagnostics = append(a.diagnostics, child.Diagnostics()...)
	}
}

func (a *Analyzer) mergeModuleExpressionResults() {
	if a.moduleGraph == nil || a.moduleNode != a.moduleGraph.Root {
		return
	}
	for _, child := range a.moduleAnalyses {
		for expression, result := range child.exprTypes {
			a.exprTypes[expression] = result
		}
		for expression, value := range child.enumValues {
			a.enumValues[expression] = value
		}
		for expression, value := range child.enumContexts {
			a.enumContexts[expression] = value
		}
	}
}

func (a *Analyzer) defineModuleImport(scope *Scope, declaration *syntax.ImportDecl) {
	if a.moduleNode == nil {
		return
	}
	module := a.moduleNode.Imports[declaration]
	if module == nil {
		return
	}
	if symbol, ok := scope.LookupLocal(declaration.Alias); ok {
		symbol.Module = module
	}
	registerAction := func(name string, original *schema.ActionSchema) {
		action := *original
		action.ID, action.CallableName = name, name
		a.registry.RegisterCustomAction(&action)
	}
	if module.Builtin != "" {
		for _, action := range a.baseRegistry.AllActions() {
			if action.Module == module.Builtin {
				registerAction(declaration.Alias+"."+action.CallableName, action)
			}
		}
		return
	}
	child := a.moduleAnalyses[module]
	if child == nil || child.moduleScope == nil {
		a.error("E_MODULE_DEPENDENCY", declaration.Span, "resolved module has no analyzed local namespace")
		return
	}
	for _, exported := range module.Program.Declarations {
		switch exported := exported.(type) {
		case *syntax.FunctionDecl:
			symbol, ok := child.moduleScope.LookupLocal(exported.Name)
			if !ok {
				continue
			}
			copy := *symbol
			copy.Name = declaration.Alias + "." + exported.Name
			scope.Define(&copy)
			a.userFunctions[copy.Name] = true
			if module.Key == "builtin:stdfunc" {
				copy := *symbol
				scope.Define(&copy)
				a.userFunctions[copy.Name] = true
			}
		case *syntax.ActionDecl:
			if action, ok := child.registry.LookupAction(exported.Name); ok {
				registerAction(declaration.Alias+"."+exported.Name, action)
			}
		}
	}
}

func (a *Analyzer) moduleCallName(callee syntax.Expression, scope *Scope) (string, bool) {
	var target, property string
	switch member := callee.(type) {
	case *syntax.MemberExpr:
		identifier, ok := member.Target.(*syntax.IdentExpr)
		if !ok {
			return "", false
		}
		target, property = identifier.Name, member.Property
	case *syntax.EnumMemberExpr:
		// Capitalization is only a parser hint. The bound import symbol decides
		// whether this qualified call belongs to a module or to an enum.
		target, property = member.TypeName, member.Member
	default:
		return "", false
	}
	symbol, ok := scope.Lookup(target)
	if !ok || symbol.Module == nil {
		return "", false
	}
	name := target + "." + property
	if function, ok := scope.Lookup(name); ok && function.Function != nil {
		return name, true
	}
	if _, ok := a.registry.LookupAction(name); ok {
		return name, true
	}
	a.error(CodeUnknownAction, callee.NodeSpan(), fmt.Sprintf("module %q does not export callable %q", target, property))
	return "", true
}

// RegistryForExpression returns the namespace that actually analyzed this AST node.
// Returned registries are read-only; action/function shadowing is resolved separately.
func (a *Analyzer) RegistryForExpression(expression syntax.Expression) *schema.Registry {
	for _, child := range a.moduleAnalyses {
		if _, owned := child.exprTypes[expression]; owned {
			return child.Registry()
		}
	}
	return a.Registry()
}
