package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

type controlValueEvidence struct {
	value                    types.Type
	knownAbsent, hasPresence bool
}

// Only normally completing branches contribute a joined value. A literal or
// path-local known None supplies absence, not an unknown present inner value.
// An opaque native Unknown retains its unknown inner value and no presence.
func (a *Analyzer) normalControlValueEvidence(expr syntax.Expression) []controlValueEvidence {
	var values []controlValueEvidence
	add := func(body *syntax.BlockStmt) {
		if body == nil || !statementMayContinue(body) {
			return
		}
		result := syntax.BlockResultExpression(body)
		if result != nil && len(body.Statements) != 0 {
			if _, yielded := body.Statements[len(body.Statements)-1].(*syntax.YieldStmt); yielded {
				value := a.exprTypes[result]
				if value == nil {
					value = types.AnyContent
				}
				values = append(values, controlValueEvidence{value, a.exprAbsent[result], a.exprPresence[result]})
				return
			}
		}
		switch control := result.(type) {
		case *syntax.IfStmt, *syntax.MenuStmt:
			values = append(values, a.normalControlValueEvidence(control)...)
		case *syntax.ForStmt, *syntax.RepeatStmt:
			value := a.controlExpressionResultType(control)
			values = append(values, controlValueEvidence{value, false, types.DefinitelyPresent(value)})
		default:
			values = append(values, controlValueEvidence{types.AnyContent, false, false})
		}
	}
	switch control := expr.(type) {
	case *syntax.IfStmt:
		add(control.ThenBlock)
		switch alternate := control.ElseBlock.(type) {
		case *syntax.BlockStmt:
			add(alternate)
		case *syntax.IfStmt:
			if statementMayContinue(alternate) {
				values = append(values, a.normalControlValueEvidence(alternate)...)
			}
		default:
			values = append(values, controlValueEvidence{types.AnyContent, false, false})
		}
	case *syntax.MenuStmt:
		for _, item := range control.Cases {
			add(item.BodyBlock)
		}
	}
	return values
}

func joinControlValueEvidence(values []controlValueEvidence) types.Type {
	var present []types.Type
	mayBeAbsent := false
	var add func(types.Type)
	add = func(value types.Type) {
		switch typed := value.(type) {
		case types.OptionalType:
			mayBeAbsent = true
			add(typed.Inner)
		case types.UnionType:
			for _, member := range typed.Members {
				add(member)
			}
		default:
			present = append(present, value)
		}
	}
	for _, branch := range values {
		if branch.knownAbsent {
			mayBeAbsent = true
		} else {
			add(branch.value)
		}
	}
	joined := types.NewUnion(present...)
	if mayBeAbsent {
		// NewOptional(Unknown) intentionally collapses to Unknown for an
		// unproved annotation; an observed None path must retain absence.
		return types.OptionalType{Inner: joined}
	}
	return joined
}

func (a *Analyzer) controlValueHasPresence(expr syntax.Expression) bool {
	values := a.normalControlValueEvidence(expr)
	if len(values) == 0 {
		return false
	}
	for _, branch := range values {
		if !branch.hasPresence {
			return false
		}
	}
	return true
}

func (a *Analyzer) controlValueKnownAbsent(expr syntax.Expression) bool {
	values := a.normalControlValueEvidence(expr)
	if len(values) == 0 {
		return false
	}
	for _, branch := range values {
		if !branch.knownAbsent {
			return false
		}
	}
	return true
}
