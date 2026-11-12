/*
 * Copyright (c) Cherri
 */

package main

import (
	"crypto/sha1"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/electrikmilk/args-parser"
	"github.com/google/uuid"
)

var variables map[string]varValue

type varValue struct {
	variableType string
	valueType    tokenType
	value        any
	getAs        string
	coerce       string
	coerceFirst  bool
	constant     bool
	repeatItem   bool
	prompt       string
	descriptor   *ReferenceDescriptor
}

var globals = map[string]varValue{
	ShortcutInput: {
		variableType: "ExtensionInput",
		valueType:    String,
		value:        ShortcutInput,
	},
	"CurrentDate": {
		variableType: "CurrentDate",
		valueType:    Date,
		value:        "CurrentDate",
	},
	"Clipboard": {
		variableType: "Clipboard",
		valueType:    String,
		value:        "Clipboard",
	},
	"Device": {
		variableType: "DeviceDetails",
		valueType:    String,
		value:        "DeviceDetails",
	},
	"Ask": {
		variableType: "Ask",
		valueType:    String,
		value:        "Ask",
	},
	"RepeatItem": {
		variableType: "Variable",
		valueType:    String,
		value:        "Repeat Item",
	},
	"RepeatIndex": {
		variableType: "Variable",
		valueType:    Integer,
		value:        "Repeat Index",
	},
}

func init() {
	for i := 2; i <= 10; i++ {
		globals[fmt.Sprintf("RepeatItem%d", i)] = varValue{
			variableType: "Variable",
			valueType:    String,
			value:        fmt.Sprintf("Repeat Item %d", i),
		}
		globals[fmt.Sprintf("RepeatIndex%d", i)] = varValue{
			variableType: "Variable",
			valueType:    Integer,
			value:        fmt.Sprintf("Repeat Index %d", i),
		}
	}
}

func availableIdentifier(identifier *string) {
	if v, found := variables[*identifier]; found {
		if v.constant {
			parserError(fmt.Sprintf("Cannot redefine constant '%s'.", *identifier))
		}
		if v.repeatItem {
			v.repeatItem = false
			variables[*identifier] = v
			return
		}
	}
	if _, found := globals[*identifier]; found {
		parserError(fmt.Sprintf("Cannot redefine global variable '%s'.", *identifier))
	}
	if _, found := questions[*identifier]; found {
		parserError(fmt.Sprintf("Reference conflicts with defined import question '%s'.", *identifier))
	}
}

func createUUIDReference(identifier string) string {
	if identifier == "" {
		return createUUID(&identifier)
	}
	if existing, ok := uuids[identifier]; ok && existing != "" {
		return existing
	}
	var actionUUID = createUUID(&identifier)
	uuids[identifier] = actionUUID

	return actionUUID
}

func wrapVariableReference(s *string) {
	if validReference(*s) {
		*s = fmt.Sprintf("{%s}", *s)
	}
}

func validReference(identifier string) bool {
	if _, found := globals[identifier]; found {
		isInputVariable(identifier)
		return true
	}
	if _, found := variables[identifier]; found {
		return true
	}

	return false
}

func validGlobalReference(identifier *string) bool {
	if _, found := globals[*identifier]; found {
		isInputVariable(*identifier)
		return true
	}

	return false
}

// Checks if identifier is a variable that is not a constant.
func validActionReference(identifier *string) bool {
	if v, found := variables[*identifier]; found {
		if !v.constant {
			parserError(fmt.Sprintf("Unknown reference '%s'. Variable references must be prepended with @.", *identifier))
		}
		return true
	}

	return false
}

// Checks if identifier is a variable that is not a constant.
func validVariableReference(identifier *string) bool {
	if v, found := variables[*identifier]; !found || v.constant {
		return false
	}
	return true
}

func getVariableValue(identifier string) (variableValue *varValue, found bool) {
	if value, found := globals[identifier]; found {
		isInputVariable(identifier)
		return &value, true
	}
	if value, found := variables[identifier]; found {
		return &value, true
	}

	return nil, false
}

func makeVariableReferenceString(value varValue) string {
	var identifier strings.Builder
	identifier.WriteString(value.value.(string))

	if value.getAs != "" {
		identifier.WriteString(fmt.Sprintf("['%s']", value.getAs))
	}
	if value.coerce != "" {
		identifier.WriteString(fmt.Sprintf(".%s", value.coerce))
	}

	return identifier.String()
}

var currentOutputName string
var duplicateDelta int

var namedVars = make(map[string]bool)

func isUsedOutputName(candidate string) bool {
	for name := range namedVars {
		if strings.EqualFold(name, candidate) {
			return true
		}
	}
	for k, v := range uuids {
		if strings.EqualFold(k, candidate) || strings.EqualFold(v, candidate) {
			return true
		}
	}
	for gName := range globals {
		if strings.EqualFold(gName, candidate) {
			return true
		}
	}
	return false
}

func checkDuplicateOutputName(name string) string {
	candidate := name
	counter := 1
	for isUsedOutputName(candidate) {
		if args.Using("import") {
			candidate = fmt.Sprintf("%s%d", name, counter)
		} else {
			candidate = fmt.Sprintf("%s %d", name, counter)
		}
		counter++
	}
	return candidate
}

func duplicateOutputName() string {
	duplicateDelta++

	var revChars = []rune(currentOutputName)
	slices.Reverse(revChars)
	var numChars []rune
	for _, rc := range revChars {
		if rc >= '0' && rc <= '9' {
			numChars = append(numChars, rc)
		} else {
			break
		}
	}

	if len(numChars) != 0 {
		slices.Reverse(numChars)
		var num = string(numChars)
		var endingDelta, numErr = strconv.Atoi(num)
		handle(numErr)
		endingDelta++

		currentOutputName, _ = strings.CutSuffix(currentOutputName, num)
		duplicateDelta = endingDelta
	}

	if !args.Using("import") && !strings.Contains(currentOutputName, " ") {
		currentOutputName = fmt.Sprintf("%s ", currentOutputName)
	}

	currentOutputName = fmt.Sprintf("%s%d", currentOutputName, duplicateDelta)

	return currentOutputName
}

func createUUID(salt *string) string {
	var deterministic = args.Using("derive-uuids")
	if deterministic {
		var seeds = [][]byte{
			[]byte(workflowName),
			[]byte(*salt),
		}
		return deterministicUUID(seeds...)
	}

	return uuid.New().String()
}

// deterministicUUID returns a UUID derived from the given pieces.
func deterministicUUID(pieces ...[]byte) string {
	var hash = sha1.New()
	for i, piece := range pieces {
		if i > 0 {
			hash.Write([]byte{0x00})
		}
		hash.Write(piece)
	}
	var seed = hash.Sum(nil)

	return uuid.NewSHA1(uuid.NameSpaceURL, seed).String()
}
