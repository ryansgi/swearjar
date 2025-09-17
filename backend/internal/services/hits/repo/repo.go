// Package repo provides the ClickHouse implementation for the hits service
package repo

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"strings"

	"swearjar/internal/platform/store"
	dom "swearjar/internal/services/hits/domain"
)

// CH implements the hits repo with ClickHouse
type CH struct {
	ch store.Clickhouse
}

// NewCH constructs a new hits repo with a required CH instance
func NewCH(ch store.Clickhouse) *CH { return &CH{ch: ch} }

// WriteBatch inserts hits into swearjar.hits using a column list.
// If LangCode is empty for any hit, we hydrate it from swearjar.utterances(id)
// so that hits.lang_code mirrors the language computed at utterance insert
func (r *CH) WriteBatch(ctx context.Context, xs []dom.HitWrite) error {
	if len(xs) == 0 {
		return nil
	}

	// Collect utterance_ids that need lang hydration
	missing := make([]string, 0, len(xs))
	for _, h := range xs {
		if strings.TrimSpace(h.LangCode) == "" {
			missing = append(missing, h.UtteranceID)
		}
	}

	// Batch-lookup lang_code from utterances for the missing set
	if len(missing) > 0 {
		langByUtt, err := r.lookupLangByUtterance(ctx, missing)
		if err != nil {
			return err
		}
		for i := range xs {
			if strings.TrimSpace(xs[i].LangCode) == "" {
				if lc, ok := langByUtt[xs[i].UtteranceID]; ok && strings.TrimSpace(lc) != "" {
					xs[i].LangCode = lc
				}
			}
		}
	}

	const table = "swearjar.hits (" +
		"id, utterance_id, created_at, source, repo_hid, actor_hid, " +
		"lang_code, term, category, severity, " +
		"ctx_action, target_type, target_id, target_name, target_span_start, target_span_end, target_distance, " +
		"span_start, span_end, " +
		"detector_version, detector_source, pre_context, post_context, zones, " +
		"ingest_batch_id, ver" +
		")"

	batchID := batchID64(xs)

	rows := make([][]any, 0, len(xs))
	for _, h := range xs {
		// lang_code -> Nullable(String)
		var lang any
		if strings.TrimSpace(h.LangCode) == "" {
			lang = nil
		} else {
			lang = h.LangCode
		}

		// detector_source -> Enum8 label
		dsrc := h.DetectorSource
		if dsrc == "" {
			dsrc = "lemma"
		}

		// zones -> Array(String)
		zones := h.Zones
		if zones == nil {
			zones = []string{}
		}

		// context targeting fields
		ctxAction := h.CtxAction
		if ctxAction == "" {
			ctxAction = "none"
		}
		tType := h.TargetType
		if tType == "" {
			tType = "none"
		}

		tID := h.TargetID // LowCardinality(String) non-null; empty "" is OK
		var tName any
		if h.TargetName == nil || strings.TrimSpace(*h.TargetName) == "" {
			tName = nil
		} else {
			tName = *h.TargetName
		}

		var tStart any
		if h.TargetSpanStart != nil {
			tStart = int32(*h.TargetSpanStart)
		} else {
			tStart = nil
		}
		var tEnd any
		if h.TargetSpanEnd != nil {
			tEnd = int32(*h.TargetSpanEnd)
		} else {
			tEnd = nil
		}
		var tDist any
		if h.TargetDistance != nil {
			tDist = int32(*h.TargetDistance)
		} else {
			tDist = nil
		}

		rows = append(rows, []any{
			h.DeterministicUUID().String(), // id
			h.UtteranceID,                  // utterance_id
			h.CreatedAt.UTC(),              // created_at
			h.Source,                       // source (Enum8 label)
			[]byte(h.RepoHID),              // repo_hid
			[]byte(h.ActorHID),             // actor_hid
			lang,                           // lang_code (Nullable)

			h.Term,     // term
			h.Category, // category (Enum8 label)
			h.Severity, // severity (Enum8 label)

			ctxAction, // ctx_action (Enum8 label)
			tType,     // target_type (Enum8 label)
			tID,       // target_id (LC(String))
			tName,     // target_name (Nullable)
			tStart,    // target_span_start (Nullable(Int32))
			tEnd,      // target_span_end   (Nullable(Int32))
			tDist,     // target_distance   (Nullable(Int32))

			h.SpanStart,       // span_start
			h.SpanEnd,         // span_end
			h.DetectorVersion, // detector_version
			dsrc,              // detector_source
			h.PreContext,      // pre_context
			h.PostContext,     // post_context
			zones,             // zones (Array(String))
			batchID,           // ingest_batch_id
			batchID,           // ver
		})
	}

	return r.ch.Insert(ctx, table, rows)
}

// lookupLangByUtterance returns a map[utterance_id]string for a set of IDs.
// It chunks the IN-list to avoid oversized query params on large batches
func (r *CH) lookupLangByUtterance(ctx context.Context, uttIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(uttIDs))
	if len(uttIDs) == 0 {
		return out, nil
	}

	const chunkSize = 1000
	for i := 0; i < len(uttIDs); i += chunkSize {
		j := i + chunkSize
		if j > len(uttIDs) {
			j = len(uttIDs)
		}
		chunk := uttIDs[i:j]

		// Build "(toUUID(?), toUUID(?), ...)" with args = chunk ids
		ph := make([]string, len(chunk))
		args := make([]any, 0, len(chunk))
		for k, id := range chunk {
			ph[k] = "toUUID(?)"
			args = append(args, id)
		}
		q := `
			SELECT toString(id) AS utterance_id, lang_code
			FROM swearjar.utterances
			WHERE id IN (` + strings.Join(ph, ",") + `)
		`

		rows, err := r.ch.Query(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var lang *string // Nullable(String)
			if err := rows.Scan(&id, &lang); err != nil {
				rows.Close()
				return nil, err
			}
			if lang != nil && strings.TrimSpace(*lang) != "" {
				out[id] = *lang
			}
		}
		rows.Close()
	}
	return out, nil
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

func batchID64(xs []dom.HitWrite) uint64 {
	const N = 32
	h := fnv.New64a()
	n := min(len(xs), N)
	var buf [8]byte
	for i := 0; i < n; i++ {
		x := xs[i]
		_, _ = h.Write([]byte(x.UtteranceID))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(x.Term))
		_, _ = h.Write([]byte{0})

		binary.LittleEndian.PutUint32(buf[0:], uint32(x.SpanStart))
		binary.LittleEndian.PutUint32(buf[4:], uint32(x.SpanEnd))
		_, _ = h.Write(buf[:])

		// Category & Severity are strings; include full labels
		if x.Category != "" {
			_, _ = h.Write([]byte(x.Category))
		}
		_, _ = h.Write([]byte{0})
		if x.Severity != "" {
			_, _ = h.Write([]byte(x.Severity))
		}
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}
