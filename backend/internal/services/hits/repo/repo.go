// Package repo provides the hits repository implementation.
package repo

import (
	"context"
	"fmt"
	"strings"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/hits/domain"
)

type (
	pg     struct{ q repokit.Queryer }
	binder struct{}
)

// NewPG constructs a new repo binder for Postgres
func NewPG() repokit.Binder[Storage] { return binder{} }

// Bind implements repokit.Binder
func (binder) Bind(q repokit.Queryer) Storage { return &pg{q: q} }

// Storage defines the hits repository
type Storage interface {
	WriteBatch(ctx context.Context, xs []domain.HitWrite) error
	ListSamples(
		ctx context.Context,
		w domain.Window,
		f domain.Filters,
		after domain.AfterKey,
		limit int,
	) ([]domain.Sample, domain.AfterKey, error)
	AggByLang(ctx context.Context, w domain.Window, f domain.Filters) ([]domain.AggByLangRow, error)
	AggByRepo(ctx context.Context, w domain.Window, f domain.Filters, limit int) ([]domain.AggByRepoRow, error)
	AggByCategory(ctx context.Context, w domain.Window, f domain.Filters) ([]domain.AggByCategoryRow, error)
}

// WriteBatch implements Storage
func (s *pg) WriteBatch(ctx context.Context, xs []domain.HitWrite) error {
	if len(xs) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(`INSERT INTO hits
		(utterance_id, created_at, term, category, severity, span_start, span_end,
		detector_version, source, repo_hid, actor_hid, lang_code) VALUES `)

	args := make([]any, 0, len(xs)*12)
	for i, h := range xs {
		if i > 0 {
			sb.WriteByte(',')
		}
		base := i*12 + 1
		fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base, base+1, base+2, base+3, base+4, base+5,
			base+6, base+7, base+8, base+9, base+10, base+11)

		args = append(args,
			h.UtteranceID, h.CreatedAt, h.Term, h.Category, h.Severity,
			h.SpanStart, h.SpanEnd, h.DetectorVersion, h.Source,
			h.RepoHID, h.ActorHID, h.LangCode,
		)
	}
	// Idempotent for same detector_version & span
	sb.WriteString(` ON CONFLICT (utterance_id, term, span_start, span_end, detector_version) DO NOTHING`)
	_, err := s.q.Exec(ctx, sb.String(), args...)
	return err
}

func (s *pg) ListSamples(
	ctx context.Context,
	w domain.Window,
	f domain.Filters,
	after domain.AfterKey,
	limit int,
) ([]domain.Sample, domain.AfterKey, error) {
	var sb strings.Builder
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	sb.WriteString(`
		SELECT
			h.utterance_id::text,
			u.created_at,
			u.repo_name,
			u.lang_code,
			u.source::text,
			h.term, h.category::text, h.severity::text, h.span_start, h.span_end
		FROM hits h
		JOIN utterances u ON u.id = h.utterance_id
		WHERE u.created_at >= ` + arg(w.Since) + ` AND u.created_at < ` + arg(w.Until) + `
			AND NOT EXISTS (SELECT 1 FROM active_deny_repos r WHERE r.principal_hid = u.repo_hid)
			AND NOT EXISTS (SELECT 1 FROM active_deny_actors a WHERE a.principal_hid = u.actor_hid)
	`)
	// Keyset only when AfterKey is set (avoid ""::uuid on first page)
	if after.UtteranceID != "" {
		sb.WriteString(
			"  AND (u.created_at, u.id) > (" +
				arg(after.CreatedAt) + ", " +
				arg(after.UtteranceID) + "::uuid)\n",
		)
	}

	if f.RepoName != "" {
		sb.WriteString("  AND u.repo_name = " + arg(f.RepoName) + "\n")
	}
	if f.Owner != "" {
		sb.WriteString("  AND split_part(u.repo_name, '/', 1) = " + arg(f.Owner) + "\n")
	}
	if f.RepoID != nil {
		sb.WriteString("  AND u.repo_id = " + arg(*f.RepoID) + "\n")
	}
	if f.ActorLogin != "" {
		sb.WriteString("  AND u.actor_login = " + arg(f.ActorLogin) + "\n")
	}
	if f.ActorID != nil {
		sb.WriteString("  AND u.actor_id = " + arg(*f.ActorID) + "\n")
	}
	if f.LangCode != "" {
		sb.WriteString("  AND u.lang_code = " + arg(f.LangCode) + "\n")
	}
	if f.Category != "" {
		sb.WriteString("  AND h.category = " + arg(f.Category) + "::hit_category_enum\n")
	}
	if f.Severity != "" {
		sb.WriteString("  AND h.severity = " + arg(f.Severity) + "::hit_severity_enum\n")
	}
	if f.Version != nil {
		sb.WriteString("  AND h.detector_version = " + arg(*f.Version) + "\n")
	}

	sb.WriteString("ORDER BY u.created_at, u.id, h.span_start\nLIMIT " + arg(limit))

	rows, err := s.q.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, domain.AfterKey{}, err
	}
	defer rows.Close()

	out := make([]domain.Sample, 0, limit)
	var last domain.AfterKey
	for rows.Next() {
		var srow domain.Sample
		if err := rows.Scan(
			&srow.UtteranceID, &srow.CreatedAt, &srow.RepoName, &srow.LangCode, &srow.Source,
			&srow.Term, &srow.Category, &srow.Severity, &srow.SpanStart, &srow.SpanEnd,
		); err != nil {
			return nil, domain.AfterKey{}, err
		}
		out = append(out, srow)
		last = domain.AfterKey{CreatedAt: srow.CreatedAt, UtteranceID: srow.UtteranceID}
	}
	return out, last, rows.Err()
}

// AggByLang implements Storage
func (s *pg) AggByLang(ctx context.Context, w domain.Window, f domain.Filters) ([]domain.AggByLangRow, error) {
	var sb strings.Builder
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	sb.WriteString(`
		SELECT date_trunc('day', u.created_at) AS day, u.lang_code, COUNT(*) AS hits, h.detector_version
		FROM hits h
		JOIN utterances u ON u.id = h.utterance_id
		WHERE u.created_at >= ` + arg(w.Since) + ` AND u.created_at < ` + arg(w.Until) + `
			AND NOT EXISTS (SELECT 1 FROM active_deny_repos r WHERE r.principal_hid = u.repo_hid)
			AND NOT EXISTS (SELECT 1 FROM active_deny_actors a WHERE a.principal_hid = u.actor_hid)
	`)
	if f.LangCode != "" {
		sb.WriteString("  AND u.lang_code = " + arg(f.LangCode) + "\n")
	}
	if f.Version != nil {
		sb.WriteString("  AND h.detector_version = " + arg(*f.Version) + "\n")
	}
	sb.WriteString("GROUP BY day, u.lang_code, h.detector_version ORDER BY day ASC")

	rows, err := s.q.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AggByLangRow
	for rows.Next() {
		var r domain.AggByLangRow
		if err := rows.Scan(&r.Day, &r.LangCode, &r.Hits, &r.DetectorVersion); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AggByRepo implements Storage
func (s *pg) AggByRepo(
	ctx context.Context,
	w domain.Window,
	f domain.Filters,
	limit int,
) ([]domain.AggByRepoRow, error) {
	var sb strings.Builder
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	sb.WriteString(`
		SELECT u.repo_name, COUNT(*) AS hits
		FROM hits h
		JOIN utterances u ON u.id = h.utterance_id
		WHERE u.created_at >= ` + arg(w.Since) + ` AND u.created_at < ` + arg(w.Until) + `
			AND NOT EXISTS (SELECT 1 FROM active_deny_repos r WHERE r.principal_hid = u.repo_hid)
			AND NOT EXISTS (SELECT 1 FROM active_deny_actors a WHERE a.principal_hid = u.actor_hid)
	`)
	if f.Owner != "" {
		sb.WriteString("  AND split_part(u.repo_name, '/', 1) = " + arg(f.Owner) + "\n")
	}
	if f.Version != nil {
		sb.WriteString("  AND h.detector_version = " + arg(*f.Version) + "\n")
	}
	sb.WriteString("GROUP BY u.repo_name ORDER BY hits DESC, u.repo_name ASC LIMIT " + arg(limit))

	rows, err := s.q.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AggByRepoRow
	for rows.Next() {
		var r domain.AggByRepoRow
		if err := rows.Scan(&r.RepoName, &r.Hits); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AggByCategory implements Storage
func (s *pg) AggByCategory(ctx context.Context, w domain.Window, f domain.Filters) ([]domain.AggByCategoryRow, error) {
	var sb strings.Builder
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	sb.WriteString(`
	SELECT h.category::text, h.severity::text, COUNT(*) AS hits
	FROM hits h
	JOIN utterances u ON u.id = h.utterance_id
	WHERE u.created_at >= ` + arg(w.Since) + ` AND u.created_at < ` + arg(w.Until) + `
		AND NOT EXISTS (SELECT 1 FROM active_deny_repos r WHERE r.principal_hid = u.repo_hid)
		AND NOT EXISTS (SELECT 1 FROM active_deny_actors a WHERE a.principal_hid = u.actor_hid)
	`)
	if f.Category != "" {
		sb.WriteString("  AND h.category = " + arg(f.Category) + "::hit_category_enum\n")
	}
	if f.Version != nil {
		sb.WriteString("  AND h.detector_version = " + arg(*f.Version) + "\n")
	}
	sb.WriteString("GROUP BY h.category, h.severity ORDER BY h.category, h.severity")

	rows, err := s.q.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AggByCategoryRow
	for rows.Next() {
		var r domain.AggByCategoryRow
		if err := rows.Scan(&r.Category, &r.Severity, &r.Hits); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
