//go:build ios && cgo

/*
 * Copyright (c) Cherri
 */

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"unsafe"

	args "github.com/electrikmilk/args-parser"
	"howett.net/plist"
)

type mobileCompileResponse struct {
	OK           bool   `json:"ok"`
	Name         string `json:"name,omitempty"`
	PlistBase64  string `json:"plistBase64,omitempty"`
	SignedBase64 string `json:"signedBase64,omitempty"`
	Source       string `json:"source,omitempty"`
	Error        string `json:"error,omitempty"`
	Line         int    `json:"line,omitempty"`
	Column       int    `json:"column,omitempty"`
}

type mobileActionParameter struct {
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

type mobileActionInfo struct {
	Name               string                  `json:"name"`
	ShortcutIdentifier string                  `json:"shortcutIdentifier,omitempty"`
	Title              string                  `json:"title,omitempty"`
	Description        string                  `json:"description,omitempty"`
	Category           string                  `json:"category,omitempty"`
	Subcategory        string                  `json:"subcategory,omitempty"`
	Parameters         []mobileActionParameter `json:"parameters,omitempty"`
	OutputType         string                  `json:"outputType,omitempty"`
	MacOnly            bool                    `json:"macOnly,omitempty"`
	NonMacOnly         bool                    `json:"nonMacOnly,omitempty"`
	MinVersion         float64                 `json:"minVersion,omitempty"`
	MaxVersion         float64                 `json:"maxVersion,omitempty"`
}

type mobileActionCatalogResponse struct {
	OK      bool               `json:"ok"`
	Actions []mobileActionInfo `json:"actions,omitempty"`
	Error   string             `json:"error,omitempty"`
}

var mobileCompileMu sync.Mutex
var mobileBaseActions map[string]*actionDefinition
var mobileBaseEnumerations map[string][]string

// CherriCompile compiles Cherri source in-process and returns a JSON response.
// The plist payload is base64 encoded so the C ABI only has to pass one string.
//
//export CherriCompile
func CherriCompile(source *C.char, requestedName *C.char) *C.char {
	return encodeMobileResponse(compileForMobile(C.GoString(source), C.GoString(requestedName), false))
}

// CherriCompileSigned compiles Cherri source and signs it with Cherri's existing
// HubSign integration. This performs a network request and should only be called
// after an explicit user action.
//
//export CherriCompileSigned
func CherriCompileSigned(source *C.char, requestedName *C.char) *C.char {
	return encodeMobileResponse(compileForMobile(C.GoString(source), C.GoString(requestedName), true))
}

// CherriDecompilePlist decompiles Shortcut plist data using Cherri's existing
// decompiler. The plist is base64 encoded only to keep the exported C ABI small
// and safe for arbitrary binary input.
//
//export CherriDecompilePlist
func CherriDecompilePlist(plistBase64 *C.char, requestedName *C.char) *C.char {
	encoded := C.GoString(plistBase64)
	plistBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return encodeMobileResponse(mobileCompileResponse{OK: false, Error: "Invalid base64 Shortcut data."})
	}
	return encodeMobileResponse(decompileForMobile(plistBytes, C.GoString(requestedName)))
}

// CherriActionCatalog returns the action definitions currently available to the
// compiler. This includes Cherri's Go built-ins and actions brought into the
// current source through its normal include/action-definition machinery.
//
//export CherriActionCatalog
func CherriActionCatalog() *C.char {
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()

	catalog := currentMobileActionCatalog()
	encoded, err := json.Marshal(mobileActionCatalogResponse{OK: true, Actions: catalog})
	if err != nil {
		encoded, _ = json.Marshal(mobileActionCatalogResponse{OK: false, Error: err.Error()})
	}
	return C.CString(string(encoded))
}

func encodeMobileResponse(response mobileCompileResponse) *C.char {
	encoded, err := json.Marshal(response)
	if err != nil {
		encoded = []byte(`{"ok":false,"error":"unable to encode compiler response"}`)
	}
	return C.CString(string(encoded))
}

// CherriFree releases strings returned by the mobile bridge.
//
//export CherriFree
func CherriFree(pointer *C.char) {
	C.free(unsafe.Pointer(pointer))
}

func compileForMobile(source string, requestedName string, sign bool) (response mobileCompileResponse) {
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()

	embeddedCompilerMode = true
	previousArgs := args.Args
	args.Args = map[string]string{
		"skip-sign": "",
		"no-ansi":   "",
	}

	defer func() {
		embeddedCompilerMode = false
		args.Args = previousArgs
		if recovered := recover(); recovered != nil {
			response = mobileCompileResponse{
				OK:     false,
				Error:  embeddedErrorMessage(recovered),
				Line:   max(lineIdx+1, 1),
				Column: max(lineCharIdx+1, 1),
			}
			resetEmbeddedFailureState()
		}
	}()

	resetMobileLanguageState()
	resetMobileDecompileState()
	name := normalizedMobileName(requestedName)

	filePath = ""
	filename = name + ".cherri"
	basename = name
	relativePath = ""
	inputPath = ""
	outputPath = ""
	workflowName = name
	contents = source

	initParse()
	generateShortcut()

	plistBytes, err := plist.Marshal(shortcut, plist.XMLFormat)
	if err != nil {
		panic(err)
	}

	response = mobileCompileResponse{
		OK:          true,
		Name:        workflowName,
		PlistBase64: base64.StdEncoding.EncodeToString(plistBytes),
	}

	if sign {
		service := hubSign()
		signedShortcut := requestSignedShortcut(&service)
		if len(signedShortcut) == 0 {
			panic(embeddedCompilerPanic{message: "Signing service returned no Shortcut data."})
		}
		if !looksLikeSignedShortcut(signedShortcut) {
			panic(embeddedCompilerPanic{message: "Signing server response does not look like a Shortcut file."})
		}
		response.SignedBase64 = base64.StdEncoding.EncodeToString(signedShortcut)
	}

	return response
}

func decompileForMobile(plistBytes []byte, requestedName string) (response mobileCompileResponse) {
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()

	embeddedCompilerMode = true
	previousArgs := args.Args
	args.Args = map[string]string{"no-ansi": ""}

	defer func() {
		embeddedCompilerMode = false
		args.Args = previousArgs
		if recovered := recover(); recovered != nil {
			response = mobileCompileResponse{
				OK:     false,
				Error:  embeddedErrorMessage(recovered),
				Line:   max(lineIdx+1, 1),
				Column: max(lineCharIdx+1, 1),
			}
			resetEmbeddedFailureState()
		}
	}()

	resetMobileLanguageState()
	resetMobileDecompileState()
	name := normalizedMobileName(requestedName)
	basename = strings.ReplaceAll(name, " ", "_")
	workflowName = name
	filename = name + ".shortcut"
	filePath = ""
	relativePath = ""
	inputPath = ""
	outputPath = ""

	if _, err := plist.Unmarshal(plistBytes, &shortcut); err != nil {
		panic(embeddedCompilerPanic{message: "Unable to read Shortcut plist: " + err.Error()})
	}

	loadBasicStandardActions()
	resetParse()
	firstChar()

	mapVariables()
	mapSplitActions()
	mapIdentifiers()
	mapControlFlowOutputs()
	defineName()
	decompileIcon()
	decompileMobileWorkflowMetadata()
	decompileActions()

	return mobileCompileResponse{
		OK:     true,
		Name:   name,
		Source: code.String(),
	}
}

// The upstream decompiler currently emits name/icon/action source but not the
// Shortcut Details workflow/quick-action switches. The iOS visual editor edits
// those exact plist fields, so preserve them as the existing Cherri definitions
// documented for the same settings.
func decompileMobileWorkflowMetadata() {
	workflowOrder := []string{"menubar", "quickactions", "sharesheet", "notifications", "sleepmode", "watch", "onscreen", "search", "spotlight"}
	quickActionOrder := []string{"finder", "services"}

	from := mobileDefinitionValues(shortcut.WFWorkflowTypes, workflowTypes, workflowOrder)
	quick := mobileDefinitionValues(shortcut.WFQuickActionSurfaces, quickActions, quickActionOrder)

	wroteDefinition := false
	if len(from) != 0 {
		newCodeLine(fmt.Sprintf("#define from %s\n", strings.Join(from, ", ")))
		wroteDefinition = true
	}
	if len(quick) != 0 {
		newCodeLine(fmt.Sprintf("#define quickactions %s\n", strings.Join(quick, ", ")))
		wroteDefinition = true
	}
	if wroteDefinition {
		newCodeLine("\n")
	}
}

func mobileDefinitionValues(selected []string, definitions map[string]string, order []string) []string {
	selectedSet := make(map[string]struct{}, len(selected))
	for _, value := range selected {
		selectedSet[value] = struct{}{}
	}

	values := make([]string, 0, len(selected))
	for _, cherriValue := range order {
		shortcutValue, found := definitions[cherriValue]
		if !found {
			continue
		}
		if _, selected := selectedSet[shortcutValue]; selected {
			values = append(values, cherriValue)
		}
	}
	return values
}

func currentMobileActionCatalog() []mobileActionInfo {
	names := make([]string, 0, len(actions))
	for name := range actions {
		names = append(names, name)
	}
	sort.Strings(names)

	catalog := make([]mobileActionInfo, 0, len(names))
	for _, name := range names {
		definition := actions[name]
		if definition == nil {
			continue
		}

		parameters := make([]mobileActionParameter, 0, len(definition.parameters))
		for _, parameter := range definition.parameters {
			item := mobileActionParameter{
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

		catalog = append(catalog, mobileActionInfo{
			Name:               name,
			ShortcutIdentifier: mobileShortcutIdentifier(name, definition),
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
		})
	}
	return catalog
}

func mobileShortcutIdentifier(name string, definition *actionDefinition) string {
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

func ensureMobileBaseLanguageState() {
	if mobileBaseActions == nil {
		mobileBaseActions = maps.Clone(actions)
	}
	if mobileBaseEnumerations == nil {
		mobileBaseEnumerations = maps.Clone(enumerations)
	}
}

// Cherri's CLI normally exits after one compilation. The iOS host compiles on
// every edit, so definitions introduced by one document must not leak into the
// next compilation and make includes/custom actions or Shortcut metadata persist.
func resetMobileLanguageState() {
	ensureMobileBaseLanguageState()
	actions = maps.Clone(mobileBaseActions)
	enumerations = maps.Clone(mobileBaseEnumerations)
	included = []string{}
	includes = []include{}
	includedBasicStandardActions = false
	includedFile = false
	functions = nil
	usingFunctions = false
	hasShortcutInputVariables = false
	inputs = []string{}
	outputs = []string{}
	definedWorkflowTypes = []string{}
	definedQuickActions = []string{}
	noInput = nil
	iconColor = 3031607807
	iconGlyph = 61440
	iosVersion = 26.4
	clientVersion = versions["26.4"]
	shortcut = Shortcut{}
}

func normalizedMobileName(requestedName string) string {
	name := strings.TrimSpace(requestedName)
	if name == "" {
		return "Shortcut"
	}
	return name
}

func embeddedErrorMessage(recovered any) string {
	switch value := recovered.(type) {
	case embeddedCompilerPanic:
		return value.Error()
	case error:
		return value.Error()
	default:
		return fmt.Sprint(value)
	}
}

func resetMobileDecompileState() {
	code.Reset()
	actionIndex = 0
	tabLevel = 0
	groupingIdx = 0
	currentVariableValue = ""
	varUUIDs = nil
	constUUIDs = nil
	identifierMap = nil
	variables = map[string]varValue{}
	questions = map[string]*question{}
	menus = map[string][]varValue{}
	uuids = map[string]string{}
	controlFlowGroups = map[int]controlFlowGroup{}
	includes = []include{}
	definitions = map[string]any{}
	contents = ""
	originalContents = ""
	chars = []rune{}
	lines = []string{}
	char = -1
	idx = -1
	lineIdx = 0
	lineCharIdx = -1
}

// A failed parse exits before initParse's normal cleanup. Restore the cursor and
// per-compilation collections so the next edit can compile in the same process.
func resetEmbeddedFailureState() {
	resetMobileDecompileState()
	tokens = []token{}
}
