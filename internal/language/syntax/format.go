package syntax

import (
	"fmt"
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/source"
)

// Formatter formats a Cherri v2 AST into canonical Cherri source code.
type Formatter struct {
	indent     int
	buf        strings.Builder
	comments   []source.Trivia
	commentIdx int
}

// NewFormatter creates a new Formatter.
func NewFormatter() *Formatter {
	return &Formatter{}
}

// Format formats a Program into canonical source text.
func Format(prog *Program) string {
	f := NewFormatter()
	if prog != nil {
		f.comments = prog.Comments
	}
	f.formatProgram(prog)
	return f.buf.String()
}

func (f *Formatter) writeIndent() {
	for i := 0; i < f.indent; i++ {
		f.buf.WriteString("    ")
	}
}

func (f *Formatter) emitCommentsBefore(byteOffset int) {
	for f.commentIdx < len(f.comments) {
		c := f.comments[f.commentIdx]
		if c.Span.Start.ByteOffset > byteOffset {
			break
		}
		f.writeIndent()
		f.buf.WriteString(c.Text)
		f.buf.WriteString("\n")
		f.commentIdx++
	}
}

func (f *Formatter) formatProgram(prog *Program) {
	for i, decl := range prog.Declarations {
		if i > 0 {
			f.buf.WriteString("\n")
		}
		f.emitCommentsBefore(decl.NodeSpan().Start.ByteOffset)
		f.formatDeclaration(decl)
		f.buf.WriteString("\n")
	}

	if len(prog.Declarations) > 0 && len(prog.Statements) > 0 {
		f.buf.WriteString("\n")
	}

	for i, stmt := range prog.Statements {
		if i > 0 {
			f.buf.WriteString("\n")
		}
		f.emitCommentsBefore(stmt.NodeSpan().Start.ByteOffset)
		f.formatStatement(stmt)
		f.buf.WriteString("\n")
	}

	for f.commentIdx < len(f.comments) {
		c := f.comments[f.commentIdx]
		f.writeIndent()
		f.buf.WriteString(c.Text)
		f.buf.WriteString("\n")
		f.commentIdx++
	}
}

func (f *Formatter) formatDeclaration(decl Declaration) {
	switch d := decl.(type) {
	case *ImportDecl:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "import %s as %s", quoteStringLiteral(d.Path), d.Alias)
	case *ShortcutDecl:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "shortcut %s {\n", quoteStringLiteral(d.Name))
		f.indent++
		if d.Metadata != nil {
			for _, field := range d.Metadata.Fields {
				f.writeIndent()
				fmt.Fprintf(&f.buf, "%s: ", field.Name)
				f.formatExpression(field.Value)
				f.buf.WriteString("\n")
			}
		}
		for _, trig := range d.Triggers {
			f.writeIndent()
			var args []string
			if trig.Event != "" {
				args = append(args, fmt.Sprintf("event: .%s", trig.Event))
			}
			if trig.HasLevel {
				args = append(args, fmt.Sprintf("level: %g", trig.Level))
			}
			if len(trig.Locations) > 0 {
				var locStrs []string
				for _, loc := range trig.Locations {
					locStrs = append(locStrs, quoteStringLiteral(loc))
				}
				args = append(args, fmt.Sprintf("locations: [%s]", strings.Join(locStrs, ", ")))
			}
			if len(args) > 0 {
				fmt.Fprintf(&f.buf, "trigger %s(%s)\n", trig.Family, strings.Join(args, ", "))
			} else {
				fmt.Fprintf(&f.buf, "trigger %s\n", trig.Family)
			}
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")
	case *TriggerDecl:
		f.writeIndent()
		var args []string
		if d.Event != "" {
			args = append(args, fmt.Sprintf("event: .%s", d.Event))
		}
		if d.HasLevel {
			args = append(args, fmt.Sprintf("level: %g", d.Level))
		}
		if len(d.Locations) > 0 {
			var locStrs []string
			for _, loc := range d.Locations {
				locStrs = append(locStrs, quoteStringLiteral(loc))
			}
			args = append(args, fmt.Sprintf("locations: [%s]", strings.Join(locStrs, ", ")))
		}
		if len(args) > 0 {
			fmt.Fprintf(&f.buf, "trigger %s(%s)", d.Family, strings.Join(args, ", "))
		} else {
			fmt.Fprintf(&f.buf, "trigger %s", d.Family)
		}
	case *CopyBlockStmt:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "copy %s ", d.Name)
		f.formatBlockStmt(d.Body, false)
	case *SetupDecl:
		f.writeIndent()
		tStr := ""
		if d.TypeExpr != nil {
			tStr = ": " + formatTypeAnnotation(d.TypeExpr)
		}
		fmt.Fprintf(&f.buf, "setup %s%s {\n", d.Name, tStr)
		f.indent++
		if d.Prompt != "" {
			f.writeIndent()
			fmt.Fprintf(&f.buf, "prompt: %s\n", quoteStringLiteral(d.Prompt))
		}
		if d.DefaultValue != "" {
			f.writeIndent()
			fmt.Fprintf(&f.buf, "default: %s\n", quoteStringLiteral(d.DefaultValue))
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")
	case *TypeDecl:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "type %s = %s", d.Name, formatTypeAnnotation(d.TypeExpr))
	case *EnumDecl:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "enum %s {\n", d.Name)
		f.indent++
		for i, m := range d.Members {
			f.writeIndent()
			name := m.Name
			if !IsIdentifier(name) {
				name = quoteStringLiteral(name)
			}
			if m.WireValue != m.Name {
				fmt.Fprintf(&f.buf, "%s = %s", name, quoteStringLiteral(m.WireValue))
			} else {
				f.buf.WriteString(name)
			}
			if i < len(d.Members)-1 {
				f.buf.WriteString(",")
			}
			f.buf.WriteString("\n")
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")
	case *FunctionDecl:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "function %s(", d.Name)
		for i, p := range d.Parameters {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			opt := ""
			if p.Optional {
				opt = "?"
			}
			fmt.Fprintf(&f.buf, "%s%s: %s", p.Name, opt, formatTypeAnnotation(p.TypeExpr))
			if p.DefaultExpr != nil {
				f.buf.WriteString(" = ")
				f.formatExpression(p.DefaultExpr)
			}
		}
		f.buf.WriteString(")")
		if d.ReturnType != nil {
			fmt.Fprintf(&f.buf, " -> %s", formatTypeAnnotation(d.ReturnType))
		}
		f.buf.WriteString(" ")
		f.formatBlockStmt(d.Body, false)
	case *ActionDecl:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "action %s(", d.Name)
		for i, p := range d.Parameters {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			fmt.Fprintf(&f.buf, "%s: %s", p.Name, formatTypeAnnotation(p.TypeExpr))
			if p.DefaultExpr != nil {
				f.buf.WriteString(" = ")
				f.formatExpression(p.DefaultExpr)
			}
		}
		f.buf.WriteString(")")
		if d.ReturnType != nil {
			fmt.Fprintf(&f.buf, " -> %s", formatTypeAnnotation(d.ReturnType))
		}
		f.buf.WriteString(" {\n")
		f.indent++
		if d.AppleIdentifier != "" {
			f.writeIndent()
			fmt.Fprintf(&f.buf, "identifier: %s\n", quoteStringLiteral(d.AppleIdentifier))
		}
		if d.PrimaryParam != "" {
			f.writeIndent()
			fmt.Fprintf(&f.buf, "primary: %s\n", quoteStringLiteral(d.PrimaryParam))
		}
		if len(d.Bindings) > 0 {
			f.writeIndent()
			f.buf.WriteString("bindings: {\n")
			f.indent++
			bindingKeys := make([]string, 0, len(d.Bindings))
			for k := range d.Bindings {
				bindingKeys = append(bindingKeys, k)
			}
			sort.Strings(bindingKeys)
			for _, k := range bindingKeys {
				b := d.Bindings[k]
				f.writeIndent()
				fmt.Fprintf(&f.buf, "%s: {\n", k)
				f.indent++
				if b.WireKey != "" {
					f.writeIndent()
					fmt.Fprintf(&f.buf, "key: %s\n", quoteStringLiteral(b.WireKey))
				}
				if b.Codec != "" {
					f.writeIndent()
					fmt.Fprintf(&f.buf, "codec: %s\n", quoteStringLiteral(b.Codec))
				}
				f.indent--
				f.writeIndent()
				f.buf.WriteString("},\n")
			}
			f.indent--
			f.writeIndent()
			f.buf.WriteString("}\n")
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")
	}
}

func (f *Formatter) formatStatement(stmt Statement) {
	switch s := stmt.(type) {
	case *BindingStmt:
		f.writeIndent()
		kw := "let"
		if s.Mutable {
			kw = "var"
		}
		f.buf.WriteString(kw)
		f.buf.WriteString(" ")
		f.buf.WriteString(s.Name)
		if s.TypeExpr != nil {
			fmt.Fprintf(&f.buf, ": %s", formatTypeAnnotation(s.TypeExpr))
		}
		if s.Value != nil {
			f.buf.WriteString(" = ")
			f.formatExpression(s.Value)
		}
	case *AssignStmt:
		f.writeIndent()
		opStr := "="
		switch s.Op {
		case TokenPlusAssign:
			opStr = "+="
		case TokenMinusAssign:
			opStr = "-="
		case TokenStarAssign:
			opStr = "*="
		case TokenSlashAssign:
			opStr = "/="
		}
		fmt.Fprintf(&f.buf, "%s %s ", s.Name, opStr)
		f.formatExpression(s.Value)
	case *ExprStmt:
		f.writeIndent()
		f.formatExpression(s.Expr)
	case *ReturnStmt:
		f.writeIndent()
		f.buf.WriteString("return")
		if s.Value != nil {
			f.buf.WriteString(" ")
			f.formatExpression(s.Value)
		}
	case *YieldStmt:
		f.writeIndent()
		f.buf.WriteString("yield ")
		f.formatExpression(s.Value)
	case *BlockStmt:
		f.formatBlockStmt(s, true)
	case *IfStmt:
		f.formatIfStmt(s, true)
	case *ForStmt:
		f.writeIndent()
		f.buf.WriteString("for ")
		if s.IndexVar != "" {
			fmt.Fprintf(&f.buf, "(%s, %s)", s.IndexVar, s.ItemVar)
		} else {
			f.buf.WriteString(s.ItemVar)
		}
		f.buf.WriteString(" in ")
		f.formatControlExpression(s.Iterable)
		f.buf.WriteString(" ")
		f.formatBlockStmt(s.BodyBlock, false)
	case *RepeatStmt:
		f.writeIndent()
		f.formatRepeatHeader(s)
		f.buf.WriteString(" ")
		f.formatBlockStmt(s.BodyBlock, false)
	case *MenuStmt:
		f.writeIndent()
		f.formatMenuHeader(s)
		f.buf.WriteString(" {\n")
		f.indent++
		for _, c := range s.Cases {
			f.writeIndent()
			f.formatMenuCaseHeader(c)
			f.formatBlockStmt(c.BodyBlock, false)
			f.buf.WriteString("\n")
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")
	case *CopyBlockStmt:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "copy %s ", s.Name)
		f.formatBlockStmt(s.Body, false)
	case *PasteStmt:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "paste %s", s.Name)
	}
}

func (f *Formatter) formatBlockStmt(blk *BlockStmt, withIndent bool) {
	if withIndent {
		f.writeIndent()
	}
	f.buf.WriteString("{\n")
	f.indent++
	for _, stmt := range blk.Statements {
		f.emitCommentsBefore(stmt.NodeSpan().Start.ByteOffset)
		f.formatStatement(stmt)
		f.buf.WriteString("\n")
	}
	f.emitCommentsBefore(blk.Span.End.ByteOffset)
	f.indent--
	f.writeIndent()
	f.buf.WriteString("}")
}

func (f *Formatter) formatIfStmt(s *IfStmt, withIndent bool) {
	if withIndent {
		f.writeIndent()
	}
	f.buf.WriteString("if ")
	if s.Modern {
		f.buf.WriteString("modern: ")
	}
	f.formatControlExpression(s.Condition)
	f.buf.WriteString(" ")
	f.formatBlockStmt(s.ThenBlock, false)
	if s.ElseBlock != nil {
		f.buf.WriteString(" else ")
		if elseIf, ok := s.ElseBlock.(*IfStmt); ok {
			f.formatIfStmt(elseIf, false)
		} else if elseBlk, ok := s.ElseBlock.(*BlockStmt); ok {
			f.formatBlockStmt(elseBlk, false)
		}
	}
}

func (f *Formatter) formatExpression(expr Expression) {
	switch e := expr.(type) {
	case *LiteralExpr:
		switch e.Type {
		case TokenString:
			f.buf.WriteString(quoteStringLiteral(e.Value))
		case TokenRawString:
			f.buf.WriteString(quoteRawStringLiteral(e.Value))
		default:
			f.buf.WriteString(e.Value)
		}
	case *FStringExpr:
		f.buf.WriteString("f\"")
		for _, part := range e.Parts {
			if part.IsExpr {
				f.buf.WriteString("{")
				f.formatExpression(part.Expr)
				f.buf.WriteString("}")
			} else {
				f.buf.WriteString(escapeFStringPart(part.Text))
			}
		}
		f.buf.WriteString("\"")
	case *IdentExpr:
		f.buf.WriteString(e.Name)
	case *MemberExpr:
		f.formatPostfixTarget(e.Target)
		fmt.Fprintf(&f.buf, ".%s", e.Property)
	case *EnumMemberExpr:
		if e.TypeName != "" {
			fmt.Fprintf(&f.buf, "%s.%s", e.TypeName, e.Member)
		} else {
			fmt.Fprintf(&f.buf, ".%s", e.Member)
		}
	case *IndexExpr:
		f.formatPostfixTarget(e.Target)
		f.buf.WriteString("[")
		f.formatExpression(e.Index)
		f.buf.WriteString("]")
	case *CallExpr:
		f.formatPostfixTarget(e.Callee)
		f.buf.WriteString("(")
		first := true
		if e.PrimaryArg != nil {
			f.formatExpression(e.PrimaryArg)
			first = false
		}
		for _, arg := range e.NamedArgs {
			if !first {
				f.buf.WriteString(", ")
			}
			if arg.Label != "" {
				fmt.Fprintf(&f.buf, "%s: ", arg.Label)
			}
			f.formatExpression(arg.Value)
			first = false
		}
		f.buf.WriteString(")")
		if e.Block != nil {
			f.buf.WriteString(" ")
			f.formatBlockStmt(e.Block.Body, false)
		}
	case *UnaryExpr:
		opStr := "!"
		if e.Op == TokenMinus {
			opStr = "-"
		} else if e.Op == TokenPlus {
			opStr = "+"
		}
		f.buf.WriteString(opStr)
		if _, isBin := e.Operand.(*BinaryExpr); isBin {
			f.buf.WriteString("(")
			f.formatExpression(e.Operand)
			f.buf.WriteString(")")
		} else {
			f.formatExpression(e.Operand)
		}
	case *BinaryExpr:
		f.formatBinaryOperand(e.Left, e.Op, false)
		f.buf.WriteString(" ")
		opStr := "+"
		switch e.Op {
		case TokenPlus:
			opStr = "+"
		case TokenMinus:
			opStr = "-"
		case TokenStar:
			opStr = "*"
		case TokenSlash:
			opStr = "/"
		case TokenPercent:
			opStr = "%"
		case TokenEqual:
			opStr = "=="
		case TokenNotEqual:
			opStr = "!="
		case TokenLess:
			opStr = "<"
		case TokenLessEqual:
			opStr = "<="
		case TokenGreater:
			opStr = ">"
		case TokenGreaterEqual:
			opStr = ">="
		case TokenAnd:
			opStr = "&&"
		case TokenOr:
			opStr = "||"
		case TokenContains:
			opStr = "contains"
		case TokenBeginsWith:
			opStr = "beginsWith"
		case TokenEndsWith:
			opStr = "endsWith"
		}
		f.buf.WriteString(opStr)
		f.buf.WriteString(" ")
		f.formatBinaryOperand(e.Right, e.Op, true)
	case *ListExpr:
		f.buf.WriteString("[")
		for i, el := range e.Elements {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			f.formatExpression(el)
		}
		f.buf.WriteString("]")
	case *MapExpr:
		f.buf.WriteString("{")
		for i, entry := range e.Entries {
			if i > 0 {
				f.buf.WriteString(", ")
			}
			f.formatExpression(entry.Key)
			f.buf.WriteString(": ")
			f.formatExpression(entry.Value)
		}
		f.buf.WriteString("}")
	case *IfStmt:
		f.formatIfStmt(e, false)
	case *ForStmt:
		f.buf.WriteString("for ")
		if e.IndexVar != "" {
			fmt.Fprintf(&f.buf, "(%s, %s)", e.IndexVar, e.ItemVar)
		} else {
			f.buf.WriteString(e.ItemVar)
		}
		f.buf.WriteString(" in ")
		f.formatControlExpression(e.Iterable)
		f.buf.WriteString(" ")
		f.formatBlockStmt(e.BodyBlock, false)
	case *RepeatStmt:
		f.formatRepeatHeader(e)
		f.buf.WriteString(" ")
		f.formatBlockStmt(e.BodyBlock, false)
	case *MenuStmt:
		f.formatMenuHeader(e)
		f.buf.WriteString(" {\n")
		f.indent++
		for _, c := range e.Cases {
			f.writeIndent()
			f.formatMenuCaseHeader(c)
			f.formatBlockStmt(c.BodyBlock, false)
			f.buf.WriteString("\n")
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")
	}
}

func (f *Formatter) formatRepeatHeader(s *RepeatStmt) {
	f.buf.WriteString("repeat ")
	if s.LegacyIndex && s.IndexVar != "" {
		fmt.Fprintf(&f.buf, "%s for ", s.IndexVar)
	}
	if s.Count != nil {
		f.formatControlExpression(s.Count)
	} else {
		f.buf.WriteString("none")
	}
	if !s.LegacyIndex && s.IndexVar != "" {
		fmt.Fprintf(&f.buf, " as %s", s.IndexVar)
	}
}

func (f *Formatter) formatMenuHeader(s *MenuStmt) {
	f.buf.WriteString("menu")
	if s.Prompt != nil || s.LegacyCancel {
		f.buf.WriteString("(")
		wrote := false
		if s.Prompt != nil {
			f.formatExpression(s.Prompt)
			wrote = true
		}
		if s.LegacyCancel {
			if wrote {
				f.buf.WriteString(", ")
			}
			f.buf.WriteString("legacyCancel: true")
		}
		f.buf.WriteString(")")
	}
}

func (f *Formatter) formatMenuCaseHeader(c MenuCase) {
	f.buf.WriteString("case ")
	if c.Value != nil {
		f.formatControlExpression(c.Value)
	} else {
		f.buf.WriteString(quoteStringLiteral(c.Label))
	}
	if c.HasTitle {
		fmt.Fprintf(&f.buf, " title %s", quoteStringLiteral(c.Title))
	}
	if c.Plain {
		f.buf.WriteString(" plain")
	}
	f.buf.WriteString(" ")
}

func formatTypeAnnotation(t TypeAnnotation) string {
	switch a := t.(type) {
	case *NamedTypeAnnotation:
		if a.Namespace != "" {
			return a.Namespace + "." + a.Name
		}
		return a.Name
	case *OptionalTypeAnnotation:
		return formatTypeAnnotation(a.Inner) + "?"
	case *GenericTypeAnnotation:
		var args []string
		for _, arg := range a.TypeArgs {
			args = append(args, formatTypeAnnotation(arg))
		}
		return fmt.Sprintf("%s<%s>", a.Name, strings.Join(args, ", "))
	case *UnionTypeAnnotation:
		var parts []string
		for _, m := range a.Members {
			parts = append(parts, formatTypeAnnotation(m))
		}
		return strings.Join(parts, " | ")
	case *RecordTypeAnnotation:
		var fields []string
		for _, f := range a.Fields {
			opt := ""
			if f.Optional {
				opt = "?"
			}
			fields = append(fields, fmt.Sprintf("%s%s: %s", f.Name, opt, formatTypeAnnotation(f.TypeExpr)))
		}
		return "{" + strings.Join(fields, ", ") + "}"
	default:
		return "Unknown"
	}
}

func escapeFStringPart(s string) string {
	quoted := quoteStringLiteral(s)
	return strings.NewReplacer("{", "{{", "}", "}}").Replace(quoted[1 : len(quoted)-1])
}
