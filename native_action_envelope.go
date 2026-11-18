package main

import (
	"encoding/json"
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// rawNativeEnvelope is carried by the ordinary ShortcutAction. It is source
// data from an explicit escape, not import metadata or a semantic overlay.
func (action ShortcutAction) nativeEnvelopeMap() map[string]any {
	result := deepClonePlistMap(action.rawNativeEnvelope)
	result["WFWorkflowActionIdentifier"] = action.WFWorkflowActionIdentifier
	if _, present := result["WFWorkflowActionParameters"]; present {
		result["WFWorkflowActionParameters"] = action.WFWorkflowActionParameters
	}
	return result
}

func (action ShortcutAction) MarshalPlist() (any, error) {
	if action.rawNativeEnvelope != nil {
		return action.nativeEnvelopeMap(), nil
	}
	type plainShortcutAction ShortcutAction
	return plainShortcutAction(action), nil
}

func (action ShortcutAction) MarshalJSON() ([]byte, error) {
	if action.rawNativeEnvelope != nil {
		return json.Marshal(action.nativeEnvelopeMap())
	}
	type plainShortcutAction ShortcutAction
	return json.Marshal(plainShortcutAction(action))
}

func rawLegacyActionEnvelope(arguments []actionArgument) (map[string]any, bool, error) {
	if len(arguments) > 3 {
		return nil, true, fmt.Errorf("NATIVE_ACTION_ENVELOPE_ARGUMENTS: too many arguments")
	}
	if len(arguments) < 3 || arguments[2].value == nil {
		return nil, false, nil
	}
	if arguments[1].value != nil {
		return nil, true, fmt.Errorf("NATIVE_ACTION_ENVELOPE_CONFLICT: parameters and an exact envelope cannot both be supplied")
	}
	raw, ok := arguments[2].value.(rawPlistDict)
	if !ok {
		return nil, true, fmt.Errorf("NATIVE_ACTION_ENVELOPE_EXACT_REQUIRED: envelope requires rawPlist")
	}
	identifier, _ := getArgValue(arguments[0]).(string)
	if _, _, err := backend.ValidateNativeActionEnvelope(identifier, raw.Data); err != nil {
		return nil, true, err
	}
	return deepClonePlistMap(raw.Data), true, nil
}

func (session *CanonicalBackendSession) EmitExactRawActionEnvelope(identifier string, envelope map[string]any) error {
	params, present, err := backend.ValidateNativeActionEnvelope(identifier, envelope)
	if err != nil {
		return err
	}
	if err := session.EmitExactRawAction(identifier, params, present); err != nil {
		return err
	}
	action := &session.shortcut.WFWorkflowActions[len(session.shortcut.WFWorkflowActions)-1]
	action.rawNativeEnvelope = deepClonePlistMap(envelope)
	return nil
}

func hasNativeActionEnvelopeFields(record NativeActionRecord) bool {
	for key := range record.RawAction {
		if key != "WFWorkflowActionIdentifier" && key != "WFWorkflowActionParameters" {
			return true
		}
	}
	return false
}

// Explicit envelopes are validated by both renderers and the canonical emitter.
// The compatibility renderers must not turn malformed raw actions into source
// that would only fail later during compilation.
func exactNativeActionEnvelopeLiteral(record NativeActionRecord) (string, error) {
	if _, _, err := backend.ValidateNativeActionEnvelope(record.Identifier, record.RawAction); err != nil {
		return "", err
	}
	return encodePlistValueToLiteral(record.RawAction)
}
