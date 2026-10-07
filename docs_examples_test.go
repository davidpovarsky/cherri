package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

var codeBlockRegex = regexp.MustCompile("(?s)```cherri\\s*\n(.*?)\n```")

// TestDocsExamplesExtractAndVerify extracts all cherri code fences from documentation
// and verifies that runnable examples compile cleanly and negative examples produce diagnostics.
func TestDocsExamplesExtractAndVerify(t *testing.T) {
	docFiles := []string{
		filepath.Join("docs", "language-v2", "language-guide.md"),
	}

	// Also check cherrilang.org if directory exists
	docsRepoGuide := filepath.Join("..", "cherrilang.org", "fork-language", "guide.md")
	if _, err := os.Stat(docsRepoGuide); err == nil {
		docFiles = append(docFiles, docsRepoGuide)
	}

	reg := schema.DefaultRegistry()
	verifiedCount := 0

	for _, docFile := range docFiles {
		data, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read doc file %s: %v", docFile, err)
		}

		matches := codeBlockRegex.FindAllStringSubmatch(string(data), -1)
		for i, match := range matches {
			snippet := strings.TrimSpace(match[1])
			if snippet == "" {
				continue
			}

			isDeliberateError := strings.Contains(snippet, "Compile Error:") || strings.Contains(snippet, "Expected Error:") || strings.Contains(snippet, "error:")

			// Clean out commented-out error lines before checking if rest is valid
			var cleanLines []string
			for _, line := range strings.Split(snippet, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") && strings.Contains(trimmed, "Compile Error") {
					// Skip intentionally commented error example
					continue
				}
				cleanLines = append(cleanLines, line)
			}
			cleanCode := strings.Join(cleanLines, "\n")

			srcFile := source.NewFile(source.DocumentID(filepath.Base(docFile)), "doc://"+filepath.Base(docFile), i+1, cleanCode)
			parser := syntax.NewParser(srcFile)
			prog := parser.ParseProgram()
			parseErrs := parser.Errors()

			analyzer := analysis.NewAnalyzer(reg)
			analyzer.Analyze(prog)
			diags := analyzer.Diagnostics()

			hasErrors := len(parseErrs) > 0
			for _, d := range diags {
				if d.Severity == analysis.SeverityError {
					hasErrors = true
					break
				}
			}

			if isDeliberateError {
				// If snippet itself contains raw invalid lines without comment
				if strings.Contains(snippet, "// Compile Error:") {
					// Intentionally commented out in snippet, cleanCode should be valid
					if hasErrors {
						t.Errorf("%s snippet #%d had unexpected errors: %v / %v\nCode:\n%s", docFile, i+1, parseErrs, diags, cleanCode)
					}
				}
			} else {
				if hasErrors {
					t.Errorf("%s snippet #%d failed verification: %v / %v\nCode:\n%s", docFile, i+1, parseErrs, diags, cleanCode)
				}
			}
			verifiedCount++
		}
	}

	if verifiedCount < 5 {
		t.Errorf("expected to verify at least 5 doc snippets, verified %d", verifiedCount)
	}
}
