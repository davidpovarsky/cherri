package syntax

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/source"
)

// TokenType represents lexical token categories in Cherri v2.0.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError

	// Literals
	TokenIdent
	TokenString      // "literal text"
	TokenFStringPart // parts of f"..."
	TokenRawString   // r"..."
	TokenNumber      // 123, 3.14
	TokenTrue
	TokenFalse
	TokenNone
	TokenNull // Accepted only in explicit JSON context

	// Keywords
	TokenLet
	TokenVar
	TokenFunction
	TokenReturn
	TokenYield
	TokenIf
	TokenElse
	TokenFor
	TokenIn
	TokenRepeat
	TokenAs
	TokenMenu
	TokenCase
	TokenImport
	TokenShortcut
	TokenSetup
	TokenTypeKW // type
	TokenEnum
	TokenAction
	TokenTrigger
	TokenWhile
	TokenBreak
	TokenContinue

	// Legacy tokens for migration diagnostics
	TokenConst     // const
	TokenAtIdent   // @variable
	TokenDirective // #include, #define, etc.

	// Delimiters & punctuation
	TokenLParen    // (
	TokenRParen    // )
	TokenLBrace    // {
	TokenRBrace    // }
	TokenLBracket  // [
	TokenRBracket  // ]
	TokenComma     // ,
	TokenColon     // :
	TokenSemicolon // ;
	TokenDot       // .
	TokenArrow     // ->
	TokenQuestion  // ?
	TokenPipe      // |

	// Operators
	TokenAssign       // =
	TokenPlusAssign   // +=
	TokenMinusAssign  // -=
	TokenStarAssign   // *=
	TokenSlashAssign  // /=
	TokenPlus         // +
	TokenMinus        // -
	TokenStar         // *
	TokenSlash        // /
	TokenPercent      // %
	TokenBang         // !
	TokenEqual        // ==
	TokenNotEqual     // !=
	TokenLess         // <
	TokenLessEqual    // <=
	TokenGreater      // >
	TokenGreaterEqual // >=
	TokenAnd          // &&
	TokenOr           // ||

	// F-String tokens
	TokenFStringStart // f"
	TokenFStringEnd   // " closing an f-string
)

var keywords = map[string]TokenType{
	"let":      TokenLet,
	"var":      TokenVar,
	"function": TokenFunction,
	"return":   TokenReturn,
	"yield":    TokenYield,
	"if":       TokenIf,
	"else":     TokenElse,
	"for":      TokenFor,
	"in":       TokenIn,
	"repeat":   TokenRepeat,
	"as":       TokenAs,
	"menu":     TokenMenu,
	"case":     TokenCase,
	"import":   TokenImport,
	"shortcut": TokenShortcut,
	"setup":    TokenSetup,
	"type":     TokenTypeKW,
	"enum":     TokenEnum,
	"action":   TokenAction,
	"trigger":  TokenTrigger,
	"while":    TokenWhile,
	"break":    TokenBreak,
	"continue": TokenContinue,
	"const":    TokenConst,
	"true":     TokenTrue,
	"false":    TokenFalse,
	"none":     TokenNone,
	"null":     TokenNull,
}

// Token represents a lexical token with its text and source span.
type Token struct {
	Type    TokenType
	Text    string
	Span    source.Span
	Leading []source.Trivia
}

func (t Token) String() string {
	return fmt.Sprintf("%v(%q)@%s", t.Type, t.Text, t.Span)
}

// KeywordTokenType returns the TokenType if s is a keyword, or TokenIdent.
func KeywordTokenType(s string) TokenType {
	if tt, ok := keywords[s]; ok {
		return tt
	}
	return TokenIdent
}
