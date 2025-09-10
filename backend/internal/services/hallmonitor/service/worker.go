// Package service contains hallmonitor workflows
package service

import (
	"context"
	"encoding/hex"
	"errors"
	"math/rand"
	"strings"
	"time"

	perr "swearjar/internal/platform/errors"
)

// Run starts the hallmonitor worker loops (repo and actor)
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
			jobs, err := s.Repo.LeaseRepos(ctx, batch, leaseFor) // HID-keyed jobs
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				continue
			}

			for _, j := range jobs {
				hidHex := hex.EncodeToString(j.RepoHID)

				// DryRun: just ack the HID job
				if s.config.DryRun {
					if err := s.Repo.AckRepoHID(ctx, j.RepoHID); err != nil {
						return err
					}
					continue
				}

				// Resolve HID -> numeric GitHub repo ID (needed even pre opt-in)
				ghRepoID, ok, err := s.Repo.ResolveRepoGHID(ctx, j.RepoHID)
				if err != nil {
					s.handleRepoErrorHID(ctx, j.RepoHID, j.Attempts, err)
					continue
				}
				if !ok || ghRepoID == 0 {
					// Mapping not present yet - back off briefly and try later
					s.handleRepoErrorHID(
						ctx,
						j.RepoHID,
						j.Attempts,
						perr.Newf(perr.ErrorCodeUnavailable, "missing gh_repo_id for %s", hidHex),
					)
					continue
				}

				// Local hints (don't require consent to read; full_name won't be stored unless opted-in)
				var etagIn string
				var owner, name string

				if fn, et, gone, _, _, err := s.Repo.RepoHintsHID(ctx, j.RepoHID); err == nil {
					// If repo is tombstoned, drop the job immediately (no GH call)
					if gone {
						_ = s.Repo.AckRepoHID(ctx, j.RepoHID)
						continue
					}
					if et != nil {
						etagIn = *et
					}
					if fn != nil {
						owner, name = splitOwnerName(*fn)
					}
				}

				// Fetch repo by numeric ID with conditional ETag
				repoDoc, etagOut, notmod, err := s.gh.RepoByID(ctx, ghRepoID, etagIn)
				if err != nil {
					s.handleRepoErrorHID(ctx, j.RepoHID, j.Attempts, err)
					continue
				}
				if notmod {
					stars, pushedPtr, _ := s.Repo.RepoCadenceInputsHID(ctx, j.RepoHID)
					var pushed time.Time
					if pushedPtr != nil {
						pushed = *pushedPtr
					}
					next := nextRefreshRepoFromFields(s.config.Cadence, stars, pushed, time.Now().UTC())
					if err := s.Repo.TouchRepository304HID(ctx, j.RepoHID, next, etagOut); err != nil {
						s.handleRepoErrorHID(ctx, j.RepoHID, j.Attempts, err)
						continue
					}
					if err := s.Repo.AckRepoHID(ctx, j.RepoHID); err != nil {
						return err
					}
					continue
				}

				// Languages: prefer owner/name; if absent, try from repoDoc
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
					if lm, _, _, lerr := s.gh.RepoLanguages(ctx, owner, name, ""); lerr == nil {
						langs = lm
					}
				}

				rec := mapRepoToRecord(s.config.Cadence, repoDoc, langs, etagOut)
				// NOTE: UpsertRepositoryHID will only persist PII (full_name) when an active opt-in exists
				if err := s.Repo.UpsertRepositoryHID(ctx, j.RepoHID, rec); err != nil {
					s.handleRepoErrorHID(ctx, j.RepoHID, j.Attempts, err)
					continue
				}
				if err := s.Repo.AckRepoHID(ctx, j.RepoHID); err != nil {
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
			jobs, err := s.Repo.LeaseActors(ctx, batch, leaseFor) // HID-keyed jobs
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				continue
			}

			for _, j := range jobs {
				hidHex := hex.EncodeToString(j.ActorHID)

				if s.config.DryRun {
					if err := s.Repo.AckActorHID(ctx, j.ActorHID); err != nil {
						return err
					}
					continue
				}

				ghUserID, ok, err := s.Repo.ResolveActorGHID(ctx, j.ActorHID)
				if err != nil {
					s.handleActorErrorHID(ctx, j.ActorHID, j.Attempts, err)
					continue
				}
				if !ok || ghUserID == 0 {
					s.handleActorErrorHID(ctx, j.ActorHID, j.Attempts, perr.Newf(
						perr.ErrorCodeUnavailable,
						"missing gh_actor_id for %s",
						hidHex,
					))
					continue
				}

				// Hints (skip if tombstoned)
				var etagIn string
				if _, et, gone, _, _, err := s.Repo.ActorHintsHID(ctx, j.ActorHID); err == nil {
					if gone {
						_ = s.Repo.AckActorHID(ctx, j.ActorHID)
						continue
					}
					if et != nil {
						etagIn = *et
					}
				}

				userDoc, etagOut, notmod, err := s.gh.UserByID(ctx, ghUserID, etagIn)
				if err != nil {
					s.handleActorErrorHID(ctx, j.ActorHID, j.Attempts, err)
					continue
				}
				if notmod {
					followers, _ := s.Repo.ActorCadenceInputsHID(ctx, j.ActorHID)
					next := nextRefreshActorFromFields(s.config.Cadence, followers, time.Now().UTC())
					if err := s.Repo.TouchActor304HID(ctx, j.ActorHID, next, etagOut); err != nil {
						s.handleActorErrorHID(ctx, j.ActorHID, j.Attempts, err)
						continue
					}
					if err := s.Repo.AckActorHID(ctx, j.ActorHID); err != nil {
						return err
					}
					continue
				}

				rec := mapUserToActorRecord(s.config.Cadence, userDoc, etagOut)
				// NOTE: UpsertActorHID will only persist PII (login/name) when an active opt-in exists
				if err := s.Repo.UpsertActorHID(ctx, j.ActorHID, rec); err != nil {
					s.handleActorErrorHID(ctx, j.ActorHID, j.Attempts, err)
					continue
				}
				if err := s.Repo.AckActorHID(ctx, j.ActorHID); err != nil {
					return err
				}
			}
		}
	}
}

// --- Error handling (HID variants) -------------------------------------------

func (s *Svc) handleRepoErrorHID(ctx context.Context, repoHID []byte, attempts int, err error) {
	// Terminal? -> tombstone + ACK (no more hot retries)
	if term, code, reason := classifyTerminal(err); term {
		slow := jitter90to180()
		_ = s.Repo.TombstoneRepositoryHID(ctx, repoHID, code, reason, slow)
		_ = s.Repo.AckRepoHID(ctx, repoHID)
		s.deps.Log.Info().
			Str("repo_hid", hex.EncodeToString(repoHID)).
			Int("code", code).
			Str("reason", reason).
			Dur("next_refresh_in", slow).
			Msg("tombstoned repository after terminal error")
		return
	}

	// Non-terminal -> NACK with exponential backoff (+ extra for 429)
	msg := trimErr(err)
	back := backoffFor(attempts, s.config.RetryBaseMs)
	if perr.IsCode(err, perr.ErrorCodeTooManyRequests) {
		back += 5 * time.Second
	}
	_ = s.Repo.NackRepoHID(ctx, repoHID, back, msg)
	s.deps.Log.Warn().
		Str("repo_hid", hex.EncodeToString(repoHID)).
		Dur("backoff", back).
		Msg("repo job failed; scheduled retry")
}

func (s *Svc) handleActorErrorHID(ctx context.Context, actorHID []byte, attempts int, err error) {
	if term, code, reason := classifyTerminal(err); term {
		slow := jitter90to180()
		_ = s.Repo.TombstoneActorHID(ctx, actorHID, code, reason, slow)
		_ = s.Repo.AckActorHID(ctx, actorHID)
		s.deps.Log.Info().
			Str("actor_hid", hex.EncodeToString(actorHID)).
			Int("code", code).
			Str("reason", reason).
			Dur("next_refresh_in", slow).
			Msg("tombstoned actor after terminal error")
		return
	}

	msg := trimErr(err)
	back := backoffFor(attempts, s.config.RetryBaseMs)
	if perr.IsCode(err, perr.ErrorCodeTooManyRequests) {
		back += 5 * time.Second
	}
	_ = s.Repo.NackActorHID(ctx, actorHID, back, msg)
	s.deps.Log.Warn().
		Str("actor_hid", hex.EncodeToString(actorHID)).
		Dur("backoff", back).
		Msg("actor job failed; scheduled retry")
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

// classifyTerminal maps HTTP status to {terminal, code, reason}
func classifyTerminal(err error) (bool, int, string) {
	switch httpStatusFromError(err) {
	case 404:
		return true, 404, "not_found"
	case 410:
		return true, 410, "gone"
	case 451:
		return true, 451, "legal"
	default:
		return false, 0, ""
	}
}

// jitter90to180 returns a duration between 90 & 180 days
func jitter90to180() time.Duration {
	days := 90 + rand.Intn(91) // 90..180
	return time.Duration(days) * 24 * time.Hour
}

// httpStatusFromError tries hard to extract an HTTP status from err
func httpStatusFromError(err error) int {
	// perr helper, if your client wrapped it that way
	if s := perr.HTTPStatus(err); s != 0 {
		return s
	}

	// common typed shapes
	type httpStatusProvider interface{ HTTPStatus() int }
	var hsp httpStatusProvider
	if errors.As(err, &hsp) {
		if s := hsp.HTTPStatus(); s != 0 {
			return s
		}
	}
	type statusCodeProvider interface{ StatusCode() int }
	var scp statusCodeProvider
	if errors.As(err, &scp) {
		if s := scp.StatusCode(); s != 0 {
			return s
		}
	}

	// perr error codes (if your client maps 404 -> ErrorCodeNotFound, etc)
	if perr.IsCode(err, perr.ErrorCodeNotFound) {
		return 404
	}
	if perr.IsCode(err, perr.ErrorCodeGone) {
		return 410
	}
	if perr.IsCode(err, perr.ErrorCodeLegal) {
		return 451
	}

	// last-resort string sniffing (matches your log text)
	e := err.Error()
	switch {
	case strings.Contains(e, "status 404") || strings.Contains(e, `"status":"404"`) || strings.Contains(e, `status=404`):
		return 404
	case strings.Contains(e, "status 410") || strings.Contains(e, `"status":"410"`) || strings.Contains(e, `status=410`):
		return 410
	case strings.Contains(e, "status 451") || strings.Contains(e, `"status":"451"`) || strings.Contains(e, `status=451`):
		return 451
	}
	return 0
}
