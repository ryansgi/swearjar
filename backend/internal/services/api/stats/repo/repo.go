// Package repo provides the stats repository implementation
package repo

import (
	"context"

	"swearjar/internal/modkit/repokit"
)

// Repo defines the stats repository contract
type Repo interface {
	ByLang(ctx context.Context, start, end, repo, lang, minSev string) ([]RowByLang, error)
	ByRepo(ctx context.Context, start, end, lang string) ([]RowByRepo, error)
	ByCategory(ctx context.Context, start, end, repo string) ([]RowByCategory, error)
}

// RowByLang is a bucket of hits by language and day
type RowByLang struct {
	Day        string
	Lang       string
	Hits       int64
	Utterances int64
}

// RowByRepo is a repo and its hit count
type RowByRepo struct {
	Repo string
	Hits int64
}

// RowByCategory is a bucket of hits by category and severity
type RowByCategory struct {
	Category string
	Severity string
	Hits     int64
}

type (
	// PG is a Postgres stats repository
	PG      struct{}
	queries struct{ q repokit.Queryer }
)

// NewPG constructs a Postgres stats repository (implements Repo
func NewPG() repokit.Binder[Repo] { return PG{} }

// Bind binds a Queryer to a Postgres implementation of Repo
func (PG) Bind(q repokit.Queryer) Repo { return &queries{q: q} }

// Daily by language (with optional repo/lang filters and min_severity threshold)
// Counts hits and distinct utterances per (day, lang).
func (r *queries) ByLang(ctx context.Context, start, end, repo, lang, minSev string) ([]RowByLang, error) {
	const sql = `
		with base as (
			select
				u.id                           as uid,
				u.created_at::date             as day,
				coalesce(u.lang_code,'und')    as lang
			from utterances u
			where u.created_at::date between $1 and $2
				and ($3 = '' or u.repo_name = $3)
				and ($4 = '' or u.lang_code = $4)
		),
		hit_counts as (
			select
				b.day, b.lang,
				count(h.*) as hits
			from base b
			left join hits h
				on h.utterance_id = b.uid
			and ($5 = '' or h.severity >= $5::hit_severity_enum)
			group by b.day, b.lang
		),
		utt_counts as (
			select day, lang, count(distinct uid) as utterances
			from base
			group by day, lang
		)
		select u.day::text, u.lang, coalesce(h.hits,0) as hits, u.utterances
		from utt_counts u
		left join hit_counts h using (day, lang)
		order by u.day asc, u.lang asc`
	rows, err := r.q.Query(ctx, sql, start, end, repo, lang, minSev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RowByLang
	for rows.Next() {
		var rr RowByLang
		if err := rows.Scan(&rr.Day, &rr.Lang, &rr.Hits, &rr.Utterances); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

// Top repos by hits (optional lang filter).
func (r *queries) ByRepo(ctx context.Context, start, end, lang string) ([]RowByRepo, error) {
	const sql = `
		select u.repo_name as repo, count(h.*) as hits
		from hits h
		join utterances u on u.id = h.utterance_id
		where u.created_at::date between $1 and $2
			and ($3 = '' or h.lang_code = $3)
		group by u.repo_name
		order by hits desc
		limit 200`
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

// Buckets by category/severity (optional repo filter).
func (r *queries) ByCategory(ctx context.Context, start, end, repo string) ([]RowByCategory, error) {
	const sql = `
		select h.category::text, h.severity::text, count(*) as hits
		from hits h
		join utterances u on u.id = h.utterance_id
		where u.created_at::date between $1 and $2
			and ($3 = '' or u.repo_name = $3)
		group by h.category, h.severity
		order by hits desc`
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
