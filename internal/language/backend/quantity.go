package backend

// QuantityValue retains evaluated magnitude references until the canonical
// quantity builder chooses their bare native representation.
type QuantityValue struct {
	Magnitude SemanticValue
	Unit      SemanticValue
}
