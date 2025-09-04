package ingest

import "swearjar/internal/services/backfill/domain"

// Normalizer wraps a core normalizer to satisfy domain.Normalizer.
type normalizer struct {
	inner interface{ Normalize(string) string }
}

// NewNormalizer constructs a new Normalizer
func NewNormalizer(inner interface{ Normalize(string) string }) domain.Normalizer {
	return normalizer{inner: inner}
}

// Normalize normalizes the given string
func (n normalizer) Normalize(s string) string { return n.inner.Normalize(s) }
