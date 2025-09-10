// Package repo provides the hallmonitor repository implementation
package repo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"swearjar/internal/modkit/repokit"
	perr "swearjar/internal/platform/errors"
	"swearjar/internal/services/hallmonitor/domain"
)

// Repo defines the hallmonitor repository contract
type Repo interface {
	// Signals from ingest/backfill to record sightings and enqueue work if needed (numeric convenience)
	SeenRepo(ctx context.Context, repoID int64, fullName string, seenAt time.Time) error
	SeenActor(ctx context.Context, actorID int64, login string, seenAt time.Time) error

	// HID-native signal helpers (preferred internally)
	SeenRepoHID(ctx context.Context, repoHID []byte, fullName string, seenAt time.Time) error
	SeenActorHID(ctx context.Context, actorHID []byte, login string, seenAt time.Time) error

	// Queue leasing for workers with best-effort reservation semantics (HID keyed)
	LeaseRepos(ctx context.Context, n int, leaseFor time.Duration) ([]domain.Job, error)
	LeaseActors(ctx context.Context, n int, leaseFor time.Duration) ([]domain.ActorJob, error)

	// Queue completion with retry backoff (numeric wrappers + HID-native)
	AckRepo(ctx context.Context, repoID int64) error
	NackRepo(ctx context.Context, repoID int64, backoff time.Duration, lastErr string) error
	AckActor(ctx context.Context, actorID int64) error
	NackActor(ctx context.Context, actorID int64, backoff time.Duration, lastErr string) error
	AckRepoHID(ctx context.Context, repoHID []byte) error
	NackRepoHID(ctx context.Context, repoHID []byte, backoff time.Duration, lastErr string) error
	AckActorHID(ctx context.Context, actorHID []byte) error
	NackActorHID(ctx context.Context, actorHID []byte, backoff time.Duration, lastErr string) error

	// Metadata upserts after successful fetches from GitHub (numeric wrappers + HID-native)
	UpsertRepository(ctx context.Context, r domain.RepositoryRecord) error
	UpsertRepositoryHID(ctx context.Context, repoHID []byte, r domain.RepositoryRecord) error
	TouchRepository304(ctx context.Context, repoID int64, nextRefreshAt time.Time, etag string) error
	TouchRepository304HID(ctx context.Context, repoHID []byte, nextRefreshAt time.Time, etag string) error
	UpsertActor(ctx context.Context, a domain.ActorRecord) error
	UpsertActorHID(ctx context.Context, actorHID []byte, a domain.ActorRecord) error
	TouchActor304(ctx context.Context, actorID int64, nextRefreshAt time.Time, etag string) error
	TouchActor304HID(ctx context.Context, actorHID []byte, nextRefreshAt time.Time, etag string) error

	// Seed/refresh helpers to fill queues in bulk (all HID in SQL, numeric sentinels OK)
	EnqueueMissingReposFromUtterances(ctx context.Context, since, until time.Time, limit int) (int, error)
	EnqueueMissingActorsFromUtterances(ctx context.Context, since, until time.Time, limit int) (int, error)
	EnqueueDueRepos(ctx context.Context, since, until time.Time, limit int) (int, error)
	EnqueueDueActors(ctx context.Context, since, until time.Time, limit int) (int, error)

	// Hints to avoid unnecessary fetches (numeric wrappers + HID-native)
	RepoHints(
		ctx context.Context,
		repoID int64,
	) (fullName, etag *string, gone bool, code *int16, reason *string, err error)
	RepoHintsHID(
		ctx context.Context,
		repoHID []byte,
	) (fullName, etag *string, gone bool, code *int16, reason *string, err error)
	ActorHints(
		ctx context.Context,
		actorID int64,
	) (login, etag *string, gone bool, code *int16, reason *string, err error)
	ActorHintsHID(
		ctx context.Context,
		actorHID []byte,
	) (
		login, etag *string,
		gone bool,
		code *int16,
		reason *string,
		err error,
	)

	// Cadence inputs for next_refresh_at (numeric wrappers + HID-native)
	RepoCadenceInputs(ctx context.Context, repoID int64) (int, *time.Time, error)
	RepoCadenceInputsHID(ctx context.Context, repoHID []byte) (int, *time.Time, error)
	ActorCadenceInputs(ctx context.Context, actorID int64) (int, error)
	ActorCadenceInputsHID(ctx context.Context, actorHID []byte) (int, error)

	// Read-side helpers for primary and per-actor language stats
	PrimaryLanguageOfRepo(ctx context.Context, repoID int64) (string, bool, error)
	LanguagesOfRepo(ctx context.Context, repoID int64) (map[string]int64, bool, error)
	PrimaryLanguageOfActor(ctx context.Context, actorID int64, w domain.LangWindow) (string, bool, error)
	LanguagesOfActor(ctx context.Context, actorID int64, w domain.LangWindow) (map[string]int64, error)
	PrimaryLanguageOfActorHID(ctx context.Context, actorHID []byte, w domain.LangWindow) (string, bool, error)
	LanguagesOfActorHID(ctx context.Context, actorHID []byte, w domain.LangWindow) (map[string]int64, error)

	ResolveRepoGHID(ctx context.Context, repoHID []byte) (id int64, ok bool, err error)
	ResolveActorGHID(ctx context.Context, actorHID []byte) (id int64, ok bool, err error)

	TombstoneRepository(ctx context.Context, repoID int64, code int, reason string, nextRefresh time.Duration) error
	TombstoneRepositoryHID(ctx context.Context, repoHID []byte, code int, reason string, nextRefresh time.Duration) error
	TombstoneActor(ctx context.Context, actorID int64, code int, reason string, nextRefresh time.Duration) error
	TombstoneActorHID(ctx context.Context, actorHID []byte, code int, reason string, nextRefresh time.Duration) error
}

type (
	// PG is a Postgres hallmonitor repository
	PG      struct{}
	queries struct{ q repokit.Queryer }
)

// NewPG constructs a Postgres hallmonitor repository
func NewPG() repokit.Binder[Repo] { return PG{} }

// Bind binds a Queryer to a Postgres implementation of Repo
func (PG) Bind(q repokit.Queryer) Repo { return &queries{q: q} }

// HID derivation (must match ingest/backfill)
func makeRepoHID(repoID int64) []byte {
	h := sha256.Sum256([]byte("repo:" + strconv.FormatInt(repoID, 10)))
	return h[:]
}

func makeActorHID(actorID int64) []byte {
	h := sha256.Sum256([]byte("actor:" + strconv.FormatInt(actorID, 10)))
	return h[:]
}

// --- Signals ---------------------------------------------------------------

func (r *queries) SeenRepo(ctx context.Context, repoID int64, fullName string, seenAt time.Time) error {
	return r.SeenRepoHID(ctx, makeRepoHID(repoID), fullName, seenAt)
}

// Ensure minimal principal + enqueue (no ident.* writes here)
func (r *queries) SeenRepoHID(ctx context.Context, repoHID []byte, fullName string, seenAt time.Time) error {
	// Skip enqueue if explicitly denied; principal row still allowed
	const denySQL = `SELECT EXISTS (SELECT 1 FROM active_deny_repos WHERE principal_hid=$1)`
	var denied bool
	if err := r.q.QueryRow(ctx, denySQL, repoHID).Scan(&denied); err != nil {
		return err
	}
	// Ensure principal (idempotent)
	if _, err := r.q.Exec(ctx,
		`INSERT INTO principals_repos (repo_hid) VALUES ($1) ON CONFLICT DO NOTHING`,
		repoHID,
	); err != nil {
		return fmt.Errorf("ensure principals_repos: %w", err)
	}
	// If we know a friendly label, set it; guarded by CHECK(can_expose_repo(...))
	if fullName != "" {
		_, _ = r.q.Exec(ctx, `UPDATE principals_repos SET _label_explicit = $2 WHERE repo_hid = $1`, repoHID, fullName)
	}
	if denied {
		return nil
	}
	// Enqueue
	_, err := r.q.Exec(ctx, `
		INSERT INTO repo_catalog_queue (repo_hid, priority, next_attempt_at, enqueued_at)
		VALUES ($1, 0, now(), now())
		ON CONFLICT (repo_hid) DO NOTHING
	`, repoHID)
	return err
}

func (r *queries) SeenActor(ctx context.Context, actorID int64, login string, seenAt time.Time) error {
	return r.SeenActorHID(ctx, makeActorHID(actorID), login, seenAt)
}

func (r *queries) SeenActorHID(ctx context.Context, actorHID []byte, login string, seenAt time.Time) error {
	const denySQL = `SELECT EXISTS (SELECT 1 FROM active_deny_actors WHERE principal_hid=$1)`
	var denied bool
	if err := r.q.QueryRow(ctx, denySQL, actorHID).Scan(&denied); err != nil {
		return err
	}
	if _, err := r.q.Exec(
		ctx,
		`INSERT INTO principals_actors (actor_hid) VALUES ($1) ON CONFLICT DO NOTHING`,
		actorHID,
	); err != nil {
		return fmt.Errorf("ensure principals_actors: %w", err)
	}
	if login != "" {
		_, _ = r.q.Exec(ctx, `UPDATE principals_actors SET _label_explicit = $2 WHERE actor_hid = $1`, actorHID, login)
	}
	if denied {
		return nil
	}
	_, err := r.q.Exec(ctx, `
		INSERT INTO actor_catalog_queue (actor_hid, priority, next_attempt_at, enqueued_at)
		VALUES ($1, 0, now(), now())
		ON CONFLICT (actor_hid) DO NOTHING
	`, actorHID)
	return err
}

// === Leasing ================================================================

func (r *queries) LeaseRepos(ctx context.Context, n int, leaseFor time.Duration) ([]domain.Job, error) {
	const sqlQ = `
		WITH due AS (
			SELECT repo_hid
			FROM repo_catalog_queue
			WHERE next_attempt_at <= now()
			ORDER BY priority DESC, next_attempt_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE repo_catalog_queue q
		SET next_attempt_at = now() + $2::interval
		FROM due
		WHERE q.repo_hid = due.repo_hid
		RETURNING q.repo_hid, q.priority, q.attempts, q.next_attempt_at
	`
	rows, err := r.q.Query(ctx, sqlQ, n, leaseFor.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Job, 0, n)
	for rows.Next() {
		var j domain.Job
		if err := rows.Scan(&j.RepoHID, &j.Priority, &j.Attempts, &j.NextAttemptAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (r *queries) LeaseActors(ctx context.Context, n int, leaseFor time.Duration) ([]domain.ActorJob, error) {
	const sqlQ = `
		WITH due AS (
			SELECT actor_hid
			FROM actor_catalog_queue
			WHERE next_attempt_at <= now()
			ORDER BY priority DESC, next_attempt_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE actor_catalog_queue q
		SET next_attempt_at = now() + $2::interval
		FROM due
		WHERE q.actor_hid = due.actor_hid
		RETURNING q.actor_hid, q.priority, q.attempts, q.next_attempt_at
	`
	rows, err := r.q.Query(ctx, sqlQ, n, leaseFor.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.ActorJob, 0, n)
	for rows.Next() {
		var j domain.ActorJob
		if err := rows.Scan(&j.ActorHID, &j.Priority, &j.Attempts, &j.NextAttemptAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// === Ack/Nack ==============================================================

func (r *queries) AckRepo(ctx context.Context, repoID int64) error {
	return r.AckRepoHID(ctx, makeRepoHID(repoID))
}

func (r *queries) NackRepo(ctx context.Context, repoID int64, backoff time.Duration, lastErr string) error {
	return r.NackRepoHID(ctx, makeRepoHID(repoID), backoff, lastErr)
}

func (r *queries) AckActor(ctx context.Context, actorID int64) error {
	return r.AckActorHID(ctx, makeActorHID(actorID))
}

func (r *queries) NackActor(ctx context.Context, actorID int64, backoff time.Duration, lastErr string) error {
	return r.NackActorHID(ctx, makeActorHID(actorID), backoff, lastErr)
}

func (r *queries) AckRepoHID(ctx context.Context, repoHID []byte) error {
	_, err := r.q.Exec(ctx, `DELETE FROM repo_catalog_queue WHERE repo_hid = $1`, repoHID)
	return err
}

func (r *queries) NackRepoHID(ctx context.Context, repoHID []byte, backoff time.Duration, lastErr string) error {
	_, err := r.q.Exec(ctx, `
		UPDATE repo_catalog_queue
		SET attempts = attempts + 1,
		    last_error = LEFT($2, 500),
		    next_attempt_at = now() + $3::interval
		WHERE repo_hid = $1
	`, repoHID, lastErr, backoff.String())
	return err
}

func (r *queries) AckActorHID(ctx context.Context, actorHID []byte) error {
	_, err := r.q.Exec(ctx, `DELETE FROM actor_catalog_queue WHERE actor_hid = $1`, actorHID)
	return err
}

func (r *queries) NackActorHID(ctx context.Context, actorHID []byte, backoff time.Duration, lastErr string) error {
	_, err := r.q.Exec(ctx, `
		UPDATE actor_catalog_queue
		SET attempts = attempts + 1,
		    last_error = LEFT($2, 500),
		    next_attempt_at = now() + $3::interval
		WHERE actor_hid = $1
	`, actorHID, lastErr, backoff.String())
	return err
}

// --- Upserts (consent-aware, HID-native core) --------------------------------

func (r *queries) UpsertRepository(ctx context.Context, rec domain.RepositoryRecord) error {
	return r.UpsertRepositoryHID(ctx, makeRepoHID(rec.RepoID), rec)
}

// UpsertRepositoryHID upserts a repository record using repo_hid
func (r *queries) UpsertRepositoryHID(ctx context.Context, repoHID []byte, rec domain.RepositoryRecord) error {
	// Ensure principal exists (idempotent)
	if _, err := r.q.Exec(ctx,
		`INSERT INTO principals_repos (repo_hid) VALUES ($1) ON CONFLICT DO NOTHING`,
		repoHID,
	); err != nil {
		return perr.FromPostgresWithField(err, "ensure principals_repos")
	}

	// Optional consent_id
	var consentID *string
	if err := r.q.QueryRow(ctx, `
		SELECT consent_id::text
		FROM consent_receipts
		WHERE principal='repo' AND principal_hid=$1 AND action='opt_in' AND state='active'
		LIMIT 1
	`, repoHID).Scan(&consentID); err != nil {
		if err.Error() != "no rows in result set" {
			return perr.FromPostgresWithField(err, "lookup repo consent")
		}
	}

	const sqlUpsert = `
		INSERT INTO repositories (
			repo_hid, consent_id,
			full_name, api_url,
			default_branch, primary_lang, languages,
			stars, forks, subscribers, open_issues, license_key, is_fork,
			pushed_at, updated_at, fetched_at, next_refresh_at, etag
		) VALUES (
			$1, $2::uuid,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($3,'') ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($17,'') ELSE NULL END,
			NULLIF($4,''), NULLIF($5,''), $6,
			$7, $8, $9, $10, NULLIF($11,''), $12,
			$13, $14, now(), $15, NULLIF($16,'')
		)
		ON CONFLICT (repo_hid) DO UPDATE SET
			default_branch  = COALESCE(excluded.default_branch, repositories.default_branch),
			primary_lang    = COALESCE(excluded.primary_lang, repositories.primary_lang),
			languages       = COALESCE(excluded.languages, repositories.languages),
			stars           = COALESCE(excluded.stars, repositories.stars),
			forks           = COALESCE(excluded.forks, repositories.forks),
			subscribers     = COALESCE(excluded.subscribers, repositories.subscribers),
			open_issues     = COALESCE(excluded.open_issues, repositories.open_issues),
			license_key     = COALESCE(excluded.license_key, repositories.license_key),
			is_fork         = COALESCE(excluded.is_fork, repositories.is_fork),
			pushed_at       = COALESCE(excluded.pushed_at, repositories.pushed_at),
			updated_at      = COALESCE(excluded.updated_at, repositories.updated_at),
			fetched_at      = now(),
			next_refresh_at = COALESCE(excluded.next_refresh_at, repositories.next_refresh_at),
			etag            = COALESCE(excluded.etag, repositories.etag),
			consent_id      = COALESCE(excluded.consent_id, repositories.consent_id),
			full_name       = CASE WHEN COALESCE(repositories.consent_id, excluded.consent_id) IS NOT NULL
			                       THEN COALESCE(excluded.full_name, repositories.full_name)
			                       ELSE NULL END,
			api_url         = CASE WHEN COALESCE(repositories.consent_id, excluded.consent_id) IS NOT NULL
			                       THEN COALESCE(excluded.api_url, repositories.api_url)
			                       ELSE NULL END
	`
	_, err := r.q.Exec(ctx, sqlUpsert,
		repoHID, consentID,
		rec.FullName,
		rec.DefaultBranch, rec.PrimaryLang, rec.Languages,
		rec.Stars, rec.Forks, rec.Subscribers, rec.OpenIssues, rec.LicenseKey, rec.IsFork,
		rec.PushedAt, rec.UpdatedAt, rec.NextRefreshAt, rec.ETag,
		rec.APIURL,
	)
	return perr.FromPostgresWithField(err, "upsert repositories (HID)")
}

func (r *queries) TouchRepository304(ctx context.Context, repoID int64, nextRefreshAt time.Time, etag string) error {
	return r.TouchRepository304HID(ctx, makeRepoHID(repoID), nextRefreshAt, etag)
}

func (r *queries) TouchRepository304HID(
	ctx context.Context,
	repoHID []byte,
	nextRefreshAt time.Time,
	etag string,
) error {
	_, err := r.q.Exec(ctx, `
		UPDATE repositories
		SET fetched_at = now(),
		    next_refresh_at = $2,
		    etag = COALESCE(NULLIF($3,''), etag)
		WHERE repo_hid = $1
	`, repoHID, nextRefreshAt, etag)
	return err
}

func (r *queries) UpsertActor(ctx context.Context, a domain.ActorRecord) error {
	return r.UpsertActorHID(ctx, makeActorHID(a.ActorID), a)
}

func (r *queries) UpsertActorHID(ctx context.Context, actorHID []byte, a domain.ActorRecord) error {
	// Ensure principal exists (idempotent)
	if _, err := r.q.Exec(ctx,
		`INSERT INTO principals_actors (actor_hid) VALUES ($1) ON CONFLICT DO NOTHING`,
		actorHID,
	); err != nil {
		return perr.FromPostgresWithField(err, "ensure principals_actors")
	}

	// Optional consent_id (NULL when no active opt-in)
	var consentID *string
	if err := r.q.QueryRow(ctx, `
		SELECT consent_id::text
		FROM consent_receipts
		WHERE principal='actor' AND principal_hid=$1 AND action='opt_in' AND state='active'
		LIMIT 1
	`, actorHID).Scan(&consentID); err != nil {
		if err.Error() != "no rows in result set" {
			return perr.FromPostgresWithField(err, "lookup actor consent")
		}
	}

	const sqlUpsert = `
		INSERT INTO actors (
			actor_hid, consent_id,
			login, name, type, company, location, bio, blog, twitter_username, api_url,
			followers, following, public_repos, public_gists,
			created_at, updated_at, fetched_at, next_refresh_at, etag
		) VALUES (
			$1, $2::uuid,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($3,'')  ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($4,'')  ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($5,'')  ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($6,'')  ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($7,'')  ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($8,'')  ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($9,'')  ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($10,'') ELSE NULL END,
			CASE WHEN $2::uuid IS NOT NULL THEN NULLIF($19,'') ELSE NULL END,
			$11, $12, $13, $14,
			$15, $16, now(), $17, NULLIF($18,'')
		)
		ON CONFLICT (actor_hid) DO UPDATE SET
			followers       = COALESCE(excluded.followers, actors.followers),
			following       = COALESCE(excluded.following, actors.following),
			public_repos    = COALESCE(excluded.public_repos, actors.public_repos),
			public_gists    = COALESCE(excluded.public_gists, actors.public_gists),
			created_at      = COALESCE(excluded.created_at, actors.created_at),
			updated_at      = COALESCE(excluded.updated_at, actors.updated_at),
			fetched_at      = now(),
			next_refresh_at = COALESCE(excluded.next_refresh_at, actors.next_refresh_at),
			etag            = COALESCE(excluded.etag, actors.etag),
			consent_id      = COALESCE(excluded.consent_id, actors.consent_id),
			login = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			             THEN COALESCE(excluded.login, actors.login) ELSE NULL END,
			name = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			            THEN COALESCE(excluded.name, actors.name) ELSE NULL END,
			type = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			            THEN COALESCE(excluded.type, actors.type) ELSE actors.type END,
			company = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			               THEN COALESCE(excluded.company, actors.company) ELSE NULL END,
			location = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			                THEN COALESCE(excluded.location, actors.location) ELSE NULL END,
			bio = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			           THEN COALESCE(excluded.bio, actors.bio) ELSE NULL END,
			blog = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			            THEN COALESCE(excluded.blog, actors.blog) ELSE NULL END,
			twitter_username = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			                        THEN COALESCE(excluded.twitter_username, actors.twitter_username) ELSE NULL END,
			api_url = CASE WHEN COALESCE(actors.consent_id, excluded.consent_id) IS NOT NULL
			               THEN COALESCE(excluded.api_url, actors.api_url) ELSE NULL END
	`
	_, err := r.q.Exec(ctx, sqlUpsert,
		actorHID, consentID,
		a.Login, a.Name, a.Type, a.Company, a.Location, a.Bio, a.Blog, a.Twitter,
		a.Followers, a.Following, a.PublicRepos, a.PublicGists,
		a.CreatedAt, a.UpdatedAt, a.NextRefreshAt, a.ETag, a.APIURL,
	)
	return perr.FromPostgresWithField(err, "upsert actors (HID)")
}

func (r *queries) TouchActor304(ctx context.Context, actorID int64, nextRefreshAt time.Time, etag string) error {
	return r.TouchActor304HID(ctx, makeActorHID(actorID), nextRefreshAt, etag)
}

func (r *queries) TouchActor304HID(ctx context.Context, actorHID []byte, nextRefreshAt time.Time, etag string) error {
	_, err := r.q.Exec(ctx, `
		UPDATE actors
		SET fetched_at = now(),
		    next_refresh_at = $2,
		    etag = COALESCE(NULLIF($3,''), etag)
		WHERE actor_hid = $1
	`, actorHID, nextRefreshAt, etag)
	return err
}

// --- Enqueue (bulk seed/refresh) ---------------------------------------------

func (r *queries) EnqueueMissingReposFromUtterances(
	ctx context.Context,
	since, until time.Time,
	limit int,
) (int, error) {
	const sqlQ = `
		WITH missing AS (
			SELECT DISTINCT u.repo_hid
			FROM utterances u
			LEFT JOIN repositories r ON r.repo_hid = u.repo_hid
			WHERE u.created_at >= $1 AND u.created_at < $2 AND r.repo_hid IS NULL
			LIMIT NULLIF($3, 0)
		)
		INSERT INTO repo_catalog_queue (repo_hid, priority, next_attempt_at, enqueued_at)
		SELECT repo_hid, 0, now(), now() FROM missing
		ON CONFLICT (repo_hid) DO NOTHING
		RETURNING 1
	`
	rows, err := r.q.Query(ctx, sqlQ, since, until, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var one int
		if err := rows.Scan(&one); err != nil {
			return n, err
		}
		n += one
	}
	return n, rows.Err()
}

func (r *queries) EnqueueMissingActorsFromUtterances(
	ctx context.Context,
	since, until time.Time,
	limit int,
) (int, error) {
	const sqlQ = `
		WITH missing AS (
			SELECT DISTINCT u.actor_hid
			FROM utterances u
			LEFT JOIN actors a ON a.actor_hid = u.actor_hid
			WHERE u.created_at >= $1 AND u.created_at < $2 AND a.actor_hid IS NULL
			LIMIT NULLIF($3, 0)
		)
		INSERT INTO actor_catalog_queue (actor_hid, priority, next_attempt_at, enqueued_at)
		SELECT actor_hid, 0, now(), now() FROM missing
		ON CONFLICT (actor_hid) DO NOTHING
		RETURNING 1
	`
	rows, err := r.q.Query(ctx, sqlQ, since, until, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var one int
		if err := rows.Scan(&one); err != nil {
			return n, err
		}
		n += one
	}
	return n, rows.Err()
}

func (r *queries) EnqueueDueRepos(ctx context.Context, since, until time.Time, limit int) (int, error) {
	const sqlQ = `
		WITH due AS (
			SELECT repo_hid
			FROM repositories
			WHERE next_refresh_at IS NOT NULL
			  AND next_refresh_at <= now()
			  AND ($1 = to_timestamp(0) OR updated_at IS NULL OR updated_at >= $1)
			  AND ($2 = to_timestamp(0) OR updated_at IS NULL OR updated_at <  $2)
			ORDER BY next_refresh_at ASC
			LIMIT NULLIF($3, 0)
		)
		INSERT INTO repo_catalog_queue (repo_hid, priority, next_attempt_at, enqueued_at)
		SELECT repo_hid, 0, now(), now() FROM due
		ON CONFLICT (repo_hid) DO NOTHING
		RETURNING 1
	`
	sinceOrEpoch := time.Unix(0, 0)
	untilOrEpoch := time.Unix(0, 0)
	if !since.IsZero() {
		sinceOrEpoch = since
	}
	if !until.IsZero() {
		untilOrEpoch = until
	}
	rows, err := r.q.Query(ctx, sqlQ, sinceOrEpoch, untilOrEpoch, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var one int
		if err := rows.Scan(&one); err != nil {
			return n, err
		}
		n += one
	}
	return n, rows.Err()
}

func (r *queries) EnqueueDueActors(ctx context.Context, since, until time.Time, limit int) (int, error) {
	const sqlQ = `
		WITH due AS (
			SELECT actor_hid
			FROM actors
			WHERE next_refresh_at IS NOT NULL
			  AND next_refresh_at <= now()
			  AND ($1 = to_timestamp(0) OR updated_at IS NULL OR updated_at >= $1)
			  AND ($2 = to_timestamp(0) OR updated_at IS NULL OR updated_at <  $2)
			ORDER BY next_refresh_at ASC
			LIMIT NULLIF($3, 0)
		)
		INSERT INTO actor_catalog_queue (actor_hid, priority, next_attempt_at, enqueued_at)
		SELECT actor_hid, 0, now(), now() FROM due
		ON CONFLICT (actor_hid) DO NOTHING
		RETURNING 1
	`
	sinceOrEpoch := time.Unix(0, 0)
	untilOrEpoch := time.Unix(0, 0)
	if !since.IsZero() {
		sinceOrEpoch = since
	}
	if !until.IsZero() {
		untilOrEpoch = until
	}
	rows, err := r.q.Query(ctx, sqlQ, sinceOrEpoch, untilOrEpoch, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var one int
		if err := rows.Scan(&one); err != nil {
			return n, err
		}
		n += one
	}
	return n, rows.Err()
}

// Helpers
func nullStrPtr(ns sql.NullString) *string {
	if ns.Valid && ns.String != "" {
		return &ns.String
	}
	return nil
}
