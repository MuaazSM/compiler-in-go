package vm

import (
	"monkey/code"
	"monkey/object"
)

// Frame is one function call in progress: which function is running and how
// far into its bytecode we are.
// Concept: call frame — the bookkeeping the VM keeps for each active call.
type Frame struct {
	fn *object.CompiledFunction
	// ip starts at -1 because the run loop adds 1 before reading, so the
	// first instruction it reads is at 0.
	ip int

	// basePointer is where this call's locals start on the stack. Local
	// number i lives at stack[basePointer+i].
	// Concept: base pointer — the stack index where the current call's locals begin.
	basePointer int
}

// NewFrame sets up a frame that will start at the first instruction of fn,
// with its locals starting at basePointer.
func NewFrame(fn *object.CompiledFunction, basePointer int) *Frame {
	return &Frame{fn: fn, ip: -1, basePointer: basePointer}
}

// Instructions returns the bytecode this frame is running.
func (f *Frame) Instructions() code.Instructions {
	return f.fn.Instructions
}
