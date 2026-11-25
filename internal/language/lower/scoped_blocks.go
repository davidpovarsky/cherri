package lower

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// lowerScopedBlockCall adds lexical scope and value flow to an inline compiler
// construct. It uses the canonical backend's existing timer expansion, never a
// second native serializer or a Run Shortcut/function invocation for the body.
func (l *Lowerer) lowerScopedBlockCall(call *syntax.CallExpr) (interface{}, error) {
	name, simple := call.Callee.(*syntax.IdentExpr)
	if !simple || name.Name != "measure" || l.functions[name.Name] != nil {
		return nil, fmt.Errorf("E_BLOCK_UNSUPPORTED at %s: this call has no scoped block lowering contract", call.Span)
	}
	if l.session == nil {
		return nil, fmt.Errorf("E_BLOCK_CANONICAL_BACKEND at %s: scoped blocks require the canonical backend session", call.Span)
	}
	if call.Block == nil || call.Block.Body == nil {
		return nil, fmt.Errorf("E_BLOCK_REQUIRED at %s: measure requires a scoped body", call.Span)
	}
	if l.loopResultLocalPlan != nil && l.loopResultLocalPlan.ScopedBodies[call] != call.Block.Body {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE at %s: scoped loop cells require their original call and body identity", call.Span)
	}
	sourceResult := syntax.BlockResultExpression(call.Block.Body)
	bodyType := l.exprTypes[call]
	voidBody := bodyType != nil && bodyType.Kind() == types.KindVoid
	if voidBody {
		if l.currentBindingName != "" || !l.inExprStmt {
			return nil, fmt.Errorf("E_BLOCK_VOID_VALUE at %s: a Void scoped body has no value to consume", call.Span)
		}
		if sourceResult != nil {
			resultType := l.exprTypes[sourceResult]
			if resultType == nil || resultType.Kind() != types.KindVoid {
				return nil, fmt.Errorf("E_BLOCK_RESULT_TYPE at %s: Void classification does not match the authored result", sourceResult.NodeSpan())
			}
			if _, explicit := call.Block.Body.Statements[len(call.Block.Body.Statements)-1].(*syntax.YieldStmt); explicit {
				return nil, fmt.Errorf("E_BLOCK_VALUE_REQUIRED at %s: a Void action cannot supply an explicit yield", sourceResult.NodeSpan())
			}
		}
	} else if sourceResult == nil {
		return nil, fmt.Errorf("E_BLOCK_RESULT_TYPE at %s: a body without a final value requires analyzed Void semantics", call.Span)
	} else if resultType := l.exprTypes[sourceResult]; resultType != nil && resultType.Kind() == types.KindVoid {
		return nil, fmt.Errorf("E_BLOCK_RESULT_TYPE at %s: a Void tail cannot supply a value result", sourceResult.NodeSpan())
	}
	labelExpr := call.PrimaryArg
	for _, argument := range call.NamedArgs {
		if argument.Label != "label" || labelExpr != nil {
			return nil, fmt.Errorf("E_BLOCK_LABEL at %s: measure accepts exactly one literal label", argument.Span)
		}
		labelExpr = argument.Value
	}
	label, literal := labelExpr.(*syntax.LiteralExpr)
	if !literal || (label.Type != syntax.TokenString && label.Type != syntax.TokenRawString) || strings.TrimSpace(label.Value) == "" {
		return nil, fmt.Errorf("E_BLOCK_LABEL at %s: measure requires a non-empty literal label", call.Span)
	}

	previousBinding, previousStatement := l.currentBindingName, l.inExprStmt
	l.currentBindingName, l.inExprStmt = "", false
	defer func() {
		l.currentBindingName, l.inExprStmt = previousBinding, previousStatement
	}()
	body := call.Block.Body
	prefix := "__cherri_block_" + strings.ReplaceAll(l.GenerateUUID(), "-", "")
	restoreScope, err := l.beginScopedBlockScope(body, prefix)
	if err != nil {
		return nil, err
	}
	defer restoreScope()

	// Keep the authored AST and every source-owned type/presence/cell fact.
	// Only native declaration names change through the shared lexical scope.
	resultExpr := syntax.BlockResultExpression(body)
	// Each measurement has a distinct timer key, including repeated labels.
	timerLabel := label.Value + ":" + prefix
	if voidBody {
		err := backend.EmitScopedMeasureVoid(
			timerLabel,
			func(name, key string) error { return l.emitBlockTimer(name, key, call) },
			func() error {
				for _, statement := range body.Statements {
					if err := l.lowerStatement(statement); err != nil {
						return err
					}
				}
				return nil
			},
			func() error {
				l.emitAction(&ir.NativeActionNode{
					NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.nothing",
				})
				return nil
			},
		)
		return nil, err
	}
	return backend.EmitScopedMeasure(
		timerLabel,
		func(name, key string) error { return l.emitBlockTimer(name, key, call) },
		func() (*ir.AttachmentToken, error) {
			for index, statement := range body.Statements {
				if index == len(body.Statements)-1 {
					break
				}
				if err := l.lowerStatement(statement); err != nil {
					return nil, err
				}
			}
			l.inExprStmt = false
			result, err := l.lowerExpression(resultExpr)
			if err != nil {
				return nil, err
			}
			value := l.materializeToAttachment(result)
			if value == nil {
				return nil, fmt.Errorf("E_BLOCK_VALUE_REQUIRED at %s: the final expression did not produce a native value", resultExpr.NodeSpan())
			}
			return value, nil
		},
		func(value *ir.AttachmentToken) bool { return value.Type == "ActionOutput" },
		func(value *ir.AttachmentToken, final bool) (*ir.AttachmentToken, error) {
			return l.emitBlockValueAlias(value), nil
		},
	)
}

func (l *Lowerer) emitBlockTimer(name, label string, original *syntax.CallExpr) error {
	previousBinding, previousStatement := l.currentBindingName, l.inExprStmt
	l.currentBindingName, l.inExprStmt = "", true
	defer func() { l.currentBindingName, l.inExprStmt = previousBinding, previousStatement }()
	_, err := l.lowerCall(&syntax.CallExpr{
		Span:       original.Span,
		Callee:     &syntax.IdentExpr{Span: original.Span, Name: name},
		PrimaryArg: &syntax.LiteralExpr{Span: original.Span, Type: syntax.TokenString, Value: label},
	})
	return err
}

func (l *Lowerer) emitBlockValueAlias(value *ir.AttachmentToken) *ir.AttachmentToken {
	id := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: id, AppleIdentifier: "is.workflow.actions.getvariable", OutputUUID: id,
		Parameters: map[string]interface{}{"WFVariable": value},
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: id, OutputName: value.OutputName}
}

// A scoped construct uses the same source nodes as ordinary lowering. The
// scope changes native storage names, never AST identities or analysis maps.
func (l *Lowerer) beginScopedBlockScope(body *syntax.BlockStmt, prefix string) (func(), error) {
	if body == nil {
		return nil, fmt.Errorf("E_BLOCK_REQUIRED: scoped body is missing")
	}
	if err := validateScopedBlockSyntax(body); err != nil {
		return nil, err
	}
	restore := l.beginLexicalScope()
	previous := l.lexical.privatePrefix
	l.lexical.privatePrefix = prefix
	return func() {
		l.lexical.privatePrefix = previous
		restore()
	}, nil
}

// Validate the same finite syntax surface before emitting any body effects.
// Absolute native calls cannot promise lexical names. Unknown statement and
// expression forms remain explicit errors; no source node is copied here.
func validateScopedBlockSyntax(body *syntax.BlockStmt) error {
	var result error
	optionalDemandWalk(body, func(node syntax.Node) {
		if result != nil {
			return
		}
		switch value := node.(type) {
		case *syntax.CallExpr:
			if name, ok := value.Callee.(*syntax.IdentExpr); ok && (name.Name == "rawAction" || name.Name == "action") {
				result = fmt.Errorf("E_BLOCK_RAW_NATIVE at %s: raw native calls use absolute variable names and cannot promise lexical block scope", value.Span)
			}
			if target, ok := value.Callee.(*syntax.MemberExpr); ok {
				if module, ok := target.Target.(*syntax.IdentExpr); ok && module.Name == "native" {
					result = fmt.Errorf("E_BLOCK_RAW_NATIVE at %s: raw native calls use absolute variable names and cannot promise lexical block scope", value.Span)
				}
			}
		case *syntax.BindingStmt, *syntax.AssignStmt, *syntax.ExprStmt,
			*syntax.ReturnStmt, *syntax.YieldStmt, *syntax.BlockStmt,
			*syntax.IfStmt, *syntax.ForStmt, *syntax.RepeatStmt, *syntax.MenuStmt,
			*syntax.LiteralExpr, *syntax.IdentExpr, *syntax.EnumMemberExpr,
			*syntax.UnaryExpr, *syntax.BinaryExpr, *syntax.MemberExpr,
			*syntax.IndexExpr, *syntax.FStringExpr, *syntax.ListExpr,
			*syntax.MapExpr, *syntax.RecordExpr:
		default:
			if _, expression := node.(syntax.Expression); expression {
				result = fmt.Errorf("E_BLOCK_EXPRESSION at %s: %T has no lexical block contract", node.NodeSpan(), node)
			} else if _, statement := node.(syntax.Statement); statement {
				result = fmt.Errorf("E_BLOCK_STATEMENT at %s: %T has no lexical block contract", node.NodeSpan(), node)
			}
		}
	})
	return result
}
