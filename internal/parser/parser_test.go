package parser

import (
	"strings"
	"testing"

	"github.com/aria-lang/aria/internal/lexer"
)

func parse(source string) (*Program, *Parser) {
	l := lexer.New("test.aria", source)
	tokens := l.Tokenize()
	p := New(tokens)
	prog := p.Parse()
	return prog, p
}

func parseNoErrors(t *testing.T, source string) *Program {
	t.Helper()
	prog, p := parse(source)
	if p.Diagnostics().HasErrors() {
		t.Fatalf("unexpected parse errors for %q:\n%v", source, p.Diagnostics().Diagnostics)
	}
	return prog
}

// ===== Phase 2.1: AST Types =====

func TestASTProgramStructure(t *testing.T) {
	prog := parseNoErrors(t, "mod main\nuse std.fs")
	if prog.Module == nil {
		t.Fatal("expected module declaration")
	}
	if prog.Module.Name != "main" {
		t.Errorf("expected module 'main', got %q", prog.Module.Name)
	}
	if len(prog.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(prog.Imports))
	}
	if strings.Join(prog.Imports[0].Path, ".") != "std.fs" {
		t.Errorf("expected import 'std.fs', got %q", strings.Join(prog.Imports[0].Path, "."))
	}
}

// ===== Phase 2.2: Declarations =====

func TestParseFnDecl(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn add(a: i64, b: i64) -> i64 = a + b`)
	if len(prog.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(prog.Decls))
	}
	fn, ok := prog.Decls[0].(*FnDecl)
	if !ok {
		t.Fatalf("expected FnDecl, got %T", prog.Decls[0])
	}
	if fn.Name != "add" {
		t.Errorf("expected fn name 'add', got %q", fn.Name)
	}
	if len(fn.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.ReturnType == nil {
		t.Error("expected return type")
	}
}

func TestParseFnDeclWithBlock(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn greet(name: str) {
    println("Hello")
}`)
	if len(prog.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(prog.Decls))
	}
	fn := prog.Decls[0].(*FnDecl)
	if fn.Name != "greet" {
		t.Errorf("expected fn name 'greet', got %q", fn.Name)
	}
	body, ok := fn.Body.(*BlockExpr)
	if !ok {
		t.Fatalf("expected BlockExpr body, got %T", fn.Body)
	}
	// The block should have some content
	_ = body
}

func TestParseFnDeclWithGenerics(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn map[T, U](list: [T], f: fn(T) -> U) -> [U] {
    []
}`)
	fn := prog.Decls[0].(*FnDecl)
	if len(fn.GenericParams) != 2 {
		t.Fatalf("expected 2 generic params, got %d", len(fn.GenericParams))
	}
	if fn.GenericParams[0].Name != "T" || fn.GenericParams[1].Name != "U" {
		t.Error("wrong generic param names")
	}
}

func TestParseFnDeclWithErrorsAndEffects(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn readFile(path: str) -> str ! IoError with [Io, Fs] {
    ""
}`)
	fn := prog.Decls[0].(*FnDecl)
	if len(fn.ErrorTypes) != 1 {
		t.Fatalf("expected 1 error type, got %d", len(fn.ErrorTypes))
	}
	if len(fn.Effects) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(fn.Effects))
	}
	if fn.Effects[0] != "Io" || fn.Effects[1] != "Fs" {
		t.Errorf("expected effects [Io, Fs], got %v", fn.Effects)
	}
}

func TestParseFnDeclWithMultipleErrors(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn process() -> Response ! AuthError | DbError {
    Response{}
}`)
	fn := prog.Decls[0].(*FnDecl)
	if len(fn.ErrorTypes) != 2 {
		t.Fatalf("expected 2 error types, got %d", len(fn.ErrorTypes))
	}
}

func TestParsePubFn(t *testing.T) {
	prog := parseNoErrors(t, `mod main
pub fn hello() = println("hi")`)
	fn := prog.Decls[0].(*FnDecl)
	if fn.Vis != Public {
		t.Error("expected Public visibility")
	}
}

func TestParseStructDecl(t *testing.T) {
	prog := parseNoErrors(t, `mod main
struct Point {
    x: f64
    y: f64
}`)
	td := prog.Decls[0].(*TypeDecl)
	if td.Kind != StructDecl {
		t.Fatalf("expected StructDecl, got %v", td.Kind)
	}
	if td.Name != "Point" {
		t.Errorf("expected name 'Point', got %q", td.Name)
	}
	if len(td.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(td.Fields))
	}
}

func TestParseTypeAsStruct(t *testing.T) {
	prog := parseNoErrors(t, `mod main
type User {
    name: str
    age: u8 = 0
} derives [Eq, Hash]`)
	td := prog.Decls[0].(*TypeDecl)
	if td.Kind != StructDecl {
		t.Fatalf("expected StructDecl, got %v", td.Kind)
	}
	if len(td.Derives) != 2 {
		t.Fatalf("expected 2 derives, got %d", len(td.Derives))
	}
	if td.Fields[1].Default == nil {
		t.Error("expected default value for 'age' field")
	}
}

func TestParseSumType(t *testing.T) {
	prog := parseNoErrors(t, `mod main
type Shape =
    | Circle(f64)
    | Rect { w: f64, h: f64 }
    | Point`)
	td := prog.Decls[0].(*TypeDecl)
	if td.Kind != SumTypeDecl {
		t.Fatalf("expected SumTypeDecl, got %v", td.Kind)
	}
	if len(td.Variants) != 3 {
		t.Fatalf("expected 3 variants, got %d", len(td.Variants))
	}
	if td.Variants[0].Name != "Circle" {
		t.Error("expected first variant 'Circle'")
	}
	if len(td.Variants[0].Types) != 1 {
		t.Error("expected Circle to have 1 tuple type")
	}
	if len(td.Variants[1].Fields) != 2 {
		t.Error("expected Rect to have 2 fields")
	}
	if len(td.Variants[2].Types) != 0 && len(td.Variants[2].Fields) != 0 {
		t.Error("expected Point to be a unit variant")
	}
}

func TestParseNewtype(t *testing.T) {
	prog := parseNoErrors(t, `mod main
type UserId = i64`)
	td := prog.Decls[0].(*TypeDecl)
	if td.Kind != NewtypeDecl {
		t.Fatalf("expected NewtypeDecl, got %v", td.Kind)
	}
	if td.Underlying == nil {
		t.Fatal("expected underlying type")
	}
}

func TestParseEnum(t *testing.T) {
	prog := parseNoErrors(t, `mod main
enum Color { Red, Green, Blue }`)
	ed := prog.Decls[0].(*EnumDecl)
	if ed.Name != "Color" {
		t.Errorf("expected 'Color', got %q", ed.Name)
	}
	if len(ed.Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(ed.Members))
	}
}

func TestParseAlias(t *testing.T) {
	prog := parseNoErrors(t, `mod main
alias Headers = Map[str, str]`)
	ad := prog.Decls[0].(*AliasDecl)
	if ad.Name != "Headers" {
		t.Errorf("expected 'Headers', got %q", ad.Name)
	}
}

func TestParseTrait(t *testing.T) {
	prog := parseNoErrors(t, `mod main
trait Display {
    fn display(self) -> str
}`)
	td := prog.Decls[0].(*TraitDecl)
	if td.Name != "Display" {
		t.Errorf("expected 'Display', got %q", td.Name)
	}
	if len(td.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(td.Methods))
	}
}

func TestParseTraitWithSupertrait(t *testing.T) {
	prog := parseNoErrors(t, `mod main
trait Ord: Eq {
    fn cmp(self, other: Self) -> i64
}`)
	td := prog.Decls[0].(*TraitDecl)
	if len(td.Supertraits) != 1 || td.Supertraits[0] != "Eq" {
		t.Errorf("expected supertrait 'Eq', got %v", td.Supertraits)
	}
}

func TestParseImpl(t *testing.T) {
	prog := parseNoErrors(t, `mod main
impl Display for Point {
    fn display(self) -> str = "point"
}`)
	id := prog.Decls[0].(*ImplDecl)
	if id.TraitName != "Display" {
		t.Errorf("expected trait 'Display', got %q", id.TraitName)
	}
	if id.TypeName != "Point" {
		t.Errorf("expected type 'Point', got %q", id.TypeName)
	}
}

func TestParseInherentImpl(t *testing.T) {
	prog := parseNoErrors(t, `mod main
impl Point {
    fn distance(self) -> f64 = 0.0
}`)
	id := prog.Decls[0].(*ImplDecl)
	if id.TraitName != "" {
		t.Errorf("expected no trait name for inherent impl, got %q", id.TraitName)
	}
	if id.TypeName != "Point" {
		t.Errorf("expected type 'Point', got %q", id.TypeName)
	}
}

func TestParseConst(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const MAX_RETRIES = 3`)
	cd := prog.Decls[0].(*ConstDecl)
	if cd.Name != "MAX_RETRIES" {
		t.Errorf("expected 'MAX_RETRIES', got %q", cd.Name)
	}
}

func TestParseEntry(t *testing.T) {
	prog := parseNoErrors(t, `mod main
entry {
    println("hello")
}`)
	eb := prog.Decls[0].(*EntryBlock)
	if eb.Body == nil {
		t.Fatal("expected entry body")
	}
}

func TestParseTest(t *testing.T) {
	prog := parseNoErrors(t, `mod main
test "addition" {
    assert 1 + 1 == 2
}`)
	tb := prog.Decls[0].(*TestBlock)
	if tb.Name != "addition" {
		t.Errorf("expected test name 'addition', got %q", tb.Name)
	}
}

// ===== Phase 2.3: Expressions =====

func TestParseBinaryExpr(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = 2 + 3 * 4`)
	cd := prog.Decls[0].(*ConstDecl)
	bin, ok := cd.Value.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", cd.Value)
	}
	if bin.Op != lexer.Plus {
		t.Errorf("expected +, got %s", bin.Op)
	}
	// Right side should be 3 * 4
	right, ok := bin.Right.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected right BinaryExpr, got %T", bin.Right)
	}
	if right.Op != lexer.Star {
		t.Errorf("expected *, got %s", right.Op)
	}
}

func TestParseUnaryExpr(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = -42`)
	cd := prog.Decls[0].(*ConstDecl)
	un, ok := cd.Value.(*UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", cd.Value)
	}
	if un.Op != lexer.Minus {
		t.Errorf("expected -, got %s", un.Op)
	}
}

func TestParsePostfixQuestion(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn foo() -> i64 ! Error {
    bar()?
}`)
	fn := prog.Decls[0].(*FnDecl)
	body := fn.Body.(*BlockExpr)
	exprStmt := body.Expr
	pf, ok := exprStmt.(*PostfixExpr)
	if !ok {
		t.Fatalf("expected PostfixExpr, got %T", exprStmt)
	}
	if pf.Op != lexer.Question {
		t.Errorf("expected ?, got %s", pf.Op)
	}
}

func TestParseFieldAccess(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = point.x`)
	cd := prog.Decls[0].(*ConstDecl)
	fa, ok := cd.Value.(*FieldAccessExpr)
	if !ok {
		t.Fatalf("expected FieldAccessExpr, got %T", cd.Value)
	}
	if fa.Field != "x" {
		t.Errorf("expected field 'x', got %q", fa.Field)
	}
}

func TestParseMethodCall(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = items.map(f)`)
	cd := prog.Decls[0].(*ConstDecl)
	mc, ok := cd.Value.(*MethodCallExpr)
	if !ok {
		t.Fatalf("expected MethodCallExpr, got %T", cd.Value)
	}
	if mc.Method != "map" {
		t.Errorf("expected method 'map', got %q", mc.Method)
	}
}

func TestParseFunctionCall(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = foo(1, 2)`)
	cd := prog.Decls[0].(*ConstDecl)
	call, ok := cd.Value.(*CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", cd.Value)
	}
	if len(call.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(call.Args))
	}
}

func TestParsePipeline(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = input |> parse |> validate`)
	cd := prog.Decls[0].(*ConstDecl)
	pipe, ok := cd.Value.(*PipelineExpr)
	if !ok {
		t.Fatalf("expected PipelineExpr, got %T", cd.Value)
	}
	// Should be left-associative: (input |> parse) |> validate
	_, ok = pipe.Left.(*PipelineExpr)
	if !ok {
		t.Fatalf("expected left PipelineExpr, got %T", pipe.Left)
	}
}

func TestParseRange(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = 1..10`)
	cd := prog.Decls[0].(*ConstDecl)
	r, ok := cd.Value.(*RangeExpr)
	if !ok {
		t.Fatalf("expected RangeExpr, got %T", cd.Value)
	}
	if r.Inclusive {
		t.Error("expected half-open range, got inclusive")
	}
}

func TestParseInclusiveRange(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = 1..=10`)
	cd := prog.Decls[0].(*ConstDecl)
	r, ok := cd.Value.(*RangeExpr)
	if !ok {
		t.Fatalf("expected RangeExpr, got %T", cd.Value)
	}
	if !r.Inclusive {
		t.Error("expected inclusive range")
	}
}

func TestParseIfExpr(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn f(x: i64) -> str {
    if x > 0 {
        "positive"
    } else {
        "non-positive"
    }
}`)
	fn := prog.Decls[0].(*FnDecl)
	body := fn.Body.(*BlockExpr)
	ifExpr, ok := body.Expr.(*IfExpr)
	if !ok {
		t.Fatalf("expected IfExpr, got %T", body.Expr)
	}
	if ifExpr.Else == nil {
		t.Error("expected else branch")
	}
}

func TestParseMatchExpr(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn f(s: Shape) -> f64 = match s {
    Circle(r) => 3.14 * r * r
    Rect(w, h) => w * h
    Point => 0.0
}`)
	fn := prog.Decls[0].(*FnDecl)
	m, ok := fn.Body.(*MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", fn.Body)
	}
	if len(m.Arms) != 3 {
		t.Fatalf("expected 3 arms, got %d", len(m.Arms))
	}
}

func TestParseClosureExpr(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const F = fn(x: i64) => x * 2`)
	cd := prog.Decls[0].(*ConstDecl)
	cl, ok := cd.Value.(*ClosureExpr)
	if !ok {
		t.Fatalf("expected ClosureExpr, got %T", cd.Value)
	}
	if len(cl.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(cl.Params))
	}
}

func TestParseArrayLiteral(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = [1, 2, 3]`)
	cd := prog.Decls[0].(*ConstDecl)
	arr, ok := cd.Value.(*ArrayExpr)
	if !ok {
		t.Fatalf("expected ArrayExpr, got %T", cd.Value)
	}
	if len(arr.Elements) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr.Elements))
	}
}

func TestParseStructExpr(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const P = Point{x: 1.0, y: 2.0}`)
	cd := prog.Decls[0].(*ConstDecl)
	se, ok := cd.Value.(*StructExpr)
	if !ok {
		t.Fatalf("expected StructExpr, got %T", cd.Value)
	}
	if se.TypeName != "Point" {
		t.Errorf("expected 'Point', got %q", se.TypeName)
	}
	if len(se.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(se.Fields))
	}
}

func TestParseInterpolatedString(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = "Hello, {name}!"`)
	cd := prog.Decls[0].(*ConstDecl)
	interp, ok := cd.Value.(*InterpolatedStringExpr)
	if !ok {
		t.Fatalf("expected InterpolatedStringExpr, got %T", cd.Value)
	}
	if len(interp.Parts) < 2 {
		t.Errorf("expected at least 2 parts, got %d", len(interp.Parts))
	}
}

func TestParseListComprehension(t *testing.T) {
	prog := parseNoErrors(t, `mod main
const X = [x * x for x in items]`)
	cd := prog.Decls[0].(*ConstDecl)
	lc, ok := cd.Value.(*ListCompExpr)
	if !ok {
		t.Fatalf("expected ListCompExpr, got %T", cd.Value)
	}
	if lc.Var != "x" {
		t.Errorf("expected var 'x', got %q", lc.Var)
	}
}

// ===== Phase 2.4: Patterns & Types =====

func TestParseImportGrouped(t *testing.T) {
	prog := parseNoErrors(t, `mod main
use std.{json, http}`)
	if len(prog.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(prog.Imports))
	}
	imp := prog.Imports[0]
	if len(imp.Names) != 2 {
		t.Fatalf("expected 2 grouped names, got %d", len(imp.Names))
	}
	if imp.Names[0] != "json" || imp.Names[1] != "http" {
		t.Errorf("expected [json, http], got %v", imp.Names)
	}
}

func TestParseImportAlias(t *testing.T) {
	prog := parseNoErrors(t, `mod main
use crypto.sha256 as sha`)
	imp := prog.Imports[0]
	if imp.Alias != "sha" {
		t.Errorf("expected alias 'sha', got %q", imp.Alias)
	}
}

func TestParseArrayType(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn f(x: [i64]) = x`)
	fn := prog.Decls[0].(*FnDecl)
	at, ok := fn.Params[0].Type.(*ArrayTypeExpr)
	if !ok {
		t.Fatalf("expected ArrayTypeExpr, got %T", fn.Params[0].Type)
	}
	nt := at.Element.(*NamedTypeExpr)
	if nt.Path[0] != "i64" {
		t.Errorf("expected element type 'i64', got %q", nt.Path[0])
	}
}

func TestParseGenericType(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn f(x: Map[str, i64]) = x`)
	fn := prog.Decls[0].(*FnDecl)
	nt, ok := fn.Params[0].Type.(*NamedTypeExpr)
	if !ok {
		t.Fatalf("expected NamedTypeExpr, got %T", fn.Params[0].Type)
	}
	if nt.Path[0] != "Map" {
		t.Errorf("expected 'Map', got %q", nt.Path[0])
	}
	if len(nt.TypeArgs) != 2 {
		t.Errorf("expected 2 type args, got %d", len(nt.TypeArgs))
	}
}

func TestParseOptionalType(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn f(x: str?) = x`)
	fn := prog.Decls[0].(*FnDecl)
	_, ok := fn.Params[0].Type.(*OptionalTypeExpr)
	if !ok {
		t.Fatalf("expected OptionalTypeExpr, got %T", fn.Params[0].Type)
	}
}

func TestParseFunctionType(t *testing.T) {
	prog := parseNoErrors(t, `mod main
fn f(callback: fn(i64) -> str) = callback(1)`)
	fn := prog.Decls[0].(*FnDecl)
	ft, ok := fn.Params[0].Type.(*FunctionTypeExpr)
	if !ok {
		t.Fatalf("expected FunctionTypeExpr, got %T", fn.Params[0].Type)
	}
	if len(ft.Params) != 1 {
		t.Errorf("expected 1 param type, got %d", len(ft.Params))
	}
}

// ===== Statements =====

func TestParseVarDecl(t *testing.T) {
	prog := parseNoErrors(t, `mod main
entry {
    x := 42
    mut y := 0
}`)
	eb := prog.Decls[0].(*EntryBlock)
	if len(eb.Body.Stmts) < 1 {
		t.Fatal("expected at least 1 statement")
	}
	vd, ok := eb.Body.Stmts[0].(*VarDeclStmt)
	if !ok {
		t.Fatalf("expected VarDeclStmt, got %T", eb.Body.Stmts[0])
	}
	if vd.Name != "x" {
		t.Errorf("expected name 'x', got %q", vd.Name)
	}
	if vd.Mutable {
		t.Error("expected immutable binding")
	}
}

func TestParseForLoop(t *testing.T) {
	prog := parseNoErrors(t, `mod main
entry {
    for x in items {
        println(x)
    }
}`)
	eb := prog.Decls[0].(*EntryBlock)
	fs, ok := eb.Body.Stmts[0].(*ForStmt)
	if !ok {
		t.Fatalf("expected ForStmt, got %T", eb.Body.Stmts[0])
	}
	bp, ok := fs.Pattern.(*BindingPattern)
	if !ok {
		t.Fatalf("expected BindingPattern, got %T", fs.Pattern)
	}
	if bp.Name != "x" {
		t.Errorf("expected 'x', got %q", bp.Name)
	}
}

func TestParseWhileLoop(t *testing.T) {
	prog := parseNoErrors(t, `mod main
entry {
    while x > 0 {
        x = x - 1
    }
}`)
	eb := prog.Decls[0].(*EntryBlock)
	_, ok := eb.Body.Stmts[0].(*WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt, got %T", eb.Body.Stmts[0])
	}
}

// ===== AST Serialization =====

func TestFormatAST(t *testing.T) {
	prog := parseNoErrors(t, `mod main
use std.fs

fn add(a: i64, b: i64) -> i64 = a + b

entry {
    println("hello")
}`)
	output := FormatAST(prog)
	if !strings.Contains(output, "(program") {
		t.Error("expected (program in output")
	}
	if !strings.Contains(output, "(mod main)") {
		t.Error("expected (mod main) in output")
	}
	if !strings.Contains(output, "(fn add") {
		t.Error("expected (fn add in output")
	}
	if !strings.Contains(output, "(entry") {
		t.Error("expected (entry in output")
	}
}

// ===== Error Recovery =====

func TestParseErrorRecovery(t *testing.T) {
	// Parser should continue after an error and report multiple issues
	_, p := parse(`mod main
fn foo( {
}
fn bar() = 42`)
	if !p.Diagnostics().HasErrors() {
		t.Error("expected parse errors")
	}
}

// ===== Full program =====

func TestParseFullProgram(t *testing.T) {
	source := `mod main

use std.fs

type Shape =
    | Circle(f64)
    | Rect { w: f64, h: f64 }
    | Point

fn area(s: Shape) -> f64 = match s {
    Circle(r) => 3.14 * r * r
    Rect(w, h) => w * h
    Point => 0.0
}

entry {
    s := Circle(5.0)
    a := area(s)
    println("Area: {a}")
}

test "circle area" {
    assert area(Circle(1.0)) == 3.14
}`
	prog := parseNoErrors(t, source)
	// Should have: TypeDecl, FnDecl, EntryBlock, TestBlock
	if len(prog.Decls) != 4 {
		t.Fatalf("expected 4 declarations, got %d", len(prog.Decls))
	}
}
