package syntax

// A trailing block in an unparenthesized control operand is ambiguous with the
// control's own body. Preserve parentheses when formatting that new syntax.
func (f *Formatter) formatControlExpression(expr Expression) {
	parenthesize := containsScopedBlock(expr)
	if parenthesize {
		f.buf.WriteString("(")
	}
	f.formatExpression(expr)
	if parenthesize {
		f.buf.WriteString(")")
	}
}

func containsScopedBlock(expr Expression) bool {
	switch e := expr.(type) {
	case *CallExpr:
		if e.Block != nil || containsScopedBlock(e.Callee) || containsScopedBlock(e.PrimaryArg) {
			return true
		}
		for _, arg := range e.NamedArgs {
			if containsScopedBlock(arg.Value) {
				return true
			}
		}
	case *BinaryExpr:
		return containsScopedBlock(e.Left) || containsScopedBlock(e.Right)
	case *UnaryExpr:
		return containsScopedBlock(e.Operand)
	case *MemberExpr:
		return containsScopedBlock(e.Target)
	case *IndexExpr:
		return containsScopedBlock(e.Target) || containsScopedBlock(e.Index)
	}
	return false
}
