package syntax

import (
	"fmt"
	"strings"
)

// Cherri escape syntax is deliberately smaller than Go's %q vocabulary.
// In particular \x, \a, \b, \v and \f must never change the decoded value.
func quoteStringLiteral(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, char := range value {
		switch char {
		case '\\':
			out.WriteString(`\\`)
		case '"':
			out.WriteString(`\"`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if char < 0x20 || char == 0x7f {
				fmt.Fprintf(&out, `\u%04x`, char)
			} else {
				out.WriteRune(char)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

// Raw strings do not interpret backslashes. Quoting their content with Go's
// %q therefore changes both bytes and subsequent formatting passes.
func quoteRawStringLiteral(value string) string {
	if !strings.ContainsAny(value, "\"\r\n") {
		return `r"` + value + `"`
	}
	if !strings.Contains(value, `"""`) && !strings.HasSuffix(value, `"`) {
		return `r"""` + value + `"""`
	}
	// A closing quote next to the triple delimiter cannot remain a raw token.
	// Ordinary Cherri text has the same decoded value and no interpolation.
	return quoteStringLiteral(value)
}
