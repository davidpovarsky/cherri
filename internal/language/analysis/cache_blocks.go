package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func (a *Analyzer) analyzeCacheBlock(call *syntax.CallExpr, scope *Scope, contract *schema.ActionSchema) types.Type {
	if _, shadowed := scope.Lookup("cache"); shadowed {
		a.error("E_CACHE_CONTRACT", call.Span, "a lexical declaration cannot acquire the compiler cache lifecycle")
		return types.Unknown
	}
	a.resolvedActions[call] = contract
	a.setEditorAction(call, contract)
	a.checkTargetVersion(contract, "cache", call.Span)
	if _, err := schema.CacheCallOptions(call); err != nil {
		a.error("E_CACHE_OPTION", call.Span, err.Error())
	}
	if call.Block == nil || call.Block.Body == nil {
		a.error("E_BLOCK_REQUIRED", call.Span, "cache requires a scoped value body")
		return types.Unknown
	}
	result := a.analyzeScopedValueBody(call.Block.Body, scope)
	if schema.CacheValueKind(result) == "" {
		a.error("E_CACHE_VALUE_TYPE", call.Span, "cache supports present source-typed Text, Number, Bool or Date; Optional, JSON, containers and other native objects require a separately qualified persistence codec")
	}
	return result
}
