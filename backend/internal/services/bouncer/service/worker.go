package service

import (
	"context"
	"time"

	"swearjar/internal/platform/logger"
)

// Run starts the worker loop to process verification jobs
func (s *Svc) Run(ctx context.Context) error {
	log := logger.Named("bouncer-worker")
	sem := make(chan struct{}, max(1, s.cfg.Concurrency))
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// lease a small batch; process concurrently with a simple semaphore
			jobs, err := s.repo.LeaseVerifications(ctx, "bouncer", s.cfg.QueueTakeBatch, 60*time.Second)
			if err != nil {
				log.Error().Err(err).Msg("lease verifications failed")
				continue
			}
			for i := range jobs {
				sem <- struct{}{}
				j := jobs[i]
				go func() {
					defer func() { <-sem }()
					if err := s.handleJob(ctx, j); err != nil {
						log.Warn().Err(err).Str("job_id", j.JobID).Msg("job failed")
					}
				}()
			}
		}
	}
}
