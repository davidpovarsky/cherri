package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// CompletionItemKind represents categories of completion suggestions.
type CompletionItemKind int

const (
	CompletionKindFunction CompletionItemKind = iota
	CompletionKindVariable
	CompletionKindParameter
	CompletionKindEnum
	CompletionKindKeyword
	CompletionKindSnippet
)

// CompletionItem represents a completion candidate.
type CompletionItem struct {
	Label         string             `json:"label"`
	Kind          CompletionItemKind `json:"kind"`
	Detail        string             `json:"detail,omitempty"`
	Documentation string             `json:"documentation,omitempty"`
	InsertText    string             `json:"insertText,omitempty"`
}

// HoverResult represents hover information at a position.
type HoverResult struct {
	Contents string      `json:"contents"` // Markdown
	Span     source.Span `json:"span"`
}

// SignatureHelpResult represents parameter help for an active call expression.
type SignatureHelpResult struct {
	Label           string   `json:"label"`
	Documentation   string   `json:"documentation,omitempty"`
	Parameters      []string `json:"parameters"`
	ActiveParameter int      `json:"activeParameter"`
}

// TextEdit represents a range replacement in source text.
type TextEdit struct {
	Span    source.Span `json:"span"`
	NewText string      `json:"newText"`
}

// DocumentState holds in-memory parsed and analyzed state for an open file.
type DocumentState struct {
	URI         string
	Version     int
	File        *source.File
	Program     *syntax.Program
	ParseErrors []syntax.ParseError
	Analyzer    *analysis.Analyzer
}

// Service is the shared language service implementation for CLI, LSP, and iOS bridge.
type Service struct {
	mu        sync.RWMutex
	registry  *schema.Registry
	documents map[string]*DocumentState
}

// NewService creates a new language service instance.
func NewService(registry *schema.Registry) *Service {
	if registry == nil {
		registry = schema.DefaultRegistry()
	}
	return &Service{
		registry:  registry,
		documents: make(map[string]*DocumentState),
	}
}

// OpenDocument opens or creates a document in the service.
// If version > 0 and an existing document has a strictly newer version, the stale edit is ignored.
func (s *Service) OpenDocument(uri string, version int, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.documents[uri]; ok && version > 0 && existing.Version > version {
		return
	}

	file := source.NewFile(source.DocumentID(uri), uri, version, text)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	analyzer := analysis.NewAnalyzer(s.registry)
	analyzer.Analyze(prog)

	s.documents[uri] = &DocumentState{
		URI:         uri,
		Version:     version,
		File:        file,
		Program:     prog,
		ParseErrors: parser.Errors(),
		Analyzer:    analyzer,
	}
}

// ChangeDocument updates content of an open document, ignoring stale lower-version updates.
func (s *Service) ChangeDocument(uri string, version int, text string) {
	s.OpenDocument(uri, version, text)
}

// DocumentVersion returns the current version of an open document, or 0 if not open.
func (s *Service) DocumentVersion(uri string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if doc, ok := s.documents[uri]; ok {
		return doc.Version
	}
	return 0
}

// CloseDocument removes a document from the service.
func (s *Service) CloseDocument(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.documents, uri)
}

// Analyze returns all parse and semantic diagnostics for a document.
func (s *Service) Analyze(uri string) ([]analysis.Diagnostic, string) {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	if !ok {
		return nil, s.registry.Fingerprint()
	}

	var diags []analysis.Diagnostic

	// 1. Add parse errors
	for _, pe := range doc.ParseErrors {
		code := analysis.CodeSyntax
		if strings.Contains(pe.Message, analysis.CodeLegacySyntax) {
			code = analysis.CodeLegacySyntax
		}
		diags = append(diags, analysis.Diagnostic{
			Code:     code,
			Severity: analysis.SeverityError,
			Span:     pe.Span,
			Message:  pe.Message,
		})
	}

	// 2. Add semantic diagnostics
	if doc.Analyzer != nil {
		diags = append(diags, doc.Analyzer.Diagnostics()...)
	}

	return diags, s.registry.Fingerprint()
}

// SchemaFingerprint returns the fingerprint of the underlying schema registry.
func (s *Service) SchemaFingerprint() string {
	return s.registry.Fingerprint()
}

// Format formats the document text using canonical Cherri v2 formatting.
func (s *Service) Format(uri string) (string, error) {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("document not found: %s", uri)
	}

	return syntax.Format(doc.Program), nil
}

var labelRegex = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*:`)

func completionKindPriority(kind CompletionItemKind) int {
	switch kind {
	case CompletionKindParameter:
		return 0
	case CompletionKindEnum:
		return 1
	case CompletionKindVariable:
		return 2
	case CompletionKindFunction:
		return 3
	case CompletionKindKeyword:
		return 4
	default:
		return 5
	}
}

func sliceLineRunesUpToCol(lineText string, col int) string {
	runes := []rune(lineText)
	if col-1 >= 0 && col-1 <= len(runes) {
		return string(runes[:col-1])
	}
	return lineText
}

// Complete returns contextual completions at the given 1-based line and column.
func (s *Service) Complete(uri string, line, col int) []CompletionItem {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	var items []CompletionItem
	if !ok {
		return items
	}

	lines := strings.Split(doc.File.Content, "\n")
	var linePrefix string
	if line-1 >= 0 && line-1 < len(lines) {
		linePrefix = sliceLineRunesUpToCol(lines[line-1], col)
	}

	// Determine if inside an open call
	prefixRunes := []rune(linePrefix)
	openParen := -1
	parenDepth := 0
	for i := len(prefixRunes) - 1; i >= 0; i-- {
		ch := prefixRunes[i]
		if ch == ')' {
			parenDepth++
		} else if ch == '(' {
			if parenDepth > 0 {
				parenDepth--
			} else {
				openParen = i
				break
			}
		}
	}

	var callIdent string
	usedLabels := make(map[string]bool)
	trimmedPrefix := strings.TrimSpace(linePrefix)
	isDot := strings.HasSuffix(trimmedPrefix, ".")

	if openParen >= 0 {
		// Find call identifier right before '('
		identEnd := openParen
		for identEnd > 0 && (prefixRunes[identEnd-1] == ' ' || prefixRunes[identEnd-1] == '\t') {
			identEnd--
		}
		identStart := identEnd
		for identStart > 0 && isIdentChar(prefixRunes[identStart-1]) {
			identStart--
		}
		callIdent = string(prefixRunes[identStart:identEnd])

		// Extract used labels inside args so far
		argsText := string(prefixRunes[openParen+1:])
		matches := labelRegex.FindAllStringSubmatch(argsText, -1)
		for _, m := range matches {
			if len(m) > 1 {
				usedLabels[m[1]] = true
			}
		}
	}

	// 1. Contextual parameter labels and enum values for the active call
	if callIdent != "" {
		if act, ok := s.registry.LookupAction(callIdent); ok {
			for _, p := range act.Parameters {
				if p.Label != "" && !usedLabels[p.Label] {
					items = append(items, CompletionItem{
						Label:         p.Label,
						Kind:          CompletionKindParameter,
						Detail:        fmt.Sprintf("%s: %s (parameter)", p.Label, p.TypeName),
						Documentation: p.DisplayName,
						InsertText:    p.Label + ": ",
					})
				}
				for _, ev := range p.EnumValues {
					items = append(items, CompletionItem{
						Label:         "." + ev,
						Kind:          CompletionKindEnum,
						Detail:        fmt.Sprintf("enum %s (%s)", p.EnumName, p.Label),
						Documentation: p.DisplayName,
						InsertText:    "." + ev,
					})
				}
			}
		}
		if doc.Program != nil {
			for _, decl := range doc.Program.Declarations {
				if fn, ok := decl.(*syntax.FunctionDecl); ok && fn.Name == callIdent {
					for _, p := range fn.Parameters {
						if p.Name != "" && !usedLabels[p.Name] {
							typeStr := "parameter"
							if nt, ok := p.TypeExpr.(*syntax.NamedTypeAnnotation); ok {
								typeStr = nt.Name
							}
							items = append(items, CompletionItem{
								Label:      p.Name,
								Kind:       CompletionKindParameter,
								Detail:     fmt.Sprintf("%s: %s (parameter)", p.Name, typeStr),
								InsertText: p.Name + ": ",
							})
						}
					}
				}
			}
		}
	}

	// 2. If dot prefix, offer general enum members
	if isDot {
		for enumName, vals := range s.registry.AllEnums() {
			for _, v := range vals {
				items = append(items, CompletionItem{
					Label:      "." + v,
					Kind:       CompletionKindEnum,
					Detail:     "enum " + enumName,
					InsertText: "." + v,
				})
			}
		}
	}

	// 3. Local variable & function completions
	if doc.Program != nil {
		for _, decl := range doc.Program.Declarations {
			if fn, ok := decl.(*syntax.FunctionDecl); ok {
				items = append(items, CompletionItem{
					Label:  fn.Name,
					Kind:   CompletionKindFunction,
					Detail: fmt.Sprintf("function %s(...)", fn.Name),
				})
			}
		}
		for _, stmt := range doc.Program.Statements {
			if b, ok := stmt.(*syntax.BindingStmt); ok {
				items = append(items, CompletionItem{
					Label:  b.Name,
					Kind:   CompletionKindVariable,
					Detail: "variable",
				})
			}
		}
	}

	// 4. Action completions
	for _, a := range s.registry.AllActions() {
		items = append(items, CompletionItem{
			Label:         a.CallableName,
			Kind:          CompletionKindFunction,
			Detail:        fmt.Sprintf("action %s(...) -> %s", a.CallableName, a.OutputTypeName),
			Documentation: a.Docs.Description,
			InsertText:    a.Docs.InsertionSnippet,
		})
	}

	// 5. Keywords
	keywords := []string{"let", "var", "function", "return", "yield", "if", "else", "for", "repeat", "menu", "import", "shortcut"}
	for _, kw := range keywords {
		items = append(items, CompletionItem{
			Label: kw,
			Kind:  CompletionKindKeyword,
		})
	}

	// Deduplicate items preserving priority order
	seen := make(map[string]bool)
	var deduped []CompletionItem
	for _, it := range items {
		key := fmt.Sprintf("%s:%d", it.Label, it.Kind)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, it)
		}
	}

	// Sort items: contextual Parameters and Enums first, then Variables, Functions, Keywords, Actions.
	// Within the same kind, sort by Label.
	sort.SliceStable(deduped, func(i, j int) bool {
		ki := completionKindPriority(deduped[i].Kind)
		kj := completionKindPriority(deduped[j].Kind)
		if ki != kj {
			return ki < kj
		}
		return deduped[i].Label < deduped[j].Label
	})

	return deduped
}

func extractWordAtRuneCol(lineText string, col int) (word string, startRune int, endRune int, ok bool) {
	runes := []rune(lineText)
	if len(runes) == 0 {
		return "", 0, 0, false
	}
	colIdx := col - 1
	if colIdx < 0 {
		colIdx = 0
	}
	if colIdx >= len(runes) {
		colIdx = len(runes) - 1
	}
	if !isIdentChar(runes[colIdx]) && colIdx > 0 && isIdentChar(runes[colIdx-1]) {
		colIdx--
	}
	if !isIdentChar(runes[colIdx]) {
		return "", 0, 0, false
	}
	start := colIdx
	for start > 0 && isIdentChar(runes[start-1]) {
		start--
	}
	end := colIdx
	for end < len(runes) && isIdentChar(runes[end]) {
		end++
	}
	if start >= end {
		return "", 0, 0, false
	}
	return string(runes[start:end]), start, end, true
}

// Hover returns hover information at 1-based line and column.
func (s *Service) Hover(uri string, line, col int) *HoverResult {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	lines := strings.Split(doc.File.Content, "\n")
	if line-1 < 0 || line-1 >= len(lines) {
		return nil
	}
	lineText := lines[line-1]

	word, start, end, foundWord := extractWordAtRuneCol(lineText, col)
	if !foundWord {
		return nil
	}

	if a, found := s.registry.LookupAction(word); found {
		var md strings.Builder
		fmt.Fprintf(&md, "### %s\n\n", a.CallableName)
		fmt.Fprintf(&md, "**Action:** `%s`\n\n", a.AppleIdentifier)
		if a.Docs.Description != "" {
			fmt.Fprintf(&md, "%s\n\n", a.Docs.Description)
		}
		md.WriteString("**Parameters:**\n")
		for _, p := range a.Parameters {
			opt := ""
			if p.Optional {
				opt = " (optional)"
			}
			fmt.Fprintf(&md, "- `%s`: `%s`%s\n", p.Label, p.TypeName, opt)
		}
		if a.OutputTypeName != "" && a.OutputTypeName != "Unknown" {
			fmt.Fprintf(&md, "\n**Returns:** `%s`\n", a.OutputTypeName)
		}

		span := source.Span{
			Document: source.DocumentID(uri),
			Start:    source.Position{Line: line, Column: start + 1, UTF16Column: start + 1},
			End:      source.Position{Line: line, Column: end + 1, UTF16Column: end + 1},
		}

		return &HoverResult{
			Contents: md.String(),
			Span:     span,
		}
	}

	if doc.Program != nil {
		for _, decl := range doc.Program.Declarations {
			if fn, ok := decl.(*syntax.FunctionDecl); ok && fn.Name == word {
				var params []string
				for _, p := range fn.Parameters {
					params = append(params, p.Name)
				}
				span := source.Span{
					Document: source.DocumentID(uri),
					Start:    source.Position{Line: line, Column: start + 1, UTF16Column: start + 1},
					End:      source.Position{Line: line, Column: end + 1, UTF16Column: end + 1},
				}
				return &HoverResult{
					Contents: fmt.Sprintf("```cherri\nfunction %s(%s)\n```", fn.Name, strings.Join(params, ", ")),
					Span:     span,
				}
			}
		}
		for _, stmt := range doc.Program.Statements {
			if b, ok := stmt.(*syntax.BindingStmt); ok && b.Name == word {
				span := source.Span{
					Document: source.DocumentID(uri),
					Start:    source.Position{Line: line, Column: start + 1, UTF16Column: start + 1},
					End:      source.Position{Line: line, Column: end + 1, UTF16Column: end + 1},
				}
				return &HoverResult{
					Contents: fmt.Sprintf("```cherri\n(variable) %s\n```", b.Name),
					Span:     span,
				}
			}
		}
	}

	return nil
}

// SignatureHelp returns active signature and parameter information at 1-based line and column.
func (s *Service) SignatureHelp(uri string, line, col int) *SignatureHelpResult {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	lines := strings.Split(doc.File.Content, "\n")
	if line-1 < 0 || line-1 >= len(lines) {
		return nil
	}
	linePrefix := sliceLineRunesUpToCol(lines[line-1], col)
	prefixRunes := []rune(linePrefix)

	openParen := -1
	parenDepth := 0
	for i := len(prefixRunes) - 1; i >= 0; i-- {
		ch := prefixRunes[i]
		if ch == ')' {
			parenDepth++
		} else if ch == '(' {
			if parenDepth > 0 {
				parenDepth--
			} else {
				openParen = i
				break
			}
		}
	}
	if openParen < 0 {
		return nil
	}

	identEnd := openParen
	for identEnd > 0 && (prefixRunes[identEnd-1] == ' ' || prefixRunes[identEnd-1] == '\t') {
		identEnd--
	}
	identStart := identEnd
	for identStart > 0 && isIdentChar(prefixRunes[identStart-1]) {
		identStart--
	}
	callIdent := string(prefixRunes[identStart:identEnd])
	if callIdent == "" {
		return nil
	}

	argsText := string(prefixRunes[openParen+1:])
	activeParam := strings.Count(argsText, ",")

	if act, found := s.registry.LookupAction(callIdent); found {
		params := make([]string, len(act.Parameters))
		for i, p := range act.Parameters {
			opt := ""
			if p.Optional {
				opt = "?"
			}
			params[i] = fmt.Sprintf("%s%s: %s", p.Label, opt, p.TypeName)
		}
		if activeParam >= len(params) && len(params) > 0 {
			activeParam = len(params) - 1
		}
		ret := "Void"
		if act.OutputTypeName != "" {
			ret = act.OutputTypeName
		}
		return &SignatureHelpResult{
			Label:           fmt.Sprintf("%s(%s) -> %s", act.CallableName, strings.Join(params, ", "), ret),
			Documentation:   act.Docs.Description,
			Parameters:      params,
			ActiveParameter: activeParam,
		}
	}

	if doc.Program != nil {
		for _, decl := range doc.Program.Declarations {
			if fn, ok := decl.(*syntax.FunctionDecl); ok && fn.Name == callIdent {
				params := make([]string, len(fn.Parameters))
				for i, p := range fn.Parameters {
					params[i] = p.Name
				}
				if activeParam >= len(params) && len(params) > 0 {
					activeParam = len(params) - 1
				}
				return &SignatureHelpResult{
					Label:           fmt.Sprintf("function %s(%s)", fn.Name, strings.Join(params, ", ")),
					Parameters:      params,
					ActiveParameter: activeParam,
				}
			}
		}
	}

	return nil
}

type scopedDeclaration struct {
	name      string
	declSpan  source.Span // exact span of the identifier at declaration site
	scopeSpan source.Span // enclosing lexical scope span
	isFunc    bool
}

type identRef struct {
	name string
	span source.Span
}

func collectScopesAndRefs(file *source.File, prog *syntax.Program) ([]scopedDeclaration, []identRef) {
	if file == nil || prog == nil {
		return nil, nil
	}

	// Tokenize file to locate exact identifier token spans for declaration sites
	lexer := syntax.NewLexer(file)
	var identTokens []syntax.Token
	for {
		tok := lexer.NextToken()
		if tok.Type == syntax.TokenEOF {
			break
		}
		if tok.Type == syntax.TokenIdent {
			identTokens = append(identTokens, tok)
		}
	}

	findIdentSpanAfter := func(offset int, name string) source.Span {
		for _, tok := range identTokens {
			if tok.Span.Start.ByteOffset >= offset && tok.Text == name {
				return tok.Span
			}
		}
		return file.SpanForOffsets(offset, offset+len(name))
	}

	progScope := prog.Span
	if progScope.End.ByteOffset < len(file.Content) {
		progScope = file.SpanForOffsets(0, len(file.Content))
	}

	var decls []scopedDeclaration
	var refs []identRef

	var walkExpr func(expr syntax.Expression)
	var walkStmt func(stmt syntax.Statement, currentScope source.Span)

	walkExpr = func(expr syntax.Expression) {
		if expr == nil {
			return
		}
		switch e := expr.(type) {
		case *syntax.IdentExpr:
			refs = append(refs, identRef{name: e.Name, span: e.Span})
		case *syntax.FStringExpr:
			for _, part := range e.Parts {
				if part.IsExpr {
					walkExpr(part.Expr)
				}
			}
		case *syntax.UnaryExpr:
			walkExpr(e.Operand)
		case *syntax.BinaryExpr:
			walkExpr(e.Left)
			walkExpr(e.Right)
		case *syntax.CallExpr:
			walkExpr(e.Callee)
			if e.PrimaryArg != nil {
				walkExpr(e.PrimaryArg)
			}
			for _, arg := range e.NamedArgs {
				walkExpr(arg.Value)
			}
		case *syntax.MemberExpr:
			walkExpr(e.Target)
		case *syntax.IndexExpr:
			walkExpr(e.Target)
			walkExpr(e.Index)
		case *syntax.ListExpr:
			for _, el := range e.Elements {
				walkExpr(el)
			}
		case *syntax.MapExpr:
			for _, entry := range e.Entries {
				walkExpr(entry.Key)
				walkExpr(entry.Value)
			}
		case *syntax.RecordExpr:
			for _, f := range e.Fields {
				walkExpr(f.Value)
			}
		case *syntax.IfStmt:
			walkStmt(e, progScope)
		case *syntax.RepeatStmt:
			walkStmt(e, progScope)
		case *syntax.ForStmt:
			walkStmt(e, progScope)
		case *syntax.MenuStmt:
			walkStmt(e, progScope)
		}
	}

	walkStmt = func(stmt syntax.Statement, currentScope source.Span) {
		if stmt == nil {
			return
		}
		switch s := stmt.(type) {
		case *syntax.BindingStmt:
			walkExpr(s.Value)
			declSpan := findIdentSpanAfter(s.Span.Start.ByteOffset, s.Name)
			decls = append(decls, scopedDeclaration{
				name:      s.Name,
				declSpan:  declSpan,
				scopeSpan: file.SpanForOffsets(declSpan.Start.ByteOffset, currentScope.End.ByteOffset),
			})
			refs = append(refs, identRef{name: s.Name, span: declSpan})
		case *syntax.AssignStmt:
			targetSpan := findIdentSpanAfter(s.Span.Start.ByteOffset, s.Name)
			refs = append(refs, identRef{name: s.Name, span: targetSpan})
			walkExpr(s.Value)
		case *syntax.ExprStmt:
			walkExpr(s.Expr)
		case *syntax.ReturnStmt:
			walkExpr(s.Value)
		case *syntax.YieldStmt:
			walkExpr(s.Value)
		case *syntax.BlockStmt:
			for _, child := range s.Statements {
				walkStmt(child, s.Span)
			}
		case *syntax.IfStmt:
			walkExpr(s.Condition)
			if s.ThenBlock != nil {
				walkStmt(s.ThenBlock, s.ThenBlock.Span)
			}
			if s.ElseBlock != nil {
				walkStmt(s.ElseBlock, s.ElseBlock.NodeSpan())
			}
		case *syntax.ForStmt:
			walkExpr(s.Iterable)
			if s.BodyBlock != nil {
				if s.IndexVar != "" {
					idxSpan := findIdentSpanAfter(s.Span.Start.ByteOffset, s.IndexVar)
					decls = append(decls, scopedDeclaration{
						name:      s.IndexVar,
						declSpan:  idxSpan,
						scopeSpan: s.BodyBlock.Span,
					})
					refs = append(refs, identRef{name: s.IndexVar, span: idxSpan})
				}
				if s.ItemVar != "" {
					itSpan := findIdentSpanAfter(s.Span.Start.ByteOffset, s.ItemVar)
					decls = append(decls, scopedDeclaration{
						name:      s.ItemVar,
						declSpan:  itSpan,
						scopeSpan: s.BodyBlock.Span,
					})
					refs = append(refs, identRef{name: s.ItemVar, span: itSpan})
				}
				walkStmt(s.BodyBlock, s.BodyBlock.Span)
			}
		case *syntax.RepeatStmt:
			walkExpr(s.Count)
			if s.BodyBlock != nil {
				if s.IndexVar != "" {
					idxSpan := findIdentSpanAfter(s.Span.Start.ByteOffset, s.IndexVar)
					decls = append(decls, scopedDeclaration{
						name:      s.IndexVar,
						declSpan:  idxSpan,
						scopeSpan: s.BodyBlock.Span,
					})
					refs = append(refs, identRef{name: s.IndexVar, span: idxSpan})
				}
				walkStmt(s.BodyBlock, s.BodyBlock.Span)
			}
		case *syntax.MenuStmt:
			walkExpr(s.Prompt)
			for _, c := range s.Cases {
				if c.BodyBlock != nil {
					walkStmt(c.BodyBlock, c.BodyBlock.Span)
				}
			}
		}
	}

	for _, decl := range prog.Declarations {
		switch d := decl.(type) {
		case *syntax.FunctionDecl:
			fnSpan := findIdentSpanAfter(d.Span.Start.ByteOffset, d.Name)
			decls = append(decls, scopedDeclaration{
				name:      d.Name,
				declSpan:  fnSpan,
				scopeSpan: progScope,
				isFunc:    true,
			})
			refs = append(refs, identRef{name: d.Name, span: fnSpan})
			bodyScope := d.Span
			if d.Body != nil {
				bodyScope = d.Body.Span
			}
			for _, p := range d.Parameters {
				decls = append(decls, scopedDeclaration{
					name:      p.Name,
					declSpan:  p.Span,
					scopeSpan: bodyScope,
				})
				refs = append(refs, identRef{name: p.Name, span: p.Span})
				if p.DefaultExpr != nil {
					walkExpr(p.DefaultExpr)
				}
			}
			if d.Body != nil {
				walkStmt(d.Body, d.Body.Span)
			}
		case *syntax.SetupDecl:
			sdSpan := findIdentSpanAfter(d.Span.Start.ByteOffset, d.Name)
			decls = append(decls, scopedDeclaration{
				name:      d.Name,
				declSpan:  sdSpan,
				scopeSpan: progScope,
			})
			refs = append(refs, identRef{name: d.Name, span: sdSpan})
		}
	}

	for _, stmt := range prog.Statements {
		walkStmt(stmt, progScope)
	}

	return decls, refs
}

func resolveDeclForRef(decls []scopedDeclaration, name string, offset int) *scopedDeclaration {
	var best *scopedDeclaration
	bestScopeLen := int(^uint(0) >> 1)
	for i := range decls {
		d := &decls[i]
		if d.name != name {
			continue
		}
		if offset >= d.scopeSpan.Start.ByteOffset && offset <= d.scopeSpan.End.ByteOffset {
			scopeLen := d.scopeSpan.End.ByteOffset - d.scopeSpan.Start.ByteOffset
			if scopeLen <= bestScopeLen {
				best = d
				bestScopeLen = scopeLen
			}
		}
	}
	return best
}

func byteOffsetAtLineCol(file *source.File, line, col int) int {
	if file == nil {
		return 0
	}
	lines := strings.Split(file.Content, "\n")
	if line-1 < 0 || line-1 >= len(lines) {
		return 0
	}
	byteOff := 0
	for i := 0; i < line-1; i++ {
		byteOff += len(lines[i]) + 1
	}
	runes := []rune(lines[line-1])
	runeIdx := col - 1
	if runeIdx < 0 {
		runeIdx = 0
	}
	if runeIdx > len(runes) {
		runeIdx = len(runes)
	}
	byteOff += len(string(runes[:runeIdx]))
	return byteOff
}

// Definition returns the declaration span of the symbol at 1-based (line, col).
func (s *Service) Definition(uri string, line, col int) *source.Span {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	if !ok || doc.File == nil || doc.Program == nil {
		return nil
	}

	lines := strings.Split(doc.File.Content, "\n")
	if line-1 < 0 || line-1 >= len(lines) {
		return nil
	}
	word, startRune, _, found := extractWordAtRuneCol(lines[line-1], col)
	if !found {
		return nil
	}

	targetOffset := byteOffsetAtLineCol(doc.File, line, startRune+1)
	decls, _ := collectScopesAndRefs(doc.File, doc.Program)
	resolved := resolveDeclForRef(decls, word, targetOffset)
	if resolved == nil {
		return nil
	}
	span := resolved.declSpan
	return &span
}

func isValidNewIdent(name string) bool {
	if name == "" {
		return false
	}
	reserved := map[string]bool{
		"let": true, "var": true, "function": true, "return": true, "yield": true,
		"if": true, "else": true, "for": true, "in": true, "repeat": true, "as": true,
		"menu": true, "case": true, "import": true, "shortcut": true, "trigger": true,
		"setup": true, "type": true, "enum": true, "action": true, "true": true,
		"false": true, "none": true, "null": true, "while": true, "break": true, "continue": true,
	}
	if reserved[name] {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else if !isIdentChar(r) {
			return false
		}
	}
	return true
}

// Rename performs a token- and scope-aware rename of the symbol at 1-based (line, col).
// It ignores string literals, comments, and shadowed inner bindings.
func (s *Service) Rename(uri string, line, col int, newName string) (string, []TextEdit, error) {
	if !isValidNewIdent(newName) {
		return "", nil, fmt.Errorf("invalid identifier %q for rename", newName)
	}

	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	if !ok || doc.File == nil || doc.Program == nil {
		return "", nil, fmt.Errorf("document not found: %s", uri)
	}

	lines := strings.Split(doc.File.Content, "\n")
	if line-1 < 0 || line-1 >= len(lines) {
		return "", nil, fmt.Errorf("position %d:%d out of range", line, col)
	}
	word, startRune, _, found := extractWordAtRuneCol(lines[line-1], col)
	if !found {
		return "", nil, fmt.Errorf("no identifier at %d:%d", line, col)
	}

	targetOffset := byteOffsetAtLineCol(doc.File, line, startRune+1)
	decls, refs := collectScopesAndRefs(doc.File, doc.Program)
	targetDecl := resolveDeclForRef(decls, word, targetOffset)
	if targetDecl == nil {
		return "", nil, fmt.Errorf("symbol %q is not a renameable local or function declaration", word)
	}

	seenOffsets := make(map[int]bool)
	var edits []TextEdit
	for _, ref := range refs {
		if ref.name != word {
			continue
		}
		if seenOffsets[ref.span.Start.ByteOffset] {
			continue
		}
		resolved := resolveDeclForRef(decls, ref.name, ref.span.Start.ByteOffset)
		if resolved != nil && resolved.declSpan.Start.ByteOffset == targetDecl.declSpan.Start.ByteOffset {
			seenOffsets[ref.span.Start.ByteOffset] = true
			edits = append(edits, TextEdit{
				Span:    ref.span,
				NewText: newName,
			})
		}
	}

	if len(edits) == 0 {
		return "", nil, fmt.Errorf("no occurrences found for symbol %q", word)
	}

	// Sort edits in ascending order of offset for deterministic reporting, then apply in reverse order
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].Span.Start.ByteOffset < edits[j].Span.Start.ByteOffset
	})

	updated := doc.File.Content
	for i := len(edits) - 1; i >= 0; i-- {
		ed := edits[i]
		if ed.Span.Start.ByteOffset >= 0 && ed.Span.End.ByteOffset <= len(updated) && ed.Span.Start.ByteOffset <= ed.Span.End.ByteOffset {
			updated = updated[:ed.Span.Start.ByteOffset] + ed.NewText + updated[ed.Span.End.ByteOffset:]
		}
	}

	return updated, edits, nil
}

// ApplyVisualSourceEdit applies a bounded AST source splice for a user-written action call at actionIndex.
// It preserves surrounding comments and formatting, and rejects out-of-range or generated/unmappable actions.
func (s *Service) ApplyVisualSourceEdit(src string, actionIndex int, paramLabel string, newExpr string) (string, error) {
	if actionIndex < 0 {
		return src, fmt.Errorf("action index %d cannot be negative", actionIndex)
	}
	trimmedExpr := strings.TrimSpace(newExpr)
	if trimmedExpr == "" {
		return src, fmt.Errorf("replacement expression cannot be empty")
	}

	// Validate replacement expression syntax
	exprFile := source.NewFile("edit-expr", "edit-expr", 1, trimmedExpr)
	exprParser := syntax.NewParser(exprFile)
	exprProg := exprParser.ParseProgram()
	if len(exprParser.Errors()) > 0 || len(exprProg.Statements) != 1 {
		return src, fmt.Errorf("invalid replacement expression %q", newExpr)
	}
	if _, ok := exprProg.Statements[0].(*syntax.ExprStmt); !ok {
		return src, fmt.Errorf("replacement %q is not a valid expression", newExpr)
	}

	file := source.NewFile("visual-edit", "visual-edit", 1, src)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		return src, fmt.Errorf("cannot apply visual edit to source with syntax errors")
	}

	if actionIndex >= len(prog.Statements) {
		return src, fmt.Errorf("action index %d is out of range or refers to a generated action (top-level statements: %d)", actionIndex, len(prog.Statements))
	}

	var call *syntax.CallExpr
	switch stmt := prog.Statements[actionIndex].(type) {
	case *syntax.ExprStmt:
		if c, ok := stmt.Expr.(*syntax.CallExpr); ok {
			call = c
		}
	case *syntax.BindingStmt:
		if c, ok := stmt.Value.(*syntax.CallExpr); ok {
			call = c
		}
	}
	if call == nil {
		return src, fmt.Errorf("statement at index %d is not a direct action call", actionIndex)
	}

	var calleeName string
	if ident, ok := call.Callee.(*syntax.IdentExpr); ok {
		calleeName = ident.Name
	} else if mem, ok := call.Callee.(*syntax.MemberExpr); ok {
		calleeName = mem.Property
	}
	if calleeName == "" || calleeName == "action" || calleeName == "rawAction" {
		return src, fmt.Errorf("action call at index %d is a raw/unmappable call", actionIndex)
	}

	actSchema, hasSchema := s.registry.LookupAction(calleeName)
	canonicalLabel := paramLabel
	isPrimaryTarget := paramLabel == "" || paramLabel == "primary"

	if hasSchema {
		if prim, hasPrim := actSchema.PrimaryParameter(); hasPrim {
			if strings.EqualFold(paramLabel, prim.Label) || strings.EqualFold(paramLabel, prim.WireKey) {
				isPrimaryTarget = true
				canonicalLabel = prim.Label
			}
		}
		if !isPrimaryTarget {
			matched := false
			for i, p := range actSchema.Parameters {
				if strings.EqualFold(p.Label, paramLabel) || strings.EqualFold(p.WireKey, paramLabel) {
					canonicalLabel = p.Label
					if i == 0 && call.PrimaryArg != nil {
						isPrimaryTarget = true
					}
					matched = true
					break
				}
			}
			if !matched {
				return src, fmt.Errorf("parameter %q does not belong to action %s", paramLabel, calleeName)
			}
		}
	}

	// 1. Check if editing existing PrimaryArg
	if isPrimaryTarget && call.PrimaryArg != nil {
		sp := call.PrimaryArg.NodeSpan()
		return src[:sp.Start.ByteOffset] + trimmedExpr + src[sp.End.ByteOffset:], nil
	}

	// 2. Check if editing an existing NamedArg
	for _, arg := range call.NamedArgs {
		if arg.Label == canonicalLabel || (paramLabel != "" && arg.Label == paramLabel) {
			sp := arg.Value.NodeSpan()
			return src[:sp.Start.ByteOffset] + trimmedExpr + src[sp.End.ByteOffset:], nil
		}
	}

	// 3. Insert new argument before closing ')' of CallExpr
	closeParenOffset := call.Span.End.ByteOffset - 1
	if closeParenOffset < 0 || closeParenOffset >= len(src) || src[closeParenOffset] != ')' {
		return src, fmt.Errorf("unable to locate closing parenthesis for call %s", calleeName)
	}

	hasExistingArgs := call.PrimaryArg != nil || len(call.NamedArgs) > 0
	var insertion string
	if isPrimaryTarget && !hasExistingArgs {
		insertion = trimmedExpr
	} else if hasExistingArgs {
		insertion = fmt.Sprintf(", %s: %s", canonicalLabel, trimmedExpr)
	} else {
		insertion = fmt.Sprintf("%s: %s", canonicalLabel, trimmedExpr)
	}

	return src[:closeParenOffset] + insertion + src[closeParenOffset:], nil
}

func isIdentChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

