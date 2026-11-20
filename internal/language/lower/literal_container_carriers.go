package lower

import (
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
)

// Native Dictionary/array text fields are token templates even when the source
// value is an ordinary literal. WorkflowKit consumes an unbound U+FFFC there.
// Reuse the existing plain Text producer and shared text/reference codec only
// after every authored expression in this consumed value has been evaluated.
// RawNative and raw plist maps remain opaque: they are not semantic containers.
func (l *Lowerer) prepareLiteralContainerValue(value backend.SemanticValue, field bool) backend.SemanticValue {
	switch value.Kind {
	case backend.ValString:
		if field && strings.ContainsRune(value.StrVal, '\uFFFC') {
			reference := l.toSemanticValue(l.materializeLiteralTextCarrier(value.StrVal)).Ref
			return backend.SemanticValue{Kind: backend.ValTextSegments, Segments: []backend.TextSegment{{IsExpr: true, Reference: reference}}}
		}
	case backend.ValTextSegments:
		value.Segments = l.prepareLiteralContainerSegments(value.Segments)
	case backend.ValList:
		value.ListVal = append([]backend.SemanticValue(nil), value.ListVal...)
		for index := range value.ListVal {
			value.ListVal[index] = l.prepareLiteralContainerValue(value.ListVal[index], true)
		}
	case backend.ValDict:
		value.DictVal = append([]backend.DictEntry(nil), value.DictVal...)
		for index := range value.DictVal {
			entry := &value.DictVal[index]
			if len(entry.KeySegments) != 0 {
				entry.KeySegments = l.prepareLiteralContainerSegments(entry.KeySegments)
			} else if strings.ContainsRune(entry.Key, '\uFFFC') {
				reference := l.toSemanticValue(l.materializeLiteralTextCarrier(entry.Key)).Ref
				entry.KeySegments = []backend.TextSegment{{IsExpr: true, Reference: reference}}
				// The key is still the exact source literal, so the existing shared
				// codec must keep its literal-key sort instead of computed order.
				entry.KeyHasLiteralValue = true
			}
			entry.Value = l.prepareLiteralContainerValue(entry.Value, true)
		}
	}
	return value
}

func (l *Lowerer) prepareLiteralContainerSegments(segments []backend.TextSegment) []backend.TextSegment {
	result := append([]backend.TextSegment(nil), segments...)
	for index := range result {
		segment := &result[index]
		if !segment.IsExpr && strings.ContainsRune(segment.Text, '\uFFFC') {
			reference := l.toSemanticValue(l.materializeLiteralTextCarrier(segment.Text)).Ref
			*segment = backend.TextSegment{IsExpr: true, Reference: reference}
		}
	}
	return result
}

// Ordinary IR parameters carry a typed SemanticValue explicitly. Do not walk
// arbitrary map[string]any: native.action and native.plist own those bytes.
// Plain helper Text parameters are strings, so helper emission cannot recurse.
func (l *Lowerer) prepareLiteralContainerParameters(node *ir.NativeActionNode) {
	if node.ExactRawEnvelope != nil || node.ExactRawParameters {
		return
	}
	keys := make([]string, 0, len(node.Parameters))
	for key := range node.Parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value, typed := node.Parameters[key].(backend.SemanticValue); typed {
			node.Parameters[key] = l.prepareLiteralContainerValue(value, false)
		}
	}
}

func (l *Lowerer) resolvedCallConsumesRuntimeArguments(call backend.ResolvedCall) bool {
	if policy, available := l.session.(interface {
		ResolvedCallConsumesRuntimeArguments(backend.ResolvedCall) bool
	}); available {
		return policy.ResolvedCallConsumesRuntimeArguments(call)
	}
	return true
}

func (l *Lowerer) prepareResolvedCallLiteralContainers(call *backend.ResolvedCall, consumesRuntimeValues bool) {
	if !consumesRuntimeValues {
		return
	}
	for index := range call.Arguments {
		call.Arguments[index].Value = l.prepareLiteralContainerValue(call.Arguments[index].Value, false)
	}
}

// This is called only for values of the compiler's private function envelopes,
// after all written arguments (or the selected default/return) have executed.
// Keep their existing wrapper/empty-value contracts and reuse each prepared
// value for both the named slot and positional mirror. Native maps are opaque.
func (l *Lowerer) prepareFunctionEnvelopeLiteralText(value interface{}) interface{} {
	switch item := value.(type) {
	case backend.SemanticValue:
		return l.prepareLiteralContainerValue(item, true)
	case string:
		if strings.ContainsRune(item, '\uFFFC') {
			return l.prepareLiteralContainerValue(backend.SemanticValue{Kind: backend.ValString, StrVal: item}, true)
		}
	case []interface{}:
		result := make([]interface{}, len(item))
		for index, child := range item {
			result[index] = l.prepareFunctionEnvelopeLiteralText(child)
		}
		return result
	}
	return value
}
