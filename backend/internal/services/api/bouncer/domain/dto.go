// Package domain holds DTOs for bouncer http and service contracts
package domain

// IssueInput asks the service to mint and record a challenge for a subject and scope
type IssueInput struct {
	SubjectType SubjectType `json:"subject_type" validate:"required,oneof=repo actor" example:"repo"`
	SubjectKey  string      `json:"subject_key"  validate:"required,min=1,max=200,printascii" example:"golang/go"`
	Scope       Scope       `json:"scope"        validate:"required,oneof=allow deny" example:"allow"`
}

// IssueOutput returns the issued hash and suggested artifact filenames
type IssueOutput struct {
	Hash         string `json:"hash" example:"b4b1f6c9a3e44d2f8a4f5b6c..."`
	RepoFilename string `json:"repo_filename,omitempty" example:".b4b1f6c9a3e44d2f8a4f5b6c....txt"`
	GistFilename string `json:"gist_filename,omitempty" example:"b4b1f6c9a3e44d2f8a4f5b6c....txt"`
	Instructions string `json:"instructions" example:"create the file on the default branch or as a gist then call reverify"` //nolint:lll
}

// ReverifyInput asks the service to re check the artifact for this subject
type ReverifyInput struct {
	SubjectType SubjectType `json:"subject_type" validate:"required,oneof=repo actor" example:"actor"`
	SubjectKey  string      `json:"subject_key"  validate:"required,min=1,max=200,printascii" example:"octocat"`
}

// StatusQuery retrieves current effective consent and evidence
// status is POST in this module so we bind from json not form
type StatusQuery struct {
	SubjectType SubjectType `json:"subject_type" validate:"required,oneof=repo actor" example:"repo"`
	SubjectKey  string      `json:"subject_key"  validate:"required,min=1,max=200,printascii" example:"golang/go"`
}

// StatusRow is the service view of effective consent and freshness
type StatusRow struct {
	State          EffectiveState `json:"state" example:"allow"`
	SinceUnix      int64          `json:"since_unix,omitempty"  example:"1725731200"`
	EvidenceKind   EvidenceKind   `json:"evidence_kind,omitempty" example:"repo_file"`
	EvidenceURL    string         `json:"evidence_url,omitempty" example:"https://raw.githubusercontent.com/golang/go/HEAD/.b4b1f6....txt"` //nolint:lll
	Hash           string         `json:"hash,omitempty"          example:"b4b1f6c9a3e44d2f8a4f5b6c..."`
	LastVerifiedAt int64          `json:"last_verified_unix,omitempty" example:"1725734800"`
	Staleness      string         `json:"staleness,omitempty"    validate:"omitempty,oneof=fresh stale revocation_pending" example:"fresh"` //nolint:lll
}

// LatestChallenge is a recent challenge row
type LatestChallenge struct {
	Action       string // 'opt_in'|'opt_out'
	EvidenceKind string // 'repo_file'|'actor_gist'
	ArtifactHint string // ".<hash>.txt" or "<hash>.txt"
	Hash         string
	IssuedAtUnix int64
}
