package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestP15ParseDiagnosticCodesSharedRoutes(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	var diagnosticPrefix = regexp.MustCompile(`syntax error: \[(E_[A-Z0-9_]+)\]`)
	var cases = []struct{ name, text, code string }{
		{"legacy_variable", "@value = \"historical\"\n", "E_LEGACY_SYNTAX"},
		{"legacy_constant", "const value = \"historical\"\n", "E_LEGACY_SYNTAX"},
		{"legacy_directive", "#define name Historical\n", "E_LEGACY_SYNTAX"},
		{"legacy_reference", "show(@value)\n", "E_LEGACY_SYNTAX"},
		{"malformed_function", "function broken(value: Text -> Text {\n return value\n}\n", "E_SYNTAX"},
		{"code_lookalike_token", "function broken() E_LEGACY_SYNTAX\n", "E_SYNTAX"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var directory = t.TempDir()
			var path = filepath.Join(directory, "source.cherri")
			if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
				t.Fatal(err)
			}
			var parser = syntax.NewParser(source.NewFile(source.DocumentID("file://"+path), "file://"+path, 1, tc.text))
			parser.ParseProgram()
			var parseErrors = parser.Errors()
			if len(parseErrors) == 0 {
				t.Fatal("invalid source has no parse errors")
			}
			var expectedLines []int
			for _, parseError := range parseErrors {
				if parseError.Span.Start.Line > 0 {
					expectedLines = append(expectedLines, parseError.Span.Start.Line)
				}
			}
			var languageService = service.NewService(schema.DefaultRegistry())
			var diagnostics, _, _ = languageService.AnalyzeVersioned("file://"+path, 1, tc.text)
			if len(diagnostics) == 0 || diagnostics[0].Code != tc.code ||
				diagnostics[0].Message != parseErrors[0].Message ||
				diagnostics[0].Span != parseErrors[0].Span {
				t.Fatalf("service changed parse identity, code or location: %+v", diagnostics)
			}
			var requireCode = func(message string) {
				t.Helper()
				var match = diagnosticPrefix.FindStringSubmatch(message)
				if len(match) != 2 || match[1] != tc.code {
					t.Fatalf("compile diagnostic code: got %q, want %s", message, tc.code)
				}
			}
			var workflow, compileError = CompileSourceToPlist(path, tc.text)
			if compileError == nil || len(workflow) != 0 {
				t.Fatal("invalid source emitted a workflow")
			}
			requireCode(compileError.Error())
			var lines, isolationError = collectV2CompileErrorLines(path, tc.text)
			if isolationError == nil || !reflect.DeepEqual(lines, expectedLines) {
				t.Fatalf("isolation lost the original error locations: %v %v", lines, isolationError)
			}
			requireCode(isolationError.Error())
			var mobile = compileForMobileMode(tc.text, "Diagnostic", "v2", false)
			if mobile.OK || mobile.PlistBase64 != "" || mobile.SignedBase64 != "" {
				t.Fatalf("invalid mobile source emitted a workflow: %+v", mobile)
			}
			requireCode(mobile.Error)
			var checkOut, checkErr, checked = p15RunCLI(t, binary, directory, nil, "check", path, "--json", "--no-ansi")
			var response protocol.AnalyzeResponse
			if err := json.Unmarshal(checkOut, &response); err != nil {
				t.Fatal(err, string(checkOut), string(checkErr))
			}
			if checked == nil || response.Valid || len(response.Diagnostics) == 0 || response.Diagnostics[0].Code != tc.code {
				t.Fatalf("CLI check lost the specific parse diagnostic: %+v %v", response, checked)
			}
			for _, form := range []string{"file", "compile"} {
				t.Run(form, func(t *testing.T) {
					var target = filepath.Join(directory, form+".shortcut")
					var args = []string{path, "--skip-sign", "--no-ansi", "--output=" + target}
					if form == "compile" {
						args = append([]string{"compile"}, append(args, "--mode=v2")...)
					}
					var stdout, stderr, err = p15RunCLI(t, binary, directory, nil, args...)
					if err == nil {
						t.Fatal("invalid source compiled successfully")
					}
					requireCode(string(stdout) + "\n" + string(stderr))
					for _, output := range []string{target, filepath.Join(directory, form+"_unsigned.shortcut"), filepath.Join(directory, "source_unsigned.shortcut")} {
						if _, statError := os.Stat(output); !os.IsNotExist(statError) {
							t.Fatal("invalid source created an output", output, statError)
						}
					}
					t.Logf("PARSE_DIAGNOSTIC_ORIGINAL code=%s stdout=%q stderr=%q", tc.code, stdout, stderr)
				})
			}
		})
	}
}
