package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"swearjar/internal/services/api/swearjar/domain"
)

// HeatmapWeekly queries ClickHouse for day-of-week x hour activity
func (s *hybridStore) HeatmapWeekly(
	ctx context.Context,
	in domain.HeatmapWeeklyInput,
) (domain.HeatmapWeeklyResp, error) {
	tz := strings.TrimSpace(in.TZ)
	if tz == "" {
		tz = "UTC"
	}
	metric := strings.ToLower(strings.TrimSpace(in.Metric))
	switch metric {
	case "intensity", "coverage", "rarity", "counts":
	default:
		metric = "counts"
	}
	series := strings.ToLower(strings.TrimSpace(in.Series))
	switch series {
	case "hits", "offending_utterances", "all_utterances":
	default:
		series = "hits"
	}

	start, err := time.Parse("2006-01-02", in.Range.Start)
	if err != nil {
		return domain.HeatmapWeeklyResp{}, err
	}
	endIncl, err := time.Parse("2006-01-02", in.Range.End)
	if err != nil {
		return domain.HeatmapWeeklyResp{}, err
	}
	endExcl := endIncl.Add(24 * time.Hour)

	// Numerator (crimes): from swearjar.commit_crimes
	crWhere := []string{"created_at >= ? AND created_at < ?"}
	crArgs := []any{start, endExcl}

	if len(in.DetVer) > 0 {
		crWhere = append(crWhere, "detver IN ?")
		crArgs = append(crArgs, in.DetVer)
	}
	if len(in.RepoHIDs) > 0 {
		crWhere = append(crWhere, "repo_hid IN ?")
		crArgs = append(crArgs, in.RepoHIDs)
	}
	if len(in.ActorHIDs) > 0 {
		crWhere = append(crWhere, "actor_hid IN ?")
		crArgs = append(crArgs, in.ActorHIDs)
	}
	if len(in.NLLangs) > 0 {
		crWhere = append(crWhere, "lang_code IN ?")
		crArgs = append(crArgs, in.NLLangs)
	}
	if in.LangReliable != nil {
		if *in.LangReliable {
			crWhere = append(crWhere, "lang_reliable = 1")
		} else {
			crWhere = append(crWhere, "lang_reliable = 0")
		}
	}
	// NOTE: commit_crimes doesn't currently have code_lang; skip CodeLangs here

	// Denominator (all utterances): from swearjar.utt_hour_agg
	utWhere := []string{"bucket_hour >= ? AND bucket_hour < ?"}
	utArgs := []any{start, endExcl}

	// Mirror feasible filters onto utt_hour_agg (same slice)
	if len(in.RepoHIDs) > 0 {
		utWhere = append(utWhere, "repo_hid IN ?")
		utArgs = append(utArgs, in.RepoHIDs)
	}
	if len(in.ActorHIDs) > 0 {
		utWhere = append(utWhere, "actor_hid IN ?")
		utArgs = append(utArgs, in.ActorHIDs)
	}
	if len(in.NLLangs) > 0 {
		utWhere = append(utWhere, "lang_code IN ?")
		utArgs = append(utArgs, in.NLLangs)
	}
	if in.LangReliable != nil {
		if *in.LangReliable {
			utWhere = append(utWhere, "lang_reliable = 1")
		} else {
			utWhere = append(utWhere, "lang_reliable = 0")
		}
	}
	// NOTE: utt_hour_agg has no code_lang; skip CodeLangs here too

	sql := fmt.Sprintf(`
		WITH
		crimes AS (
			SELECT
				(toDayOfWeek(toTimeZone(created_at, ?)) %% 7) AS dow,  -- 0..6 (Sun=0)
				toHour(toTimeZone(created_at, ?))            AS hour, -- 0..23
				count()                                      AS hits,
				uniqCombined(12)(utterance_id)               AS off_utt
			FROM swearjar.commit_crimes
			WHERE %s
			GROUP BY dow, hour
		),
		utt AS (
			SELECT
				(toDayOfWeek(toTimeZone(bucket_hour, ?)) %% 7) AS dow,
				toHour(toTimeZone(bucket_hour, ?))            AS hour,
				countMerge(cnt_state)                         AS all_utt
			FROM swearjar.utt_hour_agg
			WHERE %s
			GROUP BY dow, hour
		)
		SELECT
			coalesce(c.dow,  u.dow)   AS dow,
			coalesce(c.hour, u.hour)  AS hour,
			ifNull(c.hits,    0)      AS hits,
			ifNull(c.off_utt, 0)      AS off_utt,
			ifNull(u.all_utt, 0)      AS all_utt
		FROM crimes c
		FULL OUTER JOIN utt u ON c.dow = u.dow AND c.hour = u.hour
		ORDER BY dow ASC, hour ASC
	`, strings.Join(crWhere, " AND "), strings.Join(utWhere, " AND "))

	args := []any{tz, tz}
	args = append(args, crArgs...)
	args = append(args, tz, tz)
	args = append(args, utArgs...)

	rs, err := s.ch.Query(ctx, sql, args...)
	if err != nil {
		return domain.HeatmapWeeklyResp{}, err
	}
	defer rs.Close()

	type row struct {
		dow    uint8
		hour   uint8
		hits   uint64
		offUtt uint64
		allUtt uint64
	}

	byKey := make(map[[2]uint8]row, 168)
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.dow, &r.hour, &r.hits, &r.offUtt, &r.allUtt); err != nil {
			return domain.HeatmapWeeklyResp{}, err
		}
		byKey[[2]uint8{r.dow, r.hour}] = r
	}
	if err := rs.Err(); err != nil {
		return domain.HeatmapWeeklyResp{}, err
	}

	// Emit dense 7x24; compute chosen metric/series
	grid := make([]domain.HeatmapCell, 0, 7*24)
	for d := range 7 {
		for h := range 24 {
			r := byKey[[2]uint8{uint8(d), uint8(h)}]

			var ratio float64
			switch metric {
			case "intensity":
				// hits per offending utterance
				if r.offUtt > 0 {
					ratio = float64(r.hits) / float64(r.offUtt)
				}
			case "coverage":
				// offending utterances / all utterances
				if r.allUtt > 0 {
					ratio = float64(r.offUtt) / float64(r.allUtt)
				}
			case "rarity":
				// hits per all utterances
				if r.allUtt > 0 {
					ratio = float64(r.hits) / float64(r.allUtt)
				}
			default: // "counts"
				// Ratio unused; series decides Z later
			}

			cell := domain.HeatmapCell{
				DOW:                 d,
				Hour:                h,
				Hits:                int64(r.hits),
				OffendingUtterances: int64(r.offUtt),
				Utterances:          int64(r.allUtt),
				Ratio:               ratio,
			}
			grid = append(grid, cell)
		}
	}

	// Decide Z (what the frontend should color by)
	var z string
	if metric == "counts" {
		switch series {
		case "offending_utterances":
			z = "offending_utterances"
		case "all_utterances":
			z = "all_utterances"
		default:
			z = "hits"
		}
	} else {
		z = "ratio"
	}

	return domain.HeatmapWeeklyResp{
		Z:    z,
		Grid: grid,
	}, nil
}
