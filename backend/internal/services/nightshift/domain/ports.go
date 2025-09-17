// Package domain defines Nightshift core ports and types
package domain

import (
	"context"
	"time"
)

// RunnerPort is the public entrypoint exposed by the module.
// Backfill/Detect will call ApplyHour when an hour finishes,
// and operators can run batch jobs via RunRange/RunResume
type RunnerPort interface {
	// ApplyHour runs Nightshift for exactly one hour (idempotent per hour).
	// Intended to be called by backfill/detect hooks after "detected"
	ApplyHour(ctx context.Context, hour time.Time) error

	// RunRange iterates [start,end] inclusive, applying Nightshift per hour
	RunRange(ctx context.Context, start, end time.Time) error

	// RunResume drains any hours that are in states requiring nightshift work
	RunResume(ctx context.Context) error
}

// StorageRepo encapsulates all storage actions Nightshift performs.
// Typical impl: PG for ingest_hours state; CH for archives/rollups/pruning
type StorageRepo interface {
	// Start marks Nightshift processing for an hour (separate from backfill's StartHour)
	Start(ctx context.Context, hour time.Time) error

	// WriteArchives persists append-only snapshots for the hour into:
	//   - utterance_features_archive
	//   - hit_archive
	// It should be safe to re-run (idempotent via hour+ids)
	WriteArchives(ctx context.Context, hour time.Time, detver int) (hits int, err error)

	// SnapshotUttHourAgg inserts hourly uniq/count/etc states for the hour
	SnapshotUttHourAgg(ctx context.Context, hour time.Time) error

	// PruneRaw applies the configured retention policy to raw utterances (and anything else):
	//   - "full": no-op
	//   - "timebox:<Nd>": delete raw older than cutoff, keeping hit-backed rows if policy says so
	//   - "aggressive": delete raw utterances without hits immediately for the hour
	// Returns counts for book-keeping
	PruneRaw(ctx context.Context, hour time.Time, retention string) (deletedUtt, sparedUtt int, err error)

	// Finish marks the hour as "retention_applied" or final "done" depending on the pipeline
	Finish(ctx context.Context, hour time.Time, fin FinishInfo) error

	// NextHourNeedingWork returns the next hour that should have Nightshift applied.
	// Implementations may look at ingest_hours.status or a dedicated column
	NextHourNeedingWork(ctx context.Context) (time.Time, bool, error)
}

// FinishInfo captures metrics/outcomes for an hour of Nightshift work
type FinishInfo struct {
	Status       string // typically "retention_applied" or "done"
	DetVer       int
	HitsArchived int
	DeletedRaw   int
	SparedRaw    int
	ArchiveMS    int
	PruneMS      int
	TotalMS      int
	ErrText      string
}
