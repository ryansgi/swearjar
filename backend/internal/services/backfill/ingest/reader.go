package ingest

import (
	"io"

	"swearjar/internal/platform/logger"
	"swearjar/internal/services/backfill/domain"

	"swearjar/internal/adapters/ingest/gharchive"
)

// readerFactory adapts gharchive.NewReader to the domain.ReaderFactory
type readerFactory struct{}

// NewReaderFactory returns a factory that wraps gharchive.NewReader
func NewReaderFactory() domain.ReaderFactory { return readerFactory{} }

func (readerFactory) New(rc io.ReadCloser) (domain.ReaderPort, error) {
	r, err := gharchive.NewReader(rc, gharchive.ReaderOptions{
		LogFirstLine:     false,
		FailOnFirstError: true,
	})
	if err != nil {
		return nil, err
	}
	return &reader{r: r}, nil
}

type reader struct {
	r interface {
		Next() (gharchive.EventEnvelope, error)
		Close() error
		// Optional stats hook: if not present, zero values returned
		Stats() (int, int64)
	}
}

func (r *reader) Next() (domain.EventEnvelope, error) {
	ev, err := r.r.Next()

	// I think this should be part of the backfill service, but I'm not sure.
	// Leaving here for now
	if ev.Actor.ID == 0 || ev.Repo.ID == 0 {
		ev.FillSyntheticIDs()

		// logger.Get().Debug().
		// 	Str("type", ev.Type).
		// 	Str("repo", ev.Repo.Name).
		// 	Int64("actor", ev.Actor.ID).
		// 	Int64("repo_id", ev.Repo.ID).
		// 	Str("actor_id", ev.Actor.Login).
		// 	Time("created_at", ev.CreatedAt).
		// 	Str("payload", string(ev.RawPayload)).
		// 	Msg("gharchive: processed")
	}
	if ev.ID == "" || ev.ID == "0" {
		logger.Get().Debug().
			Str("type", ev.Type).
			Str("repo", ev.Repo.Name).
			Int64("actor", ev.Actor.ID).
			Int64("repo_id", ev.Repo.ID).
			Str("actor_id", ev.Actor.Login).
			Time("created_at", ev.CreatedAt).
			Str("payload", string(ev.RawPayload)).
			Msg("gharchive: processed")
	}

	// domain.EventEnvelope is an alias to gharchive.EventEnvelope; return directly
	return ev, err
}

func (r *reader) Close() error { return r.r.Close() }

func (r *reader) Stats() (events int, bytes int64) {
	// If underlying reader lacks Stats, the zero interface will panic; we guard by type assertion
	type statser interface{ Stats() (int, int64) }
	if s, ok := any(r.r).(statser); ok {
		return s.Stats()
	}
	return 0, 0
}
