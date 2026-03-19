package parser

import (
	"fmt"

	"github.com/aria-lang/aria/internal/diagnostic"
	"github.com/aria-lang/aria/internal/lexer"
)

// Parser parses a stream of tokens into an AST.
type Parser struct {
	tokens      []lexer.Token
	pos         int
	diagnostics *diagnostic.DiagnosticList
}

// New creates a new parser from a token stream.
func New(tokens []lexer.Token) *Parser {
	return &Parser{
		tokens:      tokens,
		diagnostics: &diagnostic.DiagnosticList{},
	}
}

// Parse parses the full program.
func (p *Parser) Parse() *Program {
	prog := &Program{Pos: p.currentPos()}

	// Module declaration
	if p.check(lexer.Mod) {
		prog.Module = p.parseModDecl()
	}

	// Import declarations
	for p.check(lexer.Use) {
		prog.Imports = append(prog.Imports, p.parseImportDecl())
	}

	// Top-level declarations
	for !p.isAtEnd() {
		decl := p.parseTopLevelDecl()
		if decl != nil {
			prog.Decls = append(prog.Decls, decl)
		} else {
			// Error recovery: skip to next line
			p.advance()
			p.skipNewlines()
		}
	}

	return prog
}

// Diagnostics returns the accumulated diagnostics.
func (p *Parser) Diagnostics() *diagnostic.DiagnosticList {
	return p.diagnostics
}

// ---------- Token helpers ----------

func (p *Parser) peek() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peekType() lexer.TokenType {
	return p.peek().Type
}

func (p *Parser) advance() lexer.Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) check(types ...lexer.TokenType) bool {
	cur := p.peekType()
	for _, t := range types {
		if cur == t {
			return true
		}
	}
	return false
}

func (p *Parser) match(types ...lexer.TokenType) bool {
	if p.check(types...) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expect(typ lexer.TokenType) lexer.Token {
	if p.peekType() == typ {
		return p.advance()
	}
	p.error(fmt.Sprintf("expected %s, got %s", typ, p.peekType()))
	return lexer.Token{Type: lexer.Illegal, Pos: p.currentPos()}
}

func (p *Parser) isAtEnd() bool {
	return p.peekType() == lexer.EOF
}

func (p *Parser) currentPos() Pos {
	return p.peek().Pos
}

func (p *Parser) skipNewlines() {
	for p.match(lexer.Newline) {
	}
}

func (p *Parser) error(msg string) {
	tok := p.peek()
	p.diagnostics.Add(diagnostic.Diagnostic{
		Code:     diagnostic.E0005,
		Severity: diagnostic.Error,
		Message:  msg,
		File:     tok.Pos.File,
		Line:     tok.Pos.Line,
		Column:   tok.Pos.Column,
		Span:     [2]int{tok.Pos.Offset, tok.Pos.Offset + len(tok.Literal)},
		Labels: []diagnostic.Label{{
			File:    tok.Pos.File,
			Line:    tok.Pos.Line,
			Column:  tok.Pos.Column,
			Span:    [2]int{tok.Pos.Offset, tok.Pos.Offset + len(tok.Literal)},
			Message: msg,
			Style:   diagnostic.Primary,
		}},
	})
}

// ---------- Module and Imports ----------

func (p *Parser) parseModDecl() *ModDecl {
	pos := p.currentPos()
	p.expect(lexer.Mod)
	name := p.expect(lexer.Ident)
	p.skipNewlines()
	return &ModDecl{Name: name.Literal, Pos: pos}
}

func (p *Parser) parseImportDecl() *ImportDecl {
	pos := p.currentPos()
	p.expect(lexer.Use)

	imp := &ImportDecl{Pos: pos}

	// Parse first path segment
	first := p.expect(lexer.Ident)
	imp.Path = append(imp.Path, first.Literal)

	// Parse remaining path segments
	for p.match(lexer.Dot) || p.check(lexer.DotBrace) {
		if p.check(lexer.LBrace) || p.check(lexer.DotBrace) {
			// Grouped import: use std.{json, http}
			// .{ may be lexed as a single DotBrace token
			if p.check(lexer.DotBrace) {
				p.advance() // consume .{
			} else {
				p.advance() // consume {
			}
			for {
				name := p.expect(lexer.Ident)
				imp.Names = append(imp.Names, name.Literal)
				if !p.match(lexer.Comma) {
					break
				}
			}
			p.expect(lexer.RBrace)
			break
		}
		name := p.expect(lexer.Ident)
		imp.Path = append(imp.Path, name.Literal)
	}

	// Optional alias
	if p.match(lexer.As) {
		alias := p.expect(lexer.Ident)
		imp.Alias = alias.Literal
	}

	p.skipNewlines()
	return imp
}

// ---------- Top-Level Declarations ----------

func (p *Parser) parseTopLevelDecl() Decl {
	p.skipNewlines()
	if p.isAtEnd() {
		return nil
	}

	// Parse optional visibility
	vis := Private
	visPos := p.currentPos()
	if p.match(lexer.Pub) {
		vis = Public
		if p.match(lexer.LParen) {
			p.expect(lexer.Ident) // "pkg"
			p.expect(lexer.RParen)
			vis = PackagePublic
		}
	}

	switch p.peekType() {
	case lexer.Fn:
		return p.parseFnDecl(vis)
	case lexer.Type:
		return p.parseTypeDecl(vis)
	case lexer.Struct:
		return p.parseStructDecl(vis)
	case lexer.Enum:
		return p.parseEnumDecl(vis)
	case lexer.Trait:
		return p.parseTraitDecl(vis)
	case lexer.Impl:
		return p.parseImplDecl()
	case lexer.Const:
		return p.parseConstDecl(vis)
	case lexer.Entry:
		return p.parseEntryBlock()
	case lexer.Test:
		return p.parseTestBlock()
	case lexer.Alias:
		return p.parseAliasDecl(vis)
	default:
		if vis != Private {
			_ = visPos
			p.error(fmt.Sprintf("expected declaration after visibility modifier, got %s", p.peekType()))
		} else {
			p.error(fmt.Sprintf("expected declaration, got %s", p.peekType()))
		}
		return nil
	}
}

// ---------- Function Declaration ----------

func (p *Parser) parseFnDecl(vis Visibility) *FnDecl {
	pos := p.currentPos()
	p.expect(lexer.Fn)
	name := p.expect(lexer.Ident)

	fn := &FnDecl{
		Vis:  vis,
		Name: name.Literal,
		Pos:  pos,
	}

	// Optional generic params
	if p.check(lexer.LBracket) {
		fn.GenericParams = p.parseGenericParams()
	}

	// Parameters
	p.expect(lexer.LParen)
	if !p.check(lexer.RParen) {
		fn.Params = p.parseParamList()
	}
	p.expect(lexer.RParen)

	// Optional return type (don't consume ! here — that's the error clause)
	if p.match(lexer.Arrow) {
		fn.ReturnType = p.parseTypeExprNoError()
	}

	// Optional error clause
	if p.match(lexer.Bang) {
		fn.ErrorTypes = append(fn.ErrorTypes, p.parseTypeExprNoError())
		for p.match(lexer.Pipe) {
			fn.ErrorTypes = append(fn.ErrorTypes, p.parseTypeExpr())
		}
	}

	// Optional effect clause
	if p.match(lexer.With) {
		p.expect(lexer.LBracket)
		for {
			eff := p.expect(lexer.Ident)
			fn.Effects = append(fn.Effects, eff.Literal)
			if !p.match(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.RBracket)
	}

	// Body
	if p.match(lexer.Eq) {
		fn.Body = p.parseExpression()
	} else if p.check(lexer.LBrace) {
		fn.Body = p.parseBlockExpr()
	}

	p.skipNewlines()
	return fn
}

func (p *Parser) parseGenericParams() []*GenericParam {
	p.expect(lexer.LBracket)
	var params []*GenericParam
	for {
		gp := &GenericParam{Pos: p.currentPos()}
		gp.Name = p.expect(lexer.Ident).Literal
		if p.match(lexer.Colon) {
			gp.Bounds = p.parseTraitBound()
		}
		params = append(params, gp)
		if !p.match(lexer.Comma) {
			break
		}
	}
	p.expect(lexer.RBracket)
	return params
}

func (p *Parser) parseTraitBound() []string {
	var bounds []string
	bounds = append(bounds, p.expect(lexer.Ident).Literal)
	for p.match(lexer.Plus) {
		bounds = append(bounds, p.expect(lexer.Ident).Literal)
	}
	return bounds
}

func (p *Parser) parseParamList() []*Param {
	var params []*Param
	for {
		param := p.parseParam()
		params = append(params, param)
		if !p.match(lexer.Comma) {
			break
		}
		// Allow trailing comma
		if p.check(lexer.RParen) {
			break
		}
	}
	return params
}

func (p *Parser) parseParam() *Param {
	pos := p.currentPos()
	param := &Param{Pos: pos}

	if p.match(lexer.Mut) {
		param.Mutable = true
	}

	// Accept `self` as a parameter name (it's a keyword but valid here)
	if p.check(lexer.Self_) {
		param.Name = p.advance().Literal
		// `self` param has no type annotation (or optional one)
		if p.match(lexer.Colon) {
			param.Type = p.parseTypeExpr()
		}
		return param
	}

	param.Name = p.expect(lexer.Ident).Literal
	p.expect(lexer.Colon)
	param.Type = p.parseTypeExpr()

	// Optional default value
	if p.match(lexer.Eq) {
		param.Default = p.parseExpression()
	}

	return param
}

// ---------- Type Declarations ----------

func (p *Parser) parseTypeDecl(vis Visibility) Decl {
	pos := p.currentPos()
	p.expect(lexer.Type)
	name := p.expect(lexer.Ident)

	td := &TypeDecl{
		Vis:  vis,
		Name: name.Literal,
		Pos:  pos,
	}

	// Optional generic params
	if p.check(lexer.LBracket) {
		td.GenericParams = p.parseGenericParams()
	}

	// type Name { ... } is a struct
	if p.check(lexer.LBrace) {
		td.Kind = StructDecl
		td.Fields = p.parseStructBody()
		if p.check(lexer.Derives) {
			td.Derives = p.parseDerives()
		}
		p.skipNewlines()
		return td
	}

	// type Name = ...
	if !p.match(lexer.Eq) {
		// Just "type Name" without body - error
		p.error("expected '=' or '{' after type name")
		p.skipNewlines()
		return td
	}

	p.skipNewlines()

	// Sum type: type Name = | Variant1 | Variant2
	if p.check(lexer.Pipe) {
		td.Kind = SumTypeDecl
		td.Variants = p.parseSumVariants()
		if p.check(lexer.Derives) {
			td.Derives = p.parseDerives()
		}
		p.skipNewlines()
		return td
	}

	// Newtype: type Name = ExistingType
	td.Kind = NewtypeDecl
	td.Underlying = p.parseTypeExpr()
	if p.check(lexer.Derives) {
		td.Derives = p.parseDerives()
	}
	p.skipNewlines()
	return td
}

func (p *Parser) parseStructDecl(vis Visibility) *TypeDecl {
	pos := p.currentPos()
	p.expect(lexer.Struct)
	name := p.expect(lexer.Ident)

	td := &TypeDecl{
		Vis:  vis,
		Name: name.Literal,
		Kind: StructDecl,
		Pos:  pos,
	}

	// Optional generic params
	if p.check(lexer.LBracket) {
		td.GenericParams = p.parseGenericParams()
	}

	td.Fields = p.parseStructBody()
	if p.check(lexer.Derives) {
		td.Derives = p.parseDerives()
	}
	p.skipNewlines()
	return td
}

func (p *Parser) parseStructBody() []*FieldDecl {
	p.expect(lexer.LBrace)
	p.skipNewlines()
	var fields []*FieldDecl
	for !p.check(lexer.RBrace) && !p.isAtEnd() {
		field := p.parseFieldDecl()
		fields = append(fields, field)
		p.match(lexer.Comma)
		p.skipNewlines()
	}
	p.expect(lexer.RBrace)
	return fields
}

func (p *Parser) parseFieldDecl() *FieldDecl {
	pos := p.currentPos()
	field := &FieldDecl{Pos: pos}

	if p.match(lexer.Pub) {
		field.Vis = Public
	}

	field.Name = p.expect(lexer.Ident).Literal
	p.expect(lexer.Colon)
	field.Type = p.parseTypeExpr()

	if p.match(lexer.Eq) {
		field.Default = p.parseExpression()
	}

	return field
}

func (p *Parser) parseSumVariants() []*VariantDecl {
	var variants []*VariantDecl
	for p.match(lexer.Pipe) {
		p.skipNewlines()
		v := &VariantDecl{Pos: p.currentPos()}
		v.Name = p.expect(lexer.Ident).Literal

		if p.check(lexer.LBrace) {
			// Struct variant: | Variant { field: type }
			v.Fields = p.parseStructBody()
		} else if p.match(lexer.LParen) {
			// Tuple variant: | Variant(type1, type2)
			for {
				v.Types = append(v.Types, p.parseTypeExpr())
				if !p.match(lexer.Comma) {
					break
				}
			}
			p.expect(lexer.RParen)
		}
		// else: unit variant (no payload)

		p.skipNewlines()
		variants = append(variants, v)
	}
	return variants
}

func (p *Parser) parseDerives() []string {
	p.expect(lexer.Derives)
	p.expect(lexer.LBracket)
	var derives []string
	for {
		derives = append(derives, p.expect(lexer.Ident).Literal)
		if !p.match(lexer.Comma) {
			break
		}
	}
	p.expect(lexer.RBracket)
	return derives
}

// ---------- Enum, Trait, Impl, Const, Entry, Test, Alias ----------

func (p *Parser) parseEnumDecl(vis Visibility) *EnumDecl {
	pos := p.currentPos()
	p.expect(lexer.Enum)
	name := p.expect(lexer.Ident)
	p.expect(lexer.LBrace)
	p.skipNewlines()

	var members []string
	for !p.check(lexer.RBrace) && !p.isAtEnd() {
		members = append(members, p.expect(lexer.Ident).Literal)
		p.match(lexer.Comma)
		p.skipNewlines()
	}
	p.expect(lexer.RBrace)
	p.skipNewlines()
	return &EnumDecl{Vis: vis, Name: name.Literal, Members: members, Pos: pos}
}

func (p *Parser) parseTraitDecl(vis Visibility) *TraitDecl {
	pos := p.currentPos()
	p.expect(lexer.Trait)
	name := p.expect(lexer.Ident)

	td := &TraitDecl{Vis: vis, Name: name.Literal, Pos: pos}

	if p.check(lexer.LBracket) {
		td.GenericParams = p.parseGenericParams()
	}

	// Supertraits
	if p.match(lexer.Colon) {
		td.Supertraits = p.parseTraitBound()
	}

	p.expect(lexer.LBrace)
	p.skipNewlines()
	for !p.check(lexer.RBrace) && !p.isAtEnd() {
		method := p.parseFnDecl(Private)
		td.Methods = append(td.Methods, method)
		p.skipNewlines()
	}
	p.expect(lexer.RBrace)
	p.skipNewlines()
	return td
}

func (p *Parser) parseImplDecl() *ImplDecl {
	pos := p.currentPos()
	p.expect(lexer.Impl)

	impl := &ImplDecl{Pos: pos}

	// Optional generic params
	if p.check(lexer.LBracket) {
		impl.GenericParams = p.parseGenericParams()
	}

	// impl TraitName for TypeName or impl TypeName
	firstName := p.expect(lexer.Ident).Literal

	if p.match(lexer.For) {
		impl.TraitName = firstName
		impl.TypeName = p.expect(lexer.Ident).Literal
	} else {
		impl.TypeName = firstName
	}

	// Optional type args
	if p.check(lexer.LBracket) {
		p.advance()
		for {
			impl.TypeArgs = append(impl.TypeArgs, p.parseTypeExpr())
			if !p.match(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.RBracket)
	}

	// Optional where clause
	if p.check(lexer.Where) {
		impl.WhereClause = p.parseWhereClause()
	}

	p.expect(lexer.LBrace)
	p.skipNewlines()
	for !p.check(lexer.RBrace) && !p.isAtEnd() {
		method := p.parseFnDecl(Private)
		impl.Methods = append(impl.Methods, method)
		p.skipNewlines()
	}
	p.expect(lexer.RBrace)
	p.skipNewlines()
	return impl
}

func (p *Parser) parseWhereClause() []*WhereItem {
	p.expect(lexer.Where)
	var items []*WhereItem
	for {
		item := &WhereItem{Pos: p.currentPos()}
		item.Name = p.expect(lexer.Ident).Literal
		p.expect(lexer.Colon)
		item.Bounds = p.parseTraitBound()
		items = append(items, item)
		if !p.match(lexer.Comma) {
			break
		}
	}
	return items
}

func (p *Parser) parseConstDecl(vis Visibility) *ConstDecl {
	pos := p.currentPos()
	p.expect(lexer.Const)
	name := p.expect(lexer.Ident)

	cd := &ConstDecl{Vis: vis, Name: name.Literal, Pos: pos}

	if p.match(lexer.Colon) {
		cd.Type = p.parseTypeExpr()
	}

	p.expect(lexer.Eq)
	cd.Value = p.parseExpression()
	p.skipNewlines()
	return cd
}

func (p *Parser) parseEntryBlock() *EntryBlock {
	pos := p.currentPos()
	p.expect(lexer.Entry)
	body := p.parseBlockExpr()
	p.skipNewlines()
	return &EntryBlock{Body: body, Pos: pos}
}

func (p *Parser) parseTestBlock() *TestBlock {
	pos := p.currentPos()
	p.expect(lexer.Test)

	var name string
	if p.check(lexer.StringLit) {
		name = p.advance().Literal
	} else {
		name = p.expect(lexer.Ident).Literal
	}

	body := p.parseBlockExpr()
	p.skipNewlines()
	return &TestBlock{Name: name, Body: body, Pos: pos}
}

func (p *Parser) parseAliasDecl(vis Visibility) *AliasDecl {
	pos := p.currentPos()
	p.expect(lexer.Alias)
	name := p.expect(lexer.Ident)
	p.expect(lexer.Eq)
	target := p.parseTypeExpr()
	p.skipNewlines()
	return &AliasDecl{Vis: vis, Name: name.Literal, Target: target, Pos: pos}
}

// ---------- Statements ----------

func (p *Parser) parseStatement() Stmt {
	p.skipNewlines()

	switch p.peekType() {
	case lexer.Mut:
		return p.parseVarDecl(true)
	case lexer.For:
		return p.parseForStmt()
	case lexer.While:
		return p.parseWhileStmt()
	case lexer.Loop:
		return p.parseLoopStmt()
	case lexer.Return:
		return p.parseReturnStmt()
	case lexer.Break:
		return p.parseBreakStmt()
	case lexer.Continue:
		return p.parseContinueStmt()
	case lexer.Defer:
		return p.parseDeferStmt()
	default:
		// Could be: var decl (name := expr), assignment (expr = expr), or expr stmt
		return p.parseExprOrVarDecl()
	}
}

func (p *Parser) parseVarDecl(mutable bool) *VarDeclStmt {
	pos := p.currentPos()
	if mutable {
		p.expect(lexer.Mut)
	}

	name := p.expect(lexer.Ident).Literal

	stmt := &VarDeclStmt{Mutable: mutable, Name: name, Pos: pos}

	if p.match(lexer.Colon) {
		// name: type = expr
		stmt.Type = p.parseTypeExpr()
		p.expect(lexer.Eq)
	} else {
		// name := expr
		p.expect(lexer.ColonEq)
	}

	stmt.Value = p.parseExpression()
	p.skipNewlines()
	return stmt
}

func (p *Parser) parseExprOrVarDecl() Stmt {
	pos := p.currentPos()

	// Check for name := expr pattern
	if p.check(lexer.Ident) {
		// Look ahead for := or : type =
		savedPos := p.pos
		p.advance() // consume ident

		if p.check(lexer.ColonEq) {
			// It's a var decl: name := expr
			p.pos = savedPos
			return p.parseVarDecl(false)
		}

		if p.check(lexer.Colon) {
			// Could be name: type = expr (annotated var decl)
			// or could be part of an expression
			// Look for type followed by =
			p.pos = savedPos
			return p.parseVarDecl(false)
		}

		// Restore position and parse as expression
		p.pos = savedPos
	}

	expr := p.parseExpression()

	// Check for assignment: expr = value
	if p.match(lexer.Eq) {
		value := p.parseExpression()
		p.skipNewlines()
		return &AssignStmt{Target: expr, Value: value, Pos: pos}
	}

	p.skipNewlines()
	return &ExprStmt{Expr: expr, Pos: pos}
}

func (p *Parser) parseForStmt() *ForStmt {
	pos := p.currentPos()
	p.expect(lexer.For)
	pattern := p.parsePattern()
	p.expect(lexer.In)
	iter := p.parseExpression()

	var where Expr
	if p.match(lexer.Where) {
		where = p.parseExpression()
	}

	body := p.parseBlockExpr()
	p.skipNewlines()
	return &ForStmt{Pattern: pattern, Iter: iter, Where: where, Body: body, Pos: pos}
}

func (p *Parser) parseWhileStmt() *WhileStmt {
	pos := p.currentPos()
	p.expect(lexer.While)
	cond := p.parseExpression()
	body := p.parseBlockExpr()
	p.skipNewlines()
	return &WhileStmt{Cond: cond, Body: body, Pos: pos}
}

func (p *Parser) parseLoopStmt() *LoopStmt {
	pos := p.currentPos()
	p.expect(lexer.Loop)
	body := p.parseBlockExpr()
	p.skipNewlines()
	return &LoopStmt{Body: body, Pos: pos}
}

func (p *Parser) parseReturnStmt() *ReturnStmt {
	pos := p.currentPos()
	p.expect(lexer.Return)
	var value Expr
	if !p.check(lexer.Newline) && !p.check(lexer.RBrace) && !p.isAtEnd() {
		value = p.parseExpression()
	}
	p.skipNewlines()
	return &ReturnStmt{Value: value, Pos: pos}
}

func (p *Parser) parseBreakStmt() *BreakStmt {
	pos := p.currentPos()
	p.expect(lexer.Break)
	var value Expr
	if !p.check(lexer.Newline) && !p.check(lexer.RBrace) && !p.isAtEnd() {
		value = p.parseExpression()
	}
	p.skipNewlines()
	return &BreakStmt{Value: value, Pos: pos}
}

func (p *Parser) parseContinueStmt() *ContinueStmt {
	pos := p.currentPos()
	p.expect(lexer.Continue)
	p.skipNewlines()
	return &ContinueStmt{Pos: pos}
}

func (p *Parser) parseDeferStmt() *DeferStmt {
	pos := p.currentPos()
	p.expect(lexer.Defer)
	var expr Expr
	if p.check(lexer.LBrace) {
		expr = p.parseBlockExpr()
	} else {
		expr = p.parseExpression()
	}
	p.skipNewlines()
	return &DeferStmt{Expr: expr, Pos: pos}
}

// ---------- Block ----------

func (p *Parser) parseBlockExpr() *BlockExpr {
	pos := p.currentPos()
	p.expect(lexer.LBrace)
	p.skipNewlines()

	block := &BlockExpr{Pos: pos}

	for !p.check(lexer.RBrace) && !p.isAtEnd() {
		stmt := p.parseStatement()
		if stmt == nil {
			break
		}
		block.Stmts = append(block.Stmts, stmt)
		p.skipNewlines()
	}

	// The last statement might actually be a trailing expression
	// Convert ExprStmt to trailing expression
	if len(block.Stmts) > 0 {
		if last, ok := block.Stmts[len(block.Stmts)-1].(*ExprStmt); ok {
			block.Expr = last.Expr
			block.Stmts = block.Stmts[:len(block.Stmts)-1]
		}
	}

	p.expect(lexer.RBrace)
	return block
}

// ---------- Type Expressions ----------

// parseTypeExprNoError parses a type without consuming ! (used in fn return types
// where ! starts the error clause, not a result type).
func (p *Parser) parseTypeExprNoError() TypeExpr {
	t := p.parsePrimaryType()

	// Postfix ? for optional
	if p.match(lexer.Question) {
		t = &OptionalTypeExpr{Inner: t, Pos: t.GetPos()}
	}

	// -> for function type (type -> type)
	if p.match(lexer.Arrow) {
		ret := p.parseTypeExpr()
		t = &FunctionTypeExpr{Params: []TypeExpr{t}, Return: ret, Pos: t.GetPos()}
	}

	return t
}

func (p *Parser) parseTypeExpr() TypeExpr {
	t := p.parsePrimaryType()

	// Postfix ? for optional
	if p.match(lexer.Question) {
		t = &OptionalTypeExpr{Inner: t, Pos: t.GetPos()}
	}

	// Postfix ! for result type
	if p.match(lexer.Bang) {
		errType := p.parsePrimaryType()
		t = &ResultTypeExpr{Ok: t, Err: errType, Pos: t.GetPos()}
	}

	// -> for function type (type -> type)
	if p.match(lexer.Arrow) {
		ret := p.parseTypeExpr()
		t = &FunctionTypeExpr{Params: []TypeExpr{t}, Return: ret, Pos: t.GetPos()}
	}

	return t
}

func (p *Parser) parsePrimaryType() TypeExpr {
	pos := p.currentPos()

	switch p.peekType() {
	case lexer.LBracket:
		// Array type: [T]
		p.advance()
		elem := p.parseTypeExpr()
		p.expect(lexer.RBracket)
		return &ArrayTypeExpr{Element: elem, Pos: pos}

	case lexer.LBrace:
		// Map type {K: V} or Set type {T}
		p.advance()
		first := p.parseTypeExpr()
		if p.match(lexer.Colon) {
			// Map type
			val := p.parseTypeExpr()
			p.expect(lexer.RBrace)
			return &MapTypeExpr{Key: first, Value: val, Pos: pos}
		}
		// Set type
		p.expect(lexer.RBrace)
		return &SetTypeExpr{Element: first, Pos: pos}

	case lexer.LParen:
		// Tuple type or grouped type or function type
		p.advance()
		first := p.parseTypeExpr()
		if p.match(lexer.Comma) {
			// Tuple type
			elems := []TypeExpr{first}
			elems = append(elems, p.parseTypeExpr())
			for p.match(lexer.Comma) {
				if p.check(lexer.RParen) {
					break
				}
				elems = append(elems, p.parseTypeExpr())
			}
			p.expect(lexer.RParen)
			return &TupleTypeExpr{Elements: elems, Pos: pos}
		}
		p.expect(lexer.RParen)
		// Check for function type: (T) -> U
		if p.match(lexer.Arrow) {
			ret := p.parseTypeExpr()
			return &FunctionTypeExpr{Params: []TypeExpr{first}, Return: ret, Pos: pos}
		}
		return first // grouped type

	case lexer.Fn:
		// fn(A, B) -> C
		p.advance()
		p.expect(lexer.LParen)
		var params []TypeExpr
		if !p.check(lexer.RParen) {
			params = append(params, p.parseTypeExpr())
			for p.match(lexer.Comma) {
				params = append(params, p.parseTypeExpr())
			}
		}
		p.expect(lexer.RParen)
		p.expect(lexer.Arrow)
		ret := p.parseTypeExpr()
		return &FunctionTypeExpr{Params: params, Return: ret, Pos: pos}

	case lexer.Ident, lexer.Self_, lexer.SelfTy:
		return p.parseNamedType()

	default:
		p.error(fmt.Sprintf("expected type, got %s", p.peekType()))
		return &NamedTypeExpr{Path: []string{"?"}, Pos: pos}
	}
}

func (p *Parser) parseNamedType() *NamedTypeExpr {
	pos := p.currentPos()
	name := p.advance() // ident
	path := []string{name.Literal}

	// Dotted path
	for p.check(lexer.Dot) && p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].Type == lexer.Ident {
		p.advance() // consume .
		path = append(path, p.advance().Literal)
	}

	t := &NamedTypeExpr{Path: path, Pos: pos}

	// Generic args [T, U]
	if p.check(lexer.LBracket) {
		p.advance()
		for {
			t.TypeArgs = append(t.TypeArgs, p.parseTypeExpr())
			if !p.match(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.RBracket)
	}

	return t
}

// ---------- Patterns ----------

func (p *Parser) parsePattern() Pattern {
	pat := p.parsePrimaryPattern()

	// Or-pattern: pat | pat
	if p.match(lexer.Pipe) {
		right := p.parsePattern()
		pat = &OrPattern{Left: pat, Right: right, Pos: pat.GetPos()}
	}

	// Named pattern: name @ pattern
	if ident, ok := pat.(*BindingPattern); ok && p.match(lexer.At) {
		inner := p.parsePattern()
		return &NamedPattern{Name: ident.Name, Pattern: inner, Pos: ident.Pos}
	}

	return pat
}

func (p *Parser) parsePrimaryPattern() Pattern {
	pos := p.currentPos()

	switch p.peekType() {
	case lexer.Ident:
		name := p.advance().Literal
		if name == "_" {
			return &WildcardPattern{Pos: pos}
		}

		// Variant pattern: Name(args) or Name { fields }
		if p.check(lexer.LParen) {
			p.advance()
			var args []Pattern
			if !p.check(lexer.RParen) {
				args = append(args, p.parsePattern())
				for p.match(lexer.Comma) {
					args = append(args, p.parsePattern())
				}
			}
			p.expect(lexer.RParen)
			return &VariantPattern{Name: name, Args: args, Pos: pos}
		}

		// Struct pattern: Name { field, field: pat, .. }
		if p.check(lexer.LBrace) {
			p.advance()
			p.skipNewlines()
			var fields []*FieldPattern
			rest := false
			for !p.check(lexer.RBrace) && !p.isAtEnd() {
				if p.match(lexer.DotDot) {
					rest = true
					break
				}
				fp := &FieldPattern{Pos: p.currentPos()}
				fp.Name = p.expect(lexer.Ident).Literal
				if p.match(lexer.Colon) {
					fp.Pattern = p.parsePattern()
				}
				fields = append(fields, fp)
				p.match(lexer.Comma)
				p.skipNewlines()
			}
			p.expect(lexer.RBrace)
			return &StructPattern{TypeName: name, Fields: fields, Rest: rest, Pos: pos}
		}

		return &BindingPattern{Name: name, Pos: pos}

	case lexer.Mut:
		p.advance()
		name := p.expect(lexer.Ident).Literal
		return &BindingPattern{Mutable: true, Name: name, Pos: pos}

	case lexer.IntLit:
		tok := p.advance()
		return &LiteralPattern{Value: &IntLitExpr{Value: tok.Literal, Pos: pos}, Pos: pos}

	case lexer.FloatLit:
		tok := p.advance()
		return &LiteralPattern{Value: &FloatLitExpr{Value: tok.Literal, Pos: pos}, Pos: pos}

	case lexer.StringLit:
		tok := p.advance()
		return &LiteralPattern{Value: &StringLitExpr{Value: tok.Literal, Pos: pos}, Pos: pos}

	case lexer.BoolLit:
		tok := p.advance()
		return &LiteralPattern{Value: &BoolLitExpr{Value: tok.Literal == "true", Pos: pos}, Pos: pos}

	case lexer.LParen:
		// Tuple pattern
		p.advance()
		var elems []Pattern
		if !p.check(lexer.RParen) {
			elems = append(elems, p.parsePattern())
			for p.match(lexer.Comma) {
				if p.check(lexer.RParen) {
					break
				}
				elems = append(elems, p.parsePattern())
			}
		}
		p.expect(lexer.RParen)
		return &TuplePattern{Elements: elems, Pos: pos}

	case lexer.LBracket:
		// Array pattern
		p.advance()
		var elems []Pattern
		restName := ""
		hasRest := false
		for !p.check(lexer.RBracket) && !p.isAtEnd() {
			if p.match(lexer.DotDot) {
				hasRest = true
				if p.check(lexer.Ident) {
					restName = p.advance().Literal
				}
				break
			}
			elems = append(elems, p.parsePattern())
			if !p.match(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.RBracket)
		return &ArrayPattern{Elements: elems, Rest: restName, HasRest: hasRest, Pos: pos}

	case lexer.DotDot:
		// Rest pattern
		p.advance()
		name := ""
		if p.check(lexer.Ident) {
			name = p.advance().Literal
		}
		return &RestPattern{Name: name, Pos: pos}

	default:
		p.error(fmt.Sprintf("expected pattern, got %s", p.peekType()))
		return &WildcardPattern{Pos: pos}
	}
}
