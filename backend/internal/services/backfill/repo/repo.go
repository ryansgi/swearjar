// Package repo provides postgres access for backfill writes
package repo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/backfill/domain"
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

// HIDs (one-way; privacy-first)
func makeRepoHID(repoID int64) []byte {
	h := sha256.Sum256([]byte("repo:" + strconv.FormatInt(repoID, 10)))
	return h[:] // 32 bytes
}

func makeActorHID(actorID int64) []byte {
	h := sha256.Sum256([]byte("actor:" + strconv.FormatInt(actorID, 10)))
	return h[:]
}

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

// EnsurePrincipalsAndMaps inserts missing principals and GH maps via temp-table anti-join
// The goal here is to avoid long transactions and heavy lock contention
// on the main principals_* tables. This is a best-effort operation;
// if it fails, the caller can retry the entire batch.
func (r *queries) EnsurePrincipalsAndMaps(ctx context.Context,
	repos map[[32]byte]int64, actors map[[32]byte]int64,
) error {
	if len(repos) == 0 && len(actors) == 0 {
		return nil
	}

	keysSorted := func(m map[[32]byte]int64) [][32]byte {
		hs := make([][32]byte, 0, len(m))
		for h := range m {
			hs = append(hs, h)
		}
		sort.Slice(hs, func(i, j int) bool { return bytes.Compare(hs[i][:], hs[j][:]) < 0 })
		return hs
	}
	makeHexes := func(hs [][32]byte) []string {
		xs := make([]string, len(hs))
		for i, h := range hs {
			xs[i] = fmt.Sprintf("%x", h)
		}
		return xs
	}
	makeHexesAndIDs := func(m map[[32]byte]int64, hs [][32]byte) ([]string, []int64) {
		hexes := make([]string, len(hs))
		ids := make([]int64, len(hs))
		for i, h := range hs {
			hexes[i], ids[i] = fmt.Sprintf("%x", h), m[h]
		}
		return hexes, ids
	}

	// Prior versions used ON CONFLICT path; now it removes a ton of per-row unique checks & hot-leaf thrash

	// Principals: repos
	if len(repos) > 0 {
		hs := keysSorted(repos)
		hexes := makeHexes(hs)

		// stage and anti-join -> principals_repos (NO ON CONFLICT path)
		if _, err := r.q.Exec(ctx, `
			CREATE TEMP TABLE IF NOT EXISTS _hid_repo(stage_hid hid_bytes) ON COMMIT DROP;
			TRUNCATE _hid_repo;
		`); err != nil {
			return fmt.Errorf("stage repos: create temp: %w", err)
		}
		if _, err := r.q.Exec(ctx, `
			INSERT INTO _hid_repo(stage_hid) SELECT decode(x,'hex')::hid_bytes FROM unnest($1::text[]) AS t(x);
		`, hexes); err != nil {
			return fmt.Errorf("stage repos: load: %w", err)
		}
		if _, err := r.q.Exec(ctx, `
			INSERT INTO principals_repos (repo_hid)
			SELECT s.stage_hid FROM _hid_repo s
			LEFT JOIN principals_repos p ON p.repo_hid = s.stage_hid
			WHERE p.repo_hid IS NULL;
		`); err != nil {
			return fmt.Errorf("ensure principals_repos: %w", err)
		}

		// ident map (bulk shim; runs as SECURITY DEFINER)
		hexes, ids := makeHexesAndIDs(repos, hs)
		if _, err := r.q.Exec(ctx,
			`SELECT ident.bulk_upsert_gh_repo_map($1::text[], $2::bigint[])`,
			hexes, ids,
		); err != nil {
			return fmt.Errorf("repo map bulk upsert: %w", err)
		}
	}

	// Principals: actors
	if len(actors) > 0 {
		hs := keysSorted(actors)
		hexes := makeHexes(hs)

		if _, err := r.q.Exec(ctx, `
			CREATE TEMP TABLE IF NOT EXISTS _hid_actor(stage_hid hid_bytes) ON COMMIT DROP;
			TRUNCATE _hid_actor;
		`); err != nil {
			return fmt.Errorf("stage actors: create temp: %w", err)
		}
		if _, err := r.q.Exec(ctx, `
			INSERT INTO _hid_actor(stage_hid) SELECT decode(x,'hex')::hid_bytes FROM unnest($1::text[]) AS t(x);
		`, hexes); err != nil {
			return fmt.Errorf("stage actors: load: %w", err)
		}
		if _, err := r.q.Exec(ctx, `
			INSERT INTO principals_actors (actor_hid)
			SELECT s.stage_hid FROM _hid_actor s LEFT JOIN principals_actors p ON p.actor_hid = s.stage_hid
			WHERE p.actor_hid IS NULL;
		`); err != nil {
			return fmt.Errorf("ensure principals_actors: %w", err)
		}

		hexes, ids := makeHexesAndIDs(actors, hs)
		if _, err := r.q.Exec(ctx,
			`SELECT ident.bulk_upsert_gh_actor_map($1::text[], $2::bigint[])`,
			hexes, ids,
		); err != nil {
			return fmt.Errorf("actor map bulk upsert: %w", err)
		}
	}

	return nil
}

func (r *queries) InsertUtterances(ctx context.Context, us []domain.Utterance) (int, int, error) {
	const insertUttSQL = `
        INSERT INTO utterances (
            event_id, event_type,
            repo_hid, actor_hid, hid_key_version,
            created_at,
            source, source_detail, ordinal,
            text_raw, text_normalized
        ) VALUES (
            $1, $2,
            $3, $4, $5,
            $6,
            $7::source_enum, $8, $9,
            $10, $11
        )
        ON CONFLICT (event_id, source, ordinal) DO NOTHING;`

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
		repoHID := makeRepoHID(u.RepoID)
		actorHID := makeActorHID(u.ActorID)
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
