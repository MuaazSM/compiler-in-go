// Package object defines the values a Monkey program works with while it runs:
// numbers, strings, arrays, functions and so on. The compiler stores them in
// the constant pool, and the VM pushes them on its stack.
package object

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"monkey/ast"
	"monkey/code"
	"strings"
)

// BuiltinFunction is the shape of every builtin written in Go, like len.
type BuiltinFunction func(args ...Object) Object

// ObjectType names the kind of a value, so code can check it before using it,
// for example before adding two values.
type ObjectType string

const (
	NULL_OBJ  = "NULL"
	ERROR_OBJ = "ERROR"

	INTEGER_OBJ = "INTEGER"
	BOOLEAN_OBJ = "BOOLEAN"
	STRING_OBJ  = "STRING"

	RETURN_VALUE_OBJ = "RETURN_VALUE"

	FUNCTION_OBJ = "FUNCTION"
	BUILTIN_OBJ  = "BUILTIN"

	ARRAY_OBJ = "ARRAY"
	HASH_OBJ  = "HASH"

	COMPILED_FUNCTION_OBJ = "COMPILED_FUNCTION_OBJ"
	CLOSURE_OBJ           = "CLOSURE"
)

// HashKey is what a value turns into when used as a hash map key. Two equal
// strings give the same HashKey, so person["name"] finds the right entry even
// though the two "name" strings are separate objects.
type HashKey struct {
	Type  ObjectType
	Value uint64
}

// Hashable is any value that can be a hash key: integers, booleans and
// strings. Arrays and functions can't, and using one gives an error.
type Hashable interface {
	HashKey() HashKey
}

// Object is every Monkey value. Type says what kind it is, and Inspect prints
// it the way the REPL shows it.
type Object interface {
	Type() ObjectType
	Inspect() string
}

// Integer is a whole number. Monkey has no decimals.
type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

// Boolean is true or false.
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) HashKey() HashKey {
	var value uint64

	if b.Value {
		value = 1
	} else {
		value = 0
	}

	return HashKey{Type: b.Type(), Value: value}
}

// Null is the "no value" value, for example the result of an if with no else
// when the condition is false.
type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

// ReturnValue is only used by the tree-walking interpreter. It wraps the value
// of a return statement so the interpreter knows to stop the function early.
// The VM uses OpReturnValue instead.
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

// Error carries a message from a builtin that was used wrongly, like
// len(1).
type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

// Function is a function value in the tree-walking interpreter. It keeps the
// AST of its body and the environment it was created in. The compiled version
// used by the VM is CompiledFunction below.
type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("fn")
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(f.Body.String())
	out.WriteString("\n}")

	return out.String()
}

// String is a piece of text.
type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

// HashKey for a string is a fast hash of its text, so equal strings get
// equal keys.
func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(s.Value))

	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

// Builtin wraps a Go function so it can be stored and passed around like
// any other Monkey value.
type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }

// Array is an ordered list of values, like [1, 2, 3].
type Array struct {
	Elements []Object
}

func (ao *Array) Type() ObjectType { return ARRAY_OBJ }
func (ao *Array) Inspect() string {
	var out bytes.Buffer

	elements := []string{}
	for _, e := range ao.Elements {
		elements = append(elements, e.Inspect())
	}

	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}

// HashPair keeps the original key next to its value, so Inspect can print
// the key the way the user wrote it.
type HashPair struct {
	Key   Object
	Value Object
}

// Hash is a dictionary from keys to values, like {"name": "Ada"}.
type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }
func (h *Hash) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s",
			pair.Key.Inspect(), pair.Value.Inspect()))
	}

	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}

// CompiledFunction is a function after compilation: just its bytecode. It lives
// in the constant pool like any other literal value.
type CompiledFunction struct {
	Instructions code.Instructions
	// NumLocals tells the VM how many stack slots to set aside for this
	// function's local variables when it's called.
	NumLocals int
	// NumParameters is how many arguments a call has to pass.
	NumParameters int
}

func (cf *CompiledFunction) Type() ObjectType { return COMPILED_FUNCTION_OBJ }
func (cf *CompiledFunction) Inspect() string {
	return fmt.Sprintf("CompiledFunction[%p]", cf)
}

// Closure is what the VM actually calls: a compiled function plus the values
// it captured from the functions around it when it was created.
// Concept: closure - a function bundled with the variables it captured from outer scopes.
type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}

func (c *Closure) Type() ObjectType { return CLOSURE_OBJ }
func (c *Closure) Inspect() string {
	return fmt.Sprintf("Closure[%p]", c)
}
