// Package domain holds DTOs for stats http and service contracts
package domain

// Query window and filters kept small and explicit
// Times are ISO8601 without timezone

// TimeRange defines a start and end time for queries (inclusive, YYYY-MM-DD UTC)
type TimeRange struct {
	Start string `json:"start" validate:"required,datetime=2006-01-02" example:"2025-08-01"`
	End   string `json:"end"   validate:"required,datetime=2006-01-02" example:"2025-08-31"`
}

// PageOpts defines pagination options for queries
type PageOpts struct {
	Cursor string `json:"cursor,omitempty" example:"eyJvZmZzZXQiOjEwMH0"`
	Limit  int    `json:"limit,omitempty"  validate:"omitempty,min=1,max=500" example:"100"`
}

// ByLangInput buckets daily by language (optional repo/lang/severity filters)
// NOTE: min_severity uses DB enum values: mild|strong|slur_masked
type ByLangInput struct {
	Range       TimeRange `json:"range"`
	Repo        string    `json:"repo,omitempty" validate:"omitempty,regex=^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$" example:"golang/go"` //nolint:lll
	Lang        string    `json:"lang,omitempty" validate:"omitempty,regex=^[A-Za-z-]{2,10}$" example:"en"`
	MinSeverity string    `json:"min_severity,omitempty" validate:"omitempty,oneof=mild strong slur_masked" example:"mild"` //nolint:lll
}

// ByLangRow is a daily bucket of hits and utterances by language
type ByLangRow struct {
	Day        string `json:"day"        example:"2025-08-01"`
	Lang       string `json:"lang"       example:"en"`
	Hits       int64  `json:"hits"       example:"42"`
	Utterances int64  `json:"utterances" example:"420"`
}

// ByRepoInput top repos in time window (optional lang filter)
type ByRepoInput struct {
	Range TimeRange `json:"range"`
	Lang  string    `json:"lang,omitempty" validate:"omitempty,regex=^[A-Za-z-]{2,10}$" example:"en"`
}

// ByRepoRow is a repo and its hit count
type ByRepoRow struct {
	Repo string `json:"repo" example:"golang/go"`
	Hits int64  `json:"hits" example:"7"`
}

// ByCategoryInput buckets by category and severity (optional repo filter)
type ByCategoryInput struct {
	Range TimeRange `json:"range"`
	Repo  string    `json:"repo,omitempty" validate:"omitempty,regex=^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$" example:"golang/go"` //nolint:lll
}

// ByCategoryRow is a bucket of hits by category and severity
type ByCategoryRow struct {
	Category string `json:"category" example:"tooling_rage"`
	Severity string `json:"severity" example:"mild"`
	Hits     int64  `json:"hits"     example:"9"`
}
