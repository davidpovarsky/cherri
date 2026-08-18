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

var mobileCompileMu sync.Mutex

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
	decompileActions()

	return mobileCompileResponse{
		OK:     true,
		Name:   name,
		Source: code.String(),
	}
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
