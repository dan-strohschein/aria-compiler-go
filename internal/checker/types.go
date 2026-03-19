package checker

import (
	"fmt"
	"strings"
)

// Type represents a resolved type in the Aria type system.
type Type interface {
	typeNode()
	String() string
	Equals(other Type) bool
}

// ---------- Primitive types ----------

type PrimitiveType struct {
	Name string // "i8", "i16", "i32", "i64", "u8", ..., "f32", "f64", "str", "bool", "byte", "usize"
}

func (t *PrimitiveType) typeNode() {}
func (t *PrimitiveType) String() string { return t.Name }
func (t *PrimitiveType) Equals(other Type) bool {
	if o, ok := other.(*PrimitiveType); ok {
		return t.Name == o.Name
	}
	return false
}

// Standard primitive types (singletons).
var (
	TypeI8    = &PrimitiveType{"i8"}
	TypeI16   = &PrimitiveType{"i16"}
	TypeI32   = &PrimitiveType{"i32"}
	TypeI64   = &PrimitiveType{"i64"}
	TypeU8    = &PrimitiveType{"u8"}
	TypeU16   = &PrimitiveType{"u16"}
	TypeU32   = &PrimitiveType{"u32"}
	TypeU64   = &PrimitiveType{"u64"}
	TypeF32   = &PrimitiveType{"f32"}
	TypeF64   = &PrimitiveType{"f64"}
	TypeStr   = &PrimitiveType{"str"}
	TypeBool  = &PrimitiveType{"bool"}
	TypeByte  = &PrimitiveType{"byte"}
	TypeUsize = &PrimitiveType{"usize"}
)

// PrimitiveByName returns the singleton primitive type for a name.
func PrimitiveByName(name string) *PrimitiveType {
	switch name {
	case "i8":
		return TypeI8
	case "i16":
		return TypeI16
	case "i32":
		return TypeI32
	case "i64":
		return TypeI64
	case "u8":
		return TypeU8
	case "u16":
		return TypeU16
	case "u32":
		return TypeU32
	case "u64":
		return TypeU64
	case "f32":
		return TypeF32
	case "f64":
		return TypeF64
	case "str":
		return TypeStr
	case "bool":
		return TypeBool
	case "byte":
		return TypeByte
	case "usize":
		return TypeUsize
	default:
		return nil
	}
}

// IsIntegerType checks if a type is an integer primitive.
func IsIntegerType(t Type) bool {
	if p, ok := t.(*PrimitiveType); ok {
		switch p.Name {
		case "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64", "byte", "usize":
			return true
		}
	}
	return false
}

// IsFloatType checks if a type is a float primitive.
func IsFloatType(t Type) bool {
	if p, ok := t.(*PrimitiveType); ok {
		return p.Name == "f32" || p.Name == "f64"
	}
	return false
}

// IsNumericType checks if a type is numeric (integer or float).
func IsNumericType(t Type) bool {
	return IsIntegerType(t) || IsFloatType(t)
}

// ---------- Special types ----------

// UnitType is the () type — no value.
type UnitType struct{}

func (t *UnitType) typeNode()      {}
func (t *UnitType) String() string { return "()" }
func (t *UnitType) Equals(other Type) bool {
	_, ok := other.(*UnitType)
	return ok
}

// NeverType is the ! type — computation never completes.
type NeverType struct{}

func (t *NeverType) typeNode()      {}
func (t *NeverType) String() string { return "!" }
func (t *NeverType) Equals(other Type) bool {
	_, ok := other.(*NeverType)
	return ok
}

// UnresolvedType is a placeholder for types not yet resolved.
type UnresolvedType struct {
	Name string
}

func (t *UnresolvedType) typeNode()      {}
func (t *UnresolvedType) String() string { return "?" + t.Name }
func (t *UnresolvedType) Equals(other Type) bool { return false }

var TypeUnit = &UnitType{}
var TypeNever = &NeverType{}

// ---------- Compound types ----------

// ArrayType represents [T].
type ArrayType struct {
	Element Type
}

func (t *ArrayType) typeNode()      {}
func (t *ArrayType) String() string { return fmt.Sprintf("[%s]", t.Element) }
func (t *ArrayType) Equals(other Type) bool {
	if o, ok := other.(*ArrayType); ok {
		return t.Element.Equals(o.Element)
	}
	return false
}

// MapType represents Map[K, V].
type MapType struct {
	Key   Type
	Value Type
}

func (t *MapType) typeNode()      {}
func (t *MapType) String() string { return fmt.Sprintf("Map[%s, %s]", t.Key, t.Value) }
func (t *MapType) Equals(other Type) bool {
	if o, ok := other.(*MapType); ok {
		return t.Key.Equals(o.Key) && t.Value.Equals(o.Value)
	}
	return false
}

// SetType represents Set[T].
type SetType struct {
	Element Type
}

func (t *SetType) typeNode()      {}
func (t *SetType) String() string { return fmt.Sprintf("Set[%s]", t.Element) }
func (t *SetType) Equals(other Type) bool {
	if o, ok := other.(*SetType); ok {
		return t.Element.Equals(o.Element)
	}
	return false
}

// TupleType represents (T, U, ...).
type TupleType struct {
	Elements []Type
}

func (t *TupleType) typeNode() {}
func (t *TupleType) String() string {
	parts := make([]string, len(t.Elements))
	for i, e := range t.Elements {
		parts[i] = e.String()
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
func (t *TupleType) Equals(other Type) bool {
	if o, ok := other.(*TupleType); ok {
		if len(t.Elements) != len(o.Elements) {
			return false
		}
		for i := range t.Elements {
			if !t.Elements[i].Equals(o.Elements[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// OptionalType represents T? (sugar for Option[T]).
type OptionalType struct {
	Inner Type
}

func (t *OptionalType) typeNode()      {}
func (t *OptionalType) String() string { return t.Inner.String() + "?" }
func (t *OptionalType) Equals(other Type) bool {
	if o, ok := other.(*OptionalType); ok {
		return t.Inner.Equals(o.Inner)
	}
	return false
}

// ResultType represents T ! E.
type ResultType struct {
	Ok  Type
	Err Type
}

func (t *ResultType) typeNode()      {}
func (t *ResultType) String() string { return fmt.Sprintf("%s ! %s", t.Ok, t.Err) }
func (t *ResultType) Equals(other Type) bool {
	if o, ok := other.(*ResultType); ok {
		return t.Ok.Equals(o.Ok) && t.Err.Equals(o.Err)
	}
	return false
}

// ---------- Named types ----------

// StructType represents a struct declaration's type.
type StructType struct {
	Name   string
	Fields []StructField
}

type StructField struct {
	Name    string
	Type    Type
	Default bool // has a default value
}

func (t *StructType) typeNode()      {}
func (t *StructType) String() string { return t.Name }
func (t *StructType) Equals(other Type) bool {
	if o, ok := other.(*StructType); ok {
		return t.Name == o.Name
	}
	return false
}

// SumType represents a sum type declaration's type.
type SumType struct {
	Name     string
	Variants []SumVariant
}

type SumVariant struct {
	Name   string
	Fields []StructField // struct variant
	Types  []Type        // tuple variant
}

func (t *SumType) typeNode()      {}
func (t *SumType) String() string { return t.Name }
func (t *SumType) Equals(other Type) bool {
	if o, ok := other.(*SumType); ok {
		return t.Name == o.Name
	}
	return false
}

// EnumType represents an enum declaration's type.
type EnumType struct {
	Name    string
	Members []string
}

func (t *EnumType) typeNode()      {}
func (t *EnumType) String() string { return t.Name }
func (t *EnumType) Equals(other Type) bool {
	if o, ok := other.(*EnumType); ok {
		return t.Name == o.Name
	}
	return false
}

// NewtypeType represents a newtype wrapper.
type NewtypeType struct {
	Name       string
	Underlying Type
}

func (t *NewtypeType) typeNode()      {}
func (t *NewtypeType) String() string { return t.Name }
func (t *NewtypeType) Equals(other Type) bool {
	if o, ok := other.(*NewtypeType); ok {
		return t.Name == o.Name
	}
	return false
}

// AliasType is transparent — resolves to its target.
type AliasType struct {
	Name   string
	Target Type
}

func (t *AliasType) typeNode()      {}
func (t *AliasType) String() string { return t.Name }
func (t *AliasType) Equals(other Type) bool {
	// Aliases are transparent: compare against the target
	return t.Target.Equals(other)
}

// ---------- Function types ----------

// FunctionType represents fn(A, B) -> C.
type FunctionType struct {
	Params  []Type
	Return  Type
	Effects []string // effect annotations
	Errors  []Type   // error types (for ! signatures)
}

func (t *FunctionType) typeNode() {}
func (t *FunctionType) String() string {
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = p.String()
	}
	ret := "()"
	if t.Return != nil {
		ret = t.Return.String()
	}
	return fmt.Sprintf("fn(%s) -> %s", strings.Join(params, ", "), ret)
}
func (t *FunctionType) Equals(other Type) bool {
	if o, ok := other.(*FunctionType); ok {
		if len(t.Params) != len(o.Params) {
			return false
		}
		for i := range t.Params {
			if !t.Params[i].Equals(o.Params[i]) {
				return false
			}
		}
		if t.Return == nil && o.Return == nil {
			return true
		}
		if t.Return == nil || o.Return == nil {
			return false
		}
		return t.Return.Equals(o.Return)
	}
	return false
}

// ---------- Generic types ----------

// TypeParam represents a generic type parameter T.
type TypeParam struct {
	Name   string
	Bounds []string // trait bounds
}

func (t *TypeParam) typeNode()      {}
func (t *TypeParam) String() string { return t.Name }
func (t *TypeParam) Equals(other Type) bool {
	if o, ok := other.(*TypeParam); ok {
		return t.Name == o.Name
	}
	return false
}

// GenericType represents a concrete instantiation of a generic type.
type GenericType struct {
	Base     Type
	TypeArgs []Type
}

func (t *GenericType) typeNode() {}
func (t *GenericType) String() string {
	args := make([]string, len(t.TypeArgs))
	for i, a := range t.TypeArgs {
		args[i] = a.String()
	}
	return fmt.Sprintf("%s[%s]", t.Base, strings.Join(args, ", "))
}
func (t *GenericType) Equals(other Type) bool {
	if o, ok := other.(*GenericType); ok {
		if !t.Base.Equals(o.Base) {
			return false
		}
		if len(t.TypeArgs) != len(o.TypeArgs) {
			return false
		}
		for i := range t.TypeArgs {
			if !t.TypeArgs[i].Equals(o.TypeArgs[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// ---------- Error union ----------

// ErrorUnion represents multiple possible error types (AuthError | DbError).
type ErrorUnion struct {
	Types []Type
}

func (t *ErrorUnion) typeNode() {}
func (t *ErrorUnion) String() string {
	parts := make([]string, len(t.Types))
	for i, et := range t.Types {
		parts[i] = et.String()
	}
	return strings.Join(parts, " | ")
}
func (t *ErrorUnion) Equals(other Type) bool {
	if o, ok := other.(*ErrorUnion); ok {
		if len(t.Types) != len(o.Types) {
			return false
		}
		for i := range t.Types {
			if !t.Types[i].Equals(o.Types[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// ---------- Type utilities ----------

// Unwrap resolves aliases to their underlying type.
func Unwrap(t Type) Type {
	if a, ok := t.(*AliasType); ok {
		return Unwrap(a.Target)
	}
	return t
}

// IsAssignable checks if `from` can be assigned to a location expecting `to`.
// NeverType is assignable to anything (divergent expressions).
func IsAssignable(from, to Type) bool {
	if _, ok := from.(*NeverType); ok {
		return true
	}
	return Unwrap(from).Equals(Unwrap(to))
}
