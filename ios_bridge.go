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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync"
	"unsafe"

	args "github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
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

// The mobile bridge consumes the shared machine-readable action catalog so the
// CLI (--actions-json), tooling, and the iOS app all expose identical metadata.
// The aliases below keep the historical local names used in this file.
type mobileActionParameter = catalogParameter
type mobileActionInfo = catalogActionInfo
type mobileActionCatalogResponse = actionCatalogResponse

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

// CherriAnalyze parses and analyzes Cherri v2 source and returns JSON diagnostics.
//
//export CherriAnalyze
func CherriAnalyze(source *C.char) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	diags, fp := svc.Analyze(uri)

	wireDiags := make([]protocol.DiagnosticItem, 0)
	hasErrors := false
	for _, d := range diags {
		wireDiags = append(wireDiags, protocol.ToDiagnosticItem(d))
		if d.Severity == analysis.SeverityError {
			hasErrors = true
		}
	}

	resp := protocol.AnalyzeResponse{
		URI:               uri,
		Version:           1,
		LanguageVersion:   schema.LanguageVersion,
		SchemaFingerprint: fp,
		Diagnostics:       wireDiags,
		Valid:             !hasErrors,
	}

	encoded, _ := json.Marshal(resp)
	return C.CString(string(encoded))
}

// CherriComplete computes contextual completions at (line, column) in source.
//
//export CherriComplete
func CherriComplete(source *C.char, line C.int, column C.int) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	items := svc.Complete(uri, int(line), int(column))

	wireItems := make([]protocol.CompletionItem, 0)
	for _, it := range items {
		wireItems = append(wireItems, protocol.CompletionItem{
			Label:         it.Label,
			Kind:          int(it.Kind),
			Detail:        it.Detail,
			Documentation: it.Documentation,
			InsertText:    it.InsertText,
		})
	}

	resp := protocol.CompleteResponse{
		URI:               uri,
		Version:           1,
		LanguageVersion:   schema.LanguageVersion,
		SchemaFingerprint: svc.SchemaFingerprint(),
		Items:             wireItems,
	}

	encoded, _ := json.Marshal(resp)
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

func compileForMobile(src string, requestedName string, sign bool) (response mobileCompileResponse) {
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

	// Try Cherri v2 compiler first for modern v2 syntax
	isLegacy := strings.Contains(src, "#include") ||
		strings.Contains(src, "#define") ||
		strings.Contains(src, "#import") ||
		strings.Contains(src, "@") ||
		strings.Contains(src, "const ") ||
		strings.Contains(src, "Ask")

	if !isLegacy {
		v2Bytes, err := CompileSourceToPlist(name+".cherri", src)
		if err == nil {
			response = mobileCompileResponse{
				OK:          true,
				Name:        name,
				PlistBase64: base64.StdEncoding.EncodeToString(v2Bytes),
			}
			if sign {
				service := hubSign()
				signedShortcut, signErr := SignShortcutBytes(&service, name, v2Bytes)
				if signErr != nil {
					response.OK = false
					response.Error = fmt.Sprintf("Signing error: %v", signErr)
					return response
				}
				if len(signedShortcut) > 0 && looksLikeSignedShortcut(signedShortcut) {
					response.SignedBase64 = base64.StdEncoding.EncodeToString(signedShortcut)
				} else {
					response.OK = false
					response.Error = "Signing server response does not look like a signed Shortcut (missing AEA1 magic)"
					return response
				}
			}
			return response
		}
	}

	inputPath = ""
	outputPath = ""
	workflowName = name
	contents = src

	initParse()
	generateShortcut()

	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("\t")
	if err := enc.Encode(shortcut); err != nil {
		panic(err)
	}
	legacyBytes := buf.Bytes()

	response = mobileCompileResponse{
		OK:          true,
		Name:        workflowName,
		PlistBase64: base64.StdEncoding.EncodeToString(legacyBytes),
	}

	if sign {
		service := hubSign()
		signedShortcut, signErr := SignShortcutBytes(&service, workflowName, legacyBytes)
		if signErr != nil {
			response.OK = false
			response.Error = fmt.Sprintf("Signing error: %v", signErr)
			return response
		}
		if len(signedShortcut) > 0 && looksLikeSignedShortcut(signedShortcut) {
			response.SignedBase64 = base64.StdEncoding.EncodeToString(signedShortcut)
		} else {
			response.OK = false
			response.Error = "Signing server response does not look like a signed Shortcut (missing AEA1 magic)"
			return response
		}
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
		if jsonErr := json.Unmarshal(plistBytes, &shortcut); jsonErr != nil {
			panic(embeddedCompilerPanic{message: "Unable to read Shortcut plist: " + err.Error()})
		}
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
	decompileWorkflowMetadata()
	decompileActions()

	return mobileCompileResponse{
		OK:     true,
		Name:   name,
		Source: code.String(),
	}
}

func currentMobileActionCatalog() []mobileActionInfo {
	reg := schema.DefaultRegistry()
	var catalog []mobileActionInfo
	for _, act := range reg.AllActions() {
		var params []catalogParameter
		for _, p := range act.Parameters {
			params = append(params, catalogParameter{
				Name:       p.Label,
				Key:        p.WireKey,
				Type:       p.TypeName,
				Optional:   p.Optional,
				Enum:       p.EnumName,
				EnumValues: p.EnumValues,
				Default:    p.DefaultValue,
			})
		}
		var intent *catalogAppIntent
		if act.AppIntent != nil {
			intent = &catalogAppIntent{
				Name:                act.AppIntent.Name,
				BundleIdentifier:    act.AppIntent.BundleIdentifier,
				AppIntentIdentifier: act.AppIntent.AppIntentIdentifier,
				TeamIdentifier:      act.AppIntent.TeamIdentifier,
			}
		}
		catalog = append(catalog, catalogActionInfo{
			Name:               act.CallableName,
			ShortcutIdentifier: act.AppleIdentifier,
			Title:              act.Docs.Title,
			Description:        act.Docs.Description,
			Category:           act.Docs.Category,
			Subcategory:        act.Docs.Subcategory,
			Parameters:         params,
			OutputType:         act.OutputTypeName,
			CompilerConstruct:  act.CompilerConstruct,
			InsertionSnippet:   act.Docs.InsertionSnippet,
			AppIntent:          intent,
		})
	}
	return catalog
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
	iosVersion = 27.0
	clientVersion = versions["27"]
	automationTriggers = nil
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
	decompiledIncludes = nil
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
	importQuestionByActionParam = nil
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
