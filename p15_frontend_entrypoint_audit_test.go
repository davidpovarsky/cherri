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
		"CherriCompileDocument":      {"", "", "compileDocumentForMobile"},
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
		if policy.core == "compileDocumentForMobile" {
			parameters = "request *C.char"
		}
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
		if policy.core == "compileDocumentForMobile" {
			want = []string{"C.GoString(request)"}
		}
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

const p15RP52ApprovedSwiftCompile = `static func compile(source: String, name: String, mode: String = "v2", signed: Bool = false, buildProfile: CherriBuildProfile = .debug) async throws -> CompiledShortcut {
        try await compile(request: CherriEditorBuildRequest(
            source: source, mode: mode, name: name, buildProfile: buildProfile
        ), signed: signed)
    }`

const p15RP52ApprovedSwiftRequestCompile = `static func compile(request: CherriEditorBuildRequest, signed: Bool = false) async throws -> CompiledShortcut {
        try Task.checkCancellation()
        // The selected file's existing security scope lives across the native
        // call. This neither grants sibling access nor changes the resolver root.
        let fileURL = request.document?.fileURL
        let accessed = fileURL?.startAccessingSecurityScopedResource() ?? false
        defer { if accessed { fileURL?.stopAccessingSecurityScopedResource() } }
        let result = try await Task.detached(priority: .userInitiated) {
            if request.document != nil {
                return try callDocumentCompileBridge(request: request, signed: signed)
            }
            return try callCompileBridge(source: request.source, name: request.name,
                                         mode: request.mode, signed: signed,
                                         buildProfile: request.buildProfile)
        }.value
        // The C ABI is synchronous. Cancellation discards its completed result;
        // it does not pretend to interrupt a Go call that is already executing.
        try Task.checkCancellation()
        return result
    }`

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
        return try compiledShortcut(response, request: CherriEditorBuildRequest(
            source: source, mode: mode, name: name, buildProfile: buildProfile
        ))
    }`

const p15RP52ApprovedSwiftDocumentRequest = `private struct DocumentCompileRequest: Encodable {
        let source: String
        let name: String
        let mode: String
        let buildProfile: String
        let signed: Bool
        let document: CherriDocumentCompileContext
    }`

const p15RP52ApprovedSwiftDocumentBridge = `private static func callDocumentCompileBridge(request: CherriEditorBuildRequest, signed: Bool) throws -> CompiledShortcut {
        guard let document = request.document else {
            throw CompilationDiagnostic(message: "Document compilation requires a document context.", line: 1, column: 1)
        }
        let payload = DocumentCompileRequest(source: request.source, name: request.name,
                                             mode: request.mode, buildProfile: request.buildProfile.rawValue,
                                             signed: signed, document: document)
        let json = String(decoding: try JSONEncoder().encode(payload), as: UTF8.self)
        let pointer = json.withCString { CherriCompileDocument(UnsafeMutablePointer(mutating: $0)) }
        return try compiledShortcut(decodeBridgeResponse(pointer), request: request)
    }`

const p15RP52ApprovedSwiftCompiledShortcut = `private static func compiledShortcut(_ response: CompilerBridgeResponse, request: CherriEditorBuildRequest) throws -> CompiledShortcut {
        guard response.document == request.document,
              response.previewSourceMap == nil || response.previewSourceMap?.document == request.document else {
            throw CompilationDiagnostic(message: "Compiler response does not match the document and asset revisions.", line: 1, column: 1)
        }
        guard let plistBase64 = response.plistBase64,
              let plist = Data(base64Encoded: plistBase64) else {
            throw CompilationDiagnostic(message: "Compiler response did not contain plist data.", line: 1, column: 1)
        }

        guard let actualMode = response.mode, actualMode == "v2" || actualMode == "legacy" else {
            throw CompilationDiagnostic(message: "Compiler response did not identify its selected language mode.", line: 1, column: 1)
        }
        let signedData = response.signedBase64.flatMap { Data(base64Encoded: $0) }
        return CompiledShortcut(
            name: response.name ?? request.name,
            actualMode: actualMode,
            plist: plist,
            signedShortcut: signedData,
            sourceRequest: request,
            previewSourceMap: response.previewSourceMap
        )
    }`

const p15RP52ApprovedSwiftDecode = `private static func decodeBridgeResponse(_ resultPointer: UnsafeMutablePointer<CChar>?) throws -> CompilerBridgeResponse {
        guard let resultPointer else {
            throw CompilationDiagnostic(message: "Cherri compiler returned no response.", line: 1, column: 1)
        }
        defer { CherriFree(resultPointer) }

        let responseData = Data(String(cString: resultPointer).utf8)
        let response = try JSONDecoder().decode(CompilerBridgeResponse.self, from: responseData)

        guard response.ok else {
            throw CompilationDiagnostic(
                message: response.error ?? "Unknown Cherri compiler error.",
                line: response.line ?? 1,
                column: response.column ?? 1
            )
        }
        return response
    }`

func p15RP52SwiftEntryError(raw []byte) error {
	tokens, err := p15RP52SwiftTokens(string(raw))
	if err != nil {
		return err
	}
	// The reviewed declaration is unconditional and top-level. Do not guess
	// Swift build flags or authorize a live alternate declaration from an
	// inactive/nested canonical decoy. String/comment tokens remain opaque.
	depth, declarations := 0, 0
	for i, item := range tokens {
		if item == "#" && i+1 < len(tokens) {
			switch tokens[i+1] {
			case "if", "elseif", "else", "endif":
				return fmt.Errorf("unreviewed conditional Swift source ownership")
			}
		}
		if i+1 < len(tokens) && tokens[i+1] == "CherriCompiler" {
			switch item {
			case "enum", "struct", "class", "actor", "protocol", "extension":
				if item != "enum" || depth != 0 {
					return fmt.Errorf("unreviewed alternate or nested Swift compiler declaration")
				}
				declarations++
			}
		}
		if item == "{" {
			depth++
		}
		if item == "}" {
			depth--
			if depth < 0 {
				return fmt.Errorf("unbalanced Swift source ownership")
			}
		}
	}
	if depth != 0 || declarations != 1 {
		return fmt.Errorf("missing unique top-level Swift compiler declaration")
	}
	// These are reviewed literal declarations, not expectations read from the
	// candidate. Together they cover old source API -> request dispatch -> both
	// actual C entry points -> common failure/context/mode decoding.
	approved := map[string]string{
		"compile:source":            p15RP52ApprovedSwiftCompile,
		"compile:request":           p15RP52ApprovedSwiftRequestCompile,
		"callCompileBridge":         p15RP52ApprovedSwiftBridge,
		"DocumentCompileRequest":    p15RP52ApprovedSwiftDocumentRequest,
		"callDocumentCompileBridge": p15RP52ApprovedSwiftDocumentBridge,
		"compiledShortcut":          p15RP52ApprovedSwiftCompiledShortcut,
		"decodeBridgeResponse":      p15RP52ApprovedSwiftDecode,
	}
	wanted := map[string][]string{}
	for name, declaration := range approved {
		wanted[name], err = p15RP52SwiftTokens(declaration)
		if err != nil {
			return err
		}
	}
	seen := map[string]int{}
	enums := 0
	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i] != "enum" || tokens[i+1] != "CherriCompiler" || tokens[i+2] != "{" {
			continue
		}
		enums++
		depth, memberStart := 1, i+3
		for j := i + 3; j < len(tokens) && depth > 0; j++ {
			key := ""
			if depth == 1 && j+2 < len(tokens) && tokens[j] == "func" {
				name := tokens[j+1]
				if name == "compile" {
					if j+4 >= len(tokens) || tokens[j+2] != "(" {
						return fmt.Errorf("invalid Swift compile declaration")
					}
					key = "compile:" + tokens[j+3]
					if _, ok := wanted[key]; !ok {
						return fmt.Errorf("unreviewed Swift compile overload %s", key)
					}
				} else if _, ok := wanted[name]; ok {
					key = name
				}
			}
			if depth == 1 && j+1 < len(tokens) && tokens[j] == "struct" && tokens[j+1] == "DocumentCompileRequest" {
				key = "DocumentCompileRequest"
			}
			if key != "" {
				seen[key]++
				opening := j
				for opening < len(tokens) && tokens[opening] != "{" {
					opening++
				}
				if opening == len(tokens) {
					return fmt.Errorf("missing Swift route body %s", key)
				}
				end, bodyDepth := opening+1, 1
				for end < len(tokens) && bodyDepth > 0 {
					if tokens[end] == "{" {
						bodyDepth++
					}
					if tokens[end] == "}" {
						bodyDepth--
					}
					end++
				}
				if bodyDepth != 0 || seen[key] != 1 || !reflect.DeepEqual(tokens[memberStart:end], wanted[key]) {
					return fmt.Errorf("Swift route declaration changed forwarding/failure boundary: %s", key)
				}
			}
			if tokens[j] == "{" {
				depth++
			}
			if tokens[j] == "}" {
				depth--
				if depth == 1 {
					memberStart = j + 1
				}
			}
		}
		if depth != 0 {
			return fmt.Errorf("unterminated compiler enum")
		}
	}
	if enums != 1 || len(seen) != len(wanted) {
		return fmt.Errorf("Swift compile route inventory incomplete: enums=%d declarations=%d", enums, len(seen))
	}
	for name := range wanted {
		if seen[name] != 1 {
			return fmt.Errorf("missing unique Swift route declaration %s", name)
		}
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
	for _, fault := range []string{"auto_default", "legacy_default", "mode_literal", "comment_shadow", "string_shadow", "duplicate_entry", "duplicate_enum", "missing_bridge", "local_shadow", "unterminated_comment", "bridge_mode_literal", "bridge_mode_pointer", "bridge_swapped_abi", "duplicate_bridge", "inactive_enum_live_struct", "nested_enum_live_struct"} {
		t.Run("swift_"+fault, func(t *testing.T) {
			s := string(swiftSource)
			switch fault {
			case "auto_default":
				s = strings.Replace(s, "mode: String = \"v2\"", "mode: String = \"auto\"", 1)
			case "legacy_default":
				s = strings.Replace(s, "mode: String = \"v2\"", "mode: String = \"legacy\"", 1)
			case "mode_literal":
				s = strings.Replace(s, "source: source, mode: mode, name: name", "source: source, mode: \"auto\", name: name", 1)
			case "comment_shadow":
				s = "/* enum CherriCompiler { " + p15RP52ApprovedSwiftCompile + " } */\n" + strings.Replace(s, "mode: String = \"v2\"", "mode: String = \"auto\"", 1)
			case "string_shadow":
				s = "let unused = #\"\"\"\nenum CherriCompiler { " + p15RP52ApprovedSwiftCompile + " }\n\"\"\"#\n" + strings.Replace(s, "mode: String = \"v2\"", "mode: String = \"auto\"", 1)
			case "duplicate_entry":
				s = strings.Replace(s, "enum CherriCompiler {", "enum CherriCompiler {\n"+p15RP52ApprovedSwiftCompile, 1)
			case "duplicate_enum":
				s += "\nenum CherriCompiler { " + p15RP52ApprovedSwiftCompile + " }\n"
			case "missing_bridge":
				s = strings.Replace(s, "return try callCompileBridge(source: request.source", "return try bypassCompileBridge(source: request.source", 1)
			case "local_shadow":
				s = strings.Replace(s, "let result = try await Task.detached(priority: .userInitiated) {", "let request = CherriEditorBuildRequest(source: request.source, mode: \"auto\", name: request.name)\n        let result = try await Task.detached(priority: .userInitiated) {", 1)
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
			case "nested_enum_live_struct":
				s = "func auditDecoy() {\n" + s[strings.Index(s, "enum CherriCompiler {"):] + "\n}\n" + strings.Replace(strings.Replace(s, "enum CherriCompiler {", "struct CherriCompiler {", 1), "mode: String = \"v2\"", "mode: String = \"auto\"", 1)
			}
			if s == string(swiftSource) {
				t.Fatal("source corruption did not occur")
			}
			if p15RP52SwiftEntryError([]byte(s)) == nil {
				t.Fatal("Swift route fault accepted", fault)
			}
		})
	}

	// Each corruption changes the actual candidate's live route declaration.
	// The literal approved chain cannot be supplied by a comment or nested decoy.
	for _, fault := range []struct{ name, before, after string }{
		{"request_mode_dropped", "mode: request.mode, signed: signed", "mode: \"auto\", signed: signed"},
		{"request_source_dropped", "callCompileBridge(source: request.source", "callCompileBridge(source: \"fallback\""},
		{"document_branch_dropped", "if request.document != nil {", "if false {"},
		{"document_payload_mode_dropped", "mode: request.mode, buildProfile:", "mode: \"legacy\", buildProfile:"},
		{"document_payload_profile_dropped", "buildProfile: request.buildProfile.rawValue", "buildProfile: \"debug\""},
		{"document_payload_sign_dropped", "signed: signed, document: document", "signed: false, document: document"},
		{"document_payload_context_dropped", "signed: signed, document: document", "signed: signed, document: request.document!"},
		{"document_json_bridge_bypassed", "CherriCompileDocument(UnsafeMutablePointer(mutating: $0))", "CherriCompile(UnsafeMutablePointer(mutating: $0), nil)"},
		{"response_document_relabel", "response.document == request.document", "response.document == response.document"},
		{"response_map_context_dropped", "response.previewSourceMap == nil || response.previewSourceMap?.document == request.document", "true"},
		{"response_source_request_relabel", "sourceRequest: request,", "sourceRequest: CherriEditorBuildRequest(source: \"\", mode: \"legacy\", name: \"\"),"},
		{"response_mode_validation_dropped", "actualMode == \"v2\" || actualMode == \"legacy\"", "actualMode == actualMode"},
		{"response_ok_validation_dropped", p15RP52ApprovedSwiftDecode, strings.Replace(p15RP52ApprovedSwiftDecode, "guard response.ok else {", "guard true else {", 1)},
		{"response_memory_release_dropped", p15RP52ApprovedSwiftDecode, strings.Replace(p15RP52ApprovedSwiftDecode, "defer { CherriFree(resultPointer) }", "defer { _ = resultPointer }", 1)},
		{"request_cancellation_check_dropped", "try Task.checkCancellation()", "_ = Task.isCancelled"},
		{"request_security_scope_release_dropped", "defer { if accessed { fileURL?.stopAccessingSecurityScopedResource() } }", "defer { _ = accessed }"},
		{"payload_field_shape_changed", p15RP52ApprovedSwiftDocumentRequest, strings.Replace(p15RP52ApprovedSwiftDocumentRequest, "let document: CherriDocumentCompileContext", "let document: CherriDocumentCompileContext?", 1)},
		{"duplicate_request_entry", "enum CherriCompiler {", "enum CherriCompiler {\n" + p15RP52ApprovedSwiftRequestCompile},
		{"duplicate_document_bridge", "enum CherriCompiler {", "enum CherriCompiler {\n" + p15RP52ApprovedSwiftDocumentBridge},
		{"duplicate_response_decoder", "enum CherriCompiler {", "enum CherriCompiler {\n" + p15RP52ApprovedSwiftDecode},
		{"duplicate_payload_struct", "enum CherriCompiler {", "enum CherriCompiler {\n" + p15RP52ApprovedSwiftDocumentRequest},
		{"unknown_compile_overload", "enum CherriCompiler {", "enum CherriCompiler {\nstatic func compile(data: Data) async throws -> CompiledShortcut { fatalError() }\n"},
	} {
		t.Run("swift_"+fault.name, func(t *testing.T) {
			corrupt := strings.Replace(string(swiftSource), fault.before, fault.after, 1)
			if corrupt == string(swiftSource) {
				t.Fatal("source corruption did not occur")
			}
			if p15RP52SwiftEntryError([]byte(corrupt)) == nil {
				t.Fatal("Swift document route fault accepted", fault.name)
			}
		})
	}
	for _, fault := range []struct{ name, before, after string }{
		{"document_request_dropped", "compileDocumentForMobile(C.GoString(request))", "compileDocumentForMobile(\"\")"},
		{"document_core_bypassed", "compileDocumentForMobile(C.GoString(request))", "compileForMobile(C.GoString(request), \"\", false)"},
		{"document_wrong_type", "func CherriCompileDocument(request *C.char)", "func CherriCompileDocument(request *C.int)"},
		{"document_missing_export", "//export CherriCompileDocument", "// private document route"},
		{"document_extra_dispatch", "return encodeMobileResponse(compileDocumentForMobile(C.GoString(request)))", "_ = detectLegacySource(C.GoString(request))\nreturn encodeMobileResponse(compileDocumentForMobile(C.GoString(request)))"},
	} {
		t.Run("go_"+fault.name, func(t *testing.T) {
			corrupt := strings.Replace(string(goSource), fault.before, fault.after, 1)
			if corrupt == string(goSource) {
				t.Fatal("source corruption did not occur")
			}
			if p15RP52CExportError([]byte(corrupt)) == nil {
				t.Fatal("C document route fault accepted", fault.name)
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
