package repo

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"swearjar/internal/services/api/swearjar/domain"
)

// YearlyTrends returns monthly trends (hits, rate, mean severity), seasonality,
// mix snapshot for {maxY, maxY-1}, and detector-version first-seen markers
func (s *hybridStore) YearlyTrends(
	ctx context.Context,
	in domain.YearlyTrendsInput,
) (domain.YearlyTrendsResp, error) {
	const (
		maxSpanYears = 20
	)
	// Resolve year bounds with caps (default from data min/max)
	// We take bounds primarily from crimes (commit_crimes); if absent, fall back to utt_hour_agg

	type bounds struct{ minY, maxY int }
	getBounds := func(ctx context.Context) (bounds, error) {
		// Scope-less min/max first (filtered min/max can be quite expensive)
		var b bounds
		// Prefer commit_crimes
		rs, err := s.ch.Query(ctx, `
			WITH c AS (SELECT min(toYear(created_at)) AS ymin, max(toYear(created_at)) AS ymax FROM swearjar.commit_crimes),
			     u AS (SELECT min(toYear(bucket_hour)) AS ymin, max(toYear(bucket_hour)) AS ymax FROM swearjar.utt_hour_agg)
			SELECT
			  coalesce((SELECT ymin FROM c), (SELECT ymin FROM u), 0) AS ymin,
			  coalesce((SELECT ymax FROM c), (SELECT ymax FROM u), 0) AS ymax
		`)
		if err != nil {
			return b, err
		}
		defer rs.Close()
		if rs.Next() {
			var ymin16, ymax16 uint16
			if err := rs.Scan(&ymin16, &ymax16); err != nil {
				return b, err
			}
			b.minY = int(ymin16)
			b.maxY = int(ymax16)
		}
		if err := rs.Err(); err != nil {
			return b, err
		}
		if b.minY == 0 || b.maxY == 0 || b.maxY < b.minY {
			// no data - return empty span; caller will handle
			return b, nil
		}
		// Cap to current calendar year (UTC)
		nowY := time.Now().UTC().Year()
		if b.maxY > nowY {
			b.maxY = nowY
		}
		return b, nil
	}

	b, err := getBounds(ctx)
	if err != nil {
		return domain.YearlyTrendsResp{}, err
	}
	// If caller specified, clamp inside data bounds
	minY := b.minY
	maxY := b.maxY
	if in.YearRange != nil {
		if in.YearRange.Min > 0 {
			minY = in.YearRange.Min
		}
		if in.YearRange.Max > 0 {
			maxY = in.YearRange.Max
		}
	}
	if minY == 0 || maxY == 0 || maxY < minY {
		// no data
		return domain.YearlyTrendsResp{
			Years: []int{},
			Meta: struct {
				DataMinYear int    `json:"data_min_year" example:"2011"`
				DataMaxYear int    `json:"data_max_year" example:"2014"`
				Interval    string `json:"interval"      example:"month"`
				GeneratedAt string `json:"generated_at"  example:"2025-09-19T10:00:00Z"`
			}{
				DataMinYear: b.minY,
				DataMaxYear: b.maxY,
				Interval:    "month",
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			},
		}, nil
	}
	// Enforce span cap
	if (maxY - minY + 1) > maxSpanYears {
		minY = maxY - (maxSpanYears - 1)
	}

	// Build WHERE from GlobalOptions scope filters
	crWhere := []string{
		"created_at >= ? AND created_at < ?",
	}
	crArgs := []any{
		time.Date(minY, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(maxY+1, 1, 1, 0, 0, 0, 0, time.UTC),
	}
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
	// NOTE: commit_crimes has no code_lang filter at present

	utWhere := []string{
		"bucket_hour >= ? AND bucket_hour < ?",
	}
	utArgs := []any{
		time.Date(minY, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(maxY+1, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	// Mirror feasible filters onto utt_hour_agg
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
	// Monthly aggregates: crimes + utterances (UTC months)
	sqlMonthly := fmt.Sprintf(`
		WITH
		cr AS (
			SELECT
			  toStartOfMonth(created_at)                     AS month,
			  count()                                        AS hits,
			  sumIf(1, severity = 'mild')                    AS mild_hits,
			  sumIf(1, severity = 'strong')                  AS strong_hits,
			  sumIf(1, severity = 'slur_masked')             AS slur_hits
			FROM swearjar.commit_crimes
			WHERE %s
			GROUP BY month
		),
		ut AS (
			SELECT
			  toStartOfMonth(bucket_hour)                     AS month,
			  uniqMerge(u_state)                              AS utt
			FROM swearjar.utt_hour_agg
			WHERE %s
			GROUP BY month
		)
		SELECT
		  coalesce(cr.month, ut.month) AS month,
		  ifNull(cr.hits, 0)           AS hits,
		  ifNull(cr.mild_hits, 0)      AS mild_hits,
		  ifNull(cr.strong_hits, 0)    AS strong_hits,
		  ifNull(cr.slur_hits, 0)      AS slur_hits,
		  ifNull(ut.utt, 0)            AS utt
		FROM cr
		FULL OUTER JOIN ut USING month
		ORDER BY month ASC
	`, strings.Join(crWhere, " AND "), strings.Join(utWhere, " AND "))

	args := append([]any{}, crArgs...)
	args = append(args, utArgs...)

	type mrow struct {
		month  time.Time
		hits   uint64
		mild   uint64
		strong uint64
		slur   uint64
		utt    uint64
	}
	rs, err := s.ch.Query(ctx, sqlMonthly, args...)
	if err != nil {
		return domain.YearlyTrendsResp{}, err
	}
	defer rs.Close()

	// Allocate dense 12-per-year vectors up front
	years := make([]int, 0, maxY-minY+1)
	for y := minY; y <= maxY; y++ {
		years = append(years, y)
	}
	hitsByY := make(map[int][]int64, len(years))
	rateByY := make(map[int][]float64, len(years))
	seviByY := make(map[int][]float64, len(years))
	for _, y := range years {
		hitsByY[y] = make([]int64, 12)
		rateByY[y] = make([]float64, 12)
		seviByY[y] = make([]float64, 12)
	}

	type perMonth struct {
		hits   int64
		utt    int64
		mild   int64
		strong int64
		slur   int64
	}
	// also keep for seasonality computation across years
	byYM := make(map[[2]int]perMonth, (maxY-minY+1)*12)

	for rs.Next() {
		var r mrow
		if err := rs.Scan(&r.month, &r.hits, &r.mild, &r.strong, &r.slur, &r.utt); err != nil {
			return domain.YearlyTrendsResp{}, err
		}
		y := r.month.UTC().Year()
		m := int(r.month.UTC().Month()) // 1..12
		if y < minY || y > maxY || m < 1 || m > 12 {
			continue
		}
		pm := perMonth{
			hits:   int64(r.hits),
			utt:    int64(r.utt),
			mild:   int64(r.mild),
			strong: int64(r.strong),
			slur:   int64(r.slur),
		}
		byYM[[2]int{y, m}] = pm

		// rate & mean severity index
		hitsByY[y][m-1] = pm.hits
		if pm.utt > 0 {
			rateByY[y][m-1] = float64(pm.hits) / float64(pm.utt)
		}
		totalSev := pm.mild + pm.strong + pm.slur
		if totalSev > 0 {
			mean := (1.0*float64(pm.mild) + 2.0*float64(pm.strong) + 3.0*float64(pm.slur)) / float64(totalSev)
			seviByY[y][m-1] = mean
		}
	}
	if err := rs.Err(); err != nil {
		return domain.YearlyTrendsResp{}, err
	}

	// Seasonality bands (median/p25/p75 over selected years)
	// Helper: percentile
	percentile := func(sorted []float64, p float64) float64 {
		n := len(sorted)
		if n == 0 {
			return 0
		}
		if n == 1 {
			return sorted[0]
		}
		// linear interpolation between closest ranks
		pos := p * float64(n-1)
		i := int(pos)
		f := pos - float64(i)
		if i+1 >= n {
			return sorted[n-1]
		}
		return sorted[i] + f*(sorted[i+1]-sorted[i])
	}
	buildBands := func(get func(y int, m int) float64) []domain.MonthBand {
		out := make([]domain.MonthBand, 12)
		for m := 1; m <= 12; m++ {
			vals := make([]float64, 0, len(years))
			for _, y := range years {
				vals = append(vals, get(y, m))
			}
			sort.Float64s(vals)
			out[m-1] = domain.MonthBand{
				M:      m,
				Median: percentile(vals, 0.5),
				P25:    percentile(vals, 0.25),
				P75:    percentile(vals, 0.75),
			}
		}
		return out
	}
	seasonality := map[string][]domain.MonthBand{
		"hits":     buildBands(func(y, m int) float64 { return float64(hitsByY[y][m-1]) }),
		"rate":     buildBands(func(y, m int) float64 { return rateByY[y][m-1] }),
		"severity": buildBands(func(y, m int) float64 { return seviByY[y][m-1] }),
	}

	// Mix snapshot for {maxY, maxY-1}
	type mixRow struct {
		y   uint16
		cat *string
		h   uint64
	}
	mix := (*struct {
		ThisYear []domain.CategoryShare `json:"this_year"`
		LastYear []domain.CategoryShare `json:"last_year"`
	})(nil)

	if maxY >= minY {
		// Build the optional scope tail after the two date predicates (or empty)
		scopeTail := ""
		if len(crWhere) > 1 {
			scopeTail = " AND " + strings.Join(crWhere[1:], " AND ")
		}

		sqlMix := `
			SELECT toYear(created_at) AS y,
			       cast(category AS Nullable(String)) AS cat,
			       count() AS hits
			FROM swearjar.commit_crimes
			WHERE created_at >= toDateTime(?, 'UTC')
			  AND created_at <  toDateTime(?, 'UTC')
        ` + scopeTail + `
			GROUP BY y, cat
			ORDER BY y ASC, hits DESC
		`
		// window is [maxY-1 .. maxY]
		mStart := time.Date(maxY-1, 1, 1, 0, 0, 0, 0, time.UTC)
		mEnd := time.Date(maxY+1, 1, 1, 0, 0, 0, 0, time.UTC)
		mArgs := []any{mStart, mEnd}
		// reuse the scope filters from crimes, skipping their original date args
		if len(crArgs) > 2 {
			mArgs = append(mArgs, crArgs[2:]...)
		}
		rs2, err2 := s.ch.Query(ctx, sqlMix, mArgs...)
		if err2 != nil {
			return domain.YearlyTrendsResp{}, err2
		}
		defer rs2.Close()

		type acc struct {
			total uint64
			byCat map[string]uint64
		}
		accByYear := map[int]*acc{
			maxY:     {byCat: map[string]uint64{}},
			maxY - 1: {byCat: map[string]uint64{}},
		}
		for rs2.Next() {
			var r mixRow
			if err := rs2.Scan(&r.y, &r.cat, &r.h); err != nil {
				return domain.YearlyTrendsResp{}, err
			}
			a, ok := accByYear[int(r.y)]
			if !ok {
				continue
			}
			key := "unknown"
			if r.cat != nil && strings.TrimSpace(*r.cat) != "" {
				key = *r.cat
			}
			a.byCat[key] += r.h
			a.total += r.h
		}
		if err := rs2.Err(); err != nil {
			return domain.YearlyTrendsResp{}, err
		}

		buildShares := func(a *acc) []domain.CategoryShare {
			if a == nil || a.total == 0 {
				return nil
			}
			out := make([]domain.CategoryShare, 0, len(a.byCat))
			for k, v := range a.byCat {
				out = append(out, domain.CategoryShare{
					Key:   k,
					Hits:  int64(v),
					Share: float64(v) / float64(a.total),
				})
			}
			// Keep a stable, highest-first order
			sort.Slice(out, func(i, j int) bool {
				if out[i].Hits == out[j].Hits {
					return out[i].Key < out[j].Key
				}
				return out[i].Hits > out[j].Hits
			})
			return out
		}
		mix = &struct {
			ThisYear []domain.CategoryShare `json:"this_year"`
			LastYear []domain.CategoryShare `json:"last_year"`
		}{
			ThisYear: buildShares(accByYear[maxY]),
			LastYear: buildShares(accByYear[maxY-1]),
		}
	}

	// Detector version markers (first seen per detver)
	// Typically scope-less; but if you *want* scoped markers, reuse crWhere
	rs3, err := s.ch.Query(ctx, `
		WITH firsts AS (
			SELECT detver AS v, min(toDate(created_at)) AS first_day
			FROM swearjar.commit_crimes
			GROUP BY v
		)
		SELECT v, first_day
		FROM firsts
		ORDER BY first_day ASC
	`)
	if err != nil {
		return domain.YearlyTrendsResp{}, err
	}
	defer rs3.Close()
	markers := make([]domain.DetverMarker, 0, 16)
	for rs3.Next() {
		var ver32 int32
		var day time.Time
		if err := rs3.Scan(&ver32, &day); err != nil {
			return domain.YearlyTrendsResp{}, err
		}
		markers = append(markers, domain.DetverMarker{
			Date:    day.Format("2006-01-02"),
			Version: int(ver32),
		})
	}
	if err := rs3.Err(); err != nil {
		return domain.YearlyTrendsResp{}, err
	}

	// Fill resp and return
	out := domain.YearlyTrendsResp{
		Years: years,
		Monthly: struct {
			Hits     map[int][]int64   `json:"hits,omitempty"`
			Rate     map[int][]float64 `json:"rate,omitempty"`
			Severity map[int][]float64 `json:"severity,omitempty"`
		}{
			Hits:     hitsByY,
			Rate:     rateByY,
			Severity: seviByY,
		},
		Seasonality:   seasonality,
		Mix:           mix,
		DetverMarkers: markers,
	}
	out.Meta.DataMinYear = b.minY
	out.Meta.DataMaxYear = b.maxY
	out.Meta.Interval = "month"
	out.Meta.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	// Optional: respect Include[] to trim payload
	if len(in.Include) > 0 {
		keep := func(k string) bool {
			return slices.Contains(in.Include, k)
		}
		if !keep("hits") {
			out.Monthly.Hits = nil
		}
		if !keep("rate") {
			out.Monthly.Rate = nil
		}
		if !keep("severity") {
			out.Monthly.Severity = nil
		}
		if !keep("seasonality") {
			out.Seasonality = nil
		}
		if !keep("mix") {
			out.Mix = nil
		}
		if !keep("detver_markers") {
			out.DetverMarkers = nil
		}
	}

	return out, nil
}
