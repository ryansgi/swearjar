package testkit

import "testing"

func TestMustPanic(t *testing.T) {
	t.Parallel()

	MustPanic(t, func() {
		panic("boom")
	})
}

func TestMustNotPanic(t *testing.T) {
	t.Parallel()

	MustNotPanic(t, func() {
		// no panic
	})
}

func TestMustContain(t *testing.T) {
	t.Parallel()

	haystack := "alpha beta gamma"
	MustContain(t, haystack, "beta")
}
