/*
 * Copyright (c) Cherri Language v2.0
 * Canonical Backend Adapter
 */

package main

import (
	"bytes"
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
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

func (s *CanonicalBackendSession) EmitResolvedCall(call backend.ResolvedCall) (string, error) {
	def, ident, ok := lookupCanonicalAction(call)
	if !ok || def == nil {
		// If canonical definition is not in actions registry, emit raw action
		if call.AppleIdentifier != "" {
			params := make(map[string]any)
			for _, arg := range call.Arguments {
				if !arg.Omitted {
					params[arg.ParameterID] = ConvertSemanticValueToParam(arg.Value)
				}
			}
			return s.emitRaw(call.AppleIdentifier, params, call.OutputUUID, call.OutputName, "")
		}
		return "", fmt.Errorf("unknown action: %s", call.DefinitionID)
	}

	// Prepare arguments for canonical action
	args := make([]actionArgument, len(call.Arguments))
	for i, arg := range call.Arguments {
		args[i] = convertCallArgToActionArg(arg)
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
	if outputUUID == "" {
		outputUUID = call.NodeID
	}

	if outputUUID != "" {
		params["UUID"] = outputUUID
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
			varRef := varValue{
				variableType: "Variable",
				valueType:    Variable,
				value:        ref.ProducerName,
			}
			if ref.Kind == backend.RefActionResult || ref.Kind == backend.RefLoopResult {
				varRef.constant = true
				if ref.ProducerName != "" {
					varRef.value = ref.ProducerName
					uuids[ref.ProducerName] = ref.ProducerID
				} else {
					varRef.value = ref.ProducerID
					uuids[ref.ProducerID] = ref.ProducerID
				}
			}
			if len(ref.Transformations) > 0 {
				for _, t := range ref.Transformations {
					if t.Type == "WFPropertyVariableAggrandizement" {
						varRef.getAs = t.PropertyName
					} else if t.Type == "WFCoercionVariableAggrandizement" {
						varRef.coerce = t.CoercionItemClass
					} else if t.Type == "WFDictionaryValueVariableAggrandizement" {
						varRef.getAs = t.DictionaryKey
					}
				}
			}
			return actionArgument{
				valueType: Variable,
				value:     varRef,
			}
		}
		return actionArgument{valueType: Nil, value: nil}
	case backend.ValTextSegments:
		encoded := encodeTextSegments(val.Segments)
		return actionArgument{
			valueType: String,
			value:     encoded,
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

func (s *CanonicalBackendSession) emitRaw(appleIdentifier string, params map[string]any, outputUUID, outputName, groupingID string) (string, error) {
	if params == nil {
		params = make(map[string]any)
	}
	if outputUUID != "" {
		params["UUID"] = outputUUID
	}
	if outputName != "" {
		params["CustomOutputName"] = outputName
	}
	if groupingID != "" {
		params["GroupingIdentifier"] = groupingID
	}
	s.shortcut.WFWorkflowActions = append(s.shortcut.WFWorkflowActions, ShortcutAction{
		WFWorkflowActionIdentifier: appleIdentifier,
		WFWorkflowActionParameters: params,
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
