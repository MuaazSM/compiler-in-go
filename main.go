package main

import (
	"fmt"
	"monkey/repl"
	"os"
	"os/user"
)

func main() {
	// With a file name, run that file. Without one, start the prompt.
	if len(os.Args) > 1 {
		runFile(os.Args[1])
		return
	}

	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Hello %s! This is the Monkey programming language!\n",
		user.Username)
	fmt.Printf("Feel free to type in commands\n")
	repl.Start(os.Stdin, os.Stdout)
}

func runFile(path string) {
	input, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := repl.Run(string(input), os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", path, err)
		os.Exit(1)
	}
}
