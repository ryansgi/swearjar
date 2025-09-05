// Package repo provides the hallmonitor repository implementation
package repo

import (
	"context"
	"database/sql"
)

// RepoHints returns full_name and etag for a repository if present
func (r *queries) RepoHints(ctx context.Context, repoID int64) (*string, *string, error) {
	const sqlq = `
		SELECT full_name, etag
		FROM repositories
		WHERE repo_id = $1
	`
	row := r.q.QueryRow(ctx, sqlq, repoID)
	var fn sql.NullString
	var et sql.NullString
	if err := row.Scan(&fn, &et); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var fnp *string
	var etp *string
	if fn.Valid && fn.String != "" {
		fnp = &fn.String
	}
	if et.Valid && et.String != "" {
		etp = &et.String
	}
	return fnp, etp, nil
}

// ActorHints returns login and etag for an actor if present
func (r *queries) ActorHints(ctx context.Context, actorID int64) (*string, *string, error) {
	const sqlq = `
		SELECT login, etag
		FROM actors
		WHERE actor_id = $1
	`
	row := r.q.QueryRow(ctx, sqlq, actorID)
	var ln sql.NullString
	var et sql.NullString
	if err := row.Scan(&ln, &et); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var lnp *string
	var etp *string
	if ln.Valid && ln.String != "" {
		lnp = &ln.String
	}
	if et.Valid && et.String != "" {
		etp = &et.String
	}
	return lnp, etp, nil
}
