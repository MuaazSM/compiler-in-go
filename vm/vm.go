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

// GlobalsSize is how many global slots there are. The global index travels in a
// 2-byte operand, and 65536 is how many different values two bytes can hold,
// so every index the compiler can emit has a slot.
const GlobalsSize = 65536

// There is only ever one true and one false. Every boolean the VM pushes is one
// of these two, so checking whether two booleans are equal is a pointer compare.
var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}

// Null is the only null value, for the same reason.
var Null = &object.Null{}

// VM executes one compiled program.
// Concept: stack machine — a VM that does all its work by pushing and popping values on one stack.
type VM struct {
	constants    []object.Object
	instructions code.Instructions

	stack []object.Object
	// sp is the slot the next push will fill, so the top value lives at sp-1.
	// Concept: stack pointer (sp) — points at the next free slot; the top value is at sp-1.
	sp int

	globals []object.Object
}

// New sets up a VM for the given bytecode with an empty stack.
func New(bytecode *compiler.Bytecode) *VM {
	return &VM{
		instructions: bytecode.Instructions,
		constants:    bytecode.Constants,

		stack: make([]object.Object, StackSize),
		sp:    0,

		globals: make([]object.Object, GlobalsSize),
	}
}

// NewWithGlobalsStore is New, but with a globals slice the caller keeps. Passing
// the same slice to the next VM lets it see globals set by the previous one.
func NewWithGlobalsStore(bytecode *compiler.Bytecode, s []object.Object) *VM {
	vm := New(bytecode)
	vm.globals = s
	return vm
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

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan:
			err := vm.executeComparison(op)
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

		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil {
				return err
			}

		case code.OpMinus:
			err := vm.executeMinusOperator()
			if err != nil {
				return err
			}

		case code.OpJump:
			pos := int(code.ReadUint16(vm.instructions[ip+1:]))
			// The loop adds 1 to ip right after this, so we land one byte
			// early on purpose and the next fetch reads the target itself.
			// Concept: instruction pointer (ip) — where in the bytecode the VM is reading right now.
			ip = pos - 1

		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(vm.instructions[ip+1:]))
			// Skip the operand bytes. If we don't jump, we carry on with the
			// instruction right after this one.
			ip += 2

			condition := vm.pop()
			if !isTruthy(condition) {
				ip = pos - 1
			}

		case code.OpSetGlobal:
			globalIndex := code.ReadUint16(vm.instructions[ip+1:])
			ip += 2

			vm.globals[globalIndex] = vm.pop()

		case code.OpGetGlobal:
			globalIndex := code.ReadUint16(vm.instructions[ip+1:])
			ip += 2

			// A compiled program always sets a global before reading it. Only
			// the REPL can get here with an empty slot, when a line named a
			// global but then failed to compile, so it never stored a value.
			value := vm.globals[globalIndex]
			if value == nil {
				return fmt.Errorf("global %d was never set", globalIndex)
			}

			err := vm.push(value)
			if err != nil {
				return err
			}

		case code.OpNull:
			err := vm.push(Null)
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

// executeComparison pops two values and pushes True or False.
func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	// Both sides have to be integers here. With || a mixed pair like
	// `1 == true` would crash on the type assertion inside.
	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparison(op, left, right)
	}

	// Anything else can only be checked for (in)equality. Booleans are the
	// shared True/False singletons, so comparing pointers is enough.
	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(right == left))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(right != left))
	default:
		return fmt.Errorf("unknown operator: %d (%s %s)",
			op, left.Type(), right.Type())
	}
}

func (vm *VM) executeIntegerComparison(op code.Opcode, left, right object.Object) error {
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue == leftValue))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(rightValue != leftValue))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftValue > rightValue))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

// executeBangOperator pushes the opposite of the operand's truthiness.
// Concept: truthiness — everything counts as true except false and null.
func (vm *VM) executeBangOperator() error {
	operand := vm.pop()

	switch operand {
	case True:
		return vm.push(False)
	case False:
		return vm.push(True)
	case Null:
		return vm.push(True)
	default:
		// Any other value (like 5) is truthy, so its opposite is false.
		return vm.push(False)
	}
}

func (vm *VM) executeMinusOperator() error {
	operand := vm.pop()

	if operand.Type() != object.INTEGER_OBJ {
		return fmt.Errorf("unsupported type for negation: %s", operand.Type())
	}

	value := operand.(*object.Integer).Value
	return vm.push(&object.Integer{Value: -value})
}

// isTruthy decides which way a conditional jump goes.
// Concept: truthiness — everything counts as true except false and null.
func isTruthy(obj object.Object) bool {
	switch obj := obj.(type) {
	case *object.Boolean:
		return obj.Value
	case *object.Null:
		return false
	default:
		return true
	}
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
