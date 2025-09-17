// Package domain defines shared types for the swearjar API surface
package domain

import "time"

// QueryOptions are the global filters we'll pass into repo/service funcs
type QueryOptions struct {
	From     *time.Time
	To       *time.Time
	Interval string // "auto" | "hour" | "day" | "week" | "month"
	TZ       string // IANA tz, default "UTC"

	DetVer    []int
	RepoHIDs  [][]byte
	ActorHIDs [][]byte
	NLLangs   []string
	CodeLangs []string

	Normalize    string // "none" | "per_utterance"
	LangReliable *bool
	Limit        int
	Cursor       string
}
