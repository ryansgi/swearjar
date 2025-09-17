// Package repo provides storage binders (PG + CH) for backfill
package repo

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"sort"
	"strings"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/platform/store"
	"swearjar/internal/services/backfill/domain"
	identdom "swearjar/internal/services/ident/domain"
)

// NewHybrid returns a binder that uses:
// - Postgres (via the provided Queryer) for hour accounting (ingest_hours)
// - ClickHouse (provided here) for facts (utterances)
func NewHybrid(ch store.Clickhouse) repokit.Binder[domain.StorageRepo] {
	return &hybridBinder{ch: ch}
}

type hybridBinder struct{ ch store.Clickhouse }

func (b *hybridBinder) Bind(q repokit.Queryer) domain.StorageRepo {
	return &hybridStore{pg: q, ch: b.ch}
}

// hybridStore uses PG for ingest_hours and CH for utterances facts
type hybridStore struct {
	pg repokit.Queryer  // Postgres: ingest_hours
	ch store.Clickhouse // ClickHouse: utterances (facts)
}

func (s *hybridStore) StartHour(ctx context.Context, hour time.Time) error {
	_, err := s.pg.Exec(ctx, `
        INSERT INTO ingest_hours (hour_utc, started_at, bf_status)
        VALUES ($1, now(), 'running')
        ON CONFLICT (hour_utc) DO UPDATE
        SET started_at = now(), bf_status = 'running', error = NULL, finished_at = NULL
    `, hour.UTC())
	return err
}

func (s *hybridStore) FinishHour(ctx context.Context, hour time.Time, fin domain.HourFinish) error {
	_, err := s.pg.Exec(ctx, `
        UPDATE ingest_hours SET
            finished_at          = now(),
            bf_status            = $2,
            cache_hit            = $3,
            bytes_uncompressed   = $4,
            events_scanned       = $5,
            utterances_extracted = $6,
            inserted             = $7,
            deduped              = $8,
            fetch_ms             = $9,
            read_ms              = $10,
            db_ms                = $11,
            elapsed_ms           = $12,
            error                = NULLIF($13,'')
        WHERE hour_utc = $1
    `,
		hour.UTC(), fin.Status, fin.CacheHit, fin.BytesUncompressed, fin.Events, fin.Utterances,
		fin.Inserted, fin.Deduped, fin.FetchMS, fin.ReadMS, fin.DBMS, fin.ElapsedMS, fin.ErrText,
	)
	return err
}

// InsertUtterances writes a batch into ClickHouse.
// IDs MUST be precomputed deterministically by the caller (extractor).
// NOTE: consent gating should happen before this call
func (s *hybridStore) InsertUtterances(ctx context.Context, us []domain.Utterance) (int, int, error) {
	if len(us) == 0 {
		return 0, 0, nil
	}

	batchHour := us[0].CreatedAt.Truncate(time.Hour).UTC()
	for i := 1; i < len(us); i++ {
		h := us[i].CreatedAt.Truncate(time.Hour).UTC()
		if h.Before(batchHour) {
			batchHour = h
		}
	}
	ingestBatchID := contentStableBatchID(us, 64)

	// Prepare rows; skip incomplete records (missing repo/actor or ID)
	const tableWithCols = "swearjar.utterances (" +
		"id, event_type, repo_hid, actor_hid, hid_key_version," +
		"created_at, source, source_detail, ordinal, text_raw, text_normalized," +
		"ingest_batch_id, ver" +
		")"

	rows := make([][]any, 0, len(us))
	for _, u := range us {
		// Guard: CH facts require resolved HIDs + deterministic ID
		if u.RepoID == 0 || u.ActorID == 0 {
			continue
		}
		if strings.TrimSpace(u.UtteranceID) == "" {
			continue
		}

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
			u.UtteranceID,                         // id (UUID as string acceptable for CH UUID)
			u.EventType,                           // event_type (String)
			repoRaw,                               // repo_hid (FixedString(32))
			actorRaw,                              // actor_hid (FixedString(32))
			1,                                     // hid_key_version
			u.CreatedAt.UTC(),                     // created_at (DateTime64(3))
			coerceSource(u.Source),                // source (Enum8) - string ok
			zeroIfEmpty(u.SourceDetail, u.Source), // source_detail (String) - fallback to coarse source if empty
			int32(u.Ordinal),                      // ordinal (Int32) - already assigned by extractor
			u.TextRaw,                             // text_raw
			norm,                                  // text_normalized (Nullable(String))
			ingestBatchID,                         // ingest_batch_id
			1,                                     // looks like a mistake, but its for ReplacingMergeTree(ver)
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return 0, 0, nil
	}

	if err := s.ch.Insert(ctx, tableWithCols, rows); err != nil {
		return 0, 0, err
	}
	// ReplacingMergeTree handles idempotency; dedupe is CH-internal -> unknown here
	return len(rows), 0, nil
}

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

func (s *hybridStore) PreseedHours(ctx context.Context, startUTC, endUTC time.Time) (int, error) {
	const sql = `
        INSERT INTO ingest_hours (hour_utc, bf_status)
        SELECT h, 'pending'
        FROM generate_series($1::timestamptz, $2::timestamptz, '1 hour') AS g(h)
        ON CONFLICT (hour_utc) DO NOTHING
    `
	res, err := s.pg.Exec(ctx, sql, startUTC.UTC(), endUTC.UTC())
	if err != nil {
		return 0, err
	}
	return int(res.RowsAffected()), nil
}

// NextHourToProcess atomically claims the next pending or errored hour in the given range
// and marks it as running. Uses SELECT ... FOR UPDATE SKIP LOCKED to avoid conflicts
func (s *hybridStore) NextHourToProcess(ctx context.Context, startUTC, endUTC time.Time) (time.Time, bool, error) {
	const sql = `
        WITH next AS (
            SELECT hour_utc FROM ingest_hours
            WHERE hour_utc BETWEEN $1 AND $2 AND bf_status IN ('pending','error')
            ORDER BY hour_utc LIMIT 1 FOR UPDATE SKIP LOCKED
        )
        UPDATE ingest_hours ih
        SET bf_status = 'running', started_at = now(), error = NULL, finished_at = NULL
        FROM next WHERE ih.hour_utc = next.hour_utc
        RETURNING ih.hour_utc
    `
	row := s.pg.QueryRow(ctx, sql, startUTC.UTC(), endUTC.UTC())
	var hr time.Time
	if err := row.Scan(&hr); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return hr.UTC(), true, nil
}

func (s *hybridStore) NextHourToProcessAny(ctx context.Context) (time.Time, bool, error) {
	const sql = `
        WITH next AS (
            SELECT hour_utc
            FROM ingest_hours
            WHERE bf_status IN ('pending','error')
            ORDER BY hour_utc
            LIMIT 1
            FOR UPDATE SKIP LOCKED
        )
        UPDATE ingest_hours ih
        SET bf_status = 'running', started_at = now(), error = NULL, finished_at = NULL
        FROM next WHERE ih.hour_utc = next.hour_utc
        RETURNING ih.hour_utc
    `
	row := s.pg.QueryRow(ctx, sql)
	var hr time.Time
	if err := row.Scan(&hr); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return hr.UTC(), true, nil
}

func contentStableBatchID(us []domain.Utterance, limit int) uint64 {
	if len(us) == 0 {
		return 0
	}
	ids := make([]string, 0, len(us))
	for _, u := range us {
		if id := strings.TrimSpace(u.UtteranceID); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0
	}
	sort.Strings(ids)
	if limit <= 0 || limit > len(ids) {
		limit = len(ids)
	}
	ids = ids[:limit]

	h := sha256.New()
	for _, id := range ids {
		_, _ = io.WriteString(h, id)
		h.Write([]byte{0}) // delimiter
	}
	sum := h.Sum(nil) // 32 bytes

	// fold 256 -> 64 bits (better than truncation)
	var out uint64
	for i := 0; i < 4; i++ {
		out ^= binary.BigEndian.Uint64(sum[i*8 : (i+1)*8])
	}
	return out
}
