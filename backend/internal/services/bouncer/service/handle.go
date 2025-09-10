package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	gh "swearjar/internal/adapters/ingest/github"

	domain "swearjar/internal/services/api/bouncer/domain"
)

// handleJob processes a single verification job
// It re-reads the latest challenge to ensure we're verifying current intent,
// then checks GitHub for the artifact, and records a receipt if found.
// If not found, it marks the principal as pending revocation.
// Any errors result in a requeue with backoff
func (s *Svc) handleJob(ctx context.Context, j domain.VerificationJob) error {
	// Re-read the latest challenge to ensure we're verifying current intent
	lc, err := s.repo.LatestChallenge(ctx, j.Principal, j.Resource)
	if err != nil {
		return s.repo.RequeueVerification(ctx, j.JobID, nil, fmt.Sprintf("latest_challenge: %v", err),
			nextAfter(j.Attempts, s.cfg.RetryBaseMs), nil, nil, nil, nil)
	}
	if lc.Hash == "" || lc.Hash != j.ChallengeHash {
		// Challenge changed or missing; finish this job
		return s.repo.CompleteVerification(ctx, j.JobID, nil, "", nil, nil, nil)
	}

	var exists bool
	var url string
	var etagBranch, etagFile, etagGists *string
	var lastStatus *int
	var rateReset *time.Time

	switch lc.EvidenceKind {
	case "repo_file":
		owner, repo, _ := strings.Cut(j.Resource, "/")
		// Default branch (ETag aware)
		repoDoc, eb, _, err := s.gh.RepoByFullName(ctx, owner, repo, valOr(j.ETagBranch))
		if ghErr, ok := err.(*gh.GHStatusError); ok && (ghErr.Status == 429 || ghErr.Status == 403) {
			rateReset = ghRateReset(ghErr)
			return s.repo.RequeueVerification(
				ctx,
				j.JobID,
				&ghErr.Status,
				"rate_limited",
				rlBackoff(j.Attempts, s.cfg.RetryBaseMs, rateReset),
				rateReset,
				nil,
				nil,
				nil,
			)
		}
		if err != nil {
			return s.repo.RequeueVerification(ctx, j.JobID, lastStatusOf(err), fmt.Sprintf("repo: %v", err),
				nextAfter(j.Attempts, s.cfg.RetryBaseMs), nil, nil, nil, nil)
		}
		if eb != "" {
			etagBranch = &eb
		}
		branch := repoDoc.DefaultBranch

		// Contents API for the artifact
		html, ef, _, err := s.gh.RepoContent(ctx, owner, repo, lc.ArtifactHint, branch, valOr(j.ETagFile))
		if ghErr, ok := err.(*gh.GHStatusError); ok && (ghErr.Status == 429 || ghErr.Status == 403) {
			rateReset = ghRateReset(ghErr)
			return s.repo.RequeueVerification(
				ctx,
				j.JobID,
				&ghErr.Status,
				"rate_limited",
				rlBackoff(j.Attempts, s.cfg.RetryBaseMs, rateReset),
				rateReset,
				etagBranch,
				nil,
				nil,
			)
		}
		if err != nil {
			return s.repo.RequeueVerification(ctx, j.JobID, lastStatusOf(err), fmt.Sprintf("contents: %v", err),
				nextAfter(j.Attempts, s.cfg.RetryBaseMs), nil, etagBranch, nil, nil)
		}
		if ef != "" {
			etagFile = &ef
		}
		exists = html != ""
		url = html

	case "actor_gist":
		login := j.Resource
		page := 1
		found := ""
		var eg string
		for {
			items, egp, _, err := s.gh.ListPublicGists(ctx, login, page, 100, valOr(j.ETagGists))
			if ghErr, ok := err.(*gh.GHStatusError); ok && (ghErr.Status == 429 || ghErr.Status == 403) {
				rateReset = ghRateReset(ghErr)
				return s.repo.RequeueVerification(
					ctx,
					j.JobID,
					&ghErr.Status,
					"rate_limited",
					rlBackoff(j.Attempts, s.cfg.RetryBaseMs, rateReset),
					rateReset,
					nil,
					nil,
					nil,
				)
			}
			if err != nil {
				return s.repo.RequeueVerification(ctx, j.JobID, lastStatusOf(err), fmt.Sprintf("gists: %v", err),
					nextAfter(j.Attempts, s.cfg.RetryBaseMs), nil, nil, nil, nil)
			}
			if egp != "" {
				eg = egp
			}
			if len(items) == 0 {
				break
			}
			for _, g := range items {
				// files is a map[string]file
				if fm, ok := g["files"].(map[string]any); ok {
					if _, ok := fm[lc.ArtifactHint]; ok {
						if u, ok := g["html_url"].(string); ok {
							found = u
							break
						}
					}
					for _, v := range fm {
						if m, ok := v.(map[string]any); ok && m["filename"] == lc.ArtifactHint {
							if u, ok := g["html_url"].(string); ok {
								found = u
								break
							}
						}
					}
				}
			}
			if found != "" {
				break
			}
			page++
		}
		if eg != "" {
			etagGists = &eg
		}
		exists = found != ""
		url = found
	}

	if exists {
		if err := s.repo.UpsertReceipt(
			ctx,
			j.Principal,
			j.PrincipalHID,
			lc.Action,
			lc.EvidenceKind,
			url,
			lc.Hash,
		); err != nil {
			return s.repo.RequeueVerification(ctx, j.JobID, nil, fmt.Sprintf("upsert_receipt: %v", err),
				nextAfter(j.Attempts, s.cfg.RetryBaseMs), nil, etagBranch, etagFile, etagGists)
		}
		return s.repo.CompleteVerification(ctx, j.JobID, lastStatus, url, etagBranch, etagFile, etagGists)
	}

	// Verified missing: soft revoke marker
	if err := s.repo.MarkRevocationPending(ctx, j.Principal, j.PrincipalHID); err != nil {
		return s.repo.RequeueVerification(ctx, j.JobID, nil, fmt.Sprintf("mark_revocation: %v", err),
			nextAfter(j.Attempts, s.cfg.RetryBaseMs), nil, etagBranch, etagFile, etagGists)
	}
	return s.repo.CompleteVerification(ctx, j.JobID, lastStatus, "", etagBranch, etagFile, etagGists)
}

func valOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func nextAfter(attempt int, baseMs int) time.Time {
	back := durationMs(baseMs)
	// simple exponential w/ cap ~30s
	ms := int64(back/time.Millisecond) << uint(attempt)
	if ms > int64(30*time.Second/time.Millisecond) {
		ms = int64(30 * time.Second / time.Millisecond)
	}
	return time.Now().UTC().Add(time.Duration(ms) * time.Millisecond)
}

func rlBackoff(attempt int, baseMs int, reset *time.Time) time.Time {
	if reset != nil && reset.After(time.Now().UTC()) {
		return *reset
	}
	return nextAfter(attempt, baseMs)
}

func ghRateReset(err *gh.GHStatusError) *time.Time {
	// Client logs+tracks rate headers; we don't have direct access here,
	// so we use a conservative exponential. We should extend Client to expose parsed headers
	fmt.Println(err)
	return nil
}

func lastStatusOf(err error) *int {
	var s *int
	type httpSt interface{ HTTPStatus() int }
	if hs, ok := err.(httpSt); ok {
		v := hs.HTTPStatus()
		s = &v
	}
	return s
}
