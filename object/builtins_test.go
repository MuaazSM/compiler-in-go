package object

import "testing"

func TestBuiltinsOrder(t *testing.T) {
	// Compiled bytecode refers to builtins by position, so this order is part
	// of the bytecode format. Changing it would break compiled programs.
	expected := []string{"len", "puts", "first", "last", "rest", "push"}

	if len(Builtins) != len(expected) {
		t.Fatalf("wrong number of builtins. want=%d, got=%d",
			len(expected), len(Builtins))
	}

	for i, name := range expected {
		if Builtins[i].Name != name {
			t.Errorf("builtin %d wrong. want=%q, got=%q", i, name, Builtins[i].Name)
		}
	}
}

func TestGetBuiltinByName(t *testing.T) {
	if GetBuiltinByName("len") != Builtins[0].Builtin {
		t.Errorf("GetBuiltinByName(len) did not return the len builtin")
	}

	if GetBuiltinByName("nope") != nil {
		t.Errorf("GetBuiltinByName(nope) should be nil")
	}
}
