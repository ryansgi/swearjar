// Package testkit provides testing helpers
package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MustPanic asserts that fn panics
func MustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic, got none")
		}
	}()
	fn()
}

// MustNotPanic asserts that fn does not panic
func MustNotPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	fn()
}

// MustContain asserts that haystack contains needle. If not, writes haystack to logger_test_output.txt for debugging
func MustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		tmpfile := filepath.Join(t.TempDir(), "logger_test_output.txt")
		_ = os.WriteFile(tmpfile, []byte(haystack), 0o600)
		t.Fatalf("expected output to contain %q\n\nfull output written to %s", needle, tmpfile)
	}
}
