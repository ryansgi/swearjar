package repo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"swearjar/internal/services/api/swearjar/domain"
)

func (s *hybridStore) LangBars(
	ctx context.Context,
	in domain.LangBarsInput,
) (domain.LangBarsResp, error) {
	startDay, err := time.Parse("2006-01-02", in.Range.Start)
	if err != nil {
		return domain.LangBarsResp{}, err
	}
	endDay, err := time.Parse("2006-01-02", in.Range.End)
	if err != nil {
		return domain.LangBarsResp{}, err
	}
	startTS := time.Date(startDay.Year(), startDay.Month(), startDay.Day(), 0, 0, 0, 0, time.UTC)
	endTS := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)

	series := strings.ToLower(strings.TrimSpace(in.Series))
	switch series {
	case "hits", "offending_utterances", "all_utterances":
	default:
		series = "hits"
	}

	crWhere := []string{"created_at >= ?", "created_at < ?"}
	crArgs := []any{startTS, endTS}
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

	utWhere := []string{"bucket_hour >= ?", "bucket_hour < ?"}
	utArgs := []any{startTS, endTS}
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

	lim := in.Page.Limit
	if lim <= 0 {
		lim = 25
	}
	if lim > 200 {
		lim = 200
	}
	limitClause := fmt.Sprintf("LIMIT %d", lim)

	// Choose ORDER BY column based on series
	orderCol := "hits"
	switch series {
	case "offending_utterances":
		orderCol = "off_utt"
	case "all_utterances":
		orderCol = "all_utt"
	}

	sql := fmt.Sprintf(`
		WITH
		cr AS (
			SELECT
				ifNull(nullIf(lang_code, ''), 'unknown') AS lang,
				count()                                   AS hits,
				uniqCombined(12)(utterance_id)            AS off_utt
			FROM swearjar.commit_crimes
			WHERE %s
			GROUP BY lang
		),
		ut AS (
			SELECT
				ifNull(nullIf(lang_code, ''), 'unknown') AS lang,
				/* all utterances (not distinct) across the window */
				countMerge(cnt_state)                    AS all_utt
			FROM swearjar.utt_hour_agg
			WHERE %s
			GROUP BY lang
		)
		SELECT
			coalesce(cr.lang, ut.lang) AS lang,
			ifNull(cr.hits,    0)      AS hits,
			ifNull(cr.off_utt, 0)      AS off_utt,
			ifNull(ut.all_utt, 0)      AS all_utt
		FROM cr
		FULL OUTER JOIN ut USING(lang)
		ORDER BY %s DESC, lang ASC
		%s
	`, strings.Join(crWhere, " AND "), strings.Join(utWhere, " AND "), orderCol, limitClause)

	args := append([]any{}, crArgs...)
	args = append(args, utArgs...)

	type row struct {
		Lang   string
		Hits   uint64
		OffUtt uint64
		AllUtt uint64
	}
	rs, err := s.ch.Query(ctx, sql, args...)
	if err != nil {
		return domain.LangBarsResp{}, err
	}
	defer rs.Close()

	items := make([]domain.LangBarItem, 0, lim)

	// Sum of the *selected* series for the footer total
	var totalSelected int64

	for rs.Next() {
		var r row
		if err := rs.Scan(&r.Lang, &r.Hits, &r.OffUtt, &r.AllUtt); err != nil {
			return domain.LangBarsResp{}, err
		}

		// Map selected series into the outward-facing Hits field
		var plotted int64
		switch series {
		case "offending_utterances":
			plotted = int64(r.OffUtt)
		case "all_utterances":
			plotted = int64(r.AllUtt)
		default: // "hits"
			plotted = int64(r.Hits)
		}
		totalSelected += plotted

		it := domain.LangBarItem{
			Lang:       r.Lang,
			Hits:       plotted,
			Utterances: int64(r.AllUtt),
		}
		// Ratio remains "rarity": hits / all_utterances
		if r.AllUtt > 0 {
			it.Ratio = float64(r.Hits) / float64(r.AllUtt)
		}
		items = append(items, it)
	}
	if err := rs.Err(); err != nil {
		return domain.LangBarsResp{}, err
	}

	// Footer total reflects the currently selected series
	return domain.LangBarsResp{
		Items:     items,
		TotalHits: totalSelected,
	}, nil
}
