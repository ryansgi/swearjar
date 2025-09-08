// Package repo provides the hallmonitor repository implementation
package repo

import (
	"context"
	"database/sql"
)

func nullI16Ptr(n sql.NullInt16) *int16 {
	if n.Valid {
		v := int16(n.Int16)
		return &v
	}
	return nil
}

// --- Repo hints --------------------------------------------------------------

func (r *queries) RepoHints(ctx context.Context, repoID int64) (*string, *string, bool, *int16, *string, error) {
	return r.RepoHintsHID(ctx, makeRepoHID(repoID))
}

func (r *queries) RepoHintsHID(ctx context.Context, repoHID []byte) (*string, *string, bool, *int16, *string, error) {
	row := r.q.QueryRow(ctx, `
		SELECT
			full_name,
			etag,
			(gone_at IS NOT NULL) AS gone,
			gone_code,
			gone_reason
		FROM repositories
		WHERE repo_hid = $1
	`, repoHID)

	var fn, et, gr sql.NullString
	var gc sql.NullInt16
	var gone bool

	if err := row.Scan(&fn, &et, &gone, &gc, &gr); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, false, nil, nil, nil
		}
		return nil, nil, false, nil, nil, err
	}
	return nullStrPtr(fn), nullStrPtr(et), gone, nullI16Ptr(gc), nullStrPtr(gr), nil
}

// --- Actor hints -------------------------------------------------------------

func (r *queries) ActorHints(ctx context.Context, actorID int64) (*string, *string, bool, *int16, *string, error) {
	return r.ActorHintsHID(ctx, makeActorHID(actorID))
}

func (r *queries) ActorHintsHID(ctx context.Context, actorHID []byte) (*string, *string, bool, *int16, *string, error) {
	row := r.q.QueryRow(ctx, `
		SELECT
			login,
			etag,
			(gone_at IS NOT NULL) AS gone,
			gone_code,
			gone_reason
		FROM actors
		WHERE actor_hid = $1
	`, actorHID)

	var ln, et, gr sql.NullString
	var gc sql.NullInt16
	var gone bool

	if err := row.Scan(&ln, &et, &gone, &gc, &gr); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, false, nil, nil, nil
		}
		return nil, nil, false, nil, nil, err
	}
	return nullStrPtr(ln), nullStrPtr(et), gone, nullI16Ptr(gc), nullStrPtr(gr), nil
}
