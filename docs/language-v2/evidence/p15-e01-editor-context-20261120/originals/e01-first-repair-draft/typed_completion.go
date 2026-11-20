package service

import "github.com/electrikmilk/cherri/internal/language/types"

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
