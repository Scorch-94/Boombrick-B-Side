package version

import "testing"

func TestString(t *testing.T) {
	if got, want := String(), "Boombrick B-Side dev"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
