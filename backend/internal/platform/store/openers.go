package store

import (
	"context"
	"fmt"
	"time"

	chx "swearjar/internal/platform/store/ch"
	"swearjar/internal/platform/store/pg"
)

// openPG opens pg and wraps it with our sql adapter
func openPG(ctx context.Context, cfg Config, s *Store) (TxRunner, error) {
	var tracer pg.QueryTracer
	if cfg.PG.LogSQL {
		tracer = pg.Tracer(s.Log)
	}

	p, err := pg.Open(ctx, pg.Config{
		URL:      cfg.PG.URL,
		MaxConns: cfg.PG.MaxConns,
		SlowMs:   cfg.PG.SlowQueryMs,
	}, tracer, nil)
	if err != nil {
		return nil, err
	}

	// Connection guardrails: ping with retry/backoff using the *pool* directly
	const (
		maxAttempts    = 20
		pingTimeout    = 3 * time.Second
		backoffStart   = 150 * time.Millisecond
		backoffCeiling = 2 * time.Second
	)

	var lastErr error
	backoff := backoffStart
	for i := 0; i < maxAttempts; i++ {
		toCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		lastErr = p.Pool.Ping(toCtx) // <-- no adapter, no SQL trace line
		cancel()

		if lastErr == nil {
			a := newPGAdapter(p) // publish adapter only after the pool is healthy
			s.PG = a
			return a, nil
		}
		if ctx.Err() != nil {
			p.Close() // close the pool we opened
			return nil, ctx.Err()
		}
		time.Sleep(backoff)
		if backoff < backoffCeiling {
			backoff *= 2
			if backoff > backoffCeiling {
				backoff = backoffCeiling
			}
		}
	}

	p.Close()
	return nil, fmt.Errorf("postgres ping failed after %d attempts: %w", maxAttempts, lastErr)
}

func openCH(ctx context.Context, cfg Config, _ *Store) (Clickhouse, error) {
	c, err := chx.Open(ctx, chx.Config{URL: cfg.CH.URL})
	if err != nil {
		return nil, err
	}
	return newCHAdapter(c), nil
}
