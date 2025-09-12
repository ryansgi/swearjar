package ingest

import (
	"swearjar/internal/adapters/ingest/extract"
	sjnorm "swearjar/internal/core/normalize"
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

		// Sanitize raw-ish fields before persisting
		repo := sjnorm.Sanitize(u.Repo)
		actor := sjnorm.Sanitize(u.Actor)
		source := sjnorm.Sanitize(u.Source)          // mostly internal values, still safe to sanitize
		sourceDet := sjnorm.Sanitize(u.SourceDetail) // may contain titles, paths, etc.
		textRaw := sjnorm.Sanitize(u.TextRaw)

		out = append(out, domain.Utterance{
			UtteranceID:    u.UtteranceID,
			EventType:      u.EventType,
			Repo:           repo,
			RepoID:         env.Repo.ID,
			Actor:          actor,
			ActorID:        env.Actor.ID,
			CreatedAt:      u.CreatedAt,
			Source:         source,
			SourceDetail:   sourceDet,
			TextRaw:        textRaw,
			TextNormalized: u.TextNormalized, // already sanitized via Normalizer.Normalize
			Ordinal:        u.Ordinal,
		})
	}
	return out
}

type normalizerAdapter struct{ domain.Normalizer }

func (a normalizerAdapter) Normalize(s string) string { return a.Normalizer.Normalize(s) }
