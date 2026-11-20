package syntax

// StaticDictionaryElement requires an exact, nonempty Text key and a literal
// Dictionary with unique literal keys. A dynamic sibling key could overwrite
// the selected entry, so the complete key set must be source-owned. Callers
// must separately prove the selected semantic value is present.
func StaticDictionaryElement(dictionary *MapExpr, index Expression) (Expression, bool) {
	key, okay := dictionaryPresenceKey(index)
	if dictionary == nil || !okay {
		return nil, false
	}
	seen := make(map[string]bool)
	var selected Expression
	for _, entry := range dictionary.Entries {
		name, literal := dictionaryPresenceKey(entry.Key)
		if !literal || seen[name] {
			return nil, false
		}
		seen[name] = true
		if name == key {
			selected = entry.Value
		}
	}
	return selected, selected != nil
}

func dictionaryPresenceKey(expression Expression) (string, bool) {
	literal, okay := expression.(*LiteralExpr)
	if !okay || (literal.Type != TokenString && literal.Type != TokenRawString) || literal.Value == "" {
		return "", false
	}
	return literal.Value, true
}
