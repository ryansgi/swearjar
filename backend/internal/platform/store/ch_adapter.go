package store

import (
	"context"

	"swearjar/internal/platform/store/ch"
)

// chClient is the tiny surface we need from clickhouse
type chClient interface {
	Insert(ctx context.Context, table string, data any) error
	Query(ctx context.Context, sql string, args ...any) (ch.Rows, error)
	Close() error
}

// chAdapter wraps a chClient and implements store.Clickhouse
type chAdapter struct {
	c chClient
}

func newCHAdapter(c *ch.CH) *chAdapter { return &chAdapter{c: c} }

// Insert delegates to ch
func (a *chAdapter) Insert(ctx context.Context, table string, data any) error {
	return a.c.Insert(ctx, table, data)
}

// Query adapts ch.Rows to store.Rows
func (a *chAdapter) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	cr, err := a.c.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return chRows{r: cr}, nil
}

// Close delegates to ch
func (a *chAdapter) Close() error { return a.c.Close() }

// chRows implements store.Rows by delegating to ch.Rows
type chRows struct{ r ch.Rows }

func (x chRows) Next() bool            { return x.r.Next() }
func (x chRows) Scan(dst ...any) error { return x.r.Scan(dst...) }
func (x chRows) Err() error            { return x.r.Err() }
func (x chRows) Close()                { x.r.Close() }

// chColumns is an optional passthrough for column names if the underlying type provides it
type chColumns interface {
	Columns() []string
}

func (x chRows) Columns() []string {
	if c, ok := any(x.r).(chColumns); ok {
		return c.Columns()
	}
	return nil
}
