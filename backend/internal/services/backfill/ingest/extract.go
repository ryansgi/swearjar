package ingest

import (
	"swearjar/internal/adapters/ingest/extract"
	"swearjar/internal/services/backfill/domain"
)

type extractor struct{}

// NewExtractor constructs a new Extractor
func NewExtractor() domain.Extractor { return extractor{} }

// FromEvent extracts utterances from a gharchive event envelope
func (extractor) FromEvent(env domain.EventEnvelope, n domain.Normalizer) []domain.Utterance {
	us := extract.FromEvent(env, normalizerAdapter{n})

	out := make([]domain.Utterance, 0, len(us))
	for i := range us {
		u := us[i]
		out = append(out, domain.Utterance{
			EventID:   u.EventID,
			EventType: u.EventType,

			Repo:    u.Repo,
			RepoID:  env.Repo.ID, // <-- set from envelope
			Actor:   u.Actor,
			ActorID: env.Actor.ID, // <-- set from envelope

			CreatedAt:    u.CreatedAt,
			Source:       u.Source,
			SourceDetail: u.SourceDetail,

			TextRaw:        u.TextRaw,
			TextNormalized: u.TextNormalized,
			LangCode:       u.LangCode,
			Script:         u.Script,
		})
	}
	return out
}

type normalizerAdapter struct{ domain.Normalizer }

func (a normalizerAdapter) Normalize(s string) string { return a.Normalizer.Normalize(s) }
