package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestP15F05UnsupportedLoopControlProduction(t *testing.T) {
	for _, keyword := range []string{"break", "continue", "while"} {
		statement := keyword
		if keyword == "while" {
			statement += " true { getText(\"body\") }"
		}
		contexts := []struct{ name, text string }{
			{"top_level", statement},
			{"inside_repeat", "repeat 2 as i {\n    " + statement + "\n}"},
			{"inside_for", "for item in [1, 2] {\n    " + statement + "\n}"},
		}
		for _, context := range contexts {
			t.Run(keyword+"/"+context.name, func(t *testing.T) {
				file := source.NewFile("f05", "f05-unsupported.cherri", 1, context.text)
				parser := syntax.NewParser(file)
				_ = parser.ParseProgram()
				message := "unsupported keyword '" + keyword + "'"
				exact := false
				for _, diagnostic := range parser.Errors() {
					if strings.Contains(diagnostic.Message, message) && file.Slice(diagnostic.Span) == keyword {
						exact = true
					}
				}
				if !exact {
					t.Fatalf("missing explicit unsupported-keyword diagnostic at exact keyword span: %+v", parser.Errors())
				}
				raw, err := CompileSourceToPlist("f05-unsupported.cherri", context.text)
				if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), message) {
					t.Fatalf("unsupported loop control reached native emission or lost its diagnostic: bytes=%d err=%v", len(raw), err)
				}
			})
		}
	}
}

func TestP15F05SupportedLoopsAndKeywordText(t *testing.T) {
	for _, text := range []string{
		"repeat 2 as i {\n    getText(\"break continue while\")\n}",
		"for item in [1, 2] {\n    // break continue while\n    getText(input: \"healthy\")\n}",
	} {
		file := source.NewFile("f05", "f05-healthy.cherri", 1, text)
		parser := syntax.NewParser(file)
		_ = parser.ParseProgram()
		if diagnostics := parser.Errors(); len(diagnostics) != 0 {
			t.Fatalf("supported loop or ordinary keyword text rejected: %+v", diagnostics)
		}
		raw, err := CompileSourceToPlist("f05-healthy.cherri", text)
		if err != nil || len(raw) == 0 {
			t.Fatalf("supported loop did not compile: bytes=%d err=%v", len(raw), err)
		}
	}
}
