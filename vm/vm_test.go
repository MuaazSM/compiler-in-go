package vm

import (
	"fmt"
	"monkey/ast"
	"monkey/compiler"
	"monkey/lexer"
	"monkey/object"
	"monkey/parser"
	"testing"
)

type vmTestCase struct {
	input    string
	expected interface{}
}

func TestIntegerArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1; 2", 2},
		{"1 - 2", -1},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"5 * (2 + 10)", 60},
	}

	runVmTests(t, tests)
}

func runVmTests(t *testing.T, tests []vmTestCase) {
	t.Helper()

	for _, tt := range tests {
		program := parse(tt.input)

		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err != nil {
			t.Fatalf("vm error: %s", err)
		}

		// Every expression statement ends in OpPop, so the result has already
		// left the stack by the time Run returns.
		stackElem := vm.LastPoppedStackElem()

		testExpectedObject(t, tt.expected, stackElem)
	}
}

func parse(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}

func testExpectedObject(t *testing.T, expected interface{}, actual object.Object) {
	t.Helper()

	switch expected := expected.(type) {
	case int:
		err := testIntegerObject(int64(expected), actual)
		if err != nil {
			t.Errorf("testIntegerObject failed: %s", err)
		}
	default:
		t.Errorf("unsupported expected type %T", expected)
	}
}

func testIntegerObject(expected int64, actual object.Object) error {
	result, ok := actual.(*object.Integer)
	if !ok {
		return fmt.Errorf("object is not Integer. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%d, want=%d",
			result.Value, expected)
	}

	return nil
}

func TestStackOverflow(t *testing.T) {
	vm := New(&compiler.Bytecode{})

	// Fill every slot; the next push has nowhere to go.
	for i := 0; i < StackSize; i++ {
		if err := vm.push(&object.Integer{Value: int64(i)}); err != nil {
			t.Fatalf("push %d failed early: %s", i, err)
		}
	}

	if err := vm.push(&object.Integer{Value: 0}); err == nil {
		t.Fatalf("expected a stack overflow error, got none")
	}
}

func TestDivisionByZero(t *testing.T) {
	comp := compiler.New()
	if err := comp.Compile(parse("1 / 0")); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	err := New(comp.Bytecode()).Run()
	if err == nil {
		t.Fatalf("expected a division by zero error, got none")
	}
}
