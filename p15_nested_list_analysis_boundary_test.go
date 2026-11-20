package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15NativeListAnalysisCase struct {
	name, source string
	targets      []string
}

func p15NativeListAnalysisCases() []p15NativeListAnalysisCase {
	return []p15NativeListAnalysisCase{
		{"literal_rows", "let value=[[1,2],[3,4]]\nreturn value", []string{"[1,2]", "[3,4]"}},
		{"empty_row", "return [[]]", []string{"[]"}},
		{"immutable_alias", "let row=[1,2]\nreturn [row]", []string{"row"}},
		{"broad_alias", "let row:AnyContent=[1,2]\nreturn [row]", []string{"row"}},
		{"function_result", "function row()->List<Number>{return [1,2]}\nreturn [row()]", []string{"row()"}},
		{"function_parameter", "function wrap(row:List<Number>)->AnyContent{return [row]}\nreturn wrap([1,2])", []string{"row"}},
		{"value_if", "return [if true {yield [1,2]} else {yield [3,4]}]", []string{"if true {yield [1,2]} else {yield [3,4]}"}},
		{"value_menu", "return [menu(\"Pick\") {case \"A\" {yield [1,2]} case \"B\" {yield [3,4]}}]", []string{"menu(\"Pick\") {case \"A\" {yield [1,2]} case \"B\" {yield [3,4]}}"}},
		{"dictionary_field_call", "function consume(value:AnyContent)->AnyContent{return value}\nreturn {\"rows\":consume([[1,2]])}", []string{"[1,2]"}},
		{"dictionary_field_control", "return {\"rows\":if true {yield [[1,2]]} else {yield []}}", []string{"[1,2]"}},
		{"dictionary_context_restored", "let inline={\"rows\":[[1,2]]}\nreturn [[3,4]]", []string{"[3,4]"}},
		{"raw_context_restored", "native.action(\"com.example.p15.raw\",parameters:native.plist({\"rows\":[[1,2]]}))\nreturn [[3,4]]", []string{"[3,4]"}},
		{"unicode_prefix", "let prefix=\"🙂\";let value=[[1,2]];return value", []string{"[1,2]"}},
	}
}

// The known limitation is already enforced by the native List lowerer. These
// controls require the same precise target diagnosis before checked emission
// and in the live service. They do not assert native List<List> impossibility.
func p15NativeListAnalysisError(diagnostics []analysis.Diagnostic, tc p15NativeListAnalysisCase) error {
	var errors []analysis.Diagnostic
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			errors = append(errors, d)
		}
	}
	if len(errors) != len(tc.targets) {
		return fmt.Errorf("target diagnostics: got %d, want %d: %+v", len(errors), len(tc.targets), errors)
	}
	used := map[int]bool{}
	for _, target := range tc.targets {
		start := strings.LastIndex(tc.source, target)
		if start < 0 {
			return fmt.Errorf("oracle target absent: %q", target)
		}
		found := -1
		for index, d := range errors {
			if d.Code == "E_NESTED_NATIVE_LIST" && d.Span.Start.ByteOffset == start && d.Span.End.ByteOffset == start+len(target) &&
				strings.Contains(d.Message, "nested List-valued element") && strings.Contains(d.Message, "native content-row encoding") {
				if found >= 0 || used[index] {
					return fmt.Errorf("duplicate target diagnostic")
				}
				found = index
			}
		}
		if found < 0 {
			return fmt.Errorf("missing exact nested List diagnostic for %q: %+v", target, errors)
		}
		used[found] = true
	}
	return nil
}

func TestP15L01NativeListAnalysisMatchesCompile(t *testing.T) {
	for _, tc := range p15NativeListAnalysisCases() {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15NativeListAnalysisError(diagnostics, tc); err != nil {
				t.Error("analysis:", err)
			} else {
				for _, fault := range []string{"removed", "shifted_span", "wrong_code", "wrong_message"} {
					bad := append([]analysis.Diagnostic(nil), diagnostics...)
					for index := range bad {
						if bad[index].Severity != analysis.SeverityError {
							continue
						}
						switch fault {
						case "shifted_span":
							bad[index].Span.Start.ByteOffset++
						case "wrong_code":
							bad[index].Code = analysis.CodeArgumentType
						case "wrong_message":
							bad[index].Message = "generic invalid input"
						}
					}
					if fault == "removed" {
						bad = nil
					}
					if p15NativeListAnalysisError(bad, tc) == nil {
						t.Error("exact diagnostic oracle accepted", fault)
					}
				}
			}
			svc := service.NewService(nil)
			uri := "file:///p15-native-list-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			observed, _ := svc.Analyze(uri)
			if err := p15NativeListAnalysisError(observed, tc); err != nil {
				t.Error("service:", err)
			}
			raw, err := CompileSourceToPlist("p15-nested-list-target.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_NESTED_NATIVE_LIST") {
				t.Errorf("native compile boundary mismatch: bytes=%d error=%v", len(raw), err)
			}
			svc.ChangeDocument(uri, 2, "return {\"rows\":[[],[1,2],[[3],[]]]}")
			recovered, _ := svc.Analyze(uri)
			for _, d := range recovered {
				if d.Severity == analysis.SeverityError {
					t.Error("valid Dictionary context retained prior List error:", d)
				}
			}
		})
	}
}

func p15NativeListTargetRange(tc p15NativeListAnalysisCase, target string) protocol.Range {
	start := strings.LastIndex(tc.source, target)
	position := func(offset int) protocol.Position {
		prefix := tc.source[:offset]
		tail := prefix[strings.LastIndex(prefix, "\n")+1:]
		return protocol.Position{Line: strings.Count(prefix, "\n"), Character: len(utf16.Encode([]rune(tail)))}
	}
	return protocol.Range{Start: position(start), End: position(start + len(target))}
}

func TestP15L01NativeListPublicCheckAndCompile(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	dir := t.TempDir()
	for index, tc := range p15NativeListAnalysisCases() {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, fmt.Sprintf("case-%d.cherri", index))
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, checkErr := p15RunCLI(t, binary, dir, nil, "check", path, "--json")
			var result protocol.AnalyzeResponse
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatal("actual check response:", err, string(out), stderr)
			}
			var errors []protocol.DiagnosticItem
			for _, d := range result.Diagnostics {
				if d.Severity == "ERROR" {
					errors = append(errors, d)
				}
			}
			if checkErr == nil || result.Valid || len(errors) != len(tc.targets) {
				t.Errorf("public check accepted backend-unsupported List: exit=%v response=%+v", checkErr, result)
			}
			for _, target := range tc.targets {
				matches := 0
				for _, d := range errors {
					if d.Code == "E_NESTED_NATIVE_LIST" && d.Range == p15NativeListTargetRange(tc, target) {
						matches++
					}
				}
				if matches != 1 {
					t.Error("public diagnostic code/range mismatch", target, errors)
				}
			}
			destination := filepath.Join(dir, fmt.Sprintf("case-%d.shortcut", index))
			original := []byte("RUN_OWNED_EXISTING_DESTINATION")
			if err := os.WriteFile(destination, original, 0600); err != nil {
				t.Fatal(err)
			}
			_, compileStderr, compileErr := p15RunCLI(t, binary, dir, nil, "compile", path, "--skip-sign", "--output="+destination)
			after, readErr := os.ReadFile(destination)
			if compileErr == nil || !strings.Contains(string(compileStderr), "E_NESTED_NATIVE_LIST") || readErr != nil || string(after) != string(original) {
				t.Fatalf("actual failed compile changed destination or diagnostic: error=%v stderr=%s read=%v bytes=%q", compileErr, compileStderr, readErr, after)
			}
		})
	}
}

func TestP15L01NativeListContextPreservesInlineArrays(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"dictionary", "return {\"rows\":[[],[1,2],[[3],[]]],\"values\":[false,0,\"\"]}"},
		{"dictionary_in_list", "return [{\"rows\":[[],[1,2]]},{\"rows\":[[3,4]]}]"},
		{"map_argument", "function echo(value:Map<Text,AnyContent>)->Map<Text,AnyContent>{return value}\nreturn echo({\"rows\":[[],[1,2],[[3],[]]]})"},
		{"map_return", "function value()->Map<Text,AnyContent>{return {\"rows\":[[],[1,2]]}}\nreturn value()"},
		{"map_default", "function echo(value:Map<Text,AnyContent>={\"rows\":[[],[1,2]]})->Map<Text,AnyContent>{return value}\nreturn echo()"},
		{"flat_scalar_list", "return [false,0,\"\",7]"},
		{"flat_dictionary_list", "return [{},{\"rows\":[[],[1,2]]}]"},
		{"opaque_native_input", "let value=Clipboard\nreturn [value]"},
		{"raw_native_data", "native.action(\"is.workflow.actions.list\",parameters:native.plist({\"WFItems\":[[1,2],[]]}))\nreturn 7"},
		{"raw_then_dictionary", "native.action(\"com.example.p15.raw\",parameters:native.plist({\"rows\":[[1,2]]}))\nreturn {\"rows\":[[3,4]]}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, d := range p15TypeDiagnostics(t, tc.source) {
				if d.Severity == analysis.SeverityError {
					t.Fatal("valid context:", d)
				}
			}
			svc := service.NewService(nil)
			uri := "file:///p15-inline-array-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			observed, _ := svc.Analyze(uri)
			for _, d := range observed {
				if d.Severity == analysis.SeverityError {
					t.Fatal("valid service context:", d)
				}
			}
			raw, err := CompileSourceToPlist("p15-inline-array.cherri", tc.source)
			if err != nil || len(raw) == 0 {
				t.Fatalf("supported inline or explicitly unverified native route rejected: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}
