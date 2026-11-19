package migrate_test

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestMigrationInvalidGeneratedSyntaxFailsClosed(t *testing.T) {
	for _, input := range []string{"@value = \"unterminated\n", "@value = 1 +\n", "if @value {\n"} {
		var output, diagnostics, err = migrate.MigrateSourceWithDiagnostics(input)
		if err == nil || output != "" || len(diagnostics) == 0 || !strings.Contains(err.Error(), "invalid V2 syntax") {
			t.Fatalf("invalid conversion reported success: output=%q diagnostics=%+v error=%v", output, diagnostics, err)
		}
		for _, diagnostic := range diagnostics {
			if diagnostic.Line < 1 || diagnostic.Column < 1 || !strings.Contains(diagnostic.Message, "generated V2 source") {
				t.Fatalf("invalid conversion lacked explicit generated-source diagnostic: %+v", diagnostic)
			}
		}
		if migrate.MigrateSource(input) != input {
			t.Fatal("legacy convenience wrapper changed rejected source")
		}
	}
	var output, diagnostics, err = migrate.MigrateSourceWithDiagnostics("@value = 1\nshow(@value)\n")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("restored migration failed: %v %+v", err, diagnostics)
	}
	var parser = syntax.NewParser(source.NewFile("restored", "restored.cherri", 1, output))
	_ = parser.ParseProgram()
	if len(parser.Errors()) != 0 || !strings.Contains(output, "let value") {
		t.Fatalf("valid migration produced invalid output: %q", output)
	}
}
