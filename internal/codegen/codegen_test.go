package codegen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aria-lang/aria/internal/lexer"
	"github.com/aria-lang/aria/internal/parser"
)

func generate(source string) string {
	l := lexer.New("test.aria", source)
	tokens := l.Tokenize()
	p := parser.New(tokens)
	prog := p.Parse()
	gen := New()
	return gen.Generate(prog)
}

func TestGenHelloWorld(t *testing.T) {
	src := generate(`mod main
entry {
    println("Hello, Aria!")
}`)
	if !strings.Contains(src, `fmt.Println("Hello, Aria!")`) {
		t.Errorf("expected fmt.Println, got:\n%s", src)
	}
	if !strings.Contains(src, "func main()") {
		t.Error("expected func main()")
	}
}

func TestGenFunction(t *testing.T) {
	src := generate(`mod main
fn add(a: i64, b: i64) -> i64 = a + b`)
	if !strings.Contains(src, "func add(a int64, b int64) int64") {
		t.Errorf("expected Go function signature, got:\n%s", src)
	}
}

func TestGenStruct(t *testing.T) {
	src := generate(`mod main
struct Point { x: f64, y: f64 }`)
	if !strings.Contains(src, "type Point struct") {
		t.Errorf("expected struct, got:\n%s", src)
	}
}

func TestGenSumType(t *testing.T) {
	src := generate(`mod main
type Shape =
    | Circle(f64)
    | Rect(f64, f64)
    | Point`)
	if !strings.Contains(src, "type Shape interface") {
		t.Errorf("expected interface, got:\n%s", src)
	}
	if !strings.Contains(src, "type ShapeCircle struct") {
		t.Errorf("expected variant struct, got:\n%s", src)
	}
}

func TestGenEnum(t *testing.T) {
	src := generate(`mod main
enum Color { Red, Green, Blue }`)
	if !strings.Contains(src, "type Color int") {
		t.Errorf("expected enum type, got:\n%s", src)
	}
	if !strings.Contains(src, "Red Color = iota") {
		t.Errorf("expected iota, got:\n%s", src)
	}
}

func TestGenMatchExpr(t *testing.T) {
	src := generate(`mod main
type Shape =
    | Circle(f64)
    | Rect(f64, f64)
    | Point
fn area(s: Shape) -> f64 = match s {
    Circle(r) => 3.14 * r * r
    Rect(w, h) => w * h
    Point => 0.0
}`)
	fmt.Println("=== Generated match ===")
	fmt.Println(src)
	if !strings.Contains(src, "switch") {
		t.Error("expected switch statement in match")
	}
}

func TestGenInterpolatedString(t *testing.T) {
	src := generate(`mod main
entry {
    name := "World"
    println("Hello, {name}!")
}`)
	if !strings.Contains(src, "fmt.Sprintf") {
		t.Errorf("expected fmt.Sprintf, got:\n%s", src)
	}
}

func TestGenConst(t *testing.T) {
	src := generate(`mod main
const MAX = 100`)
	if !strings.Contains(src, "var MAX = int64(100)") {
		t.Errorf("expected var declaration, got:\n%s", src)
	}
}

func TestGenForLoop(t *testing.T) {
	src := generate(`mod main
entry {
    items := [1, 2, 3]
    for x in items {
        println(x)
    }
}`)
	if !strings.Contains(src, "for _, x := range") {
		t.Errorf("expected range loop, got:\n%s", src)
	}
}

func TestGenClosure(t *testing.T) {
	src := generate(`mod main
const F = fn(x: i64) => x * 2`)
	if !strings.Contains(src, "func(x int64)") {
		t.Errorf("expected closure, got:\n%s", src)
	}
}

func TestGenNestedIfElse(t *testing.T) {
	src := generate(`mod main
entry {
    c := "x"
    if c == "+" {
        println("plus")
    } else if c == "-" {
        next := 5
        if next < 10 {
            println("has next")
        } else {
            println("no next")
        }
    } else {
        println("other")
    }
}`)
	fmt.Println("=== Nested if/else ===")
	fmt.Println(src)
}

func TestGenFnTypeParam(t *testing.T) {
	src := generate(`mod main
fn apply(x: i64, f: fn(i64) -> i64) -> i64 = f(x)
entry {
    println(apply(21, fn(x: i64) => x * 2))
}`)
	fmt.Println("=== Generated apply ===")
	fmt.Println(src)
	if !strings.Contains(src, "func apply") {
		t.Error("expected apply function")
	}
}
