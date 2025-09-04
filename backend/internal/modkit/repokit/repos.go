// Package repokit provides common types and helpers for repository implementations
package repokit

import (
	"context"

	"swearjar/internal/platform/store"
	ch "swearjar/internal/platform/store/ch"
)

// Queryer is the minimal read and write surface for SQL repos
type Queryer = store.RowQuerier

// RowQuerier is kept for compatibility with existing callers
type RowQuerier = store.RowQuerier

// TxRunner can execute a function inside a transaction
type TxRunner = store.TxRunner

type (
	// Rows are the result set of a query
	Rows = store.Rows

	// Row is a single row result from a query
	Row = store.Row

	// CommandTag is the result of a command that modifies data
	CommandTag = store.CommandTag
)

// WithTx runs fn inside a transaction using the provided TxRunner
func WithTx(ctx context.Context, tx TxRunner, fn func(q Queryer) error) error {
	return tx.Tx(ctx, fn)
}

// PG exposes a RowQuerier for Postgres without importing a driver
func PG(_ context.Context, q store.RowQuerier) store.RowQuerier { return q }

// TX exposes a TxRunner without importing a driver
func TX(_ context.Context, tx store.TxRunner) store.TxRunner { return tx }

// CH exposes ClickHouse seam if needed by a repo
func CH(_ context.Context, db *ch.CH) *ch.CH { return db }
