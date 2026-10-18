/*
 * Copyright (c) Cherri
 */

package main

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
)

/*
 Shortcut File Format Data Structures
*/

type ShortcutIcon struct {
	WFWorkflowIconGlyphNumber int64 `plist:",omitempty"`
	WFWorkflowIconStartColor  int   `plist:",omitempty"`
}

type Shortcut struct {
	WFWorkflowIcon                       ShortcutIcon
	WFWorkflowActions                    []ShortcutAction
	WFQuickActionSurfaces                []string
	WFWorkflowInputContentItemClasses    []string
	WFWorkflowClientVersion              string
	WFWorkflowMinimumClientVersion       int
	WFWorkflowMinimumClientVersionString string
	WFWorkflowImportQuestions            []WFQuestion
	WFWorkflowTypes                      []string
	WFWorkflowOutputContentItemClasses   []string
	WFWorkflowHasShortcutInputVariables  bool
	WFWorkflowHasOutputFallback          bool
	WFWorkflowNoInputBehavior            map[string]any
	WFWorkflowTriggers                   []AutomationTrigger
}

var shortcut Shortcut

type ShortcutAction struct {
	WFWorkflowActionIdentifier string
	WFWorkflowActionParameters map[string]any `plist:",omitempty"`
}

type Value struct {
	Type                        string                       `plist:",omitempty"`
	VariableName                string                       `plist:",omitempty"`
	OutputUUID                  string                       `plist:",omitempty"`
	OutputName                  string                       `plist:",omitempty"`
	Value                       any                          `plist:",omitempty"`
	Variable                    any                          `plist:",omitempty"`
	WFDictionaryFieldValueItems []WFDictionaryFieldValueItem `plist:",omitempty"`
	AttachmentsByRange          map[string]Value             `plist:",omitempty"`
	String                      string                       `plist:",omitempty"`
	Aggrandizements             []Aggrandizement             `plist:",omitempty"`
	Prompt                      string                       `plist:",omitempty"`
}

type Aggrandizement struct {
	Type              string `plist:",omitempty"`
	CoercionItemClass string `plist:",omitempty"`
	DictionaryKey     string `plist:",omitempty"`
	PropertyName      string `plist:",omitempty"`
	PropertyUserInfo  any    `plist:",omitempty"`
}

type WFDictionaryFieldValueItem struct {
	WFKey      any `plist:",omitempty"`
	WFItemType int `plist:",omitempty"`
	WFValue    any `plist:",omitempty"`
}

type WFValue struct {
	Value               any    `plist:",omitempty"`
	String              string `plist:",omitempty"`
	WFSerializationType string `plist:",omitempty"`
}

type WFInput struct {
	Value Value `plist:",omitempty"`
}

type WFContactFieldValue struct {
	EntryType       int                    `plist:",omitempty"`
	SerializedEntry map[string]interface{} `plist:",omitempty"`
}

type SizeValue struct {
	Unit      string `plist:",omitempty"`
	Magnitude string `plist:",omitempty"`
}

type WFMeasurementUnit struct {
	WFNSUnitSymbol any       `plist:",omitempty"`
	Value          SizeValue `plist:",omitempty"`
}

type WFTextTokenAttachment struct {
	Value               Value  `plist:",omitempty"`
	WFSerializationType string `plist:",omitempty"`
}

type WFTextTokenString struct {
	Value               WFTextTokenStringValue `plist:",omitempty"`
	WFSerializationType string                 `plist:",omitempty"`
}

type WFTextTokenStringValue struct {
	AttachmentsByRange map[string]Value `plist:"attachmentsByRange,omitempty"`
	String             string           `plist:"string,omitempty"`
}

type WFDictionaryFieldValue struct {
	Value               any    `plist:",omitempty"`
	WFSerializationType string `plist:",omitempty"`
}

type WFDictionaryFieldValueWrapper struct {
	WFDictionaryFieldValueItems []WFDictionaryFieldValueItem `plist:",omitempty"`
}

type WFArrayValue struct {
	WFSerializationType string                       `plist:",omitempty"`
	Value               []WFDictionaryFieldValueItem `plist:",omitempty"`
}

type WFBoolValue struct {
	WFSerializationType string `plist:",omitempty"`
	Value               bool   `plist:"Value"` // false is the zero value; omitempty would suppress it
}

type WFQuantityValue struct {
	Magnitude any `plist:",omitempty"`
	Unit      any `plist:",omitempty"`
}

type WFQuantityFieldValue struct {
	Value               WFQuantityValue `plist:",omitempty"`
	WFSerializationType string          `plist:",omitempty"`
}


type WFContentPredicateTableTemplate struct {
	Value               WFConditionValue `plist:",omitempty"`
	WFSerializationType string           `plist:",omitempty"`
}

type WFConditionValue struct {
	WFActionParameterFilterPrefix    int
	WFActionParameterFilterTemplates []WFConditionParam `plist:",omitempty"`
}

type WFConditionParam struct {
	// WFCondition is always emitted: LessThan serializes as 0, which
	// omitempty silently dropped, losing the operator from predicate
	// templates and crashing decompilation of those exports.
	WFCondition               int             `plist:"WFCondition"`
	WFInput                   WFInputVariable `plist:",omitempty"`
	WFConditionalActionString any             `plist:",omitempty"`
	WFNumberValue             any             `plist:",omitempty"`
	WFAnotherNumber           any             `plist:",omitempty"`
	WFDate                    any             `plist:",omitempty"`
	WFAnotherDate             any             `plist:",omitempty"`
	WFDuration                any             `plist:",omitempty"`
}

type WFInputVariable struct {
	Type     string                `plist:",omitempty"`
	Variable WFTextTokenAttachment `plist:",omitempty"`
}

type WFColorValue struct {
	WFColorRepresentationType string  `plist:",omitempty"`
	RedComponent              float64 `plist:"redComponent,omitempty"`
	GreenComponent            float64 `plist:"greenComponent,omitempty"`
	BlueComponent             float64 `plist:"blueComponent,omitempty"`
	AlphaComponent            any     `plist:"alphaComponent,omitempty"`
}

type WFActionReference struct {
	CustomOutputName string `plist:",omitempty"`
	UUID             string `plist:",omitempty"`
}

var uuids map[string]string

type dictDataType int

const itemTypeText dictDataType = 0
const itemTypeNumber dictDataType = 3
const itemTypeArray dictDataType = 2
const itemTypeDict dictDataType = 1
const itemTypeBool dictDataType = 4

var noInput map[string]any

var hasShortcutInputVariables = false

// ObjectReplaceChar is a Shortcuts convention to mark the placement of inline variables in a string.
const ObjectReplaceChar = '\uFFFC'
const ObjectReplaceCharStr = "\uFFFC"

var workflowName string

var definitions map[string]any

/* Colors */

var colors = schema.Colors
var iconColor = 3031607807

var altColors = schema.AltColors

/* Inputs */

var contentItems = schema.ContentItemClasses

var revContentItems map[string]string

func reversedContentItems() map[string]string {
	if len(revContentItems) == 0 {
		revContentItems = make(map[string]string)
		for key, item := range contentItems {
			revContentItems[item] = key
		}
	}

	return revContentItems
}

var inputs []string
var outputs []string

/* Workflow Types */

var workflowTypes = schema.WorkflowTypes
var definedWorkflowTypes []string

/* Quick Actions */

var quickActions = map[string]string{
	"finder":   "Finder",
	"services": "Services",
}
var definedQuickActions []string

/* Versions */

var versions = schema.Versions
var clientVersion = versions["27"]
var iosVersion = 27.0
var iosVersionString = "27"

/* Conditionals */

type WFConditions struct {
	conditions                    []condition
	WFActionParameterFilterPrefix int
	legacyComparison              bool
}

type condition struct {
	condition int
	arguments []actionArgument
}

var conditions = map[tokenType]int{
	Is:             4,
	Not:            5,
	Any:            100,
	Empty:          101,
	Contains:       99,
	DoesNotContain: 999,
	BeginsWith:     8,
	EndsWith:       9,
	GreaterThan:    2,
	GreaterOrEqual: 3,
	LessThan:       0,
	LessOrEqual:    1,
	Between:        1003,
	IsToday:        1002,
}

var conditionFilterPrefixes = map[tokenType]int{
	Or:  0,
	And: 1,
}

var allowedConditionalTypes = map[tokenType][]tokenType{
	Is:             {String, Integer, Bool, Action, Date},
	Not:            {String, Integer, Bool, Action, Date},
	Any:            {},
	Empty:          {},
	Contains:       {String, Arr},
	DoesNotContain: {String, Arr},
	BeginsWith:     {String},
	EndsWith:       {String},
	GreaterThan:    {Integer, Float, Quantity, Date},
	GreaterOrEqual: {Integer, Float, Date},
	LessThan:       {Integer, Float, Quantity, Date},
	LessOrEqual:    {Integer, Float, Date},
	Between:        {Integer, Float, Date},
	IsToday:        {},
}

/* Menus */

var menus map[string][]varValue
