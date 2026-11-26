package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"strings"
)

// Reuse the canonical binder's exact expressions: named and positional options
// have the same literal rules, without analyzing or evaluating any argument twice.
func (a *Analyzer) requireOutputType(call *syntax.CallExpr, arguments map[string]syntax.Expression) types.Type {
	value := arguments["WFInput"]
	kind, _ := literalTextValue(arguments["WFRequireType"])
	kind = strings.ToLower(strings.TrimSpace(kind))
	nonEmpty, _ := literalBoolValue(arguments["WFRequireNonEmpty"])
	if value == nil {
		return types.Unknown
	}
	t := a.exprTypes[value]
	if t == nil {
		return types.Unknown
	}
	if kind == "" && !nonEmpty && !types.Bool.Equals(t) {
		a.error(CodeArgumentType, value.NodeSpan(), "require condition must be a source-proved Bool; other values need a supported type guard or nonEmpty:true")
	}
	if optional, ok := t.(types.OptionalType); ok && (kind != "" || nonEmpty) {
		t = optional.Inner
	}
	if kind != "" && kind != "text" && kind != "number" && kind != "dictionary" {
		a.error(CodeArgumentType, call.Span, "require: unsupported native type guard "+kind+"; supported categories: text, number, dictionary")
	}
	var expected types.Type
	switch kind {
	case "text":
		expected = types.Text
	case "number":
		expected = types.Number
	case "dictionary":
		expected = types.NewMap(types.Text, types.AnyContent)
	}
	if expected != nil {
		known := t.Kind() != types.KindAnyContent && t.Kind() != types.KindUnknown && t.Kind() != types.KindJsonValue && t.Kind() != types.KindJsonNull
		compatible := t.Equals(expected) || (kind == "dictionary" && (t.Kind() == types.KindMap || t.Kind() == types.KindRecord))
		if known && !compatible {
			a.error(CodeArgumentType, value.NodeSpan(), "require type guard "+kind+" does not match the source value type "+t.String()+"; require does not coerce its input")
		}
		if !known {
			return expected
		}
	}
	return t
}
