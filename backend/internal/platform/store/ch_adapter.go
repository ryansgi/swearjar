package store

import (
	"context"
	"errors"

	"swearjar/internal/platform/store/ch"
)

// newCHAdapter is called by openers.go to wrap an existing *ch.CH
// and return the store.Clickhouse seam (single return value)
func newCHAdapter(c *ch.CH) Clickhouse {
	return &clickhouseAdapter{inner: c}
}

// clickhouseAdapter adapts *ch.CH to the store.Clickhouse interface
type clickhouseAdapter struct {
	inner *ch.CH
}

var _ Clickhouse = (*clickhouseAdapter)(nil)

func (a *clickhouseAdapter) Insert(ctx context.Context, table string, data any) error {
	rows, ok := data.([][]any)
	if !ok {
		return errors.New("store: unsupported CH insert shape (want [][]any)")
	}
	return a.inner.Insert(ctx, table, rows)
}

func (a *clickhouseAdapter) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	r, err := a.inner.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &rowsAdapter{r: r}, nil
}

func (a *clickhouseAdapter) Close() error { return a.inner.Close() }

// rowsAdapter wraps ch.Rows as store.Rows
type rowsAdapter struct {
	r ch.Rows
}

func (r *rowsAdapter) Next() bool             { return r.r.Next() }
func (r *rowsAdapter) Scan(dest ...any) error { return r.r.Scan(dest...) }
func (r *rowsAdapter) Err() error             { return r.r.Err() }
func (r *rowsAdapter) Close()                 { _ = r.r.Close() }
func (r *rowsAdapter) Columns() []string      { return r.r.Columns() }

// Ping verifies connectivity with ClickHouse
func (a *clickhouseAdapter) Ping(ctx context.Context) error {
	if a == nil || a.inner == nil {
		return errors.New("store: nil clickhouse adapter")
	}

	// SELECT 1 in CH is UInt8
	rows, err := a.inner.Query(ctx, "SELECT toInt32(1)")
	if err != nil {
		return err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			if err == nil {
				err = cerr
			} else {
				err = errors.Join(err, cerr)
			}
		}
	}()

	if !rows.Next() {
		return errors.New("store: ch ping returned no rows")
	}

	var one int32
	if scanErr := rows.Scan(&one); scanErr != nil {
		return scanErr
	}
	if nextErr := rows.Err(); nextErr != nil {
		return nextErr
	}
	return err
}
