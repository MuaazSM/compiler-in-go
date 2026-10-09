package vm

import (
	"fmt"
	"monkey/ast"
	"monkey/code"
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
		{"-5", -5},
		{"-10", -10},
		{"-50 + 100 + -50", 0},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	runVmTests(t, tests)
}

func TestBooleanExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"(1 < 2) == true", true},
		{"(1 < 2) == false", false},
		{"(1 > 2) == true", false},
		{"(1 > 2) == false", true},
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
		{"-(5 + 10 * 2) == -25", true},
		{"!(1 < 2) == false", true},
		{"!(if (false) { 5; })", true},
	}

	runVmTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []vmTestCase{
		{"if (true) { 10 }", 10},
		{"if (true) { 10 } else { 20 }", 10},
		{"if (false) { 10 } else { 20 } ", 20},
		{"if (1) { 10 }", 10},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 < 2) { 10 } else { 20 }", 10},
		{"if (1 > 2) { 10 } else { 20 }", 20},
		{"if (1 > 2) { 10 }", Null},
		{"if (false) { 10 }", Null},
		{"if ((if (false) { 10 })) { 10 } else { 20 }", 20},
	}

	runVmTests(t, tests)
}

func TestGlobalLetStatements(t *testing.T) {
	tests := []vmTestCase{
		{"let one = 1; one", 1},
		{"let one = 1; let two = 2; one + two", 3},
		{"let one = 1; let two = one + one; one + two", 3},
	}

	runVmTests(t, tests)
}

func TestGlobalsPersistAcrossRuns(t *testing.T) {
	symbolTable := compiler.NewSymbolTable()
	constants := []object.Object{}
	globals := make([]object.Object, GlobalsSize)

	var last object.Object
	for _, line := range []string{"let a = 5;", "let b = a * 2;", "a + b"} {
		comp := compiler.NewWithState(symbolTable, constants)
		if err := comp.Compile(parse(line)); err != nil {
			t.Fatalf("compiler error on %q: %s", line, err)
		}
		constants = comp.Bytecode().Constants

		machine := NewWithGlobalsStore(comp.Bytecode(), globals)
		if err := machine.Run(); err != nil {
			t.Fatalf("vm error on %q: %s", line, err)
		}
		last = machine.LastPoppedStackElem()
	}

	testExpectedObject(t, 15, last)
}

func TestReadingUnsetGlobalIsAnError(t *testing.T) {
	// The REPL can end up here: `let c = 1; oops` fails to compile after `c`
	// was named, so a later line reads a slot nothing ever wrote to.
	bytecode := &compiler.Bytecode{
		Instructions: code.Make(code.OpGetGlobal, 0),
	}

	err := NewWithGlobalsStore(bytecode, make([]object.Object, GlobalsSize)).Run()
	if err == nil {
		t.Fatalf("expected an error for an unset global, got none")
	}
}

func TestStringExpressions(t *testing.T) {
	tests := []vmTestCase{
		{`"monkey"`, "monkey"},
		{`"mon" + "key"`, "monkey"},
		{`"mon" + "key" + "banana"`, "monkeybanana"},
	}

	runVmTests(t, tests)
}

func TestStringOperatorsOtherThanPlusAreErrors(t *testing.T) {
	comp := compiler.New()
	if err := comp.Compile(parse(`"a" - "b"`)); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	if err := New(comp.Bytecode()).Run(); err == nil {
		t.Fatalf("expected an error for string subtraction, got none")
	}
}

func TestArrayLiterals(t *testing.T) {
	tests := []vmTestCase{
		{"[]", []int{}},
		{"[1, 2, 3]", []int{1, 2, 3}},
		{"[1 + 2, 3 * 4, 5 + 6]", []int{3, 12, 11}},
	}

	runVmTests(t, tests)
}

func TestHashLiterals(t *testing.T) {
	tests := []vmTestCase{
		{"{}", map[object.HashKey]int64{}},
		{
			"{1: 2, 2: 3}",
			map[object.HashKey]int64{
				(&object.Integer{Value: 1}).HashKey(): 2,
				(&object.Integer{Value: 2}).HashKey(): 3,
			},
		},
		{
			"{1 + 1: 2 * 2, 3 + 3: 4 * 4}",
			map[object.HashKey]int64{
				(&object.Integer{Value: 2}).HashKey(): 4,
				(&object.Integer{Value: 6}).HashKey(): 16,
			},
		},
	}

	runVmTests(t, tests)
}

func TestUnhashableHashKeyIsAnError(t *testing.T) {
	comp := compiler.New()
	if err := comp.Compile(parse("{[1]: 2}")); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	if err := New(comp.Bytecode()).Run(); err == nil {
		t.Fatalf("expected an error for an array used as a hash key, got none")
	}
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
	case bool:
		err := testBooleanObject(expected, actual)
		if err != nil {
			t.Errorf("testBooleanObject failed: %s", err)
		}
	case string:
		err := testStringObject(expected, actual)
		if err != nil {
			t.Errorf("testStringObject failed: %s", err)
		}
	case []int:
		array, ok := actual.(*object.Array)
		if !ok {
			t.Errorf("object not Array: %T (%+v)", actual, actual)
			return
		}

		if len(array.Elements) != len(expected) {
			t.Errorf("wrong num of elements. want=%d, got=%d",
				len(expected), len(array.Elements))
			return
		}

		for i, expectedElem := range expected {
			err := testIntegerObject(int64(expectedElem), array.Elements[i])
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}
	case map[object.HashKey]int64:
		hash, ok := actual.(*object.Hash)
		if !ok {
			t.Errorf("object is not Hash. got=%T (%+v)", actual, actual)
			return
		}

		if len(hash.Pairs) != len(expected) {
			t.Errorf("hash has wrong number of Pairs. want=%d, got=%d",
				len(expected), len(hash.Pairs))
			return
		}

		for expectedKey, expectedValue := range expected {
			pair, ok := hash.Pairs[expectedKey]
			if !ok {
				t.Errorf("no pair for given key in Pairs")
				continue
			}

			err := testIntegerObject(expectedValue, pair.Value)
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}
	case *object.Null:
		if actual != Null {
			t.Errorf("object is not Null: %T (%+v)", actual, actual)
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

func testBooleanObject(expected bool, actual object.Object) error {
	result, ok := actual.(*object.Boolean)
	if !ok {
		return fmt.Errorf("object is not Boolean. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%t, want=%t",
			result.Value, expected)
	}

	return nil
}

func testStringObject(expected string, actual object.Object) error {
	result, ok := actual.(*object.String)
	if !ok {
		return fmt.Errorf("object is not String. got=%T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("object has wrong value. got=%q, want=%q",
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

func TestComparingMixedTypes(t *testing.T) {
	// `1 == true` must not be treated as an integer compare just because one
	// side is an integer; it falls through to the pointer compare and is false.
	tests := []vmTestCase{
		{"1 == true", false},
		{"1 != true", true},
	}

	runVmTests(t, tests)
}

func TestMinusOnNonInteger(t *testing.T) {
	comp := compiler.New()
	if err := comp.Compile(parse("-true")); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	err := New(comp.Bytecode()).Run()
	if err == nil {
		t.Fatalf("expected an error for -true, got none")
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
