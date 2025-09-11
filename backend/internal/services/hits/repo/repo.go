// Package repo provides the ClickHouse implementation for the hits service
package repo

import (
	"context"

	"swearjar/internal/platform/store"
	dom "swearjar/internal/services/hits/domain"
)

// CH implements the hits repo with ClickHouse
type CH struct {
	ch store.Clickhouse
}

// NewCH constructs a new hits repo with a required CH instance
func NewCH(ch store.Clickhouse) *CH { return &CH{ch: ch} }

// WriteBatch inserts hits into swearjar.hits using a column list
func (r *CH) WriteBatch(ctx context.Context, xs []dom.HitWrite) error {
	if len(xs) == 0 {
		return nil
	}

	const table = "swearjar.hits (" +
		"utterance_id, created_at, source, repo_hid, actor_hid, " +
		"lang_code, term, category, severity, span_start, span_end, detector_version, " +
		"ingest_batch_id, ver" +
		")"

	rows := make([][]any, 0, len(xs))
	for _, h := range xs {
		var lang any
		if h.LangCode == "" {
			lang = nil
		} else {
			lang = h.LangCode
		}

		rows = append(rows, []any{
			h.UtteranceID,      // UUID (String OK)
			h.CreatedAt.UTC(),  // DateTime64(3)
			h.Source,           // Enum8 string
			[]byte(h.RepoHID),  // FixedString(32)
			[]byte(h.ActorHID), // FixedString(32)
			lang,               // Nullable(String)
			h.Term,             // String
			h.Category,         // Enum8 string
			h.Severity,         // Enum8 string
			h.SpanStart,        // Int32
			h.SpanEnd,          // Int32
			h.DetectorVersion,  // Int32
			0,                  // ingest_batch_id
			0,                  // ver (ReplacingMergeTree)
		})
	}

	return r.ch.Insert(ctx, table, rows)
}

// ListSamples returns hits joined with utterances with keyset pagination.
// Keyset: (u.created_at, u.id) > (after.CreatedAt, toUUID(after.UtteranceID))
func (r *CH) ListSamples(
	ctx context.Context,
	w dom.Window,
	f dom.Filters,
	after dom.AfterKey,
	limit int,
) ([]dom.Sample, dom.AfterKey, error) {
	// Build WHERE with optional filters
	q := `
	SELECT
	  toString(h.utterance_id)  AS utterance_id,
	  u.created_at,
	  u.repo_name,
	  u.lang_code,
	  u.source,
	  h.term, h.category, h.severity, h.span_start, h.span_end
	FROM swearjar.hits AS h
	INNER JOIN swearjar.utterances AS u ON u.id = h.utterance_id
	WHERE u.created_at >= ? AND u.created_at < ?
	  AND ((u.created_at > ?) OR (u.created_at = ? AND h.utterance_id > toUUID(?)))
	`

	args := []any{
		w.Since.UTC(), w.Until.UTC(),
		after.CreatedAt.UTC(), after.CreatedAt.UTC(),
		coalesce(after.UtteranceID, "00000000-0000-0000-0000-000000000000"),
	}

	// Filters
	if f.RepoName != "" {
		q += "  AND u.repo_name = ?\n"
		args = append(args, f.RepoName)
	}
	if f.Owner != "" {
		// split path 'owner/repo', CH arrays are 1-based
		q += "  AND splitByChar('/', u.repo_name)[1] = ?\n"
		args = append(args, f.Owner)
	}
	if f.LangCode != "" {
		q += "  AND u.lang_code = ?\n"
		args = append(args, f.LangCode)
	}
	if f.Category != "" {
		q += "  AND h.category = ?\n"
		args = append(args, f.Category)
	}
	if f.Severity != "" {
		q += "  AND h.severity = ?\n"
		args = append(args, f.Severity)
	}
	if f.Version != nil {
		q += "  AND h.detector_version = ?\n"
		args = append(args, *f.Version)
	}

	q += "ORDER BY u.created_at, h.utterance_id, h.span_start\nLIMIT ?"
	args = append(args, limit)

	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, dom.AfterKey{}, err
	}
	defer rows.Close()

	out := make([]dom.Sample, 0, limit)
	var last dom.AfterKey
	for rows.Next() {
		var s dom.Sample
		var spanStart, spanEnd int32

		if err := rows.Scan(
			&s.UtteranceID,
			&s.CreatedAt,
			&s.RepoName,
			&s.LangCode,
			&s.Source,
			&s.Term,
			&s.Category,
			&s.Severity,
			&spanStart,
			&spanEnd,
		); err != nil {
			return nil, dom.AfterKey{}, err
		}

		s.SpanStart = int(spanStart)
		s.SpanEnd = int(spanEnd)

		out = append(out, s)
		last = dom.AfterKey{CreatedAt: s.CreatedAt, UtteranceID: s.UtteranceID}
	}
	return out, last, rows.Err()
}

// AggByLang implements domain.QueryPort
func (r *CH) AggByLang(ctx context.Context, w dom.Window, f dom.Filters) ([]dom.AggByLangRow, error) {
	q := `
	SELECT
	  toStartOfDay(u.created_at) AS day,
	  u.lang_code,
	  count() AS hits,
	  h.detector_version
	FROM swearjar.hits AS h
	INNER JOIN swearjar.utterances AS u ON u.id = h.utterance_id
	WHERE u.created_at >= ? AND u.created_at < ?
	`
	args := []any{w.Since.UTC(), w.Until.UTC()}

	if f.LangCode != "" {
		q += "  AND u.lang_code = ?\n"
		args = append(args, f.LangCode)
	}
	if f.Version != nil {
		q += "  AND h.detector_version = ?\n"
		args = append(args, *f.Version)
	}
	q += "GROUP BY day, u.lang_code, h.detector_version\nORDER BY day ASC"

	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dom.AggByLangRow
	for rows.Next() {
		var rrow dom.AggByLangRow
		var detVer int32

		if err := rows.Scan(
			&rrow.Day,
			&rrow.LangCode,
			&rrow.Hits,
			&detVer,
		); err != nil {
			return nil, err
		}
		rrow.DetectorVersion = int(detVer)
		out = append(out, rrow)
	}
	return out, rows.Err()
}

// AggByRepo implements domain.QueryPort
func (r *CH) AggByRepo(
	ctx context.Context,
	w dom.Window,
	f dom.Filters,
	limit int,
) ([]dom.AggByRepoRow, error) {
	q := `
	SELECT u.repo_name, count() AS hits
	FROM swearjar.hits AS h
	INNER JOIN swearjar.utterances AS u ON u.id = h.utterance_id
	WHERE u.created_at >= ? AND u.created_at < ?
	`
	args := []any{w.Since.UTC(), w.Until.UTC()}
	if f.Owner != "" {
		q += "  AND splitByChar('/', u.repo_name)[1] = ?\n"
		args = append(args, f.Owner)
	}
	if f.Version != nil {
		q += "  AND h.detector_version = ?\n"
		args = append(args, *f.Version)
	}
	q += "GROUP BY u.repo_name\nORDER BY hits DESC, u.repo_name ASC\nLIMIT ?"
	args = append(args, limit)

	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dom.AggByRepoRow
	for rows.Next() {
		var rrow dom.AggByRepoRow
		if err := rows.Scan(&rrow.RepoName, &rrow.Hits); err != nil {
			return nil, err
		}
		out = append(out, rrow)
	}
	return out, rows.Err()
}

// AggByCategory implements domain.QueryPort
func (r *CH) AggByCategory(ctx context.Context, w dom.Window, f dom.Filters) ([]dom.AggByCategoryRow, error) {
	q := `
	SELECT h.category, h.severity, count() AS hits
	FROM swearjar.hits AS h
	INNER JOIN swearjar.utterances AS u ON u.id = h.utterance_id
	WHERE u.created_at >= ? AND u.created_at < ?
	`
	args := []any{w.Since.UTC(), w.Until.UTC()}
	if f.Category != "" {
		q += "  AND h.category = ?\n"
		args = append(args, f.Category)
	}
	if f.Version != nil {
		q += "  AND h.detector_version = ?\n"
		args = append(args, *f.Version)
	}
	q += "GROUP BY h.category, h.severity\nORDER BY h.category, h.severity"

	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dom.AggByCategoryRow
	for rows.Next() {
		var rrow dom.AggByCategoryRow
		if err := rows.Scan(&rrow.Category, &rrow.Severity, &rrow.Hits); err != nil {
			return nil, err
		}
		out = append(out, rrow)
	}
	return out, rows.Err()
}

// helpers
func coalesce(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
