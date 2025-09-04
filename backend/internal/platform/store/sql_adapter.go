package store

import (
	"context"
	"errors"
	"time"

	"swearjar/internal/platform/store/pg"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgAdapter wraps pg.PG and implements RowQuerier + TxRunner
// it also emits query trace events when a tracer is configured on pg.PG
type pgAdapter struct {
	p *pg.PG
}

func newPGAdapter(p *pg.PG) *pgAdapter { return &pgAdapter{p: p} }

func (a *pgAdapter) Ping(ctx context.Context) error {
	if a == nil {
		return errors.New("pg: nil adapter")
	}
	var one int
	return a.QueryRow(ctx, "SELECT 1").Scan(&one)
}

func (a *pgAdapter) Close() error { a.p.Close(); return nil }

func (a *pgAdapter) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	start := time.Now()
	ct, err := a.p.Pool.Exec(ctx, sql, args...)
	a.emit(ctx, sql, args, start, err)
	return tag{ct}, err
}

func (a *pgAdapter) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	start := time.Now()
	rs, err := a.p.Pool.Query(ctx, sql, args...)
	// emit on open; if you want end-to-end timing across scan, wrap Close and emit there instead
	a.emit(ctx, sql, args, start, err)
	if err != nil {
		return nil, err
	}
	return rows{r: rs}, nil
}

func (a *pgAdapter) QueryRow(ctx context.Context, sql string, args ...any) Row {
	start := time.Now()
	r := a.p.Pool.QueryRow(ctx, sql, args...)
	// wrap to emit after Scan completes, capturing error from Scan
	return row{
		r: r,
		after: func(scanErr error) {
			a.emit(ctx, sql, args, start, scanErr)
		},
	}
}

func (a *pgAdapter) Tx(ctx context.Context, fn func(q RowQuerier) error) error {
	tx, err := a.p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	q := txQuerier{
		tx:     tx,
		tracer: a.p.Tracer,
		slowUS: int64(a.p.SlowMs) * 1000,
	}
	if err := fn(q); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// emit sends a query event to the configured tracer
func (a *pgAdapter) emit(ctx context.Context, sql string, args []any, start time.Time, err error) {
	if a == nil || a.p == nil || a.p.Tracer == nil {
		return
	}
	elapsedUS := time.Since(start).Microseconds()
	slow := a.p.SlowMs >= 0 && elapsedUS >= int64(a.p.SlowMs)*1000
	a.p.Tracer.OnQuery(ctx, pg.QueryEvent{
		SQL:       sql,
		Args:      args,
		ElapsedUS: elapsedUS,
		Err:       err,
		Slow:      slow,
	})
}

// adapters for pgx to our tiny Row/Rows/CommandTag

type row struct {
	r     pgx.Row
	after func(error)
}

func (x row) Scan(dst ...any) error {
	err := x.r.Scan(dst...)
	if x.after != nil {
		x.after(err)
	}
	return err
}

type rows struct{ r pgx.Rows }

func (x rows) Next() bool            { return x.r.Next() }
func (x rows) Scan(dst ...any) error { return x.r.Scan(dst...) }
func (x rows) Err() error            { return x.r.Err() }
func (x rows) Close()                { x.r.Close() }
func (x rows) Columns() []string {
	f := x.r.FieldDescriptions()
	out := make([]string, len(f))
	for i := range f {
		out[i] = string(f[i].Name)
	}
	return out
}

// wrap pgconn.CommandTag so we satisfy our CommandTag interface
type tag struct{ t pgconn.CommandTag }

func (t tag) String() string      { return t.t.String() }
func (t tag) RowsAffected() int64 { return t.t.RowsAffected() }

// txQuerier uses pgx.Tx to satisfy RowQuerier inside a Tx
// it mirrors pgAdapter emit behavior so queries inside transactions are also traced
type txQuerier struct {
	tx     pgx.Tx
	tracer pg.QueryTracer
	slowUS int64
}

func (t txQuerier) Exec(ctx context.Context, sql string, args ...any) (CommandTag, error) {
	start := time.Now()
	ct, err := t.tx.Exec(ctx, sql, args...)
	t.emit(ctx, sql, args, start, err)
	return tag{ct}, err
}

func (t txQuerier) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	start := time.Now()
	rs, err := t.tx.Query(ctx, sql, args...)
	t.emit(ctx, sql, args, start, err)
	if err != nil {
		return nil, err
	}
	return rows{r: rs}, nil
}

func (t txQuerier) QueryRow(ctx context.Context, sql string, args ...any) Row {
	start := time.Now()
	r := t.tx.QueryRow(ctx, sql, args...)
	return row{
		r: r,
		after: func(scanErr error) {
			t.emit(ctx, sql, args, start, scanErr)
		},
	}
}

func (t txQuerier) emit(ctx context.Context, sql string, args []any, start time.Time, err error) {
	if t.tracer == nil {
		return
	}
	elapsedUS := time.Since(start).Microseconds()
	slow := t.slowUS >= 0 && elapsedUS >= t.slowUS
	t.tracer.OnQuery(ctx, pg.QueryEvent{
		SQL:       sql,
		Args:      args,
		ElapsedUS: elapsedUS,
		Err:       err,
		Slow:      slow,
	})
}
