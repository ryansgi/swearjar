// Package http provides HTTP transport for the swearjar API
package http

import (
	stdhttp "net/http"

	"swearjar/internal/modkit/httpkit"
	"swearjar/internal/services/api/swearjar/domain"
	svc "swearjar/internal/services/api/swearjar/service"
)

// Register mounts swearjar endpoints on the given router.
// We use POST with JSON bodies for composable, future-proof query shapes
func Register(r httpkit.Router, s *svc.Service) {
	h := &handlers{svc: s}

	// 1
	httpkit.PostJSON[domain.TimeseriesHitsInput](r, "/timeseries/hits", h.timeseriesHits)
	// 2
	httpkit.PostJSON[domain.TimeseriesDetverInput](r, "/timeseries/hits-by-detver", h.timeseriesByDetver)
	// 3
	httpkit.PostJSON[domain.HeatmapWeeklyInput](r, "/heatmap/weekly", h.heatmapWeekly)
	// 4
	httpkit.PostJSON[domain.LangBarsInput](r, "/bars/nl-lang", h.langBars)
	// 5
	httpkit.PostJSON[domain.CodeLangBarsInput](r, "/bars/code-lang", h.codeLangBars)
	// 6
	httpkit.PostJSON[domain.CategoriesStackInput](r, "/stacked/categories", h.categoriesStack)
	// 7
	httpkit.PostJSON[domain.TopTermsInput](r, "/terms/top", h.topTerms)
	// 8
	httpkit.PostJSON[domain.TermTimelineInput](r, "/timeseries/term", h.termTimeline)
	// 9
	httpkit.PostJSON[domain.TargetsMixInput](r, "/targets/mix", h.targetsMix)
	// 10
	httpkit.PostJSON[domain.TermsMatrixInput](r, "/terms/matrix", h.termsMatrix)
	// 11
	httpkit.PostJSON[domain.RepoOverviewInput](r, "/repo/overview", h.repoOverview)
	// 12
	httpkit.PostJSON[domain.SamplesInput](r, "/samples/commit-crimes", h.samples)
	// 13
	httpkit.PostJSON[domain.RatiosTimeInput](r, "/ratios/time", h.ratiosTime)
	// 14
	httpkit.PostJSON[domain.SeverityTimeseriesInput](r, "/timeseries/severity", h.severityTimeseries)
	// 15
	httpkit.PostJSON[domain.SpikeDriversInput](r, "/spike/drivers", h.spikeDrivers)

	httpkit.PostJSON[domain.ActorsLeaderboardInput](r, "/leaders/actors", h.actorsLeaderboard)       // 16
	httpkit.PostJSON[domain.ReposLeaderboardInput](r, "/leaders/repos", h.reposLeaderboard)          // 17
	httpkit.PostJSON[domain.TermsSuggestInput](r, "/terms/suggest", h.termsSuggest)                  // 18
	httpkit.PostJSON[domain.TimeseriesHourlyInput](r, "/timeseries/hits-hourly", h.timeseriesHourly) // 20
	httpkit.PostJSON[domain.ActorOverviewInput](r, "/actors/overview", h.actorOverview)              // 21
	httpkit.PostJSON[domain.RepoActorCrosstabInput](r, "/crosstab/repo-actor", h.repoActorCrosstab)  // 22

	httpkit.PostJSON[domain.KPIStripInput](r, "/kpi", h.kpiStrip)                   // 23
	httpkit.PostJSON[domain.YearlyTrendsInput](r, "/yearly/trends", h.yearlyTrends) // 24
}

type handlers struct{ svc *svc.Service }

// swagger:route POST /swearjar/timeseries/hits Swearjar swearjarTimeseriesHits
// @Summary Profanity over time (hits and utterances)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.TimeseriesHitsInput true "Query"
// @Success 200 {object} domain.TimeseriesHitsResp "ok"
// @Router /swearjar/timeseries/hits [post]
func (h *handlers) timeseriesHits(r *stdhttp.Request, in domain.TimeseriesHitsInput) (any, error) {
	return h.svc.TimeseriesHits(r.Context(), in)
}

// swagger:route POST /swearjar/heatmap/weekly Swearjar swearjarHeatmapWeekly
// @Summary Weekly rhythm heatmap (day-of-week x hour)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.HeatmapWeeklyInput true "Query"
// @Success 200 {object} domain.HeatmapWeeklyResp "ok"
// @Router /swearjar/heatmap/weekly [post]
func (h *handlers) heatmapWeekly(r *stdhttp.Request, in domain.HeatmapWeeklyInput) (any, error) {
	return h.svc.HeatmapWeekly(r.Context(), in)
}

// swagger:route POST /swearjar/bars/nl-lang Swearjar swearjarLangBars
// @Summary Natural language distribution (spoken language of text)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.LangBarsInput true "Query"
// @Success 200 {object} domain.LangBarsResp "ok"
// @Router /swearjar/bars/nl-lang [post]
func (h *handlers) langBars(r *stdhttp.Request, in domain.LangBarsInput) (any, error) {
	return h.svc.LangBars(r.Context(), in)
}

// swagger:route POST /swearjar/timeseries/hits-by-detver Swearjar swearjarTimeseriesByDetver
// @Summary Detector version comparison over time
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.TimeseriesDetverInput true "Query"
// @Success 200 {object} domain.TimeseriesDetverResp "ok"
// @Router /swearjar/timeseries/hits-by-detver [post]
func (h *handlers) timeseriesByDetver(r *stdhttp.Request, in domain.TimeseriesDetverInput) (any, error) {
	return h.svc.TimeseriesByDetver(r.Context(), in)
}

// swagger:route POST /swearjar/bars/code-lang Swearjar swearjarCodeLangBars
// @Summary Programming language correlation (repo primary language)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.CodeLangBarsInput true "Query"
// @Success 200 {object} domain.CodeLangBarsResp "ok"
// @Router /swearjar/bars/code-lang [post]
func (h *handlers) codeLangBars(r *stdhttp.Request, in domain.CodeLangBarsInput) (any, error) {
	return h.svc.CodeLangBars(r.Context(), in)
}

// swagger:route POST /swearjar/stacked/categories Swearjar swearjarCategoriesStack
// @Summary Categories & severity mix
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.CategoriesStackInput true "Query"
// @Success 200 {object} domain.CategoriesStackResp "ok"
// @Router /swearjar/stacked/categories [post]
func (h *handlers) categoriesStack(r *stdhttp.Request, in domain.CategoriesStackInput) (any, error) {
	return h.svc.CategoriesStack(r.Context(), in)
}

// swagger:route POST /swearjar/terms/top Swearjar swearjarTopTerms
// @Summary Top terms (expletives) in window
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.TopTermsInput true "Query"
// @Success 200 {object} domain.TopTermsResp "ok"
// @Router /swearjar/terms/top [post]
func (h *handlers) topTerms(r *stdhttp.Request, in domain.TopTermsInput) (any, error) {
	return h.svc.TopTerms(r.Context(), in)
}

// swagger:route POST /swearjar/timeseries/term Swearjar swearjarTermTimeline
// @Summary Term timeline
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.TermTimelineInput true "Query"
// @Success 200 {object} domain.TermTimelineResp "ok"
// @Router /swearjar/timeseries/term [post]
func (h *handlers) termTimeline(r *stdhttp.Request, in domain.TermTimelineInput) (any, error) {
	return h.svc.TermTimeline(r.Context(), in)
}

// swagger:route POST /swearjar/targets/mix Swearjar swearjarTargetsMix
// @Summary Targets mix (bot/tool/generic)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.TargetsMixInput true "Query"
// @Success 200 {object} domain.TargetsMixResp "ok"
// @Router /swearjar/targets/mix [post]
func (h *handlers) targetsMix(r *stdhttp.Request, in domain.TargetsMixInput) (any, error) {
	return h.svc.TargetsMix(r.Context(), in)
}

// swagger:route POST /swearjar/terms/matrix Swearjar swearjarTermsMatrix
// @Summary Term x language matrix
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.TermsMatrixInput true "Query"
// @Success 200 {object} domain.TermsMatrixResp "ok"
// @Router /swearjar/terms/matrix [post]
func (h *handlers) termsMatrix(r *stdhttp.Request, in domain.TermsMatrixInput) (any, error) {
	return h.svc.TermsMatrix(r.Context(), in)
}

// swagger:route POST /swearjar/repo/overview Swearjar swearjarRepoOverview
// @Summary Repo lens (opt-in name reveal)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.RepoOverviewInput true "Query"
// @Success 200 {object} domain.RepoOverviewResp "ok"
// @Router /swearjar/repo/overview [post]
func (h *handlers) repoOverview(r *stdhttp.Request, in domain.RepoOverviewInput) (any, error) {
	return h.svc.RepoOverview(r.Context(), in)
}

// swagger:route POST /swearjar/samples/commit-crimes Swearjar swearjarSamples
// @Summary Samples (masked text cards)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.SamplesInput true "Query"
// @Success 200 {object} domain.SamplesResp "ok"
// @Router /swearjar/samples/commit-crimes [post]
func (h *handlers) samples(r *stdhttp.Request, in domain.SamplesInput) (any, error) {
	return h.svc.Samples(r.Context(), in)
}

// swagger:route POST /swearjar/ratios/time Swearjar swearjarRatiosTime
// @Summary Ratios over time (hits vs utterances)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.RatiosTimeInput true "Query"
// @Success 200 {object} domain.RatiosTimeResp "ok"
// @Router /swearjar/ratios/time [post]
func (h *handlers) ratiosTime(r *stdhttp.Request, in domain.RatiosTimeInput) (any, error) {
	return h.svc.RatiosTime(r.Context(), in)
}

// swagger:route POST /swearjar/timeseries/severity Swearjar swearjarSeverityTimeseries
// @Summary Severity trend over time
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.SeverityTimeseriesInput true "Query"
// @Success 200 {object} domain.SeverityTimeseriesResp "ok"
// @Router /swearjar/timeseries/severity [post]
func (h *handlers) severityTimeseries(r *stdhttp.Request, in domain.SeverityTimeseriesInput) (any, error) {
	return h.svc.SeverityTimeseries(r.Context(), in)
}

// swagger:route POST /swearjar/spike/drivers Swearjar swearjarSpikeDrivers
// @Summary "Why this spike?" explainer and drivers
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.SpikeDriversInput true "Query"
// @Success 200 {object} domain.SpikeDriversResp "ok"
// @Router /swearjar/spike/drivers [post]
func (h *handlers) spikeDrivers(r *stdhttp.Request, in domain.SpikeDriversInput) (any, error) {
	return h.svc.SpikeDrivers(r.Context(), in)
}

// swagger:route POST /swearjar/leaders/actors Swearjar swearjarActorsLeaderboard
// @Summary Leaderboard of actors in window (paginated)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.ActorsLeaderboardInput true "Query"
// @Success 200 {object} domain.ActorsLeaderboardResp "ok"
// @Router /swearjar/leaders/actors [post]
func (h *handlers) actorsLeaderboard(r *stdhttp.Request, in domain.ActorsLeaderboardInput) (any, error) {
	return h.svc.ActorsLeaderboard(r.Context(), in)
}

// swagger:route POST /swearjar/leaders/repos Swearjar swearjarReposLeaderboard
// @Summary Leaderboard of repos in window (paginated; name revealed on opt-in only)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.ReposLeaderboardInput true "Query"
// @Success 200 {object} domain.ReposLeaderboardResp "ok"
// @Router /swearjar/leaders/repos [post]
func (h *handlers) reposLeaderboard(r *stdhttp.Request, in domain.ReposLeaderboardInput) (any, error) {
	return h.svc.ReposLeaderboard(r.Context(), in)
}

// swagger:route POST /swearjar/terms/suggest Swearjar swearjarTermsSuggest
// @Summary Term autocomplete/suggestions
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.TermsSuggestInput true "Query"
// @Success 200 {object} domain.TermsSuggestResp "ok"
// @Router /swearjar/terms/suggest [post]
func (h *handlers) termsSuggest(r *stdhttp.Request, in domain.TermsSuggestInput) (any, error) {
	return h.svc.TermsSuggest(r.Context(), in)
}

// swagger:route POST /swearjar/timeseries/hits-hourly Swearjar swearjarTimeseriesHourly
// @Summary Hourly hits series (for calendar/heatmap)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.TimeseriesHourlyInput true "Query"
// @Success 200 {object} domain.TimeseriesHourlyResp "ok"
// @Router /swearjar/timeseries/hits-hourly [post]
func (h *handlers) timeseriesHourly(r *stdhttp.Request, in domain.TimeseriesHourlyInput) (any, error) {
	return h.svc.TimeseriesHourly(r.Context(), in)
}

// swagger:route POST /swearjar/actors/overview Swearjar swearjarActorOverview
// @Summary Actor lens (timeseries + mix + top terms)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.ActorOverviewInput true "Query"
// @Success 200 {object} domain.ActorOverviewResp "ok"
// @Router /swearjar/actors/overview [post]
func (h *handlers) actorOverview(r *stdhttp.Request, in domain.ActorOverviewInput) (any, error) {
	return h.svc.ActorOverview(r.Context(), in)
}

// swagger:route POST /swearjar/crosstab/repo-actor Swearjar swearjarRepoActorCrosstab
// @Summary Repo x Actor cross-tab per day
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.RepoActorCrosstabInput true "Query"
// @Success 200 {object} domain.RepoActorCrosstabResp "ok"
// @Router /swearjar/crosstab/repo-actor [post]
func (h *handlers) repoActorCrosstab(r *stdhttp.Request, in domain.RepoActorCrosstabInput) (any, error) {
	return h.svc.RepoActorCrosstab(r.Context(), in)
}

// swagger:route POST /swearjar/kpi Swearjar swearjarKPIStrip
// @Summary KPI strip (headlines for the window; use today for homepage)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.KPIStripInput true "Query"
// @Success 200 {object} domain.KPIStripResp "ok"
// @Router /swearjar/kpi [post]
func (h *handlers) kpiStrip(r *stdhttp.Request, in domain.KPIStripInput) (any, error) {
	return h.svc.KPIStrip(r.Context(), in)
}

// swagger:route POST /swearjar/yearly/trends Swearjar swearjarYearlyTrends
// @Summary Yearly trends (seasonality, yearly mix, detver markers)
// @Tags Swearjar
// @Accept json
// @Produce json
// @Param payload body domain.YearlyTrendsInput true "Query"
// @Success 200 {object} domain.YearlyTrendsResp "ok"
// @Router /swearjar/yearly/trends [post]
func (h *handlers) yearlyTrends(r *stdhttp.Request, in domain.YearlyTrendsInput) (any, error) {
	return h.svc.YearlyTrends(r.Context(), in)
}
