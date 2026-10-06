package syntax

import (
	"fmt"
	"strings"
)

// Formatter formats a Cherri v2 AST into canonical Cherri source code.
type Formatter struct {
	indent int
	buf    strings.Builder
}

// NewFormatter creates a new Formatter.
func NewFormatter() *Formatter {
	return &Formatter{}
}

// Format formats a Program into canonical source text.
func Format(prog *Program) string {
	f := NewFormatter()
	f.formatProgram(prog)
	return f.buf.String()
}

func (f *Formatter) writeIndent() {
	for i := 0; i < f.indent; i++ {
		f.buf.WriteString("    ")
	}
}

func (f *Formatter) formatProgram(prog *Program) {
	for i, decl := range prog.Declarations {
		if i > 0 {
			f.buf.WriteString("\n")
		}
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
		f.formatStatement(stmt)
		f.buf.WriteString("\n")
	}
}

func (f *Formatter) formatDeclaration(decl Declaration) {
	switch d := decl.(type) {
	case *ImportDecl:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "import %q as %s", d.Path, d.Alias)
	case *ShortcutDecl:
		f.writeIndent()
		fmt.Fprintf(&f.buf, "shortcut %q {\n", d.Name)
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
			if trig.Event != "" {
				fmt.Fprintf(&f.buf, "trigger %s(event: .%s)\n", trig.Family, trig.Event)
			} else {
				fmt.Fprintf(&f.buf, "trigger %s\n", trig.Family)
			}
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")
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
			fmt.Fprintf(&f.buf, "prompt: %q\n", d.Prompt)
		}
		if d.DefaultValue != "" {
			f.writeIndent()
			fmt.Fprintf(&f.buf, "default: %q\n", d.DefaultValue)
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
			if m.WireValue != "" && m.WireValue != m.Name {
				fmt.Fprintf(&f.buf, "%s = %q", m.Name, m.WireValue)
			} else {
				f.buf.WriteString(m.Name)
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
		}
		f.buf.WriteString(")")
		if d.ReturnType != nil {
			fmt.Fprintf(&f.buf, " -> %s", formatTypeAnnotation(d.ReturnType))
		}
		f.buf.WriteString(" {\n")
		f.indent++
		if d.AppleIdentifier != "" {
			f.writeIndent()
			fmt.Fprintf(&f.buf, "identifier: %q\n", d.AppleIdentifier)
		}
		if d.PrimaryParam != "" {
			f.writeIndent()
			fmt.Fprintf(&f.buf, "primary: %q\n", d.PrimaryParam)
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
		f.formatExpression(s.Iterable)
		f.buf.WriteString(" ")
		f.formatBlockStmt(s.BodyBlock, false)
	case *RepeatStmt:
		f.writeIndent()
		f.buf.WriteString("repeat ")
		f.formatExpression(s.Count)
		if s.IndexVar != "" {
			fmt.Fprintf(&f.buf, " as %s", s.IndexVar)
		}
		f.buf.WriteString(" ")
		f.formatBlockStmt(s.BodyBlock, false)
	case *MenuStmt:
		f.writeIndent()
		f.buf.WriteString("menu(")
		f.formatExpression(s.Prompt)
		f.buf.WriteString(") {\n")
		f.indent++
		for _, c := range s.Cases {
			f.writeIndent()
			fmt.Fprintf(&f.buf, "case %q ", c.Label)
			f.formatBlockStmt(c.BodyBlock, false)
			f.buf.WriteString("\n")
		}
		f.indent--
		f.writeIndent()
		f.buf.WriteString("}")
	}
}

func (f *Formatter) formatBlockStmt(blk *BlockStmt, withIndent bool) {
	if withIndent {
		f.writeIndent()
	}
	f.buf.WriteString("{\n")
	f.indent++
	for _, stmt := range blk.Statements {
		f.formatStatement(stmt)
		f.buf.WriteString("\n")
	}
	f.indent--
	f.writeIndent()
	f.buf.WriteString("}")
}

func (f *Formatter) formatIfStmt(s *IfStmt, withIndent bool) {
	if withIndent {
		f.writeIndent()
	}
	f.buf.WriteString("if ")
	f.formatExpression(s.Condition)
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
			fmt.Fprintf(&f.buf, "%q", e.Value)
		case TokenRawString:
			fmt.Fprintf(&f.buf, "r%q", e.Value)
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
				f.buf.WriteString(part.Text)
			}
		}
		f.buf.WriteString("\"")
	case *IdentExpr:
		f.buf.WriteString(e.Name)
	case *MemberExpr:
		f.formatExpression(e.Target)
		fmt.Fprintf(&f.buf, ".%s", e.Property)
	case *EnumMemberExpr:
		if e.TypeName != "" {
			fmt.Fprintf(&f.buf, "%s.%s", e.TypeName, e.Member)
		} else {
			fmt.Fprintf(&f.buf, ".%s", e.Member)
		}
	case *IndexExpr:
		f.formatExpression(e.Target)
		f.buf.WriteString("[")
		f.formatExpression(e.Index)
		f.buf.WriteString("]")
	case *CallExpr:
		f.formatExpression(e.Callee)
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
	case *UnaryExpr:
		opStr := "!"
		if e.Op == TokenMinus {
			opStr = "-"
		} else if e.Op == TokenPlus {
			opStr = "+"
		}
		f.buf.WriteString(opStr)
		f.formatExpression(e.Operand)
	case *BinaryExpr:
		f.formatExpression(e.Left)
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
		}
		f.buf.WriteString(opStr)
		f.buf.WriteString(" ")
		f.formatExpression(e.Right)
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
	}
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
