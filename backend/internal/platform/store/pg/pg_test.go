package pg

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"swearjar/internal/platform/testkit"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOpen_ParseError(t *testing.T) {
	t.Parallel()

	_, err := Open(context.Background(), Config{URL: "://bad"}, nil, nil)
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
}

func TestOpen_NewPoolError(t *testing.T) {
	// This test mutates a global seam; run serially to avoid bleed
	testkit.Serial(t)

	testkit.Swap(t, &newPool, func(ctx context.Context, _ *pgxpool.Config) (*pgxpool.Pool, error) {
		return nil, errors.New("boom")
	})

	// URL must parse so we reach newPool
	dsn := "postgres://user:pass@host:5432/db?sslmode=disable"
	_, err := Open(context.Background(), Config{URL: dsn}, nil, nil)
	if err == nil {
		t.Fatalf("expected newPool error, got nil")
	}
}

func TestOpen_SuccessPath_NoDB_MutatorCalled(t *testing.T) {
	testkit.Serial(t)

	fake := &pgxpool.Pool{} // not initialized; do NOT close it
	testkit.Swap(t, &newPool, func(ctx context.Context, _ *pgxpool.Config) (*pgxpool.Pool, error) {
		return fake, nil
	})

	var mutCalled atomic.Bool
	cfg := Config{URL: "postgres://u:p@h:5432/db?sslmode=disable", MaxConns: 7, SlowMs: 123}
	p, err := Open(context.Background(), cfg, nil, func(pc *pgxpool.Config) {
		mutCalled.Store(true)
		if pc.MaxConns != cfg.MaxConns {
			t.Fatalf("MaxConns not applied: got %d want %d", pc.MaxConns, cfg.MaxConns)
		}
		pc.MaxConnIdleTime = 42 * time.Second
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	// no t.Cleanup(p.Close) here; fake pool is zero-value

	if !mutCalled.Load() {
		t.Fatalf("poolCfgMut was not invoked")
	}
	if p.SlowMs != cfg.SlowMs {
		t.Fatalf("SlowMs mismatch: got %d want %d", p.SlowMs, cfg.SlowMs)
	}
	if p.Pool == nil {
		t.Fatalf("Pool is nil")
	}
}

func TestClose_NilSafe_AndIdempotent(t *testing.T) {
	t.Parallel()

	var p *PG
	p.Close() // nil receiver safe

	p = &PG{} // nil Pool safe
	p.Close()
	p.Close() // idempotent-ish
}
