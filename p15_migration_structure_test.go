package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15RP33FullProgram() (string, string) {
	reference := base64.StdEncoding.EncodeToString([]byte(`{"fileLocation":{"fileProviderDomainID":"com.apple.files","relativeSubpath":"rp33-public.txt"}}`))
	return `#define name RP33_FLOW
#define glyph apple
#define color red
#define inputs text
#define outputs text
#question recipient "Recipient?" "RP33"
#ref publicDocument ` + reference + `
#include 'helper.cherri'
#include 'actions/documents'
alert(recipient, "RP33 setup")
@word = "value"
@box = {"field": "DICT"}
const greeting = rp33Greet(@word)
show(greeting)
show(@box['field'])
saveFile("rp33-public.txt", @word, false, publicDocument)
if @word {
    show("present:{@word}")
}
repeat index for 2 {
    show("index:{@index}")
}
`, `function rp33Greet(text value): text {
    output("FUNC:{@value}")
}
`
}

type p15RP33ProgramParts struct {
	shortcut  *syntax.ShortcutDecl
	setup     *syntax.SetupDecl
	function  *syntax.FunctionDecl
	save      *syntax.CallExpr
	index     *syntax.IndexExpr
	condition *syntax.IfStmt
	repeat    *syntax.RepeatStmt
}

func p15RP33Parts(program *syntax.Program) p15RP33ProgramParts {
	var parts p15RP33ProgramParts
	for _, declaration := range program.Declarations {
		switch d := declaration.(type) {
		case *syntax.ShortcutDecl:
			parts.shortcut = d
		case *syntax.SetupDecl:
			parts.setup = d
		case *syntax.FunctionDecl:
			parts.function = d
		}
	}
	for _, statement := range program.Statements {
		switch s := statement.(type) {
		case *syntax.ExprStmt:
			if call, okay := s.Expr.(*syntax.CallExpr); okay {
				if callee, okay := call.Callee.(*syntax.IdentExpr); okay && callee.Name == "saveFile" {
					parts.save = call
				}
				if index, okay := call.PrimaryArg.(*syntax.IndexExpr); okay {
					parts.index = index
				}
			}
		case *syntax.IfStmt:
			parts.condition = s
		case *syntax.RepeatStmt:
			parts.repeat = s
		}
	}
	return parts
}

func p15RP33CheckStructure(program *syntax.Program) error {
	p := p15RP33Parts(program)
	if len(program.Declarations) != 3 || len(program.Statements) != 9 || p.shortcut == nil || p.setup == nil || p.function == nil || p.save == nil || p.index == nil || p.condition == nil || p.repeat == nil {
		return fmt.Errorf("missing or extra full-scenario syntax")
	}
	if p.shortcut.Name != "RP33_FLOW" || p.shortcut.Metadata == nil {
		return fmt.Errorf("metadata identity changed")
	}
	fields := map[string]syntax.Expression{}
	for _, field := range p.shortcut.Metadata.Fields {
		fields[field.Name] = field.Value
	}
	icon, okay := fields["icon"].(*syntax.MapExpr)
	if !okay || len(icon.Entries) != 2 {
		return fmt.Errorf("icon metadata changed")
	}
	iconFields := map[string]string{}
	for _, field := range icon.Entries {
		key, keyOkay := field.Key.(*syntax.IdentExpr)
		value, valueOkay := field.Value.(*syntax.IdentExpr)
		if keyOkay && valueOkay {
			iconFields[key.Name] = value.Name
		}
	}
	if iconFields["glyph"] != "apple" || iconFields["color"] != "red" {
		return fmt.Errorf("glyph/color changed")
	}
	for _, name := range []string{"inputs", "outputs"} {
		list, okay := fields[name].(*syntax.ListExpr)
		if !okay || len(list.Elements) != 1 {
			return fmt.Errorf("content metadata changed")
		}
		item, okay := list.Elements[0].(*syntax.IdentExpr)
		if !okay || item.Name != "WFStringContentItem" {
			return fmt.Errorf("content class changed")
		}
	}
	if p.setup.Name != "recipient" || p.setup.Prompt != "Recipient?" || p.setup.DefaultValue != "RP33" {
		return fmt.Errorf("setup directive changed")
	}
	if p.function.Name != "rp33Greet" || len(p.function.Parameters) != 1 || p.function.Parameters[0].Name != "value" || p.function.Body == nil || len(p.function.Body.Statements) != 1 {
		return fmt.Errorf("included function changed")
	}
	parameterType, okay := p.function.Parameters[0].TypeExpr.(*syntax.NamedTypeAnnotation)
	if !okay || parameterType.Name != "Text" {
		return fmt.Errorf("function parameter type changed")
	}
	returned, okay := p.function.Body.Statements[0].(*syntax.ReturnStmt)
	if !okay || !p15RP33FString(returned.Value, "FUNC:", "value") {
		return fmt.Errorf("old interpolation/function return changed")
	}
	var reference syntax.Expression
	for _, argument := range p.save.NamedArgs {
		if argument.Label == "folder" {
			reference = argument.Value
		}
	}
	call, okay := reference.(*syntax.CallExpr)
	if !okay {
		return fmt.Errorf("reference became an output attachment")
	}
	member, okay := call.Callee.(*syntax.MemberExpr)
	if !okay || member.Property != "plist" {
		return fmt.Errorf("reference native escape changed")
	}
	native, okay := member.Target.(*syntax.IdentExpr)
	if !okay || native.Name != "native" {
		return fmt.Errorf("reference namespace changed")
	}
	literal, okay := call.PrimaryArg.(*syntax.LiteralExpr)
	if !okay || literal.Type != syntax.TokenString {
		return fmt.Errorf("reference payload is no longer static data")
	}
	var payload map[string]any
	if json.Unmarshal([]byte(literal.Value), &payload) != nil {
		return fmt.Errorf("reference JSON changed")
	}
	location, okay := payload["fileLocation"].(map[string]any)
	if !okay || len(payload) != 1 || len(location) != 2 || location["fileProviderDomainID"] != "com.apple.files" || location["relativeSubpath"] != "rp33-public.txt" {
		return fmt.Errorf("reference value changed")
	}
	indexTarget, okay := p.index.Target.(*syntax.IdentExpr)
	if !okay || indexTarget.Name != "box" {
		return fmt.Errorf("indexed source identity changed")
	}
	index, okay := p.index.Index.(*syntax.LiteralExpr)
	if !okay || index.Type != syntax.TokenString || index.Value != "field" {
		return fmt.Errorf("dictionary index changed")
	}
	condition, okay := p.condition.Condition.(*syntax.IdentExpr)
	if !okay || condition.Name != "word" || p.condition.ThenBlock == nil || len(p.condition.ThenBlock.Statements) != 1 {
		return fmt.Errorf("has-value condition changed")
	}
	if !p.repeat.LegacyIndex || p.repeat.IndexVar != "index" || p.repeat.BodyBlock == nil || len(p.repeat.BodyBlock.Statements) != 1 {
		return fmt.Errorf("Legacy loop index semantics changed")
	}
	count, okay := p.repeat.Count.(*syntax.LiteralExpr)
	if !okay || count.Type != syntax.TokenNumber || count.Value != "2" {
		return fmt.Errorf("loop count changed")
	}
	return nil
}

func p15RP33FString(value syntax.Expression, prefix, name string) bool {
	text, okay := value.(*syntax.FStringExpr)
	if !okay {
		return false
	}
	var before, after strings.Builder
	seen := false
	for _, part := range text.Parts {
		if part.IsExpr {
			identifier, okay := part.Expr.(*syntax.IdentExpr)
			if seen || !okay || identifier.Name != name {
				return false
			}
			seen = true
		} else if seen {
			after.WriteString(part.Text)
		} else {
			before.WriteString(part.Text)
		}
	}
	return seen && before.String() == prefix && after.Len() == 0
}

func p15RP33Parse(t *testing.T, text string) *syntax.Program {
	t.Helper()
	parser := syntax.NewParser(source.NewFile("rp33-structure", "rp33.cherri", 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal("migrated source is not canonical syntax", parser.Errors())
	}
	return program
}

func TestP15RP33StructuralMigrationProgram(t *testing.T) {
	directory := t.TempDir()
	legacy, helper := p15RP33FullProgram()
	if err := os.WriteFile(filepath.Join(directory, "helper.cherri"), []byte(helper), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(directory)
	converted, diagnostics, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	if err := p15RP33CheckStructure(p15RP33Parse(t, converted)); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"metadata", "setup", "include", "interpolation", "reference", "index", "has_value", "legacy_repeat"} {
		t.Run(fault, func(t *testing.T) {
			program := p15RP33Parse(t, converted)
			parts := p15RP33Parts(program)
			switch fault {
			case "metadata":
				parts.shortcut.Name = "Changed"
			case "setup":
				parts.setup.Prompt = "Changed"
			case "include":
				parts.function.Name = "Changed"
			case "interpolation":
				parts.function.Body.Statements[0].(*syntax.ReturnStmt).Value = &syntax.LiteralExpr{Type: syntax.TokenString, Value: "FUNC:{value}"}
			case "reference":
				for index := range parts.save.NamedArgs {
					if parts.save.NamedArgs[index].Label == "folder" {
						parts.save.NamedArgs[index].Value = &syntax.IdentExpr{Name: "publicDocument"}
					}
				}
			case "index":
				parts.index.Index = &syntax.LiteralExpr{Type: syntax.TokenString, Value: "other"}
			case "has_value":
				parts.condition.Condition = &syntax.LiteralExpr{Type: syntax.TokenTrue, Value: "true"}
			case "legacy_repeat":
				parts.repeat.LegacyIndex = false
			}
			if p15RP33CheckStructure(program) == nil {
				t.Fatal("full scenario syntax mutation passed", fault)
			}
		})
	}
}

func TestP15RP33ReferenceLexicalScope(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`{"fileLocation":{"relativeSubpath":"public"}}`))
	legacy := "#ref document " + encoded + "\n" + `function shadow(text document): text {
    output("local:{@document}")
}
show(document)
text('document {@document}')
`
	converted, diagnostics, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	program := p15RP33Parse(t, converted)
	if len(program.Declarations) != 1 || len(program.Statements) != 2 {
		t.Fatal("reference source structure changed", converted)
	}
	function := program.Declarations[0].(*syntax.FunctionDecl)
	if !p15RP33FString(function.Body.Statements[0].(*syntax.ReturnStmt).Value, "local:", "document") {
		t.Fatal("global reference captured a same-named parameter", converted)
	}
	call := program.Statements[0].(*syntax.ExprStmt).Expr.(*syntax.CallExpr)
	if _, okay := call.PrimaryArg.(*syntax.CallExpr); !okay {
		t.Fatal("unshadowed reference was not inlined", converted)
	}
	text := program.Statements[1].(*syntax.ExprStmt).Expr.(*syntax.CallExpr).PrimaryArg.(*syntax.LiteralExpr)
	if text.Value != "document {@document}" {
		t.Fatal("reference rewrite changed literal text", converted)
	}
}
