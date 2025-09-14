// Package repo provides the Nightshift storage repository implementation
package repo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/platform/store"
	nsdom "swearjar/internal/services/nightshift/domain"
	"swearjar/internal/services/nightshift/guardrails"
)

// NewHybrid returns a binder that uses
// - Postgres for ingest_hours coordination/state
// - ClickHouse for archives/rollups/pruning
func NewHybrid(ch store.Clickhouse) repokit.Binder[nsdom.StorageRepo] {
	return &hybridBinder{ch: ch}
}

type hybridBinder struct{ ch store.Clickhouse }

func (b *hybridBinder) Bind(q repokit.Queryer) nsdom.StorageRepo {
	return &hybridStore{pg: q, ch: b.ch}
}

type hybridStore struct {
	pg repokit.Queryer
	ch store.Clickhouse
}

// Start marks Nightshift processing for an hour (separate from backfill's StartHour)
func (s *hybridStore) Start(ctx context.Context, hour time.Time) error {
	res, err := s.pg.Exec(ctx, `
	  UPDATE ingest_hours
	     SET ns_started_at = COALESCE(ns_started_at, now()), ns_status = 'running'
	   WHERE hour_utc = $1 AND ns_status IN ('pending','error')`,
		hour.UTC(),
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return guardrails.ErrLeaseHeld // sentinel = "no work/already running or done"
	}
	return nil
}

func (s *hybridStore) WriteArchives(ctx context.Context, hour time.Time, detver int) (int, error) {
	// Populate denormalized commit_crimes for the hour+detver
	start := hour.Truncate(time.Hour).UTC()
	end := start.Add(time.Hour)

	// If no raw hits exist for this hour, keep existing slice and skip
	hasHits, err := s.ch.ScalarUInt64(ctx, `
		SELECT toUInt64(count())
		FROM swearjar.hits
		WHERE created_at >= ? AND created_at < ?`,
		start, end,
	)
	if err != nil {
		return 0, err
	}
	if hasHits == 0 {
		return 0, nil
	}

	// Clear hour slice (idempotent) and block until applied so reads are consistent
	if err := s.ch.Exec(ctx, `
		ALTER TABLE swearjar.commit_crimes
		DELETE WHERE bucket_hour = toStartOfHour(?) AND detver = ?
		SETTINGS mutations_sync=1`,
		start, detver,
	); err != nil {
		return 0, err
	}

	// Insert from hits & utterances
	if err := s.ch.Exec(ctx, `
		INSERT INTO swearjar.commit_crimes
		(
		  created_at, bucket_hour, detver,
		  hit_id, utterance_id, repo_hid, actor_hid,
		  source, source_detail,
		  lang_code, lang_confidence, lang_reliable, sentiment_score, text_len,
		  term_id, term, category, severity, target,
		  span_start, span_end
		)
		SELECT
		  h.created_at,
		  toStartOfHour(h.created_at)                    AS bucket_hour,
		  ?                                              AS detver,
		  h.id                                           AS hit_id,
		  h.utterance_id                                 AS utterance_id,
		  h.repo_hid, h.actor_hid,
		  h.source,
		  u.source_detail,
		  u.lang_code, u.lang_confidence, u.lang_reliable, u.sentiment_score,
		  length(u.text_raw)                             AS text_len,
		  cityHash64(lower(h.term))                      AS term_id,
		  h.term,
		  h.category, h.severity,
		  CAST(NULL AS Nullable(String))                 AS target,
		  h.span_start, h.span_end
		FROM swearjar.hits h
		INNER JOIN swearjar.utterances u ON u.id = h.utterance_id
		WHERE h.created_at >= ? AND h.created_at < ?`,
		detver, start, end,
	); err != nil {
		return 0, err
	}

	// Count inserted rows for metrics
	rows, err := s.ch.ScalarUInt64(ctx, `
		SELECT toUInt64(count())
		FROM swearjar.commit_crimes
		WHERE bucket_hour = toStartOfHour(?) AND detver = ?`,
		start, detver,
	)
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}

// PruneRaw applies the configured retention policy to raw facts in ClickHouse.
//   - "full": no-op
//   - "timebox:<Nd>": for hours older than cutoff, delete *both* hits & utterances
//   - "aggressive": immediately delete *both* hits & utterances for this hour
//
// Returns (deletedUtterances, sparedUtterancesForHour)
// PruneRaw applies the configured retention policy to ClickHouse facts.
// Returns (deletedUtterances, sparedUtterancesForHour), error
func (s *hybridStore) PruneRaw(ctx context.Context, hour time.Time, retention string) (int, int, error) {
	start := hour.Truncate(time.Hour).UTC()
	end := start.Add(time.Hour)

	mode := strings.TrimSpace(strings.ToLower(retention))
	if mode == "" {
		mode = "full"
	}

	// Hour-scoped utterance total for metrics
	totalUtt, err := s.ch.ScalarUInt64(ctx, `
		SELECT toUInt64(count())
		FROM swearjar.utterances
		WHERE created_at >= ? AND created_at < ?`,
		start, end,
	)
	if err != nil {
		return 0, 0, err
	}

	// Helper: delete this hour from hits then utterances, synchronously
	deleteHourBoth := func() error {
		if err := s.ch.Exec(ctx, `
			ALTER TABLE swearjar.hits
			DELETE WHERE created_at >= ? AND created_at < ?
			SETTINGS mutations_sync=1`,
			start, end,
		); err != nil {
			return err
		}
		if err := s.ch.Exec(ctx, `
			ALTER TABLE swearjar.utterances
			DELETE WHERE created_at >= ? AND created_at < ?
			SETTINGS mutations_sync=1`,
			start, end,
		); err != nil {
			return err
		}
		return nil
	}

	switch {
	case mode == "full":
		// Keep everything for this hour
		return 0, int(totalUtt), nil

	case mode == "aggressive":
		// Drop both hits & utterances for this hour
		if err := deleteHourBoth(); err != nil {
			return 0, 0, err
		}
		return int(totalUtt), 0, nil

	case strings.HasPrefix(mode, "timebox:"):
		days, perr := parseTimeboxDays(mode) // e.g., "timebox:45d"
		if perr != nil || days <= 0 {
			return 0, int(totalUtt), nil
		}
		cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
		if end.Before(cutoff) {
			if err := deleteHourBoth(); err != nil {
				return 0, 0, err
			}
			return int(totalUtt), 0, nil
		}
		return 0, int(totalUtt), nil

	default:
		// Unknown mode -> no-op
		return 0, int(totalUtt), nil
	}
}

// parseTimeboxDays extracts the integer day window from "timebox:Nd"
func parseTimeboxDays(mode string) (int, error) {
	// Accept forms: timebox:45d, timebox:45, timebox: 45d
	s := strings.TrimPrefix(mode, "timebox:")
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "d")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty timebox days")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Finish marks the hour as "retention_applied" or final "done" depending on the pipeline
func (s *hybridStore) Finish(ctx context.Context, hour time.Time, fin nsdom.FinishInfo) error {
	_, err := s.pg.Exec(ctx, `
	  UPDATE ingest_hours
	     SET ns_finished_at     = COALESCE(ns_finished_at, now()),
	         ns_status          = $2,
	         ns_detver          = $3,
	         ns_hits_archived   = $4,
	         ns_deleted_raw     = $5,
	         ns_spared_raw      = $6,
	         ns_archive_ms      = $7,
	         ns_prune_ms        = $8,
	         ns_total_ms        = $9,
	         ns_error           = NULLIF($10,''),
	         ns_lease_claimed_at = NULL,
	         ns_lease_owner      = NULL,
	         ns_lease_expires_at = NULL
	   WHERE hour_utc = $1
	`, hour.UTC(),
		fin.Status, fin.DetVer, fin.HitsArchived, fin.DeletedRaw, fin.SparedRaw,
		fin.ArchiveMS, fin.PruneMS, fin.TotalMS, fin.ErrText,
	)
	return err
}

func (s *hybridStore) NextHourNeedingWork(ctx context.Context) (time.Time, bool, error) {
	// Claim the next hour that finished backfill (status='ok') and hasn't run Nightshift yet
	row := s.pg.QueryRow(ctx, `
		WITH next AS (
			SELECT hour_utc
				FROM ingest_hours
			WHERE bf_status = 'ok'
				AND ns_status IN ('pending','error')
			ORDER BY hour_utc
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE ingest_hours ih
			SET ns_status    = 'running',
					ns_started_at = COALESCE(ih.ns_started_at, now())
			FROM next
		WHERE ih.hour_utc = next.hour_utc
		RETURNING ih.hour_utc
	`)
	var hr time.Time
	if err := row.Scan(&hr); err != nil {
		// No rows -> nothing to do
		if strings.Contains(err.Error(), "no rows") {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return hr.UTC(), true, nil
}
