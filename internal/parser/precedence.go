package parser

import (
	"fmt"

	"github.com/aria-lang/aria/internal/lexer"
)

// Binding powers for Pratt parsing.
// Higher = binds tighter.
const (
	bpNone       = 0
	bpAssign     = 1  // := =
	bpPipeline   = 2  // |>
	bpCoalesce   = 3  // ??
	bpLogicalOr  = 4  // ||
	bpLogicalAnd = 5  // &&
	bpComparison = 6  // == != < > <= >=
	bpRange      = 7  // .. ..=
	bpBitwiseOr  = 8  // |
	bpBitwiseXor = 9  // ^
	bpBitwiseAnd = 10 // &
	bpShift      = 11 // << >>
	bpAdditive   = 12 // + -
	bpMult       = 13 // * / %
	bpUnary      = 14 // - ! ~ (prefix)
	bpPostfix    = 15 // ? ! (postfix)
	bpAccess     = 16 // . ?. [idx] (args) .{...}
)

type assoc int

const (
	assocLeft  assoc = iota
	assocRight
	assocNone
)

type infixInfo struct {
	bp    int
	assoc assoc
}

func infixBindingPower(tok lexer.TokenType) (infixInfo, bool) {
	switch tok {
	// Level 2: Pipeline
	case lexer.PipeGt:
		return infixInfo{bpPipeline, assocLeft}, true

	// Level 3: Null coalesce
	case lexer.QuestionQuestion:
		return infixInfo{bpCoalesce, assocLeft}, true

	// Level 4: Logical OR
	case lexer.PipePipe:
		return infixInfo{bpLogicalOr, assocLeft}, true

	// Level 5: Logical AND
	case lexer.AmpAmp:
		return infixInfo{bpLogicalAnd, assocLeft}, true

	// Level 6: Comparison (non-associative)
	case lexer.EqEq, lexer.BangEq, lexer.Lt, lexer.Gt, lexer.LtEq, lexer.GtEq:
		return infixInfo{bpComparison, assocNone}, true

	// Level 7: Range (non-associative)
	case lexer.DotDot, lexer.DotDotEq:
		return infixInfo{bpRange, assocNone}, true

	// Level 8: Bitwise OR
	case lexer.Pipe:
		return infixInfo{bpBitwiseOr, assocLeft}, true

	// Level 9: Bitwise XOR
	case lexer.Caret:
		return infixInfo{bpBitwiseXor, assocLeft}, true

	// Level 10: Bitwise AND
	case lexer.Amp:
		return infixInfo{bpBitwiseAnd, assocLeft}, true

	// Level 11: Shift
	case lexer.LtLt, lexer.GtGt:
		return infixInfo{bpShift, assocLeft}, true

	// Level 12: Additive
	case lexer.Plus, lexer.Minus:
		return infixInfo{bpAdditive, assocLeft}, true

	// Level 13: Multiplicative
	case lexer.Star, lexer.Slash, lexer.Percent:
		return infixInfo{bpMult, assocLeft}, true

	default:
		return infixInfo{}, false
	}
}

// parseExpression parses an expression using Pratt parsing.
func (p *Parser) parseExpression() Expr {
	return p.parsePratt(bpNone)
}

func (p *Parser) parsePratt(minBP int) Expr {
	left := p.parseUnary()

	for {
		tok := p.peekType()

		// Handle postfix operators at level 15: ? and !
		if tok == lexer.Question || tok == lexer.Bang {
			// Only treat as postfix if it's not at the start of a line
			// (the lexer handles this via newline rules)
			if bpPostfix > minBP {
				op := p.advance()
				left = &PostfixExpr{Op: op.Type, Operand: left, Pos: op.Pos}
				continue
			}
		}

		// Handle access operators at level 16: . ?. [idx] (args) .{...}
		if tok == lexer.Dot || tok == lexer.QuestionDot || tok == lexer.LBracket ||
			tok == lexer.LParen || tok == lexer.DotBrace {
			if bpAccess > minBP {
				left = p.parsePostfixAccess(left)
				continue
			}
		}

		// Handle catch as postfix
		if tok == lexer.Catch {
			left = p.parseCatchExpr(left)
			continue
		}

		// Binary operators
		info, ok := infixBindingPower(tok)
		if !ok {
			break
		}
		// Standard Pratt: stop when operator bp is below our minimum
		if info.bp < minBP {
			break
		}

		op := p.advance()

		// Compute right-side minimum binding power
		nextMinBP := info.bp // right-assoc: same bp
		if info.assoc == assocLeft || info.assoc == assocNone {
			nextMinBP = info.bp + 1 // left-assoc/non-assoc: higher bp required
		}

		// Pipeline special case: rhs can be .field shorthand
		if op.Type == lexer.PipeGt {
			var right Expr
			if p.check(lexer.Dot) && !p.check(lexer.DotDot) {
				// .field shorthand
				p.advance() // consume .
				field := p.expect(lexer.Ident)
				right = &FieldShorthandExpr{Field: field.Literal, Pos: field.Pos}
			} else {
				right = p.parsePratt(nextMinBP)
			}
			left = &PipelineExpr{Left: left, Right: right, Pos: op.Pos}
			continue
		}

		// Range operators produce RangeExpr
		if op.Type == lexer.DotDot || op.Type == lexer.DotDotEq {
			right := p.parsePratt(nextMinBP)
			left = &RangeExpr{
				Start:     left,
				End:       right,
				Inclusive: op.Type == lexer.DotDotEq,
				Pos:       op.Pos,
			}
			continue
		}

		right := p.parsePratt(nextMinBP)
		left = &BinaryExpr{Op: op.Type, Left: left, Right: right, Pos: op.Pos}
	}

	return left
}

func (p *Parser) parseUnary() Expr {
	pos := p.currentPos()

	switch p.peekType() {
	case lexer.Minus, lexer.Bang, lexer.Tilde:
		op := p.advance()
		operand := p.parseUnary() // right-associative
		return &UnaryExpr{Op: op.Type, Operand: operand, Pos: pos}
	default:
		return p.parsePrimary()
	}
}

func (p *Parser) parsePostfixAccess(left Expr) Expr {
	switch p.peekType() {
	case lexer.Dot:
		p.advance()
		if !p.check(lexer.Ident) {
			p.error("expected field name after '.'")
			return left
		}
		field := p.advance()

		// Method call: obj.method(args)
		if p.check(lexer.LParen) {
			p.advance()
			args := p.parseArgList()
			p.expect(lexer.RParen)
			return &MethodCallExpr{
				Object: left,
				Method: field.Literal,
				Args:   args,
				Pos:    field.Pos,
			}
		}

		return &FieldAccessExpr{Object: left, Field: field.Literal, Pos: field.Pos}

	case lexer.QuestionDot:
		p.advance()
		field := p.expect(lexer.Ident)
		return &OptionalChainExpr{Object: left, Field: field.Literal, Pos: field.Pos}

	case lexer.LBracket:
		pos := p.currentPos()
		p.advance()
		index := p.parseExpression()
		p.expect(lexer.RBracket)
		return &IndexExpr{Object: left, Index: index, Pos: pos}

	case lexer.LParen:
		pos := p.currentPos()
		p.advance()
		args := p.parseArgList()
		p.expect(lexer.RParen)
		return &CallExpr{Func: left, Args: args, Pos: pos}

	case lexer.DotBrace:
		pos := p.currentPos()
		p.advance() // consume .{
		fields := p.parseFieldInitList()
		p.expect(lexer.RBrace)
		return &RecordUpdateExpr{Object: left, Fields: fields, Pos: pos}

	default:
		return left
	}
}

func (p *Parser) parseArgList() []*Arg {
	var args []*Arg
	if p.check(lexer.RParen) {
		return args
	}
	for {
		arg := p.parseArg()
		args = append(args, arg)
		if !p.match(lexer.Comma) {
			break
		}
		if p.check(lexer.RParen) {
			break // trailing comma
		}
	}
	return args
}

func (p *Parser) parseArg() *Arg {
	pos := p.currentPos()

	// Check for named argument: name: expr
	if p.check(lexer.Ident) && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == lexer.Colon {
		name := p.advance().Literal
		p.advance() // consume :
		value := p.parseExpression()
		return &Arg{Name: name, Value: value, Pos: pos}
	}

	value := p.parseExpression()
	return &Arg{Value: value, Pos: pos}
}

func (p *Parser) parseCatchExpr(expr Expr) Expr {
	pos := p.currentPos()
	p.expect(lexer.Catch)

	// catch |err| { ... }
	if p.match(lexer.Pipe) {
		errName := p.expect(lexer.Ident).Literal
		p.expect(lexer.Pipe)
		body := p.parseBlockExpr()
		return &CatchExpr{Expr: expr, ErrName: errName, Body: body, Pos: pos}
	}

	// catch { pattern => expr, ... }
	body := p.parseBlockExpr()
	return &CatchExpr{Expr: expr, Body: body, Pos: pos}
}

// ---------- Primary Expressions ----------

func (p *Parser) parsePrimary() Expr {
	pos := p.currentPos()

	switch p.peekType() {
	case lexer.IntLit:
		tok := p.advance()
		return &IntLitExpr{Value: tok.Literal, Pos: pos}

	case lexer.FloatLit:
		tok := p.advance()
		return &FloatLitExpr{Value: tok.Literal, Pos: pos}

	case lexer.StringLit:
		tok := p.advance()
		return &StringLitExpr{Value: tok.Literal, Pos: pos}

	case lexer.StringStart:
		return p.parseInterpolatedString()

	case lexer.BoolLit:
		tok := p.advance()
		return &BoolLitExpr{Value: tok.Literal == "true", Pos: pos}

	case lexer.Ident:
		return p.parseIdentOrStructExpr()

	case lexer.Self_:
		tok := p.advance()
		return &IdentExpr{Name: tok.Literal, Pos: pos}

	case lexer.LParen:
		return p.parseParenOrTuple()

	case lexer.LBrace:
		return p.parseBlockExpr()

	case lexer.LBracket:
		return p.parseArrayOrListComp()

	case lexer.If:
		return p.parseIfExpr()

	case lexer.Match:
		return p.parseMatchExpr()

	case lexer.Fn:
		return p.parseClosureExpr(false)

	case lexer.Mut:
		// move fn closure
		if p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == lexer.Fn {
			p.advance() // skip "move"... wait, this is Mut not Move
		}
		p.error(fmt.Sprintf("unexpected %s in expression", p.peekType()))
		p.advance()
		return &IdentExpr{Name: "?", Pos: pos}

	case lexer.Assert:
		return p.parseAssertExpr()

	default:
		p.error(fmt.Sprintf("expected expression, got %s", p.peekType()))
		p.advance()
		return &IdentExpr{Name: "?", Pos: pos}
	}
}

func (p *Parser) parseIdentOrStructExpr() Expr {
	pos := p.currentPos()
	name := p.advance() // consume ident

	// Check for struct literal: Name { field: value }
	// Struct literal: Name { field: value, ... }
	// Only treat as struct literal if:
	// 1. Name starts with uppercase (type name)
	// 2. The { is followed by ident: (field initializer pattern) or }
	// This avoids misinterpreting blocks as struct literals (e.g., `Red { return true }`)
	if p.check(lexer.LBrace) && len(name.Literal) > 0 && name.Literal[0] >= 'A' && name.Literal[0] <= 'Z' {
		if p.looksLikeStructLiteral() {
			p.advance() // consume {
			p.skipNewlines()
			fields := p.parseFieldInitList()
			p.expect(lexer.RBrace)
			return &StructExpr{TypeName: name.Literal, Fields: fields, Pos: pos}
		}
	}

	// Don't build PathExpr here — let the Pratt loop handle '.' as postfix access.
	// This ensures `point.x` becomes FieldAccessExpr and `obj.method()` becomes MethodCallExpr.
	return &IdentExpr{Name: name.Literal, Pos: pos}
}

// looksLikeStructLiteral peeks inside { to check if it contains field: value patterns.
// Returns true if { is followed by } (empty struct) or ident: (field initializer).
func (p *Parser) looksLikeStructLiteral() bool {
	// Save position and look ahead
	savedPos := p.pos
	p.advance() // consume {

	// Skip newlines inside
	for p.check(lexer.Newline) {
		p.advance()
	}

	// Empty struct: {}
	if p.check(lexer.RBrace) {
		p.pos = savedPos
		return true
	}

	// Check for ident: pattern (field initializer)
	if p.check(lexer.Ident) {
		p.advance() // consume ident
		isStruct := p.check(lexer.Colon) || p.check(lexer.Comma) || p.check(lexer.RBrace)
		p.pos = savedPos
		return isStruct
	}

	p.pos = savedPos
	return false
}

func (p *Parser) parseParenOrTuple() Expr {
	pos := p.currentPos()
	p.advance() // consume (

	if p.check(lexer.RParen) {
		// Unit: ()
		p.advance()
		return &TupleExpr{Pos: pos}
	}

	first := p.parseExpression()

	if p.match(lexer.Comma) {
		// Tuple
		elems := []Expr{first}
		if !p.check(lexer.RParen) {
			elems = append(elems, p.parseExpression())
			for p.match(lexer.Comma) {
				if p.check(lexer.RParen) {
					break
				}
				elems = append(elems, p.parseExpression())
			}
		}
		p.expect(lexer.RParen)
		return &TupleExpr{Elements: elems, Pos: pos}
	}

	p.expect(lexer.RParen)
	return &GroupExpr{Inner: first, Pos: pos}
}

func (p *Parser) parseArrayOrListComp() Expr {
	pos := p.currentPos()
	p.advance() // consume [

	if p.check(lexer.RBracket) {
		p.advance()
		return &ArrayExpr{Pos: pos}
	}

	first := p.parseExpression()

	// List comprehension: [expr for x in iter]
	if p.check(lexer.For) {
		p.advance()
		varName := p.expect(lexer.Ident).Literal
		p.expect(lexer.In)
		iter := p.parseExpression()
		var where Expr
		if p.match(lexer.Where) {
			where = p.parseExpression()
		}
		p.expect(lexer.RBracket)
		return &ListCompExpr{Expr: first, Var: varName, Iter: iter, Where: where, Pos: pos}
	}

	// Array literal
	elems := []Expr{first}
	for p.match(lexer.Comma) {
		if p.check(lexer.RBracket) {
			break
		}
		elems = append(elems, p.parseExpression())
	}
	p.expect(lexer.RBracket)
	return &ArrayExpr{Elements: elems, Pos: pos}
}

func (p *Parser) parseIfExpr() Expr {
	pos := p.currentPos()
	p.expect(lexer.If)
	cond := p.parseExpression()
	then := p.parseBlockExpr()

	var elseExpr Expr
	if p.match(lexer.Else) {
		if p.check(lexer.If) {
			elseExpr = p.parseIfExpr()
		} else {
			elseExpr = p.parseBlockExpr()
		}
	}

	return &IfExpr{Cond: cond, Then: then, Else: elseExpr, Pos: pos}
}

func (p *Parser) parseMatchExpr() Expr {
	pos := p.currentPos()
	p.expect(lexer.Match)
	subject := p.parseExpression()

	p.expect(lexer.LBrace)
	p.skipNewlines()

	var arms []*MatchArm
	for !p.check(lexer.RBrace) && !p.isAtEnd() {
		arm := &MatchArm{Pos: p.currentPos()}
		arm.Pattern = p.parsePattern()

		// Optional guard
		if p.match(lexer.If) {
			arm.Guard = p.parseExpression()
		}

		p.expect(lexer.FatArrow)
		arm.Body = p.parseExpression()
		arms = append(arms, arm)

		p.match(lexer.Comma)
		p.skipNewlines()
	}

	p.expect(lexer.RBrace)
	return &MatchExpr{Subject: subject, Arms: arms, Pos: pos}
}

func (p *Parser) parseClosureExpr(move bool) Expr {
	pos := p.currentPos()
	p.expect(lexer.Fn)
	p.expect(lexer.LParen)

	var params []*Param
	if !p.check(lexer.RParen) {
		params = p.parseParamList()
	}
	p.expect(lexer.RParen)

	var retType TypeExpr
	if p.match(lexer.Arrow) {
		retType = p.parseTypeExpr()
	}

	p.expect(lexer.FatArrow)
	body := p.parseExpression()

	return &ClosureExpr{
		Move:   move,
		Params: params,
		Return: retType,
		Body:   body,
		Pos:    pos,
	}
}

func (p *Parser) parseInterpolatedString() Expr {
	pos := p.currentPos()
	var parts []Expr

	// StringStart
	start := p.advance()
	if start.Literal != "" {
		parts = append(parts, &StringLitExpr{Value: start.Literal, Pos: start.Pos})
	}

	// Parse interpolated expression
	parts = append(parts, p.parseExpression())

	// StringMiddle / StringEnd
	for {
		tok := p.peek()
		if tok.Type == lexer.StringEnd {
			p.advance()
			if tok.Literal != "" {
				parts = append(parts, &StringLitExpr{Value: tok.Literal, Pos: tok.Pos})
			}
			break
		} else if tok.Type == lexer.StringMiddle {
			p.advance()
			if tok.Literal != "" {
				parts = append(parts, &StringLitExpr{Value: tok.Literal, Pos: tok.Pos})
			}
			parts = append(parts, p.parseExpression())
		} else {
			p.error("expected string continuation")
			break
		}
	}

	return &InterpolatedStringExpr{Parts: parts, Pos: pos}
}

func (p *Parser) parseAssertExpr() Expr {
	pos := p.currentPos()
	p.expect(lexer.Assert)
	expr := p.parseExpression()
	return &CallExpr{
		Func: &IdentExpr{Name: "assert", Pos: pos},
		Args: []*Arg{{Value: expr, Pos: expr.GetPos()}},
		Pos:  pos,
	}
}

func (p *Parser) parseFieldInitList() []*FieldInit {
	var fields []*FieldInit
	p.skipNewlines()
	for !p.check(lexer.RBrace) && !p.isAtEnd() {
		fi := &FieldInit{Pos: p.currentPos()}
		fi.Name = p.expect(lexer.Ident).Literal
		if p.match(lexer.Colon) {
			fi.Value = p.parseExpression()
		}
		// else: shorthand (name == variable)
		fields = append(fields, fi)
		p.match(lexer.Comma)
		p.skipNewlines()
	}
	return fields
}
