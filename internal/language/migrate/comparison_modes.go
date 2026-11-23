package migrate

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func migrationComparisonTokens(text string) []syntax.Token {
	var lexer = syntax.NewLexer(source.NewFile("migration-comparisons", "migration.cherri", 1, text))
	var tokens []syntax.Token
	for token := lexer.NextToken(); token.Type != syntax.TokenEOF; token = lexer.NextToken() {
		tokens = append(tokens, token)
	}
	return tokens
}

// Repeated entry migration has explicit canonical syntax, including the
// shortcut header emitted by the first pass. Preserve that V2 default. Common
// syntax alone remains Legacy input to this Legacy conversion API.
func migrationComparisonCanonicalSource(text string) bool {
	var explicit bool
	for _, token := range migrationComparisonTokens(text) {
		switch token.Type {
		case syntax.TokenLet, syntax.TokenVar, syntax.TokenShortcut, syntax.TokenSetup,
			syntax.TokenTypeKW, syntax.TokenImport, syntax.TokenReturn, syntax.TokenYield,
			syntax.TokenArrow:
			explicit = true
		}
	}
	if !explicit {
		return false
	}
	var parser = syntax.NewParser(source.NewFile("migration-canonical", "migration.cherri", 1, text))
	parser.ParseProgram()
	return len(parser.Errors()) == 0
}

// Capture each original If before the existing condition rewrites. The lexer
// ignores protected comments/literals and recognizes inline/nested Ifs too.
// Removing only the actual legacy keyword keeps the existing !contains,
// between and date rewrites independent of the later V2 mode prefix.
func prepareMigrationComparisonModes(text string, canonical bool) (string, []string) {
	if canonical {
		return text, nil
	}
	var tokens = migrationComparisonTokens(text)
	var body strings.Builder
	var modes []string
	var last int
	for index, token := range tokens {
		if token.Type != syntax.TokenIf {
			continue
		}
		mode := "modern"
		if index+1 < len(tokens) && tokens[index+1].Type == syntax.TokenIdent {
			var next = tokens[index+1]
			if (next.Text == "modern" || next.Text == "legacy") && index+2 < len(tokens) && tokens[index+2].Type == syntax.TokenColon {
				mode = "" // an explicit V2 mode is already present
			} else if next.Text == "legacy" {
				mode = "legacy"
				body.WriteString(text[last:next.Span.Start.ByteOffset])
				last = next.Span.End.ByteOffset
			}
		}
		modes = append(modes, mode)
	}
	body.WriteString(text[last:])
	return body.String(), modes
}

// Compatibility rewrites retain the ordered If inventory. Fail closed if a
// future rewrite changes it, rather than attaching a captured mode to another
// condition. Literal/comment restoration happens only after this pass.
func restoreMigrationComparisonModes(text string, modes []string) (string, error) {
	if modes == nil {
		return text, nil
	}
	var body strings.Builder
	var last, count int
	for _, token := range migrationComparisonTokens(text) {
		if token.Type != syntax.TokenIf {
			continue
		}
		if count >= len(modes) {
			return "", fmt.Errorf("conditional-mode identity gained an If")
		}
		if modes[count] != "" {
			var end = token.Span.End.ByteOffset
			body.WriteString(text[last:end])
			body.WriteString(" " + modes[count] + ":")
			last = end
		}
		count++
	}
	if count != len(modes) {
		return "", fmt.Errorf("conditional-mode identity lost an If")
	}
	body.WriteString(text[last:])
	return body.String(), nil
}
