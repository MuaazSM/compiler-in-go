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

// There is only ever one true and one false. Every boolean the VM pushes is one
// of these two, so checking whether two booleans are equal is a pointer compare.
var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}

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

// LastPoppedStackElem returns the value the last OpPop removed. Popping only
// moves sp down, so that value is still sitting in the slot sp now points at.
// Concept: stack pointer (sp) — points at the next free slot; the top value is at sp-1.
func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
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

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv:
			err := vm.executeBinaryOperation(op)
			if err != nil {
				return err
			}

		case code.OpTrue:
			err := vm.push(True)
			if err != nil {
				return err
			}

		case code.OpFalse:
			err := vm.push(False)
			if err != nil {
				return err
			}

		case code.OpPop:
			vm.pop()
		}
	}

	return nil
}

// executeBinaryOperation pops two operands and pushes the result of op.
// The right operand was pushed last, so it comes off first.
func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeBinaryIntegerOperation(op, left, right)
	}

	return fmt.Errorf("unsupported types for binary operation: %s %s",
		left.Type(), right.Type())
}

func (vm *VM) executeBinaryIntegerOperation(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	var result int64

	switch op {
	case code.OpAdd:
		result = leftValue + rightValue
	case code.OpSub:
		result = leftValue - rightValue
	case code.OpMul:
		result = leftValue * rightValue
	case code.OpDiv:
		// Go panics on integer division by zero, which would take the whole
		// REPL down. Report it as a normal error instead.
		if rightValue == 0 {
			return fmt.Errorf("division by zero")
		}
		result = leftValue / rightValue
	default:
		return fmt.Errorf("unknown integer operator: %d", op)
	}

	return vm.push(&object.Integer{Value: result})
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return True
	}
	return False
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
