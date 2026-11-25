package lower

import (
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"sort"
	"strings"
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
type providedFunctionValue struct {
	value, presence interface{}
	supplied        bool
}

func (l *Lowerer) lowerProvidedFunctionValues(fn *syntax.FunctionDecl, call *syntax.CallExpr) ([]providedFunctionValue, error) {
	// Bind identities without executing any expression. The array is indexed
	// by written position (primary, then named/remaining positional arguments).
	slots := make([]int, len(call.NamedArgs)+1)
	for i := range slots {
		slots[i] = -1
	}
	unnamed := 0
	for i, param := range fn.Parameters {
		written := -1
		if i == 0 && call.PrimaryArg != nil {
			written = 0
		}
		if written < 0 {
			for n, argument := range call.NamedArgs {
				if argument.Label != "" && (argument.Label == param.Name || strings.TrimPrefix(param.Name, "_") == argument.Label) {
					written = n + 1
					break
				}
			}
		}
		if written < 0 {
			for unnamed < len(call.NamedArgs) {
				n := unnamed
				unnamed++
				if call.NamedArgs[n].Label == "" {
					written = n + 1
					break
				}
			}
		}
		if written >= 0 {
			if slots[written] >= 0 {
				return nil, fmt.Errorf("ambiguous function argument at %s", call.Span)
			}
			slots[written] = i
		} else if !param.Optional && param.DefaultExpr == nil {
			return nil, fmt.Errorf("missing function argument %q at %s", param.Name, call.Span)
		}
	}
	values := make([]providedFunctionValue, len(fn.Parameters))
	for written, slot := range slots {
		expr := call.PrimaryArg
		if written > 0 {
			expr = call.NamedArgs[written-1].Value
		}
		if expr == nil {
			continue
		}
		if slot < 0 {
			return nil, fmt.Errorf("unbound function argument at %s", expr.NodeSpan())
		}
		value, presence, err := l.lowerFunctionArgumentValue(call, written-1, expr, l.optionalFunctionParameter(fn.Parameters[slot].TypeExpr), l.resolveTypeAnnotation(fn.Parameters[slot].TypeExpr))
		if err != nil {
			return nil, err
		}
		values[slot] = providedFunctionValue{value, presence, true}
	}
	return values, nil
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
	return l.captureCallArgumentValue(expr, value, writes)
}

func (l *Lowerer) captureCallArgumentValue(expr syntax.Expression, value interface{}, writes map[string]bool) (interface{}, error) {
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
			entry := &value.DictVal[index]
			if len(entry.KeySegments) > 0 {
				key := l.captureCallSemanticValue(backend.SemanticValue{Kind: backend.ValTextSegments, Segments: entry.KeySegments}, writes)
				entry.KeySegments = key.Segments
			}
			entry.Value = l.captureCallSemanticValue(entry.Value, writes)
		}
	}
	return value
}
