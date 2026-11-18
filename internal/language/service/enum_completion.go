package service

import (
	"strconv"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/enums"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// enumCompletionContext tokenizes up to the cursor, so nested calls, comments,
// quoted commas and multiline arguments cannot select an unrelated parameter.
type enumCallContext struct {
	name      string
	arguments [][]syntax.Token
}

func enumCompletionTokens(file *source.File, offset int) ([]syntax.Token, enumCallContext) {
	lexer := syntax.NewLexer(source.NewFile(file.ID, file.URI, file.Version, file.Content[:offset]))
	var tokens []syntax.Token
	var stack []int
	for {
		token := lexer.NextToken()
		if token.Type == syntax.TokenEOF {
			break
		}
		tokens = append(tokens, token)
		switch token.Type {
		case syntax.TokenLParen, syntax.TokenLBracket, syntax.TokenLBrace:
			stack = append(stack, len(tokens)-1)
		case syntax.TokenRParen, syntax.TokenRBracket, syntax.TokenRBrace:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) == 0 {
		return tokens, enumCallContext{}
	}
	open := stack[len(stack)-1]
	if tokens[open].Type != syntax.TokenLParen || open == 0 || tokens[open-1].Type != syntax.TokenIdent {
		return tokens, enumCallContext{}
	}
	context := enumCallContext{name: tokens[open-1].Text}
	start, depth := open+1, 0
	for index := start; index < len(tokens); index++ {
		switch tokens[index].Type {
		case syntax.TokenLParen, syntax.TokenLBracket, syntax.TokenLBrace:
			depth++
		case syntax.TokenRParen, syntax.TokenRBracket, syntax.TokenRBrace:
			depth--
		case syntax.TokenComma:
			if depth == 0 {
				context.arguments = append(context.arguments, tokens[start:index])
				start = index + 1
			}
		}
	}
	context.arguments = append(context.arguments, tokens[start:])
	return tokens, context
}

type enumCompletionParameter struct{ name, label, domain string }

func enumArgumentLabel(tokens []syntax.Token) (string, []syntax.Token) {
	if len(tokens) >= 2 && tokens[0].Type == syntax.TokenIdent && tokens[1].Type == syntax.TokenColon {
		return tokens[0].Text, tokens[2:]
	}
	return "", tokens
}

func enumValuePrefix(tokens []syntax.Token) bool {
	if len(tokens) == 0 {
		return true
	}
	if len(tokens) == 1 {
		return tokens[0].Type == syntax.TokenIdent || tokens[0].Type == syntax.TokenDot
	}
	if len(tokens) == 2 {
		return tokens[0].Type == syntax.TokenDot && tokens[1].Type == syntax.TokenIdent ||
			tokens[0].Type == syntax.TokenIdent && tokens[1].Type == syntax.TokenDot
	}
	return len(tokens) == 3 && tokens[0].Type == syntax.TokenIdent &&
		tokens[1].Type == syntax.TokenDot && tokens[2].Type == syntax.TokenIdent
}

func enumCallDomain(registry *schema.Registry, catalog enums.Catalog, program *syntax.Program, context enumCallContext) string {
	if context.name == "" || len(context.arguments) == 0 {
		return ""
	}
	var parameters []enumCompletionParameter
	primary := -1
	if program != nil {
		for _, declaration := range program.Declarations {
			if function, ok := declaration.(*syntax.FunctionDecl); ok && function.Name == context.name {
				for _, parameter := range function.Parameters {
					parameters = append(parameters, enumCompletionParameter{parameter.Name, strings.TrimPrefix(parameter.Name, "_"), catalog.TypeName(parameter.TypeExpr)})
				}
				if len(parameters) > 0 {
					primary = 0
				}
				break
			}
		}
	}
	if parameters == nil {
		action, ok := registry.LookupAction(context.name)
		if !ok {
			return ""
		}
		for index, parameter := range action.Parameters {
			parameters = append(parameters, enumCompletionParameter{parameter.ID, parameter.Label, parameter.EnumName})
			if candidate, ok := action.PrimaryParameter(); ok && candidate == &action.Parameters[index] {
				primary = index
			}
		}
	}
	bound := make(map[int]bool)
	for argumentIndex, argument := range context.arguments {
		label, value := enumArgumentLabel(argument)
		index := -1
		if label != "" {
			for candidate, parameter := range parameters {
				if label == parameter.name || label == parameter.label {
					if index == -1 || !bound[candidate] {
						index = candidate
					}
					if !bound[candidate] {
						break
					}
				}
			}
		} else if argumentIndex == 0 {
			index = primary
		} else {
			for candidate := range parameters {
				if !bound[candidate] {
					index = candidate
					break
				}
			}
		}
		if argumentIndex == len(context.arguments)-1 {
			if index >= 0 && enumValuePrefix(value) {
				return parameters[index].domain
			}
			return ""
		}
		if index >= 0 {
			bound[index] = true
		}
	}
	return ""
}

func (s *Service) enumCompletions(doc *DocumentState, line, col int) []CompletionItem {
	offset := byteOffsetAtLineCol(doc.File, line, col)
	tokens, context := enumCompletionTokens(doc.File, offset)
	catalog := enums.New(s.registry, doc.Program)
	expected := enumCallDomain(s.registry, catalog, doc.Program, context)
	// Type annotations and function returns can supply an enum context without
	// a call. Use only the cursor's own leaf expression, never an enclosing
	// typed call whose unrelated arguments have their own contracts.
	if expected == "" && doc.Analyzer != nil {
		bestWidth := len(doc.File.Content) + 1
		for expression, name := range doc.Analyzer.EnumContexts() {
			switch expression.(type) {
			case *syntax.EnumMemberExpr, *syntax.MemberExpr, *syntax.IdentExpr:
				span := expression.NodeSpan()
				width := span.End.ByteOffset - span.Start.ByteOffset
				if span.Start.ByteOffset <= offset && offset <= span.End.ByteOffset && width < bestWidth {
					expected, bestWidth = name, width
				}
			}
		}
	}
	index := len(tokens) - 1
	if index >= 0 && tokens[index].Type == syntax.TokenIdent && tokens[index].Span.End.ByteOffset == offset {
		index--
	}
	dotted := index >= 0 && tokens[index].Type == syntax.TokenDot
	domain := expected
	if dotted && index > 0 && tokens[index-1].Type == syntax.TokenIdent {
		qualifier := tokens[index-1].Text
		if !catalog.Has(qualifier) || expected != "" && expected != qualifier {
			return nil
		}
		declarations, _ := collectScopesAndRefs(doc.File, doc.Program)
		if resolveDeclForRef(declarations, qualifier, tokens[index-1].Span.Start.ByteOffset) != nil {
			return nil
		}
		domain = qualifier
	}
	if !catalog.Has(domain) {
		return nil
	}
	var items []CompletionItem
	for _, member := range catalog[domain] {
		insert := "." + member.Name
		if dotted {
			insert = member.Name
		}
		if !syntax.IsIdentifier(member.Name) {
			if dotted {
				continue
			}
			insert = strconv.Quote(member.WireValue)
		}
		items = append(items, CompletionItem{
			Label: member.Name, Kind: CompletionKindEnum, Detail: "enum " + domain,
			InsertText: insert,
		})
	}
	return items
}
