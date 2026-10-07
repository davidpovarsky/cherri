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
	registry       *schema.Registry
	workflow       *ir.NativeWorkflow
	bindings       map[string]BindingRef
	functions      map[string]*syntax.FunctionDecl
	setupQuestions map[string]*syntax.SetupDecl
	WorkflowName   string
	uuidCounter    int
}

// NewLowerer creates a new AST lowerer.
func NewLowerer(registry *schema.Registry) *Lowerer {
	if registry == nil {
		registry = schema.DefaultRegistry()
	}
	return &Lowerer{
		registry:       registry,
		workflow:       ir.NewNativeWorkflow(),
		bindings:       make(map[string]BindingRef),
		functions:      make(map[string]*syntax.FunctionDecl),
		setupQuestions: make(map[string]*syntax.SetupDecl),
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
	// 1. Process declarations (metadata, shortcut header, functions, setup)
	for _, decl := range prog.Declarations {
		switch d := decl.(type) {
		case *syntax.ShortcutDecl:
			l.lowerShortcutHeader(d)
		case *syntax.FunctionDecl:
			l.functions[d.Name] = d
		case *syntax.SetupDecl:
			l.setupQuestions[d.Name] = d
			prompt := d.Prompt
			if prompt == "" {
				prompt = "Enter " + d.Name
			}
			q := map[string]interface{}{
				"Category":     "Parameter",
				"ParameterKey": d.Name,
				"Text":         prompt,
			}
			if d.DefaultValue != "" {
				q["DefaultValue"] = d.DefaultValue
			}
			l.workflow.ImportQuestions = append(l.workflow.ImportQuestions, q)
		}
	}

	// 2. If functions are declared, isolate dispatcher in Mode 0 and main statements in Mode 1 (Otherwise)
	if len(l.functions) > 0 {
		return l.lowerProgramWithFunctions(prog)
	}

	// 3. Process statements in source order
	for _, stmt := range prog.Statements {
		if err := l.lowerStatement(stmt); err != nil {
			return nil, err
		}
	}

	return l.workflow, nil
}

func (l *Lowerer) lowerShortcutHeader(d *syntax.ShortcutDecl) {
	if d.Name != "" {
		l.WorkflowName = d.Name
		l.workflow.Name = d.Name
	}
	// Defaults
	l.workflow.IconGlyph = 59789
	l.workflow.IconColor = 4282601983

	if d.Metadata != nil {
		for _, f := range d.Metadata.Fields {
			switch f.Name {
			case "icon":
				if rec, ok := f.Value.(*syntax.RecordExpr); ok {
					for _, rf := range rec.Fields {
						if rf.Name == "color" {
							if lit, ok := rf.Value.(*syntax.LiteralExpr); ok {
								var c int
								fmt.Sscanf(lit.Value, "%d", &c)
								l.workflow.IconColor = c
							}
						} else if rf.Name == "glyph" {
							if lit, ok := rf.Value.(*syntax.LiteralExpr); ok {
								var g int
								fmt.Sscanf(lit.Value, "%d", &g)
								l.workflow.IconGlyph = g
							}
						}
					}
				} else if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
					var g int
					fmt.Sscanf(lit.Value, "%d", &g)
					l.workflow.IconGlyph = g
				}
			case "targets":
				if list, ok := f.Value.(*syntax.ListExpr); ok {
					var targets []string
					for _, el := range list.Elements {
						if lit, ok := el.(*syntax.LiteralExpr); ok {
							targets = append(targets, lit.Value)
						}
					}
					if len(targets) > 0 {
						l.workflow.WorkflowTypes = targets
					}
				}
			case "from":
				var fromList []string
				if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
					fromList = append(fromList, lit.Value)
				} else if ident, ok := f.Value.(*syntax.IdentExpr); ok {
					fromList = append(fromList, ident.Name)
				} else if list, ok := f.Value.(*syntax.ListExpr); ok {
					for _, el := range list.Elements {
						if lit, ok := el.(*syntax.LiteralExpr); ok {
							fromList = append(fromList, lit.Value)
						} else if ident, ok := el.(*syntax.IdentExpr); ok {
							fromList = append(fromList, ident.Name)
						}
					}
				}
				l.workflow.WorkflowTypes = nil
				for _, surface := range fromList {
					switch surface {
					case "sharesheet":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "ActionExtension")
					case "menubar":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "MenuBar")
					case "quickactions":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "QuickActions")
					case "notifications":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "NCWidget")
					case "watch":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "Watch")
					case "search":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "WFWorkflowTypeShowInSearch")
					case "spotlight":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "WFWorkflowTypeReceivesInputFromSearch")
					case "sleepmode":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "Sleep")
					case "onscreen":
						l.workflow.WorkflowTypes = append(l.workflow.WorkflowTypes, "ReceivesOnScreenContent")
					}
				}
			case "inputs":
				var inputList []string
				if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
					inputList = append(inputList, lit.Value)
				} else if ident, ok := f.Value.(*syntax.IdentExpr); ok {
					inputList = append(inputList, ident.Name)
				} else if list, ok := f.Value.(*syntax.ListExpr); ok {
					for _, el := range list.Elements {
						if lit, ok := el.(*syntax.LiteralExpr); ok {
							inputList = append(inputList, lit.Value)
						} else if ident, ok := el.(*syntax.IdentExpr); ok {
							inputList = append(inputList, ident.Name)
						}
					}
				}
				l.workflow.InputContentItemClasses = nil
				for _, item := range inputList {
					switch item {
					case "text":
						l.workflow.InputContentItemClasses = append(l.workflow.InputContentItemClasses, "WFStringContentItem")
					case "url":
						l.workflow.InputContentItemClasses = append(l.workflow.InputContentItemClasses, "WFURLContentItem")
					case "file":
						l.workflow.InputContentItemClasses = append(l.workflow.InputContentItemClasses, "WFGenericFileContentItem")
					case "image":
						l.workflow.InputContentItemClasses = append(l.workflow.InputContentItemClasses, "WFImageContentItem")
					default:
						l.workflow.InputContentItemClasses = append(l.workflow.InputContentItemClasses, item)
					}
				}
			}
		}
	}
}

func (l *Lowerer) lowerProgramWithFunctions(prog *syntax.Program) (*ir.NativeWorkflow, error) {
	l.workflow.HasShortcutInputVariables = true

	// 1. Initialize _cherri_is_fn = 0.0
	initNumUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          initNumUUID,
		AppleIdentifier: "is.workflow.actions.number",
		OutputUUID:      initNumUUID,
		Parameters: map[string]interface{}{
			"WFNumberActionNumber": 0.0,
		},
	})
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{
			"WFVariableName": "_cherri_is_fn",
			"WFInput":        &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: initNumUUID},
		},
	})

	inputToken := &ir.AttachmentToken{
		Type:       "ExtensionInput",
		OutputName: "ShortcutInput",
	}

	// 2. Guard: Only check function payload if ShortcutInput has a value
	hasInputGroupUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: hasInputGroupUUID,
		ControlFlowMode:    0,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": hasInputGroupUUID,
			"WFControlFlowMode":  0,
			"WFInput":            inputToken,
			"WFCondition":        100, // Has Any Value
		},
	})

	dictUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          dictUUID,
		AppleIdentifier: "is.workflow.actions.detect.dictionary",
		OutputUUID:      dictUUID,
		Parameters: map[string]interface{}{
			"WFInput": inputToken,
		},
	})

	chkUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          chkUUID,
		AppleIdentifier: "is.workflow.actions.getvalueforkey",
		OutputUUID:      chkUUID,
		Parameters: map[string]interface{}{
			"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: dictUUID},
			"WFDictionaryKey":          "cherri_functions",
			"WFGetDictionaryValueType": "Value",
		},
	})

	chkGroupUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: chkGroupUUID,
		ControlFlowMode:    0,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": chkGroupUUID,
			"WFControlFlowMode":  0,
			"WFInput":            &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: chkUUID},
			"WFCondition":        4, // Is
			"WFNumberValue":      1.0,
		},
	})

	setOneNumUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          setOneNumUUID,
		AppleIdentifier: "is.workflow.actions.number",
		OutputUUID:      setOneNumUUID,
		Parameters: map[string]interface{}{
			"WFNumberActionNumber": 1.0,
		},
	})
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{
			"WFVariableName": "_cherri_is_fn",
			"WFInput":        &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: setOneNumUUID},
		},
	})

	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: chkGroupUUID,
		ControlFlowMode:    2,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": chkGroupUUID,
			"WFControlFlowMode":  2,
		},
	})

	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: hasInputGroupUUID,
		ControlFlowMode:    2,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": hasInputGroupUUID,
			"WFControlFlowMode":  2,
		},
	})

	// 3. Main branching conditional:
	// Mode 0: If _cherri_is_fn == 1.0 -> Function dispatcher
	// Mode 1: Otherwise -> Main program statements
	// Mode 2: End If
	mainGroupUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: mainGroupUUID,
		ControlFlowMode:    0,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": mainGroupUUID,
			"WFControlFlowMode":  0,
			"WFInput":            &ir.AttachmentToken{Type: "Variable", OutputName: "_cherri_is_fn"},
			"WFCondition":        4, // Is
			"WFNumberValue":      1.0,
		},
	})

	// Mode 0: Function Dispatcher
	fnDictUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          fnDictUUID,
		AppleIdentifier: "is.workflow.actions.detect.dictionary",
		OutputUUID:      fnDictUUID,
		Parameters: map[string]interface{}{
			"WFInput": inputToken,
		},
	})

	fnNameUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          fnNameUUID,
		AppleIdentifier: "is.workflow.actions.getvalueforkey",
		OutputUUID:      fnNameUUID,
		Parameters: map[string]interface{}{
			"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: fnDictUUID},
			"WFDictionaryKey":          "function",
			"WFGetDictionaryValueType": "Value",
		},
	})

	argsUUID := l.GenerateUUID()
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          argsUUID,
		AppleIdentifier: "is.workflow.actions.getvalueforkey",
		OutputUUID:      argsUUID,
		Parameters: map[string]interface{}{
			"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: fnDictUUID},
			"WFDictionaryKey":          "arguments",
			"WFGetDictionaryValueType": "Value",
		},
	})

	for _, fn := range l.functions {
		fnGroupUUID := l.GenerateUUID()
		l.workflow.AddAction(&ir.NativeActionNode{
			NodeID:             l.GenerateUUID(),
			AppleIdentifier:    "is.workflow.actions.conditional",
			GroupingIdentifier: fnGroupUUID,
			ControlFlowMode:    0,
			Parameters: map[string]interface{}{
				"GroupingIdentifier":        fnGroupUUID,
				"WFControlFlowMode":         0,
				"WFInput":                   &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: fnNameUUID},
				"WFCondition":               4, // Is
				"WFConditionalActionString": fn.Name,
			},
		})

		savedBindings := make(map[string]BindingRef)
		for k, v := range l.bindings {
			savedBindings[k] = v
		}

		for idx, param := range fn.Parameters {
			argItemUUID := l.GenerateUUID()
			l.workflow.AddAction(&ir.NativeActionNode{
				NodeID:          argItemUUID,
				AppleIdentifier: "is.workflow.actions.getitemfromlist",
				OutputUUID:      argItemUUID,
				Parameters: map[string]interface{}{
					"WFInput":         &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: argsUUID},
					"WFItemSpecifier": "Item At Index",
					"WFItemIndex":     float64(idx + 1),
				},
			})
			l.bindings[param.Name] = BindingRef{
				IsVariable: false,
				OutputUUID: argItemUUID,
				OutputName: param.Name,
			}
		}

		hasReturn := false
		for _, stmt := range fn.Body.Statements {
			if _, ok := stmt.(*syntax.ReturnStmt); ok {
				hasReturn = true
			}
			if err := l.lowerStatement(stmt); err != nil {
				return nil, fmt.Errorf("function %s: %w", fn.Name, err)
			}
		}

		l.bindings = savedBindings

		if !hasReturn {
			l.workflow.AddAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.output",
			})
		}

		l.workflow.AddAction(&ir.NativeActionNode{
			NodeID:             l.GenerateUUID(),
			AppleIdentifier:    "is.workflow.actions.conditional",
			GroupingIdentifier: fnGroupUUID,
			ControlFlowMode:    2,
			Parameters: map[string]interface{}{
				"GroupingIdentifier": fnGroupUUID,
				"WFControlFlowMode":  2,
			},
		})
	}

	// Exit the child execution immediately so it never falls through
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.exit",
	})

	// Mode 1: Otherwise -> Main Statements
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: mainGroupUUID,
		ControlFlowMode:    1,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": mainGroupUUID,
			"WFControlFlowMode":  1,
		},
	})

	for _, stmt := range prog.Statements {
		if err := l.lowerStatement(stmt); err != nil {
			return nil, err
		}
	}

	// Mode 2: End If
	l.workflow.AddAction(&ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: mainGroupUUID,
		ControlFlowMode:    2,
		Parameters: map[string]interface{}{
			"GroupingIdentifier": mainGroupUUID,
			"WFControlFlowMode":  2,
		},
	})

	return l.workflow, nil
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
		} else {
			node := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.output",
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
		// let captures the value at this source point.
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
		} else if ref, ok := val.(*ir.AttachmentToken); ok && ref.Type == "Variable" {
			// Materialize mutable variable read with getvariable so later mutations don't affect this let binding!
			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.getvariable",
				OutputUUID:      uuid,
				OutputName:      b.Name,
				Parameters: map[string]interface{}{
					"WFVariable": map[string]interface{}{
						"Value": map[string]interface{}{
							"Type":         "Variable",
							"VariableName": ref.OutputName,
						},
						"WFSerializationType": "WFTextTokenAttachment",
					},
				},
			}
			l.workflow.AddAction(node)
			l.bindings[b.Name] = BindingRef{
				IsVariable: false,
				OutputUUID: uuid,
				OutputName: b.Name,
			}
		} else {
			// Materialize literal or computed value based on its semantic type
			uuid := l.GenerateUUID()
			switch v := val.(type) {
			case float64:
				node := &ir.NativeActionNode{
					NodeID:          uuid,
					AppleIdentifier: "is.workflow.actions.number",
					OutputUUID:      uuid,
					OutputName:      b.Name,
					Parameters: map[string]interface{}{
						"WFNumberActionNumber": v,
					},
				}
				l.workflow.AddAction(node)
			case map[string]interface{}:
				if serType, ok := v["WFSerializationType"].(string); ok && serType == "WFTextTokenString" {
					node := &ir.NativeActionNode{
						NodeID:          uuid,
						AppleIdentifier: "is.workflow.actions.gettext",
						OutputUUID:      uuid,
						OutputName:      b.Name,
						Parameters: map[string]interface{}{
							"WFTextActionText": v,
						},
					}
					l.workflow.AddAction(node)
				} else {
					node := &ir.NativeActionNode{
						NodeID:          uuid,
						AppleIdentifier: "is.workflow.actions.dictionary",
						OutputUUID:      uuid,
						OutputName:      b.Name,
						Parameters: map[string]interface{}{
							"WFItems": v,
						},
					}
					l.workflow.AddAction(node)
				}
			case []interface{}:
				node := &ir.NativeActionNode{
					NodeID:          uuid,
					AppleIdentifier: "is.workflow.actions.list",
					OutputUUID:      uuid,
					OutputName:      b.Name,
					Parameters: map[string]interface{}{
						"WFItems": v,
					},
				}
				l.workflow.AddAction(node)
			case bool:
				bNum := 0.0
				if v {
					bNum = 1.0
				}
				node := &ir.NativeActionNode{
					NodeID:          uuid,
					AppleIdentifier: "is.workflow.actions.number",
					OutputUUID:      uuid,
					OutputName:      b.Name,
					Parameters: map[string]interface{}{
						"WFNumberActionNumber": bNum,
					},
				}
				l.workflow.AddAction(node)
			default:
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
			}
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

	finalVal := val
	if a.Op != syntax.TokenAssign {
		// Compound assignment: +=, -=, *=, /=
		var mathOp string
		switch a.Op {
		case syntax.TokenPlusAssign:
			mathOp = "+"
		case syntax.TokenMinusAssign:
			mathOp = "-"
		case syntax.TokenStarAssign:
			mathOp = "×"
		case syntax.TokenSlashAssign:
			mathOp = "÷"
		}
		mathUUID := l.GenerateUUID()
		leftToken := &ir.AttachmentToken{
			Type:       "Variable",
			OutputName: a.Name,
		}
		mathNode := &ir.NativeActionNode{
			NodeID:          mathUUID,
			AppleIdentifier: "is.workflow.actions.math",
			OutputUUID:      mathUUID,
			Parameters: map[string]interface{}{
				"WFMathOperation": mathOp,
				"WFInput":         leftToken,
				"WFMathOperand":   val,
			},
		}
		l.workflow.AddAction(mathNode)
		finalVal = &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: mathUUID,
		}
	}

	node := &ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{
			"WFVariableName": a.Name,
			"WFInput":        finalVal,
		},
	}
	l.workflow.AddAction(node)
	return nil
}

func (l *Lowerer) materializeToAttachment(val interface{}) *ir.AttachmentToken {
	if tok, ok := val.(*ir.AttachmentToken); ok {
		return tok
	}
	uuid := l.GenerateUUID()
	switch v := val.(type) {
	case float64:
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.number",
			OutputUUID:      uuid,
			OutputName:      "Number",
			Parameters: map[string]interface{}{
				"WFNumberActionNumber": v,
			},
		}
		l.workflow.AddAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "Number",
		}
	case string:
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.gettext",
			OutputUUID:      uuid,
			OutputName:      "Text",
			Parameters: map[string]interface{}{
				"WFTextActionText": v,
			},
		}
		l.workflow.AddAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "Text",
		}
	default:
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.gettext",
			OutputUUID:      uuid,
			OutputName:      "Text",
			Parameters: map[string]interface{}{
				"WFTextActionText": fmt.Sprintf("%v", v),
			},
		}
		l.workflow.AddAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "Text",
		}
	}
}

func (l *Lowerer) lowerIf(stmt *syntax.IfStmt) error {
	groupUUID := l.GenerateUUID()

	params := map[string]interface{}{
		"GroupingIdentifier": groupUUID,
		"WFControlFlowMode":  0,
	}

	// Check if Condition is a comparison BinaryExpr
	if binExpr, ok := stmt.Condition.(*syntax.BinaryExpr); ok {
		var condCode int
		isComp := true
		switch binExpr.Op {
		case syntax.TokenEqual:
			condCode = 4 // Is
		case syntax.TokenNotEqual:
			condCode = 5 // Not
		case syntax.TokenGreater:
			condCode = 2 // Greater Than
		case syntax.TokenGreaterEqual:
			condCode = 3 // Greater Than Or Equal
		case syntax.TokenLess:
			condCode = 0 // Less Than
		case syntax.TokenLessEqual:
			condCode = 1 // Less Than Or Equal
		default:
			isComp = false
		}

		if isComp {
			leftVal, err := l.lowerExpression(binExpr.Left)
			if err != nil {
				return err
			}
			rightVal, err := l.lowerExpression(binExpr.Right)
			if err != nil {
				return err
			}

			params["WFInput"] = l.materializeToAttachment(leftVal)
			params["WFCondition"] = condCode
			switch r := rightVal.(type) {
			case float64:
				params["WFNumberValue"] = r
			case string:
				params["WFConditionalActionString"] = r
			case bool:
				bNum := 0.0
				if r {
					bNum = 1.0
				}
				params["WFNumberValue"] = bNum
			default:
				params["WFNumberValue"] = rightVal
			}
		} else {
			condVal, err := l.lowerExpression(stmt.Condition)
			if err != nil {
				return err
			}
			params["WFInput"] = l.materializeToAttachment(condVal)
			params["WFCondition"] = 4
			params["WFNumberValue"] = 1.0
		}
	} else {
		condVal, err := l.lowerExpression(stmt.Condition)
		if err != nil {
			return err
		}
		params["WFInput"] = l.materializeToAttachment(condVal)
		params["WFCondition"] = 4
		params["WFNumberValue"] = 1.0
	}

	// 1. Begin block (Mode 0)
	beginNode := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters:         params,
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

	var prevBinding BindingRef
	var hadBinding bool
	if stmt.IndexVar != "" {
		prevBinding, hadBinding = l.bindings[stmt.IndexVar]
		mathUUID := l.GenerateUUID()
		mathNode := &ir.NativeActionNode{
			NodeID:          mathUUID,
			AppleIdentifier: "is.workflow.actions.math",
			OutputUUID:      mathUUID,
			Parameters: map[string]interface{}{
				"WFMathOperation": "-",
				"WFInput": &ir.AttachmentToken{
					Type:       "Variable",
					OutputName: "Repeat Index",
				},
				"WFMathOperand": 1.0,
			},
		}
		l.workflow.AddAction(mathNode)
		l.bindings[stmt.IndexVar] = BindingRef{
			IsVariable: false,
			OutputUUID: mathUUID,
			OutputName: stmt.IndexVar,
		}
	}

	for _, s := range stmt.BodyBlock.Statements {
		if err := l.lowerStatement(s); err != nil {
			return err
		}
	}

	if stmt.IndexVar != "" {
		if hadBinding {
			l.bindings[stmt.IndexVar] = prevBinding
		} else {
			delete(l.bindings, stmt.IndexVar)
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

	var prevItemBinding BindingRef
	var hadItemBinding bool
	if stmt.ItemVar != "" {
		prevItemBinding, hadItemBinding = l.bindings[stmt.ItemVar]
		l.bindings[stmt.ItemVar] = BindingRef{
			IsVariable:   true,
			VariableName: "Repeat Item",
		}
	}

	var prevIndexBinding BindingRef
	var hadIndexBinding bool
	if stmt.IndexVar != "" {
		prevIndexBinding, hadIndexBinding = l.bindings[stmt.IndexVar]
		mathUUID := l.GenerateUUID()
		mathNode := &ir.NativeActionNode{
			NodeID:          mathUUID,
			AppleIdentifier: "is.workflow.actions.math",
			OutputUUID:      mathUUID,
			Parameters: map[string]interface{}{
				"WFMathOperation": "-",
				"WFInput": &ir.AttachmentToken{
					Type:       "Variable",
					OutputName: "Repeat Index",
				},
				"WFMathOperand": 1.0,
			},
		}
		l.workflow.AddAction(mathNode)
		l.bindings[stmt.IndexVar] = BindingRef{
			IsVariable: false,
			OutputUUID: mathUUID,
			OutputName: stmt.IndexVar,
		}
	}

	for _, s := range stmt.BodyBlock.Statements {
		if err := l.lowerStatement(s); err != nil {
			return err
		}
	}

	if stmt.ItemVar != "" {
		if hadItemBinding {
			l.bindings[stmt.ItemVar] = prevItemBinding
		} else {
			delete(l.bindings, stmt.ItemVar)
		}
	}
	if stmt.IndexVar != "" {
		if hadIndexBinding {
			l.bindings[stmt.IndexVar] = prevIndexBinding
		} else {
			delete(l.bindings, stmt.IndexVar)
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

	case *syntax.UnaryExpr:
		operand, err := l.lowerExpression(e.Operand)
		if err != nil {
			return nil, err
		}
		switch e.Op {
		case syntax.TokenBang:
			if b, ok := operand.(bool); ok {
				return !b, nil
			}
			groupUUID := l.GenerateUUID()
			uuid := l.GenerateUUID()
			beginNode := &ir.NativeActionNode{
				NodeID:             l.GenerateUUID(),
				AppleIdentifier:    "is.workflow.actions.conditional",
				GroupingIdentifier: groupUUID,
				ControlFlowMode:    0,
				Parameters: map[string]interface{}{
					"GroupingIdentifier": groupUUID,
					"WFControlFlowMode":  0,
					"WFInput":            operand,
					"WFCondition":        4, // Is
					"WFNumberValue":      1.0,
				},
			}
			l.workflow.AddAction(beginNode)
			fNode := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 0.0},
			}
			l.workflow.AddAction(fNode)
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
			tNode := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 1.0},
			}
			l.workflow.AddAction(tNode)
			endNode := &ir.NativeActionNode{
				NodeID:             uuid,
				AppleIdentifier:    "is.workflow.actions.conditional",
				GroupingIdentifier: groupUUID,
				ControlFlowMode:    2,
				OutputUUID:         uuid,
				Parameters: map[string]interface{}{
					"GroupingIdentifier": groupUUID,
					"WFControlFlowMode":  2,
				},
			}
			l.workflow.AddAction(endNode)
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil

		case syntax.TokenMinus:
			if n, ok := operand.(float64); ok {
				return -n, nil
			}
			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.math",
				OutputUUID:      uuid,
				Parameters: map[string]interface{}{
					"WFMathOperation": "-",
					"WFInput":         0.0,
					"WFMathOperand":   operand,
				},
			}
			l.workflow.AddAction(node)
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil

		case syntax.TokenPlus:
			return operand, nil
		}

	case *syntax.IndexExpr:
		targetVal, err := l.lowerExpression(e.Target)
		if err != nil {
			return nil, err
		}
		indexVal, err := l.lowerExpression(e.Index)
		if err != nil {
			return nil, err
		}
		if n, ok := indexVal.(float64); ok {
			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.getitemfromlist",
				OutputUUID:      uuid,
				Parameters: map[string]interface{}{
					"WFInput":         targetVal,
					"WFItemSpecifier": "Item At Index",
					"WFItemIndex":     n + 1.0, // 1-based index in Apple Shortcuts
				},
			}
			l.workflow.AddAction(node)
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
		} else if strKey, ok := indexVal.(string); ok {
			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.getvalueforkey",
				OutputUUID:      uuid,
				Parameters: map[string]interface{}{
					"WFInput":                  targetVal,
					"WFDictionaryKey":          strKey,
					"WFGetDictionaryValueType": "Value",
				},
			}
			l.workflow.AddAction(node)
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
		} else {
			// Dynamic index: add 1
			mathUUID := l.GenerateUUID()
			mathNode := &ir.NativeActionNode{
				NodeID:          mathUUID,
				AppleIdentifier: "is.workflow.actions.math",
				OutputUUID:      mathUUID,
				Parameters: map[string]interface{}{
					"WFMathOperation": "+",
					"WFInput":         indexVal,
					"WFMathOperand":   1.0,
				},
			}
			l.workflow.AddAction(mathNode)

			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.getitemfromlist",
				OutputUUID:      uuid,
				Parameters: map[string]interface{}{
					"WFInput":         targetVal,
					"WFItemSpecifier": "Item At Index",
					"WFItemIndex":     &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: mathUUID},
				},
			}
			l.workflow.AddAction(node)
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
		}

	case *syntax.MemberExpr:
		targetVal, err := l.lowerExpression(e.Target)
		if err != nil {
			return nil, err
		}
		uuid := l.GenerateUUID()
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.getvalueforkey",
			OutputUUID:      uuid,
			Parameters: map[string]interface{}{
				"WFInput":                  targetVal,
				"WFDictionaryKey":          e.Property,
				"WFGetDictionaryValueType": "Value",
			},
		}
		l.workflow.AddAction(node)
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil

	case *syntax.BinaryExpr:
		// Logical short-circuiting:
		if e.Op == syntax.TokenAnd {
			left, err := l.lowerExpression(e.Left)
			if err != nil {
				return nil, err
			}
			if b, ok := left.(bool); ok {
				if !b {
					return false, nil
				}
				return l.lowerExpression(e.Right)
			}
		} else if e.Op == syntax.TokenOr {
			left, err := l.lowerExpression(e.Left)
			if err != nil {
				return nil, err
			}
			if b, ok := left.(bool); ok {
				if b {
					return true, nil
				}
				return l.lowerExpression(e.Right)
			}
		}

		left, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}
		right, err := l.lowerExpression(e.Right)
		if err != nil {
			return nil, err
		}

		// Constant folding for numeric operations
		if lNum, ok1 := left.(float64); ok1 {
			if rNum, ok2 := right.(float64); ok2 {
				switch e.Op {
				case syntax.TokenPlus:
					return lNum + rNum, nil
				case syntax.TokenMinus:
					return lNum - rNum, nil
				case syntax.TokenStar:
					return lNum * rNum, nil
				case syntax.TokenSlash:
					if rNum != 0 {
						return lNum / rNum, nil
					}
				case syntax.TokenPercent:
					if int64(rNum) != 0 {
						return float64(int64(lNum) % int64(rNum)), nil
					}
				case syntax.TokenEqual:
					return lNum == rNum, nil
				case syntax.TokenNotEqual:
					return lNum != rNum, nil
				case syntax.TokenLess:
					return lNum < rNum, nil
				case syntax.TokenLessEqual:
					return lNum <= rNum, nil
				case syntax.TokenGreater:
					return lNum > rNum, nil
				case syntax.TokenGreaterEqual:
					return lNum >= rNum, nil
				}
			}
		}

		// Constant folding for boolean equality
		if lBool, ok1 := left.(bool); ok1 {
			if rBool, ok2 := right.(bool); ok2 {
				switch e.Op {
				case syntax.TokenEqual:
					return lBool == rBool, nil
				case syntax.TokenNotEqual:
					return lBool != rBool, nil
				case syntax.TokenAnd:
					return lBool && rBool, nil
				case syntax.TokenOr:
					return lBool || rBool, nil
				}
			}
		}

		// Constant folding for string equality/concatenation
		if lStr, ok1 := left.(string); ok1 {
			if rStr, ok2 := right.(string); ok2 {
				switch e.Op {
				case syntax.TokenPlus:
					return lStr + rStr, nil
				case syntax.TokenEqual:
					return lStr == rStr, nil
				case syntax.TokenNotEqual:
					return lStr != rStr, nil
				}
			}
		}

		// Dynamic comparison operations
		var condCode int
		isComp := true
		switch e.Op {
		case syntax.TokenEqual:
			condCode = 4
		case syntax.TokenNotEqual:
			condCode = 5
		case syntax.TokenGreater:
			condCode = 2
		case syntax.TokenGreaterEqual:
			condCode = 3
		case syntax.TokenLess:
			condCode = 0
		case syntax.TokenLessEqual:
			condCode = 1
		default:
			isComp = false
		}

		if isComp {
			groupUUID := l.GenerateUUID()
			uuid := l.GenerateUUID()
			params := map[string]interface{}{
				"GroupingIdentifier": groupUUID,
				"WFControlFlowMode":  0,
				"WFInput":            left,
				"WFCondition":        condCode,
			}
			switch r := right.(type) {
			case float64:
				params["WFNumberValue"] = r
			case string:
				params["WFConditionalActionString"] = r
			case bool:
				bNum := 0.0
				if r {
					bNum = 1.0
				}
				params["WFNumberValue"] = bNum
			default:
				params["WFNumberValue"] = right
			}

			beginNode := &ir.NativeActionNode{
				NodeID:             l.GenerateUUID(),
				AppleIdentifier:    "is.workflow.actions.conditional",
				GroupingIdentifier: groupUUID,
				ControlFlowMode:    0,
				Parameters:         params,
			}
			l.workflow.AddAction(beginNode)

			// Then branch: 1.0 (true)
			tNode := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 1.0},
			}
			l.workflow.AddAction(tNode)

			// Else branch: 0.0 (false)
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

			fNode := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 0.0},
			}
			l.workflow.AddAction(fNode)

			endNode := &ir.NativeActionNode{
				NodeID:             uuid,
				AppleIdentifier:    "is.workflow.actions.conditional",
				GroupingIdentifier: groupUUID,
				ControlFlowMode:    2,
				OutputUUID:         uuid,
				Parameters: map[string]interface{}{
					"GroupingIdentifier": groupUUID,
					"WFControlFlowMode":  2,
				},
			}
			l.workflow.AddAction(endNode)
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
		}

		// Modulus
		if e.Op == syntax.TokenPercent {
			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.math",
				OutputUUID:      uuid,
				Parameters: map[string]interface{}{
					"WFMathOperation":           "…",
					"WFScientificMathOperation": "Modulus",
					"WFInput":                   left,
					"WFScientificMathOperand":   right,
				},
			}
			l.workflow.AddAction(node)
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
		}

		// Arithmetic: +, -, *, /
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

			if tok, ok := val.(*ir.AttachmentToken); ok {
				startUTF16 := len(utf16.Encode([]rune(fullText.String())))
				fullText.WriteString("\uFFFC") // Object replacement char

				rangeKey := fmt.Sprintf("{%d, 1}", startUTF16)
				att := map[string]interface{}{
					"Type": tok.Type,
				}
				if tok.OutputUUID != "" {
					att["OutputUUID"] = tok.OutputUUID
				}
				if tok.Type == "Variable" {
					att["VariableName"] = tok.OutputName
				} else if tok.OutputName != "" {
					att["OutputName"] = tok.OutputName
				}
				if len(tok.Aggrandizements) > 0 {
					att["Aggrandizements"] = tok.Aggrandizements
				}
				attachmentsByRange[rangeKey] = att
			} else {
				// Literal value: format directly into text!
				switch v := val.(type) {
				case float64:
					if v == float64(int64(v)) {
						fullText.WriteString(fmt.Sprintf("%d", int64(v)))
					} else {
						fullText.WriteString(fmt.Sprintf("%g", v))
					}
				case bool:
					fullText.WriteString(fmt.Sprintf("%t", v))
				case string:
					fullText.WriteString(v)
				case nil:
					fullText.WriteString("")
				default:
					fullText.WriteString(fmt.Sprint(v))
				}
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

	// Check native escape: native.action, action, rawAction
	if actionName == "native.action" || actionName == "action" || actionName == "rawAction" {
		uuid := l.GenerateUUID()
		node := &ir.NativeActionNode{
			NodeID:     uuid,
			OutputUUID: uuid,
			Parameters: make(map[string]interface{}),
		}
		if call.PrimaryArg != nil {
			val, _ := l.lowerExpression(call.PrimaryArg)
			if s, ok := val.(string); ok {
				node.AppleIdentifier = s
			} else {
				node.AppleIdentifier = fmt.Sprintf("%v", val)
			}
		}
		for _, arg := range call.NamedArgs {
			if arg.Label == "identifier" {
				if lit, ok := arg.Value.(*syntax.LiteralExpr); ok {
					node.AppleIdentifier = fmt.Sprintf("%v", lit.Value)
				}
			} else if arg.Label == "parameters" || arg.Label == "" {
				val, _ := l.lowerExpression(arg.Value)
				if m, ok := val.(map[string]interface{}); ok {
					for k, v := range m {
						node.Parameters[k] = v
					}
				}
			}
		}
		l.workflow.AddAction(node)
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
	}

	// Check user-defined function
	if fn, ok := l.functions[actionName]; ok {
		argsList := make([]interface{}, len(fn.Parameters))
		for i, param := range fn.Parameters {
			var argVal interface{}
			found := false
			if i == 0 && call.PrimaryArg != nil {
				v, err := l.lowerExpression(call.PrimaryArg)
				if err != nil {
					return nil, err
				}
				argVal = v
				found = true
			}
			if !found {
				for _, nArg := range call.NamedArgs {
					if nArg.Label == param.Name {
						v, err := l.lowerExpression(nArg.Value)
						if err != nil {
							return nil, err
						}
						argVal = v
						found = true
						break
					}
				}
			}
			if !found && param.DefaultExpr != nil {
				v, err := l.lowerExpression(param.DefaultExpr)
				if err != nil {
					return nil, err
				}
				argVal = v
				found = true
			}
			argsList[i] = argVal
		}

		dispatchDict := map[string]interface{}{
			"cherri_functions": 1.0,
			"function":         actionName,
			"arguments":        argsList,
		}

		wfTarget := map[string]interface{}{
			"workflowIdentifier": l.GenerateUUID(),
			"isSelf":             true,
		}
		if l.WorkflowName != "" {
			wfTarget["workflowName"] = l.WorkflowName
		}

		runParams := map[string]interface{}{
			"WFWorkflow": wfTarget,
			"WFInput":    dispatchDict,
		}
		if l.WorkflowName != "" {
			runParams["WFWorkflowName"] = l.WorkflowName
		}

		uuid := l.GenerateUUID()
		runNode := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.runworkflow",
			OutputUUID:      uuid,
			OutputName:      actionName + "Result",
			Parameters:      runParams,
		}
		l.workflow.AddAction(runNode)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: actionName + "Result",
		}, nil
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
	for k, v := range actionSchema.StaticParameters {
		node.Parameters[k] = v
	}
	if actionSchema.AppIntent != nil {
		node.Parameters["AppIntentDescriptor"] = map[string]interface{}{
			"AppIntentIdentifier": actionSchema.AppIntent.AppIntentIdentifier,
			"BundleIdentifier":    actionSchema.AppIntent.BundleIdentifier,
			"Name":                actionSchema.AppIntent.Name,
			"TeamIdentifier":      actionSchema.AppIntent.TeamIdentifier,
		}
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
			if primParam.TypeName == "Text" || primParam.TypeName == "String" || (primParam.Type != nil && (primParam.Type.Name() == "Text" || primParam.Type.Name() == "String")) {
				if tok, isTok := primVal.(*ir.AttachmentToken); isTok {
					primVal = map[string]interface{}{
						"WFSerializationType": "WFTextTokenString",
						"Value": map[string]interface{}{
							"string": "\uFFFC",
							"attachmentsByRange": map[string]interface{}{
								"{0, 1}": tok,
							},
						},
					}
				}
			}
			node.Parameters[wireKey] = primVal

			if ident, isIdent := call.PrimaryArg.(*syntax.IdentExpr); isIdent {
				if setupQ, isSetup := l.setupQuestions[ident.Name]; isSetup {
					found := false
					for idx, existingQ := range l.workflow.ImportQuestions {
						if existingQ["ParameterKey"] == ident.Name {
							l.workflow.ImportQuestions[idx]["ActionIndex"] = len(l.workflow.Actions)
							l.workflow.ImportQuestions[idx]["ParameterKey"] = wireKey
							found = true
							break
						}
					}
					if !found {
						qMap := map[string]interface{}{
							"ActionIndex":  len(l.workflow.Actions),
							"ParameterKey": wireKey,
							"Text":         setupQ.Prompt,
							"DefaultValue": setupQ.DefaultValue,
							"Category":     "Parameter",
						}
						l.workflow.ImportQuestions = append(l.workflow.ImportQuestions, qMap)
					}
					if setupQ.DefaultValue != "" {
						node.Parameters[wireKey] = setupQ.DefaultValue
					}
				}
			}
		}
	}

	// 2. Named and positional arguments
	paramIndex := 1
	if call.PrimaryArg == nil {
		paramIndex = 0
	}
	for _, nArg := range call.NamedArgs {
		var param *schema.ParameterSchema
		var exists bool
		if nArg.Label != "" {
			param, exists = actionSchema.ParameterByLabel(nArg.Label)
		} else {
			if paramIndex < len(actionSchema.Parameters) {
				param = &actionSchema.Parameters[paramIndex]
				exists = true
				paramIndex++
			}
		}
		if exists && param != nil {
			val, err := l.lowerExpression(nArg.Value)
			if err != nil {
				return nil, err
			}
			wireKey := param.WireKey
			if wireKey == "" {
				wireKey = param.Label
			}
			if param.TypeName == "Text" || param.TypeName == "String" || (param.Type != nil && (param.Type.Name() == "Text" || param.Type.Name() == "String")) {
				if tok, isTok := val.(*ir.AttachmentToken); isTok {
					val = map[string]interface{}{
						"WFSerializationType": "WFTextTokenString",
						"Value": map[string]interface{}{
							"string": "\uFFFC",
							"attachmentsByRange": map[string]interface{}{
								"{0, 1}": tok,
							},
						},
					}
				}
			}
			node.Parameters[wireKey] = val

			if ident, isIdent := nArg.Value.(*syntax.IdentExpr); isIdent {
				if setupQ, isSetup := l.setupQuestions[ident.Name]; isSetup {
					found := false
					for idx, existingQ := range l.workflow.ImportQuestions {
						if existingQ["ParameterKey"] == ident.Name {
							l.workflow.ImportQuestions[idx]["ActionIndex"] = len(l.workflow.Actions)
							l.workflow.ImportQuestions[idx]["ParameterKey"] = wireKey
							found = true
							break
						}
					}
					if !found {
						qMap := map[string]interface{}{
							"ActionIndex":  len(l.workflow.Actions),
							"ParameterKey": wireKey,
							"Text":         setupQ.Prompt,
							"DefaultValue": setupQ.DefaultValue,
							"Category":     "Parameter",
						}
						l.workflow.ImportQuestions = append(l.workflow.ImportQuestions, qMap)
					}
					if setupQ.DefaultValue != "" {
						node.Parameters[wireKey] = setupQ.DefaultValue
					}
				}
			}
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
