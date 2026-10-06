package syntax

import (
	"github.com/electrikmilk/cherri/internal/language/source"
)

// Node is the base interface for all syntax tree nodes.
type Node interface {
	NodeSpan() source.Span
}

// Program is the root of the AST.
type Program struct {
	Span         source.Span
	Declarations []Declaration
	Statements   []Statement
}

func (p *Program) NodeSpan() source.Span { return p.Span }

// Declaration interface for top-level constructs.
type Declaration interface {
	Node
	declarationNode()
}

// Statement interface.
type Statement interface {
	Node
	statementNode()
}

// Expression interface.
type Expression interface {
	Node
	expressionNode()
}

// --- Declarations ---

type ImportDecl struct {
	Span   source.Span
	Path   string
	Alias  string
}
func (d *ImportDecl) NodeSpan() source.Span { return d.Span }
func (d *ImportDecl) declarationNode()      {}

type ShortcutDecl struct {
	Span     source.Span
	Name     string
	Metadata *RecordExpr // header properties
	Triggers []TriggerDecl
}
func (d *ShortcutDecl) NodeSpan() source.Span { return d.Span }
func (d *ShortcutDecl) declarationNode()      {}

type TriggerDecl struct {
	Span   source.Span
	Family string
	Event  string
}
func (d *TriggerDecl) NodeSpan() source.Span { return d.Span }

type SetupDecl struct {
	Span         source.Span
	Name         string
	TypeExpr     TypeAnnotation
	Prompt       string
	DefaultValue string
}
func (d *SetupDecl) NodeSpan() source.Span { return d.Span }
func (d *SetupDecl) declarationNode()      {}

type TypeDecl struct {
	Span     source.Span
	Name     string
	TypeExpr TypeAnnotation
}
func (d *TypeDecl) NodeSpan() source.Span { return d.Span }
func (d *TypeDecl) declarationNode()      {}

type EnumDecl struct {
	Span    source.Span
	Name    string
	Members []EnumMember
}
func (d *EnumDecl) NodeSpan() source.Span { return d.Span }
func (d *EnumDecl) declarationNode()      {}

type EnumMember struct {
	Span      source.Span
	Name      string
	WireValue string
}

type ParameterDecl struct {
	Span         source.Span
	Name         string
	Optional     bool
	TypeExpr     TypeAnnotation
	DefaultExpr  Expression
}

type FunctionDecl struct {
	Span       source.Span
	Name       string
	Parameters []ParameterDecl
	ReturnType TypeAnnotation
	Body       *BlockStmt
}
func (d *FunctionDecl) NodeSpan() source.Span { return d.Span }
func (d *FunctionDecl) declarationNode()      {}

type ActionDecl struct {
	Span            source.Span
	Name            string
	Parameters      []ParameterDecl
	ReturnType      TypeAnnotation
	AppleIdentifier string
	PrimaryParam    string
	Bindings        map[string]ActionBinding
}
func (d *ActionDecl) NodeSpan() source.Span { return d.Span }
func (d *ActionDecl) declarationNode()      {}

type ActionBinding struct {
	WireKey string
	Codec   string
}

// --- Type Annotations in AST ---

type TypeAnnotation interface {
	Node
	typeAnnotationNode()
}

type NamedTypeAnnotation struct {
	Span      source.Span
	Namespace string
	Name      string
}
func (t *NamedTypeAnnotation) NodeSpan() source.Span { return t.Span }
func (t *NamedTypeAnnotation) typeAnnotationNode()  {}

type OptionalTypeAnnotation struct {
	Span  source.Span
	Inner TypeAnnotation
}
func (t *OptionalTypeAnnotation) NodeSpan() source.Span { return t.Span }
func (t *OptionalTypeAnnotation) typeAnnotationNode()  {}

type GenericTypeAnnotation struct {
	Span     source.Span
	Name     string
	TypeArgs []TypeAnnotation
}
func (t *GenericTypeAnnotation) NodeSpan() source.Span { return t.Span }
func (t *GenericTypeAnnotation) typeAnnotationNode()  {}

type UnionTypeAnnotation struct {
	Span    source.Span
	Members []TypeAnnotation
}
func (t *UnionTypeAnnotation) NodeSpan() source.Span { return t.Span }
func (t *UnionTypeAnnotation) typeAnnotationNode()  {}

type RecordTypeAnnotation struct {
	Span   source.Span
	Fields []RecordTypeField
}
func (t *RecordTypeAnnotation) NodeSpan() source.Span { return t.Span }
func (t *RecordTypeAnnotation) typeAnnotationNode()  {}

type RecordTypeField struct {
	Name     string
	Optional bool
	TypeExpr TypeAnnotation
}

// --- Statements ---

type BindingStmt struct {
	Span     source.Span
	Mutable  bool // false = let, true = var
	Name     string
	TypeExpr TypeAnnotation
	Value    Expression
}
func (s *BindingStmt) NodeSpan() source.Span { return s.Span }
func (s *BindingStmt) statementNode()        {}

type AssignStmt struct {
	Span  source.Span
	Name  string
	Op    TokenType // =, +=, -=, *=, /=
	Value Expression
}
func (s *AssignStmt) NodeSpan() source.Span { return s.Span }
func (s *AssignStmt) statementNode()        {}

type ExprStmt struct {
	Span source.Span
	Expr Expression
}
func (s *ExprStmt) NodeSpan() source.Span { return s.Span }
func (s *ExprStmt) statementNode()        {}

type ReturnStmt struct {
	Span  source.Span
	Value Expression
}
func (s *ReturnStmt) NodeSpan() source.Span { return s.Span }
func (s *ReturnStmt) statementNode()        {}

type YieldStmt struct {
	Span  source.Span
	Value Expression
}
func (s *YieldStmt) NodeSpan() source.Span { return s.Span }
func (s *YieldStmt) statementNode()        {}

type BlockStmt struct {
	Span       source.Span
	Statements []Statement
}
func (s *BlockStmt) NodeSpan() source.Span { return s.Span }
func (s *BlockStmt) statementNode()        {}

type IfStmt struct {
	Span      source.Span
	Condition Expression
	ThenBlock *BlockStmt
	ElseBlock Statement // *BlockStmt or *IfStmt
}
func (s *IfStmt) NodeSpan() source.Span { return s.Span }
func (s *IfStmt) statementNode()        {}
func (s *IfStmt) expressionNode()       {}

type ForStmt struct {
	Span      source.Span
	IndexVar  string
	ItemVar   string
	Iterable  Expression
	BodyBlock *BlockStmt
}
func (s *ForStmt) NodeSpan() source.Span { return s.Span }
func (s *ForStmt) statementNode()        {}
func (s *ForStmt) expressionNode()       {}

type RepeatStmt struct {
	Span      source.Span
	Count     Expression
	IndexVar  string
	BodyBlock *BlockStmt
}
func (s *RepeatStmt) NodeSpan() source.Span { return s.Span }
func (s *RepeatStmt) statementNode()        {}
func (s *RepeatStmt) expressionNode()       {}

type MenuStmt struct {
	Span       source.Span
	Prompt     Expression
	Cases      []MenuCase
}
func (s *MenuStmt) NodeSpan() source.Span { return s.Span }
func (s *MenuStmt) statementNode()        {}
func (s *MenuStmt) expressionNode()       {}

type MenuCase struct {
	Span      source.Span
	Label     string
	BodyBlock *BlockStmt
}

// --- Expressions ---

type LiteralExpr struct {
	Span  source.Span
	Type  TokenType // TokenString, TokenRawString, TokenNumber, TokenTrue, TokenFalse, TokenNone, TokenNull
	Value string
}
func (e *LiteralExpr) NodeSpan() source.Span { return e.Span }
func (e *LiteralExpr) expressionNode()       {}

type FStringPart struct {
	IsExpr bool
	Text   string
	Expr   Expression
}

type FStringExpr struct {
	Span  source.Span
	Parts []FStringPart
}
func (e *FStringExpr) NodeSpan() source.Span { return e.Span }
func (e *FStringExpr) expressionNode()       {}

type IdentExpr struct {
	Span source.Span
	Name string
}
func (e *IdentExpr) NodeSpan() source.Span { return e.Span }
func (e *IdentExpr) expressionNode()       {}

type MemberExpr struct {
	Span     source.Span
	Target   Expression
	Property string
}
func (e *MemberExpr) NodeSpan() source.Span { return e.Span }
func (e *MemberExpr) expressionNode()       {}

type IndexExpr struct {
	Span   source.Span
	Target Expression
	Index  Expression
}
func (e *IndexExpr) NodeSpan() source.Span { return e.Span }
func (e *IndexExpr) expressionNode()       {}

type NamedArg struct {
	Span  source.Span
	Label string
	Value Expression
}

type CallExpr struct {
	Span         source.Span
	Callee       Expression
	PrimaryArg   Expression // Optional single unlabeled argument
	NamedArgs    []NamedArg // Labeled arguments
}
func (e *CallExpr) NodeSpan() source.Span { return e.Span }
func (e *CallExpr) expressionNode()       {}

type UnaryExpr struct {
	Span     source.Span
	Op       TokenType
	Operand  Expression
}
func (e *UnaryExpr) NodeSpan() source.Span { return e.Span }
func (e *UnaryExpr) expressionNode()       {}

type BinaryExpr struct {
	Span  source.Span
	Left  Expression
	Op    TokenType
	Right Expression
}
func (e *BinaryExpr) NodeSpan() source.Span { return e.Span }
func (e *BinaryExpr) expressionNode()       {}

type ListExpr struct {
	Span     source.Span
	Elements []Expression
}
func (e *ListExpr) NodeSpan() source.Span { return e.Span }
func (e *ListExpr) expressionNode()       {}

type MapEntry struct {
	Key   Expression
	Value Expression
}

type MapExpr struct {
	Span    source.Span
	Entries []MapEntry
}
func (e *MapExpr) NodeSpan() source.Span { return e.Span }
func (e *MapExpr) expressionNode()       {}

type RecordFieldExpr struct {
	Name  string
	Value Expression
}

type RecordExpr struct {
	Span   source.Span
	Fields []RecordFieldExpr
}
func (e *RecordExpr) NodeSpan() source.Span { return e.Span }
func (e *RecordExpr) expressionNode()       {}

type EnumMemberExpr struct {
	Span     source.Span
	TypeName string
	Member   string
}
func (e *EnumMemberExpr) NodeSpan() source.Span { return e.Span }
func (e *EnumMemberExpr) expressionNode()       {}
