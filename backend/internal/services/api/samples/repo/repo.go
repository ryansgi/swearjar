// Package repo provides postgres access for samples
package repo

import (
	"context"

	"swearjar/internal/modkit/repokit"
)

// Repo defines the repository contract for samples
type Repo interface {
	Recent(ctx context.Context, repo, lang, category, severity string, limit int) ([]RowSample, error)
}

// RowSample represents a sample row from the database
type RowSample struct {
	UtteranceID  string
	Repo         string
	Lang         string
	Source       string
	SourceDetail string
	Text         string
	Term         string
	Category     string
	Severity     string
	DetectorVer  int
	CreatedAt    string
}

type (
	// PG implements the Repo interface using Postgres
	PG struct{}

	// queries holds the database query methods
	queries struct{ q repokit.Queryer }
)

// NewPG creates a new Postgres repository binder
func NewPG() repokit.Binder[Repo] { return PG{} }

// Bind binds a Postgres queryer to the Repo implementation
func (PG) Bind(q repokit.Queryer) Repo { return &queries{q: q} }

func (r *queries) Recent(ctx context.Context, repo, lang, category, severity string, limit int) ([]RowSample, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const sql = `
select u.id::text as utterance_id, u.repo, u.lang_code, u.source::text, u.source_detail, u.text_raw,
h.term, h.category::text, h.severity::text, h.detector_version, u.created_at::text
from hits h
join utterances u on u.id = h.utterance_id
where ($1 = '' or u.repo = $1)
and ($2 = '' or u.lang_code = $2)
and ($3 = '' or h.category::text = $3)
and ($4 = '' or h.severity::text = $4)
order by u.created_at desc
limit $5
`
	rows, err := r.q.Query(ctx, sql, repo, lang, category, severity, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RowSample
	for rows.Next() {
		var rr RowSample
		if err := rows.Scan(
			&rr.UtteranceID,
			&rr.Repo,
			&rr.Lang,
			&rr.Source,
			&rr.SourceDetail,
			&rr.Text,
			&rr.Term,
			&rr.Category,
			&rr.Severity,
			&rr.DetectorVer,
			&rr.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}
