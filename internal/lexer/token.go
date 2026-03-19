package lexer

import "fmt"

// Position represents a source location.
type Position struct {
	File   string
	Line   int
	Column int
	Offset int
}

func (p Position) String() string {
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
}

// Token represents a single lexical token.
type Token struct {
	Type    TokenType
	Literal string
	Pos     Position
}

func (t Token) String() string {
	if t.Literal != "" {
		return fmt.Sprintf("%s(%q)", t.Type, t.Literal)
	}
	return t.Type.String()
}

// TokenType identifies the kind of token.
type TokenType int

const (
	// Special tokens
	Illegal TokenType = iota
	EOF
	Newline // significant newline (statement terminator)

	// Identifiers and literals
	Ident     // identifier
	IntLit    // integer literal
	FloatLit  // float literal
	StringLit // simple string literal (no interpolation)
	BoolLit   // true or false

	// String interpolation tokens
	StringStart    // opening part of interpolated string: "hello {
	StringMiddle   // middle part between interpolations: } world {
	StringEnd      // closing part: } goodbye"

	// Keywords
	keywordsStart
	Mod
	Use
	Pub
	Fn
	Type
	Enum
	Trait
	Impl
	Struct
	Entry
	If
	Else
	Match
	For
	In
	While
	Loop
	Break
	Continue
	Return
	Mut
	Const
	Spawn   // recognized but not supported in bootstrap
	Scope   // recognized but not supported in bootstrap
	Select  // recognized but not supported in bootstrap
	From    // recognized but not supported in bootstrap
	After   // recognized but not supported in bootstrap
	Defer
	With
	Catch
	Yield
	Test
	Assert
	Derives
	Where
	As
	Is
	Self_   // self (receiver)
	SelfTy  // Self (type)
	True
	False
	Alias
	keywordsEnd

	// Arithmetic operators
	Plus     // +
	Minus    // -
	Star     // *
	Slash    // /
	Percent  // %

	// Comparison operators
	EqEq   // ==
	BangEq // !=
	Lt     // <
	Gt     // >
	LtEq   // <=
	GtEq   // >=

	// Logical operators
	AmpAmp   // &&
	PipePipe // ||
	Bang     // !

	// Bitwise operators
	Amp   // &
	Pipe  // |
	Caret // ^
	Tilde // ~
	LtLt  // <<
	GtGt  // >>

	// Pipeline
	PipeGt // |>

	// Error propagation / optional chaining
	Question     // ?
	QuestionDot  // ?.
	QuestionQuestion // ??

	// Range
	DotDot  // ..
	DotDotEq // ..=

	// Binding and assignment
	ColonEq // :=
	Eq      // =
	FatArrow // =>
	Arrow   // ->

	// Access and update
	Dot    // .
	DotBrace // .{

	// Annotation
	At // @

	// Delimiters
	LParen   // (
	RParen   // )
	LBracket // [
	RBracket // ]
	LBrace   // {
	RBrace   // }
	Comma    // ,
	Colon    // :
)

var tokenNames = map[TokenType]string{
	Illegal:   "ILLEGAL",
	EOF:       "EOF",
	Newline:   "NEWLINE",
	Ident:     "IDENT",
	IntLit:    "INT",
	FloatLit:  "FLOAT",
	StringLit: "STRING",
	BoolLit:   "BOOL",

	StringStart:  "STRING_START",
	StringMiddle: "STRING_MIDDLE",
	StringEnd:    "STRING_END",

	Mod:      "mod",
	Use:      "use",
	Pub:      "pub",
	Fn:       "fn",
	Type:     "type",
	Enum:     "enum",
	Trait:    "trait",
	Impl:     "impl",
	Struct:   "struct",
	Entry:    "entry",
	If:       "if",
	Else:     "else",
	Match:    "match",
	For:      "for",
	In:       "in",
	While:    "while",
	Loop:     "loop",
	Break:    "break",
	Continue: "continue",
	Return:   "return",
	Mut:      "mut",
	Const:    "const",
	Spawn:    "spawn",
	Scope:    "scope",
	Select:   "select",
	From:     "from",
	After:    "after",
	Defer:    "defer",
	With:     "with",
	Catch:    "catch",
	Yield:    "yield",
	Test:     "test",
	Assert:   "assert",
	Derives:  "derives",
	Where:    "where",
	As:       "as",
	Is:       "is",
	Self_:    "self",
	SelfTy:   "Self",
	True:     "true",
	False:    "false",
	Alias:    "alias",

	Plus:    "+",
	Minus:   "-",
	Star:    "*",
	Slash:   "/",
	Percent: "%",

	EqEq:   "==",
	BangEq: "!=",
	Lt:     "<",
	Gt:     ">",
	LtEq:   "<=",
	GtEq:   ">=",

	AmpAmp:   "&&",
	PipePipe: "||",
	Bang:     "!",

	Amp:   "&",
	Pipe:  "|",
	Caret: "^",
	Tilde: "~",
	LtLt:  "<<",
	GtGt:  ">>",

	PipeGt: "|>",

	Question:         "?",
	QuestionDot:      "?.",
	QuestionQuestion: "??",

	DotDot:   "..",
	DotDotEq: "..=",

	ColonEq:  ":=",
	Eq:       "=",
	FatArrow: "=>",
	Arrow:    "->",

	Dot:      ".",
	DotBrace: ".{",

	At: "@",

	LParen:   "(",
	RParen:   ")",
	LBracket: "[",
	RBracket: "]",
	LBrace:   "{",
	RBrace:   "}",
	Comma:    ",",
	Colon:    ":",
}

func (t TokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		return name
	}
	return fmt.Sprintf("TokenType(%d)", t)
}

// keywords maps keyword strings to their token types.
var keywords = map[string]TokenType{
	"mod":      Mod,
	"use":      Use,
	"pub":      Pub,
	"fn":       Fn,
	"type":     Type,
	"enum":     Enum,
	"trait":    Trait,
	"impl":     Impl,
	"struct":   Struct,
	"entry":    Entry,
	"if":       If,
	"else":     Else,
	"match":    Match,
	"for":      For,
	"in":       In,
	"while":    While,
	"loop":     Loop,
	"break":    Break,
	"continue": Continue,
	"return":   Return,
	"mut":      Mut,
	"const":    Const,
	"spawn":    Spawn,
	"scope":    Scope,
	"select":   Select,
	"from":     From,
	"after":    After,
	"defer":    Defer,
	"with":     With,
	"catch":    Catch,
	"yield":    Yield,
	"test":     Test,
	"assert":   Assert,
	"derives":  Derives,
	"where":    Where,
	"as":       As,
	"is":       Is,
	"self":     Self_,
	"Self":     SelfTy,
	"true":     True,
	"false":    False,
	"alias":    Alias,
}

// LookupIdent returns the token type for an identifier string,
// checking if it is a keyword.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return Ident
}

// IsKeyword returns true if the token type is a keyword.
func (t TokenType) IsKeyword() bool {
	return t > keywordsStart && t < keywordsEnd
}

// terminatesStatement returns true if this token type can end a statement
// (i.e., a following newline should be treated as a statement terminator).
func (t TokenType) TerminatesStatement() bool {
	switch t {
	case Ident, IntLit, FloatLit, StringLit, BoolLit,
		StringEnd,
		RParen, RBracket, RBrace,
		Question, Bang,
		Break, Continue, Return,
		True, False, Self_, SelfTy:
		return true
	default:
		return false
	}
}
