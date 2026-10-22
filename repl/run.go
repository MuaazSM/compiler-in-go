package repl

import (
	"fmt"
	"io"
	"monkey/compiler"
	"monkey/lexer"
	"monkey/parser"
	"monkey/vm"
	"strings"
)

// Run compiles and runs a whole program in one go, the way a script file is
// run, and writes the value of its last expression to out.
func Run(input string, out io.Writer) error {
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		return fmt.Errorf("parser errors:\n\t%s", strings.Join(p.Errors(), "\n\t"))
	}

	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		return fmt.Errorf("compilation failed: %s", err)
	}

	machine := vm.New(comp.Bytecode())
	if err := machine.Run(); err != nil {
		return fmt.Errorf("executing bytecode failed: %s", err)
	}

	// A script usually ends with a puts call, which gives back null. Printing
	// that would just add a stray "null" after the real output.
	last := machine.LastPoppedStackElem()
	if last != nil && last != vm.Null {
		fmt.Fprintln(out, last.Inspect())
	}
	return nil
}
