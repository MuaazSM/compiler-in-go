// Command benchmark runs the same recursive fibonacci program on either the
// tree-walking evaluator or the bytecode VM, and reports how long it took.
//
//	go build -o fibonacci ./benchmark
//	./fibonacci -engine=eval
//	./fibonacci -engine=vm
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"monkey/ast"
	"monkey/compiler"
	"monkey/evaluator"
	"monkey/lexer"
	"monkey/object"
	"monkey/parser"
	"monkey/vm"
)

var engine = flag.String("engine", "vm", "use 'vm' or 'eval'")

// Naive recursive fibonacci makes about 30 million calls for n=35. Almost all
// of the time goes on calling functions, adding and comparing, so it's a fair
// test of how fast each engine does the basic work.
var input = `
let fibonacci = fn(x) {
	if (x == 0) {
		0
	} else {
		if (x == 1) {
			return 1;
		} else {
			fibonacci(x - 1) + fibonacci(x - 2);
		}
	}
};
fibonacci(35);
`

func main() {
	flag.Parse()

	program, err := parse(input)
	if err != nil {
		fail(err)
	}

	var duration time.Duration
	var result object.Object

	// Parsing (and compiling, for the VM) happens before the clock starts.
	// We only time running the program, since that's where the two engines
	// actually differ.
	switch *engine {
	case "vm":
		comp := compiler.New()
		if err := comp.Compile(program); err != nil {
			fail(fmt.Errorf("compiler error: %s", err))
		}

		machine := vm.New(comp.Bytecode())

		// The VM wins because the slow decisions were made ahead of time. It
		// reads a flat run of bytes instead of walking a tree of Go structs
		// and switching on node types, and every value sits on one stack
		// instead of in a chain of environment maps looked up by name.
		start := time.Now()
		if err := machine.Run(); err != nil {
			fail(fmt.Errorf("vm error: %s", err))
		}
		duration = time.Since(start)
		result = machine.LastPoppedStackElem()

	case "eval":
		env := object.NewEnvironment()

		start := time.Now()
		result = evaluator.Eval(program, env)
		duration = time.Since(start)

	default:
		fail(fmt.Errorf("unknown engine %q, use 'vm' or 'eval'", *engine))
	}

	fmt.Printf("engine=%s, result=%s, duration=%s\n",
		*engine, result.Inspect(), duration)
}

func parse(input string) (*ast.Program, error) {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		return nil, fmt.Errorf("parser errors: %v", p.Errors())
	}
	return program, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
