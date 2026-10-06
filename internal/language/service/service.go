package service

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

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
func (s *Service) OpenDocument(uri string, version int, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

// ChangeDocument updates content of an open document.
func (s *Service) ChangeDocument(uri string, version int, text string) {
	s.OpenDocument(uri, version, text)
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
		currentLine := lines[line-1]
		if col-1 >= 0 && col-1 <= len(currentLine) {
			linePrefix = currentLine[:col-1]
		} else {
			linePrefix = currentLine
		}
	}

	// Determine if inside an open call
	openParen := -1
	parenDepth := 0
	for i := len(linePrefix) - 1; i >= 0; i-- {
		ch := linePrefix[i]
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
		for identEnd > 0 && (linePrefix[identEnd-1] == ' ' || linePrefix[identEnd-1] == '\t') {
			identEnd--
		}
		identStart := identEnd
		for identStart > 0 && isIdentChar(rune(linePrefix[identStart-1])) {
			identStart--
		}
		callIdent = linePrefix[identStart:identEnd]

		// Extract used labels inside args so far
		argsText := linePrefix[openParen+1:]
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

// Hover returns hover information at 1-based line and column.
func (s *Service) Hover(uri string, line, col int) *HoverResult {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	lines := strings.Split(doc.File.Content, "\n")
	if line-1 >= len(lines) {
		return nil
	}
	lineText := lines[line-1]

	// Find word around col
	colIdx := col - 1
	if colIdx < 0 {
		colIdx = 0
	}
	if colIdx >= len(lineText) {
		colIdx = len(lineText) - 1
	}

	start := colIdx
	for start > 0 && isIdentChar(rune(lineText[start-1])) {
		start--
	}
	end := colIdx
	for end < len(lineText) && isIdentChar(rune(lineText[end])) {
		end++
	}

	if start >= end {
		return nil
	}

	word := lineText[start:end]

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
			Start:    source.Position{Line: line, Column: start + 1},
			End:      source.Position{Line: line, Column: end + 1},
		}

		return &HoverResult{
			Contents: md.String(),
			Span:     span,
		}
	}

	if doc.Program != nil {
		for _, stmt := range doc.Program.Statements {
			if b, ok := stmt.(*syntax.BindingStmt); ok && b.Name == word {
				span := source.Span{
					Document: source.DocumentID(uri),
					Start:    source.Position{Line: line, Column: start + 1},
					End:      source.Position{Line: line, Column: end + 1},
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

func isIdentChar(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}
