package lower

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
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
	if call.Block == nil || call.Block.Body == nil || syntax.BlockResultExpression(call.Block.Body) == nil {
		return nil, fmt.Errorf("E_BLOCK_VALUE_REQUIRED at %s: measure requires an explicit final value", call.Span)
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
	previousBindings := l.bindings
	l.bindings = make(map[string]BindingRef, len(previousBindings))
	for key, value := range previousBindings {
		l.bindings[key] = value
	}
	defer func() {
		l.bindings = previousBindings
		l.currentBindingName, l.inExprStmt = previousBinding, previousStatement
	}()

	// Alpha-renaming is an AST lowering pass, not textual substitution. It makes
	// native variable storage lexical, including nested branches and loop locals.
	// Original nodes/source spans are retained for diagnostics and formatting.
	renamer := scopedBlockRenamer{lowerer: l, prefix: "__cherri_block_" + strings.ReplaceAll(l.GenerateUUID(), "-", "")}
	body, err := renamer.block(call.Block.Body, nil)
	if err != nil {
		return nil, err
	}
	resultExpr := syntax.BlockResultExpression(body)
	// Each source measurement has a distinct timer key even when labels repeat.
	// Iterations reuse their own key sequentially; nested measurements cannot
	// overwrite one another's start time.
	timerLabel := label.Value + ":" + renamer.prefix
	if err := l.emitBlockTimer("debugStartTimer", timerLabel, call); err != nil {
		return nil, err
	}
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
	resultToken := l.materializeToAttachment(result)
	if resultToken == nil {
		return nil, fmt.Errorf("E_BLOCK_VALUE_REQUIRED at %s: the final expression did not produce a native value", resultExpr.NodeSpan())
	}
	if resultToken.Type != "ActionOutput" {
		// Freeze mutable/system values at the body boundary. CurrentDate must not
		// be read again after the timing work, nor may a later assignment retarget
		// the value returned by a completed measurement.
		resultToken = l.emitBlockValueAlias(resultToken)
	}
	if err := l.emitBlockTimer("debugStopTimer", timerLabel, call); err != nil {
		return nil, err
	}
	// The timer's elapsed Number must not become implicit Repeat Results or the
	// next action's input. Get Variable forwards the object without Text/Number
	// coercion. Explicit consumers still reference the original body producer.
	l.emitBlockValueAlias(resultToken)
	return resultToken, nil
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

type scopedBlockRenamer struct {
	lowerer *Lowerer
	prefix  string
	next    int
}

func blockNames(parent map[string]string) map[string]string {
	result := make(map[string]string, len(parent))
	for key, value := range parent {
		result[key] = value
	}
	return result
}

func blockName(name string, names map[string]string) string {
	if replacement, found := names[name]; found {
		return replacement
	}
	return name
}

func (r *scopedBlockRenamer) bind(name string, names map[string]string) string {
	if name == "" || name == "_" {
		return name
	}
	r.next++
	replacement := fmt.Sprintf("%s_%d", r.prefix, r.next)
	names[name] = replacement
	return replacement
}

func (r *scopedBlockRenamer) block(block *syntax.BlockStmt, parent map[string]string) (*syntax.BlockStmt, error) {
	if block == nil {
		return nil, nil
	}
	names := blockNames(parent)
	result := &syntax.BlockStmt{Span: block.Span}
	for _, statement := range block.Statements {
		cloned, err := r.statement(statement, names)
		if err != nil {
			return nil, err
		}
		result.Statements = append(result.Statements, cloned)
	}
	return result, nil
}

func (r *scopedBlockRenamer) statement(statement syntax.Statement, names map[string]string) (syntax.Statement, error) {
	switch s := statement.(type) {
	case *syntax.BindingStmt:
		value, err := r.expression(s.Value, names)
		if err != nil {
			return nil, err
		}
		copy := *s
		copy.Value = value
		copy.Name = r.bind(s.Name, names)
		return &copy, nil
	case *syntax.AssignStmt:
		value, err := r.expression(s.Value, names)
		copy := *s
		copy.Value = value
		copy.Name = blockName(s.Name, names)
		return &copy, err
	case *syntax.ExprStmt:
		value, err := r.expression(s.Expr, names)
		return &syntax.ExprStmt{Span: s.Span, Expr: value}, err
	case *syntax.ReturnStmt:
		value, err := r.expression(s.Value, names)
		return &syntax.ReturnStmt{Span: s.Span, Value: value}, err
	case *syntax.YieldStmt:
		value, err := r.expression(s.Value, names)
		return &syntax.YieldStmt{Span: s.Span, Value: value}, err
	case *syntax.BlockStmt:
		return r.block(s, names)
	case *syntax.IfStmt:
		condition, err := r.expression(s.Condition, names)
		if err != nil {
			return nil, err
		}
		thenBody, err := r.block(s.ThenBlock, names)
		if err != nil {
			return nil, err
		}
		var elseBody syntax.Statement
		if s.ElseBlock != nil {
			elseBody, err = r.statement(s.ElseBlock, blockNames(names))
			if err != nil {
				return nil, err
			}
		}
		return &syntax.IfStmt{Span: s.Span, Modern: s.Modern, Condition: condition, ThenBlock: thenBody, ElseBlock: elseBody}, nil
	case *syntax.ForStmt:
		iterable, err := r.expression(s.Iterable, names)
		if err != nil {
			return nil, err
		}
		locals := blockNames(names)
		copy := *s
		copy.Iterable = iterable
		copy.ItemVar = r.bind(s.ItemVar, locals)
		copy.IndexVar = r.bind(s.IndexVar, locals)
		copy.BodyBlock, err = r.block(s.BodyBlock, locals)
		return &copy, err
	case *syntax.RepeatStmt:
		count, err := r.expression(s.Count, names)
		if err != nil {
			return nil, err
		}
		locals := blockNames(names)
		copy := *s
		copy.Count = count
		copy.IndexVar = r.bind(s.IndexVar, locals)
		copy.BodyBlock, err = r.block(s.BodyBlock, locals)
		return &copy, err
	case *syntax.MenuStmt:
		prompt, err := r.expression(s.Prompt, names)
		if err != nil {
			return nil, err
		}
		copy := *s
		copy.Prompt = prompt
		copy.Cases = make([]syntax.MenuCase, len(s.Cases))
		for index, item := range s.Cases {
			copy.Cases[index] = item
			copy.Cases[index].Value, err = r.expression(item.Value, names)
			if err != nil {
				return nil, err
			}
			copy.Cases[index].BodyBlock, err = r.block(item.BodyBlock, names)
			if err != nil {
				return nil, err
			}
		}
		return &copy, nil
	default:
		return nil, fmt.Errorf("E_BLOCK_STATEMENT at %s: %T has no lexical block contract", statement.NodeSpan(), statement)
	}
}

func (r *scopedBlockRenamer) expression(expression syntax.Expression, names map[string]string) (result syntax.Expression, err error) {
	if expression == nil {
		return nil, nil
	}
	defer func() {
		if result != nil && r.lowerer.enumValues != nil {
			if value, found := r.lowerer.enumValues[expression]; found {
				r.lowerer.enumValues[result] = value
			}
		}
		if result != nil && r.lowerer.exprTypes != nil {
			if inferred, found := r.lowerer.exprTypes[expression]; found {
				r.lowerer.exprTypes[result] = inferred
			}
		}
	}()
	switch e := expression.(type) {
	case *syntax.LiteralExpr:
		copy := *e
		return &copy, nil
	case *syntax.IdentExpr:
		copy := *e
		copy.Name = blockName(e.Name, names)
		return &copy, nil
	case *syntax.EnumMemberExpr:
		copy := *e
		copy.TypeName = blockName(e.TypeName, names)
		return &copy, nil
	case *syntax.UnaryExpr:
		copy := *e
		copy.Operand, err = r.expression(e.Operand, names)
		return &copy, err
	case *syntax.BinaryExpr:
		copy := *e
		copy.Left, err = r.expression(e.Left, names)
		if err != nil {
			return nil, err
		}
		copy.Right, err = r.expression(e.Right, names)
		return &copy, err
	case *syntax.MemberExpr:
		copy := *e
		copy.Target, err = r.expression(e.Target, names)
		return &copy, err
	case *syntax.IndexExpr:
		copy := *e
		copy.Target, err = r.expression(e.Target, names)
		if err != nil {
			return nil, err
		}
		copy.Index, err = r.expression(e.Index, names)
		return &copy, err
	case *syntax.FStringExpr:
		copy := *e
		copy.Parts = append([]syntax.FStringPart(nil), e.Parts...)
		for index, part := range e.Parts {
			if part.IsExpr {
				copy.Parts[index].Expr, err = r.expression(part.Expr, names)
				if err != nil {
					return nil, err
				}
			}
		}
		return &copy, nil
	case *syntax.ListExpr:
		copy := *e
		copy.Elements = make([]syntax.Expression, len(e.Elements))
		for index, item := range e.Elements {
			copy.Elements[index], err = r.expression(item, names)
			if err != nil {
				return nil, err
			}
		}
		return &copy, nil
	case *syntax.MapExpr:
		copy := *e
		copy.Entries = make([]syntax.MapEntry, len(e.Entries))
		for index, item := range e.Entries {
			copy.Entries[index].Key, err = r.expression(item.Key, names)
			if err != nil {
				return nil, err
			}
			copy.Entries[index].Value, err = r.expression(item.Value, names)
			if err != nil {
				return nil, err
			}
		}
		return &copy, nil
	case *syntax.RecordExpr:
		copy := *e
		copy.Fields = append([]syntax.RecordFieldExpr(nil), e.Fields...)
		for index, item := range e.Fields {
			copy.Fields[index].Value, err = r.expression(item.Value, names)
			if err != nil {
				return nil, err
			}
		}
		return &copy, nil
	case *syntax.CallExpr:
		if name, ok := e.Callee.(*syntax.IdentExpr); ok && (name.Name == "rawAction" || name.Name == "action") {
			return nil, fmt.Errorf("E_BLOCK_RAW_NATIVE at %s: raw native calls use absolute variable names and cannot promise lexical block scope", e.Span)
		}
		if target, ok := e.Callee.(*syntax.MemberExpr); ok {
			if module, ok := target.Target.(*syntax.IdentExpr); ok && module.Name == "native" {
				return nil, fmt.Errorf("E_BLOCK_RAW_NATIVE at %s: raw native calls use absolute variable names and cannot promise lexical block scope", e.Span)
			}
		}
		copy := *e
		copy.PrimaryArg, err = r.expression(e.PrimaryArg, names)
		if err != nil {
			return nil, err
		}
		copy.NamedArgs = append([]syntax.NamedArg(nil), e.NamedArgs...)
		for index, argument := range e.NamedArgs {
			copy.NamedArgs[index].Value, err = r.expression(argument.Value, names)
			if err != nil {
				return nil, err
			}
		}
		if e.Block != nil {
			body, blockErr := r.block(e.Block.Body, names)
			if blockErr != nil {
				return nil, blockErr
			}
			copy.Block = &syntax.ScopedBlock{Span: e.Block.Span, Body: body}
		}
		return &copy, nil
	case *syntax.IfStmt, *syntax.ForStmt, *syntax.RepeatStmt, *syntax.MenuStmt:
		statement, err := r.statement(expression.(syntax.Statement), names)
		if err != nil {
			return nil, err
		}
		return statement.(syntax.Expression), nil
	default:
		return nil, fmt.Errorf("E_BLOCK_EXPRESSION at %s: %T has no lexical block contract", expression.NodeSpan(), expression)
	}
}
