// Package service implements the detect service
package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"swearjar/internal/core/detector"
	"swearjar/internal/core/rulepack"
	str "swearjar/internal/platform/strings"
	hitsdom "swearjar/internal/services/hits/domain"
	utdom "swearjar/internal/services/utterances/domain"
)

// Config for the detect service
type Config struct {
	Version       int
	Workers       int
	PageSize      int
	MaxRangeHours int // 0 = unlimited
	DryRun        bool
}

// Service implements domain.RunnerPort
type Service struct {
	Utters utdom.ReaderPort
	Hits   hitsdom.WriterPort
	Det    *detector.Detector
	Cfg    Config
}

// New constructs a new detect service
func New(utters utdom.ReaderPort, hits hitsdom.WriterPort, rp *rulepack.Pack, cfg Config) *Service {
	w := cfg.Workers
	if w <= 0 {
		w = 1
	}
	ps := cfg.PageSize
	if ps <= 0 {
		ps = 5000
	}

	det := detector.NewWithOptions(rp, cfg.Version, detector.Options{
		MaxTotalHits:              8000,
		AllowOverlapping:          false,
		ContextWindow:             64,
		SeverityDeltaInCodeFence:  -1,
		SeverityDeltaInCodeInline: -1,
		SeverityDeltaInQuote:      -1,
	})

	return &Service{
		Utters: utters,
		Hits:   hits,
		Det:    det,
		Cfg: Config{
			Version:       cfg.Version,
			Workers:       w,
			PageSize:      ps,
			MaxRangeHours: cfg.MaxRangeHours,
			DryRun:        cfg.DryRun,
		},
	}
}

// RunRange processes utterances in the given time range, detecting hits and writing them to the hits service
func (s *Service) RunRange(ctx context.Context, start, end time.Time) error {
	start = start.Truncate(time.Hour).UTC()
	end = end.Truncate(time.Hour).UTC()
	if end.Before(start) {
		return errors.New("end before start")
	}
	if s.Cfg.MaxRangeHours > 0 && int(end.Sub(start).Hours()) > s.Cfg.MaxRangeHours {
		return errors.New("range exceeds MaxRangeHours")
	}

	// ranking: template > lemma; bot_rage > tooling_rage > lang_rage > self_own > generic; then severity
	srcPri := func(src detector.Source) int {
		if src == detector.SourceTemplate {
			return 2
		}
		return 1
	}
	catPri := func(cat string) int {
		switch cat {
		case "bot_rage":
			return 500
		case "tooling_rage":
			return 400
		case "lang_rage":
			return 300
		case "self_own":
			return 200
		case "generic":
			return 100
		default:
			return 0
		}
	}
	type spanKey struct {
		start, end int
		term       string
	}

	after := utdom.AfterKey{}
	for {
		rows, next, err := s.Utters.List(ctx, utdom.ListInput{
			Since: start, Until: end,
			After: after, Limit: s.Cfg.PageSize,
		})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		type chunk struct{ xs []hitsdom.HitWrite }
		out := make([]chunk, len(rows))

		sem := make(chan struct{}, s.Cfg.Workers)
		wg := sync.WaitGroup{}

		for i := range rows {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer func() { <-sem; wg.Done() }()

				u := rows[i]
				if u.TextNorm == "" {
					return
				}

				matches := s.Det.Scan(u.TextNorm)

				// best-per-(span,term)
				type winner struct {
					score int
					hit   detector.Hit
					span  [2]int
				}
				best := make(map[spanKey]winner, len(matches))
				for _, m := range matches {
					for _, sp := range m.Spans {
						k := spanKey{start: sp[0], end: sp[1], term: m.Term}
						score := srcPri(m.Source)*10000 + catPri(m.Category)*100 + m.Severity
						if cur, ok := best[k]; !ok || score > cur.score {
							cp := m
							cp.Spans = [][2]int{sp}
							best[k] = winner{score: score, hit: cp, span: sp}
						}
					}
				}

				// IMPORTANT: propagate utterance lang exactly
				lang := str.Deref(u.LangCode)

				buf := make([]hitsdom.HitWrite, 0, len(best))
				for _, w := range best {
					m, sp := w.hit, w.span

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

					hw := hitsdom.HitWrite{
						UtteranceID:     u.ID,
						CreatedAt:       u.CreatedAt,
						Term:            m.Term,
						Category:        mapCategory(m.Category),
						Severity:        mapSeverity(m.Severity),
						SpanStart:       sp[0],
						SpanEnd:         sp[1],
						DetectorVersion: s.Cfg.Version,
						Source:          u.Source,
						RepoHID:         u.RepoHID,
						ActorHID:        u.ActorHID,
						LangCode:        lang,

						DetectorSource: string(m.Source),
						PreContext:     m.Pre,
						PostContext:    m.Post,
						Zones:          append([]string(nil), m.Zones...),

						CtxAction:       strings.TrimSpace(m.CtxAction),
						TargetType:      strings.TrimSpace(m.TargetType),
						TargetID:        strings.TrimSpace(m.TargetID),
						TargetName:      tName,
						TargetSpanStart: tStart,
						TargetSpanEnd:   tEnd,
						TargetDistance:  tDist,
					}
					if hw.CtxAction == "" {
						hw.CtxAction = "none"
					}
					if hw.TargetType == "" {
						hw.TargetType = "none"
					}

					buf = append(buf, hw)
				}
				out[i] = chunk{xs: buf}
			}(i)
		}
		wg.Wait()

		if !s.Cfg.DryRun {
			flat := make([]hitsdom.HitWrite, 0, 512)
			for i := range out {
				flat = append(flat, out[i].xs...)
			}
			if len(flat) > 0 {
				if err := s.Hits.WriteBatch(ctx, flat); err != nil {
					return err
				}
			}
		}

		after = next
	}
}
