package main

import (
	"fmt"
	"reflect"
	"strings"
)

// Only the lowerer's actual freshly emitted Text snapshot can enroll here.
// Exact output identity and the complete resolved payload join the source-owned
// call to the canonical sequence. Authored Text or a matching "~" marker alone
// does not enroll an action and cannot restore another source cursor.
func (s *CanonicalBackendSession) BindExactTextSnapshotInput(snapshotUUID string, payload any, referenceUUID, defaultUUID string, knownOutput func(string) bool) error {
	actions := s.shortcut.WFWorkflowActions
	if len(actions) == 0 || snapshotUUID == "" {
		return fmt.Errorf("N03_EXACT_TEXT_SNAPSHOT: missing emitted operand snapshot")
	}
	current := actions[len(actions)-1]
	if current.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		current.WFWorkflowActionParameters["UUID"] != snapshotUUID {
		return fmt.Errorf("N03_EXACT_TEXT_SNAPSHOT: emitted operand identity differs")
	}
	resolved, err := s.resolveNativeActionResultValue(payload)
	if err != nil {
		return err
	}
	expected := normalizeV2RawParamValue(sanitizeRawParamValue(resolved))
	if !reflect.DeepEqual(current.WFWorkflowActionParameters["WFTextActionText"], expected) {
		return fmt.Errorf("N03_EXACT_TEXT_SNAPSHOT: emitted operand payload differs")
	}
	valid := false
	switch text := expected.(type) {
	case string:
		valid = strings.HasPrefix(text, "~")
	case map[string]any:
		body, okay := text["Value"].(map[string]any)
		literal, typed := body["string"].(string)
		valid = text["WFSerializationType"] == "WFTextTokenString" && okay && typed && strings.HasPrefix(literal, "~")
	}
	if !valid {
		return fmt.Errorf("N03_EXACT_TEXT_SNAPSHOT: operand is not a nonempty Text snapshot")
	}
	// The existing canonical cursor machinery includes macro expansion and
	// preserves exact raw producer identities. Defer an unavailable cursor
	// until an omitted native input actually consumes it; explicit inputs and
	// ordinary RHS expressions must not acquire an unrelated input dependency.
	if err := s.bindTextCarrierPreviousInput(snapshotUUID, referenceUUID, defaultUUID, knownOutput); err != nil {
		if s.literalCarrierInputs == nil {
			s.literalCarrierInputs = make(map[string]literalCarrierInputBinding)
		}
		s.literalCarrierInputs[snapshotUUID] = literalCarrierInputBinding{failure: err}
	}
	saved := s.literalCarrierInputs[snapshotUUID]
	saved.exactTextSnapshot = true
	s.literalCarrierInputs[snapshotUUID] = saved
	return nil
}

func (s *CanonicalBackendSession) HasCurrentExactTextSnapshot() bool {
	if len(s.shortcut.WFWorkflowActions) == 0 {
		return false
	}
	current := s.shortcut.WFWorkflowActions[len(s.shortcut.WFWorkflowActions)-1]
	id, _ := current.WFWorkflowActionParameters["UUID"].(string)
	return s.literalCarrierInputs[id].exactTextSnapshot
}
