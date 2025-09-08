package repo

import (
	"context"
	"database/sql"
)

// ResolveRepoGHID returns the GitHub numeric repo ID for a given HID.
// Always writes an audit row; audit failures are ignored.
func (r *queries) ResolveRepoGHID(ctx context.Context, repoHID []byte) (int64, bool, error) {
	const q = `SELECT ident.resolve_repo($1, $2, $3)`
	var id sql.NullInt64
	err := r.q.QueryRow(ctx, q, repoHID, "hallmonitor", "hallmonitor_refresh").Scan(&id)
	if err != nil {
		return 0, false, err
	}
	if !id.Valid {
		// audited inside ident.resolve_repo; treat as clean miss
		return 0, false, nil
	}
	return id.Int64, true, nil
}

// ResolveActorGHID returns the GitHub numeric user ID for a given HID.
// Always writes an audit row; audit failures are ignored.
func (r *queries) ResolveActorGHID(ctx context.Context, actorHID []byte) (int64, bool, error) {
	const q = `SELECT ident.resolve_actor($1, $2, $3)`
	var id sql.NullInt64
	err := r.q.QueryRow(ctx, q, actorHID, "hallmonitor", "hallmonitor_refresh").Scan(&id)
	if err != nil {
		return 0, false, err
	}
	if !id.Valid {
		return 0, false, nil
	}
	return id.Int64, true, nil
}
