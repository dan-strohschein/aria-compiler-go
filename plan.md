# Aria Bootstrap Compiler -- Implementation Plan

## Context

Aria is a programming language designed for AI code generation -- optimized for token efficiency, type safety, and unambiguous grammar. The bootstrap compiler is a throwaway Go program whose sole purpose is to compile enough Aria to build the self-hosting compiler (written in Aria itself).

**Key decisions:**
- **Code generation:** Transpile Aria -> Go source code, then invoke `go build`
- **Feature scope:** Only the Aria subset needed to write a compiler (no concurrency, no FFI, no memory annotations)
- **Self-hosting compiler:** Starts single-threaded; concurrency is a later optimization
- **Testing:** Compiler-focused test cases, not the spec's example programs

**Aria -> Go type mappings:**

| Aria | Go |
|---|---|
| Structs | Go structs |
| Sum types | Interface + variant structs |
| Pattern match | `switch v := expr.(type)` |
| `?` propagation | `if err != nil { return ..., err }` |
| Traits | Go interfaces |
| Generics | Go generics (1.18+) |
| `Option[T]` | Pointer `*T` or custom generic |
| `Result[T,E]` / `!E` | `(T, error)` multi-return |
| Effects | Erased (compile-time only) |
| String interpolation | `fmt.Sprintf(...)` |
| Closures | Go function literals |
| `[T]`, `Map[K,V]` | Go slices, maps |

---

## Milestone 1: Foundation -- Lexer & Project Scaffolding

**Goal:** Tokenize Aria source files. `aria lex <file>` dumps the token stream.

### Phase 1.1: Project Scaffolding

**US-1.1.1: Initialize Go module and directory structure**
- Create `go.mod`, full directory tree (`cmd/aria/`, `internal/{lexer,parser,resolver,checker,ir,codegen,runtime,diagnostic}/`, `testdata/`)
- Minimal `cmd/aria/main.go` that prints version
- AC: `go build ./cmd/aria && ./aria --version` prints "aria 0.1.0-bootstrap"

**US-1.1.2: CLI framework with subcommands**
- Implement CLI parsing: `lex`, `parse`, `check`, `build`, `run`, `test` subcommands
- Each accepts a file path; `--format` flag (text/json)
- AC: `./aria lex test.aria` prints "not yet implemented"; unknown commands print usage

**US-1.1.3: Diagnostic infrastructure**
- `internal/diagnostic/`: `Diagnostic` struct (Code, Severity, Message, File, Line, Column, Span, SourceLine, Labels, Notes, Suggestions)
- Error codes: E0001-E0099 (syntax), W0001+ (warnings)
- Human-readable renderer with source line + caret
- JSON renderer per `compiler-diagnostics.md`
- AC: Unit tests produce formatted error output matching spec format

### Phase 1.2: Token Types & Lexer Core

**US-1.2.1: Token type definitions**
- `internal/lexer/token.go`: Token struct (Type, Literal, Position)
- All TokenType constants from `formal-grammar.md` section 2.1
- Concurrency keywords (spawn, scope, select) recognized but flagged as unsupported
- AC: All token types defined; `TokenType.String()` works

**US-1.2.2: Lexer core -- identifiers, keywords, whitespace, comments**
- `internal/lexer/lexer.go`: `Lexer` struct, `New()`, `NextToken()`
- Skip whitespace/tabs, line comments (`//`), identifiers vs keywords, position tracking
- AC: `"mod main\n// comment\nuse std.fs"` -> correct token sequence

**US-1.2.3: Lexer -- numeric literals**
- Decimal with underscore separators, hex (`0xFF`), octal (`0o77`), binary (`0b1010`)
- Float literals requiring digits on both sides of `.`
- No suffixes (Decision 10)
- AC: Table-driven tests cover all numeric forms and edge cases

**US-1.2.4: Lexer -- string literals with interpolation**
- Escape sequences: `\n`, `\r`, `\t`, `\\`, `\"`, `\{`, `\}`, `\0`
- Interpolation: `"hello {name}"` emits StringStart + expression tokens + StringEnd
- AC: Interpolation token sequence is correct

**US-1.2.5: Lexer -- operators and punctuation**
- All single/multi-char operators: `+`, `-`, `*`, `/`, `%`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `&&`, `||`, `!`, `&`, `|`, `^`, `~`, `<<`, `>>`, `|>`, `?`, `?.`, `??`, `..`, `..=`, `:=`, `=`, `=>`, `->`, `.`, `.{`, `@`
- Delimiters: `(`, `)`, `[`, `]`, `{`, `}`, `,`, `:`
- AC: Longest-match works (`|>` not `|` then `>`)

### Phase 1.3: Newline Rules & Completion

**US-1.3.1: Automatic statement termination**
- Newline IS terminator when prev token is: IDENT, literal, `)`, `]`, `}`, `?`, `!`, `break`, `continue`, `return`
- Newline NOT terminator after: binary ops, `|>`, `:=`, `=`, `=>`, `->`, `,`, `.`, open delimiters
- Inside balanced delimiters, newlines are whitespace
- AC: Multi-line pipelines, method chains, struct literals lex correctly

**US-1.3.2: Lexer error handling**
- Invalid chars -> E0001, unterminated strings -> E0002, bad escapes -> E0003, bad numbers -> E0004
- All errors include source line text and position
- AC: Each error case produces diagnostic with correct code and suggestion

**US-1.3.3: Lexer test suite**
- `testdata/lexer/` with test files, table-driven tests
- Edge cases: empty file, only comments, deeply nested interpolation, all newline rules
- AC: `go test ./internal/lexer/ -v` passes 100%

### Milestone 1 Verification
- `./aria lex testdata/lexer/*.aria` dumps token streams
- JSON output works; all tests pass; diagnostics match spec format

---

## Milestone 2: Parser -- Source to AST

**Goal:** Parse Aria subset into a typed AST. `aria parse <file>` dumps the AST.

### Phase 2.1: AST Node Types

**US-2.1.1: Declaration AST nodes**
- `internal/parser/ast.go`: concrete types (not interfaces), every node has `Pos`
- Declarations: `Program`, `ModDecl`, `ImportDecl`, `FnDecl`, `TypeDecl`, `EnumDecl`, `TraitDecl`, `ImplDecl`, `ConstDecl`, `EntryBlock`, `TestBlock`
- `FnDecl`: name, generic params, params, return type, error types, effects, body
- `TypeDecl`: structs, sum types, newtypes; `derives` clause
- AC: All types compile; each has `Pos`

**US-2.1.2: Expression AST nodes**
- Binary/unary/postfix, call, index, field access, optional chain, method call, record update
- Literals: int, float, string, bool, interpolated string
- Compound: if, match, closure, block, struct, array, map, tuple
- Special: pipeline, coalesce, range, error prop (`?`), assert (`!`)
- AC: All expression types compile; debug string serialization works

**US-2.1.3: Statement, pattern, and type AST nodes**
- Statements: VarDecl, Assignment, ExprStmt, ForLoop, WhileLoop, Loop, Return, Break, Continue, Defer
- Patterns: Wildcard, Binding, Literal, Struct, Variant, Tuple, Array, Or, Rest, Named (`@`)
- Types: Named, Function, Tuple, Array, Map, Set, Optional, Result, Generic
- AC: Complete AST can represent all needed Aria code

**US-2.1.4: AST serialization**
- Indented text output and JSON serialization
- AC: Parsed AST round-trips to readable text and valid JSON

### Phase 2.2: Parsing Declarations & Statements

**US-2.2.1: Parser infrastructure and top-level structure**
- `internal/parser/parser.go`: `Parser` struct, `New(tokens)`, `Parse() (*Program, []Diagnostic)`
- Parse `mod` + `use` + top-level declarations
- Error recovery: skip to next statement boundary on error
- AC: `mod main\nuse std.fs` parses correctly

**US-2.2.2: Function declarations**
- Visibility, name, generics `[T: Bound]`, params (types, defaults, mut), return type, `! Error`, `with [Effect]`, body (`= expr` or `{ block }`)
- AC: Both single-expression and block functions parse

**US-2.2.3: Type declarations**
- `struct Name { fields }` and `type Name { fields }` (identical AST, Decision 1)
- Sum types: `type Shape = | Circle(f64) | Rect { w: f64, h: f64 }`
- Newtypes: `type UserId = i64`; Enums: `enum Color { Red, Green, Blue }`
- Aliases: `alias Headers = Map[str, str]`; `derives [...]`
- Disambiguation: `{` = struct, `|` = sum type, single type = newtype
- AC: All type forms parse

**US-2.2.4: Trait and impl declarations**
- Traits with supertraits, methods with optional default bodies
- `impl Trait for Type`, `impl Type` (inherent), generic impls with where clauses
- AC: Trait/impl blocks parse correctly

**US-2.2.5: Entry, test, const**
- `entry { ... }`, `test "name" { ... }`, `const NAME [: type] = expr`
- AC: All three forms parse

**US-2.2.6: Statement parsing**
- Variable decls: `[mut] pattern := expr`, `name: type = expr`
- Assignment, for/while/loop, return/break/continue, defer
- AC: All statement forms parse; table-driven tests

### Phase 2.3: Expression Parser (Pratt)

**US-2.3.1: Precedence climbing infrastructure**
- `internal/parser/precedence.go`: Pratt parser with binding powers from `operator-precedence.md` (16 levels)
- Prefix (nud) vs infix (led); non-associative operators reject chaining
- AC: `2 + 3 * 4` -> `Add(2, Mul(3, 4))`; `a < b < c` -> error

**US-2.3.2: Binary and unary operators**
- All arithmetic, comparison, logical, bitwise operators
- Prefix unary: `-`, `!`, `~`
- AC: Mixed-precedence expressions parse correctly

**US-2.3.3: Postfix operators and field access**
- Postfix `?` and `!`; field access `.field`; optional chain `?.`; method call; index `[idx]`; function call `(args)`; record update `.{ field: val }`
- Disambiguate prefix `!` from postfix `!` by position
- AC: `fs.read("file")?.trim()!` parses correctly

**US-2.3.4: Pipeline, coalesce, range**
- Pipeline `|>` with `.field` shorthand; null coalesce `??`; ranges `..` and `..=`
- AC: `input |> parse? |> validate |> transform` parses

**US-2.3.5: Primary expressions**
- Literals, identifiers, paths, parenthesized, blocks, struct construction, array/map/tuple literals
- Disambiguate `{` after identifier (struct literal vs block)
- AC: All primary forms parse

**US-2.3.6: If, match, closure expressions**
- `if/else` chains as expressions; `match` with patterns and guards; closures `fn(params) => expr`
- AC: Nested if/match and closures parse

### Phase 2.4: Patterns & Completion

**US-2.4.1: Pattern parsing**
- All pattern types: wildcard, binding, literal, variant, struct, tuple, array, or-pattern, named (`@`), rest (`..`/`..name`)
- AC: All pattern forms from spec parse correctly

**US-2.4.2: Type annotation parsing**
- Named, generic, function, array, map, set, optional (`T?`), result (`T ! E`), tuple types
- Disambiguate `[T]` as generic vs array type by context
- AC: All type forms parse

**US-2.4.3: Import path parsing**
- Simple `use std.fs`, grouped `use std.{json, http}`, aliased `use crypto.sha256 as sha`
- AC: All import forms parse

**US-2.4.4: Parser test suite**
- `testdata/parser/` with test files; table-driven tests for all forms
- Error recovery tests; ambiguity resolution tests
- AC: `go test ./internal/parser/ -v` passes comprehensively

### Milestone 2 Verification
- `./aria parse testdata/parser/*.aria` dumps ASTs
- JSON output works; parser handles all needed Aria syntax
- Error recovery produces multiple diagnostics per file

---

## Milestone 3: Name Resolution & Type Checking

**Goal:** Resolve names and verify type correctness. `aria check <file>` reports errors or "OK".

### Phase 3.1: Name Resolution

**US-3.1.1: Scope hierarchy and symbol table**
- `internal/resolver/scope.go`: `Scope` with parent pointer, bindings map, level (Universe/Package/Module/Import/Function/Block)
- Symbol types with name, kind, declaration node, type (filled by checker)
- AC: Scope chain lookup works; shadowing creates new bindings

**US-3.1.2: Universe scope with built-ins**
- Built-in types: `i8`-`i64`, `u8`-`u64`, `f32`, `f64`, `str`, `bool`, `byte`, `usize`
- Built-in generics: `Option[T]`, `Result[T, E]`
- Built-in functions: `print`, `println`, `panic`, `assert`
- AC: Built-ins resolve from any scope

**US-3.1.3: Module-level pre-pass**
- Register all top-level names before resolving bodies (forward references)
- Detect duplicate top-level names
- AC: Functions can reference each other regardless of declaration order

**US-3.1.4: Import resolution**
- Resolve `use` to symbols; simple module registry for known paths (std.io, etc.)
- Handle aliases; detect import conflicts
- AC: `use std.fs` makes `fs.read` available

**US-3.1.5: Block-level name resolution**
- Walk function bodies resolving identifiers; shadowing within blocks
- `for` loop vars, `match` arm bindings, closure captures
- Detect undefined identifiers (E0701)
- AC: All scope levels tested; undefined names produce diagnostics

**US-3.1.6: Resolver test suite**
- Forward references, shadowing, captures, error cases
- AC: `go test ./internal/resolver/ -v` passes

### Phase 3.2: Type System Foundation

**US-3.2.1: Type representation**
- `internal/checker/types.go`: primitives, compounds (array, map, set, tuple, optional, result), named (struct, sum, enum, newtype, alias), function, generic (TypeParam, GenericInstance), special (Unit, Never, Unresolved, ErrorUnion)
- AC: All type kinds constructable and comparable

**US-3.2.2: Type environment and context**
- `internal/checker/checker.go`: `Checker` with type env, current function context (return type, error type, effects), mutability tracking
- AC: Context correctly tracks function info

**US-3.2.3: Trait registry**
- `internal/checker/traits.go`: registry of trait definitions + implementations
- Built-in traits: Eq, Ord, Hash, Clone, Debug, Display, Default, Add, Sub, Mul, Div, Mod, Neg, Numeric, Convert, TryConvert, From, Iterable, Iterator
- Built-in impls for all primitives
- AC: Trait/method lookup works

### Phase 3.3: Type Checking Core

**US-3.3.1: Literal and expression type inference**
- Literals: `42` -> `i64`, `3.14` -> `f64`, `"hi"` -> `str`, `true` -> `bool`
- Binary ops: arithmetic requires same numeric type, comparison requires Eq/Ord, logical requires bool
- AC: `2 + 3` infers `i64`; `2 + 3.0` -> type error

**US-3.3.2: Function call type checking**
- Arg count/types, named args, defaults, return type, method calls
- AC: Wrong arg count/type produces diagnostics

**US-3.3.3: Control flow type checking**
- if/else branches same type; match arms same type; block type = last expr
- for/while/loop are `()`; break with value types loop
- AC: Mismatched branch types -> error

**US-3.3.4: Variable binding and assignment**
- Inferred (`x := expr`), annotated (`x: T = expr`), assignment (`x = expr`)
- Mutability: immutable bindings can't be reassigned
- AC: `x := 5; x = 6` -> error (not mut)

**US-3.3.5: Compound type checking**
- Struct construction (required fields, types), array (same element type), map (same K/V types), tuples, record update
- AC: Missing/mistyped struct fields -> diagnostics

### Phase 3.4: Advanced Type Checking

**US-3.4.1: Error type checking**
- `! E` return type, `?` propagation, `!` assert, `catch` blocks, `yield`
- `?` in function without `!` -> compile error
- AC: Error propagation chains type-check correctly

**US-3.4.2: Pattern exhaustiveness**
- `internal/checker/patterns.go`: exhaustiveness algorithm
- Sum types: all variants covered; enums: all members; bools: both values
- Guards don't count toward exhaustiveness; or-patterns bind same names
- AC: Missing arms -> E0401 listing missing variants

**US-3.4.3: Generic instantiation**
- Type parameter substitution, trait bound checking, multiple bounds, where clauses
- AC: `identity(42)` infers `T = i64`

**US-3.4.4: Trait verification and derives**
- `impl` must provide all required methods with matching signatures
- `derives [Eq, Hash]` verifies all fields implement those traits
- AC: Missing methods -> diagnostic; derives on non-qualifying field -> error

**US-3.4.5: Closure type checking**
- Type `fn(A) -> B`, bidirectional inference from context, capture analysis
- AC: `items.map(fn(x) => x * 2)` infers closure type

**US-3.4.6: Mutability checking**
- Immutable bindings, deep immutability, `mut ref self` receivers
- AC: All mutability violations produce clear diagnostics

**US-3.4.7: Effect checking (simplified)**
- Track `Io` and `Fs` only; pure functions can't call effectful functions
- Entry/test blocks have all effects
- AC: Pure fn calling `println` -> E0301

**US-3.4.8: Type checker test suite**
- `testdata/checker/pass/` and `testdata/checker/fail/` with expected error codes
- AC: `go test ./internal/checker/ -v` passes

### Milestone 3 Verification
- `aria check` reports "OK" for valid programs, correct diagnostics for invalid
- JSON diagnostic output works; all tests pass

---

## Milestone 4: IR & Go Code Generation

**Goal:** Compile type-checked Aria -> Go source -> native binary. `aria build` and `aria run` work.

### Phase 4.1: IR Generation

**US-4.1.1: IR node types**
- `internal/ir/ir.go`: SSA-form IR (Module, Function, Block, Instruction)
- Instructions: Const, BinOp, UnaryOp, Call, Return, Branch, Phi, Alloc, Load, Store, FieldAccess, ArrayIndex, TypeAssert
- AC: IR types compile; can represent simple programs

**US-4.1.2: AST-to-IR lowering -- basics**
- `internal/ir/builder.go`: lower literals, variables, binary/unary ops, assignments, blocks, functions
- AC: `fn add(a: i64, b: i64) -> i64 = a + b` lowers to IR

**US-4.1.3: AST-to-IR lowering -- control flow**
- if/else -> branches; match -> switch/decision tree; for/while/loop; break/continue/return
- AC: Control flow IR is correct

**US-4.1.4: AST-to-IR lowering -- types and errors**
- Struct construction/access, sum type variants, `?`/`!`/`catch`, defer
- AC: Error handling patterns lower correctly

### Phase 4.2: Go Code Generation

**US-4.2.1: Codegen infrastructure**
- `internal/codegen/codegen.go`: `GoGenerator`, Go source emitter, `package main`, `import` block, `main()` from entry
- AC: Empty program -> valid Go that compiles

**US-4.2.2: Primitives and operations**
- Type mapping (i64->int64, etc.), arithmetic/comparison/logical/bitwise, string interpolation -> `fmt.Sprintf`, var decls, consts
- AC: Arithmetic expressions compile and run

**US-4.2.3: Functions**
- Go functions, parameter types, default params, `! E` -> `(T, error)` return, function calls, pipeline desugar `x |> f` -> `f(x)`
- AC: Function decls and calls produce working Go

**US-4.2.4: Structs and sum types**
- Structs -> Go structs
- Sum types -> Go interface + variant structs (`type Shape interface { isShape() }`, `type ShapeCircle struct { ... }`)
- Newtypes -> Go `type X Y`; enums -> `const` iota; record update -> struct copy
- AC: Sum type construction and dispatch work

**US-4.2.5: Pattern matching**
- Sum types -> `switch v := expr.(type)` with type assertions
- Struct/tuple destructuring, literal matching, wildcards, or-patterns, guards
- AC: Pattern matching produces correct Go switches

**US-4.2.6: Error handling**
- `?` -> `val, err := expr; if err != nil { return ..., err }`
- `!` -> `if err != nil { panic(err) }`
- `catch` -> `if err != nil { val = default }`
- Error sum types implement Go `error` interface
- AC: Error chains work end-to-end

**US-4.2.7: Closures, collections, control flow**
- Closures -> Go func literals; arrays -> slices; maps -> Go maps
- Collection methods (map, filter, fold) -> generic helper functions or inline loops
- for/while/loop -> Go for; break/continue; defer; list comprehension -> loop+append
- AC: Collection operations produce correct Go

**US-4.2.8: Traits and generics**
- Traits -> Go interfaces; impl -> Go methods; inherent impl -> methods on type
- Generics -> Go generics; trait bounds -> type constraints
- derives -> generated method implementations
- AC: Generic functions and trait dispatch work

### Phase 4.3: Build Pipeline

**US-4.3.1: Go build invocation**
- Generate Go source files, create temp Go module, invoke `go build`, copy binary, cleanup
- `--keep-generated` flag to inspect output
- AC: `aria build hello.aria` produces working binary

**US-4.3.2: Runtime library**
- `internal/runtime/`: Go package copied into generated projects
- Option/Result types, collection utils (map, filter, fold), string ops, file I/O wrappers, println/print
- AC: Runtime functions work in generated code

**US-4.3.3: `aria run`**
- Build + execute + forward exit code + cleanup
- AC: `aria run hello.aria` prints output

**US-4.3.4: `aria test`**
- Generate Go test functions from Aria `test` blocks
- `assert` -> `if !expr { t.Fatal(...) }` with value display
- Invoke `go test`; report in Aria format
- AC: `aria test` runs tests and reports pass/fail

**US-4.3.5: End-to-end integration tests**
- `testdata/programs/`: hello world, fibonacci, structs, sum types, match, errors, closures, generics, traits, collections
- Each specifies expected output and exit code
- AC: All programs compile, run, produce expected output

### Milestone 4 Verification
- `aria build/run/test` all work
- Programs using structs, sum types, match, errors, closures, generics, traits compile and run
- Generated Go is correctly formatted

---

## Milestone 5: Self-Hosting Readiness

**Goal:** Handle everything needed for the self-hosting compiler. Validate with a compiler-like test program.

### Phase 5.1: Standard Library Stubs

**US-5.1.1: String operations**
- split, trim, contains, startsWith, endsWith, replace, toLower, toUpper, len, charAt, substring, indexOf, join
- AC: All string ops work in generated code

**US-5.1.2: Collection operations**
- `[T]`: len, append, map, filter, fold, contains, sort, sortBy, reverse, take, drop, first, last, isEmpty, enumerate, zip, flatten, flatMap, any, all, find, groupBy
- `Map[K,V]`: get, set, delete, contains, keys, values, entries, len, merge
- `Set[T]`: add, remove, contains, len, union, intersection, difference
- AC: All collection methods work

**US-5.1.3: File I/O**
- io.readFile, io.writeFile, io.appendFile, io.exists, io.listDir, println, print
- AC: File operations work correctly

**US-5.1.4: Type conversions**
- Integer widening `i64(myI32)`, checked narrowing `.to[T]()`, string conversions `toStr()`, `parseInt`, `parseFloat`
- AC: Conversions generate correct Go

### Phase 5.2: Advanced Features

**US-5.2.1: Multi-module compilation**
- Compile multiple `.aria` files with `use`; build dependency graph; compile in order; generate Go packages per module
- AC: 3+ module program compiles and runs

**US-5.2.2: Visibility enforcement**
- Private (default), `pub`, `pub(pkg)` -- check at import/use sites
- AC: Accessing private decls from another module -> diagnostics

**US-5.2.3: Recursive types**
- Detect recursive references (Decision 4); auto-box with pointers in Go
- AC: Trees and linked lists compile and work

**US-5.2.4: Complex derives**
- Eq (field-by-field equality), Hash, Clone (deep copy), Debug, Display, Default
- AC: All derivable traits produce correct Go

**US-5.2.5: Improved diagnostics**
- "Did you mean?" (Levenshtein), import suggestions, multi-span diagnostics, fix suggestions with applicability levels
- AC: Diagnostic quality matches spec

### Phase 5.3: Validation

**US-5.3.1: Compiler-like test program**
- Write a 300+ line Aria program resembling a compiler component: token sum types, simple lexer with match, structs with derives, generics, traits, error handling, closures+collections, string interpolation, file I/O
- AC: Compiles and runs correctly with `aria run`

**US-5.3.2: Performance baseline**
- Target: < 2s for 1000-line programs (including Go compilation)
- Profile and optimize if needed
- AC: Compilation time acceptable for iterative development

**US-5.3.3: Error message quality audit**
- Every error has: code, location, source line, explanation
- Type errors show expected vs actual; missing match arms list all missing variants
- AC: Error messages clear enough for AI to fix code in one iteration

**US-5.3.4: Documentation and cleanup**
- README with build/run/test instructions; document Aria->Go mapping; known limitations; `go vet`/`golint` clean
- AC: New developer/AI can build and use the compiler from README

### Milestone 5 Verification
- 300+ line compiler-like program compiles and runs
- Multi-module programs work; stdlib stubs functional
- Diagnostics are high quality; performance acceptable

---

## Critical Spec Files

| File | Used For |
|---|---|
| `../aria-docs/spec/formal-grammar.md` | Lexer token types, parser grammar productions, disambiguation |
| `../aria-docs/spec/operator-precedence.md` | Pratt parser binding powers |
| `../aria-docs/spec/scoping-rules.md` | Name resolution algorithm, scope hierarchy |
| `../aria-docs/spec/trait-system.md` | Traits, impls, derives, method resolution |
| `../aria-docs/spec/generics-type-parameters.md` | Generics, monomorphization, type inference |
| `../aria-docs/spec/pattern-matching.md` | Pattern types, exhaustiveness algorithm |
| `../aria-docs/spec/error-handling.md` | `?`/`!`/`catch`, error unions, Result/Option |
| `../aria-docs/spec/compiler-diagnostics.md` | Error codes, output format, suggestions |
| `../aria-docs/spec/design-decisions-v01.md` | Key design decisions affecting implementation |

---

## Risk Mitigation

1. **Sum types -> Go**: The interface + variant struct mapping is the trickiest part. Prototype early (US-4.2.4). Fallback: tagged struct with variant fields instead of interfaces.
2. **Go generics limitations**: Go lacks method-level type params and has limited constraints. Fallback: monomorphize at Aria level (generate separate Go types per instantiation).
3. **Scope creep**: The "NOT needed" list is critical. No spawn/scope/channels, no memory annotations, no FFI, no complex effects. If borderline, leave it out.
4. **Go compilation overhead**: The `go build` step adds latency. Mitigate: cache generated Go module, only regenerate changed files (defer to M5).
