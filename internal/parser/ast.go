package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aria-lang/aria/internal/lexer"
)

// Pos is a source position.
type Pos = lexer.Position

// ---------- Program ----------

// Program is the root AST node for a source file.
type Program struct {
	Module  *ModDecl
	Imports []*ImportDecl
	Decls   []Decl
	Pos     Pos
}

// ---------- Declarations ----------

// Decl is the interface for all declaration nodes.
type Decl interface {
	declNode()
	GetPos() Pos
}

type ModDecl struct {
	Name string
	Pos  Pos
}

func (d *ModDecl) declNode() {}
func (d *ModDecl) GetPos() Pos { return d.Pos }

type ImportDecl struct {
	Path    []string // e.g., ["std", "fs"]
	Names   []string // grouped imports: ["json", "http"] for use std.{json, http}
	Alias   string   // alias for "as" imports
	Pos     Pos
}

func (d *ImportDecl) declNode() {}
func (d *ImportDecl) GetPos() Pos { return d.Pos }

// Visibility represents pub, pub(pkg), or private (empty).
type Visibility int

const (
	Private Visibility = iota
	Public
	PackagePublic
)

type FnDecl struct {
	Vis        Visibility
	Name       string
	GenericParams []*GenericParam
	Params     []*Param
	ReturnType TypeExpr
	ErrorTypes []TypeExpr
	Effects    []string
	WhereClause []*WhereItem
	Body       Expr // BlockExpr or single expression
	Pos        Pos
}

func (d *FnDecl) declNode() {}
func (d *FnDecl) GetPos() Pos { return d.Pos }

type GenericParam struct {
	Name   string
	Bounds []string // trait bounds: Eq, Hash, etc.
	Pos    Pos
}

type Param struct {
	Mutable bool
	Name    string
	Pattern Pattern // for pattern params; nil if simple name
	Type    TypeExpr
	Default Expr // nil if no default
	Pos     Pos
}

// TypeDecl covers structs, sum types, newtypes.
type TypeDecl struct {
	Vis        Visibility
	Name       string
	GenericParams []*GenericParam
	Kind       TypeDeclKind
	Fields     []*FieldDecl   // for struct kinds
	Variants   []*VariantDecl // for sum type kinds
	Underlying TypeExpr       // for newtype kinds
	Derives    []string
	Pos        Pos
}

type TypeDeclKind int

const (
	StructDecl  TypeDeclKind = iota
	SumTypeDecl
	NewtypeDecl
)

func (d *TypeDecl) declNode() {}
func (d *TypeDecl) GetPos() Pos { return d.Pos }

type FieldDecl struct {
	Vis     Visibility
	Name    string
	Type    TypeExpr
	Default Expr
	Pos     Pos
}

type VariantDecl struct {
	Name   string
	Fields []*FieldDecl // struct variant
	Types  []TypeExpr   // tuple variant
	Pos    Pos
}

type EnumDecl struct {
	Vis     Visibility
	Name    string
	Members []string
	Pos     Pos
}

func (d *EnumDecl) declNode() {}
func (d *EnumDecl) GetPos() Pos { return d.Pos }

type AliasDecl struct {
	Vis    Visibility
	Name   string
	Target TypeExpr
	Pos    Pos
}

func (d *AliasDecl) declNode() {}
func (d *AliasDecl) GetPos() Pos { return d.Pos }

type TraitDecl struct {
	Vis        Visibility
	Name       string
	GenericParams []*GenericParam
	Supertraits []string
	Methods    []*FnDecl
	Pos        Pos
}

func (d *TraitDecl) declNode() {}
func (d *TraitDecl) GetPos() Pos { return d.Pos }

type ImplDecl struct {
	GenericParams []*GenericParam
	TraitName     string   // empty for inherent impls
	TypeName      string
	TypeArgs      []TypeExpr
	WhereClause   []*WhereItem
	Methods       []*FnDecl
	Pos           Pos
}

func (d *ImplDecl) declNode() {}
func (d *ImplDecl) GetPos() Pos { return d.Pos }

type WhereItem struct {
	Name   string
	Bounds []string
	Pos    Pos
}

type ConstDecl struct {
	Vis   Visibility
	Name  string
	Type  TypeExpr // nil if inferred
	Value Expr
	Pos   Pos
}

func (d *ConstDecl) declNode() {}
func (d *ConstDecl) GetPos() Pos { return d.Pos }

type EntryBlock struct {
	Body *BlockExpr
	Pos  Pos
}

func (d *EntryBlock) declNode() {}
func (d *EntryBlock) GetPos() Pos { return d.Pos }

type TestBlock struct {
	Name string
	Body *BlockExpr
	Pos  Pos
}

func (d *TestBlock) declNode() {}
func (d *TestBlock) GetPos() Pos { return d.Pos }

// ---------- Expressions ----------

// Expr is the interface for all expression nodes.
type Expr interface {
	exprNode()
	GetPos() Pos
}

// Literal expressions

type IntLitExpr struct {
	Value string
	Pos   Pos
}
func (e *IntLitExpr) exprNode() {}
func (e *IntLitExpr) GetPos() Pos { return e.Pos }

type FloatLitExpr struct {
	Value string
	Pos   Pos
}
func (e *FloatLitExpr) exprNode() {}
func (e *FloatLitExpr) GetPos() Pos { return e.Pos }

type StringLitExpr struct {
	Value string
	Pos   Pos
}
func (e *StringLitExpr) exprNode() {}
func (e *StringLitExpr) GetPos() Pos { return e.Pos }

type BoolLitExpr struct {
	Value bool
	Pos   Pos
}
func (e *BoolLitExpr) exprNode() {}
func (e *BoolLitExpr) GetPos() Pos { return e.Pos }

type InterpolatedStringExpr struct {
	Parts []Expr // alternating StringLitExpr and arbitrary expressions
	Pos   Pos
}
func (e *InterpolatedStringExpr) exprNode() {}
func (e *InterpolatedStringExpr) GetPos() Pos { return e.Pos }

// Identifier and path

type IdentExpr struct {
	Name string
	Pos  Pos
}
func (e *IdentExpr) exprNode() {}
func (e *IdentExpr) GetPos() Pos { return e.Pos }

type PathExpr struct {
	Parts []string // ["std", "fs", "read"]
	Pos   Pos
}
func (e *PathExpr) exprNode() {}
func (e *PathExpr) GetPos() Pos { return e.Pos }

// Binary/unary/postfix

type BinaryExpr struct {
	Op    lexer.TokenType
	Left  Expr
	Right Expr
	Pos   Pos
}
func (e *BinaryExpr) exprNode() {}
func (e *BinaryExpr) GetPos() Pos { return e.Pos }

type UnaryExpr struct {
	Op      lexer.TokenType
	Operand Expr
	Pos     Pos
}
func (e *UnaryExpr) exprNode() {}
func (e *UnaryExpr) GetPos() Pos { return e.Pos }

type PostfixExpr struct {
	Op      lexer.TokenType // Question or Bang
	Operand Expr
	Pos     Pos
}
func (e *PostfixExpr) exprNode() {}
func (e *PostfixExpr) GetPos() Pos { return e.Pos }

// Access expressions

type FieldAccessExpr struct {
	Object Expr
	Field  string
	Pos    Pos
}
func (e *FieldAccessExpr) exprNode() {}
func (e *FieldAccessExpr) GetPos() Pos { return e.Pos }

type OptionalChainExpr struct {
	Object Expr
	Field  string
	Pos    Pos
}
func (e *OptionalChainExpr) exprNode() {}
func (e *OptionalChainExpr) GetPos() Pos { return e.Pos }

type IndexExpr struct {
	Object Expr
	Index  Expr
	Pos    Pos
}
func (e *IndexExpr) exprNode() {}
func (e *IndexExpr) GetPos() Pos { return e.Pos }

type CallExpr struct {
	Func Expr
	Args []*Arg
	Pos  Pos
}
func (e *CallExpr) exprNode() {}
func (e *CallExpr) GetPos() Pos { return e.Pos }

type Arg struct {
	Name  string // empty for positional args
	Value Expr
	Pos   Pos
}

type MethodCallExpr struct {
	Object Expr
	Method string
	Args   []*Arg
	Pos    Pos
}
func (e *MethodCallExpr) exprNode() {}
func (e *MethodCallExpr) GetPos() Pos { return e.Pos }

type RecordUpdateExpr struct {
	Object Expr
	Fields []*FieldInit
	Pos    Pos
}
func (e *RecordUpdateExpr) exprNode() {}
func (e *RecordUpdateExpr) GetPos() Pos { return e.Pos }

// Pipeline

type PipelineExpr struct {
	Left  Expr
	Right Expr // the function/target
	Pos   Pos
}
func (e *PipelineExpr) exprNode() {}
func (e *PipelineExpr) GetPos() Pos { return e.Pos }

// Field shorthand in pipeline: .fieldName
type FieldShorthandExpr struct {
	Field string
	Pos   Pos
}
func (e *FieldShorthandExpr) exprNode() {}
func (e *FieldShorthandExpr) GetPos() Pos { return e.Pos }

// Range

type RangeExpr struct {
	Start     Expr
	End       Expr
	Inclusive bool // ..= vs ..
	Pos       Pos
}
func (e *RangeExpr) exprNode() {}
func (e *RangeExpr) GetPos() Pos { return e.Pos }

// Compound expressions

type BlockExpr struct {
	Stmts []Stmt
	Expr  Expr // trailing expression (may be nil)
	Pos   Pos
}
func (e *BlockExpr) exprNode() {}
func (e *BlockExpr) GetPos() Pos { return e.Pos }

type IfExpr struct {
	Cond Expr
	Then *BlockExpr
	Else Expr // *BlockExpr or *IfExpr for else-if chains, or nil
	Pos  Pos
}
func (e *IfExpr) exprNode() {}
func (e *IfExpr) GetPos() Pos { return e.Pos }

type MatchExpr struct {
	Subject Expr
	Arms    []*MatchArm
	Pos     Pos
}
func (e *MatchExpr) exprNode() {}
func (e *MatchExpr) GetPos() Pos { return e.Pos }

type MatchArm struct {
	Pattern Pattern
	Guard   Expr // nil if no guard
	Body    Expr
	Pos     Pos
}

type ClosureExpr struct {
	Move   bool
	Params []*Param
	Return TypeExpr // nil if not annotated
	Body   Expr
	Pos    Pos
}
func (e *ClosureExpr) exprNode() {}
func (e *ClosureExpr) GetPos() Pos { return e.Pos }

type StructExpr struct {
	TypeName string
	TypeArgs []TypeExpr
	Fields   []*FieldInit
	Pos      Pos
}
func (e *StructExpr) exprNode() {}
func (e *StructExpr) GetPos() Pos { return e.Pos }

type FieldInit struct {
	Name  string
	Value Expr // nil for shorthand (name == variable name)
	Pos   Pos
}

type ArrayExpr struct {
	Elements []Expr
	Pos      Pos
}
func (e *ArrayExpr) exprNode() {}
func (e *ArrayExpr) GetPos() Pos { return e.Pos }

type MapExpr struct {
	Entries []*MapEntry
	Pos     Pos
}
func (e *MapExpr) exprNode() {}
func (e *MapExpr) GetPos() Pos { return e.Pos }

type MapEntry struct {
	Key   Expr
	Value Expr
	Pos   Pos
}

type TupleExpr struct {
	Elements []Expr
	Pos      Pos
}
func (e *TupleExpr) exprNode() {}
func (e *TupleExpr) GetPos() Pos { return e.Pos }

type ListCompExpr struct {
	Expr  Expr
	Var   string
	Iter  Expr
	Where Expr // nil if no where clause
	Pos   Pos
}
func (e *ListCompExpr) exprNode() {}
func (e *ListCompExpr) GetPos() Pos { return e.Pos }

type GroupExpr struct {
	Inner Expr
	Pos   Pos
}
func (e *GroupExpr) exprNode() {}
func (e *GroupExpr) GetPos() Pos { return e.Pos }

// CatchExpr represents expr catch |err| { ... } or expr catch { Pat => ... }
type CatchExpr struct {
	Expr     Expr
	ErrName  string // for catch |err| form
	Body     Expr
	Pos      Pos
}
func (e *CatchExpr) exprNode() {}
func (e *CatchExpr) GetPos() Pos { return e.Pos }

// ---------- Statements ----------

// Stmt is the interface for all statement nodes.
type Stmt interface {
	stmtNode()
	GetPos() Pos
}

type VarDeclStmt struct {
	Mutable bool
	Name    string
	Pattern Pattern  // nil if simple name binding
	Type    TypeExpr // nil if inferred
	Value   Expr
	Pos     Pos
}
func (s *VarDeclStmt) stmtNode() {}
func (s *VarDeclStmt) GetPos() Pos { return s.Pos }

type AssignStmt struct {
	Target Expr
	Value  Expr
	Pos    Pos
}
func (s *AssignStmt) stmtNode() {}
func (s *AssignStmt) GetPos() Pos { return s.Pos }

type ExprStmt struct {
	Expr Expr
	Pos  Pos
}
func (s *ExprStmt) stmtNode() {}
func (s *ExprStmt) GetPos() Pos { return s.Pos }

type ForStmt struct {
	Pattern Pattern
	Iter    Expr
	Where   Expr // nil if no where clause
	Body    *BlockExpr
	Pos     Pos
}
func (s *ForStmt) stmtNode() {}
func (s *ForStmt) GetPos() Pos { return s.Pos }

type WhileStmt struct {
	Cond Expr
	Body *BlockExpr
	Pos  Pos
}
func (s *WhileStmt) stmtNode() {}
func (s *WhileStmt) GetPos() Pos { return s.Pos }

type LoopStmt struct {
	Body *BlockExpr
	Pos  Pos
}
func (s *LoopStmt) stmtNode() {}
func (s *LoopStmt) GetPos() Pos { return s.Pos }

type ReturnStmt struct {
	Value Expr // nil for bare return
	Pos   Pos
}
func (s *ReturnStmt) stmtNode() {}
func (s *ReturnStmt) GetPos() Pos { return s.Pos }

type BreakStmt struct {
	Value Expr // nil for bare break
	Pos   Pos
}
func (s *BreakStmt) stmtNode() {}
func (s *BreakStmt) GetPos() Pos { return s.Pos }

type ContinueStmt struct {
	Pos Pos
}
func (s *ContinueStmt) stmtNode() {}
func (s *ContinueStmt) GetPos() Pos { return s.Pos }

type DeferStmt struct {
	Expr Expr
	Pos  Pos
}
func (s *DeferStmt) stmtNode() {}
func (s *DeferStmt) GetPos() Pos { return s.Pos }

// ---------- Patterns ----------

// Pattern is the interface for all pattern nodes.
type Pattern interface {
	patternNode()
	GetPos() Pos
}

type WildcardPattern struct {
	Pos Pos
}
func (p *WildcardPattern) patternNode() {}
func (p *WildcardPattern) GetPos() Pos { return p.Pos }

type BindingPattern struct {
	Mutable bool
	Name    string
	Pos     Pos
}
func (p *BindingPattern) patternNode() {}
func (p *BindingPattern) GetPos() Pos { return p.Pos }

type LiteralPattern struct {
	Value Expr // IntLitExpr, FloatLitExpr, StringLitExpr, BoolLitExpr
	Pos   Pos
}
func (p *LiteralPattern) patternNode() {}
func (p *LiteralPattern) GetPos() Pos { return p.Pos }

type VariantPattern struct {
	Name    string
	Args    []Pattern
	Pos     Pos
}
func (p *VariantPattern) patternNode() {}
func (p *VariantPattern) GetPos() Pos { return p.Pos }

type StructPattern struct {
	TypeName string
	Fields   []*FieldPattern
	Rest     bool // has ".." to ignore remaining
	Pos      Pos
}
func (p *StructPattern) patternNode() {}
func (p *StructPattern) GetPos() Pos { return p.Pos }

type FieldPattern struct {
	Name    string
	Pattern Pattern // nil for shorthand (bind field name directly)
	Pos     Pos
}

type TuplePattern struct {
	Elements []Pattern
	Pos      Pos
}
func (p *TuplePattern) patternNode() {}
func (p *TuplePattern) GetPos() Pos { return p.Pos }

type ArrayPattern struct {
	Elements []Pattern
	Rest     string // name after ".." or empty
	HasRest  bool
	Pos      Pos
}
func (p *ArrayPattern) patternNode() {}
func (p *ArrayPattern) GetPos() Pos { return p.Pos }

type OrPattern struct {
	Left  Pattern
	Right Pattern
	Pos   Pos
}
func (p *OrPattern) patternNode() {}
func (p *OrPattern) GetPos() Pos { return p.Pos }

type RestPattern struct {
	Name string // empty for bare ".."
	Pos  Pos
}
func (p *RestPattern) patternNode() {}
func (p *RestPattern) GetPos() Pos { return p.Pos }

type NamedPattern struct {
	Name    string
	Pattern Pattern
	Pos     Pos
}
func (p *NamedPattern) patternNode() {}
func (p *NamedPattern) GetPos() Pos { return p.Pos }

// ---------- Type Expressions ----------

// TypeExpr is the interface for all type annotation nodes.
type TypeExpr interface {
	typeExprNode()
	GetPos() Pos
}

type NamedTypeExpr struct {
	Path     []string   // ["std", "io", "Reader"] or just ["i64"]
	TypeArgs []TypeExpr // generic args: Map[str, i64] -> TypeArgs = [str, i64]
	Pos      Pos
}
func (t *NamedTypeExpr) typeExprNode() {}
func (t *NamedTypeExpr) GetPos() Pos { return t.Pos }

type FunctionTypeExpr struct {
	Params []TypeExpr
	Return TypeExpr
	Pos    Pos
}
func (t *FunctionTypeExpr) typeExprNode() {}
func (t *FunctionTypeExpr) GetPos() Pos { return t.Pos }

type TupleTypeExpr struct {
	Elements []TypeExpr
	Pos      Pos
}
func (t *TupleTypeExpr) typeExprNode() {}
func (t *TupleTypeExpr) GetPos() Pos { return t.Pos }

type ArrayTypeExpr struct {
	Element TypeExpr
	Pos     Pos
}
func (t *ArrayTypeExpr) typeExprNode() {}
func (t *ArrayTypeExpr) GetPos() Pos { return t.Pos }

type MapTypeExpr struct {
	Key   TypeExpr
	Value TypeExpr
	Pos   Pos
}
func (t *MapTypeExpr) typeExprNode() {}
func (t *MapTypeExpr) GetPos() Pos { return t.Pos }

type SetTypeExpr struct {
	Element TypeExpr
	Pos     Pos
}
func (t *SetTypeExpr) typeExprNode() {}
func (t *SetTypeExpr) GetPos() Pos { return t.Pos }

type OptionalTypeExpr struct {
	Inner TypeExpr
	Pos   Pos
}
func (t *OptionalTypeExpr) typeExprNode() {}
func (t *OptionalTypeExpr) GetPos() Pos { return t.Pos }

type ResultTypeExpr struct {
	Ok  TypeExpr
	Err TypeExpr
	Pos Pos
}
func (t *ResultTypeExpr) typeExprNode() {}
func (t *ResultTypeExpr) GetPos() Pos { return t.Pos }

// ---------- AST Serialization ----------

// FormatAST returns a human-readable indented representation of the AST.
func FormatAST(prog *Program) string {
	var sb strings.Builder
	formatProgram(&sb, prog, 0)
	return sb.String()
}

func indent(sb *strings.Builder, level int) {
	for i := 0; i < level; i++ {
		sb.WriteString("  ")
	}
}

func formatProgram(sb *strings.Builder, prog *Program, level int) {
	indent(sb, level)
	fmt.Fprintf(sb, "(program\n")
	if prog.Module != nil {
		indent(sb, level+1)
		fmt.Fprintf(sb, "(mod %s)\n", prog.Module.Name)
	}
	for _, imp := range prog.Imports {
		indent(sb, level+1)
		path := strings.Join(imp.Path, ".")
		if len(imp.Names) > 0 {
			fmt.Fprintf(sb, "(use %s.{%s})\n", path, strings.Join(imp.Names, ", "))
		} else if imp.Alias != "" {
			fmt.Fprintf(sb, "(use %s as %s)\n", path, imp.Alias)
		} else {
			fmt.Fprintf(sb, "(use %s)\n", path)
		}
	}
	for _, decl := range prog.Decls {
		formatDecl(sb, decl, level+1)
	}
	indent(sb, level)
	sb.WriteString(")\n")
}

func formatDecl(sb *strings.Builder, decl Decl, level int) {
	switch d := decl.(type) {
	case *FnDecl:
		indent(sb, level)
		fmt.Fprintf(sb, "(fn %s", d.Name)
		if len(d.GenericParams) > 0 {
			sb.WriteString("[")
			for i, gp := range d.GenericParams {
				if i > 0 { sb.WriteString(", ") }
				sb.WriteString(gp.Name)
				if len(gp.Bounds) > 0 {
					sb.WriteString(": ")
					sb.WriteString(strings.Join(gp.Bounds, " + "))
				}
			}
			sb.WriteString("]")
		}
		sb.WriteString("(")
		for i, p := range d.Params {
			if i > 0 { sb.WriteString(", ") }
			if p.Mutable { sb.WriteString("mut ") }
			sb.WriteString(p.Name)
			if p.Type != nil {
				sb.WriteString(": ")
				formatTypeExpr(sb, p.Type)
			}
		}
		sb.WriteString(")")
		if d.ReturnType != nil {
			sb.WriteString(" -> ")
			formatTypeExpr(sb, d.ReturnType)
		}
		if len(d.ErrorTypes) > 0 {
			sb.WriteString(" !")
			for i, et := range d.ErrorTypes {
				if i > 0 { sb.WriteString(" | ") }
				sb.WriteString(" ")
				formatTypeExpr(sb, et)
			}
		}
		if d.Body != nil {
			sb.WriteString("\n")
			formatExpr(sb, d.Body, level+1)
		} else {
			sb.WriteString(")\n")
		}

	case *TypeDecl:
		indent(sb, level)
		switch d.Kind {
		case StructDecl:
			fmt.Fprintf(sb, "(struct %s", d.Name)
			for _, f := range d.Fields {
				sb.WriteString("\n")
				indent(sb, level+1)
				fmt.Fprintf(sb, "(%s: ", f.Name)
				formatTypeExpr(sb, f.Type)
				sb.WriteString(")")
			}
			sb.WriteString(")\n")
		case SumTypeDecl:
			fmt.Fprintf(sb, "(sum-type %s", d.Name)
			for _, v := range d.Variants {
				sb.WriteString("\n")
				indent(sb, level+1)
				fmt.Fprintf(sb, "(variant %s", v.Name)
				for _, t := range v.Types {
					sb.WriteString(" ")
					formatTypeExpr(sb, t)
				}
				sb.WriteString(")")
			}
			sb.WriteString(")\n")
		case NewtypeDecl:
			fmt.Fprintf(sb, "(newtype %s = ", d.Name)
			formatTypeExpr(sb, d.Underlying)
			sb.WriteString(")\n")
		}

	case *EnumDecl:
		indent(sb, level)
		fmt.Fprintf(sb, "(enum %s %s)\n", d.Name, strings.Join(d.Members, " "))

	case *AliasDecl:
		indent(sb, level)
		fmt.Fprintf(sb, "(alias %s = ", d.Name)
		formatTypeExpr(sb, d.Target)
		sb.WriteString(")\n")

	case *TraitDecl:
		indent(sb, level)
		fmt.Fprintf(sb, "(trait %s)\n", d.Name)

	case *ImplDecl:
		indent(sb, level)
		if d.TraitName != "" {
			fmt.Fprintf(sb, "(impl %s for %s)\n", d.TraitName, d.TypeName)
		} else {
			fmt.Fprintf(sb, "(impl %s)\n", d.TypeName)
		}

	case *ConstDecl:
		indent(sb, level)
		fmt.Fprintf(sb, "(const %s ", d.Name)
		formatExpr(sb, d.Value, 0)
		sb.WriteString(")\n")

	case *EntryBlock:
		indent(sb, level)
		sb.WriteString("(entry\n")
		formatExpr(sb, d.Body, level+1)
		indent(sb, level)
		sb.WriteString(")\n")

	case *TestBlock:
		indent(sb, level)
		fmt.Fprintf(sb, "(test %q\n", d.Name)
		formatExpr(sb, d.Body, level+1)
		indent(sb, level)
		sb.WriteString(")\n")

	default:
		indent(sb, level)
		fmt.Fprintf(sb, "(unknown-decl)\n")
	}
}

func formatExpr(sb *strings.Builder, expr Expr, level int) {
	if expr == nil {
		indent(sb, level)
		sb.WriteString("nil")
		return
	}
	switch e := expr.(type) {
	case *IntLitExpr:
		indent(sb, level)
		fmt.Fprintf(sb, "%s", e.Value)
	case *FloatLitExpr:
		indent(sb, level)
		fmt.Fprintf(sb, "%s", e.Value)
	case *StringLitExpr:
		indent(sb, level)
		fmt.Fprintf(sb, "%q", e.Value)
	case *BoolLitExpr:
		indent(sb, level)
		fmt.Fprintf(sb, "%t", e.Value)
	case *IdentExpr:
		indent(sb, level)
		sb.WriteString(e.Name)
	case *PathExpr:
		indent(sb, level)
		sb.WriteString(strings.Join(e.Parts, "."))
	case *BinaryExpr:
		indent(sb, level)
		fmt.Fprintf(sb, "(%s ", e.Op)
		formatExpr(sb, e.Left, 0)
		sb.WriteString(" ")
		formatExpr(sb, e.Right, 0)
		sb.WriteString(")")
	case *UnaryExpr:
		indent(sb, level)
		fmt.Fprintf(sb, "(%s ", e.Op)
		formatExpr(sb, e.Operand, 0)
		sb.WriteString(")")
	case *PostfixExpr:
		indent(sb, level)
		sb.WriteString("(")
		formatExpr(sb, e.Operand, 0)
		fmt.Fprintf(sb, "%s)", e.Op)
	case *CallExpr:
		indent(sb, level)
		sb.WriteString("(call ")
		formatExpr(sb, e.Func, 0)
		for _, arg := range e.Args {
			sb.WriteString(" ")
			formatExpr(sb, arg.Value, 0)
		}
		sb.WriteString(")")
	case *FieldAccessExpr:
		indent(sb, level)
		sb.WriteString("(. ")
		formatExpr(sb, e.Object, 0)
		fmt.Fprintf(sb, " %s)", e.Field)
	case *MethodCallExpr:
		indent(sb, level)
		sb.WriteString("(.call ")
		formatExpr(sb, e.Object, 0)
		fmt.Fprintf(sb, " %s", e.Method)
		for _, arg := range e.Args {
			sb.WriteString(" ")
			formatExpr(sb, arg.Value, 0)
		}
		sb.WriteString(")")
	case *IndexExpr:
		indent(sb, level)
		sb.WriteString("(index ")
		formatExpr(sb, e.Object, 0)
		sb.WriteString(" ")
		formatExpr(sb, e.Index, 0)
		sb.WriteString(")")
	case *BlockExpr:
		indent(sb, level)
		sb.WriteString("(block\n")
		for _, stmt := range e.Stmts {
			formatStmt(sb, stmt, level+1)
		}
		if e.Expr != nil {
			formatExpr(sb, e.Expr, level+1)
			sb.WriteString("\n")
		}
		indent(sb, level)
		sb.WriteString(")")
	case *IfExpr:
		indent(sb, level)
		sb.WriteString("(if ")
		formatExpr(sb, e.Cond, 0)
		sb.WriteString("\n")
		formatExpr(sb, e.Then, level+1)
		if e.Else != nil {
			sb.WriteString("\n")
			indent(sb, level+1)
			sb.WriteString("(else\n")
			formatExpr(sb, e.Else, level+2)
			sb.WriteString(")")
		}
		sb.WriteString(")")
	case *MatchExpr:
		indent(sb, level)
		sb.WriteString("(match ")
		formatExpr(sb, e.Subject, 0)
		sb.WriteString(")\n")
	case *ClosureExpr:
		indent(sb, level)
		sb.WriteString("(closure ...)")
	case *StructExpr:
		indent(sb, level)
		fmt.Fprintf(sb, "(struct-init %s ...)", e.TypeName)
	case *ArrayExpr:
		indent(sb, level)
		sb.WriteString("(array")
		for _, el := range e.Elements {
			sb.WriteString(" ")
			formatExpr(sb, el, 0)
		}
		sb.WriteString(")")
	case *TupleExpr:
		indent(sb, level)
		sb.WriteString("(tuple")
		for _, el := range e.Elements {
			sb.WriteString(" ")
			formatExpr(sb, el, 0)
		}
		sb.WriteString(")")
	case *PipelineExpr:
		indent(sb, level)
		sb.WriteString("(|> ")
		formatExpr(sb, e.Left, 0)
		sb.WriteString(" ")
		formatExpr(sb, e.Right, 0)
		sb.WriteString(")")
	case *RangeExpr:
		indent(sb, level)
		op := ".."
		if e.Inclusive { op = "..=" }
		fmt.Fprintf(sb, "(%s ", op)
		formatExpr(sb, e.Start, 0)
		sb.WriteString(" ")
		formatExpr(sb, e.End, 0)
		sb.WriteString(")")
	case *InterpolatedStringExpr:
		indent(sb, level)
		sb.WriteString("(interpolated-string ...)")
	case *GroupExpr:
		formatExpr(sb, e.Inner, level)
	case *CatchExpr:
		indent(sb, level)
		sb.WriteString("(catch ")
		formatExpr(sb, e.Expr, 0)
		sb.WriteString(" ...)")
	default:
		indent(sb, level)
		fmt.Fprintf(sb, "(expr %T)", expr)
	}
}

func formatStmt(sb *strings.Builder, stmt Stmt, level int) {
	switch s := stmt.(type) {
	case *VarDeclStmt:
		indent(sb, level)
		if s.Mutable {
			fmt.Fprintf(sb, "(mut %s := ", s.Name)
		} else {
			fmt.Fprintf(sb, "(%s := ", s.Name)
		}
		formatExpr(sb, s.Value, 0)
		sb.WriteString(")\n")
	case *AssignStmt:
		indent(sb, level)
		sb.WriteString("(= ")
		formatExpr(sb, s.Target, 0)
		sb.WriteString(" ")
		formatExpr(sb, s.Value, 0)
		sb.WriteString(")\n")
	case *ExprStmt:
		formatExpr(sb, s.Expr, level)
		sb.WriteString("\n")
	case *ReturnStmt:
		indent(sb, level)
		sb.WriteString("(return")
		if s.Value != nil {
			sb.WriteString(" ")
			formatExpr(sb, s.Value, 0)
		}
		sb.WriteString(")\n")
	case *ForStmt:
		indent(sb, level)
		sb.WriteString("(for ...)\n")
	case *WhileStmt:
		indent(sb, level)
		sb.WriteString("(while ...)\n")
	case *LoopStmt:
		indent(sb, level)
		sb.WriteString("(loop ...)\n")
	case *BreakStmt:
		indent(sb, level)
		sb.WriteString("(break)\n")
	case *ContinueStmt:
		indent(sb, level)
		sb.WriteString("(continue)\n")
	case *DeferStmt:
		indent(sb, level)
		sb.WriteString("(defer ")
		formatExpr(sb, s.Expr, 0)
		sb.WriteString(")\n")
	default:
		indent(sb, level)
		fmt.Fprintf(sb, "(stmt %T)\n", stmt)
	}
}

func formatTypeExpr(sb *strings.Builder, te TypeExpr) {
	switch t := te.(type) {
	case *NamedTypeExpr:
		sb.WriteString(strings.Join(t.Path, "."))
		if len(t.TypeArgs) > 0 {
			sb.WriteString("[")
			for i, ta := range t.TypeArgs {
				if i > 0 { sb.WriteString(", ") }
				formatTypeExpr(sb, ta)
			}
			sb.WriteString("]")
		}
	case *ArrayTypeExpr:
		sb.WriteString("[")
		formatTypeExpr(sb, t.Element)
		sb.WriteString("]")
	case *OptionalTypeExpr:
		formatTypeExpr(sb, t.Inner)
		sb.WriteString("?")
	case *ResultTypeExpr:
		formatTypeExpr(sb, t.Ok)
		sb.WriteString(" ! ")
		formatTypeExpr(sb, t.Err)
	case *FunctionTypeExpr:
		sb.WriteString("fn(")
		for i, p := range t.Params {
			if i > 0 { sb.WriteString(", ") }
			formatTypeExpr(sb, p)
		}
		sb.WriteString(") -> ")
		formatTypeExpr(sb, t.Return)
	case *TupleTypeExpr:
		sb.WriteString("(")
		for i, el := range t.Elements {
			if i > 0 { sb.WriteString(", ") }
			formatTypeExpr(sb, el)
		}
		sb.WriteString(")")
	case *MapTypeExpr:
		sb.WriteString("{")
		formatTypeExpr(sb, t.Key)
		sb.WriteString(": ")
		formatTypeExpr(sb, t.Value)
		sb.WriteString("}")
	case *SetTypeExpr:
		sb.WriteString("{")
		formatTypeExpr(sb, t.Element)
		sb.WriteString("}")
	default:
		sb.WriteString("?type?")
	}
}

// FormatJSON returns the AST as JSON.
func FormatJSON(prog *Program) (string, error) {
	data, err := json.MarshalIndent(prog, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
