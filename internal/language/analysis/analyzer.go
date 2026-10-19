package analysis

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Symbol represents a declared entity.
type Symbol struct {
	Name        string
	Type        types.Type
	Mutable     bool
	Initialized bool
	Span        source.Span
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
	registry          *schema.Registry
	diagnostics       []Diagnostic
	currentFn         *syntax.FunctionDecl
	inValueBlk        bool
	topLevelVars      map[string]source.Span
	yieldedTypes      []types.Type
	importStack       []string
	isMacTarget       *bool
	exprTypes         map[syntax.Expression]types.Type
	targets           []string
	hasExplicitTarget bool
	minTargetVersion  float64
	maxTargetVersion  float64
	userFunctions     map[string]bool
	loopDepth         int
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

// MinTargetVersion returns the declared minimum target OS version.
func (a *Analyzer) MinTargetVersion() float64 {
	return a.minTargetVersion
}

// SetMinTargetVersion sets the minimum target OS version.
func (a *Analyzer) SetMinTargetVersion(v float64) {
	a.minTargetVersion = v
}

// MaxTargetVersion returns the declared maximum target OS version.
func (a *Analyzer) MaxTargetVersion() float64 {
	return a.maxTargetVersion
}

// SetMaxTargetVersion sets the maximum target OS version.
func (a *Analyzer) SetMaxTargetVersion(v float64) {
	a.maxTargetVersion = v
}

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

	scope := NewScope(nil)
	a.topLevelVars = make(map[string]source.Span)
	a.userFunctions = make(map[string]bool)

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
			if !scope.Define(&Symbol{Name: d.Name, Type: fnType, Initialized: true, Span: d.Span}) {
				a.error(CodeDuplicateName, d.Span, fmt.Sprintf("duplicate function declaration %q", d.Name))
			}
		case *syntax.SetupDecl:
			var sType types.Type = types.Text
			if d.TypeExpr != nil {
				sType = a.resolveType(d.TypeExpr)
			}
			if !scope.Define(&Symbol{Name: d.Name, Type: sType, Initialized: true, Span: d.Span}) {
				a.error(CodeDuplicateName, d.Span, fmt.Sprintf("duplicate setup declaration %q", d.Name))
			}
		case *syntax.EnumDecl:
			if !scope.Define(&Symbol{Name: d.Name, Type: types.Text, Initialized: true, Span: d.Span}) {
				a.error(CodeDuplicateName, d.Span, fmt.Sprintf("duplicate enum declaration %q", d.Name))
			}
		case *syntax.ActionDecl:
			var actParams []schema.ParameterSchema
			for _, p := range d.Parameters {
				var pType types.Type = types.Unknown
				if p.TypeExpr != nil {
					pType = a.resolveType(p.TypeExpr)
				}
				actParams = append(actParams, schema.ParameterSchema{
					ID:       p.Name,
					Label:    p.Name,
					Type:     pType,
					Optional: p.Optional || p.DefaultExpr != nil,
				})
			}
			retTypeName := "Void"
			if d.ReturnType != nil {
				retTypeName = a.resolveType(d.ReturnType).Name()
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
		if fn, ok := decl.(*syntax.FunctionDecl); ok {
			a.analyzeFunction(fn, scope)
		}
	}

	// Pass 3: Analyze top-level statements
	for _, stmt := range prog.Statements {
		a.analyzeStatement(stmt, scope)
	}
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
			if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
				if v, err := strconv.ParseFloat(lit.Value, 64); err == nil {
					a.minTargetVersion = v
				}
			}
		case "maxVersion":
			if lit, ok := f.Value.(*syntax.LiteralExpr); ok {
				if v, err := strconv.ParseFloat(lit.Value, 64); err == nil {
					a.maxTargetVersion = v
				}
			}
		}
	}
}

func (a *Analyzer) analyzeFunction(fn *syntax.FunctionDecl, parentScope *Scope) {
	fnScope := NewScope(parentScope)
	a.currentFn = fn

	for _, p := range fn.Parameters {
		pType := a.resolveType(p.TypeExpr)
		if p.DefaultExpr != nil {
			defType := a.analyzeExpression(p.DefaultExpr, fnScope)
			if defType != nil && !defType.AssignableTo(pType) {
				a.error(CodeArgumentType, p.DefaultExpr.NodeSpan(), fmt.Sprintf("default value of type %s is not assignable to parameter type %s", defType, pType))
			}
		}
		fnScope.Define(&Symbol{
			Name:        p.Name,
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
			valType = a.analyzeExpression(s.Value, scope)
		}

		declType := valType
		if s.TypeExpr != nil {
			annotated := a.resolveType(s.TypeExpr)
			if valType != nil && !valType.AssignableTo(annotated) {
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

		if _, found := scope.LookupLocal(s.Name); found {
			a.error(CodeDuplicateName, s.Span, fmt.Sprintf("duplicate variable declaration %q in this scope", s.Name))
			return
		}

		if parentSym, exists := scope.Lookup(s.Name); exists {
			a.warn(CodeShadowing, s.Span, fmt.Sprintf("variable %q shadows an existing declaration at %s", s.Name, parentSym.Span))
		}

		scope.Define(&Symbol{
			Name:        s.Name,
			Type:        declType,
			Mutable:     s.Mutable,
			Initialized: s.Value != nil,
			Span:        s.Span,
		})

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

		valType := a.analyzeExpression(s.Value, scope)
		if valType != nil && sym.Type != nil {
			if s.Op == syntax.TokenPlusAssign {
				if listType, ok := sym.Type.(types.ListType); ok {
					if !valType.AssignableTo(listType.Element) && !valType.AssignableTo(sym.Type) && valType.Kind() != types.KindUnknown {
						a.error(CodeArgumentType, s.Value.NodeSpan(), fmt.Sprintf("value of type %s is not assignable to list element of type %s", valType, listType.Element))
					}
				} else if !valType.AssignableTo(sym.Type) {
					a.error(CodeArgumentType, s.Value.NodeSpan(), fmt.Sprintf("value of type %s is not assignable to variable %q of type %s", valType, s.Name, sym.Type))
				}
			} else if !valType.AssignableTo(sym.Type) {
				a.error(CodeArgumentType, s.Value.NodeSpan(), fmt.Sprintf("value of type %s is not assignable to variable %q of type %s", valType, s.Name, sym.Type))
			}
		}
		sym.Initialized = true

	case *syntax.ExprStmt:
		a.analyzeExpression(s.Expr, scope)

	case *syntax.ReturnStmt:
		var retType types.Type = types.Void
		if s.Value != nil {
			retType = a.analyzeExpression(s.Value, scope)
		}
		if a.currentFn != nil && a.currentFn.ReturnType != nil {
			expected := a.resolveType(a.currentFn.ReturnType)
			if !retType.AssignableTo(expected) {
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
		thenScope := NewScope(scope)
		for _, ts := range s.ThenBlock.Statements {
			a.analyzeStatement(ts, thenScope)
		}
		if s.ElseBlock != nil {
			a.analyzeStatement(s.ElseBlock, scope)
		}

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
		a.loopDepth++
		for _, bs := range s.BodyBlock.Statements {
			a.analyzeStatement(bs, forScope)
		}
		a.loopDepth--

	case *syntax.RepeatStmt:
		repScope := NewScope(scope)
		cntType := a.analyzeExpression(s.Count, scope)
		if cntType != nil && !cntType.AssignableTo(types.Number) && cntType.Kind() != types.KindUnknown {
			a.error(CodeArgumentType, s.Count.NodeSpan(), fmt.Sprintf("repeat count requires Number, got %s", cntType))
		}
		if s.IndexVar != "" && s.IndexVar != "_" {
			repScope.Define(&Symbol{Name: s.IndexVar, Type: types.Number, Initialized: true, Span: s.Span})
		}
		repScope.Define(&Symbol{Name: "RepeatIndex", Type: types.Number, Initialized: true, Span: s.Span})
		a.loopDepth++
		for _, bs := range s.BodyBlock.Statements {
			a.analyzeStatement(bs, repScope)
		}
		a.loopDepth--

	case *syntax.MenuStmt:
		a.analyzeExpression(s.Prompt, scope)
		for _, c := range s.Cases {
			caseScope := NewScope(scope)
			for _, cs := range c.BodyBlock.Statements {
				a.analyzeStatement(cs, caseScope)
			}
		}
	}
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
	if a.exprTypes != nil {
		a.exprTypes[expr] = t
	}
	return t
}

func (a *Analyzer) analyzeExpressionInternal(expr syntax.Expression, scope *Scope) types.Type {

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
			return types.JsonNull
		default:
			return types.Unknown
		}

	case *syntax.FStringExpr:
		for _, part := range e.Parts {
			if part.IsExpr {
				a.analyzeExpression(part.Expr, scope)
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
		if !sym.Initialized {
			a.error(CodeReadBeforeInit, e.Span, fmt.Sprintf("variable %q read before initialization", e.Name))
		}
		return sym.Type

	case *syntax.MemberExpr:
		targetType := a.analyzeExpression(e.Target, scope)
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
			return types.Unknown
		}

	case *syntax.EnumMemberExpr:
		return types.Text

	case *syntax.IndexExpr:
		targetType := a.analyzeExpression(e.Target, scope)
		idxType := a.analyzeExpression(e.Index, scope)
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
		if e.Op == syntax.TokenBang {
			return types.Bool
		}
		if e.Op == syntax.TokenMinus || e.Op == syntax.TokenPlus {
			if opType != nil && !opType.AssignableTo(types.Number) && opType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Operand.NodeSpan(), fmt.Sprintf("unary '-' requires Number, got %s", opType))
			}
			return types.Number
		}
		return types.Unknown

	case *syntax.BinaryExpr:
		leftType := a.analyzeExpression(e.Left, scope)
		rightType := a.analyzeExpression(e.Right, scope)

		switch e.Op {
		case syntax.TokenPlus, syntax.TokenMinus, syntax.TokenStar, syntax.TokenSlash, syntax.TokenPercent:
			if e.Op == syntax.TokenPlus && leftType != nil && rightType != nil && leftType.AssignableTo(types.Text) && rightType.AssignableTo(types.Text) {
				return types.Text
			}
			if leftType != nil && !leftType.AssignableTo(types.Number) && leftType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Left.NodeSpan(), fmt.Sprintf("numeric operator requires Number, got %s", leftType))
			}
			if rightType != nil && !rightType.AssignableTo(types.Number) && rightType.Kind() != types.KindUnknown {
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
				if t == nil || t.Kind() == types.KindUnknown || t.Kind() == types.KindAnyContent {
					return true
				}
				if t.AssignableTo(types.Number) || t.AssignableTo(types.Date) {
					return true
				}
				return false
			}
			if !isComparable(leftType) {
				a.error(CodeArgumentType, e.Left.NodeSpan(), fmt.Sprintf("comparison requires Number or Date, got %s", leftType))
			}
			if !isComparable(rightType) {
				a.error(CodeArgumentType, e.Right.NodeSpan(), fmt.Sprintf("comparison requires Number or Date, got %s", rightType))
			}
			return types.Bool

		case syntax.TokenAnd, syntax.TokenOr:
			return types.Bool

		case syntax.TokenContains, syntax.TokenBeginsWith, syntax.TokenEndsWith:
			return types.Bool
		}

	case *syntax.ListExpr:
		var elemType types.Type = types.Unknown
		for _, el := range e.Elements {
			t := a.analyzeExpression(el, scope)
			if elemType.Kind() == types.KindUnknown {
				elemType = t
			}
		}
		return types.NewList(elemType)

	case *syntax.MapExpr:
		var valType types.Type = types.Unknown
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
			a.analyzeExpression(entry.Key, scope)
			vt := a.analyzeExpression(entry.Value, scope)
			if valType.Kind() == types.KindUnknown {
				valType = vt
			}
		}
		return types.NewMap(types.Text, valType)

	case *syntax.IfStmt:
		oldVal := a.inValueBlk
		oldYields := a.yieldedTypes
		a.inValueBlk = true
		a.yieldedTypes = nil
		a.analyzeStatement(e, scope)
		yields := a.yieldedTypes
		a.yieldedTypes = oldYields
		a.inValueBlk = oldVal
		if e.ElseBlock == nil {
			a.error(CodeYieldContext, e.Span, "value-producing 'if' expression requires 'else'")
		}
		if len(yields) > 0 {
			return yields[0]
		}
		return types.AnyContent

	case *syntax.RepeatStmt:
		oldVal := a.inValueBlk
		oldYields := a.yieldedTypes
		a.inValueBlk = true
		a.yieldedTypes = nil
		a.analyzeStatement(e, scope)
		yields := a.yieldedTypes
		a.yieldedTypes = oldYields
		a.inValueBlk = oldVal
		elemType := types.Type(types.AnyContent)
		if len(yields) > 0 {
			elemType = yields[0]
		}
		return types.NewList(elemType)

	case *syntax.ForStmt:
		oldVal := a.inValueBlk
		oldYields := a.yieldedTypes
		a.inValueBlk = true
		a.yieldedTypes = nil
		a.analyzeStatement(e, scope)
		yields := a.yieldedTypes
		a.yieldedTypes = oldYields
		a.inValueBlk = oldVal
		elemType := types.Type(types.AnyContent)
		if len(yields) > 0 {
			elemType = yields[0]
		}
		return types.NewList(elemType)

	case *syntax.MenuStmt:
		oldVal := a.inValueBlk
		oldYields := a.yieldedTypes
		a.inValueBlk = true
		a.yieldedTypes = nil
		a.analyzeStatement(e, scope)
		yields := a.yieldedTypes
		a.yieldedTypes = oldYields
		a.inValueBlk = oldVal
		for _, c := range e.Cases {
			if !blockHasYield(c.BodyBlock) {
				a.error(CodeYieldContext, c.Span, fmt.Sprintf("value-producing 'menu' case %q requires 'yield'", c.Label))
			}
		}
		if len(yields) > 0 {
			return yields[0]
		}
		return types.Unknown
	}

	return types.Unknown
}

func (a *Analyzer) analyzeCall(call *syntax.CallExpr, scope *Scope) types.Type {
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
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		for _, nArg := range call.NamedArgs {
			a.analyzeExpression(nArg.Value, scope)
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

	// 1c. Check isToday predicate
	if callName == "isToday" {
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		return types.Bool
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
		if a.minTargetVersion > 0 && actionSchema.MinVersion > 0 && actionSchema.MinVersion > a.minTargetVersion {
			a.error(CodeUnsupportedTarget, call.Span, fmt.Sprintf("action %q requires minimum version %v, but shortcut target version is %v", callName, actionSchema.MinVersion, a.minTargetVersion))
		}
		if a.maxTargetVersion > 0 && actionSchema.MaxVersion > 0 && actionSchema.MaxVersion < a.maxTargetVersion {
			a.error(CodeUnsupportedTarget, call.Span, fmt.Sprintf("action %q is unsupported above version %v (shortcut target version is %v)", callName, actionSchema.MaxVersion, a.maxTargetVersion))
		}
	} else {
		if actionSchema.MacOnly {
			a.warn(CodeUnverifiedForm, call.Span, fmt.Sprintf("action %q is only available on macOS, but shortcut target is unspecified; specify target in shortcut metadata", callName))
		}
		if actionSchema.NonMacOnly {
			a.warn(CodeUnverifiedForm, call.Span, fmt.Sprintf("action %q is not available on macOS, but shortcut target is unspecified; specify target in shortcut metadata", callName))
		}
	}

	// Step-by-step argument binding algorithm per §7.2
	boundParams := make(map[string]bool)

	// Step 2: Primary argument binding
	if call.PrimaryArg != nil {
		primParam, hasPrim := actionSchema.PrimaryParameter()
		if !hasPrim {
			a.error(CodePositionalArg, call.PrimaryArg.NodeSpan(), fmt.Sprintf("action %q does not accept an unlabeled primary argument; use named arguments", callName))
		} else {
			boundParams[primParam.ID] = true
			if primParam.WireKey == "WFDictionaryKey" {
				if lit, isLit := call.PrimaryArg.(*syntax.LiteralExpr); isLit && lit.Value == "" {
					a.error(CodeArgumentType, call.PrimaryArg.NodeSpan(), "empty dictionary key is not allowed")
				}
			}
			a.checkAcceptedForms(primParam, call.PrimaryArg, callName)
			argType := a.analyzeExpression(call.PrimaryArg, scope)
			if primParam.Type != nil && argType != nil && !argType.AssignableTo(primParam.Type) {
				if !(primParam.Type == types.Number && argType == types.Text && isNumericLiteralOrTokenString(call.PrimaryArg)) {
					a.error(CodeArgumentType, call.PrimaryArg.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", primParam.Label, primParam.Type, argType))
				}
			}
		}
	}

	// Step 3 & 4: Named arguments binding
	paramIndex := 1
	if call.PrimaryArg == nil {
		paramIndex = 0
	}
	for _, nArg := range call.NamedArgs {
		var param *schema.ParameterSchema
		var paramExists bool
		if nArg.Label != "" {
			param, paramExists = actionSchema.ParameterByLabel(nArg.Label)
		} else {
			for paramIndex < len(actionSchema.Parameters) && boundParams[actionSchema.Parameters[paramIndex].ID] {
				paramIndex++
			}
			if paramIndex < len(actionSchema.Parameters) {
				param = &actionSchema.Parameters[paramIndex]
				paramExists = true
				paramIndex++
			} else if len(actionSchema.Parameters) > 0 && actionSchema.Parameters[len(actionSchema.Parameters)-1].Variadic {
				param = &actionSchema.Parameters[len(actionSchema.Parameters)-1]
				paramExists = true
			}
		}
		if !paramExists {
			a.error(CodeUnknownArgument, nArg.Span, fmt.Sprintf("unknown argument label %q in call to %q", nArg.Label, callName))
			a.analyzeExpression(nArg.Value, scope)
			continue
		}

		if boundParams[param.ID] && !param.Variadic {
			a.error(CodeDuplicateArgument, nArg.Span, fmt.Sprintf("duplicate argument for parameter %q in call to %q", nArg.Label, callName))
		}
		boundParams[param.ID] = true

		if param.WireKey == "WFDictionaryKey" || nArg.Label == "key" {
			if lit, isLit := nArg.Value.(*syntax.LiteralExpr); isLit && lit.Value == "" {
				a.error(CodeArgumentType, nArg.Value.NodeSpan(), "empty dictionary key is not allowed")
			}
		}

		a.checkAcceptedForms(param, nArg.Value, callName)
		argType := a.analyzeExpression(nArg.Value, scope)
		if param.Type != nil && argType != nil && !argType.AssignableTo(param.Type) {
			if !(param.Type == types.Number && argType == types.Text && isNumericLiteralOrTokenString(nArg.Value)) {
				a.error(CodeArgumentType, nArg.Value.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", param.Label, param.Type, argType))
			}
		}

		// Enum validation
		if param.Codec == "enum" && len(param.EnumValues) > 0 {
			if lit, isLit := nArg.Value.(*syntax.LiteralExpr); isLit {
				foundVal := false
				for _, ev := range param.EnumValues {
					if ev == lit.Value {
						foundVal = true
						break
					}
				}
				if !foundVal {
					a.error(CodeEnumMember, nArg.Value.NodeSpan(), fmt.Sprintf("invalid enum value %q for parameter %q; valid choices: %s", lit.Value, param.Label, strings.Join(param.EnumValues, ", ")))
				}
			}
		}
	}

	// Step 6: Validate requiredness
	for _, p := range actionSchema.Parameters {
		if !p.Optional && !boundParams[p.ID] {
			a.error(CodeMissingArgument, call.Span, fmt.Sprintf("missing required argument %q in call to %q", p.Label, callName))
		}
	}

	if actionSchema.OutputType != nil {
		return actionSchema.OutputType
	}
	return types.Unknown
}

func (a *Analyzer) checkAcceptedForms(param *schema.ParameterSchema, arg syntax.Expression, callName string) {
	if param == nil || len(param.AcceptedForms) == 0 {
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

	if param.EvidenceStatus == schema.EvidenceConfirmed {
		switch expr := arg.(type) {
		case *syntax.LiteralExpr:
			if expr.Type == syntax.TokenNone || expr.Type == syntax.TokenNull {
				return
			}
			if param.WireKey == "WFInput" {
				return
			}
			if !hasLiteral && !hasTokenString {
				a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q does not accept literal values; an attachment (variable or action output) is required", param.Label, callName))
			}
		case *syntax.ListExpr, *syntax.MapExpr:
			if param.WireKey == "WFInput" || param.WireKey == "WFContentItemFilter" {
				return
			}
			if !hasLiteral && !hasTokenString {
				a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q does not accept literal values; an attachment (variable or action output) is required", param.Label, callName))
			}
		case *syntax.FStringExpr:
			if !hasTokenString && !hasLiteral && param.WireKey != "WFInput" {
				a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q does not accept text token strings; an attachment is required", param.Label, callName))
			}
		case *syntax.IdentExpr, *syntax.MemberExpr, *syntax.CallExpr, *syntax.IndexExpr:
			if !hasAttachment && !hasTokenString {
				a.error(CodeArgumentType, expr.NodeSpan(), fmt.Sprintf("parameter %q in call to %q requires a literal value, got dynamic variable/attachment", param.Label, callName))
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
