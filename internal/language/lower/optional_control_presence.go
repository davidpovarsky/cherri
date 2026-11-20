package lower

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Presence is emitted only for a nullable control value with an actual source
// consumer. Unknown generated nodes retain the conservative demand policy.
// Ordinary unconsumed control values keep their existing native last output.
func (l *Lowerer) optionalJoinNeeded(expr syntax.Expression) bool {
	switch expr.(type) {
	case *syntax.IfStmt, *syntax.MenuStmt:
	default:
		return false
	}
	needed, analyzed := l.optionalJoinDemand[expr]
	return (!analyzed || needed) && types.MayBeAbsent(l.exprTypes[expr])
}

// Each normally completing branch supplies a private tagged Dictionary. A
// return exits the invocation using its existing function-result carrier and
// does not supply a join value. The tag is consumed once after the control end.
func (l *Lowerer) lowerOptionalControlValue(expr syntax.Expression) (*ir.AttachmentToken, error) {
	leaves := map[*syntax.YieldStmt]bool{}
	if err := collectOptionalJoinYields(expr, leaves); err != nil {
		return nil, err
	}
	if len(leaves) == 0 {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: Optional control has no normal value path at %s", expr.NodeSpan())
	}
	previous := l.optionalJoinYields
	l.optionalJoinYields = make(map[*syntax.YieldStmt]bool, len(previous)+len(leaves))
	for yield, tracked := range previous {
		l.optionalJoinYields[yield] = tracked
	}
	for yield := range leaves {
		l.optionalJoinYields[yield] = true
	}
	defer func() { l.optionalJoinYields = previous }()

	var carrier *ir.AttachmentToken
	var err error
	switch control := expr.(type) {
	case *syntax.IfStmt:
		carrier, err = l.lowerIfWithOutput(control, true)
	case *syntax.MenuStmt:
		carrier, err = l.lowerMenuWithOutput(control, true)
	}
	if err != nil {
		return nil, err
	}
	if carrier == nil || carrier.OutputUUID == "" {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: Optional control has no joined carrier at %s", expr.NodeSpan())
	}
	value, presence := l.decodeOptionalCarrier(carrier.OutputUUID, "has_value", "value", "")
	if l.optionalExpressionPresence == nil {
		l.optionalExpressionPresence = map[syntax.Expression]interface{}{}
	}
	l.optionalExpressionPresence[expr] = &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: presence}
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: value}, nil
}

func (l *Lowerer) lowerOptionalJoinYield(yield *syntax.YieldStmt) error {
	previous := l.inExprStmt
	l.inExprStmt = false
	defer func() { l.inExprStmt = previous }()
	var value interface{}
	var err error
	switch yield.Value.(type) {
	case *syntax.MapExpr, *syntax.ListExpr:
		// An ordinary container retains its AST identity and explicit empty
		// fields inside the private carrier. It is never a raw native template.
		value, err = l.lowerSemanticValue(yield.Value)
	default:
		value, err = l.lowerRuntimeLiteral(yield.Value)
	}
	if err != nil {
		return err
	}
	presence, err := l.optionalValuePresence(yield.Value, value)
	if err != nil {
		return err
	}
	l.optionalCarrier(value, presence, "has_value", "value", "_cherri_optional_join_v1")
	return nil
}

func collectOptionalJoinYields(expr syntax.Expression, leaves map[*syntax.YieldStmt]bool) error {
	var collectBlock func(*syntax.BlockStmt) error
	collectBlock = func(body *syntax.BlockStmt) error {
		if !optionalJoinMayContinue(body) {
			return nil
		}
		if body == nil || len(body.Statements) == 0 {
			return fmt.Errorf("E_TARGET_VALUE_SHAPE: Optional control branch has no explicit value")
		}
		switch last := body.Statements[len(body.Statements)-1].(type) {
		case *syntax.YieldStmt:
			leaves[last] = true
			return nil
		case *syntax.IfStmt:
			return collectOptionalJoinYields(last, leaves)
		case *syntax.MenuStmt:
			return collectOptionalJoinYields(last, leaves)
		default:
			return fmt.Errorf("E_TARGET_VALUE_SHAPE: Optional control transport requires an explicit normal-branch yield at %s", body.NodeSpan())
		}
	}
	switch control := expr.(type) {
	case *syntax.IfStmt:
		if err := collectBlock(control.ThenBlock); err != nil {
			return err
		}
		switch alternate := control.ElseBlock.(type) {
		case *syntax.BlockStmt:
			return collectBlock(alternate)
		case *syntax.IfStmt:
			return collectOptionalJoinYields(alternate, leaves)
		default:
			return fmt.Errorf("E_TARGET_VALUE_SHAPE: Optional If has no alternate value at %s", control.NodeSpan())
		}
	case *syntax.MenuStmt:
		for _, item := range control.Cases {
			if err := collectBlock(item.BodyBlock); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("E_TARGET_VALUE_SHAPE: unsupported Optional control %T", expr)
}

// Match the analyzer's normal-completion boundary. Loop bodies do not prove
// that the loop executes, whereas a return reached unconditionally terminates
// the invocation, including through an If or every Menu case.
func optionalJoinMayContinue(statement syntax.Statement) bool {
	switch node := statement.(type) {
	case *syntax.ReturnStmt:
		return false
	case *syntax.BlockStmt:
		if node == nil {
			return true
		}
		for _, child := range node.Statements {
			if !optionalJoinMayContinue(child) {
				return false
			}
		}
	case *syntax.IfStmt:
		return optionalJoinMayContinue(node.ThenBlock) || node.ElseBlock == nil || optionalJoinMayContinue(node.ElseBlock)
	case *syntax.MenuStmt:
		if len(node.Cases) == 0 {
			return true
		}
		for _, item := range node.Cases {
			if optionalJoinMayContinue(item.BodyBlock) {
				return true
			}
		}
		return false
	}
	return true
}
