package analysis

import (
	"fmt"
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
	registry    *schema.Registry
	diagnostics []Diagnostic
	currentFn   *syntax.FunctionDecl
	inValueBlk  bool
}

// NewAnalyzer creates a new Analyzer with the action registry.
func NewAnalyzer(registry *schema.Registry) *Analyzer {
	if registry == nil {
		registry = schema.DefaultRegistry()
	}
	return &Analyzer{
		registry: registry,
	}
}

// Diagnostics returns all diagnostics produced during analysis.
func (a *Analyzer) Diagnostics() []Diagnostic {
	return a.diagnostics
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

// Analyze performs full semantic verification of a Program.
func (a *Analyzer) Analyze(prog *syntax.Program) {
	scope := NewScope(nil)

	// Builtin namespace symbols
	scope.Define(&Symbol{Name: "system", Type: types.Unknown, Initialized: true})
	scope.Define(&Symbol{Name: "native", Type: types.Unknown, Initialized: true})
	scope.Define(&Symbol{Name: "builtin", Type: types.Unknown, Initialized: true})

	// Pass 1: Collect declarations
	for _, decl := range prog.Declarations {
		switch d := decl.(type) {
		case *syntax.FunctionDecl:
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
		case *syntax.ImportDecl:
			if !scope.Define(&Symbol{Name: d.Alias, Type: types.Unknown, Initialized: true, Span: d.Span}) {
				a.error(CodeDuplicateName, d.Span, fmt.Sprintf("duplicate import alias %q", d.Alias))
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
		if valType != nil && sym.Type != nil && !valType.AssignableTo(sym.Type) {
			a.error(CodeArgumentType, s.Value.NodeSpan(), fmt.Sprintf("value of type %s is not assignable to variable %q of type %s", valType, s.Name, sym.Type))
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
		a.analyzeExpression(s.Value, scope)

	case *syntax.BlockStmt:
		blockScope := NewScope(scope)
		for _, bs := range s.Statements {
			a.analyzeStatement(bs, blockScope)
		}

	case *syntax.IfStmt:
		condType := a.analyzeExpression(s.Condition, scope)
		if condType != nil && !condType.AssignableTo(types.Bool) && condType.Kind() != types.KindUnknown {
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
		if s.IndexVar != "" {
			forScope.Define(&Symbol{Name: s.IndexVar, Type: types.Number, Initialized: true, Span: s.Span})
		}
		if s.ItemVar != "" {
			forScope.Define(&Symbol{Name: s.ItemVar, Type: elemType, Initialized: true, Span: s.Span})
		}
		for _, bs := range s.BodyBlock.Statements {
			a.analyzeStatement(bs, forScope)
		}

	case *syntax.RepeatStmt:
		repScope := NewScope(scope)
		cntType := a.analyzeExpression(s.Count, scope)
		if cntType != nil && !cntType.AssignableTo(types.Number) && cntType.Kind() != types.KindUnknown {
			a.error(CodeArgumentType, s.Count.NodeSpan(), fmt.Sprintf("repeat count requires Number, got %s", cntType))
		}
		if s.IndexVar != "" {
			repScope.Define(&Symbol{Name: s.IndexVar, Type: types.Number, Initialized: true, Span: s.Span})
		}
		for _, bs := range s.BodyBlock.Statements {
			a.analyzeStatement(bs, repScope)
		}

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

func (a *Analyzer) analyzeExpression(expr syntax.Expression, scope *Scope) types.Type {
	if expr == nil {
		return types.Unknown
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
		return types.Unknown

	case *syntax.EnumMemberExpr:
		return types.Text

	case *syntax.IndexExpr:
		targetType := a.analyzeExpression(e.Target, scope)
		idxType := a.analyzeExpression(e.Index, scope)
		if idxType != nil && !idxType.AssignableTo(types.Number) && idxType.Kind() != types.KindUnknown {
			a.error(CodeArgumentType, e.Index.NodeSpan(), fmt.Sprintf("index expression requires Number, got %s", idxType))
		}
		if listType, ok := targetType.(types.ListType); ok {
			return types.NewOptional(listType.Element)
		}
		if mapType, ok := targetType.(types.MapType); ok {
			return types.NewOptional(mapType.Value)
		}
		return types.Unknown

	case *syntax.CallExpr:
		return a.analyzeCall(e, scope)

	case *syntax.UnaryExpr:
		opType := a.analyzeExpression(e.Operand, scope)
		if e.Op == syntax.TokenBang {
			if opType != nil && !opType.AssignableTo(types.Bool) && opType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Operand.NodeSpan(), fmt.Sprintf("unary '!' requires Bool, got %s", opType))
			}
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
			if leftType != nil && !leftType.AssignableTo(types.Number) && leftType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Left.NodeSpan(), fmt.Sprintf("numeric operator requires Number, got %s", leftType))
			}
			if rightType != nil && !rightType.AssignableTo(types.Number) && rightType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Right.NodeSpan(), fmt.Sprintf("numeric operator requires Number, got %s", rightType))
			}
			return types.Number

		case syntax.TokenEqual, syntax.TokenNotEqual:
			return types.Bool

		case syntax.TokenLess, syntax.TokenLessEqual, syntax.TokenGreater, syntax.TokenGreaterEqual:
			if leftType != nil && !leftType.AssignableTo(types.Number) && leftType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Left.NodeSpan(), fmt.Sprintf("comparison requires Number, got %s", leftType))
			}
			if rightType != nil && !rightType.AssignableTo(types.Number) && rightType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Right.NodeSpan(), fmt.Sprintf("comparison requires Number, got %s", rightType))
			}
			return types.Bool

		case syntax.TokenAnd, syntax.TokenOr:
			if leftType != nil && !leftType.AssignableTo(types.Bool) && leftType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Left.NodeSpan(), fmt.Sprintf("logical operator requires Bool, got %s", leftType))
			}
			if rightType != nil && !rightType.AssignableTo(types.Bool) && rightType.Kind() != types.KindUnknown {
				a.error(CodeArgumentType, e.Right.NodeSpan(), fmt.Sprintf("logical operator requires Bool, got %s", rightType))
			}
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
		a.inValueBlk = true
		a.analyzeStatement(e, scope)
		a.inValueBlk = oldVal
		return types.Unknown

	case *syntax.RepeatStmt:
		oldVal := a.inValueBlk
		a.inValueBlk = true
		a.analyzeStatement(e, scope)
		a.inValueBlk = oldVal
		return types.NewList(types.Unknown)

	case *syntax.ForStmt:
		oldVal := a.inValueBlk
		a.inValueBlk = true
		a.analyzeStatement(e, scope)
		a.inValueBlk = oldVal
		return types.NewList(types.Unknown)

	case *syntax.MenuStmt:
		oldVal := a.inValueBlk
		a.inValueBlk = true
		a.analyzeStatement(e, scope)
		a.inValueBlk = oldVal
		return types.Unknown
	}

	return types.Unknown
}

func (a *Analyzer) analyzeCall(call *syntax.CallExpr, scope *Scope) types.Type {
	calleeIdent, ok := call.Callee.(*syntax.IdentExpr)
	if !ok {
		// e.g. callee is a member access or complex expression
		a.analyzeExpression(call.Callee, scope)
		return types.Unknown
	}

	callName := calleeIdent.Name

	// 0. Check native escape / rawAction
	if callName == "rawAction" || callName == "native.action" || callName == "action" {
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
		// Evaluate arguments
		if call.PrimaryArg != nil {
			a.analyzeExpression(call.PrimaryArg, scope)
		}
		for _, nArg := range call.NamedArgs {
			a.analyzeExpression(nArg.Value, scope)
		}
		return sym.Type
	}

	// 2. Check Action in registry
	actionSchema, isAction := a.registry.LookupAction(callName)
	if !isAction {
		a.error(CodeUnknownAction, call.Callee.NodeSpan(), fmt.Sprintf("unknown action or function %q", callName))
		return types.Unknown
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
			argType := a.analyzeExpression(call.PrimaryArg, scope)
			if primParam.Type != nil && argType != nil && !argType.AssignableTo(primParam.Type) {
				a.error(CodeArgumentType, call.PrimaryArg.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", primParam.Label, primParam.Type, argType))
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
			if paramIndex < len(actionSchema.Parameters) {
				param = &actionSchema.Parameters[paramIndex]
				paramExists = true
				paramIndex++
			}
		}
		if !paramExists {
			a.error(CodeUnknownArgument, nArg.Span, fmt.Sprintf("unknown argument label %q in call to %q", nArg.Label, callName))
			a.analyzeExpression(nArg.Value, scope)
			continue
		}

		if boundParams[param.ID] {
			a.error(CodeDuplicateArgument, nArg.Span, fmt.Sprintf("duplicate argument for parameter %q in call to %q", nArg.Label, callName))
		}
		boundParams[param.ID] = true

		argType := a.analyzeExpression(nArg.Value, scope)
		if param.Type != nil && argType != nil && !argType.AssignableTo(param.Type) {
			a.error(CodeArgumentType, nArg.Value.NodeSpan(), fmt.Sprintf("argument for %s expects %s, got %s", param.Label, param.Type, argType))
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
