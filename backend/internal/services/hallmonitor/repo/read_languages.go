// Package repo provides the hallmonitor repository implementation
package repo

import (
	"context"
	"database/sql"
	"time"

	"swearjar/internal/services/hallmonitor/domain"
)

// PrimaryLanguageOfRepo returns repositories.primary_lang for a given repo id
func (r *queries) PrimaryLanguageOfRepo(ctx context.Context, repoID int64) (string, bool, error) {
	const sqlq = `
		SELECT primary_lang
		FROM repositories
		WHERE repo_id = $1
	`
	row := r.q.QueryRow(ctx, sqlq, repoID)
	var lang sql.NullString
	if err := row.Scan(&lang); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	if !lang.Valid || lang.String == "" {
		return "", false, nil
	}
	return lang.String, true, nil
}

// LanguagesOfRepo returns the languages jsonb as a flat map for a repo
func (r *queries) LanguagesOfRepo(ctx context.Context, repoID int64) (map[string]int64, bool, error) {
	const sqlq = `
		SELECT languages
		FROM repositories
		WHERE repo_id = $1
	`
	row := r.q.QueryRow(ctx, sqlq, repoID)
	var raw map[string]int64
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(raw) == 0 {
		return nil, false, nil
	}
	return raw, true, nil
}

// PrimaryLanguageOfActor returns the dominant repo primary language used by an actor over a window
func (r *queries) PrimaryLanguageOfActor(
	ctx context.Context,
	actorID int64,
	w domain.LangWindow,
) (string, bool, error) {
	const sqlq = `
		WITH scoped AS (
			SELECT u.repo_id
			FROM utterances u
			WHERE u.actor_id = $1
			  AND ($2::timestamptz = TO_TIMESTAMP(0) OR u.created_at >= $2)
			  AND ($3::timestamptz = TO_TIMESTAMP(0) OR u.created_at <  $3)
		)
		SELECT r.primary_lang, COUNT(*) AS c
		FROM scoped s
		JOIN repositories r ON r.repo_id = s.repo_id
		WHERE r.primary_lang IS NOT NULL AND r.primary_lang <> ''
		GROUP BY r.primary_lang
		ORDER BY c DESC
		LIMIT 1
	`
	since := epochIfZero(w.Since)
	until := epochIfZero(w.Until)

	var lang string
	err := r.q.QueryRow(ctx, sqlq, actorID, since, until).Scan(&lang)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return lang, true, nil
}

// LanguagesOfActor returns a frequency map of repo primary languages for an actor over a window
func (r *queries) LanguagesOfActor(ctx context.Context, actorID int64, w domain.LangWindow) (map[string]int64, error) {
	const sqlq = `
		WITH scoped AS (
			SELECT u.repo_id
			FROM utterances u
			WHERE u.actor_id = $1
			  AND ($2::timestamptz = TO_TIMESTAMP(0) OR u.created_at >= $2)
			  AND ($3::timestamptz = TO_TIMESTAMP(0) OR u.created_at <  $3)
		)
		SELECT r.primary_lang, COUNT(*) AS c
		FROM scoped s
		JOIN repositories r ON r.repo_id = s.repo_id
		WHERE r.primary_lang IS NOT NULL AND r.primary_lang <> ''
		GROUP BY r.primary_lang
	`
	since := epochIfZero(w.Since)
	until := epochIfZero(w.Until)

	rows, err := r.q.Query(ctx, sqlq, actorID, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var lang string
		var c int64
		if err := rows.Scan(&lang, &c); err != nil {
			return nil, err
		}
		out[lang] = c
	}
	return out, rows.Err()
}

// PrimaryLanguageOfActorHID returns the dominant repo primary language using actor_hid
func (r *queries) PrimaryLanguageOfActorHID(
	ctx context.Context,
	actorHID []byte,
	w domain.LangWindow,
) (string, bool, error) {
	const sqlq = `
		WITH scoped AS (
			SELECT u.repo_id
			FROM utterances u
			WHERE u.actor_hid = $1
			  AND ($2::timestamptz = TO_TIMESTAMP(0) OR u.created_at >= $2)
			  AND ($3::timestamptz = TO_TIMESTAMP(0) OR u.created_at <  $3)
		)
		SELECT r.primary_lang, COUNT(*) AS c
		FROM scoped s
		JOIN repositories r ON r.repo_id = s.repo_id
		WHERE r.primary_lang IS NOT NULL AND r.primary_lang <> ''
		GROUP BY r.primary_lang
		ORDER BY c DESC
		LIMIT 1
	`
	since := epochIfZero(w.Since)
	until := epochIfZero(w.Until)

	var lang string
	err := r.q.QueryRow(ctx, sqlq, actorHID, since, until).Scan(&lang)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return lang, true, nil
}

// LanguagesOfActorHID returns a frequency map using actor_hid
func (r *queries) LanguagesOfActorHID(
	ctx context.Context,
	actorHID []byte,
	w domain.LangWindow,
) (map[string]int64, error) {
	const sqlq = `
		WITH scoped AS (
			SELECT u.repo_id
			FROM utterances u
			WHERE u.actor_hid = $1
			  AND ($2::timestamptz = TO_TIMESTAMP(0) OR u.created_at >= $2)
			  AND ($3::timestamptz = TO_TIMESTAMP(0) OR u.created_at <  $3)
		)
		SELECT r.primary_lang, COUNT(*) AS c
		FROM scoped s
		JOIN repositories r ON r.repo_id = s.repo_id
		WHERE r.primary_lang IS NOT NULL AND r.primary_lang <> ''
		GROUP BY r.primary_lang
	`
	since := epochIfZero(w.Since)
	until := epochIfZero(w.Until)

	rows, err := r.q.Query(ctx, sqlq, actorHID, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var lang string
		var c int64
		if err := rows.Scan(&lang, &c); err != nil {
			return nil, err
		}
		out[lang] = c
	}
	return out, rows.Err()
}

func epochIfZero(t time.Time) time.Time {
	if t.IsZero() {
		return time.Unix(0, 0).UTC()
	}
	return t.UTC()
}
