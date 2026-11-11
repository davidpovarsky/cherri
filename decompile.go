/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/electrikmilk/args-parser"
	"howett.net/plist"
)

const (
	UUID          = "UUID"
	ShortcutInput = "ShortcutInput"
	Ask           = "Ask"
)

var code strings.Builder
var specialCharsRegex *regexp.Regexp
var decompilingRawTextToken = false
var actionByUUID map[string]*ShortcutAction
var actionIndexByUUID map[string]int
var controlFlowGroupByUUID map[string]string

// decompiledIncludes records the standard action include categories
// ('actions/<cat>') already emitted into the generated source during the
// current decompilation, so each required include appears exactly once.
var decompiledIncludes []string

func getUint64(m map[string]any, key string) uint64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case uint64:
		return n
	case int:
		if n >= 0 {
			return uint64(n)
		}
		return 0
	case int64:
		if n >= 0 {
			return uint64(n)
		}
		return 0
	case float64:
		if n >= 0 && !math.IsNaN(n) && !math.IsInf(n, 0) {
			return uint64(n)
		}
		return 0
	case json.Number:
		if u, err := strconv.ParseUint(n.String(), 10, 64); err == nil {
			return u
		}
	case string:
		if u, err := strconv.ParseUint(n, 10, 64); err == nil {
			return u
		}
	}
	return 0
}

func getInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		if i, err := strconv.Atoi(n.String()); err == nil {
			return i
		}
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return 0
}

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func getMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if res, ok := v.(map[string]any); ok {
		return res
	}
	if res, ok := v.(map[string]interface{}); ok {
		return res
	}
	return nil
}

// decodeShortcutDocument decodes raw shortcut bytes from JSON, XML plist, or binary plist
// in a unified, type-safe manner that prevents integer/float type mismatches.
func decodeShortcutDocument(b []byte) (string, error) {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}))
	if len(trimmed) == 0 {
		return "", fmt.Errorf("empty shortcut data")
	}

	if hasSignedBytes(trimmed) {
		return "signed", fmt.Errorf("import: Signed Shortcuts are currently not supported :(\nYou can use an iCloud link instead by sharing the Shortcut and selecting \"Copy iCloud Link\".")
	}

	// 1. JSON detection: starts with '{' or '['
	if trimmed[0] == '{' || trimmed[0] == '[' {
		d := json.NewDecoder(bytes.NewReader(trimmed))
		d.UseNumber()
		var root any
		if err := d.Decode(&root); err != nil {
			return "json", fmt.Errorf("failed to parse JSON shortcut: %w", err)
		}
		normalized := normalizePlistNumbers(root)
		plistBytes, err := plist.Marshal(normalized, plist.XMLFormat)
		if err != nil {
			var binErr error
			plistBytes, binErr = plist.Marshal(normalized, plist.BinaryFormat)
			if binErr != nil {
				return "json", fmt.Errorf("failed to encode normalized JSON shortcut to plist: %w", err)
			}
		}
		if _, unmarshalErr := plist.Unmarshal(plistBytes, &shortcut); unmarshalErr != nil {
			return "json", fmt.Errorf("failed to unmarshal JSON shortcut into shortcut model: %w", unmarshalErr)
		}
		return "json", nil
	}

	// 2. Plist detection (binary or XML)
	var _, plistErr = plist.Unmarshal(trimmed, &shortcut)
	if plistErr == nil {
		if bytes.HasPrefix(trimmed, []byte("bplist")) {
			return "binary_plist", nil
		}
		return "xml_plist", nil
	}

	// 3. Fallback: try json decoding if plist unmarshal failed
	d := json.NewDecoder(bytes.NewReader(trimmed))
	d.UseNumber()
	var root any
	if err := d.Decode(&root); err == nil {
		normalized := normalizePlistNumbers(root)
		if plistBytes, mErr := plist.Marshal(normalized, plist.XMLFormat); mErr == nil {
			if _, uErr := plist.Unmarshal(plistBytes, &shortcut); uErr == nil {
				return "json", nil
			}
		}
	}

	return "unknown", fmt.Errorf("unrecognized or invalid shortcut file format: %w", plistErr)
}

func decompile(b []byte) {
	if isLosslessImportFidelity() {
		doc, err := decodeShortcutRawMap(b)
		handle(err)
		src, err := renderLosslessLegacySource(doc)
		handle(err)
		outputPath = getOutputPath(basename + ".cherri")
		if writeErr := os.WriteFile(outputPath, []byte(src), 0600); writeErr != nil {
			handle(writeErr)
		}
		return
	}

	_, err := decodeShortcutDocument(b)
	if err != nil {
		handle(err)
	}

	variables = make(map[string]varValue)
	uuids = make(map[string]string)
	actionByUUID = make(map[string]*ShortcutAction)
	controlFlowGroupByUUID = make(map[string]string)
	controlFlowGroups = make(map[int]controlFlowGroup)
	varUUIDs = []string{}
	constUUIDs = []string{}
	pendingFoldUUID = ""
	currentVariableValue = ""

	basename = strings.ReplaceAll(basename, " ", "_")
	outputPath = getOutputPath(basename + ".cherri")
	if args.Using("no-ansi") {
		filePath = basename + ".cherri"
	} else {
		filePath = outputPath
	}

	loadBasicStandardActions()
	loadAllStandardActionsForDecompile()
	resetParse()
	firstChar()

	initNamedVars()
	mapVariables()
	mapSplitActions()
	mapIdentifiers()
	mapControlFlowOutputs()

	defineName()
	decompileIcon()
	decompileWorkflowMetadata()

	decompileActions()

	if args.Using("debug") {
		printDecompDebug()
	}

	var writeErr = os.WriteFile(outputPath, []byte(code.String()), 0600)
	handle(writeErr)
}

func countUUIDsInParams(val any, counts map[string]int) {
	if val == nil {
		return
	}
	switch v := val.(type) {
	case map[string]any:
		if u, ok := v["OutputUUID"].(string); ok && u != "" {
			counts[u]++
		}
		for _, item := range v {
			countUUIDsInParams(item, counts)
		}
	case []any:
		for _, item := range v {
			countUUIDsInParams(item, counts)
		}
	}
}

var emittedConstants = make(map[string]bool)
var activeControlFlowGroups []string

func initNamedVars() {
	namedVars = make(map[string]bool)
	emittedConstants = make(map[string]bool)
	activeControlFlowGroups = nil
	for i := range shortcut.WFWorkflowActions {
		if params := shortcut.WFWorkflowActions[i].WFWorkflowActionParameters; params != nil {
			if vName := getString(params, "WFVariableName"); vName != "" {
				sName := vName
				sanitizeIdentifier(&sName)
				namedVars[sName] = true
			}
		}
	}
	referenced := make(map[string]bool)
	for i := range shortcut.WFWorkflowActions {
		collectReferencedVariableNames(shortcut.WFWorkflowActions[i].WFWorkflowActionParameters, referenced)
	}
	for rawVar := range referenced {
		sVar := rawVar
		sanitizeIdentifier(&sVar)
		namedVars[sVar] = true
	}
}

func collectReferencedVariableNames(val any, referenced map[string]bool) {
	if val == nil {
		return
	}
	switch v := val.(type) {
	case map[string]any:
		if vName, ok := v["VariableName"].(string); ok && vName != "" {
			if t, ok := v["Type"].(string); !ok || t == "Variable" {
				referenced[vName] = true
			}
		}
		for _, item := range v {
			collectReferencedVariableNames(item, referenced)
		}
	case []any:
		for _, item := range v {
			collectReferencedVariableNames(item, referenced)
		}
	}
}

// mapIdentifiers creates a map of variable identifiers and UUIDs that are assigned throughout the Shortcut.
func mapIdentifiers() {
	for i := range shortcut.WFWorkflowActions {
		if params := shortcut.WFWorkflowActions[i].WFWorkflowActionParameters; params != nil {
			if vName := getString(params, "WFVariableName"); vName != "" {
				sName := vName
				sanitizeIdentifier(&sName)
				namedVars[sName] = true
			}
		}
	}

	actionIndexByUUID = make(map[string]int)
	if actionByUUID == nil {
		actionByUUID = make(map[string]*ShortcutAction)
	}
	if controlFlowGroupByUUID == nil {
		controlFlowGroupByUUID = make(map[string]string)
	}
	if emittedConstants == nil {
		emittedConstants = make(map[string]bool)
	}
	for i := range shortcut.WFWorkflowActions {
		action := &shortcut.WFWorkflowActions[i]
		params := action.WFWorkflowActionParameters
		if params == nil {
			continue
		}
		if uuidStr := getString(params, "UUID"); uuidStr != "" {
			actionByUUID[uuidStr] = action
			actionIndexByUUID[uuidStr] = i
			if outStr := getString(params, "CustomOutputName"); outStr != "" {
				mapUUID(uuidStr, outStr)
			}
		}
	}

	setVarRefs := make(map[string]int)
	setVarFirstIdx := make(map[string]int)
	setVarHasAggr := make(map[string]bool)
	setVarFallbackName := make(map[string]string)
	otherRefs := make(map[string]int)

	for i := range shortcut.WFWorkflowActions {
		action := &shortcut.WFWorkflowActions[i]
		params := action.WFWorkflowActionParameters
		if params == nil {
			continue
		}
		if action.WFWorkflowActionIdentifier == SetVariableIdentifier {
			if inputMap := getMap(params, "WFInput"); inputMap != nil {
				var wfInput WFInput
				mapToStruct(inputMap, &wfInput)
				if u := wfInput.Value.OutputUUID; u != "" {
					setVarRefs[u]++
					if setVarRefs[u] == 1 {
						setVarFirstIdx[u] = i
						setVarHasAggr[u] = len(wfInput.Value.Aggrandizements) > 0
						outName := wfInput.Value.OutputName
						if outName == "" {
							outName = getString(params, "WFVariableName")
						}
						if outName != "" {
							setVarFallbackName[u] = outName
						}
					}
				}
			}
			continue
		}
		countUUIDsInParams(params, otherRefs)
	}

	isControlFlow := func(id string) bool {
		return id == "is.workflow.actions.conditional" ||
			id == "is.workflow.actions.choosefrommenu" ||
			id == "is.workflow.actions.repeat.count" ||
			id == "is.workflow.actions.repeat.each"
	}

	for i := range shortcut.WFWorkflowActions {
		action := &shortcut.WFWorkflowActions[i]
		params := action.WFWorkflowActionParameters
		if params == nil {
			continue
		}
		u := getString(params, "UUID")
		if u == "" {
			continue
		}
		if setVarRefs[u]+otherRefs[u] == 0 {
			continue
		}
		foldable := setVarRefs[u] == 1 &&
			otherRefs[u] == 0 &&
			!setVarHasAggr[u] &&
			setVarFirstIdx[u] == i+1 &&
			!isControlFlow(action.WFWorkflowActionIdentifier) &&
			action.WFWorkflowActionIdentifier != AppendVariableIdentifier

		if foldable {
			varUUIDs = append(varUUIDs, u)
		} else {
			constUUIDs = append(constUUIDs, u)
		}
		if _, found := uuids[u]; !found {
			outName := getString(params, "CustomOutputName")
			if outName == "" {
				outName = setVarFallbackName[u]
			}
			if outName == "" {
				outName = defaultActionOutputName(action.WFWorkflowActionIdentifier)
			}
			mapUUID(u, outName)
		}
	}

	for _, action := range shortcut.WFWorkflowActions {
		currentAction.identifier = action.WFWorkflowActionIdentifier
		var params = action.WFWorkflowActionParameters
		if params == nil || action.WFWorkflowActionIdentifier == SetVariableIdentifier {
			continue
		}

		checkParamIdentifiers(params)
	}
}

func mapControlFlowOutputs() {
	controlFlowGroupByUUID = make(map[string]string)
	for _, action := range shortcut.WFWorkflowActions {
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.conditional",
			"is.workflow.actions.choosefrommenu",
			"is.workflow.actions.repeat.count",
			"is.workflow.actions.repeat.each":
			var params = action.WFWorkflowActionParameters
			if params == nil {
				continue
			}
			if getUint64(params, "WFControlFlowMode") != endStatement {
				continue
			}

			groupingID := getString(params, "GroupingIdentifier")
			var groupName string
			if params["UUID"] != nil {
				var endingUUID = getString(params, "UUID")
				if endingUUID != "" && uuids[endingUUID] != "" {
					groupName = uuids[endingUUID]
				}
			}

			if groupingID != "" && groupName != "" {
				controlFlowGroupByUUID[groupingID] = groupName
			}

			controlFlowGroups[groupingIdx] = controlFlowGroup{
				groupType:  Conditional,
				identifier: groupName,
			}
			groupingIdx++
		}
	}
	groupingIdx = 0
}

// Map out variables in the Shortcut so Pass 2 / decompilation recognizes assigned variable names.
func mapVariables() {
	for _, action := range shortcut.WFWorkflowActions {
		currentAction.identifier = action.WFWorkflowActionIdentifier
		var params = action.WFWorkflowActionParameters
		if params == nil {
			continue
		}
		if action.WFWorkflowActionIdentifier == SetVariableIdentifier || action.WFWorkflowActionIdentifier == AppendVariableIdentifier {
			var varName = getString(params, "WFVariableName")
			if varName != "" {
				sanitizeIdentifier(&varName)
				if _, found := variables[varName]; !found {
					variables[varName] = varValue{}
				}
			}
		}
	}
}

func peekActions(peek int) ShortcutAction {
	return shortcut.WFWorkflowActions[actionIndex+peek]
}

// insertCodeComment creates a comment in the decompiled code.
func insertCodeComment(comment string) {
	newCodeLine(fmt.Sprintf("// %s\n", comment))
}

func checkParamIdentifiers(params map[string]interface{}) {
	for _, value := range params {
		if value == nil {
			continue
		}

		if sliceVal, ok := value.([]any); ok {
			for _, item := range sliceVal {
				if itemMap, isMap := item.(map[string]any); isMap {
					checkParamIdentifiers(itemMap)
				} else if itemMap, isMap := item.(map[string]interface{}); isMap {
					checkParamIdentifiers(itemMap)
				}
			}
			continue
		}

		var paramValues map[string]interface{}
		if m, ok := value.(map[string]interface{}); ok {
			paramValues = m
		} else if m, ok := value.(map[string]any); ok {
			paramValues = m
		} else {
			continue
		}
		checkParamValueAttachments(paramValues)

		if paramValues["Variable"] != nil {
			if paramVariable, ok := paramValues["Variable"].(map[string]interface{}); ok && paramVariable != nil {
				checkParamValueAttachments(paramVariable)
			} else if paramVariable, ok := paramValues["Variable"].(map[string]any); ok && paramVariable != nil {
				checkParamValueAttachments(paramVariable)
			}
		}

		if paramValues["WFConditions"] != nil {
			var wfConditions map[string]interface{}
			if m, ok := paramValues["WFConditions"].(map[string]interface{}); ok {
				wfConditions = m
			} else if m, ok := paramValues["WFConditions"].(map[string]any); ok {
				wfConditions = m
			}
			if wfConditions != nil {
				var valMap map[string]interface{}
				if m, ok := wfConditions["Value"].(map[string]interface{}); ok {
					valMap = m
				} else if m, ok := wfConditions["Value"].(map[string]any); ok {
					valMap = m
				}
				if valMap != nil && valMap["WFActionParameterFilterTemplates"] != nil {
					if templates, ok := valMap["WFActionParameterFilterTemplates"].([]any); ok {
						for _, filtertemplate := range templates {
							if paramFilterTemplate, ok := filtertemplate.(map[string]any); ok && paramFilterTemplate != nil {
								if wfInput, ok := paramFilterTemplate["WFInput"].(map[string]any); ok && wfInput != nil {
									checkParamIdentifiers(wfInput)
								}
							}
						}
					}
				}
			}
		}
	}
}

// checkParamValueAttachments checks for attachments on values to map them.
func checkParamValueAttachments(param map[string]interface{}) {
	if param == nil || param["Value"] == nil {
		return
	}
	var paramValue map[string]interface{}
	if m, ok := param["Value"].(map[string]interface{}); ok && m != nil {
		paramValue = m
	} else if m, ok := param["Value"].(map[string]any); ok && m != nil {
		paramValue = m
	} else {
		return
	}

	var inputValue Value
	mapToStruct(paramValue, &inputValue)
	mapValueReference(inputValue)
	checkConstantUUID(inputValue)

	if inputValue.AttachmentsByRange != nil {
		mapAttachmentIdentifiers(inputValue.AttachmentsByRange)
	}
	if inputValue.WFDictionaryFieldValueItems != nil {
		mapDictionaryValueIdentifiers(inputValue.WFDictionaryFieldValueItems)
	}
}

func mapDictionaryValueIdentifiers(items []WFDictionaryFieldValueItem) {
	for _, item := range items {
		if item.WFValue == nil {
			continue
		}
		var wfValue map[string]interface{}
		if m, ok := item.WFValue.(map[string]interface{}); ok && m != nil {
			wfValue = m
		} else if m, ok := item.WFValue.(map[string]any); ok && m != nil {
			wfValue = m
		} else {
			continue
		}
		if wfValue["Value"] == nil {
			continue
		}

		checkParamValueAttachments(wfValue)

		if vMap, ok := wfValue["Value"].(map[string]interface{}); ok && vMap != nil {
			if dictionaryItems, ok := serializedDictionaryItems(wfValue); ok {
				mapDictionaryValueIdentifiers(dictionaryItems)
			} else if dictionaryItems, ok2 := serializedDictionaryItems(vMap); ok2 {
				mapDictionaryValueIdentifiers(dictionaryItems)
			}
		} else if vMap, ok := wfValue["Value"].(map[string]any); ok && vMap != nil {
			if dictionaryItems, ok := serializedDictionaryItems(wfValue); ok {
				mapDictionaryValueIdentifiers(dictionaryItems)
			} else if dictionaryItems, ok2 := serializedDictionaryItems(vMap); ok2 {
				mapDictionaryValueIdentifiers(dictionaryItems)
			}
		} else if vSlice, ok := wfValue["Value"].([]interface{}); ok && vSlice != nil {
			var dictionaryItems = mapDictionaryItems(vSlice)
			mapDictionaryValueIdentifiers(dictionaryItems)
		} else if vSlice, ok := wfValue["Value"].([]any); ok && vSlice != nil {
			var dictionaryItems = mapDictionaryItems(vSlice)
			mapDictionaryValueIdentifiers(dictionaryItems)
		}
	}
}

func mapDictionaryItems(items any) (dictionaryItems []WFDictionaryFieldValueItem) {
	if slice, ok := items.([]any); ok {
		for _, value := range slice {
			if value == nil || reflect.TypeOf(value).Kind() != reflect.Map {
				continue
			}
			var dictionaryItem WFDictionaryFieldValueItem
			mapToStruct(value, &dictionaryItem)
			dictionaryItems = append(dictionaryItems, dictionaryItem)
		}
	}
	return
}

// mapAttachmentIdentifiers maps the UUID and output name of an attachment value.
func mapAttachmentIdentifiers(attachments map[string]Value) {
	for _, attachmentValue := range attachments {
		mapValueReference(attachmentValue)
		checkConstantUUID(attachmentValue)
	}
}

func checkConstantUUID(value Value) {
	if value.OutputUUID != "" && value.Type == "ActionOutput" && actionByUUID[value.OutputUUID] != nil {
		if !slices.Contains(constUUIDs, value.OutputUUID) {
			constUUIDs = append(constUUIDs, value.OutputUUID)
		}
	}
}

func defaultActionOutputName(actionID string) string {
	parts := strings.Split(actionID, ".")
	last := parts[len(parts)-1]
	if last == "" || last == "actions" {
		return "actionOutput"
	}
	return last
}

func mapValueReference(value Value) {
	if value.OutputUUID == "" || actionByUUID[value.OutputUUID] == nil {
		return
	}
	name := value.OutputName
	if name == "" {
		if act, ok := actionByUUID[value.OutputUUID]; ok && act != nil {
			name = defaultActionOutputName(act.WFWorkflowActionIdentifier)
		}
		if name == "" {
			name = "actionOutput"
		}
	}
	mapUUID(value.OutputUUID, name)
}

func mapUUID(uuid string, varName string) {
	var outputName string
	if _, found := uuids[uuid]; !found {
		outputName = varName
		sanitizeIdentifier(&outputName)
		uuids[uuid] = checkDuplicateOutputName(outputName)
	}
}

var cherriKeywords = map[string]bool{
	"if": true, "else": true, "menu": true, "item": true, "repeat": true, "for": true,
	"macro": true, "action": true, "const": true, "let": true,
	"true": true, "false": true, "nil": true, "return": true, "Ask": true, "ShortcutInput": true,
}

func fnv1a(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

// sanitizeIdentifier strips special characters and replaces dashes with underscores.
// Guarantees non-empty, keyword-safe identifier names.
func sanitizeIdentifier(identifier *string) {
	if identifier == nil {
		return
	}
	if strings.TrimSpace(*identifier) == "" {
		*identifier = ""
		return
	}
	orig := *identifier
	if specialCharsRegex == nil {
		specialCharsRegex = regexp.MustCompile("[^a-zA-Z0-9_]+")
	}
	if strings.Contains(*identifier, " ") {
		var words = strings.Split(strings.TrimSpace(*identifier), " ")
		for i, word := range words {
			words[i] = capitalize(word)
		}
		*identifier = strings.Join(words, "")
	}

	*identifier = specialCharsRegex.ReplaceAllString(*identifier, "")
	*identifier = strings.ReplaceAll(*identifier, "-", "_")

	// If empty after stripping special characters (e.g. Hebrew or non-ASCII name)
	if *identifier == "" {
		if orig != "" {
			h := fnv1a(orig)
			*identifier = fmt.Sprintf("v_%x", h)
		} else {
			*identifier = "v_output"
		}
	} else if (*identifier)[0] >= '0' && (*identifier)[0] <= '9' {
		// Cannot start with a digit
		*identifier = "v_" + *identifier
	} else if cherriKeywords[*identifier] {
		*identifier = *identifier + "_val"
	}
}

type actionValue struct {
	identifier string
	definition *actionDefinition
}

var identifierMap map[string][]actionValue

// mapSplitActions creates a map of actions that have been split into a few actions to reduce the number of arguments.
func mapSplitActions() {
	identifierMap = make(map[string][]actionValue)
	for identifier, action := range actions {
		var ident = action.identifier
		if action.identifier == "" {
			ident = identifier
		}
		if action.overrideIdentifier != "" {
			ident = action.overrideIdentifier
		}

		ident = strings.ToLower(ident)

		var splitActionValue = actionValue{
			identifier: identifier,
			definition: action,
		}
		if identifierMap[ident] != nil && slices.Contains(identifierMap[ident], splitActionValue) {
			continue
		}

		identifierMap[ident] = append(identifierMap[ident], splitActionValue)
	}
	for identifier, actions := range identifierMap {
		if len(actions) < 2 {
			delete(identifierMap, identifier)
			continue
		}
		sort.Slice(actions, func(i, j int) bool {
			return actions[i].identifier < actions[j].identifier
		})
		identifierMap[identifier] = actions
	}
}

// newCodeLine writes s on a new line in the code.
func newCodeLine(s string) {
	if tabLevel > 0 {
		for i := 0; i < tabLevel; i++ {
			code.WriteRune('\t')
		}
	}
	code.WriteString(s)
}

// defineName writes the name of the imported Shortcut to code as a name definition if the basename contains a space.
func defineName() {
	if strings.Contains(basename, " ") {
		newCodeLine(fmt.Sprintf("#define name %s\n", basename))
	}
}

func decompileIcon() {
	var icon = shortcut.WFWorkflowIcon
	var hasIcon = icon.WFWorkflowIconStartColor != iconColor || icon.WFWorkflowIconGlyphNumber != iconGlyph

	if icon.WFWorkflowIconStartColor != iconColor {
		defineColors(icon, colors)
		defineColors(icon, altColors)
	}

	if icon.WFWorkflowIconGlyphNumber != iconGlyph {
		for name, i := range glyphs {
			if icon.WFWorkflowIconGlyphNumber != int64(i) {
				continue
			}

			newCodeLine(fmt.Sprintf("#define glyph %s\n", name))
		}
	}

	if hasIcon {
		newCodeLine("\n")
	}
}

func defineColors(icon ShortcutIcon, colors map[string]int) {
	for name, i := range colors {
		if icon.WFWorkflowIconStartColor != i {
			continue
		}

		newCodeLine(fmt.Sprintf("#define color %s\n", name))
	}
}

var importQuestionByActionParam map[int]map[string]string

func decompileWorkflowMetadata() {
	importQuestionByActionParam = make(map[int]map[string]string)

	workflowOrder := []string{"menubar", "quickactions", "sharesheet", "notifications", "sleepmode", "watch", "onscreen", "search", "spotlight"}
	quickActionOrder := []string{"finder", "services"}

	from := metadataDefinitionValues(shortcut.WFWorkflowTypes, workflowTypes, workflowOrder)
	quick := metadataDefinitionValues(shortcut.WFQuickActionSurfaces, quickActions, quickActionOrder)

	wroteDefinition := false

	if len(from) != 0 {
		newCodeLine(fmt.Sprintf("#define from %s\n", strings.Join(from, ", ")))
		wroteDefinition = true
	}
	if len(quick) != 0 {
		newCodeLine(fmt.Sprintf("#define quickactions %s\n", strings.Join(quick, ", ")))
		wroteDefinition = true
	}

	revItems := reversedContentItems()
	if (shortcut.WFWorkflowHasShortcutInputVariables || slices.Contains(shortcut.WFWorkflowTypes, "ActionExtension") || slices.Contains(shortcut.WFWorkflowTypes, "QuickActions")) && shortcut.WFWorkflowInputContentItemClasses != nil {
		if len(shortcut.WFWorkflowInputContentItemClasses) == 0 {
			newCodeLine("#define inputs none\n")
			wroteDefinition = true
		} else if len(shortcut.WFWorkflowInputContentItemClasses) < 20 {
			var inputTypes []string
			for _, cls := range shortcut.WFWorkflowInputContentItemClasses {
				if cherriType, found := revItems[cls]; found {
					inputTypes = append(inputTypes, cherriType)
				}
			}
			if len(inputTypes) > 0 {
				sort.Strings(inputTypes)
				newCodeLine(fmt.Sprintf("#define inputs %s\n", strings.Join(inputTypes, ", ")))
				wroteDefinition = true
			}
		}
	}

	if len(shortcut.WFWorkflowOutputContentItemClasses) > 0 {
		var outputTypes []string
		for _, cls := range shortcut.WFWorkflowOutputContentItemClasses {
			if cherriType, found := revItems[cls]; found {
				outputTypes = append(outputTypes, cherriType)
			}
		}
		if len(outputTypes) > 0 {
			sort.Strings(outputTypes)
			newCodeLine(fmt.Sprintf("#define outputs %s\n", strings.Join(outputTypes, ", ")))
			wroteDefinition = true
		}
	}

	if shortcut.WFWorkflowNoInputBehavior != nil {
		behaviorName := shortcut.WFWorkflowNoInputBehavior["Name"]
		params, _ := shortcut.WFWorkflowNoInputBehavior["Parameters"].(map[string]any)
		switch behaviorName {
		case "WFWorkflowNoInputBehaviorShowError":
			errMsg, _ := params["Error"].(string)
			newCodeLine(fmt.Sprintf("#define noinput stopwith \"%s\"\n", escapeString(errMsg)))
			wroteDefinition = true
		case "WFWorkflowNoInputBehaviorAskForInput":
			if itemClass, ok := params["ItemClass"].(string); ok {
				if cherriType, found := revItems[itemClass]; found {
					newCodeLine(fmt.Sprintf("#define noinput askfor %s\n", cherriType))
					wroteDefinition = true
				}
			}
		case "WFWorkflowNoInputBehaviorGetClipboard":
			newCodeLine("#define noinput getclipboard\n")
			wroteDefinition = true
		}
	}

	if len(shortcut.WFWorkflowImportQuestions) > 0 {
		for i, q := range shortcut.WFWorkflowImportQuestions {
			qName := fmt.Sprintf("question%d", i+1)
			if importQuestionByActionParam[q.ActionIndex] == nil {
				importQuestionByActionParam[q.ActionIndex] = make(map[string]string)
			}
			importQuestionByActionParam[q.ActionIndex][q.ParameterKey] = qName

			defaultValStr := ""
			if q.DefaultValue != nil {
				switch v := q.DefaultValue.(type) {
				case string:
					defaultValStr = escapeString(v)
				default:
					defaultValStr = fmt.Sprintf("%v", v)
				}
			}
			newCodeLine(fmt.Sprintf("#question %s \"%s\" \"%s\"\n", qName, escapeString(q.Text), defaultValStr))
			wroteDefinition = true
		}
	}

	firstRefIndex := make(map[string]int)
	firstAssignIndex := make(map[string]int)
	for i, action := range shortcut.WFWorkflowActions {
		actionRefs := make(map[string]bool)
		collectReferencedVariableNames(action.WFWorkflowActionParameters, actionRefs)
		for rawVar := range actionRefs {
			sVar := rawVar
			sanitizeIdentifier(&sVar)
			if _, exists := firstRefIndex[sVar]; !exists {
				firstRefIndex[sVar] = i
			}
		}

		if action.WFWorkflowActionIdentifier == SetVariableIdentifier || action.WFWorkflowActionIdentifier == AppendVariableIdentifier {
			if vName := getString(action.WFWorkflowActionParameters, "WFVariableName"); vName != "" {
				sVar := vName
				sanitizeIdentifier(&sVar)
				if _, exists := firstAssignIndex[sVar]; !exists {
					firstAssignIndex[sVar] = i
				}
			}
		}
	}
	var uninitVars []string
	for sVar, refIdx := range firstRefIndex {
		assignIdx, hasAssign := firstAssignIndex[sVar]
		if (!hasAssign || refIdx <= assignIdx) && globals[sVar].value == nil && !strings.HasPrefix(sVar, "RepeatItem") && !strings.HasPrefix(sVar, "RepeatIndex") {
			uninitVars = append(uninitVars, sVar)
			namedVars[sVar] = true
		}
	}
	slices.Sort(uninitVars)
	for _, sVar := range uninitVars {
		newCodeLine(fmt.Sprintf("@%s = nil\n", sVar))
		wroteDefinition = true
	}

	if wroteDefinition {
		newCodeLine("\n")
	}
}

func metadataDefinitionValues(selected []string, definitions map[string]string, order []string) []string {
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

var currentVariableValue string
var pendingFoldUUID string

func isFoldableAction(action *ShortcutAction) bool {
	if action == nil || action.WFWorkflowActionParameters == nil {
		return false
	}
	u, ok := action.WFWorkflowActionParameters[UUID].(string)
	if !ok || u == "" {
		return false
	}
	return slices.Contains(varUUIDs, u) && !slices.Contains(constUUIDs, u)
}

func canDecompDictionaryGetValue(action *ShortcutAction) bool {
	if !isFoldableAction(action) {
		return false
	}
	if dictType, ok := action.WFWorkflowActionParameters["WFGetDictionaryValueType"].(string); ok && dictType != "" && dictType != "Value" {
		return false
	}
	keyStr, ok := action.WFWorkflowActionParameters["WFDictionaryKey"].(string)
	if !ok || strings.ContainsAny(keyStr, "'[]{}\n\r") {
		return false
	}
	inputRef := decompValue(action.WFWorkflowActionParameters["WFInput"])
	if inputRef == "" || strings.ContainsAny(inputRef, "[].\"'{ \t\n\r") {
		return false
	}
	return true
}

func decompileActions() {
	for i, action := range shortcut.WFWorkflowActions {
		actionIndex = i
		if action.WFWorkflowActionIdentifier != SetVariableIdentifier {
			currentVariableValue = ""
			pendingFoldUUID = ""
		}
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.comment":
			decompComment(&action)
		case "is.workflow.actions.gettext":
			if !hasUnrepresentableActionTextParams(action.WFWorkflowActionParameters) {
				decompTextValue(&action)
			} else {
				decompAction(&action)
			}
		case "is.workflow.actions.number":
			var nonLiteral = decompNumberValue(&action)
			if nonLiteral {
				decompAction(&action)
			}
		case "is.workflow.actions.dictionary":
			decompDictionary(&action)
		case "is.workflow.actions.math":
			if action.WFWorkflowActionParameters["WFMathOperation"] == "…" {
				decompAction(&action)
			} else {
				decompBasicExpression(&action)
			}
		case "is.workflow.actions.calculateexpression":
			decompExpression(&action)
		case SetVariableIdentifier:
			decompVariable(&action)
		case "is.workflow.actions.conditional":
			decompConditional(&action)
		case "is.workflow.actions.repeat.count":
			decompRepeat(&action)
		case "is.workflow.actions.repeat.each":
			decompFor(&action)
		case "is.workflow.actions.choosefrommenu":
			decompMenu(&action)
		case "is.workflow.actions.getvalueforkey":
			if canDecompDictionaryGetValue(&action) {
				decompDictionaryGetValue(&action)
				continue
			}
			fallthrough
		default:
			decompAction(&action)
		}
	}
}

var varUUIDs []string
var constUUIDs []string

// checkConstantLiteral determines if action should be written out on a new line as a constant or deferred for folding.
func checkConstantLiteral(action *ShortcutAction) {
	if action == nil || action.WFWorkflowActionParameters == nil {
		return
	}
	actionUUID, ok := action.WFWorkflowActionParameters[UUID].(string)
	if !ok || actionUUID == "" {
		return
	}
	if isFoldableAction(action) {
		pendingFoldUUID = actionUUID
		return
	}
	if outputName, found := uuids[actionUUID]; found {
		if emittedConstants[strings.ToLower(outputName)] {
			outputName = checkDuplicateOutputName(outputName)
			uuids[actionUUID] = outputName
		}
		emittedConstants[strings.ToLower(outputName)] = true
		newCodeLine(fmt.Sprintf("const %s = ", outputName))
		code.WriteString(currentVariableValue)
		code.WriteRune('\n')
		currentVariableValue = ""
		pendingFoldUUID = ""
	}
}

func makeConstantLiteral(action *ShortcutAction) {
	if _, found := action.WFWorkflowActionParameters[UUID]; !found {
		return
	}
	var actionUUID = action.WFWorkflowActionParameters[UUID].(string)
	if outputName, found := uuids[actionUUID]; found {
		if emittedConstants[strings.ToLower(outputName)] {
			outputName = checkDuplicateOutputName(outputName)
			uuids[actionUUID] = outputName
		}
		emittedConstants[strings.ToLower(outputName)] = true
		newCodeLine(fmt.Sprintf("const %s = ", outputName))
		code.WriteString(currentVariableValue)
		code.WriteRune('\n')
		currentVariableValue = ""
		pendingFoldUUID = ""
	}
}

func emitOrDeferActionCall(action *ShortcutAction) {
	var isConstant, isVariableValue = checkOutputType(action)
	if isConstant && isVariableValue {
		makeConstantLiteral(action)
	} else if isConstant {
		checkConstantLiteral(action)
	} else if isVariableValue {
		if u, ok := action.WFWorkflowActionParameters[UUID].(string); ok && u != "" {
			pendingFoldUUID = u
		}
	} else {
		code.WriteString(tabbedLine(currentVariableValue))
		code.WriteRune('\n')
		currentVariableValue = ""
		pendingFoldUUID = ""
	}
}

func decompComment(action *ShortcutAction) {
	if importQuestionByActionParam != nil && importQuestionByActionParam[actionIndex] != nil {
		if qName, isQ := importQuestionByActionParam[actionIndex]["WFCommentActionText"]; isQ {
			newCodeLine(fmt.Sprintf("comment(%s)\n\n", qName))
			return
		}
	}

	var commentText string
	if action != nil && action.WFWorkflowActionParameters != nil {
		if s, ok := action.WFWorkflowActionParameters["WFCommentActionText"].(string); ok {
			commentText = s
		} else if action.WFWorkflowActionParameters["WFCommentActionText"] != nil {
			commentText = strings.Trim(decompValue(action.WFWorkflowActionParameters["WFCommentActionText"]), "\"")
		}
	}

	// Always emit comment(...) action call to preserve exact Shortcut action sequence in lossless roundtrip.
	newCodeLine(fmt.Sprintf("comment(\"%s\")\n\n", escapeString(commentText)))
}

var decompilingText = false

func emitUnreferencedTextValue(action *ShortcutAction) {
	if currentVariableValue != "" && !isFoldableAction(action) {
		newCodeLine(fmt.Sprintf("text(%s)\n", currentVariableValue))
		currentVariableValue = ""
		pendingFoldUUID = ""
	}
}

func decompTextValue(action *ShortcutAction) {
	if importQuestionByActionParam != nil && importQuestionByActionParam[actionIndex] != nil {
		if qName, isQ := importQuestionByActionParam[actionIndex]["WFTextActionText"]; isQ {
			if action.WFWorkflowActionParameters[UUID] != nil {
				currentVariableValue = qName
				checkConstantLiteral(action)
				emitUnreferencedTextValue(action)
			} else {
				newCodeLine(fmt.Sprintf("text(%s)\n\n", qName))
			}
			return
		}
	}

	decompilingText = true
	if hasUnrepresentableActionTextParams(action.WFWorkflowActionParameters) {
		decompilingText = false
		decompAction(action)
		return
	}
	rawText := action.WFWorkflowActionParameters["WFTextActionText"]
	if rawStr, isStr := rawText.(string); isStr && checkInlineVarRegex.MatchString(rawStr) && !strings.ContainsAny(rawStr, "\\'\r\n") {
		currentVariableValue = fmt.Sprintf("'%s'", rawStr)
	} else {
		currentVariableValue = decompValue(rawText)
		if currentVariableValue == "" {
			currentVariableValue = "\"\""
		} else if !isStr {
			if !strings.HasPrefix(currentVariableValue, "\"") {
				currentVariableValue = "\"" + currentVariableValue
			}
			if !strings.HasSuffix(currentVariableValue, "\"") || strings.HasSuffix(currentVariableValue, "\\\"") {
				currentVariableValue = currentVariableValue + "\""
			}
		}
	}
	checkConstantLiteral(action)
	emitUnreferencedTextValue(action)
	decompilingText = false
}

func decompNumberValue(action *ShortcutAction) (nonLiteral bool) {
	if action == nil || action.WFWorkflowActionParameters == nil {
		return
	}
	var value = action.WFWorkflowActionParameters["WFNumberActionNumber"]
	if value == nil {
		return
	}
	if reflect.TypeOf(value).Kind() == reflect.Map {
		nonLiteral = true
		return
	}

	var number any
	if reflect.TypeOf(value).Kind() == reflect.String {
		str := value.(string)
		if strings.Contains(str, ".") {
			if f, err := strconv.ParseFloat(str, 64); err == nil {
				number = f
			} else {
				number = str
			}
		} else if strings.Contains(str, ",") {
			if f, err := strconv.ParseFloat(strings.Replace(str, ",", ".", 1), 64); err == nil {
				number = f
			} else {
				number = str
			}
		} else if str != "" {
			if i, err := strconv.ParseInt(str, 10, 64); err == nil {
				number = i
			} else if u, err := strconv.ParseUint(str, 10, 64); err == nil {
				number = u
			} else if f, err := strconv.ParseFloat(str, 64); err == nil {
				number = f
			} else {
				number = str
			}
		}
	} else {
		switch v := value.(type) {
		case uint64:
			number = int64(v)
		case int64:
			number = v
		case int:
			number = v
		case float64:
			number = v
		default:
			number = value
		}
	}

	currentVariableValue = decompValue(number)
	checkConstantLiteral(action)
	if currentVariableValue != "" && !isFoldableAction(action) {
		newCodeLine(fmt.Sprintf("number(%s)\n", currentVariableValue))
		currentVariableValue = ""
		pendingFoldUUID = ""
	}

	return
}

func decompBasicExpression(action *ShortcutAction) {
	var input = action.WFWorkflowActionParameters["WFInput"]
	var operand = action.WFWorkflowActionParameters["WFMathOperand"]
	var operation = "+"
	if action.WFWorkflowActionParameters["WFMathOperation"] != nil {
		switch action.WFWorkflowActionParameters["WFMathOperation"] {
		case "×":
			operation = "*"
		case "÷":
			operation = "/"
		default:
			operation = action.WFWorkflowActionParameters["WFMathOperation"].(string)
		}
	}
	var expression = strings.Trim(decompValue(input), "\"") + " " + strings.Trim(operation, "\"") + " " + strings.Trim(decompValue(operand), "\"")
	var varRegex = regexp.MustCompile(`{(.*?)}`)
	currentVariableValue = varRegex.ReplaceAllString(expression, "$1")

	checkConstantLiteral(action)
	if currentVariableValue != "" && !isFoldableAction(action) {
		currentVariableValue = ""
		pendingFoldUUID = ""
		decompAction(action)
	}
}

func decompExpression(action *ShortcutAction) {
	var raw = action.WFWorkflowActionParameters["Input"]
	var expression string
	if s, ok := raw.(string); ok {
		expression = s
	} else {
		expression = strings.Trim(decompValue(raw.(map[string]interface{})["Value"]), "\"")
	}
	outsideVars := regexp.MustCompile(`{(.*?)}`).ReplaceAllString(expression, "")
	if strings.Contains(expression, ",") || regexp.MustCompile(`[a-zA-Z]`).MatchString(outsideVars) {
		currentVariableValue = makeRawAction(action)
		emitOrDeferActionCall(action)
		return
	}

	var varRegex = regexp.MustCompile(`{(.*?)}`)
	currentVariableValue = varRegex.ReplaceAllString(expression, "$1")

	checkConstantLiteral(action)
	if currentVariableValue != "" && !isFoldableAction(action) {
		currentVariableValue = makeRawAction(action)
		emitOrDeferActionCall(action)
	}
}

var skipNextLoopVarAction bool

func peekLoopVariableAction(expectedPrefix string) string {
	if actionIndex+1 >= len(shortcut.WFWorkflowActions) {
		return ""
	}
	nextAction := peekActions(1)
	if nextAction.WFWorkflowActionIdentifier != SetVariableIdentifier {
		return ""
	}
	varName, ok := nextAction.WFWorkflowActionParameters["WFVariableName"].(string)
	if !ok || varName == "" {
		return ""
	}
	wfInput, ok := nextAction.WFWorkflowActionParameters["WFInput"].(map[string]any)
	if !ok {
		return ""
	}
	val, ok := wfInput["Value"].(map[string]any)
	if !ok {
		return ""
	}
	refName, _ := val["VariableName"].(string)
	if !strings.HasPrefix(refName, expectedPrefix) {
		return ""
	}
	if varName != "_" {
		sanitizeIdentifier(&varName)
	}
	return varName
}

func decompVariable(action *ShortcutAction) {
	if skipNextLoopVarAction {
		skipNextLoopVarAction = false
		currentVariableValue = ""
		pendingFoldUUID = ""
		return
	}
	if action.WFWorkflowActionIdentifier == SetVariableIdentifier &&
		action.WFWorkflowActionParameters["WFInput"] == nil &&
		action.WFWorkflowActionParameters["WFVariableName"] != nil {
		currentVariableValue = makeRawAction(action)
		emitOrDeferActionCall(action)
		return
	}
	var variableName = action.WFWorkflowActionParameters["WFVariableName"].(string)
	sanitizeIdentifier(&variableName)
	newCodeLine(fmt.Sprintf("@%s", variableName))

	var inputUUID string
	if inputMap := getMap(action.WFWorkflowActionParameters, "WFInput"); inputMap != nil {
		var wfInput WFInput
		mapToStruct(inputMap, &wfInput)
		inputUUID = wfInput.Value.OutputUUID
	}

	if currentVariableValue != "" && pendingFoldUUID != "" && inputUUID == pendingFoldUUID {
		code.WriteRune(' ')
		if action.WFWorkflowActionIdentifier == AppendVariableIdentifier {
			code.WriteString("+= ")
		} else {
			code.WriteString("= ")
		}

		code.WriteString(currentVariableValue)
	} else {
		var decompInput = decompValue(action.WFWorkflowActionParameters["WFInput"])
		if decompInput != "" {
			if action.WFWorkflowActionIdentifier == AppendVariableIdentifier {
				code.WriteString(fmt.Sprintf(" += %s", decompInput))
			} else {
				code.WriteString(fmt.Sprintf(" = %s", decompInput))
			}
		}
	}

	currentVariableValue = ""
	pendingFoldUUID = ""
	code.WriteRune('\n')
}

func decompDictionaryGetValue(action *ShortcutAction) {
	var dictionaryValueRef strings.Builder
	dictionaryValueRef.WriteString(decompValue(action.WFWorkflowActionParameters["WFInput"]))

	if action.WFWorkflowActionParameters["WFDictionaryKey"] != nil {
		dictionaryValueRef.WriteRune('[')
		var keyStr string
		if reflect.TypeOf(action.WFWorkflowActionParameters["WFDictionaryKey"]).Kind() == reflect.String {
			keyStr = action.WFWorkflowActionParameters["WFDictionaryKey"].(string)
		} else {
			keyStr = decompValue(action.WFWorkflowActionParameters["WFDictionaryKey"])
			if strings.HasPrefix(keyStr, "\"") && strings.HasSuffix(keyStr, "\"") {
				keyStr = strings.Trim(keyStr, "\"")
			}
		}
		dictionaryValueRef.WriteRune('\'')
		dictionaryValueRef.WriteString(keyStr)
		dictionaryValueRef.WriteRune('\'')
		dictionaryValueRef.WriteRune(']')
		currentVariableValue = dictionaryValueRef.String()
		checkConstantLiteral(action)
	}
}

func checkControlFlowOutput(action *ShortcutAction) bool {
	switch action.WFWorkflowActionIdentifier {
	case "is.workflow.actions.conditional",
		"is.workflow.actions.choosefrommenu",
		"is.workflow.actions.repeat.count",
		"is.workflow.actions.repeat.each":
		if getUint64(action.WFWorkflowActionParameters, "WFControlFlowMode") == startStatement {
			groupingID := getString(action.WFWorkflowActionParameters, "GroupingIdentifier")
			if groupingID != "" {
				if groupName, ok := controlFlowGroupByUUID[groupingID]; ok && groupName != "" {
					if emittedConstants[strings.ToLower(groupName)] {
						groupName = checkDuplicateOutputName(groupName)
					}
					emittedConstants[strings.ToLower(groupName)] = true
					newCodeLine(fmt.Sprintf("const %s = ", groupName))
					return true
				}
			}
		}
	}
	return false
}

func beginStatement(t tokenType, action *ShortcutAction) {
	if tabLevel == 0 {
		newCodeLine("\n")
	}

	var statementKeyword = fmt.Sprintf("%s ", strings.TrimSpace(string(t)))
	if checkControlFlowOutput(action) {
		code.WriteString(statementKeyword)
	} else {
		newCodeLine(statementKeyword)
	}
}

func decompMenuItemValue(item any) string {
	if item == nil {
		return `""`
	}
	if m, ok := item.(map[string]interface{}); ok {
		if val, found := m["WFValue"]; found {
			return decompValue(val)
		}
	} else if m, ok := item.(map[string]any); ok {
		if val, found := m["WFValue"]; found {
			return decompValue(val)
		}
	}
	return decompValue(item)
}

func popControlFlowGroup(groupingUUID string) {
	if tabLevel <= 0 || len(activeControlFlowGroups) == 0 {
		return
	}
	if groupingUUID != "" {
		idx := -1
		for i := len(activeControlFlowGroups) - 1; i >= 0; i-- {
			if activeControlFlowGroups[i] == groupingUUID {
				idx = i
				break
			}
		}
		if idx == -1 {
			return
		}
		activeControlFlowGroups = activeControlFlowGroups[:idx]
	} else {
		activeControlFlowGroups = activeControlFlowGroups[:len(activeControlFlowGroups)-1]
	}
	tabLevel--
	newCodeLine("}\n")
}

func decompMenu(action *ShortcutAction) {
	if len(menus) == 0 {
		menus = make(map[string][]varValue)
	}
	var controlFlowMode = getUint64(action.WFWorkflowActionParameters, "WFControlFlowMode")
	var groupingUUID = getString(action.WFWorkflowActionParameters, "GroupingIdentifier")
	switch controlFlowMode {
	case startStatement:
		activeControlFlowGroups = append(activeControlFlowGroups, groupingUUID)
		beginStatement(Menu, action)

		menus[groupingUUID] = []varValue{}
		if items, ok := action.WFWorkflowActionParameters["WFMenuItems"].([]interface{}); ok {
			for _, item := range items {
				menus[groupingUUID] = append(menus[groupingUUID], varValue{value: item})
			}
		} else if items, ok := action.WFWorkflowActionParameters["WFMenuItems"].([]any); ok {
			for _, item := range items {
				menus[groupingUUID] = append(menus[groupingUUID], varValue{value: item})
			}
		}
		code.WriteString(decompValue(action.WFWorkflowActionParameters["WFMenuPrompt"]))
		code.WriteString(" {\n")
		tabLevel++
	case statementPart:
		if tabLevel > 0 {
			tabLevel--
		}
		newCodeLine("item ")
		if _, found := action.WFWorkflowActionParameters["WFMenuItemAttributedTitle"]; found {
			code.WriteString(decompMenuItemValue(action.WFWorkflowActionParameters["WFMenuItemAttributedTitle"]))
		} else if menus[groupingUUID] != nil && len(menus[groupingUUID]) > 0 {
			var menuItem = menus[groupingUUID][0]
			menus[groupingUUID] = menus[groupingUUID][1:]
			code.WriteString(decompMenuItemValue(menuItem.value))
		}
		code.WriteString(":\n")
		tabLevel++
	case endStatement:
		popControlFlowGroup(groupingUUID)
	}
}

func decompRepeat(action *ShortcutAction) {
	var controlFlowMode = getUint64(action.WFWorkflowActionParameters, "WFControlFlowMode")
	var groupingUUID = getString(action.WFWorkflowActionParameters, "GroupingIdentifier")
	switch controlFlowMode {
	case startStatement:
		activeControlFlowGroups = append(activeControlFlowGroups, groupingUUID)
		beginStatement(Repeat, action)

		loopVar := peekLoopVariableAction("Repeat Index")
		if loopVar != "" {
			skipNextLoopVarAction = true
		} else {
			loopVar = "_"
		}
		code.WriteString(loopVar + " for ")

		code.WriteString(decompValue(action.WFWorkflowActionParameters["WFRepeatCount"]))

		code.WriteString(" {\n")
		tabLevel++
	case endStatement:
		popControlFlowGroup(groupingUUID)
	}
}

func decompFor(action *ShortcutAction) {
	var controlFlowMode = getUint64(action.WFWorkflowActionParameters, "WFControlFlowMode")
	var groupingUUID = getString(action.WFWorkflowActionParameters, "GroupingIdentifier")
	switch controlFlowMode {
	case startStatement:
		activeControlFlowGroups = append(activeControlFlowGroups, groupingUUID)
		beginStatement(RepeatWithEach, action)

		loopVar := peekLoopVariableAction("Repeat Item")
		if loopVar != "" {
			skipNextLoopVarAction = true
		} else {
			loopVar = "_"
		}
		code.WriteString(loopVar + " in ")

		code.WriteString(decompValue(action.WFWorkflowActionParameters["WFInput"]))

		code.WriteString(" {\n")
		tabLevel++
	case endStatement:
		popControlFlowGroup(groupingUUID)
	}
}

func decompConditional(action *ShortcutAction) {
	var controlFlowMode = getUint64(action.WFWorkflowActionParameters, "WFControlFlowMode")
	var groupingUUID = getString(action.WFWorkflowActionParameters, "GroupingIdentifier")
	switch controlFlowMode {
	case startStatement:
		activeControlFlowGroups = append(activeControlFlowGroups, groupingUUID)
		beginStatement(If, action)

		if action.WFWorkflowActionParameters["WFConditionalLegacyComparisonBehavior"] == true {
			code.WriteString("legacy ")
		}

		if action.WFWorkflowActionParameters["WFConditions"] != nil {
			var filterTemplates []any
			var paramFilterPrefix string
			conds := getMap(action.WFWorkflowActionParameters, "WFConditions")
			if conds != nil {
				valMap, _ := conds["Value"].(map[string]interface{})
				if valMap == nil {
					valMap, _ = conds["Value"].(map[string]any)
				}
				if valMap != nil {
					intFilterPrefix := getInt(valMap, "WFActionParameterFilterPrefix")
					paramFilterPrefix = convertFilterPrefix(intFilterPrefix)
					if ft, ok := valMap["WFActionParameterFilterTemplates"].([]interface{}); ok {
						filterTemplates = ft
					} else if ft, ok := valMap["WFActionParameterFilterTemplates"].([]any); ok {
						filterTemplates = ft
					}
				}
			}
			conditionsLen := len(filterTemplates)
			for i, condition := range filterTemplates {
				if condMap, ok := condition.(map[string]interface{}); ok {
					decompCondition(condMap, action)
				} else if condMap, ok := condition.(map[string]any); ok {
					decompCondition(condMap, action)
				}

				if i != conditionsLen-1 {
					code.WriteString(fmt.Sprintf(" %s ", paramFilterPrefix))
				}
			}
		} else {
			decompCondition(action.WFWorkflowActionParameters, action)
		}

		code.WriteString(" {\n")
		tabLevel++
	case statementPart:
		if tabLevel > 0 {
			tabLevel--
		}
		newCodeLine("} else {\n")
		tabLevel++
	case endStatement:
		popControlFlowGroup(groupingUUID)
	}
}

func matchConditionOperator(number int) tokenType {
	switch number {
	case 1000:
		return GreaterThan
	case 1001:
		return LessThan
	case 1002:
		return GreaterOrEqual
	case 1003:
		return Between
	}
	for operator, cond := range conditions {
		if cond == number {
			return operator
		}
	}
	return ""
}

func decompCondition(condition map[string]interface{}, action *ShortcutAction) {
	// Exports predating the always-emitted WFCondition fix omit the key for
	// LessThan (0); treat absence as LessThan instead of panicking.
	var conditionValue, found = condition["WFCondition"]
	if !found {
		conditionValue = uint64(0)
	}
	var conditionInt int
	switch v := conditionValue.(type) {
	case uint64:
		conditionInt = int(v)
	case int64:
		conditionInt = int(v)
	case int:
		conditionInt = v
	case float64:
		conditionInt = int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			conditionInt = int(i)
		}
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			conditionInt = i
		}
	}
	var conditionalOperator = matchConditionOperator(conditionInt)
	if conditionalOperator == "" {
		decompError(fmt.Sprintf("Invalid conditional %v", conditionInt), action)
	}
	if conditionalOperator == Empty {
		code.WriteRune('!')
	}

	code.WriteString(decompValue(condition["WFInput"]))

	if conditionalOperator == Any || conditionalOperator == Empty {
		return
	}

	hasSecondOperand := false
	var secondOperandStr string
	if condition["WFNumberValue"] != nil && condition["WFNumberValue"] != "" {
		hasSecondOperand = true
		if reflect.TypeOf(condition["WFNumberValue"]).Kind() == reflect.String {
			str := condition["WFNumberValue"].(string)
			if numberValue, convErr := strconv.Atoi(str); convErr == nil {
				secondOperandStr = decompValue(numberValue)
			} else if floatValue, fErr := strconv.ParseFloat(str, 64); fErr == nil {
				secondOperandStr = decompValue(floatValue)
			} else {
				secondOperandStr = str
			}
		} else {
			secondOperandStr = decompValue(condition["WFNumberValue"])
		}
	} else if _, foundStr := condition["WFConditionalActionString"]; foundStr {
		hasSecondOperand = true
		rawStr := condition["WFConditionalActionString"]
		if s, ok := rawStr.(string); ok && strings.Contains(s, "{") {
			secondOperandStr = fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "\\'"))
		} else {
			secondOperandStr = decompValue(rawStr)
		}
	} else if _, foundDate := condition["WFDate"]; foundDate {
		hasSecondOperand = true
		secondOperandStr = decompValue(condition["WFDate"])
	}

	if hasSecondOperand {
		code.WriteString(fmt.Sprintf(" %s %s", conditionalOperator, secondOperandStr))
	} else if conditionalOperator == Is {
		code.WriteString(" == true")
	} else if conditionalOperator == Not {
		code.WriteString(" == false")
	}
}

type DictionaryActionParameters struct {
	WFItems WFItems
}

type WFItems struct {
	Value Value
}

var decompilingDictionary = false

func decompDictionary(action *ShortcutAction) {
	var params DictionaryActionParameters
	mapToStruct(action.WFWorkflowActionParameters, &params)

	currentVariableValue = decompDictionaryValue(params.WFItems.Value.WFDictionaryFieldValueItems)

	checkConstantLiteral(action)
	if currentVariableValue != "" && !isFoldableAction(action) {
		currentVariableValue = makeRawAction(action)
		emitOrDeferActionCall(action)
	}
}

func decompDictionaryValue(items []WFDictionaryFieldValueItem) string {
	decompilingDictionary = true

	var dictionary = decompDictionaryItems(items)
	var jsonBytes, jsonErr = json.MarshalIndent(dictionary, strings.Repeat("\t", tabLevel), "\t")
	handle(jsonErr)

	decompilingDictionary = false

	return string(jsonBytes)
}

func isReferenceValue(value any) bool {
	if value == nil {
		return false
	}
	if mapValue, ok := value.(map[string]interface{}); ok {
		if mapValue["Value"] != nil && mapValue["WFSerializationType"] == "WFTextTokenAttachment" {
			return true
		}
	} else if mapValue, ok := value.(map[string]any); ok {
		if mapValue["Value"] != nil && mapValue["WFSerializationType"] == "WFTextTokenAttachment" {
			return true
		}
	}

	return false
}

func decompDictionaryItems(items []WFDictionaryFieldValueItem) (dictionary map[string]interface{}) {
	dictionary = make(map[string]interface{})
	for _, item := range items {
		var itemKey = strings.Trim(decompValue(item.WFKey), "\"")
		dictionary[itemKey] = decompDictionaryItem(item)
	}
	return
}

var numericRegex = regexp.MustCompile("^[0-9]+$")

func decompDictionaryItem(item WFDictionaryFieldValueItem) any {
	var itemStringValue = decompValue(item.WFValue)
	var itemValueMap map[string]interface{}
	if m, ok := item.WFValue.(map[string]interface{}); ok {
		itemValueMap = m
	} else if m, ok := item.WFValue.(map[string]any); ok {
		itemValueMap = m
	}

	if itemValueMap != nil && isReferenceValue(itemValueMap["Value"]) {
		return fmt.Sprintf("{%s}", itemStringValue)
	}

	switch dictDataType(item.WFItemType) {
	case itemTypeNumber:
		valStr := decompValue(item.WFValue)
		valTrimmed := strings.Trim(valStr, "\"")
		if i, err := strconv.ParseInt(valTrimmed, 10, 64); err == nil {
			return i
		} else if u, err := strconv.ParseUint(valTrimmed, 10, 64); err == nil {
			return u
		} else if f, err := strconv.ParseFloat(valTrimmed, 64); err == nil {
			return f
		}
		return valTrimmed
	case itemTypeBool:
		if itemValueMap != nil {
			switch b := itemValueMap["Value"].(type) {
			case bool:
				return b
			case int, int64, uint64:
				return b != 0
			case string:
				return strings.ToLower(b) == "true"
			}
		}
		return false
	case itemTypeText:
		return strings.Trim(itemStringValue, "\"")
	case itemTypeArray:
		if itemValueMap != nil {
			if arr, ok := itemValueMap["Value"].([]interface{}); ok {
				return decompArray(arr)
			} else if arr, ok := itemValueMap["Value"].([]any); ok {
				return decompArray(arr)
			}
		}
		return []interface{}{}
	case itemTypeDict:
		var dictionaryItems []WFDictionaryFieldValueItem
		if itemValueMap != nil {
			if items, ok := serializedDictionaryItems(itemValueMap); ok {
				dictionaryItems = items
			} else if value, ok2 := itemValueMap["Value"].(map[string]interface{}); ok2 {
				if inner, ok3 := value["Value"].(map[string]interface{}); ok3 {
					mapToStruct(inner["WFDictionaryFieldValueItems"], &dictionaryItems)
				}
			} else if value, ok2 := itemValueMap["Value"].(map[string]any); ok2 {
				if inner, ok3 := value["Value"].(map[string]any); ok3 {
					mapToStruct(inner["WFDictionaryFieldValueItems"], &dictionaryItems)
				}
			}
		}
		return decompDictionaryItems(dictionaryItems)
	default:
		return itemStringValue
	}
}

func decompArray(items []interface{}) (array []interface{}) {
	for _, item := range items {
		if reflect.TypeOf(item).Kind() == reflect.Map {
			var fieldValueItem WFDictionaryFieldValueItem
			mapToStruct(item, &fieldValueItem)
			array = append(array, decompDictionaryItem(fieldValueItem))
			continue
		}

		array = append(array, strings.Trim(decompValue(item), "\""))
	}
	return
}

func decompValue(value any) string {
	if value == nil {
		return "nil"
	}

	var valueType = reflect.TypeOf(value).Kind()
	switch valueType {
	case reflect.Map:
		return decompValueObject(value.(map[string]interface{}))
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(value)
		var elems []string
		for i := 0; i < s.Len(); i++ {
			elemStr := decompValue(s.Index(i).Interface())
			if !strings.HasPrefix(elemStr, "\"") && !strings.HasPrefix(elemStr, "{") && !strings.HasPrefix(elemStr, "[") {
				if _, err := strconv.ParseFloat(elemStr, 64); err != nil && elemStr != "true" && elemStr != "false" && elemStr != "null" && elemStr != "nil" {
					elemStr = fmt.Sprintf("\"${%s}\"", strings.TrimPrefix(elemStr, "@"))
				}
			}
			elems = append(elems, elemStr)
		}
		return fmt.Sprintf("[%s]", strings.Join(elems, ", "))
	case reflect.String:
		return fmt.Sprintf("\"%s\"", escapeString(value.(string)))
	case reflect.Float32, reflect.Float64:
		f := reflect.ValueOf(value).Float()
		if f == math.Trunc(f) {
			return strconv.FormatFloat(f, 'f', 0, 64)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func escapeString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\t", "\\t")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\"", `\"`)
	return value
}

func decompValueObject(value map[string]interface{}) string {
	if isPlainDictionaryValue(value) {
		return decompPlainDictionary(value)
	}

	if v, found := value["Value"]; found {
		if reflect.TypeOf(v).Kind() == reflect.Map {
			value = v.(map[string]interface{})
		}
	}

	if value["WFDictionaryFieldValueItems"] != nil {
		var items []WFDictionaryFieldValueItem
		mapToStruct(value["WFDictionaryFieldValueItems"], &items)

		return decompDictionaryValue(items)
	}

	switch value["Type"] {
	case "Variable":
		if _, found := value["VariableName"]; found {
			var variableName = value["VariableName"].(string)
			sanitizeIdentifier(&variableName)

			if value["Aggrandizements"] == nil {
				if _, isGlobal := globals[variableName]; isGlobal {
					return variableName
				}
				return fmt.Sprintf("@%s", variableName)
			}
		} else if value["Variable"] != nil {
			var variableValue = value["Variable"].(map[string]interface{})
			return decompValue(variableValue["Value"])
		}
		break
	case "ActionOutput":
		if outUUID, found := value["OutputUUID"].(string); found {
			if targetIdx, isTarget := actionIndexByUUID[outUUID]; isTarget && targetIdx >= actionIndex {
				return "nil"
			}
			if actionByUUID[outUUID] == nil {
				return "nil"
			}
			var outputName = uuids[outUUID]
			sanitizeIdentifier(&outputName)

			if aggrs, ok := value["Aggrandizements"].([]any); ok && len(aggrs) > 0 {
				var ref = outputName
				var aggrList []Aggrandizement
				mapToStruct(aggrs, &aggrList)
				decompAggrandizements(&ref, aggrList)
				return ref
			}
			return outputName
		}
		break
	case "Ask":
		if value["Prompt"] == nil {
			return Ask
		}

		return fmt.Sprintf("Ask: \"%s\"", value["Prompt"])
	case globals[ShortcutInput].variableType:
		return ShortcutInput
	default:
		if valType, ok := value["Type"].(string); ok {
			for name, global := range globals {
				if global.variableType == valType {
					return name
				}
			}
			if valType == "ExtensionInput" {
				return ShortcutInput
			}
		}
	}

	return decompObjectValue(value)
}

// isPlainDictionaryValue reports whether the map is a plain nested dictionary
// (an App Intent descriptor, folder reference payload, app picker, or workflow
// reference) rather than a serialized Shortcuts value or text token, which
// require their specialized handling.
func isPlainDictionaryValue(value map[string]interface{}) bool {
	if value == nil {
		return false
	}
	var serializationMarkers = []string{
		"WFSerializationType",
		"WFDictionaryFieldValueItems",
		"Value",
		"Type",
		"string",
		"attachmentsByRange",
		"Aggrandizements",
	}
	for _, marker := range serializationMarkers {
		if _, found := value[marker]; found {
			return false
		}
	}
	return true
}

// decompPlainDictionary renders a plain nested dictionary as a Cherri
// dictionary literal so unknown-action parameters (rawAction) and structured
// parameters survive decompilation instead of collapsing to empty output.
func decompPlainDictionary(value map[string]interface{}) string {
	decompilingDictionary = true
	defer func() { decompilingDictionary = false }()

	var rendered = make(map[string]any, len(value))
	for key, item := range value {
		rendered[key] = decompStructuredValue(item)
	}
	var jsonBytes, jsonErr = json.MarshalIndent(rendered, strings.Repeat("\t", tabLevel), "\t")
	handle(jsonErr)

	return string(jsonBytes)
}

// decompPlainDictionaryTree renders a plain nested dictionary as a generic
// tree so callers embedding it inside another literal keep native nesting.
func decompPlainDictionaryTree(value map[string]interface{}) map[string]any {
	var rendered = make(map[string]any, len(value))
	for key, item := range value {
		rendered[key] = decompStructuredValue(item)
	}
	return rendered
}

// decompStructuredValue renders one structured value for dictionary-literal
// contexts: reference envelopes become {@name} interpolation strings, plain
// nested dictionaries and serialized dictionaries stay native trees so they
// regenerate losslessly, and scalars pass through untouched so booleans and
// numbers survive recompilation.
func decompStructuredValue(value any) any {
	switch typed := value.(type) {
	case map[string]interface{}:
		if hasUnrepresentableTextToken(typed) {
			return typed
		}
		if isReferenceValue(typed) {
			return fmt.Sprintf("${%s}", decompValueObject(typed))
		}
		if serType, ok := typed["WFSerializationType"].(string); ok && serType == "WFTextTokenString" {
			prevRawText := decompilingRawTextToken
			decompilingRawTextToken = true
			defer func() { decompilingRawTextToken = prevRawText }()
			return decompValueObject(typed)
		}
		if isPlainDictionaryValue(typed) {
			return decompPlainDictionaryTree(typed)
		}
		if items, ok := serializedDictionaryItems(typed); ok {
			return decompDictionaryItems(items)
		}
		if _, hasAtt := typed["attachmentsByRange"]; hasAtt {
			return decompValueObject(typed)
		}
		return decompPlainDictionaryTree(typed)
	case []interface{}:
		var rendered = make([]any, len(typed))
		for i, element := range typed {
			rendered[i] = decompStructuredValue(element)
		}
		return rendered
	default:
		return value
	}
}

// serializedDictionaryItems extracts dictionary items from a Shortcuts
// serialized dictionary value (WFDictionaryFieldValue or any wrapper carrying
// Value.WFDictionaryFieldValueItems).
func serializedDictionaryItems(value map[string]any) ([]WFDictionaryFieldValueItem, bool) {
	var inner, found = value["WFDictionaryFieldValueItems"]
	if !found {
		var wrapper, hasWrapper = value["Value"]
		if !hasWrapper {
			return nil, false
		}
		var wrapperMap, isMap = wrapper.(map[string]any)
		if !isMap {
			return nil, false
		}
		inner, found = wrapperMap["WFDictionaryFieldValueItems"]
		if !found {
			return nil, false
		}
	}

	var items []WFDictionaryFieldValueItem
	mapToStruct(inner, &items)
	return items, true
}

func decompObjectValue(valueObj any) string {
	var valueType = reflect.TypeOf(valueObj).Kind()
	if valueType != reflect.Map {
		return fmt.Sprintf("%v", valueObj)
	}

	var value = valueObj.(map[string]interface{})

	var attachmentString string
	if value["Value"] != nil {
		if reflect.TypeOf(value["Value"]).Kind() != reflect.Map {
			return fmt.Sprintf("%v", valueObj)
		}
		value = value["Value"].(map[string]interface{})
	}

	if _, found := value["string"]; found {
		attachmentString = value["string"].(string)
	}

	if value["Aggrandizements"] != nil {
		attachmentString = ObjectReplaceCharStr
		decompAttachmentString(&attachmentString, map[string]any{
			"{0, 1}": value,
		})
		if !decompilingRawTextToken {
			attachmentString = strings.Trim(attachmentString, "\"{}")
		}
	}

	if attachments, found := value["attachmentsByRange"]; found {
		decompAttachmentString(&attachmentString, attachments.(map[string]interface{}))
	}

	return attachmentString
}

func utf16OffsetToRuneIndex(s string, targetUTF16Offset int) int {
	var currentUTF16 = 0
	var runeIdx = 0
	for _, r := range s {
		if currentUTF16 >= targetUTF16Offset {
			return runeIdx
		}
		if r >= utf16BMPThreshold {
			currentUTF16 += 2
		} else {
			currentUTF16 += 1
		}
		runeIdx++
	}
	return runeIdx
}

func decompAttachmentString(attachmentString *string, attachments map[string]interface{}) {
	var originalString = *attachmentString
	var runes = []rune(originalString)
	var replacements = make(map[int]string, len(attachments))

	for attachmentRange, a := range attachments {
		var attachmentRanges = strings.Split(attachmentRange, ",")
		var attachmentPosition = strings.TrimPrefix(attachmentRanges[0], "{")
		var position, convErr = strconv.Atoi(attachmentPosition)
		handle(convErr)

		var attachment Value
		mapToStruct(a, &attachment)

		var variableName = attachment.VariableName
		if targetIdx, isTarget := actionIndexByUUID[attachment.OutputUUID]; attachment.Type == "ActionOutput" && isTarget && targetIdx >= actionIndex {
			variableName = "nil"
		} else if _, uuid := uuids[attachment.OutputUUID]; uuid {
			variableName = uuids[attachment.OutputUUID]
		} else if attachment.Type == "ActionOutput" && (attachment.OutputUUID == "" || actionByUUID[attachment.OutputUUID] == nil) {
			variableName = "nil"
		}
		sanitizeIdentifier(&variableName)
		if variableName == "" && attachment.Type == globals[ShortcutInput].variableType {
			variableName = ShortcutInput
		}
		if attachment.Type != "Variable" {
			for name, global := range globals {
				if global.variableType == attachment.Type {
					variableName = name
				}
			}
		} else if _, isGlobal := globals[variableName]; !isGlobal {
			variableName = fmt.Sprintf("@%s", variableName)
		}

		if len(attachment.Aggrandizements) != 0 {
			decompAggrandizements(&variableName, attachment.Aggrandizements)
		}

		var runeIdx = utf16OffsetToRuneIndex(originalString, position)
		replacements[runeIdx] = fmt.Sprintf("{%s}", variableName)
	}

	var result strings.Builder
	for i, r := range runes {
		if rep, ok := replacements[i]; ok {
			result.WriteString(rep)
		} else {
			result.WriteRune(r)
		}
	}

	if decompilingRawTextToken {
		*attachmentString = result.String()
		return
	}

	*attachmentString = escapeString(result.String())

	if !decompilingDictionary && !decompilingText {
		if originalString == ObjectReplaceCharStr {
			*attachmentString = strings.Trim(*attachmentString, "{}")
		} else if !strings.Contains("\"", *attachmentString) {
			*attachmentString = fmt.Sprintf("\"%s\"", *attachmentString)
		}
	}
}

func decompAggrandizements(reference *string, aggrs []Aggrandizement) {
	var index string
	var coerce string
	var revContentItems = reversedContentItems()
	for _, aggr := range aggrs {
		switch aggr.Type {
		case "WFCoercionVariableAggrandizement":
			if c, found := revContentItems[aggr.CoercionItemClass]; found {
				coerce = c
			}
		case "WFDictionaryValueVariableAggrandizement":
			index = aggr.DictionaryKey
		case "WFPropertyVariableAggrandizement":
			index = aggr.PropertyName
		}
	}

	if index != "" {
		*reference = fmt.Sprintf("%s['%s']", *reference, index)
	}
	if coerce != "" {
		*reference = fmt.Sprintf("%s.%s", *reference, coerce)
	}
}

// popLine adds line to the top of the generated Cherri code.
func popLine(line string) {
	var saveCode = code.String()
	code.Reset()
	code.WriteString(fmt.Sprintf("%s\n%s", line, saveCode))
}

var macDefinition bool
var setMacDefinition bool

func shortcutPlatformRequirements() (hasMacOnly bool, hasNonMacOnly bool) {
	for _, a := range shortcut.WFWorkflowActions {
		_, def := matchAction(&a)
		if def.macOnly {
			hasMacOnly = true
		}
		if def.nonMacOnly {
			hasNonMacOnly = true
		}
	}
	return
}

func decompAction(action *ShortcutAction) {
	if skipDecompAction(action) {
		return
	}

	currentVariableValue = makeActionCallCode(action)
	emitOrDeferActionCall(action)
}

func makeActionCallCode(action *ShortcutAction) string {
	var actionCallCode strings.Builder
	var matchedIdentifier, matchedAction = matchAction(action)
	if matchedIdentifier == "" {
		checkMissingStandardInclude(&action.WFWorkflowActionIdentifier, false)

		matchedIdentifier, matchedAction = matchAction(action)
		if matchedIdentifier == "" {
			actionCallCode.WriteString(makeRawAction(action))
			return actionCallCode.String()
		}
	} else if matchedAction.includeCategory == "" && matchedAction.doc.category != "" &&
		slices.Contains(actionIncludes, matchedAction.doc.category) &&
		!slices.Contains(included, "actions/"+matchedAction.doc.category) {
		lines = []string{fmt.Sprintf("#include 'actions/%s'\n", matchedAction.doc.category)}
		resetParse()
		handleIncludes()
		currentCategory = matchedAction.doc.category
		handleActionDefinitions()
		currentCategory = ""
		mapSplitActions()
		matchedIdentifier, matchedAction = matchAction(action)
	}

	if !isActionRepresentable(action, &matchedAction) {
		actionCallCode.WriteString(makeRawAction(action))
		return actionCallCode.String()
	}

	// Self-contained decompilation invariant: any action whose normal source
	// representation lives in a standard action include (actions/<cat>.cherri)
	// must carry that include in the generated source. checkMissingStandardInclude
	// already emitted it for the probe that first located the action; this
	// covers actions that matched from categories loaded as a side effect of
	// an earlier probe.
	emitDecompiledInclude(matchedAction.includeCategory)
	if matchedAction.includeCategory == "" && (matchedIdentifier == "editEvent" || matchedIdentifier == "editReminder" || matchedIdentifier == "updateContact") {
		emitDecompiledInclude(matchedAction.doc.category)
	}

	if (matchedAction.macOnly || matchedAction.nonMacOnly) && !setMacDefinition {
		hasMacOnly, hasNonMacOnly := shortcutPlatformRequirements()
		if hasMacOnly && !hasNonMacOnly {
			macDefinition = true
			popLine("#define mac true")
		} else if hasNonMacOnly && !hasMacOnly {
			macDefinition = false
			popLine("#define mac false")
		}
		setMacDefinition = true
	}

	actionCallCode.WriteString(fmt.Sprintf("%s(", matchedIdentifier))

	if matchedAction.makeParams != nil || matchedAction.decomp != nil {
		decompActionCustom(&actionCallCode, &matchedAction, action)
	} else {
		var matchedParamsSize = len(matchedAction.parameters)
		if matchedParamsSize > 0 {
			decompActionArguments(&actionCallCode, &matchedAction, action)
		}
	}

	actionCallCode.WriteString(")")

	return actionCallCode.String()
}

func isActionRepresentable(action *ShortcutAction, def *actionDefinition) bool {
	if def == nil {
		return false
	}
	knownKeys := make(map[string]bool, len(def.parameters)+len(def.appendParams)+len(def.emittedKeys)+8)
	knownKeys[UUID] = true
	knownKeys["CustomOutputName"] = true
	knownKeys["GroupingIdentifier"] = true

	for _, param := range def.parameters {
		if param.key != "" {
			knownKeys[param.key] = true
		} else if param.name != "" {
			knownKeys[param.name] = true
		}
	}

	for k := range def.appendParams {
		knownKeys[k] = true
	}

	for _, k := range def.emittedKeys {
		knownKeys[k] = true
	}

	if def.appendParamsFunc != nil {
		func() {
			defer func() { _ = recover() }()
			for k := range def.appendParamsFunc([]actionArgument{}) {
				knownKeys[k] = true
			}
		}()
	}

	if def.appIntent.name != "" || def.appIntent.bundleIdentifier != "" || def.appIntent.appIntentIdentifier != "" {
		knownKeys["AppIntentDescriptor"] = true
	}

	if intentApp, hasIntentApp := action.WFWorkflowActionParameters["IntentAppDefinition"]; hasIntentApp {
		if !plistCanonicalEqual(def.appendParams["IntentAppDefinition"], intentApp) {
			return false
		}
	}

	for k := range action.WFWorkflowActionParameters {
		if strings.HasPrefix(k, "Show-") {
			continue
		}
		if !knownKeys[k] {
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.setters.contacts" && strings.HasPrefix(k, "WFContactContentItem") {
				continue
			}
			return false
		}
	}

	for _, param := range def.parameters {
		if param.infinite || param.key == "" || param.key == "WFAlertActionMessage" {
			continue
		}
		if !param.optional && param.defaultValue == nil {
			key := param.key
			val, found := action.WFWorkflowActionParameters[key]
			if !found || val == nil {
				return false
			}
			if s, isStr := val.(string); isStr && s == "" && (param.validType == Integer || param.validType == Float) {
				return false
			}
		}
	}

	// Validate dynamic setter properties and modes
	if action.WFWorkflowActionIdentifier == "is.workflow.actions.setters.reminders" {
		if prop, ok := action.WFWorkflowActionParameters["WFContentItemPropertyName"].(string); ok {
			validProps := map[string]bool{
				"Title": true, "List": true, "Due Date": true, "Priority": true,
				"Is Completed": true, "Notes": true, "URL": true, "Is Flagged": true,
				"Images": true, "Parent Reminder": true, "Tags": true,
				"When Messaging": true, "Subtasks": true,
			}
			if !validProps[prop] {
				return false
			}
		}
		if mode, ok := action.WFWorkflowActionParameters["Mode"].(string); ok {
			if !strings.EqualFold(mode, "Set") && !strings.EqualFold(mode, "Remove") {
				return false
			}
		}
	}
	if action.WFWorkflowActionIdentifier == "is.workflow.actions.setters.calendarevents" {
		if prop, ok := action.WFWorkflowActionParameters["WFContentItemPropertyName"].(string); ok {
			validProps := map[string]bool{
				"Start Date": true, "End Date": true, "Is All Day": true,
				"Location": true, "Duration": true, "My Status": true,
				"Attendees": true, "URL": true, "Title": true, "Notes": true,
				"Attachments": true,
			}
			if !validProps[prop] {
				return false
			}
		}
		if mode, ok := action.WFWorkflowActionParameters["Mode"].(string); ok {
			if !strings.EqualFold(mode, "Set") && !strings.EqualFold(mode, "Remove") {
				return false
			}
		}
	}

	// Filter structures cannot yet be decompiled into Cherri high-level filter expressions.
	// Fall back to lossless rawAction so valid, re-compilable Cherri code is produced.
	if filtVal, ok := action.WFWorkflowActionParameters["WFContentItemFilter"]; ok && filtVal != nil {
		return false
	}

	// Reject unrepresentable custom serialized values to trigger lossless rawAction fallback
	for _, v := range action.WFWorkflowActionParameters {
		if m, isMap := v.(map[string]any); isMap && m != nil {
			if serType, hasSer := m["WFSerializationType"].(string); hasSer {
				if serType == "WFUnknownCustomSerializedType" || strings.HasPrefix(serType, "WFUnsupported") {
					return false
				}
			}
		}
	}

	// Reject unrepresentable parameter types or enum values to trigger lossless rawAction fallback
	for _, param := range def.parameters {
		if param.key != "" {
			val, ok := action.WFWorkflowActionParameters[param.key]
			if !ok {
				continue
			}
			// String parameter cannot accept array or non-attachment dictionary
			if (param.validType == String || param.validType == RawString) && val != nil {
				if _, isArr := val.([]any); isArr {
					return false
				}
				if m, isMap := val.(map[string]any); isMap && !isReferenceValue(m) && m["WFSerializationType"] != "WFTextTokenString" {
					return false
				}
			}
			// Boolean parameter cannot accept non-boolean types (e.g. string enum like "No Alert")
			if param.validType == Bool && val != nil {
				if _, isBool := val.(bool); !isBool {
					if !isReferenceValue(val) {
						return false
					}
				}
			}
			// Enum parameter must match one of the enum values
			if param.enum != "" {
				if strVal, isStr := val.(string); isStr {
					enumVals := getEnum(param.enum)
					if len(enumVals) > 0 && !slices.Contains(enumVals, strVal) {
						return false
					}
				}
			}
		}
	}

	// Reject WFTextTokenString containing orphaned references or unresolvable inline braces
	rawStringKeys := map[string]bool{}
	if def.decomp == nil {
		for _, param := range def.parameters {
			if param.key != "" && (param.validType == String || param.validType == RawString || param.validType == Variable) {
				rawStringKeys[param.key] = true
			}
		}
	}
	for k, v := range action.WFWorkflowActionParameters {
		if hasUnrepresentableTextTokenRaw(v, rawStringKeys[k]) {
			return false
		}
	}

	return true
}

func isLiteralRawStringCandidate(s string) bool {
	return checkInlineVarRegex.MatchString(s) && !strings.ContainsAny(s, "\\'\r\n")
}

func hasUnrepresentableActionTextParams(params map[string]any) bool {
	for k, v := range params {
		if hasUnrepresentableTextTokenRaw(v, k == "WFTextActionText") {
			return true
		}
	}
	return false
}

func hasUnrepresentableTextToken(v any) bool {
	return hasUnrepresentableTextTokenRaw(v, false)
}

func isOrphanedActionOutput(attMap map[string]any) bool {
	if attMap == nil || attMap["Type"] != "ActionOutput" {
		return false
	}
	u, _ := attMap["OutputUUID"].(string)
	if u == "" || actionByUUID[u] == nil {
		return true
	}
	if targetIdx, isTarget := actionIndexByUUID[u]; isTarget && targetIdx >= actionIndex {
		return true
	}
	return false
}

func hasUnrepresentableTextTokenRaw(v any, allowTopLevelRaw bool) bool {
	if str, isStr := v.(string); isStr {
		if strings.Contains(str, "{") && checkInlineVarRegex.MatchString(str) {
			return !(allowTopLevelRaw && isLiteralRawStringCandidate(str))
		}
		return false
	}
	m, isMap := v.(map[string]any)
	if !isMap || m == nil {
		if arr, isArr := v.([]any); isArr {
			for _, elem := range arr {
				if hasUnrepresentableTextToken(elem) {
					return true
				}
			}
		}
		return false
	}
	if m["WFSerializationType"] == "WFTextTokenAttachment" {
		valMap, ok := m["Value"].(map[string]any)
		return ok && isOrphanedActionOutput(valMap)
	}
	if m["WFSerializationType"] == "WFTextTokenString" {
		valMap, ok := m["Value"].(map[string]any)
		if !ok {
			return false
		}
		// 1. Check for orphaned ActionOutput references in attachmentsByRange
		if atts, ok := valMap["attachmentsByRange"].(map[string]any); ok {
			for _, a := range atts {
				if attMap, ok := a.(map[string]any); ok {
					if isOrphanedActionOutput(attMap) {
						return true
					}
				}
			}
		}
		// 2. Check for literal {identifier} that is not an attachment and would trigger undefined inline variable
		if str, ok := valMap["string"].(string); ok {
			if strings.Contains(str, "{") {
				matches := checkInlineVarRegex.FindAllStringSubmatch(str, -1)
				for _, match := range matches {
					if len(match) >= 2 && match[1] != "" {
						ident := match[1]
						if !isDecompKnownReference(ident) {
							return true
						}
					}
				}
			}
		}
		return false
	}
	for _, subVal := range m {
		if hasUnrepresentableTextToken(subVal) {
			return true
		}
	}
	return false
}

func isDecompKnownReference(ident string) bool {
	if strings.HasPrefix(ident, "@") {
		return true
	}
	if _, isGlobal := globals[ident]; isGlobal {
		return true
	}
	if ident == "Ask" || ident == ShortcutInput || ident == "CurrentDate" || ident == "Clipboard" || ident == "Device" {
		return true
	}
	for uUUID, uName := range uuids {
		if uName == ident {
			if slices.Contains(varUUIDs, uUUID) && !slices.Contains(constUUIDs, uUUID) {
				continue
			}
			if targetIdx, ok := actionIndexByUUID[uUUID]; ok && targetIdx < actionIndex {
				return true
			}
		}
	}
	return false
}

// emitDecompiledInclude prepends a standard action include to the generated
// source exactly once per category. Only categories whose actions actually
// appear in the decompiled output are emitted; basic and builtin/Go-defined
// actions have an empty includeCategory and never trigger an include.
func emitDecompiledInclude(category string) {
	if category == "" || category == "basic" || slices.Contains(decompiledIncludes, category) {
		return
	}
	popLine(fmt.Sprintf("#include 'actions/%s'", category))
	decompiledIncludes = append(decompiledIncludes, category)
}

// checkOutputType determines if action output is a constant or a variable.
// If it is a constant we will write a constant statement on a new line to prepend the action.
func checkOutputType(action *ShortcutAction) (isConstant bool, isVariableValue bool) {
	if action.WFWorkflowActionParameters[UUID] == nil {
		return
	}
	var uuid = action.WFWorkflowActionParameters[UUID].(string)
	if _, found := uuids[uuid]; !found {
		return
	}

	isConstant = slices.Contains(constUUIDs, uuid) || !slices.Contains(varUUIDs, uuid)
	isVariableValue = slices.Contains(varUUIDs, uuid)

	return
}

// skipDecompAction skips actions we don't support or when necessary.
func skipDecompAction(action *ShortcutAction) bool {
	var identifier = actionIdentifierEnd(action.WFWorkflowActionIdentifier)
	if identifier == "nothing" && len(action.WFWorkflowActionParameters) == 0 && actionIndex+1 < len(shortcut.WFWorkflowActions) {
		var nextAction = peekActions(1)
		var controlflowActionIdentifiers = []string{"conditional", "repeat.each", "repeat.count", "choosefrommenu"}
		var nextActionIdentifier = actionIdentifierEnd(nextAction.WFWorkflowActionIdentifier)
		if slices.Contains(controlflowActionIdentifiers, nextActionIdentifier) {
			var controlFlowMode = nextAction.WFWorkflowActionParameters["WFControlFlowMode"]
			if controlFlowMode == endStatement || controlFlowMode == statementPart {
				return true
			}
		}
	}

	return false
}

func actionIdentifierEnd(identifier string) string {
	return strings.Replace(identifier, "is.workflow.actions.", "", 1)
}

func decompActionArguments(actionCallCode *strings.Builder, matchedAction *actionDefinition, action *ShortcutAction) {
	wroteAny := false
	for i, param := range matchedAction.parameters {
		if param.key == "" && !(param.name == "defaultValue" && matchedAction.identifier == "ask") {
			continue
		}

		var argValue string
		if importQuestionByActionParam != nil && importQuestionByActionParam[actionIndex] != nil {
			if qName, isQ := importQuestionByActionParam[actionIndex][param.key]; isQ {
				argValue = qName
			}
		}
		if argValue == "" {
			if value, found := action.WFWorkflowActionParameters[param.key]; found {
				if (param.validType == String || param.validType == RawString || param.validType == Variable) && !isReferenceValue(value) {
					if strVal, isStr := value.(string); isStr && isLiteralRawStringCandidate(strVal) {
						argValue = fmt.Sprintf("'%s'", strVal)
					} else {
						switch reflect.TypeOf(value).Kind() {
						case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
							reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
							reflect.Float32, reflect.Float64:
							argValue = fmt.Sprintf("\"%v\"", value)
						default:
							argValue = decompValue(value)
						}
					}
				} else {
					argValue = decompValue(value)
				}
			} else if param.key == "WFAskActionDefaultAnswer" || (param.name == "defaultValue" && matchedAction.identifier == "ask") {
				for _, altKey := range []string{
					"WFAskActionDefaultAnswer",
					"WFAskActionDefaultAnswerNumber",
					"WFAskActionDefaultAnswerURL",
					"WFAskActionDefaultAnswerDate",
					"WFAskActionDefaultAnswerTime",
					"WFAskActionDefaultAnswerDateAndTime",
				} {
					if val, foundAlt := action.WFWorkflowActionParameters[altKey]; foundAlt {
						argValue = decompValue(val)
						break
					}
				}
			}
			if argValue == "" {
				hasLaterProvided := false
				for j := i + 1; j < len(matchedAction.parameters); j++ {
					laterParam := matchedAction.parameters[j]
					laterKey := laterParam.key
					if laterKey != "" {
						if _, ok := action.WFWorkflowActionParameters[laterKey]; ok {
							hasLaterProvided = true
							break
						}
					}
					if laterKey == "WFAskActionDefaultAnswer" || (laterParam.name == "defaultValue" && matchedAction.identifier == "ask") {
						for _, altKey := range []string{
							"WFAskActionDefaultAnswer",
							"WFAskActionDefaultAnswerNumber",
							"WFAskActionDefaultAnswerURL",
							"WFAskActionDefaultAnswerDate",
							"WFAskActionDefaultAnswerTime",
							"WFAskActionDefaultAnswerDateAndTime",
						} {
							if _, okAlt := action.WFWorkflowActionParameters[altKey]; okAlt {
								hasLaterProvided = true
								break
							}
						}
						if hasLaterProvided {
							break
						}
					}
				}
				if hasLaterProvided {
					if param.defaultValue != nil || param.enum != "" {
						argValue = makeDefaultValue(param)
					} else {
						argValue = "nil"
					}
				} else if !param.optional && param.defaultValue == nil {
					argValue = makeDefaultValue(param)
				}
			}
		}

		switch param.validType {
		case Bool:
			if argValue == "0" {
				argValue = "false"
			} else if argValue == "1" {
				argValue = "true"
			}
		case Integer:
			if startsWith(Ask, argValue) {
				break
			}
			argValue = strings.Trim(argValue, "\"")
		}

		if argValue != "" {
			if !wroteAny {
				actionCallCode.WriteString(argValue)
				wroteAny = true
			} else {
				actionCallCode.WriteString(fmt.Sprintf(", %s", argValue))
			}
		}
	}
}

func makeDefaultValue(param parameterDefinition) string {
	if param.defaultValue != nil {
		if reflect.TypeOf(param.defaultValue).Kind() == reflect.String {
			return fmt.Sprintf("\"%s\"", param.defaultValue)
		}

		return fmt.Sprintf("%v", param.defaultValue)
	}

	if param.enum != "" {
		if enumVals := getEnum(param.enum); len(enumVals) > 0 {
			return fmt.Sprintf("\"%s\"", enumVals[0])
		}
	}

	switch param.validType {
	case Integer:
		return "0"
	case String:
		return "\"\""
	case Arr:
		return "[]"
	case Dict:
		return "{}"
	case Bool:
		return "false"
	}

	return "nil"
}

func makeRawAction(action *ShortcutAction) string {
	var rawParams = processRawParameters(action.WFWorkflowActionParameters)
	if len(rawParams) == 0 {
		return fmt.Sprintf("rawAction(\"%s\")", action.WFWorkflowActionIdentifier)
	}

	var jb, jsonErr = json.MarshalIndent(rawParams, strings.Repeat("\t", tabLevel), "\t")
	handle(jsonErr)

	var arguments = strings.Join([]string{fmt.Sprintf("\"%s\"", action.WFWorkflowActionIdentifier), string(jb)}, ", ")

	return fmt.Sprintf("rawAction(%s)", arguments)
}

func processRawParameters(params map[string]any) map[string]any {
	var cleanParams = make(map[string]any, len(params))
	for key, value := range params {
		if key == UUID || key == "CustomOutputName" {
			continue
		}

		// Structured values (descriptors, references, nested dictionaries)
		// must reach the generated rawAction source losslessly; strings and
		// scalars pass through unchanged.
		cleanParams[key] = decompStructuredValue(value)
	}

	return cleanParams
}

func matchAction(action *ShortcutAction) (name string, definition actionDefinition) {
	for call, def := range actions {
		var identifier = strings.ToLower(call)
		if def.identifier != "" {
			identifier = def.identifier
		}
		var appIdentifier = "is.workflow.actions"
		if def.appIdentifier != "" {
			appIdentifier = def.appIdentifier
		}
		var actionIdentifier = fmt.Sprintf("%s.%s", appIdentifier, identifier)
		if actionIdentifier != action.WFWorkflowActionIdentifier && def.overrideIdentifier != action.WFWorkflowActionIdentifier {
			continue
		}

		definition = *def
		name = call

		var splitKey = strings.ToLower(identifier)
		if def.overrideIdentifier != "" {
			splitKey = strings.ToLower(def.overrideIdentifier)
		}
		if splitActions, found := identifierMap[splitKey]; found {
			matchSplitAction(&splitActions, action.WFWorkflowActionParameters, &name, &definition)
			if identifier == "documentpicker.open" {
				if wfFile, ok := action.WFWorkflowActionParameters["WFFile"].(map[string]interface{}); ok && wfFile["fileLocation"] != nil {
					name = "getFileFromFolder"
					definition = *actions["getFileFromFolder"]
				} else if actions["getFile"] != nil {
					name = "getFile"
					definition = *actions["getFile"]
				}
				return
			}
			if name != "run" && name != "runSelf" {
				return
			}
			workflow, ok := action.WFWorkflowActionParameters["WFWorkflow"].(map[string]interface{})
			if !ok {
				return
			}
			if _, isSelf := workflow["isSelf"]; !isSelf {
				return
			}
			isSelfVal, _ := workflow["isSelf"].(bool)
			if isSelfVal || action.WFWorkflowActionParameters["WFWorkflowName"] == basename {
				name = "runSelf"
				definition = *actions["runSelf"]
			} else {
				// Split matching may have landed on either shared-identifier
				// definition on a scoring tie; the descriptor decides, so the
				// definition must follow the name or run loses its
				// shortcutName argument.
				name = "run"
				definition = *actions["run"]
			}
		}

		return
	}
	return
}

type actionMatch struct {
	params           uint
	values           uint
	addValues        uint
	addParamsViolated bool
	action           actionValue
}

func matchSplitAction(splitActions *[]actionValue, parameters map[string]any, identifier *string, definition *actionDefinition) {
	if args.Using("debug") {
		fmt.Println("## MATCHING SPLIT ACTIONS ##")
		fmt.Println("parameters", parameters)
	}

	var defaultAction, hasDefaultAction = getDefaultAction(splitActions)
	if hasDefaultAction {
		*identifier = defaultAction.identifier
		*definition = *defaultAction.definition
		if args.Using("debug") {
			fmt.Println("has default action", defaultAction.identifier)
		}
	}

	var matches = getSplitActionMatches(splitActions, parameters)

	if len(matches) == 0 {
		return
	}

	if len(matches) == 1 {
		*identifier = matches[0].action.identifier
		*definition = *matches[0].action.definition
		return
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].addValues != matches[j].addValues {
			return matches[i].addValues > matches[j].addValues
		}
		if matches[i].values != matches[j].values {
			return matches[i].values > matches[j].values
		}
		if matches[i].params != matches[j].params {
			return matches[i].params > matches[j].params
		}
		if matches[i].action.definition.defaultAction != matches[j].action.definition.defaultAction {
			return matches[i].action.definition.defaultAction
		}
		return matches[i].action.identifier < matches[j].action.identifier
	})

	if args.Using("debug") {
		for _, match := range matches[0:] {
			fmt.Printf("%s()\n", match.action.identifier)
			fmt.Println("params:", match.params, " values:", match.values, " addValues:", match.addValues)
			fmt.Println("---")
		}
		fmt.Print("\n")
	}

	if !competingMatches(matches) {
		// If the default action itself had its appendParams violated (so it is not in matches),
		// pick the best non-violated match rather than falling back to a violated default action.
		defaultInMatches := false
		if hasDefaultAction {
			for _, m := range matches {
				if m.action.identifier == defaultAction.identifier {
					defaultInMatches = true
					break
				}
			}
		}
		if hasDefaultAction && defaultInMatches {
			if args.Using("debug") {
				fmt.Println("matches are not competing! using default")
			}
			return
		}
	}

	var matchedAction = matches[0]
	*identifier = matchedAction.action.identifier
	*definition = *matchedAction.action.definition
}

func getSplitActionMatches(splitActions *[]actionValue, parameters map[string]any) (matches []actionMatch) {
	var nonViolated []actionMatch
	var allScored []actionMatch
	for _, splitAction := range *splitActions {
		var matchedParams, matchedValues, addValues, violated = scoreActionMatch(splitAction, splitAction.definition.parameters, parameters)
		m := actionMatch{
			params:            matchedParams,
			values:            matchedValues,
			addValues:         addValues,
			addParamsViolated: violated,
			action:            splitAction,
		}
		if !violated {
			nonViolated = append(nonViolated, m)
		}
		if matchedParams > 0 {
			allScored = append(allScored, m)
		}
	}
	if len(nonViolated) > 0 {
		// Prefer non-violated matches that matched at least one parameter, or all non-violated if none matched parameters (e.g. 0-arg call)
		var positiveNonViolated []actionMatch
		for _, m := range nonViolated {
			if m.params > 0 {
				positiveNonViolated = append(positiveNonViolated, m)
			}
		}
		if len(positiveNonViolated) > 0 {
			return positiveNonViolated
		}
		return nonViolated
	}
	return allScored
}

// competingMatches determines if the matches for this identifier have more values than 1 matching this action.
func competingMatches(matches []actionMatch) bool {
	if len(matches) >= 2 && (matches[0].addValues > matches[1].addValues || matches[0].values > matches[1].values || matches[0].params > matches[1].params) {
		return true
	}
	var hasCompetingValues bool
	for _, match := range matches {
		if match.addValues > 0 || match.values > 1 {
			hasCompetingValues = true
			break
		}
	}
	if hasCompetingValues {
		return true
	}
	for _, match := range matches {
		if match.params > 1 {
			return true
		}
	}
	return false
}

func scoreActionMatch(splitAction actionValue, splitActionParams []parameterDefinition, parameters map[string]any) (matchedParams uint, matchedValues uint, addValues uint, addParamsViolated bool) {
	var paramMatches, valueMatches, requiredMissing = scoreActionParams(&splitActionParams, parameters)
	matchedParams += paramMatches
	matchedValues += valueMatches

	var splitActionAddParams []parameterDefinition
	if splitAction.definition.appendParamsFunc != nil {
		func() {
			defer func() { _ = recover() }()
			for key, value := range splitAction.definition.appendParamsFunc([]actionArgument{}) {
				splitActionAddParams = append(splitActionAddParams, parameterDefinition{
					key:          key,
					defaultValue: value,
				})
			}
		}()
	}

	if len(splitAction.definition.appendParams) != 0 {
		for key, value := range splitAction.definition.appendParams {
			splitActionAddParams = append(splitActionAddParams, parameterDefinition{
				key:          key,
				defaultValue: value,
			})
		}
	}

	var addParamMatches, addValueMatches, addViolated = scoreActionAddParams(splitAction, &splitActionAddParams, parameters)
	matchedParams += addParamMatches
	matchedValues += addValueMatches
	addValues = addValueMatches
	addParamsViolated = addViolated || requiredMissing

	return
}

func scoreActionParams(splitActionParams *[]parameterDefinition, parameters map[string]any) (matchedParams uint, matchedValues uint, requiredMissing bool) {
	for _, param := range *splitActionParams {
		if param.key == "" {
			continue
		}
		if value, found := parameters[param.key]; found {
			matchedParams++
			if len(param.enum) > 0 && slices.Contains(getEnum(param.enum), fmt.Sprintf("%s", value)) {
				matchedValues++
			}
			if param.defaultValue != nil {
				var defaultValue = fmt.Sprintf("%v", param.defaultValue)
				var rawValue = strings.Trim(decompValue(value), "\"")
				if defaultValue == rawValue && defaultValue != "" && rawValue != "" {
					matchedValues++
				}
			}
		} else if !param.optional && param.defaultValue == nil {
			requiredMissing = true
		}
	}
	return
}

func plistCanonicalEqual(expected, actual any) bool {
	if plistValuesMatch(expected, actual) {
		return true
	}
	bExp, errExp := plist.Marshal(expected, plist.XMLFormat)
	bAct, errAct := plist.Marshal(actual, plist.XMLFormat)
	if errExp != nil || errAct != nil {
		return false
	}
	var vExp, vAct any
	if _, err := plist.Unmarshal(bExp, &vExp); err != nil {
		return false
	}
	if _, err := plist.Unmarshal(bAct, &vAct); err != nil {
		return false
	}
	return reflect.DeepEqual(vExp, vAct)
}

func plistValuesMatch(expected, actual any) bool {
	if expected == nil && actual == nil {
		return true
	}
	if expected == nil || actual == nil {
		return false
	}
	if reflect.DeepEqual(expected, actual) {
		return true
	}
	expKind := reflect.TypeOf(expected).Kind()
	actKind := reflect.TypeOf(actual).Kind()
	if expKind == reflect.Map && actKind == reflect.Map {
		expVal := reflect.ValueOf(expected)
		actVal := reflect.ValueOf(actual)
		if expVal.Len() != actVal.Len() {
			return false
		}
		for _, k := range expVal.MapKeys() {
			av := actVal.MapIndex(k)
			if !av.IsValid() {
				return false
			}
			if !plistValuesMatch(expVal.MapIndex(k).Interface(), av.Interface()) {
				return false
			}
		}
		return true
	}
	if expKind == reflect.Struct && actKind == reflect.Map {
		var defMap map[string]any
		mapToStruct(expected, &defMap)
		return plistValuesMatch(defMap, actual)
	}
	return fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", actual)
}

func scoreActionAddParams(splitAction actionValue, splitActionAddParams *[]parameterDefinition, parameters map[string]any) (matchedParams uint, matchedValues uint, violated bool) {
	for _, param := range *splitActionAddParams {
		if param.key == "" {
			continue
		}
		value, found := parameters[param.key]
		if !found {
			violated = true
			continue
		}
		if plistValuesMatch(param.defaultValue, value) {
			matchedParams++
			matchedValues++
		} else {
			violated = true
		}
	}
	return
}

func decompActionCustom(actionCode *strings.Builder, matchedAction *actionDefinition, action *ShortcutAction) {
	var arguments = matchedAction.decomp(action)
	if len(arguments) == 0 {
		return
	}
	actionCode.WriteString(strings.Join(arguments, ", "))
}

func glueToChar(glue string) string {
	switch glue {
	case "New Lines":
		return "\n"
	case "Spaces":
		return " "
	case "Every Character":
		return ""
	default:
		return glue
	}
}

// getDefaultAction gets the default action from a slice of split actions.
func getDefaultAction(splitActions *[]actionValue) (action actionValue, found bool) {
	for _, splitAction := range *splitActions {
		if splitAction.definition.defaultAction {
			action = splitAction
			found = true
			return
		}
	}

	return
}

func printDecompDebug() {
	fmt.Println(ansi("##### DEBUG #####\n", red))

	fmt.Println("### VARIABLES ###")
	printVariables()
	fmt.Print("\n")

	fmt.Println("### VARIABLE UUIDs ###")
	fmt.Println(varUUIDs)
	fmt.Print("\n")

	fmt.Println("### CONSTANT UUIDs ###")
	fmt.Println(constUUIDs)
	fmt.Print("\n")

	fmt.Println("### UUIDS ###")
	for uuid, name := range uuids {
		fmt.Printf("%s | %s\n", uuid, name)
	}
	fmt.Print("\n")
}

func decompWarning(message string) {
	var linesLen = strings.Count(code.String(), "\n")
	fmt.Println(ansi("Warning:", orange, bold), fmt.Sprintf("%s (%s:%d:0)\n", message, filePath, linesLen+1))
}

func decompError(message string, action *ShortcutAction) {
	fmt.Println(ansi(fmt.Sprintf("Error: %s\n\n", message), red, bold))

	fmt.Println("Action identifier:", action.WFWorkflowActionIdentifier)
	lines = strings.Split(code.String(), "\n")
	var linesLen = len(lines)
	var lastWrittenLine = lines[linesLen-1]
	var prevWrittenLine = lines[linesLen-2]
	fmt.Printf("\nStopped while writing line %d:\n", linesLen)
	fmt.Printf("%s %s\n", ansi(fmt.Sprintf("%d |", linesLen-1), dim), ansi(prevWrittenLine, dim))
	fmt.Printf("%s %s\n", ansi(fmt.Sprintf("%d |", linesLen), dim), ansi(lastWrittenLine, red))

	os.Exit(1)
}
