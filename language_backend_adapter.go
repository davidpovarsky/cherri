/*
 * Copyright (c) Cherri Language v2.0
 * Canonical Backend Adapter
 */

package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"howett.net/plist"
)

type CanonicalBackendSession struct {
	tx          *BackendTransaction
	cleanup     func()
	shortcut    Shortcut
	rawWorkflow map[string]any
}

func NewCanonicalBackendSession() *CanonicalBackendSession {
	tx, cleanup := BeginBackendTransaction()
	sc := BuildWorkflowHeader()
	sc.WFWorkflowImportQuestions = make([]WFQuestion, 0)
	return &CanonicalBackendSession{
		tx:       tx,
		cleanup:  cleanup,
		shortcut: sc,
	}
}

func (s *CanonicalBackendSession) Close() {
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

	def, ident, ok := lookupCanonicalAction(call)
	if !ok || def == nil {
		return "", fmt.Errorf("canonical definition not found for action %q: typed actions must resolve canonically", call.DefinitionID)
	}

	// Prepare arguments for canonical action matching definition parameters
	var args []actionArgument
	if len(def.parameters) > 0 {
		args = make([]actionArgument, len(def.parameters))
		for i := range args {
			args[i] = actionArgument{valueType: Nil, value: nil}
		}
		for _, callArg := range call.Arguments {
			pos := callArg.Position
			if pos < 0 || pos >= len(args) {
				matched := false
				for pIdx, pDef := range def.parameters {
					if pDef.name == callArg.ParameterID || pDef.key == callArg.ParameterID {
						pos = pIdx
						matched = true
						break
					}
				}
				if !matched {
					return "", fmt.Errorf("unknown parameter %q for canonical action %q", callArg.ParameterID, ident)
				}
			}
			args[pos] = convertCallArgToActionArg(callArg)
		}
	} else {
		args = make([]actionArgument, len(call.Arguments))
		for i, callArg := range call.Arguments {
			args[i] = convertCallArgToActionArg(callArg)
		}
	}

	// Trim trailing omitted/nil arguments so len(args) reflects passed arguments
	for len(args) > 0 && args[len(args)-1].valueType == Nil {
		args = args[:len(args)-1]
	}

	// Prepare action reference
	actRef := actionReference{
		identifier: ident,
		definition: *def,
		arguments:  args,
	}
	if ident == "rawAction" && len(args) > 0 {
		if rawIdent, ok := getArgValue(args[0]).(string); ok {
			actRef.definition.overrideIdentifier = rawIdent
		}
	}
	s.tx.PushAction(actRef)
	defer s.tx.PopAction()

	// Run canonical checkAction
	checkAction()

	// Run canonical getActionParameters
	params := getActionParameters(args)

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
		return actionArgument{valueType: String, value: val.StrVal}
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
		dictMap := make(map[string]interface{}, len(val.DictVal))
		for _, entry := range val.DictVal {
			dictMap[entry.Key] = ConvertSemanticValueToParam(entry.Value)
		}
		return actionArgument{
			valueType: Dict,
			value:     dictMap,
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

func sanitizeRawParamValue(v any) any {
	return sanitizeRawParamValueContext(v, false)
}

func sanitizeRawParamValueContext(v any, inAttachmentsByRange bool) any {
	if tok, ok := v.(*ir.AttachmentToken); ok {
		desc := ReferenceDescriptor{
			Kind:       tok.Type,
			OutputUUID: tok.OutputUUID,
			OutputName: tok.OutputName,
		}
		if tok.Type == "Variable" || tok.Type == "ExtensionInput" {
			desc.VariableName = tok.OutputName
		}
		if len(tok.Aggrandizements) > 0 {
			desc.Aggrandizements = make([]Aggrandizement, len(tok.Aggrandizements))
			for i, a := range tok.Aggrandizements {
				tStr, _ := a["Type"].(string)
				pName, _ := a["PropertyName"].(string)
				cItem, _ := a["CoercionItemClass"].(string)
				dKey, _ := a["DictionaryKey"].(string)
				pInfo := a["PropertyUserInfo"]
				desc.Aggrandizements[i] = Aggrandizement{
					Type:              tStr,
					PropertyName:     pName,
					CoercionItemClass: cItem,
					DictionaryKey:     dKey,
					PropertyUserInfo:  pInfo,
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
		cleanParams[k] = sanitizeRawParamValue(v)
	}
	for k, v := range cleanParams {
		if k == "WFWorkflow" {
			continue
		}
		cleanParams[k] = normalizeV2RawParamValue(v)
	}
	if appleIdentifier == "is.workflow.actions.dictionary" {
		if dictVal, ok := cleanParams["WFItems"].(WFDictionaryFieldValue); ok {
			cleanParams["WFItems"] = dictVal
		} else if m, ok := cleanParams["WFItems"].(map[string]any); ok {
			if ser, hasSer := m["WFSerializationType"]; hasSer && ser == "WFDictionaryFieldValue" {
				// already wrapped
			} else {
				var valAny any = m
				cleanParams["WFItems"] = makeDictionaryValue(&valAny)
			}
		} else if m, ok := cleanParams["WFItems"].(map[string]interface{}); ok {
			if ser, hasSer := m["WFSerializationType"]; hasSer && ser == "WFDictionaryFieldValue" {
				// already wrapped
			} else {
				var valAny any = m
				cleanParams["WFItems"] = makeDictionaryValue(&valAny)
			}
		} else if items, ok := cleanParams["WFItems"].([]WFDictionaryFieldValueItem); ok {
			var valAny any = items
			cleanParams["WFItems"] = makeDictionaryValue(&valAny)
		} else if items, ok := cleanParams["WFItems"].([]any); ok {
			var valAny any = items
			cleanParams["WFItems"] = makeDictionaryValue(&valAny)
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
		if mode, ok := cleanParams["WFControlFlowMode"].(int); !ok || mode == 0 {
			cleanParams["WFConditionalLegacyComparisonBehavior"] = true
		}
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
	}
	if appleIdentifier == "is.workflow.actions.output" {
		if att, ok := cleanParams["WFOutput"].(WFTextTokenAttachment); ok {
			cleanParams["WFOutput"] = map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value": map[string]any{
					"string": "\uFFFC",
					"attachmentsByRange": map[string]any{
						"{0, 1}": att.Value,
					},
				},
			}
		} else if outMap, ok := cleanParams["WFOutput"].(map[string]any); ok {
			if outMap["WFSerializationType"] == "WFTextTokenAttachment" {
				cleanParams["WFOutput"] = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string": "\uFFFC",
						"attachmentsByRange": map[string]any{
							"{0, 1}": outMap["Value"],
						},
					},
				}
			}
		} else if num, ok := cleanParams["WFOutput"].(float64); ok {
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
				} else if s, ok := it.(string); ok {
					menuItems[i] = WFMenuItem{
						WFItemType: 0,
						WFValue:    s,
					}
				}
			}
			cleanParams["WFMenuItems"] = menuItems
		}
		if title, ok := cleanParams["WFMenuItemTitle"].(string); ok {
			if _, hasAttr := cleanParams["WFMenuItemAttributedTitle"]; !hasAttr {
				cleanParams["WFMenuItemAttributedTitle"] = title
			}
		}
	}
	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, ShortcutAction{
		WFWorkflowActionIdentifier: appleIdentifier,
		WFWorkflowActionParameters: cleanParams,
	})
	return outputUUID, nil
}

func (s *CanonicalBackendSession) RenameOutput(outputUUID string, newName string) {
	for i := range s.shortcut.WFWorkflowActions {
		params := s.shortcut.WFWorkflowActions[i].WFWorkflowActionParameters
		if params != nil {
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
			switch str {
			case "clipboard", "getClipboard":
				s.shortcut.WFWorkflowNoInputBehavior = map[string]any{
					"Name": "WFWorkflowNoInputBehaviorGetClipboard",
				}
			case "ask", "askForInput":
				s.shortcut.WFWorkflowNoInputBehavior = map[string]any{
					"Name": "WFWorkflowNoInputBehaviorAskForInput",
				}
			case "error", "showError":
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
		} else if m, ok := value.(map[string]interface{}); ok {
			s.rawWorkflow = m
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

	s.shortcut.WFWorkflowImportQuestions = append(s.shortcut.WFWorkflowImportQuestions, WFQuestion{
		ActionIndex:  actionIndex,
		ParameterKey: paramKey,
		Text:         text,
		DefaultValue: defVal,
		Category:     category,
	})
}

func (s *CanonicalBackendSession) BindImportQuestion(questionName string, wireKey string) {
	for idx, q := range s.shortcut.WFWorkflowImportQuestions {
		if q.ParameterKey == questionName {
			s.shortcut.WFWorkflowImportQuestions[idx].ActionIndex = len(s.shortcut.WFWorkflowActions)
			s.shortcut.WFWorkflowImportQuestions[idx].ParameterKey = wireKey
			break
		}
	}
}

func (s *CanonicalBackendSession) Finalize() ([]byte, error) {
	if len(s.rawWorkflow) > 0 {
		merged := make(map[string]any, len(s.rawWorkflow)+10)
		for k, v := range s.rawWorkflow {
			merged[k] = v
		}
		var sBuf bytes.Buffer
		if err := plist.NewEncoder(&sBuf).Encode(s.shortcut); err == nil {
			var sMap map[string]any
			if _, err := plist.Unmarshal(sBuf.Bytes(), &sMap); err == nil {
				for k, v := range sMap {
					merged[k] = v
				}
			}
		}
		var buf bytes.Buffer
		enc := plist.NewEncoder(&buf)
		enc.Indent("\t")
		if err := enc.Encode(merged); err != nil {
			return nil, fmt.Errorf("plist encoding error: %w", err)
		}
		return buf.Bytes(), nil
	}

	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("\t")
	if err := enc.Encode(s.shortcut); err != nil {
		return nil, fmt.Errorf("plist encoding error: %w", err)
	}
	return buf.Bytes(), nil
}
