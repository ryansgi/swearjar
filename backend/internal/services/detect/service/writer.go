package service

import (
	"context"
	"strings"

	"swearjar/internal/core/detector"
	"swearjar/internal/core/rulepack"
	str "swearjar/internal/platform/strings"
	dom "swearjar/internal/services/detect/domain"
	hitsdom "swearjar/internal/services/hits/domain"
)

// WriterConfig controls detector stamping
type WriterConfig struct {
	Version int // detector_version to stamp into hits
	DryRun  bool
}

// WriterService implements domain.WriterPort
type WriterService struct {
	cfg WriterConfig
	det *detector.Detector
	hw  hitsdom.WriterPort // dependency: hits writer
}

// NewWriter constructs the detect writer service
func NewWriter(hw hitsdom.WriterPort, cfg WriterConfig) *WriterService {
	rp, err := rulepack.Load()
	if err != nil {
		panic(err)
	}
	return &WriterService{
		cfg: cfg,
		det: detector.NewWithOptions(rp, cfg.Version, detector.Options{
			MaxTotalHits:              0,
			AllowOverlapping:          false,
			ContextWindow:             64,
			SeverityDeltaInCodeFence:  -1,
			SeverityDeltaInCodeInline: -1,
			SeverityDeltaInQuote:      -1,
		}),
		hw: hw,
	}
}

// Write implements domain.WriterPort
func (s *WriterService) Write(ctx context.Context, xs []dom.WriteInput) (int, error) {
	type key struct {
		id   string // utterance_id
		term string
		a, b int // span_start, span_end
	}
	type ranked struct {
		srcRank int              // template:2, lemma:1
		catRank int              // bot_rage > tooling_rage > lang_rage > self_own > generic
		hw      hitsdom.HitWrite // candidate hit
	}

	best := make(map[key]ranked, len(xs)*2)

	for _, u := range xs {
		if u.UtteranceID == "" || u.TextNorm == "" {
			continue
		}

		matches := s.det.Scan(u.TextNorm)
		lang := str.Deref(u.LangCode) // "" => repo writes NULL

		for _, m := range matches {
			srcRank := 1
			if m.Source == detector.SourceTemplate {
				srcRank = 2
			}
			cat := mapCategory(m.Category)
			sev := mapSeverity(m.Severity)
			cRank := categoryRank(cat)

			for _, sp := range m.Spans {
				k := key{u.UtteranceID, m.Term, sp[0], sp[1]}

				// Optional targeting -> pointers for Nullable columns
				var tName *string
				if strings.TrimSpace(m.TargetName) != "" {
					v := m.TargetName
					tName = &v
				}
				var tStart, tEnd, tDist *int
				if m.TargetStart > 0 || m.TargetEnd > 0 || m.TargetDistance != 0 {
					ts, te, td := m.TargetStart, m.TargetEnd, m.TargetDistance
					tStart, tEnd, tDist = &ts, &te, &td
				}

				cand := hitsdom.HitWrite{
					UtteranceID:     u.UtteranceID,
					CreatedAt:       u.CreatedAt.UTC(),
					Source:          u.Source,
					RepoHID:         u.RepoHID,
					ActorHID:        u.ActorHID,
					LangCode:        lang,   // "" => NULL
					Term:            m.Term, // normalized term
					Category:        cat,
					Severity:        sev,
					SpanStart:       sp[0],
					SpanEnd:         sp[1],
					DetectorVersion: s.cfg.Version,

					DetectorSource: string(m.Source),
					PreContext:     m.Pre,
					PostContext:    m.Post,
					Zones:          append([]string(nil), m.Zones...),

					// NEW: context gating / targeting
					CtxAction:       strings.TrimSpace(m.CtxAction),
					TargetType:      strings.TrimSpace(m.TargetType),
					TargetID:        strings.TrimSpace(m.TargetID),
					TargetName:      tName,
					TargetSpanStart: tStart,
					TargetSpanEnd:   tEnd,
					TargetDistance:  tDist,
				}

				if cand.CtxAction == "" {
					cand.CtxAction = "none"
				}
				if cand.TargetType == "" {
					cand.TargetType = "none"
				}

				if cur, ok := best[k]; !ok || srcRank > cur.srcRank || (srcRank == cur.srcRank && cRank > cur.catRank) {
					best[k] = ranked{srcRank: srcRank, catRank: cRank, hw: cand}
				}
			}
		}
	}

	if len(best) == 0 {
		return 0, nil
	}

	out := make([]hitsdom.HitWrite, 0, len(best))
	for _, v := range best {
		out = append(out, v.hw)
	}

	if s.cfg.DryRun || len(out) == 0 {
		return len(out), nil
	}
	if err := s.hw.WriteBatch(ctx, out); err != nil {
		return 0, err
	}
	return len(out), nil
}

// WriteOne implements domain.WriterPort
func (s *WriterService) WriteOne(ctx context.Context, x dom.WriteInput) error {
	_, err := s.Write(ctx, []dom.WriteInput{x})
	return err
}

// mapSeverity coerces rulepack severities into the DB enum
func mapSeverity(n int) string {
	if n <= 1 {
		return "mild"
	}
	// reserve "slur_masked" for future specialized pack
	return "strong"
}

// mapCategory coerces rulepack categories into the DB enum
func mapCategory(c string) string {
	switch c {
	case "bot_rage", "tooling_rage", "self_own", "generic", "lang_rage":
		return c
	case "harassment":
		return "generic"
	default:
		return "generic"
	}
}

func categoryRank(c string) int {
	switch c {
	case "bot_rage":
		return 5
	case "tooling_rage":
		return 4
	case "lang_rage":
		return 3
	case "self_own":
		return 2
	default:
		return 1 // "generic" & others
	}
}
