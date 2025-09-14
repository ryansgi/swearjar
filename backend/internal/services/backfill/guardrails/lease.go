package guardrails

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/platform/store"
)

// ErrLeaseHeld signals another worker owns the hour already
var ErrLeaseHeld = errors.New("backfill: hour lease already held")

// MakeAdvisoryLease returns a function that tries to claim a cooperative lease
// for the given hour inside the unified ingest_hours table. It uses an
// expires_at field to auto-reclaim leases after crashes.
//
// Schema fields used (on ingest_hours):
//
// - bf_lease_claimed_at timestamptz
// - bf_lease_owner      text
// - bf_lease_expires_at timestamptz
//
// The function treats a non-claimed/non-expired row as claimable, and returns
// ErrLeaseHeld when another worker currently owns a non-expired lease.
//
// No explicit release is performed - the lease is time-based; if you want,
// you can clear/refresh it elsewhere, but this keeps the simple one-shot model
func MakeAdvisoryLease(
	deps modkit.Deps,
	owner string,
	ttl time.Duration,
) func(ctx context.Context, hour time.Time, do func(context.Context) error) error {
	owner = fmt.Sprintf("%s:%d", owner, os.Getpid())

	if ttl <= 0 {
		ttl = 3 * time.Minute
	}

	toInterval := func(d time.Duration) string {
		return fmt.Sprintf("%d seconds", int64(d/time.Second))
	}

	return func(ctx context.Context, hour time.Time, do func(context.Context) error) error {
		var claimed bool
		err := deps.PG.Tx(ctx, func(q store.RowQuerier) error {
			row := q.QueryRow(ctx, `
				UPDATE ingest_hours
				   SET bf_lease_claimed_at = now(), bf_lease_owner = $2, bf_lease_expires_at = now() + ($3)::interval
				 WHERE hour_utc = $1
				   AND (bf_lease_claimed_at IS NULL OR bf_lease_expires_at <= now())
				RETURNING true
			`, hour.UTC(), owner, toInterval(ttl))
			var ok bool
			if scanErr := row.Scan(&ok); scanErr != nil {
				// no rows -> couldn't claim
				return nil
			}
			claimed = ok
			return nil
		})
		if err != nil {
			return err
		}
		if !claimed {
			return ErrLeaseHeld
		}
		return do(ctx)
	}
}
