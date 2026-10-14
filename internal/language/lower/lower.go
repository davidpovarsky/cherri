package lower

import (
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

// BindingRef represents how a variable is stored and referenced in Shortcuts.
type BindingRef struct {
	IsVariable      bool   // true = setvariable/getvariable, false = direct action output token
	OutputUUID      string // for action output
	OutputName      string
	VariableName    string // for setvariable
	Aggrandizements []map[string]interface{}
}

var contentItemClasses = map[string]string{
	"text":        "WFStringContentItem",
	"number":      "WFNumberContentItem",
	"date":        "WFDateContentItem",
	"boolean":     "WFBooleanContentItem",
	"dictionary":  "WFDictionaryContentItem",
	"image":       "WFImageContentItem",
	"url":         "WFURLContentItem",
	"file":        "WFGenericFileContentItem",
	"pdf":         "WFPDFContentItem",
	"contact":     "WFContactContentItem",
	"phonenumber": "WFPhoneNumberContentItem",
	"email":       "WFEmailAddressContentItem",
	"location":    "WFLocationContentItem",
}

// Lowerer transforms a Cherri v2 AST into Native Shortcut IR.
type Lowerer struct {
	registry           *schema.Registry
	session            backend.Session
	workflow           *ir.NativeWorkflow
	bindings           map[string]BindingRef
	functions          map[string]*syntax.FunctionDecl
	setupQuestions     map[string]*syntax.SetupDecl
	enums              map[string]*syntax.EnumDecl
	types              map[string]*syntax.TypeDecl
	actions            map[string]*syntax.ActionDecl
	importStack        []string
	modules            map[string]string
	virtualModules     map[string]string
	baseFilePath       string
	importedFunctions  []*syntax.FunctionDecl
	WorkflowName       string
	uuidCounter        int
	repeatDepth        int
	currentBindingName string
	emitErr            error
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
		enums:          make(map[string]*syntax.EnumDecl),
		types:          make(map[string]*syntax.TypeDecl),
		actions:        make(map[string]*syntax.ActionDecl),
		modules:        make(map[string]string),
		virtualModules: make(map[string]string),
	}
}

// SetVirtualModule configures an in-memory virtual module for testing or embedded compilation.
func (l *Lowerer) SetVirtualModule(path, content string) {
	if l.virtualModules == nil {
		l.virtualModules = make(map[string]string)
	}
	l.virtualModules[path] = content
}

// SetVirtualModules sets multiple virtual modules.
func (l *Lowerer) SetVirtualModules(vm map[string]string) {
	if l.virtualModules == nil {
		l.virtualModules = make(map[string]string)
	}
	for k, v := range vm {
		l.virtualModules[k] = v
	}
}

// SetBaseFilePath sets the base file path used to resolve relative disk imports.
func (l *Lowerer) SetBaseFilePath(path string) {
	l.baseFilePath = path
}

// NewLowererWithSession creates a new AST lowerer connected to a backend session.
func NewLowererWithSession(registry *schema.Registry, session backend.Session) *Lowerer {
	l := NewLowerer(registry)
	l.session = session
	return l
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

func (l *Lowerer) emitAction(node *ir.NativeActionNode) {
	if l.session != nil && l.emitErr == nil {
		if err := l.session.EmitRawAction(node.AppleIdentifier, node.Parameters, node.OutputUUID, node.OutputName, node.GroupingIdentifier); err != nil {
			l.emitErr = err
		}
	}
	l.workflow.AddAction(node)
}

func (l *Lowerer) emitIfStart(groupUUID string, input any, condCode int, compareVal any, isLegacy bool) *ir.NativeActionNode {
	node := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters:         backend.BuildIfStart(groupUUID, input, condCode, compareVal, isLegacy),
	}
	l.emitAction(node)
	return node
}

func (l *Lowerer) emitIfElse(groupUUID string) *ir.NativeActionNode {
	node := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    1,
		Parameters:         backend.BuildIfElse(groupUUID),
	}
	l.emitAction(node)
	return node
}

func (l *Lowerer) emitIfEnd(groupUUID string, outputUUID string, outputName string) *ir.NativeActionNode {
	nodeID := outputUUID
	if nodeID == "" {
		nodeID = l.GenerateUUID()
	}
	node := &ir.NativeActionNode{
		NodeID:             nodeID,
		AppleIdentifier:    "is.workflow.actions.conditional",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    2,
		OutputUUID:         outputUUID,
		OutputName:         outputName,
		Parameters:         backend.BuildIfEnd(groupUUID, outputUUID),
	}
	l.emitAction(node)
	return node
}

func (l *Lowerer) emitRepeatCountStart(groupUUID string, count any) *ir.NativeActionNode {
	node := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.repeat.count",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters:         backend.BuildRepeatCountStart(groupUUID, count),
	}
	l.emitAction(node)
	return node
}

func (l *Lowerer) emitRepeatEachStart(groupUUID string, iterable any) *ir.NativeActionNode {
	node := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.repeat.each",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters:         backend.BuildRepeatEachStart(groupUUID, iterable),
	}
	l.emitAction(node)
	return node
}

func (l *Lowerer) emitLoopEnd(groupUUID string, outputUUID string, isRepeatEach bool) *ir.NativeActionNode {
	appleID := "is.workflow.actions.repeat.count"
	if isRepeatEach {
		appleID = "is.workflow.actions.repeat.each"
	}
	nodeID := outputUUID
	if nodeID == "" {
		nodeID = l.GenerateUUID()
	}
	node := &ir.NativeActionNode{
		NodeID:             nodeID,
		AppleIdentifier:    appleID,
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    2,
		OutputUUID:         outputUUID,
		Parameters:         backend.BuildLoopEnd(groupUUID, outputUUID),
	}
	l.emitAction(node)
	return node
}

func (l *Lowerer) emitMenuStart(groupUUID string, prompt any, itemTitles []string) *ir.NativeActionNode {
	node := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.choosefrommenu",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    0,
		Parameters:         backend.BuildMenuStart(groupUUID, prompt, itemTitles),
	}
	l.emitAction(node)
	return node
}

func (l *Lowerer) emitMenuCase(groupUUID string, title string) *ir.NativeActionNode {
	node := &ir.NativeActionNode{
		NodeID:             l.GenerateUUID(),
		AppleIdentifier:    "is.workflow.actions.choosefrommenu",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    1,
		Parameters:         backend.BuildMenuCase(groupUUID, title),
	}
	l.emitAction(node)
	return node
}

func (l *Lowerer) emitMenuEnd(groupUUID string, outputUUID string) *ir.NativeActionNode {
	nodeID := outputUUID
	if nodeID == "" {
		nodeID = l.GenerateUUID()
	}
	node := &ir.NativeActionNode{
		NodeID:             nodeID,
		AppleIdentifier:    "is.workflow.actions.choosefrommenu",
		GroupingIdentifier: groupUUID,
		ControlFlowMode:    2,
		OutputUUID:         outputUUID,
		Parameters:         backend.BuildMenuEnd(groupUUID, outputUUID),
	}
	l.emitAction(node)
	return node
}

// LowerProgramToSession compiles an AST Program into the configured backend.Session.
func (l *Lowerer) LowerProgramToSession(prog *syntax.Program) error {
	// 1. Process declarations (metadata, shortcut header, functions, setup, custom actions, enums, types)
	for _, decl := range prog.Declarations {
		switch d := decl.(type) {
		case *syntax.ShortcutDecl:
			if err := l.lowerShortcutHeader(d); err != nil {
				return err
			}
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
			if l.session != nil {
				l.session.AddImportQuestion(q)
			}
		case *syntax.ActionDecl:
			if d.Name == "" || d.AppleIdentifier == "" {
				return fmt.Errorf("action declaration must specify name and identifier")
			}
			l.actions[d.Name] = d
		case *syntax.EnumDecl:
			if d.Name == "" {
				return fmt.Errorf("enum declaration must specify name")
			}
			l.enums[d.Name] = d
		case *syntax.TypeDecl:
			if d.Name == "" {
				return fmt.Errorf("type alias declaration must specify name")
			}
			l.types[d.Name] = d
		case *syntax.ImportDecl:
			if err := l.lowerImport(d); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unhandled declaration %T: declarations must not be silently dropped", decl)
		}
	}

	// 2. If functions are declared, isolate dispatcher in Mode 0 and main statements in Mode 1 (Otherwise)
	if len(l.functions) > 0 {
		_, err := l.lowerProgramWithFunctions(prog)
		if err != nil {
			return err
		}
		return l.emitErr
	}

	// 3. Process statements in source order
	for _, stmt := range prog.Statements {
		if err := l.lowerStatement(stmt); err != nil {
			return err
		}
		if l.emitErr != nil {
			return l.emitErr
		}
	}

	return l.emitErr
}

// LowerProgram compiles an AST Program into NativeWorkflow IR.
func (l *Lowerer) LowerProgram(prog *syntax.Program) (*ir.NativeWorkflow, error) {
	if err := l.LowerProgramToSession(prog); err != nil {
		return nil, err
	}
	return l.workflow, nil
}

func (l *Lowerer) lowerImport(d *syntax.ImportDecl) error {
	if strings.HasPrefix(d.Path, "http://") || strings.HasPrefix(d.Path, "https://") {
		return fmt.Errorf("remote package imports not permitted: %q", d.Path)
	}
	cleanPath := filepath.Clean(d.Path)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(d.Path) {
		return fmt.Errorf("path traversal outside workspace not permitted: %q", d.Path)
	}

	for _, p := range l.importStack {
		if p == d.Path || p == cleanPath {
			return fmt.Errorf("import cycle detected: %s -> %s", strings.Join(l.importStack, " -> "), d.Path)
		}
	}

	// 1. Check if built-in module from registry
	isBuiltin := false
	if l.registry != nil {
		for _, act := range l.registry.AllActions() {
			if act.Module != "" && (act.Module == d.Path || act.Module == cleanPath) {
				isBuiltin = true
				break
			}
		}
	}
	if isBuiltin {
		if l.modules == nil {
			l.modules = make(map[string]string)
		}
		l.modules[d.Alias] = d.Path
		return nil
	}

	// 2. Resolve local module content
	var content string
	var resolvedPath string
	if vmContent, ok := l.virtualModules[d.Path]; ok {
		content = vmContent
	} else if vmContent, ok := l.virtualModules[cleanPath]; ok {
		content = vmContent
	} else {
		filePath := cleanPath
		if l.baseFilePath != "" && !filepath.IsAbs(filePath) {
			filePath = filepath.Join(filepath.Dir(l.baseFilePath), filePath)
		}
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if _, err2 := os.Stat(filePath + ".cherri"); err2 == nil {
				filePath = filePath + ".cherri"
			} else {
				return fmt.Errorf("cannot resolve module %q: file not found", d.Path)
			}
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("cannot read module %q: %w", d.Path, err)
		}
		content = string(data)
		resolvedPath = filePath
	}

	// 3. Parse module
	srcFile := source.NewFile(source.DocumentID(d.Path), d.Path, 1, content)
	parser := syntax.NewParser(srcFile)
	modProg := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		migrated := migrate.MigrateSource(content)
		srcFile = source.NewFile(source.DocumentID(d.Path), d.Path, 1, migrated)
		parser = syntax.NewParser(srcFile)
		modProg = parser.ParseProgram()
		if len(parser.Errors()) > 0 {
			return fmt.Errorf("syntax error in imported module %q: %v", d.Path, parser.Errors()[0])
		}
	}

	// 4. Check top-level statements (IM02)
	if len(modProg.Statements) > 0 {
		return fmt.Errorf("module top-level runtime initialization rejected: imported module %q contains top-level executable statements", d.Path)
	}

	// 5. Register declarations
	l.importStack = append(l.importStack, d.Path)
	oldBase := l.baseFilePath
	if resolvedPath != "" {
		l.baseFilePath = resolvedPath
	}
	defer func() {
		l.baseFilePath = oldBase
		if len(l.importStack) > 0 {
			l.importStack = l.importStack[:len(l.importStack)-1]
		}
	}()

	for _, decl := range modProg.Declarations {
		switch m := decl.(type) {
		case *syntax.FunctionDecl:
			if d.Alias != "" {
				l.functions[d.Alias+"."+m.Name] = m
			}
			l.functions[m.Name] = m
			l.importedFunctions = append(l.importedFunctions, m)
		case *syntax.EnumDecl:
			if d.Alias != "" {
				l.enums[d.Alias+"."+m.Name] = m
			}
			l.enums[m.Name] = m
		case *syntax.TypeDecl:
			if d.Alias != "" {
				l.types[d.Alias+"."+m.Name] = m
			}
			l.types[m.Name] = m
		case *syntax.ActionDecl:
			if d.Alias != "" {
				l.actions[d.Alias+"."+m.Name] = m
			}
			l.actions[m.Name] = m
		case *syntax.ImportDecl:
			if err := l.lowerImport(m); err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *Lowerer) lowerShortcutHeader(d *syntax.ShortcutDecl) error {
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
			case "color":
				if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
					var c int
					fmt.Sscanf(lit.Value, "%d", &c)
					l.workflow.IconColor = c
				}
			case "glyph":
				if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
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
						l.workflow.QuickActionSurfaces = []string{"Finder", "Services"}
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
			case "outputs":
				var outputList []string
				if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
					outputList = append(outputList, lit.Value)
				} else if ident, ok := f.Value.(*syntax.IdentExpr); ok {
					outputList = append(outputList, ident.Name)
				} else if list, ok := f.Value.(*syntax.ListExpr); ok {
					for _, el := range list.Elements {
						if lit, ok := el.(*syntax.LiteralExpr); ok {
							outputList = append(outputList, lit.Value)
						} else if ident, ok := el.(*syntax.IdentExpr); ok {
							outputList = append(outputList, ident.Name)
						}
					}
				}
				l.workflow.OutputContentItemClasses = nil
				for _, item := range outputList {
					switch item {
					case "text":
						l.workflow.OutputContentItemClasses = append(l.workflow.OutputContentItemClasses, "WFStringContentItem")
					case "url":
						l.workflow.OutputContentItemClasses = append(l.workflow.OutputContentItemClasses, "WFURLContentItem")
					case "file":
						l.workflow.OutputContentItemClasses = append(l.workflow.OutputContentItemClasses, "WFGenericFileContentItem")
					case "image":
						l.workflow.OutputContentItemClasses = append(l.workflow.OutputContentItemClasses, "WFImageContentItem")
					default:
						l.workflow.OutputContentItemClasses = append(l.workflow.OutputContentItemClasses, item)
					}
				}
			case "noInput":
				if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
					l.workflow.NoInputBehavior = lit.Value
				} else if ident, ok := f.Value.(*syntax.IdentExpr); ok {
					l.workflow.NoInputBehavior = ident.Name
				}
			case "quickActions", "quickactions":
				l.workflow.QuickActionSurfaces = []string{"Finder", "Services"}
			default:
				return fmt.Errorf("unknown shortcut metadata field %q", f.Name)
			}
		}
	}

	for _, trig := range d.Triggers {
		var appleID string
		switch trig.Family {
		case "bluetooth":
			appleID = "WFBluetoothTrigger"
		case "wifi":
			appleID = "WFWifiTrigger"
		case "stageManager":
			appleID = "WFStageManagerTrigger"
		case "display":
			appleID = "WFExternalDisplayTrigger"
		case "screenshot":
			appleID = "WFScreenshotTrigger"
		case "battery":
			appleID = "WFBatteryLevelTrigger"
		case "charging":
			appleID = "WFPlugInTrigger"
		default:
			return fmt.Errorf("unsupported trigger family %q; supported triggers are: bluetooth, wifi, stageManager, display, screenshot, battery, charging", trig.Family)
		}
		params := make(map[string]any)
		switch trig.Family {
		case "bluetooth":
			connType := trig.Event
			if connType == "" {
				connType = "connect"
			}
			params["WFBluetoothConnectionType"] = connType
		case "wifi":
			connType := trig.Event
			if connType == "" {
				connType = "joined"
			}
			params["WFConnectionType"] = connType
		case "charging", "display":
			connType := trig.Event
			if connType == "" {
				connType = "connect"
			}
			params["WFConnectionType"] = connType
		case "stageManager":
			smType := trig.Event
			if smType == "" {
				smType = "on"
			}
			params["WFStageManagerType"] = smType
		case "screenshot":
			params["ScreenshotLocations"] = []string{"photos"}
		case "battery":
			params["WFBatteryLevel"] = 0.5
		}
		triggerUUID := l.GenerateUUID()
		if l.session != nil {
			l.session.SetMetadata("trigger", map[string]any{
				"Identifier": appleID,
				"Parameters": params,
				"UUID":       triggerUUID,
			})
		}
	}

	if l.session != nil {
		if len(l.workflow.WorkflowTypes) > 0 {
			l.session.SetMetadata("workflowTypes", l.workflow.WorkflowTypes)
		}
		if len(l.workflow.InputContentItemClasses) > 0 {
			l.session.SetMetadata("inputContentItemClasses", l.workflow.InputContentItemClasses)
		}
		if len(l.workflow.OutputContentItemClasses) > 0 {
			l.session.SetMetadata("outputContentItemClasses", l.workflow.OutputContentItemClasses)
		}
		if l.workflow.IconGlyph != 0 {
			l.session.SetMetadata("iconGlyph", l.workflow.IconGlyph)
		}
		if l.workflow.IconColor != 0 {
			l.session.SetMetadata("iconColor", l.workflow.IconColor)
		}
		if l.workflow.NoInputBehavior != "" {
			l.session.SetMetadata("noInputBehavior", l.workflow.NoInputBehavior)
		}
		if len(l.workflow.QuickActionSurfaces) > 0 {
			l.session.SetMetadata("quickActionSurfaces", l.workflow.QuickActionSurfaces)
		}
	}
	return nil
}

func (l *Lowerer) bindSetupQuestion(qName string, wireKey string) {
	for idx, existingQ := range l.workflow.ImportQuestions {
		if existingQ["ParameterKey"] == qName {
			l.workflow.ImportQuestions[idx]["ActionIndex"] = len(l.workflow.Actions)
			l.workflow.ImportQuestions[idx]["ParameterKey"] = wireKey
			break
		}
	}
	if l.session != nil {
		l.session.BindImportQuestion(qName, wireKey)
	}
}

func (l *Lowerer) lowerProgramWithFunctions(prog *syntax.Program) (*ir.NativeWorkflow, error) {
	l.workflow.HasShortcutInputVariables = true
	if l.session != nil {
		l.session.SetMetadata("hasShortcutInputVariables", true)
	}

	// 1. Initialize _cherri_is_fn = "0"
	initNumUUID := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          initNumUUID,
		AppleIdentifier: "is.workflow.actions.gettext",
		OutputUUID:      initNumUUID,
		Parameters: map[string]interface{}{
			"WFTextActionText": "0",
		},
	})
	l.emitAction(&ir.NativeActionNode{
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

	// Save initial caller ShortcutInput into _cherri_caller_input before function dispatch
	l.emitAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{
			"WFVariableName": "_cherri_caller_input",
			"WFInput":        inputToken,
		},
	})

	// 2. Guard: Only check function payload if ShortcutInput has a value
	hasInputGroupUUID := l.GenerateUUID()
	l.emitIfStart(hasInputGroupUUID, inputToken, 100, nil, true)

	dictUUID := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          dictUUID,
		AppleIdentifier: "is.workflow.actions.detect.dictionary",
		OutputUUID:      dictUUID,
		Parameters: map[string]interface{}{
			"WFInput": inputToken,
		},
	})

	chkUUID := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          chkUUID,
		AppleIdentifier: "is.workflow.actions.getvalueforkey",
		OutputUUID:      chkUUID,
		Parameters: map[string]interface{}{
			"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: dictUUID},
			"WFDictionaryKey":          "cherri_functions",
			"WFGetDictionaryValueType": "Value",
		},
	})

	magicUUID := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          magicUUID,
		AppleIdentifier: "is.workflow.actions.getvalueforkey",
		OutputUUID:      magicUUID,
		Parameters: map[string]interface{}{
			"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: dictUUID},
			"WFDictionaryKey":          "_cherri_magic",
			"WFGetDictionaryValueType": "Value",
		},
	})

	chkGroupUUID := l.GenerateUUID()
	l.emitIfStart(chkGroupUUID, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: magicUUID}, 4, "__cherri_fn_envelope_v2__", true)

	setOneNumUUID := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          setOneNumUUID,
		AppleIdentifier: "is.workflow.actions.gettext",
		OutputUUID:      setOneNumUUID,
		Parameters: map[string]interface{}{
			"WFTextActionText": "1",
		},
	})
	l.emitAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{
			"WFVariableName": "_cherri_is_fn",
			"WFInput":        &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: setOneNumUUID},
		},
	})

	callerInUUID := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          callerInUUID,
		AppleIdentifier: "is.workflow.actions.getvalueforkey",
		OutputUUID:      callerInUUID,
		Parameters: map[string]interface{}{
			"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: dictUUID},
			"WFDictionaryKey":          "_cherri_caller_input",
			"WFGetDictionaryValueType": "Value",
		},
	})
	l.emitAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters: map[string]interface{}{
			"WFVariableName": "_cherri_caller_input",
			"WFInput":        &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: callerInUUID},
		},
	})

	l.emitIfEnd(chkGroupUUID, "", "")
	l.emitIfEnd(hasInputGroupUUID, "", "")

	// 3. Main branching conditional:
	// Mode 0: If _cherri_is_fn == "1" -> Function dispatcher
	// Mode 1: Otherwise -> Main program statements
	// Mode 2: End If
	mainGroupUUID := l.GenerateUUID()
	l.emitIfStart(mainGroupUUID, &ir.AttachmentToken{Type: "Variable", OutputName: "_cherri_is_fn"}, 4, "1", true)

	// Mode 0: Function Dispatcher
	fnDictUUID := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          fnDictUUID,
		AppleIdentifier: "is.workflow.actions.detect.dictionary",
		OutputUUID:      fnDictUUID,
		Parameters: map[string]interface{}{
			"WFInput": inputToken,
		},
	})

	fnNameUUID := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
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
	l.emitAction(&ir.NativeActionNode{
		NodeID:          argsUUID,
		AppleIdentifier: "is.workflow.actions.getvalueforkey",
		OutputUUID:      argsUUID,
		Parameters: map[string]interface{}{
			"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: fnDictUUID},
			"WFDictionaryKey":          "arguments",
			"WFGetDictionaryValueType": "Value",
		},
	})

	allFuncs := make([]*syntax.FunctionDecl, 0, len(l.functions))
	seenFuncs := make(map[string]bool)
	for _, decl := range prog.Declarations {
		if fn, ok := decl.(*syntax.FunctionDecl); ok {
			if !seenFuncs[fn.Name] {
				allFuncs = append(allFuncs, fn)
				seenFuncs[fn.Name] = true
			}
		}
	}
	for _, fn := range l.importedFunctions {
		if !seenFuncs[fn.Name] {
			allFuncs = append(allFuncs, fn)
			seenFuncs[fn.Name] = true
		}
	}

	for _, fn := range allFuncs {
		fnGroupUUID := l.GenerateUUID()
		l.emitIfStart(fnGroupUUID, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: fnNameUUID}, 4, fn.Name, true)

		savedBindings := make(map[string]BindingRef)
		for k, v := range l.bindings {
			savedBindings[k] = v
		}
		l.bindings = make(map[string]BindingRef)

		for idx, param := range fn.Parameters {
			argKey := fmt.Sprintf("arg_%d", idx)
			isNumParam := false
			if named, ok := param.TypeExpr.(*syntax.NamedTypeAnnotation); ok && (named.Name == "Number" || named.Name == "Bool") {
				isNumParam = true
			}
			if param.DefaultExpr != nil {
				hasKey := fmt.Sprintf("has_arg_%d", idx)
				hasUUID := l.GenerateUUID()
				l.emitAction(&ir.NativeActionNode{
					NodeID:          hasUUID,
					AppleIdentifier: "is.workflow.actions.getvalueforkey",
					OutputUUID:      hasUUID,
					Parameters: map[string]interface{}{
						"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: fnDictUUID},
						"WFDictionaryKey":          hasKey,
						"WFGetDictionaryValueType": "Value",
					},
				})

				defGroupUUID := l.GenerateUUID()
				l.emitIfStart(defGroupUUID, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: hasUUID}, 4, "1", true)

				suppliedUUID := l.GenerateUUID()
				l.emitAction(&ir.NativeActionNode{
					NodeID:          suppliedUUID,
					AppleIdentifier: "is.workflow.actions.getvalueforkey",
					OutputUUID:      suppliedUUID,
					Parameters: map[string]interface{}{
						"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: fnDictUUID},
						"WFDictionaryKey":          argKey,
						"WFGetDictionaryValueType": "Value",
					},
				})

				l.emitIfElse(defGroupUUID)

				defVal, err := l.lowerExpression(param.DefaultExpr)
				if err != nil {
					return nil, fmt.Errorf("function %s default param %s: %w", fn.Name, param.Name, err)
				}
				l.materializeYieldValue(defVal)

				argItemUUID := l.GenerateUUID()
				l.emitIfEnd(defGroupUUID, argItemUUID, param.Name)
				if isNumParam {
					numUUID := l.GenerateUUID()
					l.emitAction(&ir.NativeActionNode{
						NodeID:          numUUID,
						AppleIdentifier: "is.workflow.actions.detect.number",
						OutputUUID:      numUUID,
						OutputName:      param.Name,
						Parameters: map[string]interface{}{
							"WFInput": &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: argItemUUID},
						},
					})
					argItemUUID = numUUID
				}
				l.bindings[param.Name] = BindingRef{
					IsVariable: false,
					OutputUUID: argItemUUID,
					OutputName: param.Name,
				}
			} else {
				argItemUUID := l.GenerateUUID()
				l.emitAction(&ir.NativeActionNode{
					NodeID:          argItemUUID,
					AppleIdentifier: "is.workflow.actions.getvalueforkey",
					OutputUUID:      argItemUUID,
					OutputName:      param.Name,
					Parameters: map[string]interface{}{
						"WFInput":                  &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: fnDictUUID},
						"WFDictionaryKey":          argKey,
						"WFGetDictionaryValueType": "Value",
					},
				})
				if isNumParam {
					numUUID := l.GenerateUUID()
					l.emitAction(&ir.NativeActionNode{
						NodeID:          numUUID,
						AppleIdentifier: "is.workflow.actions.detect.number",
						OutputUUID:      numUUID,
						OutputName:      param.Name,
						Parameters: map[string]interface{}{
							"WFInput": &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: argItemUUID},
						},
					})
					argItemUUID = numUUID
				}
				l.bindings[param.Name] = BindingRef{
					IsVariable: false,
					OutputUUID: argItemUUID,
					OutputName: param.Name,
				}
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
			l.emitAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.nothing",
			})
			l.emitAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.output",
			})
		}

		l.emitIfEnd(fnGroupUUID, "", "")
	}

	// Exit the child execution immediately so it never falls through
	l.emitAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.exit",
	})

	// Mode 1: Otherwise -> Main Statements
	l.emitIfElse(mainGroupUUID)

	for _, stmt := range prog.Statements {
		if err := l.lowerStatement(stmt); err != nil {
			return nil, err
		}
	}

	// Mode 2: End If
	l.emitIfEnd(mainGroupUUID, "", "")

	return l.workflow, nil
}

func (l *Lowerer) materializeYieldValue(val interface{}) {
	if tok, ok := val.(*ir.AttachmentToken); ok {
		if tok.Type == "Variable" {
			uuid := l.GenerateUUID()
			l.emitAction(&ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.getvariable",
				OutputUUID:      uuid,
				Parameters: map[string]interface{}{
					"WFVariable": tok,
				},
			})
			return
		}
		if len(l.workflow.Actions) == 0 || l.workflow.Actions[len(l.workflow.Actions)-1].OutputUUID != tok.OutputUUID {
			uuid := l.GenerateUUID()
			l.emitAction(&ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.getvariable",
				OutputUUID:      uuid,
				Parameters: map[string]interface{}{
					"WFVariable": tok,
				},
			})
		}
		return
	}
	if b, ok := val.(bool); ok {
		bNum := 0.0
		if b {
			bNum = 1.0
		}
		uuid := l.GenerateUUID()
		l.emitAction(&ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.number",
			OutputUUID:      uuid,
			Parameters: map[string]interface{}{
				"WFNumberActionNumber": bNum,
			},
		})
		return
	}
	_ = l.materializeToAttachment(val)
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
	case *syntax.YieldStmt:
		val, err := l.lowerExpression(s.Value)
		if err != nil {
			return err
		}
		l.materializeYieldValue(val)
		return nil
	case *syntax.ReturnStmt:
		l.workflow.HasExplicitReturn = true
		if s.Value != nil {
			val, err := l.lowerExpression(s.Value)
			if err != nil {
				return err
			}
			val = l.materializeToAttachment(val)
			// Emit output action: is.workflow.actions.output
			node := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.output",
				Parameters: map[string]interface{}{
					"WFOutput": val,
				},
			}
			l.emitAction(node)
		} else {
			l.emitAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.nothing",
			})
			node := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.output",
			}
			l.emitAction(node)
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
		return fmt.Errorf("unhandled statement %T: statements must not be silently dropped", stmt)
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

	if !b.Mutable {
		l.currentBindingName = b.Name
	}
	val, err := l.lowerExpression(b.Value)
	l.currentBindingName = ""
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
				"WFInput":        l.materializeToAttachment(val),
			},
		}
		l.emitAction(node)
		l.bindings[b.Name] = BindingRef{
			IsVariable:   true,
			VariableName: b.Name,
		}
	} else {
		// let captures the value at this source point.
		// If val is already an action output reference from the last action
		if ref, ok := val.(*ir.AttachmentToken); ok && ref.Type == "ActionOutput" {
			l.bindings[b.Name] = BindingRef{
				IsVariable:      false,
				OutputUUID:      ref.OutputUUID,
				OutputName:      b.Name,
				Aggrandizements: ref.Aggrandizements,
			}
			if len(ref.Aggrandizements) == 0 {
				for _, act := range l.workflow.Actions {
					if act.NodeID == ref.OutputUUID || act.OutputUUID == ref.OutputUUID {
						act.OutputName = b.Name
						break
					}
				}
				if l.session != nil {
					l.session.RenameOutput(ref.OutputUUID, b.Name)
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
			l.emitAction(node)
			l.bindings[b.Name] = BindingRef{
				IsVariable:      false,
				OutputUUID:      uuid,
				OutputName:      b.Name,
				Aggrandizements: ref.Aggrandizements,
			}
		} else if ref, ok := val.(*ir.AttachmentToken); ok && ref.Type == "Clipboard" {
			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.getclipboard",
				OutputUUID:      uuid,
				OutputName:      b.Name,
				Parameters:      map[string]interface{}{},
			}
			l.emitAction(node)
			l.bindings[b.Name] = BindingRef{
				IsVariable:      false,
				OutputUUID:      uuid,
				OutputName:      b.Name,
				Aggrandizements: ref.Aggrandizements,
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
				l.emitAction(node)
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
					l.emitAction(node)
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
					l.emitAction(node)
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
				l.emitAction(node)
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
				l.emitAction(node)
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
				l.emitAction(node)
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
		l.emitAction(mathNode)
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
			"WFInput":        l.materializeToAttachment(finalVal),
		},
	}
	l.emitAction(node)
	return nil
}

func (l *Lowerer) materializeToAttachment(val interface{}) *ir.AttachmentToken {
	if tok, ok := val.(*ir.AttachmentToken); ok {
		return tok
	}
	uuid := l.GenerateUUID()
	switch v := val.(type) {
	case bool:
		bNum := 0.0
		if v {
			bNum = 1.0
		}
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.number",
			OutputUUID:      uuid,
			OutputName:      "Number",
			Parameters: map[string]interface{}{
				"WFNumberActionNumber": bNum,
			},
		}
		l.emitAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "Number",
		}
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
		l.emitAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "Number",
		}
	case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		var n float64
		switch num := v.(type) {
		case int:
			n = float64(num)
		case int64:
			n = float64(num)
		case int32:
			n = float64(num)
		case int16:
			n = float64(num)
		case int8:
			n = float64(num)
		case uint:
			n = float64(num)
		case uint64:
			n = float64(num)
		case uint32:
			n = float64(num)
		case uint16:
			n = float64(num)
		case uint8:
			n = float64(num)
		}
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.number",
			OutputUUID:      uuid,
			OutputName:      "Number",
			Parameters: map[string]interface{}{
				"WFNumberActionNumber": n,
			},
		}
		l.emitAction(node)
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
		l.emitAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "Text",
		}
	case map[string]interface{}:
		if serType, ok := v["WFSerializationType"].(string); ok && serType == "WFTextTokenString" {
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.gettext",
				OutputUUID:      uuid,
				OutputName:      "Text",
				Parameters: map[string]interface{}{
					"WFTextActionText": v,
				},
			}
			l.emitAction(node)
			return &ir.AttachmentToken{
				Type:       "ActionOutput",
				OutputUUID: uuid,
				OutputName: "Text",
			}
		}
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.dictionary",
			OutputUUID:      uuid,
			OutputName:      "Dictionary",
			Parameters: map[string]interface{}{
				"WFItems": v,
			},
		}
		l.emitAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "Dictionary",
		}
	case []interface{}:
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.list",
			OutputUUID:      uuid,
			OutputName:      "List",
			Parameters: map[string]interface{}{
				"WFItems": v,
			},
		}
		l.emitAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "List",
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
		l.emitAction(node)
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: "Text",
		}
	}
}

func (l *Lowerer) lowerIf(stmt *syntax.IfStmt) error {
	_, err := l.lowerIfWithOutput(stmt, false)
	return err
}

func (l *Lowerer) lowerIfWithOutput(stmt *syntax.IfStmt, withOutput bool) (*ir.AttachmentToken, error) {
	groupUUID := l.GenerateUUID()
	var input any
	var condCode int = 4
	var compareVal any = 1.0

	// Check if Condition is a comparison BinaryExpr
	if binExpr, ok := stmt.Condition.(*syntax.BinaryExpr); ok {
		var code int
		isComp := true
		switch binExpr.Op {
		case syntax.TokenEqual:
			code = 4 // Is
		case syntax.TokenNotEqual:
			code = 5 // Not
		case syntax.TokenGreater:
			code = 2 // Greater Than
		case syntax.TokenGreaterEqual:
			code = 3 // Greater Than Or Equal
		case syntax.TokenLess:
			code = 0 // Less Than
		case syntax.TokenLessEqual:
			code = 1 // Less Than Or Equal
		default:
			isComp = false
		}

		if isComp {
			leftVal, err := l.lowerExpression(binExpr.Left)
			if err != nil {
				return nil, err
			}
			rightVal, err := l.lowerExpression(binExpr.Right)
			if err != nil {
				return nil, err
			}

			input = l.materializeToAttachment(leftVal)
			condCode = code
			compareVal = rightVal
		} else {
			condVal, err := l.lowerExpression(stmt.Condition)
			if err != nil {
				return nil, err
			}
			input = l.materializeToAttachment(condVal)
		}
	} else {
		condVal, err := l.lowerExpression(stmt.Condition)
		if err != nil {
			return nil, err
		}
		input = l.materializeToAttachment(condVal)
	}

	// 1. Begin block (Mode 0)
	l.emitIfStart(groupUUID, input, condCode, compareVal, false)

	// 2. Then block statements
	for _, ts := range stmt.ThenBlock.Statements {
		if err := l.lowerStatement(ts); err != nil {
			return nil, err
		}
	}

	// 3. Else block (Mode 1) if present
	if stmt.ElseBlock != nil {
		l.emitIfElse(groupUUID)

		if err := l.lowerStatement(stmt.ElseBlock); err != nil {
			return nil, err
		}
	}

	// 4. End block (Mode 2)
	endUUID := ""
	if withOutput {
		endUUID = l.GenerateUUID()
	}
	endNode := l.emitIfEnd(groupUUID, endUUID, "")
	if withOutput {
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: endNode.OutputUUID}, nil
	}
	return nil, nil
}

func (l *Lowerer) lowerRepeat(stmt *syntax.RepeatStmt) error {
	_, err := l.lowerRepeatWithOutput(stmt, false)
	return err
}

func (l *Lowerer) lowerRepeatWithOutput(stmt *syntax.RepeatStmt, withOutput bool) (*ir.AttachmentToken, error) {
	l.repeatDepth++
	defer func() { l.repeatDepth-- }()

	groupUUID := l.GenerateUUID()
	countVal, err := l.lowerExpression(stmt.Count)
	if err != nil {
		return nil, err
	}

	l.emitRepeatCountStart(groupUUID, countVal)

	repeatIndexName := "Repeat Index"
	if l.repeatDepth > 1 {
		repeatIndexName = fmt.Sprintf("Repeat Index %d", l.repeatDepth)
	}

	var prevBinding BindingRef
	var hadBinding bool
	if stmt.IndexVar != "" {
		prevBinding, hadBinding = l.bindings[stmt.IndexVar]
		mathUUID := l.GenerateUUID()
		mathNode := &ir.NativeActionNode{
			NodeID:          mathUUID,
			AppleIdentifier: "is.workflow.actions.math",
			OutputUUID:      mathUUID,
			OutputName:      stmt.IndexVar,
			Parameters: map[string]interface{}{
				"WFMathOperation": "-",
				"WFInput": &ir.AttachmentToken{
					Type:       "Variable",
					OutputName: repeatIndexName,
				},
				"WFMathOperand": 1.0,
			},
		}
		l.emitAction(mathNode)
		l.bindings[stmt.IndexVar] = BindingRef{
			IsVariable: false,
			OutputUUID: mathUUID,
			OutputName: stmt.IndexVar,
		}
	}

	for _, s := range stmt.BodyBlock.Statements {
		if err := l.lowerStatement(s); err != nil {
			return nil, err
		}
	}

	if stmt.IndexVar != "" {
		if hadBinding {
			l.bindings[stmt.IndexVar] = prevBinding
		} else {
			delete(l.bindings, stmt.IndexVar)
		}
	}

	var endUUID string
	if withOutput {
		endUUID = l.GenerateUUID()
	}
	l.emitLoopEnd(groupUUID, endUUID, false)
	if withOutput {
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: endUUID}, nil
	}
	return nil, nil
}

func (l *Lowerer) lowerFor(stmt *syntax.ForStmt) error {
	_, err := l.lowerForWithOutput(stmt, false)
	return err
}

func (l *Lowerer) lowerForWithOutput(stmt *syntax.ForStmt, withOutput bool) (*ir.AttachmentToken, error) {
	l.repeatDepth++
	defer func() { l.repeatDepth-- }()

	groupUUID := l.GenerateUUID()
	iterVal, err := l.lowerExpression(stmt.Iterable)
	if err != nil {
		return nil, err
	}

	iterToken := l.materializeToAttachment(iterVal)

	l.emitRepeatEachStart(groupUUID, iterToken)

	repeatItemName := "Repeat Item"
	repeatIndexName := "Repeat Index"
	if l.repeatDepth > 1 {
		repeatItemName = fmt.Sprintf("Repeat Item %d", l.repeatDepth)
		repeatIndexName = fmt.Sprintf("Repeat Index %d", l.repeatDepth)
	}

	var prevItemBinding BindingRef
	var hadItemBinding bool
	if stmt.ItemVar != "" {
		prevItemBinding, hadItemBinding = l.bindings[stmt.ItemVar]
		setVarNode := &ir.NativeActionNode{
			NodeID:          l.GenerateUUID(),
			AppleIdentifier: "is.workflow.actions.setvariable",
			Parameters: map[string]interface{}{
				"WFVariableName": stmt.ItemVar,
				"WFInput": &ir.AttachmentToken{
					Type:       "Variable",
					OutputName: repeatItemName,
				},
			},
		}
		l.emitAction(setVarNode)
		l.bindings[stmt.ItemVar] = BindingRef{
			IsVariable:   true,
			VariableName: stmt.ItemVar,
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
			OutputName:      stmt.IndexVar,
			Parameters: map[string]interface{}{
				"WFMathOperation": "-",
				"WFInput": &ir.AttachmentToken{
					Type:       "Variable",
					OutputName: repeatIndexName,
				},
				"WFMathOperand": 1.0,
			},
		}
		l.emitAction(mathNode)
		l.bindings[stmt.IndexVar] = BindingRef{
			IsVariable: false,
			OutputUUID: mathUUID,
			OutputName: stmt.IndexVar,
		}
	}

	for _, s := range stmt.BodyBlock.Statements {
		if err := l.lowerStatement(s); err != nil {
			return nil, err
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

	var endUUID string
	if withOutput {
		endUUID = l.GenerateUUID()
	}
	l.emitLoopEnd(groupUUID, endUUID, true)
	if withOutput {
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: endUUID}, nil
	}
	return nil, nil
}

func (l *Lowerer) lowerMenu(stmt *syntax.MenuStmt) error {
	_, err := l.lowerMenuWithOutput(stmt, false)
	return err
}

func (l *Lowerer) lowerMenuWithOutput(stmt *syntax.MenuStmt, withOutput bool) (*ir.AttachmentToken, error) {
	groupUUID := l.GenerateUUID()
	promptVal, err := l.lowerExpression(stmt.Prompt)
	if err != nil {
		return nil, err
	}

	menuItems := make([]string, len(stmt.Cases))
	for i, c := range stmt.Cases {
		menuItems[i] = c.Label
	}

	l.emitMenuStart(groupUUID, promptVal, menuItems)

	for _, c := range stmt.Cases {
		l.emitMenuCase(groupUUID, c.Label)

		for _, s := range c.BodyBlock.Statements {
			if err := l.lowerStatement(s); err != nil {
				return nil, err
			}
		}
	}

	var endUUID string
	if withOutput {
		endUUID = l.GenerateUUID()
	}
	l.emitMenuEnd(groupUUID, endUUID)
	if withOutput {
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: endUUID}, nil
	}
	return nil, nil
}

func (l *Lowerer) lowerExpression(expr syntax.Expression) (interface{}, error) {
	if expr == nil {
		return nil, nil
	}

	switch e := expr.(type) {
	case *syntax.IfStmt:
		return l.lowerIfWithOutput(e, true)
	case *syntax.RepeatStmt:
		return l.lowerRepeatWithOutput(e, true)
	case *syntax.ForStmt:
		return l.lowerForWithOutput(e, true)
	case *syntax.MenuStmt:
		return l.lowerMenuWithOutput(e, true)

	case *syntax.LiteralExpr:
		switch e.Type {
		case syntax.TokenString, syntax.TokenRawString:
			return e.Value, nil
		case syntax.TokenNumber:
			if !strings.ContainsAny(e.Value, ".eE") {
				if i, err := strconv.ParseInt(e.Value, 10, 64); err == nil {
					return i, nil
				}
				if u, err := strconv.ParseUint(e.Value, 10, 64); err == nil {
					return u, nil
				}
			}
			if f, err := strconv.ParseFloat(e.Value, 64); err == nil {
				return f, nil
			}
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
				tok := &ir.AttachmentToken{
					Type:       "Variable",
					OutputName: ref.VariableName,
				}
				if len(ref.Aggrandizements) > 0 {
					tok.Aggrandizements = make([]map[string]interface{}, len(ref.Aggrandizements))
					copy(tok.Aggrandizements, ref.Aggrandizements)
				}
				return tok, nil
			}
			tok := &ir.AttachmentToken{
				Type:       "ActionOutput",
				OutputUUID: ref.OutputUUID,
				OutputName: ref.OutputName,
			}
			if len(ref.Aggrandizements) > 0 {
				tok.Aggrandizements = make([]map[string]interface{}, len(ref.Aggrandizements))
				copy(tok.Aggrandizements, ref.Aggrandizements)
			}
			return tok, nil
		}
		if q, isQ := l.setupQuestions[e.Name]; isQ {
			return q.DefaultValue, nil
		}
		// Built-in / system checks
		if e.Name == "clipboard" {
			return &ir.AttachmentToken{Type: "Clipboard"}, nil
		}
		if e.Name == "ShortcutInput" {
			if len(l.functions) > 0 {
				return &ir.AttachmentToken{Type: "Variable", OutputName: "_cherri_caller_input"}, nil
			}
			return &ir.AttachmentToken{Type: "ExtensionInput", OutputName: "ShortcutInput"}, nil
		}
		if e.Name == "Ask" {
			return &ir.AttachmentToken{Type: "Ask"}, nil
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
			kVal, err := l.lowerExpression(entry.Key)
			if err != nil {
				return nil, err
			}
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
			l.emitIfStart(groupUUID, operand, 4, 1.0, false)
			fNode := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 0.0},
			}
			l.emitAction(fNode)
			l.emitIfElse(groupUUID)
			tNode := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 1.0},
			}
			l.emitAction(tNode)
			l.emitIfEnd(groupUUID, uuid, "")
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
			l.emitAction(node)
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
		if tok, ok := targetVal.(*ir.AttachmentToken); ok {
			if strKey, ok := indexVal.(string); ok {
				newTok := &ir.AttachmentToken{
					Type:            tok.Type,
					OutputUUID:      tok.OutputUUID,
					OutputName:      tok.OutputName,
					Aggrandizements: make([]map[string]interface{}, len(tok.Aggrandizements)),
				}
				copy(newTok.Aggrandizements, tok.Aggrandizements)
				newTok.Aggrandizements = append(newTok.Aggrandizements, map[string]interface{}{
					"Type":          "WFDictionaryValueVariableAggrandizement",
					"DictionaryKey": strKey,
				})
				return newTok, nil
			}
		}
		if n, ok := indexVal.(float64); ok {
			// Safe 0-based indexing: negative or non-integer index produces nothing without unsafe native lookup
			if n < 0 || n != math.Trunc(n) {
				uuid := l.GenerateUUID()
				l.emitAction(&ir.NativeActionNode{
					NodeID:          uuid,
					AppleIdentifier: "is.workflow.actions.nothing",
					OutputUUID:      uuid,
					Parameters:      map[string]interface{}{},
				})
				return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
			}
			targetTok := l.materializeToAttachment(targetVal)
			oneBased := n + 1.0
			countUUID := l.GenerateUUID()
			l.emitAction(&ir.NativeActionNode{
				NodeID:          countUUID,
				AppleIdentifier: "is.workflow.actions.count",
				OutputUUID:      countUUID,
				Parameters: map[string]interface{}{
					"Input":       targetTok,
					"WFCountType": "Items",
				},
			})

			groupUUID := l.GenerateUUID()
			endUUID := l.GenerateUUID()
			l.emitIfStart(groupUUID, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: countUUID}, 3, oneBased, false)

			getUUID := l.GenerateUUID()
			l.emitAction(&ir.NativeActionNode{
				NodeID:          getUUID,
				AppleIdentifier: "is.workflow.actions.getitemfromlist",
				OutputUUID:      getUUID,
				Parameters: map[string]interface{}{
					"WFInput":         targetTok,
					"WFItemSpecifier": "Item At Index",
					"WFItemIndex":     oneBased,
				},
			})

			l.emitIfElse(groupUUID)
			l.emitAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.nothing",
				Parameters:      map[string]interface{}{},
			})

			l.emitIfEnd(groupUUID, endUUID, "")
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: endUUID}, nil
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
			l.emitAction(node)
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
		} else {
			// Dynamic index: add 1 and guard 1 <= idx+1 <= count(list)
			targetTok := l.materializeToAttachment(targetVal)
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
			l.emitAction(mathNode)
			oneBasedTok := &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: mathUUID}

			countUUID := l.GenerateUUID()
			l.emitAction(&ir.NativeActionNode{
				NodeID:          countUUID,
				AppleIdentifier: "is.workflow.actions.count",
				OutputUUID:      countUUID,
				Parameters: map[string]interface{}{
					"Input":       targetTok,
					"WFCountType": "Items",
				},
			})
			countTok := &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: countUUID}

			diffUUID := l.GenerateUUID()
			l.emitAction(&ir.NativeActionNode{
				NodeID:          diffUUID,
				AppleIdentifier: "is.workflow.actions.math",
				OutputUUID:      diffUUID,
				Parameters: map[string]interface{}{
					"WFMathOperation": "-",
					"WFInput":         countTok,
					"WFMathOperand":   oneBasedTok,
				},
			})
			diffTok := &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: diffUUID}

			outerGroup := l.GenerateUUID()
			outerEndUUID := l.GenerateUUID()
			l.emitIfStart(outerGroup, oneBasedTok, 3, 1.0, false)

			innerGroup := l.GenerateUUID()
			innerEndUUID := l.GenerateUUID()
			l.emitIfStart(innerGroup, diffTok, 3, 0.0, false)

			uuid := l.GenerateUUID()
			node := &ir.NativeActionNode{
				NodeID:          uuid,
				AppleIdentifier: "is.workflow.actions.getitemfromlist",
				OutputUUID:      uuid,
				Parameters: map[string]interface{}{
					"WFInput":         targetTok,
					"WFItemSpecifier": "Item At Index",
					"WFItemIndex":     oneBasedTok,
				},
			}
			l.emitAction(node)

			l.emitIfElse(innerGroup)
			l.emitAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.nothing",
				Parameters:      map[string]interface{}{},
			})
			l.emitIfEnd(innerGroup, innerEndUUID, "")

			l.emitIfElse(outerGroup)
			l.emitAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.nothing",
				Parameters:      map[string]interface{}{},
			})
			l.emitIfEnd(outerGroup, outerEndUUID, "")
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: outerEndUUID}, nil
		}

	case *syntax.EnumMemberExpr:
		if enumDecl, isEnum := l.enums[e.TypeName]; isEnum {
			for _, m := range enumDecl.Members {
				if m.Name == e.Member {
					if m.WireValue != "" {
						return m.WireValue, nil
					}
					return m.Name, nil
				}
			}
			return nil, fmt.Errorf("enum %q has no member %q", e.TypeName, e.Member)
		}
		return nil, fmt.Errorf("unknown enum %q", e.TypeName)

	case *syntax.MemberExpr:
		if targetIdent, ok := e.Target.(*syntax.IdentExpr); ok {
			if enumDecl, isEnum := l.enums[targetIdent.Name]; isEnum {
				for _, m := range enumDecl.Members {
					if m.Name == e.Property {
						if m.WireValue != "" {
							return m.WireValue, nil
						}
						return m.Name, nil
					}
				}
				return nil, fmt.Errorf("enum %q has no member %q", targetIdent.Name, e.Property)
			}
			if targetIdent.Name == "system" {
				switch e.Property {
				case "input":
					if len(l.functions) > 0 {
						return &ir.AttachmentToken{Type: "Variable", OutputName: "_cherri_caller_input"}, nil
					}
					return &ir.AttachmentToken{Type: "ExtensionInput", OutputName: "ShortcutInput"}, nil
				case "clipboard":
					return &ir.AttachmentToken{Type: "Clipboard"}, nil
				}
			}
		}
		targetVal, err := l.lowerExpression(e.Target)
		if err != nil {
			return nil, err
		}
		if tok, ok := targetVal.(*ir.AttachmentToken); ok {
			newTok := &ir.AttachmentToken{
				Type:            tok.Type,
				OutputUUID:      tok.OutputUUID,
				OutputName:      tok.OutputName,
				Aggrandizements: make([]map[string]interface{}, len(tok.Aggrandizements)),
			}
			copy(newTok.Aggrandizements, tok.Aggrandizements)
			if itemClass, ok := contentItemClasses[strings.ToLower(e.Property)]; ok {
				newTok.Aggrandizements = append(newTok.Aggrandizements, map[string]interface{}{
					"Type":              "WFCoercionVariableAggrandizement",
					"CoercionItemClass": itemClass,
				})
			} else {
				newTok.Aggrandizements = append(newTok.Aggrandizements, map[string]interface{}{
					"Type":         "WFPropertyVariableAggrandizement",
					"PropertyName": e.Property,
				})
			}
			return newTok, nil
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
		l.emitAction(node)
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil

	case *syntax.BinaryExpr:
		left, err := l.lowerExpression(e.Left)
		if err != nil {
			return nil, err
		}

		// Logical short-circuiting (compile-time and dynamic):
		if e.Op == syntax.TokenAnd {
			if b, ok := left.(bool); ok {
				if !b {
					return false, nil
				}
				return l.lowerExpression(e.Right)
			}
			// Dynamic short-circuiting for &&: evaluate e.Right only inside Then branch (when left == 1.0)
			groupUUID := l.GenerateUUID()
			endUUID := l.GenerateUUID()
			l.emitIfStart(groupUUID, l.materializeToAttachment(left), 4, 1.0, false)
			rightVal, err := l.lowerExpression(e.Right)
			if err != nil {
				return nil, err
			}
			l.materializeYieldValue(rightVal)

			l.emitIfElse(groupUUID)
			l.emitAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 0.0},
			})
			l.emitIfEnd(groupUUID, endUUID, "")
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: endUUID}, nil
		} else if e.Op == syntax.TokenOr {
			if b, ok := left.(bool); ok {
				if b {
					return true, nil
				}
				return l.lowerExpression(e.Right)
			}
			// Dynamic short-circuiting for ||: evaluate e.Right only inside Else branch (when left != 1.0)
			groupUUID := l.GenerateUUID()
			endUUID := l.GenerateUUID()
			l.emitIfStart(groupUUID, l.materializeToAttachment(left), 4, 1.0, false)
			l.emitAction(&ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 1.0},
			})
			l.emitIfElse(groupUUID)
			rightVal, err := l.lowerExpression(e.Right)
			if err != nil {
				return nil, err
			}
			l.materializeYieldValue(rightVal)

			l.emitIfEnd(groupUUID, endUUID, "")
			return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: endUUID}, nil
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
			l.emitIfStart(groupUUID, left, condCode, right, false)

			// Then branch: 1.0 (true)
			tNode := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 1.0},
			}
			l.emitAction(tNode)

			// Else branch: 0.0 (false)
			l.emitIfElse(groupUUID)

			fNode := &ir.NativeActionNode{
				NodeID:          l.GenerateUUID(),
				AppleIdentifier: "is.workflow.actions.number",
				Parameters:      map[string]interface{}{"WFNumberActionNumber": 0.0},
			}
			l.emitAction(fNode)

			l.emitIfEnd(groupUUID, uuid, "")
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
			l.emitAction(node)
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
		l.emitAction(node)
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

func (l *Lowerer) lowerSemanticValue(expr syntax.Expression) (backend.SemanticValue, error) {
	if m, ok := expr.(*syntax.MapExpr); ok {
		entries := make([]backend.DictEntry, 0, len(m.Entries))
		for _, entry := range m.Entries {
			kVal, err := l.lowerExpression(entry.Key)
			if err != nil {
				return backend.SemanticValue{}, err
			}
			vVal, err := l.lowerSemanticValue(entry.Value)
			if err != nil {
				return backend.SemanticValue{}, err
			}
			entries = append(entries, backend.DictEntry{
				Key:   fmt.Sprint(kVal),
				Value: vVal,
			})
		}
		return backend.SemanticValue{
			Kind:    backend.ValDict,
			DictVal: entries,
		}, nil
	}
	if list, ok := expr.(*syntax.ListExpr); ok {
		items := make([]backend.SemanticValue, 0, len(list.Elements))
		for _, el := range list.Elements {
			itemVal, err := l.lowerSemanticValue(el)
			if err != nil {
				return backend.SemanticValue{}, err
			}
			items = append(items, itemVal)
		}
		return backend.SemanticValue{
			Kind:    backend.ValList,
			ListVal: items,
		}, nil
	}
	if f, ok := expr.(*syntax.FStringExpr); ok {
		var segments []backend.TextSegment
		hasRef := false
		for _, part := range f.Parts {
			if !part.IsExpr {
				segments = append(segments, backend.TextSegment{
					IsExpr: false,
					Text:   part.Text,
				})
			} else {
				val, err := l.lowerExpression(part.Expr)
				if err != nil {
					return backend.SemanticValue{}, err
				}
				semVal := l.toSemanticValue(val)
				if semVal.Kind == backend.ValReference {
					hasRef = true
					segments = append(segments, backend.TextSegment{
						IsExpr:    true,
						Reference: semVal.Ref,
					})
				} else {
					segments = append(segments, backend.TextSegment{
						IsExpr: false,
						Text:   formatSemanticLiteral(semVal),
					})
				}
			}
		}
		if hasRef {
			return backend.SemanticValue{
				Kind:     backend.ValTextSegments,
				Segments: segments,
			}, nil
		}
		var full strings.Builder
		for _, seg := range segments {
			full.WriteString(seg.Text)
		}
		return backend.SemanticValue{
			Kind:   backend.ValString,
			StrVal: full.String(),
		}, nil
	}

	val, err := l.lowerExpression(expr)
	if err != nil {
		return backend.SemanticValue{}, err
	}
	return l.toSemanticValue(val), nil
}

func formatSemanticLiteral(val backend.SemanticValue) string {
	switch val.Kind {
	case backend.ValBool:
		return fmt.Sprintf("%t", val.BoolVal)
	case backend.ValInt:
		return fmt.Sprintf("%d", val.IntVal)
	case backend.ValFloat:
		if val.FloatVal == float64(int64(val.FloatVal)) {
			return fmt.Sprintf("%d", int64(val.FloatVal))
		}
		return fmt.Sprintf("%g", val.FloatVal)
	case backend.ValString:
		return val.StrVal
	default:
		return ""
	}
}

func (l *Lowerer) toSemanticValue(val interface{}) backend.SemanticValue {
	if val == nil {
		return backend.SemanticValue{Kind: backend.ValNil}
	}
	switch v := val.(type) {
	case bool:
		return backend.SemanticValue{Kind: backend.ValBool, BoolVal: v}
	case int:
		return backend.SemanticValue{Kind: backend.ValInt, IntVal: int64(v)}
	case int64:
		return backend.SemanticValue{Kind: backend.ValInt, IntVal: v}
	case float64:
		return backend.SemanticValue{Kind: backend.ValFloat, FloatVal: v}
	case string:
		return backend.SemanticValue{Kind: backend.ValString, StrVal: v}
	case *ir.AttachmentToken:
		var kind backend.ReferenceKind
		switch v.Type {
		case "ActionOutput":
			kind = backend.RefActionResult
		case "Variable":
			kind = backend.RefMutableBinding
		case "ExtensionInput":
			kind = backend.RefExtensionInput
		case "Ask":
			kind = backend.RefAskEachTime
		default:
			kind = backend.RefActionResult
		}
		var transforms []backend.Transformation
		if len(v.Aggrandizements) > 0 {
			for _, m := range v.Aggrandizements {
				t := backend.Transformation{
					Type: fmt.Sprintf("%v", m["Type"]),
				}
				if prop, ok := m["PropertyName"].(string); ok {
					t.PropertyName = prop
				}
				if cClass, ok := m["CoercionItemClass"].(string); ok {
					t.CoercionItemClass = cClass
				}
				if dKey, ok := m["DictionaryKey"].(string); ok {
					t.DictionaryKey = dKey
				}
				transforms = append(transforms, t)
			}
		}
		return backend.SemanticValue{
			Kind: backend.ValReference,
			Ref: &backend.Reference{
				Kind:            kind,
				ProducerID:      v.OutputUUID,
				ProducerName:    v.OutputName,
				Transformations: transforms,
			},
		}
	case *backend.Reference:
		return backend.SemanticValue{
			Kind: backend.ValReference,
			Ref:  v,
		}
	case []interface{}:
		items := make([]backend.SemanticValue, len(v))
		for i, el := range v {
			items[i] = l.toSemanticValue(el)
		}
		return backend.SemanticValue{Kind: backend.ValList, ListVal: items}
	case map[string]interface{}:
		entries := make([]backend.DictEntry, 0, len(v))
		for k, el := range v {
			entries = append(entries, backend.DictEntry{Key: k, Value: l.toSemanticValue(el)})
		}
		return backend.SemanticValue{Kind: backend.ValDict, DictVal: entries}
	default:
		return backend.SemanticValue{Kind: backend.ValRawNative, RawNativeVal: v}
	}
}

func (l *Lowerer) lowerCall(call *syntax.CallExpr) (interface{}, error) {
	var actionName string
	switch c := call.Callee.(type) {
	case *syntax.IdentExpr:
		actionName = c.Name
	case *syntax.MemberExpr:
		if targetIdent, ok := c.Target.(*syntax.IdentExpr); ok && targetIdent.Name == "native" {
			actionName = "native." + c.Property
		} else if targetIdent, ok := c.Target.(*syntax.IdentExpr); ok {
			fullName := targetIdent.Name + "." + c.Property
			if _, ok := l.functions[fullName]; ok {
				actionName = fullName
			} else if _, ok := l.functions[c.Property]; ok {
				actionName = c.Property
			} else {
				actionName = c.Property
			}
		} else {
			actionName = c.Property
		}
	default:
		return nil, fmt.Errorf("complex callee not supported in lowering")
	}

	// Check native escape: native.action, action, rawAction
	if actionName == "native.action" || actionName == "action" || actionName == "rawAction" {
		uuid := l.GenerateUUID()
		outputUUID := ""
		if l.currentBindingName != "" {
			outputUUID = uuid
		}
		node := &ir.NativeActionNode{
			NodeID:     uuid,
			OutputUUID: outputUUID,
			OutputName: l.currentBindingName,
			Parameters: make(map[string]interface{}),
		}
		if call.PrimaryArg != nil {
			val, err := l.lowerExpression(call.PrimaryArg)
			if err != nil {
				return nil, fmt.Errorf("failed to lower action identifier: %w", err)
			}
			if s, ok := val.(string); ok {
				node.AppleIdentifier = s
			} else {
				node.AppleIdentifier = fmt.Sprintf("%v", val)
			}
		}
		for _, arg := range call.NamedArgs {
			if arg.Label == "identifier" {
				val, err := l.lowerExpression(arg.Value)
				if err != nil {
					return nil, fmt.Errorf("failed to lower action identifier: %w", err)
				}
				if s, ok := val.(string); ok {
					node.AppleIdentifier = s
				} else {
					node.AppleIdentifier = fmt.Sprintf("%v", val)
				}
			} else if arg.Label == "parameters" || arg.Label == "" {
				val, err := l.lowerExpression(arg.Value)
				if err != nil {
					return nil, fmt.Errorf("failed to lower action parameters: %w", err)
				}
				if m, ok := val.(map[string]interface{}); ok {
					for k, v := range m {
						node.Parameters[k] = v
					}
				} else if m, ok := val.(map[string]any); ok {
					for k, v := range m {
						node.Parameters[k] = v
					}
				} else {
					return nil, fmt.Errorf("parameters argument to %s must be a dictionary, got %T", actionName, val)
				}
			} else if arg.Label == "uuid" || arg.Label == "outputUUID" {
				val, err := l.lowerExpression(arg.Value)
				if err != nil {
					return nil, err
				}
				node.OutputUUID = fmt.Sprint(val)
			} else if arg.Label == "outputName" || arg.Label == "customOutputName" {
				val, err := l.lowerExpression(arg.Value)
				if err != nil {
					return nil, err
				}
				node.OutputName = fmt.Sprint(val)
			} else if arg.Label == "groupingIdentifier" {
				val, err := l.lowerExpression(arg.Value)
				if err != nil {
					return nil, err
				}
				node.GroupingIdentifier = fmt.Sprint(val)
			} else {
				val, err := l.lowerExpression(arg.Value)
				if err != nil {
					return nil, fmt.Errorf("failed to lower parameter %s: %w", arg.Label, err)
				}
				node.Parameters[arg.Label] = val
			}
		}
		if strings.TrimSpace(node.AppleIdentifier) == "" {
			return nil, fmt.Errorf("%s requires a non-empty action identifier", actionName)
		}
		l.emitAction(node)
		if l.emitErr != nil {
			return nil, l.emitErr
		}
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
	}
	if actionName == "native.plist" {
		var targetExpr syntax.Expression
		if call.PrimaryArg != nil {
			targetExpr = call.PrimaryArg
		} else {
			for _, arg := range call.NamedArgs {
				if arg.Label == "data" || arg.Label == "plist" || arg.Label == "value" || arg.Label == "" {
					targetExpr = arg.Value
					break
				}
			}
		}
		if targetExpr == nil {
			return nil, fmt.Errorf("native.plist requires a plist expression or dictionary")
		}
		val, err := l.lowerExpression(targetExpr)
		if err != nil {
			return nil, fmt.Errorf("failed to lower native.plist data: %w", err)
		}
		if str, ok := val.(string); ok && (strings.HasPrefix(strings.TrimSpace(str), "<?xml") || strings.HasPrefix(strings.TrimSpace(str), "<plist")) {
			var parsed any
			if _, err := plist.Unmarshal([]byte(str), &parsed); err != nil {
				return nil, fmt.Errorf("native.plist failed to parse XML plist: %w", err)
			}
			return &ir.RawPlistValue{Data: parsed}, nil
		}
		return &ir.RawPlistValue{Data: val}, nil
	}
	if actionName == "native.ref" {
		var kind string
		var outputUUID string
		var outputName string
		var aggrs []map[string]interface{}

		if call.PrimaryArg != nil {
			val, err := l.lowerExpression(call.PrimaryArg)
			if err != nil {
				return nil, fmt.Errorf("failed to lower native.ref kind: %w", err)
			}
			kind = fmt.Sprint(val)
		}

		for _, arg := range call.NamedArgs {
			val, err := l.lowerExpression(arg.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to lower native.ref argument %s: %w", arg.Label, err)
			}
			switch arg.Label {
			case "kind", "type":
				kind = fmt.Sprint(val)
			case "uuid", "outputUUID":
				outputUUID = fmt.Sprint(val)
			case "name", "variable", "outputName":
				outputName = fmt.Sprint(val)
			case "aggrandizements":
				if list, ok := val.([]interface{}); ok {
					for _, item := range list {
						if m, ok := item.(map[string]interface{}); ok {
							aggrs = append(aggrs, m)
						} else if m, ok := item.(map[string]any); ok {
							aggrs = append(aggrs, m)
						}
					}
				}
			case "coerce", "coercion":
				aggrs = append(aggrs, map[string]interface{}{
					"Type":              "WFCoercionVariableAggrandizement",
					"CoercionItemClass": fmt.Sprint(val),
				})
			case "property":
				aggrs = append(aggrs, map[string]interface{}{
					"Type":         "WFPropertyVariableAggrandizement",
					"PropertyName": fmt.Sprint(val),
				})
			case "key", "dictionaryKey":
				aggrs = append(aggrs, map[string]interface{}{
					"Type":          "WFDictionaryValueVariableAggrandizement",
					"DictionaryKey": fmt.Sprint(val),
				})
			}
		}

		normKind := strings.ToLower(strings.TrimSpace(kind))
		switch normKind {
		case "actionoutput", "output":
			kind = "ActionOutput"
			if strings.TrimSpace(outputUUID) == "" {
				return nil, fmt.Errorf("native.ref for ActionOutput requires a non-empty uuid")
			}
		case "variable", "var":
			kind = "Variable"
			if strings.TrimSpace(outputName) == "" {
				return nil, fmt.Errorf("native.ref for Variable requires a non-empty name")
			}
		case "shortcutinput", "extensioninput", "input":
			kind = "ExtensionInput"
		case "clipboard":
			kind = "Clipboard"
		case "ask", "askeachtime":
			kind = "Ask"
		default:
			if kind == "" {
				return nil, fmt.Errorf("native.ref requires a reference kind (e.g. ActionOutput, Variable, ShortcutInput)")
			}
			return nil, fmt.Errorf("unsupported or invalid native.ref kind: %q", kind)
		}

		tok := &ir.AttachmentToken{
			Type:            kind,
			OutputUUID:      outputUUID,
			OutputName:      outputName,
			Aggrandizements: aggrs,
		}
		return tok, nil
	}
	if actionName == "native.workflow" {
		var targetExpr syntax.Expression
		if call.PrimaryArg != nil {
			targetExpr = call.PrimaryArg
		} else {
			for _, arg := range call.NamedArgs {
				if arg.Label == "workflow" || arg.Label == "data" || arg.Label == "plist" || arg.Label == "" {
					targetExpr = arg.Value
					break
				}
			}
		}
		if targetExpr == nil {
			return nil, fmt.Errorf("native.workflow requires a workflow dictionary or plist data")
		}
		val, err := l.lowerExpression(targetExpr)
		if err != nil {
			return nil, fmt.Errorf("failed to lower native.workflow: %w", err)
		}
		var rawMap map[string]interface{}
		if str, ok := val.(string); ok && (strings.HasPrefix(strings.TrimSpace(str), "<?xml") || strings.HasPrefix(strings.TrimSpace(str), "<plist")) {
			var parsed any
			if _, err := plist.Unmarshal([]byte(str), &parsed); err != nil {
				return nil, fmt.Errorf("native.workflow failed to parse XML plist: %w", err)
			}
			if m, ok := parsed.(map[string]interface{}); ok {
				rawMap = m
			} else if m, ok := parsed.(map[string]any); ok {
				rawMap = m
			}
		} else if m, ok := val.(map[string]interface{}); ok {
			rawMap = m
		} else if m, ok := val.(map[string]any); ok {
			rawMap = m
		} else {
			return nil, fmt.Errorf("native.workflow argument must be a workflow dictionary or XML plist string, got %T", val)
		}

		l.workflow.IsFullNativePass = true
		l.workflow.NativeWorkflowRaw = rawMap
		if l.session != nil {
			l.session.SetMetadata("rawWorkflow", rawMap)
		}

		if v, ok := rawMap["WFWorkflowClientVersion"].(string); ok {
			l.workflow.ClientVersion = v
			if l.session != nil {
				l.session.SetMetadata("clientVersion", v)
			}
		}
		if types, ok := rawMap["WFWorkflowTypes"].([]string); ok {
			l.workflow.WorkflowTypes = types
			if l.session != nil {
				l.session.SetMetadata("workflowTypes", types)
			}
		}
		if classes, ok := rawMap["WFWorkflowInputContentItemClasses"].([]string); ok {
			l.workflow.InputContentItemClasses = classes
			if l.session != nil {
				l.session.SetMetadata("inputContentItemClasses", classes)
			}
		}

		if acts, ok := rawMap["WFWorkflowActions"].([]interface{}); ok {
			for _, act := range acts {
				if actMap, ok := act.(map[string]interface{}); ok {
					ident, _ := actMap["WFWorkflowActionIdentifier"].(string)
					params, _ := actMap["WFWorkflowActionParameters"].(map[string]interface{})
					if params == nil {
						params = make(map[string]interface{})
					}
					node := &ir.NativeActionNode{
						NodeID:          l.GenerateUUID(),
						AppleIdentifier: ident,
						Parameters:      params,
					}
					if u, ok := params["UUID"].(string); ok {
						node.OutputUUID = u
					}
					if n, ok := params["CustomOutputName"].(string); ok {
						node.OutputName = n
					}
					if g, ok := params["GroupingIdentifier"].(string); ok {
						node.GroupingIdentifier = g
					}
					l.emitAction(node)
					if l.emitErr != nil {
						return nil, l.emitErr
					}
				}
			}
		}
		return nil, nil
	}

	// Check user-declared action
	if actDecl, ok := l.actions[actionName]; ok {
		uuid := l.GenerateUUID()
		outputUUID := ""
		if l.currentBindingName != "" {
			outputUUID = uuid
		}
		params := make(map[string]interface{})
		if call.PrimaryArg != nil && actDecl.PrimaryParam != "" {
			val, err := l.lowerExpression(call.PrimaryArg)
			if err != nil {
				return nil, err
			}
			wireKey := actDecl.PrimaryParam
			if b, hasB := actDecl.Bindings[actDecl.PrimaryParam]; hasB && b.WireKey != "" {
				wireKey = b.WireKey
			}
			params[wireKey] = val
		}
		for _, arg := range call.NamedArgs {
			val, err := l.lowerExpression(arg.Value)
			if err != nil {
				return nil, err
			}
			wireKey := arg.Label
			if b, hasB := actDecl.Bindings[arg.Label]; hasB && b.WireKey != "" {
				wireKey = b.WireKey
			}
			params[wireKey] = val
		}
		node := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: actDecl.AppleIdentifier,
			OutputUUID:      outputUUID,
			OutputName:      l.currentBindingName,
			Parameters:      params,
		}
		l.emitAction(node)
		return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}, nil
	}

	// Check user-defined function
	if fn, ok := l.functions[actionName]; ok {
		argsList := make([]interface{}, len(fn.Parameters))
		dispatchDict := map[string]interface{}{
			"_cherri_magic":        "__cherri_fn_envelope_v2__",
			"cherri_functions":     "1",
			"function":             fn.Name,
			"_cherri_caller_input": &ir.AttachmentToken{Type: "Variable", OutputName: "_cherri_caller_input"},
		}
		for i, param := range fn.Parameters {
			var argVal interface{}
			supplied := false
			if i == 0 && call.PrimaryArg != nil {
				v, err := l.lowerExpression(call.PrimaryArg)
				if err != nil {
					return nil, err
				}
				argVal = v
				supplied = true
			}
			if !supplied {
				for _, nArg := range call.NamedArgs {
					if nArg.Label == param.Name {
						v, err := l.lowerExpression(nArg.Value)
						if err != nil {
							return nil, err
						}
						argVal = v
						supplied = true
						break
					}
				}
			}
			if supplied {
				if b, isBool := argVal.(bool); isBool {
					if b {
						argVal = 1.0
					} else {
						argVal = 0.0
					}
				}
				dispatchDict[fmt.Sprintf("has_arg_%d", i)] = "1"
				dispatchDict[fmt.Sprintf("arg_%d", i)] = argVal
				argsList[i] = argVal
			} else {
				dispatchDict[fmt.Sprintf("has_arg_%d", i)] = "0"
				if param.DefaultExpr != nil {
					v, err := l.lowerExpression(param.DefaultExpr)
					if err != nil {
						return nil, err
					}
					if b, isBool := v.(bool); isBool {
						if b {
							v = 1.0
						} else {
							v = 0.0
						}
					}
					argsList[i] = v
				}
			}
		}
		dispatchDict["arguments"] = argsList

		dispatchUUID := l.GenerateUUID()
		dispatchNode := &ir.NativeActionNode{
			NodeID:          dispatchUUID,
			AppleIdentifier: "is.workflow.actions.dictionary",
			OutputUUID:      dispatchUUID,
			OutputName:      "FunctionPayload",
			Parameters: map[string]interface{}{
				"WFItems": dispatchDict,
			},
		}
		l.emitAction(dispatchNode)

		wfTarget := map[string]interface{}{
			"isSelf": true,
		}
		if l.WorkflowName != "" {
			wfTarget["workflowName"] = l.WorkflowName
		}

		runParams := map[string]interface{}{
			"WFWorkflow": wfTarget,
			"WFInput": &ir.AttachmentToken{
				Type:       "ActionOutput",
				OutputUUID: dispatchUUID,
				OutputName: "FunctionPayload",
			},
		}
		if l.WorkflowName != "" {
			runParams["WFWorkflowName"] = l.WorkflowName
		}

		uuid := l.GenerateUUID()
		outName := actionName + "Result"
		if l.currentBindingName != "" {
			outName = l.currentBindingName
		}
		runNode := &ir.NativeActionNode{
			NodeID:          uuid,
			AppleIdentifier: "is.workflow.actions.runworkflow",
			OutputUUID:      uuid,
			OutputName:      outName,
			Parameters:      runParams,
		}
		l.emitAction(runNode)

		if named, ok := fn.ReturnType.(*syntax.NamedTypeAnnotation); ok && (named.Name == "Number" || named.Name == "Int" || named.Name == "Float") {
			detectUUID := l.GenerateUUID()
			detectNode := &ir.NativeActionNode{
				NodeID:          detectUUID,
				AppleIdentifier: "is.workflow.actions.detect.number",
				OutputUUID:      detectUUID,
				OutputName:      outName,
				Parameters: map[string]interface{}{
					"WFInput": &ir.AttachmentToken{
						Type:       "ActionOutput",
						OutputUUID: uuid,
					},
				},
			}
			l.emitAction(detectNode)
			return &ir.AttachmentToken{
				Type:       "ActionOutput",
				OutputUUID: detectUUID,
				OutputName: outName,
			}, nil
		}

		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: uuid,
			OutputName: outName,
		}, nil
	}

	actionSchema, ok := l.registry.LookupAction(actionName)
	if !ok {
		return nil, fmt.Errorf("unknown action in lowering: %s", actionName)
	}

	uuid := l.GenerateUUID()

	if l.session != nil {
		callArgs := make([]backend.CallArgument, 0, len(call.NamedArgs)+1)
		if call.PrimaryArg != nil {
			if primParam, hasPrim := actionSchema.PrimaryParameter(); hasPrim {
				var primVal backend.SemanticValue
				if primParam.WireKey == "WFInput" || primParam.Codec == "token" {
					if _, isLit := call.PrimaryArg.(*syntax.LiteralExpr); isLit {
						rawVal, err := l.lowerExpression(call.PrimaryArg)
						if err == nil {
							att := l.materializeToAttachment(rawVal)
							primVal = backend.SemanticValue{
								Kind: backend.ValReference,
								Ref: &backend.Reference{
									Kind:         backend.RefActionResult,
									ProducerID:   att.OutputUUID,
									ProducerName: att.OutputName,
								},
							}
						}
					}
				}
				if primVal.Kind == backend.ValNil {
					var err error
					primVal, err = l.lowerSemanticValue(call.PrimaryArg)
					if err != nil {
						return nil, err
					}
				}
				wireKey := primParam.WireKey
				if wireKey == "" {
					wireKey = primParam.Label
				}
				if ident, isIdent := call.PrimaryArg.(*syntax.IdentExpr); isIdent {
					if _, isSetup := l.setupQuestions[ident.Name]; isSetup {
						l.bindSetupQuestion(ident.Name, wireKey)
					}
				}
				callArgs = append(callArgs, backend.CallArgument{
					ParameterID: wireKey,
					Position:    0,
					Value:       primVal,
				})
			}
		}

		seenParams := make(map[string]bool)
		if call.PrimaryArg != nil {
			if primParam, hasPrim := actionSchema.PrimaryParameter(); hasPrim {
				seenParams[primParam.ID] = true
			}
		}

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
				if seenParams[param.ID] {
					return nil, fmt.Errorf("duplicate parameter %q for action %q", param.Label, actionName)
				}
				seenParams[param.ID] = true

				val, err := l.lowerSemanticValue(nArg.Value)
				if err != nil {
					return nil, err
				}
				wireKey := param.WireKey
				if wireKey == "" {
					wireKey = param.Label
				}
				if ident, isIdent := nArg.Value.(*syntax.IdentExpr); isIdent {
					if _, isSetup := l.setupQuestions[ident.Name]; isSetup {
						l.bindSetupQuestion(ident.Name, wireKey)
					}
				}
				pos := -1
				for pIdx, p := range actionSchema.Parameters {
					if p.ID == param.ID || p.Label == param.Label {
						pos = pIdx
						break
					}
				}
				if pos < 0 {
					pos = paramIndex
				}
				callArgs = append(callArgs, backend.CallArgument{
					ParameterID: wireKey,
					Position:    pos,
					Value:       val,
				})
			} else if nArg.Label != "" {
				return nil, fmt.Errorf("unknown parameter label %q for action %q", nArg.Label, actionName)
			} else {
				return nil, fmt.Errorf("too many positional arguments for action %q", actionName)
			}
		}

		outputName := l.currentBindingName
		var outputUUID string
		if outputName != "" || (actionSchema.OutputTypeName != "" && actionSchema.OutputTypeName != "Void") {
			outputUUID = uuid
		}

		resolvedCall := backend.ResolvedCall{
			DefinitionID:    actionName,
			AppleIdentifier: actionSchema.AppleIdentifier,
			OutputUUID:      outputUUID,
			OutputName:      outputName,
			Arguments:       callArgs,
			NodeID:          uuid,
		}

		outUUID, err := l.session.EmitResolvedCall(resolvedCall)
		if err != nil {
			return nil, err
		}

		if outUUID != "" {
			outRefName := outputName
			if outRefName == "" {
				outRefName = actionSchema.Docs.Title
				if outRefName == "" {
					outRefName = actionSchema.OutputTypeName
				}
			}
			return &ir.AttachmentToken{
				Type:       "ActionOutput",
				OutputUUID: outUUID,
				OutputName: outRefName,
			}, nil
		}
		return nil, nil
	}

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

	l.emitAction(node)

	if node.OutputUUID != "" {
		return &ir.AttachmentToken{
			Type:       "ActionOutput",
			OutputUUID: node.OutputUUID,
		}, nil
	}
	return nil, nil
}
