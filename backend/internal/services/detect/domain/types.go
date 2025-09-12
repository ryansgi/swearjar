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
	UtteranceID string    // required
	TextNorm    string    // required (already normalized upstream)
	CreatedAt   time.Time // required (for partitioning/TTL)
	Source      string    // "commit" | "issue" | "pr" | "comment"
	RepoHID     []byte    // len=32, FixedString(32)
	ActorHID    []byte    // len=32, FixedString(32)
	LangCode    *string   // optional (nil => unknown/auto)
}
