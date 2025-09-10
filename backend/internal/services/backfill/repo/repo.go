// Package repo provides postgres access for backfill writes
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/backfill/domain"
	identdom "swearjar/internal/services/ident/domain"
)

type (
	// PG is a Postgres binder for domain.StorageRepo
	PG      struct{}
	queries struct{ q repokit.Queryer }
)

// NewPG returns a Postgres binder for domain.StorageRepo
func NewPG() repokit.Binder[domain.StorageRepo] { return PG{} }

// Bind implements repokit.Binder
func (PG) Bind(q repokit.Queryer) domain.StorageRepo { return &queries{q: q} }

// StartHour marks the start of a backfill hour (idempotent)
func (r *queries) StartHour(ctx context.Context, hour time.Time) error {
	_, err := r.q.Exec(ctx, `
		INSERT INTO ingest_hours (hour_utc, started_at, status)
		VALUES ($1, now(), 'running')
		ON CONFLICT (hour_utc) DO UPDATE
		SET started_at = now(), status = 'running', error = null, finished_at = null
	`, hour.UTC())
	return err
}

// FinishHour marks the end of a backfill hour (idempotent)
func (r *queries) FinishHour(ctx context.Context, hour time.Time, fin domain.HourFinish) error {
	_, err := r.q.Exec(ctx, `
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

func (r *queries) InsertUtterances(ctx context.Context, us []domain.Utterance) (int, int, error) {
	const insertUttSQL = `
		INSERT INTO utterances (
			event_id, event_type, repo_hid, actor_hid, hid_key_version,
			created_at, source, source_detail, ordinal, text_raw, text_normalized
		)
		SELECT
			$1, $2, $3::hid_bytes, $4::hid_bytes, $5,
			$6, $7::source_enum, $8, $9, $10, $11
		WHERE
			NOT EXISTS (
				SELECT 1 FROM consent_receipts r
				WHERE r.principal='repo' AND r.action='opt_out' AND r.state='active' AND r.principal_hid=$3::hid_bytes
			)
			AND NOT EXISTS (
				SELECT 1 FROM consent_receipts r
				WHERE r.principal='actor' AND r.action='opt_out' AND r.state='active' AND r.principal_hid=$4::hid_bytes
			)
		ON CONFLICT (event_id, source, ordinal) DO NOTHING
	`

	// assign ordinals deterministically per (event_id, source)
	type k struct{ event, source string }
	counts := map[k]int{}
	var batch []domain.Utterance
	for _, u := range us {
		if u.RepoID == 0 || u.ActorID == 0 {
			continue
		}
		key := k{u.EventID, u.Source}
		counts[key]++
		u.Ordinal = counts[key] - 1
		batch = append(batch, u)
	}

	attempts, inserted := 0, 0
	for _, u := range batch {
		repoHID := identdom.RepoHID32(u.RepoID).Bytes()    // pass []byte
		actorHID := identdom.ActorHID32(u.ActorID).Bytes() // pass []byte
		attempts++
		tag, err := r.q.Exec(ctx, insertUttSQL,
			u.EventID, u.EventType,
			repoHID, actorHID, int16(1),
			u.CreatedAt,
			u.Source, u.SourceDetail, u.Ordinal,
			u.TextRaw, u.TextNormalized,
		)
		if err != nil {
			return inserted, attempts - inserted,
				fmt.Errorf("insert utterance %s/%s[%d]: %w", u.EventID, u.Source, u.Ordinal, err)
		}
		if tag.RowsAffected() > 0 {
			inserted++
		}
	}
	return inserted, attempts - inserted, nil
}

// LookupIDs resolves DB IDs (+ lang_code) for a set of (event_id, source, ordinal)
func (r *queries) LookupIDs(ctx context.Context, keys []domain.UKey) (map[domain.UKey]domain.LookupRow, error) {
	out := make(map[domain.UKey]domain.LookupRow, len(keys))
	if len(keys) == 0 {
		return out, nil
	}

	evs := make([]string, 0, len(keys))
	srcs := make([]string, 0, len(keys))
	ords := make([]int, 0, len(keys))
	for _, k := range keys {
		evs = append(evs, k.EventID)
		srcs = append(srcs, k.Source)
		ords = append(ords, k.Ordinal)
	}

	const q = `
		WITH k AS (
			SELECT * FROM UNNEST($1::text[], $2::source_enum[], $3::int[])
			AS t(event_id, source, ordinal)
		)
		SELECT u.event_id, u.source::text, u.ordinal, u.id::text, u.lang_code
		FROM utterances u
		JOIN k ON u.event_id = k.event_id AND u.source = k.source AND u.ordinal = k.ordinal
	`

	rows, err := r.q.Query(ctx, q, evs, srcs, ords)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ev, src, id string
		var ord int
		var lang sql.NullString
		if err := rows.Scan(&ev, &src, &ord, &id, &lang); err != nil {
			return nil, err
		}
		var lp *string
		if lang.Valid {
			v := lang.String
			lp = &v
		}
		out[domain.UKey{EventID: ev, Source: src, Ordinal: ord}] = domain.LookupRow{
			ID:       id,
			LangCode: lp,
		}
	}
	return out, rows.Err()
}
