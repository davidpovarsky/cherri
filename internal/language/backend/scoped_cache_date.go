package backend

// Date content is an instant, not its locale-dependent textual display.
// Use the existing native date/quantity operations and refuse a lossy archive;
// no precision tolerance can silently turn a different instant into a hit.
func (b *scopedCacheEmitter) dateOffset(value *Reference) *Reference {
	return b.produce("gettimebetweendates", map[string]any{
		"WFTimeUntilFromDate": cacheText(b.epoch),
		"WFInput":             cacheText(value),
		"WFTimeUntilUnit":     "Seconds",
	})
}

func (b *scopedCacheEmitter) restoreDate(seconds *Reference) *Reference {
	return b.produce("adjustdate", map[string]any{
		"WFDate":            cacheText(b.epoch),
		"WFAdjustOperation": "Add",
		"WFDuration":        &QuantityValue{Magnitude: cacheReference(seconds), Unit: SemanticValue{Kind: ValString, StrVal: "sec"}},
	})
}

func (b *scopedCacheEmitter) writeDatePayload(value *Reference) *Reference {
	var seconds = b.dateOffset(value)
	b.failIf(b.count(seconds), 5, 1, OperandNumber)
	var actual = b.produce("getitemtype", map[string]any{"WFInput": cacheReference(seconds)})
	b.failIf(actual, 5, cacheText(b.numberType), OperandString)
	var restored = b.restoreDate(seconds)
	b.failIf(b.count(restored), 5, 1, OperandNumber)
	var restoredType = b.produce("getitemtype", map[string]any{"WFInput": cacheReference(restored)})
	b.failIf(restoredType, 5, cacheText(b.dateType), OperandString)
	b.failIf(restored, 5, cacheText(value), OperandDate)
	return seconds
}

func (b *scopedCacheEmitter) readDatePayload(seconds *Reference) {
	var value = b.restoreDate(seconds)
	b.rejectIf(b.count(value), 5, 1, OperandNumber)
	var actual = b.produce("getitemtype", map[string]any{"WFInput": cacheReference(value)})
	b.rejectIf(actual, 5, cacheText(b.dateType), OperandString)
	b.when(b.get("valid"), 4, 1, OperandNumber, func() {
		var restoredOffset = b.dateOffset(value)
		b.rejectIf(b.count(restoredOffset), 5, 1, OperandNumber)
		var offsetType = b.produce("getitemtype", map[string]any{"WFInput": cacheReference(restoredOffset)})
		b.rejectIf(offsetType, 5, cacheText(b.numberType), OperandString)
		b.rejectIf(restoredOffset, 5, cacheText(seconds), OperandNumber)
		b.when(b.get("valid"), 4, 1, OperandNumber, func() {
			b.set("result", value)
			b.set("hit", b.one)
		})
	})
}
