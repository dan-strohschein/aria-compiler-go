package resolver

import (
	"fmt"

	"github.com/aria-lang/aria/internal/diagnostic"
	"github.com/aria-lang/aria/internal/parser"
)

// Resolver performs name resolution on an AST, building scope chains
// and verifying that all identifiers refer to valid declarations.
type Resolver struct {
	universe    *Scope
	module      *Scope
	current     *Scope
	diagnostics *diagnostic.DiagnosticList

	// Module registry for simple import resolution.
	// Maps module paths (e.g., "std.fs") to available symbols.
	modules map[string][]string
}

// New creates a new Resolver.
func New() *Resolver {
	r := &Resolver{
		universe:    NewUniverseScope(),
		diagnostics: &diagnostic.DiagnosticList{},
		modules:     defaultModules(),
	}
	return r
}

// defaultModules returns a simple registry of known standard library modules
// and their exported symbols. This is a stub for bootstrap.
func defaultModules() map[string][]string {
	return map[string][]string{
		"std.fs":     {"read", "write", "exists", "listDir", "readFile", "writeFile", "appendFile"},
		"std.io":     {"readLine", "print", "println"},
		"std.json":   {"parse", "stringify"},
		"std.http":   {"get", "post", "request"},
		"std.str":    {"split", "trim", "contains", "replace", "join"},
		"std.math":   {"abs", "min", "max", "sqrt", "pow", "floor", "ceil"},
		"std.collections": {"HashMap", "HashSet", "LinkedList", "Queue", "Stack"},
	}
}

// Resolve performs name resolution on the program.
func (r *Resolver) Resolve(prog *parser.Program) *Scope {
	// Create module scope under universe
	r.module = NewScope(ModuleScope, r.universe)
	r.current = r.module

	// Phase 1: Register all top-level declarations (forward references)
	r.registerTopLevel(prog)

	// Phase 2: Resolve imports
	r.resolveImports(prog)

	// Phase 3: Resolve all declaration bodies
	for _, decl := range prog.Decls {
		r.resolveDecl(decl)
	}

	return r.module
}

// Diagnostics returns accumulated diagnostics.
func (r *Resolver) Diagnostics() *diagnostic.DiagnosticList {
	return r.diagnostics
}

// ---------- Phase 1: Register top-level names ----------

func (r *Resolver) registerTopLevel(prog *parser.Program) {
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *parser.FnDecl:
			r.defineOrError(d.Name, &Symbol{
				Name: d.Name,
				Kind: SymFunction,
				Decl: d,
				Pos:  d.Pos,
			})
		case *parser.TypeDecl:
			r.defineOrError(d.Name, &Symbol{
				Name: d.Name,
				Kind: SymType,
				Decl: d,
				Pos:  d.Pos,
			})
			// Also register variant names for sum types
			if d.Kind == parser.SumTypeDecl {
				for _, v := range d.Variants {
					r.defineOrError(v.Name, &Symbol{
						Name: v.Name,
						Kind: SymVariant,
						Decl: d,
						Pos:  v.Pos,
					})
				}
			}
		case *parser.EnumDecl:
			r.defineOrError(d.Name, &Symbol{
				Name: d.Name,
				Kind: SymEnum,
				Decl: d,
				Pos:  d.Pos,
			})
			// Register enum members
			for _, member := range d.Members {
				r.defineOrError(member, &Symbol{
					Name: member,
					Kind: SymVariant,
					Decl: d,
					Pos:  d.Pos,
				})
			}
		case *parser.TraitDecl:
			r.defineOrError(d.Name, &Symbol{
				Name: d.Name,
				Kind: SymTrait,
				Decl: d,
				Pos:  d.Pos,
			})
		case *parser.ConstDecl:
			r.defineOrError(d.Name, &Symbol{
				Name: d.Name,
				Kind: SymConst,
				Decl: d,
				Pos:  d.Pos,
			})
		case *parser.AliasDecl:
			r.defineOrError(d.Name, &Symbol{
				Name: d.Name,
				Kind: SymType,
				Decl: d,
				Pos:  d.Pos,
			})
		case *parser.ImplDecl:
			// Impl blocks don't introduce a name into scope
		case *parser.EntryBlock:
			// Entry blocks don't introduce a name
		case *parser.TestBlock:
			// Test blocks don't introduce a name
		}
	}
}

func (r *Resolver) defineOrError(name string, sym *Symbol) {
	if !r.current.Define(sym) {
		r.error(sym.Pos, diagnostic.E0702,
			fmt.Sprintf("duplicate declaration of '%s'", name))
	}
}

// ---------- Phase 2: Resolve imports ----------

func (r *Resolver) resolveImports(prog *parser.Program) {
	for _, imp := range prog.Imports {
		r.resolveImport(imp)
	}
}

func (r *Resolver) resolveImport(imp *parser.ImportDecl) {
	fullPath := joinPath(imp.Path)

	if imp.Names != nil {
		// Grouped import: use std.{json, http}
		for _, name := range imp.Names {
			subPath := fullPath + "." + name
			if _, ok := r.modules[subPath]; ok {
				importName := name
				r.defineOrError(importName, &Symbol{
					Name: importName,
					Kind: SymImport,
					Pos:  imp.Pos,
				})
			} else {
				// Also check if it's a member of the parent module
				if members, ok := r.modules[fullPath]; ok {
					found := false
					for _, m := range members {
						if m == name {
							found = true
							break
						}
					}
					if found {
						r.defineOrError(name, &Symbol{
							Name: name,
							Kind: SymImport,
							Pos:  imp.Pos,
						})
					} else {
						r.error(imp.Pos, diagnostic.E0700,
							fmt.Sprintf("unresolved import '%s.%s'", fullPath, name))
					}
				} else {
					r.error(imp.Pos, diagnostic.E0700,
						fmt.Sprintf("unresolved import '%s.%s'", fullPath, name))
				}
			}
		}
		return
	}

	// Simple import: use std.fs
	// The last segment becomes the local name
	localName := imp.Path[len(imp.Path)-1]
	if imp.Alias != "" {
		localName = imp.Alias
	}

	if _, ok := r.modules[fullPath]; ok {
		r.defineOrError(localName, &Symbol{
			Name: localName,
			Kind: SymImport,
			Pos:  imp.Pos,
		})
	} else {
		// Allow unknown imports with a warning for bootstrap flexibility
		r.defineOrError(localName, &Symbol{
			Name: localName,
			Kind: SymImport,
			Pos:  imp.Pos,
		})
	}
}

func joinPath(parts []string) string {
	result := parts[0]
	for _, p := range parts[1:] {
		result += "." + p
	}
	return result
}

// ---------- Phase 3: Resolve declaration bodies ----------

func (r *Resolver) resolveDecl(decl parser.Decl) {
	switch d := decl.(type) {
	case *parser.FnDecl:
		r.resolveFnDecl(d)
	case *parser.TypeDecl:
		r.resolveTypeDecl(d)
	case *parser.TraitDecl:
		r.resolveTraitDecl(d)
	case *parser.ImplDecl:
		r.resolveImplDecl(d)
	case *parser.ConstDecl:
		if d.Value != nil {
			r.resolveExpr(d.Value)
		}
	case *parser.EntryBlock:
		r.pushScope(BlockScope)
		r.resolveBlock(d.Body)
		r.popScope()
	case *parser.TestBlock:
		r.pushScope(BlockScope)
		r.resolveBlock(d.Body)
		r.popScope()
	case *parser.EnumDecl:
		// Nothing to resolve in enum body
	case *parser.AliasDecl:
		// Type aliases just reference types, resolved during type checking
	}
}

func (r *Resolver) resolveFnDecl(fn *parser.FnDecl) {
	r.pushScope(FunctionScope)

	// Register generic type params in function scope
	for _, gp := range fn.GenericParams {
		r.defineOrError(gp.Name, &Symbol{
			Name: gp.Name,
			Kind: SymGenericParam,
			Pos:  gp.Pos,
		})
	}

	// Register parameters
	for _, param := range fn.Params {
		if param.Name == "self" {
			// self is implicitly available
			r.defineOrError("self", &Symbol{
				Name: "self",
				Kind: SymParam,
				Pos:  param.Pos,
			})
		} else {
			r.defineOrError(param.Name, &Symbol{
				Name:    param.Name,
				Kind:    SymParam,
				Mutable: param.Mutable,
				Pos:     param.Pos,
			})
		}
	}

	// Resolve body
	if fn.Body != nil {
		r.resolveExpr(fn.Body)
	}

	r.popScope()
}

func (r *Resolver) resolveTypeDecl(td *parser.TypeDecl) {
	// Type declarations are already registered; nothing to resolve in bodies
	// for the bootstrap compiler (field types are checked during type checking).
}

func (r *Resolver) resolveTraitDecl(td *parser.TraitDecl) {
	for _, method := range td.Methods {
		r.resolveFnDecl(method)
	}
}

func (r *Resolver) resolveImplDecl(impl *parser.ImplDecl) {
	// Verify the type name exists
	if r.current.Lookup(impl.TypeName) == nil {
		r.error(impl.Pos, diagnostic.E0701,
			fmt.Sprintf("unresolved type '%s' in impl block", impl.TypeName))
	}

	// Verify trait name if present
	if impl.TraitName != "" {
		if r.current.Lookup(impl.TraitName) == nil {
			r.error(impl.Pos, diagnostic.E0701,
				fmt.Sprintf("unresolved trait '%s' in impl block", impl.TraitName))
		}
	}

	for _, method := range impl.Methods {
		r.resolveFnDecl(method)
	}
}

// ---------- Resolve expressions ----------

func (r *Resolver) resolveExpr(expr parser.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *parser.IdentExpr:
		if e.Name == "?" || e.Name == "_" {
			return
		}
		if r.current.Lookup(e.Name) == nil {
			r.error(e.Pos, diagnostic.E0701,
				fmt.Sprintf("unresolved name '%s'", e.Name))
		}

	case *parser.PathExpr:
		// Resolve the first part
		if r.current.Lookup(e.Parts[0]) == nil {
			r.error(e.Pos, diagnostic.E0701,
				fmt.Sprintf("unresolved name '%s'", e.Parts[0]))
		}

	case *parser.BinaryExpr:
		r.resolveExpr(e.Left)
		r.resolveExpr(e.Right)

	case *parser.UnaryExpr:
		r.resolveExpr(e.Operand)

	case *parser.PostfixExpr:
		r.resolveExpr(e.Operand)

	case *parser.CallExpr:
		r.resolveExpr(e.Func)
		for _, arg := range e.Args {
			r.resolveExpr(arg.Value)
		}

	case *parser.MethodCallExpr:
		r.resolveExpr(e.Object)
		for _, arg := range e.Args {
			r.resolveExpr(arg.Value)
		}

	case *parser.FieldAccessExpr:
		r.resolveExpr(e.Object)
		// Field name resolved during type checking

	case *parser.OptionalChainExpr:
		r.resolveExpr(e.Object)

	case *parser.IndexExpr:
		r.resolveExpr(e.Object)
		r.resolveExpr(e.Index)

	case *parser.PipelineExpr:
		r.resolveExpr(e.Left)
		r.resolveExpr(e.Right)

	case *parser.RangeExpr:
		r.resolveExpr(e.Start)
		r.resolveExpr(e.End)

	case *parser.BlockExpr:
		r.pushScope(BlockScope)
		r.resolveBlock(e)
		r.popScope()

	case *parser.IfExpr:
		r.resolveExpr(e.Cond)
		r.pushScope(BlockScope)
		r.resolveBlock(e.Then)
		r.popScope()
		if e.Else != nil {
			r.resolveExpr(e.Else)
		}

	case *parser.MatchExpr:
		r.resolveExpr(e.Subject)
		for _, arm := range e.Arms {
			r.pushScope(BlockScope)
			r.resolvePattern(arm.Pattern)
			if arm.Guard != nil {
				r.resolveExpr(arm.Guard)
			}
			r.resolveExpr(arm.Body)
			r.popScope()
		}

	case *parser.ClosureExpr:
		r.pushScope(FunctionScope)
		for _, param := range e.Params {
			r.defineOrError(param.Name, &Symbol{
				Name:    param.Name,
				Kind:    SymParam,
				Mutable: param.Mutable,
				Pos:     param.Pos,
			})
		}
		r.resolveExpr(e.Body)
		r.popScope()

	case *parser.StructExpr:
		// Verify the type name exists
		if r.current.Lookup(e.TypeName) == nil {
			r.error(e.Pos, diagnostic.E0701,
				fmt.Sprintf("unresolved type '%s'", e.TypeName))
		}
		for _, f := range e.Fields {
			if f.Value != nil {
				r.resolveExpr(f.Value)
			}
		}

	case *parser.ArrayExpr:
		for _, el := range e.Elements {
			r.resolveExpr(el)
		}

	case *parser.TupleExpr:
		for _, el := range e.Elements {
			r.resolveExpr(el)
		}

	case *parser.MapExpr:
		for _, entry := range e.Entries {
			r.resolveExpr(entry.Key)
			r.resolveExpr(entry.Value)
		}

	case *parser.InterpolatedStringExpr:
		for _, part := range e.Parts {
			r.resolveExpr(part)
		}

	case *parser.RecordUpdateExpr:
		r.resolveExpr(e.Object)
		for _, f := range e.Fields {
			if f.Value != nil {
				r.resolveExpr(f.Value)
			}
		}

	case *parser.CatchExpr:
		r.resolveExpr(e.Expr)
		if e.ErrName != "" {
			r.pushScope(BlockScope)
			r.defineOrError(e.ErrName, &Symbol{
				Name: e.ErrName,
				Kind: SymVariable,
				Pos:  e.Pos,
			})
			r.resolveExpr(e.Body)
			r.popScope()
		} else {
			r.resolveExpr(e.Body)
		}

	case *parser.ListCompExpr:
		r.pushScope(BlockScope)
		r.defineOrError(e.Var, &Symbol{
			Name: e.Var,
			Kind: SymVariable,
			Pos:  e.Pos,
		})
		r.resolveExpr(e.Iter)
		if e.Where != nil {
			r.resolveExpr(e.Where)
		}
		r.resolveExpr(e.Expr)
		r.popScope()

	case *parser.GroupExpr:
		r.resolveExpr(e.Inner)

	case *parser.IntLitExpr, *parser.FloatLitExpr, *parser.StringLitExpr,
		*parser.BoolLitExpr, *parser.FieldShorthandExpr:
		// Literals need no resolution
	}
}

// ---------- Resolve statements ----------

func (r *Resolver) resolveBlock(block *parser.BlockExpr) {
	for _, stmt := range block.Stmts {
		r.resolveStmt(stmt)
	}
	if block.Expr != nil {
		r.resolveExpr(block.Expr)
	}
}

func (r *Resolver) resolveStmt(stmt parser.Stmt) {
	switch s := stmt.(type) {
	case *parser.VarDeclStmt:
		// Resolve value first (before the binding is visible)
		r.resolveExpr(s.Value)
		// Variable declarations allow shadowing within the same block
		r.current.Bindings[s.Name] = &Symbol{
			Name:    s.Name,
			Kind:    SymVariable,
			Mutable: s.Mutable,
			Pos:     s.Pos,
		}

	case *parser.AssignStmt:
		r.resolveExpr(s.Target)
		r.resolveExpr(s.Value)

	case *parser.ExprStmt:
		r.resolveExpr(s.Expr)

	case *parser.ForStmt:
		r.resolveExpr(s.Iter)
		r.pushScope(BlockScope)
		r.resolvePattern(s.Pattern)
		if s.Where != nil {
			r.resolveExpr(s.Where)
		}
		r.resolveBlock(s.Body)
		r.popScope()

	case *parser.WhileStmt:
		r.resolveExpr(s.Cond)
		r.pushScope(BlockScope)
		r.resolveBlock(s.Body)
		r.popScope()

	case *parser.LoopStmt:
		r.pushScope(BlockScope)
		r.resolveBlock(s.Body)
		r.popScope()

	case *parser.ReturnStmt:
		if s.Value != nil {
			r.resolveExpr(s.Value)
		}

	case *parser.BreakStmt:
		if s.Value != nil {
			r.resolveExpr(s.Value)
		}

	case *parser.ContinueStmt:
		// Nothing to resolve

	case *parser.DeferStmt:
		r.resolveExpr(s.Expr)
	}
}

// ---------- Resolve patterns ----------

func (r *Resolver) resolvePattern(pat parser.Pattern) {
	if pat == nil {
		return
	}

	switch p := pat.(type) {
	case *parser.BindingPattern:
		if p.Name != "_" {
			r.defineOrError(p.Name, &Symbol{
				Name:    p.Name,
				Kind:    SymVariable,
				Mutable: p.Mutable,
				Pos:     p.Pos,
			})
		}

	case *parser.VariantPattern:
		// Verify variant name exists
		if r.current.Lookup(p.Name) == nil {
			r.error(p.Pos, diagnostic.E0701,
				fmt.Sprintf("unresolved variant '%s'", p.Name))
		}
		for _, arg := range p.Args {
			r.resolvePattern(arg)
		}

	case *parser.StructPattern:
		if r.current.Lookup(p.TypeName) == nil {
			r.error(p.Pos, diagnostic.E0701,
				fmt.Sprintf("unresolved type '%s'", p.TypeName))
		}
		for _, fp := range p.Fields {
			if fp.Pattern != nil {
				r.resolvePattern(fp.Pattern)
			} else {
				// Shorthand: field name becomes binding
				r.defineOrError(fp.Name, &Symbol{
					Name: fp.Name,
					Kind: SymVariable,
					Pos:  fp.Pos,
				})
			}
		}

	case *parser.TuplePattern:
		for _, el := range p.Elements {
			r.resolvePattern(el)
		}

	case *parser.ArrayPattern:
		for _, el := range p.Elements {
			r.resolvePattern(el)
		}
		if p.HasRest && p.Rest != "" {
			r.defineOrError(p.Rest, &Symbol{
				Name: p.Rest,
				Kind: SymVariable,
				Pos:  p.Pos,
			})
		}

	case *parser.OrPattern:
		r.resolvePattern(p.Left)
		r.resolvePattern(p.Right)

	case *parser.NamedPattern:
		r.defineOrError(p.Name, &Symbol{
			Name: p.Name,
			Kind: SymVariable,
			Pos:  p.Pos,
		})
		r.resolvePattern(p.Pattern)

	case *parser.RestPattern:
		if p.Name != "" {
			r.defineOrError(p.Name, &Symbol{
				Name: p.Name,
				Kind: SymVariable,
				Pos:  p.Pos,
			})
		}

	case *parser.WildcardPattern, *parser.LiteralPattern:
		// Nothing to resolve
	}
}

// ---------- Scope helpers ----------

func (r *Resolver) pushScope(level ScopeLevel) {
	r.current = NewScope(level, r.current)
}

func (r *Resolver) popScope() {
	r.current = r.current.Parent
}

// ---------- Error helpers ----------

func (r *Resolver) error(pos parser.Pos, code string, msg string) {
	r.diagnostics.Add(diagnostic.Diagnostic{
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
