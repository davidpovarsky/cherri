package syntax

// Preserve the parsed tree when formatting binary operands. Pratt operators are
// left associative; a right child at the same precedence therefore needs
// parentheses even when its arithmetic value happens to be equivalent.
func (f *Formatter) formatBinaryOperand(expr Expression, parent TokenType, right bool) {
	parenthesize := false
	if child, ok := expr.(*BinaryExpr); ok {
		childPrec, parentPrec := infixPrecedence(child.Op), infixPrecedence(parent)
		parenthesize = childPrec < parentPrec || (right && childPrec == parentPrec) ||
			(isComparisonOperator(parent) && isComparisonOperator(child.Op))
	}
	f.formatGroupedExpression(expr, parenthesize)
}

func (f *Formatter) formatPostfixTarget(expr Expression) {
	_, binary := expr.(*BinaryExpr)
	_, unary := expr.(*UnaryExpr)
	f.formatGroupedExpression(expr, binary || unary)
}

func (f *Formatter) formatGroupedExpression(expr Expression, parenthesize bool) {
	if parenthesize {
		f.buf.WriteString("(")
	}
	f.formatExpression(expr)
	if parenthesize {
		f.buf.WriteString(")")
	}
}
