// Package repo provides storage binders (PG + CH) for backfill
package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/platform/store"
	"swearjar/internal/services/backfill/domain"
	identdom "swearjar/internal/services/ident/domain"
)

// NewHybrid returns a binder that uses:
//   - Postgres (via the provided Queryer) for hour accounting (ingest_hours)
//   - ClickHouse (provided here) for facts (utterances) and lookups
func NewHybrid(ch store.Clickhouse) repokit.Binder[domain.StorageRepo] {
	return &hybridBinder{ch: ch}
}

type hybridBinder struct{ ch store.Clickhouse }

func (b *hybridBinder) Bind(q repokit.Queryer) domain.StorageRepo {
	return &hybridStore{pg: q, ch: b.ch}
}

// hybridStore uses PG for ingest_hours and CH for utterances facts/lookups
type hybridStore struct {
	pg repokit.Queryer  // Postgres: ingest_hours
	ch store.Clickhouse // ClickHouse: utterances (facts) + lookups
}

func (s *hybridStore) StartHour(ctx context.Context, hour time.Time) error {
	_, err := s.pg.Exec(ctx, `
		INSERT INTO ingest_hours (hour_utc, started_at, status)
		VALUES ($1, now(), 'running')
		ON CONFLICT (hour_utc) DO UPDATE
		SET started_at = now(), status = 'running', error = null, finished_at = null
	`, hour.UTC())
	return err
}

func (s *hybridStore) FinishHour(ctx context.Context, hour time.Time, fin domain.HourFinish) error {
	_, err := s.pg.Exec(ctx, `
		UPDATE ingest_hours SET
			finished_at = now(),
			status = $2,
			cache_hit = $3,
			bytes_uncompressed = $4,
			events_scanned = $5,
			utterances_extracted = $6,
			inserted = $7,
			deduped = $8,
			fetch_ms = $9,
			read_ms = $10,
			db_ms = $11,
			elapsed_ms = $12,
			error = NULLIF($13,'')
		WHERE hour_utc = $1
	`,
		hour.UTC(), fin.Status, fin.CacheHit, fin.BytesUncompressed, fin.Events, fin.Utterances,
		fin.Inserted, fin.Deduped, fin.FetchMS, fin.ReadMS, fin.DBMS, fin.ElapsedMS, fin.ErrText,
	)
	return err
}

// InsertUtterances writes a batch into ClickHouse.
// NOTE: consent gating should happen before this call
func (s *hybridStore) InsertUtterances(ctx context.Context, us []domain.Utterance) (int, int, error) {
	if len(us) == 0 {
		return 0, 0, nil
	}

	// Assign ordinals per (event_id, source)
	type k struct{ ev, src string }
	counts := map[k]int{}
	batch := make([]domain.Utterance, 0, len(us))
	for _, u := range us {
		if u.RepoID == 0 || u.ActorID == 0 {
			continue
		}
		key := k{u.EventID, u.Source}
		counts[key]++
		u.Ordinal = counts[key] - 1
		batch = append(batch, u)
	}
	if len(batch) == 0 {
		return 0, 0, nil
	}

	// Insert column list (omit `id` so CH uses DEFAULT generateUUIDv7())
	const tableWithCols = "swearjar.utterances (" +
		"event_id, event_type, repo_hid, actor_hid, hid_key_version," +
		"created_at, source, source_detail, ordinal, text_raw, text_normalized," +
		"ingest_batch_id, ver" +
		")"

	rows := make([][]any, 0, len(batch))
	for _, u := range batch {
		repoRaw := []byte(identdom.RepoHID32(u.RepoID).Bytes())    // []byte, len=32
		actorRaw := []byte(identdom.ActorHID32(u.ActorID).Bytes()) // []byte, len=32

		// Nullable normalized text
		var norm any
		if n := strings.TrimSpace(u.TextNormalized); n == "" {
			norm = nil
		} else {
			norm = n
		}

		row := []any{
			u.EventID,              // event_id (String)
			u.EventType,            // event_type (String/Enum)
			repoRaw,                // repo_hid (FixedString(32))
			actorRaw,               // actor_hid (FixedString(32))
			1,                      // hid_key_version
			u.CreatedAt.UTC(),      // created_at (DateTime64(3))
			coerceSource(u.Source), // source (Enum8) - string ok
			zeroIfEmpty(u.SourceDetail, u.Source),
			int32(u.Ordinal), // ordinal (Int32)
			u.TextRaw,        // text_raw
			norm,             // text_normalized (Nullable(String))
			0,                // ingest_batch_id
			0,                // ver
		}
		rows = append(rows, row)
	}

	if err := s.ch.Insert(ctx, tableWithCols, rows); err != nil {
		return 0, 0, err
	}
	return len(rows), 0, nil
}

// LookupIDs resolves (event_id, source, ordinal) -> (id, lang_code) from CH
func (s *hybridStore) LookupIDs(ctx context.Context, keys []domain.UKey) (map[domain.UKey]domain.LookupRow, error) {
	out := make(map[domain.UKey]domain.LookupRow, len(keys))
	if len(keys) == 0 {
		return out, nil
	}

	var sb strings.Builder
	sb.WriteString(`
	  SELECT event_id, toString(source) AS source, ordinal,
	         any(id) AS id, any(lang_code) AS lang_code
	  FROM swearjar.utterances
	  WHERE (event_id, toString(source), ordinal) IN (`)
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("('%s','%s',%d)", esc(k.EventID), esc(coerceSource(k.Source)), k.Ordinal))
	}
	sb.WriteString(") GROUP BY event_id, source, ordinal")

	rows, err := s.ch.Query(ctx, sb.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ev, src, id string
		var ord32 int32
		var lang *string
		if err := rows.Scan(&ev, &src, &ord32, &id, &lang); err != nil {
			return nil, err
		}
		out[domain.UKey{EventID: ev, Source: src, Ordinal: int(ord32)}] = domain.LookupRow{
			ID: id, LangCode: lang,
		}
	}
	return out, rows.Err()
}

func esc(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

// func escNullable(s *string) (string, bool) {
// 	if s == nil {
// 		return "", true
// 	}
// 	return esc(*s), false
// }

func zeroIfEmpty(v, fb string) string {
	if v == "" {
		return fb
	}
	return v
}

func coerceSource(s string) string {
	ls := strings.ToLower(s)
	switch {
	case strings.HasPrefix(ls, "push:"):
		return "commit"
	case strings.HasPrefix(ls, "issues:"):
		return "issue"
	case strings.HasPrefix(ls, "pr:"):
		return "pr"
	}
	if strings.Contains(ls, "comment:") {
		return "comment"
	}
	return ls
}
