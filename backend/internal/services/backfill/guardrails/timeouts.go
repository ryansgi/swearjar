// Package guardrails holds cross cutting safety helpers for backfill
package guardrails

import (
	"context"
	"time"
)

// Timeouts is an optional budget bundle for a single hour of work.
// Zero values mean no extra timeout at that level
type Timeouts struct {
	// Hour is the overall time budget for processing one gharchive hour
	Hour time.Duration

	// Fetch caps the network fetch step
	Fetch time.Duration

	// Read caps the gzip ndjson read and extract step
	Read time.Duration

	// DB caps the insert step
	DB time.Duration
}

// WithHour returns a context limited by the given hour budget without extending any parent deadline.
// if Hour is zero it returns a cancelable child that simply inherits the parent deadline
func WithHour(parent context.Context, t Timeouts) (context.Context, context.CancelFunc) {
	return withChildTimeout(parent, t.Hour)
}

// ForFetch returns a sub context for the fetch phase bounded by Fetch and any remaining parent budget
func ForFetch(parent context.Context, t Timeouts) (context.Context, context.CancelFunc) {
	return withChildTimeout(parent, t.Fetch)
}

// ForRead returns a sub context for the read phase bounded by Read and any remaining parent budget
func ForRead(parent context.Context, t Timeouts) (context.Context, context.CancelFunc) {
	return withChildTimeout(parent, t.Read)
}

// ForDB returns a sub context for the db phase bounded by DB and any remaining parent budget
func ForDB(parent context.Context, t Timeouts) (context.Context, context.CancelFunc) {
	return withChildTimeout(parent, t.DB)
}

// Remaining returns the time until the deadline on ctx or zero when none is set or already expired
func Remaining(ctx context.Context) time.Duration {
	if dl, ok := ctx.Deadline(); ok {
		d := time.Until(dl)
		if d > 0 {
			return d
		}
	}
	return 0
}

// withChildTimeout chooses the tighter of the requested duration and any parent remainder.
// Never extends the parent deadline
// When d is zero it returns a simple cancelable child inheriting the parent deadline
func withChildTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	// Zero means no additional limit, still return a cancelable child for symmetry
	if d <= 0 {
		return context.WithCancel(parent)
	}

	// respect any parent deadline by taking the minimum
	if rem := Remaining(parent); rem > 0 && rem < d {
		// if rem is tiny still honor it
		return context.WithTimeout(parent, rem)
	}
	return context.WithTimeout(parent, d)
}
