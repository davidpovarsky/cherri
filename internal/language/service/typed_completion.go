package service

import (
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Filter only the cursor's own argument value. A nested list/string/expression
// must not borrow the enclosing parameter's type. Types and flow facts come
// from the actual shared analyzer at this exact callee and written slot.
func (s *Service) compatibleCompletionVariables(doc *DocumentState, context enumCallContext) (map[string]types.Type, bool) {
	if doc == nil || doc.Analyzer == nil || context.name == "" || len(context.arguments) == 0 {
		return nil, false
	}
	info, visible, found := doc.Analyzer.EditorCallAt(context.calleeSpan.End.ByteOffset, len(context.arguments)-1)
	if !found {
		return nil, false
	}
	parameters := make([]enumCompletionParameter, len(info.Parameters))
	for i, parameter := range info.Parameters {
		parameters[i] = enumCompletionParameter{name: parameter.Name, label: parameter.Label}
	}
	index, value := activeCallParameter(parameters, info.Primary, context)
	if index < 0 || index >= len(info.Parameters) || !enumValuePrefix(value) {
		return nil, false
	}
	expected := info.Parameters[index].Type
	if expected == nil {
		return nil, false
	}
	result := make(map[string]types.Type)
	for name, actual := range visible {
		if types.HasAssignmentEvidence(actual, expected) {
			result[name] = actual
		}
	}
	return result, true
}

// A primary value binds its actual parameter just as a named value does.
// Exact analyzer call identity also prevents a same-named native action from
// donating labels to an authored function or a qualified module call.
func (s *Service) contextualParameterCompletions(doc *DocumentState, context enumCallContext) []CompletionItem {
	if doc == nil || doc.Analyzer == nil || len(context.arguments) == 0 {
		return nil
	}
	info, _, found := doc.Analyzer.EditorCallAt(context.calleeSpan.End.ByteOffset, len(context.arguments)-1)
	if !found {
		return nil
	}
	parameters := make([]enumCompletionParameter, len(info.Parameters))
	for i, parameter := range info.Parameters {
		parameters[i] = enumCompletionParameter{name: parameter.Name, label: parameter.Label}
	}
	bound := make(map[int]bool)
	for i, argument := range context.arguments {
		label, _ := enumArgumentLabel(argument)
		if i == len(context.arguments)-1 && label == "" {
			break
		}
		prefix := context
		prefix.arguments = context.arguments[:i+1]
		index, _ := activeCallParameter(parameters, info.Primary, prefix)
		if index >= 0 {
			bound[index] = true
		}
	}
	var items []CompletionItem
	for index, parameter := range info.Parameters {
		if bound[index] || parameter.Label == "" {
			continue
		}
		typeName := "AnyContent"
		if parameter.Type != nil {
			typeName = parameter.Type.String()
		}
		items = append(items, CompletionItem{
			Label: parameter.Label, Kind: CompletionKindParameter,
			Detail:     fmt.Sprintf("%s: %s (parameter)", parameter.Label, typeName),
			InsertText: parameter.Label + ": ",
		})
	}
	return items
}
