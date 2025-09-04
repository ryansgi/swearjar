package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeTxNoPing satisfies TxRunner but not Pinger
type fakeTxNoPing struct{}

func (f *fakeTxNoPing) Tx(ctx context.Context, fn func(q RowQuerier) error) error { return nil }
func (f *fakeTxNoPing) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	var z CommandTag
	return z, nil
}

func (f *fakeTxNoPing) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	var z Rows
	return z, nil
}

func (f *fakeTxNoPing) QueryRow(ctx context.Context, sql string, args ...any) Row {
	var z Row
	return z
}

// fakeTxWithPing satisfies TxRunner and Pinger
type fakeTxWithPing struct {
	fakeTxNoPing
	err error
}

func (f *fakeTxWithPing) Ping(context.Context) error { return f.err }

func TestGuard_NilStore(t *testing.T) {
	t.Parallel()

	var s *Store = nil
	if err := s.Guard(context.Background()); err == nil {
		t.Fatalf("nil store should return error")
	}
}

func TestGuard_NoSeams(t *testing.T) {
	t.Parallel()

	s := &Store{}
	if err := s.Guard(context.Background()); err != nil {
		t.Fatalf("expected nil error when no seams are set, got %v", err)
	}
}

func TestGuard_PG_NotPinger_Ignored(t *testing.T) {
	t.Parallel()

	s := &Store{PG: &fakeTxNoPing{}}
	if err := s.Guard(context.Background()); err != nil {
		t.Fatalf("expected nil error when PG is not a Pinger, got %v", err)
	}
}

func TestGuard_PG_PingOK(t *testing.T) {
	t.Parallel()

	s := &Store{PG: &fakeTxWithPing{err: nil}}
	if err := s.Guard(context.Background()); err != nil {
		t.Fatalf("expected nil error when PG.Ping succeeds, got %v", err)
	}
}

func TestGuard_PG_PingError_Wrapped(t *testing.T) {
	t.Parallel()

	s := &Store{PG: &fakeTxWithPing{err: errors.New("boom")}}
	err := s.Guard(context.Background())
	if err == nil {
		t.Fatalf("expected non-nil error when PG.Ping fails")
	}
	// Guard prefixes PG errors with "pg: "
	if !strings.HasPrefix(err.Error(), "pg: ") {
		t.Fatalf("expected error to be prefixed with 'pg: ', got %q", err.Error())
	}
}
