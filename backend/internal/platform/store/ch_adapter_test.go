package store

// import (
// 	"context"
// 	"errors"
// 	"testing"

// 	"swearjar/internal/platform/store/ch"
// )

// // TestNewAdapter_DelegatesInsert ensures Insert calls through to ch and returns the same error
// func TestNewAdapter_DelegatesInsert(t *testing.T) {
// 	t.Parallel()

// 	c := &ch.CH{}
// 	a := newCHAdapter(c)

// 	err := a.Insert(context.Background(), "some_table", struct{}{})
// 	if err == nil {
// 		t.Fatalf("Insert expected error, got nil")
// 	}
// 	// we can't match a sentinel yet, but it must be an error
// 	if !errors.Is(err, err) {
// 		t.Fatalf("Insert didn't return an error type we can observe: %v", err)
// 	}
// }

// // TestQuery_WrapsRows verifies the adapter wraps ch.Rows and behaves like empty rows
// func TestQuery_WrapsRows(t *testing.T) {
// 	t.Parallel()

// 	c := &ch.CH{}
// 	a := newCHAdapter(c)

// 	rows, err := a.Query(context.Background(), "SELECT 1")
// 	if err != nil {
// 		t.Fatalf("Query returned error: %v", err)
// 	}
// 	if rows == nil {
// 		t.Fatalf("Query returned nil rows")
// 	}
// 	defer rows.Close()

// 	if rows.Next() {
// 		t.Fatalf("Next returned true on empty rows")
// 	}
// 	var got int
// 	if scanErr := rows.Scan(&got); scanErr != nil {
// 		t.Fatalf("Scan returned error on empty rows: %v", scanErr)
// 	}
// 	if rows.Err() != nil {
// 		t.Fatalf("rows.Err not nil: %v", rows.Err())
// 	}
// }

// // TestQuery_WithArgs passes variadic args through without changing behavior
// func TestQuery_WithArgs(t *testing.T) {
// 	t.Parallel()

// 	c := &ch.CH{}
// 	a := newCHAdapter(c)

// 	rows, err := a.Query(context.Background(), "SELECT ? + ?", 1, 2)
// 	if err != nil {
// 		t.Fatalf("Query with args returned error: %v", err)
// 	}
// 	defer rows.Close()

// 	if rows.Next() {
// 		t.Fatalf("Next returned true on empty rows with args")
// 	}
// }

// // TestRows_ColumnsPassthroughNil confirms Columns() returns nil when underlying doesn't support it
// func TestRows_ColumnsPassthroughNil(t *testing.T) {
// 	t.Parallel()

// 	c := &ch.CH{}
// 	a := newCHAdapter(c)

// 	rows, err := a.Query(context.Background(), "SELECT 1")
// 	if err != nil {
// 		t.Fatalf("Query returned error: %v", err)
// 	}
// 	defer rows.Close()

// 	// chRows exposes Columns() on the adapter side
// 	type colser interface{ Columns() []string }
// 	if cr, ok := rows.(colser); ok {
// 		if cols := cr.Columns(); cols != nil {
// 			t.Fatalf("Columns expected nil for stub, got: %v", cols)
// 		}
// 	} else {
// 		t.Fatalf("rows does not expose Columns method")
// 	}
// }

// // TestClose_Delegates confirms the adapter Close calls through to ch
// func TestClose_Delegates(t *testing.T) {
// 	t.Parallel()

// 	c := &ch.CH{}
// 	a := newCHAdapter(c)

// 	if err := a.Close(); err != nil {
// 		t.Fatalf("Close returned error: %v", err)
// 	}
// }

// type fakeChRowsWithCols struct {
// 	nexts  int
// 	closed bool
// 	err    error
// }

// func (f *fakeChRowsWithCols) Next() bool             { f.nexts++; return false }
// func (f *fakeChRowsWithCols) Scan(dest ...any) error { return nil }
// func (f *fakeChRowsWithCols) Err() error             { return f.err }
// func (f *fakeChRowsWithCols) Close()                 { f.closed = true }
// func (f *fakeChRowsWithCols) Columns() []string      { return []string{"alpha", "beta"} }

// func TestCHRows_ColumnsPassthrough_NonNilAndDelegations(t *testing.T) {
// 	t.Parallel()

// 	f := &fakeChRowsWithCols{}
// 	x := chRows{r: f}

// 	// Columns should pass through to the underlying fake
// 	cols := x.Columns()
// 	if len(cols) != 2 || cols[0] != "alpha" || cols[1] != "beta" {
// 		t.Fatalf("Columns mismatch: %#v", cols)
// 	}

// 	// Delegation sanity: Next, Scan, Err, Close
// 	if x.Next() { // our fake returns false
// 		t.Fatalf("Next should be false on fake")
// 	}
// 	var v int
// 	if err := x.Scan(&v); err != nil {
// 		t.Fatalf("Scan returned error: %v", err)
// 	}
// 	if x.Err() != nil {
// 		t.Fatalf("Err should be nil")
// 	}
// 	x.Close()
// 	if !f.closed {
// 		t.Fatalf("Close did not delegate to underlying Rows")
// 	}
// }

// type fakeErrClient struct{}

// var errBoom = errors.New("boom")

// func (fakeErrClient) Insert(ctx context.Context, table string, data any) error {
// 	// not used in this test; just satisfy the interface
// 	return nil
// }

// func (fakeErrClient) Query(ctx context.Context, sql string, args ...any) (ch.Rows, error) {
// 	return nil, errBoom
// }

// func (fakeErrClient) Close() error { return nil }

// // TestQuery_ReturnsClientError ensures the adapter propagates underlying errors
// func TestQuery_ReturnsClientError(t *testing.T) {
// 	t.Parallel()

// 	a := &chAdapter{c: fakeErrClient{}}

// 	rows, err := a.Query(context.Background(), "SELECT 1")
// 	if err == nil {
// 		t.Fatalf("expected error, got nil")
// 	}
// 	if !errors.Is(err, errBoom) {
// 		t.Fatalf("expected boom error, got %v", err)
// 	}
// 	if rows != nil {
// 		t.Fatalf("expected nil rows on error, got %#v", rows)
// 	}
// }
