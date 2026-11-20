package migrate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

type migrationLexeme struct {
	kind, text, replacement string
	start, end              int
}

type migrationDirective struct {
	text string
	line int
}

type migrationLexicalSource struct {
	text         string
	directives   []migrationDirective
	replacements []string
}

// Statement compatibility rewrites only see inert literal/comment placeholders.
// Directives are collected from active lexical records, never arbitrary lines.
// The final source is still parsed, transformed and formatted by the V2 AST.
func prepareMigrationSource(input string) (migrationLexicalSource, []MigrationDiagnostic) {
	lexemes, diags := scanMigrationLexemes(input)
	if len(diags) != 0 {
		return migrationLexicalSource{}, diags
	}
	prefix := "__cherri_migration_literal_"
	for strings.Contains(input, prefix) {
		prefix += "_"
	}
	var result migrationLexicalSource
	var body strings.Builder
	last := 0
	for index, item := range lexemes {
		body.WriteString(input[last:item.start])
		switch item.kind {
		case "directive":
			result.directives = append(result.directives, migrationDirective{
				text: item.text, line: 1 + strings.Count(input[:item.start], "\n"),
			})
		case "string":
			marker := fmt.Sprintf("\"%s%d__\"", prefix, index)
			body.WriteString(marker)
			result.replacements = append(result.replacements, marker, item.replacement)
		case "comment":
			marker := fmt.Sprintf("/*%scomment_%d__%s*/", prefix, index, strings.Repeat("\n", strings.Count(item.text, "\n")))
			if strings.HasPrefix(item.text, "//") {
				marker = fmt.Sprintf("//%scomment_%d__", prefix, index)
			}
			body.WriteString(marker)
			result.replacements = append(result.replacements, marker, item.text)
		}
		last = item.end
	}
	body.WriteString(input[last:])
	result.text = body.String()
	return result, nil
}

func (source migrationLexicalSource) restore(text string) string {
	return strings.NewReplacer(source.replacements...).Replace(text)
}

// Legacy double-quoted strings and single-quoted raw strings have their own
// escape rules and allow physical newlines. This adapter preserves those rules;
// it does not broaden V2's lexer. JSON containers use JSON string decoding.
func scanMigrationLexemes(input string) ([]migrationLexeme, []MigrationDiagnostic) {
	var result []migrationLexeme
	var calls []bool
	lineHasCode, jsonUntil := false, 0
	fail := func(offset int, message string) ([]migrationLexeme, []MigrationDiagnostic) {
		span := source.NewFile("migration", "migration.cherri", 1, input).SpanForOffsets(offset, offset)
		return nil, []MigrationDiagnostic{{Line: span.Start.Line, Column: span.Start.Column, Message: message}}
	}
	for offset := 0; offset < len(input); {
		start := offset
		ch := input[offset]
		if ch == '\n' {
			lineHasCode = false
			offset++
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\r' {
			offset++
			continue
		}
		if ch == '#' && !lineHasCode {
			for offset < len(input) && input[offset] != '\n' {
				offset++
			}
			result = append(result, migrationLexeme{kind: "directive", text: input[start:offset], start: start, end: offset})
			lineHasCode = true
			continue
		}
		if strings.HasPrefix(input[offset:], "//") {
			for offset < len(input) && input[offset] != '\n' {
				offset++
			}
			result = append(result, migrationLexeme{kind: "comment", text: input[start:offset], start: start, end: offset})
			continue
		}
		if strings.HasPrefix(input[offset:], "/*") {
			depth := 1
			offset += 2
			for offset < len(input) && depth > 0 {
				switch {
				case strings.HasPrefix(input[offset:], "/*"):
					depth++
					offset += 2
				case strings.HasPrefix(input[offset:], "*/"):
					depth--
					offset += 2
				default:
					if input[offset] == '\n' {
						lineHasCode = false
					}
					offset++
				}
			}
			if depth != 0 {
				return fail(start, "unterminated Legacy block comment")
			}
			result = append(result, migrationLexeme{kind: "comment", text: input[start:offset], start: start, end: offset})
			continue
		}
		modern := strings.HasPrefix(input[offset:], "\"\"\"") || ((ch == 'f' || ch == 'r') && offset+1 < len(input) && input[offset+1] == '"' && (offset == 0 || !migrationIdentifierByte(input[offset-1])))
		if modern {
			end, err := migrationModernStringEnd(input[offset:])
			if err != nil {
				return fail(start, err.Error())
			}
			offset += end
			original := input[start:offset]
			// Formatting the expression also collapses physical newlines, keeping
			// generated binding line identities stable when placeholders restore.
			parser := syntax.NewParser(source.NewFile("migration-literal", "literal.cherri", 1, "let value = "+original))
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				return fail(start, "invalid canonical string during migration")
			}
			formatted := strings.TrimSpace(syntax.Format(program))
			replacement := strings.TrimPrefix(formatted, "let value = ")
			result = append(result, migrationLexeme{kind: "string", text: original, replacement: replacement, start: start, end: offset})
			lineHasCode = true
			continue
		}
		if (ch == '[' || ch == '{') && offset >= jsonUntil {
			decoder := json.NewDecoder(strings.NewReader(input[offset:]))
			var value json.RawMessage
			if decoder.Decode(&value) == nil {
				jsonUntil = offset + int(decoder.InputOffset())
			}
		}
		if ch == '\'' || ch == '"' {
			value, end, err := migrationLegacyString(input, offset, offset < jsonUntil)
			if err != nil {
				return fail(start, err.Error())
			}
			offset = end
			interpolate := ch == '"' && (len(calls) == 0 || !calls[len(calls)-1])
			replacement := migrationLiteralSource(value, interpolate)
			result = append(result, migrationLexeme{kind: "string", text: input[start:end], replacement: replacement, start: start, end: end})
			lineHasCode = true
			continue
		}
		if ch == '(' {
			callee := migrationPreviousCallee(input[:offset])
			raw := callee == "rawAction" || callee == "native.plist" || (len(calls) != 0 && calls[len(calls)-1])
			calls = append(calls, raw)
		} else if ch == ')' && len(calls) != 0 {
			calls = calls[:len(calls)-1]
		}
		lineHasCode = true
		offset++
	}
	return result, nil
}

func migrationIdentifierByte(ch byte) bool {
	return ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch >= utf8.RuneSelf
}

func migrationPreviousCallee(prefix string) string {
	prefix = strings.TrimRight(prefix, " \t\r\n")
	start := len(prefix)
	for start > 0 && (migrationIdentifierByte(prefix[start-1]) || prefix[start-1] == '.') {
		start--
	}
	return prefix[start:]
}

func migrationModernStringEnd(input string) (int, error) {
	lexer := syntax.NewLexer(source.NewFile("migration-literal", "literal.cherri", 1, input))
	first := lexer.NextToken()
	if first.Type == syntax.TokenString || first.Type == syntax.TokenRawString {
		return first.Span.End.ByteOffset, nil
	}
	if first.Type != syntax.TokenFStringStart {
		return 0, fmt.Errorf("invalid canonical string during migration")
	}
	depth := 1
	for depth != 0 {
		token := lexer.NextToken()
		switch token.Type {
		case syntax.TokenFStringStart:
			depth++
		case syntax.TokenFStringEnd:
			depth--
		case syntax.TokenEOF, syntax.TokenError:
			return 0, fmt.Errorf("unterminated canonical interpolation during migration")
		}
		if depth == 0 {
			return token.Span.End.ByteOffset, nil
		}
	}
	return 0, fmt.Errorf("invalid canonical interpolation during migration")
}

func migrationLegacyString(input string, start int, jsonString bool) (string, int, error) {
	quote := input[start]
	var value strings.Builder
	for offset := start + 1; offset < len(input); {
		ch := input[offset]
		if ch == quote {
			end := offset + 1
			if jsonString && quote == '"' {
				var decoded string
				if err := json.Unmarshal([]byte(input[start:end]), &decoded); err != nil {
					return "", 0, err
				}
				return decoded, end, nil
			}
			return value.String(), end, nil
		}
		if ch == '\\' && offset+1 < len(input) && (quote == '"' || input[offset+1] == '\'') {
			offset++
			escaped, size := utf8.DecodeRuneInString(input[offset:])
			if quote == '"' {
				switch escaped {
				case 'n':
					escaped = '\n'
				case 't':
					escaped = '\t'
				case 'r':
					escaped = '\r'
				}
			}
			value.WriteRune(escaped)
			offset += size
			continue
		}
		r, size := utf8.DecodeRuneInString(input[offset:])
		value.WriteRune(r)
		offset += size
	}
	return "", 0, fmt.Errorf("unterminated Legacy string literal")
}

// This is Legacy's interpolation token grammar, applied only to a decoded
// normal string. Raw strings, comments, directives and native payloads never
// enter this path. Literal braces are escaped for the V2 interpolation parser.
var migrationInlineVariable = regexp.MustCompile("\\{@?([a-zA-Z_][a-zA-Z0-9_]*)(?:\\['(.*?)'])?(?:\\.(.*?))?}")

func migrationQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func migrationLiteralSource(value string, interpolate bool) string {
	matches := migrationInlineVariable.FindAllStringSubmatchIndex(value, -1)
	if !interpolate || len(matches) == 0 {
		return migrationQuote(value)
	}
	var out strings.Builder
	out.WriteString("f\"")
	part := func(text string) {
		quoted := migrationQuote(text)
		out.WriteString(strings.NewReplacer("{", "{{", "}", "}}").Replace(quoted[1 : len(quoted)-1]))
	}
	last := 0
	for _, match := range matches {
		part(value[last:match[0]])
		out.WriteByte('{')
		out.WriteString(value[match[2]:match[3]])
		if match[4] >= 0 {
			out.WriteByte('[')
			out.WriteString(migrationQuote(value[match[4]:match[5]]))
			out.WriteByte(']')
		}
		if match[6] >= 0 {
			out.WriteByte('.')
			out.WriteString(value[match[6]:match[7]])
		}
		out.WriteByte('}')
		last = match[1]
	}
	part(value[last:])
	out.WriteByte('"')
	return out.String()
}

func stripMigrationAtVariables(text string) string {
	lexer := syntax.NewLexer(source.NewFile("migration-line", "migration.cherri", 1, text))
	var out strings.Builder
	last := 0
	for {
		token := lexer.NextToken()
		if token.Type == syntax.TokenEOF || token.Type == syntax.TokenError {
			break
		}
		if token.Type == syntax.TokenAtIdent {
			start := token.Span.Start.ByteOffset
			out.WriteString(text[last:start])
			last = start + 1
		}
	}
	out.WriteString(text[last:])
	return out.String()
}
