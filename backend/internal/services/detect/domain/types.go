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

// Utterance is the minimal row we need to evaluate
type Utterance struct {
	ID             int64
	TextNormalized string // may be empty
	TextRaw        string // used to compute norm if TextNorm empty
	CommittedAt    time.Time
}

// Hit is the row we persist
type Hit struct {
	UtteranceID     int64
	Term            string
	Category        string
	Severity        int
	StartOffset     int
	EndOffset       int
	DetectorVersion int
}
