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

// MaxFrames caps how deep calls can nest.
const MaxFrames = 1024

// There is only ever one true and one false. Every boolean the VM pushes is one
// of these two, so checking whether two booleans are equal is a pointer compare.
var True = &object.Boolean{Value: true}
var False = &object.Boolean{Value: false}

// Null is the only null value, for the same reason.
var Null = &object.Null{}

// VM executes one compiled program.
// Concept: stack machine — a VM that does all its work by pushing and popping values on one stack.
type VM struct {
	constants []object.Object

	stack []object.Object
	// sp is the slot the next push will fill, so the top value lives at sp-1.
	// Concept: stack pointer (sp) — points at the next free slot; the top value is at sp-1.
	sp int

	globals []object.Object

	// frames[framesIndex-1] is the call that's running right now.
	frames      []*Frame
	framesIndex int
}

// New sets up a VM for the given bytecode with an empty stack.
func New(bytecode *compiler.Bytecode) *VM {
	// The top-level program runs inside a frame too, as if it were the body
	// of a function nobody called. That way the run loop always reads from
	// "the current frame" and never needs a separate path for the main code.
	mainFn := &object.CompiledFunction{Instructions: bytecode.Instructions}
	mainFrame := NewFrame(mainFn, 0)

	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants: bytecode.Constants,

		stack: make([]object.Object, StackSize),
		sp:    0,

		globals: make([]object.Object, GlobalsSize),

		frames:      frames,
		framesIndex: 1,
	}
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) pushFrame(f *Frame) error {
	if vm.framesIndex >= MaxFrames {
		return fmt.Errorf("stack overflow: calls nested deeper than %d", MaxFrames)
	}

	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
	return nil
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
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
	var ip int
	var ins code.Instructions
	var op code.Opcode

	for vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++

		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()
		op = code.Opcode(ins[ip])

		switch op {
		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			// Jump over the two operand bytes we just read, or the loop
			// would try to run them as opcodes.
			vm.currentFrame().ip += 2

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
			pos := int(code.ReadUint16(ins[ip+1:]))
			// The loop adds 1 to ip right after this, so we land one byte
			// early on purpose and the next fetch reads the target itself.
			// Concept: instruction pointer (ip) — where in the bytecode the VM is reading right now.
			vm.currentFrame().ip = pos - 1

		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(ins[ip+1:]))
			// Skip the operand bytes. If we don't jump, we carry on with the
			// instruction right after this one.
			vm.currentFrame().ip += 2

			condition := vm.pop()
			if !isTruthy(condition) {
				vm.currentFrame().ip = pos - 1
			}

		case code.OpSetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

			vm.globals[globalIndex] = vm.pop()

		case code.OpGetGlobal:
			globalIndex := code.ReadUint16(ins[ip+1:])
			vm.currentFrame().ip += 2

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

		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			array := vm.buildArray(vm.sp-numElements, vm.sp)
			// Drop the elements now that they live inside the array.
			vm.sp = vm.sp - numElements

			err := vm.push(array)
			if err != nil {
				return err
			}

		case code.OpHash:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			hash, err := vm.buildHash(vm.sp-numElements, vm.sp)
			if err != nil {
				return err
			}
			vm.sp = vm.sp - numElements

			err = vm.push(hash)
			if err != nil {
				return err
			}

		case code.OpIndex:
			index := vm.pop()
			left := vm.pop()

			err := vm.executeIndexExpression(left, index)
			if err != nil {
				return err
			}

		case code.OpCall:
			numArgs := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			err := vm.callFunction(int(numArgs))
			if err != nil {
				return err
			}

		case code.OpReturnValue:
			returnValue := vm.pop()

			// A `return` at the top level has no caller to go back to. Treat
			// it as "stop here": the value is popped, so it's the result.
			if vm.framesIndex == 1 {
				return nil
			}

			// The callee sits one slot below basePointer. Dropping sp to
			// basePointer-1 throws away the locals and the callee in one go,
			// leaving the stack exactly as it was before the call.
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err := vm.push(returnValue)
			if err != nil {
				return err
			}

		case code.OpReturn:
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1

			err := vm.push(Null)
			if err != nil {
				return err
			}

		case code.OpSetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			frame := vm.currentFrame()
			vm.stack[frame.basePointer+int(localIndex)] = vm.pop()

		case code.OpGetLocal:
			localIndex := code.ReadUint8(ins[ip+1:])
			vm.currentFrame().ip += 1

			frame := vm.currentFrame()
			err := vm.push(vm.stack[frame.basePointer+int(localIndex)])
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

// buildArray copies stack[startIndex:endIndex] into a new array. The elements
// were pushed in source order, so the lowest slot is the first element.
func (vm *VM) buildArray(startIndex, endIndex int) object.Object {
	elements := make([]object.Object, endIndex-startIndex)

	for i := startIndex; i < endIndex; i++ {
		elements[i-startIndex] = vm.stack[i]
	}

	return &object.Array{Elements: elements}
}

// buildHash walks stack[startIndex:endIndex] two slots at a time: a key,
// then its value. Keys have to be hashable (integers, booleans, strings).
func (vm *VM) buildHash(startIndex, endIndex int) (object.Object, error) {
	hashedPairs := make(map[object.HashKey]object.HashPair)

	for i := startIndex; i < endIndex; i += 2 {
		key := vm.stack[i]
		value := vm.stack[i+1]

		pair := object.HashPair{Key: key, Value: value}

		hashKey, ok := key.(object.Hashable)
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
		}

		hashedPairs[hashKey.HashKey()] = pair
	}

	return &object.Hash{Pairs: hashedPairs}, nil
}

// callFunction starts a call to the function sitting below its numArgs
// arguments on the stack.
//
// While a function runs, the stack looks like this. Everything from
// basePointer up belongs to the call:
//
//	           ┌───────────────┐
//	sp ──────▶ │               │
//	           │ local k       │
//	           │ ...           │
//	           │ arg n-1       │
//	           │ arg 0         │ ◀── basePointer
//	           │ callee (fn)   │
//	           │ caller's data │
//	           └───────────────┘
//
// Concept: calling convention — the agreed stack layout for callee, arguments and locals during a call.
func (vm *VM) callFunction(numArgs int) error {
	fn, ok := vm.stack[vm.sp-1-numArgs].(*object.CompiledFunction)
	if !ok {
		return fmt.Errorf("calling non-function")
	}

	if numArgs != fn.NumParameters {
		return fmt.Errorf("wrong number of arguments: want=%d, got=%d",
			fn.NumParameters, numArgs)
	}

	// The arguments were pushed right before the call, so they already sit
	// in the first slots above the callee. Starting the locals there means
	// argument i *is* local i. Nothing gets copied; we just point the frame
	// at the slots the caller filled in.
	frame := NewFrame(fn, vm.sp-numArgs)
	err := vm.pushFrame(frame)
	if err != nil {
		return err
	}

	// Bump sp past the remaining local slots so pushes during the call
	// can't land on top of them. NumLocals already counts the parameters.
	if frame.basePointer+fn.NumLocals > StackSize {
		return fmt.Errorf("stack overflow")
	}
	vm.sp = frame.basePointer + fn.NumLocals

	return nil
}

// executeIndexExpression pushes left[index]. A missing element gives null
// rather than an error, the same way the tree-walking evaluator behaves. Asking
// for something that can't exist (a bad type) is still an error.
func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return vm.executeArrayIndex(left, index)
	case left.Type() == object.HASH_OBJ:
		return vm.executeHashIndex(left, index)
	default:
		return fmt.Errorf("index operator not supported: %s", left.Type())
	}
}

func (vm *VM) executeArrayIndex(array, index object.Object) error {
	arrayObject := array.(*object.Array)
	i := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	// Out of range in either direction is just "nothing there".
	if i < 0 || i > max {
		return vm.push(Null)
	}

	return vm.push(arrayObject.Elements[i])
}

func (vm *VM) executeHashIndex(hash, index object.Object) error {
	hashObject := hash.(*object.Hash)

	key, ok := index.(object.Hashable)
	if !ok {
		return fmt.Errorf("unusable as hash key: %s", index.Type())
	}

	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return vm.push(Null)
	}

	return vm.push(pair.Value)
}

// executeBinaryOperation pops two operands and pushes the result of op.
// The right operand was pushed last, so it comes off first.
func (vm *VM) executeBinaryOperation(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return vm.executeBinaryIntegerOperation(op, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return vm.executeBinaryStringOperation(op, left, right)
	default:
		return fmt.Errorf("unsupported types for binary operation: %s %s",
			left.Type(), right.Type())
	}
}

// executeBinaryStringOperation handles string + string. Nothing else makes
// sense for strings, so any other operator is an error.
func (vm *VM) executeBinaryStringOperation(op code.Opcode, left, right object.Object) error {
	if op != code.OpAdd {
		return fmt.Errorf("unknown string operator: %d", op)
	}

	leftValue := left.(*object.String).Value
	rightValue := right.(*object.String).Value

	return vm.push(&object.String{Value: leftValue + rightValue})
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
