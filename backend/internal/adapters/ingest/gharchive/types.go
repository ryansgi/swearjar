package gharchive

import (
	"encoding/json"
	"fmt"
	"time"
)

// HourRef identifies a GH Archive hour (UTC).
type HourRef struct {
	Year  int
	Month int
	Day   int
	Hour  int
}

// NewHourRef creates an HourRef from a time.Time, converting to UTC
func NewHourRef(t time.Time) HourRef {
	ut := t.UTC()
	return HourRef{Year: ut.Year(), Month: int(ut.Month()), Day: ut.Day(), Hour: ut.Hour()}
}

// String returns the string representation of the HourRef in GH Archive format
func (h HourRef) String() string {
	// Matches GH Archive naming: YYYY-MM-DD-H.json.gz
	return fmtHour(h.Year, h.Month, h.Day, h.Hour)
}

// fmtHour formats the hour in GH Archive format: YYYY-MM-DD-H
func fmtHour(y, m, d, h int) string {
	return fmt.Sprintf("%04d-%02d-%02d-%d", y, m, d, h)
}

// EventEnvelope is the outer event format GH Archive stores per line.
// We keep only the fields we need for extraction; Payload is raw for type-specific decode.
type EventEnvelope struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Actor     Actor           `json:"actor"`
	Repo      Repo            `json:"repo"`
	Payload   json.RawMessage `json:"payload"`
	Public    bool            `json:"public"`
	CreatedAt time.Time       `json:"created_at"`
}

// Actor is the user who triggered the event
type Actor struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// Repo is the repository the event occurred in
type Repo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"` // owner/name
}

// Minimal payload structs we care about for text extraction.
// We intentionally model only the text-bearing parts.

type pushPayload struct {
	// GitHub's PushEvent payload
	Commits []struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
	} `json:"commits"`
}

type issuesPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"issue"`
}

type issueCommentPayload struct {
	Action  string `json:"action"`
	Comment struct {
		Body string `json:"body"`
	} `json:"comment"`
	Issue struct {
		Title string `json:"title"`
	} `json:"issue"`
}

type prPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"pull_request"`
}

type prReviewCommentPayload struct {
	Action  string `json:"action"`
	Comment struct {
		Body string `json:"body"`
	} `json:"comment"`
}

type commitCommentPayload struct {
	Action  string `json:"action"`
	Comment struct {
		Body string `json:"body"`
	} `json:"comment"`
}
