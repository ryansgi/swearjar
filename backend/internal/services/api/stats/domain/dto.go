// Package domain holds DTOs for stats http and service contracts
package domain

// Query window and filters kept small and explicit
// Times are ISO8601 without timezone

// TimeRange defines a start and end time for queries
type TimeRange struct {
	Start string `json:"start" validate:"required,datetime=2006-01-02" example:"2025-08-01"`
	End   string `json:"end" validate:"required,datetime=2006-01-02" example:"2025-08-31"`
}

// PageOpts defines pagination options
type PageOpts struct {
	Cursor string `json:"cursor,omitempty" example:"eyJvZmZzZXQiOjEwMH0"`
	Limit  int    `json:"limit,omitempty" validate:"omitempty,min=1,max=500" example:"100"`
}

// ByLangInput buckets samples by programming language
type ByLangInput struct {
	Range TimeRange `json:"range"`
	// optional filters
	Repo   string `json:"repo,omitempty" validate:"omitempty,min=1,max=200" example:"golang/go"`
	Lang   string `json:"lang,omitempty" validate:"omitempty,alpha" example:"en"`
	MinSev string `json:"min_severity,omitempty" validate:"omitempty,oneof=info low medium high" example:"low"`
}

// ByLangRow represents a row in the ByLang output
type ByLangRow struct {
	Day       string `json:"day" example:"2025-08-01"`
	Lang      string `json:"lang" example:"en"`
	Hits      int64  `json:"hits" example:"42"`
	Utterings int64  `json:"utterances" example:"420"`
}

// Repo buckets

// ByRepoInput is the input for repo buckets
type ByRepoInput struct {
	Range TimeRange `json:"range"`
	Lang  string    `json:"lang,omitempty" validate:"omitempty,alpha" example:"en"`
}

// ByRepoRow represents a row in the ByRepo output
type ByRepoRow struct {
	Repo string `json:"repo" example:"golang/go"`
	Hits int64  `json:"hits" example:"7"`
}

// Category buckets

// ByCategoryInput is the input for category buckets
type ByCategoryInput struct {
	Range TimeRange `json:"range"`
	Repo  string    `json:"repo,omitempty" validate:"omitempty,min=1,max=200" example:"golang/go"`
}

// ByCategoryRow represents a row in the ByCategory output
type ByCategoryRow struct {
	Category string `json:"category" example:"bot-directed"`
	Severity string `json:"severity" example:"medium"`
	Hits     int64  `json:"hits" example:"9"`
}
