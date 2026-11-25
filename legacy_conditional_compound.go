package main

import (
	"fmt"
	"strings"
)

// Modern native filters can retain pure already-evaluated inputs. A later
// action, arithmetic expression, dynamic interpolation or control region must
// remain behind its logical guard; evaluating it before the filter is eager.
func legacyConditionalStagesRequireGuards(stages [][]token) bool {
	if len(stages) < 2 {
		return false
	}
	for _, stage := range stages[1:] {
		for _, item := range stage {
			if item.typeof != Variable {
				return true
			}
			switch item.valueType {
			case Integer, Float, Bool, RawString:
			case String:
				value, okay := item.value.(string)
				if !okay || checkInlineVarRegex.MatchString(value) {
					return true
				}
			default:
				return true
			}
		}
	}
	return false
}

func legacyConditionalResultName() string {
	for attempt := 0; ; attempt++ {
		salt := fmt.Sprintf("conditional-result:%d:%d:%d:%d", lineIdx, idx, len(tokens), attempt)
		name := "__cherri_condition_" + strings.ReplaceAll(createUUID(&salt), "-", "")
		if !legacyInlineNameTaken(name) {
			return name
		}
	}
}

// Flat homogeneous Legacy predicates become a linear sequence of guarded
// ordinary conditions. The Boolean carrier is initialized inside the authored
// control location, and each operand's original producer tokens run exactly
// once only while the preceding result still requires that operand.
func lowerLegacyConditionalStages(original WFConditions, stages [][]token) WFConditions {
	if len(original.conditions) < 2 || len(original.conditions) != len(stages) {
		parserError("E_CONDITION_INPUT: compound condition producer stages are incomplete")
	}
	enable := 1
	switch original.WFActionParameterFilterPrefix {
	case conditionFilterPrefixes[And]:
	case conditionFilterPrefixes[Or]:
		enable = 0
	default:
		parserError("E_CONDITION_INPUT: compound condition has no supported logical operator")
	}
	name := legacyConditionalResultName()
	definition := varValue{variableType: "Variable", valueType: Integer, value: enable}
	variables[name] = definition
	if legacyScopedParsing != nil {
		legacyScopedParsing.step.vars[name] = definition
	}
	reference := actionArgument{valueType: Variable, value: varValue{variableType: "Variable", valueType: Integer, value: name}}
	set := func(value int) {
		tokens = append(tokens, token{typeof: Variable, ident: name, valueType: Integer, value: value})
	}
	test := func(value int) WFConditions {
		return WFConditions{
			conditions: []condition{{condition: conditions[Is], arguments: []actionArgument{
				reference, {valueType: Integer, value: value},
			}}},
			WFActionParameterFilterPrefix: -1,
		}
	}
	end := func(group string) {
		tokens = append(tokens, token{typeof: Conditional, ident: group, valueType: EndClosure, value: ""})
	}
	set(enable)
	for i, predicate := range original.conditions {
		guardSalt := fmt.Sprintf("%s:guard:%d", name, i)
		conditionSalt := fmt.Sprintf("%s:predicate:%d", name, i)
		guard, group := createUUID(&guardSalt), createUUID(&conditionSalt)
		tokens = append(tokens, token{typeof: Conditional, ident: guard, valueType: If, value: test(enable)})
		tokens = append(tokens, stages[i]...)
		tokens = append(tokens, token{typeof: Conditional, ident: group, valueType: If, value: WFConditions{
			conditions: []condition{predicate}, WFActionParameterFilterPrefix: -1,
			legacyComparison: original.legacyComparison,
		}})
		set(1)
		tokens = append(tokens, token{typeof: Conditional, ident: group, valueType: Else})
		set(0)
		end(group)
		end(guard)
	}
	return test(1)
}
