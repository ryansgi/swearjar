// Package domain defines the types and interfaces for the hits service
package domain

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

// Window defines a time range with a start (Since) and end (Until)
type Window struct {
	Since time.Time
	Until time.Time
}
type (
	Category = string
	Severity = string
)

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
	UtteranceID string
	CreatedAt   time.Time
	Source      string
	RepoHID     []byte
	ActorHID    []byte
	LangCode    string // nil => NULL in DB
	Term        string
	Category    Category
	Severity    Severity
	// Category        string // hit_category_enum
	// Severity        string // hit_severity_enum
	SpanStart       int
	SpanEnd         int
	DetectorVersion int
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

// DeterministicUUID builds a stable UUID for a hit based on fields that uniquely identify a hit
func (h HitWrite) DeterministicUUID() uuid.UUID {
	var u uuid.UUID
	if uu, err := uuid.Parse(h.UtteranceID); err == nil {
		u = uu
	}

	d := sha256.New()
	d.Write([]byte("hit"))
	d.Write(u[:])
	d.Write([]byte{0x1f})

	// Identity within the utterance
	d.Write([]byte(h.Term))
	d.Write([]byte{0})
	d.Write([]byte(h.Category))
	d.Write([]byte{0})
	d.Write([]byte(h.Severity))

	var span [8]byte
	binary.LittleEndian.PutUint32(span[0:], uint32(h.SpanStart))
	binary.LittleEndian.PutUint32(span[4:], uint32(h.SpanEnd))
	d.Write(span[:])

	sum := d.Sum(nil)
	var out [16]byte
	copy(out[:], sum[:16])
	out[6] = (out[6] & 0x0f) | 0x50
	out[8] = (out[8] & 0x3f) | 0x80
	id, _ := uuid.FromBytes(out[:])
	return id
}
