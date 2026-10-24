package object

// NewEnclosedEnvironment makes the environment for a function call. It points
// back to the environment the function was created in, so the body can still
// read variables from outside.
func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

// NewEnvironment makes an empty, top-level environment.
func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil}
}

// Environment is how the tree-walking interpreter remembers variables: a map
// from name to value, plus a link to the environment around it. The VM doesn't
// use this at all; the compiler turns names into slot numbers instead.
// Concept: environment - a chain of name-to-value maps, searched from the inside out.
type Environment struct {
	store map[string]Object
	outer *Environment
}

// Get looks for a name here first, then in each outer environment in turn.
// This lookup by name, on every use, is a big part of why the interpreter is
// slower than the VM.
func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

// Set stores a value in this environment only, never in an outer one.
func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}
