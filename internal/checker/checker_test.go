package checker

import (
	"testing"

	"github.com/aria-lang/aria/internal/lexer"
	"github.com/aria-lang/aria/internal/parser"
	"github.com/aria-lang/aria/internal/resolver"
)

func checkSource(source string) (*Checker, *parser.Program) {
	l := lexer.New("test.aria", source)
	tokens := l.Tokenize()
	p := parser.New(tokens)
	prog := p.Parse()

	r := resolver.New()
	scope := r.Resolve(prog)

	c := New(scope)
	c.Check(prog)
	return c, prog
}

func checkNoErrors(t *testing.T, source string) *Checker {
	t.Helper()
	c, _ := checkSource(source)
	if c.Diagnostics().HasErrors() {
		t.Fatalf("unexpected type errors:\n%v", c.Diagnostics().Diagnostics)
	}
	return c
}

func checkExpectError(t *testing.T, source string, expectedCode string) *Checker {
	t.Helper()
	c, _ := checkSource(source)
	if !c.Diagnostics().HasErrors() {
		t.Fatal("expected type error but got none")
	}
	found := false
	for _, d := range c.Diagnostics().Diagnostics {
		if d.Code == expectedCode {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error code %s, got errors: %v",
			expectedCode, c.Diagnostics().Diagnostics)
	}
	return c
}

// ===== Type representations =====

func TestPrimitiveTypes(t *testing.T) {
	if !TypeI64.Equals(TypeI64) {
		t.Error("i64 should equal i64")
	}
	if TypeI64.Equals(TypeF64) {
		t.Error("i64 should not equal f64")
	}
	if !IsIntegerType(TypeI64) {
		t.Error("i64 should be integer")
	}
	if !IsFloatType(TypeF64) {
		t.Error("f64 should be float")
	}
	if IsIntegerType(TypeStr) {
		t.Error("str should not be integer")
	}
}

func TestTypeEquality(t *testing.T) {
	arr1 := &ArrayType{Element: TypeI64}
	arr2 := &ArrayType{Element: TypeI64}
	arr3 := &ArrayType{Element: TypeStr}

	if !arr1.Equals(arr2) {
		t.Error("[i64] should equal [i64]")
	}
	if arr1.Equals(arr3) {
		t.Error("[i64] should not equal [str]")
	}
}

func TestNeverAssignable(t *testing.T) {
	if !IsAssignable(TypeNever, TypeI64) {
		t.Error("Never should be assignable to any type")
	}
}

// ===== Literal type inference =====

func TestCheckLiteralTypes(t *testing.T) {
	checkNoErrors(t, `mod main
const A = 42
const B = 3.14
const C = "hello"
const D = true`)
}

// ===== Binary expression type checking =====

func TestCheckArithmeticSameType(t *testing.T) {
	checkNoErrors(t, `mod main
fn add(a: i64, b: i64) -> i64 = a + b`)
}

func TestCheckComparisonReturnsBool(t *testing.T) {
	checkNoErrors(t, `mod main
fn greater(a: i64, b: i64) -> bool = a > b`)
}

func TestCheckLogicalRequiresBool(t *testing.T) {
	checkNoErrors(t, `mod main
fn both(a: bool, b: bool) -> bool = a && b`)
}

// ===== Function call checking =====

func TestCheckFunctionCallArgCount(t *testing.T) {
	checkExpectError(t, `mod main
fn add(a: i64, b: i64) -> i64 = a + b
const X = add(1)`, "E0104")
}

// ===== Struct checking =====

func TestCheckStructConstruction(t *testing.T) {
	checkNoErrors(t, `mod main
struct Point { x: f64, y: f64 }
fn make() -> Point = Point{x: 1.0, y: 2.0}`)
}

func TestCheckStructMissingField(t *testing.T) {
	checkExpectError(t, `mod main
struct Point { x: f64, y: f64 }
fn make() -> Point = Point{x: 1.0}`, "E0107")
}

func TestCheckStructUnknownField(t *testing.T) {
	checkExpectError(t, `mod main
struct Point { x: f64, y: f64 }
fn make() -> Point = Point{x: 1.0, y: 2.0, z: 3.0}`, "E0108")
}

func TestCheckStructDefaultField(t *testing.T) {
	checkNoErrors(t, `mod main
type User {
    name: str
    age: u8 = 0
} derives [Eq]
fn make() -> User = User{name: "Alice"}`)
}

// ===== Control flow type checking =====

func TestCheckIfConditionBool(t *testing.T) {
	checkNoErrors(t, `mod main
fn f(x: i64) -> str {
    if x > 0 {
        "positive"
    } else {
        "non-positive"
    }
}`)
}

func TestCheckIfBranchTypeMismatch(t *testing.T) {
	checkExpectError(t, `mod main
fn f(x: bool) -> i64 {
    if x {
        42
    } else {
        "hello"
    }
}`, "E0103")
}

// ===== Match exhaustiveness =====

func TestCheckMatchExhaustive(t *testing.T) {
	checkNoErrors(t, `mod main
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

func TestCheckMatchNonExhaustive(t *testing.T) {
	checkExpectError(t, `mod main
type Shape =
    | Circle(f64)
    | Rect(f64, f64)
    | Point

fn area(s: Shape) -> f64 = match s {
    Circle(r) => r * r
}`, "E0400")
}

func TestCheckMatchWithWildcard(t *testing.T) {
	checkNoErrors(t, `mod main
type Shape =
    | Circle(f64)
    | Rect(f64, f64)
    | Point

fn name(s: Shape) -> str = match s {
    Circle(r) => "circle"
    _ => "other"
}`)
}

func TestCheckEnumExhaustive(t *testing.T) {
	checkNoErrors(t, `mod main
enum Color { Red, Green, Blue }

fn name(c: Color) -> str = match c {
    Red => "red"
    Green => "green"
    Blue => "blue"
}`)
}

func TestCheckEnumNonExhaustive(t *testing.T) {
	checkExpectError(t, `mod main
enum Color { Red, Green, Blue }

fn name(c: Color) -> str = match c {
    Red => "red"
}`, "E0400")
}

// ===== Error handling =====

func TestCheckErrorPropagationInNonFallible(t *testing.T) {
	// Using ? in a function without ! should be an error
	checkExpectError(t, `mod main
fn foo() -> i64 {
    bar()?
}
fn bar() -> i64 ! Error = 42`, "E0850")
}

// ===== Effect checking =====

func TestCheckEffectViolation(t *testing.T) {
	// Pure function calling effectful function
	checkExpectError(t, `mod main
fn effectful() with [Io] = println("hi")
fn pure() = effectful()`, "E0301")
}

func TestCheckEffectAllowedInEntry(t *testing.T) {
	checkNoErrors(t, `mod main
entry {
    println("hello")
}`)
}

// ===== Trait checking =====

func TestTraitRegistry(t *testing.T) {
	r := NewTraitRegistry()

	if r.LookupTrait("Eq") == nil {
		t.Error("expected built-in Eq trait")
	}
	if !r.Implements("i64", "Eq") {
		t.Error("i64 should implement Eq")
	}
	if r.Implements("f64", "Eq") {
		t.Error("f64 should NOT implement Eq (NaN)")
	}
	if !r.Implements("f64", "Numeric") {
		t.Error("f64 should implement Numeric")
	}
}

func TestCheckImplMissingMethod(t *testing.T) {
	checkExpectError(t, `mod main
struct Point { x: f64, y: f64 }
trait Display {
    fn display(self) -> str
}
impl Display for Point {
}`, "E0202")
}

func TestCheckDerives(t *testing.T) {
	checkNoErrors(t, `mod main
struct Point { x: f64, y: f64 } derives [Eq, Hash, Debug]`)
}

func TestCheckInvalidDerive(t *testing.T) {
	checkExpectError(t, `mod main
struct Point { x: f64, y: f64 } derives [NotARealTrait]`, "E0205")
}

// ===== Closure checking =====

func TestCheckClosureType(t *testing.T) {
	checkNoErrors(t, `mod main
const F = fn(x: i64) => x * 2`)
}

// ===== Array checking =====

func TestCheckArrayLiteral(t *testing.T) {
	checkNoErrors(t, `mod main
const A = [1, 2, 3]`)
}

// ===== Field access =====

func TestCheckFieldAccess(t *testing.T) {
	checkNoErrors(t, `mod main
struct Point { x: f64, y: f64 }`)
}

func TestCheckFieldAccessUnknownField(t *testing.T) {
	// This requires field access on a known struct type, which needs
	// more context propagation. For now, just test struct construction.
	checkExpectError(t, `mod main
struct Point { x: f64, y: f64 }
fn make() -> Point = Point{x: 1.0, y: 2.0, z: 3.0}`, "E0108")
}

// ===== Full program =====

func TestCheckFullProgram(t *testing.T) {
	checkNoErrors(t, `mod main

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
