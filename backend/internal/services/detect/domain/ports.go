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

// WriterPort accepts utterances and writes hits
type WriterPort interface {
	// Write processes a batch of normalized utterances and persists hits.
	// Returns number of hits written (after ON CONFLICT DO NOTHING)
	Write(ctx context.Context, xs []WriteInput) (int, error)

	// WriteOne convenience wrapper
	WriteOne(ctx context.Context, x WriteInput) error
}
