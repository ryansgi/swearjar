// Package repo provides repository implementations for utterances
package repo

import (
	"context"
	"time"

	"swearjar/internal/platform/store"
	utdom "swearjar/internal/services/utterances/domain"
)

// CH implements a ClickHouse-backed reader
type CH struct {
	ch store.Clickhouse
}

// NewCH returns a CH-backed utterances repository
func NewCH(ch store.Clickhouse) *CH { return &CH{ch: ch} }

// List returns up to hardLimit rows ordered by (created_at, id)
// Notes:
//   - CH schema does not contain repo_name/repo_id/actor_login/actor_id yet; we return zero values.
//   - Filters depending on those fields are ignored for now.
//   - LangCode filter is supported
func (r *CH) List(ctx context.Context, in utdom.ListInput, hardLimit int) ([]utdom.Row, utdom.AfterKey, error) {
	afterID := in.After.ID
	if afterID == "" {
		afterID = "00000000-0000-0000-0000-000000000000"
	}
	afterT := in.After.CreatedAt
	if afterT.IsZero() {
		// allow rows at the boundary
		afterT = in.Since.Add(-time.Nanosecond)
	}

	// Base query (selects only what exists in CH schema; zero-fill the rest)
	q := `
SELECT
  toString(id)                             AS id,
  created_at                               AS created_at,
  ''                                       AS repo_name,   -- not present in CH (zero-fill)
  toInt64(0)                                AS repo_id,     -- not present in CH (zero-fill)
  repo_hid                                  AS repo_hid,
  ''                                       AS actor_login, -- not present in CH (zero-fill)
  toInt64(0)                                AS actor_id,    -- not present in CH (zero-fill)
  actor_hid                                 AS actor_hid,
  source                                    AS source,
  source_detail                             AS source_detail,
  lang_code                                 AS lang_code,
  lang_script                               AS script,
  coalesce(text_normalized, '')             AS text_norm
FROM swearjar.utterances
WHERE created_at >= ? AND created_at < ?
  AND ((created_at > ?) OR (created_at = ? AND id > toUUID(?)))
`

	args := []any{
		in.Since.UTC(), in.Until.UTC(),
		afterT.UTC(), afterT.UTC(), afterID,
	}

	// Filters we can currently support
	if in.LangCode != "" {
		q += "  AND lang_code = ?\n"
		args = append(args, in.LangCode)
	}

	// (RepoName/Owner/RepoID/ActorID filters are ignored for now

	q += "ORDER BY created_at, id\nLIMIT ?"
	args = append(args, hardLimit)

	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, utdom.AfterKey{}, err
	}
	defer rows.Close()

	out := make([]utdom.Row, 0, hardLimit)
	var last utdom.AfterKey

	for rows.Next() {
		var rrow utdom.Row
		if err := rows.Scan(
			&rrow.ID,
			&rrow.CreatedAt,
			&rrow.RepoName,
			&rrow.RepoID,
			&rrow.RepoHID,
			&rrow.ActorLogin,
			&rrow.ActorID,
			&rrow.ActorHID,
			&rrow.Source,
			&rrow.SourceDetail,
			&rrow.LangCode,
			&rrow.Script,
			&rrow.TextNorm,
		); err != nil {
			return nil, utdom.AfterKey{}, err
		}
		out = append(out, rrow)
		last = utdom.AfterKey{CreatedAt: rrow.CreatedAt, ID: rrow.ID}
	}
	return out, last, rows.Err()
}
