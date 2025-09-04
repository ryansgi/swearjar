package repokit

import (
	"context"
	"errors"
	"testing"

	"swearjar/internal/platform/store"
	ch "swearjar/internal/platform/store/ch"
)

func TestPG_ReturnsSameRowQuerier(t *testing.T) {
	t.Parallel()
	var q store.RowQuerier = nil // typed nil is fine; we only check identity
	if got := PG(context.Background(), q); got != q {
		t.Fatalf("PG should return the same RowQuerier instance")
	}
}

func TestTX_ReturnsSameTxRunner(t *testing.T) {
	t.Parallel()
	var tx store.TxRunner = nil
	if got := TX(context.Background(), tx); got != tx {
		t.Fatalf("TX should return the same TxRunner instance")
	}
}

func TestCH_ReturnsSameHandle(t *testing.T) {
	t.Parallel()
	var db *ch.CH = nil
	if got := CH(context.Background(), db); got != db {
		t.Fatalf("CH should return the same *ch.CH instance")
	}
}

// fakeTxRunner records calls and forwards to the provided fn with its q
type fakeTxRunner struct {
	q      Queryer
	err    error
	called int
}

func (f *fakeTxRunner) Tx(ctx context.Context, fn func(q Queryer) error) error {
	f.called++
	if fn != nil {
		if err := fn(f.q); err != nil {
			return err
		}
	}
	return f.err
}

func (f *fakeTxRunner) Exec(ctx context.Context, sql string, args ...any) (store.CommandTag, error) {
	if f.q != nil {
		return f.q.Exec(ctx, sql, args...)
	}
	var z store.CommandTag
	return z, nil
}

func (f *fakeTxRunner) Query(ctx context.Context, sql string, args ...any) (store.Rows, error) {
	if f.q != nil {
		return f.q.Query(ctx, sql, args...)
	}
	var z store.Rows
	return z, nil
}

func (f *fakeTxRunner) QueryRow(ctx context.Context, sql string, args ...any) store.Row {
	if f.q != nil {
		return f.q.QueryRow(ctx, sql, args...)
	}
	var z store.Row
	return z
}

func TestWithTx_DelegatesAndPassesQueryer(t *testing.T) {
	t.Parallel()

	ftx := &fakeTxRunner{q: &fakeQ{}}
	seen := false

	err := WithTx(context.Background(), ftx, func(q Queryer) error {
		// ensure the same q we injected is surfaced to fn
		if q != ftx.q {
			t.Fatalf("fn received unexpected Queryer")
		}
		seen = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithTx returned unexpected error: %v", err)
	}
	if ftx.called != 1 {
		t.Fatalf("TxRunner.Tx call count = %d want 1", ftx.called)
	}
	if !seen {
		t.Fatalf("callback was not invoked")
	}
}

func TestWithTx_PropagatesFnError(t *testing.T) {
	t.Parallel()

	ftx := &fakeTxRunner{q: &fakeQ{}}
	want := errors.New("boom")

	err := WithTx(context.Background(), ftx, func(q Queryer) error {
		return want
	})

	if err == nil || !errors.Is(err, want) {
		t.Fatalf("WithTx did not propagate fn error, got %v want %v", err, want)
	}
	if ftx.called != 1 {
		t.Fatalf("TxRunner.Tx call count = %d want 1", ftx.called)
	}
}

func TestWithTx_PropagatesTxRunnerError(t *testing.T) {
	t.Parallel()

	want := errors.New("txerr")
	ftx := &fakeTxRunner{q: &fakeQ{}, err: want}

	err := WithTx(context.Background(), ftx, func(q Queryer) error { return nil })

	if err == nil || !errors.Is(err, want) {
		t.Fatalf("WithTx did not return TxRunner error, got %v want %v", err, want)
	}
	if ftx.called != 1 {
		t.Fatalf("TxRunner.Tx call count = %d want 1", ftx.called)
	}
}
