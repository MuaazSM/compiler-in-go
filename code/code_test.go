package code

import "testing"

func TestMake(t *testing.T) {
	tests := []struct {
		op       Opcode
		operands []int
		expected []byte
	}{
		// 65534 is 0xFFFE, so big-endian puts 0xFF first.
		{OpConstant, []int{65534}, []byte{byte(OpConstant), 255, 254}},
		{OpAdd, []int{}, []byte{byte(OpAdd)}},
		{OpPop, []int{}, []byte{byte(OpPop)}},
		{OpSub, []int{}, []byte{byte(OpSub)}},
		{OpTrue, []int{}, []byte{byte(OpTrue)}},
		{OpGreaterThan, []int{}, []byte{byte(OpGreaterThan)}},
		{OpBang, []int{}, []byte{byte(OpBang)}},
		{OpJump, []int{258}, []byte{byte(OpJump), 1, 2}},
		{OpJumpNotTruthy, []int{65535}, []byte{byte(OpJumpNotTruthy), 255, 255}},
		{OpSetGlobal, []int{1}, []byte{byte(OpSetGlobal), 0, 1}},
		{OpGetGlobal, []int{256}, []byte{byte(OpGetGlobal), 1, 0}},
		{OpArray, []int{3}, []byte{byte(OpArray), 0, 3}},
		{OpHash, []int{4}, []byte{byte(OpHash), 0, 4}},
		{OpIndex, []int{}, []byte{byte(OpIndex)}},
		{OpCall, []int{2}, []byte{byte(OpCall), 2}},
		{OpReturnValue, []int{}, []byte{byte(OpReturnValue)}},
		{OpReturn, []int{}, []byte{byte(OpReturn)}},
		{OpGetLocal, []int{255}, []byte{byte(OpGetLocal), 255}},
		{OpGetBuiltin, []int{5}, []byte{byte(OpGetBuiltin), 5}},
		{OpClosure, []int{65534, 255}, []byte{byte(OpClosure), 255, 254, 255}},
		{OpGetFree, []int{3}, []byte{byte(OpGetFree), 3}},
	}

	for _, tt := range tests {
		instruction := Make(tt.op, tt.operands...)

		if len(instruction) != len(tt.expected) {
			t.Errorf("instruction has wrong length. want=%d, got=%d",
				len(tt.expected), len(instruction))
		}

		for i, b := range tt.expected {
			if instruction[i] != tt.expected[i] {
				t.Errorf("wrong byte at pos %d. want=%d, got=%d",
					i, b, instruction[i])
			}
		}
	}
}

func TestInstructionsString(t *testing.T) {
	instructions := []Instructions{
		Make(OpAdd),
		Make(OpGetLocal, 1),
		Make(OpConstant, 2),
		Make(OpConstant, 65535),
		Make(OpClosure, 65535, 255),
	}

	expected := `0000 OpAdd
0001 OpGetLocal 1
0003 OpConstant 2
0006 OpConstant 65535
0009 OpClosure 65535 255
`

	concatted := Instructions{}
	for _, ins := range instructions {
		concatted = append(concatted, ins...)
	}

	if concatted.String() != expected {
		t.Errorf("instructions wrongly formatted.\nwant=%q\ngot=%q",
			expected, concatted.String())
	}
}

func TestReadOperands(t *testing.T) {
	tests := []struct {
		op        Opcode
		operands  []int
		bytesRead int
	}{
		{OpConstant, []int{65535}, 2},
		{OpGetLocal, []int{255}, 1},
		{OpClosure, []int{65535, 255}, 3},
	}

	for _, tt := range tests {
		instruction := Make(tt.op, tt.operands...)

		def, err := Lookup(byte(tt.op))
		if err != nil {
			t.Fatalf("definition not found: %q\n", err)
		}

		// Skip the opcode byte; ReadOperands only sees the operands.
		operandsRead, n := ReadOperands(def, instruction[1:])
		if n != tt.bytesRead {
			t.Fatalf("n wrong. want=%d, got=%d", tt.bytesRead, n)
		}

		for i, want := range tt.operands {
			if operandsRead[i] != want {
				t.Errorf("operand wrong. want=%d, got=%d", want, operandsRead[i])
			}
		}
	}
}
