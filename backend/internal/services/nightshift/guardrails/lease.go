// Package guardrails provides helper functions to manage worker leases for nightshift processing
package guardrails

import (
	"context"
	"fmt"
	"os"
	"time"

	"swearjar/internal/modkit"
	"swearjar/internal/platform/store"
)

// ErrLeaseHeld signals another worker owns the hour already
var ErrLeaseHeld = fmt.Errorf("nightshift: hour lease already held")

// MakeNSLease claims the ns_ lease columns (auto-reclaim via expires_at).
// Uses: ns_lease_claimed_at, ns_lease_owner, ns_lease_expires_at
func MakeNSLease(
	deps modkit.Deps,
	owner string,
	ttl time.Duration,
) func(ctx context.Context, hour time.Time, do func(context.Context) error) error {
	owner = fmt.Sprintf("%s:%d", owner, os.Getpid())

	if ttl <= 0 {
		ttl = 3 * time.Minute
	}

	toInterval := func(d time.Duration) string { return fmt.Sprintf("%d seconds", int64(d/time.Second)) }

	return func(ctx context.Context, hour time.Time, do func(context.Context) error) error {
		var claimed bool
		if err := deps.PG.Tx(ctx, func(q store.RowQuerier) error {
			row := q.QueryRow(ctx, `
				UPDATE ingest_hours
				   SET ns_lease_claimed_at = now(), ns_lease_owner = $2, ns_lease_expires_at = now() + ($3)::interval
				 WHERE hour_utc = $1
				   AND (ns_lease_claimed_at IS NULL OR ns_lease_expires_at <= now())
				RETURNING true
			`, hour.UTC(), owner, toInterval(ttl))
			var ok bool
			if err := row.Scan(&ok); err != nil {
				return nil // no rows -> couldn't claim
			}
			claimed = ok
			return nil
		}); err != nil {
			return err
		}
		if !claimed {
			return ErrLeaseHeld
		}
		return do(ctx)
	}
}
