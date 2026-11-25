package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"strings"
)

// Arguments were analyzed once by the ordinary call binder before this hook.
func (a *Analyzer) requireOutputType(call *syntax.CallExpr) types.Type {
	value := call.PrimaryArg
	kind := ""
	nonEmpty := false
	for _, arg := range call.NamedArgs {
		switch arg.Label {
		case "value":
			if value == nil {
				value = arg.Value
			}
		case "type":
			if lit, ok := arg.Value.(*syntax.LiteralExpr); ok {
				kind = strings.ToLower(strings.TrimSpace(lit.Value))
			}
		case "nonEmpty":
			if lit, ok := arg.Value.(*syntax.LiteralExpr); ok {
				nonEmpty = lit.Type == syntax.TokenTrue
			}
		}
	}
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
