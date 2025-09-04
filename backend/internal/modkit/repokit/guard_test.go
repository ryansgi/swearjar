package repokit

import (
	"context"
	"strings"
	"testing"
	"time"
)

// fakePinger records the ctx it was invoked with and returns a preset error
type fakePinger struct {
	lastCtx context.Context
	err     error
}

func (f *fakePinger) Ping(ctx context.Context) error {
	f.lastCtx = ctx
	return f.err
}

// assertPanicContains runs fn and asserts it panics with a message containing wantSub
func assertPanicContains(t *testing.T, name, wantSub string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected panic, got none", name)
			return
		}
		var msg string
		switch x := r.(type) {
		case string:
			msg = x
		case error:
			msg = x.Error()
		default:
			// best effort stringify
			msg = ""
		}
		if !strings.Contains(msg, wantSub) {
			t.Fatalf("%s: panic message mismatch, got %q want contains %q", name, msg, wantSub)
		}
	}()
	fn()
}

func TestMustPing_PanicsOnNilDependency(t *testing.T) {
	t.Parallel()
	assertPanicContains(t, "MustPing(nil)", "pg: nil dependency", func() {
		MustPing(context.Background(), "pg", nil)
	})
}

func TestMustPing_AddsDefaultTimeoutWhenNone(t *testing.T) {
	t.Parallel()

	fp := &fakePinger{err: nil}
	start := time.Now()

	MustPing(context.Background(), "pg", fp) // should not panic

	// Verify the pinger saw a context with a deadline around +5s
	if fp.lastCtx == nil {
		t.Fatalf("expected fakePinger to receive a context")
	}
	dl, ok := fp.lastCtx.Deadline()
	if !ok {
		t.Fatalf("expected a deadline to be set by MustPing")
	}
	if time.Until(dl) <= 0 {
		t.Fatalf("deadline already expired")
	}
	// around 5s (tolerate jitter)
	if got := dl.Sub(start); got < 4*time.Second || got > 6*time.Second {
		t.Fatalf("default deadline not ~5s: got %v", got)
	}
}

func TestMustPing_HonorsExistingDeadline(t *testing.T) {
	t.Parallel()

	fp := &fakePinger{err: nil}

	parent, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	MustPing(parent, "pg", fp) // should not panic

	dlWant, okWant := parent.Deadline()
	dlGot, okGot := fp.lastCtx.Deadline()
	if !okWant || !okGot {
		t.Fatalf("both contexts should have deadlines: parent=%v child=%v", okWant, okGot)
	}
	// Child should reflect the parent's deadline (not a fresh ~5s one)
	diff := dlGot.Sub(dlWant)
	if diff < -2*time.Millisecond || diff > 2*time.Millisecond {
		t.Fatalf("child deadline should match parent: got %v want %v (diff %v)", dlGot, dlWant, diff)
	}
}

func TestMustPing_PanicsOnPingError(t *testing.T) {
	t.Parallel()

	fp := &fakePinger{err: errBoom("boom")}
	assertPanicContains(t, "MustPing(error)", "pg ping failed: boom", func() {
		MustPing(context.Background(), "pg", fp)
	})
}

func (e errBoom) Error() string { return string(e) }

func TestMustGuard_SkippedUntilStoreSeamExists(t *testing.T) {
	t.Skip("MustGuard delegates to (*store.Store).Guard(ctx); needs a real store or seam to assert reliably")
}

// fakeGuard lets us force Guard() to succeed or fail
type fakeGuard struct{ err error }

func (f fakeGuard) Guard(context.Context) error { return f.err }

func TestMustGuard_PanicsOnError(t *testing.T) {
	t.Parallel()

	assertPanicContains(t, "MustGuard(error)", "dependency guard failed: boom", func() {
		MustGuard(context.Background(), fakeGuard{err: errBoom("boom")})
	})
}

func TestMustGuard_NoPanicOnNilError(t *testing.T) {
	t.Parallel()

	// should not panic when Guard returns nil
	MustGuard(context.Background(), fakeGuard{err: nil})
}

// minimal error type to avoid importing errors
type errBoom string
