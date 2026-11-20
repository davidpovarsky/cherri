package main

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

type literalCarrierInputBinding struct {
	reference *backend.Reference
	failure   error
}

// The canonical emitted sequence includes resolved actions and macro expansion;
// the lowerer's partial IR list cannot establish the previous runtime value.
func (s *CanonicalBackendSession) BindLiteralTextCarrierInput(carrierUUID, referenceUUID, defaultUUID string, knownOutput func(string) bool) error {
	actions := s.shortcut.WFWorkflowActions
	if len(actions) == 0 {
		return fmt.Errorf("S01_LITERAL_CARRIER_INTERNAL: missing emitted literal carrier")
	}
	carrier := actions[len(actions)-1]
	literal, plain := carrier.WFWorkflowActionParameters["WFTextActionText"].(string)
	if carrier.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		carrier.WFWorkflowActionParameters["UUID"] != carrierUUID || !plain || !strings.ContainsRune(literal, '\uFFFC') {
		return fmt.Errorf("S01_LITERAL_CARRIER_INTERNAL: emitted literal carrier identity differs")
	}
	if s.literalCarrierInputs == nil {
		s.literalCarrierInputs = make(map[string]literalCarrierInputBinding)
	}
	if len(actions) == 1 {
		s.literalCarrierInputs[carrierUUID] = literalCarrierInputBinding{reference: &backend.Reference{Kind: backend.RefExtensionInput}}
		return nil
	}
	index := len(actions) - 2
	previous := actions[index]
	if id, ok := previous.WFWorkflowActionParameters["UUID"].(string); ok {
		if saved, found := s.literalCarrierInputs[id]; found {
			s.literalCarrierInputs[carrierUUID] = saved
			return nil
		}
	}
	if previous.WFWorkflowActionIdentifier == "is.workflow.actions.nothing" {
		s.literalCarrierInputs[carrierUUID] = literalCarrierInputBinding{}
		return nil
	}
	switch previous.WFWorkflowActionIdentifier {
	case "is.workflow.actions.conditional", "is.workflow.actions.choosefrommenu",
		"is.workflow.actions.repeat.count", "is.workflow.actions.repeat.each":
		if fmt.Sprint(previous.WFWorkflowActionParameters["WFControlFlowMode"]) != "2" {
			s.literalCarrierInputs[carrierUUID] = literalCarrierInputBinding{failure: fmt.Errorf("S01_IMPLICIT_INPUT_UNAVAILABLE: an open control boundary has no proven result identity; supply an explicit native input")}
			return nil
		}
	}
	// Use the caller's existing checked registry, not UUID presence or a second
	// native output catalog, to establish a value-producing contract.
	if knownOutput == nil || !knownOutput(previous.WFWorkflowActionIdentifier) {
		s.literalCarrierInputs[carrierUUID] = literalCarrierInputBinding{failure: fmt.Errorf("S01_IMPLICIT_INPUT_UNAVAILABLE: preceding action has no established value-output contract; supply an explicit native input")}
		return nil
	}
	exact := s.isExactNativeParameters(previous.WFWorkflowActionParameters)
	if err := s.bindNativeActionResultAt(index, referenceUUID, defaultUUID, exact, false, ""); err != nil {
		return err
	}
	s.literalCarrierInputs[carrierUUID] = literalCarrierInputBinding{reference: &backend.Reference{Kind: backend.RefActionResult, ProducerID: referenceUUID}}
	return nil
}

// Return a snapshot only while the actual current action is an owned pure
// carrier. Later authored actions establish their own input/output boundary.
func (s *CanonicalBackendSession) CurrentLiteralCarrierInput() (*backend.Reference, bool, error) {
	if len(s.shortcut.WFWorkflowActions) == 0 {
		return nil, false, nil
	}
	current := s.shortcut.WFWorkflowActions[len(s.shortcut.WFWorkflowActions)-1]
	id, _ := current.WFWorkflowActionParameters["UUID"].(string)
	saved, found := s.literalCarrierInputs[id]
	if !found {
		return nil, false, nil
	}
	if saved.failure != nil {
		return nil, true, saved.failure
	}
	if saved.reference == nil {
		return nil, true, nil
	}
	copy := *saved.reference
	return &copy, true, nil
}
