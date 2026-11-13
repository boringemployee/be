package version

import "testing"

func TestString(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })

	Version = "v0.1.0"
	if got, want := String(), "be v0.1.0"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
