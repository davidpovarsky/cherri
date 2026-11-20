/*
 * Copyright (c) Cherri Language v2.0
 * Canonical Backend Adapter
 */

package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/assets"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"howett.net/plist"
)

type CanonicalBackendSession struct {
	assetEntryDocument       string
	assetCache               *assets.Cache
	assetScope               *assets.Scope
	tx                       *BackendTransaction
	cleanup                  func()
	shortcut                 Shortcut
	rawWorkflow              map[string]any
	hasRawWorkflow           bool
	rawWorkflowExact         bool
	exactActionParams        map[int]bool
	buildProfile             BuildProfile
	nativeActionResults      map[string]*nativeActionResultBinding
	exactNativeParameterMaps map[uintptr]bool
	importQuestionBindings   []importQuestionBinding
	resolvedQuestionNodes    map[string]map[string]any
	platformVersions         *canonicalPlatformVersionContext
}

func NewCanonicalBackendSession() *CanonicalBackendSession {
	var tx, cleanup = BeginBackendTransaction()
	return newCanonicalBackendSession(tx, cleanup)
}

// newCanonicalBackendSessionLocked is only used by an entry point that owns
// compilerSessionMutex for its complete request, including error recovery.
func newCanonicalBackendSessionLocked() *CanonicalBackendSession {
	var tx, cleanup = beginBackendTransactionLocked()
	return newCanonicalBackendSession(tx, cleanup)
}

func newCanonicalBackendSession(tx *BackendTransaction, cleanup func()) *CanonicalBackendSession {
	sc := BuildWorkflowHeader()
	sc.WFWorkflowImportQuestions = make([]WFQuestion, 0)
	sc.WFWorkflowTypes = schema.DefaultWorkflowTypes()
	sc.WFWorkflowInputContentItemClasses = schema.DefaultInputContentItemClasses()
	return &CanonicalBackendSession{
		tx:           tx,
		cleanup:      cleanup,
		shortcut:     sc,
		buildProfile: BuildProfileDebug,
	}
}

func (s *CanonicalBackendSession) Close() {
	if s.assetScope != nil {
		_ = s.assetScope.Close()
		s.assetScope = nil
	}
	if s.cleanup != nil {
		s.cleanup()
		s.cleanup = nil
	}
}

var backendEmitHook func(call backend.ResolvedCall) error

func (s *CanonicalBackendSession) EmitResolvedCall(call backend.ResolvedCall) (string, error) {
	if backendEmitHook != nil {
		if err := backendEmitHook(call); err != nil {
			return "", err
		}
	}

	def, ident, ok := lookupCanonicalActionLocked(call)
	if !ok || def == nil {
		return "", fmt.Errorf("canonical definition not found for action %q: typed actions must resolve canonically", call.DefinitionID)
	}

	// Resolve only compiler-private native result handles at this actual
	// consumption boundary. Exact raw producer parameters remain untouched.
	// Removed diagnostics have evaluated their argument effects, but emit no
	// consumer; the existing profile path still validates their options below.
	if s.buildProfile != BuildProfileRelease || !isRemovableDiagnosticMacro(ident) {
		for index := range call.Arguments {
			value, err := s.resolveNativeActionResultValue(call.Arguments[index].Value)
			if err != nil {
				return "", err
			}
			call.Arguments[index].Value = value.(backend.SemanticValue)
		}
	}

	// Prepare arguments for canonical action matching definition parameters
	var args []actionArgument
	providedIndices := make(map[int]bool)
	if len(def.parameters) > 0 {
		args = make([]actionArgument, len(def.parameters))
		for i := range args {
			args[i] = actionArgument{valueType: Nil, value: nil}
		}
		for _, callArg := range call.Arguments {
			pos := callArg.Position
			// Variadic values grow args, but canonical parameter positions remain fixed.
			if pos < 0 || pos >= len(def.parameters) {
				matched := false
				for pIdx, pDef := range def.parameters {
					if pDef.name == callArg.ParameterID || pDef.key == callArg.ParameterID {
						pos = pIdx
						matched = true
						break
					}
				}
				if !matched {
					lastIdx := len(def.parameters) - 1
					if lastIdx >= 0 && def.parameters[lastIdx].infinite {
						pos = lastIdx
						matched = true
					}
				}
				if !matched {
					return "", fmt.Errorf("unknown parameter %q for canonical action %q", callArg.ParameterID, ident)
				}
			}
			if pos < len(def.parameters) && def.parameters[pos].infinite && providedIndices[pos] {
				args = append(args, convertCallArgForCanonicalParameter(callArg, def.parameters[pos].key))
				providedIndices[len(args)-1] = true
			} else {
				args[pos] = convertCallArgForCanonicalParameter(callArg, def.parameters[pos].key)
				providedIndices[pos] = true
			}
		}
	} else {
		args = make([]actionArgument, len(call.Arguments))
		for i, callArg := range call.Arguments {
			args[i] = convertCallArgToActionArg(callArg)
			providedIndices[i] = true
		}
	}

	// Trim trailing unprovided parameters so len(args) reflects passed arguments
	for len(args) > 0 && !providedIndices[len(args)-1] {
		args = args[:len(args)-1]
	}

	// Canonical calls already bind newValue to its declared slot. Keep the
	// absent fourth mode slot so a Text value "Remove" cannot be reinterpreted
	// as the Legacy three-position shorthand by the shared setter builder.
	if (ident == "editEvent" || ident == "editReminder") && len(args) == 3 {
		args = append(args, actionArgument{valueType: Nil, value: nil})
	}

	// Prepare action reference
	actRef := actionReference{
		identifier: ident,
		definition: *def,
		arguments:  args,
	}
	if err := s.prepareEmbeddedAsset(call, &actRef); err != nil {
		return "", err
	}
	if ident == "rawAction" && len(args) > 0 {
		if rawIdent, ok := getArgValue(args[0]).(string); ok {
			actRef.definition.overrideIdentifier = rawIdent
		}
	}
	s.tx.PushAction(actRef)
	defer s.tx.PopAction()

	if err := s.applyPlatformVersionPolicy(call, ident); err != nil {
		return "", err
	}

	// Run canonical checkAction
	checkAction()

	// Run canonical getActionParameters
	params := getActionParameters(args)

	if isDebugStdlibMacro(ident) {
		if ident == "require" {
			for _, argument := range call.Arguments {
				if argument.ParameterID == "WFInput" || (argument.ParameterID == "" && argument.Position == 0) {
					params[requireSourceTypeKey] = argument.StaticType
					break
				}
			}
		}
		macroName := ident
		expanded, err := resolveCompilerMacroCall(macroName, params, s.shortcut.WFWorkflowActions, call.OutputUUID, call.OutputName, s.buildProfile)
		if err != nil {
			return "", err
		}
		if s.buildProfile != BuildProfileRelease && debugMacroRequestsDates(macroName, params) {
			withDates, _, backfillErr := backfillDebugLogDates(s.shortcut.WFWorkflowActions)
			if backfillErr != nil {
				return "", backfillErr
			}
			s.shortcut.WFWorkflowActions = withDates
		}
		if s.buildProfile != BuildProfileRelease && debugMacroRequestsLabels(macroName, params) {
			withLabels, _, backfillErr := backfillDebugLabelContext(s.shortcut.WFWorkflowActions)
			if backfillErr != nil {
				return "", backfillErr
			}
			s.shortcut.WFWorkflowActions = withLabels
		}
		recordDebugExpandedActions(macroName, len(s.shortcut.WFWorkflowActions), expanded)
		s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, expanded...)
		return call.OutputUUID, nil
	}

	// Determine Apple identifier
	fullIdent := getFullActionIdentifier()
	if call.AppleIdentifier != "" && (fullIdent == "is.workflow.actions." || fullIdent == "") {
		fullIdent = call.AppleIdentifier
	}

	outputUUID := call.OutputUUID
	if call.OutputUUID != "" {
		params["UUID"] = call.OutputUUID
	}
	if call.OutputName != "" {
		params["CustomOutputName"] = call.OutputName
	}

	action := ShortcutAction{
		WFWorkflowActionIdentifier: fullIdent,
		WFWorkflowActionParameters: params,
	}
	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, action)
	if len(s.importQuestionBindings) > 0 && call.NodeID != "" {
		if s.resolvedQuestionNodes == nil {
			s.resolvedQuestionNodes = map[string]map[string]any{}
		}
		if _, exists := s.resolvedQuestionNodes[call.NodeID]; exists {
			return "", fmt.Errorf("E_SETUP_BINDING: duplicate emitted call node")
		}
		s.resolvedQuestionNodes[call.NodeID] = params
	}
	return outputUUID, nil
}

func convertCallArgToActionArg(arg backend.CallArgument) actionArgument {
	if arg.Omitted {
		return actionArgument{valueType: Nil, value: nil}
	}
	val := arg.Value
	switch val.Kind {
	case backend.ValNil:
		return actionArgument{valueType: Nil, value: nil}
	case backend.ValBool:
		return actionArgument{valueType: Bool, value: val.BoolVal}
	case backend.ValInt:
		return actionArgument{valueType: Integer, value: val.IntVal}
	case backend.ValFloat:
		return actionArgument{valueType: Float, value: val.FloatVal}
	case backend.ValString:
		// V2 interpolation is explicit ValTextSegments, never inferred by a
		// legacy regular expression from an ordinary string's contents.
		return actionArgument{valueType: RawString, value: val.StrVal}
	case backend.ValReference:
		ref := val.Ref
		if ref != nil {
			desc := ReferenceToDescriptor(ref)
			varRef := varValue{
				variableType: desc.Kind,
				valueType:    Variable,
				value:        desc.VariableName,
				descriptor:   &desc,
			}
			if desc.Kind == "ActionOutput" {
				varRef.constant = true
				varRef.value = desc.OutputName
				if varRef.value == "" {
					varRef.value = desc.OutputUUID
				}
				uuids[desc.OutputName] = desc.OutputUUID
				uuids[desc.OutputUUID] = desc.OutputUUID
			}
			return actionArgument{
				valueType: Variable,
				value:     varRef,
			}
		}
		return actionArgument{valueType: Nil, value: nil}
	case backend.ValTextSegments:
		encoded := EncodeTextSegmentsShared(val.Segments)
		return actionArgument{
			valueType: String,
			value:     encoded,
		}
	case backend.ValDict:
		return actionArgument{
			valueType: Dict,
			value:     encodeTypedLiteralDictionaryItems(val.DictVal),
		}
	case backend.ValList:
		items := make([]any, len(val.ListVal))
		for i, item := range val.ListVal {
			items[i] = ConvertSemanticValueToParam(item)
		}
		return actionArgument{
			valueType: Arr,
			value:     items,
		}
	default:
		return actionArgument{
			valueType: String,
			value:     ConvertSemanticValueToParam(val),
		}
	}
}

func (s *CanonicalBackendSession) BeginControl(region backend.ControlRegion) error {
	var ident string
	var params map[string]any

	switch region.Kind {
	case backend.ControlIf:
		ident = "is.workflow.actions.conditional"
		var input any
		if region.ConditionInput != nil {
			input = map[string]any{
				"Type":     "Variable",
				"Variable": encodeReferenceMap(region.ConditionInput),
			}
		}
		params = BuildIfStart(region.GroupingIdentifier, input, region.ConditionOperator, region.ConditionValue, true)
	case backend.ControlElse:
		ident = "is.workflow.actions.conditional"
		params = BuildIfElse(region.GroupingIdentifier)
	case backend.ControlRepeat:
		ident = "is.workflow.actions.repeat.count"
		params = BuildRepeatCountStart(region.GroupingIdentifier, region.Count)
	case backend.ControlFor:
		ident = "is.workflow.actions.repeat.each"
		var iterable any = region.Iterable
		if ref, ok := region.Iterable.(*backend.Reference); ok {
			iterable = encodeReferenceMap(ref)
		}
		params = BuildRepeatEachStart(region.GroupingIdentifier, iterable)
	case backend.ControlMenu:
		ident = "is.workflow.actions.choosefrommenu"
		params = BuildMenuStart(region.GroupingIdentifier, region.MenuPrompt, region.MenuCases)
	case backend.ControlMenuCase:
		ident = "is.workflow.actions.choosefrommenu"
		params = BuildMenuCase(region.GroupingIdentifier, region.MenuCaseTitle)
	}

	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, ShortcutAction{
		WFWorkflowActionIdentifier: ident,
		WFWorkflowActionParameters: params,
	})
	return nil
}

func (s *CanonicalBackendSession) EndControl(region backend.ControlRegion) error {
	var ident string
	var params map[string]any

	switch region.Kind {
	case backend.ControlIf:
		ident = "is.workflow.actions.conditional"
		params = BuildIfEnd(region.GroupingIdentifier, region.OutputUUID)
	case backend.ControlRepeat:
		ident = "is.workflow.actions.repeat.count"
		params = BuildLoopEnd(region.GroupingIdentifier, region.OutputUUID)
	case backend.ControlFor:
		ident = "is.workflow.actions.repeat.each"
		params = BuildLoopEnd(region.GroupingIdentifier, region.OutputUUID)
	case backend.ControlMenu:
		ident = "is.workflow.actions.choosefrommenu"
		params = BuildMenuEnd(region.GroupingIdentifier, region.OutputUUID)
	}

	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, ShortcutAction{
		WFWorkflowActionIdentifier: ident,
		WFWorkflowActionParameters: params,
	})
	return nil
}

func (s *CanonicalBackendSession) EmitRawAction(appleIdentifier string, params map[string]any, outputUUID, outputName, groupingID string) error {
	_, err := s.emitRaw(appleIdentifier, params, outputUUID, outputName, groupingID)
	return err
}

func (s *CanonicalBackendSession) EmitExactRawAction(appleIdentifier string, params map[string]any, hasParams bool) error {
	if strings.TrimSpace(appleIdentifier) == "" {
		return fmt.Errorf("action identifier must not be empty")
	}
	idx := len(s.shortcut.WFWorkflowActions)
	if s.exactActionParams == nil {
		s.exactActionParams = make(map[int]bool)
	}
	s.exactActionParams[idx] = hasParams

	var clonedParams map[string]any
	if hasParams {
		if params == nil {
			clonedParams = map[string]any{}
		} else {
			clonedParams = deepClonePlistMap(params)
		}
	}

	s.rememberExactNativeParameters(clonedParams)
	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, ShortcutAction{
		WFWorkflowActionIdentifier: appleIdentifier,
		WFWorkflowActionParameters: clonedParams,
	})
	return nil
}

func sanitizeRawParamValue(v any) any {
	return sanitizeRawParamValueContext(v, false)
}

func sanitizeRawParamValueContext(v any, inAttachmentsByRange bool) any {
	if literal, ok := v.(backend.SemanticValue); ok {
		return encodeTypedLiteralParam(literal)
	}
	if tok, ok := v.(*ir.AttachmentToken); ok {
		if tok == nil {
			return nil
		}
		desc := ReferenceDescriptor{
			Kind:       tok.Type,
			OutputUUID: tok.OutputUUID,
			OutputName: tok.OutputName,
		}
		if tok.Type == "Variable" {
			desc.VariableName = tok.OutputName
		} else if tok.Type == "Ask" {
			desc.Prompt = tok.OutputName
		}
		if len(tok.Aggrandizements) > 0 {
			desc.Aggrandizements = make([]Aggrandizement, len(tok.Aggrandizements))
			for i, a := range tok.Aggrandizements {
				tStr, _ := a["Type"].(string)
				pName, _ := a["PropertyName"].(string)
				cItem, _ := a["CoercionItemClass"].(string)
				dKey, _ := a["DictionaryKey"].(string)
				pInfo := a["PropertyUserInfo"]
				dfFmt, _ := a["WFDateFormat"].(string)
				dfStyle, _ := a["WFDateFormatStyle"].(string)
				tfStyle, _ := a["WFTimeFormatStyle"].(string)
				var isoIncTime *bool
				if bPtr, ok := a["WFISO8601IncludeTime"].(*bool); ok && bPtr != nil {
					b := *bPtr
					isoIncTime = &b
				} else if bVal, ok := a["WFISO8601IncludeTime"].(bool); ok {
					isoIncTime = &bVal
				}
				desc.Aggrandizements[i] = Aggrandizement{
					rawNativeFields:      deepClonePlistMap(a),
					Type:                 tStr,
					PropertyName:         pName,
					CoercionItemClass:    cItem,
					DictionaryKey:        dKey,
					PropertyUserInfo:     pInfo,
					WFDateFormat:         dfFmt,
					WFDateFormatStyle:    dfStyle,
					WFTimeFormatStyle:    tfStyle,
					WFISO8601IncludeTime: isoIncTime,
				}
			}
		}
		if inAttachmentsByRange {
			return EncodeReferenceValue(desc)
		}
		return EncodeReferenceAttachment(desc, "WFTextTokenAttachment")
	}
	if ref, ok := v.(*backend.Reference); ok {
		desc := ReferenceToDescriptor(ref)
		if inAttachmentsByRange {
			return EncodeReferenceValue(desc)
		}
		return EncodeReferenceAttachment(desc, "WFTextTokenAttachment")
	}
	if raw, ok := v.(*ir.RawPlistValue); ok {
		return &ir.RawPlistValue{Data: sanitizeRawParamValueContext(raw.Data, inAttachmentsByRange)}
	}
	if raw, ok := v.(ir.RawPlistValue); ok {
		return &ir.RawPlistValue{Data: sanitizeRawParamValueContext(raw.Data, inAttachmentsByRange)}
	}
	if att, ok := v.(WFTextTokenAttachment); ok {
		if inAttachmentsByRange {
			return att.Value
		}
		return att
	}
	if m, ok := v.(map[string]any); ok {
		res := make(map[string]any, len(m))
		for k, val := range m {
			res[k] = sanitizeRawParamValueContext(val, inAttachmentsByRange || k == "attachmentsByRange")
		}
		return res
	}
	if m, ok := v.(map[string]interface{}); ok {
		res := make(map[string]any, len(m))
		for k, val := range m {
			res[k] = sanitizeRawParamValueContext(val, inAttachmentsByRange || k == "attachmentsByRange")
		}
		return res
	}
	if s, ok := v.([]any); ok {
		res := make([]any, len(s))
		for i, el := range s {
			res[i] = sanitizeRawParamValueContext(el, inAttachmentsByRange)
		}
		return res
	}
	if s, ok := v.([]interface{}); ok {
		res := make([]any, len(s))
		for i, el := range s {
			res[i] = sanitizeRawParamValueContext(el, inAttachmentsByRange)
		}
		return res
	}
	return v
}

func normalizeRawPlistData(data any) any {
	if data == nil {
		return nil
	}
	switch d := data.(type) {
	case *ir.RawPlistValue:
		return normalizeRawPlistData(d.Data)
	case ir.RawPlistValue:
		return normalizeRawPlistData(d.Data)
	case map[string]any:
		res := make(map[string]any, len(d))
		for k, v := range d {
			res[k] = normalizeRawPlistData(v)
		}
		return res
	case []any:
		res := make([]any, len(d))
		for i, v := range d {
			res[i] = normalizeRawPlistData(v)
		}
		return res
	default:
		return d
	}
}

func normalizeV2RawParamValue(value any) any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case *ir.RawPlistValue:
		return normalizeRawPlistData(v.Data)
	case ir.RawPlistValue:
		return normalizeRawPlistData(v.Data)
	case string:
		return v
	case map[string]any:
		if _, hasType := v["WFSerializationType"]; hasType {
			return v
		}
		if _, hasColor := v["WFColorRepresentationType"]; hasColor {
			return v
		}
		if isPlainNativeDictionary(v) {
			return normalizeNativeDescriptor(v)
		}
		normalized := make(map[string]any, len(v))
		for key, item := range v {
			normalized[key] = normalizeV2RawParamValue(item)
		}
		var valAny any = normalized
		return makeDictionaryValue(&valAny)
	case []any:
		var items = make([]WFDictionaryFieldValueItem, 0, len(v))
		for _, item := range v {
			items = append(items, makeDictionaryItem("", normalizeV2RawParamValue(item)))
		}
		return makeArrayValue(items)
	default:
		return value
	}
}

func (s *CanonicalBackendSession) emitRaw(appleIdentifier string, params map[string]any, outputUUID, outputName, groupingID string) (string, error) {
	if strings.TrimSpace(appleIdentifier) == "" {
		return "", fmt.Errorf("action identifier must not be empty")
	}
	if params == nil {
		params = make(map[string]any)
	}
	cleanParams := make(map[string]any, len(params))
	for k, v := range params {
		resolved, err := s.resolveNativeActionResultValue(v)
		if err != nil {
			return "", err
		}
		cleanParams[k] = sanitizeRawParamValue(resolved)
	}
	for k, v := range cleanParams {
		if k == "WFWorkflow" ||
			(appleIdentifier == "is.workflow.actions.list" && k == "WFItems") ||
			(appleIdentifier == "is.workflow.actions.choosefrommenu" && (k == "WFMenuItems" || k == "WFMenuItemAttributedTitle")) {
			continue
		}
		if k == "AppIntentDescriptor" || k == "IntentAppDefinition" {
			cleanParams[k] = normalizeNativeDescriptor(v)
			continue
		}
		cleanParams[k] = normalizeV2RawParamValue(v)
	}
	if appleIdentifier == "is.workflow.actions.dictionary" {
		if dictVal, ok := cleanParams["WFItems"].(WFDictionaryFieldValue); ok {
			cleanParams["WFItems"] = toExplicitDictionaryFieldValue(dictVal)
		} else if m, ok := cleanParams["WFItems"].(map[string]any); ok {
			if ser, hasSer := m["WFSerializationType"]; hasSer && ser == "WFDictionaryFieldValue" {
				ensureExplicitDictionaryMapItems(m)
			} else {
				var valAny any = m
				cleanParams["WFItems"] = makeExplicitDictionaryValue(&valAny)
			}
		} else if m, ok := cleanParams["WFItems"].(map[string]interface{}); ok {
			if ser, hasSer := m["WFSerializationType"]; hasSer && ser == "WFDictionaryFieldValue" {
				ensureExplicitDictionaryMapItems(m)
			} else {
				var valAny any = m
				cleanParams["WFItems"] = makeExplicitDictionaryValue(&valAny)
			}
		} else if items, ok := cleanParams["WFItems"].([]WFDictionaryFieldValueItem); ok {
			var valAny any = items
			cleanParams["WFItems"] = makeExplicitDictionaryValue(&valAny)
		} else if items, ok := cleanParams["WFItems"].([]any); ok {
			var valAny any = items
			cleanParams["WFItems"] = makeExplicitDictionaryValue(&valAny)
		}
	}
	if outputUUID != "" {
		cleanParams["UUID"] = outputUUID
	}
	if outputName != "" {
		cleanParams["CustomOutputName"] = outputName
	}

	if groupingID != "" {
		cleanParams["GroupingIdentifier"] = groupingID
	}
	if appleIdentifier == "is.workflow.actions.conditional" {
		if att, ok := cleanParams["WFInput"].(WFTextTokenAttachment); ok {
			cleanParams["WFInput"] = map[string]any{
				"Type":     "Variable",
				"Variable": att,
			}
		} else if input, ok := cleanParams["WFInput"].(map[string]any); ok {
			if _, hasVar := input["Variable"]; !hasVar {
				cleanParams["WFInput"] = map[string]any{
					"Type":     "Variable",
					"Variable": input,
				}
			}
		}
		if att, ok := cleanParams["WFConditionalActionString"].(WFTextTokenAttachment); ok {
			cleanParams["WFConditionalActionString"] = map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": "\uFFFC",
					"attachmentsByRange": map[string]any{
						"{0, 1}": att.Value,
					},
				},
			}
		} else if strMap, ok := cleanParams["WFConditionalActionString"].(map[string]any); ok {
			if strMap["WFSerializationType"] == "WFTextTokenAttachment" {
				cleanParams["WFConditionalActionString"] = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "\uFFFC",
						"attachmentsByRange": map[string]any{
							"{0, 1}": strMap["Value"],
						},
					},
				}
			}
		}
	}
	if appleIdentifier == "is.workflow.actions.output" {
		if num, ok := cleanParams["WFOutput"].(float64); ok {
			strVal := fmt.Sprintf("%v", num)
			if num == float64(int64(num)) {
				strVal = fmt.Sprintf("%d", int64(num))
			}
			cleanParams["WFOutput"] = map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": strVal,
				},
			}
		} else if num, ok := cleanParams["WFOutput"].(int); ok {
			cleanParams["WFOutput"] = map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": fmt.Sprintf("%d", num),
				},
			}
		} else if str, ok := cleanParams["WFOutput"].(string); ok {
			cleanParams["WFOutput"] = map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": str,
				},
			}
		} else if att, ok := cleanParams["WFOutput"].(map[string]any); ok && att["WFSerializationType"] == "WFTextTokenAttachment" {
			cleanParams["WFOutput"] = map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": "\uFFFC",
					"attachmentsByRange": map[string]any{
						"{0, 1}": att["Value"],
					},
				},
			}
		} else if att, ok := cleanParams["WFOutput"].(WFTextTokenAttachment); ok {
			cleanParams["WFOutput"] = map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": "\uFFFC",
					"attachmentsByRange": map[string]any{
						"{0, 1}": att.Value,
					},
				},
			}
		}
	}
	if appleIdentifier == "is.workflow.actions.list" {
		if arr, ok := cleanParams["WFItems"].(WFArrayValue); ok {
			cleanParams["WFItems"] = arr.Value
		} else if m, ok := cleanParams["WFItems"].(map[string]any); ok {
			if m["WFSerializationType"] == "WFArrayParameterState" {
				if val, exists := m["Value"]; exists {
					cleanParams["WFItems"] = val
				}
			}
		} else if items, ok := cleanParams["WFItems"].([]any); ok {
			var dictItems []WFDictionaryFieldValueItem
			for _, item := range items {
				if dItem, ok := item.(WFDictionaryFieldValueItem); ok {
					dictItems = append(dictItems, dItem)
				} else if m, ok := item.(map[string]any); ok && m["WFItemType"] != nil {
					itemType := 0
					if t, ok := m["WFItemType"].(int); ok {
						itemType = t
					}
					dictItems = append(dictItems, WFDictionaryFieldValueItem{
						WFItemType: itemType,
						WFValue:    m["WFValue"],
					})
				} else if m, ok := item.(map[string]interface{}); ok && m["WFItemType"] != nil {
					itemType := 0
					if t, ok := m["WFItemType"].(int); ok {
						itemType = t
					}
					dictItems = append(dictItems, WFDictionaryFieldValueItem{
						WFItemType: itemType,
						WFValue:    m["WFValue"],
					})
				} else if s, ok := item.(string); ok {
					dictItems = append(dictItems, WFDictionaryFieldValueItem{
						WFItemType: 0,
						WFValue:    s,
					})
				} else {
					dictItems = append(dictItems, makeDictionaryItem("", item))
				}
			}
			cleanParams["WFItems"] = dictItems
		}
	}
	if appleIdentifier == "is.workflow.actions.choosefrommenu" {
		switch items := cleanParams["WFMenuItems"].(type) {
		case []WFMenuItem:
			// already typed []WFMenuItem
		case []string:
			menuItems := make([]WFMenuItem, len(items))
			for i, str := range items {
				menuItems[i] = WFMenuItem{
					WFItemType: 0,
					WFValue:    str,
				}
			}
			cleanParams["WFMenuItems"] = menuItems
		case []map[string]any:
			menuItems := make([]WFMenuItem, len(items))
			for i, m := range items {
				mItem := WFMenuItem{WFItemType: 0}
				if t, ok := m["WFItemType"].(int); ok {
					mItem.WFItemType = t
				}
				if v, ok := m["WFValue"]; ok {
					mItem.WFValue = v
				}
				menuItems[i] = mItem
			}
			cleanParams["WFMenuItems"] = menuItems
		case []any:
			hasRawString := false
			for _, it := range items {
				if _, ok := it.(string); ok {
					hasRawString = true
					break
				}
			}
			if hasRawString {
				menuAny := make([]any, len(items))
				for i, it := range items {
					switch v := it.(type) {
					case string, WFMenuItem:
						menuAny[i] = v
					case map[string]any:
						mItem := WFMenuItem{WFItemType: 0}
						if t, ok := v["WFItemType"].(int); ok {
							mItem.WFItemType = t
						}
						if val, ok := v["WFValue"]; ok {
							mItem.WFValue = val
						}
						menuAny[i] = mItem
					default:
						menuAny[i] = WFMenuItem{WFItemType: 0, WFValue: it}
					}
				}
				cleanParams["WFMenuItems"] = menuAny
			} else {
				menuItems := make([]WFMenuItem, len(items))
				for i, it := range items {
					if mItem, ok := it.(WFMenuItem); ok {
						menuItems[i] = mItem
					} else if m, ok := it.(map[string]any); ok {
						mItem := WFMenuItem{WFItemType: 0}
						if t, ok := m["WFItemType"].(int); ok {
							mItem.WFItemType = t
						}
						if v, ok := m["WFValue"]; ok {
							mItem.WFValue = v
						}
						menuItems[i] = mItem
					} else if m, ok := it.(map[string]interface{}); ok {
						mItem := WFMenuItem{WFItemType: 0}
						if t, ok := m["WFItemType"].(int); ok {
							mItem.WFItemType = t
						}
						if v, ok := m["WFValue"]; ok {
							mItem.WFValue = v
						}
						menuItems[i] = mItem
					}
				}
				cleanParams["WFMenuItems"] = menuItems
			}
		}
	}
	if _, hasLimit := cleanParams["WFContentItemLimitNumber"]; hasLimit {
		cleanParams["WFContentItemLimitEnabled"] = true
	}
	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, ShortcutAction{
		WFWorkflowActionIdentifier: appleIdentifier,
		WFWorkflowActionParameters: cleanParams,
	})
	return outputUUID, nil
}

func (s *CanonicalBackendSession) RenameOutput(outputUUID string, newName string) {
	if binding, found := s.nativeActionResults[outputUUID]; found {
		if !binding.exact && binding.params != nil {
			binding.params["CustomOutputName"] = newName
		}
		return
	}
	for i := range s.shortcut.WFWorkflowActions {
		params := s.shortcut.WFWorkflowActions[i].WFWorkflowActionParameters
		if params != nil && !s.isExactNativeParameters(params) {
			if uuid, ok := params["UUID"].(string); ok && uuid == outputUUID {
				params["CustomOutputName"] = newName
				return
			}
		}
	}
}

func (s *CanonicalBackendSession) SetMetadata(name string, value any) {
	switch name {
	case "clientVersion":
		if str, ok := value.(string); ok {
			s.shortcut.WFWorkflowClientVersion = str
		}
	case "iosVersion":
		if str, ok := value.(string); ok {
			iosVersionString = str
			var parsed float64
			if _, err := fmt.Sscanf(str, "%f", &parsed); err == nil && parsed > 0 {
				iosVersion = parsed
			}
		}
	case "mac":
		if b, ok := value.(bool); ok {
			if definitions == nil {
				definitions = make(map[string]any)
			}
			definitions["mac"] = b
		}
	case "workflowTypes":
		if types, ok := value.([]string); ok {
			s.shortcut.WFWorkflowTypes = types
		}
	case "inputContentItemClasses":
		if classes, ok := value.([]string); ok {
			s.shortcut.WFWorkflowInputContentItemClasses = classes
		}
	case "outputContentItemClasses":
		if classes, ok := value.([]string); ok {
			s.shortcut.WFWorkflowOutputContentItemClasses = classes
		}
	case "noInputBehavior":
		if m, ok := value.(map[string]any); ok {
			s.shortcut.WFWorkflowNoInputBehavior = m
		} else if str, ok := value.(string); ok {
			switch strings.ToLower(str) {
			case "clipboard", "getclipboard":
				s.shortcut.WFWorkflowNoInputBehavior = map[string]any{
					"Name": "WFWorkflowNoInputBehaviorGetClipboard",
				}
			case "ask", "askforinput":
				s.shortcut.WFWorkflowNoInputBehavior = map[string]any{
					"Name": "WFWorkflowNoInputBehaviorAskForInput",
				}
			case "error", "showerror":
				s.shortcut.WFWorkflowNoInputBehavior = map[string]any{
					"Name": "WFWorkflowNoInputBehaviorShowError",
				}
			default:
				s.shortcut.WFWorkflowNoInputBehavior = map[string]any{
					"Name": str,
				}
			}
		}
	case "quickActionSurfaces":
		if surfaces, ok := value.([]string); ok {
			s.shortcut.WFQuickActionSurfaces = surfaces
		}
	case "triggers":
		if triggers, ok := value.([]AutomationTrigger); ok {
			s.shortcut.WFWorkflowTriggers = triggers
		}
	case "trigger":
		if trigger, ok := value.(AutomationTrigger); ok {
			s.shortcut.WFWorkflowTriggers = append(s.shortcut.WFWorkflowTriggers, trigger)
		} else if m, ok := value.(map[string]any); ok {
			ident, _ := m["Identifier"].(string)
			params, _ := m["Parameters"].(map[string]any)
			u, _ := m["UUID"].(string)
			s.shortcut.WFWorkflowTriggers = append(s.shortcut.WFWorkflowTriggers, AutomationTrigger{
				WFTriggerIdentifier:           ident,
				WFTriggerSerializedParameters: params,
				WFTriggerUUID:                 u,
			})
		}
	case "iconGlyph":
		if g, ok := value.(int); ok {
			s.shortcut.WFWorkflowIcon.WFWorkflowIconGlyphNumber = int64(g)
		} else if g, ok := value.(int64); ok {
			s.shortcut.WFWorkflowIcon.WFWorkflowIconGlyphNumber = g
		}
	case "iconColor":
		if c, ok := value.(int); ok {
			s.shortcut.WFWorkflowIcon.WFWorkflowIconStartColor = c
		} else if c, ok := value.(int64); ok {
			s.shortcut.WFWorkflowIcon.WFWorkflowIconStartColor = int(c)
		}
	case "hasShortcutInputVariables":
		if b, ok := value.(bool); ok {
			s.shortcut.WFWorkflowHasShortcutInputVariables = b
		}
	case "rawWorkflow":
		if m, ok := value.(map[string]any); ok {
			s.rawWorkflow = m
			s.hasRawWorkflow = true
		}
	case "rawWorkflowExact":
		if b, ok := value.(bool); ok {
			s.rawWorkflowExact = b
		}
	}
}

func (s *CanonicalBackendSession) AddImportQuestion(q map[string]any) {
	actionIndex := 0
	if idx, ok := q["ActionIndex"].(int); ok {
		actionIndex = idx
	} else if idx, ok := q["ActionIndex"].(int64); ok {
		actionIndex = int(idx)
	}
	paramKey, _ := q["ParameterKey"].(string)
	text, _ := q["Text"].(string)
	defVal, _ := q["DefaultValue"].(string)
	category, _ := q["Category"].(string)

	s.importQuestionBindings = append(s.importQuestionBindings, importQuestionBinding{name: paramKey, questionIndex: len(s.shortcut.WFWorkflowImportQuestions)})
	s.shortcut.WFWorkflowImportQuestions = append(s.shortcut.WFWorkflowImportQuestions, WFQuestion{
		ActionIndex:  actionIndex,
		ParameterKey: paramKey,
		Text:         text,
		DefaultValue: defVal,
		Category:     category,
	})
}

func (s *CanonicalBackendSession) Finalize() ([]byte, error) {
	if err := s.finalizeImportQuestions(); err != nil {
		return nil, err
	}
	if err := s.validateNativeActionResults(); err != nil {
		return nil, err
	}
	if err := validateReleaseMacroReferences(s.shortcut.WFWorkflowActions, s.buildProfile); err != nil {
		return nil, err
	}
	if s.hasRawWorkflow || len(s.rawWorkflow) > 0 {
		if !s.rawWorkflowExact {
			if len(s.shortcut.WFWorkflowTypes) == 0 {
				s.shortcut.WFWorkflowTypes = schema.DefaultWorkflowTypes()
			}
			if len(s.shortcut.WFWorkflowInputContentItemClasses) == 0 {
				s.shortcut.WFWorkflowInputContentItemClasses = schema.DefaultInputContentItemClasses()
			}
		}
		merged := mergeWorkflowHeaderMapWithDialect(
			"v2",
			s.rawWorkflow,
			s.shortcut.WFWorkflowActions,
			s.exactActionParams,
			s.shortcut,
			!s.rawWorkflowExact,
		)
		var buf bytes.Buffer
		format := plist.XMLFormat
		if plistValueHasXMLUnsafeStringChar(merged) {
			format = plist.BinaryFormat
		}
		enc := plist.NewEncoderForFormat(&buf, format)
		if format == plist.XMLFormat {
			enc.Indent("\t")
		}
		if err := enc.Encode(merged); err != nil {
			return nil, fmt.Errorf("plist encoding error: %w", err)
		}
		return buf.Bytes(), nil
	}

	if len(s.shortcut.WFWorkflowTypes) == 0 {
		s.shortcut.WFWorkflowTypes = schema.DefaultWorkflowTypes()
	}
	if len(s.shortcut.WFWorkflowInputContentItemClasses) == 0 {
		s.shortcut.WFWorkflowInputContentItemClasses = schema.DefaultInputContentItemClasses()
	}

	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("\t")
	if err := enc.Encode(s.shortcut); err != nil {
		return nil, fmt.Errorf("plist encoding error: %w", err)
	}
	return buf.Bytes(), nil
}

func toExplicitDictionaryFieldValue(v WFDictionaryFieldValue) WFDictionaryFieldValue {
	ser := v.WFSerializationType
	if ser == "" {
		ser = "WFDictionaryFieldValue"
	}
	var items []WFDictionaryFieldValueItem
	switch inner := v.Value.(type) {
	case WFExplicitDictionaryFieldValueWrapper:
		items = inner.WFDictionaryFieldValueItems
	case *WFExplicitDictionaryFieldValueWrapper:
		if inner != nil {
			items = inner.WFDictionaryFieldValueItems
		}
	case WFDictionaryFieldValueWrapper:
		items = inner.WFDictionaryFieldValueItems
	case *WFDictionaryFieldValueWrapper:
		if inner != nil {
			items = inner.WFDictionaryFieldValueItems
		}
	case map[string]any:
		ensureExplicitDictionaryInnerMap(inner)
		return WFDictionaryFieldValue{Value: inner, WFSerializationType: ser}
	}
	if len(items) > 0 {
		return WFDictionaryFieldValue{
			Value: WFDictionaryFieldValueWrapper{
				WFDictionaryFieldValueItems: items,
			},
			WFSerializationType: ser,
		}
	}
	return WFDictionaryFieldValue{
		Value: WFExplicitDictionaryFieldValueWrapper{
			WFDictionaryFieldValueItems: []WFDictionaryFieldValueItem{},
		},
		WFSerializationType: ser,
	}
}

func ensureExplicitDictionaryMapItems(m map[string]any) {
	if m == nil {
		return
	}
	val, hasVal := m["Value"]
	if !hasVal || val == nil {
		m["Value"] = map[string]any{"WFDictionaryFieldValueItems": []any{}}
		return
	}
	switch inner := val.(type) {
	case WFExplicitDictionaryFieldValueWrapper:
		if len(inner.WFDictionaryFieldValueItems) > 0 {
			m["Value"] = WFDictionaryFieldValueWrapper{WFDictionaryFieldValueItems: inner.WFDictionaryFieldValueItems}
		} else if inner.WFDictionaryFieldValueItems == nil {
			inner.WFDictionaryFieldValueItems = []WFDictionaryFieldValueItem{}
			m["Value"] = inner
		}
	case WFDictionaryFieldValueWrapper:
		if len(inner.WFDictionaryFieldValueItems) == 0 {
			m["Value"] = WFExplicitDictionaryFieldValueWrapper{WFDictionaryFieldValueItems: []WFDictionaryFieldValueItem{}}
		}
	case map[string]any:
		ensureExplicitDictionaryInnerMap(inner)
	}
}

func ensureExplicitDictionaryInnerMap(inner map[string]any) {
	if inner == nil {
		return
	}
	if rawItems, exists := inner["WFDictionaryFieldValueItems"]; !exists || rawItems == nil {
		inner["WFDictionaryFieldValueItems"] = []any{}
	}
}

func plistValueHasXMLUnsafeStringChar(v any) bool {
	switch x := v.(type) {
	case string:
		for _, r := range x {
			if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
				return true
			}
		}
		return false
	case map[string]any:
		for k, val := range x {
			if plistValueHasXMLUnsafeStringChar(k) || plistValueHasXMLUnsafeStringChar(val) {
				return true
			}
		}
		return false
	case []any:
		for _, item := range x {
			if plistValueHasXMLUnsafeStringChar(item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
