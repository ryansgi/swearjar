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

	// InsertUtterances inserts a batch of utterances into the storage
	InsertUtterances(ctx context.Context, us []Utterance) (inserted, deduped int, err error)
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
