package migrate

import (
	"sort"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A Legacy mutable variable may be explicitly cleared. Carry that contract into
// its own V2 declaration rather than weakening authored assignment evidence.
// Origins identify generated declarations, not whole names or whole documents.
func preserveLegacyOptionalBindings(program *syntax.Program, text string, origins map[int]string, headerLines int, registry *schema.Registry) {
	if len(origins) == 0 {
		return
	}
	originals := make(map[int]*syntax.BindingStmt)
	hasClear := false
	walkMigrationBindings(program, func(binding *syntax.BindingStmt) {
		if binding.Mutable && origins[binding.Span.Start.Line-headerLines] == binding.Name {
			originals[binding.Span.Start.ByteOffset] = binding
		}
	}, func(assignment *syntax.AssignStmt, _ *syntax.BindingStmt) {
		hasClear = hasClear || isExplicitClear(assignment)
	})
	if !hasClear || len(originals) == 0 {
		return
	}

	// Analyze expands copy/paste in place. Use the same canonical parser and
	// analyzer on a separate evidence tree so formatting preserves source form.
	parser := syntax.NewParser(source.NewFile("migration-evidence", "migration.cherri", 1, text))
	evidence := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		return
	}
	analyzer := analysis.NewAnalyzer(registry)
	analyzer.Analyze(evidence)
	expressionTypes := analyzer.ExprTypes()
	walkMigrationBindings(evidence, nil, func(assignment *syntax.AssignStmt, binding *syntax.BindingStmt) {
		if !isExplicitClear(assignment) || binding == nil {
			return
		}
		original := originals[binding.Span.Start.ByteOffset]
		if original == nil || original.Name != binding.Name {
			return
		}
		if _, optional := original.TypeExpr.(*syntax.OptionalTypeAnnotation); optional {
			return
		}
		inner := original.TypeExpr
		if inner == nil {
			valueType := expressionTypes[binding.Value]
			if valueType == nil || valueType.Kind() == types.KindUnknown || valueType.Kind() == types.KindAnyContent {
				return
			}
			inner = migrationTypeAnnotation(valueType, original.Span)
			if inner == nil {
				return
			}
		}
		original.TypeExpr = &syntax.OptionalTypeAnnotation{Span: original.Span, Inner: inner}
	})
}

func isExplicitClear(assignment *syntax.AssignStmt) bool {
	literal, ok := assignment.Value.(*syntax.LiteralExpr)
	return assignment.Op == syntax.TokenAssign && ok && literal.Type == syntax.TokenNone
}

// Structural translation of an already inferred type, never new inference or
// a cast. Unrepresentable types stay unchanged and retain compiler diagnostics.
func migrationTypeAnnotation(value types.Type, span source.Span) syntax.TypeAnnotation {
	if value == nil || value.Kind() == types.KindVoid {
		return nil
	}
	switch t := value.(type) {
	case types.ListType:
		element := migrationTypeAnnotation(t.Element, span)
		if element == nil {
			return nil
		}
		return &syntax.GenericTypeAnnotation{Span: span, Name: "List", TypeArgs: []syntax.TypeAnnotation{element}}
	case types.MapType:
		key, item := migrationTypeAnnotation(t.Key, span), migrationTypeAnnotation(t.Value, span)
		if key == nil || item == nil {
			return nil
		}
		return &syntax.GenericTypeAnnotation{Span: span, Name: "Map", TypeArgs: []syntax.TypeAnnotation{key, item}}
	case types.OptionalType:
		inner := migrationTypeAnnotation(t.Inner, span)
		if inner != nil {
			return &syntax.OptionalTypeAnnotation{Span: span, Inner: inner}
		}
	case types.UnionType:
		members := make([]syntax.TypeAnnotation, 0, len(t.Members))
		for _, member := range t.Members {
			annotation := migrationTypeAnnotation(member, span)
			if annotation == nil {
				return nil
			}
			members = append(members, annotation)
		}
		return &syntax.UnionTypeAnnotation{Span: span, Members: members}
	case types.RecordType:
		names := make([]string, 0, len(t.Fields))
		for name := range t.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		fields := make([]syntax.RecordTypeField, 0, len(names))
		for _, name := range names {
			field := t.Fields[name]
			annotation := migrationTypeAnnotation(field.Type, span)
			if annotation == nil {
				return nil
			}
			fields = append(fields, syntax.RecordTypeField{Name: name, Optional: field.Optional, TypeExpr: annotation})
		}
		return &syntax.RecordTypeAnnotation{Span: span, Fields: fields}
	default:
		if canonical, ok := types.ParseNamedType(value.Name()); ok && canonical.Equals(value) {
			return &syntax.NamedTypeAnnotation{Span: span, Name: value.Name()}
		}
	}
	return nil
}

type migrationBindingScope struct {
	parent *migrationBindingScope
	names  map[string]*syntax.BindingStmt
}

func newMigrationBindingScope(parent *migrationBindingScope, names ...string) *migrationBindingScope {
	scope := &migrationBindingScope{parent: parent, names: make(map[string]*syntax.BindingStmt)}
	for _, name := range names {
		if name != "" {
			scope.names[name] = nil // Parameters and loop variables mask outer bindings.
		}
	}
	return scope
}

func (scope *migrationBindingScope) lookup(name string) *syntax.BindingStmt {
	for current := scope; current != nil; current = current.parent {
		if binding, exists := current.names[name]; exists {
			return binding
		}
	}
	return nil
}

// Follow lexical identity through the canonical AST. Initializers are visited
// before declaration, and function/loop/scoped-block locals cannot capture an
// unrelated same-named outer declaration for Optional annotation.
func walkMigrationBindings(program *syntax.Program, onBinding func(*syntax.BindingStmt), onAssignment func(*syntax.AssignStmt, *syntax.BindingStmt)) {
	var expression func(syntax.Expression, *migrationBindingScope)
	var statement func(syntax.Statement, *migrationBindingScope)
	var block func(*syntax.BlockStmt, *migrationBindingScope)
	block = func(body *syntax.BlockStmt, parent *migrationBindingScope) {
		if body == nil {
			return
		}
		scope := newMigrationBindingScope(parent)
		for _, child := range body.Statements {
			statement(child, scope)
		}
	}
	expression = func(value syntax.Expression, scope *migrationBindingScope) {
		switch e := value.(type) {
		case *syntax.CallExpr:
			expression(e.Callee, scope)
			expression(e.PrimaryArg, scope)
			for _, argument := range e.NamedArgs {
				expression(argument.Value, scope)
			}
			if e.Block != nil {
				block(e.Block.Body, scope)
			}
		case *syntax.FStringExpr:
			for _, part := range e.Parts {
				expression(part.Expr, scope)
			}
		case *syntax.BinaryExpr:
			expression(e.Left, scope)
			expression(e.Right, scope)
		case *syntax.UnaryExpr:
			expression(e.Operand, scope)
		case *syntax.MemberExpr:
			expression(e.Target, scope)
		case *syntax.IndexExpr:
			expression(e.Target, scope)
			expression(e.Index, scope)
		case *syntax.ListExpr:
			for _, item := range e.Elements {
				expression(item, scope)
			}
		case *syntax.MapExpr:
			for _, item := range e.Entries {
				expression(item.Key, scope)
				expression(item.Value, scope)
			}
		case *syntax.RecordExpr:
			for _, field := range e.Fields {
				expression(field.Value, scope)
			}
		case *syntax.IfStmt:
			statement(e, scope)
		case *syntax.ForStmt:
			statement(e, scope)
		case *syntax.RepeatStmt:
			statement(e, scope)
		case *syntax.MenuStmt:
			statement(e, scope)
		}
	}
	statement = func(value syntax.Statement, scope *migrationBindingScope) {
		switch s := value.(type) {
		case *syntax.BindingStmt:
			expression(s.Value, scope)
			if onBinding != nil {
				onBinding(s)
			}
			scope.names[s.Name] = s
		case *syntax.AssignStmt:
			expression(s.Value, scope)
			if onAssignment != nil {
				onAssignment(s, scope.lookup(s.Name))
			}
		case *syntax.BlockStmt:
			block(s, scope)
		case *syntax.ExprStmt:
			expression(s.Expr, scope)
		case *syntax.ReturnStmt:
			expression(s.Value, scope)
		case *syntax.YieldStmt:
			expression(s.Value, scope)
		case *syntax.IfStmt:
			expression(s.Condition, scope)
			block(s.ThenBlock, scope)
			if s.ElseBlock != nil {
				statement(s.ElseBlock, scope)
			}
		case *syntax.ForStmt:
			expression(s.Iterable, scope)
			loop := newMigrationBindingScope(scope, s.IndexVar, s.ItemVar, "RepeatIndex", "RepeatItem")
			block(s.BodyBlock, loop)
		case *syntax.RepeatStmt:
			expression(s.Count, scope)
			loop := newMigrationBindingScope(scope, s.IndexVar, "RepeatIndex")
			block(s.BodyBlock, loop)
		case *syntax.MenuStmt:
			expression(s.Prompt, scope)
			for _, item := range s.Cases {
				expression(item.Value, scope)
				block(item.BodyBlock, scope)
			}
		case *syntax.CopyBlockStmt:
			block(s.Body, scope)
		}
	}
	root := newMigrationBindingScope(nil)
	for _, child := range program.Statements {
		statement(child, root)
	}
	for _, declaration := range program.Declarations {
		switch d := declaration.(type) {
		case *syntax.FunctionDecl:
			function := newMigrationBindingScope(nil)
			for _, parameter := range d.Parameters {
				expression(parameter.DefaultExpr, function)
				function.names[parameter.Name] = nil
			}
			block(d.Body, function)
		case *syntax.CopyBlockStmt:
			block(d.Body, root)
		}
	}
}
