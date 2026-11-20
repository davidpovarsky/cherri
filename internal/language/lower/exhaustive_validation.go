package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Lowering may legitimately omit unused function/copy bodies and consume
// metadata without evaluating it. Validate node kinds before those paths can
// hide an unsupported AST node or before a native session receives actions.
func validateLoweringProgram(program *syntax.Program) error {
	if program == nil {
		return fmt.Errorf("cannot lower a nil program")
	}
	for _, declaration := range program.Declarations {
		if err := validateLoweringDeclaration(declaration); err != nil {
			return err
		}
	}
	for _, statement := range program.Statements {
		if err := validateLoweringStatement(statement); err != nil {
			return err
		}
	}
	return nil
}

func validateLoweringDeclaration(declaration syntax.Declaration) error {
	switch node := declaration.(type) {
	case *syntax.ImportDecl, *syntax.TriggerDecl, *syntax.SetupDecl, *syntax.TypeDecl, *syntax.EnumDecl:
		return nil
	case *syntax.ShortcutDecl:
		if node.Metadata != nil {
			if err := validateLoweringExpression(node.Metadata); err != nil {
				return err
			}
		}
		for index := range node.Triggers {
			if err := validateLoweringDeclaration(&node.Triggers[index]); err != nil {
				return err
			}
		}
		return nil
	case *syntax.FunctionDecl:
		for _, parameter := range node.Parameters {
			if err := validateLoweringExpression(parameter.DefaultExpr); err != nil {
				return err
			}
		}
		return validateLoweringBlock(node.Body)
	case *syntax.ActionDecl:
		for _, parameter := range node.Parameters {
			if err := validateLoweringExpression(parameter.DefaultExpr); err != nil {
				return err
			}
		}
		return nil
	case *syntax.CopyBlockStmt:
		return validateLoweringBlock(node.Body)
	default:
		return fmt.Errorf("unhandled declaration %T: declarations must not be silently dropped", declaration)
	}
}

func validateLoweringBlock(block *syntax.BlockStmt) error {
	if block == nil {
		return nil
	}
	for _, statement := range block.Statements {
		if err := validateLoweringStatement(statement); err != nil {
			return err
		}
	}
	return nil
}

func validateLoweringStatement(statement syntax.Statement) error {
	switch node := statement.(type) {
	case *syntax.BindingStmt:
		return validateLoweringExpression(node.Value)
	case *syntax.AssignStmt:
		return validateLoweringExpression(node.Value)
	case *syntax.ExprStmt:
		return validateLoweringExpression(node.Expr)
	case *syntax.ReturnStmt:
		return validateLoweringExpression(node.Value)
	case *syntax.YieldStmt:
		return validateLoweringExpression(node.Value)
	case *syntax.BlockStmt:
		return validateLoweringBlock(node)
	case *syntax.IfStmt:
		if err := validateLoweringExpression(node.Condition); err != nil {
			return err
		}
		if err := validateLoweringBlock(node.ThenBlock); err != nil {
			return err
		}
		if node.ElseBlock != nil {
			return validateLoweringStatement(node.ElseBlock)
		}
		return nil
	case *syntax.ForStmt:
		if err := validateLoweringExpression(node.Iterable); err != nil {
			return err
		}
		return validateLoweringBlock(node.BodyBlock)
	case *syntax.RepeatStmt:
		if err := validateLoweringExpression(node.Count); err != nil {
			return err
		}
		return validateLoweringBlock(node.BodyBlock)
	case *syntax.MenuStmt:
		if err := validateLoweringExpression(node.Prompt); err != nil {
			return err
		}
		for _, entry := range node.Cases {
			if err := validateLoweringExpression(entry.Value); err != nil {
				return err
			}
			if err := validateLoweringBlock(entry.BodyBlock); err != nil {
				return err
			}
		}
		return nil
	case *syntax.CopyBlockStmt:
		return validateLoweringBlock(node.Body)
	case *syntax.PasteStmt:
		return nil
	default:
		return fmt.Errorf("unhandled statement %T: statements must not be silently dropped", statement)
	}
}

func validateLoweringExpression(expression syntax.Expression) error {
	if expression == nil {
		return nil
	}
	switch node := expression.(type) {
	case *syntax.LiteralExpr, *syntax.IdentExpr, *syntax.EnumMemberExpr:
		return nil
	case *syntax.FStringExpr:
		for _, part := range node.Parts {
			if part.IsExpr {
				if err := validateLoweringExpression(part.Expr); err != nil {
					return err
				}
			}
		}
		return nil
	case *syntax.MemberExpr:
		return validateLoweringExpression(node.Target)
	case *syntax.IndexExpr:
		if err := validateLoweringExpression(node.Target); err != nil {
			return err
		}
		return validateLoweringExpression(node.Index)
	case *syntax.CallExpr:
		if err := validateLoweringExpression(node.Callee); err != nil {
			return err
		}
		if err := validateLoweringExpression(node.PrimaryArg); err != nil {
			return err
		}
		for _, argument := range node.NamedArgs {
			if err := validateLoweringExpression(argument.Value); err != nil {
				return err
			}
		}
		if node.Block != nil {
			return validateLoweringBlock(node.Block.Body)
		}
		return nil
	case *syntax.UnaryExpr:
		return validateLoweringExpression(node.Operand)
	case *syntax.BinaryExpr:
		if err := validateLoweringExpression(node.Left); err != nil {
			return err
		}
		return validateLoweringExpression(node.Right)
	case *syntax.ListExpr:
		for _, element := range node.Elements {
			if err := validateLoweringExpression(element); err != nil {
				return err
			}
		}
		return nil
	case *syntax.MapExpr:
		for _, entry := range node.Entries {
			if err := validateLoweringExpression(entry.Key); err != nil {
				return err
			}
			if err := validateLoweringExpression(entry.Value); err != nil {
				return err
			}
		}
		return nil
	case *syntax.RecordExpr:
		for _, field := range node.Fields {
			if err := validateLoweringExpression(field.Value); err != nil {
				return err
			}
		}
		return nil
	case *syntax.IfStmt:
		return validateLoweringStatement(node)
	case *syntax.ForStmt:
		return validateLoweringStatement(node)
	case *syntax.RepeatStmt:
		return validateLoweringStatement(node)
	case *syntax.MenuStmt:
		return validateLoweringStatement(node)
	default:
		return fmt.Errorf("unhandled expression %T: expressions must not be silently dropped", expression)
	}
}
