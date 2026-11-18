package syntax

// Parentheses explicitly disambiguate comparisons. The AST deliberately keeps
// no grouping node, so the parser records grouping only during this decision.
func (p *Parser) unparenthesizedComparison(expr Expression) bool {
	binary, ok := expr.(*BinaryExpr)
	return ok && !p.parenthesized[expr] && isComparisonOperator(binary.Op)
}

func isComparisonOperator(op TokenType) bool {
	switch op {
	case TokenEqual, TokenNotEqual, TokenLess, TokenLessEqual, TokenGreater, TokenGreaterEqual,
		TokenContains, TokenBeginsWith, TokenEndsWith:
		return true
	}
	return false
}
