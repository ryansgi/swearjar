// Package service implements the swearjar API facade
package service

import (
	"context"

	"swearjar/internal/modkit/repokit"
	"swearjar/internal/services/api/swearjar/domain"
	srepo "swearjar/internal/services/api/swearjar/repo"
)

// Service is the concrete implementation of domain.ServicePort
type Service struct {
	DB   repokit.TxRunner
	Repo repokit.Binder[srepo.StorageRepo]
}

// New constructs a swearjar service
func New(db repokit.TxRunner, binder repokit.Binder[srepo.StorageRepo]) *Service {
	if db == nil {
		panic("swearjar.Service requires a non-nil TxRunner")
	}
	if binder == nil {
		panic("swearjar.Service requires a non-nil repo Binder")
	}
	return &Service{DB: db, Repo: binder}
}

// TimeseriesHits returns timeseries of the swearjar
func (s *Service) TimeseriesHits(
	ctx context.Context,
	in domain.TimeseriesHitsInput,
) (domain.TimeseriesHitsResp, error) {
	var out domain.TimeseriesHitsResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).TimeseriesHits(ctx, in)
		return e
	})
	return out, err
}

// HeatmapWeekly is unimplemented
func (s *Service) HeatmapWeekly(ctx context.Context, in domain.HeatmapWeeklyInput) (domain.HeatmapWeeklyResp, error) {
	var out domain.HeatmapWeeklyResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).HeatmapWeekly(ctx, in)
		return e
	})
	return out, err
}

// LangBars is unimplemented
func (s *Service) LangBars(ctx context.Context, in domain.LangBarsInput) (domain.LangBarsResp, error) {
	var out domain.LangBarsResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).LangBars(ctx, in)
		return e
	})
	return out, err
}

// TimeseriesByDetver is unimplemented
func (s *Service) TimeseriesByDetver(
	ctx context.Context,
	in domain.TimeseriesDetverInput,
) (domain.TimeseriesDetverResp, error) {
	var out domain.TimeseriesDetverResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).TimeseriesByDetver(ctx, in)
		return e
	})
	return out, err
}

// CodeLangBars is unimplemented
func (s *Service) CodeLangBars(ctx context.Context, in domain.CodeLangBarsInput) (domain.CodeLangBarsResp, error) {
	var out domain.CodeLangBarsResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).CodeLangBars(ctx, in)
		return e
	})
	return out, err
}

// CategoriesStack is unimplemented
func (s *Service) CategoriesStack(
	ctx context.Context,
	in domain.CategoriesStackInput,
) (domain.CategoriesStackResp, error) {
	var out domain.CategoriesStackResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).CategoriesStack(ctx, in)
		return e
	})
	return out, err
}

// TopTerms is unimplemented
func (s *Service) TopTerms(ctx context.Context, in domain.TopTermsInput) (domain.TopTermsResp, error) {
	var out domain.TopTermsResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).TopTerms(ctx, in)
		return e
	})
	return out, err
}

// TermTimeline is unimplemented
func (s *Service) TermTimeline(ctx context.Context, in domain.TermTimelineInput) (domain.TermTimelineResp, error) {
	var out domain.TermTimelineResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).TermTimeline(ctx, in)
		return e
	})
	return out, err
}

// TargetsMix is unimplemented
func (s *Service) TargetsMix(ctx context.Context, in domain.TargetsMixInput) (domain.TargetsMixResp, error) {
	var out domain.TargetsMixResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).TargetsMix(ctx, in)
		return e
	})
	return out, err
}

// TermsMatrix is unimplemented
func (s *Service) TermsMatrix(ctx context.Context, in domain.TermsMatrixInput) (domain.TermsMatrixResp, error) {
	var out domain.TermsMatrixResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).TermsMatrix(ctx, in)
		return e
	})
	return out, err
}

// RepoOverview is unimplemented
func (s *Service) RepoOverview(ctx context.Context, in domain.RepoOverviewInput) (domain.RepoOverviewResp, error) {
	var out domain.RepoOverviewResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).RepoOverview(ctx, in)
		return e
	})
	return out, err
}

// Samples is unimplemented
func (s *Service) Samples(ctx context.Context, in domain.SamplesInput) (domain.SamplesResp, error) {
	var out domain.SamplesResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).Samples(ctx, in)
		return e
	})
	return out, err
}

// RatiosTime is unimplemented
func (s *Service) RatiosTime(ctx context.Context, in domain.RatiosTimeInput) (domain.RatiosTimeResp, error) {
	var out domain.RatiosTimeResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).RatiosTime(ctx, in)
		return e
	})
	return out, err
}

// SeverityTimeseries is unimplemented
func (s *Service) SeverityTimeseries(
	ctx context.Context,
	in domain.SeverityTimeseriesInput,
) (domain.SeverityTimeseriesResp, error) {
	var out domain.SeverityTimeseriesResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).SeverityTimeseries(ctx, in)
		return e
	})
	return out, err
}

// SpikeDrivers is unimplemented
func (s *Service) SpikeDrivers(ctx context.Context, in domain.SpikeDriversInput) (domain.SpikeDriversResp, error) {
	var out domain.SpikeDriversResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).SpikeDrivers(ctx, in)
		return e
	})
	return out, err
}

// ActorsLeaderboard is unimplemented
func (s *Service) ActorsLeaderboard(
	_ context.Context,
	_ domain.ActorsLeaderboardInput,
) (domain.ActorsLeaderboardResp, error) {
	return domain.ActorsLeaderboardResp{Items: []domain.ActorsLeaderboardRow{}}, nil
}

// ReposLeaderboard is unimplemented
func (s *Service) ReposLeaderboard(
	_ context.Context,
	_ domain.ReposLeaderboardInput,
) (domain.ReposLeaderboardResp, error) {
	return domain.ReposLeaderboardResp{Items: []domain.ReposLeaderboardRow{}}, nil
}

// TermsSuggest is unimplemented
func (s *Service) TermsSuggest(_ context.Context, _ domain.TermsSuggestInput) (domain.TermsSuggestResp, error) {
	return domain.TermsSuggestResp{Terms: []string{}}, nil
}

// TimeseriesHourly is unimplemented
func (s *Service) TimeseriesHourly(
	_ context.Context,
	_ domain.TimeseriesHourlyInput,
) (domain.TimeseriesHourlyResp, error) {
	return domain.TimeseriesHourlyResp{Series: []domain.TimeseriesHourlyPoint{}}, nil
}

// ActorOverview is unimplemented
func (s *Service) ActorOverview(_ context.Context, in domain.ActorOverviewInput) (domain.ActorOverviewResp, error) {
	out := domain.ActorOverviewResp{}
	out.Actor.HID = in.ActorHID
	out.Actor.Label = in.ActorHID[0:6] + "..." + in.ActorHID[len(in.ActorHID)-6:]
	out.Series = []domain.TimeseriesPoint{}
	out.Mix = map[string]int64{}
	out.Top = []struct {
		Term string `json:"term" example:"fuck"`
		Hits int64  `json:"hits" example:"4"`
	}{}
	return out, nil
}

// RepoActorCrosstab is unimplemented
func (s *Service) RepoActorCrosstab(
	_ context.Context,
	_ domain.RepoActorCrosstabInput,
) (domain.RepoActorCrosstabResp, error) {
	return domain.RepoActorCrosstabResp{Cells: []domain.RepoActorCell{}}, nil
}

// KPIStrip returns headline KPIs (for the window; pass today for homepage)
func (s *Service) KPIStrip(
	ctx context.Context,
	in domain.KPIStripInput,
) (domain.KPIStripResp, error) {
	var out domain.KPIStripResp
	err := s.DB.Tx(ctx, func(q repokit.Queryer) error {
		var e error
		out, e = s.Repo.Bind(q).KPIStrip(ctx, in)
		return e
	})
	return out, err
}
