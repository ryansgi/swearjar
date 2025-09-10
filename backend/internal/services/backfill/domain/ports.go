package domain

import (
	"context"
	"io"
	"time"
)

// RunnerPort is public port exposed by the module (what other modules would call)
type RunnerPort interface {
	RunRange(ctx context.Context, start, end time.Time) error
}

// StorageRepo is the storage repository interface
type StorageRepo interface {
	// StartHour marks the beginning of a backfill hour
	StartHour(ctx context.Context, hour time.Time) error

	// FinishHour marks the end of a backfill hour
	FinishHour(ctx context.Context, hour time.Time, fin HourFinish) error

	// InsertUtterances inserts utterances in bulk, returning counts of inserted and deduped rows.
	// It is safe to call with an empty slice (no-op)
	// It is recommended to batch inserts (e.g. 1000s of rows) for performance
	InsertUtterances(ctx context.Context, us []Utterance) (inserted, deduped int, err error)

	// LookupIDs resolves DB UUIDs (as text) for the given natural keys.
	// The result is a map from UKey -> utterances.id::text
	LookupIDs(ctx context.Context, keys []UKey) (map[UKey]LookupRow, error)
}

// LookupRow is what LookupIDs returns per natural key
type LookupRow struct {
	ID       string  // utterances.id::text
	LangCode *string // utterances.lang_code (nil if NULL)
}

// Fetcher is the data fetcher interface
type Fetcher interface {
	Fetch(ctx context.Context, hr HourRef) (io.ReadCloser, error)
}

// ReaderPort is the event reader interface
type ReaderPort interface {
	Next() (EventEnvelope, error)
	Close() error
	Stats() (events int, bytes int64) // return zeros if not supported
}

// ReaderFactory is the event reader factory interface
type ReaderFactory interface {
	New(io.ReadCloser) (ReaderPort, error)
}

// Extractor is the event extractor interface
type Extractor interface {
	FromEvent(env EventEnvelope, n Normalizer) []Utterance
}

// Normalizer is the event normalizer interface
type Normalizer interface {
	Normalize(s string) string
}
