// Package service contains hallmonitor workflows
package service

import (
	"context"
	"time"

	perr "swearjar/internal/platform/errors"
)

// Run starts the hallmonitor worker loops for repos and actors
func (s *Svc) Run(ctx context.Context) error {
	if s.config.Concurrency <= 0 {
		s.config.Concurrency = 1
	}
	leaseFor := 30 * time.Second
	batch := s.config.QueueTakeBatch
	if batch <= 0 {
		batch = 64
	}

	errCh := make(chan error, 2)
	go func() { errCh <- s.runRepoLoop(ctx, batch, leaseFor) }()
	go func() { errCh <- s.runActorLoop(ctx, batch, leaseFor) }()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (s *Svc) runRepoLoop(ctx context.Context, batch int, leaseFor time.Duration) error {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			jobs, err := s.Repo.LeaseRepos(ctx, batch, leaseFor)
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				continue
			}

			for _, j := range jobs {
				if s.config.DryRun {
					if err := s.Repo.AckRepo(ctx, j.RepoID); err != nil {
						return err
					}
					continue
				}

				var etagIn string
				var owner string
				var name string
				if fn, et, err := s.Repo.RepoHints(ctx, j.RepoID); err == nil {
					if et != nil {
						etagIn = *et
					}
					if fn != nil {
						owner, name = splitOwnerName(*fn)
					}
				}

				repoDoc, etagOut, notmod, err := s.gh.RepoByID(ctx, j.RepoID, etagIn)
				if err != nil {
					s.handleRepoError(ctx, j.RepoID, j.Attempts, err)
					continue
				}
				if notmod {
					stars, pushedPtr, _ := s.Repo.RepoCadenceInputs(ctx, j.RepoID)
					var pushed time.Time
					if pushedPtr != nil {
						pushed = *pushedPtr
					}
					next := nextRefreshRepoFromFields(s.config.Cadence, stars, pushed, time.Now().UTC())
					if err := s.Repo.TouchRepository304(ctx, j.RepoID, next, etagOut); err != nil {
						s.handleRepoError(ctx, j.RepoID, j.Attempts, err)
						continue
					}
					if err := s.Repo.AckRepo(ctx, j.RepoID); err != nil {
						return err
					}
					continue
				}

				if owner == "" || name == "" {
					if repoDoc.FullName != "" {
						owner, name = splitOwnerName(repoDoc.FullName)
					} else if repoDoc.Owner.Login != "" && repoDoc.Name != "" {
						owner = repoDoc.Owner.Login
						name = repoDoc.Name
					}
				}

				var langs map[string]int64
				if owner != "" && name != "" {
					lm, _, _, lerr := s.gh.RepoLanguages(ctx, owner, name, "")
					if lerr == nil {
						langs = lm
					}
				}

				rec := mapRepoToRecord(s.config.Cadence, repoDoc, langs, etagOut)
				if err := s.Repo.UpsertRepository(ctx, rec); err != nil {
					s.handleRepoError(ctx, j.RepoID, j.Attempts, err)
					continue
				}
				if err := s.Repo.AckRepo(ctx, j.RepoID); err != nil {
					return err
				}
			}
		}
	}
}

func (s *Svc) runActorLoop(ctx context.Context, batch int, leaseFor time.Duration) error {
	t := time.NewTicker(750 * time.Millisecond)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			jobs, err := s.Repo.LeaseActors(ctx, batch, leaseFor)
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				continue
			}

			for _, j := range jobs {
				if s.config.DryRun {
					if err := s.Repo.AckActor(ctx, j.ActorID); err != nil {
						return err
					}
					continue
				}

				var etagIn string
				if _, et, err := s.Repo.ActorHints(ctx, j.ActorID); err == nil && et != nil {
					etagIn = *et
				}

				userDoc, etagOut, notmod, err := s.gh.UserByID(ctx, j.ActorID, etagIn)
				if err != nil {
					s.handleActorError(ctx, j.ActorID, j.Attempts, err)
					continue
				}
				if notmod {
					followers, _ := s.Repo.ActorCadenceInputs(ctx, j.ActorID)
					next := nextRefreshActorFromFields(s.config.Cadence, followers, time.Now().UTC())
					if err := s.Repo.TouchActor304(ctx, j.ActorID, next, etagOut); err != nil {
						s.handleActorError(ctx, j.ActorID, j.Attempts, err)
						continue
					}
					if err := s.Repo.AckActor(ctx, j.ActorID); err != nil {
						return err
					}
					continue
				}

				rec := mapUserToActorRecord(s.config.Cadence, userDoc, etagOut)
				if err := s.Repo.UpsertActor(ctx, rec); err != nil {
					s.handleActorError(ctx, j.ActorID, j.Attempts, err)
					continue
				}
				if err := s.Repo.AckActor(ctx, j.ActorID); err != nil {
					return err
				}
			}
		}
	}
}

func (s *Svc) handleRepoError(ctx context.Context, repoID int64, attempts int, err error) {
	msg := trimErr(err)
	back := backoffFor(attempts, s.config.RetryBaseMs)
	if perr.IsCode(err, perr.ErrorCodeTooManyRequests) {
		back += 5 * time.Second
	}
	_ = s.Repo.NackRepo(ctx, repoID, back, msg)
	s.deps.Log.Warn().Int64("repo_id", repoID).Dur("backoff", back).Msg("repo job failed scheduled retry")
}

func (s *Svc) handleActorError(ctx context.Context, actorID int64, attempts int, err error) {
	msg := trimErr(err)
	back := backoffFor(attempts, s.config.RetryBaseMs)
	if perr.IsCode(err, perr.ErrorCodeTooManyRequests) {
		back += 5 * time.Second
	}
	_ = s.Repo.NackActor(ctx, actorID, back, msg)
	s.deps.Log.Warn().Int64("actor_id", actorID).Dur("backoff", back).Msg("actor job failed scheduled retry")
}

func trimErr(err error) string {
	const n = 500
	s := err.Error()
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func backoffFor(attempts int, baseMs int) time.Duration {
	if baseMs <= 0 {
		baseMs = 500
	}
	if attempts < 0 {
		attempts = 0
	}
	ms := min(int64(baseMs)<<uint(attempts), int64(10*time.Minute/time.Millisecond))
	return time.Duration(ms) * time.Millisecond
}
