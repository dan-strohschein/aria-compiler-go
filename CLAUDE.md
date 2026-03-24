# Aria Bootstrap Compiler — AI Assistant Guide

## What This Is

This is the **bootstrap compiler for the Aria programming language**, written in Go. Its sole purpose is to compile enough Aria to build the self-hosting compiler (which will be written in Aria itself). This is throwaway code — optimize for velocity, not elegance.

## The Spec Is the Authority

The language specification lives in a sibling repository:

```
../aria-docs/
```

**Every design question is answered there.** The spec repo contains 33 formal specification files, a high-level design document, an AI guide, 16 example programs, and a design decisions document. When in doubt, read the spec.

### Key spec files for compiler implementation

| File | What it tells you |
|---|---|
| `../aria-docs/high-level-design.md` | Language overview — start here |
| `../aria-docs/CLAUDE.md` | Design principles, conventions, anti-patterns |
| `../aria-docs/spec/formal-grammar.md` | **EBNF grammar — the parser's blueprint** |
| `../aria-docs/spec/operator-precedence.md` | Precedence table — drives the expression parser |
| `../aria-docs/spec/scoping-rules.md` | Name resolution algorithm |
| `../aria-docs/spec/trait-system.md` | Traits, bounds, derives, method resolution |
| `../aria-docs/spec/generics-type-parameters.md` | Generics, monomorphization |
| `../aria-docs/spec/type-conversions.md` | Convert/TryConvert, three conversion mechanisms |
| `../aria-docs/spec/equality-comparison.md` | Eq, Ord, Hash — operator trait mapping |
| `../aria-docs/spec/newtype-aliases.md` | Newtypes vs aliases — parser disambiguation |
| `../aria-docs/spec/pattern-matching.md` | Match expressions, exhaustiveness checking |
| `../aria-docs/spec/error-handling.md` | Result types, ?, catch, error traces |
| `../aria-docs/spec/effect-system.md` | Effect declarations, purity verification |
| `../aria-docs/spec/memory-management.md` | GC, @stack, @arena, @inline, Drop |
| `../aria-docs/spec/closures-capture-semantics.md` | Closures, capture, method references |
| `../aria-docs/spec/concurrency-design.md` | Tasks, scope, channels, select |
| `../aria-docs/spec/string-handling.md` | str representation, SSO, indexing semantics |
| `../aria-docs/spec/design-decisions-v01.md` | Resolved design questions (operator traits, `as`, recursion, mutability, closures, `with`, bootstrap) |
| `../aria-docs/spec/garbage-collector.md` | GC algorithm (bootstrap uses Go's GC) |
| `../aria-docs/spec/task-scheduler.md` | Scheduler design (bootstrap uses goroutines) |
| `../aria-docs/spec/compiler-architecture.md` | Compilation pipeline overview |
| `../aria-docs/spec/compiler-diagnostics.md` | Error message format, error codes, JSON output |
| `../aria-docs/spec/testing-framework.md` | Test blocks, assertions, mocking |
| `../aria-docs/spec/annotations.md` | Closed annotation set, @deprecated, @cold |
| `../aria-docs/spec/const-evaluation.md` | Compile-time constants |

### The 5 Design Pillars (Non-Negotiable)

These govern the language. The compiler must enforce them:

1. **Every token carries meaning** — no boilerplate, no ceremony
2. **The type system is the AI's pair programmer** — sum types, exhaustive matching, effects
3. **Compilation is instantaneous** — unambiguous grammar, minimal lookahead
4. **Performance is opt-in granular** — GC default, manual per-block
5. **No implicit behavior ever** — no implicit conversions, no hidden exceptions, no null

---

## Project Structure

This is a Go project. The target architecture:

```
aria-compiler-go/
├── CLAUDE.md                  # This file
├── README.md                  # Project overview
├── go.mod                     # Go module definition
├── go.sum                     # Go dependency checksums
├── cmd/
│   └── aria/
│       └── main.go            # CLI entry point (aria build, aria run, aria check, aria test)
├── internal/
│   ├── lexer/                 # Stage 1: Source → Token stream
│   │   ├── lexer.go           # Lexer implementation
│   │   ├── token.go           # Token types and definitions
│   │   └── lexer_test.go
│   ├── parser/                # Stage 2: Tokens → AST
│   │   ├── parser.go          # Recursive descent parser
│   │   ├── ast.go             # AST node types
│   │   ├── precedence.go      # Operator precedence climbing
│   │   └── parser_test.go
│   ├── resolver/              # Stage 3: Name resolution + imports
│   │   ├── resolver.go        # Name resolution, scope building
│   │   ├── scope.go           # Scope hierarchy
│   │   └── resolver_test.go
│   ├── checker/               # Stage 4: Type checking + inference
│   │   ├── checker.go         # Type checker main loop
│   │   ├── types.go           # Type representations
│   │   ├── traits.go          # Trait resolution, bounds checking
│   │   ├── generics.go        # Generic instantiation
│   │   ├── effects.go         # Effect checking
│   │   ├── patterns.go        # Exhaustiveness checking
│   │   └── checker_test.go
│   ├── ir/                    # Stage 5: AST → IR (SSA form)
│   │   ├── ir.go              # IR node types
│   │   ├── builder.go         # IR construction from typed AST
│   │   └── ir_test.go
│   ├── codegen/               # Stage 6-7: IR → machine code (Tier 1 only)
│   │   ├── codegen.go         # Code generation
│   │   ├── target.go          # Target platform abstraction
│   │   └── codegen_test.go
│   ├── runtime/               # Aria runtime (linked into every binary)
│   │   ├── gc.go              # GC (uses Go's GC for bootstrap)
│   │   ├── scheduler.go       # Task scheduler (uses goroutines for bootstrap)
│   │   ├── channels.go        # Channel implementation
│   │   └── runtime_test.go
│   └── diagnostic/            # Error reporting
│       ├── diagnostic.go      # Diagnostic types (error, warning, suggestion)
│       ├── codes.go           # Error code registry
│       ├── render.go          # Human-readable rendering
│       ├── json.go            # JSON output (--format=json)
│       └── diagnostic_test.go
└── testdata/                  # Aria source files for testing
    ├── lexer/                 # Input files for lexer tests
    ├── parser/                # Input files for parser tests
    ├── checker/               # Input files for type checker tests
    └── programs/              # Full programs for end-to-end tests
```

---

## Build & Test

**IMPORTANT: Always run ALL tests with a 6 GB memory limit to prevent system crashes.**

```bash
# Build the compiler
go build -o aria ./cmd/aria

# Run tests (ALWAYS use -memprofile or GOMEMLIMIT=6GiB)
GOMEMLIMIT=6GiB go test ./...

# Run a specific package's tests
GOMEMLIMIT=6GiB go test ./internal/lexer/

# Run with verbose output
GOMEMLIMIT=6GiB go test -v ./internal/parser/

# Run the compiler on an Aria file
./aria check examples/01-hello-world.aria
./aria build examples/01-hello-world.aria
./aria run examples/01-hello-world.aria
```

---

## Implementation Order

Build the compiler in this order. Each stage depends on the previous one.

### Phase 1: Lexer + Parser (can produce AST from source)

1. **Lexer** — tokenize `.aria` files per `formal-grammar.md` §2
   - Keywords, identifiers, literals (int, float, string, bool, duration, size)
   - Operators and punctuation
   - String interpolation (emit interpolation token sequences)
   - Newline-as-statement-terminator rules (§5 of formal-grammar)
   - Line comments (`//`)

2. **Parser** — recursive descent, per `formal-grammar.md` §3-4
   - Program structure: `mod`, `use`, top-level declarations
   - Expressions: precedence climbing per `operator-precedence.md`
   - Statements: variable bindings, assignments, control flow
   - Type declarations: structs, sum types, newtypes, enums, aliases
   - Function declarations: parameters, return types, error clauses, effect clauses
   - Traits and impl blocks
   - Match expressions with patterns
   - Closures, spawn, scope, select, with, defer

### Phase 2: Name Resolution + Type Checking (can type-check an AST)

3. **Name resolution** — per `scoping-rules.md`
   - Build scope hierarchy (universe → package → module → import → function → block)
   - Resolve identifiers to declarations
   - Handle shadowing (legal in Aria)
   - Resolve imports (`use` declarations)

4. **Type checking** — the largest stage
   - Type inference (bidirectional: arguments inward, return context outward)
   - Trait bounds verification
   - Generic instantiation (monomorphization)
   - Exhaustive match checking
   - Effect checking (pure functions can't call effectful functions)
   - Operator desugaring to trait method calls (per `design-decisions-v01.md` Decision 2)
   - Error type checking (`!` signatures, `?` propagation compatibility)
   - Mutability checking (binding-level, per Decision 5)
   - Send/Share checking for `spawn` boundaries

### Phase 3: IR + Codegen (can compile to executable)

5. **IR generation** — typed AST → SSA-form IR
6. **Code generation** — Tier 1 only (fast backend, no LLVM)
7. **Linker** — produce standalone executable with runtime

### Phase 4: Runtime (can execute Aria programs)

8. **GC** — use Go's GC for bootstrap (per `garbage-collector.md` §13)
9. **Task scheduler** — map Aria tasks to goroutines (per `task-scheduler.md` §15)
10. **Channels** — implement Aria channels on top of Go channels
11. **Basic stdlib** — `io`, `str` operations, collections — enough to self-host

---

## Coding Conventions

### Go style

- Follow standard Go conventions (`gofmt`, `golint`)
- Use `internal/` for all compiler packages (not importable by external code)
- Error handling: return `error` from functions that can fail; don't panic except for compiler bugs
- Tests: table-driven tests with testdata files where appropriate

### AST design

- AST nodes should be concrete types (not interfaces) for performance
- Use tagged unions (type field + switch) for node kinds — Go doesn't have sum types, so simulate them
- Every AST node carries source position (file, line, column) for diagnostics
- AST should be immutable after construction — no in-place mutation during checking

### Diagnostic design

- Every diagnostic has an error code (E0001, W0001, etc.) per `compiler-diagnostics.md`
- Support both human-readable and JSON output from the start
- Include source line text, labeled spans, and fix suggestions in every diagnostic
- The diagnostic system is the compiler's primary output — invest in it early

### Testing strategy

- Lexer tests: input string → expected token sequence
- Parser tests: input string → expected AST (serialized as S-expressions or JSON)
- Type checker tests: input file → expected diagnostics (or success)
- End-to-end tests: input `.aria` file → expected output, expected exit code, or expected compile errors
- Use the example programs from `../aria-docs/examples/` as end-to-end test cases

---

## What the Bootstrap Compiler Does NOT Need

Per `design-decisions-v01.md` Decision 9:

- ❌ LLVM backend (Tier 2) — optimization comes with self-hosting
- ❌ Full concurrent GC — use Go's GC
- ❌ Full task scheduler — use goroutines
- ❌ Full stdlib — only what the self-hosting compiler needs
- ❌ Full diagnostics — basic error messages first, refine later
- ❌ Property-based testing in the test framework — basic `test` + `assert` is sufficient
- ❌ Benchmarking blocks — not needed for bootstrap
- ❌ `aria fix` command — nice-to-have, not essential

Focus on correctness first. Get a correct Aria-to-executable pipeline working, then optimize.

---

## Key Syntax Quick Reference (for test case writing)

```aria
mod main

use std.fs

// Variables
x := 42
mut y := 0
name: str = "Aria"

// Functions
fn add(a: i64, b: i64) -> i64 = a + b
fn greet(name: str) with [Io] { println("Hello, {name}") }
fn readConfig(path: str) -> Config ! IoError with [Io, Fs] {
    content := fs.read(path)?
    parseConfig(content)?
}

// Types
struct Point { x: f64, y: f64 }
type Shape = Circle(f64) | Rect(f64, f64) | Point
type UserId = i64                        // newtype
alias Headers = Map[str, str]            // alias

// Traits
trait Display { fn display(self) -> str }
impl Display for Point {
    fn display(self) -> str = "({self.x}, {self.y})"
}

// Pattern matching
fn area(s: Shape) -> f64 = match s {
    Circle(r) => 3.14159 * r * r
    Rect(w, h) => w * h
    Point => 0.0
}

// Concurrency
scope {
    a := spawn fetchUsers()
    b := spawn fetchOrders()
    (a.await()?, b.await()?)
}

// Entry point
entry {
    config := readConfig("app.json") catch |_| { yield Config{} }
    println("Starting on {config.host}:{config.port}")
}

// Tests
test "addition" {
    assert add(2, 3) == 5
}
```

---

## Common Pitfalls

- **`[T]` is both generics and array type** — parser disambiguates by context (see `formal-grammar.md` §4.1 and `generics-type-parameters.md` §10)
- **`!` is both logical NOT (prefix) and assert-success (postfix)** — position determines meaning
- **`|` is both bitwise OR and sum-type variant separator** — context determines meaning (type declaration vs expression)
- **No semicolons** — newline termination rules in `formal-grammar.md` §5 are critical for the lexer
- **`struct` is sugar for `type`** — both produce identical AST nodes (see `design-decisions-v01.md` Decision 1)
- **`as` is ONLY for import aliases** — NOT for type casts (Decision 3)
- **Recursive types are auto-boxed** — compiler must detect and insert indirection (Decision 4)
- **Mutability is on bindings, not fields** — `mut x` makes everything mutable (Decision 5)
- **Closures are GC-boxed** — one type `fn(A) -> B`, uniform representation (Decision 7)
- **No integer literal suffixes** — `42` is always `i64`, use type annotation for others (Decision 10)
