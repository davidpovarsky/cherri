package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LegacyCatalog matches the structure of baseline-catalog.json
type LegacyCatalog struct {
	Actions []LegacyAction `json:"actions"`
}

type LegacyAction struct {
	Name               string            `json:"name"`
	ShortcutIdentifier string            `json:"shortcutIdentifier"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Category           string            `json:"category"`
	Subcategory        string            `json:"subcategory"`
	Parameters         []LegacyParameter `json:"parameters"`
	OutputType         string            `json:"outputType"`
	CompilerConstruct  bool              `json:"compilerConstruct"`
	Builtin            bool              `json:"builtin"`
	Custom             bool              `json:"custom"`
	InsertionSnippet   string            `json:"insertionSnippet"`
	AppIntent          *LegacyAppIntent  `json:"appIntent"`
}

type LegacyParameter struct {
	Name       string   `json:"name"`
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Optional   bool     `json:"optional"`
	Default    string   `json:"default"`
	Enum       string   `json:"enum"`
	EnumValues []string `json:"enumValues"`
	Infinite   bool     `json:"infinite"`
}

type LegacyAppIntent struct {
	Name                string `json:"name"`
	BundleIdentifier    string `json:"bundleIdentifier"`
	AppIntentIdentifier string `json:"appIntentIdentifier"`
	TeamIdentifier      string `json:"teamIdentifier"`
}

// Facet overrides for semantic enrichment of legacy definitions
type ActionFacet struct {
	PrimaryParameterID string
	OutputTypeOverride string
	ParamTypeOverrides map[string]string // paramName -> semantic type
}

var facets = map[string]ActionFacet{
	"resizeImage": {
		PrimaryParameterID: "image",
		OutputTypeOverride: "Image",
		ParamTypeOverrides: map[string]string{
			"image":  "Image",
			"width":  "Number",
			"height": "Number",
		},
	},
	"convertImage": {
		PrimaryParameterID: "image",
		OutputTypeOverride: "Image",
		ParamTypeOverrides: map[string]string{
			"image": "Image",
		},
	},
	"text": {
		PrimaryParameterID: "text",
		OutputTypeOverride: "Text",
		ParamTypeOverrides: map[string]string{
			"text": "Text",
		},
	},
	"show": {
		PrimaryParameterID: "input",
		OutputTypeOverride: "Void",
	},
	"alert": {
		PrimaryParameterID: "alert",
		OutputTypeOverride: "Void",
		ParamTypeOverrides: map[string]string{
			"alert": "Text",
			"title": "Text",
		},
	},
	"count": {
		PrimaryParameterID: "input",
		OutputTypeOverride: "Number",
		ParamTypeOverrides: map[string]string{
			"input": "AnyContent",
		},
	},
	"date": {
		OutputTypeOverride: "Date",
	},
	"formatDate": {
		PrimaryParameterID: "date",
		OutputTypeOverride: "Text",
		ParamTypeOverrides: map[string]string{
			"date": "Date",
		},
	},
}

func validateFacets(catalog *LegacyCatalog) error {
	actionsByName := make(map[string]*LegacyAction)
	for i := range catalog.Actions {
		actionsByName[catalog.Actions[i].Name] = &catalog.Actions[i]
	}

	for facetName, facet := range facets {
		act, ok := actionsByName[facetName]
		if !ok {
			return fmt.Errorf("facet references unknown action %q", facetName)
		}

		if facet.PrimaryParameterID != "" {
			paramFound := false
			for _, p := range act.Parameters {
				if p.Name == facet.PrimaryParameterID {
					paramFound = true
					break
				}
			}
			if !paramFound {
				return fmt.Errorf("facet for %q references unknown primary parameter %q", facetName, facet.PrimaryParameterID)
			}
		}

		for paramName := range facet.ParamTypeOverrides {
			paramFound := false
			for _, p := range act.Parameters {
				if p.Name == paramName {
					paramFound = true
					break
				}
			}
			if !paramFound {
				return fmt.Errorf("facet for %q references unknown parameter %q in type overrides", facetName, paramName)
			}
		}
	}
	return nil
}

func main() {
	var data []byte
	var catalogPath string

	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		catalogPath = os.Args[1]
		var err error
		data, err = os.ReadFile(catalogPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading catalog file %s: %v\n", catalogPath, err)
			os.Exit(1)
		}
	} else {
		// Prefer current live definition facts from root compiler via --actions-json
		catalogPath = filepath.Join("docs", "language-v2", "baseline-catalog.json")
		if _, err := os.Stat(catalogPath); err == nil {
			var readErr error
			data, readErr = os.ReadFile(catalogPath)
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "Error reading baseline catalog: %v\n", readErr)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "No catalog path provided and %s not found\n", catalogPath)
			os.Exit(1)
		}
	}

	var catalog LegacyCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing catalog JSON: %v\n", err)
		os.Exit(1)
	}

	if err := validateFacets(&catalog); err != nil {
		fmt.Fprintf(os.Stderr, "Facet validation failed: %v\n", err)
		os.Exit(1)
	}

	outPath := filepath.Join("internal", "language", "schema", "generated_registry.go")
	if len(os.Args) > 2 {
		outPath = os.Args[2]
	}

	code := generateGoCode(&catalog)
	if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing generated code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s successfully from %d actions (all facets valid).\n", outPath, len(catalog.Actions))
}

func mapType(legacyType string) string {
	switch strings.ToLower(legacyType) {
	case "text", "string":
		return "Text"
	case "number", "int", "integer", "float":
		return "Number"
	case "bool", "boolean":
		return "Bool"
	case "dictionary", "dict":
		return "Map<Text, AnyContent>"
	case "array", "list":
		return "List<AnyContent>"
	case "variable":
		return "AnyContent"
	default:
		return "Unknown"
	}
}

func mapCodec(legacyType string) string {
	switch strings.ToLower(legacyType) {
	case "text", "string":
		return "text"
	case "number", "int", "integer", "float":
		return "number"
	case "bool", "boolean":
		return "bool"
	case "dictionary", "dict":
		return "dictionary"
	case "array", "list":
		return "array"
	case "variable":
		return "token"
	default:
		return "token"
	}
}

func escapeString(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\"", "\\\"")
}

func generateGoCode(catalog *LegacyCatalog) string {
	var b strings.Builder
	b.WriteString(`// Code generated by tools/language-schema; DO NOT EDIT.

package schema

import (
	"github.com/electrikmilk/cherri/internal/language/types"
)

var defaultRegistryInstance *Registry

// DefaultRegistry returns the immutable shared action registry.
func DefaultRegistry() *Registry {
	if defaultRegistryInstance == nil {
		defaultRegistryInstance = initDefaultRegistry()
	}
	return defaultRegistryInstance
}

func initDefaultRegistry() *Registry {
	actions := []*ActionSchema{
`)

	allEnums := make(map[string][]string)

	for _, a := range catalog.Actions {
		facet, hasFacet := facets[a.Name]

		primaryID := ""
		if hasFacet && facet.PrimaryParameterID != "" {
			primaryID = facet.PrimaryParameterID
		} else if len(a.Parameters) > 0 && !a.Parameters[0].Infinite {
			primaryID = a.Parameters[0].Name
		}

		outType := "types.Unknown"
		outTypeName := "Unknown"
		if hasFacet && facet.OutputTypeOverride != "" {
			outTypeName = facet.OutputTypeOverride
			outType = mapGoType(outTypeName)
		} else if a.OutputType != "" {
			outTypeName = mapType(a.OutputType)
			outType = mapGoType(outTypeName)
		}

		b.WriteString("\t\t{\n")
		fmt.Fprintf(&b, "\t\t\tID: %q,\n", a.Name)
		fmt.Fprintf(&b, "\t\t\tCallableName: %q,\n", a.Name)
		fmt.Fprintf(&b, "\t\t\tModule: %q,\n", a.Category)
		fmt.Fprintf(&b, "\t\t\tAppleIdentifier: %q,\n", a.ShortcutIdentifier)
		if primaryID != "" {
			fmt.Fprintf(&b, "\t\t\tPrimaryParameterID: %q,\n", primaryID)
		}
		fmt.Fprintf(&b, "\t\t\tOutputType: %s,\n", outType)
		fmt.Fprintf(&b, "\t\t\tOutputTypeName: %q,\n", outTypeName)
		fmt.Fprintf(&b, "\t\t\tCompilerConstruct: %t,\n", a.CompilerConstruct)
		fmt.Fprintf(&b, "\t\t\tEvidenceStatus: EvidenceConfirmed,\n")
		b.WriteString("\t\t\tDocs: ActionDocs{\n")
		fmt.Fprintf(&b, "\t\t\t\tTitle: %q,\n", a.Title)
		fmt.Fprintf(&b, "\t\t\t\tDescription: %q,\n", escapeString(a.Description))
		fmt.Fprintf(&b, "\t\t\t\tCategory: %q,\n", a.Category)
		fmt.Fprintf(&b, "\t\t\t\tSubcategory: %q,\n", a.Subcategory)
		fmt.Fprintf(&b, "\t\t\t\tInsertionSnippet: %q,\n", escapeString(a.InsertionSnippet))
		b.WriteString("\t\t\t},\n")

		if a.AppIntent != nil {
			b.WriteString("\t\t\tAppIntent: &AppIntentDescriptor{\n")
			fmt.Fprintf(&b, "\t\t\t\tName: %q,\n", a.AppIntent.Name)
			fmt.Fprintf(&b, "\t\t\t\tBundleIdentifier: %q,\n", a.AppIntent.BundleIdentifier)
			fmt.Fprintf(&b, "\t\t\t\tAppIntentIdentifier: %q,\n", a.AppIntent.AppIntentIdentifier)
			fmt.Fprintf(&b, "\t\t\t\tTeamIdentifier: %q,\n", a.AppIntent.TeamIdentifier)
			b.WriteString("\t\t\t},\n")
		}

		b.WriteString("\t\t\tParameters: []ParameterSchema{\n")
		for _, p := range a.Parameters {
			pType := mapType(p.Type)
			if hasFacet && facet.ParamTypeOverrides != nil {
				if override, ok := facet.ParamTypeOverrides[p.Name]; ok {
					pType = override
				}
			}

			codec := mapCodec(p.Type)
			if p.Enum != "" {
				codec = "enum"
				if len(p.EnumValues) > 0 {
					allEnums[p.Enum] = p.EnumValues
				}
			}

			b.WriteString("\t\t\t\t{\n")
			fmt.Fprintf(&b, "\t\t\t\t\tID: %q,\n", p.Name)
			fmt.Fprintf(&b, "\t\t\t\t\tLabel: %q,\n", p.Name)
			fmt.Fprintf(&b, "\t\t\t\t\tDisplayName: %q,\n", p.Name)
			fmt.Fprintf(&b, "\t\t\t\t\tType: %s,\n", mapGoType(pType))
			fmt.Fprintf(&b, "\t\t\t\t\tTypeName: %q,\n", pType)
			fmt.Fprintf(&b, "\t\t\t\t\tOptional: %t,\n", p.Optional)
			fmt.Fprintf(&b, "\t\t\t\t\tWireKey: %q,\n", p.Key)
			fmt.Fprintf(&b, "\t\t\t\t\tCodec: %q,\n", codec)
			if p.Default != "" {
				fmt.Fprintf(&b, "\t\t\t\t\tDefaultValue: %q,\n", escapeString(p.Default))
			}
			if p.Enum != "" {
				fmt.Fprintf(&b, "\t\t\t\t\tEnumName: %q,\n", p.Enum)
				if len(p.EnumValues) > 0 {
					b.WriteString("\t\t\t\t\tEnumValues: []string{")
					for i, ev := range p.EnumValues {
						if i > 0 {
							b.WriteString(", ")
						}
						fmt.Fprintf(&b, "%q", ev)
					}
					b.WriteString("},\n")
				}
			}
			fmt.Fprintf(&b, "\t\t\t\t\tVariadic: %t,\n", p.Infinite)
			fmt.Fprintf(&b, "\t\t\t\t\tEvidenceStatus: EvidenceConfirmed,\n")
			b.WriteString("\t\t\t\t},\n")
		}
		b.WriteString("\t\t\t},\n")
		b.WriteString("\t\t},\n")
	}

	b.WriteString("\t}\n\n")

	b.WriteString("\tenums := map[string][]string{\n")
	sortedEnumNames := make([]string, 0, len(allEnums))
	for k := range allEnums {
		sortedEnumNames = append(sortedEnumNames, k)
	}
	sort.Strings(sortedEnumNames)
	for _, k := range sortedEnumNames {
		vals := allEnums[k]
		fmt.Fprintf(&b, "\t\t%q: {", k)
		for i, v := range vals {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q", v)
		}
		b.WriteString("},\n")
	}
	b.WriteString("\t}\n\n")

	b.WriteString("\treturn NewRegistry(actions, enums)\n}\n")
	return b.String()
}

func mapGoType(t string) string {
	switch t {
	case "Text":
		return "types.Text"
	case "Number":
		return "types.Number"
	case "Bool":
		return "types.Bool"
	case "Image":
		return "types.Image"
	case "File":
		return "types.File"
	case "URL":
		return "types.URL"
	case "Date":
		return "types.Date"
	case "Contact":
		return "types.Contact"
	case "CalendarEvent":
		return "types.CalendarEvent"
	case "AnyContent":
		return "types.AnyContent"
	case "Void":
		return "types.Void"
	case "List<Image>":
		return "types.NewList(types.Image)"
	case "List<AnyContent>":
		return "types.NewList(types.AnyContent)"
	case "Map<Text, AnyContent>":
		return "types.NewMap(types.Text, types.AnyContent)"
	default:
		return "types.Unknown"
	}
}
