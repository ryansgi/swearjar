// Package domain defines the public ports for the hallmonitor service
package domain

import (
	"context"
	"time"
)

// WorkerPort runs the long-lived queue processors (repos + actors)
type WorkerPort interface {
	Run(ctx context.Context) error
}

// One-shot operations

// SeederPort seeds repo/actor queues retroactively from historical utterances
type SeederPort interface {
	SeedFromUtterances(ctx context.Context, r SeedRange) error
}

// RefresherPort finds stale catalog entries and (re)enqueues them for refresh
type RefresherPort interface {
	RefreshDue(ctx context.Context, p RefreshParams) error
}

// SignalsPort is a non-blocking enqueue surface for ingest/backfill.
// Call these when we "see" a repo/actor in event flow; the worker does the rest
type SignalsPort interface {
	SeenRepo(ctx context.Context, repoID int64, fullName string, seenAt time.Time) error
	SeenActor(ctx context.Context, actorID int64, login string, seenAt time.Time) error
}

// Read-side helpers

// ReaderPort provides language lookups for repos and actors.
// Repo language reads are direct (from repositories table).
// Actor language reads are derived from activity against repos over a window
type ReaderPort interface {
	// Repositories
	PrimaryLanguageOfRepo(ctx context.Context, repoID int64) (lang string, ok bool, err error)
	LanguagesOfRepo(ctx context.Context, repoID int64) (byBytes map[string]int64, ok bool, err error)

	// Actors (by numeric id) - internal analytics; prefer HID variants for privacy in APIs
	PrimaryLanguageOfActor(ctx context.Context, actorID int64, w LangWindow) (lang string, ok bool, err error)
	LanguagesOfActor(ctx context.Context, actorID int64, w LangWindow) (byWeight map[string]int64, err error)

	// Privacy-first variants (by hashed id from utterances.actor_hid)
	PrimaryLanguageOfActorHID(ctx context.Context, actorHID []byte, w LangWindow) (lang string, ok bool, err error)
	LanguagesOfActorHID(ctx context.Context, actorHID []byte, w LangWindow) (byWeight map[string]int64, err error)
}
