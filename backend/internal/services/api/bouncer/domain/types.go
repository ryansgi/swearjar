// Package domain holds bouncer core types independent of transport or storage
package domain

import "time"

// SubjectType marks what entity is giving consent
type SubjectType string

const (
	// SubjectRepo is a GitHub repository
	SubjectRepo SubjectType = "repo"

	// SubjectActor is a GitHub user or org
	SubjectActor SubjectType = "actor"
)

// Scope is the intent of a consent action
type Scope string

const (
	// ScopeAllow means opt in for de masking
	ScopeAllow Scope = "allow"

	// ScopeDeny means opt out and block all usage
	ScopeDeny Scope = "deny"
)

// EffectiveState is the resolved consent state at this moment
type EffectiveState string

const (
	// StateNone means no consent exists
	StateNone EffectiveState = "none"

	// StateAllow means opt in is active
	StateAllow EffectiveState = "allow"

	// StateDeny means opt out is active
	StateDeny EffectiveState = "deny"

	// StatePending means artifact disappeared but still inside grace
	StatePending EffectiveState = "revocation_pending"
)

// EvidenceKind is how we verified proof of control
type EvidenceKind string

const (
	// EvidenceRepo is a dotfile committed at repo root in default branch
	EvidenceRepo EvidenceKind = "repo_file"

	// EvidenceGist is a public gist file owned by the actor
	EvidenceGist EvidenceKind = "gist_file"
)

// VerificationJob is a leased unit of work returned to the worker
type VerificationJob struct {
	JobID         string
	Principal     string
	Resource      string
	PrincipalHID  []byte
	ChallengeHash string

	Attempts   int
	LastStatus *int
	LastURL    string

	ETagBranch *string
	ETagFile   *string
	ETagGists  *string

	RateResetAt   *time.Time
	NextAttemptAt time.Time
	LeaseExpires  time.Time
	LeasedBy      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
