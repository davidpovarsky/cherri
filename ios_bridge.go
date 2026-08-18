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
	OK          bool   `json:"ok"`
	Name        string `json:"name,omitempty"`
	PlistBase64 string `json:"plistBase64,omitempty"`
	Error       string `json:"error,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
}

var mobileCompileMu sync.Mutex

// CherriCompile compiles Cherri source in-process and returns a JSON response.
// The plist payload is base64 encoded so the C ABI only has to pass one string.
//
//export CherriCompile
func CherriCompile(source *C.char, requestedName *C.char) *C.char {
	response := compileForMobile(C.GoString(source), C.GoString(requestedName))
	encoded, err := json.Marshal(response)
	if err != nil {
		encoded = []byte(`{"ok":false,"error":"unable to encode compiler response"}`)
	}
	return C.CString(string(encoded))
}

// CherriFree releases strings returned by CherriCompile.
//
//export CherriFree
func CherriFree(pointer *C.char) {
	C.free(unsafe.Pointer(pointer))
}

func compileForMobile(source string, requestedName string) (response mobileCompileResponse) {
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

	name := strings.TrimSpace(requestedName)
	if name == "" {
		name = "Shortcut"
	}

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

	return mobileCompileResponse{
		OK:          true,
		Name:        workflowName,
		PlistBase64: base64.StdEncoding.EncodeToString(plistBytes),
	}
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

// A failed parse exits before initParse's normal cleanup. Restore the cursor and
// per-compilation collections so the next edit can compile in the same process.
func resetEmbeddedFailureState() {
	contents = ""
	originalContents = ""
	char = -1
	idx = -1
	lineIdx = 0
	lineCharIdx = -1
	tokens = []token{}
	chars = []rune{}
	lines = []string{}
	controlFlowGroups = map[int]controlFlowGroup{}
	groupingIdx = 0
	variables = map[string]varValue{}
	questions = map[string]*question{}
	menus = map[string][]varValue{}
	uuids = map[string]string{}
	includes = []include{}
	definitions = map[string]any{}
}
