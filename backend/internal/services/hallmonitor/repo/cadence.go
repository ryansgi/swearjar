// Package repo provides the hallmonitor repository implementation
package repo

import (
	"context"
	"database/sql"
	"time"
)

func (r *queries) RepoCadenceInputs(ctx context.Context, repoID int64) (int, *time.Time, error) {
	return r.RepoCadenceInputsHID(ctx, makeRepoHID(repoID))
}

func (r *queries) RepoCadenceInputsHID(ctx context.Context, repoHID []byte) (int, *time.Time, error) {
	row := r.q.QueryRow(ctx, `SELECT stars, pushed_at FROM repositories WHERE repo_hid = $1`, repoHID)
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

func (r *queries) ActorCadenceInputs(ctx context.Context, actorID int64) (int, error) {
	return r.ActorCadenceInputsHID(ctx, makeActorHID(actorID))
}

func (r *queries) ActorCadenceInputsHID(ctx context.Context, actorHID []byte) (int, error) {
	row := r.q.QueryRow(ctx, `SELECT followers FROM actors WHERE actor_hid = $1`, actorHID)
	var followers sql.NullInt64
	if err := row.Scan(&followers); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return int(followers.Int64), nil
}
