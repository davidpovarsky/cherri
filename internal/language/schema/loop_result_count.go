package schema

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// LoopResultItemsCountBody admits only a closed, inline consumer of private
// loop-result cells. The cells are never a general source List representation.
// A different count mode, aliased collection, user function, raw action, or
// incomplete action schema must keep the ordinary fail-closed loop boundary.
func LoopResultItemsCountBody(call *syntax.CallExpr, action *ActionSchema) *syntax.BlockStmt {
	if !LoopResultItemsCountConsumer(call, action) {
		return nil
	}
	var body *syntax.BlockStmt
	switch loop := call.PrimaryArg.(type) {
	case *syntax.RepeatStmt:
		body = loop.BodyBlock
	case *syntax.ForStmt:
		body = loop.BodyBlock
	default:
		return nil
	}
	if body == nil || !loopResultExplicitStatement(body) {
		return nil
	}
	return body
}

// The same exact schema gate also authorizes Items Count of an enrolled
// local sequence. No call-name-only or alternate count-mode permission.
func LoopResultItemsCountConsumer(call *syntax.CallExpr, action *ActionSchema) bool {
	if call == nil || action == nil || call.Block != nil || len(call.NamedArgs) != 0 ||
		action.ID != "count" || action.CallableName != "count" || action.Module != "" ||
		action.AppleIdentifier != "is.workflow.actions.count" || action.PrimaryParameterID != "input" ||
		action.OutputType == nil || !types.Number.Equals(action.OutputType) ||
		action.AppIntent != nil || action.CompilerConstruct || action.ScopedBlock ||
		len(action.StaticParameters) != 0 || len(action.Parameters) != 2 {
		return false
	}
	input, mode := action.Parameters[0], action.Parameters[1]
	if input.ID != "input" || input.Label != "input" || input.WireKey != "Input" ||
		input.Codec != "token" || input.Optional || input.Variadic || input.Reference ||
		input.Type == nil || input.Type.Kind() != types.KindAnyContent ||
		mode.ID != "type" || mode.Label != "type" || mode.WireKey != "WFCountType" ||
		mode.Codec != "enum" || !mode.Optional || mode.Variadic || mode.Reference ||
		mode.DefaultValue == nil || *mode.DefaultValue != "Items" ||
		mode.Type == nil || !types.Text.Equals(mode.Type) ||
		mode.EnumName != "countType" {
		return false
	}
	return true
}

// Private initialization and append actions must not become a source action's
// omitted Previous input. This finite first consumer admits only explicit-input
// expressions in the collected body. It does not reinterpret raw/native calls,
// copied bodies, user actions, or nested loops as transparent compiler helpers.
func loopResultExplicitStatement(statement syntax.Statement) bool {
	switch node := statement.(type) {
	case *syntax.BlockStmt:
		if node == nil {
			return false
		}
		for _, child := range node.Statements {
			if !loopResultExplicitStatement(child) {
				return false
			}
		}
		return true
	case *syntax.BindingStmt:
		return loopResultExplicitExpression(node.Value)
	case *syntax.AssignStmt:
		return loopResultExplicitExpression(node.Value)
	case *syntax.ExprStmt:
		return loopResultExplicitExpression(node.Expr)
	case *syntax.YieldStmt:
		return loopResultExplicitExpression(node.Value)
	case *syntax.ReturnStmt:
		return node.Value == nil || loopResultExplicitExpression(node.Value)
	case *syntax.IfStmt:
		return loopResultExplicitExpression(node.Condition) &&
			loopResultExplicitStatement(node.ThenBlock) &&
			(node.ElseBlock == nil || loopResultExplicitStatement(node.ElseBlock))
	case *syntax.MenuStmt:
		if !loopResultExplicitExpression(node.Prompt) {
			return false
		}
		for _, item := range node.Cases {
			if item.Value != nil && !loopResultExplicitExpression(item.Value) ||
				!loopResultExplicitStatement(item.BodyBlock) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func loopResultExplicitExpression(expression syntax.Expression) bool {
	switch node := expression.(type) {
	case *syntax.LiteralExpr, *syntax.IdentExpr, *syntax.EnumMemberExpr:
		return true
	case *syntax.FStringExpr:
		for _, part := range node.Parts {
			if part.IsExpr && !loopResultExplicitExpression(part.Expr) {
				return false
			}
		}
		return true
	case *syntax.UnaryExpr:
		return loopResultExplicitExpression(node.Operand)
	case *syntax.BinaryExpr:
		return loopResultExplicitExpression(node.Left) && loopResultExplicitExpression(node.Right)
	case *syntax.IndexExpr:
		return loopResultExplicitExpression(node.Target) && loopResultExplicitExpression(node.Index)
	case *syntax.MemberExpr:
		return loopResultExplicitExpression(node.Target)
	case *syntax.ListExpr:
		for _, element := range node.Elements {
			if !loopResultExplicitExpression(element) {
				return false
			}
		}
		return true
	case *syntax.MapExpr:
		for _, item := range node.Entries {
			if !loopResultExplicitExpression(item.Key) || !loopResultExplicitExpression(item.Value) {
				return false
			}
		}
		return true
	case *syntax.IfStmt:
		return loopResultExplicitStatement(node)
	case *syntax.MenuStmt:
		return loopResultExplicitStatement(node)
	default:
		return false
	}
}
