package domain

import (
	"context"
	"time"

	hitsdom "swearjar/internal/services/hits/domain"
	utdom "swearjar/internal/services/utterances/domain"
)

// RunnerPort is the external port for the detect job
type RunnerPort interface {
	RunRange(ctx context.Context, start, end time.Time) error
}

// Ports are dependencies injected into the detect module
type Ports struct {
	Utterances utdom.ReaderPort   // required
	HitsWriter hitsdom.WriterPort // required
}

// StorageRepo persists and queries utterances and hits
type StorageRepo interface {
	// Keyset page over utterances within [since, until)
	ListUtterances(ctx context.Context,
		since, until time.Time,
		afterCommitted time.Time, afterID int64,
		limit int,
	) ([]Utterance, error)

	// Batch insert hits (idempotent for same version via ON CONFLICT)
	WriteHitsBatch(ctx context.Context, xs []Hit) error
}

// WriterPort accepts utterances and writes hits.
type WriterPort interface {
	// Write processes a batch of normalized utterances and persists hits.
	// Returns number of hits written (after ON CONFLICT DO NOTHING).
	Write(ctx context.Context, xs []WriteInput) (int, error)

	// WriteOne convenience wrapper.
	WriteOne(ctx context.Context, x WriteInput) error
}
