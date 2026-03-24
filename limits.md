# Bootstrap Compiler — Known Limitations

These are bugs and limitations discovered while building the self-hosting Aria compiler (`../aria/`) using this bootstrap compiler. They were found by writing ~3,300 lines of Aria across diagnostics, lexer, and parser modules and running `aria check`, `aria build`, and `aria test` extensively.

**Key insight:** `aria check` (lexer → parser → resolver → checker) works correctly for almost all patterns. The bugs are concentrated in **Go codegen** (`internal/codegen/codegen.go`).

---

## Bug 1: Struct variant pattern destructuring generates broken Go

**Severity:** High — blocks natural Aria pattern matching style

**Reproduction:**
```aria
mod main

type Expr =
    | IntLit { value: i64 }
    | BoolLit { value: bool }

fn eval(e: Expr) -> i64 = match e {
    IntLit{value: v} => v
    BoolLit{value: v} => if v { 1 } else { 0 }
}

entry { println("{eval(IntLit{value: 42})}") }
```

`aria check` passes. `aria build` fails:
```
undefined: v
multiple defaults (first at ...)
```

**Root cause:** `genMatchArm()` in `internal/codegen/codegen.go` (around line 956) only handles `VariantPattern` (tuple variants with positional args) and `BindingPattern` (unit variants or catch-all). It does NOT handle `StructPattern` — struct variant patterns like `IntLit{value: v}` fall through to the `BindingPattern` case and generate `default:` in the Go type switch, causing "multiple defaults" errors. The field bindings (`v`) are never emitted.

**Fix location:** `internal/codegen/codegen.go`, function `genMatchArm()`, around line 956. Add a `case *parser.StructPattern:` handler that:
1. Maps the variant name to the Go type (e.g., `ExprIntLit`)
2. Emits `case ExprIntLit:` in the type switch
3. Binds each field: `v := tmp.Value` (mapping Aria field names to Go exported field names)

---

## Bug 2: Closures returning non-bool types generate wrong Go signatures

**Severity:** High — blocks functional programming patterns

**Reproduction:**
```aria
mod main

fn apply(x: i64, f: fn(i64) -> i64) -> i64 = f(x)

entry {
    result := apply(5, fn(x: i64) => x * x)
    println("{result}")
}
```

`aria check` passes. `aria build` fails:
```
cannot use func(x int64) {...} as func(int64) int64 value
too many return values
```

**Root cause:** `genClosureExpr()` in `internal/codegen/codegen.go` (around line 1053) generates the Go closure's return type incorrectly — it omits the return type, generating `func(x int64) { return x * x }` instead of `func(x int64) int64 { return x * x }`. Closures returning `bool` work because Go can infer the type in some bool contexts.

**Fix location:** `internal/codegen/codegen.go`, function `genClosureExpr()`. The closure's return type needs to be looked up from the checker's type information and emitted in the Go func signature.

---

## Bug 3: `\n` in interpolated strings embeds literal newline in Go format string

**Severity:** Medium — easy workaround but surprising

**Reproduction:**
```aria
mod main
entry {
    x := 42
    s := "{x}\n"
    println(s)
}
```

`aria build` fails with `newline in string` Go compilation error.

**Root cause:** `genInterpolatedString()` in `internal/codegen/codegen.go` (around line 795) joins string literal parts directly into a `fmt.Sprintf` format string. When a string part contains `\n` (which the lexer has already converted to a real newline character), it gets embedded as a literal newline in the Go source `fmt.Sprintf("...\n...")`, breaking the Go string literal.

**Fix location:** `internal/codegen/codegen.go`, function `genInterpolatedString()`, around line 804. The `escaped` variable (which only escapes `%`) also needs to escape newlines, tabs, backslashes, and quotes for Go string literal safety. Replace:
```go
escaped := strings.ReplaceAll(sl.Value, "%", "%%")
```
with:
```go
escaped := strings.ReplaceAll(sl.Value, "%", "%%")
escaped = strings.ReplaceAll(escaped, "\\", "\\\\")
escaped = strings.ReplaceAll(escaped, "\n", "\\n")
escaped = strings.ReplaceAll(escaped, "\t", "\\t")
escaped = strings.ReplaceAll(escaped, "\r", "\\r")
escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
```

---

## Bug 4: Array literals containing function calls generate `[]interface{}` instead of typed slices

**Severity:** Medium — forces verbose struct literals

**Reproduction:**
```aria
mod main

struct Token { kind: i64, text: str }

fn make_token() -> Token = Token{kind: 0, text: ""}

entry {
    tokens := [make_token()]  // Fails
    // tokens := [Token{kind: 0, text: ""}]  // Works
}
```

`aria build` fails: `cannot use []interface{}{...} as []Token value`

**Root cause:** The codegen for array literals (`genArrayExpr()` or similar) doesn't propagate the expected element type when the array contains function-call expressions. It falls back to `[]interface{}`. When the array contains struct literals directly, Go can infer the type.

**Fix location:** `internal/codegen/codegen.go`, array literal generation. The generated Go code should use a typed slice literal `[]Token{...}` rather than `[]interface{}{...}` when the element type is known from the checker.

---

## Bug 5: Sum type variant names conflict with struct declarations

**Severity:** Medium — blocks natural AST representation patterns

**Reproduction:**
```aria
mod main

type Expr =
    | ExprInt
    | ExprBinary

struct ExprInt { value: i64 }
struct ExprBinary { op: str, left: Expr, right: Expr }
```

`aria check` fails: `duplicate declaration of 'ExprInt'`

**Root cause:** `internal/resolver/resolver.go` registers sum type variant names AND struct names in the same scope. When a variant name matches a struct name, it's flagged as a duplicate even though this is the intended pattern (variant + associated data struct).

**Fix location:** `internal/resolver/resolver.go`. Either: (a) allow struct declarations that match sum type variant names (treat them as the variant's data definition), or (b) use a separate namespace for variant names.

---

## Bug 6: Recursive sum types with separate struct variants hang the compiler

**Severity:** High — blocks recursive AST types

**Reproduction:**
```aria
mod main

type Expr =
    | IntLit
    | Binary

struct IntLit { value: i64 }
struct Binary { op: str, left: Expr, right: Expr }

entry { println("ok") }
```

`aria check` hangs indefinitely (killed after 30+ seconds).

**Root cause:** Likely in `internal/checker/checker.go` — the type checker enters infinite recursion when resolving the recursive `Expr` type through the `Binary` struct's `left: Expr` and `right: Expr` fields. The auto-boxing detection for recursive types may not handle the indirection through separate struct declarations.

**Fix location:** `internal/checker/checker.go` or `internal/checker/types.go`. The recursive type detection needs to handle the case where recursion goes through a struct that's associated with a sum type variant, not just direct self-reference.

---

## Bug 7: Pipeline operator `|>` type inference is broken

**Severity:** Low — easy workaround

**Reproduction:**
```aria
mod main
fn double(x: i64) -> i64 = x * 2
entry {
    result := 3 |> double
    assert result == 6  // Type error: cannot compare fn(i64) -> i64 with i64
}
```

**Root cause:** The checker infers the type of `3 |> double` as `fn(i64) -> i64` (the function type) rather than `i64` (the result of calling `double(3)`).

**Fix location:** `internal/checker/checker.go`, pipeline expression type checking. The result type should be the return type of the right-hand function applied to the left-hand value.

---

## Bug 8: `use X` conflicts when module X is in the same compilation unit

**Severity:** Low — documentation/design issue

**Reproduction:**
```
# File: src/diagnostic/diagnostic.aria
mod diagnostic
fn new_bag() -> DiagnosticBag { ... }

# File: src/diagnostic/render.aria
mod render
use diagnostic   // <-- ERROR: duplicate declaration of 'diagnostic'
```

When compiled together: `aria check src/diagnostic/`

**Root cause:** `internal/resolver/resolver.go` — when compiling multiple files together, all top-level declarations from all files are merged into a shared scope. A `use diagnostic` declaration tries to register "diagnostic" as an import, but "diagnostic" is already registered as a module name from `diagnostic.aria`.

**Fix location:** `internal/resolver/resolver.go` or `cmd/aria/main.go`. Either: (a) skip `use` declarations that reference modules already in the compilation unit, or (b) properly scope modules so that `use` imports from another module's namespace rather than re-registering the name.

---

## Bug 9: `aria test` on recursive directories shows "no tests found"

**Severity:** Low

`aria test src/diagnostic/` works. `aria test src/` (which should find files recursively) reports "no tests found" even though test blocks exist in subdirectories.

**Fix location:** `cmd/aria/main.go`, function `discoverAriaFiles()` or test runner logic. It may not be recursing into subdirectories when discovering test files.

---

## Bug 10: Test failures show no details

**Severity:** Low but frustrating

When tests fail, the output is just:
```
Some tests failed.
```

No indication of WHICH test failed or what assertion was violated.

**Fix location:** `internal/codegen/build.go`, test runner. The generated Go test functions should print the test name and assertion details on failure before exiting.

---

## Non-Bug Limitations (by design, but worth noting)

- **No `args()` function** — command-line argument access is not implemented
- **No concurrency** (`spawn`, `scope`, channels, `select`) — intentionally omitted
- **No FFI** — pure Aria only
- **No memory annotations** (`@stack`, `@arena`, `@inline`)
- **Functions ending in `println()` without return type annotation get type errors** — the checker doesn't infer void/unit return properly when the last expression is a void call
