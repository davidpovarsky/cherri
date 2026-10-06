package lower

import (
	"crypto/rand"
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// BindingRef represents how a variable is stored and referenced in Shortcuts.
type BindingRef struct {
	IsVariable   bool   // true = setvariable/getvariable, false = direct action output token
	OutputUUID   string // for action output
	OutputName   string
	VariableName string // for setvariable
}

// Lowerer transforms a Cherri v2 AST into Native Shortcut IR.
type Lowerer struct {
	registry    *schema.Registry
	workflow    *ir.NativeWorkflow
	bindings    map[string]BindingRef
	uuidCounter int
}

// NewLowerer creates a new AST lowerer.
func NewLowerer(registry *schema.Registry) *Lowerer {
	if registry == nil {
		registry = schema.DefaultRegistry()
	}
	return &Lowerer{
		registry: registry,
		workflow: ir.NewNativeWorkflow(),
		bindings: make(map[string]BindingRef),
	}
}

// GenerateUUID returns a formatted UUID.
func (l *Lowerer) GenerateUUID() string {
	l.uuidCounter++
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08X-%04X-%04X-%04X-%012X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// LowerProgram compiles an AST Program into NativeWorkflow IR.
func (l *Lowerer) LowerProgram(prog *syntax.Program) (*ir.NativeWorkflow, error) {
	// 1. Process declarations (metadata, shortcut header, functions)
	for _, decl := range prog.Declarations {
		switch d := decl.(type) {
		case *syntax.ShortcutDecl:
			l.lowerShortcutHeader(d)
		}
	}

	// 2. Process statements in source order
	for _, stmt := range prog.Statements {
		if err := l.lowerStatement(stmt); err != nil {
			return nil, err
		}
	}

	return l.workflow, nil
}

func (l *Lowerer) lowerShortcutHeader(d *syntax.ShortcutDecl) {
	// Defaults
	l.workflow.IconGlyph = 59789
	l.workflow.IconColor = 4282601983

	if d.Metadata != nil {
		for _, f := range d.Metadata.Fields {
			if f.Name == "icon" {
				if rec, ok := f.Value.(*syntax.RecordExpr); ok {
					for _, rf := range rec.Fields {
						if rf.Name == "color" {
							if lit, ok := rf.Value.(*syntax.LiteralExpr); ok {
								var c int
								fmt.Sscanf(lit.Value, "%d", &c)
								l.workflow.IconColor = c
							}
						}
					}
				}
			}
		}
	}
}

func (l *Lowerer) lowerStatement(stmt syntax.Statement) error {
	switch s := stmt.(type) {
	case *syntax.BindingStmt:
		return l.lowerBinding(s)
	case *syntax.AssignStmt:
		return l.lowerAssign(s)
	case *syntax.ExprStmt:
		_, err := l.lowerExpression(s.Expr)
		return err
	case *syntax.IfStmt:
		return l.lowerIf(s)
	case *syntax.RepeatStmt:
		return l.lowerRepeat(s)
	case *syntax.ForStmt:
		return l.lowerFor(s)
	case *syntax.MenuStmt:
		return l.lowerMenu(s)
	case *syntax.ReturnStmt:
		l.workflow.HasExplicitReturn = true
		if s.Value != nil {
			val, err := l.lowerExpression(s.Value)
			if err != nil {
				return err
			}
			// Emit output action: is.workflow.actions.output
			node := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.output",
				Parameters: map[string]interface{}{
					"WFOutput": val,
				},
			}
			l.workflow.AddAction(node)
		}
		return nil
	case *syntax.BlockStmt:
		for _, bs := range s.Statements {
			if err := l.lowerStatement(bs); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func (l *Lowerer) lowerBinding(b *syntax.BindingStmt) error {
	if b.Value == nil {
		// uninitialized var
		l.bindings[b.Name] = BindingRef{
			IsVariable:   true,
			VariableName: b.Name,
		}
		return nil
	}

	val, err := l.lowerExpression(b.Value)
	if err != nil {
		return err
	}

	if b.Mutable {
		// Emit is.workflow.actions.setvariable
		node := &ir.NativeActionNode{
			NodeID:          l.GenerateUUID(),
			AppleIdentifier: "is.workflow.actions.setvariable",
			Parameters: map[string]interface{}{
				"WFVariableName": b.Name,
				"WFInput":        val,
			},
		}
		l.workflow.AddAction(node)
		l.bindings[b.Name] = BindingRef{
			IsVariable:   true,
			VariableName: b.Name,
		}
	} else {
		// If val is already an action output reference from the last action
		if ref, ok := val.(*ir.AttachmentToken); ok && ref.Type == "ActionOutput" {
			l.bindings[b.Name] = BindingRef{
				IsVariable: false,
				OutputUUID: ref.OutputUUID,
				OutputName: b.Name,
			}
			for _, act := range l.workflow.Actions {
				if act.NodeID == ref.OutputUUID || act.OutputUUID == ref.OutputUUID {
					act.OutputName = b.Name
					break
				}
			}
		} else {
			// Emit a pass-through action: gettext or dictionary or calculation
			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.gettext",
				OutputUUID:      uuid,
				OutputName:      b.Name,
				Parameters: map[string]interface{}{
					"WFTextActionText": val,
				},
			}
			l.workflow.AddAction(node)
			l.bindings[b.Name] = BindingRef{
				IsVariable: false,
				OutputUUID: uuid,
				OutputName: b.Name,
			}
		}
	}
	return nil
}

func (l *Lowerer) lowerAssign(a *syntax.AssignStmt) error {
	val, err := l.lowerExpression(a.Value)
	if err != nil {
		return err
	}

	node := &ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{
			"WFVariableName": a.Name,
			"WFInput":        val,
		},
	}
	l.workflow.AddAction(node)
	return nil
}

func (l *Lowerer) lowerIf(stmt *syntax.IfStmt) error {
	groupUUID := l.GenerateUUID()

	// Condition expression
	condVal, err := l.lowerExpression(stmt.Condition)
	if err != nil {
		return err
	}

	// 1. Begin block (Mode 0)
	beginNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": groupUUID,
			"WFControlFlowMode":  0,
			"WFInput":            condVal,
		},
	}
	l.workflow.AddAction(beginNode)

	// 2. Then block statements
	for _, ts := range stmt.ThenBlock.Statements {
		if err := l.lowerStatement(ts); err != nil {
			return err
		}
	}

	// 3. Else block (Mode 1) if present
	if stmt.ElseBlock != nil {
		elseNode := &ir.NativeActionNode{
			NodeID:             l.GenerateUUID(),
			AppleIdentifier:    "is.workflow.actions.conditional",
			GroupingIdentifier: groupUUID,
			ControlFlowMode:    1,
			Parameters: map[string]interface{}{
				"GroupingIdentifier": groupUUID,
				"WFControlFlowMode":  1,
			},
		}
		l.workflow.AddAction(elseNode)

		if err := l.lowerStatement(stmt.ElseBlock); err != nil {
			return err
		}
	}

	// 4. End block (Mode 2)
	endNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    2,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": groupUUID,
			"WFControlFlowMode":  2,
		},
	}
	l.workflow.AddAction(endNode)
	return nil
}

func (l *Lowerer) lowerRepeat(stmt *syntax.RepeatStmt) error {
	groupUUID := l.GenerateUUID()
	countVal, err := l.lowerExpression(stmt.Count)
	if err != nil {
		return err
	}

	beginNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.repeat.count",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": groupUUID,
			"WFControlFlowMode":  0,
			"WFRepeatCount":      countVal,
		},
	}
	l.workflow.AddAction(beginNode)

	for _, s := range stmt.BodyBlock.Statements {
		if err := l.lowerStatement(s); err != nil {
			return err
		}
	}

	endNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.repeat.count",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    2,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": groupUUID,
			"WFControlFlowMode":  2,
		},
	}
	l.workflow.AddAction(endNode)
	return nil
}

func (l *Lowerer) lowerFor(stmt *syntax.ForStmt) error {
	groupUUID := l.GenerateUUID()
	iterVal, err := l.lowerExpression(stmt.Iterable)
	if err != nil {
		return err
	}

	beginNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.repeat.each",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": groupUUID,
			"WFControlFlowMode":  0,
			"WFInput":            iterVal,
		},
	}
	l.workflow.AddAction(beginNode)

	for _, s := range stmt.BodyBlock.Statements {
		if err := l.lowerStatement(s); err != nil {
			return err
		}
	}

	endNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.repeat.each",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    2,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": groupUUID,
			"WFControlFlowMode":  2,
		},
	}
	l.workflow.AddAction(endNode)
	return nil
}

func (l *Lowerer) lowerMenu(stmt *syntax.MenuStmt) error {
	groupUUID := l.GenerateUUID()
	promptVal, err := l.lowerExpression(stmt.Prompt)
	if err != nil {
		return err
	}

	items := make([]string, len(stmt.Cases))
	for i, c := range stmt.Cases {
		items[i] = c.Label
	}

	beginNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.choosefrommenu",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": groupUUID,
			"WFControlFlowMode":  0,
			"WFMenuPrompt":       promptVal,
			"WFMenuItems":        items,
		},
	}
	l.workflow.AddAction(beginNode)

	for _, c := range stmt.Cases {
		caseNode := &ir.NativeActionNode{
			NodeID:             l.GenerateUUID(),
			AppleIdentifier:    "is.workflow.actions.choosefrommenu",
			GroupingIdentifier: groupUUID,
			ControlFlowMode:    1,
			Parameters: map[string]interface{}{
				"GroupingIdentifier": groupUUID,
				"WFControlFlowMode":  1,
				"WFMenuItemTitle":    c.Label,
			},
		}
		l.workflow.AddAction(caseNode)

		for _, s := range c.BodyBlock.Statements {
			if err := l.lowerStatement(s); err != nil {
				return err
			}
		}
	}

	endNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.choosefrommenu",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    2,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": groupUUID,
			"WFControlFlowMode":  2,
		},
	}
	l.workflow.AddAction(endNode)
	return nil
}

func (l *Lowerer) lowerExpression(expr syntax.Expression) (interface{}, error) {
	if expr == nil {
		return nil, nil
	}

	switch e := expr.(type) {
	case *syntax.LiteralExpr:
		switch e.Type {
		case syntax.TokenString, syntax.TokenRawString:
			return e.Value, nil
		case syntax.TokenNumber:
			var n float64
			fmt.Sscanf(e.Value, "%f", &n)
			return n, nil
		case syntax.TokenTrue:
			return true, nil
		case syntax.TokenFalse:
			return false, nil
		case syntax.TokenNone, syntax.TokenNull:
			return nil, nil
		default:
			return e.Value, nil
		}

	case *syntax.IdentExpr:
		if ref, found := l.bindings[e.Name]; found {
			if ref.IsVariable {
				return &ir.AttachmentToken{
					Type:       "Variable",
					OutputName: ref.VariableName,
				}, nil
			}
			return &ir.AttachmentToken{
				Type:       "ActionOutput",
				OutputUUID: ref.OutputUUID,
				OutputName: ref.OutputName,
			}, nil
		}
		// Built-in / system checks
		if e.Name == "clipboard" {
			return &ir.AttachmentToken{Type: "Clipboard"}, nil
		}
		return e.Name, nil

	case *syntax.FStringExpr:
		return l.lowerFString(e)

	case *syntax.CallExpr:
		return l.lowerCall(e)

	case *syntax.ListExpr:
		list := make([]interface{}, len(e.Elements))
		for i, el := range e.Elements {
			val, err := l.lowerExpression(el)
			if err != nil {
				return nil, err
			}
			list[i] = val
		}
		return list, nil

	case *syntax.MapExpr:
		dict := make(map[string]interface{})
		for _, entry := range e.Entries {
			kVal, _ := l.lowerExpression(entry.Key)
			vVal, err := l.lowerExpression(entry.Value)
			if err != nil {
				return nil, err
			}
			dict[fmt.Sprint(kVal)] = vVal
		}
		return dict, nil

	case *syntax.BinaryExpr:
		// Arithmetic / comparison calculation
		left, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := l.lowerExpression(e.Right)
		if err != nil {
			return nil, err
		}
		opStr := "+"
		switch e.Op {
		case syntax.TokenPlus:
			opStr = "+"
		case syntax.TokenMinus:
			opStr = "-"
		case syntax.TokenStar:
			opStr = "×"
		case syntax.TokenSlash:
			opStr = "÷"
		}
		uuid := l.GenerateUUID()
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.math",
			OutputUUID:      uuid,
			Parameters: map[string]interface{}{
				"WFMathOperation": opStr,
				"WFInput":         left,
				"WFMathOperand":   right,
			},
		}
		l.workflow.AddAction(node)
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
	}

	return nil, nil
}

func (l *Lowerer) lowerFString(f *syntax.FStringExpr) (interface{}, error) {
	var fullText strings.Builder
	attachmentsByRange := make(map[string]interface{})

	for _, part := range f.Parts {
		if !part.IsExpr {
			fullText.WriteString(part.Text)
		} else {
			val, err := l.lowerExpression(part.Expr)
			if err != nil {
				return nil, err
			}

			startUTF16 := len(utf16.Encode([]rune(fullText.String())))
			fullText.WriteString("\uFFFC") // Object replacement char

			rangeKey := fmt.Sprintf("{%d, 1}", startUTF16)
			if tok, ok := val.(*ir.AttachmentToken); ok {
				att := map[string]interface{}{
					"Type": tok.Type,
				}
				if tok.OutputUUID != "" {
					att["OutputUUID"] = tok.OutputUUID
				}
				if tok.OutputName != "" {
					att["OutputName"] = tok.OutputName
				}
				if tok.Type == "Variable" {
					att["VariableName"] = tok.OutputName
				}
				attachmentsByRange[rangeKey] = att
			}
		}
	}

	if len(attachmentsByRange) == 0 {
		return fullText.String(), nil
	}

	return map[string]interface{}{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]interface{}{
			"string":             fullText.String(),
			"attachmentsByRange": attachmentsByRange,
		},
	}, nil
}

func (l *Lowerer) lowerCall(call *syntax.CallExpr) (interface{}, error) {
	var actionName string
	switch c := call.Callee.(type) {
	case *syntax.IdentExpr:
		actionName = c.Name
	case *syntax.MemberExpr:
		if targetIdent, ok := c.Target.(*syntax.IdentExpr); ok && targetIdent.Name == "native" {
			actionName = "native." + c.Property
		} else {
			actionName = c.Property
		}
	default:
		return nil, fmt.Errorf("complex callee not supported in lowering")
	}

	// Check native escape: native.action
	if actionName == "native.action" || actionName == "action" {
		// Native action node
		uuid := l.GenerateUUID()
		node := &ir.NativeActionNode{
			NodeID:     uuid,
			OutputUUID: uuid,
			Parameters: make(map[string]interface{}),
		}
		for _, arg := range call.NamedArgs {
			if arg.Label == "identifier" {
				if lit, ok := arg.Value.(*syntax.LiteralExpr); ok {
					node.AppleIdentifier = lit.Value
				}
			} else if arg.Label == "parameters" {
				val, _ := l.lowerExpression(arg.Value)
				if m, ok := val.(map[string]interface{}); ok {
					node.Parameters = m
				}
			}
		}
		l.workflow.AddAction(node)
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
	}

	actionSchema, ok := l.registry.LookupAction(actionName)
	if !ok {
		return nil, fmt.Errorf("unknown action in lowering: %s", actionName)
	}

	uuid := l.GenerateUUID()
	node := &ir.NativeActionNode{
		NodeID:          uuid,
		AppleIdentifier: actionSchema.AppleIdentifier,
		Parameters:      make(map[string]interface{}),
	}
	if actionSchema.OutputTypeName != "" && actionSchema.OutputTypeName != "Void" {
		node.OutputUUID = uuid
	}

	// 1. Primary argument
	if call.PrimaryArg != nil {
		if primParam, hasPrim := actionSchema.PrimaryParameter(); hasPrim {
			primVal, err := l.lowerExpression(call.PrimaryArg)
			if err != nil {
				return nil, err
			}
			wireKey := primParam.WireKey
			if wireKey == "" {
				wireKey = "WFInput"
			}
			node.Parameters[wireKey] = primVal
		}
	}

	// 2. Named arguments
	for _, nArg := range call.NamedArgs {
		param, exists := actionSchema.ParameterByLabel(nArg.Label)
		if exists {
			val, err := l.lowerExpression(nArg.Value)
			if err != nil {
				return nil, err
			}
			wireKey := param.WireKey
			if wireKey == "" {
				wireKey = param.Label
			}
			node.Parameters[wireKey] = val
		}
	}

	l.workflow.AddAction(node)

	if node.OutputUUID != "" {
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: node.OutputUUID,
		}, nil
	}
	return nil, nil
}
