// Package repo provides the hallmonitor repository implementation
package repo

import (
	"context"
	"database/sql"
	"time"
)

// RepoCadenceInputs returns stars and pushed_at for a repo if present
func (r *queries) RepoCadenceInputs(ctx context.Context, repoID int64) (int, *time.Time, error) {
	const sqlq = `
		SELECT stars, pushed_at
		FROM repositories
		WHERE repo_id = $1
	`
	row := r.q.QueryRow(ctx, sqlq, repoID)
	var stars sql.NullInt64
	var pushed sql.NullTime
	if err := row.Scan(&stars, &pushed); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	var pt *time.Time
	if pushed.Valid {
		pt = &pushed.Time
	}
	return int(stars.Int64), pt, nil
}

// ActorCadenceInputs returns followers for an actor if present
func (r *queries) ActorCadenceInputs(ctx context.Context, actorID int64) (int, error) {
	const sqlq = `
		SELECT followers
		FROM actors
		WHERE actor_id = $1
	`
	row := r.q.QueryRow(ctx, sqlq, actorID)
	var followers sql.NullInt64
	if err := row.Scan(&followers); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return int(followers.Int64), nil
}
