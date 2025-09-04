// Package domain defines the types and interfaces for the hits service
package domain

import "time"

// Window defines a time range with a start (Since) and end (Until)
type Window struct {
	Since time.Time
	Until time.Time
}

// AfterKey is used for pagination in listing samples
type AfterKey struct {
	CreatedAt   time.Time
	UtteranceID string // uuid
}

// Filters for querying hits and samples
type Filters struct {
	RepoName   string
	Owner      string
	RepoID     *int64
	ActorLogin string
	ActorID    *int64
	LangCode   string
	Category   string
	Severity   string
	Version    *int
}

// HitWrite represents a hit to be written to the storage
type HitWrite struct {
	UtteranceID     string
	CreatedAt       time.Time
	Term            string
	Category        string // hit_category_enum
	Severity        string // hit_severity_enum
	SpanStart       int
	SpanEnd         int
	DetectorVersion int
	Source          string // source_enum (NOT NULL in hits)
	RepoName        string // NOT NULL in hits
	RepoHID         []byte
	ActorHID        []byte
	LangCode        string
}

// Sample represents a hit sample with associated metadata
type Sample struct {
	UtteranceID string
	CreatedAt   time.Time
	RepoName    string
	LangCode    *string
	Source      string
	Term        string
	Category    string
	Severity    string
	SpanStart   int
	SpanEnd     int
}

// AggByLangRow represents an aggregation of hits by language and day
type AggByLangRow struct {
	Day             time.Time
	LangCode        *string
	Hits            int64
	DetectorVersion int
}

// AggByRepoRow represents an aggregation of hits by repository
type AggByRepoRow struct {
	RepoName string
	Hits     int64
}

// AggByCategoryRow represents an aggregation of hits by category and severity
type AggByCategoryRow struct {
	Category string
	Severity string
	Hits     int64
}
