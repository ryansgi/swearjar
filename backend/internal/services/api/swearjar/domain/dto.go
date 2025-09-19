// Package domain holds DTOs for the Swearjar HTTP and service contracts
package domain

// Query windows and filters are small and explicit
// Times use ISO8601 without timezone in UTC

// TimeRange defines an inclusive start and end date in YYYY-MM-DD UTC
type TimeRange struct {
	Start string `json:"start" validate:"required,datetime=2006-01-02" example:"2025-08-01"`
	End   string `json:"end"   validate:"required,datetime=2006-01-02" example:"2025-08-31"`
}

// PageOpts defines cursor pagination options for ranked lists
type PageOpts struct {
	Cursor string `json:"cursor,omitempty" example:"eyJvZmZzZXQiOjEwMH0"`
	Limit  int    `json:"limit,omitempty"  validate:"omitempty,min=1,max=200" example:"100"`
}

// GlobalOptions is a shared bundle of filters and options for queries
// Embed this in endpoint specific inputs to keep shapes consistent
type GlobalOptions struct {
	Range        TimeRange `json:"range"`
	Interval     string    `json:"interval,omitempty"  validate:"omitempty,oneof=auto hour day week month" example:"day"`
	TZ           string    `json:"tz,omitempty"        validate:"omitempty,printascii,max=64" example:"UTC"`
	Normalize    string    `json:"normalize,omitempty" validate:"omitempty,oneof=none per_utterance" example:"none"`
	LangReliable *bool     `json:"lang_reliable,omitempty" example:"true"`

	// Filters accept one or many values
	// Swagger prefers single element examples for slices
	DetVer    []int    `json:"detver,omitempty"     example:"1"`
	RepoHIDs  []string `json:"repo_hids,omitempty"  validate:"omitempty,dive,hexadecimal,len=64" example:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` //nolint:lll
	ActorHIDs []string `json:"actor_hids,omitempty" validate:"omitempty,dive,hexadecimal,len=64" example:"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"` //nolint:lll
	NLLangs   []string `json:"nl_langs,omitempty"   validate:"omitempty,dive" example:"en"`
	CodeLangs []string `json:"code_langs,omitempty" validate:"omitempty,dive,printascii" example:"JavaScript"`

	// Pagination options used by ranked endpoints
	Page PageOpts `json:"page,omitempty"`
}

// KPIStripInput carries shared options for the KPI strip
type KPIStripInput struct {
	GlobalOptions
}

// KPIStripResp returns totals for the window
type KPIStripResp struct {
	// Echo back the window start (YYYY-MM-DD) for labeling on UI
	Day                 string  `json:"day"                    example:"2025-09-18"`
	Hits                int64   `json:"hits"                   example:"6157"`
	OffendingUtterances int64   `json:"offending_utterances"   example:"4123"`
	Repos               int64   `json:"repos"                  example:"198"`
	Actors              int64   `json:"actors"                 example:"743"`
	AllUtterances       int64   `json:"all_utterances,omitempty" example:"1381384"`
	Intensity           float64 `json:"intensity,omitempty"      example:"1.49"`   // hits / offending_utterances
	Coverage            float64 `json:"coverage,omitempty"       example:"0.3"`    // offending_utterances / all_utter.
	Rarity              float64 `json:"rarity,omitempty"         example:"0.0045"` // hits / all_utterances
}

// TimeseriesHitsInput carries shared options for the hits series
type TimeseriesHitsInput struct {
	GlobalOptions
}

// TimeseriesPoint represents one bucket in the hits timeseries.
//
// Semantics:
//   - Hits: total hits (matches) in the bucket
//   - OffendingUtterances: distinct utterances that had > 1 hit
//   - AllUtterances: total utterances observed (from utt_hour_agg)
//   - Intensity = Hits / OffendingUtterances            (avg hits per offending utterance)
//   - Coverage  = OffendingUtterances / AllUtterances   (share of utterances that were flagged)
//   - Rarity    = Hits / AllUtterances                  (overall prevalence of hits)
type TimeseriesPoint struct {
	T                   string  `json:"t"                    example:"2025-08-01"`
	Hits                int64   `json:"hits"                 example:"6157"`
	OffendingUtterances int64   `json:"offending_utterances" example:"4123"`
	AllUtterances       int64   `json:"all_utterances,omitempty"  example:"1381384"`
	Intensity           float64 `json:"intensity,omitempty"  example:"1.493"`
	Coverage            float64 `json:"coverage,omitempty"   example:"0.002985"`
	Rarity              float64 `json:"rarity,omitempty"     example:"0.004457"`
}

// TimeseriesHitsResp is the response for the hits timeseries
type TimeseriesHitsResp struct {
	Interval string            `json:"interval" example:"day"`
	Series   []TimeseriesPoint `json:"series"`
}

// HeatmapWeeklyInput carries shared options for the weekly heatmap
type HeatmapWeeklyInput struct {
	GlobalOptions
}

// HeatmapCell defines a cell in the weekly heatmap
type HeatmapCell struct {
	DOW        int     `json:"dow"        example:"1"`  // 0..6
	Hour       int     `json:"hour"       example:"13"` // 0..23
	Hits       int64   `json:"hits"       example:"7"`
	Utterances int64   `json:"utterances" example:"420"`
	Ratio      float64 `json:"ratio,omitempty" example:"0.0167"`
}

// HeatmapWeeklyResp is the response for the weekly heatmap
type HeatmapWeeklyResp struct {
	Z    string        `json:"z" example:"hits"`
	Grid []HeatmapCell `json:"grid"`
}

// LangBarsInput carries shared options for natural language bars
type LangBarsInput struct {
	GlobalOptions
}

// LangBarItem is a single language row in the ranked list
type LangBarItem struct {
	Lang       string  `json:"lang"        example:"en"`
	Hits       int64   `json:"hits"        example:"15234"`
	Utterances int64   `json:"utterances"  example:"3650000"`
	Ratio      float64 `json:"ratio,omitempty" example:"0.0042"`
}

// LangBarsResp is the response for natural language bars
type LangBarsResp struct {
	Items     []LangBarItem `json:"items"`
	TotalHits int64         `json:"total_hits" example:"20000"`
}

// TimeseriesDetverInput compares detector versions over time
type TimeseriesDetverInput struct{ GlobalOptions }

// TimeseriesDetverPoint is a bucket for a detector version series
type TimeseriesDetverPoint struct {
	T    string `json:"t"     example:"2025-07-28"`
	Hits int64  `json:"hits"  example:"1040"`
}

// TimeseriesDetverSeries holds points for one detector version
type TimeseriesDetverSeries struct {
	DetVer int                     `json:"detver" example:"2"`
	Points []TimeseriesDetverPoint `json:"points"`
}

// TimeseriesDetverResp is the response for detector version comparison
type TimeseriesDetverResp struct {
	Interval string                   `json:"interval" example:"week"`
	Series   []TimeseriesDetverSeries `json:"series"`
}

// CodeLangBarsInput carries shared options for code language bars
type CodeLangBarsInput struct{ GlobalOptions }

// CodeLangBarItem is a single code language row in the ranked list
type CodeLangBarItem struct {
	CodeLang string  `json:"code_lang" example:"JavaScript"`
	Hits     int64   `json:"hits"      example:"8900"`
	Repos    int64   `json:"repos"     example:"1200"`
	Ratio    float64 `json:"ratio,omitempty" example:"0.0042"`
}

// CodeLangBarsResp is the response for code language bars
type CodeLangBarsResp struct {
	Items []CodeLangBarItem `json:"items"`
}

// CategoriesStackInput carries options for category and severity mix
type CategoriesStackInput struct{ GlobalOptions }

// CategoryStackItem is a stacked counts row for a category
type CategoryStackItem struct {
	Label      string `json:"label"        example:"bot_rage"`
	Mild       int64  `json:"mild"         example:"1200"`
	Strong     int64  `json:"strong"       example:"300"`
	SlurMasked int64  `json:"slur_masked"  example:"25"`
	Total      int64  `json:"total"        example:"1525"`
}

// CategoriesStackResp is the response for category and severity mix
type CategoriesStackResp struct {
	Stack     []CategoryStackItem `json:"stack"`
	TotalHits int64               `json:"total_hits" example:"20000"`
}

// TopTermsInput carries options for ranked top terms
type TopTermsInput struct{ GlobalOptions }

// TopTermItem is a ranked term with counts and optional ratio
type TopTermItem struct {
	Term       string  `json:"term"        example:"fuck"`
	TermID     uint64  `json:"term_id"     example:"123456789"`
	Hits       int64   `json:"hits"        example:"5400"`
	Utterances int64   `json:"utterances,omitempty" example:"420000"`
	Ratio      float64 `json:"ratio,omitempty" example:"0.0129"`
}

// TopTermsResp is the response for ranked top terms
type TopTermsResp struct {
	Items      []TopTermItem `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty" example:"eyJvZmZzZXQiOjUwLCJ0ZXJtIjoiZnVjayJ9"`
}

// TermTimelineInput requests timelines for one or many terms
type TermTimelineInput struct {
	GlobalOptions
	Terms []string `json:"terms" validate:"required,min=1,dive,printascii" example:"fuck"`
}

// TermTimelinePoint is a bucket in a single term timeline
type TermTimelinePoint struct {
	T    string `json:"t"    example:"2025-01-01"`
	Hits int64  `json:"hits" example:"420"`
}

// TermTimelineSeries is the timeline for one term
type TermTimelineSeries struct {
	Term   string              `json:"term"   example:"fuck"`
	Points []TermTimelinePoint `json:"points"`
}

// TermTimelineResp is the response for term timelines
type TermTimelineResp struct {
	Interval string               `json:"interval" example:"month"`
	Terms    []TermTimelineSeries `json:"terms"`
}

// TargetsMixInput carries options for target mix
type TargetsMixInput struct{ GlobalOptions }

// TargetMixItem is a single target with hits
type TargetMixItem struct {
	Target string `json:"target" example:"bot"`
	Hits   int64  `json:"hits"   example:"2600"`
}

// TargetsMixResp is the response for target mix
type TargetsMixResp struct {
	Items []TargetMixItem `json:"items"`
}

// TermsMatrixInput requests a term by language matrix
type TermsMatrixInput struct {
	GlobalOptions
	Terms []string `json:"terms" validate:"required,min=1,dive,printascii" example:"fuck"`
}

// TermsMatrixLang lists a language used in the matrix
type TermsMatrixLang struct {
	Lang string `json:"lang" example:"en"`
}

// TermsMatrixCell is a single term by language cell
type TermsMatrixCell struct {
	Term string `json:"term"  example:"fuck"`
	Lang string `json:"lang"  example:"en"`
	Hits int64  `json:"hits"  example:"5100"`
}

// TermsMatrixResp is the response for the term by language matrix
type TermsMatrixResp struct {
	Terms []string          `json:"terms"`
	Langs []TermsMatrixLang `json:"langs"`
	Cells []TermsMatrixCell `json:"cells"`
}

// RepoOverviewInput focuses analytics on a single repository
type RepoOverviewInput struct {
	GlobalOptions
	RepoHID string `json:"repo_hid" validate:"required,hexadecimal,len=64" example:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"` //nolint:lll
}

// RepoRef identifies a repository with stable id and labels
type RepoRef struct {
	HID       string  `json:"hid"        example:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Label     string  `json:"label"      example:"abc…def"`
	NameOptIn *string `json:"name_optin,omitempty" example:"owner/name"`
}

// RepoOverviewSeriesPoint is a bucket for repo overview series
type RepoOverviewSeriesPoint struct {
	T          string  `json:"t" example:"2025-08-01"`
	Hits       int64   `json:"hits" example:"10"`
	Utterances int64   `json:"utterances,omitempty" example:"420"`
	Ratio      float64 `json:"ratio,omitempty" example:"0.0238"`
}

// RepoOverviewResp is the response for the repo lens
type RepoOverviewResp struct {
	Repo     RepoRef                   `json:"repo"`
	Series   []RepoOverviewSeriesPoint `json:"series"`
	Mix      map[string]int64          `json:"mix"` // category to count
	TopTerms []TopTermItem             `json:"top_terms"`
}

// SamplesInput fetches example utterances and hits
type SamplesInput struct {
	GlobalOptions
	Term  string `json:"term,omitempty" validate:"omitempty,printascii" example:"fuck"`
	Limit int    `json:"limit,omitempty" validate:"omitempty,min=1,max=200" example:"20"`
}

// SampleRepo identifies a repository in a sample
type SampleRepo struct {
	HID       string  `json:"hid" example:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Label     string  `json:"label" example:"abc…def"`
	NameOptIn *string `json:"name_optin,omitempty"`
}

// SampleActor identifies an actor in a sample
type SampleActor struct {
	HID        string  `json:"hid" example:"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"`
	Label      string  `json:"label" example:"xyz…pqr"`
	LoginOptIn *string `json:"login_optin,omitempty"`
}

// SampleHit summarizes one detected hit in a sample
// For Swagger v2 avoid examples on fixed size arrays
type SampleHit struct {
	Term     string `json:"term"  example:"fuck"`
	Span     [2]int `json:"span"`
	Severity string `json:"severity" example:"mild"`
}

// SampleItem is a single sample with context and hits
type SampleItem struct {
	UtteranceID string      `json:"utterance_id" example:"00000000-0000-0000-0000-000000000000"`
	CreatedAt   string      `json:"created_at"   example:"2025-08-01T12:34:56Z"`
	Source      string      `json:"source"       example:"commit"`
	Repo        SampleRepo  `json:"repo"`
	Actor       SampleActor `json:"actor"`
	TextMasked  string      `json:"text_masked"  example:"f*** this build again"`
	Hits        []SampleHit `json:"hits"`
	DetVer      int         `json:"detver" example:"1"`
}

// SamplesResp is the response for samples
type SamplesResp struct {
	Items      []SampleItem `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// RatiosTimeInput carries options for ratios over time
type RatiosTimeInput struct{ GlobalOptions }

// RatiosTimePoint is a bucket with hits utterances and ratio
type RatiosTimePoint struct {
	T          string  `json:"t" example:"2025-07-01"`
	Hits       int64   `json:"hits" example:"1200"`
	Utterances int64   `json:"utterances" example:"520000"`
	Ratio      float64 `json:"ratio" example:"0.0023"`
}

// RatiosTimeResp is the response for ratios over time
type RatiosTimeResp struct {
	Interval string            `json:"interval" example:"month"`
	Points   []RatiosTimePoint `json:"points"`
}

// SeverityTimeseriesInput carries options for severity trend
type SeverityTimeseriesInput struct{ GlobalOptions }

// SeveritySeries is a timeseries for one severity level
type SeveritySeries struct {
	Severity string              `json:"severity" example:"mild"`
	Points   []TermTimelinePoint `json:"points"`
}

// SeverityTimeseriesResp is the response for severity trend
type SeverityTimeseriesResp struct {
	Series []SeveritySeries `json:"series"`
}

// SpikeDriversInput requests drivers for a spike window
type SpikeDriversInput struct {
	GlobalOptions
	T        string `json:"t"       validate:"required,datetime=2006-01-02" example:"2025-08-01"`
	Window   string `json:"window"  validate:"required,printascii" example:"7d"`
	Baseline string `json:"baseline,omitempty" validate:"omitempty,oneof=prev rolling" example:"prev"`
}

// SpikeDelta summarizes the change vs baseline
type SpikeDelta struct {
	Hits  int64   `json:"hits"  example:"420"`
	Ratio float64 `json:"ratio" example:"0.0003"`
}

// SpikeDriverItem is a driver key with hits delta
type SpikeDriverItem struct {
	Key       string `json:"key"        example:"dependabot"`
	HitsDelta int64  `json:"hits_delta" example:"120"`
}

// SpikeDriversResp is the response for spike drivers
type SpikeDriversResp struct {
	T       string     `json:"t"`
	Delta   SpikeDelta `json:"delta"`
	Drivers struct {
		Terms    []SpikeDriverItem `json:"terms"`
		Repos    []SpikeDriverItem `json:"repos"`
		CodeLang []SpikeDriverItem `json:"code_lang"`
		Actors   []SpikeDriverItem `json:"actors"`
	} `json:"drivers"`
}

// ActorsLeaderboardInput carries options for actors leaderboard
type ActorsLeaderboardInput struct{ GlobalOptions }

// ActorsLeaderboardRow is a ranked row for an actor
type ActorsLeaderboardRow struct {
	ActorHID string  `json:"actor_hid" example:"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"`
	Label    string  `json:"label"     example:"abc…def"`
	Hits     int64   `json:"hits"      example:"420"`
	Ratio    float64 `json:"ratio,omitempty" example:"0.0012"`
}

// ActorsLeaderboardResp is the response for actors leaderboard
type ActorsLeaderboardResp struct {
	Items    []ActorsLeaderboardRow `json:"items"`
	NextPage string                 `json:"next_page,omitempty" example:"eyJvZmZzZXQiOjEwMH0"`
}

// ReposLeaderboardInput carries options for repos leaderboard
type ReposLeaderboardInput struct{ GlobalOptions }

// ReposLeaderboardRow is a ranked row for a repository
type ReposLeaderboardRow struct {
	RepoHID string  `json:"repo_hid" example:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Label   string  `json:"label"    example:"abc…def"`
	Hits    int64   `json:"hits"     example:"900"`
	Ratio   float64 `json:"ratio,omitempty" example:"0.0021"`
}

// ReposLeaderboardResp is the response for repos leaderboard
type ReposLeaderboardResp struct {
	Items    []ReposLeaderboardRow `json:"items"`
	NextPage string                `json:"next_page,omitempty" example:"eyJvZmZzZXQiOjEwMH0"`
}

// TermsSuggestInput carries options for term autocomplete
type TermsSuggestInput struct {
	GlobalOptions
	Query string `json:"query" validate:"required,min=1,max=64" example:"dep"`
}

// TermsSuggestResp is the response for term autocomplete
type TermsSuggestResp struct {
	Terms []string `json:"terms" example:"dependabot"`
}

// DetectorMetaResp reports detector versions and tags
type DetectorMetaResp struct {
	Current int      `json:"current" example:"2"`
	Known   []int    `json:"known"   example:"1"`
	Notes   string   `json:"notes,omitempty" example:"rolling backfill in progress"`
	Tags    []string `json:"tags,omitempty"  example:"stable"`
}

// TimeseriesHourlyInput carries options for hourly series
type TimeseriesHourlyInput struct{ GlobalOptions }

// TimeseriesHourlyPoint is a single hour bucket
type TimeseriesHourlyPoint struct {
	T    string `json:"t"    example:"2025-08-01T13:00:00Z"`
	Hits int64  `json:"hits" example:"12"`
}

// TimeseriesHourlyResp is the response for hourly series
type TimeseriesHourlyResp struct {
	Series []TimeseriesHourlyPoint `json:"series"`
}

// ActorOverviewInput focuses analytics on a single actor
type ActorOverviewInput struct {
	GlobalOptions
	ActorHID string `json:"actor_hid" validate:"required,hexadecimal,len=64" example:"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"` //nolint:lll
}

// ActorOverviewResp is the response for the actor lens
type ActorOverviewResp struct {
	Actor struct {
		HID      string  `json:"hid" example:"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"`
		Label    string  `json:"label" example:"xyz…pqr"`
		LoginOpt *string `json:"login_optin,omitempty" example:"octocat"`
	} `json:"actor"`
	Series []TimeseriesPoint `json:"series"`
	Mix    map[string]int64  `json:"mix"` // category to hits
	Top    []struct {
		Term string `json:"term" example:"fuck"`
		Hits int64  `json:"hits" example:"4"`
	} `json:"top_terms"`
}

// RepoActorCrosstabInput carries options for repo by actor cross tab
type RepoActorCrosstabInput struct{ GlobalOptions }

// RepoActorCell is a single day by repo by actor cell
type RepoActorCell struct {
	Day      string `json:"day"      example:"2025-08-01"`
	RepoHID  string `json:"repo_hid" example:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	ActorHID string `json:"actor_hid" example:"abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"`
	Hits     int64  `json:"hits"     example:"3"`
}

// RepoActorCrosstabResp is the response for the cross tab
type RepoActorCrosstabResp struct {
	Cells []RepoActorCell `json:"cells"`
}
