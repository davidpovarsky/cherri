package migrate

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// #ref describes an inline native parameter value. A V2 let would materialize
// that value as a Text action, replacing the reference payload with an output
// attachment. Substitute the decoded constant through lexical AST identity,
// leaving same-named parameters, locals and loop variables untouched.
func inlineMigrationReferences(program *syntax.Program, references map[string]string, generatedCount int) error {
	if len(references) == 0 {
		return nil
	}
	if generatedCount > len(program.Statements) {
		return fmt.Errorf("generated reference declarations are missing")
	}
	for _, statement := range program.Statements[:generatedCount] {
		binding, okay := statement.(*syntax.BindingStmt)
		if !okay || binding.Mutable {
			return fmt.Errorf("generated reference declaration changed shape")
		}
		call, okay := binding.Value.(*syntax.CallExpr)
		if !okay || len(call.NamedArgs) != 0 {
			return fmt.Errorf("generated reference value changed shape")
		}
		member, okay := call.Callee.(*syntax.MemberExpr)
		if !okay || member.Property != "plist" {
			return fmt.Errorf("generated reference is no longer native.plist")
		}
		target, okay := member.Target.(*syntax.IdentExpr)
		if !okay || target.Name != "native" {
			return fmt.Errorf("generated reference is no longer native.plist")
		}
		literal, okay := call.PrimaryArg.(*syntax.LiteralExpr)
		payload, exists := references[binding.Name]
		if !okay || !exists || literal.Type != syntax.TokenString || literal.Value != payload {
			return fmt.Errorf("generated reference is detached from its decoded directive")
		}
	}
	program.Statements = program.Statements[generatedCount:]
	var expression func(syntax.Expression, *migrationBindingScope) syntax.Expression
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
	expression = func(value syntax.Expression, scope *migrationBindingScope) syntax.Expression {
		switch e := value.(type) {
		case *syntax.IdentExpr:
			for current := scope; current != nil; current = current.parent {
				if _, shadowed := current.names[e.Name]; shadowed {
					return value
				}
			}
			if payload, found := references[e.Name]; found {
				return &syntax.CallExpr{Span: e.Span,
					Callee:     &syntax.MemberExpr{Span: e.Span, Target: &syntax.IdentExpr{Span: e.Span, Name: "native"}, Property: "plist"},
					PrimaryArg: &syntax.LiteralExpr{Span: e.Span, Type: syntax.TokenString, Value: payload},
				}
			}
		case *syntax.CallExpr:
			e.Callee = expression(e.Callee, scope)
			e.PrimaryArg = expression(e.PrimaryArg, scope)
			for index := range e.NamedArgs {
				e.NamedArgs[index].Value = expression(e.NamedArgs[index].Value, scope)
			}
			if e.Block != nil {
				block(e.Block.Body, scope)
			}
		case *syntax.FStringExpr:
			for index := range e.Parts {
				if e.Parts[index].IsExpr {
					e.Parts[index].Expr = expression(e.Parts[index].Expr, scope)
				}
			}
		case *syntax.BinaryExpr:
			e.Left, e.Right = expression(e.Left, scope), expression(e.Right, scope)
		case *syntax.UnaryExpr:
			e.Operand = expression(e.Operand, scope)
		case *syntax.MemberExpr:
			e.Target = expression(e.Target, scope)
		case *syntax.IndexExpr:
			e.Target, e.Index = expression(e.Target, scope), expression(e.Index, scope)
		case *syntax.ListExpr:
			for index := range e.Elements {
				e.Elements[index] = expression(e.Elements[index], scope)
			}
		case *syntax.MapExpr:
			for index := range e.Entries {
				e.Entries[index].Key = expression(e.Entries[index].Key, scope)
				e.Entries[index].Value = expression(e.Entries[index].Value, scope)
			}
		case *syntax.RecordExpr:
			for index := range e.Fields {
				e.Fields[index].Value = expression(e.Fields[index].Value, scope)
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
		return value
	}
	statement = func(value syntax.Statement, scope *migrationBindingScope) {
		switch s := value.(type) {
		case *syntax.BindingStmt:
			s.Value = expression(s.Value, scope)
			scope.names[s.Name] = s
		case *syntax.AssignStmt:
			s.Value = expression(s.Value, scope)
		case *syntax.BlockStmt:
			block(s, scope)
		case *syntax.ExprStmt:
			s.Expr = expression(s.Expr, scope)
		case *syntax.ReturnStmt:
			s.Value = expression(s.Value, scope)
		case *syntax.YieldStmt:
			s.Value = expression(s.Value, scope)
		case *syntax.IfStmt:
			s.Condition = expression(s.Condition, scope)
			block(s.ThenBlock, scope)
			if s.ElseBlock != nil {
				statement(s.ElseBlock, scope)
			}
		case *syntax.ForStmt:
			s.Iterable = expression(s.Iterable, scope)
			block(s.BodyBlock, newMigrationBindingScope(scope, s.IndexVar, s.ItemVar, "RepeatIndex", "RepeatItem"))
		case *syntax.RepeatStmt:
			s.Count = expression(s.Count, scope)
			block(s.BodyBlock, newMigrationBindingScope(scope, s.IndexVar, "RepeatIndex"))
		case *syntax.MenuStmt:
			s.Prompt = expression(s.Prompt, scope)
			for index := range s.Cases {
				s.Cases[index].Value = expression(s.Cases[index].Value, scope)
				block(s.Cases[index].BodyBlock, scope)
			}
		case *syntax.CopyBlockStmt:
			block(s.Body, scope)
		}
	}
	root := newMigrationBindingScope(nil)
	for _, declaration := range program.Declarations {
		switch d := declaration.(type) {
		case *syntax.ImportDecl:
			root.names[d.Alias] = nil
		case *syntax.SetupDecl:
			root.names[d.Name] = nil
		}
	}
	for _, child := range program.Statements {
		statement(child, root)
	}
	for _, declaration := range program.Declarations {
		switch d := declaration.(type) {
		case *syntax.FunctionDecl:
			function := newMigrationBindingScope(nil)
			for index := range d.Parameters {
				d.Parameters[index].DefaultExpr = expression(d.Parameters[index].DefaultExpr, function)
				function.names[d.Parameters[index].Name] = nil
			}
			block(d.Body, function)
		case *syntax.CopyBlockStmt:
			block(d.Body, root)
		case *syntax.ShortcutDecl:
			if d.Metadata != nil {
				expression(d.Metadata, root)
			}
		case *syntax.ActionDecl:
			for index := range d.Parameters {
				d.Parameters[index].DefaultExpr = expression(d.Parameters[index].DefaultExpr, root)
			}
		}
	}
	return nil
}
