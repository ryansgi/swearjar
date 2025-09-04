// Package ch provides a clickhouse client
package ch

import (
	"context"
	"errors"
)

// Config configures clickhouse client
type Config struct {
	URL string
}

// Rows is the minimal result set iteration for ch
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// CH is a placeholder seam for clickhouse connectivity
type CH struct{}

// Open returns a clickhouse client stub
func Open(_ context.Context, _ Config) (*CH, error) {
	return &CH{}, nil
}

// Insert inserts data into a table using the driver specific format
func (c *CH) Insert(_ context.Context, _ string, _ any) error {
	return errors.New("ch insert not implemented")
}

// Query runs a query and returns ch.Rows
func (c *CH) Query(_ context.Context, _ string, args ...any) (Rows, error) {
	// stub implementation returns an empty rows set
	return &emptyRows{}, nil
}

// Close closes resources
func (c *CH) Close() error { return nil }

// emptyRows is a stub that returns no results
type emptyRows struct{}

func (*emptyRows) Next() bool             { return false }
func (*emptyRows) Scan(dest ...any) error { return nil }
func (*emptyRows) Err() error             { return nil }
func (*emptyRows) Close()                 {}
