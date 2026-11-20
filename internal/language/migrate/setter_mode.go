package migrate

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// The Legacy three-position Remove shorthand occupies the mode slot despite
// omitting newValue. Inspect only this already-tokenized argument after restoring
// its protected literal; never expose a whole line's strings to syntax rewrites.
// "Set" is data in Legacy's third slot, and explicit labels remain authoritative.
func migrationSetterRemovalArgument(callee string, index, count int, argument string, lexical migrationLexicalSource) (string, bool) {
	if index != 2 || count != 3 || (callee != "editEvent" && callee != "editReminder") {
		return "", false
	}
	restored := lexical.restore(argument)
	lexer := syntax.NewLexer(source.NewFile("migration-setter", "setter.cherri", 1, restored))
	literal := lexer.NextToken()
	if (literal.Type != syntax.TokenString && literal.Type != syntax.TokenRawString) ||
		!strings.EqualFold(literal.Text, "Remove") || lexer.NextToken().Type != syntax.TokenEOF {
		return "", false
	}
	// Legacy accepts case-insensitive shorthand but emits canonical Remove.
	// Preserve surrounding comments while selecting the equivalent V2 enum.
	return restored[:literal.Span.Start.ByteOffset] + "\"Remove\"" + restored[literal.Span.End.ByteOffset:], true
}
