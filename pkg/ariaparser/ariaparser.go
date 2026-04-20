// Package ariaparser is the public re-export shim for the Aria compiler's
// parser. It exists so external Go tooling (e.g. aid-gen-aria, language
// servers, doc generators) can consume the AST without reaching into the
// compiler's internal/ packages, which Go's visibility rules forbid.
//
// All types are aliases (type X = internal.X), so values flow freely between
// consumer code and the compiler without conversion.
package ariaparser

import (
	"github.com/aria-lang/aria/internal/lexer"
	"github.com/aria-lang/aria/internal/parser"
)

// ---------- Position ----------

type Pos = parser.Pos

// ---------- Enumerations ----------

type Visibility = parser.Visibility

const (
	Private       = parser.Private
	Public        = parser.Public
	PackagePublic = parser.PackagePublic
)

type TypeDeclKind = parser.TypeDeclKind

const (
	StructDecl  = parser.StructDecl
	SumTypeDecl = parser.SumTypeDecl
	NewtypeDecl = parser.NewtypeDecl
)

// ---------- Program & declarations ----------

type (
	Program = parser.Program
	Decl    = parser.Decl

	ModDecl     = parser.ModDecl
	ImportDecl  = parser.ImportDecl
	FnDecl      = parser.FnDecl
	TypeDecl    = parser.TypeDecl
	EnumDecl    = parser.EnumDecl
	AliasDecl   = parser.AliasDecl
	TraitDecl   = parser.TraitDecl
	ImplDecl    = parser.ImplDecl
	ConstDecl   = parser.ConstDecl
	EntryBlock  = parser.EntryBlock
	TestBlock   = parser.TestBlock

	GenericParam = parser.GenericParam
	Param        = parser.Param
	FieldDecl    = parser.FieldDecl
	VariantDecl  = parser.VariantDecl
	WhereItem    = parser.WhereItem
)

// ---------- Expressions ----------

type (
	Expr                   = parser.Expr
	IntLitExpr             = parser.IntLitExpr
	FloatLitExpr           = parser.FloatLitExpr
	StringLitExpr          = parser.StringLitExpr
	BoolLitExpr            = parser.BoolLitExpr
	InterpolatedStringExpr = parser.InterpolatedStringExpr
	IdentExpr              = parser.IdentExpr
	PathExpr               = parser.PathExpr
	BinaryExpr             = parser.BinaryExpr
	UnaryExpr              = parser.UnaryExpr
	PostfixExpr            = parser.PostfixExpr
	FieldAccessExpr        = parser.FieldAccessExpr
	OptionalChainExpr      = parser.OptionalChainExpr
	IndexExpr              = parser.IndexExpr
	CallExpr               = parser.CallExpr
	Arg                    = parser.Arg
	MethodCallExpr         = parser.MethodCallExpr
	RecordUpdateExpr       = parser.RecordUpdateExpr
	PipelineExpr           = parser.PipelineExpr
	FieldShorthandExpr     = parser.FieldShorthandExpr
	RangeExpr              = parser.RangeExpr
	BlockExpr              = parser.BlockExpr
	IfExpr                 = parser.IfExpr
	MatchExpr              = parser.MatchExpr
	MatchArm               = parser.MatchArm
	ClosureExpr            = parser.ClosureExpr
	StructExpr             = parser.StructExpr
	FieldInit              = parser.FieldInit
	ArrayExpr              = parser.ArrayExpr
	MapExpr                = parser.MapExpr
	MapEntry               = parser.MapEntry
	TupleExpr              = parser.TupleExpr
	ListCompExpr           = parser.ListCompExpr
	GroupExpr              = parser.GroupExpr
	CatchExpr              = parser.CatchExpr
)

// ---------- Statements ----------

type (
	Stmt         = parser.Stmt
	VarDeclStmt  = parser.VarDeclStmt
	AssignStmt   = parser.AssignStmt
	ExprStmt     = parser.ExprStmt
	ForStmt      = parser.ForStmt
	WhileStmt    = parser.WhileStmt
	LoopStmt     = parser.LoopStmt
	ReturnStmt   = parser.ReturnStmt
	BreakStmt    = parser.BreakStmt
	ContinueStmt = parser.ContinueStmt
	DeferStmt    = parser.DeferStmt
)

// ---------- Patterns ----------

type (
	Pattern         = parser.Pattern
	WildcardPattern = parser.WildcardPattern
	BindingPattern  = parser.BindingPattern
	LiteralPattern  = parser.LiteralPattern
	VariantPattern  = parser.VariantPattern
	StructPattern   = parser.StructPattern
	FieldPattern    = parser.FieldPattern
	TuplePattern    = parser.TuplePattern
	ArrayPattern    = parser.ArrayPattern
	OrPattern       = parser.OrPattern
	RestPattern     = parser.RestPattern
	NamedPattern    = parser.NamedPattern
)

// ---------- Type expressions ----------

type (
	TypeExpr         = parser.TypeExpr
	NamedTypeExpr    = parser.NamedTypeExpr
	FunctionTypeExpr = parser.FunctionTypeExpr
	TupleTypeExpr    = parser.TupleTypeExpr
	ArrayTypeExpr    = parser.ArrayTypeExpr
	MapTypeExpr      = parser.MapTypeExpr
	SetTypeExpr      = parser.SetTypeExpr
	OptionalTypeExpr = parser.OptionalTypeExpr
	ResultTypeExpr   = parser.ResultTypeExpr
)

// ---------- Parser API ----------

// Parser is the re-exported Aria parser.
type Parser = parser.Parser

// New constructs a Parser over a token stream.
func New(tokens []lexer.Token) *Parser { return parser.New(tokens) }

// Parse runs lex + parse for a single source file and returns the Program
// AST. This is the convenience entry point for tools that don't need direct
// lexer access.
func Parse(filename, source string) *Program {
	l := lexer.New(filename, source)
	toks := l.Tokenize()
	return parser.New(toks).Parse()
}

// FormatAST renders a human-readable tree.
func FormatAST(p *Program) string { return parser.FormatAST(p) }

// FormatJSON renders the AST as JSON.
func FormatJSON(p *Program) (string, error) { return parser.FormatJSON(p) }
