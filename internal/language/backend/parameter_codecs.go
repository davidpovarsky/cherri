package backend

// These are the parameter codec identities implemented by the canonical
// semantic-value adapters. A public declaration may select an existing identity;
// naming an arbitrary codec never creates a new encoding implementation.
const (
	CodecText       = "text"
	CodecTextToken  = "textToken"
	CodecNumber     = "number"
	CodecBool       = "bool"
	CodecDictionary = "dictionary"
	CodecArray      = "array"
	CodecToken      = "token"
	CodecEnum       = "enum"
)

func IsRegisteredParameterCodec(name string) bool {
	switch name {
	case CodecText, CodecTextToken, CodecNumber, CodecBool, CodecDictionary, CodecArray, CodecToken, CodecEnum:
		return true
	default:
		return false
	}
}
