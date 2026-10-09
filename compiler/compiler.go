// Package compiler turns a parsed Monkey program into bytecode for the VM.
package compiler

import (
	"fmt"
	"monkey/ast"
	"monkey/code"
	"monkey/object"
)

// EmittedInstruction remembers an opcode we emitted and where it starts.
type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

// Compiler walks the AST once and appends instructions as it goes.
type Compiler struct {
	instructions code.Instructions
	constants    []object.Object

	// The last two instructions we emitted. We sometimes need to take back the
	// last one, and then the one before it becomes "last" again.
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction

	symbolTable *SymbolTable
}

// New returns an empty compiler.
func New() *Compiler {
	return &Compiler{
		instructions: code.Instructions{},
		constants:    []object.Object{},
		symbolTable:  NewSymbolTable(),
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

		afterConsequencePos := len(c.instructions)
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

		afterAlternativePos := len(c.instructions)
		c.changeOperand(jumpPos, afterAlternativePos)

	case *ast.LetStatement:
		err := c.Compile(node.Value)
		if err != nil {
			return err
		}

		// The value is on the stack now; store it in the name's global slot.
		// Concept: global binding — a variable stored in a fixed-size globals array, looked up by index.
		symbol := c.symbolTable.Define(node.Name.Value)
		c.emit(code.OpSetGlobal, symbol.Index)

	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(node.Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", node.Value)
		}

		c.emit(code.OpGetGlobal, symbol.Index)

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

	case *ast.Boolean:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}
	}

	return nil
}

// addConstant stores obj in the constant pool and returns its index.
func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

// emit encodes one instruction, appends it and returns where it starts.
// That start position is what we need later to go back and patch a jump.
func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)

	return pos
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.previousInstruction = previous
	c.lastInstruction = last
}

func (c *Compiler) lastInstructionIsPop() bool {
	return c.lastInstruction.Opcode == code.OpPop
}

// removeLastPop chops the trailing OpPop off the instructions. The instruction
// before it becomes the last one again, so a later check doesn't see a stale OpPop.
func (c *Compiler) removeLastPop() {
	c.instructions = c.instructions[:c.lastInstruction.Position]
	c.lastInstruction = c.previousInstruction
}

// replaceInstruction overwrites bytes in place. It's only safe when the new
// instruction is the same length as the old one, which is true for patching operands.
func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	for i := 0; i < len(newInstruction); i++ {
		c.instructions[pos+i] = newInstruction[i]
	}
}

// changeOperand rebuilds the instruction at opPos with a new operand.
func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.instructions[opPos])
	newInstruction := code.Make(op, operand)

	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.instructions)
	c.instructions = append(c.instructions, ins...)
	return posNewInstruction
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
		Instructions: c.instructions,
		Constants:    c.constants,
	}
}
