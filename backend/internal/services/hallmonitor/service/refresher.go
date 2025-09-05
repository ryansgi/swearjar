package service

import (
	"context"
	"time"

	"swearjar/internal/services/hallmonitor/domain"
)

// RefreshDue enqueues repositories and actors whose next_refresh_at is due
func (s *Svc) RefreshDue(ctx context.Context, p domain.RefreshParams) error {
	since := p.Since
	until := p.Until
	if since.After(until) && !until.IsZero() {
		since = time.Time{}
	}
	limit := p.Limit
	if limit == 0 {
		limit = s.config.DefaultRefreshLimit
	}
	if _, err := s.Repo.EnqueueDueRepos(ctx, since, until, limit); err != nil {
		return err
	}
	if _, err := s.Repo.EnqueueDueActors(ctx, since, until, limit); err != nil {
		return err
	}
	return nil
}
