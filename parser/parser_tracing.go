package parser

import (
	"fmt"
	"strings"
)

// This file is a debugging helper, not part of normal parsing. Adding
// defer untrace(trace("parseExpression")) at the top of a parse function prints
// an indented BEGIN/END line for every call, which shows how the parser walks
// through an expression. It is off unless someone adds those calls.
//
// traceLevel is how deep we are, which sets the indentation.
var traceLevel int = 0

const traceIdentPlaceholder string = "\t"

func identLevel() string {
	return strings.Repeat(traceIdentPlaceholder, traceLevel-1)
}

func tracePrint(fs string) {
	fmt.Printf("%s%s\n", identLevel(), fs)
}

func incIdent() { traceLevel = traceLevel + 1 }
func decIdent() { traceLevel = traceLevel - 1 }

// trace prints BEGIN and indents one level deeper. It returns msg so it can
// be passed straight to untrace in a defer.
func trace(msg string) string {
	incIdent()
	tracePrint("BEGIN " + msg)
	return msg
}

// untrace prints END and steps back out one level.
func untrace(msg string) {
	tracePrint("END " + msg)
	decIdent()
}
