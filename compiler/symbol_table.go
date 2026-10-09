package compiler

// SymbolScope says where a name lives, which tells the compiler which
// instructions to use when reading or writing it.
// Concept: scope — where a name lives: global, local, builtin, free or the current function.
type SymbolScope string

const (
	GlobalScope SymbolScope = "GLOBAL"
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
	store          map[string]Symbol
	numDefinitions int
}

// NewSymbolTable returns an empty table.
func NewSymbolTable() *SymbolTable {
	s := make(map[string]Symbol)
	return &SymbolTable{store: s}
}

// Define records a name and gives it the next free index. Defining the same
// name again gives it a fresh index; the old slot is simply never read again.
func (s *SymbolTable) Define(name string) Symbol {
	symbol := Symbol{Name: name, Index: s.numDefinitions, Scope: GlobalScope}
	s.store[name] = symbol
	s.numDefinitions++
	return symbol
}

// Resolve looks a name up. The bool is false if it was never defined.
func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
	obj, ok := s.store[name]
	return obj, ok
}
