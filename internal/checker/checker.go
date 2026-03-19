package checker

import (
	"fmt"

	"github.com/aria-lang/aria/internal/diagnostic"
	"github.com/aria-lang/aria/internal/lexer"
	"github.com/aria-lang/aria/internal/parser"
	"github.com/aria-lang/aria/internal/resolver"
)

// FnContext tracks the current function's constraints during checking.
type FnContext struct {
	ReturnType Type
	ErrorTypes []Type
	Effects    []string
	InLoop     bool
}

// TypeEnv tracks types of variables/params in scope.
type TypeEnv struct {
	bindings map[string]Type
	parent   *TypeEnv
}

func NewTypeEnv(parent *TypeEnv) *TypeEnv {
	return &TypeEnv{bindings: make(map[string]Type), parent: parent}
}

func (e *TypeEnv) Set(name string, t Type) {
	e.bindings[name] = t
}

func (e *TypeEnv) Get(name string) Type {
	for env := e; env != nil; env = env.parent {
		if t, ok := env.bindings[name]; ok {
			return t
		}
	}
	return nil
}

// Checker performs type checking on a resolved AST.
type Checker struct {
	scope       *resolver.Scope
	diagnostics *diagnostic.DiagnosticList
	types       map[string]Type          // type name -> resolved type
	fnTypes     map[string]*FunctionType // fn name -> function type
	fnCtx       *FnContext               // current function context
	typeEnv     *TypeEnv                 // current variable type bindings
	traits      *TraitRegistry
}

// New creates a new Checker.
func New(scope *resolver.Scope) *Checker {
	c := &Checker{
		scope:       scope,
		diagnostics: &diagnostic.DiagnosticList{},
		types:       make(map[string]Type),
		fnTypes:     make(map[string]*FunctionType),
		typeEnv:     NewTypeEnv(nil),
		traits:      NewTraitRegistry(),
	}
	return c
}

// Check type-checks the entire program.
func (c *Checker) Check(prog *parser.Program) {
	// Phase 1: Register all type declarations
	c.registerTypes(prog)

	// Phase 2: Register function signatures
	c.registerFunctions(prog)

	// Phase 3: Register trait and impl declarations
	c.registerTraitsAndImpls(prog)

	// Phase 4: Check all declaration bodies
	for _, decl := range prog.Decls {
		c.checkDecl(decl)
	}
}

// Diagnostics returns accumulated diagnostics.
func (c *Checker) Diagnostics() *diagnostic.DiagnosticList {
	return c.diagnostics
}

// ---------- Phase 1: Register types ----------

func (c *Checker) registerTypes(prog *parser.Program) {
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *parser.TypeDecl:
			c.registerTypeDecl(d)
		case *parser.EnumDecl:
			c.registerEnumDecl(d)
		case *parser.AliasDecl:
			c.registerAliasDecl(d)
		}
	}
}

func (c *Checker) registerTypeDecl(td *parser.TypeDecl) {
	switch td.Kind {
	case parser.StructDecl:
		st := &StructType{Name: td.Name}
		for _, f := range td.Fields {
			st.Fields = append(st.Fields, StructField{
				Name:    f.Name,
				Type:    c.resolveTypeExpr(f.Type),
				Default: f.Default != nil,
			})
		}
		c.types[td.Name] = st

	case parser.SumTypeDecl:
		sum := &SumType{Name: td.Name}
		for _, v := range td.Variants {
			sv := SumVariant{Name: v.Name}
			for _, f := range v.Fields {
				sv.Fields = append(sv.Fields, StructField{
					Name: f.Name,
					Type: c.resolveTypeExpr(f.Type),
				})
			}
			for _, t := range v.Types {
				sv.Types = append(sv.Types, c.resolveTypeExpr(t))
			}
			sum.Variants = append(sum.Variants, sv)
		}
		c.types[td.Name] = sum

	case parser.NewtypeDecl:
		c.types[td.Name] = &NewtypeType{
			Name:       td.Name,
			Underlying: c.resolveTypeExpr(td.Underlying),
		}
	}
}

func (c *Checker) registerEnumDecl(ed *parser.EnumDecl) {
	c.types[ed.Name] = &EnumType{
		Name:    ed.Name,
		Members: ed.Members,
	}
}

func (c *Checker) registerAliasDecl(ad *parser.AliasDecl) {
	c.types[ad.Name] = &AliasType{
		Name:   ad.Name,
		Target: c.resolveTypeExpr(ad.Target),
	}
}

// ---------- Phase 2: Register function signatures ----------

func (c *Checker) registerFunctions(prog *parser.Program) {
	for _, decl := range prog.Decls {
		if fn, ok := decl.(*parser.FnDecl); ok {
			c.registerFnDecl(fn)
		}
	}
}

func (c *Checker) registerFnDecl(fn *parser.FnDecl) {
	ft := &FunctionType{
		Effects: fn.Effects,
	}
	for _, p := range fn.Params {
		if p.Name == "self" {
			// self param type resolved from context
			continue
		}
		ft.Params = append(ft.Params, c.resolveTypeExpr(p.Type))
	}
	if fn.ReturnType != nil {
		ft.Return = c.resolveTypeExpr(fn.ReturnType)
	} else {
		ft.Return = TypeUnit
	}
	for _, et := range fn.ErrorTypes {
		ft.Errors = append(ft.Errors, c.resolveTypeExpr(et))
	}
	c.fnTypes[fn.Name] = ft
}

// ---------- Phase 3: Register traits and impls ----------

func (c *Checker) registerTraitsAndImpls(prog *parser.Program) {
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *parser.TraitDecl:
			c.registerTraitDecl(d)
		case *parser.ImplDecl:
			c.registerImplDecl(d)
		}
	}
}

func (c *Checker) registerTraitDecl(td *parser.TraitDecl) {
	trait := &TraitDef{
		Name:        td.Name,
		Supertraits: td.Supertraits,
	}
	for _, m := range td.Methods {
		trait.Methods = append(trait.Methods, TraitMethod{
			Name:       m.Name,
			HasDefault: m.Body != nil,
		})
	}
	c.traits.RegisterTrait(trait)
}

func (c *Checker) registerImplDecl(impl *parser.ImplDecl) {
	if impl.TraitName == "" {
		return // inherent impls don't affect trait resolution
	}
	var methods []string
	for _, m := range impl.Methods {
		methods = append(methods, m.Name)
	}
	c.traits.RegisterImpl(impl.TraitName, impl.TypeName, methods)
}

// ---------- Phase 4: Check declaration bodies ----------

func (c *Checker) checkDecl(decl parser.Decl) {
	switch d := decl.(type) {
	case *parser.FnDecl:
		c.checkFnDecl(d)
	case *parser.ConstDecl:
		c.checkConstDecl(d)
	case *parser.EntryBlock:
		c.checkEntryBlock(d)
	case *parser.TestBlock:
		c.checkTestBlock(d)
	case *parser.ImplDecl:
		c.checkImplDecl(d)
	case *parser.TypeDecl:
		c.checkTypeDecl(d)
	}
}

func (c *Checker) checkFnDecl(fn *parser.FnDecl) {
	ft := c.fnTypes[fn.Name]
	if ft == nil {
		return
	}

	prevCtx := c.fnCtx
	prevEnv := c.typeEnv
	c.fnCtx = &FnContext{
		ReturnType: ft.Return,
		ErrorTypes: ft.Errors,
		Effects:    ft.Effects,
	}
	c.typeEnv = NewTypeEnv(prevEnv)

	// Register parameter types
	paramIdx := 0
	for _, p := range fn.Params {
		if p.Name == "self" {
			continue
		}
		if paramIdx < len(ft.Params) {
			c.typeEnv.Set(p.Name, ft.Params[paramIdx])
			paramIdx++
		}
	}

	if fn.Body != nil {
		bodyType := c.checkExpr(fn.Body)
		if bodyType != nil && ft.Return != nil {
			if !IsAssignable(bodyType, ft.Return) {
				c.error(fn.Pos, diagnostic.E0106,
					fmt.Sprintf("function '%s' return type mismatch: expected %s, got %s",
						fn.Name, ft.Return, bodyType))
			}
		}
	}

	c.typeEnv = prevEnv
	c.fnCtx = prevCtx
}

func (c *Checker) checkConstDecl(cd *parser.ConstDecl) {
	if cd.Value != nil {
		c.checkExpr(cd.Value)
	}
}

func (c *Checker) checkEntryBlock(eb *parser.EntryBlock) {
	prevCtx := c.fnCtx
	prevEnv := c.typeEnv
	c.fnCtx = &FnContext{
		ReturnType: TypeUnit,
		Effects:    []string{"Io", "Fs", "Net", "Async"},
	}
	c.typeEnv = NewTypeEnv(prevEnv)
	c.checkExpr(eb.Body)
	c.typeEnv = prevEnv
	c.fnCtx = prevCtx
}

func (c *Checker) checkTestBlock(tb *parser.TestBlock) {
	prevCtx := c.fnCtx
	prevEnv := c.typeEnv
	c.fnCtx = &FnContext{
		ReturnType: TypeUnit,
		Effects:    []string{"Io", "Fs", "Net", "Async"},
	}
	c.typeEnv = NewTypeEnv(prevEnv)
	c.checkExpr(tb.Body)
	c.typeEnv = prevEnv
	c.fnCtx = prevCtx
}

func (c *Checker) checkImplDecl(impl *parser.ImplDecl) {
	// Verify all required trait methods are implemented
	if impl.TraitName != "" {
		traitDef := c.traits.LookupTrait(impl.TraitName)
		if traitDef != nil {
			provided := make(map[string]bool)
			for _, m := range impl.Methods {
				provided[m.Name] = true
			}
			for _, req := range traitDef.Methods {
				if !req.HasDefault && !provided[req.Name] {
					c.error(impl.Pos, diagnostic.E0202,
						fmt.Sprintf("missing required method '%s' for trait '%s'",
							req.Name, impl.TraitName))
				}
			}
		}
	}

	// Check method bodies
	for _, m := range impl.Methods {
		c.checkFnBody(m)
	}
}

func (c *Checker) checkFnBody(fn *parser.FnDecl) {
	if fn.Body == nil {
		return
	}
	prevCtx := c.fnCtx
	prevEnv := c.typeEnv
	var ret Type = TypeUnit
	if fn.ReturnType != nil {
		ret = c.resolveTypeExpr(fn.ReturnType)
	}
	var errors []Type
	for _, et := range fn.ErrorTypes {
		errors = append(errors, c.resolveTypeExpr(et))
	}
	c.fnCtx = &FnContext{
		ReturnType: ret,
		ErrorTypes: errors,
		Effects:    fn.Effects,
	}
	c.typeEnv = NewTypeEnv(prevEnv)
	for _, p := range fn.Params {
		if p.Name == "self" {
			continue
		}
		if p.Type != nil {
			c.typeEnv.Set(p.Name, c.resolveTypeExpr(p.Type))
		}
	}
	c.checkExpr(fn.Body)
	c.typeEnv = prevEnv
	c.fnCtx = prevCtx
}

func (c *Checker) checkTypeDecl(td *parser.TypeDecl) {
	// Validate derives
	for _, derive := range td.Derives {
		if !c.traits.IsDerivable(derive) {
			c.error(td.Pos, diagnostic.E0205,
				fmt.Sprintf("trait '%s' cannot be derived", derive))
		}
	}
}

// ---------- Check expressions ----------

func (c *Checker) checkExpr(expr parser.Expr) Type {
	if expr == nil {
		return TypeUnit
	}

	switch e := expr.(type) {
	case *parser.IntLitExpr:
		return TypeI64

	case *parser.FloatLitExpr:
		return TypeF64

	case *parser.StringLitExpr:
		return TypeStr

	case *parser.BoolLitExpr:
		return TypeBool

	case *parser.InterpolatedStringExpr:
		for _, part := range e.Parts {
			c.checkExpr(part)
		}
		return TypeStr

	case *parser.IdentExpr:
		return c.checkIdent(e)

	case *parser.PathExpr:
		// Path expressions (module.name) — type checked when we have module resolution
		return &UnresolvedType{Name: e.Parts[len(e.Parts)-1]}

	case *parser.BinaryExpr:
		return c.checkBinaryExpr(e)

	case *parser.UnaryExpr:
		return c.checkUnaryExpr(e)

	case *parser.PostfixExpr:
		return c.checkPostfixExpr(e)

	case *parser.CallExpr:
		return c.checkCallExpr(e)

	case *parser.MethodCallExpr:
		c.checkExpr(e.Object)
		for _, arg := range e.Args {
			c.checkExpr(arg.Value)
		}
		// Infer return types for known built-in methods
		return c.inferMethodReturnType(e.Method)

	case *parser.FieldAccessExpr:
		objType := c.checkExpr(e.Object)
		return c.checkFieldAccess(objType, e.Field, e.Pos)

	case *parser.OptionalChainExpr:
		objType := c.checkExpr(e.Object)
		innerType := c.checkFieldAccess(objType, e.Field, e.Pos)
		return &OptionalType{Inner: innerType}

	case *parser.IndexExpr:
		objType := c.checkExpr(e.Object)
		c.checkExpr(e.Index)
		if at, ok := Unwrap(objType).(*ArrayType); ok {
			return at.Element
		}
		return &UnresolvedType{Name: "index"}

	case *parser.PipelineExpr:
		leftType := c.checkExpr(e.Left)
		_ = leftType
		return c.checkExpr(e.Right)

	case *parser.RangeExpr:
		startType := c.checkExpr(e.Start)
		endType := c.checkExpr(e.End)
		if !IsAssignable(startType, endType) {
			c.error(e.Pos, diagnostic.E0100,
				fmt.Sprintf("range bounds type mismatch: %s vs %s", startType, endType))
		}
		return &UnresolvedType{Name: "Range"}

	case *parser.BlockExpr:
		return c.checkBlockExpr(e)

	case *parser.IfExpr:
		return c.checkIfExpr(e)

	case *parser.MatchExpr:
		return c.checkMatchExpr(e)

	case *parser.ClosureExpr:
		return c.checkClosureExpr(e)

	case *parser.StructExpr:
		return c.checkStructExpr(e)

	case *parser.ArrayExpr:
		return c.checkArrayExpr(e)

	case *parser.TupleExpr:
		var elemTypes []Type
		for _, el := range e.Elements {
			elemTypes = append(elemTypes, c.checkExpr(el))
		}
		if len(elemTypes) == 0 {
			return TypeUnit
		}
		return &TupleType{Elements: elemTypes}

	case *parser.MapExpr:
		if len(e.Entries) == 0 {
			return &MapType{Key: &UnresolvedType{Name: "K"}, Value: &UnresolvedType{Name: "V"}}
		}
		keyType := c.checkExpr(e.Entries[0].Key)
		valType := c.checkExpr(e.Entries[0].Value)
		for _, entry := range e.Entries[1:] {
			c.checkExpr(entry.Key)
			c.checkExpr(entry.Value)
		}
		return &MapType{Key: keyType, Value: valType}

	case *parser.ListCompExpr:
		c.checkExpr(e.Iter)
		elemType := c.checkExpr(e.Expr)
		if e.Where != nil {
			c.checkExpr(e.Where)
		}
		return &ArrayType{Element: elemType}

	case *parser.GroupExpr:
		return c.checkExpr(e.Inner)

	case *parser.CatchExpr:
		exprType := c.checkExpr(e.Expr)
		c.checkExpr(e.Body)
		// catch unwraps Result to Ok type
		if rt, ok := exprType.(*ResultType); ok {
			return rt.Ok
		}
		return exprType

	case *parser.RecordUpdateExpr:
		objType := c.checkExpr(e.Object)
		for _, f := range e.Fields {
			if f.Value != nil {
				c.checkExpr(f.Value)
			}
		}
		return objType

	case *parser.FieldShorthandExpr:
		return &UnresolvedType{Name: e.Field}

	default:
		return &UnresolvedType{Name: "unknown"}
	}
}

func (c *Checker) checkIdent(e *parser.IdentExpr) Type {
	// Check type environment first (params, locals)
	if t := c.typeEnv.Get(e.Name); t != nil {
		return t
	}
	// Check for known function
	if ft, ok := c.fnTypes[e.Name]; ok {
		return ft
	}
	// Check for known type used as constructor
	if t, ok := c.types[e.Name]; ok {
		return t
	}
	// Built-in type names
	if pt := PrimitiveByName(e.Name); pt != nil {
		return pt
	}
	// Unresolved
	return &UnresolvedType{Name: e.Name}
}

func (c *Checker) checkBinaryExpr(e *parser.BinaryExpr) Type {
	leftType := c.checkExpr(e.Left)
	rightType := c.checkExpr(e.Right)

	leftUnresolved := false
	rightUnresolved := false
	if _, ok := leftType.(*UnresolvedType); ok {
		leftUnresolved = true
	}
	if _, ok := rightType.(*UnresolvedType); ok {
		rightUnresolved = true
	}

	switch e.Op {
	// Arithmetic operators: both sides must be same numeric type (or str for +)
	case lexer.Plus, lexer.Minus, lexer.Star, lexer.Slash, lexer.Percent:
		if leftUnresolved || rightUnresolved {
			if !leftUnresolved {
				return leftType
			}
			return rightType
		}
		// + is also string concatenation
		if e.Op == lexer.Plus && IsAssignable(leftType, TypeStr) {
			return TypeStr
		}
		if !IsNumericType(leftType) {
			c.error(e.Pos, diagnostic.E0100,
				fmt.Sprintf("operator %s requires numeric type, got %s", e.Op, leftType))
			return leftType
		}
		if !IsAssignable(leftType, rightType) {
			c.error(e.Pos, diagnostic.E0100,
				fmt.Sprintf("type mismatch in %s: %s vs %s", e.Op, leftType, rightType))
		}
		return leftType

	// Comparison operators: always return bool regardless of operand types
	case lexer.EqEq, lexer.BangEq, lexer.Lt, lexer.Gt, lexer.LtEq, lexer.GtEq:
		if !leftUnresolved && !rightUnresolved {
			if !IsAssignable(leftType, rightType) {
				c.error(e.Pos, diagnostic.E0100,
					fmt.Sprintf("cannot compare %s with %s", leftType, rightType))
			}
		}
		return TypeBool

	// Logical operators: always return bool
	case lexer.AmpAmp, lexer.PipePipe:
		if !leftUnresolved {
			if !IsAssignable(leftType, TypeBool) {
				c.error(e.Pos, diagnostic.E0100,
					fmt.Sprintf("operator %s requires bool, got %s", e.Op, leftType))
			}
		}
		if !rightUnresolved {
			if !IsAssignable(rightType, TypeBool) {
				c.error(e.Pos, diagnostic.E0100,
					fmt.Sprintf("operator %s requires bool, got %s", e.Op, rightType))
			}
		}
		return TypeBool

	// Bitwise operators: integer types
	case lexer.Amp, lexer.Pipe, lexer.Caret, lexer.LtLt, lexer.GtGt:
		if !leftUnresolved && !IsIntegerType(leftType) {
			c.error(e.Pos, diagnostic.E0100,
				fmt.Sprintf("bitwise operator requires integer type, got %s", leftType))
		}
		return leftType

	// Null coalesce: T? ?? T -> T
	case lexer.QuestionQuestion:
		if opt, ok := leftType.(*OptionalType); ok {
			return opt.Inner
		}
		return leftType

	default:
		return leftType
	}
}

func (c *Checker) checkUnaryExpr(e *parser.UnaryExpr) Type {
	operandType := c.checkExpr(e.Operand)

	switch e.Op {
	case lexer.Minus:
		if !IsNumericType(operandType) {
			if _, ok := operandType.(*UnresolvedType); !ok {
				c.error(e.Pos, diagnostic.E0100,
					fmt.Sprintf("unary - requires numeric type, got %s", operandType))
			}
		}
		return operandType

	case lexer.Bang:
		if !IsAssignable(operandType, TypeBool) {
			if _, ok := operandType.(*UnresolvedType); !ok {
				c.error(e.Pos, diagnostic.E0100,
					fmt.Sprintf("unary ! requires bool, got %s", operandType))
			}
		}
		return TypeBool

	case lexer.Tilde:
		if !IsIntegerType(operandType) {
			if _, ok := operandType.(*UnresolvedType); !ok {
				c.error(e.Pos, diagnostic.E0100,
					fmt.Sprintf("bitwise NOT requires integer type, got %s", operandType))
			}
		}
		return operandType

	default:
		return operandType
	}
}

func (c *Checker) checkPostfixExpr(e *parser.PostfixExpr) Type {
	operandType := c.checkExpr(e.Operand)

	switch e.Op {
	case lexer.Question:
		// ? propagation: unwraps Result[T, E] -> T, returning Err if error
		if c.fnCtx != nil && len(c.fnCtx.ErrorTypes) == 0 {
			c.error(e.Pos, diagnostic.E0850,
				"cannot use ? in a function that doesn't declare error types")
		}
		if rt, ok := operandType.(*ResultType); ok {
			return rt.Ok
		}
		if opt, ok := operandType.(*OptionalType); ok {
			return opt.Inner
		}
		return operandType

	case lexer.Bang:
		// ! assert: unwraps Result or panics
		if rt, ok := operandType.(*ResultType); ok {
			return rt.Ok
		}
		if opt, ok := operandType.(*OptionalType); ok {
			return opt.Inner
		}
		return operandType

	default:
		return operandType
	}
}

func (c *Checker) checkCallExpr(e *parser.CallExpr) Type {
	funcType := c.checkExpr(e.Func)

	// Check arguments
	for _, arg := range e.Args {
		c.checkExpr(arg.Value)
	}

	// If we know the function type, validate args
	if ft, ok := funcType.(*FunctionType); ok {
		if len(e.Args) != len(ft.Params) {
			c.error(e.Pos, diagnostic.E0104,
				fmt.Sprintf("expected %d arguments, got %d", len(ft.Params), len(e.Args)))
		}
		// Check effect propagation
		if c.fnCtx != nil {
			c.checkEffects(ft.Effects, e.Pos)
		}
		if ft.Return != nil {
			// If the function has error types, wrap return in Result
			if len(ft.Errors) > 0 {
				var errType Type
				if len(ft.Errors) == 1 {
					errType = ft.Errors[0]
				} else {
					errType = &ErrorUnion{Types: ft.Errors}
				}
				return &ResultType{Ok: ft.Return, Err: errType}
			}
			return ft.Return
		}
		return TypeUnit
	}

	return &UnresolvedType{Name: "call"}
}

func (c *Checker) checkFieldAccess(objType Type, field string, pos parser.Pos) Type {
	t := Unwrap(objType)
	switch st := t.(type) {
	case *StructType:
		for _, f := range st.Fields {
			if f.Name == field {
				return f.Type
			}
		}
		c.error(pos, diagnostic.E0108,
			fmt.Sprintf("type '%s' has no field '%s'", st.Name, field))
		return &UnresolvedType{Name: field}
	case *UnresolvedType:
		return &UnresolvedType{Name: field}
	default:
		return &UnresolvedType{Name: field}
	}
}

func (c *Checker) checkBlockExpr(block *parser.BlockExpr) Type {
	prevEnv := c.typeEnv
	c.typeEnv = NewTypeEnv(prevEnv)
	for _, stmt := range block.Stmts {
		c.checkStmt(stmt)
	}
	var result Type = TypeUnit
	if block.Expr != nil {
		result = c.checkExpr(block.Expr)
	}
	c.typeEnv = prevEnv
	return result
}

func (c *Checker) checkIfExpr(e *parser.IfExpr) Type {
	condType := c.checkExpr(e.Cond)
	if _, ok := condType.(*UnresolvedType); !ok {
		if !IsAssignable(condType, TypeBool) {
			c.error(e.Pos, diagnostic.E0100,
				fmt.Sprintf("if condition must be bool, got %s", condType))
		}
	}

	thenType := c.checkExpr(e.Then)

	if e.Else != nil {
		elseType := c.checkExpr(e.Else)
		// Both branches must produce same type
		if _, ok := thenType.(*UnresolvedType); ok {
			return elseType
		}
		if _, ok := elseType.(*UnresolvedType); ok {
			return thenType
		}
		if !IsAssignable(thenType, elseType) && !IsAssignable(elseType, thenType) {
			c.error(e.Pos, diagnostic.E0103,
				fmt.Sprintf("if/else branch type mismatch: %s vs %s", thenType, elseType))
		}
		return thenType
	}

	return TypeUnit
}

func (c *Checker) checkMatchExpr(e *parser.MatchExpr) Type {
	subjectType := c.checkExpr(e.Subject)

	// Check exhaustiveness for known types
	c.checkExhaustiveness(subjectType, e.Arms, e.Pos)

	var resultType Type
	for _, arm := range e.Arms {
		prevEnv := c.typeEnv
		c.typeEnv = NewTypeEnv(prevEnv)
		// Register pattern bindings with types inferred from subject
		c.registerPatternBindings(arm.Pattern, subjectType)
		if arm.Guard != nil {
			c.checkExpr(arm.Guard)
		}
		armType := c.checkExpr(arm.Body)
		c.typeEnv = prevEnv
		if resultType == nil {
			resultType = armType
		} else {
			if _, ok := resultType.(*UnresolvedType); ok {
				resultType = armType
			} else if _, ok := armType.(*UnresolvedType); !ok {
				if !IsAssignable(armType, resultType) && !IsAssignable(resultType, armType) {
					c.error(arm.Pos, diagnostic.E0103,
						fmt.Sprintf("match arm type mismatch: expected %s, got %s",
							resultType, armType))
				}
			}
		}
	}

	if resultType == nil {
		return TypeUnit
	}
	return resultType
}

func (c *Checker) checkClosureExpr(e *parser.ClosureExpr) Type {
	prevEnv := c.typeEnv
	c.typeEnv = NewTypeEnv(prevEnv)

	ft := &FunctionType{}
	for _, p := range e.Params {
		if p.Type != nil {
			pt := c.resolveTypeExpr(p.Type)
			ft.Params = append(ft.Params, pt)
			c.typeEnv.Set(p.Name, pt)
		} else {
			ut := &UnresolvedType{Name: p.Name}
			ft.Params = append(ft.Params, ut)
			c.typeEnv.Set(p.Name, ut)
		}
	}
	if e.Return != nil {
		ft.Return = c.resolveTypeExpr(e.Return)
	}

	bodyType := c.checkExpr(e.Body)
	if ft.Return == nil {
		ft.Return = bodyType
	}

	c.typeEnv = prevEnv
	return ft
}

func (c *Checker) checkStructExpr(e *parser.StructExpr) Type {
	st, ok := c.types[e.TypeName]
	if !ok {
		return &UnresolvedType{Name: e.TypeName}
	}

	structType, ok := st.(*StructType)
	if !ok {
		c.error(e.Pos, diagnostic.E0100,
			fmt.Sprintf("'%s' is not a struct type", e.TypeName))
		return st
	}

	// Check provided fields
	provided := make(map[string]bool)
	for _, f := range e.Fields {
		provided[f.Name] = true
		if f.Value != nil {
			c.checkExpr(f.Value)
		}
		// Verify field exists
		found := false
		for _, sf := range structType.Fields {
			if sf.Name == f.Name {
				found = true
				break
			}
		}
		if !found {
			c.error(e.Pos, diagnostic.E0108,
				fmt.Sprintf("unknown field '%s' in struct '%s'", f.Name, e.TypeName))
		}
	}

	// Check required fields are present
	for _, sf := range structType.Fields {
		if !sf.Default && !provided[sf.Name] {
			c.error(e.Pos, diagnostic.E0107,
				fmt.Sprintf("missing field '%s' in struct '%s'", sf.Name, e.TypeName))
		}
	}

	return structType
}

func (c *Checker) checkArrayExpr(e *parser.ArrayExpr) Type {
	if len(e.Elements) == 0 {
		return &ArrayType{Element: &UnresolvedType{Name: "T"}}
	}
	elemType := c.checkExpr(e.Elements[0])
	for _, el := range e.Elements[1:] {
		elType := c.checkExpr(el)
		if _, ok := elemType.(*UnresolvedType); ok {
			elemType = elType
		} else if _, ok := elType.(*UnresolvedType); !ok {
			if !IsAssignable(elType, elemType) {
				c.error(e.Pos, diagnostic.E0100,
					fmt.Sprintf("array element type mismatch: expected %s, got %s",
						elemType, elType))
			}
		}
	}
	return &ArrayType{Element: elemType}
}

// ---------- Check statements ----------

func (c *Checker) checkStmt(stmt parser.Stmt) {
	switch s := stmt.(type) {
	case *parser.VarDeclStmt:
		valType := c.checkExpr(s.Value)
		if s.Type != nil {
			declType := c.resolveTypeExpr(s.Type)
			c.typeEnv.Set(s.Name, declType)
			if _, ok := valType.(*UnresolvedType); !ok {
				if !IsAssignable(valType, declType) {
					c.error(s.Pos, diagnostic.E0100,
						fmt.Sprintf("cannot assign %s to variable of type %s",
							valType, declType))
				}
			}
		} else {
			// Infer type from value
			c.typeEnv.Set(s.Name, valType)
		}

	case *parser.AssignStmt:
		c.checkExpr(s.Target)
		c.checkExpr(s.Value)
		// Mutability check: target must be mutable
		if ident, ok := s.Target.(*parser.IdentExpr); ok {
			sym := c.scope.Lookup(ident.Name)
			if sym != nil && sym.Kind == resolver.SymVariable && !sym.Mutable {
				c.error(s.Pos, diagnostic.E0110,
					fmt.Sprintf("cannot assign to immutable binding '%s'", ident.Name))
			}
		}

	case *parser.ExprStmt:
		c.checkExpr(s.Expr)

	case *parser.ForStmt:
		c.checkExpr(s.Iter)
		if s.Where != nil {
			c.checkExpr(s.Where)
		}
		prevInLoop := false
		if c.fnCtx != nil {
			prevInLoop = c.fnCtx.InLoop
			c.fnCtx.InLoop = true
		}
		c.checkExpr(s.Body)
		if c.fnCtx != nil {
			c.fnCtx.InLoop = prevInLoop
		}

	case *parser.WhileStmt:
		condType := c.checkExpr(s.Cond)
		if _, ok := condType.(*UnresolvedType); !ok {
			if !IsAssignable(condType, TypeBool) {
				c.error(s.Pos, diagnostic.E0100,
					fmt.Sprintf("while condition must be bool, got %s", condType))
			}
		}
		prevInLoop := false
		if c.fnCtx != nil {
			prevInLoop = c.fnCtx.InLoop
			c.fnCtx.InLoop = true
		}
		c.checkExpr(s.Body)
		if c.fnCtx != nil {
			c.fnCtx.InLoop = prevInLoop
		}

	case *parser.LoopStmt:
		prevInLoop := false
		if c.fnCtx != nil {
			prevInLoop = c.fnCtx.InLoop
			c.fnCtx.InLoop = true
		}
		c.checkExpr(s.Body)
		if c.fnCtx != nil {
			c.fnCtx.InLoop = prevInLoop
		}

	case *parser.ReturnStmt:
		if s.Value != nil {
			retType := c.checkExpr(s.Value)
			if c.fnCtx != nil && c.fnCtx.ReturnType != nil {
				if _, ok := retType.(*UnresolvedType); !ok {
					if !IsAssignable(retType, c.fnCtx.ReturnType) {
						c.error(s.Pos, diagnostic.E0106,
							fmt.Sprintf("return type mismatch: expected %s, got %s",
								c.fnCtx.ReturnType, retType))
					}
				}
			}
		}

	case *parser.BreakStmt:
		if s.Value != nil {
			c.checkExpr(s.Value)
		}

	case *parser.ContinueStmt:
		// Nothing to check

	case *parser.DeferStmt:
		c.checkExpr(s.Expr)
	}
}

// ---------- Effect checking ----------

func (c *Checker) checkEffects(calledEffects []string, pos parser.Pos) {
	if c.fnCtx == nil || len(calledEffects) == 0 {
		return
	}

	allowed := make(map[string]bool)
	for _, e := range c.fnCtx.Effects {
		allowed[e] = true
	}

	for _, eff := range calledEffects {
		if !allowed[eff] {
			c.error(pos, diagnostic.E0301,
				fmt.Sprintf("pure function cannot call function with effect '%s'", eff))
		}
	}
}

// ---------- Exhaustiveness checking ----------

func (c *Checker) checkExhaustiveness(subjectType Type, arms []*parser.MatchArm, pos parser.Pos) {
	t := Unwrap(subjectType)

	switch st := t.(type) {
	case *SumType:
		c.checkSumTypeExhaustiveness(st, arms, pos)
	case *EnumType:
		c.checkEnumExhaustiveness(st, arms, pos)
	case *PrimitiveType:
		if st.Name == "bool" {
			c.checkBoolExhaustiveness(arms, pos)
		}
		// Other primitives: need wildcard/binding pattern
	}
}

func (c *Checker) checkSumTypeExhaustiveness(st *SumType, arms []*parser.MatchArm, pos parser.Pos) {
	covered := make(map[string]bool)
	hasWildcard := false

	for _, arm := range arms {
		if arm.Guard != nil {
			continue // guarded arms don't count for exhaustiveness
		}
		c.collectCoveredVariants(arm.Pattern, covered, &hasWildcard)
	}

	if hasWildcard {
		return
	}

	var missing []string
	for _, v := range st.Variants {
		if !covered[v.Name] {
			missing = append(missing, v.Name)
		}
	}

	if len(missing) > 0 {
		c.error(pos, diagnostic.E0400,
			fmt.Sprintf("non-exhaustive match: missing variants %v", missing))
	}
}

func (c *Checker) checkEnumExhaustiveness(et *EnumType, arms []*parser.MatchArm, pos parser.Pos) {
	covered := make(map[string]bool)
	hasWildcard := false

	for _, arm := range arms {
		if arm.Guard != nil {
			continue
		}
		c.collectCoveredVariants(arm.Pattern, covered, &hasWildcard)
	}

	if hasWildcard {
		return
	}

	var missing []string
	for _, m := range et.Members {
		if !covered[m] {
			missing = append(missing, m)
		}
	}

	if len(missing) > 0 {
		c.error(pos, diagnostic.E0400,
			fmt.Sprintf("non-exhaustive match: missing members %v", missing))
	}
}

func (c *Checker) checkBoolExhaustiveness(arms []*parser.MatchArm, pos parser.Pos) {
	hasTrue := false
	hasFalse := false
	hasWildcard := false

	for _, arm := range arms {
		if arm.Guard != nil {
			continue
		}
		switch p := arm.Pattern.(type) {
		case *parser.LiteralPattern:
			if bl, ok := p.Value.(*parser.BoolLitExpr); ok {
				if bl.Value {
					hasTrue = true
				} else {
					hasFalse = true
				}
			}
		case *parser.BindingPattern, *parser.WildcardPattern:
			hasWildcard = true
		}
	}

	if !hasWildcard && !(hasTrue && hasFalse) {
		c.error(pos, diagnostic.E0400, "non-exhaustive match on bool")
	}
}

func (c *Checker) collectCoveredVariants(pat parser.Pattern, covered map[string]bool, hasWildcard *bool) {
	switch p := pat.(type) {
	case *parser.VariantPattern:
		covered[p.Name] = true
	case *parser.BindingPattern:
		if p.Name == "_" {
			*hasWildcard = true
		} else {
			// Check if this name is a known variant/enum member
			if _, ok := c.types[p.Name]; ok {
				covered[p.Name] = true
			} else if sym := c.scope.Lookup(p.Name); sym != nil && sym.Kind == resolver.SymVariant {
				covered[p.Name] = true
			} else {
				*hasWildcard = true // binding pattern matches anything
			}
		}
	case *parser.WildcardPattern:
		*hasWildcard = true
	case *parser.OrPattern:
		c.collectCoveredVariants(p.Left, covered, hasWildcard)
		c.collectCoveredVariants(p.Right, covered, hasWildcard)
	case *parser.StructPattern:
		covered[p.TypeName] = true
	}
}

// ---------- Method return type inference ----------

func (c *Checker) inferMethodReturnType(method string) Type {
	switch method {
	// String methods returning str
	case "trim", "toLower", "toUpper", "replace", "charAt", "substring", "toStr":
		return TypeStr
	// String methods returning i64
	case "len", "indexOf":
		return TypeI64
	// String methods returning bool
	case "contains", "startsWith", "endsWith", "isEmpty":
		return TypeBool
	// String methods returning [str]
	case "split":
		return &ArrayType{Element: TypeStr}
	// Collection methods
	case "first", "last":
		return &UnresolvedType{Name: method}
	// Conversion methods
	case "parseInt":
		return TypeI64
	case "parseFloat":
		return TypeF64
	default:
		return &UnresolvedType{Name: method}
	}
}

// ---------- Pattern binding types ----------

func (c *Checker) registerPatternBindings(pat parser.Pattern, subjectType Type) {
	if pat == nil {
		return
	}
	switch p := pat.(type) {
	case *parser.BindingPattern:
		if p.Name != "_" {
			c.typeEnv.Set(p.Name, subjectType)
		}
	case *parser.VariantPattern:
		// Look up variant in sum type to get field types
		st, _ := Unwrap(subjectType).(*SumType)
		if st != nil {
			for _, v := range st.Variants {
				if v.Name == p.Name {
					for i, arg := range p.Args {
						if i < len(v.Types) {
							c.registerPatternBindings(arg, v.Types[i])
						}
					}
					break
				}
			}
		}
	case *parser.StructPattern:
		st, _ := Unwrap(subjectType).(*StructType)
		if st != nil {
			for _, fp := range p.Fields {
				var fieldType Type = &UnresolvedType{Name: fp.Name}
				for _, sf := range st.Fields {
					if sf.Name == fp.Name {
						fieldType = sf.Type
						break
					}
				}
				if fp.Pattern != nil {
					c.registerPatternBindings(fp.Pattern, fieldType)
				} else {
					c.typeEnv.Set(fp.Name, fieldType)
				}
			}
		}
	case *parser.TuplePattern:
		if tt, ok := subjectType.(*TupleType); ok {
			for i, el := range p.Elements {
				if i < len(tt.Elements) {
					c.registerPatternBindings(el, tt.Elements[i])
				}
			}
		}
	case *parser.OrPattern:
		c.registerPatternBindings(p.Left, subjectType)
		c.registerPatternBindings(p.Right, subjectType)
	case *parser.NamedPattern:
		c.typeEnv.Set(p.Name, subjectType)
		c.registerPatternBindings(p.Pattern, subjectType)
	}
}

// ---------- Resolve type expressions to Types ----------

func (c *Checker) resolveTypeExpr(te parser.TypeExpr) Type {
	if te == nil {
		return TypeUnit
	}

	switch t := te.(type) {
	case *parser.NamedTypeExpr:
		name := t.Path[len(t.Path)-1]
		if pt := PrimitiveByName(name); pt != nil {
			return pt
		}
		if resolved, ok := c.types[name]; ok {
			if len(t.TypeArgs) > 0 {
				var args []Type
				for _, a := range t.TypeArgs {
					args = append(args, c.resolveTypeExpr(a))
				}
				return &GenericType{Base: resolved, TypeArgs: args}
			}
			return resolved
		}
		// Check for generic type parameter
		return &UnresolvedType{Name: name}

	case *parser.ArrayTypeExpr:
		return &ArrayType{Element: c.resolveTypeExpr(t.Element)}

	case *parser.MapTypeExpr:
		return &MapType{
			Key:   c.resolveTypeExpr(t.Key),
			Value: c.resolveTypeExpr(t.Value),
		}

	case *parser.SetTypeExpr:
		return &SetType{Element: c.resolveTypeExpr(t.Element)}

	case *parser.TupleTypeExpr:
		var elems []Type
		for _, e := range t.Elements {
			elems = append(elems, c.resolveTypeExpr(e))
		}
		return &TupleType{Elements: elems}

	case *parser.OptionalTypeExpr:
		return &OptionalType{Inner: c.resolveTypeExpr(t.Inner)}

	case *parser.ResultTypeExpr:
		return &ResultType{
			Ok:  c.resolveTypeExpr(t.Ok),
			Err: c.resolveTypeExpr(t.Err),
		}

	case *parser.FunctionTypeExpr:
		ft := &FunctionType{}
		for _, p := range t.Params {
			ft.Params = append(ft.Params, c.resolveTypeExpr(p))
		}
		ft.Return = c.resolveTypeExpr(t.Return)
		return ft

	default:
		return &UnresolvedType{Name: "unknown"}
	}
}

// ---------- Error helpers ----------

func (c *Checker) error(pos parser.Pos, code string, msg string) {
	c.diagnostics.Add(diagnostic.Diagnostic{
		Code:     code,
		Severity: diagnostic.Error,
		Message:  msg,
		File:     pos.File,
		Line:     pos.Line,
		Column:   pos.Column,
		Span:     [2]int{pos.Offset, pos.Offset + 1},
		Labels: []diagnostic.Label{{
			File:    pos.File,
			Line:    pos.Line,
			Column:  pos.Column,
			Span:    [2]int{pos.Offset, pos.Offset + 1},
			Message: msg,
			Style:   diagnostic.Primary,
		}},
	})
}
