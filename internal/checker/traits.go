package checker

// TraitDef describes a trait declaration.
type TraitDef struct {
	Name        string
	Supertraits []string
	Methods     []TraitMethod
}

// TraitMethod describes a method required by a trait.
type TraitMethod struct {
	Name       string
	HasDefault bool
}

// TraitImpl records that a type implements a trait.
type TraitImpl struct {
	TraitName string
	TypeName  string
	Methods   []string
}

// TraitRegistry manages trait definitions and implementations.
type TraitRegistry struct {
	traits map[string]*TraitDef
	impls  []TraitImpl
}

// NewTraitRegistry creates a new registry with built-in traits pre-registered.
func NewTraitRegistry() *TraitRegistry {
	r := &TraitRegistry{
		traits: make(map[string]*TraitDef),
	}
	r.registerBuiltins()
	return r
}

func (r *TraitRegistry) registerBuiltins() {
	// Core traits
	r.traits["Eq"] = &TraitDef{
		Name:    "Eq",
		Methods: []TraitMethod{{Name: "eq", HasDefault: false}},
	}
	r.traits["Ord"] = &TraitDef{
		Name:        "Ord",
		Supertraits: []string{"Eq"},
		Methods:     []TraitMethod{{Name: "cmp", HasDefault: false}},
	}
	r.traits["Hash"] = &TraitDef{
		Name:        "Hash",
		Supertraits: []string{"Eq"},
		Methods:     []TraitMethod{{Name: "hash", HasDefault: false}},
	}
	r.traits["Clone"] = &TraitDef{
		Name:    "Clone",
		Methods: []TraitMethod{{Name: "clone", HasDefault: false}},
	}
	r.traits["Debug"] = &TraitDef{
		Name:    "Debug",
		Methods: []TraitMethod{{Name: "debug", HasDefault: false}},
	}
	r.traits["Display"] = &TraitDef{
		Name:    "Display",
		Methods: []TraitMethod{{Name: "display", HasDefault: false}},
	}
	r.traits["Default"] = &TraitDef{
		Name:    "Default",
		Methods: []TraitMethod{{Name: "default", HasDefault: false}},
	}

	// Arithmetic operator traits
	for _, name := range []string{"Add", "Sub", "Mul", "Div", "Mod", "Neg"} {
		r.traits[name] = &TraitDef{
			Name:    name,
			Methods: []TraitMethod{{Name: lowerFirst(name), HasDefault: false}},
		}
	}
	r.traits["Numeric"] = &TraitDef{
		Name:        "Numeric",
		Supertraits: []string{"Add", "Sub", "Mul", "Div"},
	}

	// Conversion traits
	r.traits["Convert"] = &TraitDef{
		Name:    "Convert",
		Methods: []TraitMethod{{Name: "convert", HasDefault: false}},
	}
	r.traits["TryConvert"] = &TraitDef{
		Name:    "TryConvert",
		Methods: []TraitMethod{{Name: "tryConvert", HasDefault: false}},
	}
	r.traits["From"] = &TraitDef{
		Name:    "From",
		Methods: []TraitMethod{{Name: "from", HasDefault: false}},
	}

	// Iteration traits
	r.traits["Iterable"] = &TraitDef{
		Name:    "Iterable",
		Methods: []TraitMethod{{Name: "iter", HasDefault: false}},
	}
	r.traits["Iterator"] = &TraitDef{
		Name:    "Iterator",
		Methods: []TraitMethod{{Name: "next", HasDefault: false}},
	}

	// Concurrency marker traits (no methods)
	r.traits["Send"] = &TraitDef{Name: "Send"}
	r.traits["Share"] = &TraitDef{Name: "Share"}

	// Error category traits (no methods)
	r.traits["Transient"] = &TraitDef{Name: "Transient"}
	r.traits["Permanent"] = &TraitDef{Name: "Permanent"}
	r.traits["UserFault"] = &TraitDef{Name: "UserFault"}
	r.traits["SystemFault"] = &TraitDef{Name: "SystemFault"}

	// Register built-in trait impls for primitives
	primitives := []string{"i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "usize", "byte"}
	for _, p := range primitives {
		r.impls = append(r.impls,
			TraitImpl{TraitName: "Eq", TypeName: p},
			TraitImpl{TraitName: "Ord", TypeName: p},
			TraitImpl{TraitName: "Hash", TypeName: p},
			TraitImpl{TraitName: "Clone", TypeName: p},
			TraitImpl{TraitName: "Debug", TypeName: p},
			TraitImpl{TraitName: "Display", TypeName: p},
			TraitImpl{TraitName: "Default", TypeName: p},
			TraitImpl{TraitName: "Add", TypeName: p},
			TraitImpl{TraitName: "Sub", TypeName: p},
			TraitImpl{TraitName: "Mul", TypeName: p},
			TraitImpl{TraitName: "Div", TypeName: p},
			TraitImpl{TraitName: "Mod", TypeName: p},
			TraitImpl{TraitName: "Neg", TypeName: p},
			TraitImpl{TraitName: "Numeric", TypeName: p},
		)
	}
	// Floats: Eq and Hash are NOT implemented (NaN issue)
	for _, p := range []string{"f32", "f64"} {
		r.impls = append(r.impls,
			TraitImpl{TraitName: "Clone", TypeName: p},
			TraitImpl{TraitName: "Debug", TypeName: p},
			TraitImpl{TraitName: "Display", TypeName: p},
			TraitImpl{TraitName: "Default", TypeName: p},
			TraitImpl{TraitName: "Add", TypeName: p},
			TraitImpl{TraitName: "Sub", TypeName: p},
			TraitImpl{TraitName: "Mul", TypeName: p},
			TraitImpl{TraitName: "Div", TypeName: p},
			TraitImpl{TraitName: "Neg", TypeName: p},
			TraitImpl{TraitName: "Numeric", TypeName: p},
		)
	}
	// str
	r.impls = append(r.impls,
		TraitImpl{TraitName: "Eq", TypeName: "str"},
		TraitImpl{TraitName: "Ord", TypeName: "str"},
		TraitImpl{TraitName: "Hash", TypeName: "str"},
		TraitImpl{TraitName: "Clone", TypeName: "str"},
		TraitImpl{TraitName: "Debug", TypeName: "str"},
		TraitImpl{TraitName: "Display", TypeName: "str"},
		TraitImpl{TraitName: "Default", TypeName: "str"},
	)
	// bool
	r.impls = append(r.impls,
		TraitImpl{TraitName: "Eq", TypeName: "bool"},
		TraitImpl{TraitName: "Hash", TypeName: "bool"},
		TraitImpl{TraitName: "Clone", TypeName: "bool"},
		TraitImpl{TraitName: "Debug", TypeName: "bool"},
		TraitImpl{TraitName: "Display", TypeName: "bool"},
		TraitImpl{TraitName: "Default", TypeName: "bool"},
	)
}

// RegisterTrait adds a user-defined trait.
func (r *TraitRegistry) RegisterTrait(trait *TraitDef) {
	r.traits[trait.Name] = trait
}

// RegisterImpl records that a type implements a trait.
func (r *TraitRegistry) RegisterImpl(traitName, typeName string, methods []string) {
	r.impls = append(r.impls, TraitImpl{
		TraitName: traitName,
		TypeName:  typeName,
		Methods:   methods,
	})
}

// LookupTrait returns the trait definition or nil.
func (r *TraitRegistry) LookupTrait(name string) *TraitDef {
	return r.traits[name]
}

// Implements checks if a type implements a trait.
func (r *TraitRegistry) Implements(typeName, traitName string) bool {
	for _, impl := range r.impls {
		if impl.TypeName == typeName && impl.TraitName == traitName {
			return true
		}
	}
	return false
}

// IsDerivable checks if a trait can be auto-derived.
func (r *TraitRegistry) IsDerivable(name string) bool {
	switch name {
	case "Eq", "Ord", "Hash", "Clone", "Debug", "Display", "Default", "Json":
		return true
	default:
		return false
	}
}

func lowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]+32) + s[1:]
}
