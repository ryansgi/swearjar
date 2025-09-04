// Package pg provides a Postgres client using pgxpool with optional query tracing
package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config configures pgxpool for pg
type Config struct {
	URL      string
	MaxConns int32
	SlowMs   int
}

// PG is a postgres client with pool and optional tracer
type PG struct {
	Pool   *pgxpool.Pool
	Tracer QueryTracer
	SlowMs int
}

// Option mutates PG during Open
type Option func(*PG) error

var newPool = pgxpool.NewWithConfig

// Open creates a new PG client with the given config, optional tracer, and optional pool config mutator
func Open(ctx context.Context, cfg Config, tracer QueryTracer, poolCfgMut func(*pgxpool.Config)) (*PG, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, err
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	if poolCfgMut != nil {
		poolCfgMut(pcfg)
	}
	pool, err := newPool(ctx, pcfg) // use seam
	if err != nil {
		return nil, err
	}
	return &PG{
		Pool:   pool,
		Tracer: tracer,
		SlowMs: cfg.SlowMs,
	}, nil
}

// Close closes the pool
func (p *PG) Close() {
	if p != nil && p.Pool != nil {
		p.Pool.Close()
	}
}
