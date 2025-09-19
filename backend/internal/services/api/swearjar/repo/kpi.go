package repo

import (
	"context"
	"time"

	"swearjar/internal/services/api/swearjar/domain"
)

// KPIStrip computes headline KPIs for the requested window (usually a single day)
func (s *hybridStore) KPIStrip(ctx context.Context, in domain.KPIStripInput) (domain.KPIStripResp, error) {
	// Window handling (inclusive dates in UTC)
	start, err := time.Parse("2006-01-02", in.Range.Start)
	if err != nil {
		return domain.KPIStripResp{}, err
	}
	endIncl, err := time.Parse("2006-01-02", in.Range.End)
	if err != nil {
		return domain.KPIStripResp{}, err
	}
	endExcl := endIncl.Add(24 * time.Hour)

	sql := `
		WITH crimes AS (
			SELECT
				count()                                   AS hits,
				uniqCombined(12)(utterance_id)            AS off_utt,
				uniqCombined(12)(repo_hid)                AS repos,
				uniqCombined(12)(actor_hid)               AS actors
			FROM swearjar.commit_crimes
			WHERE created_at >= ? AND created_at < ?
		),
		utts AS (
			SELECT
				countMerge(cnt_state)                      AS all_utt
			FROM swearjar.utt_hour_agg
			WHERE bucket_hour >= ? AND bucket_hour < ?
		)
		SELECT
			?                                           AS day,
			c.hits                                      AS hits,
			c.off_utt                                   AS off_utt,
			c.repos                                     AS repos,
			c.actors                                    AS actors,
			u.all_utt                                   AS all_utt
		FROM crimes c
		CROSS JOIN utts u
	`
	rs, err := s.ch.Query(ctx, sql,
		start, endExcl, // crimes range
		start, endExcl, // utt range
		start.Format("2006-01-02"), // label day in response
	)
	if err != nil {
		return domain.KPIStripResp{}, err
	}
	defer rs.Close()

	var (
		day    string
		hits   uint64
		offUtt uint64
		repos  uint64
		actors uint64
		allUtt uint64
	)
	if rs.Next() {
		if err := rs.Scan(&day, &hits, &offUtt, &repos, &actors, &allUtt); err != nil {
			return domain.KPIStripResp{}, err
		}
	}
	if err := rs.Err(); err != nil {
		return domain.KPIStripResp{}, err
	}

	resp := domain.KPIStripResp{
		Day:                 day,
		Hits:                int64(hits),
		OffendingUtterances: int64(offUtt),
		Repos:               int64(repos),
		Actors:              int64(actors),
		AllUtterances:       int64(allUtt),
	}

	// Derive optional ratios
	if offUtt > 0 {
		resp.Intensity = float64(hits) / float64(offUtt)
	}
	if allUtt > 0 {
		resp.Coverage = float64(offUtt) / float64(allUtt)
		resp.Rarity = float64(hits) / float64(allUtt)
	}

	return resp, nil
}
