package main

import (
	"fmt"
	"reflect"
)

// Compiler-only identity: the declared name is never confused with a renamed
// native parameter key. The actual parameter map follows its emitted call node
// through later action insertions, without adding an Apple UUID or metadata key.
type importQuestionBinding struct {
	name          string
	questionIndex int
	question      WFQuestion
	nodeID        string
	wireKey       string
	parameters    map[string]any
}

func (s *CanonicalBackendSession) BindImportQuestion(questionName, wireKey, nodeID string) error {
	for i := range s.importQuestionBindings {
		binding := &s.importQuestionBindings[i]
		if binding.name != questionName {
			continue
		}
		if binding.nodeID != "" {
			return fmt.Errorf("E_SETUP_BINDING: setup %q is already bound", questionName)
		}
		parameters, ok := s.resolvedQuestionNodes[nodeID]
		if !ok || nodeID == "" || wireKey == "" {
			return fmt.Errorf("E_SETUP_BINDING: setup %q has no single emitted native action/parameter", questionName)
		}
		if _, exists := parameters[wireKey]; !exists {
			return fmt.Errorf("E_SETUP_BINDING: setup %q native parameter %q was not emitted", questionName, wireKey)
		}
		binding.nodeID, binding.wireKey, binding.parameters = nodeID, wireKey, parameters
		index, err := s.importQuestionActionIndex(*binding)
		if err != nil {
			return err
		}
		s.shortcut.WFWorkflowImportQuestions[binding.questionIndex].ActionIndex = index
		s.shortcut.WFWorkflowImportQuestions[binding.questionIndex].ParameterKey = wireKey
		binding.question = s.shortcut.WFWorkflowImportQuestions[binding.questionIndex]
		return nil
	}
	return fmt.Errorf("E_SETUP_BINDING: unknown setup declaration %q", questionName)
}

func (s *CanonicalBackendSession) importQuestionActionIndex(binding importQuestionBinding) (int, error) {
	identity := reflect.ValueOf(binding.parameters).Pointer()
	found := -1
	for i, action := range s.shortcut.WFWorkflowActions {
		if action.WFWorkflowActionParameters == nil || reflect.ValueOf(action.WFWorkflowActionParameters).Pointer() != identity {
			continue
		}
		if found != -1 {
			return 0, fmt.Errorf("E_SETUP_BINDING: setup %q emitted action identity is ambiguous", binding.name)
		}
		found = i
	}
	if found < 0 {
		return 0, fmt.Errorf("E_SETUP_BINDING: setup %q emitted action is missing", binding.name)
	}
	if _, ok := s.shortcut.WFWorkflowActions[found].WFWorkflowActionParameters[binding.wireKey]; !ok {
		return 0, fmt.Errorf("E_SETUP_BINDING: setup %q parameter was removed", binding.name)
	}
	return found, nil
}

func (s *CanonicalBackendSession) finalizeImportQuestions() error {
	if len(s.importQuestionBindings) == 0 {
		return nil
	}
	questions := make([]WFQuestion, 0, len(s.importQuestionBindings))
	for _, binding := range s.importQuestionBindings {
		// An unused declaration is diagnosed by the analyzer. It must not point
		// accidentally at action zero or create an unrelated import-time prompt.
		if binding.nodeID == "" {
			continue
		}
		index, err := s.importQuestionActionIndex(binding)
		if err != nil {
			return err
		}
		question := binding.question
		question.ActionIndex, question.ParameterKey = index, binding.wireKey
		questions = append(questions, question)
	}
	s.shortcut.WFWorkflowImportQuestions = questions
	return nil
}
