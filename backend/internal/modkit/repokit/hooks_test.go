package repokit

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"swearjar/internal/platform/store"
)

// fakeQHooks is a minimal Queryer used only by this file
type fakeQHooks struct {
	execCalls     int
	queryCalls    int
	queryRowCalls int

	lastSQL  string
	lastArgs []any
}

func (f *fakeQHooks) Exec(ctx context.Context, sql string, args ...any) (store.CommandTag, error) {
	f.execCalls++
	f.lastSQL = sql
	f.lastArgs = append([]any(nil), args...)
	var zero store.CommandTag
	return zero, nil
}

func (f *fakeQHooks) Query(ctx context.Context, sql string, args ...any) (store.Rows, error) {
	f.queryCalls++
	f.lastSQL = sql
	f.lastArgs = append([]any(nil), args...)
	var zero store.Rows
	return zero, nil
}

func (f *fakeQHooks) QueryRow(ctx context.Context, sql string, args ...any) store.Row {
	f.queryRowCalls++
	f.lastSQL = sql
	f.lastArgs = append([]any(nil), args...)
	var zero store.Row
	return zero
}

// fakeTxRunnerHooks is a TxRunner used only by this file
type fakeTxRunnerHooks struct {
	q *fakeQHooks

	txCalls int

	execCalls  int
	queryCalls int
	rowCalls   int

	lastSQL  string
	lastArgs []any
}

func (f *fakeTxRunnerHooks) Tx(ctx context.Context, fn func(q Queryer) error) error {
	f.txCalls++
	return fn(f.q)
}

func (f *fakeTxRunnerHooks) Exec(ctx context.Context, sql string, args ...any) (store.CommandTag, error) {
	f.execCalls++
	f.lastSQL = sql
	f.lastArgs = append([]any(nil), args...)
	var zero store.CommandTag
	return zero, nil
}

func (f *fakeTxRunnerHooks) Query(ctx context.Context, sql string, args ...any) (store.Rows, error) {
	f.queryCalls++
	f.lastSQL = sql
	f.lastArgs = append([]any(nil), args...)
	var zero store.Rows
	return zero, nil
}

func (f *fakeTxRunnerHooks) QueryRow(ctx context.Context, sql string, args ...any) store.Row {
	f.rowCalls++
	f.lastSQL = sql
	f.lastArgs = append([]any(nil), args...)
	var zero store.Row
	return zero
}

func TestWithBeginHooks_TxRunsHooksInOrderAndThenFn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	q := &fakeQHooks{}
	inner := &fakeTxRunnerHooks{q: q}

	var seq []string

	h1 := func(ctx context.Context, gotQ Queryer) error {
		if gotQ != q {
			t.Fatalf("hook received different Queryer instance")
		}
		seq = append(seq, "hook1")
		return nil
	}
	h2 := func(ctx context.Context, gotQ Queryer) error {
		if gotQ != q {
			t.Fatalf("hook received different Queryer instance")
		}
		seq = append(seq, "hook2")
		return nil
	}

	runner := WithBeginHooks(inner, h1, h2)

	var fnRan bool
	err := runner.Tx(ctx, func(gotQ Queryer) error {
		if gotQ != q {
			t.Fatalf("fn received different Queryer instance")
		}
		fnRan = true
		seq = append(seq, "fn")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSeq := []string{"hook1", "hook2", "fn"}
	if !reflect.DeepEqual(seq, wantSeq) {
		t.Fatalf("sequence mismatch want=%v got=%v", wantSeq, seq)
	}
	if !fnRan {
		t.Fatalf("fn should have run")
	}
	if inner.txCalls != 1 {
		t.Fatalf("inner Tx should be called once")
	}
}

func TestWithBeginHooks_TxHookErrorShortCircuitsBeforeFn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	q := &fakeQHooks{}
	inner := &fakeTxRunnerHooks{q: q}

	testErr := errors.New("boom")
	var fnRan bool

	h1 := func(ctx context.Context, gotQ Queryer) error { return testErr }
	h2 := func(ctx context.Context, gotQ Queryer) error {
		t.Fatalf("second hook should not run when first fails")
		return nil
	}

	r := WithBeginHooks(inner, h1, h2)
	err := r.Tx(ctx, func(q Queryer) error { fnRan = true; return nil })

	if !errors.Is(err, testErr) {
		t.Fatalf("expected error to propagate from hook got=%v", err)
	}
	if fnRan {
		t.Fatalf("fn should not have run when hook fails")
	}
}

func TestWithBeginHooks_DelegatesExecQueryQueryRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inner := &fakeTxRunnerHooks{q: &fakeQHooks{}}
	r := WithBeginHooks(inner) // no hooks needed to test delegation

	// Exec
	_, err := r.Exec(ctx, "UPDATE x SET a=$1", 7)
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if inner.execCalls != 1 || inner.lastSQL != "UPDATE x SET a=$1" || !reflect.DeepEqual(inner.lastArgs, []any{7}) {
		t.Fatalf("Exec did not delegate correctly")
	}

	// Query
	_, err = r.Query(ctx, "SELECT * FROM x WHERE a=$1", 9)
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if inner.queryCalls != 1 || inner.lastSQL != "SELECT * FROM x WHERE a=$1" ||
		!reflect.DeepEqual(inner.lastArgs, []any{9}) {
		t.Fatalf("Query did not delegate correctly")
	}

	// QueryRow
	_ = r.QueryRow(ctx, "SELECT * FROM x WHERE id=$1", "abc")
	if inner.rowCalls != 1 || inner.lastSQL != "SELECT * FROM x WHERE id=$1" ||
		!reflect.DeepEqual(inner.lastArgs, []any{"abc"}) {
		t.Fatalf("QueryRow did not delegate correctly")
	}
}

func TestRunMidHooks_SuccessAndShortCircuit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	q := &fakeQHooks{}
	seq := []string{}

	// success path
	m1 := func(ctx context.Context, _ Queryer) error { seq = append(seq, "m1"); return nil }
	m2 := func(ctx context.Context, _ Queryer) error { seq = append(seq, "m2"); return nil }

	if err := RunMidHooks(ctx, q, m1, m2); err != nil {
		t.Fatalf("RunMidHooks returned error on success path: %v", err)
	}
	if !reflect.DeepEqual(seq, []string{"m1", "m2"}) {
		t.Fatalf("mid hooks did not run in order")
	}

	// error short circuit
	seq = seq[:0]
	testErr := errors.New("mid boom")
	mErr := func(ctx context.Context, _ Queryer) error { seq = append(seq, "mErr"); return testErr }
	mNever := func(ctx context.Context, _ Queryer) error {
		t.Fatalf("mid hook after error should not run")
		return nil
	}

	err := RunMidHooks(ctx, q, m1, mErr, mNever)
	if !errors.Is(err, testErr) {
		t.Fatalf("expected error to propagate from mid hook got=%v", err)
	}
	if !reflect.DeepEqual(seq, []string{"m1", "mErr"}) {
		t.Fatalf("mid hooks should stop on first error")
	}
}
