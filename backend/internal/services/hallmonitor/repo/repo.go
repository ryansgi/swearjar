// Package repo provides the hallmonitor repository implementation
package repo

import (
	"context"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/hallmonitor/domain"
)

// Repo defines the hallmonitor repository contract
type Repo interface {
	// Signals from ingest or backfill to record sightings and enqueue work if needed
	SeenRepo(ctx context.Context, repoID int64, fullName string, seenAt time.Time) error
	SeenActor(ctx context.Context, actorID int64, login string, seenAt time.Time) error

	// Queue leasing for workers with best effort reservation semantics
	LeaseRepos(ctx context.Context, n int, leaseFor time.Duration) ([]domain.Job, error)
	LeaseActors(ctx context.Context, n int, leaseFor time.Duration) ([]domain.ActorJob, error)

	// Queue completion updates including retry backoff on failure
	AckRepo(ctx context.Context, repoID int64) error
	NackRepo(ctx context.Context, repoID int64, backoff time.Duration, lastErr string) error
	AckActor(ctx context.Context, actorID int64) error
	NackActor(ctx context.Context, actorID int64, backoff time.Duration, lastErr string) error

	// Metadata upserts after successful fetches from GitHub
	UpsertRepository(ctx context.Context, r domain.RepositoryRecord) error
	TouchRepository304(ctx context.Context, repoID int64, nextRefreshAt time.Time, etag string) error
	UpsertActor(ctx context.Context, a domain.ActorRecord) error
	TouchActor304(ctx context.Context, actorID int64, nextRefreshAt time.Time, etag string) error

	// Seed and refresh helpers to fill queues in bulk
	EnqueueMissingReposFromUtterances(ctx context.Context, since, until time.Time, limit int) (int, error)
	EnqueueMissingActorsFromUtterances(ctx context.Context, since, until time.Time, limit int) (int, error)
	EnqueueDueRepos(ctx context.Context, since, until time.Time, limit int) (int, error)
	EnqueueDueActors(ctx context.Context, since, until time.Time, limit int) (int, error)

	// Hints to avoid unnecessary fetches
	RepoHints(ctx context.Context, repoID int64) (*string, *string, error)
	ActorHints(ctx context.Context, actorID int64) (*string, *string, error)

	// Cadence inputs for next_refresh_at
	RepoCadenceInputs(ctx context.Context, repoID int64) (int, *time.Time, error)
	ActorCadenceInputs(ctx context.Context, actorID int64) (int, error)

	// Read side helpers for primary and per actor language stats
	PrimaryLanguageOfRepo(ctx context.Context, repoID int64) (string, bool, error)
	LanguagesOfRepo(ctx context.Context, repoID int64) (map[string]int64, bool, error)
	PrimaryLanguageOfActor(ctx context.Context, actorID int64, w domain.LangWindow) (string, bool, error)
	LanguagesOfActor(ctx context.Context, actorID int64, w domain.LangWindow) (map[string]int64, error)
	PrimaryLanguageOfActorHID(ctx context.Context, actorHID []byte, w domain.LangWindow) (string, bool, error)
	LanguagesOfActorHID(ctx context.Context, actorHID []byte, w domain.LangWindow) (map[string]int64, error)
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

// SeenRepo records a repo sighting and enqueues metadata fetch if missing
func (r *queries) SeenRepo(ctx context.Context, repoID int64, fullName string, seenAt time.Time) error {
	const sql = `
		INSERT INTO repositories (repo_id, full_name, fetched_at)
		VALUES ($1, NULLIF($2,''), NOW())
		ON CONFLICT (repo_id) DO UPDATE
		SET full_name = COALESCE(excluded.full_name, repositories.full_name)
	`
	if _, err := r.q.Exec(ctx, sql, repoID, fullName); err != nil {
		return err
	}
	const qsql = `
		INSERT INTO repo_catalog_queue (repo_id, priority, next_attempt_at, enqueued_at)
		VALUES ($1, 0, NOW(), NOW())
		ON CONFLICT (repo_id) DO NOTHING
	`
	_, err := r.q.Exec(ctx, qsql, repoID)
	return err
}

// SeenActor records an actor sighting and enqueues metadata fetch if missing
func (r *queries) SeenActor(ctx context.Context, actorID int64, login string, seenAt time.Time) error {
	const sql = `
		INSERT INTO actors (actor_id, login, fetched_at)
		VALUES ($1, NULLIF($2,''), NOW())
		ON CONFLICT (actor_id) DO UPDATE
		SET login = COALESCE(excluded.login, actors.login)
	`
	if _, err := r.q.Exec(ctx, sql, actorID, login); err != nil {
		return err
	}
	const qsql = `
		INSERT INTO actor_catalog_queue (actor_id, priority, next_attempt_at, enqueued_at)
		VALUES ($1, 0, NOW(), NOW())
		ON CONFLICT (actor_id) DO NOTHING
	`
	_, err := r.q.Exec(ctx, qsql, actorID)
	return err
}

// LeaseRepos leases up to n repo jobs for a duration to avoid duplicate work
func (r *queries) LeaseRepos(ctx context.Context, n int, leaseFor time.Duration) ([]domain.Job, error) {
	const sql = `
		WITH cte AS (
			SELECT repo_id
			FROM repo_catalog_queue
			WHERE next_attempt_at <= NOW()
			ORDER BY priority DESC, next_attempt_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE repo_catalog_queue q
		SET next_attempt_at = NOW() + $2::interval
		FROM cte
		WHERE q.repo_id = cte.repo_id
		RETURNING q.repo_id, q.priority, q.attempts, q.next_attempt_at
	`
	rows, err := r.q.Query(ctx, sql, n, leaseFor.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Job
	for rows.Next() {
		var j domain.Job
		if err := rows.Scan(&j.RepoID, &j.Priority, &j.Attempts, &j.NextAttemptAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// LeaseActors leases up to n actor jobs for a duration to avoid duplicate work
func (r *queries) LeaseActors(ctx context.Context, n int, leaseFor time.Duration) ([]domain.ActorJob, error) {
	const sql = `
		WITH cte AS (
			SELECT actor_id
			FROM actor_catalog_queue
			WHERE next_attempt_at <= NOW()
			ORDER BY priority DESC, next_attempt_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE actor_catalog_queue q
		SET next_attempt_at = NOW() + $2::interval
		FROM cte
		WHERE q.actor_id = cte.actor_id
		RETURNING q.actor_id, q.priority, q.attempts, q.next_attempt_at
	`
	rows, err := r.q.Query(ctx, sql, n, leaseFor.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.ActorJob
	for rows.Next() {
		var j domain.ActorJob
		if err := rows.Scan(&j.ActorID, &j.Priority, &j.Attempts, &j.NextAttemptAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// AckRepo removes a repo job from the queue after success
func (r *queries) AckRepo(ctx context.Context, repoID int64) error {
	const sql = `DELETE FROM repo_catalog_queue WHERE repo_id = $1`
	_, err := r.q.Exec(ctx, sql, repoID)
	return err
}

// NackRepo updates attempts and schedules a retry for a repo job
func (r *queries) NackRepo(ctx context.Context, repoID int64, backoff time.Duration, lastErr string) error {
	const sql = `
		UPDATE repo_catalog_queue
		SET attempts = attempts + 1,
		    last_error = LEFT($2, 500),
		    next_attempt_at = NOW() + $3::interval
		WHERE repo_id = $1
	`
	_, err := r.q.Exec(ctx, sql, repoID, lastErr, backoff.String())
	return err
}

// AckActor removes an actor job from the queue after success
func (r *queries) AckActor(ctx context.Context, actorID int64) error {
	const sql = `DELETE FROM actor_catalog_queue WHERE actor_id = $1`
	_, err := r.q.Exec(ctx, sql, actorID)
	return err
}

// NackActor updates attempts and schedules a retry for an actor job
func (r *queries) NackActor(ctx context.Context, actorID int64, backoff time.Duration, lastErr string) error {
	const sql = `
		UPDATE actor_catalog_queue
		SET attempts = attempts + 1,
		    last_error = LEFT($2, 500),
		    next_attempt_at = NOW() + $3::interval
		WHERE actor_id = $1
	`
	_, err := r.q.Exec(ctx, sql, actorID, lastErr, backoff.String())
	return err
}

// UpsertRepository writes repository facts and schedules next refresh
func (r *queries) UpsertRepository(ctx context.Context, rec domain.RepositoryRecord) error {
	const sql = `
		INSERT INTO repositories (
			repo_id, full_name, default_branch, primary_lang, languages,
			stars, forks, subscribers, open_issues, license_key, is_fork,
			pushed_at, updated_at, fetched_at, next_refresh_at, etag, api_url
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, NOW(), $14, $15, $16
		)
		ON CONFLICT (repo_id) DO UPDATE SET
			full_name      = COALESCE(excluded.full_name, repositories.full_name),
			default_branch = COALESCE(excluded.default_branch, repositories.default_branch),
			primary_lang   = COALESCE(excluded.primary_lang, repositories.primary_lang),
			languages      = COALESCE(excluded.languages, repositories.languages),
			stars          = COALESCE(excluded.stars, repositories.stars),
			forks          = COALESCE(excluded.forks, repositories.forks),
			subscribers    = COALESCE(excluded.subscribers, repositories.subscribers),
			open_issues    = COALESCE(excluded.open_issues, repositories.open_issues),
			license_key    = COALESCE(excluded.license_key, repositories.license_key),
			is_fork        = COALESCE(excluded.is_fork, repositories.is_fork),
			pushed_at      = COALESCE(excluded.pushed_at, repositories.pushed_at),
			updated_at     = COALESCE(excluded.updated_at, repositories.updated_at),
			fetched_at     = NOW(),
			next_refresh_at= COALESCE(excluded.next_refresh_at, repositories.next_refresh_at),
			etag           = COALESCE(excluded.etag, repositories.etag),
			api_url        = COALESCE(excluded.api_url, repositories.api_url)
	`
	_, err := r.q.Exec(
		ctx, sql,
		rec.RepoID, rec.FullName, rec.DefaultBranch, rec.PrimaryLang, rec.Languages,
		rec.Stars, rec.Forks, rec.Subscribers, rec.OpenIssues, rec.LicenseKey, rec.IsFork,
		rec.PushedAt, rec.UpdatedAt, rec.NextRefreshAt, rec.ETag, rec.APIURL,
	)
	return err
}

// TouchRepository304 updates timestamps and keeps schedule on 304 not modified
func (r *queries) TouchRepository304(ctx context.Context, repoID int64, nextRefreshAt time.Time, etag string) error {
	const sql = `
		UPDATE repositories
		SET fetched_at = NOW(),
		    next_refresh_at = $2,
		    etag = COALESCE(NULLIF($3,''), etag)
		WHERE repo_id = $1
	`
	_, err := r.q.Exec(ctx, sql, repoID, nextRefreshAt, etag)
	return err
}

// UpsertActor writes actor facts and schedules next refresh
func (r *queries) UpsertActor(ctx context.Context, a domain.ActorRecord) error {
	const sql = `
		INSERT INTO actors (
			actor_id, login, name, type, company, location, bio, blog, twitter_username,
			followers, following, public_repos, public_gists,
			created_at, updated_at, fetched_at, next_refresh_at, etag, api_url, opted_in_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, NOW(), $16, $17, $18, $19
		)
		ON CONFLICT (actor_id) DO UPDATE SET
			login          = COALESCE(excluded.login, actors.login),
			name           = COALESCE(excluded.name, actors.name),
			type           = COALESCE(excluded.type, actors.type),
			company        = COALESCE(excluded.company, actors.company),
			location       = COALESCE(excluded.location, actors.location),
			bio            = COALESCE(excluded.bio, actors.bio),
			blog           = COALESCE(excluded.blog, actors.blog),
			twitter_username = COALESCE(excluded.twitter_username, actors.twitter_username),
			followers      = COALESCE(excluded.followers, actors.followers),
			following      = COALESCE(excluded.following, actors.following),
			public_repos   = COALESCE(excluded.public_repos, actors.public_repos),
			public_gists   = COALESCE(excluded.public_gists, actors.public_gists),
			created_at     = COALESCE(excluded.created_at, actors.created_at),
			updated_at     = COALESCE(excluded.updated_at, actors.updated_at),
			fetched_at     = NOW(),
			next_refresh_at= COALESCE(excluded.next_refresh_at, actors.next_refresh_at),
			etag           = COALESCE(excluded.etag, actors.etag),
			api_url        = COALESCE(excluded.api_url, actors.api_url),
			opted_in_at    = COALESCE(excluded.opted_in_at, actors.opted_in_at)
	`
	_, err := r.q.Exec(
		ctx, sql,
		a.ActorID, a.Login, a.Name, a.Type, a.Company, a.Location, a.Bio, a.Blog, a.Twitter,
		a.Followers, a.Following, a.PublicRepos, a.PublicGists,
		a.CreatedAt, a.UpdatedAt, a.NextRefreshAt, a.ETag, a.APIURL, a.OptedInAt,
	)
	return err
}

// TouchActor304 updates timestamps and keeps schedule on 304 not modified
func (r *queries) TouchActor304(ctx context.Context, actorID int64, nextRefreshAt time.Time, etag string) error {
	const sql = `
		UPDATE actors
		SET fetched_at = NOW(),
		    next_refresh_at = $2,
		    etag = COALESCE(NULLIF($3,''), etag)
		WHERE actor_id = $1
	`
	_, err := r.q.Exec(ctx, sql, actorID, nextRefreshAt, etag)
	return err
}

// EnqueueMissingReposFromUtterances backfills queue for repos not yet cataloged
func (r *queries) EnqueueMissingReposFromUtterances(
	ctx context.Context,
	since, until time.Time,
	limit int,
) (int, error) {
	const sql = `
		WITH missing AS (
			SELECT DISTINCT u.repo_id
			FROM utterances u
			LEFT JOIN repositories r ON r.repo_id = u.repo_id
			WHERE u.created_at >= $1 AND u.created_at < $2 AND r.repo_id IS NULL
			LIMIT $3
		)
		INSERT INTO repo_catalog_queue (repo_id, priority, next_attempt_at, enqueued_at)
		SELECT repo_id, 0, NOW(), NOW() FROM missing
		ON CONFLICT (repo_id) DO NOTHING
		RETURNING 1
	`
	rows, err := r.q.Query(ctx, sql, since, until, limit)
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

// EnqueueMissingActorsFromUtterances backfills queue for actors not yet cataloged
func (r *queries) EnqueueMissingActorsFromUtterances(
	ctx context.Context,
	since, until time.Time,
	limit int,
) (int, error) {
	const sql = `
		WITH missing AS (
			SELECT DISTINCT u.actor_id
			FROM utterances u
			LEFT JOIN actors a ON a.actor_id = u.actor_id
			WHERE u.created_at >= $1 AND u.created_at < $2 AND a.actor_id IS NULL
			LIMIT $3
		)
		INSERT INTO actor_catalog_queue (actor_id, priority, next_attempt_at, enqueued_at)
		SELECT actor_id, 0, NOW(), NOW() FROM missing
		ON CONFLICT (actor_id) DO NOTHING
		RETURNING 1
	`
	rows, err := r.q.Query(ctx, sql, since, until, limit)
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

// EnqueueDueRepos selects repositories due for refresh and enqueues them
func (r *queries) EnqueueDueRepos(ctx context.Context, since, until time.Time, limit int) (int, error) {
	const sql = `
		WITH due AS (
			SELECT repo_id
			FROM repositories
			WHERE next_refresh_at IS NOT NULL
			  AND next_refresh_at <= NOW()
			  AND ($1 = TO_TIMESTAMP(0) OR updated_at IS NULL OR updated_at >= $1)
			  AND ($2 = TO_TIMESTAMP(0) OR updated_at IS NULL OR updated_at <  $2)
			ORDER BY next_refresh_at ASC
			LIMIT $3
		)
		INSERT INTO repo_catalog_queue (repo_id, priority, next_attempt_at, enqueued_at)
		SELECT repo_id, 0, NOW(), NOW() FROM due
		ON CONFLICT (repo_id) DO NOTHING
		RETURNING 1
	`
	// Use zero time as sentinel converted to epoch in SQL
	sinceOrEpoch := time.Unix(0, 0)
	untilOrEpoch := time.Unix(0, 0)
	if !since.IsZero() {
		sinceOrEpoch = since
	}
	if !until.IsZero() {
		untilOrEpoch = until
	}
	rows, err := r.q.Query(ctx, sql, sinceOrEpoch, untilOrEpoch, limit)
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

// EnqueueDueActors selects actors due for refresh and enqueues them
func (r *queries) EnqueueDueActors(ctx context.Context, since, until time.Time, limit int) (int, error) {
	const sql = `
		WITH due AS (
			SELECT actor_id
			FROM actors
			WHERE next_refresh_at IS NOT NULL
			  AND next_refresh_at <= NOW()
			  AND ($1 = TO_TIMESTAMP(0) OR updated_at IS NULL OR updated_at >= $1)
			  AND ($2 = TO_TIMESTAMP(0) OR updated_at IS NULL OR updated_at <  $2)
			ORDER BY next_refresh_at ASC
			LIMIT $3
		)
		INSERT INTO actor_catalog_queue (actor_id, priority, next_attempt_at, enqueued_at)
		SELECT actor_id, 0, NOW(), NOW() FROM due
		ON CONFLICT (actor_id) DO NOTHING
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
	rows, err := r.q.Query(ctx, sql, sinceOrEpoch, untilOrEpoch, limit)
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
