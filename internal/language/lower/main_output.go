package lower

import (
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Natural V2 statements have effects, but their incidental native results are
// not an implicit main return. Nothing clears the fallthrough result, including
// inherited ShortcutInput in an empty program. The existing exact-workflow
// flag is set by the actual native.workflow lowering route, never by text.
func (l *Lowerer) finishMainOutput(program *syntax.Program) {
	if l.mainOutputMode == syntax.MainOutputLegacy || l.workflow.IsFullNativePass ||
		mainSequenceReturns(program.Statements) {
		return
	}
	switch l.lastEmittedIdentifier {
	case "is.workflow.actions.nothing", "is.workflow.actions.output", "is.workflow.actions.exit":
		// Resolved calls can go directly to the canonical session rather than
		// the IR collection. Track their actual identity as well as raw actions.
		return
	}
	l.emitAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.nothing",
	})
}

// Only unconditional structured return paths suppress the final boundary.
// A loop may execute zero times; a menu may be cancelled. Neither is treated
// as a guaranteed return merely because its body contains a return.
func mainSequenceReturns(statements []syntax.Statement) bool {
	for _, statement := range statements {
		if mainStatementReturns(statement) {
			return true
		}
	}
	return false
}

func mainStatementReturns(statement syntax.Statement) bool {
	switch node := statement.(type) {
	case *syntax.ReturnStmt:
		return true
	case *syntax.BlockStmt:
		return node != nil && mainSequenceReturns(node.Statements)
	case *syntax.IfStmt:
		return node != nil && node.ThenBlock != nil &&
			mainSequenceReturns(node.ThenBlock.Statements) &&
			node.ElseBlock != nil && mainStatementReturns(node.ElseBlock)
	default:
		return false
	}
}
