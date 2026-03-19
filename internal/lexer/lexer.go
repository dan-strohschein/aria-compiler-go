package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aria-lang/aria/internal/diagnostic"
)

// Lexer tokenizes Aria source code.
type Lexer struct {
	filename string
	source   string
	pos      int    // current byte position
	line     int    // current line (1-based)
	col      int    // current column (1-based)
	prevTok  TokenType // previous non-whitespace token type (for newline rules)

	// Balanced delimiter depth for newline suppression
	parenDepth   int
	bracketDepth int
	braceDepth   int

	// String interpolation state
	interpDepth int // nesting depth of string interpolation

	diagnostics *diagnostic.DiagnosticList
}

// New creates a new Lexer for the given source.
func New(filename, source string) *Lexer {
	return &Lexer{
		filename:    filename,
		source:      source,
		line:        1,
		col:         1,
		diagnostics: &diagnostic.DiagnosticList{},
	}
}

// Diagnostics returns the accumulated diagnostics.
func (l *Lexer) Diagnostics() *diagnostic.DiagnosticList {
	return l.diagnostics
}

// Tokenize returns all tokens from the source.
func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}
	return tokens
}

// NextToken returns the next token from the source.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	// Check for newline
	if l.atNewline() {
		l.consumeNewline()
		// Only emit newline token if previous token terminates a statement
		// and we're not inside balanced delimiters
		if l.prevTok.TerminatesStatement() && !l.insideDelimiters() {
			tok := l.makeToken(Newline, "")
			l.prevTok = Newline // prevent consecutive newlines from emitting
			return tok
		}
		// Otherwise skip the newline and continue
		return l.NextToken()
	}

	if l.pos >= len(l.source) {
		return l.makeToken(EOF, "")
	}

	// Check if we're at the end of a string interpolation
	if l.interpDepth > 0 && l.peek() == '}' {
		return l.continueInterpolatedString()
	}

	ch := l.peek()
	startPos := l.currentPos()

	var tok Token

	switch {
	case ch == '/' && l.peekAt(1) == '/':
		l.skipLineComment()
		return l.NextToken()

	case ch == '"':
		tok = l.lexString()

	case isDigit(ch):
		tok = l.lexNumber()

	case isLetter(ch):
		tok = l.lexIdentOrKeyword()

	default:
		tok = l.lexOperatorOrPunct()
	}

	if tok.Type == Illegal {
		tok.Pos = startPos
	}

	l.prevTok = tok.Type
	return tok
}

func (l *Lexer) currentPos() Position {
	return Position{
		File:   l.filename,
		Line:   l.line,
		Column: l.col,
		Offset: l.pos,
	}
}

func (l *Lexer) makeToken(typ TokenType, literal string) Token {
	return Token{
		Type:    typ,
		Literal: literal,
		Pos:     l.currentPos(),
	}
}

func (l *Lexer) makeTokenAt(typ TokenType, literal string, pos Position) Token {
	return Token{
		Type:    typ,
		Literal: literal,
		Pos:     pos,
	}
}

// peek returns the current character without advancing.
func (l *Lexer) peek() byte {
	if l.pos >= len(l.source) {
		return 0
	}
	return l.source[l.pos]
}

// peekAt returns the character at offset from current position.
func (l *Lexer) peekAt(offset int) byte {
	p := l.pos + offset
	if p >= len(l.source) {
		return 0
	}
	return l.source[p]
}

// advance moves forward one byte.
func (l *Lexer) advance() byte {
	if l.pos >= len(l.source) {
		return 0
	}
	ch := l.source[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

// match advances if the current character matches expected.
func (l *Lexer) match(expected byte) bool {
	if l.pos < len(l.source) && l.source[l.pos] == expected {
		l.advance()
		return true
	}
	return false
}

func (l *Lexer) atNewline() bool {
	if l.pos >= len(l.source) {
		return false
	}
	return l.source[l.pos] == '\n' || (l.source[l.pos] == '\r' && l.peekAt(1) == '\n')
}

func (l *Lexer) consumeNewline() {
	if l.source[l.pos] == '\r' {
		l.advance() // consume \r
	}
	l.advance() // consume \n
}

func (l *Lexer) insideDelimiters() bool {
	// Braces are NOT grouping delimiters — they are blocks.
	// Newlines inside {} ARE significant (statement terminators).
	// Only () and [] suppress newlines.
	return l.parenDepth > 0 || l.bracketDepth > 0
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch == ' ' || ch == '\t' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) skipLineComment() {
	// Skip //
	l.advance()
	l.advance()
	for l.pos < len(l.source) && l.source[l.pos] != '\n' {
		l.advance()
	}
}

// lexIdentOrKeyword scans an identifier or keyword.
func (l *Lexer) lexIdentOrKeyword() Token {
	pos := l.currentPos()
	start := l.pos
	for l.pos < len(l.source) && isIdentChar(l.source[l.pos]) {
		l.advance()
	}
	literal := l.source[start:l.pos]
	typ := LookupIdent(literal)

	// true and false are bool literals
	if typ == True || typ == False {
		return Token{Type: BoolLit, Literal: literal, Pos: pos}
	}

	return Token{Type: typ, Literal: literal, Pos: pos}
}

// lexNumber scans an integer or float literal.
func (l *Lexer) lexNumber() Token {
	pos := l.currentPos()
	start := l.pos

	// Check for hex, octal, binary prefixes
	if l.peek() == '0' && l.pos+1 < len(l.source) {
		next := l.source[l.pos+1]
		switch next {
		case 'x', 'X':
			return l.lexHexNumber(pos)
		case 'o', 'O':
			return l.lexOctNumber(pos)
		case 'b', 'B':
			// Check it's not just an identifier like 0b without digits
			if l.pos+2 < len(l.source) && isBinDigit(l.source[l.pos+2]) {
				return l.lexBinNumber(pos)
			}
		}
	}

	// Decimal integer or float
	l.scanDecimalDigits()

	// Check for float: requires digit on both sides of .
	if l.peek() == '.' && l.pos+1 < len(l.source) && isDigit(l.source[l.pos+1]) {
		l.advance() // consume .
		l.scanDecimalDigits()

		// Exponent
		if l.peek() == 'e' || l.peek() == 'E' {
			l.advance()
			if l.peek() == '+' || l.peek() == '-' {
				l.advance()
			}
			if !isDigit(l.peek()) {
				l.addError(diagnostic.E0004, "invalid float literal: expected digit after exponent")
			}
			l.scanDecimalDigits()
		}

		return Token{Type: FloatLit, Literal: l.source[start:l.pos], Pos: pos}
	}

	return Token{Type: IntLit, Literal: l.source[start:l.pos], Pos: pos}
}

func (l *Lexer) lexHexNumber(pos Position) Token {
	start := l.pos
	l.advance() // 0
	l.advance() // x
	if !isHexDigit(l.peek()) {
		l.addError(diagnostic.E0004, "invalid hex literal: expected hex digit after 0x")
	}
	for l.pos < len(l.source) && (isHexDigit(l.source[l.pos]) || l.source[l.pos] == '_') {
		l.advance()
	}
	return Token{Type: IntLit, Literal: l.source[start:l.pos], Pos: pos}
}

func (l *Lexer) lexOctNumber(pos Position) Token {
	start := l.pos
	l.advance() // 0
	l.advance() // o
	if !isOctDigit(l.peek()) {
		l.addError(diagnostic.E0004, "invalid octal literal: expected octal digit after 0o")
	}
	for l.pos < len(l.source) && (isOctDigit(l.source[l.pos]) || l.source[l.pos] == '_') {
		l.advance()
	}
	return Token{Type: IntLit, Literal: l.source[start:l.pos], Pos: pos}
}

func (l *Lexer) lexBinNumber(pos Position) Token {
	start := l.pos
	l.advance() // 0
	l.advance() // b
	if !isBinDigit(l.peek()) {
		l.addError(diagnostic.E0004, "invalid binary literal: expected 0 or 1 after 0b")
	}
	for l.pos < len(l.source) && (isBinDigit(l.source[l.pos]) || l.source[l.pos] == '_') {
		l.advance()
	}
	return Token{Type: IntLit, Literal: l.source[start:l.pos], Pos: pos}
}

func (l *Lexer) scanDecimalDigits() {
	for l.pos < len(l.source) && (isDigit(l.source[l.pos]) || l.source[l.pos] == '_') {
		l.advance()
	}
}

// lexString scans a string literal, handling interpolation.
func (l *Lexer) lexString() Token {
	pos := l.currentPos()
	l.advance() // consume opening "

	var buf strings.Builder
	hasInterpolation := false

	for l.pos < len(l.source) {
		ch := l.source[l.pos]

		if ch == '"' {
			l.advance() // consume closing "
			if hasInterpolation {
				// This shouldn't happen if we handle interpolation inline,
				// but just in case
				return Token{Type: StringLit, Literal: buf.String(), Pos: pos}
			}
			return Token{Type: StringLit, Literal: buf.String(), Pos: pos}
		}

		if ch == '{' {
			// String interpolation
			hasInterpolation = true
			l.interpDepth++
			l.braceDepth++ // track for delimiter balancing

			tok := Token{Type: StringStart, Literal: buf.String(), Pos: pos}
			l.advance() // consume {
			l.prevTok = StringStart
			return tok
		}

		if ch == '\\' {
			escaped := l.lexEscapeSequence()
			buf.WriteString(escaped)
			continue
		}

		if ch == '\n' || (ch == '\r' && l.peekAt(1) == '\n') {
			// Multi-line strings are permitted
			buf.WriteByte('\n')
			if ch == '\r' {
				l.advance()
			}
			l.advance()
			continue
		}

		// Regular character — handle UTF-8
		r, size := utf8.DecodeRuneInString(l.source[l.pos:])
		if r == utf8.RuneError && size == 1 {
			l.addError(diagnostic.E0001, "invalid UTF-8 character in string")
			l.advance()
			continue
		}
		buf.WriteRune(r)
		for i := 0; i < size; i++ {
			l.advance()
		}
	}

	l.addError(diagnostic.E0002, "unterminated string literal")
	return Token{Type: StringLit, Literal: buf.String(), Pos: pos}
}

// continueInterpolatedString continues scanning after an interpolation expression.
func (l *Lexer) continueInterpolatedString() Token {
	pos := l.currentPos()
	l.advance() // consume }
	l.braceDepth--
	l.interpDepth--

	var buf strings.Builder

	for l.pos < len(l.source) {
		ch := l.source[l.pos]

		if ch == '"' {
			l.advance() // consume closing "
			tok := Token{Type: StringEnd, Literal: buf.String(), Pos: pos}
			l.prevTok = StringEnd
			return tok
		}

		if ch == '{' {
			// Another interpolation
			l.interpDepth++
			l.braceDepth++
			tok := Token{Type: StringMiddle, Literal: buf.String(), Pos: pos}
			l.advance() // consume {
			l.prevTok = StringMiddle
			return tok
		}

		if ch == '\\' {
			escaped := l.lexEscapeSequence()
			buf.WriteString(escaped)
			continue
		}

		if ch == '\n' || (ch == '\r' && l.peekAt(1) == '\n') {
			buf.WriteByte('\n')
			if ch == '\r' {
				l.advance()
			}
			l.advance()
			continue
		}

		r, size := utf8.DecodeRuneInString(l.source[l.pos:])
		buf.WriteRune(r)
		for i := 0; i < size; i++ {
			l.advance()
		}
	}

	l.addError(diagnostic.E0002, "unterminated string literal")
	return Token{Type: StringEnd, Literal: buf.String(), Pos: pos}
}

// lexEscapeSequence processes an escape sequence and returns the escaped string.
func (l *Lexer) lexEscapeSequence() string {
	l.advance() // consume backslash
	if l.pos >= len(l.source) {
		l.addError(diagnostic.E0003, "unexpected end of file in escape sequence")
		return ""
	}
	ch := l.advance()
	switch ch {
	case 'n':
		return "\n"
	case 'r':
		return "\r"
	case 't':
		return "\t"
	case '\\':
		return "\\"
	case '"':
		return "\""
	case '{':
		return "{"
	case '}':
		return "}"
	case '0':
		return "\x00"
	default:
		l.addError(diagnostic.E0003, fmt.Sprintf("invalid escape sequence '\\%c'", ch))
		return string(ch)
	}
}

// lexOperatorOrPunct scans an operator or punctuation token.
func (l *Lexer) lexOperatorOrPunct() Token {
	pos := l.currentPos()
	ch := l.advance()

	switch ch {
	case '+':
		return l.makeTokenAt(Plus, "+", pos)
	case '-':
		if l.match('>') {
			return l.makeTokenAt(Arrow, "->", pos)
		}
		return l.makeTokenAt(Minus, "-", pos)
	case '*':
		return l.makeTokenAt(Star, "*", pos)
	case '/':
		return l.makeTokenAt(Slash, "/", pos)
	case '%':
		return l.makeTokenAt(Percent, "%", pos)

	case '=':
		if l.match('=') {
			return l.makeTokenAt(EqEq, "==", pos)
		}
		if l.match('>') {
			return l.makeTokenAt(FatArrow, "=>", pos)
		}
		return l.makeTokenAt(Eq, "=", pos)

	case '!':
		if l.match('=') {
			return l.makeTokenAt(BangEq, "!=", pos)
		}
		return l.makeTokenAt(Bang, "!", pos)

	case '<':
		if l.match('=') {
			return l.makeTokenAt(LtEq, "<=", pos)
		}
		if l.match('<') {
			return l.makeTokenAt(LtLt, "<<", pos)
		}
		return l.makeTokenAt(Lt, "<", pos)

	case '>':
		if l.match('=') {
			return l.makeTokenAt(GtEq, ">=", pos)
		}
		if l.match('>') {
			return l.makeTokenAt(GtGt, ">>", pos)
		}
		return l.makeTokenAt(Gt, ">", pos)

	case '&':
		if l.match('&') {
			return l.makeTokenAt(AmpAmp, "&&", pos)
		}
		return l.makeTokenAt(Amp, "&", pos)

	case '|':
		if l.match('|') {
			return l.makeTokenAt(PipePipe, "||", pos)
		}
		if l.match('>') {
			return l.makeTokenAt(PipeGt, "|>", pos)
		}
		return l.makeTokenAt(Pipe, "|", pos)

	case '^':
		return l.makeTokenAt(Caret, "^", pos)
	case '~':
		return l.makeTokenAt(Tilde, "~", pos)

	case '?':
		if l.match('?') {
			return l.makeTokenAt(QuestionQuestion, "??", pos)
		}
		if l.match('.') {
			return l.makeTokenAt(QuestionDot, "?.", pos)
		}
		return l.makeTokenAt(Question, "?", pos)

	case '.':
		if l.match('.') {
			if l.match('=') {
				return l.makeTokenAt(DotDotEq, "..=", pos)
			}
			return l.makeTokenAt(DotDot, "..", pos)
		}
		if l.match('{') {
			l.braceDepth++
			return l.makeTokenAt(DotBrace, ".{", pos)
		}
		return l.makeTokenAt(Dot, ".", pos)

	case ':':
		if l.match('=') {
			return l.makeTokenAt(ColonEq, ":=", pos)
		}
		return l.makeTokenAt(Colon, ":", pos)

	case '@':
		return l.makeTokenAt(At, "@", pos)

	case '(':
		l.parenDepth++
		return l.makeTokenAt(LParen, "(", pos)
	case ')':
		l.parenDepth--
		return l.makeTokenAt(RParen, ")", pos)
	case '[':
		l.bracketDepth++
		return l.makeTokenAt(LBracket, "[", pos)
	case ']':
		l.bracketDepth--
		return l.makeTokenAt(RBracket, "]", pos)
	case '{':
		l.braceDepth++
		return l.makeTokenAt(LBrace, "{", pos)
	case '}':
		if l.interpDepth > 0 {
			// Don't decrease braceDepth here; continueInterpolatedString handles it
			// But we need to rewind and let the interpolation handler deal with it
			l.pos--
			l.col--
			return l.continueInterpolatedString()
		}
		l.braceDepth--
		return l.makeTokenAt(RBrace, "}", pos)
	case ',':
		return l.makeTokenAt(Comma, ",", pos)

	default:
		// Check for valid Unicode character
		r, _ := utf8.DecodeRuneInString(l.source[l.pos-1:])
		if unicode.IsGraphic(r) {
			l.addErrorAt(diagnostic.E0001, fmt.Sprintf("unexpected character '%c'", r), pos)
		} else {
			l.addErrorAt(diagnostic.E0001, fmt.Sprintf("invalid character U+%04X", r), pos)
		}
		return l.makeTokenAt(Illegal, string(ch), pos)
	}
}

// addError adds a diagnostic at the current position.
func (l *Lexer) addError(code string, message string) {
	l.addErrorAt(code, message, l.currentPos())
}

// addErrorAt adds a diagnostic at a specific position.
func (l *Lexer) addErrorAt(code string, message string, pos Position) {
	sourceLine := l.getSourceLine(pos.Line)
	l.diagnostics.Add(diagnostic.Diagnostic{
		Code:       code,
		Severity:   diagnostic.Error,
		Message:    message,
		File:       pos.File,
		Line:       pos.Line,
		Column:     pos.Column,
		Span:       [2]int{pos.Offset, pos.Offset + 1},
		SourceLine: sourceLine,
		Labels: []diagnostic.Label{
			{
				File:    pos.File,
				Line:    pos.Line,
				Column:  pos.Column,
				Span:    [2]int{pos.Offset, pos.Offset + 1},
				Message: message,
				Style:   diagnostic.Primary,
			},
		},
	})
}

// getSourceLine returns the source line at the given line number.
func (l *Lexer) getSourceLine(lineNum int) string {
	lines := strings.Split(l.source, "\n")
	if lineNum > 0 && lineNum <= len(lines) {
		return lines[lineNum-1]
	}
	return ""
}

// Character classification helpers
func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentChar(ch byte) bool {
	return isLetter(ch) || isDigit(ch)
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isOctDigit(ch byte) bool {
	return ch >= '0' && ch <= '7'
}

func isBinDigit(ch byte) bool {
	return ch == '0' || ch == '1'
}
