// Package token lists every kind of word the Monkey language has, so the lexer
// and the parser agree on what they are passing around.
package token

// TokenType names a kind of token, like INT or PLUS. It's a string so a token
// prints as something readable when a test fails.
type TokenType string

const (
	// ILLEGAL marks a character the lexer doesn't know, and EOF marks the end
	// of the input, so the parser knows when to stop.
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers + literals
	IDENT  = "IDENT"  // add, foobar, x, y, ...
	INT    = "INT"    // 1343456
	STRING = "STRING" // "foobar"

	// Operators
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"

	LT = "<"
	GT = ">"

	EQ     = "=="
	NOT_EQ = "!="

	// Delimiters
	COMMA     = ","
	SEMICOLON = ";"
	COLON     = ":"

	LPAREN   = "("
	RPAREN   = ")"
	LBRACE   = "{"
	RBRACE   = "}"
	LBRACKET = "["
	RBRACKET = "]"

	// Keywords
	FUNCTION = "FUNCTION"
	LET      = "LET"
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	IF       = "IF"
	ELSE     = "ELSE"
	RETURN   = "RETURN"
)

// Token is one word of the program: what kind it is, and the exact text it
// came from. The parser decides what to do from Type; Literal keeps the
// actual value, like "5" or "add".
// Concept: token - the smallest meaningful piece of source code, tagged with its kind.
type Token struct {
	Type    TokenType
	Literal string
}

// keywords are the words the language reserves for itself. Anything else made
// of letters is a name the programmer chose.
var keywords = map[string]TokenType{
	"fn":     FUNCTION,
	"let":    LET,
	"true":   TRUE,
	"false":  FALSE,
	"if":     IF,
	"else":   ELSE,
	"return": RETURN,
}

// LookupIdent tells a keyword apart from a plain name: "let" becomes LET,
// while "total" stays an IDENT.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
