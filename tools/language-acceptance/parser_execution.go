package main

import (
	"regexp"
	"strings"
)

var parserProductionTests = map[string][]string{
	"P01": {"TestP15P01LiteralAndInterpolationValues"},
	"P02": {"TestP15P02BoundedRecoveryAndCompletion"},
	"P03": {"TestP15P03PrecedenceAndComparisonChains"},
	"P04": {"TestP15P04ReturnTerminators"},
	"P05": {"TestP15P05UnicodeIdentifierTokenizerCompletion"},
}

// These are the existing contract tiers. No UI, runtime, or linked repair
// requirement is inferred from a successful parser/service scenario.
func (r *Runner) runParser() {
	for _, scenario := range []struct {
		id, requirement, level string
		levels                 []string
	}{
		{"P01", "Plain, f, raw and multiline text parse without treating ordinary braces as variables", "unit", []string{"unit"}},
		{"P02", "Unterminated string/comment and partial named call recover", "unit+service", []string{"unit", "service"}},
		{"P03", "Pratt precedence, parentheses and short-circuit tree are correct", "unit", []string{"unit"}},
		{"P04", "Semicolon/newline handling and bare return are unambiguous", "unit", []string{"unit"}},
		{"P05", "Unicode identifiers and tokenizer/completion agree", "unit+service", []string{"unit", "service"}},
	} {
		tests := parserProductionTests[scenario.id]
		passed, detail := executeNamedProductionTests(tests)
		firstRecord := len(r.records)
		r.Record(scenario.id, "parser", scenario.requirement, scenario.level, scenario.levels, passed, "PASSED", detail)
		for i := firstRecord; i < len(r.records); i++ {
			r.records[i].Command = productionTestCommand(tests)
		}
	}
}

// Both execution and the receipt use the same argument vector.
func productionTestCommand(required []string) []string {
	return productionTestCommandInPackage(".", required)
}

func productionTestCommandInPackage(packagePath string, required []string) []string {
	var alternatives []string
	for _, name := range required {
		alternatives = append(alternatives, regexp.QuoteMeta(name))
	}
	return []string{"go", "test", "-count=1", "-json", "-timeout", productionTestTimeout(required).String(), packagePath, "-run", "^(" + strings.Join(alternatives, "|") + ")$"}
}
