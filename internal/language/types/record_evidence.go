package types

// Missing type knowledge and arbitrary content do not establish a user record
// contract. This includes containers whose elements/fields would otherwise gain
// record guarantees solely from an annotation. Ordinary content conversion
// compatibility is deliberately unchanged.
func requiresRecordEvidence(target Type) bool {
	switch t := target.(type) {
	case RecordType:
		return true
	case OptionalType:
		return requiresRecordEvidence(t.Inner)
	case ListType:
		return requiresRecordEvidence(t.Element)
	case MapType:
		return requiresRecordEvidence(t.Key) || requiresRecordEvidence(t.Value)
	case UnionType:
		for _, member := range t.Members {
			if requiresRecordEvidence(member) {
				return true
			}
		}
	}
	return false
}

// Inside a verified record shape, a field may remain opaque but cannot acquire
// a concrete type from Unknown/AnyContent. Recurse through field containers so
// List<Unknown> cannot silently become List<Text>, for example.
func recordFieldAssignable(source, target Type) bool {
	if target.Kind() == KindUnknown || target.Kind() == KindAnyContent {
		return source.AssignableTo(target)
	}
	if union, ok := source.(UnionType); ok {
		for _, member := range union.Members {
			if !recordFieldAssignable(member, target) {
				return false
			}
		}
		return true
	}
	if union, ok := target.(UnionType); ok {
		for _, member := range union.Members {
			if recordFieldAssignable(source, member) {
				return true
			}
		}
		return false
	}
	if source.Kind() == KindUnknown || source.Kind() == KindAnyContent {
		return false
	}
	if optional, ok := target.(OptionalType); ok {
		if from, ok := source.(OptionalType); ok {
			return recordFieldAssignable(from.Inner, optional.Inner)
		}
		return recordFieldAssignable(source, optional.Inner)
	}
	switch from := source.(type) {
	case ListType:
		if to, ok := target.(ListType); ok {
			return recordFieldAssignable(from.Element, to.Element)
		}
	case MapType:
		if to, ok := target.(MapType); ok {
			return recordFieldAssignable(from.Key, to.Key) && recordFieldAssignable(from.Value, to.Value)
		}
	}
	return source.AssignableTo(target)
}

// HasAssignmentEvidence checks an authored type guarantee using the existing
// structural evidence relation. It does not insert or promise a Content Graph
// conversion; native parameter attachment compatibility stays independent.
func HasAssignmentEvidence(source, target Type) bool {
	if source == nil || target == nil {
		return false
	}
	return recordFieldAssignable(source, target)
}
