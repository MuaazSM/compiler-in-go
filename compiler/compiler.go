// Package compiler turns a parsed Monkey program into bytecode for the VM.
package compiler

import (
	"fmt"
	"monkey/ast"
	"monkey/code"
	"monkey/object"
	"sort"
)

// EmittedInstruction remembers an opcode we emitted and where it starts.
type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

// CompilationScope is one instruction buffer. A function body gets its own, so
// its bytecode doesn't get mixed into the code around it.
// Concept: compilation scope — a separate instruction buffer used while compiling a function body.
type CompilationScope struct {
	instructions code.Instructions

	// The last two instructions we emitted. We sometimes need to take back the
	// last one, and then the one before it becomes "last" again.
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}

// Compiler walks the AST once and appends instructions as it goes.
type Compiler struct {
	constants []object.Object

	symbolTable *SymbolTable

	// scopes is a stack: scopes[0] is the main program, and each function
	// literal we're inside of adds one on top.
	scopes     []CompilationScope
	scopeIndex int
}

// New returns an empty compiler.
func New() *Compiler {
	mainScope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	symbolTable := NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	return &Compiler{
		constants:   []object.Object{},
		symbolTable: symbolTable,
		scopes:      []CompilationScope{mainScope},
		scopeIndex:  0,
	}
}

// NewWithState returns a compiler that carries on from earlier ones, reusing
// their names and constant pool. The REPL uses this so each line can see what
// earlier lines defined.
func NewWithState(s *SymbolTable, constants []object.Object) *Compiler {
	compiler := New()
	compiler.symbolTable = s
	compiler.constants = constants
	return compiler
}

// Compile emits bytecode for node and everything below it.
func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		// A statement like `1 + 2;` leaves its value on the stack, and nobody
		// is going to use it. Without this pop, a long program would keep
		// piling up leftovers until the stack overflowed.
		c.emit(code.OpPop)

	case *ast.InfixExpression:
		// Both sides go on the stack first (left, then right), and then the
		// operator instruction works on whatever is at the top.
		// Concept: stack machine — a VM that does all its work by pushing and popping values on one stack.

		// `a < b` means the same as `b > a`. So we push b first, then a, and
		// reuse OpGreaterThan. One less opcode for the VM to know about.
		if node.Operator == "<" {
			err := c.Compile(node.Right)
			if err != nil {
				return err
			}

			err = c.Compile(node.Left)
			if err != nil {
				return err
			}

			c.emit(code.OpGreaterThan)
			return nil
		}

		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "+":
			c.emit(code.OpAdd)
		case "-":
			c.emit(code.OpSub)
		case "*":
			c.emit(code.OpMul)
		case "/":
			c.emit(code.OpDiv)
		case ">":
			c.emit(code.OpGreaterThan)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.IfExpression:
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}

		// We don't know where the consequence ends yet, so we jump to a dummy
		// address (9999) for now and fix it once we've compiled the block.
		// Concept: back-patching — emit a placeholder jump, fill in the real target later.
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(node.Consequence)
		if err != nil {
			return err
		}

		// The block's last expression statement ends in OpPop, which would throw
		// away the very value the `if` is supposed to produce. Drop it, so the
		// whole `if` leaves one value behind, and the OpPop after the `if`
		// statement cleans that up instead.
		if c.lastInstructionIsPop() {
			c.removeLastPop()
		}

		// After the consequence runs, skip over the else branch.
		jumpPos := c.emit(code.OpJump, 9999)

		afterConsequencePos := len(c.currentInstructions())
		c.changeOperand(jumpNotTruthyPos, afterConsequencePos)

		// Every `if` has to leave exactly one value on the stack. With no else
		// written, a false condition would leave nothing, so we act as if the
		// else branch were just `null`.
		if node.Alternative == nil {
			c.emit(code.OpNull)
		} else {
			err := c.Compile(node.Alternative)
			if err != nil {
				return err
			}

			if c.lastInstructionIsPop() {
				c.removeLastPop()
			}
		}

		afterAlternativePos := len(c.currentInstructions())
		c.changeOperand(jumpPos, afterAlternativePos)

	case *ast.LetStatement:
		err := c.Compile(node.Value)
		if err != nil {
			return err
		}

		// The value is on the stack now; store it in the name's slot. At the
		// top level that's a global; inside a function it's a local.
		// Concept: global binding — a variable stored in a fixed-size globals array, looked up by index.
		// Concept: local binding — a variable that lives in the stack slots reserved for the current call.
		symbol := c.symbolTable.Define(node.Name.Value)
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(node.Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Value)
		}

		c.loadSymbol(symbol)

	case *ast.BlockStatement:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.PrefixExpression:
		err := c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "!":
			c.emit(code.OpBang)
		case "-":
			c.emit(code.OpMinus)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.IntegerLiteral:
		// The number itself doesn't go in the instruction. We stash it in the
		// constant pool and emit an instruction that points at it.
		integer := &object.Integer{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(integer))

	case *ast.StringLiteral:
		// Strings never change, so like integers they're stored once in the
		// pool and loaded by index.
		// Concept: constant pool — a side list of values (numbers, strings, functions) that instructions refer to by index.
		str := &object.String{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(str))

	case *ast.ArrayLiteral:
		// Push every element in order, then tell the VM how many to collect.
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpArray, len(node.Elements))

	case *ast.HashLiteral:
		keys := []ast.Expression{}
		for k := range node.Pairs {
			keys = append(keys, k)
		}

		// Go hands back map entries in a random order each run. Without
		// sorting, the same hash could compile to different bytecode every
		// time and tests would fail at random.
		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		for _, k := range keys {
			err := c.Compile(k)
			if err != nil {
				return err
			}
			err = c.Compile(node.Pairs[k])
			if err != nil {
				return err
			}
		}

		c.emit(code.OpHash, len(node.Pairs)*2)

	case *ast.IndexExpression:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}

		err = c.Compile(node.Index)
		if err != nil {
			return err
		}

		c.emit(code.OpIndex)

	case *ast.Boolean:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.FunctionLiteral:
		c.enterScope()

		// Define the parameters first, so they get locals 0..n-1. The VM
		// relies on that: it leaves the arguments in exactly those slots.
		for _, p := range node.Parameters {
			c.symbolTable.Define(p.Value)
		}

		err := c.Compile(node.Body)
		if err != nil {
			return err
		}

		// A function hands back its last expression even without `return`.
		// That expression ends in OpPop, so we swap it for OpReturnValue.
		// Concept: implicit return — the last expression in a function body is its return value.
		if c.lastInstructionIs(code.OpPop) {
			c.replaceLastPopWithReturn()
		}
		// Nothing to return (an empty body, or one ending in a `let`), so
		// return null.
		if !c.lastInstructionIs(code.OpReturnValue) {
			c.emit(code.OpReturn)
		}

		// Read this before leaveScope throws the function's table away.
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		compiledFn := &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: len(node.Parameters),
		}
		c.emit(code.OpConstant, c.addConstant(compiledFn))

	case *ast.ReturnStatement:
		err := c.Compile(node.ReturnValue)
		if err != nil {
			return err
		}

		c.emit(code.OpReturnValue)

	case *ast.CallExpression:
		err := c.Compile(node.Function)
		if err != nil {
			return err
		}

		// Arguments go on the stack right above the function, in order.
		for _, a := range node.Arguments {
			err := c.Compile(a)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpCall, len(node.Arguments))
	}

	return nil
}

// addConstant stores obj in the constant pool and returns its index.
func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

// emit encodes one instruction, appends it to the current scope and returns
// where it starts. That position is what we need later to patch a jump.
func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)

	return pos
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	c.scopes[c.scopeIndex].instructions = append(c.currentInstructions(), ins...)
	return posNewInstruction
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	// An empty scope has a zero-value lastInstruction, which would look like
	// OpConstant (opcode 0). Check the length so we don't get fooled.
	if len(c.currentInstructions()) == 0 {
		return false
	}

	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) lastInstructionIsPop() bool {
	return c.lastInstructionIs(code.OpPop)
}

// removeLastPop chops the trailing OpPop off the instructions. The instruction
// before it becomes the last one again, so a later check doesn't see a stale OpPop.
func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction

	old := c.currentInstructions()
	c.scopes[c.scopeIndex].instructions = old[:last.Position]
	c.scopes[c.scopeIndex].lastInstruction = previous
}

// replaceLastPopWithReturn turns the trailing OpPop into OpReturnValue. Both
// are one byte, so we can overwrite it in place.
func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))

	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}

// replaceInstruction overwrites bytes in place. It's only safe when the new
// instruction is the same length as the old one, which is true for patching operands.
func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	ins := c.currentInstructions()

	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

// changeOperand rebuilds the instruction at opPos with a new operand.
func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.currentInstructions()[opPos])
	newInstruction := code.Make(op, operand)

	c.replaceInstruction(opPos, newInstruction)
}

// loadSymbol emits the instruction that pushes a name's value, which depends
// on where the name lives.
func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, s.Index)
	}
}

// enterScope starts a fresh instruction buffer and a fresh symbol table for a
// function body. Names defined inside become locals of that function.
func (c *Compiler) enterScope() {
	scope := CompilationScope{
		instructions:        code.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++

	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

// leaveScope drops the innermost buffer and symbol table, and hands back what
// was compiled into the buffer.
func (c *Compiler) leaveScope() code.Instructions {
	instructions := c.currentInstructions()

	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--

	c.symbolTable = c.symbolTable.Outer

	return instructions
}

// Bytecode is what the compiler hands to the VM: the instructions plus the
// constant pool they point into.
type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

// Bytecode returns the compiled program so far.
func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
	}
}
