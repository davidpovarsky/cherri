package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// LegacyCatalog matches the structure of baseline-catalog.json or live --actions-json output
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
	StaticParameters   map[string]any    `json:"staticParameters,omitempty"`
	MacOnly            bool              `json:"macOnly,omitempty"`
	NonMacOnly         bool              `json:"nonMacOnly,omitempty"`
	MinVersion         float64           `json:"minVersion,omitempty"`
	MaxVersion         float64           `json:"maxVersion,omitempty"`
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
		ParamTypeOverrides: map[string]string{
			"input": "AnyContent",
		},
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
	"getParentDirectory": {
		PrimaryParameterID: "input",
		OutputTypeOverride: "File",
	},
	"getFile": {
		PrimaryParameterID: "path",
		OutputTypeOverride: "File",
	},
	"selectFolder": {
		OutputTypeOverride: "File",
	},
	"makeDiskImage": {
		PrimaryParameterID: "name",
		OutputTypeOverride: "File",
	},
	"makeSizedDiskImage": {
		PrimaryParameterID: "name",
		OutputTypeOverride: "File",
	},
	"takeInteractiveScreenshot": {
		OutputTypeOverride: "Image",
	},
	"startScreensaver": {
		OutputTypeOverride: "Void",
	},
	"sleep": {
		OutputTypeOverride: "Void",
	},
	"displaySleep": {
		OutputTypeOverride: "Void",
	},
	"vibrate": {
		OutputTypeOverride: "Void",
	},
	"wait": {
		PrimaryParameterID: "seconds",
		OutputTypeOverride: "Void",
	},
	"waitToReturn": {
		OutputTypeOverride: "Void",
	},
	"openFile": {
		PrimaryParameterID: "file",
		OutputTypeOverride: "Void",
	},
	"setSilentMode": {
		OutputTypeOverride: "Void",
	},
	"stopStopwatch": {
		OutputTypeOverride: "Void",
	},
	"playAudiobook": {
		PrimaryParameterID: "target",
		OutputTypeOverride: "Void",
	},
	"openBook": {
		PrimaryParameterID: "target",
		OutputTypeOverride: "Void",
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
		// Production path: Extract live definition facts directly from compiler via --actions-json
		// This guarantees that canonical action definitions in actions/*.cherri and actions_std.go are the single source of truth.
		cmd := exec.Command("go", "run", ".", "--actions-json")
		if _, err := os.Stat("main.go"); err != nil {
			dir, _ := os.Getwd()
			for i := 0; i < 3; i++ {
				parent := filepath.Dir(dir)
				if _, statErr := os.Stat(filepath.Join(parent, "main.go")); statErr == nil {
					cmd.Dir = parent
					break
				}
				dir = parent
			}
		}
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			// Fallback if go run is blocked in isolated test directory: check baseline
			catalogPath = filepath.Join("docs", "language-v2", "baseline-catalog.json")
			var readErr error
			data, readErr = os.ReadFile(catalogPath)
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "Error extracting live catalog (%v: %s) and reading baseline (%v)\n", err, stderr.String(), readErr)
				os.Exit(1)
			}
		} else {
			data = stdout.Bytes()
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

	verifyOnly := false
	for _, arg := range os.Args[1:] {
		if arg == "--verify-generated" {
			verifyOnly = true
			break
		}
	}

	outPath := filepath.Join("internal", "language", "schema", "generated_registry.go")
	if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") {
		outPath = os.Args[2]
	}

	code := generateGoCode(&catalog)

	if verifyOnly {
		existing, err := os.ReadFile(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading existing %s for verification: %v\n", outPath, err)
			os.Exit(1)
		}
		if string(existing) != code {
			fmt.Fprintf(os.Stderr, "Drift detected! Generated registry at %s differs from live catalog definition.\n", outPath)
			os.Exit(1)
		}
		fmt.Printf("Generated registry at %s is verified and up-to-date with live catalog.\n", outPath)
		return
	}

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
	case "file":
		return "File"
	case "url":
		return "URL"
	case "date":
		return "Date"
	case "image":
		return "Image"
	case "contact":
		return "Contact"
	case "calendarevent", "event":
		return "CalendarEvent"
	case "void", "none":
		return "Void"
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

type AppleParameterEncoding struct {
	Key              string   `json:"key"`
	Class            string   `json:"class"`
	SingleStateClass string   `json:"singleStateClass"`
	VariableTypes    []string `json:"variableTypes"`
	Default          any      `json:"default"`
}

type AppleEncodingsData struct {
	ActionParameters map[string][]AppleParameterEncoding `json:"actionParameters"`
}

func readEncodingsFromPath(p string) (*AppleEncodingsData, error) {
	if strings.HasSuffix(p, ".gz") {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		data, err := io.ReadAll(gz)
		if err != nil {
			return nil, err
		}
		var res AppleEncodingsData
		if err := json.Unmarshal(data, &res); err != nil {
			return nil, err
		}
		return &res, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var res AppleEncodingsData
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func loadAppleEncodings() *AppleEncodingsData {
	if custom := os.Getenv("CHERRI_APPLE_RUNTIME_DIR"); custom != "" {
		for _, name := range []string{"parameter-encodings.json.gz", "parameter-encodings.json"} {
			p := filepath.Join(custom, name)
			if res, err := readEncodingsFromPath(p); err == nil && res != nil {
				return res
			}
		}
		panic(fmt.Sprintf("CHERRI_APPLE_RUNTIME_DIR explicitly set to %q, but failed to load valid parameter encodings from it", custom))
	}
	searchDirs := []string{
		filepath.Join("docs", "apple-runtime", "macos-27"),
		filepath.Join("..", "docs", "apple-runtime", "macos-27"),
		filepath.Join("..", "..", "docs", "apple-runtime", "macos-27"),
	}
	for _, dir := range searchDirs {
		for _, name := range []string{"parameter-encodings.json.gz", "parameter-encodings.json"} {
			p := filepath.Join(dir, name)
			if res, err := readEncodingsFromPath(p); err == nil && res != nil {
				return res
			}
		}
	}
	return nil
}

func determineAcceptedFormsAndEvidence(a *LegacyAction, p LegacyParameter, codec string, appleEnc *AppleEncodingsData) ([]string, string) {
	if appleEnc != nil && a.ShortcutIdentifier != "" {
		if params, ok := appleEnc.ActionParameters[a.ShortcutIdentifier]; ok {
			for i := range params {
				if params[i].Key == p.Key {
					forms, confirmed := determineConfirmedForms(&params[i], codec)
					if confirmed {
						return forms, "EvidenceConfirmed"
					}
					return forms, "EvidenceObserved"
				}
			}
		}
	}
	return determineInferredForms(codec), "EvidenceInferred"
}

func determineConfirmedForms(ap *AppleParameterEncoding, codec string) ([]string, bool) {
	if ap.Class == "WFVariablePickerParameter" || ap.SingleStateClass == "WFVariableParameterState" {
		return []string{"attachment(output)", "attachment(variable)"}, true
	}
	if ap.Class == "WFVariableFieldParameter" || ap.SingleStateClass == "WFStringParameterState" {
		return []string{"literal"}, true
	}
	hasVar := false
	for _, vt := range ap.VariableTypes {
		if vt == "Variable" || vt == "ActionOutput" {
			hasVar = true
			break
		}
	}

	if ap.Class == "WFTextInputParameter" || ap.SingleStateClass == "WFVariableStringParameterState" {
		if hasVar {
			return []string{"literal", "tokenString", "attachment(output)", "attachment(variable)"}, true
		}
		return []string{"literal", "tokenString"}, true
	}

	if !hasVar && len(ap.VariableTypes) > 0 {
		return []string{"literal"}, true
	}

	if ap.Class != "" || ap.SingleStateClass != "" {
		switch codec {
		case "text":
			return []string{"literal", "tokenString"}, true
		case "token":
			return []string{"attachment(output)", "attachment(variable)"}, true
		default:
			return []string{"literal", "attachment(output)", "attachment(variable)"}, true
		}
	}

	return determineInferredForms(codec), false
}

func determineInferredForms(codec string) []string {
	switch codec {
	case "text":
		return []string{"literal", "tokenString"}
	case "token":
		return []string{"attachment(output)", "attachment(variable)"}
	default:
		return []string{"literal", "attachment(output)", "attachment(variable)"}
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
	appleEnc := loadAppleEncodings()

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

		actionEvidence := "EvidenceInferred"
		if appleEnc != nil && a.ShortcutIdentifier != "" && len(appleEnc.ActionParameters[a.ShortcutIdentifier]) > 0 {
			actionEvidence = "EvidenceConfirmed"
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
		fmt.Fprintf(&b, "\t\t\tEvidenceStatus: %s,\n", actionEvidence)
		if a.MacOnly {
			fmt.Fprintf(&b, "\t\t\tMacOnly: true,\n")
		}
		if a.NonMacOnly {
			fmt.Fprintf(&b, "\t\t\tNonMacOnly: true,\n")
		}
		if a.MinVersion > 0 {
			fmt.Fprintf(&b, "\t\t\tMinVersion: %v,\n", a.MinVersion)
		}
		if a.MaxVersion > 0 {
			fmt.Fprintf(&b, "\t\t\tMaxVersion: %v,\n", a.MaxVersion)
		}
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

		if len(a.StaticParameters) > 0 {
			b.WriteString("\t\t\tStaticParameters: map[string]any{\n")
			keys := make([]string, 0, len(a.StaticParameters))
			for k := range a.StaticParameters {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				val := a.StaticParameters[k]
				fmt.Fprintf(&b, "\t\t\t\t%q: %s,\n", k, formatGoLiteral(val, "\t\t\t\t"))
			}
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

			forms, paramEvidence := determineAcceptedFormsAndEvidence(&a, p, codec, appleEnc)

			b.WriteString("\t\t\t\t{\n")
			fmt.Fprintf(&b, "\t\t\t\t\tID: %q,\n", p.Name)
			fmt.Fprintf(&b, "\t\t\t\t\tLabel: %q,\n", p.Name)
			fmt.Fprintf(&b, "\t\t\t\t\tDisplayName: %q,\n", p.Name)
			fmt.Fprintf(&b, "\t\t\t\t\tType: %s,\n", mapGoType(pType))
			fmt.Fprintf(&b, "\t\t\t\t\tTypeName: %q,\n", pType)
			fmt.Fprintf(&b, "\t\t\t\t\tOptional: %t,\n", p.Optional)
			if len(forms) > 0 {
				b.WriteString("\t\t\t\t\tAcceptedForms: []string{")
				for i, af := range forms {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "%q", af)
				}
				b.WriteString("},\n")
			}
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
			fmt.Fprintf(&b, "\t\t\t\t\tEvidenceStatus: %s,\n", paramEvidence)
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

func formatGoLiteral(val any, indent string) string {
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case map[string]any:
		if len(v) == 0 {
			return "map[string]any{}"
		}
		var b bytes.Buffer
		b.WriteString("map[string]any{\n")
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(indent + "\t")
			fmt.Fprintf(&b, "%q: %s,\n", k, formatGoLiteral(v[k], indent+"\t"))
		}
		b.WriteString(indent + "}")
		return b.String()
	case []any:
		if len(v) == 0 {
			return "[]any{}"
		}
		var b bytes.Buffer
		b.WriteString("[]any{\n")
		for _, item := range v {
			b.WriteString(indent + "\t")
			fmt.Fprintf(&b, "%s,\n", formatGoLiteral(item, indent+"\t"))
		}
		b.WriteString(indent + "}")
		return b.String()
	default:
		return fmt.Sprintf("%q", fmt.Sprint(v))
	}
}
