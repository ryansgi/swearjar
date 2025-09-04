// Package store provides a unified interface to optional storage backends
package store

import (
	"context"
	"errors"
	"fmt"

	"swearjar/internal/platform/logger"
)

// Store is the facade for optional backends
// zero value is safe but does nothing
type Store struct {
	// Log is the logger used by subclients
	// zero means a no op zerolog logger
	Log logger.Logger

	// PG is the postgres sql seam, nil when disabled
	PG TxRunner

	// CH is the clickhouse seam, nil when disabled
	CH Clickhouse
}

// Row exposes the minimal scan contract a single row needs
type Row interface {
	Scan(dest ...any) error
}

// Rows exposes the minimal iteration and scan for a result set
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
	Columns() []string
}

// CommandTag is a tiny interface to inspect command results
type CommandTag interface {
	String() string
	RowsAffected() int64
}

// RowQuerier is the read and write surface repos use for sql
type RowQuerier interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// TxRunner wraps transaction execution around a function
type TxRunner interface {
	RowQuerier
	Tx(ctx context.Context, fn func(q RowQuerier) error) error
}

// Clickhouse is a tiny seam for columnar writes and queries
type Clickhouse interface {
	Insert(ctx context.Context, table string, data any) error
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	Close() error
}

// Pinger is any seam that can report readiness
type Pinger interface{ Ping(context.Context) error }

// Open constructs a Store with the requested backends
// backends not enabled in cfg remain nil on the Store
func Open(ctx context.Context, cfg Config, opts ...Option) (*Store, error) {
	s := &Store{}
	for _, o := range opts {
		if err := o(s); err != nil {
			return nil, err
		}
	}

	// defaults for zero logger to avoid nil checks
	s.Log = s.Log.With().Logger()

	if cfg.PG.Enabled {
		pgClient, err := openPG(ctx, cfg, s)
		if err != nil {
			return nil, err
		}
		s.PG = pgClient
	}

	if cfg.CH.Enabled {
		chClient, err := openCH(ctx, cfg, s)
		if err != nil {
			return nil, err
		}
		s.CH = chClient
	}

	return s, nil
}

// Guard verifies all configured seams the Store knows about.
// (Right now: PG; add NATS/Redis/CH below when they expose Ping.)
func (s *Store) Guard(ctx context.Context) error {
	if s == nil {
		return errors.New("nil store")
	}
	var errs []error
	if s.PG != nil {
		if p, ok := any(s.PG).(Pinger); ok {
			if err := p.Ping(ctx); err != nil {
				errs = append(errs, fmt.Errorf("pg: %w", err))
			}
		}
	}
	// TODO: if/when these exist & implement Ping(ctx) error:
	// if s.CH   != nil { ... }

	return errors.Join(errs...)
}

// Close closes all initialized backends gracefully
// nil backends are ignored
func (s *Store) Close(ctx context.Context) error {
	var errs []error

	if s.CH != nil {
		if e := s.CH.Close(); e != nil {
			errs = append(errs, e)
		}
	}

	if c, ok := s.PG.(interface{ Close() error }); ok {
		if e := c.Close(); e != nil {
			errs = append(errs, e)
		}
	}

	return errors.Join(errs...)
}
