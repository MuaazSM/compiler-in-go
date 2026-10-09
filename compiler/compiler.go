// Package compiler turns a parsed Monkey program into bytecode for the VM.
package compiler

import (
	"fmt"
	"monkey/ast"
	"monkey/code"
	"monkey/object"
)

// Compiler walks the AST once and appends instructions as it goes.
type Compiler struct {
	instructions code.Instructions
	constants    []object.Object
}

// New returns an empty compiler.
func New() *Compiler {
	return &Compiler{
		instructions: code.Instructions{},
		constants:    []object.Object{},
	}
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
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *ast.IntegerLiteral:
		// The number itself doesn't go in the instruction. We stash it in the
		// constant pool and emit an instruction that points at it.
		integer := &object.Integer{Value: node.Value}
		c.emit(code.OpConstant, c.addConstant(integer))
	}

	return nil
}

// addConstant stores obj in the constant pool and returns its index.
func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

// emit encodes one instruction, appends it and returns where it starts.
// That start position matters later, when we need to go back and patch jumps.
func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	return c.addInstruction(ins)
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
