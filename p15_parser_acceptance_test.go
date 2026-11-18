package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15ParseProgram(text string) (*syntax.Program, []syntax.ParseError) {
	file := source.NewFile("p15-parser", "p15-parser.cherri", 1, text)
	parser := syntax.NewParser(file)
	program := parser.ParseProgram()
	return program, parser.Errors()
}

func p15BindingValue(t *testing.T, program *syntax.Program, name string) syntax.Expression {
	t.Helper()
	for _, stmt := range program.Statements {
		if binding, ok := stmt.(*syntax.BindingStmt); ok && binding.Name == name {
			return binding.Value
		}
	}
	t.Fatalf("missing binding %q", name)
	return nil
}

// No precedence table or reparsing: explicit expected trees are independent.
func p15ExpressionShape(expr syntax.Expression) string {
	switch e := expr.(type) {
	case *syntax.LiteralExpr:
		switch e.Type {
		case syntax.TokenString:
			return "text(" + fmt.Sprintf("%q", e.Value) + ")"
		case syntax.TokenRawString:
			return "raw(" + fmt.Sprintf("%q", e.Value) + ")"
		default:
			return e.Value
		}
	case *syntax.IdentExpr:
		return e.Name
	case *syntax.BinaryExpr:
		return "(" + p15Operator(e.Op) + " " + p15ExpressionShape(e.Left) + " " + p15ExpressionShape(e.Right) + ")"
	case *syntax.UnaryExpr:
		return "(" + p15Operator(e.Op) + " " + p15ExpressionShape(e.Operand) + ")"
	case *syntax.CallExpr:
		parts := []string{p15ExpressionShape(e.Callee)}
		if e.PrimaryArg != nil {
			parts = append(parts, p15ExpressionShape(e.PrimaryArg))
		}
		for _, arg := range e.NamedArgs {
			parts = append(parts, arg.Label+":"+p15ExpressionShape(arg.Value))
		}
		return "(call " + strings.Join(parts, " ") + ")"
	case *syntax.MemberExpr:
		return "(member " + p15ExpressionShape(e.Target) + " " + e.Property + ")"
	case *syntax.IndexExpr:
		return "(index " + p15ExpressionShape(e.Target) + " " + p15ExpressionShape(e.Index) + ")"
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}

func p15Operator(op syntax.TokenType) string {
	names := map[syntax.TokenType]string{
		syntax.TokenPlus: "+", syntax.TokenMinus: "-", syntax.TokenStar: "*", syntax.TokenSlash: "/", syntax.TokenPercent: "%",
		syntax.TokenEqual: "==", syntax.TokenNotEqual: "!=", syntax.TokenLess: "<", syntax.TokenLessEqual: "<=",
		syntax.TokenGreater: ">", syntax.TokenGreaterEqual: ">=", syntax.TokenAnd: "&&", syntax.TokenOr: "||", syntax.TokenBang: "!",
	}
	if name, ok := names[op]; ok {
		return name
	}
	return fmt.Sprintf("unknown:%d", op)
}

func p15ExactLiteral(expr syntax.Expression, token syntax.TokenType, want string) error {
	literal, ok := expr.(*syntax.LiteralExpr)
	if !ok {
		return fmt.Errorf("expected literal, got %T", expr)
	}
	if literal.Type != token || literal.Value != want {
		return fmt.Errorf("literal changed: token=%d value=%q, want token=%d value=%q", literal.Type, literal.Value, token, want)
	}
	return nil
}

func TestP15P01LiteralAndInterpolationValues(t *testing.T) {
	src := "let plain = \"{ordinary} JSON: {\\\"key\\\": 1} \\n\\t\\\\ \\u0041 \\U0001F30D\"\n" +
		"let raw = r\"\\d+{literal}\\n\"\n" +
		"let multiline = \"\"\"line one\n\\path {ordinary}\nשָׁלוֹם 🌍\"\"\"\n" +
		"let rawMulti = r\"\"\"\\n\n{raw}\\t\"\"\"\n" +
		"let interpolated = f\"Hello {{literal}} {1 + 2 * 3} שָׁלוֹם 🌍\"\n"
	program, errors := p15ParseProgram(src)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, tc := range []struct {
		name  string
		token syntax.TokenType
		value string
	}{
		{"plain", syntax.TokenString, "{ordinary} JSON: {\"key\": 1} \n\t\\ A 🌍"},
		{"raw", syntax.TokenRawString, "\\d+{literal}\\n"},
		{"multiline", syntax.TokenString, "line one\n\\path {ordinary}\nשָׁלוֹם 🌍"},
		{"rawMulti", syntax.TokenRawString, "\\n\n{raw}\\t"},
	} {
		if err := p15ExactLiteral(p15BindingValue(t, program, tc.name), tc.token, tc.value); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	f, ok := p15BindingValue(t, program, "interpolated").(*syntax.FStringExpr)
	if !ok || len(f.Parts) != 3 {
		t.Fatalf("wrong interpolation structure: %#v", f)
	}
	if f.Parts[0].IsExpr || f.Parts[0].Text != "Hello {literal} " || !f.Parts[1].IsExpr ||
		p15ExpressionShape(f.Parts[1].Expr) != "(+ 1 (* 2 3))" || f.Parts[2].IsExpr || f.Parts[2].Text != " שָׁלוֹם 🌍" {
		t.Fatalf("f-expression/literal parts changed: %+v", f.Parts)
	}
	// Counts cannot detect either causal mutation.
	plain := p15BindingValue(t, program, "plain").(*syntax.LiteralExpr)
	plain.Value = strings.ReplaceAll(plain.Value, "{ordinary}", "")
	if p15ExactLiteral(plain, syntax.TokenString, "{ordinary} JSON: {\"key\": 1} \n\t\\ A 🌍") == nil {
		t.Fatal("lost braces passed oracle")
	}
	raw := p15BindingValue(t, program, "raw").(*syntax.LiteralExpr)
	raw.Value = strings.ReplaceAll(raw.Value, "\\n", "\n")
	if p15ExactLiteral(raw, syntax.TokenRawString, "\\d+{literal}\\n") == nil {
		t.Fatal("decoded raw escape passed oracle")
	}
}

type p15RecoveryProbe struct {
	Text         string
	Line, Column int
}
type p15RecoveryReport struct {
	Errors      []syntax.ParseError
	Diagnostics []analysis.Diagnostic
	Bindings    map[string]string
	Completions map[string]string
}

// A child process bounds real parser/service execution, including a lexer that
// stops advancing. It cannot hang the acceptance runner indefinitely.
func TestP15ParserRecoveryChild(t *testing.T) {
	raw := os.Getenv("CHERRI_P15_PARSER_RECOVERY")
	if raw == "" {
		t.Skip("internal bounded production probe")
	}
	var probe p15RecoveryProbe
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		t.Fatal(err)
	}
	program, errors := p15ParseProgram(probe.Text)
	report := p15RecoveryReport{Errors: errors, Bindings: map[string]string{}, Completions: map[string]string{}}
	for _, stmt := range program.Statements {
		if b, ok := stmt.(*syntax.BindingStmt); ok {
			report.Bindings[b.Name] = p15ExpressionShape(b.Value)
		}
	}
	svc := service.NewService(nil)
	const uri = "file:///p15-recovery.cherri"
	svc.OpenDocument(uri, 1, probe.Text)
	report.Diagnostics, _ = svc.Analyze(uri)
	for _, item := range svc.Complete(uri, probe.Line, probe.Column) {
		switch item.Label {
		case "kept", "restored", "height", "width", "text":
			report.Completions[item.Label] = item.InsertText
		}
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("P15_RECOVERY_REPORT:" + string(encoded))
}

func p15RunRecovery(t *testing.T, probe p15RecoveryProbe) p15RecoveryReport {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestP15ParserRecoveryChild$", "-test.timeout=4s")
	encoded, _ := json.Marshal(probe)
	cmd.Env = append(os.Environ(), "CHERRI_P15_PARSER_RECOVERY="+string(encoded))
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 3000 {
			output = output[len(output)-3000:]
		}
		t.Fatalf("bounded recovery process failed: %v context=%v output=%s", err, ctx.Err(), output)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "P15_RECOVERY_REPORT:") {
			var report p15RecoveryReport
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "P15_RECOVERY_REPORT:")), &report); err != nil {
				t.Fatal(err)
			}
			return report
		}
	}
	t.Fatal("production recovery child did not return its report")
	return p15RecoveryReport{}
}

func p15HasExactError(errors []syntax.ParseError, message string, start, end int) bool {
	for _, err := range errors {
		if err.Message == message && err.Span.Start.ByteOffset == start && err.Span.End.ByteOffset == end {
			return true
		}
	}
	return false
}

func TestP15P02BoundedRecoveryAndCompletion(t *testing.T) {
	for _, tc := range []struct{ name, src, diagnostic, fragment, completion string }{
		{"string", "let kept = 1\nlet broken = \"unterminated\nlet restored = 42\nrestored", "unexpected token in expression: Unterminated single-line string literal", "\"unterminated\n", "restored"},
		{"comment", "let kept = 1\n/* outer /* nested */", "unexpected token in expression: Unterminated block comment", "/* outer /* nested */", "kept"},
		{"named-call", "let kept = 1\nresizeImage(ShortcutInput, width:", "unexpected token in expression: ", "", "height"},
		{"f-string", "let kept = 1\nlet broken = f\"unescaped }\"\nlet restored = 42\nrestored", "Unescaped '}' in f-string", "}", "restored"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(tc.src, "\n")
			report := p15RunRecovery(t, p15RecoveryProbe{Text: tc.src, Line: len(lines), Column: utf8.RuneCountInString(lines[len(lines)-1]) + 1})
			start := strings.Index(tc.src, tc.fragment)
			if tc.fragment == "" {
				start = len(tc.src)
			}
			end := start + len(tc.fragment)
			if !p15HasExactError(report.Errors, tc.diagnostic, start, end) {
				t.Fatalf("missing exact diagnostic/span %q [%d,%d]: %+v", tc.diagnostic, start, end, report.Errors)
			}
			wantErrorCount := 1
			if tc.name == "named-call" {
				wantErrorCount = 2
				closingError := fmt.Sprintf("expected %v, got %v (%q)", syntax.TokenRParen, syntax.TokenEOF, "")
				if !p15HasExactError(report.Errors, closingError, end, end) {
					t.Errorf("missing closing delimiter diagnostic at EOF: %+v", report.Errors)
				}
			}
			if len(report.Errors) != wantErrorCount {
				t.Errorf("unexpected extra or lost diagnostics: %+v", report.Errors)
			}
			if _, ok := report.Completions[tc.completion]; !ok {
				t.Errorf("missing relevant recovered completion %q: %+v", tc.completion, report.Completions)
			}
			if tc.name == "named-call" && report.Completions["height"] != "height: " {
				t.Errorf("named completion insertion lost: %+v", report.Completions)
			}
			if tc.name == "named-call" {
				if _, exists := report.Completions["width"]; exists {
					t.Error("used width label reoffered")
				}
			}
			if report.Bindings["kept"] != "1" {
				t.Error("prior valid binding was lost")
			}
			if tc.completion == "restored" && report.Bindings["restored"] != "42" {
				t.Errorf("following statement did not recover: %+v", report.Bindings)
			}
			seenSyntax := false
			for _, d := range report.Diagnostics {
				if d.Code == analysis.CodeSyntax && d.Span.Start.ByteOffset == start && d.Span.End.ByteOffset == end && d.Message == tc.diagnostic {
					seenSyntax = true
				}
			}
			if !seenSyntax {
				t.Error("language service did not publish exact parser diagnostic")
			}
			mutated := append([]syntax.ParseError{}, report.Errors...)
			for i := range mutated {
				mutated[i].Span.Start.ByteOffset++
			}
			if p15HasExactError(mutated, tc.diagnostic, start, end) {
				t.Error("off-by-one diagnostic mutation passed")
			}
			if p15HasExactError(nil, tc.diagnostic, start, end) {
				t.Error("missing diagnostic mutation passed")
			}
		})
	}
}

func TestP15P03PrecedenceAndComparisonChains(t *testing.T) {
	for _, tc := range []struct{ expr, shape string }{
		{"1 + 2 * 3 == 7 && false || true", "(|| (&& (== (+ 1 (* 2 3)) 7) false) true)"},
		{"(1 + 2) * 3", "(* (+ 1 2) 3)"},
		{"10 - 3 - 2", "(- (- 10 3) 2)"},
		{"10 - (3 - 2)", "(- 10 (- 3 2))"},
		{"!false && true || false", "(|| (&& (! false) true) false)"},
		{"fetch(1)[0].name == \"x\"", "(== (member (index (call fetch 1) 0) name) text(\"x\"))"},
		{"(1 < 2) == true", "(== (< 1 2) true)"},
		{"1 < 2 && 2 < 3", "(&& (< 1 2) (< 2 3))"},
		{"true == (1 < 2)", "(== true (< 1 2))"},
		{"1 + (2 + 3)", "(+ 1 (+ 2 3))"},
		{"(1 + 2).value", "(member (+ 1 2) value)"},
		{"(1 + 2)[0]", "(index (+ 1 2) 0)"},
		{"(f || g)(1)", "(call (|| f g) 1)"},
		{"(-1).value", "(member (- 1) value)"},
	} {
		program, errors := p15ParseProgram("let value = " + tc.expr)
		if len(errors) != 0 {
			t.Fatalf("%s: %v", tc.expr, errors)
		}
		expr := p15BindingValue(t, program, "value")
		if got := p15ExpressionShape(expr); got != tc.shape {
			t.Errorf("%s: got %s want %s", tc.expr, got, tc.shape)
		}
		formatted := syntax.Format(program)
		reparsed, formatErrors := p15ParseProgram(formatted)
		if len(formatErrors) != 0 {
			t.Errorf("formatter produced invalid source for %s: %q %v", tc.expr, formatted, formatErrors)
		} else if got := p15ExpressionShape(p15BindingValue(t, reparsed, "value")); got != tc.shape {
			t.Errorf("formatter changed tree for %s: got %s want %s", tc.expr, got, tc.shape)
		}
		if binary, ok := expr.(*syntax.BinaryExpr); ok {
			original := p15ExpressionShape(binary)
			binary.Left, binary.Right = binary.Right, binary.Left
			if p15ExpressionShape(binary) == original {
				t.Errorf("operand-order mutation escaped shape oracle: %s", tc.expr)
			}
		}
	}
	for _, expr := range []string{"1 < 2 < 3", "1 == 2 == 3", "1 < 2 == true", "true == 1 < 2"} {
		_, errors := p15ParseProgram("let invalid = " + expr)
		found := false
		for _, err := range errors {
			if err.Message == "chained comparisons require explicit parentheses or a logical operator" {
				found = true
			}
		}
		if !found {
			t.Errorf("comparison chain accepted without required diagnostic: %s (%v)", expr, errors)
		}
	}
}

func TestP15P04ReturnTerminators(t *testing.T) {
	for _, tc := range []struct {
		name, body   string
		bare, second bool
	}{
		{"semicolon", "return; text(\"next\")", true, true},
		{"newline", "return\ntext(\"next\")", true, true},
		{"crlf", "return\r\ntext(\"next\")", true, true},
		{"comment-newline", "return // comment\ntext(\"next\")", true, true},
		{"block-comment-newline", "return /* line\nbreak */ text(\"next\")", true, true},
		{"closing-brace", "return", true, false},
		{"same-line-value", "return text(\"value\")", false, false},
		{"same-line-comment-value", "return /* inline */ text(\"value\")", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, errors := p15ParseProgram("function probe() -> Void { " + tc.body + " }")
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			fn, ok := program.Declarations[0].(*syntax.FunctionDecl)
			if !ok || len(fn.Body.Statements) == 0 {
				t.Fatal("missing function body")
			}
			ret, ok := fn.Body.Statements[0].(*syntax.ReturnStmt)
			if !ok {
				t.Fatal("missing ReturnStmt")
			}
			if (ret.Value == nil) != tc.bare {
				t.Errorf("return stole or lost value: %s", p15ExpressionShape(ret.Value))
			}
			if tc.second {
				if len(fn.Body.Statements) != 2 {
					t.Fatalf("following statement lost: %d", len(fn.Body.Statements))
				}
				next, ok := fn.Body.Statements[1].(*syntax.ExprStmt)
				if !ok || p15ExpressionShape(next.Expr) != "(call text text(\"next\"))" {
					t.Fatalf("following statement changed: %#v", fn.Body.Statements[1])
				}
			}
			if !tc.bare && p15ExpressionShape(ret.Value) != "(call text text(\"value\"))" {
				t.Errorf("same-line return value changed")
			}
		})
	}
	for _, src := range []string{"let first=1; let second=2", "let first=1\nlet second=2", "let first=1;\nlet second=2;"} {
		program, errors := p15ParseProgram(src)
		if len(errors) != 0 || p15ExpressionShape(p15BindingValue(t, program, "first")) != "1" || p15ExpressionShape(p15BindingValue(t, program, "second")) != "2" {
			t.Fatalf("statement separator changed: %q %v", src, errors)
		}
	}
	newline, _ := p15ParseProgram("function f() -> Void { return\ntext(\"next\") }")
	sameLine, _ := p15ParseProgram("function f() -> Void { return text(\"next\") }")
	a := newline.Declarations[0].(*syntax.FunctionDecl).Body.Statements[0].(*syntax.ReturnStmt)
	b := sameLine.Declarations[0].(*syntax.FunctionDecl).Body.Statements[0].(*syntax.ReturnStmt)
	if a.Value != nil || b.Value == nil {
		t.Fatal("newline-removal negative control did not change the return boundary")
	}
}

func TestP15P05UnicodeIdentifierTokenizerCompletion(t *testing.T) {
	names := []string{"שלום", "שָׁלוֹם", "cafe\u0301", "א1", "𐐀name"}
	var declarations []string
	for _, name := range names {
		declarations = append(declarations, "let "+name+" = 1")
	}
	src := strings.Join(declarations, "\n")
	file := source.NewFile("unicode", "unicode.cherri", 1, src)
	lexer := syntax.NewLexer(file)
	var tokenNames []string
	for i := 0; i < 100; i++ {
		token := lexer.NextToken()
		if token.Type == syntax.TokenEOF {
			break
		}
		if token.Type == syntax.TokenError {
			t.Errorf("valid Unicode rejected: %s", token.Text)
		}
		if token.Type == syntax.TokenIdent {
			tokenNames = append(tokenNames, token.Text)
			if src[token.Span.Start.ByteOffset:token.Span.End.ByteOffset] != token.Text {
				t.Error("token/source bytes disagree")
			}
			if token.Span.End.UTF16Offset-token.Span.Start.UTF16Offset != len(utf16.Encode([]rune(token.Text))) {
				t.Error("Unicode token UTF16 span changed")
			}
		}
	}
	if !reflect.DeepEqual(tokenNames, names) {
		t.Fatalf("identifier tokens differ: got %q want %q", tokenNames, names)
	}
	program, errors := p15ParseProgram(src)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	svc := service.NewService(nil)
	const uri = "file:///p15-unicode.cherri"
	svc.OpenDocument(uri, 1, src+"\nlet selected = שָ")
	items := svc.Complete(uri, len(names)+1, utf8.RuneCountInString("let selected = שָ")+1)
	got := map[string]string{}
	for _, item := range items {
		if item.Kind == service.CompletionKindVariable {
			insert := item.InsertText
			if insert == "" {
				insert = item.Label
			}
			got[item.Label] = insert
		}
	}
	for _, name := range names {
		if p15BindingValue(t, program, name) == nil {
			t.Fatal("missing parsed Unicode binding")
		}
		if got[name] != name {
			t.Errorf("completion changed exact identifier %q: %q", name, got[name])
		}
		_, errors := p15ParseProgram(src + "\nlet selected = " + got[name])
		if len(errors) != 0 {
			t.Errorf("completion insertion fails parser: %q %v", name, errors)
		}
	}
	for _, control := range []rune{'\u202e', '\u2066', '\u200d', '\ufeff'} {
		bad := "let a" + string(control) + "bc = 1\nlet kept = 2"
		_, errors := p15ParseProgram(bad)
		start := len("let a")
		end := start + utf8.RuneLen(control)
		found := false
		for _, err := range errors {
			if err.Span.Start.ByteOffset == start && err.Span.End.ByteOffset == end {
				found = true
			}
		}
		if !found {
			t.Errorf("identifier control U+%04X was not diagnosed at exact rune span: %+v", control, errors)
		}
	}
	_, leadingErrors := p15ParseProgram("let \u0301bad = 1")
	if len(leadingErrors) == 0 {
		t.Error("combining mark accepted as identifier start")
	}
}
