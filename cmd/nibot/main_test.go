package main

import "testing"

func TestStringListFlag(t *testing.T) {
	var f stringListFlag
	if err := f.Set("a"); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("b"); err != nil {
		t.Fatal(err)
	}
	if len(f) != 2 || f[0] != "a" || f[1] != "b" {
		t.Fatalf("unexpected values: %#v", f)
	}
	if f.String() == "" {
		t.Fatalf("expected non-empty string")
	}
}

