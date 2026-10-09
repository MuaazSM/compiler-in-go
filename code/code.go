// Package code defines the bytecode that the compiler writes and the VM reads.
package code

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Instructions is a flat run of bytecode: opcodes and their operands, back to back.
// Concept: bytecode — a compact list of bytes that tells the VM what to do, step by step.
type Instructions []byte

// String disassembles the instructions into one readable line per instruction,
// each prefixed with its byte offset. Tests lean on this to show readable diffs.
// Concept: disassembler — turns raw bytecode back into readable text, mainly for tests and debugging.
func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR: %s\n", err)
			// Step over the bad byte so we keep going instead of looping forever.
			i++
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])
		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstruction(def, operands))

		i += 1 + read
	}

	return out.String()
}

func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {
	operandCount := len(def.OperandWidths)

	if len(operands) != operandCount {
		return fmt.Sprintf("ERROR: operand len %d does not match defined %d\n",
			len(operands), operandCount)
	}

	switch operandCount {
	case 0:
		return def.Name
	case 1:
		return fmt.Sprintf("%s %d", def.Name, operands[0])
	}

	return fmt.Sprintf("ERROR: unhandled operandCount for %s\n", def.Name)
}

// Opcode is the first byte of every instruction.
// Concept: opcode — the first byte of an instruction; it says which operation to run.
type Opcode byte

const (
	// OpConstant pushes a value from the constant pool. Its operand is the pool index.
	// Concept: constant pool — a side list of values (numbers, strings, functions) that instructions refer to by index.
	OpConstant Opcode = iota
	// OpAdd pops the top two values and pushes their sum. It has no operands.
	OpAdd
	// OpPop throws away the top of the stack. It ends every expression statement.
	OpPop
)

// Definition describes an opcode: a readable name and how many bytes each operand takes.
// Concept: operand — extra bytes after an opcode that give the operation its input (like an index).
type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	// Two bytes lets us index up to 65536 constants.
	OpConstant: {"OpConstant", []int{2}},
	OpAdd:      {"OpAdd", []int{}},
	OpPop:      {"OpPop", []int{}},
}

// Lookup returns the definition for an opcode byte, or an error if we don't know it.
func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}

	return def, nil
}

// Make encodes one instruction: the opcode byte followed by its operands.
// An unknown opcode gives back an empty slice.
func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	instructionLen := 1
	for _, w := range def.OperandWidths {
		instructionLen += w
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)

	offset := 1
	for i, o := range operands {
		width := def.OperandWidths[i]
		switch width {
		case 2:
			// Concept: big-endian — the most significant byte comes first in memory.
			binary.BigEndian.PutUint16(instruction[offset:], uint16(o))
		}
		offset += width
	}

	return instruction
}

// ReadOperands is the reverse of Make. It decodes the operands that follow an opcode
// and reports how many bytes it read, so the caller knows where the next instruction starts.
func ReadOperands(def *Definition, ins Instructions) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 2:
			operands[i] = int(ReadUint16(ins[offset:]))
		}

		offset += width
	}

	return operands, offset
}

// ReadUint16 reads a big-endian two-byte operand. The VM calls it directly in its hot loop
// to skip the slice allocation that ReadOperands would do.
func ReadUint16(ins Instructions) uint16 {
	return binary.BigEndian.Uint16(ins)
}
