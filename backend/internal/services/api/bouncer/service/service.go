// Package service contains bouncer workflows
package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	gh "swearjar/internal/adapters/ingest/github"
	"swearjar/internal/modkit/repokit"
	perrs "swearjar/internal/platform/errors"
	"swearjar/internal/services/api/bouncer/domain"
	"swearjar/internal/services/api/bouncer/repo"
	bdom "swearjar/internal/services/bouncer/domain"
)

// Service is the public service port
type Service interface{ domain.ServicePort }

// PrincipalResolver resolves a principal HID from a natural key
type PrincipalResolver interface {
	RepoHID(ctx context.Context, resource string) ([]byte, bool, error)
	ActorHID(ctx context.Context, login string) ([]byte, bool, error)
}

// Svc implements the service port
type Svc struct {
	Repo     repo.Repo
	binder   repokit.Binder[repo.Repo]
	db       repokit.TxRunner
	secret   []byte
	grace    time.Duration
	resolver PrincipalResolver
	evidence EvidenceProbe
	enqueuer bdom.EnqueuePort
}

// Options control service behavior
type Options struct {
	Secret string
	Grace  time.Duration

	// Resolver is required
	Resolver PrincipalResolver

	// Evidence is required
	Evidence EvidenceProbe

	// Enqueuer is optional; if set, Issue will enqueue a verification job
	Enqueuer bdom.EnqueuePort
}

// New constructs the service
func New(db repokit.TxRunner, binder repokit.Binder[repo.Repo], opt Options) *Svc {
	if db == nil {
		panic("bouncer.Service requires a non nil TxRunner")
	}
	if binder == nil {
		panic("bouncer.Service requires a non nil Repo binder")
	}
	if opt.Resolver == nil {
		panic("bouncer.Service requires a non nil PrincipalResolver")
	}
	if opt.Evidence == nil {
		panic("bouncer.Service requires a non nil EvidenceProbe")
	}
	if opt.Enqueuer == nil {
		panic("bouncer.Service requires a non nil EnqueuePort (bouncer worker)")
	}

	g := opt.Grace
	if g == 0 {
		g = 7 * 24 * time.Hour
	}

	return &Svc{
		Repo:     binder.Bind(db),
		binder:   binder,
		db:       db,
		secret:   []byte(opt.Secret),
		grace:    g,
		resolver: opt.Resolver,
		evidence: opt.Evidence,
		enqueuer: opt.Enqueuer,
	}
}

// Issue mints a deterministic hash and inserts a challenge row with explicit args
func (s *Svc) Issue(ctx context.Context, in domain.IssueInput) (domain.IssueOutput, error) {
	// deterministic daily hash
	base := string(in.SubjectType) + ":" + in.SubjectKey + ":" + time.Now().UTC().Format("2006-01-02")
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(base))
	hash := hex.EncodeToString(mac.Sum(nil))

	principal := string(in.SubjectType) // "repo" | "actor"
	resource := in.SubjectKey
	action := map[domain.Scope]string{domain.ScopeAllow: "opt_in", domain.ScopeDeny: "opt_out"}[in.Scope]
	if action == "" {
		action = "opt_in"
	}

	evidenceKind := "repo_file"
	artifactHint := "." + hash + ".txt"
	if principal == "actor" {
		evidenceKind = "actor_gist"
		artifactHint = hash + ".txt"
	}

	// persist challenge
	if err := s.Repo.InsertChallengeArgs(ctx, principal, resource, action, hash, evidenceKind, artifactHint); err != nil {
		return domain.IssueOutput{}, err
	}

	// craft subject-specific output
	out := domain.IssueOutput{Hash: hash}
	if principal == "repo" {
		out.RepoFilename = "." + hash + ".txt"
		out.Instructions = "Create a file at the repository root on the DEFAULT branch (e.g., main/master) " +
			"named ." + hash + ".txt. Commit & push. Then POST to /api/v1/bouncer/reverify with the same subject."
	} else { // actor
		out.GistFilename = hash + ".txt"
		out.Instructions = "Create a PUBLIC GitHub Gist with a file named " + hash +
			".txt (content may be empty). Then POST to /api/v1/bouncer/reverify with the same subject."
	}

	return out, nil
}

// Reverify returns status after resolving principal HID
func (s *Svc) Reverify(ctx context.Context, in domain.ReverifyInput) (domain.StatusRow, error) {
	// Resolve subject: principal + HID + resource
	rs, err := s.resolveHID(ctx, in.SubjectType, in.SubjectKey)
	if err != nil {
		return domain.StatusRow{}, err
	}

	principal := rs.principal // "repo" | "actor"
	resource := in.SubjectKey // owner/repo or login

	// Fetch latest challenge; what to prove
	lc, err := s.Repo.LatestChallenge(ctx, principal, resource)
	if err != nil {
		return domain.StatusRow{}, perrs.ErrNotFound
	}
	if lc.Hash == "" {
		// no challenge issued for this subject yet
		return domain.StatusRow{State: domain.StateNone}, nil
	}

	// Probe artifact (inline fast path)
	var exists bool
	var url string

	probe := func() error {
		switch lc.EvidenceKind {
		case "repo_file":
			branch, err := s.evidence.DefaultBranch(ctx, resource) // e.g. "main"
			if err != nil {
				return err
			}
			ex, u, err := s.evidence.RepoFile(ctx, resource, branch, lc.ArtifactHint)
			if err != nil {
				return err
			}
			exists, url = ex, u
			return nil

		case "actor_gist":
			ex, u, err := s.evidence.GistFile(ctx, resource, lc.ArtifactHint)
			if err != nil {
				return err
			}
			exists, url = ex, u
			return nil
		default:
			return fmt.Errorf("unknown evidence_kind %q", lc.EvidenceKind)
		}
	}

	if err := probe(); err != nil {
		// If this looks like a GH rate-limit or transient, enqueue and return snapshot
		if shouldEnqueue(err) && s.enqueuer != nil {
			_ = s.enqueuer.EnqueueVerification(ctx, bdom.EnqueueArgs{
				Principal:     principal,
				Resource:      resource,
				PrincipalHID:  rs.hid,
				ChallengeHash: lc.Hash,
				EvidenceKind:  lc.EvidenceKind,
				ArtifactHint:  lc.ArtifactHint,
			})

			// Return current status snapshot (non-blocking enqueue)
			st, since, eurl, h, lv, rerr := s.Repo.ResolveStatusByHID(ctx, principal, rs.hid)
			if rerr != nil {
				return domain.StatusRow{}, rerr
			}

			// derive staleness (same logic as below)
			staleCut := time.Now().UTC().Add(-s.grace).Unix()
			staleness := "fresh"
			if lv <= staleCut {
				staleness = "stale"
			}
			return domain.StatusRow{
				State:          domain.EffectiveState(st),
				SinceUnix:      since,
				EvidenceURL:    eurl,
				Hash:           h,
				LastVerifiedAt: lv,
				Staleness:      staleness,
			}, nil
		}
		// otherwise, surface the error
		return domain.StatusRow{}, err
	}

	// Persist result from fast path
	if exists {
		if err := s.Repo.UpsertReceipt(ctx,
			principal, rs.hid, lc.Action, lc.EvidenceKind, url, lc.Hash,
		); err != nil {
			return domain.StatusRow{}, err
		}
	} else {
		// signal pending revocation (grace enforcement is policy)
		if err := s.Repo.MarkRevocationPending(ctx, principal, rs.hid); err != nil {
			return domain.StatusRow{}, err
		}
	}

	// Return current status
	st, since, eurl, h, lv, err := s.Repo.ResolveStatusByHID(ctx, principal, rs.hid)
	if err != nil {
		return domain.StatusRow{}, err
	}

	// Derive staleness (fresh|stale)
	staleCut := time.Now().UTC().Add(-s.grace).Unix()
	staleness := "fresh"
	if lv <= staleCut {
		staleness = "stale"
	}

	return domain.StatusRow{
		State:          domain.EffectiveState(st),
		SinceUnix:      since,
		EvidenceURL:    eurl,
		Hash:           h,
		LastVerifiedAt: lv,
		Staleness:      staleness,
	}, nil
}

// shouldEnqueue decides if an error should trigger enqueue (rate limit / GH err)
func shouldEnqueue(err error) bool {
	if gh.IsRateLimited(err) || gh.IsTransient(err) {
		return true
	}

	// consider 401/403 from GH as transient for gist/file evidence probing
	if se, ok := err.(*gh.GHStatusError); ok && (se.Status == 401 || se.Status == 403) {
		return false
	}
	return false
}

// Status returns the current consent view for a principal
func (s *Svc) Status(ctx context.Context, q domain.StatusQuery) (domain.StatusRow, error) {
	rs, err := s.resolveHID(ctx, q.SubjectType, q.SubjectKey)
	if err != nil {
		return domain.StatusRow{}, err
	}
	st, since, url, h, lv, err := s.Repo.ResolveStatusByHID(ctx, rs.principal, rs.hid)
	if err != nil {
		return domain.StatusRow{}, err
	}
	return domain.StatusRow{
		State:          domain.EffectiveState(st),
		SinceUnix:      since,
		EvidenceURL:    url,
		Hash:           h,
		LastVerifiedAt: lv,
	}, nil
}

type hidPair struct {
	principal string
	hid       []byte
}

// resolveHID maps subject type and natural key to principal and hid
func (s *Svc) resolveHID(ctx context.Context, t domain.SubjectType, key string) (hidPair, error) {
	if s.resolver == nil {
		return hidPair{}, errors.New("bouncer.service missing PrincipalResolver")
	}
	switch t {
	case domain.SubjectRepo:
		h, ok, err := s.resolver.RepoHID(ctx, key)
		if err != nil {
			return hidPair{}, err
		}
		if !ok || len(h) == 0 {
			return hidPair{}, errors.New("repo HID not found for resource")
		}
		return hidPair{principal: "repo", hid: h}, nil
	case domain.SubjectActor:
		h, ok, err := s.resolver.ActorHID(ctx, key)
		if err != nil {
			return hidPair{}, err
		}
		if !ok || len(h) == 0 {
			return hidPair{}, errors.New("actor HID not found for login")
		}
		return hidPair{principal: "actor", hid: h}, nil
	default:
		return hidPair{}, errors.New("unknown subject type")
	}
}
