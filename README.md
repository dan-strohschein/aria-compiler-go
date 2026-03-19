# Aria Bootstrap Compiler

The bootstrap compiler for the [Aria programming language](https://github.com/dan-strohschein/aria-docs), written in Go. Transpiles Aria source code to Go, then invokes `go build` to produce native binaries.

This is throwaway code — its sole purpose is to compile enough Aria to build the self-hosting compiler (written in Aria itself).

## Build

```bash
go build -o aria ./cmd/aria
```

## Usage

```bash
# Type-check a source file
./aria check program.aria

# Compile to native binary
./aria build program.aria

# Compile and run
./aria run program.aria

# Run test blocks
./aria test program.aria

# Dump token stream
./aria lex program.aria

# Dump AST
./aria parse program.aria
```

## Test

```bash
go test ./...
```

## Architecture

The compiler pipeline: **Lexer** → **Parser** → **Resolver** → **Checker** → **Codegen** → `go build`

| Stage | Package | Purpose |
|---|---|---|
| Lexer | `internal/lexer/` | Source → token stream |
| Parser | `internal/parser/` | Tokens → AST (recursive descent + Pratt) |
| Resolver | `internal/resolver/` | Name resolution, scope hierarchy |
| Checker | `internal/checker/` | Type checking, exhaustiveness, effects |
| Codegen | `internal/codegen/` | AST → Go source, build pipeline |
| Diagnostics | `internal/diagnostic/` | Error codes, rendering |

## Aria → Go Mapping

| Aria | Go |
|---|---|
| `struct Point { x: f64 }` | `type Point struct { X float64 }` |
| `type Shape = \| Circle(f64) \| Point` | Interface + variant structs |
| `enum Color { Red, Green }` | `const` with `iota` |
| `fn add(a: i64) -> i64` | `func add(a int64) int64` |
| `match s { Circle(r) => ... }` | `switch v := s.(type) { case ShapeCircle: ... }` |
| `x |> f` | `f(x)` |
| `"hello {name}"` | `fmt.Sprintf("hello %v", name)` |
| `entry { ... }` | `func main() { ... }` |
| `test "name" { assert ... }` | `func TestName(t *testing.T) { ... }` |

## Known Limitations

- Single-file compilation only (no multi-module)
- No concurrency (`spawn`, `scope`, `select`)
- No memory annotations (`@stack`, `@arena`)
- No FFI (`extern`)
- Error handling (`?`, `catch`) is parse-checked but codegen is simplified
- Generics compile to `interface{}` (no monomorphization)
- Effects are compile-time only (erased in codegen)
