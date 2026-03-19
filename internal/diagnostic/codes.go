package diagnostic

// Syntax errors (E0001-E0099)
const (
	E0001 = "E0001" // Invalid character
	E0002 = "E0002" // Unterminated string literal
	E0003 = "E0003" // Invalid escape sequence
	E0004 = "E0004" // Invalid numeric literal
	E0005 = "E0005" // Unexpected token
	E0006 = "E0006" // Expected expression
	E0007 = "E0007" // Expected type
	E0008 = "E0008" // Expected identifier
	E0009 = "E0009" // Expected closing delimiter
	E0010 = "E0010" // Unexpected end of file
	E0011 = "E0011" // Invalid pattern
	E0012 = "E0012" // Duplicate field
	E0013 = "E0013" // Non-associative operator chaining
)

// Type errors (E0100-E0199)
const (
	E0100 = "E0100" // Type mismatch
	E0101 = "E0101" // Cannot infer type
	E0102 = "E0102" // Invalid conversion
	E0103 = "E0103" // Mismatched branch types
	E0104 = "E0104" // Wrong number of arguments
	E0105 = "E0105" // Argument type mismatch
	E0106 = "E0106" // Return type mismatch
	E0107 = "E0107" // Missing struct field
	E0108 = "E0108" // Unknown struct field
	E0109 = "E0109" // Mutability violation
	E0110 = "E0110" // Cannot assign to immutable binding
)

// Trait errors (E0200-E0299)
const (
	E0200 = "E0200" // Trait bound not satisfied
	E0201 = "E0201" // Missing trait implementation
	E0202 = "E0202" // Missing required method
	E0203 = "E0203" // Method signature mismatch
	E0204 = "E0204" // Orphan rule violation
	E0205 = "E0205" // Derive not possible
)

// Effect errors (E0300-E0399)
const (
	E0300 = "E0300" // Effect violation
	E0301 = "E0301" // Pure function calling effectful function
)

// Pattern matching errors (E0400-E0499)
const (
	E0400 = "E0400" // Non-exhaustive match
	E0401 = "E0401" // Unreachable pattern
	E0402 = "E0402" // Or-pattern binding mismatch
)

// Module/import errors (E0700-E0799)
const (
	E0700 = "E0700" // Unresolved import
	E0701 = "E0701" // Unresolved name
	E0702 = "E0702" // Duplicate declaration
	E0703 = "E0703" // Visibility violation
	E0704 = "E0704" // Circular dependency
	E0705 = "E0705" // Import conflict
)

// Const evaluation errors (E0800-E0899)
const (
	E0800 = "E0800" // Invalid const expression
	E0801 = "E0801" // Const overflow
)

// Error handling errors (E0850-E0899)
const (
	E0850 = "E0850" // Error propagation in non-fallible function
	E0851 = "E0851" // Incompatible error type for propagation
)

// Style warnings (W0001-W0099)
const (
	W0001 = "W0001" // Unused variable
	W0002 = "W0002" // Unused import
	W0003 = "W0003" // Non-conventional naming
	W0004 = "W0004" // Shadowed variable
)

// Logic warnings (W0100-W0199)
const (
	W0100 = "W0100" // Unreachable code
	W0101 = "W0101" // Redundant condition
)

// Deprecation warnings (W0200-W0299)
const (
	W0200 = "W0200" // Using deprecated function/type
)
