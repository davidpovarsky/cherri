package main

import (
	"fmt"
	"reflect"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// CurrentNativeInputUsesPrivateValue observes the actual canonical sequence,
// including resolved calls and macro expansion. It never edits a raw action or
// assigns a UUID. The enrollment sets come from checked source values, not from
// user Dictionary keys that resemble a compiler carrier.
func (s *CanonicalBackendSession) CurrentNativeInputUsesPrivateValue(outputs, variables map[string]bool) (bool, error) {
	if len(outputs) == 0 && len(variables) == 0 {
		return false, nil
	}
	actions := s.shortcut.WFWorkflowActions
	if len(actions) == 0 {
		return false, nil
	}
	positions := map[string]int{}
	parameterPositions := map[uintptr]int{}
	for i, action := range actions {
		if id, ok := action.WFWorkflowActionParameters["UUID"].(string); ok && id != "" {
			if _, duplicate := positions[id]; duplicate {
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: ambiguous native output identity at private-input boundary")
			}
			positions[id] = i
		}
		if action.WFWorkflowActionParameters != nil {
			parameterPositions[reflect.ValueOf(action.WFWorkflowActionParameters).Pointer()] = i
		}
	}
	privateParameters := map[uintptr]bool{}
	for handle, enrolled := range outputs {
		if !enrolled {
			continue
		}
		if binding := s.nativeActionResults[handle]; binding != nil && binding.params != nil {
			privateParameters[reflect.ValueOf(binding.params).Pointer()] = true
		}
	}
	active := map[int]bool{}
	var producer func(int) (bool, error)
	var output func(string) (bool, error)
	var visit func(reflect.Value, int) (bool, error)
	output = func(id string) (bool, error) {
		if outputs[id] {
			return true, nil
		}
		if binding := s.nativeActionResults[id]; binding != nil && binding.params != nil {
			if index, found := parameterPositions[reflect.ValueOf(binding.params).Pointer()]; found {
				return producer(index)
			}
		}
		if index, found := positions[id]; found {
			return producer(index)
		}
		return false, nil
	}
	visit = func(value reflect.Value, depth int) (bool, error) {
		if depth > 128 {
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: native input reference nesting exceeds the bounded observer")
		}
		if !value.IsValid() {
			return false, nil
		}
		for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return false, nil
			}
			value = value.Elem()
			depth++
			if depth > 128 {
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: native input pointer nesting exceeds the bounded observer")
			}
		}
		if value.CanInterface() {
			if reference, ok := value.Interface().(backend.Reference); ok {
				switch reference.Kind {
				case backend.RefActionResult, backend.RefLoopResult:
					return output(reference.ProducerID)
				case backend.RefMutableBinding:
					return variables[reference.ProducerName], nil
				}
			}
		}
		switch value.Kind() {
		case reflect.Map:
			if value.IsNil() || value.Type().Key().Kind() != reflect.String {
				return false, nil
			}
			key := func(name string) reflect.Value { return reflect.ValueOf(name).Convert(value.Type().Key()) }
			kind, _ := nativeResultString(value.MapIndex(key("Type")))
			switch kind {
			case "ActionOutput":
				id, _ := nativeResultString(value.MapIndex(key("OutputUUID")))
				return output(id)
			case "Variable":
				name, _ := nativeResultString(value.MapIndex(key("VariableName")))
				if name == "" {
					name, _ = nativeResultString(value.MapIndex(key("OutputName")))
				}
				if variables[name] {
					return true, nil
				}
			}
			iter := value.MapRange()
			for iter.Next() {
				found, err := visit(iter.Value(), depth+1)
				if found || err != nil {
					return found, err
				}
			}
		case reflect.Struct:
			// Canonical actions use typed Value/WFTextTokenAttachment as well
			// as imported maps. Resolve the same native descriptor fields in
			// both forms; walking their scalar strings loses the reference.
			kind, _ := nativeResultString(value.FieldByName("Type"))
			switch kind {
			case "ActionOutput":
				id, _ := nativeResultString(value.FieldByName("OutputUUID"))
				return output(id)
			case "Variable":
				name, _ := nativeResultString(value.FieldByName("VariableName"))
				if name == "" {
					name, _ = nativeResultString(value.FieldByName("OutputName"))
				}
				if variables[name] {
					return true, nil
				}
			}
			for i := 0; i < value.NumField(); i++ {
				if !value.Field(i).CanInterface() {
					continue
				}
				found, err := visit(value.Field(i), depth+1)
				if found || err != nil {
					return found, err
				}
			}
		case reflect.Array, reflect.Slice:
			for i := 0; i < value.Len(); i++ {
				found, err := visit(value.Index(i), depth+1)
				if found || err != nil {
					return found, err
				}
			}
		}
		return false, nil
	}
	producer = func(index int) (bool, error) {
		if index < 0 || index >= len(actions) {
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: invalid native input producer")
		}
		if active[index] {
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: cyclic native input alias")
		}
		active[index] = true
		defer delete(active, index)
		action := actions[index]
		parameters := action.WFWorkflowActionParameters
		if parameters != nil && privateParameters[reflect.ValueOf(parameters).Pointer()] {
			return true, nil
		}
		id, _ := parameters["UUID"].(string)
		if outputs[id] {
			return true, nil
		}
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.getvariable":
			return visit(reflect.ValueOf(parameters["WFVariable"]), 0)
		case "is.workflow.actions.setvariable", "is.workflow.actions.appendvariable":
			name, _ := parameters["WFVariableName"].(string)
			if variables[name] {
				return true, nil
			}
			return visit(reflect.ValueOf(parameters["WFInput"]), 0)
		case "is.workflow.actions.list", "is.workflow.actions.dictionary":
			return visit(reflect.ValueOf(parameters["WFItems"]), 0)
		case "is.workflow.actions.conditional", "is.workflow.actions.choosefrommenu",
			"is.workflow.actions.repeat.count", "is.workflow.actions.repeat.each":
			if fmt.Sprint(parameters["WFControlFlowMode"]) != "2" {
				return false, nil
			}
			group, _ := parameters["GroupingIdentifier"].(string)
			if group == "" {
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: private-input control has no group identity")
			}
			// A statement-only closed control can expose a branch result without
			// a named result UUID. Reject private results in that exact region;
			// unrelated earlier source values do not taint a later native call.
			for i := index - 1; i >= 0; i-- {
				p := actions[i].WFWorkflowActionParameters
				if p["GroupingIdentifier"] == group && fmt.Sprint(p["WFControlFlowMode"]) == "0" {
					return false, nil
				}
				id, _ := p["UUID"].(string)
				if outputs[id] {
					return true, nil
				}
				if p != nil && privateParameters[reflect.ValueOf(p).Pointer()] {
					return true, nil
				}
			}
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: private-input control has no matching start")
		}
		return false, nil
	}
	current := actions[len(actions)-1]
	id, _ := current.WFWorkflowActionParameters["UUID"].(string)
	if saved, found := s.literalCarrierInputs[id]; found {
		if saved.failure != nil {
			return false, saved.failure
		}
		if saved.reference == nil {
			return false, nil
		}
		return visit(reflect.ValueOf(saved.reference), 0)
	}
	return producer(len(actions) - 1)
}
