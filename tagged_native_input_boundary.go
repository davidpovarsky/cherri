package main

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// CurrentNativeInputUsesPrivateValue observes the actual canonical sequence,
// including resolved calls and macro expansion. It never edits a raw action or
// assigns a UUID. The enrollment sets come from checked source values, not from
// user Dictionary keys that resemble a compiler carrier.
func (s *CanonicalBackendSession) CurrentNativeInputUsesPrivateValue(outputs, variables map[string]bool, knownOutput func(string) bool) (bool, error) {
	if len(outputs) == 0 && len(variables) == 0 {
		return false, nil
	}
	actions := s.shortcut.WFWorkflowActions
	if len(actions) == 0 {
		return false, nil
	}
	positions := map[string]int{}
	parameterPositions := map[uintptr]int{}
	controlPositions := map[string][]int{}
	controlKinds := map[string]string{}
	controlStack := []string{}
	type nativeScope struct {
		group  string
		branch int
	}
	controlBranches := map[string]int{}
	scopes := make([][]nativeScope, len(actions))
	variableWrites := map[string][]int{}
	for i, action := range actions {
		for _, group := range controlStack {
			scopes[i] = append(scopes[i], nativeScope{group, controlBranches[group]})
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" ||
			action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
			if name, okay := action.WFWorkflowActionParameters["WFVariableName"].(string); okay && name != "" {
				variableWrites[name] = append(variableWrites[name], i)
			}
		}
		if id, ok := action.WFWorkflowActionParameters["UUID"].(string); ok && id != "" {
			if _, duplicate := positions[id]; duplicate {
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: ambiguous native output identity at private-input boundary")
			}
			positions[id] = i
		}
		if action.WFWorkflowActionParameters != nil {
			parameterPositions[reflect.ValueOf(action.WFWorkflowActionParameters).Pointer()] = i
		}
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.conditional", "is.workflow.actions.choosefrommenu",
			"is.workflow.actions.repeat.count", "is.workflow.actions.repeat.each":
			group, okay := action.WFWorkflowActionParameters["GroupingIdentifier"].(string)
			if !okay || group == "" {
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: native control has no group identity")
			}
			mode := fmt.Sprint(action.WFWorkflowActionParameters["WFControlFlowMode"])
			switch mode {
			case "0":
				if _, duplicate := controlKinds[group]; duplicate {
					return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: native control group is reused")
				}
				controlKinds[group] = action.WFWorkflowActionIdentifier
				controlStack = append(controlStack, group)
			case "1", "2":
				if len(controlStack) == 0 || controlStack[len(controlStack)-1] != group ||
					controlKinds[group] != action.WFWorkflowActionIdentifier {
					return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: native control boundaries cross or differ")
				}
				if mode == "1" {
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.count" ||
						action.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.each" ||
						action.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" && len(controlPositions[group]) != 1 {
						return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: native control alternative boundary is invalid")
					}
				} else {
					controlStack = controlStack[:len(controlStack)-1]
				}
			default:
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: native control mode is invalid")
			}
			if mode == "1" {
				controlBranches[group]++
			}
			controlPositions[group] = append(controlPositions[group], i)
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
	type observation struct {
		private bool
		err     error
	}
	active := map[int]bool{}
	memo := map[int]observation{}
	var producer func(int) (bool, error)
	var output func(string) (bool, error)
	var visit func(reflect.Value, int, int) (bool, error)
	var variable func(string, int) (bool, error)
	type variableRead struct {
		name   string
		before int
	}
	variableMemo := map[variableRead]observation{}
	variableActive := map[variableRead]bool{}
	variable = func(name string, before int) (private bool, err error) {
		if name == "" || before < 0 || before >= len(actions) {
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: invalid native variable snapshot")
		}
		read := variableRead{name, before}
		if saved, found := variableMemo[read]; found {
			return saved.private, saved.err
		}
		if variableActive[read] {
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: cyclic native variable snapshot")
		}
		variableActive[read] = true
		defer func() {
			delete(variableActive, read)
			variableMemo[read] = observation{private, err}
		}()
		writes := variableWrites[name]
		if len(writes) == 0 {
			return variables[name], nil
		}
		// A native GetVariable observes its value at that action, not the
		// final value of a source binding or a later helper's assignment.
		for i := sort.SearchInts(writes, before) - 1; i >= 0; i-- {
			index := writes[i]
			writer, reader := scopes[index], scopes[before]
			dominates := len(writer) <= len(reader)
			exclusive := false
			for j := 0; j < len(writer) && j < len(reader); j++ {
				if writer[j] == reader[j] {
					continue
				}
				// Different alternatives of the same control cannot both
				// execute on this read's path. Sequential controls can.
				exclusive = writer[j].group == reader[j].group
				dominates = false
				break
			}
			if exclusive {
				continue
			}
			if !dominates {
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: conditional native variable write has no unique snapshot")
			}
			return producer(index)
		}
		return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: native variable read precedes its established snapshot")
	}
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
	visit = func(value reflect.Value, depth, before int) (bool, error) {
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
					return variable(reference.ProducerName, before)
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
				if private, err := variable(name, before); private || err != nil {
					return private, err
				}
			}
			iter := value.MapRange()
			for iter.Next() {
				found, err := visit(iter.Value(), depth+1, before)
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
				if private, err := variable(name, before); private || err != nil {
					return private, err
				}
			}
			for i := 0; i < value.NumField(); i++ {
				if !value.Field(i).CanInterface() {
					continue
				}
				found, err := visit(value.Field(i), depth+1, before)
				if found || err != nil {
					return found, err
				}
			}
		case reflect.Array, reflect.Slice:
			for i := 0; i < value.Len(); i++ {
				found, err := visit(value.Index(i), depth+1, before)
				if found || err != nil {
					return found, err
				}
			}
		}
		return false, nil
	}
	producer = func(index int) (private bool, err error) {
		if previous, found := memo[index]; found {
			return previous.private, previous.err
		}
		if index < 0 || index >= len(actions) {
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: invalid native input producer")
		}
		if active[index] {
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: cyclic native input alias")
		}
		active[index] = true
		defer func() {
			delete(active, index)
			memo[index] = observation{private, err}
		}()
		action := actions[index]
		parameters := action.WFWorkflowActionParameters
		if parameters != nil && privateParameters[reflect.ValueOf(parameters).Pointer()] {
			return true, nil
		}
		id, _ := parameters["UUID"].(string)
		if outputs[id] {
			return true, nil
		}
		// Pure compiler Text carriers preserve the canonical previous input,
		// including when the carrier is the terminal action of a branch.
		if saved, found := s.literalCarrierInputs[id]; found {
			if saved.failure != nil {
				return false, saved.failure
			}
			if saved.reference == nil {
				return false, nil
			}
			return visit(reflect.ValueOf(saved.reference), 0, index)
		}
		switch action.WFWorkflowActionIdentifier {
		case "is.workflow.actions.nothing":
			return false, nil
		case "is.workflow.actions.comment":
			// A comment has no value result. It cannot sanitize the actual
			// previous native value merely by becoming the last graph node.
			if index == 0 {
				return false, nil
			}
			return producer(index - 1)
		case "is.workflow.actions.getvariable":
			return visit(reflect.ValueOf(parameters["WFVariable"]), 0, index)
		case "is.workflow.actions.setvariable", "is.workflow.actions.appendvariable":
			// Omitted WFInput captures the actual preceding native content;
			// an explicitly empty input remains an explicit clear.
			var inputPrivate bool
			var inputError error
			if input, supplied := parameters["WFInput"]; supplied {
				inputPrivate, inputError = visit(reflect.ValueOf(input), 0, index)
			} else if index > 0 {
				inputPrivate, inputError = producer(index - 1)
			}
			if inputError != nil {
				return false, inputError
			}
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
				name, _ := parameters["WFVariableName"].(string)
				previous, failure := variable(name, index)
				if failure != nil {
					return false, failure
				}
				return previous || inputPrivate, nil
			}
			return inputPrivate, nil
		case "is.workflow.actions.list", "is.workflow.actions.dictionary":
			return visit(reflect.ValueOf(parameters["WFItems"]), 0, index)
		case "is.workflow.actions.conditional", "is.workflow.actions.choosefrommenu",
			"is.workflow.actions.repeat.count", "is.workflow.actions.repeat.each":
			group, _ := parameters["GroupingIdentifier"].(string)
			if fmt.Sprint(parameters["WFControlFlowMode"]) != "2" {
				// Inside a checked branch, an explicit opener input establishes
				// its input kind without inventing a result UUID for the opener.
				start := controlPositions[group][0]
				if input, supplied := actions[start].WFWorkflowActionParameters["WFInput"]; supplied {
					return visit(reflect.ValueOf(input), 0, start)
				}
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.count" {
					if start == 0 {
						return false, nil
					}
					return producer(start - 1)
				}
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: open native control has no established input contract")
			}
			if group == "" {
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: private-input control has no group identity")
			}
			boundaries := controlPositions[group]
			if len(boundaries) < 2 || boundaries[len(boundaries)-1] != index {
				return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: ambiguous native control result boundary")
			}
			start := boundaries[0]
			for j, position := range boundaries {
				p := actions[position].WFWorkflowActionParameters
				mode := "1"
				if j == 0 {
					mode = "0"
				} else if j == len(boundaries)-1 {
					mode = "2"
				}
				if actions[position].WFWorkflowActionIdentifier != action.WFWorkflowActionIdentifier ||
					fmt.Sprint(p["WFControlFlowMode"]) != mode {
					return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: malformed native control result region")
				}
			}
			// Only terminal values of normal branches contribute. A private
			// temporary followed by ordinary Text is not the branch result.
			checkBranch := func(begin, end int) (bool, error) {
				terminal := end - 1
				for terminal > begin && actions[terminal].WFWorkflowActionIdentifier == "is.workflow.actions.comment" {
					terminal--
				}
				if terminal <= begin {
					return visit(reflect.ValueOf(actions[start].WFWorkflowActionParameters["WFInput"]), 0, start)
				}
				return producer(terminal)
			}
			switch action.WFWorkflowActionIdentifier {
			case "is.workflow.actions.conditional":
				if len(boundaries) > 3 {
					return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: conditional has duplicate alternative branches")
				}
				for j := 1; j < len(boundaries); j++ {
					found, failure := checkBranch(boundaries[j-1], boundaries[j])
					if found || failure != nil {
						return found, failure
					}
				}
				if len(boundaries) == 2 {
					return visit(reflect.ValueOf(actions[start].WFWorkflowActionParameters["WFInput"]), 0, start)
				}
			case "is.workflow.actions.choosefrommenu":
				if len(boundaries) < 3 {
					return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: Menu has no source-owned result branches")
				}
				for j := 2; j < len(boundaries); j++ {
					found, failure := checkBranch(boundaries[j-1], boundaries[j])
					if found || failure != nil {
						return found, failure
					}
				}
			default:
				if len(boundaries) != 2 {
					return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: Repeat has unexpected alternative boundaries")
				}
				return checkBranch(start, index)
			}
			return false, nil
		}
		if knownOutput == nil || !knownOutput(action.WFWorkflowActionIdentifier) {
			return false, fmt.Errorf("E_TARGET_VALUE_SHAPE: preceding native action has no established value-output contract")
		}
		return false, nil
	}
	return producer(len(actions) - 1)
}
