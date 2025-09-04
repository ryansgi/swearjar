package pg

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTestDB opens a PG client for tests, applies an optional pool mutator, and runs fn
// The client is closed automatically on test cleanup
func WithTestDB(t *testing.T, dsn string, poolMut func(*pgxpool.Config), fn func(p *PG)) {
	t.Helper()
	ctx := context.Background()
	client, err := Open(ctx, Config{URL: dsn}, nil, poolMut)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	fn(client)
}

// AcquireConn returns one acquired connection and releases it on cleanup
// Useful to keep TEMP tables and session settings on a single session
func AcquireConn(t *testing.T, p *PG, ctx context.Context) *pgxpool.Conn {
	t.Helper()
	conn, err := p.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	t.Cleanup(func() { conn.Release() })
	return conn
}
