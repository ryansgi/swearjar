// Package domain holds the core business logic and data structures for backfill
package domain

import (
	"time"

	"swearjar/internal/adapters/ingest/gharchive"
)

// EventEnvelope re-exports the event envelope shape used by the extractor and reader
type EventEnvelope = gharchive.EventEnvelope

// HourRef is a reference to a specific hour
type HourRef struct{ Year, Month, Day, Hour int }

// UTC returns the UTC time corresponding to the HourRef
func (h HourRef) UTC() time.Time {
	return time.Date(h.Year, time.Month(h.Month), h.Day, h.Hour, 0, 0, 0, time.UTC)
}

// HourFinish is a reference to a specific hour for a completed backfill hour
type HourFinish struct {
	Status            string
	CacheHit          bool
	BytesUncompressed int64
	Events            int
	Utterances        int
	Inserted          int
	Deduped           int
	FetchMS           int
	ReadMS            int
	DBMS              int
	ElapsedMS         int
	ErrText           string
}

// Utterance is a single utterance extracted from an event
type Utterance struct {
	UtteranceID             string // synthetic UUID, deterministic from event payload
	EventType, Repo, Actor  string
	RepoID, ActorID         int64 // used only to derive HIDs; not persisted
	CreatedAt               time.Time
	Source, SourceDetail    string
	Ordinal                 int
	TextRaw, TextNormalized string
	LangCode                *string
}

// BackfillStatus represents the status of an ingest hour
type BackfillStatus string

const (
	// BackfillPending is the initial state
	BackfillPending BackfillStatus = "pending"

	// BackfillRunning is the running state
	BackfillRunning BackfillStatus = "running"

	// BackfillOK is the ok state
	BackfillOK BackfillStatus = "ok"

	// BackfillError is the error state
	BackfillError BackfillStatus = "error"
)
