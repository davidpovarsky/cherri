package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// ResolvedAction returns the action chosen by semantic name binding for this
// exact AST call. Nil means it did not resolve to an action (for example a user
// function with the same spelling). Callers must treat the schema as read-only.
func (a *Analyzer) ResolvedAction(call *syntax.CallExpr) *schema.ActionSchema {
	if action := a.resolvedActions[call]; action != nil {
		return action
	}
	for _, child := range a.moduleAnalyses {
		if action := child.resolvedActions[call]; action != nil {
			return action
		}
	}
	return nil
}
