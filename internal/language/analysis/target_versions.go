package analysis

import (
	"fmt"
	"strconv"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// targetVersionLiteral validates the input before using the shared component
// comparator. That comparator also supports permissive legacy spellings; an
// authored malformed bound must not silently become zero or a shorter version.
func targetVersionLiteral(expr syntax.Expression) (string, bool) {
	literal, ok := expr.(*syntax.LiteralExpr)
	if !ok {
		return "", false
	}
	switch literal.Type {
	case syntax.TokenNumber, syntax.TokenString, syntax.TokenRawString:
	default:
		return "", false
	}
	return literal.Value, schema.ValidVersionComponents(literal.Value)
}

func legacyTargetVersionText(version float64) string {
	if version <= 0 {
		return ""
	}
	value := strconv.FormatFloat(version, 'f', -1, 64)
	if _, ok := targetVersionLiteral(&syntax.LiteralExpr{Type: syntax.TokenNumber, Value: value}); !ok {
		return ""
	}
	return value
}

func (a *Analyzer) analyzeTargetVersion(field string, expr syntax.Expression, maximum bool) {
	if expr == nil {
		// The parser already reported the missing value; editor recovery can
		// still analyze the rest of the incomplete declaration.
		return
	}
	value, ok := targetVersionLiteral(expr)
	if !ok {
		a.error(CodeTargetValueShape, expr.NodeSpan(), fmt.Sprintf("shortcut field %q requires a literal version with decimal components separated by dots", field))
		return
	}
	// Preserve the legacy numeric accessor for existing callers. All version
	// decisions use the complete spelling, including every minor/patch digit.
	numeric, _ := strconv.ParseFloat(value, 64)
	if maximum {
		a.maxTargetVersion = numeric
		a.maxTargetVersionSource = value
	} else {
		a.minTargetVersion = numeric
		a.minTargetVersionSource = value
	}
}

func (a *Analyzer) checkTargetVersion(action *schema.ActionSchema, name string, span source.Span) {
	if a.checkPlatformVersions(action, name, span) {
		return
	}
	minimum := legacyTargetVersionText(action.MinVersion)
	if a.minTargetVersionSource != "" && minimum != "" && schema.CompareVersions(minimum, a.minTargetVersionSource) > 0 {
		a.error(CodeUnsupportedTarget, span, fmt.Sprintf("action %q requires minimum version %s, but shortcut target version is %s", name, minimum, a.minTargetVersionSource))
	}
	maximum := legacyTargetVersionText(action.MaxVersion)
	if a.maxTargetVersionSource != "" && maximum != "" && schema.CompareVersions(maximum, a.maxTargetVersionSource) < 0 {
		a.error(CodeUnsupportedTarget, span, fmt.Sprintf("action %q is unsupported above version %s (shortcut target version is %s)", name, maximum, a.maxTargetVersionSource))
	}
}
