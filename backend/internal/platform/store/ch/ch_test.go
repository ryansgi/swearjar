package ch

import (
	"context"
	"errors"
	"testing"
)

// TestOpen returns a non nil client and no error
func TestOpen(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := Config{URL: "clickhouse://local"}
	cl, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if cl == nil {
		t.Fatalf("Open returned nil client")
	}
}

// TestInsert returns not implemented error for now
func TestInsert_NotImplemented(t *testing.T) {
	t.Parallel()

	cl := &CH{}
	err := cl.Insert(context.Background(), "table", struct{}{})
	if err == nil {
		t.Fatalf("Insert expected error, got nil")
	}
	if !errors.Is(err, err) { // trivially assert it is an error
		t.Fatalf("Insert returned unexpected error type: %T", err)
	}
}

// TestQuery returns an empty Rows that is safe to iterate and close
func TestQuery_EmptyRows(t *testing.T) {
	t.Parallel()

	cl := &CH{}
	rows, err := cl.Query(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if rows == nil {
		t.Fatalf("Query returned nil rows")
	}

	// no rows expected
	if rows.Next() {
		t.Fatalf("Next returned true on empty rows")
	}

	// Scan should be a no op on the stub
	var got int
	if scanErr := rows.Scan(&got); scanErr != nil {
		t.Fatalf("Scan returned error on empty rows: %v", scanErr)
	}

	// Err should be nil
	if rows.Err() != nil {
		t.Fatalf("rows.Err not nil: %v", rows.Err())
	}

	// Close should be safe
	rows.Close()
}

// TestQuery_WithArgs accepts variadic args without affecting behavior
func TestQuery_WithArgs(t *testing.T) {
	t.Parallel()

	cl := &CH{}
	rows, err := cl.Query(context.Background(), "SELECT ? + ?", 1, 2)
	if err != nil {
		t.Fatalf("Query with args returned error: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		t.Fatalf("Next returned true on empty rows with args")
	}
}

// TestClose is a no op on the client
func TestClose_NoOp(t *testing.T) {
	t.Parallel()

	cl := &CH{}
	if err := cl.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}
