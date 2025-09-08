package domain

import "time"

// SeedRange defines the backfill window for seeding queues from utterances
type SeedRange struct {
	Since time.Time // inclusive
	Until time.Time // optional; zero = open-ended
	Limit int       // 0 = unlimited
}

// RefreshParams controls a due-refresh sweep (based on next_refresh_at, etc)
type RefreshParams struct {
	Since time.Time // optional filter (e.g., pushed_at >= since)
	Until time.Time // optional filter
	Limit int       // 0 = unlimited
}

// LangWindow scopes language aggregations (used for actor rollups)
type LangWindow struct {
	Since time.Time // zero => unbounded
	Until time.Time // zero => now
}

// Job represents a single repo queue item
type Job struct {
	RepoHID       []byte
	Priority      int16
	Attempts      int
	NextAttemptAt time.Time
}

// ActorJob represents a single actor queue item
type ActorJob struct {
	ActorHID      []byte
	Priority      int16
	Attempts      int
	NextAttemptAt time.Time
}

// RepositoryTombstone captures terminal error state
type RepositoryTombstone struct {
	RepoID int64  // input only
	Code   int    // e.g. 404, 410, 451
	Reason string // "not_found" | "gone" | "legal"
}

// ActorTombstone captures terminal error state
type ActorTombstone struct {
	ActorID int64  // input only
	Code    int    // e.g. 404, 410, 451
	Reason  string // "not_found" | "gone" | "legal"
}

// RepositoryRecord is the repository facts payload
type RepositoryRecord struct {
	RepoID                                int64   // input only
	FullName                              *string // PII: only set when opted-in
	DefaultBranch                         *string
	PrimaryLang                           *string
	Languages                             any // jsonb map[string]int64
	Stars, Forks, Subscribers, OpenIssues *int
	LicenseKey                            *string
	IsFork                                *bool
	PushedAt, UpdatedAt, NextRefreshAt    *time.Time
	ETag, APIURL                          *string
}

// ActorRecord is the actor facts payload
type ActorRecord struct {
	ActorID                                        int64   // input only
	Login, Name                                    *string // PII: only set when opted-in
	Type, Company, Location, Bio, Blog, Twitter    *string
	Followers, Following, PublicRepos, PublicGists *int
	CreatedAt, UpdatedAt, NextRefreshAt            *time.Time
	ETag, APIURL                                   *string
}
