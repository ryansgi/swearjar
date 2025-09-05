package service

import (
	"context"

	"swearjar/internal/core/detector"
	"swearjar/internal/core/rulepack"
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
		det: detector.New(rp, cfg.Version),
		hw:  hw,
	}
}

// Write implements domain.WriterPort
func (s *WriterService) Write(ctx context.Context, xs []dom.WriteInput) (int, error) {
	out := make([]hitsdom.HitWrite, 0, len(xs)*2)
	for _, u := range xs {
		if u.UtteranceID == "" || u.TextNorm == "" {
			continue
		}

		// SAFE: lang may be nil
		lang := ""
		if u.LangCode != nil {
			lang = *u.LangCode
		}

		matches := s.det.Scan(u.TextNorm)
		for _, m := range matches {
			cat := mapCategory(m.Category)
			sev := mapSeverity(m.Severity)
			for _, sp := range m.Spans {
				out = append(out, hitsdom.HitWrite{
					UtteranceID:     u.UtteranceID,
					Term:            m.Term,
					Category:        cat,
					Severity:        sev,
					SpanStart:       sp[0],
					SpanEnd:         sp[1],
					DetectorVersion: s.cfg.Version,
					CreatedAt:       u.CreatedAt.UTC(),
					Source:          u.Source,
					RepoName:        u.RepoName,
					RepoHID:         u.RepoHID,
					ActorHID:        u.ActorHID,
					LangCode:        lang,
				})
			}
		}
	}
	if len(out) == 0 {
		return 0, nil
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
// @TODO: make DB enum match rulepack exactly
func mapSeverity(n int) string {
	if n <= 1 {
		return "mild"
	}
	return "strong" // reserve "slur_masked" for a future specialized pack
}

// mapCategory coerces rulepack categories into the DB enum
// @TODO: make DB enum match rulepack exactly
func mapCategory(c string) string {
	switch c {
	case "bot_rage", "tooling_rage", "self_own", "generic":
		return c
	case "harassment":
		return "generic"
	default:
		return "generic"
	}
}
