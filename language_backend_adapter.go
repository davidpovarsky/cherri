/*
 * Copyright (c) Cherri Language v2.0
 * Canonical Backend Adapter
 */

package main

import (
	"bytes"
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"howett.net/plist"
)

type CanonicalBackendSession struct {
	tx       *BackendTransaction
	cleanup  func()
	shortcut Shortcut
}

func NewCanonicalBackendSession() *CanonicalBackendSession {
	tx, cleanup := BeginBackendTransaction()
	return &CanonicalBackendSession{
		tx:      tx,
		cleanup: cleanup,
		shortcut: Shortcut{
			WFWorkflowClientVersion:   "4711",
			WFWorkflowActions:         make([]ShortcutAction, 0),
			WFWorkflowImportQuestions: make([]WFQuestion, 0),
		},
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
					continue
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

	// Prepare action reference
	actRef := actionReference{
		identifier: ident,
		definition: *def,
		arguments:  args,
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
		dictItems := make([]WFDictionaryFieldValueItem, len(val.DictVal))
		for i, entry := range val.DictVal {
			dictItems[i] = EncodeDictionaryItemShared(entry.Key, ConvertSemanticValueToParam(entry.Value))
		}
		return actionArgument{
			valueType: Dict,
			value:     dictItems,
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
	params := make(map[string]any)
	params["GroupingIdentifier"] = region.GroupingIdentifier
	params["WFControlFlowMode"] = 0

	switch region.Kind {
	case backend.ControlIf:
		ident = "is.workflow.actions.conditional"
		if region.ConditionInput != nil {
			params["WFInput"] = map[string]any{
				"Type":     "Variable",
				"Variable": encodeReferenceMap(region.ConditionInput),
			}
		}
		params["WFCondition"] = region.ConditionOperator
		if region.ConditionValue != nil {
			params["WFConditionalActionString"] = fmt.Sprint(region.ConditionValue)
		}
	case backend.ControlRepeat:
		ident = "is.workflow.actions.repeat.count"
		if region.Count != nil {
			params["WFRepeatCount"] = region.Count
		}
	case backend.ControlFor:
		ident = "is.workflow.actions.repeat.each"
		if region.Iterable != nil {
			if ref, ok := region.Iterable.(*backend.Reference); ok {
				params["WFInput"] = encodeReferenceMap(ref)
			} else {
				params["WFInput"] = region.Iterable
			}
		}
	case backend.ControlMenu:
		ident = "is.workflow.actions.choosefrommenu"
	}

	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, ShortcutAction{
		WFWorkflowActionIdentifier: ident,
		WFWorkflowActionParameters: params,
	})
	return nil
}

func (s *CanonicalBackendSession) EndControl(region backend.ControlRegion) error {
	var ident string
	params := make(map[string]any)
	params["GroupingIdentifier"] = region.GroupingIdentifier
	params["WFControlFlowMode"] = 2 // End

	switch region.Kind {
	case backend.ControlIf:
		ident = "is.workflow.actions.conditional"
	case backend.ControlRepeat:
		ident = "is.workflow.actions.repeat.count"
	case backend.ControlFor:
		ident = "is.workflow.actions.repeat.each"
	case backend.ControlMenu:
		ident = "is.workflow.actions.choosefrommenu"
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
		return EncodeReferenceAttachment(desc, "WFTextTokenAttachment")
	}
	if ref, ok := v.(*backend.Reference); ok {
		desc := ReferenceToDescriptor(ref)
		return EncodeReferenceAttachment(desc, "WFTextTokenAttachment")
	}
	if m, ok := v.(map[string]any); ok {
		res := make(map[string]any, len(m))
		for k, val := range m {
			res[k] = sanitizeRawParamValue(val)
		}
		return res
	}
	if m, ok := v.(map[string]interface{}); ok {
		res := make(map[string]any, len(m))
		for k, val := range m {
			res[k] = sanitizeRawParamValue(val)
		}
		return res
	}
	if s, ok := v.([]any); ok {
		res := make([]any, len(s))
		for i, el := range s {
			res[i] = sanitizeRawParamValue(el)
		}
		return res
	}
	if s, ok := v.([]interface{}); ok {
		res := make([]any, len(s))
		for i, el := range s {
			res[i] = sanitizeRawParamValue(el)
		}
		return res
	}
	return v
}

func (s *CanonicalBackendSession) emitRaw(appleIdentifier string, params map[string]any, outputUUID, outputName, groupingID string) (string, error) {
	if params == nil {
		params = make(map[string]any)
	}
	cleanParams := make(map[string]any, len(params))
	for k, v := range params {
		cleanParams[k] = sanitizeRawParamValue(v)
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
	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, ShortcutAction{
		WFWorkflowActionIdentifier: appleIdentifier,
		WFWorkflowActionParameters: cleanParams,
	})
	return outputUUID, nil
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

func (s *CanonicalBackendSession) Finalize() ([]byte, error) {
	var buf bytes.Buffer
	enc := plist.NewEncoder(&buf)
	enc.Indent("\t")
	if err := enc.Encode(s.shortcut); err != nil {
		return nil, fmt.Errorf("plist encoding error: %w", err)
	}
	return buf.Bytes(), nil
}
