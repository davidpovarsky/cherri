package types

// TaggedListElements describes a private, source-owned List of value cells.
// An ordinary scalar/Dictionary List keeps its existing native representation.
func TaggedListElements(value Type) bool {
	if optional, ok := value.(OptionalType); ok {
		return TaggedListElements(optional.Inner)
	}
	list, ok := value.(ListType)
	return ok && RequiresValueCell(list.Element)
}

// A Dictionary needs cells only for values with independent absence/null.
// Existing native Dictionary arrays retain their established typed codec.
func TaggedMapValues(value Type) bool {
	if optional, ok := value.(OptionalType); ok {
		return TaggedMapValues(optional.Inner)
	}
	dictionary, ok := value.(MapType)
	return ok && valueHasAbsenceOrNull(dictionary.Value)
}

func valueHasAbsenceOrNull(value Type) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case OptionalType:
		return true
	case UnionType:
		for _, member := range typed.Members {
			if valueHasAbsenceOrNull(member) {
				return true
			}
		}
	}
	return value.Kind() == KindJsonNull || value.Kind() == KindJsonValue
}

func RequiresValueCell(value Type) bool {
	if valueHasAbsenceOrNull(value) {
		return true
	}
	if value == nil {
		return false
	}
	if union, ok := value.(UnionType); ok {
		for _, member := range union.Members {
			if RequiresValueCell(member) {
				return true
			}
		}
	}
	return value.Kind() == KindList
}

// PrivateValueTransport is not permission to pass compiler cells to Apple.
// Callers at native/JSON boundaries must provide a real conversion or reject.
func PrivateValueTransport(value Type) bool {
	if value == nil {
		return false
	}
	if TaggedListElements(value) || TaggedMapValues(value) {
		return true
	}
	switch typed := value.(type) {
	case OptionalType:
		return PrivateValueTransport(typed.Inner)
	case UnionType:
		for _, member := range typed.Members {
			if PrivateValueTransport(member) {
				return true
			}
		}
	case ListType:
		return PrivateValueTransport(typed.Element)
	case MapType:
		return InlineDataNeedsCells(typed.Value)
	}
	return value.Kind() == KindJsonNull || value.Kind() == KindJsonValue
}

// InlineDataNeedsCells preserves the established native Dictionary-array
// codec for ordinary nested literals. Absence/null is the new private boundary.
func InlineDataNeedsCells(value Type) bool {
	if valueHasAbsenceOrNull(value) {
		return true
	}
	switch typed := value.(type) {
	case ListType:
		return InlineDataNeedsCells(typed.Element)
	case MapType:
		return InlineDataNeedsCells(typed.Value)
	case UnionType:
		for _, member := range typed.Members {
			if InlineDataNeedsCells(member) {
				return true
			}
		}
	}
	return false
}

func ContainsJSONValue(value Type) bool {
	if value == nil {
		return false
	}
	switch typed := value.(type) {
	case OptionalType:
		return ContainsJSONValue(typed.Inner)
	case UnionType:
		for _, member := range typed.Members {
			if ContainsJSONValue(member) {
				return true
			}
		}
	}
	return value.Kind() == KindJsonNull || value.Kind() == KindJsonValue
}
