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

func TestIndexExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"[1, 2, 3][1]", 2},
		{"[1, 2, 3][0 + 2]", 3},
		{"[[1, 1, 1]][0][0]", 1},
		{"[][0]", Null},
		{"[1, 2, 3][99]", Null},
		{"[1][-1]", Null},
		{"{1: 1, 2: 2}[1]", 1},
		{"{1: 1, 2: 2}[2]", 2},
		{"{1: 1}[0]", Null},
		{"{}[0]", Null},
		{`{"one": 1}["one"]`, 1},
		{"{true: 5}[true]", 5},
	}

	runVmTests(t, tests)
}

func TestIndexErrors(t *testing.T) {
	inputs := []string{
		"{1: 1}[[1]]",  // arrays can't be hash keys
		"1[0]",         // integers can't be indexed
		"[1, 2][true]", // arrays need an integer index
	}

	for _, input := range inputs {
		comp := compiler.New()
		if err := comp.Compile(parse(input)); err != nil {
			t.Fatalf("compiler error on %q: %s", input, err)
		}

		if err := New(comp.Bytecode()).Run(); err == nil {
			t.Errorf("expected an error for %q, got none", input)
		}
	}
}

func TestCallingFunctionsWithoutArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let fivePlusTen = fn() { 5 + 10; };
			fivePlusTen();
			`,
			expected: 15,
		},
		{
			input: `
			let one = fn() { 1; };
			let two = fn() { 2; };
			one() + two()
			`,
			expected: 3,
		},
		{
			input: `
			let a = fn() { 1 };
			let b = fn() { a() + 1 };
			let c = fn() { b() + 1 };
			c();
			`,
			expected: 3,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithReturnStatement(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let earlyExit = fn() { return 99; 100; };
			earlyExit();
			`,
			expected: 99,
		},
		{
			input: `
			let earlyExit = fn() { return 99; return 100; };
			earlyExit();
			`,
			expected: 99,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithoutReturnValue(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let noReturn = fn() { };
			noReturn();
			`,
			expected: Null,
		},
		{
			input: `
			let noReturn = fn() { };
			let noReturnTwo = fn() { noReturn(); };
			noReturn();
			noReturnTwo();
			`,
			expected: Null,
		},
	}

	runVmTests(t, tests)
}

func TestFirstClassFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let returnsOne = fn() { 1; };
			let returnsOneReturner = fn() { returnsOne; };
			returnsOneReturner()();
			`,
			expected: 1,
		},
		{
			input: `
			let returnsOneReturner = fn() {
				let returnsOne = fn() { 1; };
				returnsOne;
			};
			returnsOneReturner()();
			`,
			expected: 1,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let one = fn() { let one = 1; one };
			one();
			`,
			expected: 1,
		},
		{
			input: `
			let oneAndTwo = fn() { let one = 1; let two = 2; one + two; };
			oneAndTwo();
			`,
			expected: 3,
		},
		{
			input: `
			let oneAndTwo = fn() { let one = 1; let two = 2; one + two; };
			let threeAndFour = fn() { let three = 3; let four = 4; three + four; };
			oneAndTwo() + threeAndFour();
			`,
			expected: 10,
		},
		{
			// Same local name in two functions: each call gets its own slot.
			input: `
			let firstFoobar = fn() { let foobar = 50; foobar; };
			let secondFoobar = fn() { let foobar = 100; foobar; };
			firstFoobar() + secondFoobar();
			`,
			expected: 150,
		},
		{
			input: `
			let globalSeed = 50;
			let minusOne = fn() {
				let num = 1;
				globalSeed - num;
			}
			let minusTwo = fn() {
				let num = 2;
				globalSeed - num;
			}
			minusOne() + minusTwo();
			`,
			expected: 97,
		},
		{
			// A local in the caller must survive a call that uses its own locals.
			input: `
			let inner = fn() { let x = 100; x };
			let outer = fn() { let y = 1; inner(); y };
			outer();
			`,
			expected: 1,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithArgumentsAndBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let identity = fn(a) { a; };
			identity(4);
			`,
			expected: 4,
		},
		{
			input: `
			let sum = fn(a, b) { a + b; };
			sum(1, 2);
			`,
			expected: 3,
		},
		{
			input: `
			let sum = fn(a, b) {
				let c = a + b;
				c;
			};
			sum(1, 2);
			`,
			expected: 3,
		},
		{
			input: `
			let sum = fn(a, b) {
				let c = a + b;
				c;
			};
			sum(1, 2) + sum(3, 4);`,
			expected: 10,
		},
		{
			input: `
			let sum = fn(a, b) {
				let c = a + b;
				c;
			};
			let outer = fn() {
				sum(1, 2) + sum(3, 4);
			};
			outer();
			`,
			expected: 10,
		},
		{
			input: `
			let globalNum = 10;

			let sum = fn(a, b) {
				let c = a + b;
				c + globalNum;
			};

			let outer = fn() {
				sum(1, 2) + sum(3, 4) + globalNum;
			};

			outer() + globalNum;
			`,
			expected: 50,
		},
		{
			// Argument order matters: a - b, not b - a.
			input: `
			let minus = fn(a, b) { a - b };
			minus(10, 3);
			`,
			expected: 7,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithWrongArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input:    `fn() { 1; }(1);`,
			expected: `wrong number of arguments: want=0, got=1`,
		},
		{
			input:    `fn(a) { a; }();`,
			expected: `wrong number of arguments: want=1, got=0`,
		},
		{
			input:    `fn(a, b) { a + b; }(1);`,
			expected: `wrong number of arguments: want=2, got=1`,
		},
	}

	for _, tt := range tests {
		program := parse(tt.input)

		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err == nil {
			t.Fatalf("expected VM error but resulted in none.")
		}

		if err.Error() != tt.expected {
			t.Fatalf("wrong VM error: want=%q, got=%q", tt.expected, err)
		}
	}
}

func TestBuiltinFunctions(t *testing.T) {
	tests := []vmTestCase{
		{`len("")`, 0},
		{`len("four")`, 4},
		{`len("hello world")`, 11},
		{
			`len(1)`,
			&object.Error{
				Message: "argument to `len` not supported, got INTEGER",
			},
		},
		{`len("one", "two")`,
			&object.Error{
				Message: "wrong number of arguments. got=2, want=1",
			},
		},
		{`len([1, 2, 3])`, 3},
		{`len([])`, 0},
		{`puts("hello", "world!")`, Null},
		{`first([1, 2, 3])`, 1},
		{`first([])`, Null},
		{`first(1)`,
			&object.Error{
				Message: "argument to `first` must be ARRAY, got INTEGER",
			},
		},
		{`last([1, 2, 3])`, 3},
		{`last([])`, Null},
		{`last(1)`,
			&object.Error{
				Message: "argument to `last` must be ARRAY, got INTEGER",
			},
		},
		{`rest([1, 2, 3])`, []int{2, 3}},
		{`rest([])`, Null},
		{`push([], 1)`, []int{1}},
		{`push(1, 1)`,
			&object.Error{
				Message: "argument to `push` must be ARRAY, got INTEGER",
			},
		},
		{
			// Builtins work from inside functions, and their result can be
			// used like any other value.
			`let count = fn(arr) { len(arr) * 2 }; count([1, 2])`,
			4,
		},
	}

	runVmTests(t, tests)
}

func TestClosures(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let newClosure = fn(a) {
				fn() { a; };
			};
			let closure = newClosure(99);
			closure();
			`,
			expected: 99,
		},
		{
			input: `
			let newAdder = fn(a) { fn(b) { a + b } };
			newAdder(1)(2);
			`,
			expected: 3,
		},
		{
			input: `
			let newAdder = fn(a, b) {
				fn(c) { a + b + c };
			};
			let adder = newAdder(1, 2);
			adder(8);
			`,
			expected: 11,
		},
		{
			input: `
			let newAdder = fn(a, b) {
				let c = a + b;
				fn(d) { c + d };
			};
			let adder = newAdder(1, 2);
			adder(8);
			`,
			expected: 11,
		},
		{
			input: `
			let newAdderOuter = fn(a, b) {
				let c = a + b;
				fn(d) {
					let e = d + c;
					fn(f) { e + f; };
				};
			};
			let newAdderInner = newAdderOuter(1, 2)
			let adder = newAdderInner(3);
			adder(8);
			`,
			expected: 14,
		},
		{
			input: `
			let a = 1;
			let newAdderOuter = fn(b) {
				fn(c) {
					fn(d) { a + b + c + d };
				};
			};
			let newAdderInner = newAdderOuter(2)
			let adder = newAdderInner(3);
			adder(8);
			`,
			expected: 14,
		},
		{
			input: `
			let newClosure = fn(a, b) {
				let one = fn() { a; };
				let two = fn() { b; };
				fn() { one() + two(); };
			};
			let closure = newClosure(9, 90);
			closure();
			`,
			expected: 99,
		},
		{
			// Two closures from the same maker keep their own copies.
			input: `
			let newAdder = fn(a) { fn(b) { a + b } };
			let addOne = newAdder(1);
			let addTen = newAdder(10);
			addOne(1) + addTen(1);
			`,
			expected: 13,
		},
	}

	runVmTests(t, tests)
}

func TestCurrentClosure(t *testing.T) {
	// fn(x) { <the running closure> }. The argument sits on top of the
	// stack, so if OpCurrentClosure did nothing we'd get 5 back instead.
	fn := &object.CompiledFunction{
		Instructions: concat(
			code.Make(code.OpCurrentClosure),
			code.Make(code.OpReturnValue),
		),
		NumLocals:     1,
		NumParameters: 1,
	}
	bytecode := &compiler.Bytecode{
		Instructions: concat(
			code.Make(code.OpClosure, 0, 0),
			code.Make(code.OpConstant, 1),
			code.Make(code.OpCall, 1),
			code.Make(code.OpPop),
		),
		Constants: []object.Object{fn, &object.Integer{Value: 5}},
	}

	vm := New(bytecode)
	if err := vm.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}

	cl, ok := vm.LastPoppedStackElem().(*object.Closure)
	if !ok {
		t.Fatalf("expected a closure, got %T (%+v)",
			vm.LastPoppedStackElem(), vm.LastPoppedStackElem())
	}
	if cl.Fn != fn {
		t.Errorf("got a closure around the wrong function")
	}
}

func concat(parts ...[]byte) code.Instructions {
	out := code.Instructions{}
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestUnknownOpcodeIsAnError(t *testing.T) {
	// 255 isn't an opcode. Skipping it silently would hide compiler bugs.
	bytecode := &compiler.Bytecode{Instructions: code.Instructions{255}}

	err := New(bytecode).Run()
	if err == nil {
		t.Fatalf("expected an error for an unknown opcode, got none")
	}
	if err.Error() != "unknown opcode 255" {
		t.Fatalf("wrong VM error: got=%q", err)
	}
}

func TestRecursiveFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let countDown = fn(x) {
				if (x == 0) {
					return 0;
				} else {
					countDown(x - 1);
				}
			};
			countDown(1);
			`,
			expected: 0,
		},
		{
			input: `
			let countDown = fn(x) {
				if (x == 0) {
					return 0;
				} else {
					countDown(x - 1);
				}
			};
			let wrapper = fn() {
				countDown(1);
			};
			wrapper();
			`,
			expected: 0,
		},
		{
			// countDown lives inside wrapper, so it's a local there. This is
			// the case that needs OpCurrentClosure.
			input: `
			let wrapper = fn() {
				let countDown = fn(x) {
					if (x == 0) {
						return 0;
					} else {
						countDown(x - 1);
					}
				};
				countDown(1);
			};
			wrapper();
			`,
			expected: 0,
		},
		{
			// A closure nested inside the recursive function can call it too.
			input: `
			let wrapper = fn() {
				let countDown = fn(x) {
					let step = fn() { countDown(x - 1) };
					if (x == 0) { 0 } else { step() }
				};
				countDown(3);
			};
			wrapper();
			`,
			expected: 0,
		},
	}

	runVmTests(t, tests)
}

func TestRecursiveFibonacci(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			let fibonacci = fn(x) {
				if (x == 0) {
					return 0;
				} else {
					if (x == 1) {
						return 1;
					} else {
						fibonacci(x - 1) + fibonacci(x - 2);
					}
				}
			};
			fibonacci(15);
			`,
			expected: 610,
		},
	}

	runVmTests(t, tests)
}

func TestTopLevelReturn(t *testing.T) {
	// A `return` outside any function ends the program with that value,
	// the same as in the tree-walking evaluator.
	tests := []vmTestCase{
		{"return 5; 10", 5},
		{"if (true) { return 1; } 2", 1},
	}

	runVmTests(t, tests)
}

func TestTooManyFrames(t *testing.T) {
	vm := New(&compiler.Bytecode{})
	cl := &object.Closure{Fn: &object.CompiledFunction{}}

	// The main frame already takes one slot.
	for i := 1; i < MaxFrames; i++ {
		if err := vm.pushFrame(NewFrame(cl, 0)); err != nil {
			t.Fatalf("pushFrame %d failed early: %s", i, err)
		}
	}

	if err := vm.pushFrame(NewFrame(cl, 0)); err == nil {
		t.Fatalf("expected an error past MaxFrames, got none")
	}
}

func TestCallingNonFunctionIsAnError(t *testing.T) {
	comp := compiler.New()
	if err := comp.Compile(parse("1()")); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	err := New(comp.Bytecode()).Run()
	if err == nil {
		t.Fatalf("expected an error for calling an integer, got none")
	}
	if err.Error() != "calling non-function" {
		t.Fatalf("wrong VM error: got=%q", err)
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
	case *object.Error:
		errObj, ok := actual.(*object.Error)
		if !ok {
			t.Errorf("object is not Error: %T (%+v)", actual, actual)
			return
		}

		if errObj.Message != expected.Message {
			t.Errorf("wrong error message. expected=%q, got=%q",
				expected.Message, errObj.Message)
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
