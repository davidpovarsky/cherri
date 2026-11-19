package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

type moduleScope struct {
	functions map[string]*syntax.FunctionDecl
	enums     map[string]*syntax.EnumDecl
	types     map[string]*syntax.TypeDecl
	actions   map[string]*syntax.ActionDecl
	modules   map[string]string
	registry  *schema.Registry
}

func (l *Lowerer) SetModuleGraph(graph *modules.Graph) { l.moduleGraph = graph }

func (l *Lowerer) SetModuleRegistries(registries map[*modules.Module]*schema.Registry) {
	l.moduleRegistries = registries
}

func (l *Lowerer) ensureModuleGraph(program *syntax.Program) error {
	if l.moduleGraph != nil {
		return nil
	}
	graph, issue := modules.Resolve(program, l.baseFilePath, l.virtualModules, l.registry, migrate.MigrateSource)
	if issue != nil {
		return issue
	}
	l.moduleGraph = graph
	return nil
}

func (l *Lowerer) prepareModuleScopes() {
	l.moduleScopes = map[*modules.Module]*moduleScope{}
	l.functionModules = map[*syntax.FunctionDecl]*modules.Module{}
	l.functionDispatchNames = map[*syntax.FunctionDecl]string{}
	l.importedFunctions = nil
	for _, module := range l.moduleGraph.Modules {
		scope := &moduleScope{
			functions: map[string]*syntax.FunctionDecl{}, enums: map[string]*syntax.EnumDecl{},
			types: map[string]*syntax.TypeDecl{}, actions: map[string]*syntax.ActionDecl{},
			modules: map[string]string{}, registry: l.registry,
		}
		if registry := l.moduleRegistries[module]; registry != nil {
			scope.registry = registry
		}
		l.moduleScopes[module] = scope
		for _, declaration := range module.Program.Declarations {
			switch declaration := declaration.(type) {
			case *syntax.FunctionDecl:
				scope.functions[declaration.Name] = declaration
				l.functionModules[declaration] = module
				name := declaration.Name
				if module != l.moduleGraph.Root {
					name = "module:" + module.Key + "::" + declaration.Name
					l.importedFunctions = append(l.importedFunctions, declaration)
				}
				l.functionDispatchNames[declaration] = name
			case *syntax.EnumDecl:
				scope.enums[declaration.Name] = declaration
			case *syntax.TypeDecl:
				scope.types[declaration.Name] = declaration
			case *syntax.ActionDecl:
				scope.actions[declaration.Name] = declaration
			}
		}
		for _, declaration := range module.Program.Declarations {
			imported, ok := declaration.(*syntax.ImportDecl)
			if !ok {
				continue
			}
			dependency := module.Imports[imported]
			if dependency == nil {
				continue
			}
			scope.modules[imported.Alias] = dependency.Key
			if dependency.Builtin != "" {
				continue
			}
			child := l.moduleScopes[dependency]
			for _, exported := range dependency.Program.Declarations {
				switch exported := exported.(type) {
				case *syntax.FunctionDecl:
					scope.functions[imported.Alias+"."+exported.Name] = child.functions[exported.Name]
					if dependency.Key == "builtin:stdfunc" && scope.functions[exported.Name] == nil {
						scope.functions[exported.Name] = child.functions[exported.Name]
					}
				case *syntax.EnumDecl:
					scope.enums[imported.Alias+"."+exported.Name] = child.enums[exported.Name]
				case *syntax.TypeDecl:
					scope.types[imported.Alias+"."+exported.Name] = child.types[exported.Name]
				case *syntax.ActionDecl:
					scope.actions[imported.Alias+"."+exported.Name] = child.actions[exported.Name]
				}
			}
		}
	}
	l.useModuleScope(l.moduleGraph.Root)
}

func (l *Lowerer) useModuleScope(module *modules.Module) {
	scope := l.moduleScopes[module]
	if scope == nil {
		return
	}
	l.functions, l.enums, l.types = scope.functions, scope.enums, scope.types
	l.actions, l.modules, l.registry = scope.actions, scope.modules, scope.registry
}

func (l *Lowerer) functionDispatchName(function *syntax.FunctionDecl) string {
	if name := l.functionDispatchNames[function]; name != "" {
		return name
	}
	return function.Name
}

func (l *Lowerer) lowerImport(declaration *syntax.ImportDecl) error {
	if l.moduleGraph == nil || l.moduleGraph.Root.Imports[declaration] == nil {
		return fmt.Errorf("cannot resolve module %q: missing authorized dependency graph", declaration.Path)
	}
	return nil
}
