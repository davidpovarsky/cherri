/*
 * Copyright (c) Cherri Language v2.0
 * Semantic Backend Contracts
 */

package backend

import (
	"crypto/sha256"
	"encoding/hex"
)

// CodecFingerprint returns the deterministic SHA256 fingerprint of the canonical backend codec contract.
func CodecFingerprint() string {
	const codecDescriptor = "cherri-canonical-codec:v2.2\n" +
		"refs:ActionResult,MutableBinding,SystemValue,LoopItem,LoopIndex,LoopResult,ExtensionInput,AskEachTime\n" +
		"values:Nil,Bool,Int,Float,String,Reference,TextSegments,List,Dict,RawNative\n" +
		"controls:If,Repeat,For,Menu:modes=0,1,2\n" +
		"tokens:WFTextTokenAttachment,WFTextTokenString,WFDictionaryFieldValueItem,WFNumberSubstitutableState\n" +
		"aggrandizements:WFPropertyVariableAggrandizement,WFCoercionVariableAggrandizement,WFDictionaryValueVariableAggrandizement,WFDateFormatVariableAggrandizement\n" +
		"fn_envelope:__cherri_fn_envelope_v2__:_cherri_caller_input:has_arg_n:optional_arg_n=_cherri_optional_argument_v1,has_value=0-absent-1-present,value-present-only:result=_cherri_optional_result_v1,has_value,value\n" +
		"optional_control_join:_cherri_optional_join_v1:has_value=0-absent-1-present:value-present-only:normal-branch-yields:source-demand:no-truthiness\n" +
		"text_literal_object_character:plain_text_producer:deferred_after_interpolated_calls:exact_output_reference:complete_utf16_placeholder_coverage:pure_string_without_references:enclosing_native_input=existing_result_handle:checked_native_declaration_output_contract:semantic_container_fields_after_authored_values:literal_key_order_preserved:function_and_optional_fields:removed_diagnostics_without_pure_carriers\n"
	sum := sha256.Sum256([]byte(codecDescriptor))
	return hex.EncodeToString(sum[:])
}

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
	// Explicit native dictionaries retain fields beyond the typed semantic view.
	RawNativeFields      map[string]any `json:"rawNativeFields,omitempty"`
	Type                 string         `json:"type"`
	PropertyName         string         `json:"propertyName,omitempty"`
	CoercionItemClass    string         `json:"coercionItemClass,omitempty"`
	DictionaryKey        string         `json:"dictionaryKey,omitempty"`
	PropertyUserInfo     any            `json:"propertyUserInfo,omitempty"`
	WFDateFormat         string         `json:"wfDateFormat,omitempty"`
	WFDateFormatStyle    string         `json:"wfDateFormatStyle,omitempty"`
	WFTimeFormatStyle    string         `json:"wfTimeFormatStyle,omitempty"`
	WFISO8601IncludeTime *bool          `json:"wfISO8601IncludeTime,omitempty"`
}

// Reference is a structured symbolic reference.
type Reference struct {
	Kind         ReferenceKind
	ProducerID   string // action UUID or variable name
	ProducerName string // display / binding name
	// OmitOutputName preserves an explicitly nameless native ActionOutput.
	// Ordinary generated references retain their historical ID-name default.
	OmitOutputName  bool
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
	Key string
	// Explicit computed Text keys retain references and UTF-16 text segments.
	// A nil/empty segment list leaves the existing literal Key unchanged.
	KeySegments []TextSegment
	// A compiler-owned carrier may represent an unchanged literal key. Retain
	// canonical literal-key ordering instead of treating it as a computed key.
	KeyHasLiteralValue bool
	Value              SemanticValue
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
	// StaticType is compiler-only semantic information, never an Apple parameter.
	StaticType  string
	ParameterID string
	Position    int
	Value       SemanticValue
	Omitted     bool
}

// ResolvedCall describes a typed, resolved action invocation.
type ResolvedCall struct {
	// SourceDocument is source metadata for local resources, never a native parameter.
	SourceDocument  string
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
	ControlElse
	ControlRepeat
	ControlFor
	ControlMenu
	ControlMenuCase
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
	MenuPrompt         any
	MenuCases          []string
	MenuCaseTitle      string
	OutputUUID         string
}

// Session is the interface for driving emission into the canonical backend.
type Session interface {
	EmitResolvedCall(call ResolvedCall) (outputUUID string, err error)
	BeginControl(region ControlRegion) error
	EndControl(region ControlRegion) error
	EmitRawAction(appleIdentifier string, params map[string]any, outputUUID, outputName, groupingID string) error
	RenameOutput(outputUUID string, newName string)
	SetMetadata(name string, value any)
	AddImportQuestion(question map[string]any)
	BindImportQuestion(questionName string, wireKey string, nodeID string) error
	Finalize() ([]byte, error)
}
