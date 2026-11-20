package types

// MayBeAbsent describes the value itself, not the presence of a parameter or
// dictionary field. Container members have their own independent boundaries.
// Unknown native content is not evidence of either presence or absence.
func MayBeAbsent(value Type) bool {
	switch t := value.(type) {
	case OptionalType:
		return true
	case UnionType:
		for _, member := range t.Members {
			if MayBeAbsent(member) {
				return true
			}
		}
	}
	return false
}

// DefinitelyPresent is semantic type evidence. Unknown and AnyContent do not
// prove that an opaque native producer returned a value. This does not inspect
// the value's truthiness: false, zero and empty Text/List/Map remain present.
func DefinitelyPresent(value Type) bool {
	if value == nil || MayBeAbsent(value) {
		return false
	}
	if union, ok := value.(UnionType); ok {
		if len(union.Members) == 0 {
			return false
		}
		for _, member := range union.Members {
			if !DefinitelyPresent(member) {
				return false
			}
		}
		return true
	}
	switch value.Kind() {
	case KindUnknown, KindAnyContent, KindVoid, KindJsonNull:
		return false
	}
	return true
}
