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
	RepoID        int64
	Priority      int16
	Attempts      int
	NextAttemptAt time.Time
}

// ActorJob represents a single actor queue item
type ActorJob struct {
	ActorID       int64
	Priority      int16
	Attempts      int
	NextAttemptAt time.Time
}

// RepositoryRecord is the repository facts payload
type RepositoryRecord struct {
	RepoID        int64
	FullName      *string
	DefaultBranch *string
	PrimaryLang   *string
	Languages     any // driver encodes map[string]int64 to jsonb
	Stars         *int
	Forks         *int
	Subscribers   *int
	OpenIssues    *int
	LicenseKey    *string
	IsFork        *bool
	PushedAt      *time.Time
	UpdatedAt     *time.Time
	NextRefreshAt *time.Time
	ETag          *string
	APIURL        *string
}

// ActorRecord is the actor facts payload
type ActorRecord struct {
	ActorID       int64
	Login         *string
	Name          *string
	Type          *string
	Company       *string
	Location      *string
	Bio           *string
	Blog          *string
	Twitter       *string
	Followers     *int
	Following     *int
	PublicRepos   *int
	PublicGists   *int
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
	NextRefreshAt *time.Time
	ETag          *string
	APIURL        *string
	OptedInAt     *time.Time
}
