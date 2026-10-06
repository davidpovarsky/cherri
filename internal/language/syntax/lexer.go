package syntax

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/source"
)

type fStringState struct {
	braceDepth int
}

// Lexer tokenizes Cherri v2 source text.
type Lexer struct {
	file         *source.File
	src          string
	pos          int // byte offset
	fStringStack []fStringState
}

// NewLexer creates a new Lexer for a source file.
func NewLexer(file *source.File) *Lexer {
	return &Lexer{
		file: file,
		src:  file.Content,
		pos:  0,
	}
}

func (l *Lexer) peekRune() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos:])
	return r
}

func (l *Lexer) nextRune() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	l.pos += size
	return r
}

func (l *Lexer) peekRuneAt(offsetCount int) rune {
	p := l.pos
	for i := 0; i < offsetCount; i++ {
		if p >= len(l.src) {
			return 0
		}
		_, size := utf8.DecodeRuneInString(l.src[p:])
		p += size
	}
	if p >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[p:])
	return r
}

// NextToken scans and returns the next token.
func (l *Lexer) NextToken() Token {
	// If we are directly inside an f-string chunk (braceDepth == 0), scan string parts
	if len(l.fStringStack) > 0 && l.fStringStack[len(l.fStringStack)-1].braceDepth == 0 {
		return l.scanFStringChunkToken()
	}

	leadingTrivia := l.skipWhitespaceAndComments()

	startPos := l.pos
	if l.pos >= len(l.src) {
		span := l.file.SpanForOffsets(startPos, startPos)
		return Token{Type: TokenEOF, Text: "", Span: span, Leading: leadingTrivia}
	}

	r := l.nextRune()

	// Identifiers, keywords, and raw/f strings
	if isIdentStart(r) {
		// Check for r"..."
		if r == 'r' && l.pos < len(l.src) && l.src[l.pos] == '"' {
			return l.scanRawString(startPos, leadingTrivia)
		}
		// Check for f"..."
		if r == 'f' && l.pos < len(l.src) && l.src[l.pos] == '"' {
			l.nextRune() // consume '"'
			l.fStringStack = append(l.fStringStack, fStringState{braceDepth: 0})
			span := l.file.SpanForOffsets(startPos, l.pos)
			return Token{Type: TokenFStringStart, Text: "f\"", Span: span, Leading: leadingTrivia}
		}

		for isIdentContinue(l.peekRune()) {
			l.nextRune()
		}
		identText := l.src[startPos:l.pos]
		tt := KeywordTokenType(identText)
		span := l.file.SpanForOffsets(startPos, l.pos)
		return Token{Type: tt, Text: identText, Span: span, Leading: leadingTrivia}
	}

	// Numbers
	if unicode.IsDigit(r) {
		for unicode.IsDigit(l.peekRune()) {
			l.nextRune()
		}
		if l.peekRune() == '.' {
			nextNext := l.peekRuneAt(1)
			if unicode.IsDigit(nextNext) {
				l.nextRune() // consume '.'
				for unicode.IsDigit(l.peekRune()) {
					l.nextRune()
				}
			}
		}
		if l.peekRune() == 'e' || l.peekRune() == 'E' {
			l.nextRune()
			if l.peekRune() == '+' || l.peekRune() == '-' {
				l.nextRune()
			}
			for unicode.IsDigit(l.peekRune()) {
				l.nextRune()
			}
		}
		span := l.file.SpanForOffsets(startPos, l.pos)
		return Token{Type: TokenNumber, Text: l.src[startPos:l.pos], Span: span, Leading: leadingTrivia}
	}

	// Strings
	if r == '"' {
		return l.scanNormalString(startPos, leadingTrivia)
	}

	// Operators and punctuation
	switch r {
	case '(':
		return l.makeToken(TokenLParen, "(", startPos, leadingTrivia)
	case ')':
		return l.makeToken(TokenRParen, ")", startPos, leadingTrivia)
	case '{':
		if len(l.fStringStack) > 0 {
			l.fStringStack[len(l.fStringStack)-1].braceDepth++
		}
		return l.makeToken(TokenLBrace, "{", startPos, leadingTrivia)
	case '}':
		if len(l.fStringStack) > 0 {
			depth := l.fStringStack[len(l.fStringStack)-1].braceDepth
			if depth > 0 {
				l.fStringStack[len(l.fStringStack)-1].braceDepth--
			}
		}
		return l.makeToken(TokenRBrace, "}", startPos, leadingTrivia)
	case '[':
		return l.makeToken(TokenLBracket, "[", startPos, leadingTrivia)
	case ']':
		return l.makeToken(TokenRBracket, "]", startPos, leadingTrivia)
	case ',':
		return l.makeToken(TokenComma, ",", startPos, leadingTrivia)
	case ':':
		return l.makeToken(TokenColon, ":", startPos, leadingTrivia)
	case ';':
		return l.makeToken(TokenSemicolon, ";", startPos, leadingTrivia)
	case '.':
		return l.makeToken(TokenDot, ".", startPos, leadingTrivia)
	case '?':
		return l.makeToken(TokenQuestion, "?", startPos, leadingTrivia)
	case '|':
		if l.peekRune() == '|' {
			l.nextRune()
			return l.makeToken(TokenOr, "||", startPos, leadingTrivia)
		}
		return l.makeToken(TokenPipe, "|", startPos, leadingTrivia)
	case '&':
		if l.peekRune() == '&' {
			l.nextRune()
			return l.makeToken(TokenAnd, "&&", startPos, leadingTrivia)
		}
	case '=':
		if l.peekRune() == '=' {
			l.nextRune()
			return l.makeToken(TokenEqual, "==", startPos, leadingTrivia)
		}
		return l.makeToken(TokenAssign, "=", startPos, leadingTrivia)
	case '!':
		if l.peekRune() == '=' {
			l.nextRune()
			return l.makeToken(TokenNotEqual, "!=", startPos, leadingTrivia)
		}
		return l.makeToken(TokenBang, "!", startPos, leadingTrivia)
	case '<':
		if l.peekRune() == '=' {
			l.nextRune()
			return l.makeToken(TokenLessEqual, "<=", startPos, leadingTrivia)
		}
		return l.makeToken(TokenLess, "<", startPos, leadingTrivia)
	case '>':
		if l.peekRune() == '=' {
			l.nextRune()
			return l.makeToken(TokenGreaterEqual, ">=", startPos, leadingTrivia)
		}
		return l.makeToken(TokenGreater, ">", startPos, leadingTrivia)
	case '+':
		if l.peekRune() == '=' {
			l.nextRune()
			return l.makeToken(TokenPlusAssign, "+=", startPos, leadingTrivia)
		}
		return l.makeToken(TokenPlus, "+", startPos, leadingTrivia)
	case '-':
		if l.peekRune() == '=' {
			l.nextRune()
			return l.makeToken(TokenMinusAssign, "-=", startPos, leadingTrivia)
		}
		if l.peekRune() == '>' {
			l.nextRune()
			return l.makeToken(TokenArrow, "->", startPos, leadingTrivia)
		}
		return l.makeToken(TokenMinus, "-", startPos, leadingTrivia)
	case '*':
		if l.peekRune() == '=' {
			l.nextRune()
			return l.makeToken(TokenStarAssign, "*=", startPos, leadingTrivia)
		}
		return l.makeToken(TokenStar, "*", startPos, leadingTrivia)
	case '/':
		if l.peekRune() == '=' {
			l.nextRune()
			return l.makeToken(TokenSlashAssign, "/=", startPos, leadingTrivia)
		}
		return l.makeToken(TokenSlash, "/", startPos, leadingTrivia)
	case '%':
		return l.makeToken(TokenPercent, "%", startPos, leadingTrivia)
	case '@':
		for isIdentContinue(l.peekRune()) {
			l.nextRune()
		}
		text := l.src[startPos:l.pos]
		return l.makeToken(TokenAtIdent, text, startPos, leadingTrivia)
	case '#':
		for isIdentContinue(l.peekRune()) {
			l.nextRune()
		}
		text := l.src[startPos:l.pos]
		return l.makeToken(TokenDirective, text, startPos, leadingTrivia)
	}

	span := l.file.SpanForOffsets(startPos, l.pos)
	return Token{Type: TokenError, Text: string(r), Span: span, Leading: leadingTrivia}
}

func (l *Lexer) makeToken(tt TokenType, text string, startPos int, trivia []source.Trivia) Token {
	span := l.file.SpanForOffsets(startPos, l.pos)
	return Token{Type: tt, Text: text, Span: span, Leading: trivia}
}

func (l *Lexer) scanFStringChunkToken() Token {
	startPos := l.pos
	var sb strings.Builder

	for {
		if l.pos >= len(l.src) {
			l.fStringStack = l.fStringStack[:len(l.fStringStack)-1]
			span := l.file.SpanForOffsets(startPos, l.pos)
			return Token{Type: TokenError, Text: "Unterminated f-string", Span: span}
		}

		r := l.peekRune()
		if r == '"' {
			if sb.Len() > 0 {
				span := l.file.SpanForOffsets(startPos, l.pos)
				return Token{Type: TokenFStringPart, Text: sb.String(), Span: span}
			}
			l.nextRune() // consume closing '"'
			l.fStringStack = l.fStringStack[:len(l.fStringStack)-1]
			span := l.file.SpanForOffsets(startPos, l.pos)
			return Token{Type: TokenFStringEnd, Text: "\"", Span: span}
		}

		if r == '{' {
			if l.peekRuneAt(1) == '{' {
				l.nextRune()
				l.nextRune()
				sb.WriteRune('{')
				continue
			}
			if sb.Len() > 0 {
				span := l.file.SpanForOffsets(startPos, l.pos)
				return Token{Type: TokenFStringPart, Text: sb.String(), Span: span}
			}
			l.nextRune() // consume '{'
			l.fStringStack[len(l.fStringStack)-1].braceDepth = 1
			span := l.file.SpanForOffsets(startPos, l.pos)
			return Token{Type: TokenLBrace, Text: "{", Span: span}
		}

		if r == '}' {
			if l.peekRuneAt(1) == '}' {
				l.nextRune()
				l.nextRune()
				sb.WriteRune('}')
				continue
			}
			span := l.file.SpanForOffsets(startPos, l.pos+1)
			return Token{Type: TokenError, Text: "Unescaped '}' in f-string", Span: span}
		}

		if r == '\\' {
			l.nextRune()
			esc := l.scanEscapeSequence()
			sb.WriteString(esc)
			continue
		}

		if r == '\n' {
			l.fStringStack = l.fStringStack[:len(l.fStringStack)-1]
			span := l.file.SpanForOffsets(startPos, l.pos)
			return Token{Type: TokenError, Text: "Unterminated single-line f-string", Span: span}
		}

		l.nextRune()
		sb.WriteRune(r)
	}
}

func (l *Lexer) scanNormalString(startPos int, leadingTrivia []source.Trivia) Token {
	isTriple := false
	if l.peekRune() == '"' && l.peekRuneAt(1) == '"' {
		isTriple = true
		l.nextRune() // 2nd quote
		l.nextRune() // 3rd quote
	}

	var sb strings.Builder
	for {
		if l.pos >= len(l.src) {
			span := l.file.SpanForOffsets(startPos, l.pos)
			return Token{Type: TokenError, Text: "Unterminated string literal", Span: span, Leading: leadingTrivia}
		}

		r := l.nextRune()
		if isTriple {
			if r == '"' && l.peekRune() == '"' && l.peekRuneAt(1) == '"' {
				l.nextRune()
				l.nextRune()
				break
			}
			sb.WriteRune(r)
		} else {
			if r == '"' {
				break
			}
			if r == '\n' {
				span := l.file.SpanForOffsets(startPos, l.pos)
				return Token{Type: TokenError, Text: "Unterminated single-line string literal", Span: span, Leading: leadingTrivia}
			}
			if r == '\\' {
				esc := l.scanEscapeSequence()
				sb.WriteString(esc)
			} else {
				sb.WriteRune(r)
			}
		}
	}

	span := l.file.SpanForOffsets(startPos, l.pos)
	return Token{Type: TokenString, Text: sb.String(), Span: span, Leading: leadingTrivia}
}

func (l *Lexer) scanRawString(startPos int, leadingTrivia []source.Trivia) Token {
	l.nextRune() // consume '"'
	isTriple := false
	if l.peekRune() == '"' && l.peekRuneAt(1) == '"' {
		isTriple = true
		l.nextRune()
		l.nextRune()
	}

	var sb strings.Builder
	for {
		if l.pos >= len(l.src) {
			span := l.file.SpanForOffsets(startPos, l.pos)
			return Token{Type: TokenError, Text: "Unterminated raw string literal", Span: span, Leading: leadingTrivia}
		}
		r := l.nextRune()
		if isTriple {
			if r == '"' && l.peekRune() == '"' && l.peekRuneAt(1) == '"' {
				l.nextRune()
				l.nextRune()
				break
			}
			sb.WriteRune(r)
		} else {
			if r == '"' {
				break
			}
			sb.WriteRune(r)
		}
	}

	span := l.file.SpanForOffsets(startPos, l.pos)
	return Token{Type: TokenRawString, Text: sb.String(), Span: span, Leading: leadingTrivia}
}

func (l *Lexer) scanEscapeSequence() string {
	if l.pos >= len(l.src) {
		return "\\"
	}
	r := l.nextRune()
	switch r {
	case '\\':
		return "\\"
	case '"':
		return "\""
	case 'n':
		return "\n"
	case 'r':
		return "\r"
	case 't':
		return "\t"
	case '0':
		return "\x00"
	case 'u':
		hex := l.scanHexDigits(4)
		var val rune
		fmt.Sscanf(hex, "%x", &val)
		return string(val)
	case 'U':
		hex := l.scanHexDigits(8)
		var val rune
		fmt.Sscanf(hex, "%x", &val)
		return string(val)
	default:
		return string(r)
	}
}

func (l *Lexer) scanHexDigits(n int) string {
	var sb strings.Builder
	for i := 0; i < n && l.pos < len(l.src); i++ {
		r := l.peekRune()
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			l.nextRune()
			sb.WriteRune(r)
		} else {
			break
		}
	}
	return sb.String()
}

func (l *Lexer) skipWhitespaceAndComments() []source.Trivia {
	var trivia []source.Trivia

	for l.pos < len(l.src) {
		r := l.peekRune()
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			start := l.pos
			for l.pos < len(l.src) {
				nr := l.peekRune()
				if nr == ' ' || nr == '\t' || nr == '\r' || nr == '\n' {
					l.nextRune()
				} else {
					break
				}
			}
			span := l.file.SpanForOffsets(start, l.pos)
			trivia = append(trivia, source.Trivia{
				Kind: source.TriviaWhitespace,
				Text: l.src[start:l.pos],
				Span: span,
			})
			continue
		}

		if r == '/' && l.peekRuneAt(1) == '/' {
			start := l.pos
			l.nextRune() // '/'
			l.nextRune() // '/'
			for l.pos < len(l.src) && l.peekRune() != '\n' {
				l.nextRune()
			}
			span := l.file.SpanForOffsets(start, l.pos)
			trivia = append(trivia, source.Trivia{
				Kind: source.TriviaLineComment,
				Text: l.src[start:l.pos],
				Span: span,
			})
			continue
		}

		if r == '/' && l.peekRuneAt(1) == '*' {
			start := l.pos
			l.nextRune() // '/'
			l.nextRune() // '*'
			depth := 1
			for l.pos < len(l.src) && depth > 0 {
				if l.peekRune() == '/' && l.peekRuneAt(1) == '*' {
					l.nextRune()
					l.nextRune()
					depth++
				} else if l.peekRune() == '*' && l.peekRuneAt(1) == '/' {
					l.nextRune()
					l.nextRune()
					depth--
				} else {
					l.nextRune()
				}
			}
			span := l.file.SpanForOffsets(start, l.pos)
			trivia = append(trivia, source.Trivia{
				Kind: source.TriviaBlockComment,
				Text: l.src[start:l.pos],
				Span: span,
			})
			continue
		}

		break
	}

	return trivia
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentContinue(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
