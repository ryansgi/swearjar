package repo

import (
	"context"
	"time"

	"swearjar/internal/services/api/bouncer/domain"

	"github.com/google/uuid"
)

// EnqueueVerification idempotently creates (or returns) a job for this subject+challenge
func (r *queries) EnqueueVerification(
	ctx context.Context,
	principal, resource string,
	principalHID []byte,
	challengeHash string,
) (string, error) {
	const sqlq = `
        INSERT INTO consent_verifications (principal, resource, principal_hid, challenge_hash)
        VALUES ($1::principal_enum, $2, $3, $4)
        ON CONFLICT (principal, resource, challenge_hash)
        DO UPDATE SET updated_at = now()
        RETURNING job_id::text
    `
	var id string
	if err := r.q.QueryRow(ctx, sqlq, principal, resource, principalHID, challengeHash).Scan(&id); err != nil {
		return "", err
	}
	if id == "" {
		// Defensive: if RETURNING on DO UPDATE didn't fire, fetch by unique key
		const sel = `SELECT  job_id::text FROM consent_verifications
			WHERE principal=$1 AND resource=$2 AND challenge_hash=$3`
		if err := r.q.QueryRow(ctx, sel, principal, resource, challengeHash).Scan(&id); err != nil {
			return "", err
		}
	}
	return id, nil
}

// LeaseVerifications leases up to limit ready jobs; leaseFor defines the TTL
func (r *queries) LeaseVerifications(
	ctx context.Context,
	workerID string,
	limit int,
	leaseFor time.Duration,
) ([]domain.VerificationJob, error) {
	if workerID == "" {
		workerID = uuid.NewString()
	}
	const sqlq = `
        WITH ready AS (
            SELECT job_id
              FROM consent_verifications
             WHERE leased_by IS NULL
               AND next_attempt_at <= now()
               AND (rate_reset_at IS NULL OR rate_reset_at <= now())
             ORDER BY next_attempt_at ASC
             LIMIT $1
             FOR UPDATE SKIP LOCKED
        ), upd AS (
            UPDATE consent_verifications v
               SET leased_by = $2,
                   lease_expires_at = now() + $3::interval,
                   updated_at = now()
              WHERE v.job_id IN (SELECT job_id FROM ready)
            RETURNING v.*
        )
        SELECT job_id::text, principal::text, resource, principal_hid, challenge_hash,
               attempts, last_status, COALESCE(last_url, '') AS last_url,
               etag_branch, etag_file, etag_gists,
               rate_reset_at, next_attempt_at, COALESCE(lease_expires_at, now()) AS lease_expires_at,
               COALESCE(leased_by, $2) AS leased_by, created_at, updated_at
          FROM upd
    `
	// Pass leaseFor as string interval
	interval := leaseFor.String()
	rows, err := r.q.Query(ctx, sqlq, limit, workerID, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.VerificationJob
	for rows.Next() {
		var j domain.VerificationJob
		if err := rows.Scan(
			&j.JobID, &j.Principal, &j.Resource, &j.PrincipalHID, &j.ChallengeHash,
			&j.Attempts, &j.LastStatus, &j.LastURL,
			&j.ETagBranch, &j.ETagFile, &j.ETagGists,
			&j.RateResetAt, &j.NextAttemptAt, &j.LeaseExpires,
			&j.LeasedBy, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// CompleteVerification deletes a job after success/missing; persists diagnostics/etags one last time if provided
func (r *queries) CompleteVerification(
	ctx context.Context,
	jobID string,
	lastStatus *int,
	lastURL string,
	etagBranch, etagFile, etagGists *string,
) error {
	const upd = `
        UPDATE consent_verifications
           SET last_status = COALESCE($2, last_status),
               last_url    = COALESCE(NULLIF($3, ''), last_url),
               etag_branch = COALESCE($4, etag_branch),
               etag_file   = COALESCE($5, etag_file),
               etag_gists  = COALESCE($6, etag_gists),
               updated_at  = now()
         WHERE job_id = $1
    `
	if _, err := r.q.Exec(ctx, upd, jobID, lastStatus, lastURL, etagBranch, etagFile, etagGists); err != nil {
		return err
	}
	const del = `DELETE FROM consent_verifications WHERE job_id = $1`
	_, err := r.q.Exec(ctx, del, jobID)
	return err
}

// RequeueVerification re-schedules a job after errors and clears the lease
func (r *queries) RequeueVerification(
	ctx context.Context,
	jobID string,
	lastStatus *int,
	lastErr string,
	nextAttemptAt time.Time,
	rateResetAt *time.Time,
	etagBranch, etagFile, etagGists *string,
) error {
	const sqlq = `
        UPDATE consent_verifications
           SET attempts        = attempts + 1,
               last_status     = COALESCE($2, last_status),
               last_error      = NULLIF($3, ''),
               next_attempt_at = $4,
               rate_reset_at   = COALESCE($5, rate_reset_at),
               etag_branch     = COALESCE($6, etag_branch),
               etag_file       = COALESCE($7, etag_file),
               etag_gists      = COALESCE($8, etag_gists),
               leased_by       = NULL,
               lease_expires_at= NULL,
               updated_at      = now()
         WHERE job_id = $1
    `
	_, err := r.q.Exec(ctx, sqlq, jobID, lastStatus, lastErr, nextAttemptAt, rateResetAt, etagBranch, etagFile, etagGists)
	return err
}
