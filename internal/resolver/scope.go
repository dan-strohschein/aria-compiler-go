package resolver

import (
	"github.com/aria-lang/aria/internal/parser"
)

// ScopeLevel indicates what kind of scope this is.
type ScopeLevel int

const (
	UniverseScope ScopeLevel = iota
	PackageScope
	ModuleScope
	ImportScope
	FunctionScope
	BlockScope
)

// SymbolKind classifies what a symbol refers to.
type SymbolKind int

const (
	SymFunction SymbolKind = iota
	SymVariable
	SymType
	SymEnum
	SymTrait
	SymConst
	SymParam
	SymImport
	SymBuiltinType
	SymBuiltinFn
	SymField
	SymVariant
	SymMethod
	SymGenericParam
)

// Symbol represents a named entity in the program.
type Symbol struct {
	Name    string
	Kind    SymbolKind
	Decl    parser.Decl // the AST node (nil for built-ins)
	Mutable bool        // for variables
	Pos     parser.Pos  // where defined
	// Type is filled in by the type checker later.
}

// Scope is a lexical scope containing symbol bindings.
type Scope struct {
	Level    ScopeLevel
	Parent   *Scope
	Bindings map[string]*Symbol
	Children []*Scope
}

// NewScope creates a new scope with the given level and parent.
func NewScope(level ScopeLevel, parent *Scope) *Scope {
	s := &Scope{
		Level:    level,
		Parent:   parent,
		Bindings: make(map[string]*Symbol),
	}
	if parent != nil {
		parent.Children = append(parent.Children, s)
	}
	return s
}

// Define adds a symbol to this scope. Returns false if already defined in this scope.
func (s *Scope) Define(sym *Symbol) bool {
	if _, exists := s.Bindings[sym.Name]; exists {
		return false
	}
	s.Bindings[sym.Name] = sym
	return true
}

// Lookup searches for a name in this scope and all ancestor scopes.
func (s *Scope) Lookup(name string) *Symbol {
	for scope := s; scope != nil; scope = scope.Parent {
		if sym, ok := scope.Bindings[name]; ok {
			return sym
		}
	}
	return nil
}

// LookupLocal searches only this scope (no parent traversal).
func (s *Scope) LookupLocal(name string) *Symbol {
	return s.Bindings[name]
}

// NewUniverseScope creates the top-level scope with all built-in types and functions.
func NewUniverseScope() *Scope {
	s := NewScope(UniverseScope, nil)

	// Built-in primitive types
	builtinTypes := []string{
		"i8", "i16", "i32", "i64",
		"u8", "u16", "u32", "u64",
		"f32", "f64",
		"str", "bool", "byte", "usize",
	}
	for _, name := range builtinTypes {
		s.Bindings[name] = &Symbol{Name: name, Kind: SymBuiltinType}
	}

	// Built-in generic types
	builtinGenerics := []string{"Option", "Result", "Map", "Set"}
	for _, name := range builtinGenerics {
		s.Bindings[name] = &Symbol{Name: name, Kind: SymBuiltinType}
	}

	// Special types
	s.Bindings["Self"] = &Symbol{Name: "Self", Kind: SymBuiltinType}

	// Built-in functions
	builtinFns := []string{"print", "println", "eprintln", "panic", "assert",
		"_ariaWriteFile", "_ariaReadFile", "_ariaFileExists",
		"_ariaWriteBinaryFile", "_ariaArgs", "_ariaExec",
		"_ariaGetenv", "_ariaListDir", "_ariaIsDir",
		"_ariaTcpSocket", "_ariaTcpBind", "_ariaTcpListen", "_ariaTcpAccept",
		"_ariaTcpRead", "_ariaTcpWrite", "_ariaTcpClose", "_ariaTcpPeerAddr",
		"_ariaTcpSetTimeout",
		"_ariaPgConnect", "_ariaPgClose", "_ariaPgStatus", "_ariaPgError",
		"_ariaPgExec", "_ariaPgExecParams", "_ariaPgResultStatus", "_ariaPgResultError",
		"_ariaPgNrows", "_ariaPgNcols", "_ariaPgFieldName", "_ariaPgGetValue",
		"_ariaPgIsNull", "_ariaPgClear",
		"_ariaSpawn", "_ariaTaskAwait",
		"_ariaChanNew", "_ariaChanSend", "_ariaChanRecv", "_ariaChanClose",
		"_ariaSpawn2", "_ariaTaskAwait2", "_ariaTaskDone", "_ariaTaskCancel", "_ariaTaskResult", "_ariaCancelCheck",
		"_ariaChanTryRecv", "_ariaChanSelect",
		"_ariaRWMutexNew", "_ariaRWMutexRlock", "_ariaRWMutexRunlock", "_ariaRWMutexWlock", "_ariaRWMutexWunlock",
		"_ariaWgNew", "_ariaWgAdd", "_ariaWgDone", "_ariaWgWait",
		"_ariaOnceNew", "_ariaOnceCall",
		"_ariaCancelNew", "_ariaCancelChild", "_ariaCancelTrigger", "_ariaCancelIsTriggered",
		"_ariaSbNew", "_ariaSbWithCapacity", "_ariaSbAppend", "_ariaSbAppendChar", "_ariaSbLen", "_ariaSbBuild", "_ariaSbClear",
		"_ariaMutexNew", "_ariaMutexLock", "_ariaMutexUnlock"}
	for _, name := range builtinFns {
		s.Bindings[name] = &Symbol{Name: name, Kind: SymBuiltinFn}
	}

	return s
}
