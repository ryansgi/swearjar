// Package repo provides postgres access for backfill writes
package repo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/backfill/domain"
)

type (
	// PG is a binder for Postgres implementation of domain.StorageRepo
	PG      struct{}
	queries struct{ q repokit.Queryer }
)

// NewPG constructs a Postgres binder for domain.StorageRepo
func NewPG() repokit.Binder[domain.StorageRepo] { return PG{} }

// Bind binds a Queryer to a Postgres implementation of domain.StorageRepo
func (PG) Bind(q repokit.Queryer) domain.StorageRepo { return &queries{q: q} }

func makeRepoHID(repoID int64) []byte {
	h := sha256.Sum256([]byte("repo:" + strconv.FormatInt(repoID, 10)))
	return h[:] // 32 bytes
}

func makeActorHID(actorID int64) []byte {
	h := sha256.Sum256([]byte("actor:" + strconv.FormatInt(actorID, 10)))
	return h[:]
}

// StartHour marks the start of processing for the given hour, creating or updating the ingest_hours record
func (r *queries) StartHour(ctx context.Context, hour time.Time) error {
	_, err := r.q.Exec(ctx, `
		INSERT INTO ingest_hours (hour_utc, started_at, status)
		VALUES ($1, now(), 'running')
		ON CONFLICT (hour_utc) DO UPDATE
		SET started_at = now(), status = 'running', error = null, finished_at = null
	`, hour.UTC())
	return err
}

// FinishHour marks the given hour as finished, updating stats and error info
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

// InsertUtterances inserts the given utterances, returning counts of inserted and deduped rows
func (r *queries) InsertUtterances(ctx context.Context, us []domain.Utterance) (int, int, error) {
	const sql = `
        INSERT INTO utterances (
            event_id, event_type,
            repo_name, repo_id, repo_hid,
            actor_login, actor_id, actor_hid, hid_key_version,
            created_at,
            source, source_detail, ordinal,
            text_raw, text_normalized, lang_code, script
        ) VALUES (
            $1, $2,
            $3, $4, $5,
            $6, $7, $8, $9,
            $10,
            $11::source_enum, $12, $13,
            $14, $15, $16, $17
        )
        ON CONFLICT (event_id, source, ordinal) DO NOTHING;
    `

	attempts, inserted := 0, 0
	type key struct{ event, source string }
	counts := map[key]int{}

	for _, u := range us {
		// guard against missing IDs: skip rows that would violate NOT NULL
		if u.RepoID == 0 || u.ActorID == 0 {
			continue
		}
		k := key{u.EventID, u.Source}
		counts[k]++
		ord := counts[k] - 1
		attempts++

		var lang any
		if u.LangCode != "" {
			lang = u.LangCode
		}
		var script any
		if u.Script != "" {
			script = u.Script
		}

		repoHID := makeRepoHID(u.RepoID)
		actorHID := makeActorHID(u.ActorID)
		hidVer := int16(1)

		tag, err := r.q.Exec(ctx, sql,
			u.EventID, u.EventType,
			u.Repo, u.RepoID, repoHID,
			u.Actor, u.ActorID, actorHID, hidVer,
			u.CreatedAt,
			u.Source, u.SourceDetail, ord,
			u.TextRaw, u.TextNormalized, lang, script,
		)
		if err != nil {
			return inserted, attempts - inserted, fmt.Errorf("insert utterance %s/%s[%d]: %w", u.EventID, u.Source, ord, err)
		}
		if tag.RowsAffected() > 0 {
			inserted++
		}
	}
	return inserted, attempts - inserted, nil
}
