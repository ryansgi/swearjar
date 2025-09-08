package repo

import (
	"context"
	"time"

	perr "swearjar/internal/platform/errors"
)

// TombstoneRepository marks a repository as gone (404, 410, etc) with a reason and optional next-refresh delay
func (r *queries) TombstoneRepository(
	ctx context.Context,
	repoID int64,
	code int,
	reason string,
	nextRefresh time.Duration,
) error {
	return r.TombstoneRepositoryHID(ctx, makeRepoHID(repoID), code, reason, nextRefresh)
}

// TombstoneRepositoryHID marks a repository as gone (404, 410, etc) with a reason and optional next-refresh delay
func (r *queries) TombstoneRepositoryHID(
	ctx context.Context,
	repoHID []byte,
	code int,
	reason string,
	nextRefresh time.Duration,
) error {
	// Ensure principal exists (idempotent safety)
	if _, err := r.q.Exec(ctx,
		`INSERT INTO principals_repos (repo_hid) VALUES ($1) ON CONFLICT DO NOTHING`,
		repoHID,
	); err != nil {
		return perr.FromPostgresWithField(err, "ensure principals_repos")
	}

	_, err := r.q.Exec(ctx, `
		INSERT INTO repositories (
			repo_hid, fetched_at, gone_at, gone_code, gone_reason, next_refresh_at
		) VALUES (
			$1, now(), now(), $2, $3, now() + $4::interval
		)
		ON CONFLICT (repo_hid) DO UPDATE SET
			gone_at        = COALESCE(repositories.gone_at, EXCLUDED.gone_at),
			gone_code      = EXCLUDED.gone_code,
			gone_reason    = EXCLUDED.gone_reason,
			fetched_at     = now(),
			next_refresh_at= now() + $4::interval
	`, repoHID, code, reason, nextRefresh.String())
	if err != nil {
		return perr.FromPostgresWithField(err, "tombstone repository upsert")
	}

	_, err = r.q.Exec(ctx, `DELETE FROM repo_catalog_queue WHERE repo_hid=$1`, repoHID)
	return perr.FromPostgresWithField(err, "delete repo queue row")
}

// TombstoneActor marks an actor as gone (404, 410, etc) with a reason and optional next-refresh delay
func (r *queries) TombstoneActor(
	ctx context.Context,
	actorID int64,
	code int,
	reason string,
	nextRefresh time.Duration,
) error {
	return r.TombstoneActorHID(ctx, makeActorHID(actorID), code, reason, nextRefresh)
}

// TombstoneActor marks an actor as gone (404, 410, etc) with a reason and optional next-refresh delay
func (r *queries) TombstoneActorHID(
	ctx context.Context,
	actorHID []byte,
	code int,
	reason string,
	nextRefresh time.Duration,
) error {
	// Ensure principal exists (idempotent safety)
	if _, err := r.q.Exec(ctx,
		`INSERT INTO principals_actors (actor_hid) VALUES ($1) ON CONFLICT DO NOTHING`,
		actorHID,
	); err != nil {
		return perr.FromPostgresWithField(err, "ensure principals_actors")
	}

	_, err := r.q.Exec(ctx, `
		INSERT INTO actors (
			actor_hid, fetched_at, gone_at, gone_code, gone_reason, next_refresh_at
		) VALUES (
			$1, now(), now(), $2, $3, now() + $4::interval
		)
		ON CONFLICT (actor_hid) DO UPDATE SET
			gone_at        = COALESCE(actors.gone_at, EXCLUDED.gone_at),
			gone_code      = EXCLUDED.gone_code,
			gone_reason    = EXCLUDED.gone_reason,
			fetched_at     = now(),
			next_refresh_at= now() + $4::interval
	`, actorHID, code, reason, nextRefresh.String())
	if err != nil {
		return perr.FromPostgresWithField(err, "tombstone actor upsert")
	}

	_, err = r.q.Exec(ctx, `DELETE FROM actor_catalog_queue WHERE actor_hid=$1`, actorHID)
	return perr.FromPostgresWithField(err, "delete actor queue row")
}
