package lower

import (
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"sort"
)

// A consumed argument/default is a value expression even when the enclosing
// call is a standalone statement. Keep the first nested producer's identity.
func (l *Lowerer) lowerCallValue(expr syntax.Expression) (interface{}, error) {
	previous := l.inExprStmt
	l.inExprStmt = false
	defer func() { l.inExprStmt = previous }()
	return l.lowerExpression(expr)
}

// Evaluate the written argument list before binding values to envelope slots.
// Never lower an omitted function default here: the existing dispatcher owns
// its guarded evaluation in the scope of preceding parameters.
func (l *Lowerer) lowerProvidedFunctionValues(call *syntax.CallExpr) (interface{}, []interface{}, error) {
	var primary interface{}
	var err error
	if call.PrimaryArg != nil {
		primary, err = l.lowerCallArgumentValue(call, -1, call.PrimaryArg)
		if err != nil {
			return nil, nil, err
		}
	}
	named := make([]interface{}, len(call.NamedArgs))
	for index, argument := range call.NamedArgs {
		named[index], err = l.lowerCallArgumentValue(call, index, argument.Value)
		if err != nil {
			return nil, nil, err
		}
	}
	return primary, named, nil
}

// callWritesAfter resolves only actual source-visible assignments in later
// arguments. Pure imported references therefore keep their original native wire.
func (l *Lowerer) callWritesAfter(call *syntax.CallExpr, argumentIndex int) map[string]bool {
	writes := map[string]bool{}
	for _, argument := range call.NamedArgs[argumentIndex+1:] {
		for name := range analysis.OuterAssignedNames(argument.Value) {
			if binding, found := l.bindings[name]; found && binding.IsVariable {
				writes[binding.VariableName] = true
			}
		}
	}
	return writes
}

func (l *Lowerer) lowerCallArgumentValue(call *syntax.CallExpr, argumentIndex int, expr syntax.Expression) (interface{}, error) {
	value, err := l.lowerCallValue(expr)
	if err != nil {
		return nil, err
	}
	writes := l.callWritesAfter(call, argumentIndex)
	if len(writes) > 0 {
		switch expr.(type) {
		case *syntax.FStringExpr, *syntax.ListExpr, *syntax.MapExpr, *syntax.RecordExpr:
			// These aggregate values can contain deferred native references.
			// Reuse the existing typed producer at this argument boundary.
			if captured := l.materializeToAttachment(value); captured != nil {
				return captured, nil
			}
			return nil, fmt.Errorf("cannot preserve argument value before a later assignment at %s", expr.NodeSpan())
		}
	}
	return l.captureCallValue(value, writes), nil
}

func (l *Lowerer) captureCallReference(reference any) *ir.AttachmentToken {
	uuid := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: uuid, AppleIdentifier: "is.workflow.actions.getvariable",
		OutputUUID: uuid,
		Parameters: map[string]interface{}{"WFVariable": reference},
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}
}

func (l *Lowerer) captureCallValue(value interface{}, writes map[string]bool) interface{} {
	if len(writes) == 0 {
		return value
	}
	switch item := value.(type) {
	case *ir.AttachmentToken:
		if item != nil && item.Type == "Variable" && writes[item.OutputName] {
			return l.captureCallReference(item)
		}
	case *backend.Reference:
		if item != nil && item.Kind == backend.RefMutableBinding && writes[item.ProducerName] {
			return l.captureCallReference(item)
		}
	case backend.SemanticValue:
		return l.captureCallSemanticValue(item, writes)
	case []interface{}:
		result := make([]interface{}, len(item))
		for index, child := range item {
			result[index] = l.captureCallValue(child, writes)
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{}, len(item))
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			result[key] = l.captureCallValue(item[key], writes)
		}
		return result
	}
	return value
}

func (l *Lowerer) captureCallSemanticValue(value backend.SemanticValue, writes map[string]bool) backend.SemanticValue {
	if len(writes) == 0 {
		return value
	}
	switch value.Kind {
	case backend.ValReference:
		if reference := value.Ref; reference != nil && reference.Kind == backend.RefMutableBinding && writes[reference.ProducerName] {
			return l.toSemanticValue(l.captureCallReference(reference))
		}
	case backend.ValTextSegments:
		value.Segments = append([]backend.TextSegment(nil), value.Segments...)
		for index := range value.Segments {
			segment := &value.Segments[index]
			if reference := segment.Reference; reference != nil && reference.Kind == backend.RefMutableBinding && writes[reference.ProducerName] {
				segment.Reference = l.toSemanticValue(l.captureCallReference(reference)).Ref
			}
			segment.Value = l.captureCallSemanticValue(segment.Value, writes)
		}
	case backend.ValList:
		value.ListVal = append([]backend.SemanticValue(nil), value.ListVal...)
		for index := range value.ListVal {
			value.ListVal[index] = l.captureCallSemanticValue(value.ListVal[index], writes)
		}
	case backend.ValDict:
		value.DictVal = append([]backend.DictEntry(nil), value.DictVal...)
		for index := range value.DictVal {
			value.DictVal[index].Value = l.captureCallSemanticValue(value.DictVal[index].Value, writes)
		}
	}
	return value
}
