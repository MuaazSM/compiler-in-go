package repl

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPrintsFinalValue(t *testing.T) {
	var out bytes.Buffer
	input := `
let add = fn(a, b) { a + b };
add(2, 3);
`
	if err := Run(input, &out); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := out.String(); got != "5\n" {
		t.Errorf("wrong output. want=%q, got=%q", "5\n", got)
	}
}

func TestRunSkipsNullResult(t *testing.T) {
	var out bytes.Buffer
	if err := Run(`if (false) { 1 }`, &out); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := out.String(); got != "" {
		t.Errorf("expected no output, got=%q", got)
	}
}

func TestRunReportsErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`let = 5;`, "parser errors"},
		{`undefinedName`, "undefined variable undefinedName"},
		{`1 / 0`, "division by zero"},
	}

	for _, tt := range tests {
		var out bytes.Buffer
		err := Run(tt.input, &out)
		if err == nil {
			t.Errorf("%q: expected an error", tt.input)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: error %q does not mention %q", tt.input, err, tt.want)
		}
	}
}
