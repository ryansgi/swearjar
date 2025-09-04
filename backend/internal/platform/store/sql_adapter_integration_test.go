//go:build integration_pg
// +build integration_pg

package store

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"swearjar/internal/platform/logger"

	"github.com/rs/zerolog"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startPostgres launches a disposable Postgres and returns DSN + stop func
func startPostgres(t *testing.T) (dsn string, stop func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)

	req := tc.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "postgres",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections"),
		).WithDeadline(2 * time.Minute),
	}
	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		cancel()
		t.Fatalf("failed to start postgres container: %v", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(context.Background())
		cancel()
		t.Fatalf("failed to get container host: %v", err)
	}
	mp, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = c.Terminate(context.Background())
		cancel()
		t.Fatalf("failed to get mapped port: %v", err)
	}

	dsn = fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, mp.Port())
	stop = func() {
		_ = c.Terminate(context.Background())
		cancel()
	}
	return dsn, stop
}

func newTestStoreLogger() logger.Logger {
	// quiet, deterministic logs
	return zerolog.New(io.Discard)
}

func TestSQLAdapter_Integration_ExecQueryColumnsClose(t *testing.T) {
	dsn, stop := startPostgres(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Build store + config and use openPG from openers.go
	s := &Store{Log: newTestStoreLogger()}
	cfg := Config{
		PG: PGConfig{
			URL:         dsn,
			MaxConns:    2,
			SlowQueryMs: 0,
			LogSQL:      true, // hit tracer wiring path
		},
	}
	txr, err := openPG(ctx, cfg, s)
	if err != nil {
		t.Fatalf("openPG failed: %v", err)
	}
	// We need Exec/Query/QueryRow, which live on the adapter; openPG returns TxRunner
	a, ok := txr.(*pgAdapter)
	if !ok {
		t.Fatalf("openPG did not return *pgAdapter, got %T", txr)
	}
	t.Cleanup(func() { _ = a.Close() })

	// Create temp table
	if _, err := a.Exec(ctx, `
		CREATE TEMP TABLE sql_adapter_t (
			id   SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		)
	`); err != nil {
		t.Fatalf("create temp table: %v", err)
	}

	// Insert a couple rows
	if _, err := a.Exec(ctx, `INSERT INTO sql_adapter_t (name) VALUES ($1), ($2)`, "zoe", "ada"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// QueryRow flow
	var first string
	if err := a.QueryRow(ctx, `SELECT name FROM sql_adapter_t WHERE id=$1`, 1).Scan(&first); err != nil {
		t.Fatalf("queryrow scan: %v", err)
	}
	if first != "zoe" {
		t.Fatalf("unexpected name: %q", first)
	}

	// Query + Columns()
	rs, err := a.Query(ctx, `SELECT id, name FROM sql_adapter_t ORDER BY id`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rs.Close()

	cols := rs.Columns()
	if len(cols) != 2 || cols[0] != "id" || cols[1] != "name" {
		t.Fatalf("columns mismatch: %#v", cols)
	}

	var (
		ids   []int
		names []string
	)
	for rs.Next() {
		var id int
		var name string
		if err := rs.Scan(&id, &name); err != nil {
			t.Fatalf("rows scan: %v", err)
		}
		ids = append(ids, id)
		names = append(names, name)
	}
	if err := rs.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if len(ids) != 2 || names[0] != "zoe" || names[1] != "ada" {
		t.Fatalf("rows mismatch ids=%v names=%v", ids, names)
	}

	// Close is safe, and calling twice should be fine through PG.Close behavior
	if err := a.Close(); err != nil {
		t.Fatalf("adapter close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("adapter close second: %v", err)
	}
}

func TestSQLAdapter_Integration_TxCommitAndRollback(t *testing.T) {
	dsn, stop := startPostgres(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	s := &Store{Log: newTestStoreLogger()}
	cfg := Config{PG: PGConfig{URL: dsn, MaxConns: 2}}
	txr, err := openPG(ctx, cfg, s)
	if err != nil {
		t.Fatalf("openPG failed: %v", err)
	}
	a := txr.(*pgAdapter)
	t.Cleanup(func() { _ = a.Close() })

	// Isolated temp table for this test
	if _, err := a.Exec(ctx, `
		CREATE TEMP TABLE sql_adapter_tx (
			id  SERIAL PRIMARY KEY,
			val INT NOT NULL
		)
	`); err != nil {
		t.Fatalf("create temp table: %v", err)
	}

	// Commit path
	if err := a.Tx(ctx, func(q RowQuerier) error {
		_, err := q.Exec(ctx, `INSERT INTO sql_adapter_tx (val) VALUES (10)`)
		return err
	}); err != nil {
		t.Fatalf("tx commit: %v", err)
	}

	var count int
	if err := a.QueryRow(ctx, `SELECT COUNT(*) FROM sql_adapter_tx WHERE val=10`).Scan(&count); err != nil {
		t.Fatalf("count committed: %v", err)
	}
	if count != 1 {
		t.Fatalf("commit failed count=%d want=1", count)
	}

	// Rollback path
	_ = a.Tx(ctx, func(q RowQuerier) error {
		if _, err := q.Exec(ctx, `INSERT INTO sql_adapter_tx (val) VALUES (20)`); err != nil {
			return err
		}
		return errRollback
	})

	count = 0
	if err := a.QueryRow(ctx, `SELECT COUNT(*) FROM sql_adapter_tx WHERE val=20`).Scan(&count); err != nil {
		t.Fatalf("count rolled back: %v", err)
	}
	if count != 0 {
		t.Fatalf("rollback failed count=%d want=0", count)
	}
}

var errRollback = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "rollback" }
