package download

import "testing"

func TestMakeValid(t *testing.T) {
	in := "A:/<bad>|name?* with\\chars'"
	got := MakeValid(in)
	want := "A___bad__name___with_chars_"
	if got != want {
		t.Fatalf("MakeValid mismatch: got %q want %q", got, want)
	}
}
