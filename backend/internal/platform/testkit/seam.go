package testkit

import (
	"sync"
	"testing"
)

var seamMu sync.Mutex

// Swap swaps a package-level function variable for the duration of the test and restores it after
func Swap[T any](t *testing.T, target *T, replacement T) {
	t.Helper()
	orig := *target
	*target = replacement
	t.Cleanup(func() { *target = orig })
}

// Serial makes the entire test run under a global lock, preventing interference
// when tests mutate package-level seams
func Serial(t *testing.T) {
	t.Helper()
	seamMu.Lock()
	t.Cleanup(func() { seamMu.Unlock() })
}
