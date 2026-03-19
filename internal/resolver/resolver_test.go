package resolver

import (
	"testing"

	"github.com/aria-lang/aria/internal/lexer"
	"github.com/aria-lang/aria/internal/parser"
)

func parse(source string) *parser.Program {
	l := lexer.New("test.aria", source)
	tokens := l.Tokenize()
	p := parser.New(tokens)
	return p.Parse()
}

func resolveNoErrors(t *testing.T, source string) *Scope {
	t.Helper()
	prog := parse(source)
	r := New()
	scope := r.Resolve(prog)
	if r.Diagnostics().HasErrors() {
		t.Fatalf("unexpected resolve errors:\n%v", r.Diagnostics().Diagnostics)
	}
	return scope
}

func resolveExpectErrors(t *testing.T, source string) *Resolver {
	t.Helper()
	prog := parse(source)
	r := New()
	r.Resolve(prog)
	if !r.Diagnostics().HasErrors() {
		t.Fatal("expected resolve errors but got none")
	}
	return r
}

// ===== Scope and Built-ins =====

func TestUniverseScope(t *testing.T) {
	s := NewUniverseScope()

	// Built-in types
	for _, name := range []string{"i64", "str", "bool", "f64", "u8", "usize"} {
		if s.Lookup(name) == nil {
			t.Errorf("expected built-in type '%s'", name)
		}
	}

	// Built-in functions
	for _, name := range []string{"println", "print", "panic", "assert"} {
		if s.Lookup(name) == nil {
			t.Errorf("expected built-in function '%s'", name)
		}
	}
}

func TestScopeShadowing(t *testing.T) {
	parent := NewScope(ModuleScope, NewUniverseScope())
	parent.Define(&Symbol{Name: "x", Kind: SymVariable})

	child := NewScope(BlockScope, parent)
	child.Define(&Symbol{Name: "x", Kind: SymVariable})

	// Child should find its own x
	sym := child.LookupLocal("x")
	if sym == nil {
		t.Fatal("expected symbol in child scope")
	}

	// Parent's x still exists
	sym = parent.LookupLocal("x")
	if sym == nil {
		t.Fatal("expected symbol in parent scope")
	}
}

// ===== Top-Level Declarations =====

func TestResolveForwardReferences(t *testing.T) {
	// Functions should be able to reference each other regardless of order
	resolveNoErrors(t, `mod main
fn foo() -> i64 = bar()
fn bar() -> i64 = 42`)
}

func TestResolveFnParams(t *testing.T) {
	resolveNoErrors(t, `mod main
fn add(a: i64, b: i64) -> i64 = a + b`)
}

func TestResolveDuplicateTopLevel(t *testing.T) {
	r := resolveExpectErrors(t, `mod main
fn foo() = 1
fn foo() = 2`)
	found := false
	for _, d := range r.Diagnostics().Diagnostics {
		if d.Code == "E0702" {
			found = true
		}
	}
	if !found {
		t.Error("expected E0702 duplicate declaration error")
	}
}

func TestResolveStructDecl(t *testing.T) {
	resolveNoErrors(t, `mod main
struct Point {
    x: f64
    y: f64
}

fn make() -> Point = Point{x: 1.0, y: 2.0}`)
}

func TestResolveSumTypeVariants(t *testing.T) {
	resolveNoErrors(t, `mod main
type Shape =
    | Circle(f64)
    | Rect(f64, f64)
    | Point

fn test() = Circle(5.0)`)
}

func TestResolveEnum(t *testing.T) {
	resolveNoErrors(t, `mod main
enum Color { Red, Green, Blue }

fn test() = Red`)
}

func TestResolveConst(t *testing.T) {
	resolveNoErrors(t, `mod main
const MAX = 100
fn test() = MAX`)
}

func TestResolveTrait(t *testing.T) {
	resolveNoErrors(t, `mod main
struct Point { x: f64, y: f64 }

trait Display {
    fn display(self) -> str
}

impl Display for Point {
    fn display(self) -> str = "point"
}`)
}

// ===== Block-Level Resolution =====

func TestResolveVarDecl(t *testing.T) {
	resolveNoErrors(t, `mod main
entry {
    x := 42
    y := x + 1
    println(y)
}`)
}

func TestResolveUndefinedName(t *testing.T) {
	r := resolveExpectErrors(t, `mod main
entry {
    println(undefined_var)
}`)
	found := false
	for _, d := range r.Diagnostics().Diagnostics {
		if d.Code == "E0701" {
			found = true
		}
	}
	if !found {
		t.Error("expected E0701 unresolved name error")
	}
}

func TestResolveBlockScoping(t *testing.T) {
	// Inner block variable should not leak to outer
	resolveExpectErrors(t, `mod main
entry {
    if true {
        x := 42
    }
    println(x)
}`)
}

func TestResolveShadowingInBlock(t *testing.T) {
	// Shadowing should be allowed
	resolveNoErrors(t, `mod main
entry {
    x := 42
    x := "hello"
    println(x)
}`)
}

func TestResolveForLoop(t *testing.T) {
	resolveNoErrors(t, `mod main
entry {
    items := [1, 2, 3]
    for x in items {
        println(x)
    }
}`)
}

func TestResolveForLoopVarNotLeaking(t *testing.T) {
	resolveExpectErrors(t, `mod main
entry {
    items := [1, 2, 3]
    for x in items {
        println(x)
    }
    println(x)
}`)
}

func TestResolveMatchPatternBindings(t *testing.T) {
	resolveNoErrors(t, `mod main
type Shape =
    | Circle(f64)
    | Rect(f64, f64)
    | Point

fn area(s: Shape) -> f64 = match s {
    Circle(r) => r * r
    Rect(w, h) => w * h
    Point => 0.0
}`)
}

func TestResolveClosure(t *testing.T) {
	resolveNoErrors(t, `mod main
entry {
    x := 42
    f := fn(y: i64) => x + y
    println(f(1))
}`)
}

func TestResolveWhileLoop(t *testing.T) {
	resolveNoErrors(t, `mod main
entry {
    mut x := 10
    while x > 0 {
        x = x - 1
    }
}`)
}

func TestResolveListComprehension(t *testing.T) {
	resolveNoErrors(t, `mod main
entry {
    items := [1, 2, 3]
    doubled := [x * 2 for x in items]
    println(doubled)
}`)
}

// ===== Imports =====

func TestResolveSimpleImport(t *testing.T) {
	resolveNoErrors(t, `mod main
use std.fs

entry {
    println(fs)
}`)
}

func TestResolveAliasedImport(t *testing.T) {
	resolveNoErrors(t, `mod main
use std.json as j

entry {
    println(j)
}`)
}

// ===== Entry and Test =====

func TestResolveEntryBlock(t *testing.T) {
	resolveNoErrors(t, `mod main
entry {
    println("hello")
}`)
}

func TestResolveTestBlock(t *testing.T) {
	resolveNoErrors(t, `mod main
fn add(a: i64, b: i64) -> i64 = a + b

test "addition" {
    assert add(2, 3) == 5
}`)
}

// ===== Full Program =====

func TestResolveFullProgram(t *testing.T) {
	resolveNoErrors(t, `mod main

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
    println(a)
}

test "circle area" {
    assert area(Circle(1.0)) == 3.14
}`)
}

func TestResolveImplUnknownType(t *testing.T) {
	r := resolveExpectErrors(t, `mod main
trait Display {
    fn display(self) -> str
}
impl Display for UnknownType {
    fn display(self) -> str = "?"
}`)
	found := false
	for _, d := range r.Diagnostics().Diagnostics {
		if d.Code == "E0701" {
			found = true
		}
	}
	if !found {
		t.Error("expected E0701 for unresolved type in impl")
	}
}

func TestResolveImplUnknownTrait(t *testing.T) {
	r := resolveExpectErrors(t, `mod main
struct Point { x: f64, y: f64 }
impl UnknownTrait for Point {
    fn foo(self) = 1
}`)
	found := false
	for _, d := range r.Diagnostics().Diagnostics {
		if d.Code == "E0701" {
			found = true
		}
	}
	if !found {
		t.Error("expected E0701 for unresolved trait in impl")
	}
}
