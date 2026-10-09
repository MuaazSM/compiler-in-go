# compiler-in-go

[![test](https://github.com/MuaazSM/compiler-in-go/actions/workflows/test.yml/badge.svg)](https://github.com/MuaazSM/compiler-in-go/actions/workflows/test.yml)

A bytecode compiler and stack-based virtual machine for **Monkey**, a small programming language
with integers, booleans, strings, arrays, hashes, first-class functions and closures.

Monkey started life as a tree-walking interpreter. This project keeps that front end and swaps the
back end for a compiler that turns programs into compact bytecode, plus a VM that runs it. The
result runs the same programs about **3× faster**.

```monkey
let newAdder = fn(a) { fn(b) { a + b } };
let addTwo = newAdder(2);

let map = fn(arr, f) {
  let iter = fn(arr, acc) {
    if (len(arr) == 0) { acc } else { iter(rest(arr), push(acc, f(first(arr)))) }
  };
  iter(arr, []);
};

map([1, 2, 3], addTwo);
```

That last line gives `[3, 4, 5]`.

## How it works

```
source ──▶ lexer ──▶ parser ──▶ AST ──▶ compiler ──▶ bytecode ──▶ VM ──▶ result
 text      tokens               tree               opcodes +          stack
                                                   constant pool      machine
```

1. The **lexer** cuts the source text into tokens.
2. The **parser** builds an abstract syntax tree (AST) from them.
3. The **compiler** walks the tree once and emits bytecode: one-byte opcodes with fixed-width,
   big-endian operands, plus a constant pool for numbers, strings and compiled functions.
4. The **VM** runs that bytecode in a fetch-decode-execute loop, keeping every value on one stack.
   Each function call gets a frame; locals and arguments live in stack slots; closures carry the
   values they captured.

New to compilers? [docs/CONCEPTS.md](docs/CONCEPTS.md) walks through the ideas, including how
`1 + 2` becomes bytecode and what the stack looks like at each step.

## Getting started

You need Go 1.24 or newer. There are no other dependencies.

Start the REPL:

```sh
go run .
```

```
>> let fibonacci = fn(x) { if (x < 2) { x } else { fibonacci(x - 1) + fibonacci(x - 2) } };
>> fibonacci(15)
610
>> {"name": "Monkey", "age": 1}["name"]
Monkey
```

Each line you type is compiled and run on the VM, and names you define stay around for the next
line.

Run the tests:

```sh
go test ./...
go vet ./...
```

## Benchmark

`benchmark/` runs a naive recursive `fibonacci(35)` (about 30 million calls) on either engine and
times only the execution, not parsing or compiling.

```sh
go build -o fibonacci ./benchmark
./fibonacci -engine=eval
./fibonacci -engine=vm
```

On an Apple Silicon Mac (Go 1.24.6):

| Engine | Result | Time |
|---|---|---|
| `eval` (tree-walking interpreter) | 9227465 | 8.67s |
| `vm` (bytecode compiler + VM) | 9227465 | 2.74s |

That's about **3.2× faster**. The VM wins because the slow decisions are made once, ahead of time:
it reads a flat run of bytes instead of walking a tree, and finds values by slot number on one
stack instead of looking names up in a chain of maps.

## Project layout

| Path | What's in it |
|---|---|
| `token/`, `lexer/` | Token types and the lexer |
| `ast/`, `parser/` | AST node types and the Pratt parser |
| `evaluator/` | The original tree-walking interpreter, kept as a reference and benchmark baseline |
| `object/` | Runtime values (integers, strings, arrays, hashes, closures, …) and the builtin functions |
| `code/` | The bytecode format: opcodes, `Make`, operand decoding and a disassembler |
| `compiler/` | AST → bytecode, plus the symbol table that tracks global, local, builtin and free names |
| `vm/` | The stack VM, call frames and closures |
| `repl/`, `main.go` | The interactive prompt |
| `benchmark/` | `fibonacci(35)` timed on both engines |
| `docs/CONCEPTS.md` | A friendly primer on compilers, VMs and bytecode |

## How this was built

The project was built one phase at a time, test first, with the compiler and VM growing
together:

- [IMPLEMENTATION.md](IMPLEMENTATION.md) is the design and the phase-by-phase plan.
- [PROMPTBOOK.md](PROMPTBOOK.md) holds the step-by-step prompts used for each phase, the comment
  style and the git rules.

Each finished phase is tagged in git (`phase-00` … `phase-10`).

## Credits

Based on *Writing A Compiler In Go* by Thorsten Ball; starter code MIT licensed
(see [LICENSE-MONKEY-BOOK](LICENSE-MONKEY-BOOK)).

Everything else is released under the MIT License; see [LICENSE](LICENSE).
