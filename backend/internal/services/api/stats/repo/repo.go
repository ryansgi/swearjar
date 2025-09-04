// Package repo provides postgres access for stats
package repo

import (
	"context"

	"swearjar/internal/modkit/repokit"
)

// Repo is the minimal persistence surface for stats
type Repo interface {
	ByLang(ctx context.Context, start, end, repo, lang, minSev string) ([]RowByLang, error)
	ByRepo(ctx context.Context, start, end, lang string) ([]RowByRepo, error)
	ByCategory(ctx context.Context, start, end, repo string) ([]RowByCategory, error)
}

// RowByLang represents a stats row by language and day
type RowByLang struct {
	Day       string
	Lang      string
	Hits      int64
	Utterings int64
}

// RowByRepo represents a stats row by repo
type RowByRepo struct {
	Repo string
	Hits int64
}

// RowByCategory represents a stats row by category and severity
type RowByCategory struct {
	Category string
	Severity string
	Hits     int64
}

type (
	// PG is a binder that can bind the repo to a Queryer or TxRunner
	PG struct{}
	// queries implements the Repo interface
	queries struct{ q repokit.Queryer }
)

// NewPG returns a binder that can bind the repo to a Queryer or TxRunner
func NewPG() repokit.Binder[Repo] { return PG{} }

// Bind wires a Queryer to the repo
func (PG) Bind(q repokit.Queryer) Repo { return &queries{q: q} }

func (r *queries) ByLang(ctx context.Context, start, end, repo, lang, minSev string) ([]RowByLang, error) {
	// no periods in comments
	const sql = `
select day::text, lang_code, sum(hits) as hits, sum(utterances) as utterances
from agg_daily_lang_spk
where day between $1 and $2
and ($3 = '' or repo = $3)
and ($4 = '' or lang_code = $4)
and ($5 = '' or min_severity >= $5)
group by day, lang_code
order by day asc, lang_code asc
`
	rows, err := r.q.Query(ctx, sql, start, end, repo, lang, minSev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RowByLang
	for rows.Next() {
		var rr RowByLang
		if err := rows.Scan(&rr.Day, &rr.Lang, &rr.Hits, &rr.Utterings); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (r *queries) ByRepo(ctx context.Context, start, end, lang string) ([]RowByRepo, error) {
	const sql = `
select repo, sum(hits) as hits
from agg_daily_lang_spk
where day between $1 and $2
and ($3 = '' or lang_code = $3)
group by repo
order by hits desc
limit 200
`
	rows, err := r.q.Query(ctx, sql, start, end, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RowByRepo
	for rows.Next() {
		var rr RowByRepo
		if err := rows.Scan(&rr.Repo, &rr.Hits); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (r *queries) ByCategory(ctx context.Context, start, end, repo string) ([]RowByCategory, error) {
	const sql = `
select category::text, severity::text, count(1) as hits
from hits h
join utterances u on u.id = h.utterance_id
where u.created_at::date between $1 and $2
and ($3 = '' or u.repo = $3)
group by category, severity
order by hits desc
`
	rows, err := r.q.Query(ctx, sql, start, end, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RowByCategory
	for rows.Next() {
		var rr RowByCategory
		if err := rows.Scan(&rr.Category, &rr.Severity, &rr.Hits); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}
