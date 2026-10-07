/*
 * Copyright (c) Cherri Language v2.0
 * Semantic Backend Contracts
 */

package backend

// ReferenceKind describes the kind of symbolic reference.
type ReferenceKind int

const (
	RefActionResult ReferenceKind = iota
	RefMutableBinding
	RefSystemValue
	RefLoopItem
	RefLoopIndex
	RefLoopResult
	RefExtensionInput
	RefAskEachTime
)

// Transformation represents an aggrandizement (property, coercion, dict key, etc.)
type Transformation struct {
	Type              string `json:"type"`
	PropertyName      string `json:"propertyName,omitempty"`
	CoercionItemClass string `json:"coercionItemClass,omitempty"`
	DictionaryKey     string `json:"dictionaryKey,omitempty"`
	PropertyUserInfo  any    `json:"propertyUserInfo,omitempty"`
}

// Reference is a structured symbolic reference.
type Reference struct {
	Kind            ReferenceKind
	ProducerID      string // action UUID or variable name
	ProducerName    string // display / binding name
	ScopeID         string // loop or function scope ID
	Transformations []Transformation
	RawExtensions   map[string]any
}

// TextSegment represents a piece of text: literal text or an interpolated expression/reference.
type TextSegment struct {
	IsExpr    bool
	Text      string
	Value     SemanticValue
	Reference *Reference
}

// SemanticValueKind represents the type of a semantic value.
type SemanticValueKind int

const (
	ValNil SemanticValueKind = iota
	ValBool
	ValInt
	ValFloat
	ValString
	ValReference
	ValTextSegments
	ValList
	ValDict
	ValRawNative
)

// DictEntry represents a key-value pair in a semantic dictionary.
type DictEntry struct {
	Key   string
	Value SemanticValue
}

// SemanticValue encapsulates an evaluated expression value.
type SemanticValue struct {
	Kind         SemanticValueKind
	BoolVal      bool
	IntVal       int64
	FloatVal     float64
	StrVal       string
	Ref          *Reference
	Segments     []TextSegment
	ListVal      []SemanticValue
	DictVal      []DictEntry
	RawNativeVal any
	Omitted      bool
}

// CallArgument holds an evaluated argument passed to a resolved call.
type CallArgument struct {
	ParameterID string
	Position    int
	Value       SemanticValue
	Omitted     bool
}

// ResolvedCall describes a typed, resolved action invocation.
type ResolvedCall struct {
	DefinitionID    string
	VariantID       string
	AppleIdentifier string
	Arguments       []CallArgument
	OutputUUID      string
	OutputName      string
	NodeID          string
}

// ControlFlowKind defines control flow structures.
type ControlFlowKind int

const (
	ControlIf ControlFlowKind = iota
	ControlRepeat
	ControlFor
	ControlMenu
)

// ControlRegion describes a begin/middle/end of a control structure.
type ControlRegion struct {
	Kind               ControlFlowKind
	Mode               int // 0: begin, 1: else/middle, 2: end
	GroupingIdentifier string
	ConditionInput     *Reference
	ConditionOperator  int
	ConditionValue     any
	Count              any
	Iterable           any
	ItemName           string
	IndexName          string
	LoopID             string
	ParentLoopID       string
}

// Session is the interface for driving emission into the canonical backend.
type Session interface {
	EmitResolvedCall(call ResolvedCall) (outputUUID string, err error)
	BeginControl(region ControlRegion) error
	EndControl(region ControlRegion) error
	EmitRawAction(appleIdentifier string, params map[string]any, outputUUID, outputName, groupingID string) error
	SetMetadata(name string, value any)
	AddImportQuestion(question map[string]any)
	Finalize() ([]byte, error)
}
