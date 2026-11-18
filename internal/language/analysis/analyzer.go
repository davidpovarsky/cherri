package analysis

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/enums"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"howett.net/plist"
)

// Symbol represents a declared entity.
type Symbol struct {
	Name            string
	Type            types.Type
	Mutable         bool
	Initialized     bool
	Span            source.Span
	Function        *syntax.FunctionDecl
	Setup           *syntax.SetupDecl
	EnumDeclaration bool
	EnumType        string
	flowType        types.Type
}

// Scope represents a lexical scope.
type Scope struct {
	parent  *Scope
	symbols map[string]*Symbol
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		symbols: make(map[string]*Symbol),
	}
}

func (s *Scope) Define(sym *Symbol) bool {
	if _, exists := s.symbols[sym.Name]; exists {
		return false // duplicate in same scope
	}
	s.symbols[sym.Name] = sym
	return true
}

func (s *Scope) Lookup(name string) (*Symbol, bool) {
	if sym, ok := s.symbols[name]; ok {
		return sym, true
	}
	if s.parent != nil {
		return s.parent.Lookup(name)
	}
	return nil, false
}

func (s *Scope) LookupLocal(name string) (*Symbol, bool) {
	sym, ok := s.symbols[name]
	return sym, ok
}

// Analyzer performs semantic analysis on a parsed Program.
type Analyzer struct {
	registry               *schema.Registry
	diagnostics            []Diagnostic
	currentFn              *syntax.FunctionDecl
	inValueBlk             bool
	topLevelVars           map[string]source.Span
	yieldedTypes           []types.Type
	importStack            []string
	isMacTarget            *bool
	exprTypes              map[syntax.Expression]types.Type
	targets                []string
	hasExplicitTarget      bool
	minTargetVersion       float64
	maxTargetVersion       float64
	minTargetVersionSource string
	maxTargetVersionSource string
	userFunctions          map[string]bool
	enumNames              map[string]bool
	enumCatalog            enums.Catalog
	enumValues             map[syntax.Expression]string
	enumContexts           map[syntax.Expression]string
	enumErrors             map[syntax.Expression]bool
	loopDepth              int
	setupUses              map[*syntax.SetupDecl]int
	allowedSetupExpression syntax.Expression
	rawNativeDataContext   bool
}

// NewAnalyzer creates a new Analyzer with the action registry.
func NewAnalyzer(registry *schema.Registry) *Analyzer {
	if registry == nil {
		registry = schema.DefaultRegistry()
	}
	return &Analyzer{
		registry:      registry,
		topLevelVars:  make(map[string]source.Span),
		exprTypes:     make(map[syntax.Expression]types.Type),
		userFunctions: make(map[string]bool),
		enumNames:     make(map[string]bool),
	}
}

// ExprTypes returns the map of AST expression nodes to their inferred semantic types.
func (a *Analyzer) ExprTypes() map[syntax.Expression]types.Type {
	return a.exprTypes
}

// IsMacTarget returns whether the active target is Mac-only, non-Mac, or unconstrained (nil).
func (a *Analyzer) IsMacTarget() *bool {
	return a.isMacTarget
}

// SetIsMacTarget explicitly sets the active target platform constraint.
func (a *Analyzer) SetIsMacTarget(isMac *bool) {
	a.isMacTarget = isMac
	if isMac != nil {
		a.hasExplicitTarget = true
		if *isMac {
			a.targets = []string{"macos"}
		} else {
			a.targets = []string{"ios", "ipados"}
		}
	} else {
		a.hasExplicitTarget = false
		a.targets = nil
	}
}

// Targets returns the active target platforms.
func (a *Analyzer) Targets() []string {
	return a.targets
}

// SetTargets explicitly sets the target platforms.
func (a *Analyzer) SetTargets(targets []string) {
	a.targets = targets
	a.hasExplicitTarget = len(targets) > 0
	hasMac := false
	hasNonMac := false
	for _, t := range targets {
		tl := strings.ToLower(t)
		if tl == "mac" || tl == "macos" {
			hasMac = true
		} else if tl == "ios" || tl == "ipados" || tl == "watchos" {
			hasNonMac = true
		}
	}
	if hasMac && !hasNonMac {
		v := true
		a.isMacTarget = &v
	} else if hasNonMac && !hasMac {
		v := false
		a.isMacTarget = &v
	} else {
		a.isMacTarget = nil
	}
}

// MinTargetVersion returns the legacy float projection of the minimum target.
// Use MinTargetVersionString when minor/patch component precision matters.
func (a *Analyzer) MinTargetVersion() float64 {
	return a.minTargetVersion
}

// SetMinTargetVersion sets the minimum target OS version.
func (a *Analyzer) SetMinTargetVersion(v float64) {
	a.minTargetVersion = v
	a.minTargetVersionSource = legacyTargetVersionText(v)
}

// MinTargetVersionString returns the complete declared minimum version.
func (a *Analyzer) MinTargetVersionString() string { return a.minTargetVersionSource }

// MaxTargetVersion returns the legacy float projection of the maximum target.
// Use MaxTargetVersionString when minor/patch component precision matters.
func (a *Analyzer) MaxTargetVersion() float64 {
	return a.maxTargetVersion
}

// SetMaxTargetVersion sets the maximum target OS version.
func (a *Analyzer) SetMaxTargetVersion(v float64) {
	a.maxTargetVersion = v
	a.maxTargetVersionSource = legacyTargetVersionText(v)
}

// MaxTargetVersionString returns the complete declared maximum version.
func (a *Analyzer) MaxTargetVersionString() string { return a.maxTargetVersionSource }

// Diagnostics returns all diagnostics produced during analysis.
func (a *Analyzer) Diagnostics() []Diagnostic {
	return a.diagnostics
}

// SetImportStack sets the active import chain for cycle detection.
func (a *Analyzer) SetImportStack(stack []string) {
	a.importStack = stack
}

func (a *Analyzer) error(code string, span source.Span, msg string) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Code:     code,
		Severity: SeverityError,
		Span:     span,
		Message:  msg,
	})
}

func (a *Analyzer) warn(code string, span source.Span, msg string) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Code:     code,
		Severity: SeverityWarning,
		Span:     span,
		Message:  msg,
	})
}

func cloneStatements(stmts []syntax.Statement) []syntax.Statement {
	res := make([]syntax.Statement, len(stmts))
	copy(res, stmts)
	return res
}

func (a *Analyzer) expandCopyPaste(prog *syntax.Program) {
	copyBlocks := make(map[string]*syntax.BlockStmt)

	var filteredDecls []syntax.Declaration
	for _, decl := range prog.Declarations {
		if cp, ok := decl.(*syntax.CopyBlockStmt); ok {
			if _, exists := copyBlocks[cp.Name]; exists {
				a.error("E_DUPLICATE_DECLARATION", cp.Span, fmt.Sprintf("Duplicate declaration of copy/paste '%s'", cp.Name))
			} else {
				copyBlocks[cp.Name] = cp.Body
			}
		} else {
			filteredDecls = append(filteredDecls, decl)
		}
	}
	prog.Declarations = filteredDecls

	var expandStatements func(stmts []syntax.Statement) []syntax.Statement
	expandStatements = func(stmts []syntax.Statement) []syntax.Statement {
		var result []syntax.Statement
		for _, stmt := range stmts {
			switch s := stmt.(type) {
			case *syntax.CopyBlockStmt:
				if _, exists := copyBlocks[s.Name]; exists {
					a.error("E_DUPLICATE_DECLARATION", s.Span, fmt.Sprintf("Duplicate declaration of copy/paste '%s'", s.Name))
				} else {
					copyBlocks[s.Name] = s.Body
				}
			case *syntax.PasteStmt:
				block, exists := copyBlocks[s.Name]
				if !exists {
					a.error("E_UNDEFINED_COPY", s.Span, fmt.Sprintf("Unable to paste undefined copy '%s'", s.Name))
				} else {
					cloned := cloneStatements(block.Statements)
					result = append(result, expandStatements(cloned)...)
				}
			case *syntax.BlockStmt:
				s.Statements = expandStatements(s.Statements)
				result = append(result, s)
			case *syntax.IfStmt:
				if s.ThenBlock != nil {
					s.ThenBlock.Statements = expandStatements(s.ThenBlock.Statements)
				}
				if s.ElseBlock != nil {
					if blk, ok := s.ElseBlock.(*syntax.BlockStmt); ok {
						blk.Statements = expandStatements(blk.Statements)
					} else if ifBlk, ok := s.ElseBlock.(*syntax.IfStmt); ok {
						sub := expandStatements([]syntax.Statement{ifBlk})
						if len(sub) > 0 {
							s.ElseBlock = sub[0]
						}
					}
				}
				result = append(result, s)
			case *syntax.ForStmt:
				if s.BodyBlock != nil {
					s.BodyBlock.Statements = expandStatements(s.BodyBlock.Statements)
				}
				result = append(result, s)
			case *syntax.RepeatStmt:
				if s.BodyBlock != nil {
					s.BodyBlock.Statements = expandStatements(s.BodyBlock.Statements)
				}
				result = append(result, s)
			case *syntax.MenuStmt:
				for i := range s.Cases {
					if s.Cases[i].BodyBlock != nil {
						s.Cases[i].BodyBlock.Statements = expandStatements(s.Cases[i].BodyBlock.Statements)
					}
				}
				result = append(result, s)
			default:
				result = append(result, s)
			}
		}
		return result
	}

	prog.Statements = expandStatements(prog.Statements)

	for _, decl := range prog.Declarations {
		if fn, ok := decl.(*syntax.FunctionDecl); ok && fn.Body != nil {
			fn.Body.Statements = expandStatements(fn.Body.Statements)
		}
	}
}

// Analyze performs full semantic verification of a Program.
func (a *Analyzer) Analyze(prog *syntax.Program) {
	a.expandCopyPaste(prog)
	a.setupUses = make(map[*syntax.SetupDecl]int)

	scope := NewScope(nil)
	a.topLevelVars = make(map[string]source.Span)
	a.userFunctions = make(map[string]bool)
	a.enumNames = make(map[string]bool)
	a.prepareEnums(prog)

	// Builtin namespace symbols
	scope.Define(&Symbol{Name: "system", Type: types.Unknown, Initialized: true})
	scope.Define(&Symbol{Name: "native", Type: types.Unknown, Initialized: true})
	scope.Define(&Symbol{Name: "builtin", Type: types.Unknown, Initialized: true})
	scope.Define(&Symbol{Name: "Clipboard", Type: types.AnyContent, Initialized: true})
	scope.Define(&Symbol{Name: "ShortcutInput", Type: types.AnyContent, Initialized: true})
	scope.Define(&Symbol{Name: "Ask", Type: types.Unknown, Initialized: true})
	scope.Define(&Symbol{Name: "Device", Type: types.NewMap(types.Text, types.AnyContent), Initialized: true})
	scope.Define(&Symbol{Name: "CurrentDate", Type: types.Date, Initialized: true})
	scope.Define(&Symbol{Name: "RepeatIndex", Type: types.Number, Initialized: true})
	scope.Define(&Symbol{Name: "RepeatItem", Type: types.AnyContent, Initialized: true})
	for i := 2; i <= 10; i++ {
		scope.Define(&Symbol{Name: fmt.Sprintf("RepeatIndex%d", i), Type: types.Number, Initialized: true})
		scope.Define(&Symbol{Name: fmt.Sprintf("RepeatItem%d", i), Type: types.AnyContent, Initialized: true})
	}

	for _, stmt := range prog.Statements {
		if b, ok := stmt.(*syntax.BindingStmt); ok {
			a.topLevelVars[b.Name] = b.Span
		}
	}

	// Pass 1: Collect declarations
	for _, decl := range prog.Declarations {
		switch d := decl.(type) {
		case *syntax.FunctionDecl:
			a.userFunctions[d.Name] = true
			var fnType types.Type = types.Unknown
			if d.ReturnType != nil {
				fnType = a.resolveType(d.ReturnType)
			}
			if !scope.Define(&Symbol{Name: d.Name, Type: fnType, Initialized: true, Span: d.Span, Function: d}) {
				a.error(CodeDuplicateName, d.Span, fmt.Sprintf("duplicate function declaration %q", d.Name))
			}
		case *syntax.SetupDecl:
			a.setupUses[d] = 0
			var sType types.Type = types.Text
			if d.TypeExpr != nil {
				sType = a.resolveType(d.TypeExpr)
			}
			if !sType.Equals(types.Text) {
				a.error(CodeSetupBinding, d.Span, "setup currently requires Text; its native import-question default is a text value")
			}
			if !scope.Define(&Symbol{Name: d.Name, Type: sType, Initialized: true, Span: d.Span, Setup: d}) {
				a.error(CodeDuplicateName, d.Span, fmt.Sprintf("duplicate setup declaration %q", d.Name))
			}
		case *syntax.EnumDecl:
			a.enumNames[d.Name] = true
			seenMembers := make(map[string]bool)
			for _, member := range d.Members {
				if seenMembers[member.Name] {
					a.error(CodeDuplicateName, member.Span, fmt.Sprintf("duplicate enum member %q", member.Name))
				}
				seenMembers[member.Name] = true
			}
			if !scope.Define(&Symbol{Name: d.Name, Type: types.Text, Initialized: true, Span: d.Span, EnumDeclaration: true}) {
				a.error(CodeDuplicateName, d.Span, fmt.Sprintf("duplicate enum declaration %q", d.Name))
			}
		case *syntax.ActionDecl:
			var actParams []schema.ParameterSchema
			for _, p := range d.Parameters {
				var pType types.Type = types.Unknown
				if p.TypeExpr != nil {
					pType = a.resolveType(p.TypeExpr)
				}
				enumName := a.enumCatalog.TypeName(p.TypeExpr)
				binding := d.Bindings[p.Name]
				actParams = append(actParams, schema.ParameterSchema{
					ID: p.Name, Label: p.Name, Type: pType, TypeName: pType.Name(),
					Optional: p.Optional || p.DefaultExpr != nil,
					EnumName: enumName, EnumValues: a.enumCatalog.Values(enumName),
					WireKey: binding.WireKey, Codec: binding.Codec,
				})
			}
			retTypeName := "Void"
			var retType types.Type
			if d.ReturnType != nil {
				retType = a.resolveType(d.ReturnType)
				retTypeName = retType.Name()
			}
			primaryParam := d.PrimaryParam
			if primaryParam == "" && len(d.Parameters) > 0 {
				primaryParam = d.Parameters[0].Name
			}
			if a.registry != nil {
				a.registry.RegisterCustomAction(&schema.ActionSchema{
					ID:                 d.Name,
					CallableName:       d.Name,
					AppleIdentifier:    d.AppleIdentifier,
					Parameters:         actParams,
					PrimaryParameterID: primaryParam,
					OutputTypeName:     retTypeName,
					OutputType:         retType,
				})
			}
		case *syntax.ShortcutDecl:
			a.analyzeShortcutDecl(d)
		case *syntax.ImportDecl:
			if strings.HasPrefix(d.Path, "http://") || strings.HasPrefix(d.Path, "https://") {
				a.error(CodeWorkspaceAccess, d.Span, fmt.Sprintf("remote package imports not permitted: %q", d.Path))
			}
			cleanPath := filepath.Clean(d.Path)
			if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(d.Path) {
				a.error(CodeWorkspaceAccess, d.Span, fmt.Sprintf("path traversal outside workspace not permitted: %q", d.Path))
			}
			for _, p := range a.importStack {
				if p == d.Path || p == cleanPath {
					a.error(CodeImportCycle, d.Span, fmt.Sprintf("import cycle detected: %s -> %s", strings.Join(a.importStack, " -> "), d.Path))
					break
				}
			}
			if !scope.Define(&Symbol{Name: d.Alias, Type: types.Unknown, Initialized: true, Span: d.Span}) {
				a.error(CodeDuplicateName, d.Span, fmt.Sprintf("duplicate import alias %q", d.Alias))
			}
			if d.Path == "stdfunc" || d.Alias == "stdfunc" {
				scope.Define(&Symbol{Name: "runJS", Type: types.Unknown, Initialized: true, Span: d.Span})
				scope.Define(&Symbol{Name: "chooseFromVCard", Type: types.Unknown, Initialized: true, Span: d.Span})
			}
		}
	}

	// Pass 2: Analyze functions
	for _, decl := range prog.Declarations {
		switch declaration := decl.(type) {
		case *syntax.FunctionDecl:
			a.analyzeFunction(declaration, scope)
		case *syntax.ActionDecl:
			for _, parameter := range declaration.Parameters {
				if parameter.DefaultExpr != nil {
					actual := a.analyzeEnumContext(parameter.DefaultExpr, scope, a.enumCatalog.TypeName(parameter.TypeExpr))
					expected := a.resolveType(parameter.TypeExpr)
					if actual != nil && !assignmentHasEvidence(actual, expected, parameter.DefaultExpr) {
						a.error(CodeArgumentType, parameter.DefaultExpr.NodeSpan(), fmt.Sprintf("default value of type %s is not assignable to parameter type %s", actual, expected))
					}
				}
			}
		}
	}

	// Pass 3: Analyze top-level statements
	a.predefineImportProvenanceSymbols(prog.Statements, scope)
	for _, stmt := range prog.Statements {
		a.analyzeStatement(stmt, scope)
	}
	for declaration, uses := range a.setupUses {
		if uses == 0 {
			a.warn(CodeUnusedSetup, declaration.Span, fmt.Sprintf("setup %q is unused and emits no import question", declaration.Name))
		}
	}
}

func (a *Analyzer) predefineImportProvenanceSymbols(stmts []syntax.Statement, scope *Scope) {
	for _, stmt := range stmts {
		es, ok := stmt.(*syntax.ExprStmt)
		if !ok {
			continue
		}
		call, ok := es.Expr.(*syntax.CallExpr)
		if !ok {
			continue
		}
		mem, ok := call.Callee.(*syntax.MemberExpr)
		if !ok {
			continue
		}
		id, ok := mem.Target.(*syntax.IdentExpr)
		if !ok || id.Name != "native" || mem.Property != "workflow" {
			continue
		}
		rawStr := extractNativeWorkflowPlistString(call)
		if rawStr == "" {
			continue
		}
		rawMap := decodePlistDictForAnalysis(rawStr)
		if rawMap == nil {
			continue
		}
		prov, ok := rawMap["__CherriImportProvenance"].(map[string]any)
		if !ok {
			continue
		}
		syms, ok := prov["Symbols"].(map[string]any)
		if !ok {
			continue
		}
		for symName, rawEntry := range syms {
			entry, ok := rawEntry.(map[string]any)
			if !ok {
				continue
			}
			kind, _ := entry["Kind"].(string)
			mutable := kind == "Variable"
			var symType types.Type = types.AnyContent
			switch vt, _ := entry["ValueType"].(string); vt {
			case "Number", "Integer", "Float":
				symType = types.Number
			case "Text", "String":
				symType = types.Text
			case "Bool", "Boolean":
				symType = types.Bool
			case "Date":
				symType = types.Date
			case "Dict", "Dictionary":
				symType = types.NewMap(types.Text, types.AnyContent)
			case "Arr", "List":
				symType = types.NewList(types.AnyContent)
			}
			scope.Define(&Symbol{
				Name:        symName,
				Type:        symType,
				Mutable:     mutable,
				Initialized: true,
			})
		}
	}
}

func extractNativeWorkflowPlistString(call *syntax.CallExpr) string {
	var target syntax.Expression = call.PrimaryArg
	if target == nil {
		for _, n := range call.NamedArgs {
			if n.Label == "" || n.Label == "workflow" || n.Label == "plist" || n.Label == "data" {
				target = n.Value
				break
			}
		}
	}
	if target == nil {
		return ""
	}
	if lit, ok := target.(*syntax.LiteralExpr); ok {
		return lit.Value
	}
	if innerCall, ok := target.(*syntax.CallExpr); ok {
		if innerCall.PrimaryArg != nil {
			if lit, ok := innerCall.PrimaryArg.(*syntax.LiteralExpr); ok {
				return lit.Value
			}
		}
	}
	return ""
}

func decodePlistDictForAnalysis(rawStr string) map[string]any {
	trimmed := strings.TrimSpace(rawStr)
	var rawBytes []byte
	if strings.HasPrefix(trimmed, "b64:") {
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(trimmed, "b64:"))
		if err != nil {
			return nil
		}
		rawBytes = b
	} else {
		rawBytes = []byte(trimmed)
	}
	var parsed any
	if _, err := plist.Unmarshal(rawBytes, &parsed); err != nil {
		return nil
	}
	m, _ := parsed.(map[string]any)
	return m
}

func (a *Analyzer) analyzeShortcutDecl(sd *syntax.ShortcutDecl) {
	if sd.Metadata == nil {
		return
	}
	for _, f := range sd.Metadata.Fields {
		switch f.Name {
		case "mac":
			if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
				val := lit.Value == "true"
				a.isMacTarget = &val
				a.hasExplicitTarget = true
				if val {
					a.targets = []string{"macos"}
				} else {
					a.targets = []string{"ios", "ipados"}
				}
			}
		case "target":
			if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
				t := strings.ToLower(lit.Value)
				a.targets = []string{t}
				a.hasExplicitTarget = true
				val := (t == "macos" || t == "mac")
				a.isMacTarget = &val
			}
		case "targets":
			if list, ok := f.Value.(*syntax.ListExpr); ok {
				var targets []string
				hasMac := false
				hasNonMac := false
				for _, el := range list.Elements {
					if lit, ok := el.(*syntax.LiteralExpr); ok {
						v := strings.ToLower(lit.Value)
						targets = append(targets, v)
						if v == "mac" || v == "macos" {
							hasMac = true
						} else if v == "ios" || v == "ipados" || v == "watchos" {
							hasNonMac = true
						}
					}
				}
				a.targets = targets
				a.hasExplicitTarget = len(targets) > 0
				if hasMac && !hasNonMac {
					val := true
					a.isMacTarget = &val
				} else if hasNonMac && !hasMac {
					val := false
					a.isMacTarget = &val
				} else {
					a.isMacTarget = nil
				}
			}
		case "minVersion", "iosVersion", "clientVersion", "version":
			a.analyzeTargetVersion(f.Name, f.Value, false)
		case "maxVersion":
			a.analyzeTargetVersion(f.Name, f.Value, true)
		}
	}
}

func (a *Analyzer) analyzeFunction(fn *syntax.FunctionDecl, parentScope *Scope) {
	fnScope := NewScope(parentScope)
	a.currentFn = fn

	for _, p := range fn.Parameters {
		pType := a.resolveType(p.TypeExpr)
		if p.DefaultExpr != nil {
			defType := a.analyzeEnumContext(p.DefaultExpr, fnScope, a.enumCatalog.TypeName(p.TypeExpr))
			if defType != nil && !assignmentHasEvidence(defType, pType, p.DefaultExpr) {
				a.error(CodeArgumentType, p.DefaultExpr.NodeSpan(), fmt.Sprintf("default value of type %s is not assignable to parameter type %s", defType, pType))
			}
		}
		fnScope.Define(&Symbol{
			Name:        p.Name,
			EnumType:    a.enumCatalog.TypeName(p.TypeExpr),
			Type:        pType,
			Mutable:     false,
			Initialized: true,
			Span:        p.Span,
		})
	}

	for _, s := range fn.Body.Statements {
		a.analyzeStatement(s, fnScope)
	}

	a.currentFn = nil
}

func isConditionTypeAllowed(t types.Type) bool {
	if t == nil {
		return true
	}
	if t.AssignableTo(types.Bool) {
		return true
	}
	if t.Equals(types.Text) || t.AssignableTo(types.Text) {
		return true
	}
	switch t.Kind() {
	case types.KindUnknown, types.KindAnyContent, types.KindList, types.KindMap, types.KindOptional, types.KindContent, types.KindRecord:
		return true
	default:
		return false
	}
}

func (a *Analyzer) analyzeStatement(stmt syntax.Statement, scope *Scope) {
	switch s := stmt.(type) {
	case *syntax.BindingStmt:
		var valType types.Type
		if s.Value != nil {
			valType = a.analyzeEnumContext(s.Value, scope, a.enumCatalog.TypeName(s.TypeExpr))
		}
		if isVoidValue(valType) {
			if call, ok := s.Value.(*syntax.CallExpr); ok && call.Block != nil {
				a.error("E_BLOCK_VOID_VALUE", call.Span, "a void scoped block cannot initialize a binding; end the block with an explicit value")
			} else if s.Value != nil {
				a.error(CodeArgumentType, s.Value.NodeSpan(), "Void has no value to bind")
			}
		}

		declType := valType
		if s.TypeExpr != nil {
			annotated := a.resolveType(s.TypeExpr)
			if s.Value != nil && valType != nil && !assignmentHasEvidence(valType, annotated, s.Value) {
				a.error(CodeArgumentType, s.Value.NodeSpan(), fmt.Sprintf("type %s is not assignable to declared type %s", valType, annotated))
			}
			declType = annotated
		}

		if declType == nil {
			declType = types.Unknown
		}

		// Legacy @variable check
		if strings.HasPrefix(s.Name, "@") {
			a.error(CodeLegacySyntax, s.Span, "legacy variable syntax '@name' is unsupported; use 'let' or 'var'")
		}

		if existingLocal, found := scope.LookupLocal(s.Name); found {
			if existingLocal.Span.Start.Line == 0 {
				delete(scope.symbols, s.Name)
			} else {
				a.error(CodeDuplicateName, s.Span, fmt.Sprintf("duplicate variable declaration %q in this scope", s.Name))
				return
			}
		}

		if parentSym, exists := scope.Lookup(s.Name); exists && parentSym.Span.Start.Line != 0 {
			a.warn(CodeShadowing, s.Span, fmt.Sprintf("variable %q shadows an existing declaration at %s", s.Name, parentSym.Span))
		}

		symbol := &Symbol{
			Name:        s.Name,
			EnumType:    a.enumCatalog.TypeName(s.TypeExpr),
			Type:        declType,
			Mutable:     s.Mutable,
			Initialized: s.Value != nil,
			Span:        s.Span,
		}
		narrowAssignedValue(symbol, valType)
		scope.Define(symbol)

	case *syntax.AssignStmt:
		if strings.HasPrefix(s.Name, "@") {
			a.error(CodeLegacySyntax, s.Span, "legacy variable syntax '@name' is unsupported; use 'let' or 'var'")
			return
		}
		sym, found := scope.Lookup(s.Name)
		if !found {
			a.error(CodeUnknownName, s.Span, fmt.Sprintf("cannot assign to undeclared identifier %q", s.Name))
			return
		}
		if !sym.Mutable {
			a.error(CodeAssignImmutable, s.Span, fmt.Sprintf("cannot reassign immutable binding %q (declared with let)", s.Name))
			return
		}

		if s.Op != syntax.TokenAssign && !sym.Initialized {
			a.error(CodeReadBeforeInit, s.Span, fmt.Sprintf("variable %q read before initialization by compound assignment", s.Name))
		}
		valType := a.analyzeEnumContext(s.Value, scope, sym.EnumType)
		if valType != nil && sym.Type != nil {
			if s.Op == syntax.TokenPlusAssign {
				if listType, ok := sym.Type.(types.ListType); ok {
					if !assignmentHasEvidence(valType, listType.Element, s.Value) && !assignmentHasEvidence(valType, sym.Type, s.Value) {
						a.error(CodeArgumentType, s.Value.NodeSpan(), fmt.Sprintf("value of type %s is not assignable to list element of type %s", valType, listType.Element))
					}
				} else if !assignmentHasEvidence(valType, sym.Type, s.Value) {
					a.error(CodeArgumentType, s.Value.NodeSpan(), fmt.Sprintf("value of type %s is not assignable to variable %q of type %s", valType, s.Name, sym.Type))
				}
			} else if !assignmentHasEvidence(valType, sym.Type, s.Value) {
				a.error(CodeArgumentType, s.Value.NodeSpan(), fmt.Sprintf("value of type %s is not assignable to variable %q of type %s", valType, s.Name, sym.Type))
			}
		}
		sym.Initialized = true
		narrowAssignedValue(sym, valType)

	case *syntax.ExprStmt:
		a.analyzeExpression(s.Expr, scope)

	case *syntax.ReturnStmt:
		var retType types.Type = types.Void
		if s.Value != nil {
			enumName := ""
			if a.currentFn != nil {
				enumName = a.enumCatalog.TypeName(a.currentFn.ReturnType)
			}
			retType = a.analyzeEnumContext(s.Value, scope, enumName)
		}
		if a.currentFn != nil && a.currentFn.ReturnType != nil {
			expected := a.resolveType(a.currentFn.ReturnType)
			if !assignmentHasEvidence(retType, expected, s.Value) {
				a.error(CodeReturnType, s.Span, fmt.Sprintf("return value of type %s does not match expected return type %s", retType, expected))
			}
		}

	case *syntax.YieldStmt:
		if !a.inValueBlk {
			a.error(CodeYieldContext, s.Span, "'yield' statement is only permitted inside value-producing if/menu/for/repeat blocks")
		}
		yt := a.analyzeExpression(s.Value, scope)
		if yt != nil {
			a.yieldedTypes = append(a.yieldedTypes, yt)
		}

	case *syntax.BlockStmt:
		blockScope := NewScope(scope)
		for _, bs := range s.Statements {
			a.analyzeStatement(bs, blockScope)
		}

	case *syntax.IfStmt:
		condType := a.analyzeExpression(s.Condition, scope)
		if condType != nil && !isConditionTypeAllowed(condType) {
			a.error(CodeArgumentType, s.Condition.NodeSpan(), fmt.Sprintf("if condition requires Bool, got %s", condType))
		}
		entry := captureBindingFlow(scope)
		a.refineCondition(s.Condition, true, scope, nil)
		thenScope := NewScope(scope)
		for _, ts := range s.ThenBlock.Statements {
			a.analyzeStatement(ts, thenScope)
		}
		var paths []bindingFlow
		if statementMayContinue(s.ThenBlock) {
			paths = append(paths, captureBindingFlow(scope))
		}
		entry.restore()
		a.refineCondition(s.Condition, false, scope, nil)
		if s.ElseBlock != nil {
			a.analyzeStatement(s.ElseBlock, scope)
		}
		if s.ElseBlock == nil || statementMayContinue(s.ElseBlock) {
			paths = append(paths, captureBindingFlow(scope))
		}
		mergeBindingFlow(entry, paths...)

	case *syntax.ForStmt:
		forScope := NewScope(scope)
		iterType := a.analyzeExpression(s.Iterable, scope)
		var elemType types.Type = types.Unknown
		if listType, ok := iterType.(types.ListType); ok {
			elemType = listType.Element
		}
		if s.IndexVar != "" && s.IndexVar != "_" {
			forScope.Define(&Symbol{Name: s.IndexVar, Type: types.Number, Initialized: true, Span: s.Span})
		}
		if s.ItemVar != "" && s.ItemVar != "_" {
			forScope.Define(&Symbol{Name: s.ItemVar, Type: elemType, Initialized: true, Span: s.Span})
		}
		forScope.Define(&Symbol{Name: "RepeatIndex", Type: types.Number, Initialized: true, Span: s.Span})
		forScope.Define(&Symbol{Name: "RepeatItem", Type: elemType, Initialized: true, Span: s.Span})
		entry := captureBindingFlow(scope)
		invalidateLoopRefinements(s.BodyBlock, forScope)
		a.loopDepth++
		for _, bs := range s.BodyBlock.Statements {
			a.analyzeStatement(bs, forScope)
		}
		a.loopDepth--
		mergeBindingFlow(entry, entry, captureBindingFlow(scope))

	case *syntax.RepeatStmt:
		repScope := NewScope(scope)
		cntType := unwrapOptionalType(a.analyzeExpression(s.Count, scope))
		if cntType != nil && !cntType.AssignableTo(types.Number) && cntType.Kind() != types.KindUnknown {
			a.error(CodeArgumentType, s.Count.NodeSpan(), fmt.Sprintf("repeat count requires Number, got %s", cntType))
		}
		if s.IndexVar != "" && s.IndexVar != "_" {
			repScope.Define(&Symbol{Name: s.IndexVar, Type: types.Number, Initialized: true, Span: s.Span})
		}
		repScope.Define(&Symbol{Name: "RepeatIndex", Type: types.Number, Initialized: true, Span: s.Span})
		entry := captureBindingFlow(scope)
		invalidateLoopRefinements(s.BodyBlock, repScope)
		a.loopDepth++
		for _, bs := range s.BodyBlock.Statements {
			a.analyzeStatement(bs, repScope)
		}
		a.loopDepth--
		mergeBindingFlow(entry, entry, captureBindingFlow(scope))

	case *syntax.MenuStmt:
		if s.Prompt != nil {
			a.analyzeExpression(s.Prompt, scope)
		}
		for _, c := range s.Cases {
			if c.Value != nil {
				a.analyzeExpression(c.Value, scope)
			}
		}
		entry := captureBindingFlow(scope)
		var paths []bindingFlow
		for _, c := range s.Cases {
			entry.restore()
			caseScope := NewScope(scope)
			for _, cs := range c.BodyBlock.Statements {
				a.analyzeStatement(cs, caseScope)
			}
			if statementMayContinue(c.BodyBlock) {
				paths = append(paths, captureBindingFlow(scope))
			}
		}
		mergeBindingFlow(entry, paths...)
	}
}

func unwrapOptionalType(t types.Type) types.Type {
	for {
		opt, ok := t.(types.OptionalType)
		if !ok {
			break
		}
		t = opt.Inner
	}
	return t
}

func unwrapDynamicOptional(t types.Type) types.Type {
	opt, ok := t.(types.OptionalType)
	if !ok {
		return t
	}
	inner := unwrapOptionalType(opt)
	if inner == nil || inner.Kind() == types.KindAnyContent || inner.Kind() == types.KindUnknown {
		return inner
	}
	return t
}

func blockAnyYield(blk *syntax.BlockStmt) bool {
	if blk == nil {
		return false
	}
	for _, st := range blk.Statements {
		switch s := st.(type) {
		case *syntax.YieldStmt:
			return true
		case *syntax.BlockStmt:
			if blockAnyYield(s) {
				return true
			}
		case *syntax.IfStmt:
			if ifAnyYield(s) {
				return true
			}
		case *syntax.MenuStmt:
			if menuAnyYield(s) {
				return true
			}
		}
	}
	return false
}

func ifAnyYield(s *syntax.IfStmt) bool {
	if s == nil {
		return false
	}
	if blockAnyYield(s.ThenBlock) {
		return true
	}
	switch el := s.ElseBlock.(type) {
	case *syntax.BlockStmt:
		return blockAnyYield(el)
	case *syntax.IfStmt:
		return ifAnyYield(el)
	}
	return false
}

func menuAnyYield(s *syntax.MenuStmt) bool {
	if s == nil {
		return false
	}
	for _, c := range s.Cases {
		if blockAnyYield(c.BodyBlock) {
			return true
		}
	}
	return false
}

func blockHasYield(blk *syntax.BlockStmt) bool {
	if blk == nil {
		return false
	}
	for _, st := range blk.Statements {
		switch s := st.(type) {
		case *syntax.YieldStmt:
			return true
		case *syntax.BlockStmt:
			if blockHasYield(s) {
				return true
			}
		case *syntax.IfStmt:
			if ifBranchHasYield(s) {
				return true
			}
		}
	}
	return false
}

func ifBranchHasYield(s *syntax.IfStmt) bool {
	if s == nil || s.ElseBlock == nil {
		return false
	}
	if !blockHasYield(s.ThenBlock) {
		return false
	}
	switch el := s.ElseBlock.(type) {
	case *syntax.BlockStmt:
		return blockHasYield(el)
	case *syntax.IfStmt:
		return ifBranchHasYield(el)
	default:
		return false
	}
}

func isZeroNumberLiteral(expr syntax.Expression) bool {
	switch e := expr.(type) {
	case *syntax.LiteralExpr:
		if e.Type == syntax.TokenNumber {
			var n float64
			if _, err := fmt.Sscanf(e.Value, "%f", &n); err == nil && n == 0 {
				return true
			}
		}
	case *syntax.UnaryExpr:
		if e.Op == syntax.TokenMinus || e.Op == syntax.TokenPlus {
			return isZeroNumberLiteral(e.Operand)
		}
	}
	return false
}

func (a *Analyzer) analyzeExpression(expr syntax.Expression, scope *Scope) types.Type {
	if expr == nil {
		return types.Unknown
	}
	t := a.analyzeExpressionInternal(expr, scope)
	if name := a.enumContexts[expr]; name != "" {
		a.validateEnumValue(expr, name, a.enumCatalog.Values(name))
	}
	if a.exprTypes != nil {
		a.exprTypes[expr] = t
	}
	return t
}

func (a *Analyzer) analyzeExpressionInternal(expr syntax.Expression, scope *Scope) types.Type {
	if a.rawNativeDataContext {
		switch expr.(type) {
		case *syntax.LiteralExpr, *syntax.ListExpr, *syntax.MapExpr:
			// Only literal raw data inherits the escape boundary.
		default:
			a.rawNativeDataContext = false
			defer func() { a.rawNativeDataContext = true }()
		}
	}
	if a.resolveEnumExpression(expr, scope) {
		return types.Text
	}

	switch e := expr.(type) {
	case *syntax.LiteralExpr:
		switch e.Type {
		case syntax.TokenString, syntax.TokenRawString:
			return types.Text
		case syntax.TokenNumber:
			return types.Number
		case syntax.TokenTrue, syntax.TokenFalse:
			return types.Bool
		case syntax.TokenNone:
			return types.NewOptional(types.Unknown)
		case syntax.TokenNull:
			if !a.rawNativeDataContext {
				a.error(CodeNativeJSONNull, e.Span, "authored JSON null has no supported native value encoding; it cannot become none, empty text, or an omitted field")
			}
			return types.JsonNull
		default:
			return types.Unknown
		}

	case *syntax.FStringExpr:
		for _, part := range e.Parts {
			if part.IsExpr {
				if isVoidValue(a.analyzeExpression(part.Expr, scope)) {
					a.error(CodeArgumentType, part.Expr.NodeSpan(), "Void cannot be interpolated as a value")
				}
			}
		}
		return types.Text

	case *syntax.IdentExpr:
		// Legacy @variable check
		if strings.HasPrefix(e.Name, "@") {
			a.error(CodeLegacySyntax, e.Span, "legacy variable reference '@name' is unsupported; use plain name without '@'")
			return types.Unknown
		}
		sym, found := scope.Lookup(e.Name)
		if found && sym.Setup != nil && a.allowedSetupExpression != expr {
			a.error(CodeSetupBinding, e.Span, "setup must bind directly to one native action parameter; aliases, expressions and function arguments are unsupported")
		}
		if !found {
			if a.currentFn != nil {
				if _, isTopVar := a.topLevelVars[e.Name]; isTopVar {
					a.error(CodeUnknownName, e.Span, fmt.Sprintf("cannot capture outer variable %q inside function %q; pass it as a parameter", e.Name, a.currentFn.Name))
					return types.Unknown
				}
			}
			// Check if it's an action name
			if _, isAction := a.registry.LookupAction(e.Name); isAction {
				return types.Unknown // Callable action
			}
			a.error(CodeUnknownName, e.Span, fmt.Sprintf("unknown identifier %q", e.Name))
			return types.Unknown
		}
		if sym.EnumDeclaration {
			a.enumDiagnostic(e, fmt.Sprintf("enum %q is a type; select a member", e.Name))
		}
		if !sym.Initialized {
			a.error(CodeReadBeforeInit, e.Span, fmt.Sprintf("variable %q read before initialization", e.Name))
		}
		return sym.readType()

	case *syntax.MemberExpr:
		targetType := a.analyzeExpression(e.Target, scope)
		if isVoidValue(targetType) {
			a.error(CodeArgumentType, e.Target.NodeSpan(), "Void has no content view or property")
			return types.Unknown
		}
		if rec, ok := targetType.(types.RecordType); ok {
			if f, exists := rec.Fields[e.Property]; exists {
				return f.Type
			}
			a.error(CodeUnknownName, e.Span, fmt.Sprintf("property %q does not exist on record %s", e.Property, rec))
		}
		switch strings.ToLower(e.Property) {
		case "dictionary":
			return types.NewMap(types.Text, types.AnyContent)
		case "date":
			return types.Date
		case "number":
			return types.Number
		case "text", "string":
			return types.Text
		case "bool", "boolean":
			return types.Bool
		case "file":
			return types.File
		case "image":
			return types.Image
		default:
			if mayHaveListCardinality(targetType) {
				a.error(CodeArgumentType, e.Span, fmt.Sprintf("property %q requires a single value; %s may contain multiple items", e.Property, targetType))
			}
			return types.Unknown
		}

	case *syntax.EnumMemberExpr:
		// An uppercase lexical value is parsed with enum-shaped syntax.
		return a.analyzeExpressionInternal(&syntax.MemberExpr{
			Span: e.Span, Target: &syntax.IdentExpr{Span: e.Span, Name: e.TypeName}, Property: e.Member,
		}, scope)

	case *syntax.IndexExpr:
		targetType := a.analyzeExpression(e.Target, scope)
		idxType := a.analyzeExpression(e.Index, scope)
		if isVoidValue(idxType) {
			a.error(CodeArgumentType, e.Index.NodeSpan(), "Void cannot be an index value")
			return types.Unknown
		}
		if isVoidValue(targetType) {
			a.error(CodeArgumentType, e.Target.NodeSpan(), "Void has no indexable value")
			return types.Unknown
		}
		if litKey, ok := e.Index.(*syntax.LiteralExpr); ok && litKey.Value == "" {
			a.error(CodeArgumentType, litKey.Span, "empty dictionary key is not allowed")
		}
		if mapType, ok := targetType.(types.MapType); ok {
			if idxType != nil && !idxType.AssignableTo(mapType.Key) && idxType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Index.NodeSpan(), fmt.Sprintf("map index requires %s, got %s", mapType.Key, idxType))
			}
			return types.NewOptional(mapType.Value)
		}
		if targetType != nil && (targetType.Kind() == types.KindUnknown || targetType.Kind() == types.KindAnyContent) {
			return types.Unknown
		}
		if targetType != nil && targetType.Kind() != types.KindList && idxType != nil && idxType.AssignableTo(types.Text) && !idxType.AssignableTo(types.Number) {
			// Content item / coerced property lookup via string key (e.g. r.file["File Extension"], Device["Device Name"]).
			return types.AnyContent
		}
		if idxType != nil && !idxType.AssignableTo(types.Number) && idxType.Kind() != types.KindUnknown {
			a.error(CodeArgumentType, e.Index.NodeSpan(), fmt.Sprintf("index expression requires Number, got %s", idxType))
		}
		if listType, ok := targetType.(types.ListType); ok {
			return types.NewOptional(listType.Element)
		}
		return types.Unknown

	case *syntax.CallExpr:
		return a.analyzeCall(e, scope)

	case *syntax.UnaryExpr:
		opType := a.analyzeExpression(e.Operand, scope)
		if isVoidValue(opType) {
			a.error(CodeArgumentType, e.Operand.NodeSpan(), "operator requires a value, got Void")
			return types.Unknown
		}
		if e.Op == syntax.TokenBang {
			return types.Bool
		}
		if e.Op == syntax.TokenMinus || e.Op == syntax.TokenPlus {
			if opType != nil && !types.HasAssignmentEvidence(opType, types.Number) {
				a.error(CodeArgumentType, e.Operand.NodeSpan(), fmt.Sprintf("unary '-' requires Number, got %s", opType))
			}
			return types.Number
		}
		return types.Unknown

	case *syntax.BinaryExpr:
		leftType := a.analyzeExpression(e.Left, scope)
		var rightType types.Type
		if e.Op == syntax.TokenAnd || e.Op == syntax.TokenOr {
			rightType = a.analyzeLogicalRight(e, scope)
		} else {
			rightType = a.analyzeExpression(e.Right, scope)
		}

		voidOperand := false
		if isVoidValue(leftType) {
			a.error(CodeArgumentType, e.Left.NodeSpan(), "operator requires a value, got Void")
			voidOperand = true
		}
		if isVoidValue(rightType) {
			a.error(CodeArgumentType, e.Right.NodeSpan(), "operator requires a value, got Void")
			voidOperand = true
		}
		if voidOperand {
			return types.Unknown
		}
		switch e.Op {
		case syntax.TokenPlus, syntax.TokenMinus, syntax.TokenStar, syntax.TokenSlash, syntax.TokenPercent:
			if e.Op == syntax.TokenPlus && leftType != nil && rightType != nil && types.HasAssignmentEvidence(leftType, types.Text) && types.HasAssignmentEvidence(rightType, types.Text) {
				return types.Text
			}
			if leftType != nil && !types.HasAssignmentEvidence(leftType, types.Number) {
				a.error(CodeArgumentType, e.Left.NodeSpan(), fmt.Sprintf("numeric operator requires Number, got %s", leftType))
			}
			if rightType != nil && !types.HasAssignmentEvidence(rightType, types.Number) {
				a.error(CodeArgumentType, e.Right.NodeSpan(), fmt.Sprintf("numeric operator requires Number, got %s", rightType))
			}
			if (e.Op == syntax.TokenSlash || e.Op == syntax.TokenPercent) && isZeroNumberLiteral(e.Right) {
				a.error(CodeArgumentType, e.Right.NodeSpan(), "division or modulus by constant zero is not allowed")
			}
			return types.Number

		case syntax.TokenEqual, syntax.TokenNotEqual:
			return types.Bool

		case syntax.TokenLess, syntax.TokenLessEqual, syntax.TokenGreater, syntax.TokenGreaterEqual:
			isComparable := func(t types.Type) bool {
				t = unwrapOptionalType(t)
				if t == nil {
					return false
				}
				if types.HasAssignmentEvidence(t, types.Number) || types.HasAssignmentEvidence(t, types.Date) {
					return true
				}
				return false
			}
			if !isComparable(leftType) {
				a.error(CodeArgumentType, e.Left.NodeSpan(), fmt.Sprintf("comparison requires Number or Date, got %s", leftType))
			}
			if !isComparable(rightType) && !a.isKnownDateDurationComparison(leftType, e.Right, e.Op, scope) {
				a.error(CodeArgumentType, e.Right.NodeSpan(), fmt.Sprintf("comparison requires Number, Date, or a known date duration, got %s", rightType))
			}
			return types.Bool

		case syntax.TokenAnd, syntax.TokenOr:
			return types.Bool

		case syntax.TokenContains, syntax.TokenBeginsWith, syntax.TokenEndsWith:
			return types.Bool
		}

	case *syntax.ListExpr:
		elements := make([]types.Type, 0, len(e.Elements))
		for _, el := range e.Elements {
			elementType := a.analyzeExpression(el, scope)
			if isVoidValue(elementType) {
				a.error(CodeArgumentType, el.NodeSpan(), "Void cannot be stored as a list value")
			}
			elements = append(elements, elementType)
		}
		return types.NewList(types.NewUnion(elements...))

	case *syntax.MapExpr:
		values := make([]types.Type, 0, len(e.Entries))
		seenKeys := make(map[string]bool)
		for _, entry := range e.Entries {
			if litKey, ok := entry.Key.(*syntax.LiteralExpr); ok {
				if litKey.Value == "" {
					a.error(CodeArgumentType, litKey.Span, "empty dictionary key is not allowed")
				}
				if seenKeys[litKey.Value] {
					a.error("E_DUPLICATE_KEY", litKey.Span, fmt.Sprintf("duplicate key %q in map literal", litKey.Value))
				}
				seenKeys[litKey.Value] = true
			}
			keyType := a.analyzeExpression(entry.Key, scope)
			valueType := a.analyzeExpression(entry.Value, scope)
			if isVoidValue(keyType) {
				a.error(CodeArgumentType, entry.Key.NodeSpan(), "Void cannot be a dictionary key")
			}
			if isVoidValue(valueType) {
				a.error(CodeArgumentType, entry.Value.NodeSpan(), "Void cannot be stored as a dictionary value")
			}
			values = append(values, valueType)
		}
		return types.NewMap(types.Text, types.NewUnion(values...))

	case *syntax.IfStmt:
		oldVal := a.inValueBlk
		oldYields := a.yieldedTypes
		a.inValueBlk = true
		a.yieldedTypes = nil
		a.analyzeStatement(e, scope)
		a.yieldedTypes = oldYields
		a.inValueBlk = oldVal
		if ifAnyYield(e) && e.ElseBlock == nil {
			a.error(CodeYieldContext, e.Span, "value-producing 'if' expression requires 'else'")
		}
		return a.controlExpressionResultType(e)

	case *syntax.RepeatStmt:
		oldVal := a.inValueBlk
		oldYields := a.yieldedTypes
		a.inValueBlk = true
		a.yieldedTypes = nil
		a.analyzeStatement(e, scope)
		a.yieldedTypes = oldYields
		a.inValueBlk = oldVal
		return a.controlExpressionResultType(e)

	case *syntax.ForStmt:
		oldVal := a.inValueBlk
		oldYields := a.yieldedTypes
		a.inValueBlk = true
		a.yieldedTypes = nil
		a.analyzeStatement(e, scope)
		a.yieldedTypes = oldYields
		a.inValueBlk = oldVal
		return a.controlExpressionResultType(e)

	case *syntax.MenuStmt:
		oldVal := a.inValueBlk
		oldYields := a.yieldedTypes
		a.inValueBlk = true
		a.yieldedTypes = nil
		a.analyzeStatement(e, scope)
		a.yieldedTypes = oldYields
		a.inValueBlk = oldVal
		if menuAnyYield(e) {
			for _, c := range e.Cases {
				if !blockHasYield(c.BodyBlock) {
					a.error(CodeYieldContext, c.Span, fmt.Sprintf("value-producing 'menu' case %q requires 'yield'", c.Label))
				}
			}
		}
		return a.controlExpressionResultType(e)
	}

	return types.Unknown
}

func (a *Analyzer) analyzeCall(call *syntax.CallExpr, scope *Scope) types.Type {
	// Raw payload literals keep their explicit escape boundary. Nested ordinary
	// calls still obey authored value rules and cannot inherit that exemption.
	previousRawContext := a.rawNativeDataContext
	a.rawNativeDataContext = false
	defer func() { a.rawNativeDataContext = previousRawContext }()
	if call.Block != nil {
		return a.analyzeScopedBlockCall(call, scope)
	}
	if name, ok := call.Callee.(*syntax.IdentExpr); ok {
		if action, found := a.registry.LookupAction(name.Name); found && action.ScopedBlock && !a.userFunctions[name.Name] {
			a.error("E_BLOCK_REQUIRED", call.Span, "this compiler construct requires a trailing scoped block on the same line")
			return types.Unknown
		}
	}
	var callName string
	switch c := call.Callee.(type) {
	case *syntax.IdentExpr:
		callName = c.Name
	case *syntax.MemberExpr:
		if targetIdent, ok := c.Target.(*syntax.IdentExpr); ok && targetIdent.Name == "native" {
			callName = "native." + c.Property
		} else {
			a.analyzeExpression(call.Callee, scope)
			return types.Unknown
		}
	default:
		// e.g. callee is a complex expression
		a.analyzeExpression(call.Callee, scope)
		return types.Unknown
	}

	// 0. Check native escape / rawAction
	if callName == "rawAction" || callName == "native.action" || callName == "action" ||
		callName == "native.plist" || callName == "native.ref" || callName == "native.workflow" {
		a.rawNativeDataContext = true
		if (callName == "rawAction" || callName == "native.action") && call.PrimaryArg == nil {
			hasIdentNamed := false
			for _, nArg := range call.NamedArgs {
				if nArg.Label == "identifier" || nArg.Label == "" {
					hasIdentNamed = true
					break
				}
			}
			if !hasIdentNamed {
				a.error(CodeMissingArgument, call.Span, fmt.Sprintf("missing required argument \"identifier\" in call to %q", callName))
			}
		}
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		for _, nArg := range call.NamedArgs {
			a.analyzeExpression(nArg.Value, scope)
		}
		return types.AnyContent
	}

	// 1. Check user-defined function in scope
	if sym, found := scope.Lookup(callName); found && sym.Type != nil {
		if a.loopDepth > 0 && a.userFunctions[callName] {
			a.warn("W_PERF", call.Span, fmt.Sprintf("calling user-defined function %q inside a loop incurs 'Run Shortcut' overhead; consider inlining or restructuring if performance is critical", callName))
		}
		// Evaluate arguments
		if sym.Function != nil {
			a.checkUserFunctionArguments(sym.Function, call, scope)
		} else {
			if call.PrimaryArg != nil {
				a.analyzeExpression(call.PrimaryArg, scope)
			}
			for _, argument := range call.NamedArgs {
				a.analyzeExpression(argument.Value, scope)
			}
		}
		return sym.Type
	}

	// 1b. Check built-in value constructors (qty, color)
	if callName == "qty" || callName == "color" {
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		for _, nArg := range call.NamedArgs {
			a.analyzeExpression(nArg.Value, scope)
		}
		return types.AnyContent
	}

	// 1c. Check condition predicates (isToday, isTrue, isFalse, numeric, condEnum)
	if callName == "isToday" || callName == "isTrue" || callName == "isFalse" {
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		return types.Bool
	}
	if callName == "numeric" {
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		return types.Number
	}
	if callName == "condEnum" {
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		return types.Text
	}

	// 1d. Check isBetween predicate
	if callName == "isBetween" || callName == "between" {
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		for _, nArg := range call.NamedArgs {
			a.analyzeExpression(nArg.Value, scope)
		}
		return types.Bool
	}

	// 2. Check Action in registry
	actionSchema, isAction := a.registry.LookupAction(callName)
	if !isAction {
		a.error(CodeUnknownAction, call.Callee.NodeSpan(), fmt.Sprintf("unknown action or function %q", callName))
		return types.Unknown
	}

	a.checkTargetVersion(actionSchema, callName, call.Span)
	if a.hasExplicitTarget {
		for _, target := range a.targets {
			tl := strings.ToLower(target)
			if tl == "mac" || tl == "macos" {
				if actionSchema.NonMacOnly {
					a.error(CodeUnsupportedTarget, call.Span, fmt.Sprintf("action %q is not available on macOS", callName))
				}
			} else {
				if actionSchema.MacOnly {
					a.error(CodeUnsupportedTarget, call.Span, fmt.Sprintf("action %q is only available on macOS (cannot target %s)", callName, target))
				}
			}
		}
	} else {
		if actionSchema.MacOnly {
			a.warn(CodeUnverifiedForm, call.Span, fmt.Sprintf("action %q is only available on macOS, but shortcut target is unspecified; specify target in shortcut metadata", callName))
		}
		if actionSchema.NonMacOnly {
			a.warn(CodeUnverifiedForm, call.Span, fmt.Sprintf("action %q is not available on macOS, but shortcut target is unspecified; specify target in shortcut metadata", callName))
		}
	}

	// A named variadic list is a source argument pack, not one text value.
	// Reuse the ordinary per-element type/binding checks on its unchanged nodes.
	if expanded, packError := schema.ExpandNamedVariadicLists(call, actionSchema); packError != nil {
		a.error(CodeDuplicateArgument, packError.Span, packError.Message)
		return types.Unknown
	} else {
		call = expanded
	}

	// Step-by-step argument binding algorithm per §7.2
	boundIndices := make(map[int]bool)
	boundArguments := make(map[string]syntax.Expression)

	// Step 2: Primary argument binding
	if call.PrimaryArg != nil {
		primParam, hasPrim := actionSchema.PrimaryParameter()
		if !hasPrim {
			a.error(CodePositionalArg, call.PrimaryArg.NodeSpan(), fmt.Sprintf("action %q does not accept an unlabeled primary argument; use named arguments", callName))
		} else {
			for index := range actionSchema.Parameters {
				if &actionSchema.Parameters[index] == primParam {
					boundIndices[index] = true
					break
				}
			}
			boundArguments[primParam.WireKey] = call.PrimaryArg
			if primParam.WireKey == "WFDictionaryKey" {
				if lit, isLit := call.PrimaryArg.(*syntax.LiteralExpr); isLit && lit.Value == "" {
					a.error(CodeArgumentType, call.PrimaryArg.NodeSpan(), "empty dictionary key is not allowed")
				}
			}
			argType := unwrapDynamicOptional(a.analyzeSetupArgument(actionSchema, primParam, call.PrimaryArg, scope, callName))
			if isVoidValue(argType) {
				a.error(CodeArgumentType, call.PrimaryArg.NodeSpan(), "Void cannot be passed as an argument value")
			} else if primParam.Type != nil && argType != nil && !argType.AssignableTo(primParam.Type) {
				if !(primParam.Type == types.Number && argType == types.Text && isNumericLiteralOrTokenString(call.PrimaryArg)) &&
					!isAttachmentCapableTextParamSchema(actionSchema.AppleIdentifier, primParam.WireKey, call.PrimaryArg) {
					a.error(CodeArgumentType, call.PrimaryArg.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", primParam.Label, primParam.Type, argType))
				}
			}

		}
	}

	// Step 3 & 4: Named arguments binding
	// Positional compatibility fills the first remaining parameter; an
	// explicitly declared primary parameter need not occupy index zero.
	paramIndex := 0
	for _, nArg := range call.NamedArgs {
		var param *schema.ParameterSchema
		var matchedIdx = -1
		var paramExists bool
		if nArg.Label != "" {
			for pIdx := range actionSchema.Parameters {
				p := &actionSchema.Parameters[pIdx]
				if p.Label == nArg.Label || p.ID == nArg.Label {
					if matchedIdx == -1 {
						matchedIdx = pIdx
						param = p
						paramExists = true
					}
					if !boundIndices[pIdx] {
						matchedIdx = pIdx
						param = p
						paramExists = true
						break
					}
				}
			}
		} else {
			for paramIndex < len(actionSchema.Parameters) && boundIndices[paramIndex] {
				paramIndex++
			}
			if paramIndex < len(actionSchema.Parameters) {
				matchedIdx = paramIndex
				param = &actionSchema.Parameters[paramIndex]
				paramExists = true
				paramIndex++
			} else if len(actionSchema.Parameters) > 0 && actionSchema.Parameters[len(actionSchema.Parameters)-1].Variadic {
				matchedIdx = len(actionSchema.Parameters) - 1
				param = &actionSchema.Parameters[matchedIdx]
				paramExists = true
			}
		}
		if !paramExists {
			if nArg.Label == "" {
				a.error(CodePositionalArg, nArg.Span, fmt.Sprintf("too many positional arguments in call to %q", callName))
			} else {
				a.error(CodeUnknownArgument, nArg.Span, fmt.Sprintf("unknown argument label %q in call to %q", nArg.Label, callName))
			}
			a.analyzeExpression(nArg.Value, scope)
			continue
		}

		if boundIndices[matchedIdx] && !param.Variadic {
			a.error(CodeDuplicateArgument, nArg.Span, fmt.Sprintf("duplicate argument for parameter %q in call to %q", nArg.Label, callName))
		}
		boundIndices[matchedIdx] = true
		boundArguments[param.WireKey] = nArg.Value

		if param.WireKey == "WFDictionaryKey" || nArg.Label == "key" {
			if lit, isLit := nArg.Value.(*syntax.LiteralExpr); isLit && lit.Value == "" {
				a.error(CodeArgumentType, nArg.Value.NodeSpan(), "empty dictionary key is not allowed")
			}
		}

		argType := unwrapDynamicOptional(a.analyzeSetupArgument(actionSchema, param, nArg.Value, scope, callName))
		if isVoidValue(argType) {
			a.error(CodeArgumentType, nArg.Value.NodeSpan(), "Void cannot be passed as an argument value")
		} else if param.Type != nil && argType != nil && !argType.AssignableTo(param.Type) {
			if !(param.Type == types.Number && argType == types.Text && isNumericLiteralOrTokenString(nArg.Value)) &&
				!isAttachmentCapableTextParamSchema(actionSchema.AppleIdentifier, param.WireKey, nArg.Value) {
				a.error(CodeArgumentType, nArg.Value.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", param.Label, param.Type, argType))
			}
		}

	}

	// Step 6: Validate requiredness
	for pIdx, p := range actionSchema.Parameters {
		if !p.Optional && !p.Variadic && !boundIndices[pIdx] {
			a.error(CodeMissingArgument, call.Span, fmt.Sprintf("missing required argument %q in call to %q", p.Label, callName))
		}
	}

	if output, known := a.variadicListOutputType(actionSchema, call); known {
		return output
	}
	if actionSchema.CallableName == "require" && actionSchema.CompilerConstruct {
		return a.requireOutputType(call)
	}
	if actionSchema.CallableName == "paginate" && actionSchema.CompilerConstruct {
		a.checkPaginateArguments(call, boundArguments)
	}
	if result, specialized := a.variantOutputType(actionSchema, boundArguments); specialized {
		return result
	}
	if actionSchema.OutputType != nil {
		return actionSchema.OutputType
	}
	return types.Unknown
}

func (a *Analyzer) checkAcceptedForms(param *schema.ParameterSchema, arg syntax.Expression, callName string) {
	if param == nil || a.enumErrors[arg] {
		return
	}
	if value, resolved := a.enumValues[arg]; resolved {
		arg = &syntax.LiteralExpr{Span: arg.NodeSpan(), Type: syntax.TokenString, Value: value}
	}
	// HTTP dictionary fields require a literal outer container but explicitly
	// support runtime leaf values. Preserve that established codec contract.
	dictionaryContainer := param.WireKey == "WFJSONValues" || param.WireKey == "WFFormValues" || param.WireKey == "WFHTTPHeaders"
	if !dictionaryContainer && requiresLiteralArgument(param) {
		if !eligibleLiteralArgument(arg) && !acceptsLiteralTokenString(param, arg) {
			a.error(CodeLiteralRequired, arg.NodeSpan(), fmt.Sprintf("parameter %q in call to %q requires a literal value or an eligible pure literal expression; runtime bindings and actions are not literals", param.Label, callName))
		}
		return
	}
	if len(param.AcceptedForms) == 0 {
		return
	}
	hasLiteral := false
	hasTokenString := false
	hasAttachment := false
	for _, f := range param.AcceptedForms {
		switch f {
		case "literal":
			hasLiteral = true
		case "tokenString":
			hasTokenString = true
		case "attachment(output)", "attachment(variable)":
			hasAttachment = true
		}
	}
	if (callName == "editEvent" || callName == "editReminder") && param.ID == "newValue" {
		hasLiteral = true
		hasTokenString = true
		hasAttachment = true
	}
	if param.WireKey == "WFJSONValues" || param.WireKey == "WFFormValues" || param.WireKey == "WFHTTPHeaders" {
		hasLiteral = true
		hasTokenString = false
		hasAttachment = false
	}
	isDictLiteralOnlyWireKey := param.WireKey == "WFJSONValues" || param.WireKey == "WFFormValues" || param.WireKey == "WFHTTPHeaders"

	if param.EvidenceStatus == schema.EvidenceConfirmed || isDictLiteralOnlyWireKey {
		switch expr := arg.(type) {
		case *syntax.LiteralExpr:
			if expr.Type == syntax.TokenNone || expr.Type == syntax.TokenNull {
				return
			}
			if param.WireKey == "WFInput" {
				return
			}
			if !hasLiteral && !(hasTokenString && param.Codec != "token") {
				a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q does not accept literal values; an attachment (variable or action output) is required", param.Label, callName))
			}
		case *syntax.ListExpr, *syntax.MapExpr:
			if param.WireKey == "WFInput" || param.WireKey == "WFContentItemFilter" {
				return
			}
			if !hasLiteral && !(hasTokenString && param.Codec != "token") {
				a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q does not accept literal values; an attachment (variable or action output) is required", param.Label, callName))
			}
		case *syntax.FStringExpr:
			if !hasTokenString && !hasLiteral && param.WireKey != "WFInput" {
				a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q does not accept text token strings; an attachment is required", param.Label, callName))
			}
		case *syntax.IdentExpr, *syntax.MemberExpr, *syntax.CallExpr, *syntax.IndexExpr:
			if !hasAttachment && !hasTokenString {
				if param.WireKey == "WFJSONValues" || param.WireKey == "WFFormValues" || param.WireKey == "WFHTTPHeaders" {
					a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q (%s) in call to %q requires a dictionary literal {...} (which may contain dynamic variable references inside its field values); to send a dynamic dictionary/JSON variable as the request body, use fileRequest(...) (WFHTTPBodyType=File, WFRequestVariable)", param.Label, param.WireKey, callName))
				} else {
					a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q requires a literal value, got dynamic variable/attachment", param.Label, callName))
				}
			}
		}
	} else {
		switch expr := arg.(type) {
		case *syntax.LiteralExpr:
			if expr.Type == syntax.TokenNone || expr.Type == syntax.TokenNull {
				return
			}
			if param.WireKey == "WFInput" {
				return
			}
			if !hasLiteral && !hasTokenString && hasAttachment {
				a.warn(CodeUnverifiedForm, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q may not accept literal values; attachment recommended", param.Label, callName))
			}
		}
	}
}

func (a *Analyzer) resolveType(t syntax.TypeAnnotation) types.Type {
	if t == nil {
		return types.Unknown
	}

	switch ta := t.(type) {
	case *syntax.NamedTypeAnnotation:
		if nt, ok := types.ParseNamedType(ta.Name); ok {
			return nt
		}
		if ta.Namespace != "" {
			return types.NewEntity(ta.Namespace, ta.Name)
		}
		if a.enumCatalog.Has(ta.Name) {
			return types.Text
		}
		return types.Unknown

	case *syntax.OptionalTypeAnnotation:
		return types.NewOptional(a.resolveType(ta.Inner))

	case *syntax.GenericTypeAnnotation:
		if ta.Name == "List" && len(ta.TypeArgs) == 1 {
			return types.NewList(a.resolveType(ta.TypeArgs[0]))
		}
		if ta.Name == "Map" && len(ta.TypeArgs) == 2 {
			return types.NewMap(a.resolveType(ta.TypeArgs[0]), a.resolveType(ta.TypeArgs[1]))
		}
		return types.Unknown

	case *syntax.UnionTypeAnnotation:
		var members []types.Type
		for _, m := range ta.Members {
			members = append(members, a.resolveType(m))
		}
		return types.NewUnion(members...)

	case *syntax.RecordTypeAnnotation:
		var fields []types.RecordField
		for _, rf := range ta.Fields {
			fields = append(fields, types.RecordField{
				Name:     rf.Name,
				Type:     a.resolveType(rf.TypeExpr),
				Optional: rf.Optional,
			})
		}
		return types.NewRecord(fields)

	default:
		return types.Unknown
	}
}

func isNumericLiteralOrTokenString(expr syntax.Expression) bool {
	if lit, ok := expr.(*syntax.LiteralExpr); ok {
		clean := strings.TrimSpace(strings.TrimSuffix(lit.Value, "%"))
		if clean == "" {
			return false
		}
		_, err := strconv.ParseFloat(clean, 64)
		return err == nil
	}
	return false
}

func isAttachmentCapableTextParamSchema(appleIdentifier, wireKey string, expr syntax.Expression) bool {
	switch wireKey {
	case "WFImageResizeWidth",
		"WFImageResizeHeight",
		"WFImageResizePercentage",
		"WFImageResizeLength",
		"WFImageCropWidth",
		"WFImageCropHeight",
		"WFImageCropX",
		"WFImageCropY",
		"WFImageWidth",
		"WFImageHeight",
		"WFImageX",
		"WFImageY",
		"WFRotation",
		"WFOverlayImageOpacity",
		"WFMaskCornerRadius":
		if lit, ok := expr.(*syntax.LiteralExpr); ok {
			return lit.Type == syntax.TokenNumber || lit.Type == syntax.TokenString || lit.Type == syntax.TokenRawString
		}
		return isReferenceLikeExpr(expr)
	}
	switch appleIdentifier {
	case "is.workflow.actions.text.split", "is.workflow.actions.text.combine":
		if wireKey == "text" || wireKey == "" {
			return isReferenceLikeExpr(expr)
		}
	case "is.workflow.actions.text.changecase", "is.workflow.actions.correctspelling":
		if wireKey == "text" {
			return isReferenceLikeExpr(expr)
		}
	case "is.workflow.actions.openurl", "is.workflow.actions.detect.link":
		if wireKey == "WFInput" {
			return isReferenceLikeExpr(expr)
		}
	case "is.workflow.actions.geturlcomponent":
		if wireKey == "WFURL" {
			return isReferenceLikeExpr(expr)
		}
	case "is.workflow.actions.url.expand":
		if wireKey == "URL" {
			return isReferenceLikeExpr(expr)
		}
	}
	return false
}

func isReferenceLikeExpr(expr syntax.Expression) bool {
	switch expr.(type) {
	case *syntax.IdentExpr, *syntax.MemberExpr, *syntax.IndexExpr, *syntax.CallExpr:
		return true
	default:
		return false
	}
}
