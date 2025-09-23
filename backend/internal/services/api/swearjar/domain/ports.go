package domain

import (
	"context"
)

// ServicePort defines the swearjar service interface
type ServicePort interface {
	TimeseriesHits(ctx context.Context, in TimeseriesHitsInput) (TimeseriesHitsResp, error)
	HeatmapWeekly(ctx context.Context, in HeatmapWeeklyInput) (HeatmapWeeklyResp, error)
	LangBars(ctx context.Context, in LangBarsInput) (LangBarsResp, error)

	TimeseriesByDetver(ctx context.Context, in TimeseriesDetverInput) (TimeseriesDetverResp, error)
	CodeLangBars(ctx context.Context, in CodeLangBarsInput) (CodeLangBarsResp, error)
	CategoriesStack(ctx context.Context, in CategoriesStackInput) (CategoriesStackResp, error)
	TopTerms(ctx context.Context, in TopTermsInput) (TopTermsResp, error)
	TermTimeline(ctx context.Context, in TermTimelineInput) (TermTimelineResp, error)
	TargetsMix(ctx context.Context, in TargetsMixInput) (TargetsMixResp, error)
	TermsMatrix(ctx context.Context, in TermsMatrixInput) (TermsMatrixResp, error)
	RepoOverview(ctx context.Context, in RepoOverviewInput) (RepoOverviewResp, error)
	Samples(ctx context.Context, in SamplesInput) (SamplesResp, error)
	RatiosTime(ctx context.Context, in RatiosTimeInput) (RatiosTimeResp, error)
	SeverityTimeseries(ctx context.Context, in SeverityTimeseriesInput) (SeverityTimeseriesResp, error)
	SpikeDrivers(ctx context.Context, in SpikeDriversInput) (SpikeDriversResp, error)

	ActorsLeaderboard(ctx context.Context, in ActorsLeaderboardInput) (ActorsLeaderboardResp, error)
	ReposLeaderboard(ctx context.Context, in ReposLeaderboardInput) (ReposLeaderboardResp, error)
	TermsSuggest(ctx context.Context, in TermsSuggestInput) (TermsSuggestResp, error)
	TimeseriesHourly(ctx context.Context, in TimeseriesHourlyInput) (TimeseriesHourlyResp, error)
	ActorOverview(ctx context.Context, in ActorOverviewInput) (ActorOverviewResp, error)
	RepoActorCrosstab(ctx context.Context, in RepoActorCrosstabInput) (RepoActorCrosstabResp, error)

	KPIStrip(ctx context.Context, in KPIStripInput) (KPIStripResp, error)
	YearlyTrends(ctx context.Context, in YearlyTrendsInput) (YearlyTrendsResp, error)
}
