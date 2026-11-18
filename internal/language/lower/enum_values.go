package lower

import "fmt"

// Imported declarations can be lowered by the existing module loader after the
// caller's analysis pass. Retain their qualified literal path while respecting
// lexical bindings; analyzed source expressions use enumValues first.
func (l *Lowerer) qualifiedEnumValue(name, member string) (string, bool, error) {
	if _, shadowed := l.bindings[name]; shadowed {
		return "", false, nil
	}
	if declaration, found := l.enums[name]; found {
		for _, candidate := range declaration.Members {
			if candidate.Name == member {
				return candidate.WireValue, true, nil
			}
		}
		return "", true, fmt.Errorf("enum %q has no member %q", name, member)
	}
	if values, found := l.registry.LookupEnum(name); found {
		for _, candidate := range values {
			if candidate == member {
				return candidate, true, nil
			}
		}
		return "", true, fmt.Errorf("enum %q has no member %q", name, member)
	}
	return "", false, nil
}
