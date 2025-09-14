// Package domain defines core types and interfaces for utterances
package domain

import "time"

// AfterKey supports stable keyset pagination over (created_at, id)
type AfterKey struct {
	CreatedAt time.Time
	ID        string // uuid
}

// ListInput defines the input parameters for listing utterances
type ListInput struct {
	Since time.Time // inclusive
	Until time.Time // exclusive
	After AfterKey  // zero value = from start
	Limit int       // hard-capped in service

	// Optional filters (all ANDed)
	RepoName   string // "owner/name" if available
	Owner      string // derived from repo_name split; explicit filter
	RepoID     *int64
	ActorLogin string
	ActorID    *int64
	LangCode   string
}

// Row is the minimal utterance view shared across consumers
type Row struct {
	ID           string // uuid
	CreatedAt    time.Time
	RepoHID      []byte
	ActorHID     []byte
	Source       string // source_enum
	SourceDetail string
	LangCode     *string
	TextNorm     string // normalized; service guarantees non-empty when text exists
}
