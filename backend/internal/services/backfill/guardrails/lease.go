package guardrails

import (
	"context"
	"errors"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/platform/store"
)

// ErrLeaseHeld signals another worker owns the hour already.
var ErrLeaseHeld = errors.New("backfill: hour lease already held")

// MakeAdvisoryLease returns a function that uses Postgres to
// acquire an advisory lease for the given hour, running the do function
// if successful. It uses the ingest_hours_leases table to track claimed hours.
// If the hour is already claimed, it returns ErrLeaseHeld.
// It does not attempt to release the lease; it's a one-time claim.
// This is a simple way to avoid multiple workers processing the same hour
// when running multiple backfill instances concurrently.
// It assumes the ingest_hours_leases table exists.
func MakeAdvisoryLease(
	deps modkit.Deps,
) func(ctx context.Context, hour time.Time, do func(context.Context) error) error {
	return func(ctx context.Context, hour time.Time, do func(context.Context) error) error {
		var claimed bool
		err := deps.PG.Tx(ctx, func(q store.RowQuerier) error {
			rows, err := q.Query(ctx, `
				insert into ingest_hours_leases (hour_utc)
				values ($1)
				on conflict (hour_utc) do nothing
				returning true
			`, hour.UTC())
			if err != nil {
				return err
			}
			defer rows.Close()
			if rows.Next() {
				claimed = true
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !claimed {
			return ErrLeaseHeld // clean skip
		}
		// We "own" the hour. Run the work, then we're done. (TODO: keep a status column)
		return do(ctx)
	}
}
