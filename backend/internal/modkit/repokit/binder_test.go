package repokit

import (
	"context"
	"testing"

	"swearjar/internal/platform/store"
)

type fakeQ struct{}

func (f *fakeQ) Exec(ctx context.Context, sql string, args ...any) (store.CommandTag, error) {
	var z store.CommandTag
	return z, nil
}

func (f *fakeQ) Query(ctx context.Context, sql string, args ...any) (store.Rows, error) {
	var z store.Rows
	return z, nil
}

func (f *fakeQ) QueryRow(ctx context.Context, sql string, args ...any) store.Row {
	var z store.Row
	return z
}

var _ Queryer = (*fakeQ)(nil)

func mustPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

func TestBindFunc_BindCallsFunc(t *testing.T) {
	t.Parallel()

	// create a binder from a function; it should be invoked with the provided Queryer
	var q Queryer // nil is fine; BindFunc doesn't use it
	b := BindFunc[string](func(_ Queryer) string {
		return "ok"
	})

	got := b.Bind(q)
	if got != "ok" {
		t.Fatalf("BindFunc.Bind = %q, want %q", got, "ok")
	}
}

func TestRequireQueryer_PanicsOnNil(t *testing.T) {
	t.Parallel()

	var q Queryer // nil interface
	mustPanic(t, "RequireQueryer(nil)", func() {
		_ = RequireQueryer(q)
	})
}

func TestMustBind_PanicsOnNilQueryer(t *testing.T) {
	t.Parallel()

	var q Queryer // nil interface
	b := BindFunc[int](func(_ Queryer) int { return 42 })

	mustPanic(t, "MustBind(nil Queryer)", func() {
		_ = MustBind[int](b, q)
	})
}

func TestRequireQueryer_ReturnsSame(t *testing.T) {
	t.Parallel()

	var in Queryer = &fakeQ{} // non-nil
	out := RequireQueryer(in)

	if out == nil {
		t.Fatalf("RequireQueryer returned nil for non-nil input")
	}
	if out != in {
		t.Fatalf("RequireQueryer did not return the same instance")
	}
}
