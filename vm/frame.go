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
}

// NewFrame sets up a frame that will start at the first instruction of fn.
func NewFrame(fn *object.CompiledFunction) *Frame {
	return &Frame{fn: fn, ip: -1}
}

// Instructions returns the bytecode this frame is running.
func (f *Frame) Instructions() code.Instructions {
	return f.fn.Instructions
}
