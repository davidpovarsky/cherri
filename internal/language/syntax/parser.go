package syntax

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/source"
)

// ParseError records a syntax error with source span.
type ParseError struct {
	Span    source.Span
	Message string
}

func (e ParseError) String() string {
	return fmt.Sprintf("%s: %s", e.Span, e.Message)
}

// Precedence levels for Pratt expression parsing
const (
	precNone = iota
	precOr
	precAnd
	precEquality
	precRelational
	precAdditive
	precMultiplicative
	precUnary
	precPostfix
)

// Parser parses Cherri v2 source tokens into an AST.
type Parser struct {
	file   *source.File
	lexer  *Lexer
	curr   Token
	peek   Token
	errors []ParseError
}

// NewParser creates a new Parser for a source file.
func NewParser(file *source.File) *Parser {
	l := NewLexer(file)
	p := &Parser{
		file:  file,
		lexer: l,
	}
	p.curr = p.lexer.NextToken()
	p.peek = p.lexer.NextToken()
	return p
}

// Errors returns all accumulated syntax errors.
func (p *Parser) Errors() []ParseError {
	return p.errors
}

func (p *Parser) errorAt(span source.Span, msg string) {
	p.errors = append(p.errors, ParseError{Span: span, Message: msg})
}

func (p *Parser) advance() Token {
	prev := p.curr
	p.curr = p.peek
	p.peek = p.lexer.NextToken()
	return prev
}

func (p *Parser) expect(tt TokenType) (Token, bool) {
	if p.curr.Type == tt {
		return p.advance(), true
	}
	p.errorAt(p.curr.Span, fmt.Sprintf("expected %v, got %v (%q)", tt, p.curr.Type, p.curr.Text))
	return p.curr, false
}

func (p *Parser) match(tt TokenType) bool {
	if p.curr.Type == tt {
		p.advance()
		return true
	}
	return false
}

// ParseProgram parses an entire Cherri v2 file.
func (p *Parser) ParseProgram() *Program {
	prog := &Program{
		Span: source.Span{Document: p.file.ID},
	}

	for p.curr.Type != TokenEOF {
		// Skip semicolons
		if p.curr.Type == TokenSemicolon {
			p.advance()
			continue
		}

		if p.isDeclarationStart() {
			decl := p.parseDeclaration()
			if decl != nil {
				prog.Declarations = append(prog.Declarations, decl)
			}
		} else {
			stmt := p.parseStatement()
			if stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		}
	}

	if len(prog.Statements) > 0 {
		prog.Span = prog.Statements[0].NodeSpan().Merge(prog.Statements[len(prog.Statements)-1].NodeSpan())
	}
	return prog
}

func (p *Parser) isDeclarationStart() bool {
	switch p.curr.Type {
	case TokenImport, TokenShortcut, TokenSetup, TokenTypeKW, TokenEnum, TokenAction, TokenFunction:
		return true
	default:
		return false
	}
}

func (p *Parser) parseDeclaration() Declaration {
	switch p.curr.Type {
	case TokenImport:
		d := p.parseImportDecl()
		if d == nil {
			return nil
		}
		return d
	case TokenShortcut:
		d := p.parseShortcutDecl()
		if d == nil {
			return nil
		}
		return d
	case TokenSetup:
		d := p.parseSetupDecl()
		if d == nil {
			return nil
		}
		return d
	case TokenTypeKW:
		d := p.parseTypeDecl()
		if d == nil {
			return nil
		}
		return d
	case TokenEnum:
		d := p.parseEnumDecl()
		if d == nil {
			return nil
		}
		return d
	case TokenAction:
		d := p.parseActionDecl()
		if d == nil {
			return nil
		}
		return d
	case TokenFunction:
		d := p.parseFunctionDecl()
		if d == nil {
			return nil
		}
		return d
	default:
		p.errorAt(p.curr.Span, fmt.Sprintf("unexpected declaration keyword: %s", p.curr.Text))
		p.advance()
		return nil
	}
}

func (p *Parser) parseImportDecl() *ImportDecl {
	startTok := p.advance() // import
	pathTok, ok := p.expect(TokenString)
	if !ok {
		return nil
	}
	if _, ok := p.expect(TokenAs); !ok {
		return nil
	}
	aliasTok, ok := p.expect(TokenIdent)
	if !ok {
		return nil
	}
	p.match(TokenSemicolon)

	return &ImportDecl{
		Span:  startTok.Span.Merge(aliasTok.Span),
		Path:  pathTok.Text,
		Alias: aliasTok.Text,
	}
}

func (p *Parser) parseShortcutDecl() *ShortcutDecl {
	startTok := p.advance() // shortcut
	nameTok, ok := p.expect(TokenString)
	if !ok {
		return nil
	}
	if _, ok := p.expect(TokenLBrace); !ok {
		return nil
	}

	decl := &ShortcutDecl{
		Name: nameTok.Text,
	}

	for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
		if p.curr.Type == TokenTrigger {
			trig := p.parseTriggerDecl()
			if trig != nil {
				decl.Triggers = append(decl.Triggers, *trig)
			}
			continue
		}

		// Field in header metadata
		if p.curr.Type == TokenIdent {
			fieldTok := p.advance()
			if _, ok := p.expect(TokenColon); !ok {
				break
			}
			val := p.parseExpression(precNone)
			if decl.Metadata == nil {
				decl.Metadata = &RecordExpr{
					Span: fieldTok.Span,
				}
			}
			decl.Metadata.Fields = append(decl.Metadata.Fields, RecordFieldExpr{
				Name:  fieldTok.Text,
				Value: val,
			})
			p.match(TokenComma)
			continue
		}

		p.advance()
	}

	endTok, _ := p.expect(TokenRBrace)
	decl.Span = startTok.Span.Merge(endTok.Span)
	return decl
}

func (p *Parser) parseTriggerDecl() *TriggerDecl {
	startTok := p.advance() // trigger
	familyTok, ok := p.expect(TokenIdent)
	if !ok {
		return nil
	}
	event := ""
	if p.match(TokenLParen) {
		if p.curr.Type == TokenIdent && p.curr.Text == "event" {
			p.advance()
			p.expect(TokenColon)
			if p.match(TokenDot) {
				evTok, _ := p.expect(TokenIdent)
				event = evTok.Text
			}
		}
		p.expect(TokenRParen)
	}
	return &TriggerDecl{
		Span:   startTok.Span.Merge(familyTok.Span),
		Family: familyTok.Text,
		Event:  event,
	}
}

func (p *Parser) parseSetupDecl() *SetupDecl {
	startTok := p.advance() // setup
	nameTok, ok := p.expect(TokenIdent)
	if !ok {
		return nil
	}
	var typeAnno TypeAnnotation
	if p.match(TokenColon) {
		typeAnno = p.parseTypeAnnotation()
	}
	if _, ok := p.expect(TokenLBrace); !ok {
		return nil
	}

	prompt := ""
	defVal := ""
	for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
		if p.curr.Type == TokenIdent {
			field := p.advance().Text
			p.expect(TokenColon)
			valTok, _ := p.expect(TokenString)
			if field == "prompt" {
				prompt = valTok.Text
			} else if field == "default" {
				defVal = valTok.Text
			}
		} else {
			p.advance()
		}
	}
	endTok, _ := p.expect(TokenRBrace)

	return &SetupDecl{
		Span:         startTok.Span.Merge(endTok.Span),
		Name:         nameTok.Text,
		TypeExpr:     typeAnno,
		Prompt:       prompt,
		DefaultValue: defVal,
	}
}

func (p *Parser) parseTypeDecl() *TypeDecl {
	startTok := p.advance() // type
	nameTok, ok := p.expect(TokenIdent)
	if !ok {
		return nil
	}
	p.expect(TokenAssign)
	typeAnno := p.parseTypeAnnotation()
	p.match(TokenSemicolon)

	return &TypeDecl{
		Span:     startTok.Span.Merge(nameTok.Span),
		Name:     nameTok.Text,
		TypeExpr: typeAnno,
	}
}

func (p *Parser) parseEnumDecl() *EnumDecl {
	startTok := p.advance() // enum
	nameTok, ok := p.expect(TokenIdent)
	if !ok {
		return nil
	}
	p.expect(TokenLBrace)

	var members []EnumMember
	for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
		mTok, ok := p.expect(TokenIdent)
		if !ok {
			break
		}
		wire := mTok.Text
		if p.match(TokenAssign) {
			wireTok, _ := p.expect(TokenString)
			wire = wireTok.Text
		}
		members = append(members, EnumMember{
			Span:      mTok.Span,
			Name:      mTok.Text,
			WireValue: wire,
		})
		if !p.match(TokenComma) {
			break
		}
	}
	endTok, _ := p.expect(TokenRBrace)

	return &EnumDecl{
		Span:    startTok.Span.Merge(endTok.Span),
		Name:    nameTok.Text,
		Members: members,
	}
}

func (p *Parser) parseFunctionDecl() *FunctionDecl {
	startTok := p.advance() // function
	nameTok, ok := p.expect(TokenIdent)
	if !ok {
		return nil
	}
	p.expect(TokenLParen)

	var params []ParameterDecl
	for p.curr.Type != TokenRParen && p.curr.Type != TokenEOF {
		pTok, ok := p.expect(TokenIdent)
		if !ok {
			break
		}
		opt := p.match(TokenQuestion)
		p.expect(TokenColon)
		tAnno := p.parseTypeAnnotation()
		var defExpr Expression
		if p.match(TokenAssign) {
			defExpr = p.parseExpression(precNone)
		}
		params = append(params, ParameterDecl{
			Span:        pTok.Span,
			Name:        pTok.Text,
			Optional:    opt,
			TypeExpr:    tAnno,
			DefaultExpr: defExpr,
		})
		if !p.match(TokenComma) {
			break
		}
	}
	p.expect(TokenRParen)

	var retType TypeAnnotation
	if p.match(TokenArrow) {
		retType = p.parseTypeAnnotation()
	}

	body := p.parseBlockStmt()
	return &FunctionDecl{
		Span:       startTok.Span.Merge(body.Span),
		Name:       nameTok.Text,
		Parameters: params,
		ReturnType: retType,
		Body:       body,
	}
}

func (p *Parser) parseActionDecl() *ActionDecl {
	startTok := p.advance() // action
	nameTok, ok := p.expect(TokenIdent)
	if !ok {
		return nil
	}
	p.expect(TokenLParen)

	var params []ParameterDecl
	for p.curr.Type != TokenRParen && p.curr.Type != TokenEOF {
		pTok, ok := p.expect(TokenIdent)
		if !ok {
			break
		}
		p.expect(TokenColon)
		tAnno := p.parseTypeAnnotation()
		params = append(params, ParameterDecl{
			Span:     pTok.Span,
			Name:     pTok.Text,
			TypeExpr: tAnno,
		})
		if !p.match(TokenComma) {
			break
		}
	}
	p.expect(TokenRParen)

	var retType TypeAnnotation
	if p.match(TokenArrow) {
		retType = p.parseTypeAnnotation()
	}

	p.expect(TokenLBrace)
	appleIdent := ""
	primary := ""
	bindings := make(map[string]ActionBinding)

	for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
		if p.curr.Type == TokenIdent {
			field := p.advance().Text
			p.expect(TokenColon)
			if field == "identifier" {
				idTok, _ := p.expect(TokenString)
				appleIdent = idTok.Text
			} else if field == "primary" {
				primTok, _ := p.expect(TokenString)
				primary = primTok.Text
			} else if field == "bindings" {
				p.expect(TokenLBrace)
				for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
					paramNameTok, _ := p.expect(TokenIdent)
					p.expect(TokenColon)
					p.expect(TokenLBrace)
					var b ActionBinding
					for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
						bField := p.advance().Text
						p.expect(TokenColon)
						bVal, _ := p.expect(TokenString)
						if bField == "key" {
							b.WireKey = bVal.Text
						} else if bField == "codec" {
							b.Codec = bVal.Text
						}
						p.match(TokenComma)
					}
					p.expect(TokenRBrace)
					bindings[paramNameTok.Text] = b
					p.match(TokenComma)
				}
				p.expect(TokenRBrace)
			}
		} else {
			p.advance()
		}
	}
	endTok, _ := p.expect(TokenRBrace)

	return &ActionDecl{
		Span:            startTok.Span.Merge(endTok.Span),
		Name:            nameTok.Text,
		Parameters:      params,
		ReturnType:      retType,
		AppleIdentifier: appleIdent,
		PrimaryParam:    primary,
		Bindings:        bindings,
	}
}

// --- Type Annotation Parsing ---

func (p *Parser) parseTypeAnnotation() TypeAnnotation {
	primary := p.parsePrimaryTypeAnnotation()
	if p.match(TokenPipe) {
		members := []TypeAnnotation{primary}
		for {
			members = append(members, p.parsePrimaryTypeAnnotation())
			if !p.match(TokenPipe) {
				break
			}
		}
		return &UnionTypeAnnotation{
			Span:    members[0].NodeSpan().Merge(members[len(members)-1].NodeSpan()),
			Members: members,
		}
	}
	return primary
}

func (p *Parser) parsePrimaryTypeAnnotation() TypeAnnotation {
	if p.curr.Type == TokenLBrace {
		// Record type: { a: Text, b?: Number }
		startTok := p.advance()
		var fields []RecordTypeField
		for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
			fTok, _ := p.expect(TokenIdent)
			opt := p.match(TokenQuestion)
			p.expect(TokenColon)
			tAnno := p.parseTypeAnnotation()
			fields = append(fields, RecordTypeField{
				Name:     fTok.Text,
				Optional: opt,
				TypeExpr: tAnno,
			})
			if !p.match(TokenComma) {
				break
			}
		}
		endTok, _ := p.expect(TokenRBrace)
		return &RecordTypeAnnotation{
			Span:   startTok.Span.Merge(endTok.Span),
			Fields: fields,
		}
	}

	nameTok, ok := p.expect(TokenIdent)
	if !ok {
		return &NamedTypeAnnotation{Span: p.curr.Span, Name: "Unknown"}
	}

	var base TypeAnnotation
	if p.match(TokenLess) {
		// Generic type: List<T> or Map<K, V>
		var typeArgs []TypeAnnotation
		for p.curr.Type != TokenGreater && p.curr.Type != TokenEOF {
			typeArgs = append(typeArgs, p.parseTypeAnnotation())
			if !p.match(TokenComma) {
				break
			}
		}
		endTok, _ := p.expect(TokenGreater)
		base = &GenericTypeAnnotation{
			Span:     nameTok.Span.Merge(endTok.Span),
			Name:     nameTok.Text,
			TypeArgs: typeArgs,
		}
	} else if p.match(TokenDot) {
		memberTok, _ := p.expect(TokenIdent)
		base = &NamedTypeAnnotation{
			Span:      nameTok.Span.Merge(memberTok.Span),
			Namespace: nameTok.Text,
			Name:      memberTok.Text,
		}
	} else {
		base = &NamedTypeAnnotation{
			Span: nameTok.Span,
			Name: nameTok.Text,
		}
	}

	if p.match(TokenQuestion) {
		base = &OptionalTypeAnnotation{
			Span:  base.NodeSpan(),
			Inner: base,
		}
	}
	return base
}

func (p *Parser) parseStatement() Statement {
	switch p.curr.Type {
	case TokenLet, TokenVar:
		b := p.parseBindingStmt()
		if b == nil {
			return nil
		}
		return b
	case TokenReturn:
		r := p.parseReturnStmt()
		if r == nil {
			return nil
		}
		return r
	case TokenYield:
		y := p.parseYieldStmt()
		if y == nil {
			return nil
		}
		return y
	case TokenIf:
		i := p.parseIfStmt()
		if i == nil {
			return nil
		}
		return i
	case TokenFor:
		f := p.parseForStmt()
		if f == nil {
			return nil
		}
		return f
	case TokenRepeat:
		rep := p.parseRepeatStmt()
		if rep == nil {
			return nil
		}
		return rep
	case TokenMenu:
		m := p.parseMenuStmt()
		if m == nil {
			return nil
		}
		return m
	case TokenWhile, TokenBreak, TokenContinue:
		kwTok := p.advance()
		p.errorAt(kwTok.Span, fmt.Sprintf("unsupported keyword '%s': while, break, and continue are reserved unsupported future keywords in Cherri v2.0", kwTok.Text))
		for p.curr.Type != TokenSemicolon && p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
			p.advance()
		}
		p.match(TokenSemicolon)
		return nil
	case TokenLBrace:
		blk := p.parseBlockStmt()
		if blk == nil {
			return nil
		}
		return blk
	case TokenIdent:
		if p.peek.Type == TokenAssign || p.peek.Type == TokenPlusAssign ||
			p.peek.Type == TokenMinusAssign || p.peek.Type == TokenStarAssign || p.peek.Type == TokenSlashAssign {
			a := p.parseAssignStmt()
			if a == nil {
				return nil
			}
			return a
		}
		e := p.parseExprStmt()
		if e == nil {
			return nil
		}
		return e
	default:
		e := p.parseExprStmt()
		if e == nil {
			return nil
		}
		return e
	}
}

func (p *Parser) parseBindingStmt() *BindingStmt {
	startTok := p.advance() // let or var
	isVar := startTok.Type == TokenVar
	nameTok, ok := p.expect(TokenIdent)
	if !ok {
		return nil
	}

	var typeAnno TypeAnnotation
	if p.match(TokenColon) {
		typeAnno = p.parseTypeAnnotation()
	}

	var val Expression
	if p.match(TokenAssign) {
		val = p.parseExpression(precNone)
	}

	p.match(TokenSemicolon)
	span := startTok.Span.Merge(nameTok.Span)
	if val != nil {
		span = startTok.Span.Merge(val.NodeSpan())
	}

	return &BindingStmt{
		Span:     span,
		Mutable:  isVar,
		Name:     nameTok.Text,
		TypeExpr: typeAnno,
		Value:    val,
	}
}

func (p *Parser) parseAssignStmt() *AssignStmt {
	nameTok := p.advance()
	opTok := p.advance() // =, +=, -=, *=, /=
	val := p.parseExpression(precNone)
	p.match(TokenSemicolon)

	return &AssignStmt{
		Span:  nameTok.Span.Merge(val.NodeSpan()),
		Name:  nameTok.Text,
		Op:    opTok.Type,
		Value: val,
	}
}

func (p *Parser) parseReturnStmt() *ReturnStmt {
	startTok := p.advance() // return
	var val Expression
	if p.curr.Type != TokenSemicolon && p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
		val = p.parseExpression(precNone)
	}
	p.match(TokenSemicolon)
	span := startTok.Span
	if val != nil {
		span = startTok.Span.Merge(val.NodeSpan())
	}
	return &ReturnStmt{
		Span:  span,
		Value: val,
	}
}

func (p *Parser) parseYieldStmt() *YieldStmt {
	startTok := p.advance() // yield
	val := p.parseExpression(precNone)
	p.match(TokenSemicolon)
	return &YieldStmt{
		Span:  startTok.Span.Merge(val.NodeSpan()),
		Value: val,
	}
}

func (p *Parser) parseBlockStmt() *BlockStmt {
	startTok, _ := p.expect(TokenLBrace)
	var stmts []Statement
	for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
		if p.curr.Type == TokenSemicolon {
			p.advance()
			continue
		}
		stmt := p.parseStatement()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	endTok, _ := p.expect(TokenRBrace)
	return &BlockStmt{
		Span:       startTok.Span.Merge(endTok.Span),
		Statements: stmts,
	}
}

func (p *Parser) parseIfStmt() *IfStmt {
	startTok := p.advance() // if
	cond := p.parseExpression(precNone)
	thenBlock := p.parseBlockStmt()

	var elseBlock Statement
	if p.match(TokenElse) {
		if p.curr.Type == TokenIf {
			elseBlock = p.parseIfStmt()
		} else {
			elseBlock = p.parseBlockStmt()
		}
	}

	span := startTok.Span.Merge(thenBlock.Span)
	if elseBlock != nil {
		span = startTok.Span.Merge(elseBlock.NodeSpan())
	}

	return &IfStmt{
		Span:      span,
		Condition: cond,
		ThenBlock: thenBlock,
		ElseBlock: elseBlock,
	}
}

func (p *Parser) parseForStmt() *ForStmt {
	startTok := p.advance() // for
	indexVar := ""
	itemVar := ""

	if p.match(TokenLParen) {
		// (index, item)
		idxTok, _ := p.expect(TokenIdent)
		indexVar = idxTok.Text
		p.expect(TokenComma)
		itTok, _ := p.expect(TokenIdent)
		itemVar = itTok.Text
		p.expect(TokenRParen)
	} else {
		itTok, _ := p.expect(TokenIdent)
		itemVar = itTok.Text
	}

	p.expect(TokenIn)
	iter := p.parseExpression(precNone)
	body := p.parseBlockStmt()

	return &ForStmt{
		Span:      startTok.Span.Merge(body.Span),
		IndexVar:  indexVar,
		ItemVar:   itemVar,
		Iterable:  iter,
		BodyBlock: body,
	}
}

func (p *Parser) parseRepeatStmt() *RepeatStmt {
	startTok := p.advance() // repeat
	count := p.parseExpression(precNone)
	idxVar := ""
	if p.match(TokenAs) {
		idxTok, _ := p.expect(TokenIdent)
		idxVar = idxTok.Text
	}
	body := p.parseBlockStmt()

	return &RepeatStmt{
		Span:      startTok.Span.Merge(body.Span),
		Count:     count,
		IndexVar:  idxVar,
		BodyBlock: body,
	}
}

func (p *Parser) parseMenuStmt() *MenuStmt {
	startTok := p.advance() // menu
	p.expect(TokenLParen)
	prompt := p.parseExpression(precNone)
	p.expect(TokenRParen)
	p.expect(TokenLBrace)

	var cases []MenuCase
	for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
		if p.match(TokenCase) {
			labelTok, _ := p.expect(TokenString)
			body := p.parseBlockStmt()
			cases = append(cases, MenuCase{
				Span:      labelTok.Span.Merge(body.Span),
				Label:     labelTok.Text,
				BodyBlock: body,
			})
		} else {
			p.advance()
		}
	}
	endTok, _ := p.expect(TokenRBrace)

	return &MenuStmt{
		Span:   startTok.Span.Merge(endTok.Span),
		Prompt: prompt,
		Cases:  cases,
	}
}

func (p *Parser) parseExprStmt() *ExprStmt {
	expr := p.parseExpression(precNone)
	p.match(TokenSemicolon)
	if expr == nil {
		return nil
	}
	return &ExprStmt{
		Span: expr.NodeSpan(),
		Expr: expr,
	}
}

// --- Pratt Expression Parsing ---

func (p *Parser) parseExpression(minPrec int) Expression {
	left := p.parsePrefix()
	if left == nil {
		return nil
	}

	for minPrec < p.getInfixPrecedence(p.curr.Type) {
		left = p.parseInfix(left)
	}

	return left
}

func (p *Parser) parsePrefix() Expression {
	tok := p.curr

	switch tok.Type {
	case TokenString, TokenRawString, TokenNumber, TokenTrue, TokenFalse, TokenNone, TokenNull:
		p.advance()
		return &LiteralExpr{
			Span:  tok.Span,
			Type:  tok.Type,
			Value: tok.Text,
		}

	case TokenIdent:
		p.advance()
		return &IdentExpr{
			Span: tok.Span,
			Name: tok.Text,
		}

	case TokenDot:
		// Leading dot: .member (enum shorthand)
		p.advance()
		memTok, _ := p.expect(TokenIdent)
		return &EnumMemberExpr{
			Span:   tok.Span.Merge(memTok.Span),
			Member: memTok.Text,
		}

	case TokenFStringStart:
		return p.parseFString()

	case TokenBang, TokenMinus, TokenPlus:
		p.advance()
		operand := p.parseExpression(precUnary)
		return &UnaryExpr{
			Span:    tok.Span.Merge(operand.NodeSpan()),
			Op:      tok.Type,
			Operand: operand,
		}

	case TokenLParen:
		p.advance()
		expr := p.parseExpression(precNone)
		p.expect(TokenRParen)
		return expr

	case TokenLBracket:
		// List literal [a, b, c]
		startTok := p.advance()
		var elements []Expression
		for p.curr.Type != TokenRBracket && p.curr.Type != TokenEOF {
			elem := p.parseExpression(precNone)
			if elem != nil {
				elements = append(elements, elem)
			}
			if !p.match(TokenComma) {
				break
			}
		}
		endTok, _ := p.expect(TokenRBracket)
		return &ListExpr{
			Span:     startTok.Span.Merge(endTok.Span),
			Elements: elements,
		}

	case TokenLBrace:
		// Map literal {"k": v}
		startTok := p.advance()
		var entries []MapEntry
		for p.curr.Type != TokenRBrace && p.curr.Type != TokenEOF {
			key := p.parseExpression(precNone)
			p.expect(TokenColon)
			val := p.parseExpression(precNone)
			entries = append(entries, MapEntry{Key: key, Value: val})
			if !p.match(TokenComma) {
				break
			}
		}
		endTok, _ := p.expect(TokenRBrace)
		return &MapExpr{
			Span:    startTok.Span.Merge(endTok.Span),
			Entries: entries,
		}

	case TokenIf:
		return p.parseIfStmt()

	case TokenRepeat:
		return p.parseRepeatStmt()

	case TokenFor:
		return p.parseForStmt()

	case TokenMenu:
		return p.parseMenuStmt()

	default:
		p.errorAt(tok.Span, fmt.Sprintf("unexpected token in expression: %s", tok.Text))
		p.advance()
		return nil
	}
}

func (p *Parser) parseFString() *FStringExpr {
	startTok := p.advance() // consume f"
	var parts []FStringPart

	for p.curr.Type != TokenEOF {
		if p.curr.Type == TokenFStringEnd {
			endTok := p.advance()
			return &FStringExpr{
				Span:  startTok.Span.Merge(endTok.Span),
				Parts: parts,
			}
		}

		if p.curr.Type == TokenFStringPart {
			tok := p.advance()
			parts = append(parts, FStringPart{
				IsExpr: false,
				Text:   tok.Text,
			})
			continue
		}

		if p.curr.Type == TokenLBrace {
			p.advance() // consume '{'
			expr := p.parseExpression(precNone)
			p.expect(TokenRBrace)
			parts = append(parts, FStringPart{
				IsExpr: true,
				Expr:   expr,
			})
			continue
		}

		if p.curr.Type == TokenError {
			errTok := p.advance()
			p.errorAt(errTok.Span, errTok.Text)
			return &FStringExpr{
				Span:  startTok.Span.Merge(errTok.Span),
				Parts: parts,
			}
		}

		// Fallback for unexpected token
		p.errorAt(p.curr.Span, fmt.Sprintf("unexpected token in f-string: %s", p.curr.Text))
		p.advance()
	}

	p.errorAt(startTok.Span, "unterminated f-string")
	return &FStringExpr{
		Span:  startTok.Span,
		Parts: parts,
	}
}

func (p *Parser) getInfixPrecedence(tt TokenType) int {
	switch tt {
	case TokenOr:
		return precOr
	case TokenAnd:
		return precAnd
	case TokenEqual, TokenNotEqual:
		return precEquality
	case TokenLess, TokenLessEqual, TokenGreater, TokenGreaterEqual:
		return precRelational
	case TokenPlus, TokenMinus:
		return precAdditive
	case TokenStar, TokenSlash, TokenPercent:
		return precMultiplicative
	case TokenDot, TokenLBracket, TokenLParen:
		return precPostfix
	default:
		return precNone
	}
}

func (p *Parser) parseInfix(left Expression) Expression {
	opTok := p.curr

	switch opTok.Type {
	case TokenDot:
		p.advance()
		var propName string
		var propSpan source.Span
		if p.curr.Type == TokenIdent || p.curr.Type == TokenAction {
			propName = p.curr.Text
			propSpan = p.curr.Span
			p.advance()
		} else {
			p.errorAt(p.curr.Span, "expected identifier after '.'")
			return left
		}
		// If left is an identifier that looks like a type, could be EnumType.member
		if id, ok := left.(*IdentExpr); ok && isUpper(id.Name) {
			return &EnumMemberExpr{
				Span:     id.Span.Merge(propSpan),
				TypeName: id.Name,
				Member:   propName,
			}
		}
		return &MemberExpr{
			Span:     left.NodeSpan().Merge(propSpan),
			Target:   left,
			Property: propName,
		}

	case TokenLBracket:
		p.advance()
		indexExpr := p.parseExpression(precNone)
		endTok, _ := p.expect(TokenRBracket)
		return &IndexExpr{
			Span:   left.NodeSpan().Merge(endTok.Span),
			Target: left,
			Index:  indexExpr,
		}

	case TokenLParen:
		return p.parseCallExpr(left)

	default:
		prec := p.getInfixPrecedence(opTok.Type)
		p.advance()
		right := p.parseExpression(prec)
		if right == nil {
			return left
		}
		return &BinaryExpr{
			Span:  left.NodeSpan().Merge(right.NodeSpan()),
			Left:  left,
			Op:    opTok.Type,
			Right: right,
		}
	}
}

func (p *Parser) parseCallExpr(callee Expression) *CallExpr {
	p.advance() // (
	call := &CallExpr{
		Callee: callee,
	}

	// Arguments: can start with unlabeled primary argument, or named argument label: val
	first := true
	for p.curr.Type != TokenRParen && p.curr.Type != TokenEOF {
		if first && p.peek.Type != TokenColon {
			// First argument without colon is primary argument
			primary := p.parseExpression(precNone)
			call.PrimaryArg = primary
			first = false
			if !p.match(TokenComma) {
				break
			}
			continue
		}

		// Named argument: label: val
		labelTok, ok := p.expect(TokenIdent)
		if !ok {
			break
		}
		p.expect(TokenColon)
		val := p.parseExpression(precNone)
		call.NamedArgs = append(call.NamedArgs, NamedArg{
			Span:  labelTok.Span.Merge(val.NodeSpan()),
			Label: labelTok.Text,
			Value: val,
		})
		first = false
		if !p.match(TokenComma) {
			break
		}
	}

	endTok, _ := p.expect(TokenRParen)
	call.Span = callee.NodeSpan().Merge(endTok.Span)
	return call
}

func isUpper(s string) bool {
	if len(s) == 0 {
		return false
	}
	return strings.ToUpper(s[:1]) == s[:1]
}
