package syntax

// IsIdentifier reports whether text can be emitted as one non-keyword name.
// It uses the same Unicode rules as the lexer.
func IsIdentifier(text string) bool {
	if text == "" {
		return false
	}
	if _, reserved := keywords[text]; reserved {
		return false
	}
	for index, r := range text {
		if index == 0 {
			if !IsIdentifierStart(r) {
				return false
			}
		} else if !IsIdentifierContinue(r) {
			return false
		}
	}
	return true
}
