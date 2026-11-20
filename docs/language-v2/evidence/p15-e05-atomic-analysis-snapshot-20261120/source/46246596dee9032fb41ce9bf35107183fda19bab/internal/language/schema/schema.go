package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/types"
)

// LanguageVersion and CatalogSchemaVersion constants for v2.0
const (
	LanguageVersion      = "2.0"
	CatalogSchemaVersion = "3.0"
)

// OmissionPolicy indicates how an omitted parameter should be serialized.
type OmissionPolicy string

const (
	OmitPolicyUnspecified OmissionPolicy = "unspecified"
	OmitPolicyOmit        OmissionPolicy = "omit"
	OmitPolicyEmit        OmissionPolicy = "emit"
)

// EvidenceStatus represents verification state of an action or parameter.
type EvidenceStatus string

const (
	EvidenceConfirmed EvidenceStatus = "confirmed"
	EvidenceInferred  EvidenceStatus = "inferred"
	EvidenceObserved  EvidenceStatus = "observed"
)

// AppIntentDescriptor describes an App Intent binding.
type AppIntentDescriptor struct {
	Name                    string `json:"name,omitempty"`
	BundleIdentifier        string `json:"bundleIdentifier,omitempty"`
	AppIntentIdentifier     string `json:"appIntentIdentifier,omitempty"`
	TeamIdentifier          string `json:"teamIdentifier,omitempty"`
	RequiresAppInstallation *bool  `json:"requiresAppInstallation,omitempty"`
}

// ParameterSchema defines an argument accepted by an action.
type ParameterSchema struct {
	ID             string         `json:"id"`
	Label          string         `json:"label"`       // Public named argument label
	DisplayName    string         `json:"displayName"` // Human title
	Type           types.Type     `json:"-"`
	TypeName       string         `json:"type"`
	RawType        string         `json:"rawType,omitempty"`
	Optional       bool           `json:"optional"` // Whether argument may be omitted
	Reference      bool           `json:"reference,omitempty"`
	Literal        bool           `json:"literal,omitempty"`
	AcceptedForms  []string       `json:"acceptedForms,omitempty"`
	DefaultValue   *string        `json:"default,omitempty"` // nil means absent; a pointer to "" is an explicit empty default
	OmissionPolicy OmissionPolicy `json:"omissionPolicy,omitempty"`
	WireKey        string         `json:"wireKey,omitempty"`
	Codec          string         `json:"codec,omitempty"`
	EnumValues     []string       `json:"enumValues,omitempty"`
	EnumName       string         `json:"enumName,omitempty"`
	Variadic       bool           `json:"variadic,omitempty"`
	EvidenceStatus EvidenceStatus `json:"evidenceStatus,omitempty"`
}

// ActionDocs contains user documentation metadata for an action.
type ActionDocs struct {
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Category         string   `json:"category"`
	Subcategory      string   `json:"subcategory,omitempty"`
	InsertionSnippet string   `json:"insertionSnippet,omitempty"`
	Keywords         []string `json:"keywords,omitempty"`
}

// ActionSchema is the authoritative definition of an Apple Shortcut action.
type ActionSchema struct {
	ID                 string                           `json:"id"`              // Unique stable ID
	CallableName       string                           `json:"callableName"`    // Cherri v2 function/action name
	Module             string                           `json:"module"`          // Category or module
	AppleIdentifier    string                           `json:"appleIdentifier"` // is.workflow.actions.*
	Variant            string                           `json:"variant,omitempty"`
	PrimaryParameterID string                           `json:"primaryParameterId,omitempty"`
	Parameters         []ParameterSchema                `json:"parameters"`
	EmittedKeys        []string                         `json:"emittedKeys,omitempty"`
	OutputType         types.Type                       `json:"-"`
	OutputTypeName     string                           `json:"outputType,omitempty"`
	RawOutputType      string                           `json:"rawOutputType,omitempty"`
	AppIntent          *AppIntentDescriptor             `json:"appIntent,omitempty"`
	Docs               ActionDocs                       `json:"docs"`
	Builtin            bool                             `json:"builtin,omitempty"`
	Custom             bool                             `json:"custom,omitempty"`
	CompilerConstruct  bool                             `json:"compilerConstruct,omitempty"`
	ScopedBlock        bool                             `json:"scopedBlock,omitempty"`
	EvidenceStatus     EvidenceStatus                   `json:"evidenceStatus,omitempty"`
	StaticParameters   map[string]any                   `json:"staticParameters,omitempty"`
	MacOnly            bool                             `json:"macOnly,omitempty"`
	NonMacOnly         bool                             `json:"nonMacOnly,omitempty"`
	MinVersion         float64                          `json:"minVersion,omitempty"`
	MaxVersion         float64                          `json:"maxVersion,omitempty"`
	PlatformVersions   map[string]PlatformVersionBounds `json:"platformVersions,omitempty"`
}

// ParameterByLabel returns the parameter schema matching the given label.
func (a *ActionSchema) ParameterByLabel(label string) (*ParameterSchema, bool) {
	for i := range a.Parameters {
		if a.Parameters[i].Label == label {
			return &a.Parameters[i], true
		}
	}
	return nil, false
}

// PrimaryParameter returns the primary parameter schema if defined.
func (a *ActionSchema) PrimaryParameter() (*ParameterSchema, bool) {
	if a.PrimaryParameterID != "" {
		for i := range a.Parameters {
			if a.Parameters[i].ID == a.PrimaryParameterID {
				return &a.Parameters[i], true
			}
		}
	}
	for i := range a.Parameters {
		if strings.EqualFold(a.Parameters[i].ID, a.CallableName) || strings.EqualFold(a.Parameters[i].Label, a.CallableName) {
			return &a.Parameters[i], true
		}
	}
	if len(a.Parameters) == 1 {
		return &a.Parameters[0], true
	}
	return nil, false
}

// Registry is an immutable action and enum catalog.
type Registry struct {
	actionsByName       map[string]*ActionSchema
	actionsByIdentifier map[string][]*ActionSchema
	enumsByName         map[string][]string
	fingerprint         string
}

// NewRegistry constructs an immutable Registry from schemas and enums.
func NewRegistry(actions []*ActionSchema, enums map[string][]string) *Registry {
	r := &Registry{
		actionsByName:       make(map[string]*ActionSchema, len(actions)),
		actionsByIdentifier: make(map[string][]*ActionSchema),
		enumsByName:         make(map[string][]string),
	}

	for _, a := range actions {
		r.actionsByName[a.CallableName] = a
		if a.AppleIdentifier != "" {
			r.actionsByIdentifier[a.AppleIdentifier] = append(r.actionsByIdentifier[a.AppleIdentifier], a)
		}
	}

	for k, v := range enums {
		cpy := make([]string, len(v))
		copy(cpy, v)
		r.enumsByName[k] = cpy
	}

	r.fingerprint = r.computeFingerprint(actions)
	return r
}

// ResetDefaultRegistry restores the singleton DefaultRegistry to the clean generated 505-entry baseline.
func ResetDefaultRegistry() {
	defaultRegistryInstance = initDefaultRegistry()
}

// RegisterCustomAction dynamically adds or overrides an action schema in the registry.
func (r *Registry) RegisterCustomAction(a *ActionSchema) {
	if a == nil || a.CallableName == "" {
		return
	}
	// Reject unencodable metadata before publishing an action or replacing its
	// previously valid identity. NewRegistry also validates while fingerprinting.
	fingerprintJSON(a.StaticParameters)
	if r.actionsByName == nil {
		r.actionsByName = make(map[string]*ActionSchema)
	}
	// A source-local replacement must remove its old native-index entry while
	// preserving other callable variants of the same native action.
	if old := r.actionsByName[a.CallableName]; old != nil && old.AppleIdentifier != "" {
		var remaining []*ActionSchema
		for _, candidate := range r.actionsByIdentifier[old.AppleIdentifier] {
			if candidate.CallableName != a.CallableName {
				remaining = append(remaining, candidate)
			}
		}
		if len(remaining) == 0 {
			delete(r.actionsByIdentifier, old.AppleIdentifier)
		} else {
			r.actionsByIdentifier[old.AppleIdentifier] = remaining
		}
	}
	r.actionsByName[a.CallableName] = a
	if a.AppleIdentifier != "" {
		if r.actionsByIdentifier == nil {
			r.actionsByIdentifier = make(map[string][]*ActionSchema)
		}
		r.actionsByIdentifier[a.AppleIdentifier] = append(r.actionsByIdentifier[a.AppleIdentifier], a)
	}
	r.fingerprint = r.computeFingerprint(r.AllActions())
}

// CompareVersions compares two version strings (e.g., "27.10" vs "27.2", "16.4" vs "16.4.1") semantically.
// Returns -1 if v1 < v2, 0 if v1 == v2, and 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	parts1 := parseVersionParts(v1)
	parts2 := parseVersionParts(v2)

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		p1 := 0
		if i < len(parts1) {
			p1 = parts1[i]
		}
		p2 := 0
		if i < len(parts2) {
			p2 = parts2[i]
		}
		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}
	return 0
}

func parseVersionParts(v string) []int {
	v = strings.TrimSpace(v)
	if v == "" {
		return []int{0}
	}
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	fields := strings.Split(v, ".")
	parts := make([]int, 0, len(fields))
	for _, f := range fields {
		num := 0
		for _, ch := range f {
			if ch >= '0' && ch <= '9' {
				num = num*10 + int(ch-'0')
			} else {
				break
			}
		}
		parts = append(parts, num)
	}
	return parts
}

func (r *Registry) computeFingerprint(actions []*ActionSchema) string {
	// Sort actions by CallableName + AppleIdentifier to ensure deterministic order
	sorted := make([]*ActionSchema, len(actions))
	copy(sorted, actions)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].CallableName != sorted[j].CallableName {
			return sorted[i].CallableName < sorted[j].CallableName
		}
		return sorted[i].AppleIdentifier < sorted[j].AppleIdentifier
	})

	h := sha256.New()
	for _, a := range sorted {
		fmt.Fprintf(h, "ACT:%q:%q:%q:%q:%q:%t:%t:%t:%g:%g:%q\n",
			a.CallableName, a.AppleIdentifier, a.Variant, a.OutputTypeName, a.PrimaryParameterID,
			a.CompilerConstruct, a.MacOnly, a.NonMacOnly, a.MinVersion, a.MaxVersion, a.EvidenceStatus)
		if len(a.PlatformVersions) > 0 {
			platforms := make([]string, 0, len(a.PlatformVersions))
			for platform := range a.PlatformVersions {
				platforms = append(platforms, platform)
			}
			sort.Strings(platforms)
			for _, platform := range platforms {
				bounds := a.PlatformVersions[platform]
				fmt.Fprintf(h, "  PLATFORM:%q:%q:%q\n", platform, bounds.MinVersion, bounds.MaxVersion)
			}
		}
		if a.AppIntent != nil {
			fmt.Fprintf(h, "  INTENT:%q:%q:%q:%q\n", a.AppIntent.Name, a.AppIntent.BundleIdentifier, a.AppIntent.AppIntentIdentifier, a.AppIntent.TeamIdentifier)
		}
		if len(a.StaticParameters) > 0 {
			statKeys := make([]string, 0, len(a.StaticParameters))
			for k := range a.StaticParameters {
				statKeys = append(statKeys, k)
			}
			sort.Strings(statKeys)
			for _, k := range statKeys {
				fmt.Fprintf(h, "  STAT:%q=%s\n", k, fingerprintJSON(a.StaticParameters[k]))
			}
		}
		for _, p := range a.Parameters {
			fmt.Fprintf(h, "  P:%q:%q:%q:%t:%q:%q:%s:%t:%q:%q:%q\n",
				p.ID, p.Label, p.TypeName, p.Optional, p.WireKey, p.Codec, fingerprintDefault(p.DefaultValue), p.Variadic, p.OmissionPolicy, p.EnumName, p.EvidenceStatus)
			if len(p.AcceptedForms) > 0 {
				forms := make([]string, len(p.AcceptedForms))
				copy(forms, p.AcceptedForms)
				sort.Strings(forms)
				fmt.Fprintf(h, "    FORMS:%q\n", forms)
			}
			for _, ev := range p.EnumValues {
				fmt.Fprintf(h, "    E:%q\n", ev)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Fingerprint returns the deterministic SHA256 fingerprint of the semantic action definitions.
func (r *Registry) Fingerprint() string {
	return r.fingerprint
}

// LookupAction returns the ActionSchema by its callable name.
// Compiler-expanded debug/QA stdlib macros are resolved as a fallback without
// mutating the 505 canonical Apple action catalog or its fingerprint.
func (r *Registry) LookupAction(name string) (*ActionSchema, bool) {
	if a, ok := r.actionsByName[name]; ok {
		return a, true
	}
	if a, ok := compilerMacroSchemas[name]; ok {
		return a, true
	}
	return nil, false
}

var compilerMacroSchemas = map[string]*ActionSchema{
	"debugLog": {
		ID:                 "debugLog",
		CallableName:       "debugLog",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.appendvariable",
		PrimaryParameterID: "withDates",
		Parameters: []ParameterSchema{
			{ID: "withDates", Label: "withDates", DisplayName: "With Dates", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugWithDates", EvidenceStatus: EvidenceInferred},
			{ID: "showLogs", Label: "showLogs", DisplayName: "Show Logs", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugShowLogs", EvidenceStatus: EvidenceInferred},
			{ID: "showLogsAsText", Label: "showLogsAsText", DisplayName: "Show Logs As Text", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugShowLogsAsText", EvidenceStatus: EvidenceInferred},
			{ID: "label", Label: "label", DisplayName: "Label", Type: types.Text, TypeName: "text", RawType: "string", Optional: true, AcceptedForms: []string{"string"}, OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugLogLabel", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys:    []string{"WFDebugLogLabel", "WFDebugShowLogs", "WFDebugShowLogsAsText", "WFDebugWithDates", "WFVariableName"},
		OutputType:     types.Text,
		OutputTypeName: "text",
		RawOutputType:  "string",
		Docs: ActionDocs{
			Title:            "Debug Log",
			Description:      "Append previous native output without coercion. Optional withDates captures original Creation Date at source; showLogs previews native objects (with Set Name by Creation Date when enabled); showLogsAsText opts into Combine Text or dated text rows. A literal label stores separate scalar metadata and names preview copies; raw objects stay unchanged.",
			Category:         "debug",
			InsertionSnippet: "debugLog()",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugLogPrevious": {
		ID:                 "debugLogPrevious",
		CallableName:       "debugLogPrevious",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.appendvariable",
		PrimaryParameterID: "withDates",
		Parameters: []ParameterSchema{
			{ID: "withDates", Label: "withDates", DisplayName: "With Dates", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugWithDates", EvidenceStatus: EvidenceInferred},
			{ID: "showLogs", Label: "showLogs", DisplayName: "Show Logs", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugShowLogs", EvidenceStatus: EvidenceInferred},
			{ID: "showLogsAsText", Label: "showLogsAsText", DisplayName: "Show Logs As Text", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugShowLogsAsText", EvidenceStatus: EvidenceInferred},
			{ID: "label", Label: "label", DisplayName: "Label", Type: types.Text, TypeName: "text", RawType: "string", Optional: true, AcceptedForms: []string{"string"}, OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugLogLabel", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys:    []string{"WFDebugLogLabel", "WFDebugShowLogs", "WFDebugShowLogsAsText", "WFDebugWithDates"},
		OutputType:     types.Text,
		OutputTypeName: "text",
		RawOutputType:  "string",
		Docs: ActionDocs{
			Title:            "Log Previous Result",
			Description:      "Alias for debugLog with optional withDates/showLogs/showLogsAsText and a literal per-entry label.",
			Category:         "debug",
			InsertionSnippet: "debugLogPrevious()",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugShowLogs": {
		ID:                 "debugShowLogs",
		CallableName:       "debugShowLogs",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.previewdocument",
		PrimaryParameterID: "withDates",
		Parameters: []ParameterSchema{
			{ID: "withDates", Label: "withDates", DisplayName: "With Dates", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugWithDates", EvidenceStatus: EvidenceInferred},
			{ID: "showLogsAsText", Label: "showLogsAsText", DisplayName: "Show Logs As Text", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugShowLogsAsText", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys: []string{"WFDebugShowLogsAsText", "WFDebugWithDates"},
		OutputType:  types.Void,
		Docs: ActionDocs{
			Title:            "Show Debug Logs",
			Description:      "Preview native objects, naming items by Creation Date when enabled; use showLogsAsText for text formatting.",
			Category:         "debug",
			InsertionSnippet: "debugShowLogs()",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugFinish": {
		ID:                 "debugFinish",
		CallableName:       "debugFinish",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.previewdocument",
		PrimaryParameterID: "withDates",
		Parameters: []ParameterSchema{
			{ID: "withDates", Label: "withDates", DisplayName: "With Dates", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugWithDates", EvidenceStatus: EvidenceInferred},
			{ID: "showLogsAsText", Label: "showLogsAsText", DisplayName: "Show Logs As Text", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: true, AcceptedForms: []string{"bool", "expression", "reference"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugShowLogsAsText", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys: []string{"WFDebugShowLogsAsText", "WFDebugWithDates"},
		OutputType:  types.Void,
		Docs: ActionDocs{
			Title:            "Finish Debug Session",
			Description:      "Preview native objects with optional Creation Date names or display text with showLogsAsText.",
			Category:         "debug",
			InsertionSnippet: "debugFinish()",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugClearLogs": {
		ID:              "debugClearLogs",
		CallableName:    "debugClearLogs",
		Module:          "debug",
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters:      []ParameterSchema{},
		OutputType:      types.Void,
		Docs: ActionDocs{
			Title:            "Clear Debug Logs",
			Description:      "Reset the execution-local debug log to an empty value.",
			Category:         "debug",
			InsertionSnippet: "debugClearLogs()",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugTrace": {
		ID:                 "debugTrace",
		CallableName:       "debugTrace",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.appendvariable",
		PrimaryParameterID: "value",
		Parameters: []ParameterSchema{
			{ID: "value", Label: "value", DisplayName: "Value", Type: types.Text, TypeName: "text", RawType: "string", Optional: false, AcceptedForms: []string{"string", "interpolation", "expression", "reference"}, OmissionPolicy: OmitPolicyEmit, WireKey: "WFInput", Codec: "textToken", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys:    []string{"WFInput"},
		OutputType:     types.Text,
		OutputTypeName: "text",
		RawOutputType:  "string",
		Docs: ActionDocs{
			Title:            "Trace Value and Time",
			Description:      "Append current time followed by a value to the execution-local debug log.",
			Category:         "debug",
			InsertionSnippet: "debugTrace(value: <#text#>)",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugAssert": {
		ID:                 "debugAssert",
		CallableName:       "debugAssert",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.conditional",
		PrimaryParameterID: "condition",
		Parameters: []ParameterSchema{
			{ID: "condition", Label: "condition", DisplayName: "Condition", Type: types.Bool, TypeName: "bool", RawType: "bool", Optional: false, AcceptedForms: []string{"bool", "expression", "reference"}, OmissionPolicy: OmitPolicyEmit, WireKey: "WFInput", EvidenceStatus: EvidenceInferred},
			{ID: "message", Label: "message", DisplayName: "Message", Type: types.Text, TypeName: "text", RawType: "string", Optional: true, AcceptedForms: []string{"string", "interpolation", "expression", "reference"}, DefaultValue: StringDefault("Assertion failed"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFAlertActionMessage", Codec: "textToken", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys: []string{"WFAlertActionMessage", "WFInput"},
		OutputType:  types.Void,
		Docs: ActionDocs{
			Title:            "Assert Condition",
			Description:      "Stop the Shortcut with a message when the condition is false.",
			Category:         "debug",
			InsertionSnippet: "debugAssert(condition: <#bool#>)",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugStartTimer": {
		ID:                 "debugStartTimer",
		CallableName:       "debugStartTimer",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.date",
		PrimaryParameterID: "label",
		Parameters: []ParameterSchema{
			{ID: "label", Label: "label", DisplayName: "Label", Type: types.Text, TypeName: "text", RawType: "string", Optional: false, AcceptedForms: []string{"string", "interpolation", "expression", "reference"}, OmissionPolicy: OmitPolicyEmit, WireKey: "WFDebugTimerLabel", Codec: "textToken", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys: []string{"WFDebugTimerLabel"},
		OutputType:  types.Void,
		Docs: ActionDocs{
			Title:            "Start Debug Timer",
			Description:      "Remember the current time under a compile-time label.",
			Category:         "debug",
			InsertionSnippet: "debugStartTimer(label: <#text#>)",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugStopTimer": {
		ID:                 "debugStopTimer",
		CallableName:       "debugStopTimer",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.gettimebetweendates",
		PrimaryParameterID: "label",
		Parameters: []ParameterSchema{
			{ID: "label", Label: "label", DisplayName: "Label", Type: types.Text, TypeName: "text", RawType: "string", Optional: false, AcceptedForms: []string{"string", "interpolation", "expression", "reference"}, OmissionPolicy: OmitPolicyEmit, WireKey: "WFDebugTimerLabel", Codec: "textToken", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys:    []string{"WFDebugTimerLabel"},
		OutputType:     types.Number,
		OutputTypeName: "number",
		RawOutputType:  "float",
		Docs: ActionDocs{
			Title:            "Stop Debug Timer",
			Description:      "Measure elapsed seconds and append the result to the execution-local log.",
			Category:         "debug",
			InsertionSnippet: "debugStopTimer(label: <#text#>)",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugTraceEvent": {
		ID:                 "debugTraceEvent",
		CallableName:       "debugTraceEvent",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.appendvariable",
		PrimaryParameterID: "stage",
		Parameters: []ParameterSchema{
			{ID: "stage", Label: "stage", DisplayName: "Stage", Type: types.Text, TypeName: "text", RawType: "string", Optional: false, AcceptedForms: []string{"string"}, OmissionPolicy: OmitPolicyEmit, WireKey: "WFDebugTraceStage", EvidenceStatus: EvidenceInferred},
			{ID: "label", Label: "label", DisplayName: "Label", Type: types.Text, TypeName: "text", RawType: "string", Optional: true, AcceptedForms: []string{"string"}, DefaultValue: StringDefault(""), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugTraceLabel", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys:    []string{"WFDebugTraceLabel", "WFDebugTraceStage"},
		OutputType:     types.Text,
		OutputTypeName: "text",
		RawOutputType:  "string",
		Docs: ActionDocs{
			Title:            "Record QA Structured Trace Event",
			Description:      "Append a structured JSONL trace record (stage, label, step sequence, and CurrentDate timestamp) to the execution-local QA trace buffer without mutating __cherri_debug_log.",
			Category:         "debug",
			InsertionSnippet: "debugTraceEvent(stage: \"after\", label: \"step-1\")",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
	"debugExportTrace": {
		ID:                 "debugExportTrace",
		CallableName:       "debugExportTrace",
		Module:             "debug",
		AppleIdentifier:    "is.workflow.actions.gettext",
		PrimaryParameterID: "runId",
		Parameters: []ParameterSchema{
			{ID: "runId", Label: "runId", DisplayName: "Run ID", Type: types.Text, TypeName: "text", RawType: "string", Optional: true, AcceptedForms: []string{"string"}, DefaultValue: StringDefault("cherri-qa-run"), OmissionPolicy: OmitPolicyOmit, WireKey: "WFDebugTraceRunID", EvidenceStatus: EvidenceInferred},
		},
		EmittedKeys:    []string{"WFDebugTraceRunID"},
		OutputType:     types.Text,
		OutputTypeName: "text",
		RawOutputType:  "string",
		Docs: ActionDocs{
			Title:            "Export QA Structured Trace Snapshot",
			Description:      "Combine the execution-local QA trace buffer into a deterministic JSON envelope for QA inspection or callback export without mutating __cherri_debug_log.",
			Category:         "debug",
			InsertionSnippet: "debugExportTrace(runId: \"qa-run-1\")",
		},
		Builtin:        true,
		EvidenceStatus: EvidenceInferred,
	},
}

// LookupByIdentifier returns all ActionSchemas matching the Apple identifier.
func (r *Registry) LookupByIdentifier(ident string) []*ActionSchema {
	return r.actionsByIdentifier[ident]
}

// AllActions returns all registered action schemas sorted by name.
func (r *Registry) AllActions() []*ActionSchema {
	res := make([]*ActionSchema, 0, len(r.actionsByName))
	for _, a := range r.actionsByName {
		res = append(res, a)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CallableName < res[j].CallableName
	})
	return res
}

// ActionNames returns all valid callable names sorted.
func (r *Registry) ActionNames() []string {
	names := make([]string, 0, len(r.actionsByName))
	for k := range r.actionsByName {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// LookupEnum returns the valid enum values for an enum name.
func (r *Registry) LookupEnum(enumName string) ([]string, bool) {
	vals, ok := r.enumsByName[enumName]
	return vals, ok
}

// AllEnums returns all enum mappings.
func (r *Registry) AllEnums() map[string][]string {
	res := make(map[string][]string, len(r.enumsByName))
	for k, v := range r.enumsByName {
		cpy := make([]string, len(v))
		copy(cpy, v)
		res[k] = cpy
	}
	return res
}
