package guardrails

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/platform/store"
)

// MakeAdvisoryLease returns a function that wraps work in a tx-scoped advisory lock on hour.
// If another worker holds the lock, it returns a benign error so the caller can retry/backoff
func MakeAdvisoryLease(deps modkit.Deps) func(context.Context, time.Time, func(context.Context) error) error {
	return func(ctx context.Context, hour time.Time, do func(context.Context) error) error {
		key := advisoryKey(hour)

		return deps.PG.Tx(ctx, func(q store.RowQuerier) error {
			// Try to acquire transaction-scoped advisory lock
			rows, err := q.Query(ctx, `SELECT pg_try_advisory_xact_lock($1)`, key)
			if err != nil {
				return err
			}
			defer rows.Close()

			var ok bool
			if rows.Next() {
				if err := rows.Scan(&ok); err != nil {
					return err
				}
			}
			if !ok {
				// Another worker holds the lease; bubble a retryable-ish error
				return errors.New("backfill: hour lease already held")
			}

			// We hold the lock for the duration of this transaction.
			return do(ctx)
		})
	}
}

func advisoryKey(t time.Time) int64 {
	sum := sha1.Sum([]byte(t.UTC().Format(time.RFC3339)))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}
