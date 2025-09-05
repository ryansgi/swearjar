package service

import (
	"context"
	"time"

	"swearjar/internal/services/hallmonitor/domain"
)

// SeedFromUtterances scans utterances and fills repo and actor queues for missing metadata
func (s *Svc) SeedFromUtterances(ctx context.Context, r domain.SeedRange) error {
	since := r.Since
	until := r.Until
	if until.IsZero() {
		until = time.Now().UTC()
	}
	limit := r.Limit
	if limit == 0 {
		limit = s.config.DefaultSeedLimit
	}
	if _, err := s.Repo.EnqueueMissingReposFromUtterances(ctx, since, until, limit); err != nil {
		return err
	}
	if _, err := s.Repo.EnqueueMissingActorsFromUtterances(ctx, since, until, limit); err != nil {
		return err
	}
	return nil
}
