// Package repo provides repository implementations for utterances
package repo

import (
	"context"
	"strings"

	"swearjar/internal/platform/store"
	dom "swearjar/internal/services/utterances/domain"
)

// CH implements a ClickHouse reader
type CH struct {
	ch store.Clickhouse
}

// NewCH returns a CH-backed utterances repository
func NewCH(ch store.Clickhouse) *CH { return &CH{ch: ch} }

// List returns up to hardLimit rows ordered by (created_at, id)
func (r *CH) List(ctx context.Context, in dom.ListInput, limit int) ([]dom.Row, dom.AfterKey, error) {
	q := `
		SELECT
			id,
			created_at,
			repo_hid,
			actor_hid,
			source,
			source_detail,
			lang_code,
			lang_script,
			coalesce(text_normalized, '') AS text_norm
		FROM swearjar.utterances
		WHERE created_at >= ? AND created_at < ?
	`

	args := []any{in.Since.UTC(), in.Until.UTC()}

	// Optional lang filter (TODO: do better)
	if s := strings.TrimSpace(in.LangCode); s != "" {
		q += "  AND lang_code = ?\n"
		args = append(args, s)
	}

	// Keyset pagination over (created_at, id)
	const zeroUUID = "00000000-0000-0000-0000-000000000000"
	ak := in.After
	if ak.CreatedAt.IsZero() {
		ak.CreatedAt = in.Since.UTC()
	}
	idParam := strings.TrimSpace(ak.ID)
	if idParam == "" {
		idParam = zeroUUID
	}

	q += `AND ((created_at > ?) OR (created_at = ? AND id > toUUID(?)))
	      ORDER BY created_at ASC, id ASC
	      LIMIT ?`
	args = append(args, ak.CreatedAt.UTC(), ak.CreatedAt.UTC(), idParam, limit)

	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, dom.AfterKey{}, err
	}
	defer rows.Close()

	out := make([]dom.Row, 0, limit)
	for rows.Next() {
		var it dom.Row
		if err := rows.Scan(
			&it.ID, &it.CreatedAt,
			&it.RepoHID,
			&it.ActorHID,
			&it.Source, &it.SourceDetail,
			&it.LangCode, &it.Script, &it.TextNorm,
		); err != nil {
			return nil, dom.AfterKey{}, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, dom.AfterKey{}, err
	}

	var next dom.AfterKey
	if len(out) == limit {
		last := out[len(out)-1]
		next = dom.AfterKey{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, next, nil
}
