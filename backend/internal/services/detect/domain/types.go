// Package domain defines the core types and interfaces for the detect service
package domain

import "time"

// Input controls the scan window and batching
type Input struct {
	Since    time.Time
	Until    time.Time
	PageSize int
	Workers  int
	Version  int
	DryRun   bool
}

// WriteInput is the minimal per-utterance payload detect needs to compute & persist hits
type WriteInput struct {
	UtteranceID string
	TextNorm    string
	// denorm copied onto hits for hot filters
	CreatedAt time.Time
	Source    string // source_enum (as text)
	RepoHID   []byte
	ActorHID  []byte
	LangCode  *string
}
