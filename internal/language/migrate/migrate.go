/*
 * Copyright (c) Cherri Language v2.0
 */

package migrate

import (
	"regexp"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

var interpolationRegex = regexp.MustCompile(`"([^"\\]*\{[^"\\]*\}[^"\\]*)"`)

// MigrateSource converts legacy Cherri syntax into canonical Cherri v2.0 syntax.
func MigrateSource(input string) string {
	reg := schema.DefaultRegistry()
	lines := strings.Split(input, "\n")
	var outLines []string

	// Track variable mutability: if a var is assigned to multiple times, use 'var', else 'let'
	varAssignmentCount := make(map[string]int)
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "@") {
			parts := strings.Fields(trimmed)
			if len(parts) > 0 {
				varName := strings.TrimPrefix(parts[0], "@")
				varAssignmentCount[varName]++
			}
		}
	}

	seenVars := make(map[string]bool)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 1. Drop legacy preprocessor directives (#include, #define, etc.)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 2. Convert const name = val -> let name = val
		if strings.HasPrefix(trimmed, "const ") {
			line = strings.Replace(line, "const ", "let ", 1)
		}

		// 3. Convert @var = val -> let/var var = val
		if strings.HasPrefix(trimmed, "@") {
			// Check if +=, -=, *=, /=
			if strings.Contains(line, "+=") || strings.Contains(line, "-=") ||
				strings.Contains(line, "*=") || strings.Contains(line, "/=") {
				// Assignment to existing var: @name += val -> name += val
				line = strings.Replace(line, "@", "", 1)
			} else if strings.Contains(line, "=") {
				// First declaration or re-assignment
				parts := strings.SplitN(trimmed, "=", 2)
				lhs := strings.TrimSpace(parts[0])
				varName := strings.TrimPrefix(lhs, "@")

				if !seenVars[varName] {
					seenVars[varName] = true
					keyword := "let"
					if varAssignmentCount[varName] > 1 {
						keyword = "var"
					}
					// Replace leading @varName with keyword varName
					line = strings.Replace(line, "@"+varName, keyword+" "+varName, 1)
				} else {
					// Reassignment
					line = strings.Replace(line, "@"+varName, varName, 1)
				}
			}
		}

		// 4. Strip remaining @ symbols from variables in expressions: {@foo} -> {foo}, @foo -> foo
		line = strings.ReplaceAll(line, "{@", "{")

		// Strip @ outside strings / from variable tokens
		var sb strings.Builder
		inStr := false
		runes := []rune(line)
		for i := 0; i < len(runes); i++ {
			r := runes[i]
			if r == '"' && (i == 0 || runes[i-1] != '\\') {
				inStr = !inStr
				sb.WriteRune(r)
				continue
			}
			if !inStr && r == '@' {
				// skip @
				continue
			}
			sb.WriteRune(r)
		}
		line = sb.String()

		// 5. Upgrade legacy string interpolation "Hello {var}" -> f"Hello {var}"
		line = interpolationRegex.ReplaceAllStringFunc(line, func(match string) string {
			if strings.HasPrefix(match, `f"`) {
				return match
			}
			return "f" + match
		})

		// 6. Label positional arguments for known action schemas
		line = labelCallArguments(line, reg)

		outLines = append(outLines, line)
	}

	result := strings.Join(outLines, "\n")

	// Try formatting canonically
	file := source.NewFile("migration", "migration.cherri", 1, result)
	p := syntax.NewParser(file)
	prog := p.ParseProgram()
	if len(p.Errors()) == 0 {
		return syntax.Format(prog)
	}

	return result
}

func labelCallArguments(line string, reg *schema.Registry) string {
	if !strings.Contains(line, "(") || !strings.Contains(line, ")") {
		return line
	}

	openParen := strings.Index(line, "(")
	closeParen := strings.LastIndex(line, ")")
	if openParen >= closeParen {
		return line
	}

	// Extract callee name before openParen
	prefix := line[:openParen]
	callee := ""
	for i := len(prefix) - 1; i >= 0; i-- {
		ch := prefix[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			callee = string(ch) + callee
		} else {
			break
		}
	}

	if callee == "" {
		return line
	}

	actionSchema, ok := reg.LookupAction(callee)
	if !ok || len(actionSchema.Parameters) <= 1 {
		return line
	}

	argsStr := line[openParen+1 : closeParen]
	rawArgs := splitCallArgs(argsStr)
	if len(rawArgs) <= 1 {
		return line
	}

	var newArgs []string
	for i, arg := range rawArgs {
		trimmed := strings.TrimSpace(arg)
		// Check if already labeled
		colonIdx := strings.Index(trimmed, ":")
		isLabeled := false
		if colonIdx > 0 {
			lbl := strings.TrimSpace(trimmed[:colonIdx])
			if isValidIdent(lbl) {
				isLabeled = true
			}
		}

		if i > 0 && !isLabeled && i < len(actionSchema.Parameters) {
			param := actionSchema.Parameters[i]
			newArgs = append(newArgs, param.Label+": "+trimmed)
		} else {
			newArgs = append(newArgs, trimmed)
		}
	}

	return line[:openParen+1] + strings.Join(newArgs, ", ") + line[closeParen:]
}

func splitCallArgs(s string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	depth := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote {
			current.WriteByte(ch)
			if ch == quoteChar && (i == 0 || s[i-1] != '\\') {
				inQuote = false
			}
			continue
		}

		switch ch {
		case '"', '\'':
			inQuote = true
			quoteChar = ch
			current.WriteByte(ch)
		case '(', '[', '{':
			depth++
			current.WriteByte(ch)
		case ')', ']', '}':
			depth--
			current.WriteByte(ch)
		case ',':
			if depth == 0 {
				args = append(args, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func isValidIdent(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, r := range s {
		if i == 0 && !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_') {
			return false
		}
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}
