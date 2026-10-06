package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

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
	Name                string `json:"name,omitempty"`
	BundleIdentifier    string `json:"bundleIdentifier,omitempty"`
	AppIntentIdentifier string `json:"appIntentIdentifier,omitempty"`
	TeamIdentifier      string `json:"teamIdentifier,omitempty"`
}

// ParameterSchema defines an argument accepted by an action.
type ParameterSchema struct {
	ID             string            `json:"id"`
	Label          string            `json:"label"`          // Public named argument label
	DisplayName    string            `json:"displayName"`    // Human title
	Type           types.Type        `json:"-"`
	TypeName       string            `json:"type"`
	Optional       bool              `json:"optional"`       // Whether argument may be omitted
	AcceptedForms  []string          `json:"acceptedForms,omitempty"`
	DefaultValue   string            `json:"default,omitempty"`
	OmissionPolicy OmissionPolicy    `json:"omissionPolicy,omitempty"`
	WireKey        string            `json:"wireKey,omitempty"`
	Codec          string            `json:"codec,omitempty"`
	EnumValues     []string          `json:"enumValues,omitempty"`
	EnumName       string            `json:"enumName,omitempty"`
	Variadic       bool              `json:"variadic,omitempty"`
	EvidenceStatus EvidenceStatus    `json:"evidenceStatus,omitempty"`
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
	ID                 string               `json:"id"`                 // Unique stable ID
	CallableName       string               `json:"callableName"`       // Cherri v2 function/action name
	Module             string               `json:"module"`             // Category or module
	AppleIdentifier    string               `json:"appleIdentifier"`    // is.workflow.actions.*
	Variant            string               `json:"variant,omitempty"`
	PrimaryParameterID string               `json:"primaryParameterId,omitempty"`
	Parameters         []ParameterSchema    `json:"parameters"`
	OutputType         types.Type           `json:"-"`
	OutputTypeName     string               `json:"outputType,omitempty"`
	AppIntent          *AppIntentDescriptor `json:"appIntent,omitempty"`
	Docs               ActionDocs           `json:"docs"`
	CompilerConstruct  bool                 `json:"compilerConstruct,omitempty"`
	EvidenceStatus     EvidenceStatus       `json:"evidenceStatus,omitempty"`
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
	if a.PrimaryParameterID == "" {
		return nil, false
	}
	for i := range a.Parameters {
		if a.Parameters[i].ID == a.PrimaryParameterID {
			return &a.Parameters[i], true
		}
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
		fmt.Fprintf(h, "ACT:%s:%s:%s:%s\n", a.CallableName, a.AppleIdentifier, a.OutputTypeName, a.PrimaryParameterID)
		for _, p := range a.Parameters {
			fmt.Fprintf(h, "  P:%s:%s:%s:%t:%s:%s\n", p.ID, p.Label, p.TypeName, p.Optional, p.WireKey, p.Codec)
			for _, ev := range p.EnumValues {
				fmt.Fprintf(h, "    E:%s\n", ev)
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
func (r *Registry) LookupAction(name string) (*ActionSchema, bool) {
	a, ok := r.actionsByName[name]
	return a, ok
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
