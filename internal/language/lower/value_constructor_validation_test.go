package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestValueConstructorsLoweringFailsClosedWithoutAnalyzer(t *testing.T) {
	for _, text := range []string{
		`let red = 0.25; let bad = color(red, 0, 0, 1)`,
		`let bad = color("0.25", 0, 0, 1)`,
		`let bad = color(0, 0, 0, 1, 0.5)`,
		`let bad = qty()`, `let bad = qty(1)`,
		`let bad = qty(1, "sec", 9)`, `let bad = qty("oops", "sec")`,
		`let bad = qty(1, 5)`, `let unit = "sec"; let bad = qty(1, unit)`,
	} {
		t.Run(text, func(t *testing.T) {
			var parser = syntax.NewParser(source.NewFile("constructors", "constructors.cherri", 1, text))
			var program = parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				t.Fatal(parser.Errors())
			}
			var workflow, err = NewLowerer(schema.DefaultRegistry()).LowerProgram(program)
			if err == nil || workflow != nil || !strings.Contains(err.Error(), "E_") {
				t.Fatalf("lowerer silently accepted invalid constructor: %v %#v", err, workflow)
			}
		})
	}
}
