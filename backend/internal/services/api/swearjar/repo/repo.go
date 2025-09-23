// Package repo provides the storage repository implementation for swearjar service
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/platform/store"
	"swearjar/internal/services/api/swearjar/domain"
)

// StorageRepo defines the storage repository interface for swearjar service
type StorageRepo interface {
	TimeseriesHits(ctx context.Context, in domain.TimeseriesHitsInput) (domain.TimeseriesHitsResp, error)
	HeatmapWeekly(ctx context.Context, in domain.HeatmapWeeklyInput) (domain.HeatmapWeeklyResp, error)
	LangBars(ctx context.Context, in domain.LangBarsInput) (domain.LangBarsResp, error)

	TimeseriesByDetver(ctx context.Context, in domain.TimeseriesDetverInput) (domain.TimeseriesDetverResp, error)
	CodeLangBars(ctx context.Context, in domain.CodeLangBarsInput) (domain.CodeLangBarsResp, error)
	CategoriesStack(ctx context.Context, in domain.CategoriesStackInput) (domain.CategoriesStackResp, error)
	TopTerms(ctx context.Context, in domain.TopTermsInput) (domain.TopTermsResp, error)
	TermTimeline(ctx context.Context, in domain.TermTimelineInput) (domain.TermTimelineResp, error)
	TargetsMix(ctx context.Context, in domain.TargetsMixInput) (domain.TargetsMixResp, error)
	TermsMatrix(ctx context.Context, in domain.TermsMatrixInput) (domain.TermsMatrixResp, error)
	RepoOverview(ctx context.Context, in domain.RepoOverviewInput) (domain.RepoOverviewResp, error)
	Samples(ctx context.Context, in domain.SamplesInput) (domain.SamplesResp, error)
	RatiosTime(ctx context.Context, in domain.RatiosTimeInput) (domain.RatiosTimeResp, error)
	SeverityTimeseries(ctx context.Context, in domain.SeverityTimeseriesInput) (domain.SeverityTimeseriesResp, error)
	SpikeDrivers(ctx context.Context, in domain.SpikeDriversInput) (domain.SpikeDriversResp, error)
	KPIStrip(ctx context.Context, in domain.KPIStripInput) (domain.KPIStripResp, error)
	YearlyTrends(ctx context.Context, in domain.YearlyTrendsInput) (domain.YearlyTrendsResp, error)
}

// NewHybrid constructs a hybrid storage binder using PG and CH
func NewHybrid(ch store.Clickhouse) repokit.Binder[StorageRepo] { return &hybridBinder{ch: ch} }

type hybridBinder struct{ ch store.Clickhouse }

// Bind binds a Queryer to produce a StorageRepo
func (b *hybridBinder) Bind(q repokit.Queryer) StorageRepo { return &hybridStore{pg: q, ch: b.ch} }

type hybridStore struct {
	pg repokit.Queryer
	ch store.Clickhouse
}

func unimpl[T any]() (T, error) { var z T; return z, errors.New("unimplemented") }

// TimeseriesHits queries ClickHouse for hits/utterances over time
// This is a first pass. We'll make it better
func (s *hybridStore) TimeseriesHits(
	ctx context.Context,
	in domain.TimeseriesHitsInput,
) (domain.TimeseriesHitsResp, error) {
	interval := strings.ToLower(strings.TrimSpace(in.Interval))
	switch interval {
	case "", "auto":
		interval = "day"
	case "hour", "day", "week", "month":
	default:
		interval = "day"
	}
	tz := strings.TrimSpace(in.TZ)
	if tz == "" {
		tz = "UTC"
	}

	start, err := time.Parse("2006-01-02", in.Range.Start)
	if err != nil {
		return domain.TimeseriesHitsResp{}, err
	}
	endIncl, err := time.Parse("2006-01-02", in.Range.End)
	if err != nil {
		return domain.TimeseriesHitsResp{}, err
	}
	endExcl := endIncl.Add(24 * time.Hour)

	var bucketExprCrimes, bucketExprUtt, fmtMask string
	switch interval {
	case "hour":
		bucketExprCrimes = "toStartOfHour(toTimeZone(created_at, ?))"
		bucketExprUtt = "toStartOfHour(toTimeZone(bucket_hour, ?))"
		fmtMask = "%Y-%m-%dT%H:00:00"
	case "week":
		bucketExprCrimes = "toStartOfWeek(toTimeZone(created_at, ?))"
		bucketExprUtt = "toStartOfWeek(toTimeZone(bucket_hour, ?))"
		fmtMask = "%Y-%m-%d"
	case "month":
		bucketExprCrimes = "toStartOfMonth(toTimeZone(created_at, ?))"
		bucketExprUtt = "toStartOfMonth(toTimeZone(bucket_hour, ?))"
		fmtMask = "%Y-%m-01"
	default: // day
		bucketExprCrimes = "toStartOfDay(toTimeZone(created_at, ?))"
		bucketExprUtt = "toStartOfDay(toTimeZone(bucket_hour, ?))"
		fmtMask = "%Y-%m-%d"
	}

	// Build combined series from commit_crimes (hits + offending_utt) and utt_hour_agg (all_utt)
	sql := fmt.Sprintf(`
		WITH
		crimes AS (
			SELECT
				formatDateTime(%s, '%s') AS t,
				count() AS hits,
				uniqCombined(12)(utterance_id) AS off_utt
			FROM swearjar.commit_crimes
			WHERE created_at >= ? AND created_at < ?
			GROUP BY t
		),
		utt AS (
			SELECT
				formatDateTime(%s, '%s') AS t,
				countMerge(cnt_state) AS all_utt
			FROM swearjar.utt_hour_agg
			WHERE bucket_hour >= ? AND bucket_hour < ?
			GROUP BY t
		)
		SELECT
			coalesce(c.t, u.t)     AS t,
			ifNull(c.hits, 0)      AS hits,
			ifNull(c.off_utt, 0)   AS off_utt,
			ifNull(u.all_utt, 0)   AS all_utt
		FROM crimes c
		FULL OUTER JOIN utt u ON c.t = u.t
		ORDER BY t ASC
	`, bucketExprCrimes, fmtMask, bucketExprUtt, fmtMask)

	rs, err := s.ch.Query(ctx, sql,
		tz, start, endExcl, // crimes tz + range
		tz, start, endExcl, // utt tz + range
	)
	if err != nil {
		return domain.TimeseriesHitsResp{}, err
	}
	defer rs.Close()

	type row struct {
		t      string
		hits   uint64
		offUtt uint64
		allUtt uint64
	}
	var fetched []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.t, &r.hits, &r.offUtt, &r.allUtt); err != nil {
			return domain.TimeseriesHitsResp{}, err
		}
		fetched = append(fetched, r)
	}
	if err := rs.Err(); err != nil {
		return domain.TimeseriesHitsResp{}, err
	}

	emitKey := func(t time.Time) string {
		switch interval {
		case "hour":
			return t.Format("2006-01-02T15:00:00")
		default: // day|week|month (month handled below)
			return t.Format("2006-01-02")
		}
	}

	// Build lookup
	byKey := make(map[string]row, len(fetched))
	for _, r := range fetched {
		byKey[r.t] = r
	}

	// helper to build a point (compute intensity/coverage/rarity when possible)
	buildPoint := func(key string, r row) domain.TimeseriesPoint {
		pt := domain.TimeseriesPoint{
			T:                   key,
			Hits:                int64(r.hits),
			OffendingUtterances: int64(r.offUtt),
			AllUtterances:       int64(r.allUtt),
		}
		if r.offUtt > 0 {
			pt.Intensity = float64(r.hits) / float64(r.offUtt)
		}
		if r.allUtt > 0 {
			pt.Coverage = float64(r.offUtt) / float64(r.allUtt)
			pt.Rarity = float64(r.hits) / float64(r.allUtt)
		}
		return pt
	}

	var series []domain.TimeseriesPoint
	switch interval {
	case "month":
		// keep sparse months (variable step)
		series = make([]domain.TimeseriesPoint, 0, len(fetched))
		for _, r := range fetched {
			series = append(series, buildPoint(r.t, r))
		}
	default:
		// linear step fill for hour/day/week
		step := 24 * time.Hour
		switch interval {
		case "hour":
			step = time.Hour
		case "week":
			step = 7 * 24 * time.Hour
		}
		for t := start; t.Before(endExcl); t = t.Add(step) {
			key := emitKey(t)
			r := byKey[key] // zero-value row if missing
			series = append(series, buildPoint(key, r))
		}
	}

	return domain.TimeseriesHitsResp{
		Interval: interval,
		Series:   series,
	}, nil
}

// HeatmapWeekly is unimplemented
func (s *hybridStore) TimeseriesByDetver(
	ctx context.Context,
	in domain.TimeseriesDetverInput,
) (domain.TimeseriesDetverResp, error) {
	return unimpl[domain.TimeseriesDetverResp]()
}

// CodeLangBars is unimplemented
func (s *hybridStore) CodeLangBars(ctx context.Context, in domain.CodeLangBarsInput) (domain.CodeLangBarsResp, error) {
	return unimpl[domain.CodeLangBarsResp]()
}

// TopTerms is unimplemented
func (s *hybridStore) TopTerms(ctx context.Context, in domain.TopTermsInput) (domain.TopTermsResp, error) {
	return unimpl[domain.TopTermsResp]()
}

// TermTimeline is unimplemented
func (s *hybridStore) TermTimeline(ctx context.Context, in domain.TermTimelineInput) (domain.TermTimelineResp, error) {
	return unimpl[domain.TermTimelineResp]()
}

// TargetsMix is unimplemented
func (s *hybridStore) TargetsMix(ctx context.Context, in domain.TargetsMixInput) (domain.TargetsMixResp, error) {
	return unimpl[domain.TargetsMixResp]()
}

// TermsMatrix is unimplemented
func (s *hybridStore) TermsMatrix(ctx context.Context, in domain.TermsMatrixInput) (domain.TermsMatrixResp, error) {
	return unimpl[domain.TermsMatrixResp]()
}

// RepoOverview is unimplemented
func (s *hybridStore) RepoOverview(ctx context.Context, in domain.RepoOverviewInput) (domain.RepoOverviewResp, error) {
	return unimpl[domain.RepoOverviewResp]()
}

// Samples is unimplemented
func (s *hybridStore) Samples(ctx context.Context, in domain.SamplesInput) (domain.SamplesResp, error) {
	return unimpl[domain.SamplesResp]()
}

// RatiosTime is unimplemented
func (s *hybridStore) RatiosTime(ctx context.Context, in domain.RatiosTimeInput) (domain.RatiosTimeResp, error) {
	return unimpl[domain.RatiosTimeResp]()
}

// SeverityTimeseries is unimplemented
func (s *hybridStore) SeverityTimeseries(
	ctx context.Context,
	in domain.SeverityTimeseriesInput,
) (domain.SeverityTimeseriesResp, error) {
	return unimpl[domain.SeverityTimeseriesResp]()
}

// SpikeDrivers is unimplemented
func (s *hybridStore) SpikeDrivers(ctx context.Context, in domain.SpikeDriversInput) (domain.SpikeDriversResp, error) {
	return unimpl[domain.SpikeDriversResp]()
}
