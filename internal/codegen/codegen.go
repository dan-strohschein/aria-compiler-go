package codegen

import (
	"fmt"
	"strings"

	"github.com/aria-lang/aria/internal/checker"
	"github.com/aria-lang/aria/internal/lexer"
	"github.com/aria-lang/aria/internal/parser"
)

// Generator translates an Aria AST into Go source code.
type Generator struct {
	buf     strings.Builder
	indent  int
	types   map[string]checker.Type // registered type declarations
	program *parser.Program
	tmpVar  int // counter for temporary variables
}

// New creates a new code generator.
func New() *Generator {
	return &Generator{
		types: make(map[string]checker.Type),
	}
}

// Generate produces Go source from an Aria program.
func (g *Generator) Generate(prog *parser.Program) string {
	g.program = prog
	g.registerTypes(prog)

	g.writeln("package main")
	g.writeln("")
	g.writeImports(prog)
	g.writeln("")

	// Type declarations first
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *parser.TypeDecl:
			g.genTypeDecl(d)
		case *parser.EnumDecl:
			g.genEnumDecl(d)
		case *parser.AliasDecl:
			g.genAliasDecl(d)
		}
	}

	// Trait impls (Go methods)
	for _, decl := range prog.Decls {
		if impl, ok := decl.(*parser.ImplDecl); ok {
			g.genImplDecl(impl)
		}
	}

	// Function declarations
	for _, decl := range prog.Decls {
		if fn, ok := decl.(*parser.FnDecl); ok {
			g.genFnDecl(fn)
		}
	}

	// Constants
	for _, decl := range prog.Decls {
		if cd, ok := decl.(*parser.ConstDecl); ok {
			g.genConstDecl(cd)
		}
	}

	// Entry block -> func main()
	for _, decl := range prog.Decls {
		if eb, ok := decl.(*parser.EntryBlock); ok {
			g.genEntryBlock(eb)
		}
	}

	return g.buf.String()
}

// GenerateTest produces Go test source from test blocks.
func (g *Generator) GenerateTest(prog *parser.Program) string {
	var testBuf strings.Builder
	hasTests := false
	for _, decl := range prog.Decls {
		if _, ok := decl.(*parser.TestBlock); ok {
			hasTests = true
			break
		}
	}
	if !hasTests {
		return ""
	}

	testBuf.WriteString("package main\n\nimport \"testing\"\n\n")
	for _, decl := range prog.Decls {
		if tb, ok := decl.(*parser.TestBlock); ok {
			testName := sanitizeTestName(tb.Name)
			testBuf.WriteString(fmt.Sprintf("func Test%s(t *testing.T) {\n", testName))
			// Generate test body with assert -> t.Fatal
			g2 := &Generator{types: g.types, program: g.program, indent: 1}
			g2.genTestBody(tb.Body)
			testBuf.WriteString(g2.buf.String())
			testBuf.WriteString("}\n\n")
		}
	}
	return testBuf.String()
}

func sanitizeTestName(name string) string {
	// Remove quotes, capitalize words
	name = strings.Trim(name, "\"")
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	result := strings.Join(words, "")
	// Replace non-alphanumeric
	var sb strings.Builder
	for _, c := range result {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

func (g *Generator) registerTypes(prog *parser.Program) {
	for _, decl := range prog.Decls {
		switch d := decl.(type) {
		case *parser.TypeDecl:
			switch d.Kind {
			case parser.StructDecl:
				st := &checker.StructType{Name: d.Name}
				for _, f := range d.Fields {
					st.Fields = append(st.Fields, checker.StructField{
						Name:    f.Name,
						Default: f.Default != nil,
					})
				}
				g.types[d.Name] = st
			case parser.SumTypeDecl:
				sum := &checker.SumType{Name: d.Name}
				for _, v := range d.Variants {
					sv := checker.SumVariant{Name: v.Name}
					sum.Variants = append(sum.Variants, sv)
				}
				g.types[d.Name] = sum
			}
		}
	}
}

// ---------- Writing helpers ----------

func (g *Generator) write(s string) {
	g.buf.WriteString(s)
}

func (g *Generator) writeln(s string) {
	g.writeIndent()
	g.buf.WriteString(s)
	g.buf.WriteString("\n")
}

func (g *Generator) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.buf.WriteString("\t")
	}
}

func (g *Generator) newTmp() string {
	g.tmpVar++
	return fmt.Sprintf("_tmp%d", g.tmpVar)
}

// ---------- Imports ----------

func (g *Generator) writeImports(prog *parser.Program) {
	// Always import fmt for println/interpolation
	g.writeln("import (")
	g.indent++
	g.writeln(`"fmt"`)
	g.writeln(`"os"`)
	g.indent--
	g.writeln(")")
	// Suppress unused import warnings
	g.writeln("")
	g.writeln("var _ = fmt.Sprintf")
	g.writeln("var _ = os.Exit")
}

// ---------- Type declarations ----------

func (g *Generator) genTypeDecl(td *parser.TypeDecl) {
	switch td.Kind {
	case parser.StructDecl:
		g.genStructDecl(td)
	case parser.SumTypeDecl:
		g.genSumTypeDecl(td)
	case parser.NewtypeDecl:
		g.genNewtypeDecl(td)
	}
}

func (g *Generator) genStructDecl(td *parser.TypeDecl) {
	g.writeln(fmt.Sprintf("type %s struct {", td.Name))
	g.indent++
	for _, f := range td.Fields {
		goType := g.goTypeExpr(f.Type)
		g.writeln(fmt.Sprintf("%s %s", exportField(f.Name), goType))
	}
	g.indent--
	g.writeln("}")
	g.writeln("")
}

func (g *Generator) genSumTypeDecl(td *parser.TypeDecl) {
	ifaceName := td.Name
	// Interface
	g.writeln(fmt.Sprintf("type %s interface {", ifaceName))
	g.indent++
	g.writeln(fmt.Sprintf("is%s()", ifaceName))
	g.indent--
	g.writeln("}")
	g.writeln("")

	// Variant structs
	for _, v := range td.Variants {
		variantType := td.Name + v.Name
		if len(v.Fields) > 0 {
			// Struct variant
			g.writeln(fmt.Sprintf("type %s struct {", variantType))
			g.indent++
			for _, f := range v.Fields {
				g.writeln(fmt.Sprintf("%s %s", exportField(f.Name), g.goTypeExpr(f.Type)))
			}
			g.indent--
			g.writeln("}")
		} else if len(v.Types) > 0 {
			// Tuple variant
			g.writeln(fmt.Sprintf("type %s struct {", variantType))
			g.indent++
			for i, t := range v.Types {
				g.writeln(fmt.Sprintf("F%d %s", i, g.goTypeExpr(t)))
			}
			g.indent--
			g.writeln("}")
		} else {
			// Unit variant
			g.writeln(fmt.Sprintf("type %s struct{}", variantType))
		}
		// Implement the interface marker
		g.writeln(fmt.Sprintf("func (%s) is%s() {}", variantType, ifaceName))
		g.writeln("")
	}
}

func (g *Generator) genNewtypeDecl(td *parser.TypeDecl) {
	g.writeln(fmt.Sprintf("type %s %s", td.Name, g.goTypeExpr(td.Underlying)))
	g.writeln("")
}

func (g *Generator) genEnumDecl(ed *parser.EnumDecl) {
	g.writeln(fmt.Sprintf("type %s int", ed.Name))
	g.writeln("")
	g.writeln("const (")
	g.indent++
	for i, member := range ed.Members {
		if i == 0 {
			g.writeln(fmt.Sprintf("%s %s = iota", member, ed.Name))
		} else {
			g.writeln(member)
		}
	}
	g.indent--
	g.writeln(")")
	g.writeln("")
}

func (g *Generator) genAliasDecl(ad *parser.AliasDecl) {
	g.writeln(fmt.Sprintf("type %s = %s", ad.Name, g.goTypeExpr(ad.Target)))
	g.writeln("")
}

// ---------- Impl declarations ----------

func (g *Generator) genImplDecl(impl *parser.ImplDecl) {
	for _, method := range impl.Methods {
		g.genMethodDecl(impl.TypeName, method)
	}
}

func (g *Generator) genMethodDecl(typeName string, fn *parser.FnDecl) {
	receiverName := strings.ToLower(typeName[:1])

	// Build parameter list (skip self)
	var params []string
	for _, p := range fn.Params {
		if p.Name == "self" {
			continue
		}
		params = append(params, fmt.Sprintf("%s %s", p.Name, g.goTypeExpr(p.Type)))
	}

	retType := ""
	if fn.ReturnType != nil {
		retType = " " + g.goTypeExpr(fn.ReturnType)
	}

	g.writeln(fmt.Sprintf("func (%s %s) %s(%s)%s {",
		receiverName, typeName, fn.Name, strings.Join(params, ", "), retType))
	g.indent++
	g.genFnBody(fn)
	g.indent--
	g.writeln("}")
	g.writeln("")
}

// ---------- Function declarations ----------

func (g *Generator) genFnDecl(fn *parser.FnDecl) {
	var params []string
	for _, p := range fn.Params {
		if p.Name == "self" {
			continue
		}
		params = append(params, fmt.Sprintf("%s %s", p.Name, g.goTypeExpr(p.Type)))
	}

	retType := ""
	if fn.ReturnType != nil {
		goRet := g.goTypeExpr(fn.ReturnType)
		if len(fn.ErrorTypes) > 0 {
			retType = fmt.Sprintf(" (%s, error)", goRet)
		} else {
			retType = " " + goRet
		}
	} else if len(fn.ErrorTypes) > 0 {
		retType = " error"
	}

	g.writeln(fmt.Sprintf("func %s(%s)%s {", fn.Name, strings.Join(params, ", "), retType))
	g.indent++
	g.genFnBody(fn)
	g.indent--
	g.writeln("}")
	g.writeln("")
}

func (g *Generator) genFnBody(fn *parser.FnDecl) {
	if fn.Body == nil {
		return
	}
	if block, ok := fn.Body.(*parser.BlockExpr); ok {
		g.genBlockStmts(block)
		if block.Expr != nil {
			if fn.ReturnType != nil {
				g.writeIndent()
				g.write("return ")
				g.genReturnExpr(block.Expr, fn.ReturnType)
				g.write("\n")
			} else {
				g.writeIndent()
				g.genExpr(block.Expr)
				g.write("\n")
			}
		}
	} else {
		// Single expression body
		if fn.ReturnType != nil {
			g.writeIndent()
			g.write("return ")
			g.genReturnExpr(fn.Body, fn.ReturnType)
			g.write("\n")
		} else {
			g.writeIndent()
			g.genExpr(fn.Body)
			g.write("\n")
		}
	}
}

// genReturnExpr generates an expression for a return statement, adding
// type assertions when the expression returns interface{} but the
// function expects a concrete type (e.g., match expressions).
func (g *Generator) genReturnExpr(expr parser.Expr, retType parser.TypeExpr) {
	needsAssert := false
	switch expr.(type) {
	case *parser.MatchExpr, *parser.IfExpr:
		needsAssert = true
	}

	if needsAssert && retType != nil {
		goType := g.goTypeExpr(retType)
		if goType != "interface{}" {
			g.genExpr(expr)
			g.write(".(" + goType + ")")
			return
		}
	}
	g.genExpr(expr)
}

// ---------- Constants ----------

func (g *Generator) genConstDecl(cd *parser.ConstDecl) {
	g.writeIndent()
	g.write(fmt.Sprintf("var %s = ", cd.Name))
	g.genExpr(cd.Value)
	g.write("\n")
	g.writeln("")
}

// ---------- Entry block ----------

func (g *Generator) genEntryBlock(eb *parser.EntryBlock) {
	g.writeln("func main() {")
	g.indent++
	g.genBlockStmts(eb.Body)
	if eb.Body.Expr != nil {
		g.writeIndent()
		g.genExpr(eb.Body.Expr)
		g.write("\n")
	}
	g.indent--
	g.writeln("}")
	g.writeln("")
}

// ---------- Expressions ----------

func (g *Generator) genExpr(expr parser.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *parser.IntLitExpr:
		g.write("int64(" + e.Value + ")")

	case *parser.FloatLitExpr:
		// Ensure Go treats as float64
		val := e.Value
		if !strings.Contains(val, ".") {
			val += ".0"
		}
		g.write(val)

	case *parser.StringLitExpr:
		g.write(fmt.Sprintf("%q", e.Value))

	case *parser.BoolLitExpr:
		if e.Value {
			g.write("true")
		} else {
			g.write("false")
		}

	case *parser.InterpolatedStringExpr:
		g.genInterpolatedString(e)

	case *parser.IdentExpr:
		// Check if it's a sum type variant constructor (no args)
		if _, ok := g.types[e.Name]; ok {
			g.write(e.Name)
		} else if g.isSumVariant(e.Name) {
			g.write(g.variantGoType(e.Name) + "{}")
		} else {
			g.write(e.Name)
		}

	case *parser.PathExpr:
		g.write(strings.Join(e.Parts, "."))

	case *parser.BinaryExpr:
		g.write("(")
		g.genExpr(e.Left)
		g.write(" " + goOp(e.Op) + " ")
		g.genExpr(e.Right)
		g.write(")")

	case *parser.UnaryExpr:
		g.write(goOp(e.Op))
		g.genExpr(e.Operand)

	case *parser.PostfixExpr:
		// ? and ! are handled differently in statement context
		g.genExpr(e.Operand)

	case *parser.CallExpr:
		g.genCallExpr(e)

	case *parser.MethodCallExpr:
		g.genExpr(e.Object)
		g.write("." + e.Method + "(")
		for i, arg := range e.Args {
			if i > 0 {
				g.write(", ")
			}
			g.genExpr(arg.Value)
		}
		g.write(")")

	case *parser.FieldAccessExpr:
		g.genExpr(e.Object)
		g.write("." + exportField(e.Field))

	case *parser.OptionalChainExpr:
		// Simplified: just access the field
		g.genExpr(e.Object)
		g.write("." + exportField(e.Field))

	case *parser.IndexExpr:
		g.genExpr(e.Object)
		g.write("[")
		g.genExpr(e.Index)
		g.write("]")

	case *parser.PipelineExpr:
		// x |> f -> f(x)
		g.genPipeline(e)

	case *parser.RangeExpr:
		// Ranges need runtime support; for now emit a comment
		g.write("/* range */ nil")

	case *parser.BlockExpr:
		g.write("func() {\n")
		g.indent++
		g.genBlockStmts(e)
		if e.Expr != nil {
			g.writeIndent()
			g.genExpr(e.Expr)
			g.write("\n")
		}
		g.indent--
		g.writeIndent()
		g.write("}()")

	case *parser.IfExpr:
		g.genIfExpr(e)

	case *parser.MatchExpr:
		g.genMatchExpr(e)

	case *parser.ClosureExpr:
		g.genClosureExpr(e)

	case *parser.StructExpr:
		g.genStructExpr(e)

	case *parser.ArrayExpr:
		g.genArrayExpr(e)

	case *parser.TupleExpr:
		// Go doesn't have tuples; use a struct or just the elements
		if len(e.Elements) == 0 {
			g.write("struct{}{}")
		} else {
			g.genExpr(e.Elements[0])
		}

	case *parser.MapExpr:
		g.write("map[string]interface{}{")
		for i, entry := range e.Entries {
			if i > 0 {
				g.write(", ")
			}
			g.genExpr(entry.Key)
			g.write(": ")
			g.genExpr(entry.Value)
		}
		g.write("}")

	case *parser.ListCompExpr:
		g.genListComp(e)

	case *parser.GroupExpr:
		g.write("(")
		g.genExpr(e.Inner)
		g.write(")")

	case *parser.CatchExpr:
		// Catch expressions are complex; handle in statement context
		g.genExpr(e.Expr)

	case *parser.RecordUpdateExpr:
		g.genRecordUpdate(e)

	case *parser.FieldShorthandExpr:
		// In pipeline context
		g.write("." + exportField(e.Field))

	default:
		g.write("/* unsupported expr */")
	}
}

func (g *Generator) genCallExpr(e *parser.CallExpr) {
	// Special handling for built-in functions
	if ident, ok := e.Func.(*parser.IdentExpr); ok {
		switch ident.Name {
		case "println":
			g.write("fmt.Println(")
			for i, arg := range e.Args {
				if i > 0 {
					g.write(", ")
				}
				g.genExpr(arg.Value)
			}
			g.write(")")
			return
		case "print":
			g.write("fmt.Print(")
			for i, arg := range e.Args {
				if i > 0 {
					g.write(", ")
				}
				g.genExpr(arg.Value)
			}
			g.write(")")
			return
		case "panic":
			g.write("panic(")
			if len(e.Args) > 0 {
				g.genExpr(e.Args[0].Value)
			}
			g.write(")")
			return
		case "assert":
			g.write("/* assert */ if !(")
			if len(e.Args) > 0 {
				g.genExpr(e.Args[0].Value)
			}
			g.write(`) { panic("assertion failed") }`)
			return
		}

		// Check if it's a sum type variant constructor
		if g.isSumVariant(ident.Name) {
			goType := g.variantGoType(ident.Name)
			g.write(goType + "{")
			for i, arg := range e.Args {
				if i > 0 {
					g.write(", ")
				}
				g.write(fmt.Sprintf("F%d: ", i))
				g.genExpr(arg.Value)
			}
			g.write("}")
			return
		}
	}

	// General function call
	g.genExpr(e.Func)
	g.write("(")
	for i, arg := range e.Args {
		if i > 0 {
			g.write(", ")
		}
		g.genExpr(arg.Value)
	}
	g.write(")")
}

func (g *Generator) genInterpolatedString(e *parser.InterpolatedStringExpr) {
	// Collect format parts and args
	var fmtParts []string
	var args []parser.Expr

	for _, part := range e.Parts {
		if sl, ok := part.(*parser.StringLitExpr); ok {
			// Escape % in format string
			escaped := strings.ReplaceAll(sl.Value, "%", "%%")
			fmtParts = append(fmtParts, escaped)
		} else {
			fmtParts = append(fmtParts, "%v")
			args = append(args, part)
		}
	}

	g.write("fmt.Sprintf(\"")
	g.write(strings.Join(fmtParts, ""))
	g.write("\"")
	for _, arg := range args {
		g.write(", ")
		g.genExpr(arg)
	}
	g.write(")")
}

func (g *Generator) genPipeline(e *parser.PipelineExpr) {
	// x |> f -> f(x)
	if call, ok := e.Right.(*parser.CallExpr); ok {
		// x |> f(extra_args) -> f(x, extra_args)
		g.genExpr(call.Func)
		g.write("(")
		g.genExpr(e.Left)
		for _, arg := range call.Args {
			g.write(", ")
			g.genExpr(arg.Value)
		}
		g.write(")")
	} else if fs, ok := e.Right.(*parser.FieldShorthandExpr); ok {
		// x |> .field -> x.field
		g.genExpr(e.Left)
		g.write("." + exportField(fs.Field))
	} else {
		// x |> f -> f(x)
		g.genExpr(e.Right)
		g.write("(")
		g.genExpr(e.Left)
		g.write(")")
	}
}

func (g *Generator) genIfExpr(e *parser.IfExpr) {
	// Go doesn't have if expressions; use a func literal
	g.write("func() interface{} {\n")
	g.indent++
	g.writeIndent()
	g.write("if ")
	g.genExpr(e.Cond)
	g.write(" {\n")
	g.indent++
	g.writeIndent()
	g.write("return ")
	if e.Then.Expr != nil {
		g.genExpr(e.Then.Expr)
	} else {
		g.write("nil")
	}
	g.write("\n")
	g.indent--
	g.writeIndent()
	if e.Else != nil {
		g.write("} else {\n")
		g.indent++
		g.writeIndent()
		g.write("return ")
		if elseBlock, ok := e.Else.(*parser.BlockExpr); ok && elseBlock.Expr != nil {
			g.genExpr(elseBlock.Expr)
		} else if elseIf, ok := e.Else.(*parser.IfExpr); ok {
			g.genIfExpr(elseIf)
		} else {
			g.write("nil")
		}
		g.write("\n")
		g.indent--
		g.writeIndent()
		g.write("}\n")
	} else {
		g.write("}\n")
		g.writeIndent()
		g.write("return nil\n")
	}
	g.indent--
	g.writeIndent()
	g.write("}()")
}

func (g *Generator) genMatchExpr(e *parser.MatchExpr) {
	subjectType := g.lookupSumTypeForMatch(e)

	if subjectType != nil {
		// Sum type match -> type switch with concrete return
		tmp := g.newTmp()
		g.write("func() interface{} {\n")
		g.indent++
		g.writeIndent()
		g.write(fmt.Sprintf("switch %s := (", tmp))
		g.genExpr(e.Subject)
		g.write(").(type) {\n")

		for _, arm := range e.Arms {
			g.genMatchArm(arm, tmp, subjectType)
		}

		g.writeIndent()
		g.write("}\n")
		g.writeIndent()
		g.write("return nil\n")
		g.indent--
		g.writeIndent()
		g.write("}()")
	} else {
		// Value match -> switch
		g.write("func() interface{} {\n")
		g.indent++
		g.writeIndent()
		g.write("switch ")
		g.genExpr(e.Subject)
		g.write(" {\n")

		for _, arm := range e.Arms {
			g.genValueMatchArm(arm)
		}

		g.writeIndent()
		g.write("}\n")
		g.writeIndent()
		g.write("return nil\n")
		g.indent--
		g.writeIndent()
		g.write("}()")
	}
}

func (g *Generator) genMatchArm(arm *parser.MatchArm, tmpVar string, sumType *checker.SumType) {
	switch p := arm.Pattern.(type) {
	case *parser.VariantPattern:
		goType := sumType.Name + p.Name
		g.writeIndent()
		g.write(fmt.Sprintf("case %s:\n", goType))
		g.indent++
		// Bind variant fields
		for i, arg := range p.Args {
			if bp, ok := arg.(*parser.BindingPattern); ok && bp.Name != "_" {
				g.writeln(fmt.Sprintf("%s := %s.F%d", bp.Name, tmpVar, i))
			}
		}
		if arm.Guard != nil {
			g.writeIndent()
			g.write("if ")
			g.genExpr(arm.Guard)
			g.write(" {\n")
			g.indent++
		}
		g.writeIndent()
		g.write("return ")
		g.genExpr(arm.Body)
		g.write("\n")
		if arm.Guard != nil {
			g.indent--
			g.writeIndent()
			g.write("}\n")
		}
		g.indent--

	case *parser.BindingPattern:
		// Check if this name matches a known variant (unit variant like Point)
		if g.isVariantOf(p.Name, sumType) {
			goType := sumType.Name + p.Name
			g.writeIndent()
			g.write(fmt.Sprintf("case %s:\n", goType))
		} else if p.Name == "_" {
			g.writeIndent()
			g.write("default:\n")
		} else {
			g.writeIndent()
			g.write("default:\n")
			g.indent++
			g.writeln(fmt.Sprintf("%s := %s", p.Name, tmpVar))
			g.writeln("_ = " + p.Name)
			g.indent--
		}
		g.indent++
		g.writeIndent()
		g.write("return ")
		g.genExpr(arm.Body)
		g.write("\n")
		g.indent--

	case *parser.WildcardPattern:
		g.writeIndent()
		g.write("default:\n")
		g.indent++
		g.writeIndent()
		g.write("return ")
		g.genExpr(arm.Body)
		g.write("\n")
		g.indent--
	}
}

func (g *Generator) genValueMatchArm(arm *parser.MatchArm) {
	switch p := arm.Pattern.(type) {
	case *parser.LiteralPattern:
		g.writeIndent()
		g.write("case ")
		g.genExpr(p.Value)
		g.write(":\n")
	case *parser.BindingPattern:
		if p.Name == "_" {
			g.writeIndent()
			g.write("default:\n")
		} else {
			g.writeIndent()
			g.write("default:\n")
		}
	case *parser.WildcardPattern:
		g.writeIndent()
		g.write("default:\n")
	default:
		g.writeIndent()
		g.write("default:\n")
	}
	g.indent++
	g.writeIndent()
	g.write("return ")
	g.genExpr(arm.Body)
	g.write("\n")
	g.indent--
}

func (g *Generator) genClosureExpr(e *parser.ClosureExpr) {
	g.write("func(")
	for i, p := range e.Params {
		if i > 0 {
			g.write(", ")
		}
		if p.Type != nil {
			g.write(fmt.Sprintf("%s %s", p.Name, g.goTypeExpr(p.Type)))
		} else {
			g.write(fmt.Sprintf("%s interface{}", p.Name))
		}
	}
	g.write(")")
	if e.Return != nil {
		g.write(" " + g.goTypeExpr(e.Return))
	} else {
		// Infer return type from body for simple expressions
		retType := g.inferExprGoType(e.Body)
		if retType != "" {
			g.write(" " + retType)
		}
	}
	g.write(" {\n")
	g.indent++
	g.writeIndent()
	g.write("return ")
	g.genExpr(e.Body)
	g.write("\n")
	g.indent--
	g.writeIndent()
	g.write("}")
}

// inferExprGoType tries to determine the Go type of a simple expression.
func (g *Generator) inferExprGoType(expr parser.Expr) string {
	switch e := expr.(type) {
	case *parser.IntLitExpr:
		return "int64"
	case *parser.FloatLitExpr:
		return "float64"
	case *parser.StringLitExpr:
		return "string"
	case *parser.BoolLitExpr:
		return "bool"
	case *parser.InterpolatedStringExpr:
		return "string"
	case *parser.BinaryExpr:
		switch e.Op {
		case lexer.EqEq, lexer.BangEq, lexer.Lt, lexer.Gt, lexer.LtEq, lexer.GtEq,
			lexer.AmpAmp, lexer.PipePipe:
			return "bool"
		default:
			left := g.inferExprGoType(e.Left)
			if left != "" {
				return left
			}
			return g.inferExprGoType(e.Right)
		}
	case *parser.UnaryExpr:
		if e.Op == lexer.Bang {
			return "bool"
		}
		return g.inferExprGoType(e.Operand)
	case *parser.GroupExpr:
		return g.inferExprGoType(e.Inner)
	case *parser.CallExpr:
		return "" // can't easily infer
	case *parser.IdentExpr:
		return "" // would need type env
	default:
		return ""
	}
}

func (g *Generator) genStructExpr(e *parser.StructExpr) {
	g.write(e.TypeName + "{")
	for i, f := range e.Fields {
		if i > 0 {
			g.write(", ")
		}
		g.write(exportField(f.Name) + ": ")
		if f.Value != nil {
			g.genExpr(f.Value)
		} else {
			// Shorthand: field name is the variable name
			g.write(f.Name)
		}
	}
	g.write("}")
}

func (g *Generator) genArrayExpr(e *parser.ArrayExpr) {
	if len(e.Elements) == 0 {
		g.write("[]interface{}{}")
		return
	}
	// Infer element type from first element
	elemType := g.inferExprGoType(e.Elements[0])
	if elemType == "" {
		elemType = "interface{}"
	}
	g.write("[]" + elemType + "{")
	for i, el := range e.Elements {
		if i > 0 {
			g.write(", ")
		}
		g.genExpr(el)
	}
	g.write("}")
}

func (g *Generator) genListComp(e *parser.ListCompExpr) {
	// [expr for v in iter] -> func() []T { var result []T; for _, v := range iter { result = append(result, expr) }; return result }()
	g.write("func() []interface{} {\n")
	g.indent++
	g.writeln("var _result []interface{}")
	g.writeIndent()
	g.write(fmt.Sprintf("for _, %s := range ", e.Var))
	g.genExpr(e.Iter)
	g.write(" {\n")
	g.indent++
	if e.Where != nil {
		g.writeIndent()
		g.write("if ")
		g.genExpr(e.Where)
		g.write(" {\n")
		g.indent++
	}
	g.writeIndent()
	g.write("_result = append(_result, ")
	g.genExpr(e.Expr)
	g.write(")\n")
	if e.Where != nil {
		g.indent--
		g.writeIndent()
		g.write("}\n")
	}
	g.indent--
	g.writeIndent()
	g.write("}\n")
	g.writeln("return _result")
	g.indent--
	g.writeIndent()
	g.write("}()")
}

func (g *Generator) genRecordUpdate(e *parser.RecordUpdateExpr) {
	// Copy struct and update fields
	tmp := g.newTmp()
	g.write(fmt.Sprintf("func() interface{} { %s := ", tmp))
	g.genExpr(e.Object)
	g.write("; ")
	for _, f := range e.Fields {
		g.write(fmt.Sprintf("%s.%s = ", tmp, exportField(f.Name)))
		if f.Value != nil {
			g.genExpr(f.Value)
		}
		g.write("; ")
	}
	g.write(fmt.Sprintf("return %s }()", tmp))
}

// ---------- Statements ----------

func (g *Generator) genBlockStmts(block *parser.BlockExpr) {
	for _, stmt := range block.Stmts {
		g.genStmt(stmt)
	}
}

func (g *Generator) genStmt(stmt parser.Stmt) {
	switch s := stmt.(type) {
	case *parser.VarDeclStmt:
		g.writeIndent()
		g.write(s.Name + " := ")
		g.genExpr(s.Value)
		g.write("\n")
		g.writeln("_ = " + s.Name)

	case *parser.AssignStmt:
		g.writeIndent()
		g.genExpr(s.Target)
		g.write(" = ")
		g.genExpr(s.Value)
		g.write("\n")

	case *parser.ExprStmt:
		g.writeIndent()
		g.genExpr(s.Expr)
		g.write("\n")

	case *parser.ForStmt:
		g.genForStmt(s)

	case *parser.WhileStmt:
		g.writeIndent()
		g.write("for ")
		g.genExpr(s.Cond)
		g.write(" {\n")
		g.indent++
		g.genBlockStmts(s.Body)
		if s.Body.Expr != nil {
			g.writeIndent()
			g.genExpr(s.Body.Expr)
			g.write("\n")
		}
		g.indent--
		g.writeln("}")

	case *parser.LoopStmt:
		g.writeln("for {")
		g.indent++
		g.genBlockStmts(s.Body)
		if s.Body.Expr != nil {
			g.writeIndent()
			g.genExpr(s.Body.Expr)
			g.write("\n")
		}
		g.indent--
		g.writeln("}")

	case *parser.ReturnStmt:
		g.writeIndent()
		g.write("return")
		if s.Value != nil {
			g.write(" ")
			g.genExpr(s.Value)
		}
		g.write("\n")

	case *parser.BreakStmt:
		g.writeln("break")

	case *parser.ContinueStmt:
		g.writeln("continue")

	case *parser.DeferStmt:
		g.writeIndent()
		g.write("defer ")
		g.genExpr(s.Expr)
		g.write("\n")
	}
}

func (g *Generator) genForStmt(s *parser.ForStmt) {
	g.writeIndent()
	if bp, ok := s.Pattern.(*parser.BindingPattern); ok {
		g.write(fmt.Sprintf("for _, %s := range ", bp.Name))
	} else {
		g.write("for _, _v := range ")
	}
	g.genExpr(s.Iter)
	g.write(" {\n")
	g.indent++
	g.genBlockStmts(s.Body)
	if s.Body.Expr != nil {
		g.writeIndent()
		g.genExpr(s.Body.Expr)
		g.write("\n")
	}
	g.indent--
	g.writeln("}")
}

// ---------- Test body generation ----------

func (g *Generator) genTestBody(block *parser.BlockExpr) {
	for _, stmt := range block.Stmts {
		g.genTestStmt(stmt)
	}
	if block.Expr != nil {
		g.genTestExprStmt(block.Expr)
	}
}

func (g *Generator) genTestStmt(stmt parser.Stmt) {
	if es, ok := stmt.(*parser.ExprStmt); ok {
		g.genTestExprStmt(es.Expr)
		return
	}
	g.genStmt(stmt)
}

func (g *Generator) genTestExprStmt(expr parser.Expr) {
	// Convert assert calls to t.Fatal
	if call, ok := expr.(*parser.CallExpr); ok {
		if ident, ok := call.Func.(*parser.IdentExpr); ok && ident.Name == "assert" {
			if len(call.Args) > 0 {
				g.writeIndent()
				g.write("if !(")
				g.genExpr(call.Args[0].Value)
				g.write(`) { t.Fatal("assertion failed") }`)
				g.write("\n")
				return
			}
		}
	}
	g.writeIndent()
	g.genExpr(expr)
	g.write("\n")
}

// ---------- Type mapping ----------

func (g *Generator) goTypeExpr(te parser.TypeExpr) string {
	if te == nil {
		return "interface{}"
	}
	switch t := te.(type) {
	case *parser.NamedTypeExpr:
		name := t.Path[len(t.Path)-1]
		return goTypeName(name)
	case *parser.ArrayTypeExpr:
		return "[]" + g.goTypeExpr(t.Element)
	case *parser.MapTypeExpr:
		return fmt.Sprintf("map[%s]%s", g.goTypeExpr(t.Key), g.goTypeExpr(t.Value))
	case *parser.OptionalTypeExpr:
		return "*" + g.goTypeExpr(t.Inner)
	case *parser.ResultTypeExpr:
		return g.goTypeExpr(t.Ok)
	case *parser.FunctionTypeExpr:
		var params []string
		for _, p := range t.Params {
			params = append(params, g.goTypeExpr(p))
		}
		return fmt.Sprintf("func(%s) %s", strings.Join(params, ", "), g.goTypeExpr(t.Return))
	case *parser.TupleTypeExpr:
		// Use first element or interface{}
		if len(t.Elements) > 0 {
			return g.goTypeExpr(t.Elements[0])
		}
		return "interface{}"
	default:
		return "interface{}"
	}
}

func goTypeName(name string) string {
	switch name {
	case "i8":
		return "int8"
	case "i16":
		return "int16"
	case "i32":
		return "int32"
	case "i64":
		return "int64"
	case "u8":
		return "uint8"
	case "u16":
		return "uint16"
	case "u32":
		return "uint32"
	case "u64":
		return "uint64"
	case "f32":
		return "float32"
	case "f64":
		return "float64"
	case "str":
		return "string"
	case "bool":
		return "bool"
	case "byte":
		return "byte"
	case "usize":
		return "uint"
	case "Self":
		return "interface{}"
	default:
		return name
	}
}

func goOp(op lexer.TokenType) string {
	switch op {
	case lexer.Plus:
		return "+"
	case lexer.Minus:
		return "-"
	case lexer.Star:
		return "*"
	case lexer.Slash:
		return "/"
	case lexer.Percent:
		return "%"
	case lexer.EqEq:
		return "=="
	case lexer.BangEq:
		return "!="
	case lexer.Lt:
		return "<"
	case lexer.Gt:
		return ">"
	case lexer.LtEq:
		return "<="
	case lexer.GtEq:
		return ">="
	case lexer.AmpAmp:
		return "&&"
	case lexer.PipePipe:
		return "||"
	case lexer.Amp:
		return "&"
	case lexer.Pipe:
		return "|"
	case lexer.Caret:
		return "^"
	case lexer.LtLt:
		return "<<"
	case lexer.GtGt:
		return ">>"
	case lexer.Bang:
		return "!"
	case lexer.Tilde:
		return "^" // Go uses ^ for bitwise NOT
	default:
		return "?"
	}
}

func exportField(name string) string {
	if len(name) == 0 {
		return name
	}
	// Go requires exported fields for struct literals across packages
	return strings.ToUpper(name[:1]) + name[1:]
}

// ---------- Sum type helpers ----------

func (g *Generator) isSumVariant(name string) bool {
	for _, t := range g.types {
		if st, ok := t.(*checker.SumType); ok {
			for _, v := range st.Variants {
				if v.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func (g *Generator) variantGoType(variantName string) string {
	for _, t := range g.types {
		if st, ok := t.(*checker.SumType); ok {
			for _, v := range st.Variants {
				if v.Name == variantName {
					return st.Name + v.Name
				}
			}
		}
	}
	return variantName
}

func (g *Generator) isVariantOf(name string, st *checker.SumType) bool {
	for _, v := range st.Variants {
		if v.Name == name {
			return true
		}
	}
	return false
}

// lookupSumTypeForMatch determines the sum type for a match expression
// by examining the patterns used in the arms.
func (g *Generator) lookupSumTypeForMatch(e *parser.MatchExpr) *checker.SumType {
	// Look at patterns to find variant names, then match to sum types
	for _, arm := range e.Arms {
		if vp, ok := arm.Pattern.(*parser.VariantPattern); ok {
			for _, t := range g.types {
				if st, ok := t.(*checker.SumType); ok {
					for _, v := range st.Variants {
						if v.Name == vp.Name {
							return st
						}
					}
				}
			}
		}
		// Also check binding patterns that might be unit variants
		if bp, ok := arm.Pattern.(*parser.BindingPattern); ok {
			for _, t := range g.types {
				if st, ok := t.(*checker.SumType); ok {
					for _, v := range st.Variants {
						if v.Name == bp.Name {
							return st
						}
					}
				}
			}
		}
	}
	return nil
}
