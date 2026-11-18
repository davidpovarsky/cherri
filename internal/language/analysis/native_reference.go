package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// A raw native operand is a deliberate escape from scalar inference, but only
// a statically validated reference shape may bypass the ordinary Number/Date
// requirement. The lowerer uses these same IR validators before emission.
func (a *Analyzer) validNativeReferenceOperand(call *syntax.CallExpr, scope *Scope) bool {
	var kind, uuid, name string
	hasBase := false
	if call.PrimaryArg != nil {
		var literal bool
		kind, literal = literalTextValue(call.PrimaryArg)
		if !literal {
			hasBase = a.knownNativeReferenceTarget(call.PrimaryArg, scope)
			if !hasBase {
				return false
			}
		}
	}

	for _, argument := range call.NamedArgs {
		switch argument.Label {
		case "target", "ref", "symbol":
			if !a.knownNativeReferenceTarget(argument.Value, scope) {
				return false
			}
			hasBase = true
		case "kind", "type", "uuid", "outputUUID", "name", "variable", "outputName", "prompt",
			"coerce", "coercion", "property", "key", "dictionaryKey":
			value, known := literalTextValue(argument.Value)
			if !known {
				return false
			}
			switch argument.Label {
			case "kind", "type":
				kind = value
			case "uuid", "outputUUID":
				uuid = value
			case "name", "variable", "outputName", "prompt":
				name = value
			}
		case "aggrandizements":
			value, known := a.nativeReferenceLiteralShape(argument.Value, scope)
			if !known {
				return false
			}
			if _, err := ir.NativeReferenceAggrandizements(value); err != nil {
				return false
			}
		case "propertyUserInfo":
			if _, known := a.nativeReferenceLiteralShape(argument.Value, scope); !known {
				return false
			}
		default:
			// Do not infer a reference contract from ignored or unknown labels.
			return false
		}
	}
	if hasBase && kind == "" {
		return true
	}
	_, err := ir.NormalizeNativeReferenceKind(kind, uuid, name)
	return err == nil
}

func (a *Analyzer) knownNativeReferenceTarget(expression syntax.Expression, scope *Scope) bool {
	if a.nativeReferenceCalls[expression] {
		return true
	}
	identifier, ok := expression.(*syntax.IdentExpr)
	if !ok {
		return false
	}
	symbol, found := scope.Lookup(identifier.Name)
	return found && symbol.Initialized && symbol.nativeReferenceTarget
}

// This inspects literal container shape only. It does not serialize or lower
// user data, interpret property semantics, or evaluate a runtime expression.
// The existing plist decoder supplies literal imported metadata when available.
func (a *Analyzer) nativeReferenceLiteralShape(expression syntax.Expression, scope *Scope) (any, bool) {
	switch value := expression.(type) {
	case *syntax.LiteralExpr:
		return value.Value, true
	case *syntax.ListExpr:
		items := make([]any, len(value.Elements))
		for index, element := range value.Elements {
			item, known := a.nativeReferenceLiteralShape(element, scope)
			if !known {
				return nil, false
			}
			items[index] = item
		}
		return items, true
	case *syntax.MapExpr:
		object := make(map[string]any, len(value.Entries))
		for _, entry := range value.Entries {
			key, knownKey := literalTextValue(entry.Key)
			item, knownValue := a.nativeReferenceLiteralShape(entry.Value, scope)
			if !knownKey || !knownValue {
				return nil, false
			}
			object[key] = item
		}
		return object, true
	case *syntax.CallExpr:
		member, ok := value.Callee.(*syntax.MemberExpr)
		if !ok || member.Property != "plist" {
			return nil, false
		}
		namespace, ok := member.Target.(*syntax.IdentExpr)
		if !ok || namespace.Name != "native" {
			return nil, false
		}
		symbol, found := scope.Lookup(namespace.Name)
		if !found || symbol != a.nativeNamespace {
			return nil, false
		}
		target := value.PrimaryArg
		for _, argument := range value.NamedArgs {
			if target == nil && (argument.Label == "data" || argument.Label == "plist" || argument.Label == "value" || argument.Label == "") {
				target = argument.Value
			} else {
				return nil, false
			}
		}
		if target == nil {
			return nil, false
		}
		if literal, ok := literalTextValue(target); ok {
			dictionary := decodePlistDictForAnalysis(literal)
			return dictionary, dictionary != nil
		}
		return a.nativeReferenceLiteralShape(target, scope)
	}
	return nil, false
}
