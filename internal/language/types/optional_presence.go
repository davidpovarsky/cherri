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
