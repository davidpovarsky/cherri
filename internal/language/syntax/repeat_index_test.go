package syntax

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
)

func TestP15F02FormattingPreservesRepeatIndexSemantics(t *testing.T) {
	var cases = []string{
		`repeat index for 3 { number(index) }`,
		`repeat 3 as index { number(index) }`,
		`let values = repeat index for 3 { yield index }`,
		`let values = repeat 3 as index { yield index }`,
	}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			var expected = "repeat 3 as index"
			if strings.Contains(text, "repeat index for") {
				expected = "repeat index for 3"
			}
			var formatted = text
			for pass := 0; pass < 2; pass++ {
				var parser = NewParser(source.NewFile("repeat", "repeat.cherri", 1, formatted))
				var program = parser.ParseProgram()
				if len(parser.Errors()) != 0 {
					t.Fatal(parser.Errors())
				}
				formatted = Format(program)
				if !strings.Contains(formatted, expected) {
					t.Fatalf("format pass %d changed repeat index spelling/semantics: want %s, got %s", pass, expected, formatted)
				}
			}
		})
	}
}
