package syntax

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
)

func TestFormatterIdempotence(t *testing.T) {
	src := `let name = "David"
var count: Number = 1
function multiply(value: Number, factor: Number = 2) -> Number {
    return value * factor
}
let res = multiply(7, factor: 3)
`
	file := source.NewFile("fmt.cherri", "file:///fmt.cherri", 1, src)
	parser1 := NewParser(file)
	prog1 := parser1.ParseProgram()
	if len(parser1.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", parser1.Errors())
	}

	formatted1 := Format(prog1)

	// Second round
	file2 := source.NewFile("fmt2.cherri", "file:///fmt2.cherri", 2, formatted1)
	parser2 := NewParser(file2)
	prog2 := parser2.ParseProgram()
	if len(parser2.Errors()) > 0 {
		t.Fatalf("unexpected parse errors round 2: %v", parser2.Errors())
	}
	formatted2 := Format(prog2)

	if formatted1 != formatted2 {
		t.Errorf("formatter not idempotent!\n--- First ---\n%s\n--- Second ---\n%s", formatted1, formatted2)
	}
}
