package lexer

import (
	"testing"
)

// helper to tokenize and strip EOF
func lex(source string) []Token {
	l := New("test.aria", source)
	tokens := l.Tokenize()
	return tokens
}

func expectTokens(t *testing.T, source string, expected []TokenType) {
	t.Helper()
	tokens := lex(source)
	// Filter out EOF for comparison unless it's in expected
	var got []TokenType
	for _, tok := range tokens {
		got = append(got, tok.Type)
	}

	if len(got) != len(expected) {
		t.Errorf("source: %q\nexpected %d tokens, got %d\nexpected: %v\ngot:      %v",
			source, len(expected), len(got), tokenTypeNames(expected), tokenTypeNames(got))
		return
	}

	for i, exp := range expected {
		if got[i] != exp {
			t.Errorf("source: %q\ntoken[%d]: expected %s, got %s\nfull: %v",
				source, i, exp, got[i], tokenTypeNames(got))
			return
		}
	}
}

func tokenTypeNames(types []TokenType) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.String()
	}
	return names
}

// US-1.2.2: Identifiers, keywords, whitespace, comments
func TestLexIdentifiersAndKeywords(t *testing.T) {
	expectTokens(t, "mod main", []TokenType{Mod, Ident, EOF})
}

func TestLexComments(t *testing.T) {
	expectTokens(t, "mod main\n// comment\nuse io", []TokenType{
		Mod, Ident, Newline,
		// comment is skipped
		Use, Ident, EOF,
	})
}

func TestLexModUseWithDot(t *testing.T) {
	expectTokens(t, "mod main\nuse std.fs", []TokenType{
		Mod, Ident, Newline,
		Use, Ident, Dot, Ident, EOF,
	})
}

func TestLexIdentifiers(t *testing.T) {
	tests := []struct {
		input   string
		wantLit string
	}{
		{"foo", "foo"},
		{"_bar", "_bar"},
		{"camelCase", "camelCase"},
		{"x123", "x123"},
		{"_", "_"},
	}
	for _, tt := range tests {
		tokens := lex(tt.input)
		if tokens[0].Type != Ident {
			t.Errorf("input %q: expected IDENT, got %s", tt.input, tokens[0].Type)
		}
		if tokens[0].Literal != tt.wantLit {
			t.Errorf("input %q: expected literal %q, got %q", tt.input, tt.wantLit, tokens[0].Literal)
		}
	}
}

func TestLexAllKeywords(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
	}{
		{"mod", Mod}, {"use", Use}, {"pub", Pub}, {"fn", Fn},
		{"type", Type}, {"enum", Enum}, {"trait", Trait}, {"impl", Impl},
		{"struct", Struct}, {"entry", Entry}, {"if", If}, {"else", Else},
		{"match", Match}, {"for", For}, {"in", In}, {"while", While},
		{"loop", Loop}, {"break", Break}, {"continue", Continue}, {"return", Return},
		{"mut", Mut}, {"const", Const}, {"spawn", Spawn}, {"scope", Scope},
		{"select", Select}, {"from", From}, {"after", After},
		{"defer", Defer}, {"with", With}, {"catch", Catch}, {"yield", Yield},
		{"test", Test}, {"assert", Assert}, {"derives", Derives},
		{"where", Where}, {"as", As}, {"is", Is},
		{"self", Self_}, {"Self", SelfTy},
		{"alias", Alias},
	}
	for _, tt := range tests {
		tokens := lex(tt.input)
		if tokens[0].Type != tt.want {
			t.Errorf("keyword %q: expected %s, got %s", tt.input, tt.want, tokens[0].Type)
		}
	}
}

// US-1.2.3: Numeric literals
func TestLexDecimalIntegers(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"42", "42"},
		{"0", "0"},
		{"1_000_000", "1_000_000"},
		{"123_456", "123_456"},
	}
	for _, tt := range tests {
		tokens := lex(tt.input)
		if tokens[0].Type != IntLit {
			t.Errorf("input %q: expected INT, got %s", tt.input, tokens[0].Type)
		}
		if tokens[0].Literal != tt.want {
			t.Errorf("input %q: expected literal %q, got %q", tt.input, tt.want, tokens[0].Literal)
		}
	}
}

func TestLexHexOctBin(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0xFF", "0xFF"},
		{"0xDEAD_BEEF", "0xDEAD_BEEF"},
		{"0o77", "0o77"},
		{"0o755", "0o755"},
		{"0b1010", "0b1010"},
		{"0b1111_0000", "0b1111_0000"},
	}
	for _, tt := range tests {
		tokens := lex(tt.input)
		if tokens[0].Type != IntLit {
			t.Errorf("input %q: expected INT, got %s", tt.input, tokens[0].Type)
		}
		if tokens[0].Literal != tt.want {
			t.Errorf("input %q: expected literal %q, got %q", tt.input, tt.want, tokens[0].Literal)
		}
	}
}

func TestLexFloats(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"3.14", "3.14"},
		{"0.0", "0.0"},
		{"1_000.5", "1_000.5"},
		{"1.0e10", "1.0e10"},
		{"2.5E-3", "2.5E-3"},
		{"1.0e+5", "1.0e+5"},
	}
	for _, tt := range tests {
		tokens := lex(tt.input)
		if tokens[0].Type != FloatLit {
			t.Errorf("input %q: expected FLOAT, got %s", tt.input, tokens[0].Type)
		}
		if tokens[0].Literal != tt.want {
			t.Errorf("input %q: expected literal %q, got %q", tt.input, tt.want, tokens[0].Literal)
		}
	}
}

func TestLexIntegerFollowedByDot(t *testing.T) {
	// 5.field should be INT DOT IDENT, not FLOAT
	expectTokens(t, "5.field", []TokenType{IntLit, Dot, Ident, EOF})
}

// US-1.2.4: String literals with interpolation
func TestLexSimpleString(t *testing.T) {
	tokens := lex(`"hello world"`)
	if tokens[0].Type != StringLit {
		t.Errorf("expected STRING, got %s", tokens[0].Type)
	}
	if tokens[0].Literal != "hello world" {
		t.Errorf("expected literal %q, got %q", "hello world", tokens[0].Literal)
	}
}

func TestLexStringEscapes(t *testing.T) {
	tokens := lex(`"hello\nworld\t\"quoted\""`)
	if tokens[0].Type != StringLit {
		t.Errorf("expected STRING, got %s", tokens[0].Type)
	}
	if tokens[0].Literal != "hello\nworld\t\"quoted\"" {
		t.Errorf("expected escaped literal, got %q", tokens[0].Literal)
	}
}

func TestLexStringEscapeBraces(t *testing.T) {
	tokens := lex(`"use \{ and \}"`)
	if tokens[0].Type != StringLit {
		t.Errorf("expected STRING, got %s", tokens[0].Type)
	}
	if tokens[0].Literal != "use { and }" {
		t.Errorf("expected literal with braces, got %q", tokens[0].Literal)
	}
}

func TestLexStringInterpolation(t *testing.T) {
	// "hello {name}" should produce StringStart, Ident, StringEnd
	tokens := lex(`"hello {name}"`)
	expected := []TokenType{StringStart, Ident, StringEnd, EOF}
	got := make([]TokenType, len(tokens))
	for i, tok := range tokens {
		got[i] = tok.Type
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(got), tokenTypeNames(got))
	}
	for i, exp := range expected {
		if got[i] != exp {
			t.Errorf("token[%d]: expected %s, got %s (full: %v)", i, exp, got[i], tokenTypeNames(got))
		}
	}
	// Check string parts
	if tokens[0].Literal != "hello " {
		t.Errorf("StringStart literal: expected %q, got %q", "hello ", tokens[0].Literal)
	}
	if tokens[1].Literal != "name" {
		t.Errorf("Ident literal: expected %q, got %q", "name", tokens[1].Literal)
	}
	if tokens[2].Literal != "" {
		t.Errorf("StringEnd literal: expected %q, got %q", "", tokens[2].Literal)
	}
}

func TestLexStringMultipleInterpolations(t *testing.T) {
	// "hello {first} {last}!" -> StringStart Ident StringMiddle Ident StringEnd
	tokens := lex(`"hello {first} {last}!"`)
	expected := []TokenType{StringStart, Ident, StringMiddle, Ident, StringEnd, EOF}
	got := make([]TokenType, len(tokens))
	for i, tok := range tokens {
		got[i] = tok.Type
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(got), tokenTypeNames(got))
	}
	for i, exp := range expected {
		if got[i] != exp {
			t.Errorf("token[%d]: expected %s, got %s", i, exp, got[i])
		}
	}
}

func TestLexStringInterpolationWithExpr(t *testing.T) {
	// "result: {a + b}" -> StringStart Ident Plus Ident StringEnd
	tokens := lex(`"result: {a + b}"`)
	expected := []TokenType{StringStart, Ident, Plus, Ident, StringEnd, EOF}
	got := make([]TokenType, len(tokens))
	for i, tok := range tokens {
		got[i] = tok.Type
	}
	if len(got) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(got), tokenTypeNames(got))
	}
	for i, exp := range expected {
		if got[i] != exp {
			t.Errorf("token[%d]: expected %s, got %s", i, exp, got[i])
		}
	}
}

// US-1.2.5: Operators and punctuation
func TestLexOperators(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
	}{
		{"+", Plus}, {"-", Minus}, {"*", Star}, {"/", Slash}, {"%", Percent},
		{"==", EqEq}, {"!=", BangEq}, {"<", Lt}, {">", Gt}, {"<=", LtEq}, {">=", GtEq},
		{"&&", AmpAmp}, {"||", PipePipe}, {"!", Bang},
		{"&", Amp}, {"|", Pipe}, {"^", Caret}, {"~", Tilde}, {"<<", LtLt}, {">>", GtGt},
		{"|>", PipeGt},
		{"?", Question}, {"?.", QuestionDot}, {"??", QuestionQuestion},
		{"..", DotDot}, {"..=", DotDotEq},
		{":=", ColonEq}, {"=", Eq}, {"=>", FatArrow}, {"->", Arrow},
		{".", Dot},
		{"@", At},
		{"(", LParen}, {")", RParen}, {"[", LBracket}, {"]", RBracket},
		{"{", LBrace}, {"}", RBrace},
		{",", Comma}, {":", Colon},
	}
	for _, tt := range tests {
		tokens := lex(tt.input)
		if tokens[0].Type != tt.want {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.want, tokens[0].Type)
		}
	}
}

func TestLexLongestMatch(t *testing.T) {
	// |> should be PipeGt, not Pipe then Gt
	expectTokens(t, "|>", []TokenType{PipeGt, EOF})
	// .. should be DotDot, not Dot Dot
	expectTokens(t, "..", []TokenType{DotDot, EOF})
	// ..= should be DotDotEq
	expectTokens(t, "..=", []TokenType{DotDotEq, EOF})
	// => should be FatArrow
	expectTokens(t, "=>", []TokenType{FatArrow, EOF})
	// -> should be Arrow
	expectTokens(t, "->", []TokenType{Arrow, EOF})
	// := should be ColonEq
	expectTokens(t, ":=", []TokenType{ColonEq, EOF})
	// == should be EqEq
	expectTokens(t, "==", []TokenType{EqEq, EOF})
	// ?. should be QuestionDot
	expectTokens(t, "?.", []TokenType{QuestionDot, EOF})
	// ?? should be QuestionQuestion
	expectTokens(t, "??", []TokenType{QuestionQuestion, EOF})
}

func TestLexBoolLiterals(t *testing.T) {
	tokens := lex("true false")
	if tokens[0].Type != BoolLit || tokens[0].Literal != "true" {
		t.Errorf("expected BoolLit(true), got %s(%q)", tokens[0].Type, tokens[0].Literal)
	}
	if tokens[1].Type != BoolLit || tokens[1].Literal != "false" {
		t.Errorf("expected BoolLit(false), got %s(%q)", tokens[1].Type, tokens[1].Literal)
	}
}

// US-1.3.1: Automatic statement termination (newline rules)
func TestNewlineAsTerminator(t *testing.T) {
	// Two statements on separate lines: newline should be emitted
	expectTokens(t, "x := 5\ny := 10", []TokenType{
		Ident, ColonEq, IntLit, Newline,
		Ident, ColonEq, IntLit, EOF,
	})
}

func TestNewlineAfterClosingParen(t *testing.T) {
	expectTokens(t, "foo()\nbar()", []TokenType{
		Ident, LParen, RParen, Newline,
		Ident, LParen, RParen, EOF,
	})
}

func TestNewlineSuppressedAfterOperator(t *testing.T) {
	// Multi-line expression: newline after + should NOT be a terminator
	expectTokens(t, "a +\nb", []TokenType{
		Ident, Plus, Ident, EOF,
	})
}

func TestNewlineSuppressedAfterComma(t *testing.T) {
	expectTokens(t, "a,\nb", []TokenType{
		Ident, Comma, Ident, EOF,
	})
}

func TestNewlineSuppressedAfterDot(t *testing.T) {
	expectTokens(t, "a.\nb", []TokenType{
		Ident, Dot, Ident, EOF,
	})
}

func TestNewlineSuppressedAfterPipeline(t *testing.T) {
	expectTokens(t, "a |>\nb", []TokenType{
		Ident, PipeGt, Ident, EOF,
	})
}

func TestNewlineSuppressedAfterColonEq(t *testing.T) {
	expectTokens(t, "x :=\n5", []TokenType{
		Ident, ColonEq, IntLit, EOF,
	})
}

func TestNewlineSuppressedAfterEq(t *testing.T) {
	expectTokens(t, "x =\n5", []TokenType{
		Ident, Eq, IntLit, EOF,
	})
}

func TestNewlineSuppressedAfterArrow(t *testing.T) {
	expectTokens(t, "fn foo() ->\ni64", []TokenType{
		Fn, Ident, LParen, RParen, Arrow, Ident, EOF,
	})
}

func TestNewlineSuppressedAfterFatArrow(t *testing.T) {
	expectTokens(t, "Circle(r) =>\n3.14 * r", []TokenType{
		Ident, LParen, Ident, RParen, FatArrow, FloatLit, Star, Ident, EOF,
	})
}

func TestNewlineSuppressedInsideParens(t *testing.T) {
	// Inside balanced parens, newlines are transparent
	expectTokens(t, "foo(\na,\nb\n)", []TokenType{
		Ident, LParen, Ident, Comma, Ident, RParen, EOF,
	})
}

func TestNewlineSuppressedInsideBrackets(t *testing.T) {
	expectTokens(t, "[\na,\nb\n]", []TokenType{
		LBracket, Ident, Comma, Ident, RBracket, EOF,
	})
}

func TestNewlineSuppressedInsideBraces(t *testing.T) {
	expectTokens(t, "{\na\nb\n}", []TokenType{
		LBrace, Ident, Newline, Ident, Newline, RBrace, EOF,
	})
}

func TestNewlineAfterBreakContinueReturn(t *testing.T) {
	expectTokens(t, "break\ncontinue\nreturn", []TokenType{
		Break, Newline, Continue, Newline, Return, EOF,
	})
}

func TestNewlineAfterQuestion(t *testing.T) {
	expectTokens(t, "foo()?\nbar()", []TokenType{
		Ident, LParen, RParen, Question, Newline,
		Ident, LParen, RParen, EOF,
	})
}

func TestNewlineAfterBang(t *testing.T) {
	expectTokens(t, "foo()!\nbar()", []TokenType{
		Ident, LParen, RParen, Bang, Newline,
		Ident, LParen, RParen, EOF,
	})
}

// US-1.3.2: Error handling
func TestLexInvalidCharacter(t *testing.T) {
	l := New("test.aria", "$")
	tokens := l.Tokenize()
	if tokens[0].Type != Illegal {
		t.Errorf("expected ILLEGAL, got %s", tokens[0].Type)
	}
	if !l.Diagnostics().HasErrors() {
		t.Error("expected diagnostic for invalid character")
	}
	diags := l.Diagnostics().Diagnostics
	if diags[0].Code != "E0001" {
		t.Errorf("expected E0001, got %s", diags[0].Code)
	}
}

func TestLexUnterminatedString(t *testing.T) {
	l := New("test.aria", `"hello`)
	l.Tokenize()
	if !l.Diagnostics().HasErrors() {
		t.Error("expected diagnostic for unterminated string")
	}
	diags := l.Diagnostics().Diagnostics
	if diags[0].Code != "E0002" {
		t.Errorf("expected E0002, got %s", diags[0].Code)
	}
}

func TestLexInvalidEscape(t *testing.T) {
	l := New("test.aria", `"\q"`)
	l.Tokenize()
	if !l.Diagnostics().HasErrors() {
		t.Error("expected diagnostic for invalid escape")
	}
	diags := l.Diagnostics().Diagnostics
	if diags[0].Code != "E0003" {
		t.Errorf("expected E0003, got %s", diags[0].Code)
	}
}

// Edge cases
func TestLexEmptyFile(t *testing.T) {
	expectTokens(t, "", []TokenType{EOF})
}

func TestLexOnlyComments(t *testing.T) {
	expectTokens(t, "// just a comment\n// another one", []TokenType{EOF})
}

func TestLexOnlyWhitespace(t *testing.T) {
	expectTokens(t, "   \t  ", []TokenType{EOF})
}

func TestLexMultipleNewlines(t *testing.T) {
	// Multiple newlines after a terminator should produce only one
	expectTokens(t, "x\n\n\ny", []TokenType{
		Ident, Newline, Ident, EOF,
	})
}

// Position tracking
func TestPositionTracking(t *testing.T) {
	tokens := lex("mod main\nfn foo()")
	// "mod" is at line 1, col 1
	if tokens[0].Pos.Line != 1 || tokens[0].Pos.Column != 1 {
		t.Errorf("mod: expected 1:1, got %d:%d", tokens[0].Pos.Line, tokens[0].Pos.Column)
	}
	// "main" is at line 1, col 5
	if tokens[1].Pos.Line != 1 || tokens[1].Pos.Column != 5 {
		t.Errorf("main: expected 1:5, got %d:%d", tokens[1].Pos.Line, tokens[1].Pos.Column)
	}
	// After newline, "fn" is at line 2, col 1
	// tokens[2] is Newline, tokens[3] is Fn
	if tokens[3].Pos.Line != 2 || tokens[3].Pos.Column != 1 {
		t.Errorf("fn: expected 2:1, got %d:%d", tokens[3].Pos.Line, tokens[3].Pos.Column)
	}
}

// Full program snippet
func TestLexFullSnippet(t *testing.T) {
	source := `mod main

use std.fs

fn greet(name: str) -> str = "Hello, {name}!"

entry {
    msg := greet("world")
    println(msg)
}`
	l := New("test.aria", source)
	tokens := l.Tokenize()
	if l.Diagnostics().HasErrors() {
		t.Errorf("unexpected errors: %v", l.Diagnostics().Diagnostics)
	}

	// Just verify it produces a reasonable number of tokens without errors
	if len(tokens) < 20 {
		t.Errorf("expected at least 20 tokens, got %d", len(tokens))
	}

	// Last token should be EOF
	if tokens[len(tokens)-1].Type != EOF {
		t.Errorf("expected EOF as last token, got %s", tokens[len(tokens)-1].Type)
	}
}

func TestLexDotBrace(t *testing.T) {
	expectTokens(t, "x.{y: 1}", []TokenType{
		Ident, DotBrace, Ident, Colon, IntLit, RBrace, EOF,
	})
}

func TestLexQuestionQuestion(t *testing.T) {
	expectTokens(t, "a ?? b", []TokenType{
		Ident, QuestionQuestion, Ident, EOF,
	})
}
