package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	gotoken "go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// This is a repository route audit, not an execution claim for the iOS ABI.
func p15RP52CExportError(raw []byte) error {
	file, err := parser.ParseFile(gotoken.NewFileSet(), "ios_bridge.go", raw, parser.ParseComments)
	if err != nil {
		return err
	}
	expected := map[string]struct{ mode, sign, core string }{
		"CherriCompile":              {"string(MobileCompileModeV2)", "false", "compileForMobileMode"},
		"CherriCompileSigned":        {"string(MobileCompileModeV2)", "true", "compileForMobileMode"},
		"CherriCompileMode":          {"C.GoString(mode)", "false", "compileForMobileMode"},
		"CherriCompileSignedMode":    {"C.GoString(mode)", "true", "compileForMobileMode"},
		"CherriCompileProfile":       {"C.GoString(mode)", "false", "compileForMobileModeWithOptions"},
		"CherriCompileSignedProfile": {"C.GoString(mode)", "true", "compileForMobileModeWithOptions"},
	}
	expression := func(node ast.Node) string {
		var out bytes.Buffer
		if format.Node(&out, gotoken.NewFileSet(), node) != nil {
			return ""
		}
		return out.String()
	}
	seen := map[string]bool{}
	for _, declaration := range file.Decls {
		fn, okay := declaration.(*ast.FuncDecl)
		if !okay || !strings.HasPrefix(fn.Name.Name, "CherriCompile") {
			continue
		}
		policy, okay := expected[fn.Name.Name]
		if !okay || seen[fn.Name.Name] || fn.Body == nil || len(fn.Body.List) != 1 {
			return fmt.Errorf("unreviewed public source entry %s", fn.Name.Name)
		}
		seen[fn.Name.Name] = true
		parameters := "source *C.char, requestedName *C.char"
		if policy.mode == "C.GoString(mode)" {
			parameters += ", mode *C.char"
		}
		if policy.core == "compileForMobileModeWithOptions" {
			parameters += ", profile *C.char"
		}
		if fn.Recv != nil || expression(fn.Type) != "func("+parameters+") *C.char" {
			return fmt.Errorf("public compile ABI signature changed: %s", fn.Name.Name)
		}
		annotated := false
		if fn.Doc != nil {
			for _, comment := range fn.Doc.List {
				if comment.Text == "//export "+fn.Name.Name {
					annotated = true
				}
			}
		}
		if !annotated {
			return fmt.Errorf("missing actual C export annotation %s", fn.Name.Name)
		}
		ret, okay := fn.Body.List[0].(*ast.ReturnStmt)
		if !okay || len(ret.Results) != 1 {
			return fmt.Errorf("source entry has additional control flow")
		}
		encode, okay := ret.Results[0].(*ast.CallExpr)
		if !okay || expression(encode.Fun) != "encodeMobileResponse" || len(encode.Args) != 1 {
			return fmt.Errorf("source entry bypassed response boundary")
		}
		core, okay := encode.Args[0].(*ast.CallExpr)
		if !okay || expression(core.Fun) != policy.core {
			return fmt.Errorf("source entry bypassed approved compile core")
		}
		want := []string{"C.GoString(source)", "C.GoString(requestedName)", policy.mode, policy.sign}
		if policy.core == "compileForMobileModeWithOptions" {
			want = append(want, "CompileOptions{BuildProfile: BuildProfile(C.GoString(profile))}")
		}
		if len(core.Args) != len(want) {
			return fmt.Errorf("source entry changed explicit mode/profile arity")
		}
		for i, arg := range core.Args {
			if expression(arg) != want[i] {
				return fmt.Errorf("source entry lost mode/source argument %s/%d", fn.Name.Name, i)
			}
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("public compile export inventory incomplete")
	}
	return nil
}

// The small lexical audit keeps comments and all Swift string forms opaque;
// it does not replace the Swift parser or type checker. Only the canonical
// compile entry body below is approved by this repository gate.
func p15RP52SwiftCommentEnd(s string, start int) (int, bool, error) {
	if strings.HasPrefix(s[start:], "//") {
		i := strings.IndexByte(s[start:], '\n')
		if i < 0 {
			return len(s), true, nil
		}
		return start + i, true, nil
	}
	if !strings.HasPrefix(s[start:], "/*") {
		return start, false, nil
	}
	nesting := 1
	for i := start + 2; i < len(s); {
		if strings.HasPrefix(s[i:], "/*") {
			nesting++
			i += 2
			continue
		}
		if strings.HasPrefix(s[i:], "*/") {
			nesting--
			i += 2
			if nesting == 0 {
				return i, true, nil
			}
			continue
		}
		i++
	}
	return 0, true, fmt.Errorf("unterminated Swift comment")
}
func p15RP52SwiftStringEnd(s string, start, depth int) (int, bool, error) {
	if depth > 64 {
		return 0, true, fmt.Errorf("excessive Swift lexical nesting")
	}
	quote := start
	for quote < len(s) && s[quote] == '#' {
		quote++
	}
	if quote >= len(s) || s[quote] != '"' {
		return start, false, nil
	}
	hashes := s[start:quote]
	width := 1
	if strings.HasPrefix(s[quote:], "\"\"\"") {
		width = 3
	}
	close := strings.Repeat("\"", width) + hashes
	for i := quote + width; i < len(s); {
		if strings.HasPrefix(s[i:], close) {
			return i + len(close), true, nil
		}
		escape := "\\" + hashes
		if strings.HasPrefix(s[i:], escape) {
			next := i + len(escape)
			if next < len(s) && s[next] == '(' {
				end, err := p15RP52SwiftInterpolationEnd(s, next, depth+1)
				if err != nil {
					return 0, true, err
				}
				i = end
				continue
			}
			if next >= len(s) {
				return 0, true, fmt.Errorf("unterminated Swift escape")
			}
			_, size := utf8.DecodeRuneInString(s[next:])
			i = next + size
			continue
		}
		if width == 1 && (s[i] == '\n' || s[i] == '\r') {
			return 0, true, fmt.Errorf("unescaped line in Swift string")
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return 0, true, fmt.Errorf("unterminated Swift string")
}
func p15RP52SwiftInterpolationEnd(s string, start, depth int) (int, error) {
	count := 1
	for i := start + 1; i < len(s); {
		if end, yes, err := p15RP52SwiftCommentEnd(s, i); yes {
			if err != nil {
				return 0, err
			}
			i = end
			continue
		}
		if end, yes, err := p15RP52SwiftStringEnd(s, i, depth+1); yes {
			if err != nil {
				return 0, err
			}
			i = end
			continue
		}
		switch s[i] {
		case '(':
			count++
		case ')':
			count--
			if count == 0 {
				return i + 1, nil
			}
		}
		i++
	}
	return 0, fmt.Errorf("unterminated Swift interpolation")
}
func p15RP52SwiftTokens(s string) ([]string, error) {
	if len(s) > 1<<20 || !utf8.ValidString(s) {
		return nil, fmt.Errorf("invalid bounded Swift source")
	}
	var tokens []string
	for i := 0; i < len(s); {
		ch, size := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(ch) {
			i += size
			continue
		}
		if end, yes, err := p15RP52SwiftCommentEnd(s, i); yes {
			if err != nil {
				return nil, err
			}
			i = end
			continue
		}
		if end, yes, err := p15RP52SwiftStringEnd(s, i, 0); yes {
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, s[i:end])
			i = end
			continue
		}
		start := i
		i += size
		if unicode.IsLetter(ch) || ch == '_' {
			for i < len(s) {
				next, n := utf8.DecodeRuneInString(s[i:])
				if !unicode.IsLetter(next) && !unicode.IsDigit(next) && next != '_' {
					break
				}
				i += n
			}
		}
		tokens = append(tokens, s[start:i])
	}
	return tokens, nil
}

const p15RP52ApprovedSwiftCompile = "static func compile(source: String, name: String, mode: String = \"v2\", signed: Bool = false, buildProfile: CherriBuildProfile = .debug) async throws -> CompiledShortcut {\ntry await Task.detached(priority: .userInitiated) {\ntry callCompileBridge(source: source, name: name, mode: mode, signed: signed, buildProfile: buildProfile)\n}.value\n}"
const p15RP52ApprovedSwiftBridge = `private static func callCompileBridge(source: String, name: String, mode: String, signed: Bool, buildProfile: CherriBuildProfile) throws -> CompiledShortcut {
    let resultPointer: UnsafeMutablePointer<CChar>? = source.withCString { sourcePointer in
        name.withCString { namePointer in
            mode.withCString { modePointer in
                buildProfile.rawValue.withCString { profilePointer in
                    let mutableSource = UnsafeMutablePointer(mutating: sourcePointer)
                    let mutableName = UnsafeMutablePointer(mutating: namePointer)
                    let mutableMode = UnsafeMutablePointer(mutating: modePointer)
                    let mutableProfile = UnsafeMutablePointer(mutating: profilePointer)
                    return signed
                        ? CherriCompileSignedProfile(mutableSource, mutableName, mutableMode, mutableProfile)
                        : CherriCompileProfile(mutableSource, mutableName, mutableMode, mutableProfile)
                }
            }
        }
    }
    let response = try decodeBridgeResponse(resultPointer)
    guard let plistBase64 = response.plistBase64,
          let plist = Data(base64Encoded: plistBase64) else {
        throw CompilationDiagnostic(message: "Compiler response did not contain plist data.", line: 1, column: 1)
    }
    let signedData = response.signedBase64.flatMap { Data(base64Encoded: $0) }
    return CompiledShortcut(name: response.name ?? name, plist: plist, signedShortcut: signedData)
}`

func p15RP52SwiftEntryError(raw []byte) error {
	tokens, err := p15RP52SwiftTokens(string(raw))
	if err != nil {
		return err
	}
	wanted, err := p15RP52SwiftTokens(p15RP52ApprovedSwiftCompile)
	if err != nil {
		return err
	}
	bridge, err := p15RP52SwiftTokens(p15RP52ApprovedSwiftBridge)
	if err != nil {
		return err
	}
	enums, entries, bridges := 0, 0, 0
	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i] != "enum" || tokens[i+1] != "CherriCompiler" || tokens[i+2] != "{" {
			continue
		}
		enums++
		depth := 1
		for j := i + 3; j < len(tokens) && depth > 0; j++ {
			if depth == 1 && j+4 < len(tokens) && tokens[j] == "private" && tokens[j+1] == "static" && tokens[j+2] == "func" && tokens[j+3] == "callCompileBridge" && tokens[j+4] == "(" {
				bridges++
				opening := j
				for opening < len(tokens) && tokens[opening] != "{" {
					opening++
				}
				if opening == len(tokens) {
					return fmt.Errorf("missing Swift bridge body")
				}
				end := opening + 1
				bodyDepth := 1
				for end < len(tokens) && bodyDepth > 0 {
					if tokens[end] == "{" {
						bodyDepth++
					}
					if tokens[end] == "}" {
						bodyDepth--
					}
					end++
				}
				if bodyDepth != 0 || !reflect.DeepEqual(tokens[j:end], bridge) {
					return fmt.Errorf("Swift bridge changed source/mode/profile forwarding")
				}
			}
			if depth == 1 && j+3 < len(tokens) && tokens[j] == "static" && tokens[j+1] == "func" && tokens[j+2] == "compile" && tokens[j+3] == "(" {
				entries++
				opening := j
				for opening < len(tokens) && tokens[opening] != "{" {
					opening++
				}
				if opening == len(tokens) {
					return fmt.Errorf("missing Swift entry body")
				}
				end := opening + 1
				bodyDepth := 1
				for end < len(tokens) && bodyDepth > 0 {
					if tokens[end] == "{" {
						bodyDepth++
					}
					if tokens[end] == "}" {
						bodyDepth--
					}
					end++
				}
				if bodyDepth != 0 || !reflect.DeepEqual(tokens[j:end], wanted) {
					return fmt.Errorf("Swift default/forwarding entry differs from explicit V2 policy")
				}
			}
			if tokens[j] == "{" {
				depth++
			}
			if tokens[j] == "}" {
				depth--
			}
		}
		if depth != 0 {
			return fmt.Errorf("unterminated compiler enum")
		}
	}
	if enums != 1 || entries != 1 || bridges != 1 {
		return fmt.Errorf("Swift compile entry inventory differs: enums=%d entries=%d bridges=%d", enums, entries, bridges)
	}
	return nil
}

func TestP15RP52EntrypointSourceBoundaries(t *testing.T) {
	goSource, err := os.ReadFile("ios_bridge.go")
	if err != nil {
		t.Fatal(err)
	}
	swiftSource, err := os.ReadFile("ios/CherriApp/Compiler/CherriCompiler.swift")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("actual_go_exports", func(t *testing.T) {
		if err := p15RP52CExportError(goSource); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("actual_swift_default", func(t *testing.T) {
		if err := p15RP52SwiftEntryError(swiftSource); err != nil {
			t.Fatal(err)
		}
	})
	for _, fault := range []string{"unsigned_default_auto", "signed_default_auto", "explicit_mode_dropped", "profile_mode_dropped", "default_extra_statement", "unknown_export", "missing_export", "swapped_parameters", "wrong_parameter_type", "wrong_return_type"} {
		t.Run("go_"+fault, func(t *testing.T) {
			s := string(goSource)
			switch fault {
			case "unsigned_default_auto":
				s = strings.Replace(s, "string(MobileCompileModeV2), false", "string(MobileCompileModeAuto), false", 1)
			case "signed_default_auto":
				s = strings.Replace(s, "string(MobileCompileModeV2), true", "string(MobileCompileModeAuto), true", 1)
			case "explicit_mode_dropped":
				s = strings.Replace(s, "C.GoString(mode), false)", "string(MobileCompileModeLegacy), false)", 1)
			case "profile_mode_dropped":
				s = strings.Replace(s, "C.GoString(mode), false, CompileOptions", "string(MobileCompileModeLegacy), false, CompileOptions", 1)
			case "default_extra_statement":
				s = strings.Replace(s, "\treturn encodeMobileResponse(compileForMobileMode(", "\t_ = detectLegacySource(C.GoString(source))\n\treturn encodeMobileResponse(compileForMobileMode(", 1)
			case "unknown_export":
				s = strings.ReplaceAll(s, "CherriCompileSignedProfile", "CherriCompileUnknown")
			case "missing_export":
				s = strings.Replace(s, "//export CherriCompile\n", "// no compile export\n", 1)
			case "swapped_parameters":
				s = strings.Replace(s, "func CherriCompile(source *C.char, requestedName *C.char)", "func CherriCompile(requestedName *C.char, source *C.char)", 1)
			case "wrong_parameter_type":
				s = strings.Replace(s, "func CherriCompile(source *C.char", "func CherriCompile(source *C.int", 1)
			case "wrong_return_type":
				s = strings.Replace(s, "requestedName *C.char) *C.char", "requestedName *C.char) *C.int", 1)
			}
			if s == string(goSource) {
				t.Fatal("source corruption did not occur")
			}
			if p15RP52CExportError([]byte(s)) == nil {
				t.Fatal("public route fault accepted", fault)
			}
		})
	}
	for _, fault := range []string{"auto_default", "legacy_default", "mode_literal", "comment_shadow", "string_shadow", "duplicate_entry", "duplicate_enum", "missing_bridge", "local_shadow", "unterminated_comment", "bridge_mode_literal", "bridge_mode_pointer", "bridge_swapped_abi", "duplicate_bridge", "inactive_enum_live_struct"} {
		t.Run("swift_"+fault, func(t *testing.T) {
			s := string(swiftSource)
			switch fault {
			case "auto_default":
				s = strings.Replace(s, "mode: String = \"v2\"", "mode: String = \"auto\"", 1)
			case "legacy_default":
				s = strings.Replace(s, "mode: String = \"v2\"", "mode: String = \"legacy\"", 1)
			case "mode_literal":
				s = strings.Replace(s, "name: name, mode: mode, signed: signed", "name: name, mode: \"auto\", signed: signed", 1)
			case "comment_shadow":
				s = "/* enum CherriCompiler { " + p15RP52ApprovedSwiftCompile + " } */\n" + strings.Replace(s, "mode: String = \"v2\"", "mode: String = \"auto\"", 1)
			case "string_shadow":
				s = "let unused = #\"\"\"\nenum CherriCompiler { " + p15RP52ApprovedSwiftCompile + " }\n\"\"\"#\n" + strings.Replace(s, "mode: String = \"v2\"", "mode: String = \"auto\"", 1)
			case "duplicate_entry":
				s = strings.Replace(s, "enum CherriCompiler {", "enum CherriCompiler {\n"+p15RP52ApprovedSwiftCompile, 1)
			case "duplicate_enum":
				s += "\nenum CherriCompiler { " + p15RP52ApprovedSwiftCompile + " }\n"
			case "missing_bridge":
				s = strings.Replace(s, "try callCompileBridge(source: source", "try bypassCompileBridge(source: source", 1)
			case "local_shadow":
				s = strings.Replace(s, "try await Task.detached(priority: .userInitiated) {", "let mode = \"auto\"\n        return try await Task.detached(priority: .userInitiated) {", 1)
			case "unterminated_comment":
				s += "\n/*"
			case "bridge_mode_literal":
				s = strings.Replace(s, "mode.withCString { modePointer in", "\"auto\".withCString { modePointer in", 1)
			case "bridge_mode_pointer":
				s = strings.Replace(s, "mutating: modePointer", "mutating: namePointer", 1)
			case "bridge_swapped_abi":
				s = strings.Replace(s, "CherriCompileProfile(mutableSource, mutableName, mutableMode, mutableProfile)", "CherriCompileProfile(mutableSource, mutableMode, mutableName, mutableProfile)", 1)
			case "duplicate_bridge":
				s = strings.Replace(s, "enum CherriCompiler {", "enum CherriCompiler {\n"+p15RP52ApprovedSwiftBridge, 1)
			case "inactive_enum_live_struct":
				s = "#if false\n" + s + "\n#else\n" + strings.Replace(strings.Replace(s, "enum CherriCompiler {", "struct CherriCompiler {", 1), "mode: String = \"v2\"", "mode: String = \"auto\"", 1) + "\n#endif\n"
			}
			if s == string(swiftSource) {
				t.Fatal("source corruption did not occur")
			}
			if p15RP52SwiftEntryError([]byte(s)) == nil {
				t.Fatal("Swift route fault accepted", fault)
			}
		})
	}
	for _, prefix := range []struct{ name, value string }{
		{"nested_comment", "/* outer /* enum CherriCompiler { invalid } */ end */\n"},
		{"opaque_interpolation", "let note = \"\\(call(\"enum CherriCompiler { invalid }\"))\"\n"},
		{"opaque_raw_string", "let note = #\"enum CherriCompiler { invalid }\"#\n"},
	} {
		t.Run(prefix.name, func(t *testing.T) {
			if err := p15RP52SwiftEntryError(append([]byte(prefix.value), swiftSource...)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
