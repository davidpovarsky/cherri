/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// The machine-readable action catalog is the shared representation of every
// action definition currently known to the compiler. It is consumed by the
// iOS bridge (CherriActionCatalog), the CLI (--actions-json), and downstream
// tooling such as the Shortcut corpus analyzer, docs generation, and preview
// metadata. Only metadata that is reliably known by Cherri is exposed; absent
// fields are omitted rather than invented.

type catalogParameter struct {
	Name       string   `json:"name"`
	Key        string   `json:"key,omitempty"`
	Type       string   `json:"type"`
	Optional   bool     `json:"optional,omitempty"`
	Infinite   bool     `json:"infinite,omitempty"`
	Reference  bool     `json:"reference,omitempty"`
	Literal    bool     `json:"literal,omitempty"`
	Enum       string   `json:"enum,omitempty"`
	EnumValues []string `json:"enumValues,omitempty"`
	Default    string   `json:"default,omitempty"`
}

type catalogActionInfo struct {
	Name               string             `json:"name"`
	ShortcutIdentifier string             `json:"shortcutIdentifier,omitempty"`
	Title              string             `json:"title,omitempty"`
	Description        string             `json:"description,omitempty"`
	Category           string             `json:"category,omitempty"`
	Subcategory        string             `json:"subcategory,omitempty"`
	Parameters         []catalogParameter `json:"parameters,omitempty"`
	OutputType         string             `json:"outputType,omitempty"`
	MacOnly            bool               `json:"macOnly,omitempty"`
	NonMacOnly         bool               `json:"nonMacOnly,omitempty"`
	MinVersion         float64            `json:"minVersion,omitempty"`
	MaxVersion         float64            `json:"maxVersion,omitempty"`
	Builtin            bool               `json:"builtin,omitempty"`
	Custom             bool               `json:"custom,omitempty"`
}

type actionCatalogResponse struct {
	OK      bool                `json:"ok"`
	Version string              `json:"version,omitempty"`
	Count   int                 `json:"count,omitempty"`
	Actions []catalogActionInfo `json:"actions,omitempty"`
	Error   string              `json:"error,omitempty"`
}

const actionCatalogVersion = "1"

// buildActionCatalog returns a stable, sorted representation of the actions
// map. It must not mutate global compiler state.
func buildActionCatalog() []catalogActionInfo {
	names := make([]string, 0, len(actions))
	for name := range actions {
		names = append(names, name)
	}
	sort.Strings(names)

	catalog := make([]catalogActionInfo, 0, len(names))
	for _, name := range names {
		definition := actions[name]
		if definition == nil {
			continue
		}

		parameters := make([]catalogParameter, 0, len(definition.parameters))
		for _, parameter := range definition.parameters {
			item := catalogParameter{
				Name:      parameter.name,
				Key:       parameter.key,
				Type:      string(parameter.validType),
				Optional:  parameter.optional || parameter.defaultValue != nil,
				Infinite:  parameter.infinite,
				Reference: parameter.ref,
				Literal:   parameter.literal,
				Enum:      parameter.enum,
			}
			if parameter.enum != "" {
				item.EnumValues = append([]string(nil), enumerations[parameter.enum]...)
			}
			if parameter.defaultValue != nil {
				item.Default = fmt.Sprint(parameter.defaultValue)
			}
			parameters = append(parameters, item)
		}

		catalog = append(catalog, catalogActionInfo{
			Name:               name,
			ShortcutIdentifier: catalogShortcutIdentifier(name, definition),
			Title:              definition.doc.title,
			Description:        definition.doc.description,
			Category:           definition.doc.category,
			Subcategory:        definition.doc.subcategory,
			Parameters:         parameters,
			OutputType:         string(definition.outputType),
			MacOnly:            definition.macOnly,
			NonMacOnly:         definition.nonMacOnly,
			MinVersion:         definition.minVersion,
			MaxVersion:         definition.maxVersion,
			Builtin:            definition.builtin,
			Custom:             !definition.builtin,
		})
	}
	return catalog
}

func catalogShortcutIdentifier(name string, definition *actionDefinition) string {
	if definition.overrideIdentifier != "" {
		return definition.overrideIdentifier
	}

	base := "is.workflow.actions"
	if definition.appIdentifier != "" {
		base = definition.appIdentifier
	}
	identifier := definition.identifier
	if identifier == "" {
		identifier = strings.ToLower(name)
	}
	return fmt.Sprintf("%s.%s", base, identifier)
}

// generateActionsJSON prints the full machine-readable action catalog.
func generateActionsJSON() {
	catalog := buildActionCatalog()
	response := actionCatalogResponse{
		OK:      true,
		Version: actionCatalogVersion,
		Count:   len(catalog),
		Actions: catalog,
	}
	encoded, err := json.Marshal(response)
	handle(err)
	fmt.Println(string(encoded))
}
