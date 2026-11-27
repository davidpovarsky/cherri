package lower

import (
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"strings"
)

func (l *Lowerer) lowerCacheBlock(call *syntax.CallExpr) (any, error) {
	if l.functions["cache"] != nil {
		return nil, fmt.Errorf("E_CACHE_CONTRACT: a user function cannot acquire the cache lifecycle")
	}
	options, err := schema.CacheCallOptions(call)
	if err != nil {
		return nil, err
	}
	if l.session == nil {
		return nil, fmt.Errorf("E_BLOCK_CANONICAL_BACKEND: cache requires the canonical backend session")
	}
	if call.Block == nil || call.Block.Body == nil {
		return nil, fmt.Errorf("E_BLOCK_REQUIRED: cache requires a scoped value body")
	}
	body := call.Block.Body
	if l.loopResultLocalPlan != nil && l.loopResultLocalPlan.ScopedBodies[call] != body {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE: cached loop cells require their original call and body identity")
	}
	resultExpr := syntax.BlockResultExpression(body)
	kind := schema.CacheValueKind(l.exprTypes[call])
	if resultExpr == nil || kind == "" || schema.CacheValueKind(l.exprTypes[resultExpr]) != kind {
		return nil, fmt.Errorf("E_CACHE_VALUE_TYPE: cache result requires a matching present persistent source type")
	}
	previousBinding, previousStatement := l.currentBindingName, l.inExprStmt
	l.currentBindingName, l.inExprStmt = "", false
	defer func() { l.currentBindingName, l.inExprStmt = previousBinding, previousStatement }()
	prefix := "__cherri_cache_" + strings.ReplaceAll(l.GenerateUUID(), "-", "")
	restore, err := l.beginScopedBlockScope(body, prefix)
	if err != nil {
		return nil, err
	}
	defer restore()
	result, err := backend.EmitScopedCache(
		backend.ScopedCacheConfig{Namespace: options.Namespace(kind), Kind: kind, TTL: options.TTL, Global: options.Global},
		prefix, l.GenerateUUID,
		func(id string, params map[string]any, uuid, name string) error {
			nodeID := uuid
			if nodeID == "" {
				nodeID = l.GenerateUUID()
			}
			l.emitAction(&ir.NativeActionNode{NodeID: nodeID, AppleIdentifier: id, Parameters: params, OutputUUID: uuid, OutputName: name})
			return l.emitErr
		},
		func() (*backend.Reference, error) {
			for index, statement := range body.Statements {
				if index == len(body.Statements)-1 {
					break
				}
				if err := l.lowerStatement(statement); err != nil {
					return nil, err
				}
			}
			value, err := l.lowerExpression(resultExpr)
			if err != nil {
				return nil, err
			}
			token := l.materializeToAttachment(value)
			if token == nil {
				return nil, fmt.Errorf("E_CACHE_VALUE_TYPE: cache body has no native scalar value")
			}
			switch token.Type {
			case "ActionOutput", "Variable", "CurrentDate":
				// The authored source type may depend on reference coercions.
				// Preserve them through the existing canonical adapter.
				return l.toSemanticValue(token).Ref, nil
			default:
				return nil, fmt.Errorf("E_CACHE_VALUE_TYPE: unsupported native scalar source reference %q", token.Type)
			}
		},
	)
	if err != nil {
		return nil, err
	}
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: result.ProducerID, OutputName: result.ProducerName}, nil
}
