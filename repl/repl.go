// Package repl is the interactive prompt. Each line typed goes through the full
// pipeline: lexer, parser, compiler and VM.
// Concept: REPL - read a line, evaluate it, print the result, loop.
package repl

import (
	"bufio"
	"fmt"
	"io"
	"monkey/compiler"
	"monkey/lexer"
	"monkey/object"
	"monkey/parser"
	"monkey/vm"
)

// PROMPT is printed before every line the user types.
const PROMPT = ">> "

// Start runs the prompt until the input ends (Ctrl+D).
func Start(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)

	// These outlive a single line, so `let x = 1` on one line is still
	// there on the next.
	constants := []object.Object{}
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	for {
		fmt.Fprintf(out, PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			return
		}

		line := scanner.Text()
		l := lexer.New(line)
		p := parser.New(l)

		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			continue
		}

		// Every line gets a fresh compiler, but it reuses the symbol table and
		// constants from earlier lines, so names defined before still resolve.
		comp := compiler.NewWithState(symbolTable, constants)
		err := comp.Compile(program)
		if err != nil {
			fmt.Fprintf(out, "Woops! Compilation failed:\n %s\n", err)
			continue
		}

		// Appending may have moved the pool to a new array, so take the
		// latest version for the next line.
		code := comp.Bytecode()
		constants = code.Constants

		// Same idea for the VM: a fresh machine per line, sharing one globals
		// array so values stored earlier are still there.
		machine := vm.NewWithGlobalsStore(code, globals)
		err = machine.Run()
		if err != nil {
			fmt.Fprintf(out, "Woops! Executing bytecode failed:\n %s\n", err)
			continue
		}

		// The line's result was popped by its final OpPop, but it's still in
		// the slot just above the stack pointer. Nil means nothing ran.
		lastPopped := machine.LastPoppedStackElem()
		if lastPopped != nil {
			io.WriteString(out, lastPopped.Inspect())
			io.WriteString(out, "\n")
		}
	}
}

// MONKEY_FACE is shown above parse errors, to keep mistakes friendly.
const MONKEY_FACE = `            __,__
   .--.  .-"     "-.  .--.
  / .. \/  .-. .-.  \/ .. \
 | |  '|  /   Y   \  |'  | |
 | \   \  \ 0 | 0 /  /   / |
  \ '- ,\.-"""""""-./, -' /
   ''-' /_   ^ ^   _\ '-''
       |  \._   _./  |
       \   \ '~' /   /
        '._ '-=-' _.'
           '-----'
`

// printParserErrors prints every parse error at once.
func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, MONKEY_FACE)
	io.WriteString(out, "Woops! We ran into some monkey business here!\n")
	io.WriteString(out, " parser errors:\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
