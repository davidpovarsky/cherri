package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

func p15FormatComments(t *testing.T, code string) []string {
	t.Helper()
	p := syntax.NewParser(source.NewFile("p15-format", "p15-format.cherri", 1, code))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("formatter source no longer parses: %v\n%s", p.Errors(), code)
	}
	var comments []string
	for _, c := range program.Comments {
		comments = append(comments, c.Text)
	}
	return comments
}

func p15FormatNative(t *testing.T, code string) map[string]any {
	t.Helper()
	data, err := CompileSourceToPlist("p15-format.cherri", code)
	if err != nil {
		t.Fatalf("actual compile failed: %v\n%s", err, code)
	}
	var document map[string]any
	if _, err = plist.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func TestP15E04SyntaxAndServiceProduction(t *testing.T) {
	requirements := []struct{ pkg, name string }{
		{"./internal/language/syntax", "TestP15E04LiteralValuesAndIdempotence"},
		{"./internal/language/syntax", "TestP15E04AllCommentContentsPreserved"},
		{"./internal/language/service", "TestP15E04IncompleteDocumentFormatFailsClosed"},
	}
	seen := map[string]bool{}
	command := exec.Command("go", "test", "./internal/language/syntax", "./internal/language/service", "-count=1", "-json", "-timeout=60s", "-run", "^TestP15E04")
	raw, err := command.CombinedOutput()
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var event struct{ Action, Test, Package string }
		if json.Unmarshal(line, &event) != nil {
			continue
		}
		if event.Action == "skip" {
			t.Fatalf("a required format control skipped: %s", line)
		}
		if event.Action == "pass" && event.Test != "" {
			seen[event.Package+":"+event.Test] = true
		}
	}
	if err != nil {
		t.Fatalf("actual syntax/service producer failed: %v\n%s", err, raw)
	}
	for _, req := range requirements {
		pkg := "github.com/electrikmilk/cherri" + strings.TrimPrefix(req.pkg, ".")
		if !seen[pkg+":"+req.name] {
			t.Fatalf("required scenario not executed: %s %s", pkg, req.name)
		}
	}
}

func TestP15E04ActualCLIAndServiceNativeIdentity(t *testing.T) {
	binary := roundTripBinary(t)
	cases := []struct{ name, code string }{
		{"raw_path", "// header  \nlet value = r\"C:\\Temp\\folder\\n\\t\\u0041\"\nshow(value)\n// tail  \n"},
		{"raw_multiline", "let value = r\"\"\"first \"quoted\" line\nC:\\second\\שלום🙂\"\"\"\nshow(value)\n"},
		{"normal_and_fstring", "/* keep\n exact text */\nlet value = \"שלום🙂\\n\\t\\\\slash\"\nlet result = f\"{{value}}:{value}\"\nshow(result)\n"},
		{"grouping", "let a = number(7)\nlet b = (a - 2) * (3 + 1)\nshow(f\"{b}\")\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := p15FormatNative(t, tc.code)
			wantComments := p15FormatComments(t, tc.code)
			svc := service.NewService(schema.DefaultRegistry())
			uri := "file:///p15-format.cherri"
			svc.OpenDocument(uri, 1, tc.code)
			formatted, err := svc.Format(uri)
			if err != nil {
				t.Fatal(err)
			}
			if got := p15FormatComments(t, formatted); !reflect.DeepEqual(wantComments, got) {
				t.Errorf("service changed comment bytes: want %q got %q", wantComments, got)
			}
			if comparison := shortcutcompare.CompareDocumentsStrict(original, p15FormatNative(t, formatted)); !comparison.Equal {
				t.Errorf("service formatting changed native values/graph: %+v", comparison.Differences)
			}
			svc.OpenDocument(uri, 2, formatted)
			again, err := svc.Format(uri)
			if err != nil || again != formatted {
				t.Errorf("service formatting is not idempotent: %v", err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "input.cherri")
			if err := os.WriteFile(path, []byte(tc.code), 0600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err := p15RunCLI(t, binary, dir, nil, "format", path, "--no-ansi")
			if err != nil {
				t.Fatalf("public format command: %v\n%s", err, stderr)
			}
			if string(stdout) != formatted {
				t.Errorf("CLI and service formatting differ:\nCLI\n%s\nSERVICE\n%s", stdout, formatted)
			}
			unchanged, err := os.ReadFile(path)
			if err != nil || string(unchanged) != tc.code {
				t.Fatal("read-only format changed its source")
			}
			_, stderr, err = p15RunCLI(t, binary, dir, nil, "format", path, "--write", "--no-ansi")
			if err != nil {
				t.Fatalf("actual write format: %v\n%s", err, stderr)
			}
			written, err := os.ReadFile(path)
			if err != nil || string(written) != formatted {
				t.Fatal("write format did not write the checked source")
			}
			_, stderr, err = p15RunCLI(t, binary, dir, nil, "format", path, "--check", "--no-ansi")
			if err != nil {
				t.Fatalf("second public format --check failed: %v\n%s", err, stderr)
			}
		})
	}
}

func TestP15E04FormatterOracleRejectsChangedValuesAndComments(t *testing.T) {
	original := "// exact comment  \nlet value = r\"C:\\first\"\nshow(value)\n"
	mutated := strings.Replace(original, "first", "second", 1)
	if shortcutcompare.CompareDocumentsStrict(p15FormatNative(t, original), p15FormatNative(t, mutated)).Equal {
		t.Fatal("native comparison accepted an actually changed literal")
	}
	dropped := strings.Replace(original, "// exact comment  \n", "", 1)
	if reflect.DeepEqual(p15FormatComments(t, original), p15FormatComments(t, dropped)) {
		t.Fatal("comment oracle accepted a deleted comment")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.cherri")
	invalid := []byte("let value = \"unterminated\n")
	if err := os.WriteFile(path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := p15RunCLI(t, roundTripBinary(t), dir, nil, "format", path, "--write", "--no-ansi")
	if err == nil || !strings.Contains(string(stderr), "Cannot format file with syntax errors") {
		t.Fatalf("invalid public format succeeded or wrong error: %v %s", err, stderr)
	}
	actual, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(actual, invalid) {
		t.Fatal("invalid public format damaged the original source")
	}
}
