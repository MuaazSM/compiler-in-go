// Package vm runs the bytecode the compiler produces.
package vm

import (
	"fmt"
	"monkey/code"
	"monkey/compiler"
	"monkey/object"
)

// StackSize caps how many values can be on the stack at once.
const StackSize = 2048

// VM executes one compiled program.
// Concept: stack machine — a VM that does all its work by pushing and popping values on one stack.
type VM struct {
	constants    []object.Object
	instructions code.Instructions

	stack []object.Object
	// sp is the slot the next push will fill, so the top value lives at sp-1.
	// Concept: stack pointer (sp) — points at the next free slot; the top value is at sp-1.
	sp int
}

// New sets up a VM for the given bytecode with an empty stack.
func New(bytecode *compiler.Bytecode) *VM {
	return &VM{
		instructions: bytecode.Instructions,
		constants:    bytecode.Constants,

		stack: make([]object.Object, StackSize),
		sp:    0,
	}
}

// StackTop returns the value on top of the stack, or nil if it's empty.
func (vm *VM) StackTop() object.Object {
	if vm.sp == 0 {
		return nil
	}
	return vm.stack[vm.sp-1]
}

// Run executes the instructions from start to finish.
// Concept: fetch-decode-execute — read an instruction, figure out what it means, do it, repeat.
func (vm *VM) Run() error {
	for ip := 0; ip < len(vm.instructions); ip++ {
		op := code.Opcode(vm.instructions[ip])

		switch op {
		case code.OpConstant:
			constIndex := code.ReadUint16(vm.instructions[ip+1:])
			// Jump over the two operand bytes we just read, or the loop
			// would try to run them as opcodes.
			ip += 2

			err := vm.push(vm.constants[constIndex])
			if err != nil {
				return err
			}

		case code.OpAdd:
			// The right operand was pushed last, so it comes off first.
			right := vm.pop()
			left := vm.pop()

			leftInt, ok1 := left.(*object.Integer)
			rightInt, ok2 := right.(*object.Integer)
			if !ok1 || !ok2 {
				return fmt.Errorf("unsupported types for +: %s and %s",
					left.Type(), right.Type())
			}

			err := vm.push(&object.Integer{Value: leftInt.Value + rightInt.Value})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (vm *VM) push(o object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}

	vm.stack[vm.sp] = o
	vm.sp++

	return nil
}

// pop only moves sp down. The old value stays in its slot, which lets us peek
// at the last popped value later on.
func (vm *VM) pop() object.Object {
	o := vm.stack[vm.sp-1]
	vm.sp--
	return o
}
