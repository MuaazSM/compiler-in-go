package compiler

// SymbolScope says where a name lives, which tells the compiler which
// instructions to use when reading or writing it.
// Concept: scope — where a name lives: global, local, builtin, free or the current function.
type SymbolScope string

const (
	GlobalScope SymbolScope = "GLOBAL"
	LocalScope  SymbolScope = "LOCAL"
)

// Symbol is everything the compiler needs to know about a name.
type Symbol struct {
	Name  string
	Scope SymbolScope
	Index int
}

// SymbolTable hands out an index for every name the program defines, so the
// VM can find values by number instead of by string.
// Concept: symbol table — the compiler's address book: name → (scope, index).
type SymbolTable struct {
	// Outer is the table of the enclosing code. It's nil for the global table.
	Outer *SymbolTable

	store          map[string]Symbol
	numDefinitions int
}

// NewSymbolTable returns an empty table.
func NewSymbolTable() *SymbolTable {
	s := make(map[string]Symbol)
	return &SymbolTable{store: s}
}

// NewEnclosedSymbolTable returns a table for a function body, nested inside outer.
func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
	s := NewSymbolTable()
	s.Outer = outer
	return s
}

// Define records a name and gives it the next free index. A table with no
// Outer is the global one; any other table belongs to a function, so its
// names are locals. Defining the same name again gives it a fresh index; the
// old slot is simply never read again.
func (s *SymbolTable) Define(name string) Symbol {
	symbol := Symbol{Name: name, Index: s.numDefinitions}
	if s.Outer == nil {
		symbol.Scope = GlobalScope
	} else {
		symbol.Scope = LocalScope
	}

	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

// Resolve looks a name up here first, then in each enclosing table in turn.
// The bool is false if no table has it.
func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
	obj, ok := s.store[name]
	if !ok && s.Outer != nil {
		return s.Outer.Resolve(name)
	}
	return obj, ok
}
