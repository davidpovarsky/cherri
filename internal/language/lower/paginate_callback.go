package lower

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Page callbacks retain the same authored AST, typed facts and captured native
// slots. Only the immutable page binding and body-local declarations are scoped.
func (l *Lowerer) lowerPaginateBlock(call *syntax.CallExpr) (interface{}, error) {
	parameter := schema.PaginatePageParameter(call)
	action, found := l.registry.LookupAction("paginate")
	if parameter == nil || !found || l.functions["paginate"] != nil ||
		schema.LoopResultScopedPaginateBody(call, action) == nil {
		return nil, fmt.Errorf("E_PAGINATE_CONTRACT at %s: canonical typed page body required", call.Span)
	}
	if _, shadowed := l.bindings["paginate"]; shadowed {
		return nil, fmt.Errorf("E_PAGINATE_CONTRACT: a lexical binding cannot acquire page callback semantics")
	}
	if l.session == nil {
		return nil, fmt.Errorf("E_PAGINATE_BACKEND: page callbacks require the canonical backend")
	}
	if _, okay := l.session.(backend.ScopedPaginationSession); !okay {
		return nil, fmt.Errorf("E_PAGINATE_BACKEND: page callback backend is unavailable")
	}
	if l.loopResultLocalPlan != nil && !l.loopResultLocalPlan.PageCallbackSourceMatches(call) {
		return nil, fmt.Errorf("E_TARGET_VALUE_SHAPE at %s: page cells require their actual call, parameter and body identities", call.Span)
	}
	body := call.Block.Body
	if err := validateScopedBlockSyntax(body); err != nil {
		return nil, err
	}
	if err := validatePaginateControlExit(body); err != nil {
		return nil, err
	}
	if result := syntax.BlockResultExpression(body); result != nil {
		inferred := l.exprTypes[result]
		if inferred == nil || inferred.Kind() != types.KindVoid {
			return nil, fmt.Errorf("E_PAGINATE_BODY_VOID at %s: page body cannot yield or discard a typed result", result.NodeSpan())
		}
	}
	prefix := "__cherri_page_" + strings.ReplaceAll(l.GenerateUUID(), "-", "")
	callback := backend.ScopedPageCallback{
		InputName: prefix + "_callback_input",
		EmitAction: func(identifier string, parameters map[string]any) error {
			id, _ := parameters["UUID"].(string)
			name, _ := parameters["CustomOutputName"].(string)
			nodeID := id
			if nodeID == "" {
				nodeID = l.GenerateUUID()
			}
			l.emitAction(&ir.NativeActionNode{
				NodeID: nodeID, AppleIdentifier: identifier, Parameters: parameters,
				OutputUUID: id, OutputName: name,
				ExactRawParameters: true, HasExactRawParameters: true,
			})
			return l.emitErr
		},
		Body: func(page backend.Reference) error {
			if page.Kind != backend.RefActionResult || page.ProducerID == "" {
				return fmt.Errorf("E_PAGINATE_PAGE_REFERENCE: original validated page output is required")
			}
			restoreScope, err := l.beginScopedBlockScope(body, prefix)
			if err != nil {
				return err
			}
			defer restoreScope()
			previousBinding, previousStatement := l.currentBindingName, l.inExprStmt
			l.currentBindingName, l.inExprStmt = "", true
			defer func() { l.currentBindingName, l.inExprStmt = previousBinding, previousStatement }()
			old, existed := l.bindings[parameter.Name]
			l.rememberLexicalBinding(parameter.Name, old, existed)
			delete(l.provenanceSymbols, parameter.Name)
			l.bindings[parameter.Name] = BindingRef{
				ExplicitNative: true, OutputUUID: page.ProducerID, OutputName: page.ProducerName,
				Type: types.NewMap(types.Text, types.AnyContent),
			}
			for _, statement := range body.Statements {
				if err := l.lowerStatement(statement); err != nil {
					return err
				}
			}
			return l.emitErr
		},
	}
	return l.lowerCallArguments(call, &callback)
}

// Do not scan nested calls or a binding's value expression: their own typed
// blocks/loop results have independent yield contracts.
func validatePaginateControlExit(statement syntax.Statement) error {
	switch node := statement.(type) {
	case *syntax.BlockStmt:
		for _, child := range node.Statements {
			if err := validatePaginateControlExit(child); err != nil {
				return err
			}
		}
	case *syntax.ReturnStmt, *syntax.YieldStmt:
		return fmt.Errorf("E_PAGINATE_BODY_VOID at %s: page callbacks cannot return or yield a value", statement.NodeSpan())
	case *syntax.IfStmt:
		if err := validatePaginateControlExit(node.ThenBlock); err != nil {
			return err
		}
		if node.ElseBlock != nil {
			return validatePaginateControlExit(node.ElseBlock)
		}
	case *syntax.ForStmt:
		return validatePaginateControlExit(node.BodyBlock)
	case *syntax.RepeatStmt:
		return validatePaginateControlExit(node.BodyBlock)
	case *syntax.MenuStmt:
		for _, item := range node.Cases {
			if err := validatePaginateControlExit(item.BodyBlock); err != nil {
				return err
			}
		}
	case *syntax.ExprStmt:
		switch control := node.Expr.(type) {
		case *syntax.IfStmt:
			return validatePaginateControlExit(control)
		case *syntax.ForStmt:
			return validatePaginateControlExit(control)
		case *syntax.RepeatStmt:
			return validatePaginateControlExit(control)
		case *syntax.MenuStmt:
			return validatePaginateControlExit(control)
		}
	}
	return nil
}
