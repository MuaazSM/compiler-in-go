# Concepts Primer

A short tour of the ideas behind this project, before any compiler code exists. If a word in the
code ever confuses you, the [glossary](#glossary) at the bottom has a one-line meaning for it.

---

## Compiler vs interpreter

Both start the same way: a lexer and parser turn source text into a tree (the AST). After that
they split. An **interpreter** walks the tree and *does* what each node says, right away. A
**compiler** walks the tree and *writes down* instructions in some other language, which
something else runs later. Our project keeps the first half (lexer, parser, AST) and swaps the
tree-walking evaluator for a compiler plus a machine that runs what it produces.

---

## How a real CPU runs code

Strip a computer down and you get two parts that matter to us: the **CPU** and **memory**.

Memory is a long row of numbered slots. The number is the slot's *address*. Both data *and* the
program's instructions live in memory; an instruction is just bytes until the CPU reads it and
treats it as an instruction.

The CPU runs one simple loop, forever:

1. **Fetch** — read the next instruction from memory. The **program counter** holds its address.
2. **Decode** — work out which operation those bytes mean.
3. **Execute** — do it: add two numbers, move data around, jump somewhere else.

Then it moves the program counter on and goes again. That loop is the *fetch-decode-execute
cycle*, and it's the heartbeat of every machine, real or virtual.

**The stack.** One region of memory is used last-in-first-out: you push things on top and pop
them off the top. The CPU uses it to remember function calls: where to go back to when a function
returns, plus that call's arguments and local variables. Calls nest, and a stack nests naturally,
so it's a perfect fit. A special register, the **stack pointer**, remembers where the top is.

**Registers** are a handful of tiny, very fast storage slots inside the CPU itself. They hold
whatever is being worked on right now.

---

## What a virtual machine is

A virtual machine (VM) is a computer made out of software. It isn't pretending to be some real
chip. It *is* the machine, just one that only exists as code. Ours will be a Go struct with a
loop.

It copies the real thing closely: an instruction pointer (our program counter), a
fetch-decode-execute loop, and a stack. The difference is that we design its instruction set
ourselves, so it only knows the operations Monkey needs. Fewer, more focused instructions mean a
simpler and faster machine. And because the VM is plain Go, it runs anywhere Go runs.

### Why a stack machine and not a register machine?

There are two big families of VM:

- A **stack machine** does all its work on one stack. To add, push two values, then run "add",
  which pops both and pushes the sum.
- A **register machine** also has a stack, but instructions can name virtual registers directly
  ("add r1 and r2, put it in r3").

Register machines need fewer instructions to do the same work, so they can be faster. But they're
harder to build, and the compiler has to be smarter to decide what goes in which register. A
stack machine is simpler to compile to and simpler to run. We're here to learn, so we pick the
stack machine. Even so, it ends up about 3× faster than the tree-walking evaluator.

---

## What our bytecode looks like

Our compiler outputs **bytecode**: a flat `[]byte` the VM reads from start to finish.

Each instruction is:

```
┌────────┬─────────────────────────────┐
│ opcode │ operands (0 or more)        │
│ 1 byte │ fixed width per opcode      │
└────────┴─────────────────────────────┘
```

- The **opcode** is a single byte that says which operation to run (`OpConstant`, `OpAdd`, …).
  Its readable name is only for humans; in the bytes it's just a number.
- The **operands** are the opcode's inputs. Each opcode has a fixed layout. `OpConstant` always
  takes one 2-byte operand, and `OpAdd` takes none. The VM looks up the layout, so it always knows
  how many bytes to read.
- Operands wider than one byte are stored **big-endian**: the most significant byte comes first.

Values like numbers and strings don't go inside the instructions. They sit in a side list called
the **constant pool**, and instructions point to them by index. So `OpConstant 1` means "push
whatever is at index 1 of the constant pool", not "push the number 1".

The compiler hands the VM two things: the instruction bytes and the constant pool.

---

## Worked example: `1 + 2`

### What the compiler does

The parser gives us an infix-expression node: `1`, `+`, `2`. The compiler walks it like this:

1. Left side `1`: add `1` to the constant pool (index 0) and emit `OpConstant 0`.
2. Right side `2`: add `2` to the pool (index 1) and emit `OpConstant 1`.
3. The operator `+`: emit `OpAdd`.
4. It's a whole statement whose value nobody uses, so emit `OpPop` to clean up.

Result:

```
Constants: [ 1, 2 ]

0000 OpConstant 0
0003 OpConstant 1
0006 OpAdd
0007 OpPop
```

The numbers on the left are byte offsets. Each `OpConstant` takes 3 bytes (1 opcode + 2 operand
bytes); `OpAdd` and `OpPop` take 1 each. In raw bytes, if `OpConstant`, `OpAdd` and `OpPop` turn
out to be opcodes 0, 1 and 2 (the real numbers are fixed in Phase 2), that's:

```
00 00 00   00 00 01   01   02
└ op ┘└idx┘           add  pop
```

### What the VM does

`sp` (the stack pointer) always points at the **next free slot**, so the top value is at `sp-1`.

```
start            OpConstant 0      OpConstant 1      OpAdd             OpPop
                 push consts[0]    push consts[1]    pop 2, pop 1,     pop 3 and
                                                     push 1+2          throw it away
|     |          |     |           |     |           |     |           |     |
|     |          |     |           |  2  | <- top    |     |           |     |
|_____|          |__1__| <- top    |__1__|           |__3__| <- top    |_____|
 sp=0             sp=1              sp=2              sp=1              sp=0
```

Note that `OpAdd` pops the **right** operand first (it's on top), then the left. Order doesn't
matter for `+`, but it will for `-` and `/`.

After `OpPop` the stack is empty again. The `3` is still sitting in slot 0, though, because
popping just moves `sp` down and doesn't wipe the slot. Tests and the REPL use that to read the
last popped value.

---

## Building both halves together

The compiler and the VM are two sides of one contract: the opcode definitions plus the constant
pool. Building one fully before the other is confusing, because you can't tell why the bytecode
looks the way it does. So each feature grows both at once:

1. define the opcode,
2. write a compiler test, then make it pass,
3. write a VM test, then make it pass.

---

## Glossary

| Concept | One-liner |
|---|---|
| Bytecode | A compact list of bytes that tells the VM what to do, step by step. |
| Opcode | The first byte of an instruction; it says *which* operation to run. |
| Operand | Extra bytes after an opcode that give the operation its input (like an index). |
| Big-endian | The most significant byte comes first in memory. |
| Disassembler | Turns raw bytecode back into readable text, mainly for tests and debugging. |
| Constant pool | A side list of values (numbers, strings, functions) that instructions refer to by index. |
| Stack machine | A VM that does all its work by pushing and popping values on one stack. |
| Stack pointer (sp) | Points at the next free slot; the top value is at `sp-1`. |
| Fetch-decode-execute | Read an instruction, figure out what it means, do it, repeat. |
| Instruction pointer (ip) | Where in the bytecode the VM is reading right now. |
| Back-patching | Emit a placeholder jump, fill in the real target later. |
| Truthiness | Everything counts as true except `false` and `null`. |
| Symbol table | The compiler's address book: name → (scope, index). |
| Scope | Where a name lives: global, local, builtin, free or the current function. |
| Global binding | A variable stored in a fixed-size globals array, looked up by index. |
| Compilation scope | A separate instruction buffer used while compiling a function body. |
| Call frame | The bookkeeping the VM keeps for each active call. |
| Base pointer | The stack index where the current call's locals begin. |
| Local binding | A variable that lives in the stack slots reserved for the current call. |
| Calling convention | The agreed stack layout for callee, arguments and locals during a call. |
| Implicit return | The last expression in a function body is its return value. |
| Builtin function | A Go function exposed to Monkey code, looked up by index. |
| Closure | A function bundled with the variables it captured from outer scopes. |
| Free variable | A variable used inside a function but defined in an enclosing one. |
| Self-reference | A function referring to itself by name (recursion) before it's fully bound. |
