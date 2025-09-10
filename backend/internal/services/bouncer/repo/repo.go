// Package repo provides the bouncer repository implementation
package repo

import (
	"context"
	stdsql "database/sql"
	"errors"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/api/bouncer/domain"
)

// Repo is the bouncer persistence surface used by the service layer
type Repo interface {
	InsertChallengeArgs(ctx context.Context,
		principal string, resource string, action string,
		hash string, evidenceKind string, artifactHint string,
	) error

	UpsertReceipt(ctx context.Context,
		principal string, principalHID []byte, action string,
		evidenceKind string, evidenceURL string, hash string,
	) error

	MarkRevocationPending(ctx context.Context, principal string, principalHID []byte) error

	ResolveStatusByHID(ctx context.Context,
		principal string, principalHID []byte,
	) (state string, since int64, evidenceURL, hash string, lastVerified int64, err error)
	LatestChallenge(ctx context.Context, principal, resource string) (domain.LatestChallenge, error)

	EnqueueVerification(
		ctx context.Context,
		principal, resource string,
		principalHID []byte,
		challengeHash string,
	) (string, error)
	LeaseVerifications(
		ctx context.Context,
		workerID string,
		limit int,
		leaseFor time.Duration,
	) ([]domain.VerificationJob, error)
	CompleteVerification(
		ctx context.Context,
		jobID string,
		lastStatus *int,
		lastURL string,
		etagBranch, etagFile, etagGists *string,
	) error
	RequeueVerification(
		ctx context.Context,
		jobID string,
		lastStatus *int,
		lastErr string,
		nextAttemptAt time.Time,
		rateResetAt *time.Time,
		etagBranch, etagFile, etagGists *string,
	) error
}

type (
	// PG is a Postgres implementation of the bouncer repo
	PG      struct{}
	queries struct{ q repokit.Queryer }
)

// NewPG returns a binder for the Postgres implementation
func NewPG() repokit.Binder[Repo] { return PG{} }

// Bind attaches a Queryer to the Postgres implementation
func (PG) Bind(q repokit.Queryer) Repo { return &queries{q: q} }

// InsertChallengeArgs records a freshly issued challenge using explicit args
func (r *queries) InsertChallengeArgs(
	ctx context.Context,
	principal, resource, action, hash, evidenceKind, artifactHint string,
) error {
	const sql = `
		INSERT INTO consent_challenges (
			challenge_hash, principal,  resource, action,  scope, evidence_kind,  artifact_hint, issued_at,  state
		) VALUES (
			$1, $2::principal_enum, $3, $4::consent_action_enum,
			CASE
				WHEN $2::principal_enum = 'repo'::principal_enum  THEN ARRAY['demask_repo'::consent_scope_enum]
				WHEN $2::principal_enum = 'actor'::principal_enum THEN ARRAY['demask_self'::consent_scope_enum]
				ELSE NULL
			END,
			$5::evidence_kind_enum, $6, NOW(), 'pending'::consent_state_enum
		)
		ON CONFLICT (challenge_hash) DO UPDATE
		SET principal     = EXCLUDED.principal,
		    resource      = EXCLUDED.resource,
		    action        = EXCLUDED.action,
		    scope         = EXCLUDED.scope,
		    evidence_kind = EXCLUDED.evidence_kind,
		    artifact_hint = EXCLUDED.artifact_hint,
		    issued_at     = EXCLUDED.issued_at,
		    state         = EXCLUDED.state
	`
	_, err := r.q.Exec(ctx, sql, hash, principal, resource, action, evidenceKind, artifactHint)
	return err
}

// UpsertReceipt activates or refreshes a receipt for opt in or opt out
// After a successful upsert, the related challenge row is deleted by challenge_hash,
// which also removes any queued consent_verifications via ON DELETE CASCADE
func (r *queries) UpsertReceipt(ctx context.Context,
	principal string, principalHID []byte, action string,
	evidenceKind string, evidenceURL string, hash string,
) error {
	const upsert = `
		INSERT INTO consent_receipts (
			principal, principal_hid, action, scope, evidence_kind, evidence_url, evidence_fingerprint,
			created_at, last_verified_at, revoked_at, terms_version, state
		) VALUES ($1, $2, $3, NULL, $4, $5, $6, NOW(), NOW(), NULL, NULL, 'active')
		ON CONFLICT (principal, principal_hid, action) DO UPDATE
		SET evidence_url         = EXCLUDED.evidence_url,
		    evidence_fingerprint = EXCLUDED.evidence_fingerprint,
		    last_verified_at     = EXCLUDED.last_verified_at,
		    revoked_at           = NULL,
		    state                = 'active'
	`
	if _, err := r.q.Exec(ctx, upsert, principal, principalHID, action, evidenceKind, evidenceURL, hash); err != nil {
		return err
	}

	_, err := r.q.Exec(ctx, `DELETE FROM consent_challenges WHERE challenge_hash = $1`, hash)
	return err
}

// MarkRevocationPending sets a soft revoke marker without changing state logic elsewhere
func (r *queries) MarkRevocationPending(ctx context.Context, principal string, principalHID []byte) error {
	const sql = `
		UPDATE consent_receipts
		SET last_verified_at = COALESCE(last_verified_at, NOW()) - INTERVAL '1 second',
		    revoked_at       = COALESCE(revoked_at, NOW())
		WHERE principal = $1 AND principal_hid = $2
	`
	_, err := r.q.Exec(ctx, sql, principal, principalHID)
	return err
}

// ResolveStatusByHID returns the effective state along with evidence and timestamps
func (r *queries) ResolveStatusByHID(
	ctx context.Context, principal string, principalHID []byte,
) (state string, since int64, evidenceURL, hash string, lastVerified int64, err error) {
	const sql = `
		WITH latest_receipt AS (
			SELECT r.*
			FROM consent_receipts r
			WHERE r.principal = $1 AND r.principal_hid = $2
			  AND r.state IN ('active','pending','revoked')
			ORDER BY r.created_at DESC
			LIMIT 1
		)
		SELECT
			CASE
				WHEN r.action = 'opt_out' AND r.state = 'active' THEN 'deny'
				WHEN r.action = 'opt_in' AND r.state = 'active' THEN 'allow'
				ELSE 'none'
			END AS state,
			EXTRACT(EPOCH FROM COALESCE(r.created_at, NOW()))::bigint AS since_unix,
			COALESCE(r.evidence_url, '') AS evidence_url,
			COALESCE(r.hash, '') AS hash,
			EXTRACT(EPOCH FROM COALESCE(r.last_verified_at, TO_TIMESTAMP(0)))::bigint AS last_verified_unix
		FROM latest_receipt r
	`
	row := r.q.QueryRow(ctx, sql, principal, principalHID)
	var s string
	var su, lvu int64
	var url, h string
	if scanErr := row.Scan(&s, &su, &url, &h, &lvu); scanErr != nil {
		return "", 0, "", "", 0, scanErr
	}
	return s, su, url, h, lvu, nil
}

func (r *queries) LatestChallenge(ctx context.Context, principal, resource string) (domain.LatestChallenge, error) {
	const sql = `SELECT c.action::text, c.evidence_kind::text, c.artifact_hint, c.challenge_hash,
	                EXTRACT(EPOCH FROM c.issued_at)::bigint
	             FROM consent_challenges c
	             WHERE c.principal = $1::principal_enum AND c.resource = $2
	             ORDER BY c.issued_at DESC
	             LIMIT 1`
	var lc domain.LatestChallenge

	row := r.q.QueryRow(ctx, sql, principal, resource)
	if err := row.Scan(&lc.Action, &lc.EvidenceKind, &lc.ArtifactHint, &lc.Hash, &lc.IssuedAtUnix); err != nil {
		if errors.Is(err, stdsql.ErrNoRows) {
			return domain.LatestChallenge{}, nil
		}
		return domain.LatestChallenge{}, err
	}
	return lc, nil
}
